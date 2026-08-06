package screens

import (
	tea "charm.land/bubbletea/v2"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/ui/msgs"
)

func toast(text string, level models.LogLevel) tea.Cmd {
	return func() tea.Msg { return msgs.ToastMsg{Text: text, Level: level} }
}

func errToast(title string, err error) tea.Cmd {
	return func() tea.Msg { return msgs.ErrorMsg{Title: title, Detail: err.Error()} }
}
