package screens

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/files"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/msgs"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

const (
	filesParentCols = 100
	filesPreviewMin = 64
	previewMaxLines = 200
)

type promptKind int

const (
	promptNone promptKind = iota
	promptNewFile
	promptNewDir
	promptRename
)

type previewMsg struct {
	seq int
	pv  files.Preview
}

type filesDeleteMsg struct{ name string }

type filesRestoreMsg struct{ location string }

type shippedConfigsMsg struct {
	configs []mcjars.Config
	err     error
}

type Files struct {
	Base
	cfg *config.Config
	br  *files.Browser

	entries   []host.Entry
	cursor    int
	offset    int
	lastAt    map[string]string
	lastState models.ServerState

	parent       []host.Entry
	parentCursor int

	preview    files.Preview
	previewSeq int

	shipped      []mcjars.Config
	shippedTried bool

	filter    textinput.Model
	filtering bool

	prompt     promptKind
	promptFor  string
	promptText textinput.Model

	err string
}

func NewFiles(s *components.Styles, cfg *config.Config) *Files {
	fl := textinput.New()
	fl.Prompt = "/ "
	fl.Placeholder = "filter"
	fl.PromptStyle = s.Warning
	fl.PlaceholderStyle = s.Dim

	pt := textinput.New()
	pt.Prompt = ""
	pt.TextStyle = s.Text

	f := &Files{
		cfg:        cfg,
		br:         files.NewBrowser(host.NewLocal(cfg.Dir())),
		filter:     fl,
		promptText: pt,
		lastAt:     map[string]string{},
	}
	f.S = s
	f.reload("")
	return f
}

func (f *Files) Init() tea.Cmd { return f.refreshPreview() }

func (f *Files) CapturesInput() bool { return f.filtering || f.prompt != promptNone }

func (f *Files) reload(dir string) {
	if err := f.br.Load(dir); err != nil {
		f.err = err.Error()
		return
	}
	f.err = ""
	f.applyFilter()
	f.loadParent()
	f.restoreCursor()
}

func (f *Files) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(f.filter.Value()))
	all := f.br.Entries()
	if q == "" {
		f.entries = all
	} else {
		out := make([]host.Entry, 0, len(all))
		for _, e := range all {
			if strings.Contains(strings.ToLower(e.Name), q) {
				out = append(out, e)
			}
		}
		f.entries = out
	}
	f.clamp()
}

func (f *Files) loadParent() {
	if f.br.Dir() == "" {
		f.parent, f.parentCursor = nil, 0
		return
	}
	entries, err := f.br.List(files.Parent(f.br.Dir()))
	if err != nil {
		f.parent = nil
		return
	}
	f.parent = entries
	here := files.Base(f.br.Dir())
	for i, e := range entries {
		if e.Name == here {
			f.parentCursor = i
			break
		}
	}
}

func (f *Files) restoreCursor() {
	if name, ok := f.lastAt[f.br.Dir()]; ok {
		for i, e := range f.entries {
			if e.Name == name {
				f.cursor = i
				f.clamp()
				return
			}
		}
	}
	f.cursor = 0
	f.clamp()
}

func (f *Files) rows() int {
	h := f.Height - 2
	if h < 1 {
		return 1
	}
	return h
}

func (f *Files) clamp() {
	if f.cursor >= len(f.entries) {
		f.cursor = len(f.entries) - 1
	}
	if f.cursor < 0 {
		f.cursor = 0
	}
	r := f.rows()
	if f.cursor < f.offset {
		f.offset = f.cursor
	}
	if f.cursor >= f.offset+r {
		f.offset = f.cursor - r + 1
	}
	if f.offset > len(f.entries)-r {
		f.offset = len(f.entries) - r
	}
	if f.offset < 0 {
		f.offset = 0
	}
}

func (f *Files) selected() (host.Entry, bool) {
	if f.cursor < 0 || f.cursor >= len(f.entries) {
		return host.Entry{}, false
	}
	return f.entries[f.cursor], true
}

