// Package importer recognises a Minecraft server that already exists in a
// directory: which jar is the core, which build it is, and how it used to be
// started.
package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/models"
)

// skipDirs are never scanned for a core jar: they hold addons and libraries,
// not the server itself.
var skipDirs = map[string]bool{
	"plugins": true, "mods": true, "libraries": true, "cache": true,
	"versions": true, "world": true, "logs": true, ".irori": true,
	"bundler": true, "world_nether": true, "world_the_end": true,
}

type Candidate struct {
	File    string
	Size    int64
	SHA256  string
	Build   *mcjars.Build
	Type    models.ServerType
	Version string
	// Java is the minimum JDK mcjars records for the version this jar belongs
	// to, or 0 when the jar was not recognised.
	Java int
}

func (c Candidate) Identified() bool { return c.Build != nil }

func (c Candidate) Describe() string {
	switch {
	case c.Build != nil:
		return c.Build.Type.Display() + " " + c.Build.VersionID + " " + c.Build.Name
	case c.Type != "" && c.Version != "":
		return c.Type.Display() + " " + c.Version + " (guessed from filename)"
	case c.Version != "":
		return "Minecraft " + c.Version + " (guessed from filename)"
	default:
		return "unknown server core"
	}
}

// ScanJars finds candidate core jars in the top two levels of dir.
func ScanJars(dir string) ([]Candidate, error) {
	var out []Candidate
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable entry is skipped, not fatal to the scan
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			if p == dir {
				return nil
			}
			if skipDirs[strings.ToLower(d.Name())] || strings.HasPrefix(d.Name(), ".") ||
				strings.Count(rel, string(filepath.Separator)) >= 1 {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".jar") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // a jar that vanished mid-walk is not a candidate
		}
		t, v := guessFromName(d.Name())
		out = append(out, Candidate{File: filepath.ToSlash(rel), Size: info.Size(), Type: t, Version: v})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out, nil
}

var (
	versionRe = regexp.MustCompile(`\b(1\.\d{1,2}(?:\.\d{1,2})?|\d{2}\.\d{1,2}(?:\.\d{1,2})?)\b`)
	namePairs = []struct {
		token string
		typ   models.ServerType
	}{
		{"paper", models.TypePaper},
		{"purpur", models.TypePurpur},
		{"pufferfish", models.TypePufferfish},
		{"folia", models.TypeFolia},
		{"spigot", models.TypeSpigot},
		{"leaves", models.TypeLeaves},
		{"leaf", models.TypeLeaf},
		{"canvas", models.TypeCanvas},
		{"divinemc", models.TypeDivineMC},
		{"velocity", models.TypeVelocity},
		{"waterfall", models.TypeWaterfall},
		{"bungeecord", models.TypeBungeeCord},
		{"fabric", models.TypeFabric},
		{"quilt", models.TypeQuilt},
		{"neoforge", models.TypeNeoForge},
		{"forge", models.TypeForge},
		{"mohist", models.TypeMohist},
		{"arclight", models.TypeArclight},
		{"magma", models.TypeMagma},
		{"sponge", models.TypeSponge},
		{"minecraft_server", models.TypeVanilla},
		{"server", ""},
	}
)

func guessFromName(name string) (models.ServerType, string) {
	lower := strings.ToLower(name)
	var typ models.ServerType
	for _, p := range namePairs {
		if strings.Contains(lower, p.token) {
			typ = p.typ
			break
		}
	}
	version := ""
	if m := versionRe.FindStringSubmatch(lower); m != nil {
		version = m[1]
	}
	return typ, version
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Identify hashes each candidate and asks mcjars which build it is. Failures
// are not fatal: the filename guess remains as a fallback.
func Identify(ctx context.Context, c *mcjars.Client, dir string, cands []Candidate) ([]Candidate, error) {
	sums := make([]string, 0, len(cands))
	for i := range cands {
		sum, err := HashFile(filepath.Join(dir, cands[i].File))
		if err != nil {
			sum = ""
		}
		cands[i].SHA256 = sum
		sums = append(sums, sum)
	}
	if len(sums) == 0 {
		return cands, nil
	}

	matches, err := c.IdentifyBySHA256(ctx, sums)
	if err != nil {
		return cands, err
	}
	for i := range cands {
		if i >= len(matches) || matches[i] == nil {
			continue
		}
		build := matches[i].Build
		cands[i].Build = &build
		cands[i].Type = build.Type
		cands[i].Version = build.VersionID
		cands[i].Java = matches[i].Version.Java
	}
	return cands, nil
}
