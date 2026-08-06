package components

import (
	"strconv"
	"strings"

	zone "github.com/lrstanley/bubblezone/v2"
)

type Tab struct {
	ID    string
	Label string
}

func RenderTabs(tabs []Tab, active, maxWidth int, s *Styles) string {
	full := renderTabs(tabs, active, s, false)
	if lipglossWidth(full) <= maxWidth {
		return full
	}
	return renderTabs(tabs, active, s, true)
}

func renderTabs(tabs []Tab, active int, s *Styles, compact bool) string {
	parts := make([]string, 0, len(tabs))
	for i, t := range tabs {
		label := strconv.Itoa(i + 1)
		if !compact {
			label += " " + t.Label
		}
		style := s.Tab
		if i == active {
			style = s.TabActive
		}
		parts = append(parts, zone.Mark("tab:"+t.ID, style.Render(label)))
	}
	return strings.Join(parts, "")
}
