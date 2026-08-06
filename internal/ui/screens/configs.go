package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/bx-team/irori/internal/confdiff"
	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/confs"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/props"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/msgs"
)

const (
	propsFileName = "server.properties"
	detailWidth   = 38
	filesWidth    = 30
	// minDetailCols and minThreeCols are the widths at which the detail column
	// and then the file picker earn their space; below them the panes take
	// turns instead of being squeezed into unreadable columns.
	minDetailCols = 96
	minThreeCols  = 126
)

type configDiffMsg struct {
	res confdiff.Result
	err error
}

type configDeclareMsg struct{ changes []confdiff.Change }

type configPane int

const (
	paneFileList configPane = iota
	paneEntries
)

// openDoc is a config file the user has visited. Edits live here rather than in
// the Fields widget so switching files and coming back does not quietly throw
// away work that was never saved.
type openDoc struct {
	doc    *confs.Doc
	fields []components.Field
	orig   map[string]string
	saved  bool
}

func (o *openDoc) changed() []components.Field {
	var out []components.Field
	for _, f := range o.fields {
		if f.Value != o.orig[f.Key] {
			out = append(out, f)
		}
	}
	return out
}

type Configs struct {
	Base
	cfg *config.Config
	h   host.Backend

	files   *components.List
	fields  *components.Fields
	open    map[string]*openDoc
	current string

	filter    textinput.Model
	filtering bool
	pane      configPane

	running   bool
	lastState models.ServerState
	err       string
}

func NewConfigs(s *components.Styles, cfg *config.Config) *Configs {
	fl := s.TextInput("/ ", s.Warning)
	fl.Placeholder = "filter keys"

	c := &Configs{
		cfg:    cfg,
		h:      host.NewLocal(cfg.Dir()),
		files:  components.NewList(s),
		fields: components.NewFields(s),
		open:   map[string]*openDoc{},
		filter: fl,
		pane:   paneEntries,
	}
	c.S = s
	c.files.ZonePfx = "cfgfile:"
	c.files.Empty = "no configuration files found"
	c.fields.ZonePfx = "cfg:"
	c.fields.LabelWidth = 40
	c.discover()
	return c
}

func (c *Configs) Init() tea.Cmd { return nil }

func (c *Configs) CapturesInput() bool { return c.filtering || c.fields.Editing() }

func (c *Configs) discover() {
	refs := confs.Discover(c.h)

	items := make([]components.ListItem, 0, len(refs))
	for _, r := range refs {
		items = append(items, components.ListItem{
			ID:    r.Path,
			Title: r.Title,
			Badge: r.Owner,
			Data:  r,
		})
	}
	c.files.SetItems(items)

	if len(refs) == 0 {
		c.current = ""
		c.err = ""
		c.fields.SetFields(nil)
		return
	}

	want := refs[0].Path
	for _, r := range refs {
		if r.Path == propsFileName {
			want = r.Path
			break
		}
	}
	if _, ok := c.open[c.current]; !ok {
		c.select_(want)
	} else {
		c.files.SelectID(c.current)
		c.refreshBadges()
	}
}

func (c *Configs) select_(rel string) {
	if rel == c.current {
		return
	}
	c.stash()

	od, ok := c.open[rel]
	if !ok {
		doc, err := confs.Load(c.h, rel)
		if err != nil {
			c.err = err.Error()
			c.current = rel
			c.fields.SetFields(nil)
			c.files.SelectID(rel)
			return
		}
		od = newOpenDoc(c.cfg, doc)
		c.open[rel] = od
	}

	c.current = rel
	c.err = ""
	c.fields.SetFilter("")
	c.filter.SetValue("")
	c.fields.SetFields(od.fields)
	c.files.SelectID(rel)
	c.refreshBadges()
}

func (c *Configs) stash() {
	if od, ok := c.open[c.current]; ok {
		od.fields = c.fields.All()
	}
}

