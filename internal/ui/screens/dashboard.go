package screens

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/launch"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/props"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/msgs"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

const (
	sideWidth   = 34
	minSideCols = 76
	inputRows   = 3
	histLen     = 40
)

type dashFocus int

const (
	focusInput dashFocus = iota
	focusConsole
)

type powerButton struct {
	id     string
	label  string
	action msgs.PowerAction
	danger bool
}

var powerButtons = []powerButton{
	{"start", "▶ Start", msgs.PowerStart, false},
	{"stop", "■ Stop", msgs.PowerStop, false},
	{"restart", "⟳ Restart", msgs.PowerRestart, false},
	{"kill", "⨯ Kill", msgs.PowerKill, true},
}

type Dashboard struct {
	Base
	cfg     *config.Config
	console *components.Console
	input   textinput.Model
	search  textinput.Model

	status  models.Status
	linked  bool
	focus   dashFocus
	cpuHist []float64
	ramHist []float64
	xmxMB   int

	history  []string
	histIdx  int
	draft    string
	seenCmds map[string]bool
	port     string
}

func serverPort(cfg *config.Config) string {
	f, err := props.Load(filepath.Join(cfg.Dir(), "server.properties"))
	if err != nil {
		return "—"
	}
	if cfg.Server.Type.IsProxy() {
		if v, ok := f.Get("bind"); ok {
			return v
		}
	}
	return f.GetOr("server-port", "25565")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func NewDashboard(s *components.Styles, cfg *config.Config, scrollback int) *Dashboard {
	in := textinput.New()
	in.Prompt = "> "
	in.Placeholder = "server command"
	in.PromptStyle = s.Accent
	in.TextStyle = s.Text
	in.PlaceholderStyle = s.Dim
	in.Focus()

	se := textinput.New()
	se.Prompt = "/ "
	se.Placeholder = "filter console"
	se.PromptStyle = s.Warning
	se.PlaceholderStyle = s.Dim

	xmx, _ := launch.ParseMemMB(cfg.Java.Xmx)
	d := &Dashboard{
		cfg:      cfg,
		console:  components.NewConsole(s, scrollback),
		input:    in,
		search:   se,
		status:   models.Status{State: models.StateStopped},
		xmxMB:    xmx,
		histIdx:  -1,
		seenCmds: map[string]bool{},
		port:     serverPort(cfg),
	}
	d.S = s
	return d
}

func (d *Dashboard) CapturesInput() bool {
	return d.focus == focusInput || d.search.Focused()
}

func (d *Dashboard) Console() *components.Console { return d.console }

func (d *Dashboard) SetSize(w, h int) {
	d.Base.SetSize(w, h)
	cw, ch := d.consoleBox()
	d.console.SetSize(cw.ContentWidth(), ch)
	d.input.Width = cw.ContentWidth() - 3
	d.search.Width = cw.ContentWidth() - 3
}

func (d *Dashboard) sideVisible() bool { return d.Width >= minSideCols }

func (d *Dashboard) consoleBox() (components.Panel, int) {
	w := d.Width
	if d.sideVisible() {
		w -= sideWidth
	}
	p := components.Panel{Width: w, Height: d.Height - inputRows}
	return p, p.ContentHeight()
}

func (d *Dashboard) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case msgs.ConfigChangedMsg:
		d.port = serverPort(d.cfg)
		d.xmxMB, _ = launch.ParseMemMB(d.cfg.Java.Xmx)
		return d, nil

	case msgs.StatusMsg:
		d.status = m.Status
		return d, nil

	case msgs.StatsMsg:
		d.status.Stats = m.Stats
		d.cpuHist = pushHist(d.cpuHist, m.Stats.CPU)
		d.ramHist = pushHist(d.ramHist, m.Stats.RSSMB)
		return d, nil

	case msgs.HistoryMsg:
		d.console.SetLines(m.Lines)
		return d, nil

	case msgs.LogMsg:
		d.console.Append(m.Line)
		return d, nil

	case msgs.LinkUpMsg:
		d.linked = true
		return d, nil

	case msgs.LinkDownMsg:
		d.linked = false
		d.status = models.Status{State: d.status.State}
		if d.status.State.IsUp() {
			d.status.State = models.StateStopped
		}
		return d, nil

	case tea.MouseMsg:
		return d, d.handleMouse(m)

	case tea.KeyMsg:
		return d, d.handleKey(m)
	}

	return d, d.console.Update(msg)
}

