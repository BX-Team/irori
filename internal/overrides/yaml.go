package overrides

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

// applyYAML edits through the node tree rather than round-tripping a map, which
// is what keeps comments and key order intact.
func applyYAML(raw []byte, file string, keys []string, values map[string]any) ([]byte, []Change, error) {
	var doc yaml.Node
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, nil, err
		}
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		}}
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, errNotAMapping
	}

	var changes []Change
	for _, key := range keys {
		want := toYAMLNode(values[key])
		node, parent, name, found := findYAML(root, strings.Split(key, "."))
		switch {
		case found && sameYAML(node, want):
			continue
		case found:
			changes = append(changes, Change{File: file, Key: key, From: renderYAML(node), To: renderYAML(want)})
			*node = *want
		default:
			changes = append(changes, Change{File: file, Key: key, To: renderYAML(want), New: true})
			parent.Content = append(parent.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, want)
		}
	}
	if len(changes) == 0 {
		return raw, nil, nil
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), changes, nil
}

var errNotAMapping = yamlError("top level of the file is not a mapping")

type yamlError string

func (e yamlError) Error() string { return string(e) }

// findYAML walks a dotted path, creating intermediate mappings as it goes. It
// returns the value node when the whole path exists, otherwise the mapping the
// final key must be appended to.
func findYAML(root *yaml.Node, path []string) (node, parent *yaml.Node, name string, found bool) {
	cur := root
	for i, part := range path {
		last := i == len(path)-1
		var value *yaml.Node
		for j := 0; j+1 < len(cur.Content); j += 2 {
			if cur.Content[j].Value == part {
				value = cur.Content[j+1]
				break
			}
		}
		if value == nil {
			if last {
				return nil, cur, part, false
			}
			key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: part}
			value = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			cur.Content = append(cur.Content, key, value)
			cur = value
			continue
		}
		if last {
			return value, cur, part, true
		}
		if value.Kind != yaml.MappingNode {
			*value = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		cur = value
	}
	return nil, cur, "", false
}

func toYAMLNode(v any) *yaml.Node {
	n := &yaml.Node{}
	if err := n.Encode(v); err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: Scalar(v)}
	}
	// Encode wraps values in a document when they are composite.
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		return n.Content[0]
	}
	return n
}

func sameYAML(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Kind == yaml.ScalarNode && b.Kind == yaml.ScalarNode {
		return a.Value == b.Value
	}
	return renderYAML(a) == renderYAML(b)
}

func renderYAML(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return ""
	}
	if err := enc.Close(); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}
