// Package wizard is the first-run flow: it either provisions a brand new
// server from mcjars or adopts one that is already sitting in the directory.
package wizard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/importer"
	"github.com/bx-team/irori/internal/java"
	"github.com/bx-team/irori/internal/launch"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/theme"
	zone "github.com/lrstanley/bubblezone/v2"
)

type step int

const (
	stepMode step = iota
	stepType
	stepVersion
	stepBuild
	stepScan
	stepPickJar
	stepForm
	stepPreset
	stepEula
	stepInstall
	stepDone
)

const cardWidth = 78

type Model struct {
	dir    string
	s      *components.Styles
	client *mcjars.Client

	step    step
	loading bool
	err     string
	spin    spinner.Model

	list   *components.List
	filter textinput.Model
	fields []textinput.Model
	field  int

	types    []mcjars.TypeInfo
	versions []mcjars.Version
	builds   []mcjars.Build

	selType    models.ServerType
	selVersion mcjars.Version
	selBuild   *mcjars.Build

	candidates []importer.Candidate
	script     *importer.StartScript
	extraFlags []string

	progressCh chan mcjars.Progress
	progress   mcjars.Progress

	width  int
	height int

	cfg      *config.Config
	mouse    bool
	done     bool
	quitting bool
}

func Run(dir string, user *config.User) (*config.Config, error) {
	zone.NewGlobal()
	m := New(dir, user)
	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, err
	}
	res := out.(*Model)
	if !res.done {
		return nil, nil
	}
	return res.cfg, nil
}

func New(dir string, user *config.User) *Model {
	st := components.NewStyles(theme.GetTheme(user.Theme))
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = st.Accent

	list := components.NewList(st)
	list.ShowDesc = true
	list.ZonePfx = "wz:"

	filter := st.TextInput("/ ", st.Warning)
	filter.Placeholder = "type to filter"
	filter.SetWidth(cardWidth - 12)

	m := &Model{
		dir:    dir,
		s:      st,
		client: mcjars.New(),
		spin:   sp,
		list:   list,
		filter: filter,
		cfg:    config.Default(dir),
		mouse:  user.Mouse,
	}
	m.setModeItems()
	return m
}

func (m *Model) Init() tea.Cmd { return m.spin.Tick }

