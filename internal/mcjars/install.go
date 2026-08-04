package mcjars

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Progress struct {
	Step       int
	Total      int
	Label      string
	Downloaded int64
	Size       int64
}

// Install runs a build's installation recipe inside dir. Steps come from the
// API so new server types work without code changes here.
func (c *Client) Install(ctx context.Context, b Build, dir string, onProgress func(Progress)) error {
	steps := make([]Step, 0, 4)
	for _, group := range b.Installation {
		steps = append(steps, group...)
	}
	if len(steps) == 0 {
		return errors.New("build has no installation steps")
	}

	report := func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	for i, s := range steps {
		p := Progress{Step: i + 1, Total: len(steps)}
		switch s.Type {
		case "download":
			p.Label = "downloading " + s.File
			p.Size = s.Size
			report(p)
			if err := c.download(ctx, s, dir, func(n int64) {
				p.Downloaded = n
				report(p)
			}); err != nil {
				return fmt.Errorf("downloading %s: %w", s.File, err)
			}
		case "unzip":
			p.Label = "extracting " + s.File
			report(p)
			if err := unzip(filepath.Join(dir, s.File), safeJoin(dir, s.Location)); err != nil {
				return fmt.Errorf("extracting %s: %w", s.File, err)
			}
		case "remove":
			p.Label = "removing " + s.Location
			report(p)
			if err := os.RemoveAll(safeJoin(dir, s.Location)); err != nil {
				return fmt.Errorf("removing %s: %w", s.Location, err)
			}
		default:
			return fmt.Errorf("unknown installation step %q", s.Type)
		}
	}
	return nil
}

func (c *Client) download(ctx context.Context, s Step, dir string, onBytes func(int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	client := c.HTTP
	if client == nil || client.Timeout > 0 {
		client = &http.Client{Timeout: 30 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}

	dst := safeJoin(dir, s.File)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, &countingReader{r: resp.Body, onRead: onBytes})
	closeErr := f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dst)
}

type countingReader struct {
	r      io.Reader
	n      int64
	last   time.Time
	onRead func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.onRead != nil && time.Since(c.last) > 150*time.Millisecond {
		c.last = time.Now()
		c.onRead(c.n)
	}
	return n, err
}

// safeJoin keeps API-provided paths from escaping the server directory.
func safeJoin(dir, rel string) string {
	return filepath.Join(dir, filepath.Clean("/"+rel))
}

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		target := safeJoin(dst, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) && target != filepath.Clean(dst) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := extractFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc) //nolint:gosec // sizes come from a trusted index
	return err
}
