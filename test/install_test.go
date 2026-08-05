package irori_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/install"
	"github.com/bx-team/irori/internal/lock"
)

// A core recorded without its checksum renders as `jar = null` and the NixOS
// host silently gets no server jar at all. Sync computes the one thing it can
// work out on its own, so the rest of the lock is enough to build from.
func TestSyncFillsInTheCoreChecksum(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "paper-1.21.4-232.jar", "not really a jar")

	cfg := config.Default(dir)
	cfg.Server.Jar = "paper-1.21.4-232.jar"
	lf := lock.New(filepath.Join(dir, config.LockFileName))
	lf.Core = &lock.Core{
		Type: "paper", Build: "232", File: "paper-1.21.4-232.jar",
		URL: "https://api.papermc.io/paper-1.21.4-232.jar", Direct: true,
	}

	issues, err := install.Sync(install.Target{H: host.NewLocal(dir), Cfg: cfg, Lock: lf})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
	if lf.Core.SHA256 == "" || lf.Core.Size != 16 {
		t.Errorf("checksum and size were not recorded: %+v", lf.Core)
	}

	saved, err := lock.Load(lf.Path())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved.Core == nil || saved.Core.SHA256 != lf.Core.SHA256 {
		t.Error("the filled-in checksum was not written back to the lock file")
	}
}

// Sync is offline, so a jar it cannot identify must be reported rather than
// guessed at: inventing an entry would hand Nix a URL that fetches something
// else entirely.
func TestSyncReportsACoreItCannotRecord(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "server.jar", "not really a jar")

	cfg := config.Default(dir)
	lf := lock.New(filepath.Join(dir, config.LockFileName))

	issues, err := install.Sync(install.Target{H: host.NewLocal(dir), Cfg: cfg, Lock: lf})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1: %+v", len(issues), issues)
	}
	if lf.Core != nil {
		t.Errorf("Sync invented a core entry: %+v", lf.Core)
	}
}

// A jar swapped by hand no longer matches the URL the lock would hand to Nix.
func TestSyncReportsAJarThatIsNotWhatTheLockRecorded(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "server.jar", "a different jar")

	cfg := config.Default(dir)
	lf := lock.New(filepath.Join(dir, config.LockFileName))
	lf.Core = &lock.Core{
		Type: "paper", File: "server.jar", URL: "https://example.invalid/server.jar",
		SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Size:   999, Direct: true,
	}

	issues, _ := install.Sync(install.Target{H: host.NewLocal(dir), Cfg: cfg, Lock: lf})
	if len(issues) != 1 {
		t.Fatalf("a swapped jar produced %d issues, want 1: %+v", len(issues), issues)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
