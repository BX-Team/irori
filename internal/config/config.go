package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bx-team/irori/internal/java"
	"github.com/bx-team/irori/internal/models"
)

const (
	FileName     = ".irori.json"
	LockFileName = ".irori.lock.json"
	StateDirName = ".irori"
	SchemaVer    = 1
)

var ErrNotFound = errors.New("irori: no server configuration found")

type Config struct {
	Version int                       `json:"version"`
	Server  ServerSection             `json:"server"`
	Java    JavaSection               `json:"java"`
	Runtime RuntimeSection            `json:"runtime"`
	Plugins []PluginRef               `json:"plugins,omitempty"`
	Configs map[string]map[string]any `json:"configs,omitempty"`

	dir string
}

type ServerSection struct {
	Name      string            `json:"name"`
	Type      models.ServerType `json:"type"`
	MCVersion string            `json:"mcVersion"`
	Build     string            `json:"build,omitempty"`
	BuildID   string            `json:"buildId,omitempty"`
	Jar       string            `json:"jar"`
}

type JavaSection struct {
	Path       string   `json:"path"`
	Major      int      `json:"major,omitempty"`
	Xms        string   `json:"xms"`
	Xmx        string   `json:"xmx"`
	Preset     string   `json:"preset"`
	ExtraFlags []string `json:"extraFlags,omitempty"`
	Args       []string `json:"args,omitempty"`
}

type RuntimeSection struct {
	AutoRestart    bool   `json:"autoRestart"`
	StopCommand    string `json:"stopCommand"`
	StopTimeoutSec int    `json:"stopTimeoutSec"`
}

type PluginSource string

const (
	SourceModrinth PluginSource = "modrinth"
	SourceURL      PluginSource = "url"
	SourceLocal    PluginSource = "local"
)

type PluginRef struct {
	Source  PluginSource `json:"source"`
	ID      string       `json:"id,omitempty"`
	Version string       `json:"version,omitempty"`
	URL     string       `json:"url,omitempty"`
	File    string       `json:"file,omitempty"`
	Name    string       `json:"name,omitempty"`
}

func (p PluginRef) Key() string {
	switch p.Source {
	case SourceModrinth:
		return "modrinth:" + p.ID
	case SourceURL:
		return "url:" + p.URL
	default:
		return "local:" + p.File
	}
}

func (p PluginRef) Display() string {
	if p.Name != "" {
		return p.Name
	}
	if p.ID != "" {
		return p.ID
	}
	if p.File != "" {
		return p.File
	}
	return p.URL
}

func Default(dir string) *Config {
	name := filepath.Base(dir)
	if name == "." || name == string(filepath.Separator) {
		name = "server"
	}
	return &Config{
		Version: SchemaVer,
		Server: ServerSection{
			Name: name,
			Type: models.TypePaper,
			Jar:  "server.jar",
		},
		Java: JavaSection{
			Xms:    "2G",
			Xmx:    "4G",
			Preset: "aikar",
		},
		Runtime: RuntimeSection{
			AutoRestart:    false,
			StopCommand:    "stop",
			StopTimeoutSec: 60,
		},
		dir: dir,
	}
}

func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

func Load(dir string) (*Config, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(abs, FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	cfg.dir = abs
	cfg.normalize()
	return &cfg, nil
}

func LoadFrom(start string) (*Config, error) {
	dir, err := Find(start)
	if err != nil {
		return nil, err
	}
	return Load(dir)
}

func (c *Config) normalize() {
	if c.Version == 0 {
		c.Version = SchemaVer
	}
	c.Server.Type = c.Server.Type.Normalize()
	if c.Server.Jar == "" {
		c.Server.Jar = "server.jar"
	}
	if c.Server.Name == "" {
		c.Server.Name = filepath.Base(c.dir)
	}
	if c.Java.Xms == "" {
		c.Java.Xms = "1G"
	}
	if c.Java.Xmx == "" {
		c.Java.Xmx = "2G"
	}
	if c.Java.Preset == "" {
		c.Java.Preset = "none"
	}
	if c.Runtime.StopCommand == "" {
		c.Runtime.StopCommand = "stop"
	}
	if c.Runtime.StopTimeoutSec <= 0 {
		c.Runtime.StopTimeoutSec = 60
	}
}

func (c *Config) Dir() string { return c.dir }

func (c *Config) SetDir(dir string) { c.dir = dir }

func (c *Config) Path() string { return filepath.Join(c.dir, FileName) }

func (c *Config) StateDir() string { return filepath.Join(c.dir, StateDirName) }

func (c *Config) LogPath() string { return filepath.Join(c.StateDir(), "console.log") }

func (c *Config) PidPath() string { return filepath.Join(c.StateDir(), "daemon.pid") }

func (c *Config) LockPath() string { return filepath.Join(c.dir, LockFileName) }

func (c *Config) CacheDir() string { return filepath.Join(c.StateDir(), "cache") }

func (c *Config) AddonDir() string { return filepath.Join(c.dir, c.Server.Type.AddonDir()) }

func (c *Config) JarPath() string { return filepath.Join(c.dir, c.Server.Jar) }

// JavaMajor is the minimum JDK this server needs. mcjars reports it when the
// core is installed through the wizard, but a directory that was adopted, or
// set up before irori recorded the field, has nothing stored — and treating that
// as "no requirement" is what makes detection settle on whatever ancient JRE it
// finds first. Falling back to the Minecraft version keeps the floor honest.
func (c *Config) JavaMajor() int {
	if c.Java.Major > 0 {
		return c.Java.Major
	}
	return java.RequiredFor(c.Server.MCVersion)
}

func (c *Config) SocketPath() string { return SocketPathFor(c.dir) }

func (c *Config) Save() error {
	if c.dir == "" {
		return errors.New("irori: configuration directory is not set")
	}
	c.normalize()
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := c.Path() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.Path())
}

func (c *Config) EnsureStateDir() error {
	return os.MkdirAll(c.CacheDir(), 0o755)
}

func (c *Config) FindPlugin(key string) (int, bool) {
	for i, p := range c.Plugins {
		if p.Key() == key {
			return i, true
		}
	}
	return -1, false
}

func (c *Config) UpsertPlugin(ref PluginRef) {
	if i, ok := c.FindPlugin(ref.Key()); ok {
		c.Plugins[i] = ref
		return
	}
	c.Plugins = append(c.Plugins, ref)
}

func (c *Config) RemovePlugin(key string) bool {
	i, ok := c.FindPlugin(key)
	if !ok {
		return false
	}
	c.Plugins = append(c.Plugins[:i], c.Plugins[i+1:]...)
	return true
}

func (c *Config) SetOverride(file, key string, value any) {
	if c.Configs == nil {
		c.Configs = map[string]map[string]any{}
	}
	file = filepath.ToSlash(strings.TrimPrefix(file, "./"))
	if c.Configs[file] == nil {
		c.Configs[file] = map[string]any{}
	}
	c.Configs[file][key] = value
}

func (c *Config) HasOverride(file, key string) bool {
	file = filepath.ToSlash(strings.TrimPrefix(file, "./"))
	m, ok := c.Configs[file]
	if !ok {
		return false
	}
	_, ok = m[key]
	return ok
}

func (c *Config) UnsetOverride(file, key string) {
	file = filepath.ToSlash(strings.TrimPrefix(file, "./"))
	if m, ok := c.Configs[file]; ok {
		delete(m, key)
		if len(m) == 0 {
			delete(c.Configs, file)
		}
	}
}
