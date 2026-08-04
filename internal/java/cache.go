package java

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const cacheFile = "java-cache.json"
const cacheTTL = 7 * 24 * time.Hour

type cacheEntry struct {
	JDK      JDK       `json:"jdk"`
	Required int       `json:"required"`
	At       time.Time `json:"at"`
}

// ResolveCached is Resolve with an on-disk memo, since scanning for JDKs costs
// several process spawns and runs on every server start.
func ResolveCached(ctx context.Context, stateDir, explicit string, requiredMajor int) (JDK, error) {
	if explicit != "" {
		return Resolve(ctx, explicit, requiredMajor)
	}
	path := filepath.Join(stateDir, cacheFile)
	if raw, err := os.ReadFile(path); err == nil {
		var e cacheEntry
		if json.Unmarshal(raw, &e) == nil &&
			e.Required == requiredMajor &&
			time.Since(e.At) < cacheTTL {
			if fi, err := os.Stat(e.JDK.Path); err == nil && !fi.IsDir() {
				return e.JDK, nil
			}
		}
	}

	jdk, err := Resolve(ctx, explicit, requiredMajor)
	if err != nil {
		return jdk, err
	}
	if raw, err := json.MarshalIndent(cacheEntry{JDK: jdk, Required: requiredMajor, At: time.Now()}, "", "  "); err == nil {
		_ = os.MkdirAll(stateDir, 0o755)
		_ = os.WriteFile(path, raw, 0o644)
	}
	return jdk, nil
}

func ClearCache(stateDir string) error {
	err := os.Remove(filepath.Join(stateDir, cacheFile))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
