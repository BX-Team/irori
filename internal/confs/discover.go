package confs

import (
	"path"
	"sort"
	"strings"

	"github.com/bx-team/irori/internal/host"
)

type FileRef struct {
	Path  string
	Title string
	Owner string
}

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
	{"plugins/*/config.yml", ownerFromDir},
}

const ownerFromDir = ""

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

func Load(h host.Backend, rel string) (*Doc, error) {
	raw, err := h.ReadFile(rel)
	if err != nil {
		return nil, err
	}
	return Parse(rel, raw)
}

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
