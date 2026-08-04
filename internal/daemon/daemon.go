package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/ipc"
	"github.com/bx-team/irori/internal/launch"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/props"
)

const (
	ringCapacity  = 5000
	statsInterval = 2 * time.Second
	idleGrace     = 30 * time.Second
	restartDelay  = 3 * time.Second
	maxLogBytes   = 32 << 20
)

type Options struct {
	Dir     string
	NoStart bool
}

type Daemon struct {
	cfg  *config.Config
	ring *Ring
	logw *os.File
	ln   net.Listener

	mu      sync.Mutex
	clients map[*client]bool
	status  models.Status
	players map[string]bool
	seq     uint64

	proc     *proc
	xmxMB    int
	restarts int

	cmds     chan command
	quit     chan struct{}
	quitOnce sync.Once
	idleAt   time.Time
}

type command struct {
	kind  string
	data  string
	force bool
	err   error
	reply chan error
}

func Run(opts Options) error {
	cfg, err := config.Load(opts.Dir)
	if err != nil {
		return err
	}
	if err := cfg.EnsureStateDir(); err != nil {
		return err
	}

	socket := cfg.SocketPath()
	if ipc.Alive(socket) {
		return errors.New("a daemon is already running for this directory")
	}
	_ = os.Remove(socket)
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		return err
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return fmt.Errorf("could not open socket %s: %w", socket, err)
	}

	d := &Daemon{
		cfg:     cfg,
		ring:    NewRing(ringCapacity),
		ln:      ln,
		clients: map[*client]bool{},
		players: map[string]bool{},
		status:  models.Status{State: models.StateStopped},
		cmds:    make(chan command, 64),
		quit:    make(chan struct{}),
		idleAt:  time.Now(),
	}
	d.xmxMB, _ = launch.ParseMemMB(cfg.Java.Xmx)
	d.status.Stats.MaxPlayer = readMaxPlayers(cfg)
	d.openLog()
	d.writePid()

	defer func() {
		ln.Close()
		_ = os.Remove(socket)
		_ = os.Remove(cfg.PidPath())
		if d.logw != nil {
			d.logw.Close()
		}
	}()

	go d.accept()

	if !opts.NoStart {
		d.cmds <- command{kind: "start"}
	}
	d.loop()
	return nil
}

func readMaxPlayers(cfg *config.Config) int {
	f, err := props.Load(filepath.Join(cfg.Dir(), "server.properties"))
	if err != nil {
		return 20
	}
	return f.Int("max-players", 20)
}

func (d *Daemon) writePid() {
	_ = os.WriteFile(d.cfg.PidPath(), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
}

func (d *Daemon) openLog() {
	path := d.cfg.LogPath()
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLogBytes {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		d.logw = f
	}
}

func (d *Daemon) loop() {
	stats := time.NewTicker(statsInterval)
	idle := time.NewTicker(5 * time.Second)
	defer stats.Stop()
	defer idle.Stop()

	for {
		select {
		case <-d.quit:
			d.shutdownProc()
			return
		case c := <-d.cmds:
			d.handle(c)
		case <-stats.C:
			d.sampleStats()
		case <-idle.C:
			if d.shouldExit() {
				return
			}
		}
	}
}

func (d *Daemon) handle(c command) {
	var err error
	switch c.kind {
	case "start":
		err = d.doStart()
	case "stop":
		d.doStop(c.force)
	case "restart":
		d.doRestart()
	case "kill":
		d.doKill()
	case "input":
		err = d.doInput(c.data)
	case "exited":
		d.onExit(c.err)
	case "shutdown":
		d.Quit()
	}
	if c.reply != nil {
		c.reply <- err
	}
}

func (d *Daemon) shouldExit() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.proc != nil || d.status.State.IsUp() {
		d.idleAt = time.Now()
		return false
	}
	if len(d.clients) > 0 {
		d.idleAt = time.Now()
		return false
	}
	return time.Since(d.idleAt) > idleGrace
}

func (d *Daemon) Quit() {
	d.quitOnce.Do(func() { close(d.quit) })
}

func (d *Daemon) shutdownProc() {
	d.mu.Lock()
	p := d.proc
	d.mu.Unlock()
	if p == nil {
		return
	}
	d.doStop(false)
	select {
	case <-p.exited:
	case <-time.After(time.Duration(d.cfg.Runtime.StopTimeoutSec+15) * time.Second):
		p.signalGroup(sigKill)
	}
}

