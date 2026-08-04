package screens

import (
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/ui/msgs"
	tea "github.com/charmbracelet/bubbletea"
)

func toast(text string, level models.LogLevel) tea.Cmd {
	return func() tea.Msg { return msgs.ToastMsg{Text: text, Level: level} }
}

func errToast(title string, err error) tea.Cmd {
	return func() tea.Msg { return msgs.ErrorMsg{Title: title, Detail: err.Error()} }
}
