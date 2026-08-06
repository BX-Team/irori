package components

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
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

	// Width() is the whole box in lipgloss v2, border included; the frame size
	// is what has to come off for the text laid out inside it.
	inner := width - style.GetHorizontalFrameSize()
	if inner < 1 {
		inner = 1
	}
	return style.Width(width).Render(SpaceBetween(left, right, inner))
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

func Dot(s *Styles, c color.Color) string {
	return lipgloss.NewStyle().Foreground(c).Render("●")
}
