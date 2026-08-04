package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// maxSocketPath keeps addresses below the 108-byte sun_path limit with room to
// spare; longer paths get hashed into the runtime dir instead.
const maxSocketPath = 100

func SocketPathFor(serverDir string) string {
	local := filepath.Join(serverDir, StateDirName, "daemon.sock")
	if len(local) <= maxSocketPath {
		return local
	}
	sum := sha256.Sum256([]byte(serverDir))
	return filepath.Join(runtimeDir(), "irori", hex.EncodeToString(sum[:])[:12]+".sock")
}

func runtimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return os.TempDir()
}

// Sealed reports that irori must not download anything. The NixOS module sets
// it, because there the core and every plugin come from the Nix store.
func Sealed() bool {
	switch os.Getenv("IRORI_SEALED") {
	case "1", "true", "yes":
		return true
	}
	return false
}

func UserConfigDir() string {
	if d := os.Getenv("IRORI_CONFIG_DIR"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "irori")
}

func UserCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "irori")
}