func newOpenDoc(cfg *config.Config, doc *confs.Doc) *openDoc {
	entries := doc.Entries()
	od := &openDoc{doc: doc, orig: make(map[string]string, len(entries))}
	od.fields = make([]components.Field, 0, len(entries))
	for _, e := range entries {
		od.orig[e.Key] = e.Value
		od.fields = append(od.fields, components.Field{
			Key:      e.Key,
			Label:    e.Label,
			Kind:     fieldKind(e.Kind),
			Value:    e.Value,
			Options:  e.Values,
			Min:      e.Min,
			Max:      e.Max,
			Desc:     e.Desc,
			Default:  e.Default,
			Section:  e.Section,
			Secret:   e.Secret,
			ReadOnly: e.ReadOnly,
			Locked:   cfg.HasOverride(doc.Path, e.Key),
		})
	}
	return od
}

func fieldKind(k props.Kind) components.FieldKind {
	switch k {
	case props.KindBool:
		return components.FieldBool
	case props.KindInt:
		return components.FieldInt
	case props.KindEnum:
		return components.FieldEnum
	default:
		return components.FieldText
	}
}

func (c *Configs) refreshBadges() {
	items := c.files.Items()
	for i := range items {
		ref, ok := items[i].Data.(confs.FileRef)
		if !ok {
			continue
		}
		badge := ref.Owner
		if od, open := c.open[ref.Path]; open {
			if n := len(od.changed()); n > 0 {
				badge = fmt.Sprintf("%d modified", n)
			}
		}
		items[i].Badge = badge
	}
	c.files.SetItems(items)
	c.files.SelectID(c.current)
}

func (c *Configs) doc() (*openDoc, bool) {
	od, ok := c.open[c.current]
	return od, ok
}

func (c *Configs) changed() []components.Field {
	od, ok := c.doc()
	if !ok {
		return nil
	}
	od.fields = c.fields.All()
	return od.changed()
}

func (c *Configs) reload() {
	if c.current == "" {
		return
	}
	delete(c.open, c.current)
	rel := c.current
	c.current = ""
	c.select_(rel)
}

func (c *Configs) rescan() {
	c.stash()
	was := c.current
	for path, od := range c.open {
		if len(od.changed()) == 0 {
			delete(c.open, path)
		}
	}

	c.current = ""
	c.discover()
	if was == "" || was == c.current {
		return
	}
	for _, it := range c.files.Items() {
		if it.ID == was {
			c.select_(was)
			return
		}
	}
}

func (c *Configs) markDirty() {
	fields := c.fields.All()
	od, ok := c.doc()
	if !ok {
		return
	}
	for i := range fields {
		fields[i].Dirty = fields[i].Value != od.orig[fields[i].Key]
	}
	c.fields.SetFields(fields)
	od.fields = fields
	c.refreshBadges()
}

func (c *Configs) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case msgs.StatusMsg:
		c.running = m.Status.State.IsUp()
		if m.Status.State != c.lastState {
			c.lastState = m.Status.State
			c.rescan()
		}
		return c, nil

	case msgs.ConfigChangedMsg:
		c.discover()
		return c, nil

	case msgs.LinkDownMsg:
		c.running = false
		return c, nil

	case configDiffMsg:
		if m.err != nil {
			return c, errToast("Config", m.err)
		}
		return c, c.confirmDeclare(m.res)

	case configDeclareMsg:
		return c, c.declare(m.changes)

	case tea.KeyMsg:
		return c.handleKey(m)

	case tea.MouseMsg:
		if c.pane == paneFileList {
			cmd := c.files.Update(msg)
			if sel, ok := c.files.Selected(); ok {
				c.select_(sel.ID)
			}
			return c, cmd
		}
		cmd, changed := c.fields.Update(msg)
		if changed {
			c.onChange()
		}
		return c, cmd
	}

	if c.fields.Editing() {
		cmd, _ := c.fields.Update(msg)
		return c, cmd
	}
	return c, nil
}

