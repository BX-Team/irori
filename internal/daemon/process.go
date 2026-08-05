package daemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/bx-team/irori/internal/java"
	"github.com/bx-team/irori/internal/launch"
	"github.com/bx-team/irori/internal/models"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	sigTerm = syscall.SIGTERM
	sigKill = syscall.SIGKILL
)

type proc struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	pid     int
	started time.Time
	exited  chan struct{}

	stopping bool
	prevCPU  float64
	prevAt   time.Time
}

func (d *Daemon) doStart() error {
	d.mu.Lock()
	if d.proc != nil {
		d.mu.Unlock()
		return errors.New("server is already running")
	}
	d.mu.Unlock()

	if err := d.preflight(); err != nil {
		d.note("start failed: %v", err)
		d.setState(models.StateCrashed, err.Error())
		return err
	}

	required := d.cfg.JavaMajor()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	jdk, err := java.ResolveCached(ctx, d.cfg.StateDir(), d.cfg.Java.Path, required)
	cancel()
	if err != nil {
		msg := fmt.Sprintf("%v. %s", err, java.InstallHint(required))
		d.note("%s", msg)
		d.setState(models.StateCrashed, msg)
		return errors.New(msg)
	}

	// A start that silently used the wrong runtime is hard to diagnose from the
	// crash the server prints, so record which JDK was picked and why.
	d.note("java %s from %s (%s)", jdk.Display(), jdk.Source, jdk.Path)
	if required > 0 && jdk.Major < required {
		d.note("warning: this server needs Java %d or newer", required)
	}

	spec, err := launch.Build(d.cfg, jdk.Path)
	if err != nil {
		d.note("could not build command line: %v", err)
		return err
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	cmd := exec.Command(spec.Java, spec.Args...)
	cmd.Dir = d.cfg.Dir()
	cmd.Stdout = pw
	cmd.Stderr = pw
	newProcessGroup(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		pr.Close()
		pw.Close()
		return err
	}

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return fmt.Errorf("could not start java: %w", err)
	}
	// The parent's copy of the write end must go, otherwise the reader never
	// sees EOF when the server exits.
	pw.Close()

	p := &proc{
		cmd:     cmd,
		stdin:   stdin,
		pid:     cmd.Process.Pid,
		started: time.Now(),
		exited:  make(chan struct{}),
		prevAt:  time.Now(),
	}

	d.mu.Lock()
	d.proc = p
	d.players = map[string]bool{}
	d.status = models.Status{
		State: models.StateStarting,
		PID:   p.pid,
		Since: p.started,
		Stats: models.Stats{MaxPlayer: readMaxPlayers(d.cfg)},
	}
	d.mu.Unlock()

	d.note("java %s (%s), heap %s -> %s, flags %s",
		jdk.Display(), jdk.Source, d.cfg.Java.Xms, d.cfg.Java.Xmx, d.cfg.Java.Preset)
	d.note("exec: %s", spec.String())
	d.broadcastState()

	go d.scan(pr)
	go func() {
		err := cmd.Wait()
		close(p.exited)
		d.cmds <- command{kind: "exited", err: err}
	}()
	return nil
}

func (d *Daemon) preflight() error {
	jar := d.cfg.JarPath()
	if fi, err := os.Stat(jar); err != nil || fi.IsDir() {
		return fmt.Errorf("server jar not found: %s", d.cfg.Server.Jar)
	}
	if d.cfg.Server.Type.IsProxy() || d.cfg.Server.Type.IsLimbo() {
		return nil
	}
	eula := filepath.Join(d.cfg.Dir(), "eula.txt")
	raw, err := os.ReadFile(eula)
	if err != nil || !strings.Contains(string(raw), "eula=true") {
		return errors.New("EULA not accepted: eula.txt must contain eula=true")
	}
	return nil
}

func (d *Daemon) scan(r io.ReadCloser) {
	defer r.Close()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		d.onConsoleLine(strings.TrimRight(sc.Text(), "\r"))
	}
}

func (d *Daemon) doInput(text string) error {
	d.mu.Lock()
	p := d.proc
	d.mu.Unlock()
	if p == nil {
		return errors.New("server is not running")
	}
	if _, err := io.WriteString(p.stdin, text+"\n"); err != nil {
		return fmt.Errorf("could not send command: %w", err)
	}
	d.emit("> "+text, models.LevelIrori)
	return nil
}

func (d *Daemon) doStop(force bool) {
	d.mu.Lock()
	p := d.proc
	if p == nil || p.stopping {
		d.mu.Unlock()
		return
	}
	p.stopping = true
	d.status.State = models.StateStopping
	d.mu.Unlock()
	d.broadcastState()

	go d.stopSequence(p, force)
}