func (d *Dashboard) handleMouse(m tea.MouseMsg) tea.Cmd {
	if m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft {
		for _, b := range powerButtons {
			if zone.Get("pw:" + b.id).InBounds(m) {
				return d.power(b)
			}
		}
		if zone.Get("dash:console").InBounds(m) {
			d.focus = focusConsole
			d.input.Blur()
			return nil
		}
		if zone.Get("dash:input").InBounds(m) {
			d.focus = focusInput
			return d.input.Focus()
		}
	}
	return d.console.Update(m)
}

func (d *Dashboard) handleKey(k tea.KeyMsg) tea.Cmd {
	if d.search.Focused() {
		switch k.String() {
		case "esc":
			d.search.SetValue("")
			d.search.Blur()
			d.console.SetFilter("")
			return nil
		case "enter":
			d.search.Blur()
			return nil
		}
		var cmd tea.Cmd
		d.search, cmd = d.search.Update(k)
		d.console.SetFilter(d.search.Value())
		return cmd
	}

	switch k.String() {
	case "tab":
		if d.focus == focusInput {
			d.focus = focusConsole
			d.input.Blur()
			return nil
		}
		d.focus = focusInput
		return d.input.Focus()
	case "ctrl+l":
		d.console.Clear()
		return nil
	}

	if d.focus == focusInput {
		return d.inputKey(k)
	}
	return d.consoleKey(k)
}

func (d *Dashboard) inputKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "enter":
		text := strings.TrimSpace(d.input.Value())
		if text == "" {
			return nil
		}
		d.remember(text)
		d.input.SetValue("")
		d.histIdx = -1
		d.console.GotoBottom()
		return func() tea.Msg { return msgs.SendCommandMsg{Text: text} }

	case "up":
		return d.recall(1)
	case "down":
		return d.recall(-1)

	case "tab", "ctrl+i":
		if c := d.complete(d.input.Value()); c != "" {
			d.input.SetValue(c)
			d.input.CursorEnd()
		}
		return nil

	case "esc":
		d.focus = focusConsole
		d.input.Blur()
		return nil

	case "pgup", "pgdown":
		return d.console.Update(k)
	}

	var cmd tea.Cmd
	d.input, cmd = d.input.Update(k)
	return cmd
}

func (d *Dashboard) consoleKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "i", "enter":
		d.focus = focusInput
		return d.input.Focus()
	case "/":
		return d.search.Focus()
	case "G":
		d.console.GotoBottom()
		return nil
	case "g":
		d.console.GotoTop()
		return nil
	case "s":
		return d.power(powerButtons[0])
	case "x":
		return d.power(powerButtons[1])
	case "r":
		return d.power(powerButtons[2])
	case "k":
		return d.power(powerButtons[3])
	}
	return d.console.Update(k)
}

func (d *Dashboard) power(b powerButton) tea.Cmd {
	if !d.enabled(b) {
		return nil
	}
	action := b.action
	if b.danger {
		return func() tea.Msg {
			return msgs.ConfirmMsg{
				Title:  "Force kill",
				Body:   "The process will be killed with SIGKILL. Unsaved chunks and plugin data will be lost. Continue?",
				OnYes:  msgs.PowerMsg{Action: action},
				Danger: true,
			}
		}
	}
	return func() tea.Msg { return msgs.PowerMsg{Action: action} }
}

func (d *Dashboard) enabled(b powerButton) bool {
	up := d.status.State.IsUp()
	switch b.action {
	case msgs.PowerStart:
		return !up
	case msgs.PowerStop, msgs.PowerKill:
		return up
	case msgs.PowerRestart:
		return true
	}
	return false
}

func (d *Dashboard) remember(text string) {
	d.seenCmds[strings.Fields(text)[0]] = true
	if len(d.history) == 0 || d.history[0] != text {
		d.history = append([]string{text}, d.history...)
	}
	if len(d.history) > histLen {
		d.history = d.history[:histLen]
	}
}

