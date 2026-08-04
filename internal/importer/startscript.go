package importer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bx-team/irori/internal/launch"
)

type StartScript struct {
	Path       string
	Xms        string
	Xmx        string
	Jar        string
	ExtraFlags []string
	Args       []string
	Preset     string
}

var startScriptNames = []string{
	"start.bat", "start.sh", "run.bat", "run.sh",
	"launch.sh", "launch.bat", "startserver.sh", "server.sh",
}

func FindStartScripts(dir string) []string {
	var out []string
	for _, n := range startScriptNames {
		p := filepath.Join(dir, n)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

var (
	javaLineRe = regexp.MustCompile(`(?i)(^|[\s"'])java(\.exe)?["']?\s`)
	memRe      = regexp.MustCompile(`(?i)^-Xm([sx])(\d+[gGmMkK]?)$`)
)

// ParseStartScript pulls heap sizes, JVM flags and the jar name out of an
// existing launcher script. Line continuations (^ in .bat, \ in .sh) are joined
// first, since flag lists are usually wrapped.
func ParseStartScript(path string) (*StartScript, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	text = regexp.MustCompile(`\^\s*\n`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\\\s*\n`).ReplaceAllString(text, " ")

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "::") ||
			strings.HasPrefix(strings.ToLower(line), "rem ") {
			continue
		}
		if !javaLineRe.MatchString(line) || !strings.Contains(strings.ToLower(line), "-jar") {
			continue
		}
		if s := parseJavaLine(line); s != nil {
			s.Path = path
			return s, true
		}
	}
	return nil, false
}

func parseJavaLine(line string) *StartScript {
	tokens := tokenize(line)

	start := -1
	for i, t := range tokens {
		base := strings.ToLower(filepath.Base(strings.Trim(t, `"'`)))
		if base == "java" || base == "java.exe" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}

	s := &StartScript{}
	afterJar := false
	for i := start; i < len(tokens); i++ {
		t := strings.Trim(tokens[i], `"'`)
		switch {
		case afterJar:
			if s.Jar == "" {
				s.Jar = t
				continue
			}
			s.Args = append(s.Args, t)
		case strings.EqualFold(t, "-jar"):
			afterJar = true
		case memRe.MatchString(t):
			m := memRe.FindStringSubmatch(t)
			if strings.EqualFold(m[1], "s") {
				s.Xms = strings.ToUpper(m[2])
			} else {
				s.Xmx = strings.ToUpper(m[2])
			}
		case strings.HasPrefix(t, "-"):
			s.ExtraFlags = append(s.ExtraFlags, t)
		}
	}
	if s.Jar == "" {
		return nil
	}
	s.Preset = detectPreset(s.ExtraFlags, s.Xmx)
	return s
}

// detectPreset collapses an imported flag list back to a known preset when the
// two match, so the config stays declarative instead of a wall of flags.
func detectPreset(flags []string, xmx string) string {
	if len(flags) == 0 {
		return "none"
	}
	heapMB, err := launch.ParseMemMB(xmx)
	if err != nil {
		heapMB = 4096
	}
	for _, p := range launch.Presets() {
		if p.ID == "none" {
			continue
		}
		if sameFlags(flags, p.Flags(heapMB)) {
			return p.ID
		}
	}
	return "none"
}

func sameFlags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int, len(b))
	for _, f := range b {
		set[f]++
	}
	for _, f := range a {
		set[f]--
		if set[f] < 0 {
			return false
		}
	}
	return true
}

// tokenize splits a shell-ish command line, keeping quoted runs together.
func tokenize(line string) []string {
	var out []string
	var cur strings.Builder
	var quote rune

	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// StripPresetFlags removes the flags a preset already provides, leaving only
// the genuinely custom ones.
func StripPresetFlags(flags []string, preset, xmx string) []string {
	if preset == "" || preset == "none" {
		return flags
	}
	heapMB, err := launch.ParseMemMB(xmx)
	if err != nil {
		heapMB = 4096
	}
	known := map[string]bool{}
	for _, f := range launch.GetPreset(preset).Flags(heapMB) {
		known[f] = true
	}
	out := flags[:0:0]
	for _, f := range flags {
		if !known[f] {
			out = append(out, f)
		}
	}
	return out
}
