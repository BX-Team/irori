package screens

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/lock"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/modrinth"
	"github.com/bx-team/irori/internal/plugins"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/msgs"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	searchLimit    = 30
	networkTimeout = 30 * time.Second
	installTimeout = 10 * time.Minute
)

type pluginPane int

const (
	paneInstalled pluginPane = iota
	paneBrowse
)

type browseMode int

const (
	modeResults browseMode = iota
	modeVersions
)

type scannedMsg struct{ items []plugins.Item }

type searchDoneMsg struct {
	page modrinth.SearchPage
	err  error
}

type versionsDoneMsg struct {
	project modrinth.SearchResult
	list    []modrinth.Version
	err     error
}

type installDoneMsg struct {
	name string
	err  error
}

type removePluginMsg struct{ key string }

type Plugins struct {
	Base
	cfg *config.Config
	h   host.Backend

	installed *components.List
	results   *components.List
	versions  *components.List
	search    textinput.Model

	pane      pluginPane
	mode      browseMode
	searching bool

	// index, offset and anyVersion are the state of the browse query. They are
	// held here rather than rebuilt per keystroke so paging forward and
	// re-sorting do not silently reset each other.
	index      string
	offset     int
	anyVersion bool
	query      string

	project modrinth.SearchResult
	busy    string
	note    string
	total   int
}

func NewPlugins(s *components.Styles, cfg *config.Config) *Plugins {
	in := textinput.New()
	in.Prompt = "🔎 "
	in.Placeholder = "search Modrinth"
	in.PromptStyle = s.Accent
	in.PlaceholderStyle = s.Dim
	in.TextStyle = s.Text

	p := &Plugins{
		cfg:       cfg,
		h:         host.NewLocal(cfg.Dir()),
		installed: components.NewList(s),
		results:   components.NewList(s),
		versions:  components.NewList(s),
		search:    in,
		index:     modrinth.SortIndexes[0],
	}
	p.S = s
	p.installed.ZonePfx = "inst:"
	p.installed.Empty = "nothing installed yet"
	p.results.ShowDesc = true
	p.results.ZonePfx = "hit:"
	p.results.Empty = "no results"
	p.versions.ZonePfx = "ver:"
	p.versions.Empty = "no build for this server"
	return p
}

const sealedNote = "sealed mode: plugins are managed by Nix, not by irori"

func (p *Plugins) noun() string { return p.cfg.Server.Type.AddonNoun() }

func (p *Plugins) addonDir() string { return p.cfg.Server.Type.AddonDir() }

func (p *Plugins) Init() tea.Cmd {
	if config.Sealed() {
		return p.rescan()
	}
	return tea.Batch(p.rescan(), p.runSearch())
}

func (p *Plugins) CapturesInput() bool { return p.searching }

func (p *Plugins) rescan() tea.Cmd {
	cfg, h := p.cfg, p.h
	dir := p.addonDir()
	return func() tea.Msg {
		lf, err := lock.Load(cfg.LockPath())
		if err != nil {
			lf = lock.New(cfg.LockPath())
		}
		found, _ := plugins.Scan(h, dir)
		return scannedMsg{items: plugins.Reconcile(cfg, lf, found)}
	}
}

func (p *Plugins) setItems(items []plugins.Item) {
	list := make([]components.ListItem, 0, len(items))
	for _, it := range items {
		badge := it.Version
		if it.Installed != nil && it.Version == "" && it.Installed.HasMeta {
			badge = it.Installed.Meta.Version
		}
		list = append(list, components.ListItem{
			ID:     it.Key,
			Title:  it.Name,
			Badge:  strings.TrimSpace(stateLabel(it.State) + " " + badge),
			Marker: stateMarker(it.State),
			Data:   it,
		})
	}
	p.installed.SetItems(list)
}

func stateLabel(s plugins.State) string {
	switch s {
	case plugins.StateMissing:
		return "missing"
	case plugins.StateOutdated:
		return "outdated"
	case plugins.StateOrphan:
		return "orphan"
	case plugins.StateUntracked:
		return "not declared"
	default:
		return ""
	}
}

