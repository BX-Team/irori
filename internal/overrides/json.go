package overrides

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// applyJSON rewrites the whole document, since JSON carries no comments to
// preserve. Key order is normalised by the encoder.
func applyJSON(raw []byte, file string, keys []string, values map[string]any) ([]byte, []Change, error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, nil, err
		}
	}

	var changes []Change
	for _, key := range keys {
		want := values[key]
		cur, found := getJSON(root, strings.Split(key, "."))
		if found && sameJSON(cur, want) {
			continue
		}
		changes = append(changes, Change{
			File: file, Key: key, From: renderJSON(cur), To: renderJSON(want), New: !found,
		})
		setJSON(root, strings.Split(key, "."), want)
	}
	if len(changes) == 0 {
		return raw, nil, nil
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), changes, nil
}

func getJSON(root map[string]any, path []string) (any, bool) {
	cur := any(root)
	for _, part := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func setJSON(root map[string]any, path []string, value any) {
	cur := root
	for _, part := range path[:len(path)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = value
}

func sameJSON(a, b any) bool {
	ar, err1 := json.Marshal(a)
	br, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(ar, br)
}

func renderJSON(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}
