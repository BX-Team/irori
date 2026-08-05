// Package link keeps the TUI's long-lived connection to the irori daemon and
// turns incoming frames into bubbletea messages.
//
// A reader goroutine owns the socket and pushes messages onto a channel; the
// model pulls exactly one per Wait() command, which is the standard bubbletea
// way to consume an unbounded stream without blocking Update.
package link

import (
	"errors"
	"sync"

	"github.com/bx-team/irori/internal/ipc"
	"github.com/bx-team/irori/internal/ui/msgs"
	tea "github.com/charmbracelet/bubbletea"
)

const inboxSize = 512

type Link struct {
	socket string
	tail   int

	mu   sync.Mutex
	conn *ipc.Conn
	gen  uint64

	inbox chan tea.Msg
}

func New(socket string, tail int) *Link {
	return &Link{socket: socket, tail: tail, inbox: make(chan tea.Msg, inboxSize)}
}

func (l *Link) Socket() string { return l.socket }

func (l *Link) Connected() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.conn != nil
}

// Connect dials the daemon and attaches; on failure it reports LinkDownMsg
// rather than an error, since "no daemon" is the normal stopped state.
func (l *Link) Connect() tea.Cmd {
	return func() tea.Msg {
		conn, err := ipc.Dial(l.socket)
		if err != nil {
			return msgs.LinkDownMsg{Err: err}
		}
		if err := conn.Send(ipc.Frame{T: ipc.Attach, Tail: l.tail}); err != nil {
			_ = conn.Close()
			return msgs.LinkDownMsg{Err: err}
		}

		l.mu.Lock()
		if l.conn != nil {
			_ = l.conn.Close()
		}
		l.conn = conn
		l.gen++
		gen := l.gen
		l.mu.Unlock()

		go l.read(conn, gen)
		return msgs.LinkUpMsg{}
	}
}

func (l *Link) read(conn *ipc.Conn, gen uint64) {
	for {
		f, err := conn.Recv()
		if err != nil {
			l.mu.Lock()
			stale := l.gen != gen
			if !stale {
				l.conn = nil
			}
			l.mu.Unlock()
			if !stale {
				l.push(msgs.LinkDownMsg{Err: err})
			}
			return
		}
		switch f.T {
		case ipc.Log:
			if f.Line != nil {
				l.push(msgs.LogMsg{Line: *f.Line})
			}
			if len(f.Lines) > 0 {
				l.push(msgs.HistoryMsg{Lines: f.Lines})
			}
		case ipc.State:
			if f.Status != nil {
				l.push(msgs.StatusMsg{Status: *f.Status})
			}
		case ipc.Stats:
			if f.Stats != nil {
				l.push(msgs.StatsMsg{Stats: *f.Stats})
			}
		case ipc.Error:
			l.push(msgs.ErrorMsg{Title: "Daemon", Detail: f.Err})
		}
	}
}

func (l *Link) push(m tea.Msg) {
	select {
	case l.inbox <- m:
	default:
	}
}

// Wait yields the next message from the daemon. Re-issue it after every
// received message to keep the stream flowing.
func (l *Link) Wait() tea.Cmd {
	return func() tea.Msg { return <-l.inbox }
}

func (l *Link) Send(f ipc.Frame) tea.Cmd {
	return func() tea.Msg {
		l.mu.Lock()
		conn := l.conn
		l.mu.Unlock()
		if conn == nil {
			return msgs.LinkDownMsg{Err: errors.New("not connected to the daemon")}
		}
		if err := conn.Send(f); err != nil {
			return msgs.LinkDownMsg{Err: err}
		}
		return nil
	}
}

func (l *Link) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn != nil {
		_ = l.conn.Close()
		l.conn = nil
		l.gen++
	}
}
