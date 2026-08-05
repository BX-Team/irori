package modrinth

import (
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Progress struct {
	File  string
	Done  int64
	Total int64
}

func (c *Client) Download(ctx context.Context, f File, dst string, onProgress func(Progress)) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("modrinth: downloading %s: %w", f.Filename, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("modrinth: downloading %s: %s", f.Filename, resp.Status)
	}

	tmp := dst + ".irori.part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()

	var sum hash.Hash
	var want string
	switch {
	case f.Hashes.SHA512 != "":
		sum, want = sha512.New(), f.Hashes.SHA512
	case f.Hashes.SHA1 != "":
		sum, want = sha1.New(), f.Hashes.SHA1
	}

	total := f.Size
	if total == 0 {
		total = resp.ContentLength
	}
	src := io.Reader(resp.Body)
	if sum != nil {
		src = io.TeeReader(src, sum)
	}
	if onProgress != nil {
		src = &progressReader{r: src, name: f.Filename, total: total, report: onProgress}
	}

	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	if sum != nil {
		if got := hex.EncodeToString(sum.Sum(nil)); got != want {
			return fmt.Errorf("modrinth: %s failed its checksum, the download was not kept", f.Filename)
		}
	}
	return os.Rename(tmp, dst)
}

type progressReader struct {
	r      io.Reader
	name   string
	done   int64
	total  int64
	last   time.Time
	report func(Progress)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if time.Since(p.last) > 80*time.Millisecond || err == io.EOF {
		p.last = time.Now()
		p.report(Progress{File: p.name, Done: p.done, Total: p.total})
	}
	return n, err
}
