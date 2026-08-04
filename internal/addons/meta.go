// Package addons reads the descriptor a plugin or mod jar carries, so irori can
// name what is installed without asking any API.
package addons

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

var ErrNoDescriptor = errors.New("addons: jar carries no plugin or mod descriptor")

type Meta struct {
	ID          string
	Name        string
	Version     string
	Authors     []string
	Description string
	Depends     []string
	Loader      string
	APIVersion  string
	Website     string
}

func (m Meta) Display() string {
	if m.Name != "" {
		return m.Name
	}
	return m.ID
}

// descriptors are probed in order; the first one present wins, so a jar that
// ships both a Paper and a Velocity descriptor is reported as the Paper plugin
// it is installed as.
var descriptors = []struct {
	path  string
	parse func([]byte) (Meta, error)
}{
	{"plugin.yml", parseBukkit},
	{"paper-plugin.yml", parseBukkit},
	{"bungee.yml", parseBungee},
	{"velocity-plugin.json", parseVelocity},
	{"fabric.mod.json", parseFabric},
	{"quilt.mod.json", parseQuilt},
	{"META-INF/mods.toml", parseForge},
	{"META-INF/neoforge.mods.toml", parseForge},
}

func ReadMeta(jar []byte) (Meta, error) {
	zr, err := zip.NewReader(bytes.NewReader(jar), int64(len(jar)))
	if err != nil {
		return Meta{}, fmt.Errorf("addons: not a readable jar: %w", err)
	}

	index := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		index[f.Name] = f
	}
	for _, d := range descriptors {
		f, ok := index[d.path]
		if !ok {
			continue
		}
		raw, err := readAll(f)
		if err != nil {
			continue
		}
		m, err := d.parse(raw)
		if err != nil {
			continue
		}
		if m.Name == "" && m.ID == "" {
			continue
		}
		return m, nil
	}
	return Meta{}, ErrNoDescriptor
}

func readAll(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 1<<20))
}

func parseBukkit(raw []byte) (Meta, error) {
	// Versions are read as raw nodes: `version: 1.20` is a YAML float, and
	// decoding it as one would report the plugin as version 1.2.
	var d struct {
		Name        string    `yaml:"name"`
		Version     yaml.Node `yaml:"version"`
		Main        string    `yaml:"main"`
		Author      string    `yaml:"author"`
		Authors     []string  `yaml:"authors"`
		Description string    `yaml:"description"`
		Website     string    `yaml:"website"`
		APIVersion  yaml.Node `yaml:"api-version"`
		Depend      []string  `yaml:"depend"`
		SoftDepend  []string  `yaml:"softdepend"`
	}
	if err := yaml.Unmarshal(raw, &d); err != nil {
		return Meta{}, err
	}
	authors := d.Authors
	if d.Author != "" {
		authors = append([]string{d.Author}, authors...)
	}
	return Meta{
		ID:          d.Name,
		Name:        d.Name,
		Version:     d.Version.Value,
		Authors:     authors,
		Description: d.Description,
		Depends:     d.Depend,
		Website:     d.Website,
		APIVersion:  d.APIVersion.Value,
		Loader:      "bukkit",
	}, nil
}

func parseBungee(raw []byte) (Meta, error) {
	m, err := parseBukkit(raw)
	m.Loader = "bungeecord"
	return m, err
}

func parseVelocity(raw []byte) (Meta, error) {
	var d struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Version      string `json:"version"`
		Description  string `json:"description"`
		URL          string `json:"url"`
		Authors      []string
		Dependencies []struct {
			ID       string `json:"id"`
			Optional bool   `json:"optional"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return Meta{}, err
	}
	var depends []string
	for _, dep := range d.Dependencies {
		if !dep.Optional {
			depends = append(depends, dep.ID)
		}
	}
	name := d.Name
	if name == "" {
		name = d.ID
	}
	return Meta{
		ID: d.ID, Name: name, Version: d.Version, Authors: d.Authors,
		Description: d.Description, Depends: depends, Website: d.URL, Loader: "velocity",
	}, nil
}

func parseFabric(raw []byte) (Meta, error) {
	var d struct {
		ID          string            `json:"id"`
		Name        string            `json:"name"`
		Version     string            `json:"version"`
		Description string            `json:"description"`
		Authors     []json.RawMessage `json:"authors"`
		Depends     map[string]any    `json:"depends"`
		Contact     struct {
			Homepage string `json:"homepage"`
		} `json:"contact"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return Meta{}, err
	}
	name := d.Name
	if name == "" {
		name = d.ID
	}
	var depends []string
	for k := range d.Depends {
		if k != "minecraft" && k != "java" && k != "fabricloader" {
			depends = append(depends, k)
		}
	}
	return Meta{
		ID: d.ID, Name: name, Version: d.Version, Authors: fabricAuthors(d.Authors),
		Description: d.Description, Depends: depends, Website: d.Contact.Homepage, Loader: "fabric",
	}, nil
}

// fabric.mod.json allows an author to be either a bare string or an object with
// a name field.
func fabricAuthors(raw []json.RawMessage) []string {
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		var s string
		if json.Unmarshal(r, &s) == nil {
			out = append(out, s)
			continue
		}
		var o struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(r, &o) == nil && o.Name != "" {
			out = append(out, o.Name)
		}
	}
	return out
}

func parseQuilt(raw []byte) (Meta, error) {
	var d struct {
		Loader struct {
			ID       string `json:"id"`
			Version  string `json:"version"`
			Metadata struct {
				Name         string            `json:"name"`
				Description  string            `json:"description"`
				Contributors map[string]string `json:"contributors"`
			} `json:"metadata"`
		} `json:"quilt_loader"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return Meta{}, err
	}
	name := d.Loader.Metadata.Name
	if name == "" {
		name = d.Loader.ID
	}
	authors := make([]string, 0, len(d.Loader.Metadata.Contributors))
	for who := range d.Loader.Metadata.Contributors {
		authors = append(authors, who)
	}
	return Meta{
		ID: d.Loader.ID, Name: name, Version: d.Loader.Version,
		Description: d.Loader.Metadata.Description, Authors: authors, Loader: "quilt",
	}, nil
}

func parseForge(raw []byte) (Meta, error) {
	var d struct {
		Mods []struct {
			ModID       string `toml:"modId"`
			Version     string `toml:"version"`
			DisplayName string `toml:"displayName"`
			Description string `toml:"description"`
			Authors     string `toml:"authors"`
			DisplayURL  string `toml:"displayURL"`
		} `toml:"mods"`
	}
	if err := toml.Unmarshal(raw, &d); err != nil {
		return Meta{}, err
	}
	if len(d.Mods) == 0 {
		return Meta{}, ErrNoDescriptor
	}
	m := d.Mods[0]
	name := m.DisplayName
	if name == "" {
		name = m.ModID
	}
	var authors []string
	if m.Authors != "" {
		for _, a := range strings.Split(m.Authors, ",") {
			if a = strings.TrimSpace(a); a != "" {
				authors = append(authors, a)
			}
		}
	}
	return Meta{
		ID: m.ModID, Name: name, Version: m.Version, Description: m.Description,
		Authors: authors, Website: m.DisplayURL, Loader: "forge",
	}, nil
}
