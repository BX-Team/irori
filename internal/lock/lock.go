// Package lock records exactly which artifacts a server directory was built
// from: resolved URLs, checksums and file names. It is what makes an install
// reproducible, and what `irori nix` turns into fixed-output derivations.
package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const Version = 1

type Core struct {
	Type      string `json:"type"`
	MCVersion string `json:"mcVersion"`
	Build     string `json:"build"`
	File      string `json:"file"`
	URL       string `json:"url,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Size      int64  `json:"size,omitempty"`
	// Direct means the jar on disk is byte for byte what URL serves, so it can
	// be expressed as a fixed-output derivation. Installer-based cores (Forge)
	// unpack and rename things, and cannot.
	Direct bool `json:"direct,omitempty"`
}

type Addon struct {
	Key       string `json:"key"`
	Source    string `json:"source"`
	ID        string `json:"id,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
	VersionID string `json:"versionId,omitempty"`
	Name      string `json:"name,omitempty"`
	File      string `json:"file"`
	URL       string `json:"url,omitempty"`
	SHA512    string `json:"sha512,omitempty"`
	SHA1      string `json:"sha1,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

type File struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
	Core      *Core     `json:"core,omitempty"`
	Addons    []Addon   `json:"addons,omitempty"`

	path string
}

func New(path string) *File {
	return &File{Version: Version, path: path}
}

func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(path), nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if f.Version == 0 {
		f.Version = Version
	}
	f.path = path
	return &f, nil
}

func (f *File) Path() string { return f.path }

func (f *File) Save() error {
	f.Version = Version
	f.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	sort.Slice(f.Addons, func(i, j int) bool { return f.Addons[i].Key < f.Addons[j].Key })

	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *File) Find(key string) (Addon, bool) {
	for _, a := range f.Addons {
		if a.Key == key {
			return a, true
		}
	}
	return Addon{}, false
}

func (f *File) Upsert(a Addon) {
	for i := range f.Addons {
		if f.Addons[i].Key == a.Key {
			f.Addons[i] = a
			return
		}
	}
	f.Addons = append(f.Addons, a)
}

func (f *File) Remove(key string) bool {
	for i, a := range f.Addons {
		if a.Key == key {
			f.Addons = append(f.Addons[:i], f.Addons[i+1:]...)
			return true
		}
	}
	return false
}

// Files lists every addon file the lock claims, used to spot jars in the addon
// directory that irori did not put there.
func (f *File) Files() map[string]Addon {
	out := make(map[string]Addon, len(f.Addons))
	for _, a := range f.Addons {
		out[a.File] = a
	}
	return out
}