func stateMarker(s plugins.State) string {
	switch s {
	case plugins.StateMissing, plugins.StateOrphan:
		return "✗"
	case plugins.StateOutdated:
		return "↑"
	case plugins.StateUntracked:
		return "?"
	default:
		return "✓"
	}
}

func (p *Plugins) selectedItem() (plugins.Item, bool) {
	li, ok := p.installed.Selected()
	if !ok {
		return plugins.Item{}, false
	}
	item, ok := li.Data.(plugins.Item)
	return item, ok
}

func (p *Plugins) selectedHit() (modrinth.SearchResult, bool) {
	li, ok := p.results.Selected()
	if !ok {
		return modrinth.SearchResult{}, false
	}
	hit, ok := li.Data.(modrinth.SearchResult)
	return hit, ok
}

func (p *Plugins) selectedVersion() (modrinth.Version, bool) {
	li, ok := p.versions.Selected()
	if !ok {
		return modrinth.Version{}, false
	}
	v, ok := li.Data.(modrinth.Version)
	return v, ok
}

func (p *Plugins) newSearch() tea.Cmd {
	p.offset = 0
	p.results.Reset()
	return p.runSearch()
}

func (p *Plugins) runSearch() tea.Cmd {
	p.query = strings.TrimSpace(p.search.Value())
	q := modrinth.SearchQuery{
		Text:         p.query,
		ProjectTypes: p.cfg.Server.Type.ModrinthProjectTypes(),
		Loaders:      p.cfg.Server.Type.ModrinthLoaders(),
		Index:        p.index,
		Offset:       p.offset,
		Limit:        searchLimit,
	}
	if v := p.gameVersion(); v != "" {
		q.GameVersions = []string{v}
	}
	p.busy = "searching Modrinth…"
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
		defer cancel()
		page, err := modrinth.New().Search(ctx, q)
		return searchDoneMsg{page: page, err: err}
	}
}

func (p *Plugins) gameVersion() string {
	if p.anyVersion {
		return ""
	}
	return p.cfg.Server.MCVersion
}

func (p *Plugins) loadVersions(hit modrinth.SearchResult) tea.Cmd {
	loaders := p.cfg.Server.Type.ModrinthLoaders()
	var game []string
	if v := p.gameVersion(); v != "" {
		game = []string{v}
	}
	p.busy = "loading versions…"
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
		defer cancel()
		list, err := modrinth.New().Versions(ctx, hit.ProjectID, loaders, game)
		return versionsDoneMsg{project: hit, list: list, err: err}
	}
}

func (p *Plugins) install(hit modrinth.SearchResult, v modrinth.Version) tea.Cmd {
	if config.Sealed() {
		return toast(sealedNote, models.LevelWarn)
	}
	cfg, h := p.cfg, p.h
	dir := p.addonDir()
	p.busy = "installing " + hit.Title + "…"

	return func() tea.Msg {
		file, ok := v.PrimaryFile()
		if !ok {
			return installDoneMsg{name: hit.Title,
				err: fmt.Errorf("%s %s has no downloadable file", hit.Title, v.Label())}
		}

		ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
		defer cancel()
		client := modrinth.New()
		if err := client.Download(ctx, file, h.Abs(path.Join(dir, file.Filename)), nil); err != nil {
			return installDoneMsg{name: hit.Title, err: err}
		}

		ref := config.PluginRef{
			Source:  config.SourceModrinth,
			ID:      hit.Slug,
			Version: v.ID,
			Name:    hit.Title,
		}
		lf, err := lock.Load(cfg.LockPath())
		if err != nil {
			lf = lock.New(cfg.LockPath())
		}
		// Replacing an existing install leaves the old jar behind otherwise.
		if old, ok := lf.Find(ref.Key()); ok && old.File != file.Filename {
			_ = h.Remove(path.Join(dir, old.File))
		}

		cfg.UpsertPlugin(ref)
		if err := cfg.Save(); err != nil {
			return installDoneMsg{name: hit.Title, err: err}
		}
		lf.Upsert(lock.Addon{
			Key: ref.Key(), Source: string(config.SourceModrinth), ID: hit.Slug,
			ProjectID: v.ProjectID, VersionID: v.ID, Name: hit.Title,
			File: file.Filename, URL: file.URL,
			SHA512: file.Hashes.SHA512, SHA1: file.Hashes.SHA1, Size: file.Size,
		})
		if err := lf.Save(); err != nil {
			return installDoneMsg{name: hit.Title, err: err}
		}
		return installDoneMsg{name: hit.Title + " " + v.Label()}
	}
}

