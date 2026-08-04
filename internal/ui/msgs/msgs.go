package msgs

import (
	"github.com/bx-team/irori/internal/models"
	tea "github.com/charmbracelet/bubbletea"
)

type LogMsg struct{ Line models.LogLine }

type HistoryMsg struct{ Lines []models.LogLine }

type StatusMsg struct{ Status models.Status }

type StatsMsg struct{ Stats models.Stats }

// LinkDownMsg means the daemon socket went away: either it exited cleanly after
// the server stopped, or it was never there.
type LinkDownMsg struct{ Err error }

type LinkUpMsg struct{}

type ErrorMsg struct {
	Title  string
	Detail string
}

type ToastMsg struct {
	Text  string
	Level models.LogLevel
}

type ConfigChangedMsg struct{}

type SwitchTabMsg struct{ ID string }

type PowerAction string

const (
	PowerStart   PowerAction = "start"
	PowerStop    PowerAction = "stop"
	PowerRestart PowerAction = "restart"
	PowerKill    PowerAction = "kill"
)

type PowerMsg struct{ Action PowerAction }

type SendCommandMsg struct{ Text string }

// ConfirmMsg asks the shell to show a modal; OnYes is emitted when confirmed.
type ConfirmMsg struct {
	Title  string
	Body   string
	OnYes  tea.Msg
	Danger bool
}

// OpenEditorMsg asks the shell to suspend the TUI and run $EDITOR on a file.
type OpenEditorMsg struct{ Path string }
