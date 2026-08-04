package confs

import (
	"strconv"
	"strings"

	"github.com/bx-team/irori/internal/props"
)

// parseTOML scans line by line rather than decoding, for the same reason the
// writer does: velocity.toml is mostly comments explaining each option, and a
// decoder hands back values with the prose already thrown away.
func parseTOML(raw []byte) []Entry {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")

	var (
		out     []Entry
		table   string
		comment []string
	)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			// A blank line ends the block, so a file header does not end up
			// attached to the first key under it.
			comment = comment[:0]
			continue
		case strings.HasPrefix(trimmed, "#"):
			comment = append(comment, trimmed)
			continue
		case strings.HasPrefix(trimmed, "["):
			table = strings.Trim(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"), "[]")
			comment = comment[:0]
			continue
		}

		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			comment = comment[:0]
			continue
		}
		name := strings.Trim(strings.TrimSpace(trimmed[:eq]), `"'`)
		if name == "" {
			comment = comment[:0]
			continue
		}
		literal := strings.TrimSpace(trimmed[eq+1:])

		key := name
		if table != "" {
			key = table + "." + name
		}
		value, kind, readOnly := tomlLiteral(literal)
		out = append(out, Entry{
			Key:      key,
			Label:    name,
			Section:  table,
			Value:    value,
			Kind:     kind,
			Desc:     cleanComment(strings.Join(comment, "\n")),
			ReadOnly: readOnly,
		})
		comment = comment[:0]
	}
	return out
}

// tomlLiteral reads a value's type off its syntax. Arrays and inline tables are
// shown but not edited: rewriting one from a single-line form is how a config
// loses its structure.
func tomlLiteral(literal string) (value string, kind props.Kind, readOnly bool) {
	// A trailing comment belongs to the prose, not to the value.
	if i := strings.Index(literal, " #"); i >= 0 && !strings.HasPrefix(literal, `"`) {
		literal = strings.TrimSpace(literal[:i])
	}
	switch {
	case literal == "true" || literal == "false":
		return literal, props.KindBool, false
	case strings.HasPrefix(literal, "[") || strings.HasPrefix(literal, "{"):
		return literal, props.KindString, true
	case strings.HasPrefix(literal, `"`) || strings.HasPrefix(literal, "'"):
		if unquoted, err := strconv.Unquote(literal); err == nil {
			return unquoted, props.KindString, false
		}
		return strings.Trim(literal, `"'`), props.KindString, false
	}
	if _, err := strconv.Atoi(literal); err == nil {
		return literal, props.KindInt, false
	}
	if _, err := strconv.ParseFloat(literal, 64); err == nil {
		return literal, props.KindFloat, false
	}
	return literal, props.KindString, false
}
