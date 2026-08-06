package java

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type JDK struct {
	Path    string `json:"path"`
	Major   int    `json:"major"`
	Version string `json:"version"`
	Vendor  string `json:"vendor"`
	Source  string `json:"source"`
	Graal   Graal  `json:"graal,omitempty"`
}

type Graal string

const (
	GraalNone      Graal = ""
	GraalCommunity Graal = "community"
	GraalOracle    Graal = "oracle"
)

func (j JDK) Display() string {
	if j.Path == "" {
		return "not found"
	}
	v := j.Version
	if v == "" {
		v = strconv.Itoa(j.Major)
	}
	if j.Vendor != "" {
		return fmt.Sprintf("%s (%s)", v, j.Vendor)
	}
	return v
}

var ErrNotFound = errors.New("no suitable Java runtime found")

func Resolve(ctx context.Context, explicit string, requiredMajor int) (JDK, error) {
	if explicit != "" {
		jdk, err := Probe(ctx, explicit)
		if err != nil {
			return JDK{}, fmt.Errorf("java.path %q: %w", explicit, err)
		}
		jdk.Source = "config"
		return jdk, nil
	}

	var best JDK
	consider := func(j JDK) bool {
		if requiredMajor > 0 && j.Major < requiredMajor {
			return false
		}
		if best.Path == "" || betterJDK(j, best, requiredMajor) {
			best = j
		}
		return requiredMajor > 0 && best.Major == requiredMajor
	}

	for _, c := range quickCandidates() {
		jdk, err := Probe(ctx, c.path)
		if err != nil {
			continue
		}
		jdk.Source = c.source
		if consider(jdk) {
			return best, nil
		}
	}

	for _, c := range nixCandidates(requiredMajor) {
		jdk, err := Probe(ctx, c.path)
		if err != nil {
			continue
		}
		jdk.Source = "nix-store"
		if consider(jdk) {
			return best, nil
		}
	}

	if best.Path == "" {
		return JDK{}, ErrNotFound
	}
	return best, nil
}

// betterJDK ranks two candidates that both satisfy the requirement. When a
// version is required the closest one above it wins, because a server built
// against Java 21 is tested on 21 far more than on whatever is newest. When
// nothing is required there is no target to be close to, so the newest JDK
// wins — reaching for the oldest instead is how a machine with a stray Java 8
// in the nix store ends up running a modern server on 1.8.0.
func betterJDK(j, best JDK, requiredMajor int) bool {
	if requiredMajor > 0 {
		return j.Major < best.Major
	}
	return j.Major > best.Major
}

type candidate struct {
	path   string
	source string
}

func quickCandidates() []candidate {
	var out []candidate
	seen := map[string]bool{}
	add := func(p, src string) {
		if p == "" {
			return
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		if seen[p] {
			return
		}
		if fi, err := os.Stat(p); err != nil || fi.IsDir() {
			return
		}
		seen[p] = true
		out = append(out, candidate{p, src})
	}

	if h := os.Getenv("JAVA_HOME"); h != "" {
		add(filepath.Join(h, "bin", javaBin), "JAVA_HOME")
	}
	if p, err := exec.LookPath("java"); err == nil {
		add(p, "PATH")
	}
	for _, pat := range systemGlobs {
		matches, _ := filepath.Glob(pat)
		for _, m := range matches {
			add(m, "system")
		}
	}
	return out
}

var storeVersionRe = regexp.MustCompile(`-(?:openjdk|temurin-bin|zulu|graalvm[^-]*|jdk|jre)-?(\d+)`)

func nixCandidates(requiredMajor int) []candidate {
	if _, err := os.Stat("/nix/store"); err != nil {
		return nil
	}
	patterns := []string{
		"/nix/store/*-openjdk-*/bin/java",
		"/nix/store/*-temurin-bin-*/bin/java",
		"/nix/store/*-zulu*/bin/java",
		"/nix/store/*-graalvm*/bin/java",
		"/nix/store/*-jdk*/bin/java",
	}

	type scored struct {
		candidate
		major int
	}
	var found []scored
	seen := map[string]bool{}
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		for _, m := range matches {
			if seen[m] || strings.Contains(m, "-man") || strings.Contains(m, "-doc") {
				continue
			}
			seen[m] = true
			major := 0
			if g := storeVersionRe.FindStringSubmatch(m); g != nil {
				major, _ = strconv.Atoi(g[1])
				if major == 1 {
					major = 8
				}
			}
			if requiredMajor > 0 && major > 0 && major < requiredMajor {
				continue
			}
			found = append(found, scored{candidate{m, "nix-store"}, major})
		}
	}

	// Only maxProbe entries are ever executed, so the ordering here has to put
	// the ones Resolve would prefer first — ascending towards a required
	// version, descending when there is none.
	sort.Slice(found, func(i, j int) bool {
		if found[i].major != found[j].major {
			if found[i].major == 0 {
				return false
			}
			if found[j].major == 0 {
				return true
			}
			if requiredMajor > 0 {
				return found[i].major < found[j].major
			}
			return found[i].major > found[j].major
		}
		return found[i].path < found[j].path
	})

	const maxProbe = 6
	out := make([]candidate, 0, maxProbe)
	for i, f := range found {
		if i >= maxProbe {
			break
		}
		out = append(out, f.candidate)
	}
	return out
}

var propRe = regexp.MustCompile(`(?m)^\s{2,}([a-zA-Z0-9_.]+)\s*=\s*(.+)$`)

func Probe(ctx context.Context, path string) (JDK, error) {
	if !filepath.IsAbs(path) {
		resolved, err := exec.LookPath(path)
		if err != nil {
			return JDK{}, err
		}
		path = resolved
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "-XshowSettings:properties", "-version")
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return JDK{}, err
	}

	jdk := JDK{Path: path}
	var brand []string
	for _, m := range propRe.FindAllStringSubmatch(string(out), -1) {
		v := strings.TrimSpace(m[2])
		switch m[1] {
		case "java.version":
			jdk.Version = v
		case "java.vendor":
			jdk.Vendor = v
		case "java.specification.version":
			if jdk.Major == 0 {
				jdk.Major = MajorOf(v)
			}
		case "java.vendor.version", "java.vm.name", "java.vm.version", "java.runtime.name":
			brand = append(brand, v)
		}
	}
	jdk.Graal = classifyGraal(brand)
	if jdk.Version != "" {
		jdk.Major = MajorOf(jdk.Version)
	}
	if jdk.Major == 0 {
		return JDK{}, fmt.Errorf("could not determine java version at %s", path)
	}
	return jdk, nil
}

func classifyGraal(brand []string) Graal {
	s := strings.ToLower(strings.Join(brand, " "))
	if !strings.Contains(s, "graalvm") {
		return GraalNone
	}
	if strings.Contains(s, "oracle graalvm") || strings.Contains(s, "graalvm ee") {
		return GraalOracle
	}
	return GraalCommunity
}

func MajorOf(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '_' || r == '-' || r == '+' })
	if len(parts) == 0 {
		return 0
	}
	first, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	if first == 1 && len(parts) > 1 {
		second, err := strconv.Atoi(parts[1])
		if err == nil {
			return second
		}
	}
	return first
}

func InstallHint(major int) string {
	v := strconv.Itoa(major)
	if major <= 0 {
		v = "21"
	}
	if _, err := os.Stat("/nix/store"); err == nil {
		return "nix profile install nixpkgs#temurin-bin-" + v +
			"   (or nix-shell -p temurin-bin-" + v + ")"
	}
	return "install JDK " + v + " (package temurin-" + v + "-jdk / openjdk-" + v + "-jdk)"
}
