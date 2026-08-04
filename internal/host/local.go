package host

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Local struct {
	root string
}

func NewLocal(root string) *Local {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &Local{root: abs}
}

func (l *Local) Name() string { return "local" }

func (l *Local) Root() string { return l.root }

// Abs resolves a path against the server root and refuses to escape it.
func (l *Local) Abs(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(l.root, filepath.Clean("/"+path))
}

func toEntry(name string, fi os.FileInfo, dir string) Entry {
	e := Entry{
		Name:    name,
		Size:    fi.Size(),
		Mode:    fi.Mode(),
		ModTime: fi.ModTime(),
		IsDir:   fi.IsDir(),
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		if t, err := os.Readlink(filepath.Join(dir, name)); err == nil {
			e.Target = t
		}
	}
	return e
}

func (l *Local) Stat(path string) (Entry, error) {
	p := l.Abs(path)
	fi, err := os.Lstat(p)
	if err != nil {
		return Entry{}, err
	}
	e := toEntry(fi.Name(), fi, filepath.Dir(p))
	if e.Target != "" {
		if rfi, err := os.Stat(p); err == nil {
			e.IsDir = rfi.IsDir()
			e.Size = rfi.Size()
		}
	}
	return e, nil
}

func (l *Local) ReadDir(path string) ([]Entry, error) {
	p := l.Abs(path)
	des, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			continue
		}
		e := toEntry(de.Name(), fi, p)
		if e.Target != "" {
			if rfi, err := os.Stat(filepath.Join(p, de.Name())); err == nil {
				e.IsDir = rfi.IsDir()
				e.Size = rfi.Size()
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func (l *Local) ReadFile(path string) ([]byte, error) { return os.ReadFile(l.Abs(path)) }

func (l *Local) WriteFile(path string, data []byte, perm fs.FileMode) error {
	p := l.Abs(path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".irori.tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (l *Local) Open(path string) (io.ReadCloser, error) { return os.Open(l.Abs(path)) }

func (l *Local) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(l.Abs(path), perm)
}

func (l *Local) Remove(path string) error { return os.Remove(l.Abs(path)) }

func (l *Local) RemoveAll(path string) error {
	p := l.Abs(path)
	if p == l.root || p == "/" {
		return errors.New("refusing to delete the server root")
	}
	return os.RemoveAll(p)
}

func (l *Local) Rename(oldPath, newPath string) error {
	dst := l.Abs(newPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(l.Abs(oldPath), dst)
}

func (l *Local) Copy(srcPath, dstPath string) error {
	src, dst := l.Abs(srcPath), l.Abs(dstPath)
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return l.copyDir(src, dst)
	}
	return copyFile(src, dst, fi.Mode())
}

func (l *Local) copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(p, target, fi.Mode())
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (l *Local) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = l.root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (l *Local) LookPath(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		p := l.Abs(name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
		return "", exec.ErrNotFound
	}
	return exec.LookPath(name)
}

var _ Backend = (*Local)(nil)
