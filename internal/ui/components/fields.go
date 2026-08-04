package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type FieldKind int

const (
	FieldText FieldKind = iota
	FieldBool
	FieldInt
	FieldEnum
	FieldAction
)

type Field struct {
	Key      string
	Label    string
	Kind     FieldKind
	Value    string
	Options  []string
	Min      int
	Max      int
	Desc     string
	Default  string
	Section  string
	Note     string
	Locked   bool
	Secret   bool
	ReadOnly bool
	Dirty    bool

	Validate func(string) error
}

func (f Field) editable() bool { return !f.ReadOnly && f.Kind != FieldAction }

type fieldRow struct {
	header string
	idx    int
}

// Fields renders a list of typed settings rows: booleans toggle in place, enums
// cycle, and text or numeric values open an inline editor on the row itself.
type Fields struct {
	s      *Styles
	fields []Field
	rows   []fieldRow
	cursor int
	offset int

	width  int
	height int

	filter  string
	editing bool
	input   textinput.Model
	err     string

	LabelWidth int
	ZonePfx    string
	Empty      string
}

func NewFields(s *Styles) *Fields {
	in := textinput.New()
	in.Prompt = ""
	in.TextStyle = s.Text
	in.Cursor.Style = s.Cursor
	return &Fields{s: s, input: in, Empty: "nothing found"}
}

func (f *Fields) SetFields(fields []Field) {
	key, _ := f.SelectedKey()
	f.fields = fields
	f.rebuild()
	if key != "" {
		f.SelectKey(key)
	}
}

func (f *Fields) All() []Field { return f.fields }

func (f *Fields) SetSize(w, h int) {
	f.width, f.height = w, h
	f.clamp()
}

func (f *Fields) SetFilter(q string) {
	f.filter = strings.ToLower(strings.TrimSpace(q))
	f.rebuild()
	f.cursor, f.offset = 0, 0
	f.toField(1)
}

func (f *Fields) Filter() string { return f.filter }

func (f *Fields) Editing() bool { return f.editing }

func (f *Fields) Err() string { return f.err }

func (f *Fields) rebuild() {
	f.rows = f.rows[:0]
	section := ""
	for i, fl := range f.fields {
		if !f.matches(fl) {
			continue
		}
		if fl.Section != "" && fl.Section != section {
			section = fl.Section
			f.rows = append(f.rows, fieldRow{header: section, idx: -1})
		}
		f.rows = append(f.rows, fieldRow{idx: i})
	}
	f.clamp()
	f.toField(1)
}

func (f *Fields) matches(fl Field) bool {
	if f.filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(fl.Key), f.filter) ||
		strings.Contains(strings.ToLower(fl.Label), f.filter) ||
		strings.Contains(strings.ToLower(fl.Desc), f.filter)
}

func (f *Fields) capacity() int {
	if f.height < 1 {
		return 1
	}
	return f.height
}

func (f *Fields) clamp() {
	if f.cursor >= len(f.rows) {
		f.cursor = len(f.rows) - 1
	}
	if f.cursor < 0 {
		f.cursor = 0
	}
	c := f.capacity()
	if f.cursor < f.offset {
		f.offset = f.cursor
	}
	if f.cursor >= f.offset+c {
		f.offset = f.cursor - c + 1
	}
	// The window can grow after a resize, so also pull the offset back rather
	// than leaving blank rows below the last row.
	if f.offset > len(f.rows)-c {
		f.offset = len(f.rows) - c
	}
	if f.offset < 0 {
		f.offset = 0
	}
}

// toField walks in the given direction until the cursor rests on a field rather
// than a section header.
func (f *Fields) toField(dir int) {
	for f.cursor >= 0 && f.cursor < len(f.rows) && f.rows[f.cursor].idx < 0 {
		f.cursor += dir
	}
	if f.cursor >= len(f.rows) {
		f.cursor = len(f.rows) - 1
		for f.cursor >= 0 && f.rows[f.cursor].idx < 0 {
			f.cursor--
		}
	}
	if f.cursor < 0 {
		f.cursor = 0
	}
	f.clamp()
}

func (f *Fields) move(delta int) {
	if len(f.rows) == 0 {
		return
	}
	dir := 1
	if delta < 0 {
		dir = -1
	}
	for n := 0; n < abs(delta); n++ {
		next := f.cursor + dir
		for next >= 0 && next < len(f.rows) && f.rows[next].idx < 0 {
			next += dir
		}
		if next < 0 || next >= len(f.rows) {
			break
		}
		f.cursor = next
	}
	f.clamp()
}

