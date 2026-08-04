package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type Modal struct {
	Title  string
	Body   string
	Width  int
	Danger bool
}

func (m Modal) Render(s *Styles, buttons string) string {
	width := m.Width
	if width < 30 {
		width = 30
	}

	accent := s.T.Accent
	if m.Danger {
		accent = s.T.Error
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Background(s.T.Mantle).
		Padding(1, 2)

	inner := width - box.GetHorizontalFrameSize()
	title := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(m.Title)
	body := lipgloss.NewStyle().Foreground(s.T.Text).Width(inner).Render(m.Body)

	content := title + "\n\n" + body
	if buttons != "" {
		content += "\n\n" + buttons
	}
	return box.Width(inner).Render(content)
}

// ConfirmButtons renders a yes/no pair; the active one is highlighted.
func ConfirmButtons(s *Styles, yesLabel, noLabel string, yesActive, danger bool) string {
	accent := s.T.Accent
	if danger {
		accent = s.T.Error
	}
	on := lipgloss.NewStyle().Foreground(s.T.Crust).Background(accent).Bold(true).Padding(0, 2)
	off := lipgloss.NewStyle().Foreground(s.T.Subtext).Background(s.T.Surface).Padding(0, 2)

	yes, no := off, on
	if yesActive {
		yes, no = on, off
	}
	return zone.Mark("modal:yes", yes.Render(yesLabel)) + "  " +
		zone.Mark("modal:no", no.Render(noLabel))
}

// HelpSection is one titled group of key bindings in the help overlay.
type HelpSection struct {
	Title string
	Hints []Hint
}

func RenderHelp(s *Styles, width, height int, sections []HelpSection) string {
	cols := 2
	if width < 80 {
		cols = 1
	}
	colWidth := (width - 8) / cols
	if colWidth < 24 {
		colWidth = 24
	}

	blocks := make([]string, 0, len(sections))
	for _, sec := range sections {
		var b strings.Builder
		b.WriteString(s.Accent.Bold(true).Render(sec.Title) + "\n")
		for _, h := range sec.Hints {
			key := lipgloss.NewStyle().Foreground(s.T.Warning).Width(12).Render(h.Key)
			b.WriteString(key + s.Subtext.Render(h.Desc) + "\n")
		}
		blocks = append(blocks, lipgloss.NewStyle().Width(colWidth).Render(b.String()))
	}

	rows := make([]string, 0, (len(blocks)+cols-1)/cols)
	for i := 0; i < len(blocks); i += cols {
		end := i + cols
		if end > len(blocks) {
			end = len(blocks)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, blocks[i:end]...))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.T.BorderFocused).
		Background(s.T.Mantle).
		Padding(1, 2)

	content := s.Bold.Render("Keyboard shortcuts") + "\n\n" +
		strings.Join(rows, "\n") + "\n" +
		s.Dim.Render("? or Esc to close")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box.Render(content))
}