func (d *Daemon) emit(text string, level models.LogLevel) {
	d.mu.Lock()
	d.seq++
	l := models.LogLine{Seq: d.seq, TS: time.Now(), Text: text, Level: level}
	d.mu.Unlock()

	d.ring.Append(l)
	if d.logw != nil {
		_, _ = d.logw.WriteString(l.TS.Format("15:04:05") + " " + text + "\n")
	}
	d.broadcast(ipc.Frame{T: ipc.Log, Line: &l})
}

func (d *Daemon) note(format string, args ...any) {
	d.emit("[irori] "+fmt.Sprintf(format, args...), models.LevelIrori)
}

func (d *Daemon) onConsoleLine(text string) {
	level := detectLevel(text)
	d.emit(text, level)

	ev := detectEvent(text, d.cfg.Server.Type.IsProxy())
	changed := false
	d.mu.Lock()
	if ev.ready && d.status.State == models.StateStarting {
		d.status.State = models.StateRunning
		changed = true
	}
	if ev.stopping && d.status.State == models.StateRunning {
		d.status.State = models.StateStopping
		changed = true
	}
	if ev.joined != "" {
		d.players[ev.joined] = true
	}
	if ev.left != "" {
		delete(d.players, ev.left)
	}
	d.status.Stats.Players = len(d.players)
	d.mu.Unlock()

	if changed {
		d.broadcastState()
	}
}

func (d *Daemon) accept() {
	for {
		c, err := d.ln.Accept()
		if err != nil {
			return
		}
		go d.serveClient(ipc.Wrap(c))
	}
}

type client struct {
	conn *ipc.Conn
	mu   sync.Mutex
}

func (c *client) send(f ipc.Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Send(f)
}

func (d *Daemon) serveClient(conn *ipc.Conn) {
	c := &client{conn: conn}
	defer conn.Close()

	for {
		f, err := conn.Recv()
		if err != nil {
			d.mu.Lock()
			delete(d.clients, c)
			d.idleAt = time.Now()
			d.mu.Unlock()
			return
		}

		switch f.T {
		case ipc.Ping:
			_ = c.send(ipc.Frame{T: ipc.Ack})
		case ipc.Attach:
			d.mu.Lock()
			d.clients[c] = true
			d.mu.Unlock()
			tail := f.Tail
			if tail <= 0 {
				tail = 500
			}
			_ = c.send(ipc.Frame{T: ipc.Log, Lines: d.ring.Tail(tail)})
			_ = c.send(d.stateFrame())
		case ipc.GetState:
			_ = c.send(d.stateFrame())
		case ipc.Input:
			d.cmds <- command{kind: "input", data: f.Data}
		case ipc.Start:
			d.request(c, command{kind: "start"})
		case ipc.Stop:
			d.request(c, command{kind: "stop"})
		case ipc.Kill:
			d.request(c, command{kind: "kill"})
		case ipc.Restart:
			d.request(c, command{kind: "restart"})
		case ipc.Shutdown:
			_ = c.send(ipc.Frame{T: ipc.Ack})
			d.cmds <- command{kind: "shutdown"}
			return
		}
	}
}

func (d *Daemon) request(c *client, cmd command) {
	reply := make(chan error, 1)
	cmd.reply = reply
	d.cmds <- cmd
	select {
	case err := <-reply:
		if err != nil {
			_ = c.send(ipc.Frame{T: ipc.Error, Err: err.Error()})
			return
		}
	case <-time.After(30 * time.Second):
		_ = c.send(ipc.Frame{T: ipc.Error, Err: "command timed out"})
		return
	}
	_ = c.send(d.stateFrame())
}

func (d *Daemon) stateFrame() ipc.Frame {
	d.mu.Lock()
	st := d.status
	d.mu.Unlock()
	return ipc.Frame{T: ipc.State, Status: &st}
}

func (d *Daemon) broadcast(f ipc.Frame) {
	d.mu.Lock()
	list := make([]*client, 0, len(d.clients))
	for c := range d.clients {
		list = append(list, c)
	}
	d.mu.Unlock()

	for _, c := range list {
		if err := c.send(f); err != nil {
			d.mu.Lock()
			delete(d.clients, c)
			d.mu.Unlock()
		}
	}
}

func (d *Daemon) broadcastState() { d.broadcast(d.stateFrame()) }

func Spawn(ctx context.Context, dir string) error {
	socket := config.SocketPathFor(dir)
	if ipc.Alive(socket) {
		return nil
	}
	if err := spawnDetached(dir); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ipc.Alive(socket) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("daemon did not answer after start")
}
