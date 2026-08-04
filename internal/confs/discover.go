package confs

import (
	"path"
	"sort"
	"strings"

	"github.com/bx-team/irori/internal/host"
)

// FileRef is a config file present in the server directory.
type FileRef struct {
	// Path is relative to the server directory, in slash form.
	Path string
	// Title is what the picker shows, usually the base name.
	Title string
	// Owner names the software the file belongs to, for grouping.
	Owner string
}

// known lists the core configuration files irori offers a form for, in the order
// they should appear. Everything here is a file a server core writes itself;
// the Files tab remains the way to reach anything else.
//
// A directory entry ending in /* pulls in whatever that directory holds, which
// is how a fork that ships its own config under config/ is picked up without
// irori having heard of it.
var known = []struct {
	pattern string
	owner   string
}{
	{"server.properties", "Minecraft"},

	{"bukkit.yml", "Bukkit"},
	{"commands.yml", "Bukkit"},
	{"help.yml", "Bukkit"},
	{"permissions.yml", "Bukkit"},
	{"spigot.yml", "Spigot"},

	{"paper.yml", "Paper"},
	{"config/*", "Paper"},

	{"purpur.yml", "Purpur"},
	{"pufferfish.yml", "Pufferfish"},
	{"divinemc.yml", "DivineMC"},
	{"leaves.yml", "Leaves"},
	{"leaf.yml", "Leaf"},
	{"canvas.yml", "Canvas"},
	{"gale.yml", "Gale"},
	{"folia.yml", "Folia"},

	{"velocity.toml", "Velocity"},
	{"config.yml", "Proxy"},
	{"waterfall.yml", "Waterfall"},

	{"wepif.yml", "Plugins"},
	// The owner of a plugin config is the plugin, so the directory name stands
	// in for it — otherwise a dozen rows all read "config.yml / Plugins".
	{"plugins/*/config.yml", ownerFromDir},
}

// ownerFromDir asks Discover to take the owner from the file's parent
// directory instead of the table.
const ownerFromDir = ""

// Discover returns the config files that actually exist, in presentation order.
// Missing files are simply absent: a Paper server has no purpur.yml and should
// not be shown an entry that opens onto nothing.
func Discover(h host.Backend) []FileRef {
	var out []FileRef
	seen := map[string]bool{}

	add := func(rel, owner string) {
		rel = path.Clean(rel)
		if seen[rel] || Format(rel) == "" {
			return
		}
		if e, err := h.Stat(rel); err != nil || e.IsDir {
			return
		}
		if owner == ownerFromDir {
			owner = path.Base(path.Dir(rel))
		}
		seen[rel] = true
		out = append(out, FileRef{Path: rel, Title: path.Base(rel), Owner: owner})
	}

	for _, k := range known {
		dir, pattern, wildcard := strings.Cut(k.pattern, "/*")
		if !wildcard {
			add(k.pattern, k.owner)
			continue
		}
		for _, rel := range expand(h, dir, pattern) {
			add(rel, k.owner)
		}
	}

	return out
}

// expand lists a wildcard segment. The two shapes used are "dir/*" (every file
// in a directory) and "dir/*/name" (one named file inside each subdirectory).
func expand(h host.Backend, dir, rest string) []string {
	entries, err := h.ReadDir(dir)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	var out []string
	for _, e := range entries {
		switch {
		case rest == "" && !e.IsDir:
			out = append(out, path.Join(dir, e.Name))
		case rest != "" && e.IsDir:
			out = append(out, path.Join(dir, e.Name, strings.TrimPrefix(rest, "/")))
		}
	}
	return out
}

// Format reports the syntax irori will parse a path as, or "" when it is not a
// config file irori can present as a form.
func Format(rel string) string {
	switch strings.ToLower(path.Ext(rel)) {
	case ".properties":
		return "properties"
	case ".yml", ".yaml":
		return "yaml"
	case ".toml":
		return "toml"
	}
	return ""
}

// Load reads and parses one file.
func Load(h host.Backend, rel string) (*Doc, error) {
	raw, err := h.ReadFile(rel)
	if err != nil {
		return nil, err
	}
	return Parse(rel, raw)
}

// Save writes the given values back, leaving every other byte of the file
// untouched.
func Save(h host.Backend, d *Doc, values map[string]string) error {
	out, err := d.Render(values)
	if err != nil {
		return err
	}
	if err := h.WriteFile(d.Path, out, 0o644); err != nil {
		return err
	}
	d.raw = out
	return nil
}
