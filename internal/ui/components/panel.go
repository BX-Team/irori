package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const panelPadX = 1

// Panel draws a rounded box whose top border carries the title and an optional
// right-aligned badge. The top border is composed by hand rather than letting
// lipgloss draw it, so the title can sit inside the line.
type Panel struct {
	Title   string
	Badge   string
	Focused bool
	Width   int
	Height  int
}

func (p Panel) ContentWidth() int {
	w := p.Width - 2 - panelPadX*2
	if w < 1 {
		return 1
	}
	return w
}

func (p Panel) ContentHeight() int {
	h := p.Height - 2
	if h < 1 {
		return 1
	}
	return h
}

func (p Panel) Render(content string, s *Styles) string {
	if p.Width < 4 || p.Height < 3 {
		return ""
	}

	border := lipgloss.RoundedBorder()
	borderStyle := s.PanelBorder
	titleStyle := s.PanelTitle
	if p.Focused {
		borderStyle = s.PanelFocused
		titleStyle = s.PanelTitleOn
	}

	// Width()/Height() cover padding but not the border, so declare the box
	// minus the border and hand in content already sized to ContentWidth.
	body := lipgloss.NewStyle().
		Border(border, false, true, true, true).
		BorderForeground(borderStyle.GetForeground()).
		Padding(0, panelPadX).
		Width(p.Width - 2).
		Height(p.ContentHeight()).
		MaxHeight(p.Height - 1).
		Render(content)

	return p.topBorder(s, borderStyle, titleStyle) + "\n" + body
}

func (p Panel) topBorder(s *Styles, borderStyle, titleStyle lipgloss.Style) string {
	inner := p.Width - 2

	var left string
	if p.Title != "" {
		left = " " + titleStyle.Render(p.Title) + " "
	}
	var right string
	if p.Badge != "" {
		right = " " + s.Muted.Render(p.Badge) + " "
	}

	fill := inner - 1 - lipgloss.Width(left) - lipgloss.Width(right)
	if fill < 0 {
		fill = 0
		right = ""
		if over := inner - 1 - lipgloss.Width(left); over < 0 {
			left = ""
		}
	}

	line := borderStyle.Render("╭─") + left +
		borderStyle.Render(strings.Repeat("─", fill)) + right +
		borderStyle.Render("╮")
	return line
}
