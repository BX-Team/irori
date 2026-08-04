// Package files provides directory browsing, clipboard operations and file
// previews for the Files tab, on top of a host.Backend.
package files

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bx-team/irori/internal/host"
)

type ClipMode int

const (
	ClipNone ClipMode = iota
	ClipCopy
	ClipCut
)

type Clipboard struct {
	Mode  ClipMode
	Paths []string
}

func (c Clipboard) Empty() bool { return c.Mode == ClipNone || len(c.Paths) == 0 }

func (c Clipboard) Verb() string {
	if c.Mode == ClipCut {
		return "move"
	}
	return "copy"
}

// Browser tracks one directory at a time. Paths are always slash separated and
// relative to the backend root, so "" is the server directory itself.
type Browser struct {
	h          host.Backend
	dir        string
	entries    []host.Entry
	ShowHidden bool
	Clip       Clipboard
}

func NewBrowser(h host.Backend) *Browser {
	return &Browser{h: h, ShowHidden: true}
}

func (b *Browser) Dir() string { return b.dir }

func (b *Browser) Entries() []host.Entry { return b.entries }

func (b *Browser) Backend() host.Backend { return b.h }

func (b *Browser) Load(dir string) error {
	dir = Clean(dir)
	entries, err := b.List(dir)
	if err != nil {
		return err
	}
	b.dir, b.entries = dir, entries
	return nil
}

func (b *Browser) Reload() error { return b.Load(b.dir) }

func (b *Browser) List(dir string) ([]host.Entry, error) {
	entries, err := b.h.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, e := range entries {
		if !b.ShowHidden && strings.HasPrefix(e.Name, ".") {
			continue
		}
		out = append(out, e)
	}
	Sort(out)
	return out, nil
}

func Sort(entries []host.Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		a, b := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if a == b {
			return entries[i].Name < entries[j].Name
		}
		return a < b
	})
}

func Clean(p string) string {
	p = path.Clean("/" + strings.ReplaceAll(p, "\\", "/"))
	return strings.TrimPrefix(p, "/")
}

func Join(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func Parent(p string) string {
	p = Clean(p)
	if p == "" {
		return ""
	}
	return Clean(path.Dir(p))
}

func Base(p string) string {
	if p == "" {
		return "/"
	}
	return path.Base(p)
}

func (b *Browser) Mkdir(name string) error {
	if err := validName(name); err != nil {
		return err
	}
	return b.h.MkdirAll(Join(b.dir, name), 0o755)
}

func (b *Browser) Touch(name string) error {
	if err := validName(name); err != nil {
		return err
	}
	p := Join(b.dir, name)
	if _, err := b.h.Stat(p); err == nil {
		return fmt.Errorf("%s already exists", name)
	}
	return b.h.WriteFile(p, nil, 0o644)
}

func (b *Browser) Rename(oldName, newName string) error {
	if err := validName(newName); err != nil {
		return err
	}
	dst := Join(b.dir, newName)
	if _, err := b.h.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", newName)
	}
	return b.h.Rename(Join(b.dir, oldName), dst)
}

func (b *Browser) Delete(name string) error {
	return b.h.RemoveAll(Join(b.dir, name))
}

func (b *Browser) SetClipboard(mode ClipMode, paths ...string) {
	b.Clip = Clipboard{Mode: mode, Paths: paths}
}

func (b *Browser) Paste() (int, error) {
	if b.Clip.Empty() {
		return 0, nil
	}
	n := 0
	for _, src := range b.Clip.Paths {
		name := Base(src)
		dst := Join(b.dir, name)
		if src == dst {
			if b.Clip.Mode == ClipCut {
				continue
			}
			dst = Join(b.dir, uniqueName(b.h, b.dir, name))
		} else if _, err := b.h.Stat(dst); err == nil {
			dst = Join(b.dir, uniqueName(b.h, b.dir, name))
		}

		var err error
		if b.Clip.Mode == ClipCut {
			err = b.h.Rename(src, dst)
		} else {
			err = b.h.Copy(src, dst)
		}
		if err != nil {
			return n, err
		}
		n++
	}
	if b.Clip.Mode == ClipCut {
		b.Clip = Clipboard{}
	}
	return n, nil
}

func uniqueName(h host.Backend, dir, name string) string {
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (copy)%s", stem, ext)
		if i > 1 {
			candidate = fmt.Sprintf("%s (copy %d)%s", stem, i, ext)
		}
		if _, err := h.Stat(Join(dir, candidate)); err != nil {
			return candidate
		}
	}
}

func validName(name string) error {
	switch {
	case strings.TrimSpace(name) == "":
		return fmt.Errorf("name cannot be empty")
	case strings.ContainsAny(name, "/\\"):
		return fmt.Errorf("name cannot contain a path separator")
	case name == "." || name == "..":
		return fmt.Errorf("%q is not a valid name", name)
	}
	return nil
}
