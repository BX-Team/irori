{
  description = "irori — a TUI for running a Minecraft server";

  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

  outputs = {
    self,
    nixpkgs,
  }: let
    systems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];
    forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
  in {
    packages = forAllSystems (pkgs: rec {
      irori = pkgs.buildGoModule {
        pname = "irori";
        version = "0.1.0";
        src = ./.;

        vendorHash = "sha256-u5xa0xwOquyVxf7yQVdNKiugw25yShQkyikfEFLg7Z4=";

        subPackages = ["cmd/irori"];
        ldflags = ["-s" "-w" "-X" "main.version=0.1.0"];

        meta = {
          description = "TUI for managing a Minecraft server";
          homepage = "https://github.com/BX-Team/irori";
          mainProgram = "irori";
          platforms = nixpkgs.lib.platforms.unix;
        };
      };
      default = irori;
    });

    apps = forAllSystems (pkgs: rec {
      irori = {
        type = "app";
        program = nixpkgs.lib.getExe self.packages.${pkgs.system}.irori;
      };
      default = irori;
    });

    devShells = forAllSystems (pkgs: {
      default = pkgs.mkShell {
        packages = [pkgs.go pkgs.gopls pkgs.golangci-lint];
      };
    });

    formatter = forAllSystems (pkgs: pkgs.alejandra);

    nixosModules = rec {
      irori = import ./nix/module.nix self;
      default = irori;
    };
  };
}