func (m *Model) setModeItems() {
	m.list.ShowDesc = true
	m.list.Reset()
	m.list.SetItems([]components.ListItem{
		{ID: "new", Title: "Create a new server",
			Desc: "Pick a server core and version; irori downloads and lays out the files"},
		{ID: "existing", Title: "Adopt an existing server",
			Desc: "Find the jar in this folder and import settings from its start script"},
		{ID: "quit", Title: "Quit", Desc: "Do not create anything"},
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(cardWidth-4, m.listHeight())
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case typesMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.types = msg.types
		m.showTypes()
		return m, nil

	case versionsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.versions = msg.versions
		m.showVersions()
		return m, nil

	case buildsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.builds = msg.builds
		m.showBuilds()
		return m, nil

	case scanMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.candidates = msg.candidates
		m.script = msg.script
		if len(m.candidates) == 0 {
			m.err = "no jar files in this folder — choose \"create a new server\" instead."
			m.step = stepMode
			m.setModeItems()
			return m, nil
		}
		m.step = stepPickJar
		m.showCandidates()
		return m, nil

	case installProgressMsg:
		m.progress = mcjars.Progress(msg)
		return m, waitProgress(m.progressCh)

	case installDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.step = stepBuild
			return m, nil
		}
		if err := m.finish(); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.step = stepDone
		m.done = true
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		if m.usesList() {
			m.list.Update(msg)
			if components.LeftClick(msg) {
				if _, ok := m.list.Selected(); ok {
					return m, m.advance()
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) usesList() bool {
	switch m.step {
	case stepMode, stepType, stepVersion, stepBuild, stepPickJar, stepPreset:
		return !m.loading
	}
	return false
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	if m.filter.Focused() {
		switch k.String() {
		case "esc":
			m.clearFilter()
			return m, nil
		case "enter", "down", "up":
			m.filter.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(k)
		m.list.SetFilter(m.filter.Value())
		return m, cmd
	}

	if k.String() == "esc" {
		switch {
		case m.err != "":
			m.err = ""
		case m.filter.Value() != "":
			m.clearFilter()
		default:
			return m, m.back()
		}
		return m, nil
	}

	if m.step == stepForm {
		return m, m.formKey(k)
	}
	if m.step == stepEula {
		switch k.String() {
		case "y", "enter":
			return m, m.acceptEula()
		case "n":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}
	if m.usesList() {
		switch k.String() {
		case "enter", "l", "right":
			return m, m.advance()
		case "/":
			if m.filterable() {
				return m, m.filter.Focus()
			}
			return m, nil
		}
		m.list.Update(k)
	}
	return m, nil
}

func (m *Model) filterable() bool {
	switch m.step {
	case stepType, stepVersion, stepBuild:
		return true
	}
	return false
}

func (m *Model) clearFilter() {
	m.filter.SetValue("")
	m.filter.Blur()
	m.list.SetFilter("")
}

func (m *Model) advance() tea.Cmd {
	sel, ok := m.list.Selected()
	if !ok {
		return nil
	}
	if sel.Disabled {
		m.err = disabledReason(m.step)
		return nil
	}
	m.err = ""
	m.clearFilter()

	switch m.step {
	case stepMode:
		switch sel.ID {
		case "new":
			m.step, m.loading = stepType, true
			return tea.Batch(loadTypes(m.client), m.spin.Tick)
		case "existing":
			m.step, m.loading = stepScan, true
			return tea.Batch(scanExisting(m.client, m.dir), m.spin.Tick)
		default:
			m.quitting = true
			return tea.Quit
		}

	case stepType:
		m.selType = models.ServerType(sel.ID)
		m.cfg.Server.Type = m.selType
		m.step, m.loading = stepVersion, true
		return tea.Batch(loadVersions(m.client, m.selType), m.spin.Tick)

	case stepVersion:
		for _, v := range m.versions {
			if v.ID == sel.ID {
				m.selVersion = v
				break
			}
		}
		m.step, m.loading = stepBuild, true
		return tea.Batch(loadBuilds(m.client, m.selType, m.selVersion.ID), m.spin.Tick)

	case stepBuild:
		for i := range m.builds {
			if m.builds[i].UUID == sel.ID {
				m.selBuild = &m.builds[i]
				break
			}
		}
		m.applyBuild()
		m.step = stepForm
		m.buildForm()
		return textinput.Blink

	case stepPickJar:
		for _, c := range m.candidates {
			if c.File == sel.ID {
				m.applyCandidate(c)
				break
			}
		}
		m.step = stepForm
		m.buildForm()
		return textinput.Blink

	case stepPreset:
		m.cfg.Java.Preset = sel.ID
		m.cfg.Java.ExtraFlags = importer.StripPresetFlags(m.extraFlags, sel.ID, m.cfg.Java.Xmx)
		if m.selBuild != nil {
			if m.eulaAccepted() {
				return m.install()
			}
			m.step = stepEula
			return nil
		}
		if err := m.finish(); err != nil {
			m.err = err.Error()
			return nil
		}
		m.done = true
		return tea.Quit
	}
	return nil
}

func disabledReason(s step) string {
	switch s {
	case stepType:
		return "this server core is deprecated and no longer receives builds"
	case stepVersion:
		return "this version is no longer supported by the project"
	default:
		return "this entry cannot be selected"
	}
}

func (m *Model) back() tea.Cmd {
	m.clearFilter()
	switch m.step {
	case stepMode:
		m.quitting = true
		return tea.Quit
	case stepType, stepScan:
		m.step = stepMode
		m.setModeItems()
	case stepVersion:
		m.step = stepType
		m.showTypes()
	case stepBuild:
		m.step = stepVersion
		m.showVersions()
	case stepPickJar:
		m.step = stepMode
		m.setModeItems()
	case stepForm:
		if m.selBuild != nil {
			m.step = stepBuild
			m.showBuilds()
		} else {
			m.step = stepPickJar
			m.showCandidates()
		}
	case stepPreset:
		m.step = stepForm
		m.buildForm()
	case stepEula:
		m.step = stepPreset
		m.showPresets()
	}
	return nil
}

func (m *Model) applyBuild() {
	if m.selBuild == nil {
		return
	}
	m.cfg.Server.Type = m.selBuild.Type
	m.cfg.Server.MCVersion = m.selBuild.VersionID
	m.cfg.Server.Build = m.selBuild.Name
	m.cfg.Server.BuildID = m.selBuild.UUID
	m.cfg.Java.Major = m.selVersion.Java
	m.cfg.Server.Jar = m.selBuild.CoreJar()
}

func (m *Model) applyCandidate(c importer.Candidate) {
	m.cfg.Server.Jar = c.File
	if c.Type != "" {
		m.cfg.Server.Type = c.Type
	}
	if c.Version != "" {
		m.cfg.Server.MCVersion = c.Version
	}
	if c.Build != nil {
		m.cfg.Server.Build = c.Build.Name
		m.cfg.Server.BuildID = c.Build.UUID
	}
	// An adopted directory has to record the JDK floor too. Leaving it at zero
	// is what lets detection settle on the oldest runtime it can find.
	if c.Java > 0 {
		m.cfg.Java.Major = c.Java
	} else {
		m.cfg.Java.Major = java.RequiredFor(m.cfg.Server.MCVersion)
	}
	if m.script != nil {
		if m.script.Xms != "" {
			m.cfg.Java.Xms = m.script.Xms
		}
		if m.script.Xmx != "" {
			m.cfg.Java.Xmx = m.script.Xmx
		}
		if m.script.Jar != "" && m.script.Jar == filepath.Base(c.File) {
			m.cfg.Server.Jar = m.script.Jar
		}
		m.cfg.Java.Preset = m.script.Preset
		m.extraFlags = m.script.ExtraFlags
		if len(m.script.Args) > 0 {
			m.cfg.Java.Args = m.script.Args
		}
	}
}

func (m *Model) eulaAccepted() bool {
	if m.cfg.Server.Type.IsProxy() || m.cfg.Server.Type.IsLimbo() {
		return true
	}
	raw, err := os.ReadFile(filepath.Join(m.dir, "eula.txt"))
	return err == nil && strings.Contains(string(raw), "eula=true")
}

func (m *Model) acceptEula() tea.Cmd {
	body := "# accepted through irori\neula=true\n"
	if err := os.WriteFile(filepath.Join(m.dir, "eula.txt"), []byte(body), 0o644); err != nil {
		m.err = err.Error()
		return nil
	}
	return m.install()
}

func (m *Model) install() tea.Cmd {
	if m.selBuild == nil {
		return nil
	}
	m.step = stepInstall
	m.progress = mcjars.Progress{Label: "preparing", Total: 1}
	return tea.Batch(m.startInstall(*m.selBuild), m.spin.Tick)
}

func (m *Model) finish() error {
	if m.cfg.Server.Name == "" {
		m.cfg.Server.Name = filepath.Base(m.dir)
	}
	if _, err := launch.ParseMemMB(m.cfg.Java.Xmx); err != nil {
		return errors.New("invalid Xmx size")
	}
	if err := m.cfg.EnsureStateDir(); err != nil {
		return err
	}
	if err := os.MkdirAll(m.cfg.AddonDir(), 0o755); err != nil {
		return err
	}
	if err := m.cfg.Save(); err != nil {
		return fmt.Errorf("could not write %s: %w", config.FileName, err)
	}
	return nil
}