func (f *Files) remember() {
	if e, ok := f.selected(); ok {
		f.lastAt[f.br.Dir()] = e.Name
	}
}

func (f *Files) move(delta int) tea.Cmd {
	if len(f.entries) == 0 {
		return nil
	}
	before := f.cursor
	f.cursor += delta
	f.clamp()
	if f.cursor == before {
		return nil
	}
	f.remember()
	return f.refreshPreview()
}

// refreshPreview reads off the update loop: a plugin jar has to be unzipped to
// name it, and that must never stall keyboard input.
func (f *Files) refreshPreview() tea.Cmd {
	e, ok := f.selected()
	if !ok {
		f.preview = files.Preview{}
		return nil
	}
	f.previewSeq++
	seq := f.previewSeq
	p := files.Join(f.br.Dir(), e.Name)
	h := f.br.Backend()
	return func() tea.Msg {
		return previewMsg{seq: seq, pv: files.Build(h, p, e, previewMaxLines)}
	}
}

func (f *Files) enter() tea.Cmd {
	e, ok := f.selected()
	if !ok {
		return nil
	}
	if e.IsDir {
		f.remember()
		f.reload(files.Join(f.br.Dir(), e.Name))
		return f.refreshPreview()
	}
	return f.openInEditor()
}

func (f *Files) openInEditor() tea.Cmd {
	e, ok := f.selected()
	if !ok || e.IsDir {
		return nil
	}
	abs := filepath.Join(f.cfg.Dir(), filepath.FromSlash(files.Join(f.br.Dir(), e.Name)))
	return func() tea.Msg { return msgs.OpenEditorMsg{Path: abs} }
}

func (f *Files) up() tea.Cmd {
	if f.br.Dir() == "" {
		return nil
	}
	here := files.Base(f.br.Dir())
	parent := files.Parent(f.br.Dir())
	f.lastAt[parent] = here
	f.reload(parent)
	return f.refreshPreview()
}

func (f *Files) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case previewMsg:
		if m.seq == f.previewSeq {
			f.preview = m.pv
		}
		return f, nil

	case msgs.ConfigChangedMsg:
		f.reload(f.br.Dir())
		return f, f.refreshPreview()

	case msgs.StatusMsg:
		if m.Status.State == f.lastState {
			return f, nil
		}
		f.lastState = m.Status.State
		f.reload(f.br.Dir())
		return f, f.refreshPreview()

	case shippedConfigsMsg:
		f.shippedTried = true
		if m.err != nil {
			return f, errToast("Defaults", m.err)
		}
		f.shipped = m.configs
		return f, f.restorePrompt()

	case filesRestoreMsg:
		return f, f.restore(m.location)

	case filesDeleteMsg:
		if err := f.br.Delete(m.name); err != nil {
			return f, errToast("Delete", err)
		}
		delete(f.lastAt, f.br.Dir())
		f.reload(f.br.Dir())
		return f, tea.Batch(f.refreshPreview(), toast("deleted "+m.name, models.LevelIrori))

	case tea.KeyMsg:
		return f.handleKey(m)

	case tea.MouseMsg:
		return f, f.handleMouse(m)
	}
	return f, nil
}

func (f *Files) handleMouse(m tea.MouseMsg) tea.Cmd {
	switch {
	case m.Button == tea.MouseButtonWheelUp:
		return f.move(-1)
	case m.Button == tea.MouseButtonWheelDown:
		return f.move(1)
	case m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft:
		for i := f.offset; i < len(f.entries) && i < f.offset+f.rows(); i++ {
			if zone.Get("file:" + f.entries[i].Name).InBounds(m) {
				if f.cursor == i {
					return f.enter()
				}
				f.cursor = i
				f.clamp()
				f.remember()
				return f.refreshPreview()
			}
		}
	}
	return nil
}

