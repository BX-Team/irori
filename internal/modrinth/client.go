// Package modrinth is a client for the Modrinth v2 API, used to search for and
// install plugins and mods.
package modrinth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	BaseURL   = "https://api.modrinth.com/v2"
	userAgent = "bx-team/irori (github.com/bx-team/irori)"

	// Modrinth allows 300 requests a minute; staying a little under that keeps
	// browsing responsive without ever being throttled.
	minInterval = 220 * time.Millisecond
)

type Client struct {
	http *http.Client
	base string

	mu   sync.Mutex
	last time.Time
}

func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
		base: BaseURL,
	}
}

type SearchResult struct {
	ProjectID     string   `json:"project_id"`
	ProjectType   string   `json:"project_type"`
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Author        string   `json:"author"`
	Categories    []string `json:"categories"`
	Versions      []string `json:"versions"`
	Downloads     int64    `json:"downloads"`
	Follows       int64    `json:"follows"`
	IconURL       string   `json:"icon_url"`
	LatestVersion string   `json:"latest_version"`
	ClientSide    string   `json:"client_side"`
	ServerSide    string   `json:"server_side"`
	DateModified  string   `json:"date_modified"`
	License       string   `json:"license"`
}

// loaderCategories are the values Modrinth indexes loaders under. They share
// the categories field with real tags, so a list of what a project actually
// does has to have them taken back out.
var loaderCategories = map[string]bool{
	"bukkit": true, "spigot": true, "paper": true, "purpur": true, "folia": true,
	"fabric": true, "forge": true, "neoforge": true, "quilt": true, "rift": true,
	"velocity": true, "bungeecord": true, "waterfall": true, "sponge": true,
	"liteloader": true, "modloader": true, "datapack": true, "minecraft": true,
}

func (r SearchResult) Tags() []string {
	out := make([]string, 0, len(r.Categories))
	for _, c := range r.Categories {
		if !loaderCategories[c] {
			out = append(out, c)
		}
	}
	return out
}

func (r SearchResult) SupportedLoaders() []string {
	out := make([]string, 0, 4)
	for _, c := range r.Categories {
		if loaderCategories[c] && c != "minecraft" && c != "datapack" {
			out = append(out, c)
		}
	}
	return out
}

type SearchPage struct {
	Hits      []SearchResult `json:"hits"`
	Offset    int            `json:"offset"`
	Limit     int            `json:"limit"`
	TotalHits int            `json:"total_hits"`
}

type Project struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	Categories  []string `json:"categories"`
	Downloads   int64    `json:"downloads"`
	SourceURL   string   `json:"source_url"`
	IssuesURL   string   `json:"issues_url"`
	WikiURL     string   `json:"wiki_url"`
	Updated     string   `json:"updated"`
}

type Version struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	Name          string       `json:"name"`
	VersionNumber string       `json:"version_number"`
	VersionType   string       `json:"version_type"`
	DatePublished string       `json:"date_published"`
	Downloads     int64        `json:"downloads"`
	Loaders       []string     `json:"loaders"`
	GameVersions  []string     `json:"game_versions"`
	Changelog     string       `json:"changelog"`
	Dependencies  []Dependency `json:"dependencies"`
	Files         []File       `json:"files"`
}

func (v Version) Supports(gameVersion string) bool {
	if gameVersion == "" {
		return true
	}
	for _, g := range v.GameVersions {
		if g == gameVersion {
			return true
		}
	}
	return false
}

func (v Version) GameVersionRange() string {
	switch len(v.GameVersions) {
	case 0:
		return "any"
	case 1:
		return v.GameVersions[0]
	}
	return v.GameVersions[0] + "–" + v.GameVersions[len(v.GameVersions)-1]
}

func (v Version) Channel() string {
	switch v.VersionType {
	case "beta":
		return "β"
	case "alpha":
		return "α"
	default:
		return ""
	}
}

var SortIndexes = []string{"relevance", "downloads", "follows", "newest", "updated"}

func SortLabel(index string) string {
	switch index {
	case "downloads":
		return "most downloaded"
	case "follows":
		return "most followed"
	case "newest":
		return "newest"
	case "updated":
		return "recently updated"
	default:
		return "relevance"
	}
}

type Dependency struct {
	VersionID      string `json:"version_id"`
	ProjectID      string `json:"project_id"`
	FileName       string `json:"file_name"`
	DependencyType string `json:"dependency_type"`
}