// stopSequence escalates: console command, then SIGTERM, then SIGKILL.
func (d *Daemon) stopSequence(p *proc, force bool) {
	if !force {
		d.note("stopping server with %q", d.cfg.Runtime.StopCommand)
		if _, err := io.WriteString(p.stdin, d.cfg.Runtime.StopCommand+"\n"); err == nil {
			select {
			case <-p.exited:
				return
			case <-time.After(time.Duration(d.cfg.Runtime.StopTimeoutSec) * time.Second):
			}
		}
		d.note("server did not exit within %ds, sending SIGTERM", d.cfg.Runtime.StopTimeoutSec)
	}

	p.signalGroup(sigTerm)
	select {
	case <-p.exited:
		return
	case <-time.After(15 * time.Second):
	}

	d.note("process is unresponsive, sending SIGKILL")
	p.signalGroup(sigKill)
}

func (d *Daemon) doKill() {
	d.mu.Lock()
	p := d.proc
	if p != nil {
		p.stopping = true
		d.status.State = models.StateStopping
	}
	d.mu.Unlock()
	if p == nil {
		return
	}
	d.note("force killing (SIGKILL)")
	p.signalGroup(sigKill)
	d.broadcastState()
}

func (d *Daemon) doRestart() {
	d.mu.Lock()
	running := d.proc != nil
	if running {
		d.restarts = -1
	}
	d.mu.Unlock()
	if !running {
		_ = d.doStart()
		return
	}
	d.doStop(false)
}

func (d *Daemon) onExit(err error) {
	d.mu.Lock()
	p := d.proc
	d.proc = nil
	intentional := p != nil && p.stopping
	wantRestart := d.restarts == -1
	d.restarts = 0
	d.players = map[string]bool{}
	d.status.PID = 0
	d.status.Stats = models.Stats{MaxPlayer: d.status.Stats.MaxPlayer}
	d.idleAt = time.Now()
	autoRestart := d.cfg.Runtime.AutoRestart
	d.mu.Unlock()

	code := exitCode(err)
	switch {
	case wantRestart:
		d.note("server stopped (exit %d), restarting", code)
		d.setState(models.StateStopped, "")
		time.AfterFunc(time.Second, func() { d.cmds <- command{kind: "start"} })
	case intentional:
		d.note("server stopped (exit %d)", code)
		d.setState(models.StateStopped, "")
		d.Quit()
	case autoRestart:
		d.note("server crashed (exit %d), auto-restarting in %s", code, restartDelay)
		d.setState(models.StateCrashed, fmt.Sprintf("exit code %d", code))
		time.AfterFunc(restartDelay, func() { d.cmds <- command{kind: "start"} })
	default:
		d.note("server exited with code %d", code)
		d.setState(models.StateCrashed, fmt.Sprintf("exit code %d", code))
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func (d *Daemon) setState(s models.ServerState, lastErr string) {
	d.mu.Lock()
	d.status.State = s
	d.status.LastErr = lastErr
	if !s.IsUp() {
		d.status.PID = 0
	}
	d.mu.Unlock()
	d.broadcastState()
}

func (d *Daemon) sampleStats() {
	d.mu.Lock()
	p := d.proc
	d.mu.Unlock()
	if p == nil {
		return
	}

	ps, err := process.NewProcess(int32(p.pid))
	if err != nil {
		return
	}

	var cpuPct float64
	if times, err := ps.Times(); err == nil {
		cpuSec := times.User + times.System
		now := time.Now()
		if !p.prevAt.IsZero() && cpuSec >= p.prevCPU {
			elapsed := now.Sub(p.prevAt).Seconds()
			if elapsed > 0 {
				cpuPct = (cpuSec - p.prevCPU) / elapsed / float64(runtime.NumCPU()) * 100
			}
		}
		p.prevCPU, p.prevAt = cpuSec, now
	}

	var rssMB float64
	if mem, err := ps.MemoryInfo(); err == nil && mem != nil {
		rssMB = float64(mem.RSS) / (1024 * 1024)
	}

	d.mu.Lock()
	d.status.Stats.CPU = cpuPct
	d.status.Stats.RSSMB = rssMB
	d.status.Stats.UptimeSec = int64(time.Since(p.started).Seconds())
	d.status.Stats.Players = len(d.players)
	st := d.status.Stats
	d.mu.Unlock()

	d.broadcast(ipcStatsFrame(st))
}