func (f *Files) handleKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	if f.prompt != promptNone {
		return f, f.handlePrompt(k)
	}
	if f.filtering {
		switch k.String() {
		case "esc":
			f.filtering = false
			f.filter.SetValue("")
			f.filter.Blur()
			f.applyFilter()
			return f, f.refreshPreview()
		case "enter":
			f.filtering = false
			f.filter.Blur()
			return f, nil
		}
		var cmd tea.Cmd
		f.filter, cmd = f.filter.Update(k)
		f.applyFilter()
		return f, tea.Batch(cmd, f.refreshPreview())
	}

	switch k.String() {
	case "j", "down":
		return f, f.move(1)
	case "k", "up":
		return f, f.move(-1)
	case "ctrl+d", "pgdown":
		return f, f.move(f.rows() / 2)
	case "ctrl+u", "pgup":
		return f, f.move(-f.rows() / 2)
	case "g", "home":
		return f, f.move(-len(f.entries))
	case "G", "end":
		return f, f.move(len(f.entries))
	case "h", "left", "backspace":
		return f, f.up()
	case "l", "right", "enter":
		return f, f.enter()
	case "e":
		return f, f.openInEditor()
	case "R":
		f.reload(f.br.Dir())
		return f, tea.Batch(f.refreshPreview(), toast("reloaded", models.LevelIrori))
	case "D":
		return f, f.restoreDefaults()
	case ".":
		f.br.ShowHidden = !f.br.ShowHidden
		f.reload(f.br.Dir())
		return f, f.refreshPreview()
	case "/":
		f.filtering = true
		f.filter.Focus()
		return f, textinput.Blink
	case "y":
		return f, f.clip(files.ClipCopy)
	case "x":
		return f, f.clip(files.ClipCut)
	case "p":
		return f, f.paste()
	case "d":
		return f, f.confirmDelete()
	case "a":
		return f, f.openPrompt(promptNewFile, "")
	case "A":
		return f, f.openPrompt(promptNewDir, "")
	case "r":
		if e, ok := f.selected(); ok {
			return f, f.openPrompt(promptRename, e.Name)
		}
	}
	return f, nil
}

func (f *Files) restoreDefaults() tea.Cmd {
	e, ok := f.selected()
	if !ok || e.IsDir {
		return nil
	}
	if config.Sealed() {
		return toast("sealed mode: configuration is managed by Nix", models.LevelWarn)
	}
	if f.cfg.Server.BuildID == "" {
		return toast("no build id in "+config.FileName+", run irori apply first", models.LevelWarn)
	}
	if f.shippedTried {
		return f.restorePrompt()
	}

	id := f.cfg.Server.BuildID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		configs, err := mcjars.New().Configs(ctx, id)
		return shippedConfigsMsg{configs: configs, err: err}
	}
}

func (f *Files) restorePrompt() tea.Cmd {
	e, ok := f.selected()
	if !ok {
		return nil
	}
	location := files.Join(f.br.Dir(), e.Name)
	if _, ok := mcjars.FindConfig(f.shipped, location); !ok {
		return toast(location+" is not one of this core's configuration files", models.LevelWarn)
	}
	return func() tea.Msg {
		return msgs.ConfirmMsg{
			Title: "Restore " + e.Name,
			Body: "The file will be replaced with the version " + f.cfg.Server.Type.Display() +
				" ships.\nA copy is kept alongside it as " + e.Name + ".irori.bak. Continue?",
			OnYes:  filesRestoreMsg{location: location},
			Danger: true,
		}
	}
}

func (f *Files) restore(location string) tea.Cmd {
	c, ok := mcjars.FindConfig(f.shipped, location)
	if !ok {
		return nil
	}
	h := f.br.Backend()
	if _, err := h.Stat(location); err == nil {
		if err := h.Copy(location, location+".irori.bak"); err != nil {
			return errToast("Restore", err)
		}
	}
	if err := h.WriteFile(location, []byte(c.Value), 0o644); err != nil {
		return errToast("Restore", err)
	}
	f.reload(f.br.Dir())
	return tea.Batch(f.refreshPreview(),
		toast("restored "+location+", previous kept as .irori.bak", models.LevelIrori))
}

