package irori_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/theme"
)

func TestPanelRendersAtItsDeclaredWidth(t *testing.T) {
	s := components.NewStyles(theme.GetTheme("catppuccin"))

	for _, width := range []int{20, 33, 40, 80, 120} {
		p := components.Panel{Title: "Console", Width: width, Height: 6}

		if got := lipgloss.Width(p.Render("", s)); got != width {
			t.Errorf("empty panel of width %d rendered %d columns", width, got)
		}

		fitted := strings.Repeat("x", p.ContentWidth())
		if got := lipgloss.Width(p.Render(fitted, s)); got != width {
			t.Errorf("panel of width %d with ContentWidth() content rendered %d columns", width, got)
		}

		overflowing := strings.Repeat("x", width*2)
		if got := lipgloss.Width(p.Render(overflowing, s)); got != width {
			t.Errorf("panel of width %d with oversized content rendered %d columns", width, got)
		}
	}
}

// The badge shares the top border with the title; a long pair must still not
// push the border wider than the panel.
func TestPanelTitleAndBadgeDoNotWidenTheBorder(t *testing.T) {
	s := components.NewStyles(theme.GetTheme("catppuccin"))
	p := components.Panel{
		Title:  "a very long panel title that will not fit",
		Badge:  "1234/5678 items",
		Width:  30,
		Height: 4,
	}

	if got := lipgloss.Width(p.Render("body", s)); got != 30 {
		t.Errorf("panel with an oversized title rendered %d columns, want 30", got)
	}
}