func (p *Plugins) remove(item plugins.Item) tea.Cmd {
	if config.Sealed() {
		return toast(sealedNote, models.LevelWarn)
	}
	cfg, h := p.cfg, p.h
	dir := p.addonDir()
	name := item.Name

	return func() tea.Msg {
		lf, err := lock.Load(cfg.LockPath())
		if err != nil {
			lf = lock.New(cfg.LockPath())
		}
		file := ""
		switch {
		case item.Lock != nil:
			file = item.Lock.File
		case item.Installed != nil:
			file = item.Installed.File
		}
		if file != "" {
			if err := h.Remove(path.Join(dir, file)); err != nil {
				return installDoneMsg{name: name, err: err}
			}
		}

		changed := cfg.RemovePlugin(item.Key)
		if changed {
			if err := cfg.Save(); err != nil {
				return installDoneMsg{name: name, err: err}
			}
		}
		if lf.Remove(item.Key) {
			if err := lf.Save(); err != nil {
				return installDoneMsg{name: name, err: err}
			}
		}
		return installDoneMsg{name: "removed " + name}
	}
}

func (p *Plugins) declare(item plugins.Item) tea.Cmd {
	if item.Installed == nil {
		return nil
	}
	cfg := p.cfg
	name := item.Name
	file := item.Installed.File

	return func() tea.Msg {
		lf, err := lock.Load(cfg.LockPath())
		if err != nil {
			lf = lock.New(cfg.LockPath())
		}
		ref := config.PluginRef{Source: config.SourceLocal, File: file, Name: name}
		cfg.UpsertPlugin(ref)
		if err := cfg.Save(); err != nil {
			return installDoneMsg{name: name, err: err}
		}
		lf.Upsert(lock.Addon{Key: ref.Key(), Source: string(config.SourceLocal), Name: name, File: file})
		if err := lf.Save(); err != nil {
			return installDoneMsg{name: name, err: err}
		}
		return installDoneMsg{name: name + " is now declared in " + config.FileName}
	}
}

func (p *Plugins) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case scannedMsg:
		p.setItems(m.items)
		return p, nil

	case msgs.ConfigChangedMsg:
		return p, p.rescan()

	case searchDoneMsg:
		p.busy = ""
		if m.err != nil {
			p.note = m.err.Error()
			return p, errToast("Modrinth", m.err)
		}
		p.note = ""
		p.total = m.page.TotalHits
		p.setResults(m.page.Hits)
		p.mode = modeResults
		p.pane = paneBrowse
		return p, nil

	case versionsDoneMsg:
		p.busy = ""
		if m.err != nil {
			return p, errToast("Modrinth", m.err)
		}
		p.project = m.project
		p.setVersions(m.list)
		p.mode = modeVersions
		p.pane = paneBrowse
		return p, nil

	case installDoneMsg:
		p.busy = ""
		if m.err != nil {
			return p, errToast("Plugins", m.err)
		}
		return p, tea.Batch(p.rescan(), toast(m.name, models.LevelIrori))

	case removePluginMsg:
		for _, li := range p.installed.Items() {
			if item, ok := li.Data.(plugins.Item); ok && item.Key == m.key {
				return p, p.remove(item)
			}
		}
		return p, nil

	case tea.KeyMsg:
		return p.handleKey(m)

	case tea.MouseMsg:
		if p.pane == paneInstalled {
			return p, p.installed.Update(msg)
		}
		if p.mode == modeVersions {
			return p, p.versions.Update(msg)
		}
		return p, p.results.Update(msg)
	}
	return p, nil
}