func (f *Files) clip(mode files.ClipMode) tea.Cmd {
	e, ok := f.selected()
	if !ok {
		return nil
	}
	f.br.SetClipboard(mode, files.Join(f.br.Dir(), e.Name))
	verb := "copied"
	if mode == files.ClipCut {
		verb = "cut"
	}
	return toast(verb+" "+e.Name, models.LevelIrori)
}

func (f *Files) paste() tea.Cmd {
	if f.br.Clip.Empty() {
		return toast("clipboard is empty", models.LevelWarn)
	}
	n, err := f.br.Paste()
	f.reload(f.br.Dir())
	if err != nil {
		return tea.Batch(f.refreshPreview(), errToast("Paste", err))
	}
	return tea.Batch(f.refreshPreview(),
		toast(fmt.Sprintf("pasted %d item(s)", n), models.LevelIrori))
}

func (f *Files) confirmDelete() tea.Cmd {
	e, ok := f.selected()
	if !ok {
		return nil
	}
	what := "File"
	if e.IsDir {
		what = "Directory"
	}
	body := fmt.Sprintf("%s will be removed from %s permanently.\nThere is no undo. Continue?",
		e.Name, orRoot(f.br.Dir()))
	if e.IsDir {
		body = fmt.Sprintf("%s and everything inside it will be removed permanently.\nThere is no undo. Continue?", e.Name)
	}
	name := e.Name
	return func() tea.Msg {
		return msgs.ConfirmMsg{
			Title:  "Delete " + strings.ToLower(what),
			Body:   body,
			OnYes:  filesDeleteMsg{name: name},
			Danger: true,
		}
	}
}

func orRoot(dir string) string {
	if dir == "" {
		return "the server root"
	}
	return dir
}

func (f *Files) openPrompt(kind promptKind, initial string) tea.Cmd {
	f.prompt = kind
	f.promptFor = initial
	f.promptText.SetValue(initial)
	f.promptText.CursorEnd()
	f.promptText.Focus()
	return textinput.Blink
}

func (f *Files) closePrompt() {
	f.prompt = promptNone
	f.promptText.Blur()
	f.promptText.SetValue("")
}

func (f *Files) handlePrompt(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "esc":
		f.closePrompt()
		return nil
	case "enter":
		name := strings.TrimSpace(f.promptText.Value())
		kind := f.prompt
		old := f.promptFor
		f.closePrompt()

		var err error
		switch kind {
		case promptNewFile:
			err = f.br.Touch(name)
		case promptNewDir:
			err = f.br.Mkdir(name)
		case promptRename:
			if name == old {
				return nil
			}
			err = f.br.Rename(old, name)
		}
		if err != nil {
			return errToast("Files", err)
		}
		f.lastAt[f.br.Dir()] = name
		f.reload(f.br.Dir())
		return tea.Batch(f.refreshPreview(), toast(promptDone(kind, name), models.LevelIrori))
	}

	var cmd tea.Cmd
	f.promptText, cmd = f.promptText.Update(k)
	return cmd
}

func promptDone(kind promptKind, name string) string {
	switch kind {
	case promptNewDir:
		return "created directory " + name
	case promptRename:
		return "renamed to " + name
	default:
		return "created " + name
	}
}

func (f *Files) Hints() []components.Hint {
	if f.filtering {
		return []components.Hint{
			{Key: "type", Desc: "filter"},
			{Key: "Enter", Desc: "keep"},
			{Key: "Esc", Desc: "clear"},
		}
	}
	if f.prompt != promptNone {
		return []components.Hint{
			{Key: "Enter", Desc: "confirm"},
			{Key: "Esc", Desc: "cancel"},
		}
	}
	hints := []components.Hint{
		{Key: "hjkl", Desc: "navigate"},
		{Key: "e", Desc: "edit"},
		{Key: "y x p", Desc: "copy cut paste"},
		{Key: "d", Desc: "delete"},
		{Key: "a A r", Desc: "file dir rename"},
		{Key: "D", Desc: "defaults"},
		{Key: "R", Desc: "reload"},
		{Key: "/", Desc: "filter"},
	}
	if !f.br.Clip.Empty() {
		hints = append([]components.Hint{
			{Key: "p", Desc: f.br.Clip.Verb() + " " + files.Base(f.br.Clip.Paths[0])},
		}, hints...)
	}
	return hints
}

