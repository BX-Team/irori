package ui

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/daemon"
	"github.com/bx-team/irori/internal/ipc"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/link"
	"github.com/bx-team/irori/internal/ui/msgs"
	"github.com/bx-team/irori/internal/ui/screens"
	"github.com/bx-team/irori/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

const (
	barRows       = 3
	toastDuration = 4 * time.Second
	reconnectEach = 3 * time.Second
)

type reconnectMsg struct{}

type toastExpiredMsg struct{}

type App struct {
	cfg    *config.Config
	user   *config.User
	styles *components.Styles
	link   *link.Link

	width  int
	height int

	tabs    []components.Tab
	active  int
	screens []screens.Screen

	status models.Status
	linked bool

	help       bool
	confirm    *msgs.ConfirmMsg
	confirmYes bool

	toast      string
	toastLevel models.LogLevel
}

func New(cfg *config.Config, user *config.User) *App {
	st := components.NewStyles(theme.GetTheme(user.Theme))

	a := &App{
		cfg:    cfg,
		user:   user,
		styles: st,
		link:   link.New(cfg.SocketPath(), user.ConsoleLines),
		tabs: []components.Tab{
			{ID: "console", Label: "Console"},
			{ID: "files", Label: "Files"},
			{ID: "plugins", Label: addonTabLabel(cfg)},
			{ID: "props", Label: "Configs"},
			{ID: "settings", Label: "Settings"},
		},
		status: models.Status{State: models.StateStopped},
	}

	a.screens = []screens.Screen{
		screens.NewDashboard(st, cfg, user.ConsoleLines),
		screens.NewFiles(st, cfg),
		screens.NewPlugins(st, cfg),
		screens.NewConfigs(st, cfg),
		screens.NewSettings(st, cfg, user),
	}
	return a
}

func addonTabLabel(cfg *config.Config) string {
	if cfg.Server.Type.AddonDir() == "mods" {
		return "Mods"
	}
	return "Plugins"
}

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.link.Connect(), a.link.Wait(), tea.EnterAltScreen}
	for _, s := range a.screens {
		if cmd := s.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.resize()
		return a, nil

	case msgs.LinkUpMsg:
		a.linked = true
		return a, a.forward(msg)

	case msgs.LinkDownMsg:
		a.linked = false
		return a, tea.Batch(a.forward(msg), tea.Tick(reconnectEach, func(time.Time) tea.Msg {
			return reconnectMsg{}
		}))

	case reconnectMsg:
		if a.linked {
			return a, nil
		}
		return a, tea.Batch(a.link.Connect(), a.link.Wait())

	case msgs.StatusMsg:
		a.status = m.Status
		return a, tea.Batch(a.forward(msg), a.link.Wait())

	case msgs.StatsMsg, msgs.LogMsg, msgs.HistoryMsg:
		return a, tea.Batch(a.forward(msg), a.link.Wait())

	case msgs.PowerMsg:
		return a, a.power(m.Action)

	case msgs.SendCommandMsg:
		if !a.linked {
			return a, a.notify("server is not running", models.LevelWarn)
		}
		return a, a.link.Send(ipc.Frame{T: ipc.Input, Data: m.Text})

	case msgs.ConfirmMsg:
		a.confirm = &m
		a.confirmYes = !m.Danger
		return a, nil

	case msgs.ErrorMsg:
		return a, a.notify(m.Title+": "+m.Detail, models.LevelError)

	case msgs.ToastMsg:
		return a, a.notify(m.Text, m.Level)

	case toastExpiredMsg:
		a.toast = ""
		return a, nil

	case msgs.OpenEditorMsg:
		return a, a.openEditor(m.Path)

	case msgs.SwitchTabMsg:
		a.selectTab(m.ID)
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(m)

	case tea.MouseMsg:
		return a.handleMouse(m)
	}

	return a, a.forward(msg)
}

