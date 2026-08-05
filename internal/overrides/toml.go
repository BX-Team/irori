package overrides

import (
	"fmt"
	"strconv"
	"strings"
)

// applyTOML edits line by line. The TOML encoders available drop comments on
// re-encode, and velocity.toml is mostly comments explaining each option.
//
//nolint:unparam // shares the applier signature; see applyProperties.
func applyTOML(raw []byte, file string, keys []string, values map[string]any) ([]byte, []Change, error) {
	eol := "\n"
	if strings.Contains(string(raw), "\r\n") {
		eol = "\r\n"
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	var changes []Change
	for _, key := range keys {
		table, name := splitTOMLKey(key)
		want := tomlValue(values[key])

		if idx, cur, ok := findTOML(lines, table, name); ok {
			if cur == want {
				continue
			}
			changes = append(changes, Change{File: file, Key: key, From: cur, To: want})
			indent := lines[idx][:len(lines[idx])-len(strings.TrimLeft(lines[idx], " \t"))]
			lines[idx] = indent + name + " = " + want
			continue
		}
		changes = append(changes, Change{File: file, Key: key, To: want, New: true})
		lines = insertTOML(lines, table, name+" = "+want)
	}

	if len(changes) == 0 {
		return raw, nil, nil
	}
	return []byte(strings.Join(lines, eol) + eol), changes, nil
}

func splitTOMLKey(key string) (table, name string) {
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "", key
	}
	return key[:i], key[i+1:]
}

func findTOML(lines []string, table, name string) (int, string, bool) {
	current := ""
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			current = strings.Trim(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"), "[]")
			continue
		}
		if current != table {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		if strings.Trim(strings.TrimSpace(line[:eq]), `"'`) == name {
			return i, strings.TrimSpace(line[eq+1:]), true
		}
	}
	return 0, "", false
}

func insertTOML(lines []string, table, assignment string) []string {
	if table == "" {
		// Root keys must stay above the first table header.
		for i, raw := range lines {
			if strings.HasPrefix(strings.TrimSpace(raw), "[") {
				return insertAt(lines, i, assignment)
			}
		}
		return append(lines, assignment)
	}

	start := -1
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		name := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"), "[]")
		if name == table {
			start = i
			continue
		}
		if start >= 0 {
			return insertAt(lines, i, assignment)
		}
	}
	if start >= 0 {
		return append(lines, assignment)
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	return append(lines, "["+table+"]", assignment)
}

func insertAt(lines []string, at int, value string) []string {
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, value)
	return append(out, lines[at:]...)
}

func tomlValue(v any) string {
	switch t := v.(type) {
	case nil:
		return `""`
	case string:
		return strconv.Quote(t)
	case bool:
		return Scalar(t)
	case float64, int, int64:
		return Scalar(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, tomlValue(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return strconv.Quote(fmt.Sprint(t))
	}
}
