// Package apply brings a server directory in line with .irori.json: the right
// core, the declared plugins, and the declared configuration keys.
package apply

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/lock"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/modrinth"
	"github.com/bx-team/irori/internal/overrides"
	"github.com/bx-team/irori/internal/plugins"
)

// ErrSealed is returned when a step needs the network but irori was told not to
// use it, the mode a Nix-managed deployment runs in.
var ErrSealed = errors.New("irori: sealed mode is on, nothing may be downloaded")

type Options struct {
	DryRun bool
	// Update re-resolves plugins that are declared without a pinned version.
	Update bool
	Sealed bool
}

type Kind string

const (
	KindCore   Kind = "core"
	KindPlugin Kind = "plugin"
	KindConfig Kind = "config"
)

type Step struct {
	Kind   Kind
	Action string
	Target string
	Detail string
	Err    error
}

func (s Step) String() string {
	line := fmt.Sprintf("%-7s %-8s %s", s.Kind, s.Action, s.Target)
	if s.Detail != "" {
		line += "  " + s.Detail
	}
	if s.Err != nil {
		line += "  ! " + s.Err.Error()
	}
	return line
}

type Result struct {
	Steps   []Step
	Changed int
	Failed  int
}

func (r Result) Err() error {
	if r.Failed == 0 {
		return nil
	}
	return fmt.Errorf("irori apply: %d step(s) failed", r.Failed)
}

type engine struct {
	cfg  *config.Config
	h    host.Backend
	lf   *lock.File
	opts Options
	on   func(Step)
	res  *Result
}

func Run(ctx context.Context, cfg *config.Config, opts Options, onStep func(Step)) (Result, error) {
	lf, err := lock.Load(cfg.LockPath())
	if err != nil {
		return Result{}, err
	}
	res := Result{}
	e := &engine{
		cfg:  cfg,
		h:    host.NewLocal(cfg.Dir()),
		lf:   lf,
		opts: opts,
		on:   onStep,
		res:  &res,
	}

	e.core(ctx)
	e.plugins(ctx)
	e.configs()

	if !opts.DryRun && res.Changed > 0 {
		if err := e.lf.Save(); err != nil {
			return res, err
		}
	}
	return res, res.Err()
}

func (e *engine) emit(s Step) {
	if s.Err != nil {
		e.res.Failed++
	} else if s.Action != "ok" {
		e.res.Changed++
	}
	e.res.Steps = append(e.res.Steps, s)
	if e.on != nil {
		e.on(s)
	}
}

// ---- core ----

func (e *engine) core(ctx context.Context) {
	c := e.cfg.Server
	if c.Type == "" || c.MCVersion == "" {
		return
	}

	target := fmt.Sprintf("%s %s %s", c.Type.Display(), c.MCVersion, c.Build)
	jarThere := false
	if _, err := e.h.Stat(c.Jar); err == nil {
		jarThere = true
	}

	locked := e.lf.Core
	upToDate := jarThere && locked != nil &&
		locked.Type == string(c.Type) &&
		locked.MCVersion == c.MCVersion &&
		locked.Build == c.Build &&
		locked.File == c.Jar
	if upToDate {
		e.emit(Step{Kind: KindCore, Action: "ok", Target: target})
		return
	}

	reason := "core does not match the lock"
	if !jarThere {
		reason = c.Jar + " is missing"
	}
	if e.opts.DryRun {
		e.emit(Step{Kind: KindCore, Action: "install", Target: target, Detail: reason})
		return
	}
	if e.opts.Sealed {
		e.emit(Step{Kind: KindCore, Action: "install", Target: target, Err: ErrSealed})
		return
	}

	client := mcjars.New()
	build, err := e.resolveBuild(ctx, client)
	if err != nil {
		e.emit(Step{Kind: KindCore, Action: "install", Target: target, Err: err})
		return
	}
	if err := client.Install(ctx, build, e.cfg.Dir(), nil); err != nil {
		e.emit(Step{Kind: KindCore, Action: "install", Target: target, Err: err})
		return
	}

	e.cfg.Server.Build = build.Name
	e.cfg.Server.BuildID = build.UUID
	e.cfg.Server.Jar = build.CoreJar()
	e.lf.Core = &lock.Core{
		Type:      string(c.Type),
		MCVersion: c.MCVersion,
		Build:     build.Name,
		File:      build.CoreJar(),
		URL:       coreURL(build),
		Direct:    directDownload(build),
	}
	if e.lf.Core.Direct {
		if sum, size, err := hashFile(e.h, build.CoreJar()); err == nil {
			e.lf.Core.SHA256, e.lf.Core.Size = sum, size
		}
	}
	if err := e.cfg.Save(); err != nil {
		e.emit(Step{Kind: KindCore, Action: "install", Target: target, Err: err})
		return
	}
	e.emit(Step{Kind: KindCore, Action: "install", Target: target, Detail: "build " + build.Name})
}