func (a *App) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()

	if a.confirm != nil {
		switch key {
		case "left", "right", "tab", "h", "l":
			a.confirmYes = !a.confirmYes
		case "esc", "n":
			a.confirm = nil
		case "enter", "y":
			yes := a.confirmYes || key == "y"
			onYes := a.confirm.OnYes
			a.confirm = nil
			if yes && onYes != nil {
				return a, func() tea.Msg { return onYes }
			}
		}
		return a, nil
	}

	if a.help {
		if key == "?" || key == "esc" || key == "q" {
			a.help = false
		}
		return a, nil
	}

	if key == "ctrl+c" {
		return a, tea.Quit
	}

	switch key {
	case "shift+tab":
		a.active = (a.active + len(a.tabs) - 1) % len(a.tabs)
		a.resize()
		return a, nil
	}

	if a.current().CapturesInput() {
		return a, a.forwardActive(k)
	}

	switch key {
	case "q":
		return a, tea.Quit
	case "?":
		a.help = true
		return a, nil
	}
	if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(a.tabs) {
		a.active = n - 1
		a.resize()
		return a, nil
	}

	return a, a.forwardActive(k)
}

func (a *App) handleMouse(m tea.MouseMsg) (tea.Model, tea.Cmd) {
	if a.confirm != nil {
		if m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft {
			if zone.Get("modal:yes").InBounds(m) {
				onYes := a.confirm.OnYes
				a.confirm = nil
				if onYes != nil {
					return a, func() tea.Msg { return onYes }
				}
			}
			if zone.Get("modal:no").InBounds(m) {
				a.confirm = nil
			}
		}
		return a, nil
	}

	if m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft {
		for i, t := range a.tabs {
			if zone.Get("tab:" + t.ID).InBounds(m) {
				a.active = i
				a.resize()
				return a, nil
			}
		}
	}
	return a, a.forwardActive(m)
}

func (a *App) current() screens.Screen { return a.screens[a.active] }

func (a *App) selectTab(id string) {
	for i, t := range a.tabs {
		if t.ID == id {
			a.active = i
			a.resize()
			return
		}
	}
}

func (a *App) forward(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i, s := range a.screens {
		next, cmd := s.Update(msg)
		a.screens[i] = next
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// forwardActive routes input to the focused screen only. Broadcasting keys
// would let a hidden tab act on them: Ctrl+S in Settings would also toggle a
// value in Configs, and Enter would open an editor for whatever the Files
// tab happens to have under its cursor.
func (a *App) forwardActive(msg tea.Msg) tea.Cmd {
	next, cmd := a.current().Update(msg)
	a.screens[a.active] = next
	return cmd
}

func (a *App) resize() {
	h := a.height - barRows*2
	if h < 3 {
		h = 3
	}
	for _, s := range a.screens {
		s.SetSize(a.width, h)
	}
}

func (a *App) notify(text string, level models.LogLevel) tea.Cmd {
	a.toast, a.toastLevel = text, level
	return tea.Tick(toastDuration, func(time.Time) tea.Msg { return toastExpiredMsg{} })
}

func (a *App) power(action msgs.PowerAction) tea.Cmd {
	if !a.linked {
		if action == msgs.PowerStop || action == msgs.PowerKill {
			return a.notify("server is already stopped", models.LevelWarn)
		}
		dir := a.cfg.Dir()
		return tea.Batch(
			a.notify("starting server…", models.LevelIrori),
			func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				if err := daemon.Spawn(ctx, dir); err != nil {
					return msgs.ErrorMsg{Title: "Start", Detail: err.Error()}
				}
				return reconnectMsg{}
			},
		)
	}

	frame := map[msgs.PowerAction]string{
		msgs.PowerStart:   ipc.Start,
		msgs.PowerStop:    ipc.Stop,
		msgs.PowerRestart: ipc.Restart,
		msgs.PowerKill:    ipc.Kill,
	}[action]
	return a.link.Send(ipc.Frame{T: frame})
}

func (a *App) openEditor(path string) tea.Cmd {
	editor := a.user.EditorCommand()
	fields := strings.Fields(editor)
	args := append(fields[1:], path)
	cmd := exec.Command(fields[0], args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return msgs.ErrorMsg{Title: "Editor", Detail: err.Error()}
		}
		return msgs.ConfigChangedMsg{}
	})
}

