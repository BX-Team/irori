self: {
  config,
  lib,
  pkgs,
  ...
}: let
  inherit
    (lib)
    concatStringsSep
    filterAttrs
    getExe
    literalExpression
    mapAttrs'
    mapAttrsToList
    mkEnableOption
    mkIf
    mkOption
    nameValuePair
    optional
    optionalString
    types
    ;

  cfg = config.services.irori;
  enabled = filterAttrs (_: srv: srv.enable) cfg.servers;

  serverModule = {name, ...}: {
    options = {
      enable =
        (mkEnableOption "the irori-managed Minecraft server ${name}")
        // {default = true;};

      directory = mkOption {
        type = types.str;
        example = "/srv/minecraft/survival";
        description = ''
          The server directory. It must already hold a `.irori.json`, which the
          wizard writes the first time you run `irori` there.
        '';
      };

      artifacts = mkOption {
        type = types.nullOr types.path;
        default = null;
        example = literalExpression "./survival.nix";
        description = ''
          The expression `irori nix` renders from `.irori.lock.json`. Its jar and
          plugins are fetched into the store and symlinked into
          {option}`directory` before the server starts, and the unit runs sealed
          so irori never downloads anything itself.

          The `configs` attribute it carries is applied on top of the files the
          core generates, on every start. Keys declared there are therefore
          owned by this file: editing them on the host does not survive a
          restart.
        '';
      };

      user = mkOption {
        type = types.str;
        default = "irori";
        description = "User the server runs as. The default user is created for you.";
      };

      group = mkOption {
        type = types.str;
        default = "irori";
        description = "Group the server runs as. The default group is created for you.";
      };

      jdk = mkOption {
        type = types.package;
        default = pkgs.jdk21_headless;
        defaultText = literalExpression "pkgs.jdk21_headless";
        description = ''
          The JDK the server runs on. It is put on the unit's PATH and exported
          as JAVA_HOME, which is where irori looks first.
        '';
      };

      port = mkOption {
        type = types.port;
        default = 25565;
        description = "Port to open when {option}`openFirewall` is set. irori does not write it into server.properties.";
      };

      openFirewall = mkOption {
        type = types.bool;
        default = false;
        description = "Open {option}`port` for TCP and UDP.";
      };

      environment = mkOption {
        type = types.attrsOf types.str;
        default = {};
        example = {IRORI_CONFIG_DIR = "/var/lib/irori";};
        description = "Extra environment for the unit.";
      };
    };
  };

  # `irori nix` writes `{ fetchurl }: { name, type, mcVersion, jar, plugins,
  # configs }`, so the module only has to apply fetchurl and link what comes
  # back. `configs` is newer than the rest, hence the `or {}` below.
  artifactsOf = srv:
    if srv.artifacts == null
    then null
    else import srv.artifacts {inherit (pkgs) fetchurl;};

  linkArtifacts = srv: let
    a = artifactsOf srv;
  in
    optionalString (a != null) ''
      ${optionalString (a.jar != null) ''
        ln -sfn ${a.jar} ${srv.directory}/${a.jar.name}
      ''}
      mkdir -p ${srv.directory}/plugins
      # Sweep our own links first, or a plugin dropped from the lock would keep
      # loading forever. Real files someone put there by hand are left alone.
      find ${srv.directory}/plugins -maxdepth 1 -type l -delete
      ${concatStringsSep "\n" (mapAttrsToList
        (_: drv: "ln -sfn ${drv} ${srv.directory}/plugins/${drv.name}")
        a.plugins)}
    '';

  # The core writes its own defaults on first start, so the declared keys are
  # laid over whatever is there rather than replacing the file. irori touches
  # only these keys and leaves every comment around them alone.
  applyConfigs = name: srv: let
    a = artifactsOf srv;
    values =
      if a == null
      then {}
      else a.configs or {};
    file = pkgs.writeText "irori-${name}-configs.json" (builtins.toJSON values);
  in
    optionalString (values != {}) ''
      ${getExe cfg.package} apply --dir ${srv.directory} --only config --overrides ${file}
    '';

  unitFor = name: srv:
    nameValuePair "irori-${name}" {
      description = "Minecraft server ${name} (irori)";
      wantedBy = ["multi-user.target"];
      after = ["network-online.target"];
      wants = ["network-online.target"];

      path = [srv.jdk pkgs.coreutils pkgs.findutils];

      environment =
        {
          IRORI_SEALED = "1";
          JAVA_HOME = "${srv.jdk}";
        }
        // srv.environment;

      preStart = linkArtifacts srv + applyConfigs name srv;

      serviceConfig = {
        Type = "simple";
        User = srv.user;
        Group = srv.group;
        WorkingDirectory = srv.directory;
        ExecStart = "${getExe cfg.package} daemon --dir ${srv.directory}";
        ExecStop = "${getExe cfg.package} stop --dir ${srv.directory}";
        # Only the daemon gets the signal; it walks java through the server's
        # own stop command. A cgroup-wide TERM would hit java directly.
        KillMode = "mixed";
        TimeoutStopSec = 120;
        Restart = "on-failure";
        RestartSec = 5;

        StateDirectory = "irori";
        ReadWritePaths = [srv.directory];
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        RestrictSUIDSGID = true;
      };
    };
in {
  options.services.irori = {
    package = mkOption {
      type = types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.irori;
      defaultText = literalExpression "irori.packages.\${system}.irori";
      description = "The irori package to run.";
    };

    servers = mkOption {
      type = types.attrsOf (types.submodule serverModule);
      default = {};
      description = "Minecraft servers to supervise, one systemd unit each.";
      example = literalExpression ''
        {
          survival = {
            directory = "/srv/minecraft/survival";
            artifacts = ./survival.nix;
            openFirewall = true;
          };
        }
      '';
    };
  };

  config = mkIf (enabled != {}) {
    systemd.services = mapAttrs' unitFor enabled;

    users.users = mkIf (lib.any (srv: srv.user == "irori") (lib.attrValues enabled)) {
      irori = {
        isSystemUser = true;
        group = "irori";
        description = "irori Minecraft server user";
      };
    };

    users.groups = mkIf (lib.any (srv: srv.group == "irori") (lib.attrValues enabled)) {
      irori = {};
    };

    networking.firewall = {
      allowedTCPPorts = lib.concatMap (srv: optional srv.openFirewall srv.port) (lib.attrValues enabled);
      allowedUDPPorts = lib.concatMap (srv: optional srv.openFirewall srv.port) (lib.attrValues enabled);
    };
  };
}
