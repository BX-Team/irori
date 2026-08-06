package screens

import (
	tea "charm.land/bubbletea/v2"
	"github.com/bx-team/irori/internal/ui/components"
)

type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View() string
	SetSize(width, height int)

	// CapturesInput reports that a text field has focus, so the shell must not
	// steal bare letter keys for global shortcuts.
	CapturesInput() bool
	Hints() []components.Hint
}

type Base struct {
	Width  int
	Height int
	S      *components.Styles
}

func (b *Base) SetSize(w, h int) { b.Width, b.Height = w, h }

func (b *Base) CapturesInput() bool { return false }

func (b *Base) Init() tea.Cmd { return nil }