func (d *Dashboard) recall(delta int) tea.Cmd {
	if len(d.history) == 0 {
		return nil
	}
	if d.histIdx == -1 && delta > 0 {
		d.draft = d.input.Value()
	}
	idx := d.histIdx + delta
	if idx < -1 {
		idx = -1
	}
	if idx >= len(d.history) {
		idx = len(d.history) - 1
	}
	d.histIdx = idx
	if idx == -1 {
		d.input.SetValue(d.draft)
	} else {
		d.input.SetValue(d.history[idx])
	}
	d.input.CursorEnd()
	return nil
}

func (d *Dashboard) complete(prefix string) string {
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix == "" || strings.Contains(prefix, " ") {
		return ""
	}
	var match string
	for cmd := range d.seenCmds {
		if strings.HasPrefix(cmd, prefix) {
			if match != "" {
				return ""
			}
			match = cmd
		}
	}
	if match == "" || match == prefix {
		return ""
	}
	return match + " "
}

func pushHist(h []float64, v float64) []float64 {
	h = append(h, v)
	if len(h) > 120 {
		h = h[len(h)-120:]
	}
	return h
}

func (d *Dashboard) Hints() []components.Hint {
	if d.search.Focused() {
		return []components.Hint{{Key: "Esc", Desc: "clear"}, {Key: "Enter", Desc: "keep filter"}}
	}
	if d.focus == focusInput {
		return []components.Hint{
			{Key: "Enter", Desc: "send"},
			{Key: "↑↓", Desc: "history"},
			{Key: "Tab", Desc: "to console"},
			{Key: "^L", Desc: "clear"},
		}
	}
	return []components.Hint{
		{Key: "s/x/r/k", Desc: "start/stop/restart/kill"},
		{Key: "j k", Desc: "scroll"},
		{Key: "/", Desc: "filter"},
		{Key: "Tab", Desc: "to input"},
	}
}

func (d *Dashboard) View() string {
	right := d.viewConsole()
	if !d.sideVisible() {
		return right
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, d.viewSide(), right)
}

func (d *Dashboard) viewSide() string {
	serverH := 11
	resH := d.Height - serverH
	if resH < 9 {
		resH = 9
		serverH = d.Height - resH
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		d.viewServerCard(serverH),
		d.viewResources(resH),
	)
}

func (d *Dashboard) viewServerCard(height int) string {
	p := components.Panel{Title: "Server", Width: sideWidth, Height: height, Focused: d.focus == focusConsole}
	s := d.S
	w := p.ContentWidth()

	stateLabel := d.status.State.Label()
	pill := lipgloss.NewStyle().
		Foreground(s.T.Crust).
		Background(s.StateColor(stateLabel)).
		Bold(true).Padding(0, 1).
		Render(stateLabel)
	if !d.linked {
		pill = lipgloss.NewStyle().
			Foreground(s.T.Crust).Background(s.T.Stopped).
			Bold(true).Padding(0, 1).Render("OFFLINE")
	}

	meta := d.cfg.Server.Type.Display()
	if d.cfg.Server.MCVersion != "" {
		meta += " · " + d.cfg.Server.MCVersion
	}
	if d.cfg.Server.Build != "" {
		meta += " · " + d.cfg.Server.Build
	}

	rows := []string{
		s.Bold.Render(components.Truncate(d.cfg.Server.Name, w)),
		s.Muted.Render(components.Truncate(meta, w)),
		"",
		pill,
		"",
		d.viewButtons(w),
	}
	return p.Render(components.PadLines(strings.Join(rows, "\n"), p.ContentHeight()), s)
}

func (d *Dashboard) viewButtons(width int) string {
	s := d.S
	cell := (width - 1) / 2

	render := func(b powerButton) string {
		style := lipgloss.NewStyle().Width(cell).Align(lipgloss.Center).Padding(0, 0)
		switch {
		case !d.enabled(b):
			style = style.Foreground(s.T.Muted).Background(s.T.Mantle)
		case b.danger:
			style = style.Foreground(s.T.Crust).Background(s.T.Error).Bold(true)
		default:
			style = style.Foreground(s.T.Crust).Background(s.T.Accent).Bold(true)
		}
		return zone.Mark("pw:"+b.id, style.Render(b.label))
	}

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, render(powerButtons[0]), " ", render(powerButtons[1]))
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, render(powerButtons[2]), " ", render(powerButtons[3]))
	return row1 + "\n" + row2
}