func (p *Plugins) setResults(hits []modrinth.SearchResult) {
	list := make([]components.ListItem, 0, len(hits))
	for _, h := range hits {
		list = append(list, components.ListItem{
			ID:    h.ProjectID,
			Title: h.Title,
			Desc:  h.Description,
			Badge: fmt.Sprintf("%s ↓", components.HumanCount(h.Downloads)),
			Data:  h,
		})
	}
	p.results.SetItems(list)
}

func (p *Plugins) setVersions(versions []modrinth.Version) {
	want := p.cfg.Server.MCVersion
	list := make([]components.ListItem, 0, len(versions))
	for _, v := range versions {
		marker := "✓"
		if !v.Supports(want) {
			marker = "!"
		}
		badge := v.VersionType
		if c := v.Channel(); c != "" {
			badge = c + " " + badge
		}
		list = append(list, components.ListItem{
			ID:     v.ID,
			Title:  v.Label(),
			Badge:  badge + " · " + components.HumanCount(v.Downloads) + " ↓",
			Marker: marker,
			Data:   v,
		})
	}
	p.versions.SetItems(list)
	p.versions.Reset()
}

func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func (p *Plugins) handleKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	if p.searching {
		switch k.String() {
		case "esc":
			p.searching = false
			p.search.Blur()
			return p, nil
		case "enter":
			p.searching = false
			p.search.Blur()
			return p, p.newSearch()
		}
		var cmd tea.Cmd
		p.search, cmd = p.search.Update(k)
		return p, cmd
	}

	switch k.String() {
	case "tab":
		if p.pane == paneInstalled {
			p.pane = paneBrowse
		} else {
			p.pane = paneInstalled
		}
		return p, nil
	case "/":
		if config.Sealed() {
			return p, toast(sealedNote, models.LevelWarn)
		}
		p.pane = paneBrowse
		p.searching = true
		p.search.Focus()
		return p, textinput.Blink
	case "R":
		return p, p.rescan()
	case "esc":
		if p.mode == modeVersions {
			p.mode = modeResults
			return p, nil
		}
	}

	if p.pane == paneInstalled {
		return p.handleInstalledKey(k)
	}
	return p.handleBrowseKey(k)
}

func (p *Plugins) handleInstalledKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	item, ok := p.selectedItem()
	if ok {
		switch k.String() {
		case "d":
			return p, p.confirmRemove(item)
		case "c":
			if item.State == plugins.StateUntracked {
				return p, p.declare(item)
			}
			return p, toast(item.Name+" is already declared", models.LevelWarn)
		case "enter":
			if item.Ref != nil && item.Ref.Source == config.SourceModrinth {
				return p, p.loadVersions(modrinth.SearchResult{
					ProjectID: item.Ref.ID, Slug: item.Ref.ID, Title: item.Name,
				})
			}
		}
	}
	return p, p.installed.Update(k)
}

func (p *Plugins) handleBrowseKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	if p.mode == modeVersions {
		switch k.String() {
		case "enter":
			if v, ok := p.selectedVersion(); ok {
				return p, p.install(p.project, v)
			}
			return p, nil
		case "v":
			return p, p.toggleVersionFilter()
		}
		return p, p.versions.Update(k)
	}

	switch k.String() {
	case "enter":
		if hit, ok := p.selectedHit(); ok {
			return p, p.loadVersions(hit)
		}
		return p, nil
	case "s":
		return p, p.cycleSort()
	case "v":
		return p, p.toggleVersionFilter()
	case "n", "]":
		return p, p.page(1)
	case "b", "[":
		return p, p.page(-1)
	}
	return p, p.results.Update(k)
}

func (p *Plugins) cycleSort() tea.Cmd {
	if config.Sealed() {
		return toast(sealedNote, models.LevelWarn)
	}
	at := 0
	for i, idx := range modrinth.SortIndexes {
		if idx == p.index {
			at = i
			break
		}
	}
	p.index = modrinth.SortIndexes[(at+1)%len(modrinth.SortIndexes)]
	return p.newSearch()
}