func (a *App) Close() { a.link.Close() }

func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return ""
	}

	main := lipgloss.JoinVertical(lipgloss.Left,
		a.topBar(),
		a.current().View(),
		a.bottomBar(),
	)

	if a.help {
		return zone.Scan(components.RenderHelp(a.styles, a.width, a.height, a.helpSections()))
	}
	if a.confirm != nil {
		modal := components.Modal{
			Title:  a.confirm.Title,
			Body:   a.confirm.Body,
			Width:  min(64, a.width-8),
			Danger: a.confirm.Danger,
		}
		buttons := components.ConfirmButtons(a.styles, "Yes", "Cancel", a.confirmYes, a.confirm.Danger)
		return zone.Scan(lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center,
			modal.Render(a.styles, buttons)))
	}
	return zone.Scan(main)
}

func (a *App) topBar() string {
	s := a.styles
	label := a.status.State.Label()
	if !a.linked {
		label = "OFFLINE"
	}

	left := s.AppBadge.Render("irori") + " " +
		components.Dot(s, s.StateColor(label)) + " " +
		s.Bold.Render(a.cfg.Server.Name)

	tabsWidth := a.width - lipgloss.Width(left) - 6
	tabs := components.RenderTabs(a.tabs, a.active, tabsWidth, s)
	return components.Bar(left, tabs, a.width, s)
}

func (a *App) bottomBar() string {
	s := a.styles
	left := components.Hints(s, a.current().Hints()...)
	if a.toast != "" {
		style := s.Info
		switch a.toastLevel {
		case models.LevelError:
			style = s.Error
		case models.LevelWarn:
			style = s.Warning
		case models.LevelIrori:
			style = s.Accent
		}
		left = style.Render("● " + a.toast)
	}

	right := components.Hints(s,
		components.Hint{Key: "1-5", Desc: "tabs"},
		components.Hint{Key: "?", Desc: "help"},
		components.Hint{Key: "q", Desc: "quit"},
	)
	return components.Bar(left, right, a.width, s)
}

func (a *App) helpSections() []components.HelpSection {
	return []components.HelpSection{
		{Title: "General", Hints: []components.Hint{
			{Key: "1 … 5", Desc: "switch tab"},
			{Key: "Shift+Tab", Desc: "previous tab"},
			{Key: "?", Desc: "this help"},
			{Key: "q / Ctrl+C", Desc: "quit"},
		}},
		{Title: "Power (Console tab)", Hints: []components.Hint{
			{Key: "s x r k", Desc: "start stop restart kill"},
			{Key: "Click", Desc: "the buttons in the left column"},
		}},
		{Title: "Console", Hints: []components.Hint{
			{Key: "Tab", Desc: "move focus input ↔ console"},
			{Key: "Enter", Desc: "send command"},
			{Key: "↑ ↓", Desc: "command history"},
			{Key: "/", Desc: "filter output"},
			{Key: "g / G", Desc: "jump to top / bottom"},
			{Key: "Ctrl+L", Desc: "clear the console view"},
		}},
		{Title: "Configs & Settings", Hints: []components.Hint{
			{Key: "Space / ← →", Desc: "toggle or cycle a value"},
			{Key: "Enter", Desc: "edit text and numbers in place"},
			{Key: "Ctrl+S", Desc: "save"},
			{Key: "u", Desc: "undo the value under the cursor"},
			{Key: "c", Desc: "declare the key in " + config.FileName},
			{Key: "C", Desc: "declare every key that differs from the shipped default"},
			{Key: "D", Desc: "reset to the vanilla default"},
			{Key: "Tab", Desc: "switch between config files"},
			{Key: "e", Desc: "open the file in $EDITOR"},
		}},
		{Title: "Other", Hints: []components.Hint{
			{Key: "Mouse", Desc: "click tabs and buttons"},
		}},
	}
}