func (d Dependency) Required() bool { return d.DependencyType == "required" }

type File struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Primary  bool   `json:"primary"`
	Hashes   struct {
		SHA1   string `json:"sha1"`
		SHA512 string `json:"sha512"`
	} `json:"hashes"`
}

// PrimaryFile is the jar to install. Some versions ship sources or javadoc
// alongside it, and only one file is ever flagged primary.
func (v Version) PrimaryFile() (File, bool) {
	for _, f := range v.Files {
		if f.Primary {
			return f, true
		}
	}
	if len(v.Files) > 0 {
		return v.Files[0], true
	}
	return File{}, false
}

func (v Version) Label() string {
	if v.VersionNumber != "" {
		return v.VersionNumber
	}
	return v.Name
}

func (c *Client) throttle(ctx context.Context) error {
	c.mu.Lock()
	wait := minInterval - time.Since(c.last)
	c.last = time.Now().Add(max(0, wait))
	c.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("modrinth: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("modrinth: %s not found", strings.SplitN(strings.TrimPrefix(path, "/"), "?", 2)[0])
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("modrinth: rate limited, try again in a moment")
	case resp.StatusCode >= 300:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("modrinth: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type SearchQuery struct {
	Text         string
	ProjectTypes []string
	Loaders      []string
	GameVersions []string
	Categories   []string
	Index        string
	Offset       int
	Limit        int
}

// facets are AND-ed between groups and OR-ed inside one, which is how a search
// is narrowed to "a plugin, for paper or spigot, that supports 1.21.4".
func (q SearchQuery) facets() string {
	var groups [][]string
	add := func(field string, values []string) {
		if len(values) == 0 {
			return
		}
		g := make([]string, 0, len(values))
		for _, v := range values {
			g = append(g, field+":"+v)
		}
		groups = append(groups, g)
	}
	add("project_type", q.ProjectTypes)
	add("categories", q.Loaders)
	add("versions", q.GameVersions)
	add("categories", q.Categories)

	if len(groups) == 0 {
		return ""
	}
	raw, _ := json.Marshal(groups)
	return string(raw)
}

func (c *Client) Search(ctx context.Context, q SearchQuery) (SearchPage, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Index == "" {
		q.Index = "relevance"
	}

	params := url.Values{}
	if q.Text != "" {
		params.Set("query", q.Text)
	}
	params.Set("index", q.Index)
	params.Set("offset", strconv.Itoa(q.Offset))
	params.Set("limit", strconv.Itoa(q.Limit))
	if f := q.facets(); f != "" {
		params.Set("facets", f)
	}

	var page SearchPage
	err := c.get(ctx, "/search?"+params.Encode(), &page)
	return page, err
}

func (c *Client) Project(ctx context.Context, idOrSlug string) (Project, error) {
	var p Project
	err := c.get(ctx, "/project/"+url.PathEscape(idOrSlug), &p)
	return p, err
}

func (c *Client) Projects(ctx context.Context, ids []string) ([]Project, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	raw, _ := json.Marshal(ids)
	params := url.Values{"ids": {string(raw)}}
	var out []Project
	err := c.get(ctx, "/projects?"+params.Encode(), &out)
	return out, err
}

func (c *Client) Versions(ctx context.Context, idOrSlug string, loaders, gameVersions []string) ([]Version, error) {
	params := url.Values{}
	if len(loaders) > 0 {
		raw, _ := json.Marshal(loaders)
		params.Set("loaders", string(raw))
	}
	if len(gameVersions) > 0 {
		raw, _ := json.Marshal(gameVersions)
		params.Set("game_versions", string(raw))
	}
	path := "/project/" + url.PathEscape(idOrSlug) + "/version"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var out []Version
	err := c.get(ctx, path, &out)
	return out, err
}

func (c *Client) Version(ctx context.Context, versionID string) (Version, error) {
	var v Version
	err := c.get(ctx, "/version/"+url.PathEscape(versionID), &v)
	return v, err
}

func (c *Client) LatestVersion(ctx context.Context, idOrSlug string, loaders, gameVersions []string) (Version, error) {
	versions, err := c.Versions(ctx, idOrSlug, loaders, gameVersions)
	if err != nil {
		return Version{}, err
	}
	if len(versions) == 0 {
		return Version{}, fmt.Errorf("modrinth: %s has no build for this server", idOrSlug)
	}
	for _, v := range versions {
		if v.VersionType == "release" {
			return v, nil
		}
	}
	return versions[0], nil
}