func (p *Plugins) toggleVersionFilter() tea.Cmd {
	if config.Sealed() {
		return toast(sealedNote, models.LevelWarn)
	}
	if p.cfg.Server.MCVersion == "" {
		return nil
	}
	p.anyVersion = !p.anyVersion

	if p.mode == modeVersions {
		return p.loadVersions(p.project)
	}
	return p.newSearch()
}

func (p *Plugins) page(delta int) tea.Cmd {
	if config.Sealed() {
		return toast(sealedNote, models.LevelWarn)
	}
	next := p.offset + delta*searchLimit
	if next < 0 {
		return toast("already on the first page", models.LevelWarn)
	}
	if next >= p.total && p.total > 0 {
		return toast("no more results", models.LevelWarn)
	}
	p.offset = next
	p.results.Reset()
	return p.runSearch()
}

func (p *Plugins) confirmRemove(item plugins.Item) tea.Cmd {
	file := ""
	if item.Installed != nil {
		file = item.Installed.File
	} else if item.Lock != nil {
		file = item.Lock.File
	}
	body := fmt.Sprintf("%s will be deleted from %s/ and dropped from %s.\nIts configuration directory is left in place. Continue?",
		file, p.addonDir(), config.FileName)
	key := item.Key
	return func() tea.Msg {
		return msgs.ConfirmMsg{
			Title:  "Remove " + item.Name,
			Body:   body,
			OnYes:  removePluginMsg{key: key},
			Danger: true,
		}
	}
}

func (p *Plugins) Hints() []components.Hint {
	if p.searching {
		return []components.Hint{
			{Key: "Enter", Desc: "search"},
			{Key: "Esc", Desc: "cancel"},
		}
	}
	if p.pane == paneInstalled {
		return []components.Hint{
			{Key: "Tab", Desc: "browse Modrinth"},
			{Key: "Enter", Desc: "change version"},
			{Key: "d", Desc: "remove"},
			{Key: "c", Desc: "declare"},
			{Key: "/", Desc: "search"},
			{Key: "R", Desc: "rescan"},
		}
	}
	if p.mode == modeVersions {
		return []components.Hint{
			{Key: "Enter", Desc: "install this version"},
			{Key: "v", Desc: p.versionFilterHint()},
			{Key: "Esc", Desc: "back to results"},
			{Key: "Tab", Desc: "installed"},
		}
	}
	return []components.Hint{
		{Key: "/", Desc: "search"},
		{Key: "Enter", Desc: "pick a version"},
		{Key: "s", Desc: "sort: " + modrinth.SortLabel(p.index)},
		{Key: "v", Desc: p.versionFilterHint()},
		{Key: "n / b", Desc: "next / previous page"},
		{Key: "Tab", Desc: "installed"},
	}
}

func (p *Plugins) versionFilterHint() string {
	if p.anyVersion {
		return "only " + p.cfg.Server.MCVersion
	}
	return "any Minecraft version"
}

func (p *Plugins) View() string {
	leftWidth := p.Width * 42 / 100
	if p.Width < 80 {
		leftWidth = p.Width
	}

	left := p.installedPanel(leftWidth)
	if leftWidth == p.Width {
		if p.pane == paneBrowse {
			return p.browsePanel(p.Width)
		}
		return left
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, p.browsePanel(p.Width-leftWidth))
}

func (p *Plugins) installedPanel(width int) string {
	title := strings.ToUpper(p.noun()[:1]) + p.noun()[1:] + "s in " + p.addonDir() + "/"
	panel := components.Panel{
		Title:   title,
		Badge:   p.installed.ScrollBadge(),
		Focused: p.pane == paneInstalled,
		Width:   width,
		Height:  p.Height,
	}
	p.installed.SetSize(panel.ContentWidth(), panel.ContentHeight())
	return panel.Render(components.PadLines(p.installed.View(), panel.ContentHeight()), p.S)
}

