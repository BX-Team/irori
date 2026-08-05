// Package overrides writes the keys declared in .irori.json into the files they
// belong to, touching only those keys. Everything around them — comments,
// ordering, blank lines — is left exactly as it was.
package overrides

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/props"
)

// Change is one key that differed from what the config declares.
type Change struct {
	File string
	Key  string
	From string
	To   string
	New  bool
}

func (c Change) String() string {
	if c.New {
		return fmt.Sprintf("%s: %s = %s (added)", c.File, c.Key, c.To)
	}
	return fmt.Sprintf("%s: %s = %s (was %s)", c.File, c.Key, c.To, c.From)
}

// Apply enforces values in one file. With dryRun the file is never written and
// the returned changes describe what would happen.
func Apply(h host.Backend, file string, values map[string]any, dryRun bool) ([]Change, error) {
	if len(values) == 0 {
		return nil, nil
	}
	raw, err := h.ReadFile(file)
	if err != nil {
		// A declared file that does not exist yet is created from scratch.
		raw = nil
	}

	out, changes, err := Edit(file, raw, values)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 || dryRun {
		return changes, nil
	}
	return changes, h.WriteFile(file, out, 0o644)
}

// Edit rewrites raw file content so the given keys hold the given values,
// dispatching on the file's format and touching nothing else. It is the
// writing half of Apply, split out so the config editor can reuse the
// comment-preserving writers on content it already has in hand.
func Edit(file string, raw []byte, values map[string]any) ([]byte, []Change, error) {
	if len(values) == 0 {
		return raw, nil, nil
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var (
		out     []byte
		changes []Change
		err     error
	)
	switch format(file) {
	case "properties":
		out, changes, err = applyProperties(raw, file, keys, values)
	case "yaml":
		out, changes, err = applyYAML(raw, file, keys, values)
	case "toml":
		out, changes, err = applyTOML(raw, file, keys, values)
	case "json":
		out, changes, err = applyJSON(raw, file, keys, values)
	default:
		return nil, nil, fmt.Errorf("overrides: %s has no format irori can edit declaratively", file)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", file, err)
	}
	return out, changes, nil
}

// Format reports the config syntax irori will use for a path, or "" when the
// file is not one it can edit key by key.
func Format(file string) string { return format(file) }

func format(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".properties":
		return "properties"
	case ".yml", ".yaml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".json":
		return "json"
	}
	if strings.HasSuffix(file, "server.properties") {
		return "properties"
	}
	return ""
}

// The error is always nil here, but every applier shares one signature so
// format() can dispatch to them; the yaml and json ones do fail.
//
//nolint:unparam
func applyProperties(raw []byte, file string, keys []string, values map[string]any) ([]byte, []Change, error) {
	f := props.Parse(raw)
	var changes []Change
	for _, k := range keys {
		want := Scalar(values[k])
		got, had := f.Get(k)
		if had && strings.TrimSpace(got) == want {
			continue
		}
		changes = append(changes, Change{File: file, Key: k, From: strings.TrimSpace(got), To: want, New: !had})
		f.Set(k, want)
	}
	return f.Bytes(), changes, nil
}

// Scalar renders a JSON-decoded value the way each config format expects to see
// it unquoted: whole numbers must not come out as "40.000000".
func Scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		return fmt.Sprint(t)
	}
}
