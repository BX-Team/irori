// Package confs reads a server's configuration files into a flat list of
// editable entries. It exists so the settings UI can offer the same typed form
// for bukkit.yml or purpur.yml that it already offers for server.properties,
// without anyone hand-writing a catalog for every fork's config: the
// description shown next to a key is the comment the file itself carries.
//
// Only reading lives here. Writing goes back through internal/overrides, whose
// per-format editors already touch nothing but the key they were asked to
// change.
package confs

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/bx-team/irori/internal/overrides"
	"github.com/bx-team/irori/internal/props"
)

// Entry is one editable leaf of a config file.
type Entry struct {
	// Key is the dotted path the overrides writers address this leaf by, and is
	// unique within the file.
	Key string
	// Label is the leaf name on its own; the path above it lives in Section.
	Label   string
	Section string
	Value   string
	Kind    props.Kind
	Values  []string
	Min     int
	Max     int
	Desc    string
	Default string
	Secret  bool
	// ReadOnly marks values with no sensible inline editor — lists and nested
	// blocks. They stay visible so the file is not silently half-shown, and the
	// Files tab or $EDITOR handles them.
	ReadOnly bool
}

// Doc is a parsed config file plus the edits made to it.
type Doc struct {
	// Path is relative to the server directory, in slash form.
	Path    string
	Format  string
	entries []Entry
	raw     []byte
	index   map[string]int
}

func (d *Doc) Entries() []Entry { return d.entries }

func (d *Doc) Raw() []byte { return d.raw }

func (d *Doc) Entry(key string) (Entry, bool) {
	i, ok := d.index[key]
	if !ok {
		return Entry{}, false
	}
	return d.entries[i], true
}

// Parse reads file content into entries. The path decides the syntax, so it
// must be the real relative path and not a scratch name.
func Parse(relPath string, raw []byte) (*Doc, error) {
	d := &Doc{Path: path.Clean(relPath), Format: Format(relPath), raw: raw}
	var err error
	switch d.Format {
	case "properties":
		d.entries = parseProperties(raw)
	case "yaml":
		d.entries, err = parseYAML(raw)
	case "toml":
		d.entries = parseTOML(raw)
	default:
		return nil, fmt.Errorf("confs: %s is not a format irori can edit key by key", relPath)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relPath, err)
	}

	d.index = make(map[string]int, len(d.entries))
	for i, e := range d.entries {
		d.index[e.Key] = i
	}
	return d, nil
}

// Render writes the given values back into the file content, returning the new
// bytes. Values are keyed by entry key and typed according to the entry's kind,
// so a number does not come back quoted.
func (d *Doc) Render(values map[string]string) ([]byte, error) {
	typed := make(map[string]any, len(values))
	for key, value := range values {
		e, ok := d.Entry(key)
		if !ok {
			typed[key] = value
			continue
		}
		typed[key] = Typed(e.Kind, value)
	}
	out, _, err := overrides.Edit(d.Path, d.raw, typed)
	return out, err
}

// Typed converts an edited string back to the JSON-ish value the config format
// expects, so booleans and numbers are not written as quoted strings.
func Typed(kind props.Kind, value string) any {
	switch kind {
	case props.KindBool:
		return value == "true"
	case props.KindInt:
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return n
		}
	case props.KindFloat:
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return f
		}
	}
	return value
}

// parseProperties keeps the server.properties experience intact: the shipped
// catalog supplies the type, the range and the prose, and keys a fork added
// stay editable as plain strings.
func parseProperties(raw []byte) []Entry {
	f := props.Parse(raw)

	seen := map[string]bool{}
	specs := make([]props.Spec, 0, len(props.Catalog))
	for _, s := range props.Catalog {
		specs = append(specs, s)
		seen[s.Key] = true
	}
	var extra []string
	for _, k := range f.Keys() {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		specs = append(specs, props.SpecFor(k))
	}
	sort.SliceStable(specs, func(i, j int) bool {
		return props.SectionRank(specs[i].Section) < props.SectionRank(specs[j].Section)
	})

	out := make([]Entry, 0, len(specs))
	for _, s := range specs {
		out = append(out, Entry{
			Key:     s.Key,
			Label:   s.Key,
			Section: s.Section,
			Value:   f.GetOr(s.Key, s.Default),
			Kind:    s.Kind,
			Values:  s.Values,
			Min:     s.Min,
			Max:     s.Max,
			Desc:    s.Desc,
			Default: s.Default,
			Secret:  s.Secret,
		})
	}
	return out
}

// cleanComment turns a run of comment lines into one paragraph: the leading
// hashes go, blank separator lines become spaces, and the divider rows some
// generators emit are dropped entirely.
func cleanComment(raw string) string {
	var parts []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		line = strings.TrimSpace(line)
		if line == "" || strings.Trim(line, "-=*_") == "" {
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, " ")
}

// splitKey separates a dotted path into the section header and the leaf label.
func splitKey(key string) (section, label string) {
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}
