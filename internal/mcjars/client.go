// Package mcjars talks to https://mcjars.app, the index of Minecraft server
// jars. A build carries its own installation recipe, so irori never needs to
// know how a particular server type is assembled.
package mcjars

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/models"
)

const baseURL = "https://mcjars.app"

type Client struct {
	HTTP      *http.Client
	UserAgent string
}

func New() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: UserAgent,
	}
}

var UserAgent = "irori/dev (https://github.com/bx-team/irori)"

type TypeInfo struct {
	ID            models.ServerType `json:"-"`
	Category      string            `json:"-"`
	Name          string            `json:"name"`
	Icon          string            `json:"icon"`
	Color         string            `json:"color"`
	Homepage      string            `json:"homepage"`
	Deprecated    bool              `json:"deprecated"`
	Experimental  bool              `json:"experimental"`
	Description   string            `json:"description"`
	Categories    []string          `json:"categories"`
	Compatibility []string          `json:"compatibility"`
	Builds        int64             `json:"builds"`
}

type Version struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Supported bool   `json:"supported"`
	Java      int    `json:"java"`
	Builds    int64  `json:"builds"`
	Created   string `json:"created"`
	Latest    *Build `json:"latest"`
}

type Build struct {
	UUID         string            `json:"uuid"`
	VersionID    string            `json:"version_id"`
	Type         models.ServerType `json:"type"`
	Experimental bool              `json:"experimental"`
	Name         string            `json:"name"`
	Installation [][]Step          `json:"installation"`
	Changes      []string          `json:"changes"`
	Created      string            `json:"created"`
}

type Step struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	File     string `json:"file"`
	Size     int64  `json:"size"`
	Location string `json:"location"`
}

func (b Build) CoreJar() string {
	name := "server.jar"
	for _, group := range b.Installation {
		for _, s := range group {
			if s.Type == "download" && strings.HasSuffix(strings.ToLower(s.File), ".jar") {
				name = s.File
			}
		}
	}
	return name
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mcjars %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mcjars %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) Types(ctx context.Context) ([]TypeInfo, error) {
	var resp struct {
		Types map[string]map[string]TypeInfo `json:"types"`
	}
	if err := c.get(ctx, "/api/v2/types", &resp); err != nil {
		return nil, err
	}

	order := []string{"recommended", "established", "experimental", "miscellaneous", "limbos"}
	var out []TypeInfo
	for _, cat := range order {
		group := resp.Types[cat]
		ids := make([]string, 0, len(group))
		for id := range group {
			ids = append(ids, id)
		}
		sortByBuilds(group, ids)
		for _, id := range ids {
			info := group[id]
			info.ID = models.ServerType(id)
			info.Category = cat
			out = append(out, info)
		}
	}
	return out, nil
}

func sortByBuilds(group map[string]TypeInfo, ids []string) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && group[ids[j]].Builds > group[ids[j-1]].Builds; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

type Page struct {
	Total   int64 `json:"total"`
	PerPage int64 `json:"per_page"`
	Page    int64 `json:"page"`
}

func (c *Client) Versions(ctx context.Context, t models.ServerType, page, perPage int, search string) ([]Version, Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	if search != "" {
		q.Set("search", search)
	}
	var resp struct {
		Versions struct {
			Page
			Data []Version `json:"data"`
		} `json:"versions"`
	}
	path := "/api/v3/builds/types/" + url.PathEscape(string(t)) + "/versions?" + q.Encode()
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, Page{}, err
	}
	return resp.Versions.Data, resp.Versions.Page, nil
}

func (c *Client) Builds(ctx context.Context, t models.ServerType, version string, page, perPage int) ([]Build, Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	var resp struct {
		Builds struct {
			Page
			Data []Build `json:"data"`
		} `json:"builds"`
	}
	path := "/api/v3/builds/types/" + url.PathEscape(string(t)) + "/versions/" + url.PathEscape(version) + "?" + q.Encode()
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, Page{}, err
	}
	return resp.Builds.Data, resp.Builds.Page, nil
}

func (c *Client) LatestBuild(ctx context.Context, t models.ServerType, version string) (Build, error) {
	var resp struct {
		Build Build `json:"build"`
	}
	path := "/api/v3/builds/types/" + url.PathEscape(string(t)) + "/versions/" + url.PathEscape(version) + "/latest"
	if err := c.get(ctx, path, &resp); err != nil {
		return Build{}, err
	}
	return resp.Build, nil
}

type Match struct {
	Build   Build
	Version Version
}

// IdentifyBySHA256 looks a jar up by content hash, which is how an already
// existing server directory gets recognised. The result is positional: entry i
// describes sums[i], and is nil when that hash matched nothing.
func (c *Client) IdentifyBySHA256(ctx context.Context, sums []string) ([]*Match, error) {
	type hash struct {
		SHA256 string `json:"sha256"`
	}
	type query struct {
		Hash hash `json:"hash"`
	}
	body := make([]query, 0, len(sums))
	for _, s := range sums {
		body = append(body, query{Hash: hash{SHA256: s}})
	}

	var resp struct {
		Builds []*struct {
			Build   Build   `json:"build"`
			Version Version `json:"version"`
		} `json:"builds"`
	}
	if err := c.post(ctx, "/api/v3/builds/search", body, &resp); err != nil {
		return nil, err
	}

	out := make([]*Match, len(sums))
	for i, r := range resp.Builds {
		if i >= len(out) || r == nil {
			continue
		}
		m := Match{Build: r.Build, Version: r.Version}
		if m.Build.VersionID == "" {
			m.Build.VersionID = r.Version.ID
		}
		out[i] = &m
	}
	return out, nil
}

type Config struct {
	Location string `json:"location"`
	Format   string `json:"format"`
	Type     string `json:"type"`
	Value    string `json:"value"`
}

func (c *Client) Configs(ctx context.Context, buildUUID string) ([]Config, error) {
	if buildUUID == "" {
		return nil, errors.New("mcjars: no build id recorded for this server")
	}
	var resp struct {
		Configs []Config `json:"configs"`
	}
	if err := c.get(ctx, "/api/v3/builds/"+url.PathEscape(buildUUID)+"/configs", &resp); err != nil {
		return nil, err
	}
	return resp.Configs, nil
}

// FindConfig matches a path against the shipped locations, tolerating the
// leading "./" and backslashes a Windows-authored path may carry.
func FindConfig(configs []Config, location string) (Config, bool) {
	want := strings.TrimPrefix(strings.ReplaceAll(location, "\\", "/"), "./")
	for _, c := range configs {
		if strings.TrimPrefix(c.Location, "./") == want {
			return c, true
		}
	}
	return Config{}, false
}
