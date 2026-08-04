package components

import (
	"github.com/bx-team/irori/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Styles is built once per theme and shared by every component; nothing here
// may be constructed inside a render loop.
type Styles struct {
	T theme.Theme

	Text    lipgloss.Style
	Subtext lipgloss.Style
	Muted   lipgloss.Style
	Accent  lipgloss.Style
	Bold    lipgloss.Style

	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style

	PanelBorder  lipgloss.Style
	PanelFocused lipgloss.Style
	PanelTitle   lipgloss.Style
	PanelTitleOn lipgloss.Style

	Bar       lipgloss.Style
	AppBadge  lipgloss.Style
	Tab       lipgloss.Style
	TabActive lipgloss.Style

	Key  lipgloss.Style
	Desc lipgloss.Style
	Sep  lipgloss.Style

	Selected  lipgloss.Style
	Cursor    lipgloss.Style
	Highlight lipgloss.Style

	Label lipgloss.Style
	Value lipgloss.Style
	Dim   lipgloss.Style
}

func NewStyles(t theme.Theme) *Styles {
	base := lipgloss.NewStyle()
	return &Styles{
		T: t,

		Text:    base.Foreground(t.Text),
		Subtext: base.Foreground(t.Subtext),
		Muted:   base.Foreground(t.Muted),
		Accent:  base.Foreground(t.Accent),
		Bold:    base.Foreground(t.Text).Bold(true),

		Success: base.Foreground(t.Success),
		Warning: base.Foreground(t.Warning),
		Error:   base.Foreground(t.Error),
		Info:    base.Foreground(t.Info),

		PanelBorder:  base.Foreground(t.Border),
		PanelFocused: base.Foreground(t.BorderFocused),
		PanelTitle:   base.Foreground(t.Subtext).Bold(true),
		PanelTitleOn: base.Foreground(t.BorderFocused).Bold(true),

		Bar:       base.Background(t.Surface).Foreground(t.Text),
		AppBadge:  base.Background(t.Secondary).Foreground(t.Crust).Bold(true).Padding(0, 1),
		Tab:       base.Foreground(t.Muted).Padding(0, 1),
		TabActive: base.Foreground(t.Crust).Background(t.Accent).Bold(true).Padding(0, 1),

		Key:  base.Foreground(t.Accent).Bold(true),
		Desc: base.Foreground(t.Muted),
		Sep:  base.Foreground(t.Border),

		Selected:  base.Background(t.Selection).Foreground(t.Text),
		Cursor:    base.Background(t.Accent).Foreground(t.Crust).Bold(true),
		Highlight: base.Foreground(t.Warning).Bold(true),

		Label: base.Foreground(t.Muted),
		Value: base.Foreground(t.Text),
		Dim:   base.Foreground(t.Muted),
	}
}

func (s *Styles) StateColor(label string) lipgloss.Color {
	switch label {
	case "RUNNING":
		return s.T.Running
	case "STARTING":
		return s.T.Starting
	case "STOPPING":
		return s.T.Stopping
	case "CRASHED":
		return s.T.Crashed
	default:
		return s.T.Stopped
	}
}
