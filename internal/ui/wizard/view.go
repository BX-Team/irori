package wizard

import (
	"fmt"
	"strings"

	"github.com/bx-team/irori/internal/ui/components"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

func (m *Model) listHeight() int {
	h := m.height - 14
	if h < 4 {
		h = 4
	}
	if h > 18 {
		h = 18
	}
	return h
}

func (m *Model) showTypes() {
	items := make([]components.ListItem, 0, len(m.types))
	for _, t := range m.types {
		badge := fmt.Sprintf("%d builds", t.Builds)
		marker := ""
		switch {
		case t.Deprecated:
			marker = "⚠"
		case t.Experimental:
			marker = "β"
		case t.Category == "recommended":
			marker = "★"
		}
		items = append(items, components.ListItem{
			ID: string(t.ID), Title: t.Name, Desc: t.Description,
			Badge: badge, Marker: marker, Disabled: t.Deprecated,
		})
	}
	m.list.ShowDesc = true
	m.list.Reset()
	m.list.SetItems(items)
}

func (m *Model) showVersions() {
	items := make([]components.ListItem, 0, len(m.versions))
	for _, v := range m.versions {
		desc := fmt.Sprintf("%s · %d builds · Java %d", strings.ToLower(v.Type), v.Builds, v.Java)
		badge := ""
		if !v.Supported {
			badge = "unsupported"
		}
		items = append(items, components.ListItem{
			ID: v.ID, Title: v.ID, Desc: desc, Badge: badge, Disabled: !v.Supported,
		})
	}
	m.list.ShowDesc = true
	m.list.Reset()
	m.list.SetItems(items)
}

func (m *Model) showBuilds() {
	items := make([]components.ListItem, 0, len(m.builds))
	for i, b := range m.builds {
		desc := "no changelog"
		if len(b.Changes) > 0 {
			desc = b.Changes[0]
		}
		badge := ""
		if i == 0 {
			badge = "latest"
		}
		if b.Experimental {
			badge = "experimental"
		}
		items = append(items, components.ListItem{
			ID: b.UUID, Title: b.Name, Desc: desc, Badge: badge,
		})
	}
	m.list.ShowDesc = true
	m.list.Reset()
	m.list.SetItems(items)
}

func (m *Model) showCandidates() {
	items := make([]components.ListItem, 0, len(m.candidates))
	for _, c := range m.candidates {
		marker := "?"
		if c.Identified() {
			marker = "✓"
		}
		items = append(items, components.ListItem{
			ID: c.File, Title: c.File, Desc: c.Describe(),
			Badge: components.HumanBytes(c.Size), Marker: marker,
		})
	}
	m.list.ShowDesc = true
	m.list.Reset()
	m.list.SetItems(items)
}

func (m *Model) View() string {
	if m.quitting || m.done {
		return ""
	}
	if m.width == 0 {
		return ""
	}

	width := min(cardWidth, m.width-4)
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.s.T.BorderFocused).
		Padding(1, 2)
	inner := width - card.GetHorizontalFrameSize()

	content := strings.Join([]string{
		m.header(inner),
		"",
		m.body(inner),
		"",
		m.footer(inner),
	}, "\n")

	return zone.Scan(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		card.Width(width-2).Render(content)))
}

func (m *Model) header(width int) string {
	titles := map[step]string{
		stepMode:    "Server setup",
		stepType:    "Server core",
		stepVersion: "Minecraft version",
		stepBuild:   "Build",
		stepScan:    "Scanning",
		stepPickJar: "Detected cores",
		stepForm:    "Basics",
		stepPreset:  "Launch flags",
		stepEula:    "License agreement",
		stepInstall: "Installing",
	}
	left := m.s.AppBadge.Render("irori") + " " + m.s.Bold.Render(titles[m.step])
	return components.SpaceBetween(left, m.s.Muted.Render(m.breadcrumb()), width)
}

func (m *Model) breadcrumb() string {
	var parts []string
	if m.selType != "" {
		parts = append(parts, m.selType.Display())
	}
	if m.selVersion.ID != "" {
		parts = append(parts, m.selVersion.ID)
	}
	if m.selBuild != nil {
		parts = append(parts, m.selBuild.Name)
	}
	if len(parts) == 0 {
		return m.dir
	}
	return strings.Join(parts, " › ")
}