func (p *Plugins) browsePanel(width int) string {
	if p.mode == modeVersions {
		return p.versionsView(width)
	}

	detailHeight := 0
	if p.Height >= 22 {
		detailHeight = projectDetailRows
	}

	badge := p.results.ScrollBadge()
	if p.total > 0 {
		badge = fmt.Sprintf("%s of %s", badge, components.HumanCount(int64(p.total)))
	}
	panel := components.Panel{
		Title:   p.browseTitle(),
		Badge:   badge,
		Focused: p.pane == paneBrowse,
		Width:   width,
		Height:  p.Height - detailHeight,
	}

	w, h := panel.ContentWidth(), panel.ContentHeight()
	p.search.Width = w - 4
	head := p.search.View()
	if p.busy != "" {
		head = p.S.Accent.Render(p.busy)
	} else if p.note != "" {
		head = p.S.Error.Render(components.Truncate(p.note, w))
	}

	p.results.SetSize(w, h-2)
	body := head + "\n" + p.S.Muted.Render(p.facetLine(w)) + "\n" +
		components.PadLines(p.results.View(), h-2)
	list := panel.Render(body, p.S)

	if detailHeight == 0 {
		return list
	}
	return lipgloss.JoinVertical(lipgloss.Left, list, p.projectDetail(width, detailHeight))
}

func (p *Plugins) browseTitle() string {
	if p.query == "" {
		return "Modrinth · popular"
	}
	return "Modrinth · " + p.query
}

func (p *Plugins) facetLine(width int) string {
	parts := []string{p.noun() + "s"}
	if loaders := p.cfg.Server.Type.ModrinthLoaders(); len(loaders) > 0 {
		parts = append(parts, strings.Join(loaders, "/"))
	}
	switch {
	case p.cfg.Server.MCVersion == "":
	case p.anyVersion:
		parts = append(parts, "any version")
	default:
		parts = append(parts, p.cfg.Server.MCVersion)
	}
	parts = append(parts, "by "+modrinth.SortLabel(p.index))
	if p.offset > 0 {
		parts = append(parts, fmt.Sprintf("page %d", p.offset/searchLimit+1))
	}
	return components.Truncate(strings.Join(parts, " · "), width)
}

const projectDetailRows = 6

func (p *Plugins) projectDetail(width, height int) string {
	s := p.S
	panel := components.Panel{Title: "project", Width: width, Height: height}
	hit, ok := p.selectedHit()
	if !ok {
		return panel.Render(components.PadLines("", panel.ContentHeight()), s)
	}

	cw := panel.ContentWidth()
	rows := []string{
		components.SpaceBetween(
			s.Bold.Render(components.Truncate(hit.Title, cw/2)),
			s.Muted.Render(components.Truncate("by "+hit.Author, cw/2)), cw),
	}

	facts := []string{
		components.HumanCount(hit.Downloads) + " downloads",
		components.HumanCount(hit.Follows) + " followers",
	}
	if hit.License != "" {
		facts = append(facts, hit.License)
	}
	if hit.DateModified != "" {
		facts = append(facts, "updated "+shortDate(hit.DateModified))
	}
	rows = append(rows, s.Muted.Render(components.Truncate(strings.Join(facts, " · "), cw)))

	if tags := hit.Tags(); len(tags) > 0 {
		rows = append(rows, s.Subtext.Render(components.Truncate(strings.Join(tags, ", "), cw)))
	}
	if hit.ServerSide == "unsupported" {
		rows = append(rows, s.Warning.Render(components.Truncate(
			"client side only — this will do nothing on a server", cw)))
	} else if loaders := hit.SupportedLoaders(); len(loaders) > 0 {
		rows = append(rows, s.Muted.Render(components.Truncate(strings.Join(loaders, ", "), cw)))
	}

	return panel.Render(components.PadLines(strings.Join(rows, "\n"), panel.ContentHeight()), s)
}

