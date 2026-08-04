package launch

import (
	"fmt"
	"strings"

	"github.com/bx-team/irori/internal/config"
)

type Spec struct {
	Java string
	Args []string
	Dir  string
}

// Build renders the full java command line from a config. It does not resolve
// the java binary; pass the already-resolved path.
func Build(cfg *config.Config, javaPath string) (Spec, error) {
	xmxMB, err := ParseMemMB(cfg.Java.Xmx)
	if err != nil {
		return Spec{}, fmt.Errorf("xmx: %w", err)
	}
	xmsMB, err := ParseMemMB(cfg.Java.Xms)
	if err != nil {
		return Spec{}, fmt.Errorf("xms: %w", err)
	}
	if xmsMB > xmxMB {
		xmsMB = xmxMB
	}

	args := []string{"-Xms" + FormatMemMB(xmsMB), "-Xmx" + FormatMemMB(xmxMB)}
	args = append(args, GetPreset(cfg.Java.Preset).Flags(xmxMB)...)
	args = append(args, cfg.Java.ExtraFlags...)
	args = append(args, "-jar", cfg.Server.Jar)
	args = append(args, serverArgs(cfg)...)

	return Spec{Java: javaPath, Args: args, Dir: cfg.Dir()}, nil
}

func serverArgs(cfg *config.Config) []string {
	if cfg.Java.Args != nil {
		return cfg.Java.Args
	}
	if cfg.Server.Type.IsProxy() {
		return nil
	}
	return []string{"--nogui"}
}

func (s Spec) String() string {
	parts := make([]string, 0, len(s.Args)+1)
	parts = append(parts, quote(s.Java))
	for _, a := range s.Args {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\"'$") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
