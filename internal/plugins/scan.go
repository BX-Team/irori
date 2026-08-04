// Package plugins reconciles the addon directory on disk with what .irori.json
// declares and what .irori.lock.json recorded.
package plugins

import (
	"path"
	"sort"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/addons"
	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/lock"
)

const maxMetaBytes = 32 << 20

type Installed struct {
	File    string
	Size    int64
	ModTime time.Time
	Meta    addons.Meta
	HasMeta bool
	// Disabled marks the ".jar.disabled" convention servers use to keep a
	// plugin around without loading it.
	Disabled bool
}

func (i Installed) Display() string {
	if i.HasMeta {
		return i.Meta.Display()
	}
	return strings.TrimSuffix(i.File, path.Ext(i.File))
}

// Scan lists the jars in the addon directory and reads each descriptor. Missing
// directories are not an error: a fresh server has no plugins yet.
func Scan(h host.Backend, dir string) ([]Installed, error) {
	entries, err := h.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var out []Installed
	for _, e := range entries {
		if e.IsDir {
			continue
		}
		lower := strings.ToLower(e.Name)
		disabled := strings.HasSuffix(lower, ".jar.disabled")
		if !strings.HasSuffix(lower, ".jar") && !disabled {
			continue
		}

		in := Installed{File: e.Name, Size: e.Size, ModTime: e.ModTime, Disabled: disabled}
		if e.Size <= maxMetaBytes {
			if raw, err := h.ReadFile(path.Join(dir, e.Name)); err == nil {
				if m, err := addons.ReadMeta(raw); err == nil {
					in.Meta, in.HasMeta = m, true
				}
			}
		}
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Display()) < strings.ToLower(out[j].Display())
	})
	return out, nil
}

type State string

const (
	// StateOK: declared, recorded in the lock, and the file is on disk.
	StateOK State = "ok"
	// StateMissing: declared but the jar is not there, so apply installs it.
	StateMissing State = "missing"
	// StateOutdated: the declared version no longer matches the lock.
	StateOutdated State = "outdated"
	// StateUntracked: a jar nobody declared, usually dropped in by hand.
	StateUntracked State = "untracked"
	// StateOrphan: the lock still has it but the config no longer does, so
	// apply removes it.
	StateOrphan State = "orphan"
)

type Item struct {
	Key       string
	Name      string
	State     State
	Ref       *config.PluginRef
	Lock      *lock.Addon
	Installed *Installed
	Version   string
}

func Reconcile(cfg *config.Config, lf *lock.File, installed []Installed) []Item {
	byFile := make(map[string]*Installed, len(installed))
	for i := range installed {
		byFile[installed[i].File] = &installed[i]
	}

	claimed := map[string]bool{}
	seenKey := map[string]bool{}
	var items []Item

	for i := range cfg.Plugins {
		ref := &cfg.Plugins[i]
		key := ref.Key()
		seenKey[key] = true

		item := Item{Key: key, Name: ref.Display(), Ref: ref, Version: ref.Version}
		if a, ok := lf.Find(key); ok {
			locked := a
			item.Lock = &locked
			if item.Name == "" || ref.Name == "" {
				item.Name = firstNonEmpty(a.Name, ref.Display())
			}
			if in, ok := byFile[a.File]; ok {
				item.Installed = in
				claimed[a.File] = true
			}
		}

		switch {
		case item.Installed == nil:
			item.State = StateMissing
		case ref.Version != "" && item.Lock != nil && ref.Version != item.Lock.VersionID:
			item.State = StateOutdated
		default:
			item.State = StateOK
		}
		items = append(items, item)
	}

	for _, a := range lf.Addons {
		if seenKey[a.Key] {
			continue
		}
		locked := a
		item := Item{Key: a.Key, Name: firstNonEmpty(a.Name, a.File), State: StateOrphan, Lock: &locked}
		if in, ok := byFile[a.File]; ok {
			item.Installed = in
			claimed[a.File] = true
		}
		items = append(items, item)
	}

	for i := range installed {
		in := &installed[i]
		if claimed[in.File] {
			continue
		}
		version := ""
		if in.HasMeta {
			version = in.Meta.Version
		}
		items = append(items, Item{
			Key:       "local:" + in.File,
			Name:      in.Display(),
			State:     StateUntracked,
			Installed: in,
			Version:   version,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].State != items[j].State {
			return stateRank(items[i].State) < stateRank(items[j].State)
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items
}

func stateRank(s State) int {
	switch s {
	case StateMissing:
		return 0
	case StateOutdated:
		return 1
	case StateOrphan:
		return 2
	case StateOK:
		return 3
	default:
		return 4
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
