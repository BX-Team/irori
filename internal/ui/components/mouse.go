package components

import tea "charm.land/bubbletea/v2"

func WheelUp(m tea.MouseMsg) bool {
	w, ok := m.(tea.MouseWheelMsg)
	return ok && w.Button == tea.MouseWheelUp
}

func WheelDown(m tea.MouseMsg) bool {
	w, ok := m.(tea.MouseWheelMsg)
	return ok && w.Button == tea.MouseWheelDown
}

func LeftClick(m tea.MouseMsg) bool {
	c, ok := m.(tea.MouseClickMsg)
	return ok && c.Button == tea.MouseLeft
}
