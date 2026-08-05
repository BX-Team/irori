// Package ipc defines the newline-delimited JSON protocol spoken between the
// irori daemon and its clients (the TUI and the CLI) over a unix socket.
//
// One Frame type carries every message in both directions; only the fields
// relevant to T are populated. Several clients may be attached at once, and the
// daemon broadcasts log/state/stats frames to all of them.
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"

	"github.com/bx-team/irori/internal/models"
)

const (
	// client -> daemon
	Attach   = "attach"
	Input    = "input"
	Start    = "start"
	Stop     = "stop"
	Restart  = "restart"
	Kill     = "kill"
	GetState = "status"
	Shutdown = "shutdown"
	Ping     = "ping"

	// daemon -> client
	Log   = "log"
	State = "state"
	Stats = "stats"
	Ack   = "ack"
	Error = "error"
)

type Frame struct {
	T string `json:"t"`

	Tail int    `json:"tail,omitempty"`
	Data string `json:"data,omitempty"`

	Line   *models.LogLine  `json:"line,omitempty"`
	Lines  []models.LogLine `json:"lines,omitempty"`
	Status *models.Status   `json:"status,omitempty"`
	Stats  *models.Stats    `json:"stats,omitempty"`
	Err    string           `json:"err,omitempty"`
}

type Conn struct {
	net  net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	read *bufio.Reader
}

func Wrap(c net.Conn) *Conn {
	r := bufio.NewReaderSize(c, 64*1024)
	return &Conn{net: c, enc: json.NewEncoder(c), dec: json.NewDecoder(r), read: r}
}

func Dial(socket string) (*Conn, error) {
	c, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return nil, err
	}
	return Wrap(c), nil
}

func (c *Conn) Send(f Frame) error { return c.enc.Encode(f) }

func (c *Conn) Recv() (Frame, error) {
	var f Frame
	err := c.dec.Decode(&f)
	return f, err
}

func (c *Conn) Close() error { return c.net.Close() }

func (c *Conn) SetDeadline(t time.Time) error { return c.net.SetDeadline(t) }

func Alive(socket string) bool {
	c, err := Dial(socket)
	if err != nil {
		return false
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if err := c.Send(Frame{T: Ping}); err != nil {
		return false
	}
	f, err := c.Recv()
	return err == nil && (f.T == Ack || f.T == State)
}

func Query(socket string, req Frame) (Frame, error) {
	c, err := Dial(socket)
	if err != nil {
		return Frame{}, err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if err := c.Send(req); err != nil {
		return Frame{}, err
	}
	for {
		f, err := c.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return Frame{}, errors.New("daemon closed the connection without replying")
			}
			return Frame{}, err
		}
		switch f.T {
		case State, Ack, Error:
			return f, nil
		}
	}
}
