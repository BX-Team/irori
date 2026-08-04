package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func SpaceBetween(left, right string, width int) string {
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	if lw+rw+1 > width {
		if rw+2 > width {
			return ansi.Truncate(left, width, "…")
		}
		left = ansi.Truncate(left, width-rw-1, "…")
		lw = lipgloss.Width(left)
	}
	gap := width - lw - rw
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func Bar(left, right string, width int, s *Styles) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.T.Border).
		Background(s.T.Surface).
		Foreground(s.T.Text).
		Padding(0, 1)

	// lipgloss counts padding inside Width() but adds borders outside it, so the
	// declared width must exclude only the border.
	inner := width - style.GetHorizontalFrameSize()
	if inner < 1 {
		inner = 1
	}
	return style.Width(width - 2).Render(SpaceBetween(left, right, inner))
}

type Hint struct {
	Key  string
	Desc string
}

func Hints(s *Styles, hints ...Hint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, s.Key.Render(h.Key)+" "+s.Desc.Render(h.Desc))
	}
	return strings.Join(parts, s.Sep.Render(" │ "))
}

func Dot(s *Styles, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render("●")
}