func (f *Fields) Selected() (Field, bool) {
	if f.cursor < 0 || f.cursor >= len(f.rows) {
		return Field{}, false
	}
	i := f.rows[f.cursor].idx
	if i < 0 {
		return Field{}, false
	}
	return f.fields[i], true
}

func (f *Fields) SelectedKey() (string, bool) {
	fl, ok := f.Selected()
	return fl.Key, ok
}

func (f *Fields) SelectKey(key string) {
	for i, r := range f.rows {
		if r.idx >= 0 && f.fields[r.idx].Key == key {
			f.cursor = i
			f.clamp()
			return
		}
	}
	f.toField(1)
}

func (f *Fields) set(value string) {
	if i := f.rows[f.cursor].idx; i >= 0 {
		f.fields[i].Value = value
	}
}

// Update handles navigation and editing. The second return reports that a value
// changed, in which case the caller reads Selected() for the new state.
func (f *Fields) Update(msg tea.Msg) (tea.Cmd, bool) {
	if f.editing {
		return f.updateEditing(msg)
	}

	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k", "ctrl+p":
			f.move(-1)
		case "down", "j", "ctrl+n":
			f.move(1)
		case "pgup", "ctrl+u":
			f.move(-f.capacity())
		case "pgdown", "ctrl+d":
			f.move(f.capacity())
		case "home", "g":
			f.cursor = 0
			f.toField(1)
		case "end", "G":
			f.cursor = len(f.rows) - 1
			f.toField(-1)
		case "left", "h":
			return nil, f.cycle(-1)
		case "right", "l":
			return nil, f.cycle(1)
		case " ":
			return nil, f.activate()
		case "enter":
			fl, ok := f.Selected()
			if !ok {
				return nil, false
			}
			if fl.Kind == FieldText || fl.Kind == FieldInt {
				if fl.ReadOnly {
					return nil, false
				}
				f.beginEdit(fl)
				return textinput.Blink, false
			}
			return nil, f.activate()
		}

	case tea.MouseMsg:
		switch {
		case m.Button == tea.MouseButtonWheelUp:
			f.move(-1)
		case m.Button == tea.MouseButtonWheelDown:
			f.move(1)
		case m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft && f.ZonePfx != "":
			for i, r := range f.rows {
				if r.idx < 0 {
					continue
				}
				if zone.Get(f.ZonePfx + f.fields[r.idx].Key).InBounds(m) {
					if f.cursor == i {
						return nil, f.activate()
					}
					f.cursor = i
					f.clamp()
					break
				}
			}
		}
	}
	return nil, false
}

func (f *Fields) updateEditing(msg tea.Msg) (tea.Cmd, bool) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return cmd, false
	}

	switch key.String() {
	case "esc":
		f.editing, f.err = false, ""
		return nil, false
	case "enter":
		fl, _ := f.Selected()
		value := strings.TrimSpace(f.input.Value())
		if err := validate(fl, value); err != nil {
			f.err = err.Error()
			return nil, false
		}
		f.editing, f.err = false, ""
		if value == fl.Value {
			return nil, false
		}
		f.set(value)
		return nil, true
	}

	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	f.err = ""
	return cmd, false
}

func validate(fl Field, value string) error {
	if fl.Validate != nil {
		if err := fl.Validate(value); err != nil {
			return err
		}
	}
	if fl.Kind != FieldInt {
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%q is not a whole number", value)
	}
	if fl.Min != 0 || fl.Max != 0 {
		if n < fl.Min || n > fl.Max {
			return fmt.Errorf("must be between %d and %d", fl.Min, fl.Max)
		}
	}
	return nil
}

func (f *Fields) beginEdit(fl Field) {
	f.editing, f.err = true, ""
	f.input.SetValue(fl.Value)
	f.input.CursorEnd()
	f.input.Width = f.valueWidth() - 1
	f.input.Focus()
}

// activate toggles booleans and advances enums; other kinds report no change so
// the caller can treat Enter on them as its own action.
func (f *Fields) activate() bool {
	fl, ok := f.Selected()
	if !ok || !fl.editable() {
		return false
	}
	switch fl.Kind {
	case FieldBool:
		f.set(boolStr(fl.Value != "true"))
		return true
	case FieldEnum:
		return f.cycle(1)
	}
	return false
}

