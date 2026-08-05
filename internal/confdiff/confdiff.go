// Package confdiff compares the configuration files a build ships with the ones
// in the server directory, so a hand-tuned server can be turned into the set of
// declared keys .irori.json and the NixOS module know how to re-apply.
package confdiff

import (
	"strings"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/confs"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/mcjars"
)

type Change struct {
	File    string
	Key     string
	Default string
	Value   string
	Typed   any
}

type Skip struct {
	File   string
	Key    string
	Reason string
}

type Result struct {
	Compared []string
	Changes  []Change
	Skipped  []Skip
}

func (r Result) Empty() bool { return len(r.Changes) == 0 }

// Compare walks the shipped files only. A plugin writes its own config on first
// start and no pristine copy of it exists anywhere, so there is nothing to
// diff it against.
func Compare(h host.Backend, shipped []mcjars.Config) Result {
	var res Result
	for _, c := range shipped {
		location := strings.TrimPrefix(strings.ReplaceAll(c.Location, "\\", "/"), "./")
		if confs.Format(location) == "" {
			continue
		}
		raw, err := h.ReadFile(location)
		if err != nil {
			continue
		}
		def, err := confs.Parse(location, []byte(c.Value))
		if err != nil {
			res.Skipped = append(res.Skipped, Skip{File: location, Reason: "the shipped copy could not be parsed"})
			continue
		}
		cur, err := confs.Parse(location, raw)
		if err != nil {
			res.Skipped = append(res.Skipped, Skip{File: location, Reason: "could not be parsed"})
			continue
		}
		res.Compared = append(res.Compared, location)
		compare(&res, location, def, cur)
	}
	return res
}

func compare(res *Result, file string, def, cur *confs.Doc) {
	for _, e := range cur.Entries() {
		d, ok := def.Entry(e.Key)
		if !ok || d.Value == e.Value {
			continue
		}
		switch {
		case e.Secret:
			res.Skipped = append(res.Skipped, Skip{file, e.Key, "a secret, and /nix/store is world readable"})
		case e.ReadOnly || d.ReadOnly:
			res.Skipped = append(res.Skipped, Skip{file, e.Key, "a list or a block, which irori does not write key by key"})
		case bookkeeping(e.Key):
			res.Skipped = append(res.Skipped, Skip{file, e.Key, "the core's own version marker, pinning it breaks the next upgrade"})
		default:
			res.Changes = append(res.Changes, Change{
				File: file, Key: e.Key, Default: d.Value, Value: e.Value,
				Typed: confs.Typed(e.Kind, e.Value),
			})
		}
	}
}

var versionKeys = map[string]bool{"config-version": true, "config_version": true, "_version": true, "version": true}

func bookkeeping(key string) bool {
	if i := strings.LastIndex(key, "."); i >= 0 {
		key = key[i+1:]
	}
	return versionKeys[key]
}

func Declare(cfg *config.Config, changes []Change) {
	for _, c := range changes {
		cfg.SetOverride(c.File, c.Key, c.Typed)
	}
}
