package irori_test

import (
	"strings"
	"testing"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/lock"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/nixgen"
)

// The NixOS module imports whatever this renders, so the shape is a contract:
// hex digests have to arrive as SRI, and every attribute name has to be one Nix
// will accept.
func TestGenerateRendersSRIHashesAndSafeAttrNames(t *testing.T) {
	cfg := config.Default("/srv/minecraft/survival")
	cfg.Server.Type = models.TypePaper
	cfg.Server.MCVersion = "1.21.4"

	lf := lock.New("/srv/minecraft/survival/.irori.lock.json")
	lf.Core = &lock.Core{
		Type:   "paper",
		Build:  "232",
		File:   "paper-1.21.4-232.jar",
		URL:    "https://api.papermc.io/paper-1.21.4-232.jar",
		SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Direct: true,
	}
	lf.Addons = []lock.Addon{{
		Key:    "modrinth:1u6JkXh5",
		Source: "modrinth",
		ID:     "worldedit.bukkit-7.3",
		File:   "worldedit-bukkit-7.3.6.jar",
		URL:    "https://cdn.modrinth.com/worldedit.jar",
		SHA512: "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce" +
			"47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
	}}

	out, warnings := nixgen.Generate(cfg, lf)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", warnings)
	}

	for _, want := range []string{
		`{ fetchurl }:`,
		`mcVersion = "1.21.4";`,
		`hash = "sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=";`,
		`hash = "sha512-z4PhNX7vuL3xVChQ1m2AB9Yg5AULVxXcg/SpIdNs6c5H0NE8XYXysP+DGNKHfuwvY7kxvUdBeoGlODJ6+SfaPg==";`,
		`"worldedit-bukkit-7-3" = fetchurl {`, // dots are not legal unquoted, and are replaced
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered expression is missing %s\n---\n%s", want, out)
		}
	}
}

// The configs attrset is read back by the NixOS module and handed to
// `irori apply --only config`, so it has to be Nix the module can import at
// all: dotted keys need quoting, a number must not arrive as a string, and a
// motd carrying ${…} would otherwise be read as interpolation.
func TestGenerateRendersDeclaredConfigKeys(t *testing.T) {
	cfg := config.Default("/srv/minecraft/survival")
	cfg.Server.Type = models.TypePaper
	cfg.SetOverride("server.properties", "difficulty", "hard")
	cfg.SetOverride("server.properties", "view-distance", float64(10))
	cfg.SetOverride("server.properties", "motd", "welcome ${player}")
	cfg.SetOverride("config/paper-global.yml", "chunk-system.gen-parallelism", true)

	out, warnings := nixgen.Generate(cfg, lock.New("/srv/minecraft/survival/.irori.lock.json"))
	for _, want := range []string{
		`"server.properties" = {`,
		`"difficulty" = "hard";`,
		`"view-distance" = 10;`,
		`"motd" = "welcome \${player}";`,
		`"chunk-system.gen-parallelism" = true;`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered expression is missing %s\n---\n%s", want, out)
		}
	}
	for _, w := range warnings {
		if strings.Contains(w.Target, "difficulty") {
			t.Errorf("an ordinary key was reported as a secret: %+v", w)
		}
	}
}

// /nix/store is world readable, so a password declared by hand has to be
// called out rather than quietly rendered into it.
func TestGenerateWarnsAboutSecretsInTheStore(t *testing.T) {
	cfg := config.Default("/srv/minecraft/survival")
	cfg.Server.Type = models.TypePaper
	cfg.SetOverride("server.properties", "rcon.password", "hunter2")

	_, warnings := nixgen.Generate(cfg, lock.New("/srv/minecraft/survival/.irori.lock.json"))
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Target, "rcon.password") {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning for a secret rendered into the store: %+v", warnings)
	}
}

// A core that an installer produces is not one download, so it cannot become a
// fixed-output derivation. Rendering it as one would give NixOS hosts a jar
// that never matches.
func TestGenerateRefusesToFetchAnInstallerCore(t *testing.T) {
	cfg := config.Default("/srv/minecraft/modded")
	cfg.Server.Type = models.TypeForge

	lf := lock.New("/srv/minecraft/modded/.irori.lock.json")
	lf.Core = &lock.Core{Type: "forge", Build: "52.0.16", File: "forge-installer.jar", Direct: false}

	out, warnings := nixgen.Generate(cfg, lf)
	if !strings.Contains(out, "jar = null;") {
		t.Errorf("installer core was rendered as a fetchurl:\n%s", out)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %+v", len(warnings), warnings)
	}
}
