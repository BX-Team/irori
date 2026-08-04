package files

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/bx-team/irori/internal/addons"
	"github.com/bx-team/irori/internal/host"
)

type PreviewKind int

const (
	PreviewEmpty PreviewKind = iota
	PreviewDir
	PreviewText
	PreviewJar
	PreviewBinary
	PreviewError
)

type KV struct {
	Key   string
	Value string
}

type Preview struct {
	Kind    PreviewKind
	Title   string
	Lines   []string
	Fields  []KV
	Entries []host.Entry
	Note    string
}

const (
	textSniffBytes = 8 << 10
	maxJarBytes    = 96 << 20
)

func Build(h host.Backend, p string, e host.Entry, maxLines int) Preview {
	if e.IsDir {
		entries, err := h.ReadDir(p)
		if err != nil {
			return Preview{Kind: PreviewError, Note: err.Error()}
		}
		Sort(entries)
		return Preview{Kind: PreviewDir, Title: e.Name, Entries: entries,
			Note: fmt.Sprintf("%d item(s)", len(entries))}
	}

	if isJar(e.Name) {
		return jarPreview(h, p, e)
	}

	if e.Size == 0 {
		return Preview{Kind: PreviewEmpty, Title: e.Name, Note: "empty file"}
	}

	raw, err := readHead(h, p, textSniffBytes)
	if err != nil {
		return Preview{Kind: PreviewError, Title: e.Name, Note: err.Error()}
	}
	if !looksTextual(raw) {
		return Preview{Kind: PreviewBinary, Title: e.Name,
			Note: fmt.Sprintf("binary file, %s", humanSize(e.Size))}
	}

	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	truncated := len(lines) > maxLines
	if truncated {
		lines = lines[:maxLines]
	}
	if e.Size > textSniffBytes && !truncated && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return Preview{Kind: PreviewText, Title: e.Name, Lines: lines,
		Note: humanSize(e.Size)}
}

func jarPreview(h host.Backend, p string, e host.Entry) Preview {
	pv := Preview{Kind: PreviewJar, Title: e.Name, Note: humanSize(e.Size)}
	if e.Size > maxJarBytes {
		pv.Note += " — too large to inspect"
		return pv
	}
	raw, err := h.ReadFile(p)
	if err != nil {
		return Preview{Kind: PreviewError, Title: e.Name, Note: err.Error()}
	}
	meta, err := addons.ReadMeta(raw)
	if err != nil {
		pv.Note += " — " + err.Error()
		return pv
	}

	pv.Fields = []KV{{"name", meta.Display()}}
	if meta.Version != "" {
		pv.Fields = append(pv.Fields, KV{"version", meta.Version})
	}
	if len(meta.Authors) > 0 {
		pv.Fields = append(pv.Fields, KV{"author", strings.Join(meta.Authors, ", ")})
	}
	if meta.Loader != "" {
		pv.Fields = append(pv.Fields, KV{"loader", meta.Loader})
	}
	if meta.APIVersion != "" {
		pv.Fields = append(pv.Fields, KV{"api", meta.APIVersion})
	}
	if len(meta.Depends) > 0 {
		pv.Fields = append(pv.Fields, KV{"depends", strings.Join(meta.Depends, ", ")})
	}
	if meta.Website != "" {
		pv.Fields = append(pv.Fields, KV{"site", meta.Website})
	}
	if meta.Description != "" {
		pv.Lines = strings.Split(meta.Description, "\n")
	}
	return pv
}

func readHead(h host.Backend, p string, n int) ([]byte, error) {
	f, err := h.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, n)
	read := 0
	for read < n {
		got, err := f.Read(buf[read:])
		read += got
		if err != nil || got == 0 {
			break
		}
	}
	return buf[:read], nil
}

func looksTextual(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return false
	}
	bad := 0
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			bad++
		}
		i += size
	}
	return bad*100 <= len(raw)
}

func isJar(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jar", ".zip":
		return true
	}
	return false
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
