//go:build windows

package daemon

import (
	"os/exec"
	"strconv"
	"syscall"
)

// DETACHED_PROCESS. syscall exposes CREATE_NEW_PROCESS_GROUP but not this one.
const detachedProcess = 0x00000008

func newProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func newSession(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
		HideWindow:    true,
	}
}

// signalGroup has no signals to send here, so it shells out to taskkill, whose
// /T walks the tree the way killing a process group does. Only sigKill maps to
// something that reliably ends a JVM: the graceful path is the console command
// the stop sequence already tried before reaching this.
func (p *proc) signalGroup(sig syscall.Signal) {
	if p.pid <= 0 {
		return
	}
	args := []string{"/T", "/PID", strconv.Itoa(p.pid)}
	if sig == sigKill {
		args = append([]string{"/F"}, args...)
	}
	kill := exec.Command("taskkill", args...)
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = kill.Run()
}