func (d *Dashboard) viewResources(height int) string {
	p := components.Panel{Title: "Resources", Width: sideWidth, Height: height}
	s := d.S
	w := p.ContentWidth()
	st := d.status.Stats

	fill := lipgloss.NewStyle().Foreground(s.T.Accent)
	empty := lipgloss.NewStyle().Foreground(s.T.Border)
	if st.CPU > 85 {
		fill = lipgloss.NewStyle().Foreground(s.T.Error)
	} else if st.CPU > 60 {
		fill = lipgloss.NewStyle().Foreground(s.T.Warning)
	}

	ramRatio := 0.0
	if d.xmxMB > 0 {
		ramRatio = st.RSSMB / float64(d.xmxMB)
	}
	ramFill := lipgloss.NewStyle().Foreground(s.T.Success)
	if ramRatio > 0.9 {
		ramFill = lipgloss.NewStyle().Foreground(s.T.Error)
	} else if ramRatio > 0.75 {
		ramFill = lipgloss.NewStyle().Foreground(s.T.Warning)
	}

	rows := []string{
		components.SpaceBetween(s.Label.Render("CPU"), s.Value.Render(fmt.Sprintf("%.1f %%", st.CPU)), w),
		components.Sparkline(d.cpuHist, w, maxOf(d.cpuHist, 100), fill),
		"",
		components.SpaceBetween(s.Label.Render("RAM"),
			s.Value.Render(fmt.Sprintf("%s / %s", fmtGiB(st.RSSMB), fmtGiB(float64(d.xmxMB)))), w),
		components.ProgressBar(ramRatio, w, ramFill, empty),
		"",
		components.SpaceBetween(s.Label.Render("Uptime"),
			s.Value.Render(components.HumanDuration(time.Duration(st.UptimeSec)*time.Second)), w),
		components.SpaceBetween(s.Label.Render("Players"),
			s.Value.Render(fmt.Sprintf("%d / %d", st.Players, st.MaxPlayer)), w),
		"",
		components.SpaceBetween(s.Label.Render("Port"), s.Value.Render(d.port), w),
		components.SpaceBetween(s.Label.Render("Flags"), s.Value.Render(d.cfg.Java.Preset), w),
		components.SpaceBetween(s.Label.Render("Auto-restart"), s.Value.Render(yesNo(d.cfg.Runtime.AutoRestart)), w),
	}
	if d.status.LastErr != "" {
		rows = append(rows, "", s.Error.Render(components.Truncate(d.status.LastErr, w)))
	}

	return p.Render(components.PadLines(strings.Join(rows, "\n"), p.ContentHeight()), s)
}

func (d *Dashboard) viewConsole() string {
	p, _ := d.consoleBox()
	p.Title = "Console"
	p.Focused = d.focus == focusConsole
	if f := d.console.Filter(); f != "" {
		p.Badge = "filter: " + f
	} else if !d.console.AutoScroll() {
		p.Badge = fmt.Sprintf("%d%%", d.console.ScrollPercent())
	}

	body := zone.Mark("dash:console", d.console.View())

	inputPanel := components.Panel{Width: p.Width, Height: inputRows, Focused: d.focus == focusInput}
	field := d.input.View()
	if d.search.Focused() {
		field = d.search.View()
	} else if d.focus != focusInput {
		field = d.S.Dim.Render("> press Tab or i to type a command")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		p.Render(body, d.S),
		inputPanel.Render(zone.Mark("dash:input", field), d.S),
	)
}

func fmtGiB(mb float64) string {
	if mb <= 0 {
		return "0 GiB"
	}
	return fmt.Sprintf("%.1f GiB", mb/1024)
}

func maxOf(vals []float64, floor float64) float64 {
	m := floor
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}
