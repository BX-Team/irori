package components

import (
	"regexp"
	"strings"

	"github.com/bx-team/irori/internal/models"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	prefixRe  = regexp.MustCompile(`^(\[[^\]]*\](?:\s*\[[^\]]*\])?:?)(.*)$`)
	sectionRe = regexp.MustCompile("§.")
)

type Console struct {
	vp viewport.Model
	s  *Styles

	lines    []models.LogLine
	rendered []string
	max      int

	width      int
	autoScroll bool
	filter     string
}

func NewConsole(s *Styles, max int) *Console {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	return &Console{vp: vp, s: s, max: max, autoScroll: true}
}

func (c *Console) SetSize(w, h int) {
	reflow := w != c.width
	c.width = w
	c.vp.Width, c.vp.Height = w, h
	if reflow {
		c.reflow()
		return
	}
	c.sync()
}

func (c *Console) Append(l models.LogLine) {
	c.lines = append(c.lines, l)
	if len(c.lines) > c.max {
		c.lines = c.lines[len(c.lines)-c.max:]
		c.reflow()
	} else if c.matches(l) {
		c.rendered = append(c.rendered, c.render(l)...)
		c.sync()
	}
}

func (c *Console) SetLines(lines []models.LogLine) {
	if len(lines) > c.max {
		lines = lines[len(lines)-c.max:]
	}
	c.lines = append([]models.LogLine(nil), lines...)
	c.reflow()
}

func (c *Console) Clear() {
	c.lines = nil
	c.reflow()
}

func (c *Console) Lines() []models.LogLine { return c.lines }

func (c *Console) SetFilter(q string) {
	c.filter = strings.ToLower(strings.TrimSpace(q))
	c.reflow()
}

func (c *Console) Filter() string { return c.filter }

func (c *Console) AutoScroll() bool { return c.autoScroll }

func (c *Console) matches(l models.LogLine) bool {
	return c.filter == "" || strings.Contains(strings.ToLower(l.Text), c.filter)
}

func (c *Console) reflow() {
	c.rendered = c.rendered[:0]
	for _, l := range c.lines {
		if c.matches(l) {
			c.rendered = append(c.rendered, c.render(l)...)
		}
	}
	c.sync()
}

func (c *Console) sync() {
	content := c.rendered
	if pad := c.vp.Height - len(content); pad > 0 {
		content = append(make([]string, pad), content...)
	}
	c.vp.SetContent(strings.Join(content, "\n"))
	if c.autoScroll {
		c.vp.GotoBottom()
	}
}

func (c *Console) render(l models.LogLine) []string {
	text := sectionRe.ReplaceAllString(l.Text, "")
	if text == "" {
		return []string{""}
	}

	body := lipgloss.NewStyle().Foreground(c.levelColor(l.Level))
	width := c.width
	if width < 8 {
		width = 8
	}

	prefix, rest := "", text
	if m := prefixRe.FindStringSubmatch(text); m != nil && l.Level != models.LevelIrori {
		prefix, rest = m[1], m[2]
	}

	wrapped := strings.Split(ansi.Wrap(prefix+rest, width, " -/"), "\n")
	out := make([]string, 0, len(wrapped))
	for i, row := range wrapped {
		if i == 0 && prefix != "" && len(row) >= len(prefix) {
			out = append(out, c.s.Time.Render(row[:len(prefix)])+body.Render(row[len(prefix):]))
			continue
		}
		out = append(out, body.Render(row))
	}
	return out
}

func (c *Console) levelColor(l models.LogLevel) lipgloss.Color {
	switch l {
	case models.LevelWarn:
		return c.s.T.LogWarn
	case models.LevelError:
		return c.s.T.LogError
	case models.LevelDebug:
		return c.s.T.LogDebug
	case models.LevelChat:
		return c.s.T.LogChat
	case models.LevelIrori:
		return c.s.T.Secondary
	default:
		return c.s.T.LogInfo
	}
}

func (c *Console) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.vp, cmd = c.vp.Update(msg)
	c.autoScroll = c.vp.AtBottom()
	return cmd
}

func (c *Console) GotoBottom() {
	c.vp.GotoBottom()
	c.autoScroll = true
}

func (c *Console) GotoTop() {
	c.vp.GotoTop()
	c.autoScroll = false
}

func (c *Console) View() string { return c.vp.View() }

func (c *Console) ScrollPercent() int { return int(c.vp.ScrollPercent() * 100) }
