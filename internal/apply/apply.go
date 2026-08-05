// Package apply brings a server directory in line with .irori.json: the right
// core, the declared plugins, and the declared configuration keys.
package apply

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/install"
	"github.com/bx-team/irori/internal/lock"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/modrinth"
	"github.com/bx-team/irori/internal/overrides"
	"github.com/bx-team/irori/internal/plugins"
)

var ErrSealed = errors.New("irori: sealed mode is on, nothing may be downloaded")

type Options struct {
	DryRun bool
	Update bool
	Sealed bool
	// Only limits the run to one kind of step; empty means all of them.
	Only Kind
	// Configs replaces the keys declared in .irori.json. It is how the NixOS
	// module hands over the values it carries in the store instead.
	Configs map[string]map[string]any
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

func (e *engine) target() install.Target {
	return install.Target{H: e.h, Cfg: e.cfg, Lock: e.lf}
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

	if e.wants(KindCore) {
		e.core(ctx)
	}
	if e.wants(KindPlugin) {
		e.plugins(ctx)
	}
	if e.wants(KindConfig) {
		e.configs()
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
	if err := install.Core(ctx, e.target(), client, build, nil); err != nil {
		e.emit(Step{Kind: KindCore, Action: "install", Target: target, Err: err})
		return
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
				e.updateAddon(ctx, client, item)
				continue
			}
			e.emit(Step{Kind: KindPlugin, Action: "ok", Target: item.Name})

		case plugins.StateMissing, plugins.StateOutdated:
			if client == nil {
				client = modrinth.New()
			}
			e.installAddon(ctx, client, item)

		case plugins.StateOrphan:
			e.removeAddon(item)

		case plugins.StateUntracked:
			e.emit(Step{Kind: KindPlugin, Action: "ok", Target: item.Name,
				Detail: "not declared, left alone"})
		}
	}
}

func (e *engine) installAddon(ctx context.Context, client *modrinth.Client, item plugins.Item) {
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

	entry, err := e.fetchAddon(ctx, client, *item.Ref)
	if err != nil {
		step.Err = err
		e.emit(step)
		return
	}
	step.Detail = entry.File
	e.emit(step)
}

func (e *engine) updateAddon(ctx context.Context, client *modrinth.Client, item plugins.Item) {
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

	if _, err := install.Addon(ctx, e.target(), client, *item.Ref, version, nil); err != nil {
		step.Err = err
		e.emit(step)
		return
	}
	step.Detail = version.Label()
	e.emit(step)
}

func (e *engine) removeAddon(item plugins.Item) {
	step := Step{Kind: KindPlugin, Action: "remove", Target: item.Name,
		Detail: "no longer declared"}
	if e.opts.DryRun {
		e.emit(step)
		return
	}
	if item.Lock != nil {
		if err := install.RemoveAddon(e.target(), item.Lock.Key, item.Lock.File); err != nil {
			step.Err = err
			e.emit(step)
			return
		}
	}
	e.emit(step)
}

func (e *engine) loaders() []string { return e.cfg.Server.Type.ModrinthLoaders() }

func (e *engine) fetchAddon(ctx context.Context, client *modrinth.Client, ref config.PluginRef) (lock.Addon, error) {
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
		return install.Addon(ctx, e.target(), client, ref, version, nil)

	case config.SourceURL:
		return install.AddonFromURL(ctx, e.target(), client, ref)

	case config.SourceLocal:
		return install.AddonLocal(e.target(), ref)
	}
	return lock.Addon{}, fmt.Errorf("unknown plugin source %q", ref.Source)
}

func (e *engine) wants(k Kind) bool { return e.opts.Only == "" || e.opts.Only == k }

func (e *engine) configs() {
	declared := e.cfg.Configs
	if e.opts.Configs != nil {
		declared = e.opts.Configs
	}
	files := make([]string, 0, len(declared))
	for f := range declared {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, file := range files {
		changes, err := overrides.Apply(e.h, file, declared[file], e.opts.DryRun)
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