func (e *engine) resolveBuild(ctx context.Context, client *mcjars.Client) (mcjars.Build, error) {
	c := e.cfg.Server
	if c.Build == "" {
		return client.LatestBuild(ctx, c.Type, c.MCVersion)
	}
	builds, _, err := client.Builds(ctx, c.Type, c.MCVersion, 1, 100)
	if err != nil {
		return mcjars.Build{}, err
	}
	for _, b := range builds {
		if b.Name == c.Build {
			return b, nil
		}
	}
	return mcjars.Build{}, fmt.Errorf("build %s of %s %s is not on mcjars",
		c.Build, c.Type.Display(), c.MCVersion)
}

// directDownload reports a recipe that is a single jar download with no
// unpacking, which is the only shape Nix can express as one fetchurl.
func directDownload(b mcjars.Build) bool {
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

func hashFile(h host.Backend, name string) (string, int64, error) {
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

func coreURL(b mcjars.Build) string {
	for _, group := range b.Installation {
		for _, s := range group {
			if s.Type == "download" && strings.HasSuffix(strings.ToLower(s.File), ".jar") {
				return s.URL
			}
		}
	}
	return ""
}

// ---- plugins ----

func (e *engine) plugins(ctx context.Context) {
	dir := e.cfg.Server.Type.AddonDir()
	installed, _ := plugins.Scan(e.h, dir)
	items := plugins.Reconcile(e.cfg, e.lf, installed)

	var client *modrinth.Client
	for _, item := range items {
		switch item.State {
		case plugins.StateOK:
			if e.opts.Update && item.Ref != nil && item.Ref.Version == "" {
				if client == nil {
					client = modrinth.New()
				}
				e.updateAddon(ctx, client, item, dir)
				continue
			}
			e.emit(Step{Kind: KindPlugin, Action: "ok", Target: item.Name})

		case plugins.StateMissing, plugins.StateOutdated:
			if client == nil {
				client = modrinth.New()
			}
			e.installAddon(ctx, client, item, dir)

		case plugins.StateOrphan:
			e.removeAddon(item, dir)

		case plugins.StateUntracked:
			e.emit(Step{Kind: KindPlugin, Action: "ok", Target: item.Name,
				Detail: "not declared, left alone"})
		}
	}
}

func (e *engine) installAddon(ctx context.Context, client *modrinth.Client, item plugins.Item, dir string) {
	action := "install"
	if item.State == plugins.StateOutdated {
		action = "update"
	}
	step := Step{Kind: KindPlugin, Action: action, Target: item.Name}

	if e.opts.DryRun {
		step.Detail = string(item.State)
		e.emit(step)
		return
	}
	if e.opts.Sealed {
		step.Err = ErrSealed
		e.emit(step)
		return
	}

	entry, err := e.fetchAddon(ctx, client, *item.Ref, dir)
	if err != nil {
		step.Err = err
		e.emit(step)
		return
	}
	if item.Lock != nil && item.Lock.File != entry.File {
		_ = e.h.Remove(path.Join(dir, item.Lock.File))
	}
	e.lf.Upsert(entry)
	step.Detail = entry.File
	e.emit(step)
}

func (e *engine) updateAddon(ctx context.Context, client *modrinth.Client, item plugins.Item, dir string) {
	step := Step{Kind: KindPlugin, Action: "update", Target: item.Name}
	if item.Ref.Source != config.SourceModrinth {
		e.emit(Step{Kind: KindPlugin, Action: "ok", Target: item.Name})
		return
	}

	version, err := client.LatestVersion(ctx, item.Ref.ID, e.loaders(), []string{e.cfg.Server.MCVersion})
	if err != nil {
		step.Err = err
		e.emit(step)
		return
	}
	if item.Lock != nil && item.Lock.VersionID == version.ID {
		e.emit(Step{Kind: KindPlugin, Action: "ok", Target: item.Name})
		return
	}
	if e.opts.DryRun {
		step.Detail = "→ " + version.Label()
		e.emit(step)
		return
	}
	if e.opts.Sealed {
		step.Err = ErrSealed
		e.emit(step)
		return
	}

	entry, err := e.downloadVersion(ctx, client, *item.Ref, version, dir)
	if err != nil {
		step.Err = err
		e.emit(step)
		return
	}
	if item.Lock != nil && item.Lock.File != entry.File {
		_ = e.h.Remove(path.Join(dir, item.Lock.File))
	}
	e.lf.Upsert(entry)
	step.Detail = version.Label()
	e.emit(step)
}

func (e *engine) removeAddon(item plugins.Item, dir string) {
	step := Step{Kind: KindPlugin, Action: "remove", Target: item.Name,
		Detail: "no longer declared"}
	if e.opts.DryRun {
		e.emit(step)
		return
	}
	if item.Lock != nil {
		if err := e.h.Remove(path.Join(dir, item.Lock.File)); err != nil && !errors.Is(err, os.ErrNotExist) {
			step.Err = err
			e.emit(step)
			return
		}
		e.lf.Remove(item.Lock.Key)
	}
	e.emit(step)
}

func (e *engine) loaders() []string { return e.cfg.Server.Type.ModrinthLoaders() }

func (e *engine) fetchAddon(ctx context.Context, client *modrinth.Client, ref config.PluginRef, dir string) (lock.Addon, error) {
	switch ref.Source {
	case config.SourceModrinth:
		var (
			version modrinth.Version
			err     error
		)
		if ref.Version != "" {
			version, err = client.Version(ctx, ref.Version)
		} else {
			version, err = client.LatestVersion(ctx, ref.ID, e.loaders(), []string{e.cfg.Server.MCVersion})
		}
		if err != nil {
			return lock.Addon{}, err
		}
		return e.downloadVersion(ctx, client, ref, version, dir)

	case config.SourceURL:
		name := ref.File
		if name == "" {
			name = path.Base(ref.URL)
		}
		f := modrinth.File{URL: ref.URL, Filename: name}
		if err := client.Download(ctx, f, e.h.Abs(path.Join(dir, name)), nil); err != nil {
			return lock.Addon{}, err
		}
		return lock.Addon{Key: ref.Key(), Source: string(ref.Source), Name: ref.Display(),
			File: name, URL: ref.URL}, nil

	case config.SourceLocal:
		if _, err := e.h.Stat(path.Join(dir, ref.File)); err != nil {
			return lock.Addon{}, fmt.Errorf("%s is declared as a local file but is not in %s/", ref.File, dir)
		}
		return lock.Addon{Key: ref.Key(), Source: string(ref.Source), Name: ref.Display(), File: ref.File}, nil
	}
	return lock.Addon{}, fmt.Errorf("unknown plugin source %q", ref.Source)
}

func (e *engine) downloadVersion(ctx context.Context, client *modrinth.Client, ref config.PluginRef, v modrinth.Version, dir string) (lock.Addon, error) {
	file, ok := v.PrimaryFile()
	if !ok {
		return lock.Addon{}, fmt.Errorf("%s %s has no downloadable file", ref.Display(), v.Label())
	}
	if err := client.Download(ctx, file, e.h.Abs(path.Join(dir, file.Filename)), nil); err != nil {
		return lock.Addon{}, err
	}
	return lock.Addon{
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
	}, nil
}

// ---- configs ----

func (e *engine) configs() {
	files := make([]string, 0, len(e.cfg.Configs))
	for f := range e.cfg.Configs {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, file := range files {
		changes, err := overrides.Apply(e.h, file, e.cfg.Configs[file], e.opts.DryRun)
		if err != nil {
			e.emit(Step{Kind: KindConfig, Action: "set", Target: file, Err: err})
			continue
		}
		if len(changes) == 0 {
			e.emit(Step{Kind: KindConfig, Action: "ok", Target: file})
			continue
		}
		for _, c := range changes {
			detail := fmt.Sprintf("%s = %s", c.Key, c.To)
			if !c.New && c.From != "" {
				detail += " (was " + c.From + ")"
			}
			e.emit(Step{Kind: KindConfig, Action: "set", Target: file, Detail: detail})
		}
	}
}