func (f *Fields) cycle(dir int) bool {
	fl, ok := f.Selected()
	if !ok || !fl.editable() {
		return false
	}
	switch fl.Kind {
	case FieldBool:
		f.set(boolStr(fl.Value != "true"))
		return true
	case FieldEnum:
		if len(fl.Options) == 0 {
			return false
		}
		at := 0
		for i, o := range fl.Options {
			if o == fl.Value {
				at = i
				break
			}
		}
		at = ((at+dir)%len(fl.Options) + len(fl.Options)) % len(fl.Options)
		f.set(fl.Options[at])
		return true
	case FieldInt:
		n, err := strconv.Atoi(fl.Value)
		if err != nil {
			return false
		}
		n += dir
		if fl.Min != 0 || fl.Max != 0 {
			if n < fl.Min || n > fl.Max {
				return false
			}
		}
		f.set(strconv.Itoa(n))
		return true
	}
	return false
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func (f *Fields) labelWidth() int {
	if f.LabelWidth > 0 {
		return min(f.LabelWidth, f.width-8)
	}
	w := f.width / 2
	return clampInt(w, 10, 40)
}

func (f *Fields) valueWidth() int {
	w := f.width - f.labelWidth() - 3
	if w < 4 {
		return 4
	}
	return w
}

func (f *Fields) View() string {
	if len(f.rows) == 0 {
		return f.s.Muted.Render(f.Empty)
	}

	out := make([]string, 0, f.capacity())
	end := min(f.offset+f.capacity(), len(f.rows))
	for i := f.offset; i < end; i++ {
		r := f.rows[i]
		if r.idx < 0 {
			out = append(out, f.s.PanelTitle.Render(strings.ToUpper(r.header)))
			continue
		}
		out = append(out, f.renderField(f.fields[r.idx], i == f.cursor))
	}
	return strings.Join(out, "\n")
}

func (f *Fields) renderField(fl Field, selected bool) string {
	s := f.s
	labelW, valueW := f.labelWidth(), f.valueWidth()

	mark := "  "
	if fl.Dirty {
		mark = s.Accent.Render("● ")
	}

	label := fl.Label
	if label == "" {
		label = fl.Key
	}
	if fl.Locked {
		label += " [cfg]"
	}
	labelStyle := s.Text
	switch {
	case selected:
		labelStyle = s.Bold
	case fl.ReadOnly:
		labelStyle = s.Muted
	}
	label = labelStyle.Render(Truncate(label, labelW))
	label += strings.Repeat(" ", max(0, labelW-lipglossWidth(label)))

	value := f.renderValue(fl, selected, valueW)
	row := mark + label + " " + value

	if selected && !f.editing {
		row = lipgloss.NewStyle().Background(s.T.Selection).Render(
			row + strings.Repeat(" ", max(0, f.width-lipglossWidth(row))))
	}
	if f.ZonePfx != "" {
		row = zone.Mark(f.ZonePfx+fl.Key, row)
	}
	return row
}

func (f *Fields) renderValue(fl Field, selected bool, width int) string {
	s := f.s

	if selected && f.editing {
		return f.input.View()
	}

	switch fl.Kind {
	case FieldBool:
		if fl.Value == "true" {
			return s.Success.Render("◉ true")
		}
		return s.Muted.Render("◯ false")

	case FieldEnum:
		v := Truncate(fl.Value, width-4)
		if selected && !fl.ReadOnly {
			return s.Muted.Render("‹ ") + s.Accent.Render(v) + s.Muted.Render(" ›")
		}
		return s.Value.Render(v)

	case FieldAction:
		return s.Accent.Render(Truncate(fl.Note, width))
	}

	v := fl.Value
	if fl.Secret && v != "" {
		v = strings.Repeat("•", min(len(v), 12))
	}
	if v == "" {
		out := s.Muted.Render("(empty)")
		if fl.Note != "" {
			out = s.Muted.Render(Truncate(fl.Note, width))
		}
		return out
	}
	style := s.Value
	if fl.ReadOnly {
		style = s.Muted
	}
	return style.Render(Truncate(v, width))
}

// ScrollBadge counts fields, not rows, so section headers do not inflate it.
func (f *Fields) ScrollBadge() string {
	total, at := 0, 0
	for i, r := range f.rows {
		if r.idx < 0 {
			continue
		}
		total++
		if i <= f.cursor {
			at = total
		}
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", at, total)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