func (f *Files) View() string {
	parentW, listW, previewW := f.columns()

	cols := make([]string, 0, 3)
	if parentW > 0 {
		cols = append(cols, f.parentPanel(parentW))
	}
	cols = append(cols, f.listPanel(listW))
	if previewW > 0 {
		cols = append(cols, f.previewPanel(previewW))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (f *Files) columns() (parent, list, preview int) {
	switch {
	case f.Width >= filesParentCols:
		parent = f.Width * 2 / 10
		preview = f.Width * 4 / 10
		return parent, f.Width - parent - preview, preview
	case f.Width >= filesPreviewMin:
		preview = f.Width * 45 / 100
		return 0, f.Width - preview, preview
	default:
		return 0, f.Width, 0
	}
}

func (f *Files) parentPanel(width int) string {
	title := files.Base(files.Parent(f.br.Dir()))
	if f.br.Dir() == "" {
		title = filepath.Base(f.cfg.Dir())
		panel := components.Panel{Title: title, Width: width, Height: f.Height}
		return panel.Render(f.S.Muted.Render("at the server root"), f.S)
	}
	panel := components.Panel{Title: title, Width: width, Height: f.Height}

	w, h := panel.ContentWidth(), panel.ContentHeight()
	offset := 0
	if f.parentCursor >= h {
		offset = f.parentCursor - h + 1
	}
	rows := make([]string, 0, h)
	for i := offset; i < len(f.parent) && len(rows) < h; i++ {
		e := f.parent[i]
		row := components.Truncate(f.entryStyle(e).Render(icon(e)+e.Name), w)
		if i == f.parentCursor {
			row = lipgloss.NewStyle().Background(f.S.T.Selection).
				Render(components.SpaceBetween(row, "", w))
		}
		rows = append(rows, row)
	}
	return panel.Render(components.PadLines(strings.Join(rows, "\n"), h), f.S)
}

func (f *Files) listPanel(width int) string {
	title := "/" + f.br.Dir()
	badge := ""
	if n := len(f.entries); n > 0 {
		badge = fmt.Sprintf("%d/%d", f.cursor+1, n)
	}
	panel := components.Panel{Title: title, Badge: badge, Focused: true, Width: width, Height: f.Height}

	w, h := panel.ContentWidth(), panel.ContentHeight()
	if f.filtering || f.prompt != promptNone {
		h--
	}

	var rows []string
	switch {
	case f.err != "":
		rows = []string{f.S.Error.Render(components.Truncate(f.err, w))}
	case len(f.entries) == 0:
		rows = []string{f.S.Muted.Render("empty directory")}
	default:
		end := min(f.offset+h, len(f.entries))
		for i := f.offset; i < end; i++ {
			rows = append(rows, f.entryRow(f.entries[i], i == f.cursor, w))
		}
	}

	body := components.PadLines(strings.Join(rows, "\n"), h)
	if f.filtering {
		f.filter.Width = w - 3
		body += "\n" + f.filter.View()
	} else if f.prompt != promptNone {
		f.promptText.Width = w - lipgloss.Width(promptLabel(f.prompt)) - 2
		body += "\n" + f.S.Accent.Render(promptLabel(f.prompt)) + f.promptText.View()
	}
	return panel.Render(body, f.S)
}

func promptLabel(kind promptKind) string {
	switch kind {
	case promptNewDir:
		return "new directory: "
	case promptRename:
		return "rename to: "
	default:
		return "new file: "
	}
}

func (f *Files) entryRow(e host.Entry, selected bool, width int) string {
	name := icon(e) + e.Name
	if e.Target != "" {
		name += " → " + e.Target
	}

	right := ""
	if !e.IsDir {
		right = components.HumanBytes(e.Size)
	}
	if f.br.Clip.Mode == files.ClipCut {
		for _, p := range f.br.Clip.Paths {
			if p == files.Join(f.br.Dir(), e.Name) {
				right = "cut"
			}
		}
	}

	style := f.entryStyle(e)
	if selected {
		style = style.Bold(true)
	}
	row := components.SpaceBetween(
		style.Render(components.Truncate(name, width-len(right)-1)),
		f.S.Muted.Render(right), width)
	if selected {
		row = lipgloss.NewStyle().Background(f.S.T.Selection).Render(row)
	}
	return zone.Mark("file:"+e.Name, row)
}

func (f *Files) entryStyle(e host.Entry) lipgloss.Style {
	t := f.S.T
	if e.IsDir {
		return lipgloss.NewStyle().Foreground(t.Dir).Bold(true)
	}
	switch strings.ToLower(path.Ext(e.Name)) {
	case ".jar":
		return lipgloss.NewStyle().Foreground(t.Jar)
	case ".zip", ".gz", ".tar", ".xz", ".mca":
		return lipgloss.NewStyle().Foreground(t.Archive)
	case ".yml", ".yaml", ".json", ".toml", ".properties", ".conf", ".cfg":
		return lipgloss.NewStyle().Foreground(t.Config)
	case ".sh", ".bat":
		return lipgloss.NewStyle().Foreground(t.Exec)
	}
	if e.Mode&0o111 != 0 {
		return lipgloss.NewStyle().Foreground(t.Exec)
	}
	return f.S.Text
}

func icon(e host.Entry) string {
	if e.IsDir {
		return "▸ "
	}
	return "  "
}

func (f *Files) previewPanel(width int) string {
	pv := f.preview
	panel := components.Panel{Title: "preview", Badge: pv.Note, Width: width, Height: f.Height}
	w, h := panel.ContentWidth(), panel.ContentHeight()

	if pv.Title == "" {
		return panel.Render(components.PadLines(f.S.Muted.Render("nothing selected"), h), f.S)
	}

	var rows []string
	switch pv.Kind {
	case files.PreviewError:
		rows = []string{f.S.Error.Render(components.Truncate(pv.Note, w))}
	case files.PreviewDir:
		for _, e := range pv.Entries {
			if len(rows) >= h {
				break
			}
			rows = append(rows, components.Truncate(f.entryStyle(e).Render(icon(e)+e.Name), w))
		}
		if len(rows) == 0 {
			rows = []string{f.S.Muted.Render("empty directory")}
		}
	case files.PreviewBinary, files.PreviewEmpty:
		rows = []string{f.S.Muted.Render(components.Truncate(pv.Note, w))}
	case files.PreviewJar:
		for _, kv := range pv.Fields {
			rows = append(rows, components.SpaceBetween(
				f.S.Label.Render(kv.Key), f.S.Value.Render(components.Truncate(kv.Value, w*2/3)), w))
		}
		if len(pv.Lines) > 0 {
			rows = append(rows, "")
			for _, l := range components.WrapWords(strings.Join(pv.Lines, " "), w) {
				if len(rows) >= h {
					break
				}
				rows = append(rows, f.S.Muted.Render(l))
			}
		}
		if len(rows) == 0 {
			rows = []string{f.S.Muted.Render(components.Truncate(pv.Note, w))}
		}
	default:
		for _, l := range pv.Lines {
			if len(rows) >= h {
				break
			}
			rows = append(rows, f.S.Subtext.Render(components.Truncate(strings.ReplaceAll(l, "\t", "    "), w)))
		}
	}
	return panel.Render(components.PadLines(strings.Join(rows, "\n"), h), f.S)
}

// Snapshot reports the browser position as one string, for tests that assert an
// inactive tab did not move.
func (f *Files) Snapshot() string {
	name := ""
	if e, ok := f.selected(); ok {
		name = e.Name
	}
	return f.br.Dir() + "|" + name + "|" + strconv.Itoa(len(f.entries))
}
