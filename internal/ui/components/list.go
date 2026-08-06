package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

type ListItem struct {
	ID       string
	Title    string
	Desc     string
	Badge    string
	Marker   string
	Disabled bool
	Data     any
}

type List struct {
	s        *Styles
	items    []ListItem
	visible  []int
	cursor   int
	offset   int
	width    int
	height   int
	filter   string
	ShowDesc bool
	ZonePfx  string
	Empty    string
}

func NewList(s *Styles) *List {
	return &List{s: s, Empty: "nothing found"}
}

func (l *List) SetItems(items []ListItem) {
	l.items = items
	l.applyFilter()
	if l.cursor >= len(l.visible) {
		l.cursor = max(0, len(l.visible)-1)
	}
	l.clampOffset()
}

// Reset scrolls back to the top. Callers that swap the contents for something
// unrelated must use it: a list reused across wizard steps would otherwise open
// at whatever position the previous step left behind, hiding the newest entries
// above the fold.
func (l *List) Reset() {
	l.cursor, l.offset = 0, 0
}

func (l *List) Items() []ListItem { return l.items }

func (l *List) SetSize(w, h int) {
	l.width, l.height = w, h
	l.clampOffset()
}

func (l *List) SetFilter(q string) {
	l.filter = strings.ToLower(strings.TrimSpace(q))
	l.applyFilter()
	l.cursor, l.offset = 0, 0
}

func (l *List) applyFilter() {
	l.visible = l.visible[:0]
	for i, it := range l.items {
		if l.filter == "" ||
			strings.Contains(strings.ToLower(it.Title), l.filter) ||
			strings.Contains(strings.ToLower(it.Desc), l.filter) {
			l.visible = append(l.visible, i)
		}
	}
}

func (l *List) rowsPerItem() int {
	if l.ShowDesc {
		return 2
	}
	return 1
}

func (l *List) capacity() int {
	c := l.height / l.rowsPerItem()
	if c < 1 {
		return 1
	}
	return c
}

func (l *List) clampOffset() {
	cap := l.capacity()
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+cap {
		l.offset = l.cursor - cap + 1
	}
	if l.offset > len(l.visible)-cap {
		l.offset = len(l.visible) - cap
	}
	if l.offset < 0 {
		l.offset = 0
	}
}

func (l *List) Len() int { return len(l.visible) }

func (l *List) Cursor() int { return l.cursor }

func (l *List) Selected() (ListItem, bool) {
	if l.cursor < 0 || l.cursor >= len(l.visible) {
		return ListItem{}, false
	}
	return l.items[l.visible[l.cursor]], true
}

func (l *List) SelectID(id string) {
	for i, idx := range l.visible {
		if l.items[idx].ID == id {
			l.cursor = i
			l.clampOffset()
			return
		}
	}
}

func (l *List) Move(delta int) {
	if len(l.visible) == 0 {
		return
	}
	l.cursor += delta
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.cursor >= len(l.visible) {
		l.cursor = len(l.visible) - 1
	}
	l.clampOffset()
}

func (l *List) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k", "ctrl+p":
			l.Move(-1)
		case "down", "j", "ctrl+n":
			l.Move(1)
		case "pgup", "ctrl+u":
			l.Move(-l.capacity())
		case "pgdown", "ctrl+d":
			l.Move(l.capacity())
		case "home", "g":
			l.cursor = 0
			l.clampOffset()
		case "end", "G":
			l.cursor = max(0, len(l.visible)-1)
			l.clampOffset()
		}
	case tea.MouseMsg:
		switch {
		case WheelUp(m):
			l.Move(-1)
		case WheelDown(m):
			l.Move(1)
		case LeftClick(m) && l.ZonePfx != "":
			for i := range l.visible {
				if zone.Get(l.ZonePfx + l.items[l.visible[i]].ID).InBounds(m) {
					l.cursor = i
					l.clampOffset()
					break
				}
			}
		}
	}
	return nil
}

func (l *List) View() string {
	if len(l.visible) == 0 {
		return l.s.Muted.Render(l.Empty)
	}

	rows := make([]string, 0, l.height)
	end := min(l.offset+l.capacity(), len(l.visible))
	for i := l.offset; i < end; i++ {
		rows = append(rows, l.renderItem(l.items[l.visible[i]], i == l.cursor)...)
	}
	return strings.Join(rows, "\n")
}

func (l *List) renderItem(it ListItem, selected bool) []string {
	s := l.s
	marker := "  "
	if it.Marker != "" {
		marker = it.Marker + " "
	}
	if selected {
		marker = "▸ "
	}

	titleStyle := s.Text
	switch {
	case it.Disabled:
		titleStyle = s.Muted
	case selected:
		titleStyle = lipgloss.NewStyle().Foreground(s.T.Accent).Bold(true)
	}

	badge := ""
	if it.Badge != "" {
		badge = s.Muted.Render(it.Badge)
	}

	left := lipgloss.NewStyle().Foreground(s.T.Accent).Render(marker) + titleStyle.Render(it.Title)
	head := SpaceBetween(left, badge, l.width)
	if selected {
		head = lipgloss.NewStyle().Background(s.T.Selection).Render(head)
	}

	out := []string{head}
	if l.ShowDesc {
		// Each item owns exactly rowsPerItem lines, so a description carrying a
		// newline of its own — Modrinth summaries often do — has to be flattened
		// or it pushes every row below it out of alignment.
		desc := "  " + Truncate(strings.Join(strings.Fields(it.Desc), " "), l.width-2)
		out = append(out, s.Muted.Render(desc))
	}
	if l.ZonePfx != "" {
		out[0] = zone.Mark(l.ZonePfx+it.ID, out[0])
	}
	return out
}

func (l *List) ScrollBadge() string {
	if len(l.visible) == 0 {
		return ""
	}
	return itoa(l.cursor+1) + "/" + itoa(len(l.visible))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