func (m *Model) body(width int) string {
	if m.loading {
		return m.spin.View() + m.s.Muted.Render(" loading…")
	}

	switch m.step {
	case stepForm:
		return m.viewForm(width)
	case stepEula:
		return m.viewEula(width)
	case stepInstall:
		return m.viewInstall(width)
	}

	var head string
	if m.filterable() && (m.filter.Focused() || m.filter.Value() != "") {
		head = m.filter.View() + "\n\n"
	}

	m.list.SetSize(width, m.listHeight()-lipgloss.Height(head)+1)
	body := head + m.list.View()
	if m.step == stepPickJar && m.script != nil {
		body += "\n\n" + m.s.Success.Render("↳ found "+m.script.Path) + "\n" +
			m.s.Muted.Render(fmt.Sprintf("   imported: Xms %s, Xmx %s, preset %s, %d extra flags",
				orDash(m.script.Xms), orDash(m.script.Xmx), m.script.Preset, len(m.script.ExtraFlags)))
	}
	return body
}

func (m *Model) viewForm(width int) string {
	rows := make([]string, 0, len(m.fields)*2)
	for i, f := range m.fields {
		label := m.s.Label.Render(formLabels[i])
		if i == m.field {
			label = m.s.Accent.Render("▸ " + formLabels[i])
		} else {
			label = "  " + label
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.s.T.Border).
			Padding(0, 1)
		if i == m.field {
			box = box.BorderForeground(m.s.T.BorderFocused)
		}
		rows = append(rows, label, box.Width(30).Render(f.View()))
	}

	summary := fmt.Sprintf("%s %s · jar %s",
		m.cfg.Server.Type.Display(), orDash(m.cfg.Server.MCVersion), m.cfg.Server.Jar)
	rows = append(rows, "", m.s.Muted.Render(components.Truncate(summary, width)))
	return strings.Join(rows, "\n")
}

func (m *Model) viewEula(width int) string {
	return strings.Join([]string{
		m.s.Text.Render("Running a server requires accepting the Minecraft EULA:"),
		m.s.Accent.Render("https://aka.ms/MinecraftEULA"),
		"",
		lipgloss.NewStyle().Width(width).Foreground(m.s.T.Subtext).Render(
			"irori will write eula=true into eula.txt. By accepting you agree to Mojang's terms."),
	}, "\n")
}

func (m *Model) viewInstall(width int) string {
	p := m.progress
	head := fmt.Sprintf("step %d/%d · %s", p.Step, p.Total, p.Label)

	rows := []string{m.spin.View() + m.s.Text.Render(" "+components.Truncate(head, width-3))}
	if p.Size > 0 {
		ratio := float64(p.Downloaded) / float64(p.Size)
		bar := components.ProgressBar(ratio, width-12,
			lipgloss.NewStyle().Foreground(m.s.T.Accent),
			lipgloss.NewStyle().Foreground(m.s.T.Border))
		rows = append(rows, "", bar+m.s.Muted.Render(fmt.Sprintf(" %3.0f%%", ratio*100)),
			m.s.Muted.Render(fmt.Sprintf("%s of %s",
				components.HumanBytes(p.Downloaded), components.HumanBytes(p.Size))))
	}
	return strings.Join(rows, "\n")
}

func (m *Model) footer(width int) string {
	if m.err != "" {
		return m.s.Error.Render("✖ " + components.Truncate(m.err, width-2))
	}

	if m.filter.Focused() {
		return components.Hints(m.s,
			components.Hint{Key: "Enter", Desc: "back to list"},
			components.Hint{Key: "Esc", Desc: "clear filter"})
	}

	var hints []components.Hint
	switch m.step {
	case stepForm:
		hints = []components.Hint{
			{Key: "Tab", Desc: "next field"}, {Key: "Enter", Desc: "continue"}, {Key: "Esc", Desc: "back"},
		}
	case stepEula:
		hints = []components.Hint{{Key: "y", Desc: "accept"}, {Key: "n", Desc: "quit"}}
	case stepInstall:
		hints = []components.Hint{{Key: "Ctrl+C", Desc: "abort"}}
	default:
		hints = []components.Hint{
			{Key: "↑↓", Desc: "select"}, {Key: "Enter", Desc: "continue"}, {Key: "Esc", Desc: "back"},
		}
		if m.filterable() {
			hints = append(hints, components.Hint{Key: "/", Desc: "filter"})
		}
	}
	return components.Hints(m.s, hints...)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