func (c *Configs) handleKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	if c.filtering {
		switch k.String() {
		case "esc":
			c.filtering = false
			c.filter.SetValue("")
			c.filter.Blur()
			c.fields.SetFilter("")
			return c, nil
		case "enter":
			c.filtering = false
			c.filter.Blur()
			return c, nil
		}
		var cmd tea.Cmd
		c.filter, cmd = c.filter.Update(k)
		c.fields.SetFilter(c.filter.Value())
		return c, cmd
	}

	if c.fields.Editing() {
		cmd, changed := c.fields.Update(k)
		if changed {
			c.onChange()
		}
		return c, cmd
	}

	switch k.String() {
	case "tab":
		if c.pane == paneEntries {
			c.pane = paneFileList
		} else {
			c.pane = paneEntries
		}
		return c, nil
	case "R":
		c.discover()
		return c, toast("rescanned configuration files", models.LevelIrori)
	}

	if c.pane == paneFileList {
		return c.handleFileKey(k)
	}
	return c.handleEntryKey(k)
}

func (c *Configs) handleFileKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	switch k.String() {
	case "enter":
		c.pane = paneEntries
		return c, nil
	case "R":
		c.rescan()
		return c, toast("rescanned", models.LevelIrori)
	}
	cmd := c.files.Update(k)
	if sel, ok := c.files.Selected(); ok {
		c.select_(sel.ID)
	}
	return c, cmd
}

func (c *Configs) handleEntryKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	switch k.String() {
	case "/":
		c.filtering = true
		c.filter.Focus()
		return c, textinput.Blink
	case "ctrl+s":
		return c, c.save()
	case "r":
		c.reload()
		return c, toast("reloaded "+c.current, models.LevelIrori)
	case "R":
		c.rescan()
		return c, toast("rescanned", models.LevelIrori)
	case "D":
		return c, c.resetToDefault()
	case "u":
		c.undo()
		return c, nil
	case "c":
		return c, c.toggleDeclared()
	case "C":
		return c, c.declareChanged()
	case "e":
		if c.current == "" {
			return c, nil
		}
		p := c.h.Abs(c.current)
		return c, func() tea.Msg { return msgs.OpenEditorMsg{Path: p} }
	}

	cmd, changed := c.fields.Update(k)
	if changed {
		c.onChange()
	}
	return c, cmd
}

func (c *Configs) declareChanged() tea.Cmd {
	if config.Sealed() {
		return toast("sealed mode: configuration is managed by Nix", models.LevelWarn)
	}
	if c.cfg.Server.BuildID == "" {
		return toast("no build id in "+config.FileName+", run irori apply first", models.LevelWarn)
	}
	cfg, h := c.cfg, c.h
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		shipped, err := mcjars.New().Configs(ctx, cfg.Server.BuildID)
		if err != nil {
			return configDiffMsg{err: err}
		}
		return configDiffMsg{res: confdiff.Compare(h, shipped)}
	}
}

func (c *Configs) confirmDeclare(res confdiff.Result) tea.Cmd {
	if res.Empty() {
		return toast("every shipped key still holds the value "+c.cfg.Server.Type.Display()+" ships",
			models.LevelIrori)
	}
	body := fmt.Sprintf("%d key(s) in %d file(s) differ from what the core ships.\n"+
		"They will be declared in %s, so irori writes them back after every install.",
		len(res.Changes), len(res.Compared), config.FileName)
	if n := len(res.Skipped); n > 0 {
		body += fmt.Sprintf("\n%d more are left alone: secrets, lists and version markers.", n)
	}
	return func() tea.Msg {
		return msgs.ConfirmMsg{Title: "Declare changed keys", Body: body + "\nContinue?",
			OnYes: configDeclareMsg{changes: res.Changes}}
	}
}

func (c *Configs) declare(changes []confdiff.Change) tea.Cmd {
	confdiff.Declare(c.cfg, changes)
	if err := c.cfg.Save(); err != nil {
		return errToast("Config", err)
	}
	c.refreshLocks()
	return toast(fmt.Sprintf("declared %d key(s) in %s", len(changes), config.FileName), models.LevelIrori)
}

func (c *Configs) refreshLocks() {
	for file, od := range c.open {
		for i := range od.fields {
			od.fields[i].Locked = c.cfg.HasOverride(file, od.fields[i].Key)
		}
	}
	if od, ok := c.doc(); ok {
		c.fields.SetFields(od.fields)
	}
}

func (c *Configs) onChange() {
	if od, ok := c.doc(); ok {
		od.saved = false
	}
	c.markDirty()
}

