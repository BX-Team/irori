package screens

import (
	"github.com/bx-team/irori/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Placeholder struct {
	Base
	title string
	note  string
}

func NewPlaceholder(s *components.Styles, title, note string) *Placeholder {
	p := &Placeholder{title: title, note: note}
	p.S = s
	return p
}

func (p *Placeholder) Update(tea.Msg) (Screen, tea.Cmd) { return p, nil }

func (p *Placeholder) Hints() []components.Hint { return nil }

func (p *Placeholder) View() string {
	panel := components.Panel{Title: p.title, Width: p.Width, Height: p.Height}
	body := lipgloss.Place(
		panel.ContentWidth(), panel.ContentHeight(),
		lipgloss.Center, lipgloss.Center,
		p.S.Muted.Render(p.note),
	)
	return panel.Render(body, p.S)
}
