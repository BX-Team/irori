//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

func newProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func newSession(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// signalGroup targets the whole process group: java spawns helper processes and
// a bare kill on the pid can leave them behind.
func (p *proc) signalGroup(sig syscall.Signal) {
	if p.pid <= 0 {
		return
	}
	if err := syscall.Kill(-p.pid, sig); err != nil {
		_ = syscall.Kill(p.pid, sig)
	}
}