func (p *Plugins) versionsView(width int) string {
	detailHeight := p.Height / 2
	if detailHeight < 8 {
		detailHeight = 0
	}
	if detailHeight > 16 {
		detailHeight = 16
	}

	panel := components.Panel{
		Title:   p.project.Title,
		Badge:   p.versions.ScrollBadge(),
		Focused: p.pane == paneBrowse,
		Width:   width,
		Height:  p.Height - detailHeight,
	}
	w, h := panel.ContentWidth(), panel.ContentHeight()

	scope := p.cfg.Server.Type.Display()
	if v := p.gameVersion(); v != "" {
		scope += " " + v
	} else {
		scope += " · every version"
	}
	head := p.S.Muted.Render(components.Truncate("builds for "+scope, w))
	if p.busy != "" {
		head = p.S.Accent.Render(p.busy)
	}

	p.versions.SetSize(w, h-1)
	list := panel.Render(head+"\n"+components.PadLines(p.versions.View(), h-1), p.S)

	if detailHeight == 0 {
		return list
	}
	return lipgloss.JoinVertical(lipgloss.Left, list, p.versionDetail(width, detailHeight))
}

func (p *Plugins) versionDetail(width, height int) string {
	s := p.S
	panel := components.Panel{Title: "version", Width: width, Height: height}
	v, ok := p.selectedVersion()
	if !ok {
		return panel.Render(components.PadLines("", panel.ContentHeight()), s)
	}
	w := panel.ContentWidth()

	compat := s.Success.Render("runs on " + p.cfg.Server.MCVersion)
	if !v.Supports(p.cfg.Server.MCVersion) {
		compat = s.Warning.Render("not built for " + p.cfg.Server.MCVersion)
	}
	if p.cfg.Server.MCVersion == "" {
		compat = s.Muted.Render("no server version recorded")
	}

	rows := []string{
		components.SpaceBetween(
			s.Bold.Render(components.Truncate(v.Label(), w/2)),
			compat, w),
		components.SpaceBetween(
			s.Label.Render("minecraft"),
			s.Value.Render(components.Truncate(v.GameVersionRange(), w/2)), w),
		components.SpaceBetween(
			s.Label.Render("loaders"),
			s.Value.Render(components.Truncate(strings.Join(v.Loaders, ", "), w/2)), w),
	}

	if f, ok := v.PrimaryFile(); ok {
		rows = append(rows, components.SpaceBetween(
			s.Label.Render("file"),
			s.Value.Render(components.Truncate(f.Filename+" · "+components.HumanBytes(f.Size), w*2/3)), w))
	}
	rows = append(rows, components.SpaceBetween(
		s.Label.Render("released"),
		s.Value.Render(shortDate(v.DatePublished)+" · "+components.HumanCount(v.Downloads)+" ↓"), w))

	if deps := requiredDeps(v); deps > 0 {
		rows = append(rows, components.SpaceBetween(
			s.Label.Render("requires"),
			s.Warning.Render(fmt.Sprintf("%d other project(s)", deps)), w))
	}

	if room := panel.ContentHeight() - len(rows) - 1; room > 0 {
		if text := shortChangelog(v.Changelog); text != "" {
			rows = append(rows, "")
			lines := components.WrapWords(text, w)
			for i, l := range lines {
				if i >= room-1 {
					rows = append(rows, s.Dim.Render("…"))
					break
				}
				rows = append(rows, s.Muted.Render(l))
			}
		}
	}

	return panel.Render(components.PadLines(strings.Join(rows, "\n"), panel.ContentHeight()), s)
}

func requiredDeps(v modrinth.Version) int {
	n := 0
	for _, d := range v.Dependencies {
		if d.Required() {
			n++
		}
	}
	return n
}

func shortChangelog(raw string) string {
	var parts []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#>*-+ \t")
		line = mdLink.ReplaceAllString(line, "$1")
		line = strings.ReplaceAll(line, "`", "")
		line = strings.ReplaceAll(line, "**", "")
		if line == "" || strings.Trim(line, "-=_") == "" {
			continue
		}
		parts = append(parts, line)
		if len(parts) >= 12 {
			break
		}
	}
	return strings.Join(parts, " · ")
}

var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
