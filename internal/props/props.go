// Package props reads and writes server.properties while preserving the
// original line order, comments and blank lines, so editing one key never
// rewrites the whole file.
package props

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type line struct {
	key   string
	value string
	raw   string
}

func (l line) isPair() bool { return l.key != "" }

type File struct {
	lines []line
	index map[string]int
	eol   string
}

func Parse(data []byte) *File {
	eol := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
	}
	f := &File{index: map[string]int{}, eol: eol}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return f
	}
	for _, raw := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			f.lines = append(f.lines, line{raw: raw})
			continue
		}
		eq := strings.Index(raw, "=")
		if eq < 0 {
			f.lines = append(f.lines, line{raw: raw})
			continue
		}
		key := strings.TrimSpace(raw[:eq])
		if key == "" {
			f.lines = append(f.lines, line{raw: raw})
			continue
		}
		f.index[key] = len(f.lines)
		f.lines = append(f.lines, line{key: key, value: raw[eq+1:]})
	}
	return f
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{index: map[string]int{}, eol: "\n"}, nil
		}
		return nil, err
	}
	return Parse(data), nil
}

func (f *File) Get(key string) (string, bool) {
	i, ok := f.index[key]
	if !ok {
		return "", false
	}
	return f.lines[i].value, true
}

func (f *File) GetOr(key, def string) string {
	if v, ok := f.Get(key); ok {
		return v
	}
	return def
}

func (f *File) Int(key string, def int) int {
	v, ok := f.Get(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

func (f *File) Bool(key string, def bool) bool {
	v, ok := f.Get(key)
	if !ok {
		return def
	}
	return strings.TrimSpace(v) == "true"
}

func (f *File) Set(key, value string) {
	if i, ok := f.index[key]; ok {
		f.lines[i].value = value
		return
	}
	f.index[key] = len(f.lines)
	f.lines = append(f.lines, line{key: key, value: value})
}

func (f *File) Delete(key string) {
	i, ok := f.index[key]
	if !ok {
		return
	}
	f.lines = append(f.lines[:i], f.lines[i+1:]...)
	delete(f.index, key)
	for k, idx := range f.index {
		if idx > i {
			f.index[k] = idx - 1
		}
	}
}

func (f *File) Has(key string) bool {
	_, ok := f.index[key]
	return ok
}

// Keys returns the keys in file order.
func (f *File) Keys() []string {
	out := make([]string, 0, len(f.index))
	for _, l := range f.lines {
		if l.isPair() {
			out = append(out, l.key)
		}
	}
	return out
}

func (f *File) Bytes() []byte {
	var b strings.Builder
	for _, l := range f.lines {
		if l.isPair() {
			b.WriteString(l.key)
			b.WriteString("=")
			b.WriteString(l.value)
		} else {
			b.WriteString(l.raw)
		}
		b.WriteString(f.eol)
	}
	return []byte(b.String())
}

func (f *File) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".irori.tmp"
	if err := os.WriteFile(tmp, f.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