func (c *Configs) resetToDefault() tea.Cmd {
	f, ok := c.fields.Selected()
	if !ok {
		return nil
	}
	if f.Default == "" && f.Kind != components.FieldBool {
		return toast("no shipped default for this key — restore the whole file from the Files tab",
			models.LevelWarn)
	}
	c.setValue(f.Key, f.Default)
	return toast(f.Key+" reset to default", models.LevelIrori)
}

func (c *Configs) undo() {
	f, ok := c.fields.Selected()
	od, open := c.doc()
	if !ok || !open {
		return
	}
	c.setValue(f.Key, od.orig[f.Key])
}

func (c *Configs) setValue(key, value string) {
	fields := c.fields.All()
	for i := range fields {
		if fields[i].Key == key {
			fields[i].Value = value
		}
	}
	c.fields.SetFields(fields)
	c.onChange()
}

func (c *Configs) toggleDeclared() tea.Cmd {
	f, ok := c.fields.Selected()
	od, open := c.doc()
	if !ok || !open {
		return nil
	}
	if f.Locked {
		c.cfg.UnsetOverride(od.doc.Path, f.Key)
	} else {
		c.cfg.SetOverride(od.doc.Path, f.Key, typedValue(od.doc, f))
	}
	if err := c.cfg.Save(); err != nil {
		return errToast("Config", err)
	}

	fields := c.fields.All()
	for i := range fields {
		if fields[i].Key == f.Key {
			fields[i].Locked = !f.Locked
		}
	}
	c.fields.SetFields(fields)
	od.fields = fields

	if f.Locked {
		return toast(f.Key+" is no longer declared in "+config.FileName, models.LevelIrori)
	}
	return toast(f.Key+" is now declared in "+config.FileName, models.LevelIrori)
}

func typedValue(doc *confs.Doc, f components.Field) any {
	if e, ok := doc.Entry(f.Key); ok {
		return confs.Typed(e.Kind, f.Value)
	}
	return f.Value
}

func (c *Configs) save() tea.Cmd {
	od, ok := c.doc()
	if !ok {
		return toast("nothing to save", models.LevelWarn)
	}
	changed := c.changed()
	if len(changed) == 0 {
		return toast("nothing to save", models.LevelWarn)
	}

	values := make(map[string]string, len(changed))
	declaredTouched := false
	for _, f := range changed {
		values[f.Key] = f.Value
		if f.Locked {
			c.cfg.SetOverride(od.doc.Path, f.Key, typedValue(od.doc, f))
			declaredTouched = true
		}
	}
	if err := confs.Save(c.h, od.doc, values); err != nil {
		return errToast("Save", err)
	}
	if declaredTouched {
		if err := c.cfg.Save(); err != nil {
			return errToast("Config", err)
		}
	}

	for _, f := range changed {
		od.orig[f.Key] = f.Value
	}
	od.saved = true
	c.markDirty()

	text := fmt.Sprintf("saved %d change(s) to %s", len(changed), od.doc.Path)
	if c.running {
		text += " — restart to apply"
	}
	return toast(text, models.LevelIrori)
}

func (c *Configs) Hints() []components.Hint {
	if c.filtering {
		return []components.Hint{
			{Key: "type", Desc: "filter"},
			{Key: "Enter", Desc: "keep"},
			{Key: "Esc", Desc: "clear"},
		}
	}
	if c.fields.Editing() {
		return []components.Hint{
			{Key: "Enter", Desc: "commit"},
			{Key: "Esc", Desc: "cancel"},
		}
	}
	if c.pane == paneFileList {
		return []components.Hint{
			{Key: "↑↓", Desc: "pick a file"},
			{Key: "Enter/Tab", Desc: "edit it"},
			{Key: "R", Desc: "rescan"},
		}
	}
	return []components.Hint{
		{Key: "Tab", Desc: "switch file"},
		{Key: "Space/←→", Desc: "change"},
		{Key: "Enter", Desc: "edit"},
		{Key: "Ctrl+S", Desc: "save"},
		{Key: "c/C", Desc: "declare key/all"},
		{Key: "r/R", Desc: "reload rescan"},
		{Key: "u", Desc: "undo"},
		{Key: "e", Desc: "$EDITOR"},
		{Key: "/", Desc: "filter"},
	}
}

