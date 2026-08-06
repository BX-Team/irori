package wizard

import (
	"errors"
	"fmt"

	"github.com/bx-team/irori/internal/launch"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var formLabels = []string{"Server name", "Initial heap (Xms)", "Maximum heap (Xmx)"}

func (m *Model) buildForm() {
	values := []string{m.cfg.Server.Name, m.cfg.Java.Xms, m.cfg.Java.Xmx}
	placeholders := []string{"survival", "2G", "4G"}

	m.fields = make([]textinput.Model, len(values))
	for i := range m.fields {
		in := textinput.New()
		in.Prompt = ""
		in.SetValue(values[i])
		in.Placeholder = placeholders[i]
		in.TextStyle = m.s.Text
		in.PlaceholderStyle = m.s.Dim
		in.Width = 24
		m.fields[i] = in
	}
	m.field = 0
	m.fields[0].Focus()
}

func (m *Model) formKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "tab", "down":
		m.focusField(m.field + 1)
		return nil
	case "shift+tab", "up":
		m.focusField(m.field - 1)
		return nil
	case "enter":
		if m.field < len(m.fields)-1 {
			m.focusField(m.field + 1)
			return nil
		}
		if err := m.applyForm(); err != nil {
			m.err = err.Error()
			return nil
		}
		m.step = stepPreset
		m.showPresets()
		return nil
	}

	var cmd tea.Cmd
	m.fields[m.field], cmd = m.fields[m.field].Update(k)
	return cmd
}

func (m *Model) focusField(i int) {
	if i < 0 {
		i = len(m.fields) - 1
	}
	i %= len(m.fields)
	m.fields[m.field].Blur()
	m.field = i
	m.fields[i].Focus()
}

func (m *Model) applyForm() error {
	name := m.fields[0].Value()
	xms, xmx := m.fields[1].Value(), m.fields[2].Value()

	xmsMB, err := launch.ParseMemMB(xms)
	if err != nil {
		return fmt.Errorf("invalid Xms: %w", err)
	}
	xmxMB, err := launch.ParseMemMB(xmx)
	if err != nil {
		return fmt.Errorf("invalid Xmx: %w", err)
	}
	if xmsMB > xmxMB {
		return errors.New("initial heap (Xms) cannot exceed the maximum (Xmx)")
	}
	if xmxMB < 512 {
		return errors.New("maximum heap (Xmx) below 512M — the server will not come up")
	}

	m.cfg.Server.Name = name
	m.cfg.Java.Xms = launch.FormatMemMB(xmsMB)
	m.cfg.Java.Xmx = launch.FormatMemMB(xmxMB)
	return nil
}

func (m *Model) showPresets() {
	xmxMB, _ := launch.ParseMemMB(m.cfg.Java.Xmx)
	items := make([]components.ListItem, 0, 4)
	for _, p := range launch.Presets() {
		badge := ""
		if n := len(p.AllFlags(xmxMB)); n > 0 {
			badge = fmt.Sprintf("up to %d flags", n)
		}
		items = append(items, components.ListItem{
			ID: p.ID, Title: p.Name, Desc: p.Summary, Badge: badge,
		})
	}
	m.list.ShowDesc = true
	m.list.Reset()
	m.list.SetItems(items)
	m.list.SelectID(m.cfg.Java.Preset)
}
