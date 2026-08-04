package confs

import (
	"bytes"
	"strings"

	"github.com/bx-team/irori/internal/props"
	"gopkg.in/yaml.v3"
)

// parseYAML flattens the document into dotted keys. Scalars at a level come
// before the sub-blocks under it, so each section header is emitted once
// instead of once per run of keys the file happens to interleave.
func parseYAML(raw []byte) ([]Entry, error) {
	var doc yaml.Node
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errNotAMapping
	}

	var out []Entry
	walkYAML(root, "", &out)
	return out, nil
}

type yamlError string

func (e yamlError) Error() string { return string(e) }

const errNotAMapping = yamlError("the top level of this file is not a mapping")

func walkYAML(node *yaml.Node, prefix string, out *[]Entry) {
	var nested []*yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		path := key.Value
		if prefix != "" {
			path = prefix + "." + key.Value
		}
		if value.Kind == yaml.MappingNode && len(value.Content) > 0 {
			nested = append(nested, key, value)
			continue
		}
		*out = append(*out, yamlEntry(path, key, value))
	}
	for i := 0; i+1 < len(nested); i += 2 {
		key, value := nested[i], nested[i+1]
		path := key.Value
		if prefix != "" {
			path = prefix + "." + key.Value
		}
		walkYAML(value, path, out)
	}
}

func yamlEntry(path string, key, value *yaml.Node) Entry {
	section, label := splitKey(path)
	e := Entry{
		Key:     path,
		Label:   label,
		Section: section,
		Desc:    yamlComment(key, value),
		Kind:    props.KindString,
	}

	switch {
	case value.Kind != yaml.ScalarNode:
		e.Value = inlineYAML(value)
		e.ReadOnly = true
	default:
		e.Value = value.Value
		e.Kind = yamlKind(value)
		if e.Kind == props.KindBool {
			// The writers emit canonical booleans, so a file using yes/on must
			// still present the two values the toggle produces.
			e.Value = boolText(strings.EqualFold(value.Value, "true") ||
				strings.EqualFold(value.Value, "yes") || strings.EqualFold(value.Value, "on"))
		}
	}
	// Default stays empty on purpose. The only pristine copy of a fork's config
	// is the one mcjars ships with the build, and restoring from it is a
	// whole-file operation the Files tab already offers.
	return e
}

func yamlKind(n *yaml.Node) props.Kind {
	switch n.Tag {
	case "!!bool":
		return props.KindBool
	case "!!int":
		return props.KindInt
	case "!!float":
		return props.KindFloat
	default:
		return props.KindString
	}
}

// yamlComment prefers the block above a key, which is where every Paper-family
// config puts its prose, and falls back to a trailing comment on either the key
// or its value.
func yamlComment(key, value *yaml.Node) string {
	for _, c := range []string{key.HeadComment, key.LineComment, value.LineComment, value.HeadComment} {
		if text := cleanComment(c); text != "" {
			return text
		}
	}
	return ""
}

// inlineYAML renders a list or block on one line, purely for display.
func inlineYAML(n *yaml.Node) string {
	if n.Kind == yaml.SequenceNode && len(n.Content) == 0 {
		return "[]"
	}
	if n.Kind == yaml.MappingNode && len(n.Content) == 0 {
		return "{}"
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return "…"
	}
	enc.Close()

	fields := strings.Fields(buf.String())
	text := strings.Join(fields, " ")
	if len(text) > 200 {
		text = text[:200] + "…"
	}
	return text
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
