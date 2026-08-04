// Package host abstracts filesystem and command access to the machine the
// Minecraft server lives on. Only the local backend exists today; an ssh
// backend can be added without touching callers.
//
// Process supervision deliberately stays out of this interface: in a future
// remote mode the daemon runs on the remote host and only its socket is
// tunnelled, so nothing here needs to model a long-lived attached process.
package host

import (
	"context"
	"io"
	"io/fs"
	"time"
)

type Entry struct {
	Name    string
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
	IsDir   bool
	Target  string
}

type Backend interface {
	Name() string
	Root() string

	Abs(path string) string
	Stat(path string) (Entry, error)
	ReadDir(path string) ([]Entry, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	Open(path string) (io.ReadCloser, error)
	MkdirAll(path string, perm fs.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Rename(oldPath, newPath string) error
	Copy(srcPath, dstPath string) error

	Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)
	LookPath(name string) (string, error)
}
