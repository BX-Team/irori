// Package install is the one path by which a jar reaches the server directory.
// Every function here writes .irori.lock.json before it returns: recording an
// artifact is part of installing it, not a step a caller can forget.
package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/lock"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/modrinth"
)

type Target struct {
	H    host.Backend
	Cfg  *config.Config
	Lock *lock.File
}

func (t Target) AddonDir() string { return t.Cfg.Server.Type.AddonDir() }

func Core(ctx context.Context, t Target, c *mcjars.Client, b mcjars.Build, onProgress func(mcjars.Progress)) error {
	if err := c.Install(ctx, b, t.Cfg.Dir(), onProgress); err != nil {
		return err
	}

	if t.Cfg.Server.MCVersion == "" {
		t.Cfg.Server.MCVersion = b.VersionID
	}
	t.Cfg.Server.Build = b.Name
	t.Cfg.Server.BuildID = b.UUID
	t.Cfg.Server.Jar = b.CoreJar()

	entry := &lock.Core{
		Type:      string(t.Cfg.Server.Type),
		MCVersion: t.Cfg.Server.MCVersion,
		Build:     b.Name,
		File:      b.CoreJar(),
		URL:       CoreURL(b),
		Direct:    DirectDownload(b),
	}
	if entry.Direct {
		if sum, size, err := HashFile(t.H, entry.File); err == nil {
			entry.SHA256, entry.Size = sum, size
		}
	}
	t.Lock.Core = entry
	return t.Lock.Save()
}

func Addon(ctx context.Context, t Target, c *modrinth.Client, ref config.PluginRef, v modrinth.Version, onProgress func(modrinth.Progress)) (lock.Addon, error) {
	file, ok := v.PrimaryFile()
	if !ok {
		return lock.Addon{}, fmt.Errorf("%s %s has no downloadable file", ref.Display(), v.Label())
	}
	dir := t.AddonDir()
	if err := c.Download(ctx, file, t.H.Abs(path.Join(dir, file.Filename)), onProgress); err != nil {
		return lock.Addon{}, err
	}
	return t.record(ref, lock.Addon{
		Key:       ref.Key(),
		Source:    string(config.SourceModrinth),
		ID:        ref.ID,
		ProjectID: v.ProjectID,
		VersionID: v.ID,
		Name:      ref.Display(),
		File:      file.Filename,
		URL:       file.URL,
		SHA512:    file.Hashes.SHA512,
		SHA1:      file.Hashes.SHA1,
		Size:      file.Size,
	})
}

func AddonFromURL(ctx context.Context, t Target, c *modrinth.Client, ref config.PluginRef) (lock.Addon, error) {
	name := ref.File
	if name == "" {
		name = path.Base(ref.URL)
	}
	dir := t.AddonDir()
	f := modrinth.File{URL: ref.URL, Filename: name}
	if err := c.Download(ctx, f, t.H.Abs(path.Join(dir, name)), nil); err != nil {
		return lock.Addon{}, err
	}
	return t.record(ref, lock.Addon{
		Key: ref.Key(), Source: string(config.SourceURL), Name: ref.Display(),
		File: name, URL: ref.URL,
	})
}

func AddonLocal(t Target, ref config.PluginRef) (lock.Addon, error) {
	dir := t.AddonDir()
	if _, err := t.H.Stat(path.Join(dir, ref.File)); err != nil {
		return lock.Addon{}, fmt.Errorf("%s is declared as a local file but is not in %s/", ref.File, dir)
	}
	return t.record(ref, lock.Addon{
		Key: ref.Key(), Source: string(config.SourceLocal), Name: ref.Display(), File: ref.File,
	})
}

// A plugin carries its version in the file name, so an update writes a second
// jar and the server loads both unless the one it replaces is removed here.
func (t Target) record(ref config.PluginRef, entry lock.Addon) (lock.Addon, error) {
	if old, ok := t.Lock.Find(entry.Key); ok && old.File != entry.File {
		_ = t.H.Remove(path.Join(t.AddonDir(), old.File))
	}
	t.Cfg.UpsertPlugin(ref)
	t.Lock.Upsert(entry)
	return entry, t.Lock.Save()
}

func RemoveAddon(t Target, key, file string) error {
	if file != "" {
		if err := t.H.Remove(path.Join(t.AddonDir(), file)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	t.Cfg.RemovePlugin(key)
	if !t.Lock.Remove(key) {
		return nil
	}
	return t.Lock.Save()
}

// DirectDownload reports a recipe that is a single jar download with no
// unpacking, which is the only shape Nix can express as one fetchurl.
func DirectDownload(b mcjars.Build) bool {
	downloads := 0
	for _, group := range b.Installation {
		for _, s := range group {
			switch s.Type {
			case "download":
				if !strings.HasSuffix(strings.ToLower(s.File), ".jar") {
					return false
				}
				downloads++
			default:
				return false
			}
		}
	}
	return downloads == 1
}

func CoreURL(b mcjars.Build) string {
	for _, group := range b.Installation {
		for _, s := range group {
			if s.Type == "download" && strings.HasSuffix(strings.ToLower(s.File), ".jar") {
				return s.URL
			}
		}
	}
	return ""
}

func HashFile(h host.Backend, name string) (string, int64, error) {
	f, err := h.Open(name)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	sum := sha256.New()
	size, err := io.Copy(sum, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(sum.Sum(nil)), size, nil
}