func (c *Configs) View() string {
	if c.files.Len() == 0 {
		return c.emptyPanel()
	}

	wide := c.Width >= minDetailCols
	three := c.Width >= minThreeCols

	if !three && c.pane == paneFileList {
		return c.filesPanel(c.Width)
	}

	entriesWidth := c.Width
	var cols []string
	if three {
		entriesWidth -= filesWidth
		cols = append(cols, c.filesPanel(filesWidth))
	}
	if wide {
		entriesWidth -= detailWidth
	}

	cols = append(cols, c.entriesPanel(entriesWidth, wide))
	if wide {
		cols = append(cols, c.detail())
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (c *Configs) emptyPanel() string {
	panel := components.Panel{Title: "configs", Width: c.Width, Height: c.Height, Focused: true}
	note := fmt.Sprintf("No configuration files found in %s.\nStart the server once so it writes them, or use the Files tab.",
		c.cfg.Dir())
	if c.err != "" {
		note = c.err
	}
	body := lipgloss.Place(panel.ContentWidth(), panel.ContentHeight(),
		lipgloss.Center, lipgloss.Center, c.S.Muted.Render(note))
	return panel.Render(body, c.S)
}

func (c *Configs) filesPanel(width int) string {
	panel := components.Panel{
		Title:   "config files",
		Badge:   c.files.ScrollBadge(),
		Focused: c.pane == paneFileList,
		Width:   width,
		Height:  c.Height,
	}
	c.files.SetSize(panel.ContentWidth(), panel.ContentHeight())
	return panel.Render(components.PadLines(c.files.View(), panel.ContentHeight()), c.S)
}

func (c *Configs) entriesPanel(width int, wide bool) string {
	title := c.current
	if title == "" {
		title = "configs"
	}
	panel := components.Panel{
		Title:   title,
		Badge:   c.badge(),
		Focused: c.pane == paneEntries,
		Width:   width,
		Height:  c.Height,
	}

	if c.err != "" {
		body := lipgloss.Place(panel.ContentWidth(), panel.ContentHeight(),
			lipgloss.Center, lipgloss.Center, c.S.Error.Render("! "+c.err))
		return panel.Render(body, c.S)
	}

	body := c.header(panel.ContentWidth()) + "\n" + c.fieldsView(panel, wide)
	return panel.Render(body, c.S)
}

func (c *Configs) fieldsView(panel components.Panel, wide bool) string {
	h := panel.ContentHeight() - 1
	if !wide {
		h -= 2
	}
	if h < 1 {
		h = 1
	}
	c.fields.SetSize(panel.ContentWidth(), h)

	view := components.PadLines(c.fields.View(), h)
	if wide {
		return view
	}
	return view + "\n" + c.inlineDesc(panel.ContentWidth())
}

func (c *Configs) header(width int) string {
	if c.filtering {
		c.filter.SetWidth(width - 3)
		return c.filter.View()
	}

	s := c.S
	od, open := c.doc()
	n := len(c.changed())
	var left string
	switch {
	case n > 0:
		left = s.Accent.Render(fmt.Sprintf("● %d unsaved change(s)", n))
	case open && od.saved && c.running:
		left = s.Warning.Render("saved — restart required")
	case open && od.saved:
		left = s.Success.Render("saved")
	default:
		left = s.Muted.Render(c.summary())
	}
	if e := c.fields.Err(); e != "" {
		left = s.Error.Render("! " + e)
	}
	return components.SpaceBetween(left, s.Muted.Render(c.fields.ScrollBadge()), width)
}

func (c *Configs) summary() string {
	od, ok := c.doc()
	if !ok {
		return "no pending changes"
	}
	return fmt.Sprintf("%s · %d keys", od.doc.Format, len(od.fields))
}

func (c *Configs) badge() string {
	if n := len(c.changed()); n > 0 {
		return fmt.Sprintf("%d modified", n)
	}
	return ""
}

func (c *Configs) inlineDesc(width int) string {
	f, ok := c.fields.Selected()
	if !ok {
		return ""
	}
	desc := f.Desc
	if desc == "" {
		desc = "no description for this key"
	}
	wrapped := strings.Split(lipgloss.NewStyle().Width(width).Render(desc), "\n")
	out := c.S.Muted.Render(components.Truncate(wrapped[0], width))
	if len(wrapped) > 1 {
		out += "\n" + c.S.Muted.Render(components.Truncate(wrapped[1], width))
	} else {
		out += "\n"
	}
	return out
}

func (c *Configs) detail() string {
	s := c.S
	panel := components.Panel{Title: "detail", Width: detailWidth, Height: c.Height}
	f, ok := c.fields.Selected()
	od, open := c.doc()
	if !ok || !open {
		return panel.Render("", s)
	}

	w := panel.ContentWidth()
	rows := []string{
		s.Bold.Render(components.Truncate(f.Label, w)),
	}
	if f.Section != "" {
		rows = append(rows, s.Muted.Render("in "+tailPath(f.Section, w-3)))
	}
	rows = append(rows, "",
		components.SpaceBetween(s.Label.Render("type"), s.Value.Render(kindName(f.Kind)), w))

	if f.ReadOnly {
		rows = append(rows, components.SpaceBetween(
			s.Label.Render("editing"), s.Warning.Render("$EDITOR only"), w))
	}
	if f.Default != "" {
		rows = append(rows, components.SpaceBetween(
			s.Label.Render("default"), s.Value.Render(components.Truncate(f.Default, w/2)), w))
	}
	if f.Kind == components.FieldInt && (f.Min != 0 || f.Max != 0) {
		rows = append(rows, components.SpaceBetween(
			s.Label.Render("range"), s.Value.Render(fmt.Sprintf("%d … %d", f.Min, f.Max)), w))
	}
	if f.Value != od.orig[f.Key] {
		rows = append(rows, components.SpaceBetween(
			s.Label.Render("was"), s.Muted.Render(components.Truncate(od.orig[f.Key], w/2)), w))
	}
	rows = append(rows, components.SpaceBetween(
		s.Label.Render("declared"), s.Value.Render(yesNo(f.Locked)), w))

	if len(f.Options) > 0 {
		rows = append(rows, "", s.Label.Render("values"))
		for _, o := range f.Options {
			marker := s.Muted.Render("  ")
			if o == f.Value {
				marker = s.Accent.Render("▸ ")
			}
			rows = append(rows, marker+s.Value.Render(components.Truncate(o, w-2)))
		}
	}

	if f.ReadOnly {
		rows = append(rows, "", s.Label.Render("value"),
			s.Value.Render(lipgloss.NewStyle().Width(w).Render(f.Value)))
	}

	if f.Desc != "" {
		rows = append(rows, "", s.Muted.Render(lipgloss.NewStyle().Width(w).Render(f.Desc)))
	} else {
		rows = append(rows, "", s.Dim.Render(lipgloss.NewStyle().Width(w).Render(
			"This file carries no comment for this key.")))
	}

	return panel.Render(components.PadLines(strings.Join(rows, "\n"), panel.ContentHeight()), s)
}

func tailPath(section string, width int) string {
	if len(section) <= width {
		return section
	}
	parts := strings.Split(section, ".")
	for i := 1; i < len(parts); i++ {
		tail := "…" + strings.Join(parts[i:], ".")
		if len(tail) <= width {
			return tail
		}
	}
	return components.Truncate(parts[len(parts)-1], width)
}

func kindName(k components.FieldKind) string {
	switch k {
	case components.FieldBool:
		return "boolean"
	case components.FieldInt:
		return "integer"
	case components.FieldEnum:
		return "enum"
	default:
		return "string"
	}
}

// ServerIsUp reports what the last status broadcast said, so the shell's tests
// can prove a background tab still receives state.
func (c *Configs) ServerIsUp() bool { return c.running }

// Current is the file being edited, used by tests to prove the picker moved.
func (c *Configs) Current() string { return c.current }

// FileList is the discovered set, in picker order.
func (c *Configs) FileList() []string {
	items := c.files.Items()
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}
