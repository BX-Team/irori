package daemon

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bx-team/irori/internal/ipc"
	"github.com/bx-team/irori/internal/models"
)

func ipcStatsFrame(s models.Stats) ipc.Frame {
	return ipc.Frame{T: ipc.Stats, Stats: &s}
}

// spawnDetached re-executes irori in daemon mode with a new session, so the
// server survives the terminal that started it.
func spawnDetached(dir string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(self, "daemon", "--dir", dir)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	newSession(cmd)
	cmd.Env = append(os.Environ(), "IRORI_DAEMON=1")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start daemon: %w", err)
	}
	return cmd.Process.Release()
}
