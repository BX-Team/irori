package screens

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/java"
	"github.com/bx-team/irori/internal/launch"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/bx-team/irori/internal/models"
	"github.com/bx-team/irori/internal/ui/components"
	"github.com/bx-team/irori/internal/ui/msgs"
	"github.com/bx-team/irori/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const previewRows = 6

type javaDetectedMsg struct {
	jdk java.JDK
	err error
}

type buildCheckedMsg struct {
	build mcjars.Build
	err   error
}

type installBuildMsg struct{}

type buildInstalledMsg struct {
	build mcjars.Build
	err   error
}

type Settings struct {
	Base
	cfg  *config.Config
	user *config.User

	fields *components.Fields
	jdk    java.JDK
	jdkErr error

	newBuild   *mcjars.Build
	checking   bool
	installing bool

	running bool
	saved   bool
}

func NewSettings(s *components.Styles, cfg *config.Config, user *config.User) *Settings {
	st := &Settings{
		cfg:    cfg,
		user:   user,
		fields: components.NewFields(s),
	}
	st.S = s
	st.fields.ZonePfx = "set:"
	st.fields.LabelWidth = 26
	st.rebuild()
	return st
}

func (s *Settings) Init() tea.Cmd { return s.detectJava() }

func (s *Settings) CapturesInput() bool { return s.fields.Editing() }

func (s *Settings) rebuild() { s.build(true) }

func (s *Settings) reset() { s.build(false) }

func (s *Settings) build(keepEdits bool) {
	c, u := s.cfg, s.user

	edited := map[string]string{}
	if keepEdits && s.fields != nil {
		for _, f := range s.fields.All() {
			edited[f.Key] = f.Value
		}
	}

	presetOpts := make([]string, 0, len(launch.Presets()))
	for _, p := range launch.Presets() {
		presetOpts = append(presetOpts, p.ID)
	}

	fields := []components.Field{
		{Key: "server.name", Label: "Server name", Section: "Server", Value: c.Server.Name,
			Desc: "Shown in the header and in irori status.", Validate: notEmpty},
		{Key: "server.jar", Label: "Jar file", Section: "Server", Value: c.Server.Jar,
			Desc: "File passed to java -jar, relative to the server directory.", Validate: notEmpty},
		{Key: "server.type", Label: "Core", Section: "Server", Value: c.Server.Type.Display(), ReadOnly: true,
			Desc: "Server software. Changing it means installing a different core."},
		{Key: "server.version", Label: "Minecraft version", Section: "Server", Value: c.Server.MCVersion, ReadOnly: true},
		{Key: "server.build", Label: "Build", Section: "Server", Value: orDash(c.Server.Build), ReadOnly: true},
		{Key: "server.update", Label: "Core update", Section: "Server", Kind: components.FieldAction,
			Note: s.updateNote(), Desc: "Ask mcjars for the newest build of this core and version."},

		{Key: "java.xms", Label: "Initial heap (Xms)", Section: "Java", Value: c.Java.Xms,
			Desc: "Heap the JVM starts with, for example 2G or 2048M.", Validate: validMem},
		{Key: "java.xmx", Label: "Maximum heap (Xmx)", Section: "Java", Value: c.Java.Xmx,
			Desc: "Hard heap ceiling. Leave a gigabyte or two for the JVM itself and the OS.", Validate: validMem},
		{Key: "java.preset", Label: "Flag preset", Section: "Java", Kind: components.FieldEnum,
			Value: c.Java.Preset, Options: presetOpts, Desc: launch.GetPreset(c.Java.Preset).Summary},
		{Key: "java.extraFlags", Label: "Extra flags", Section: "Java", Value: strings.Join(c.Java.ExtraFlags, " "),
			Desc: "Space separated JVM flags appended after the preset."},
		{Key: "java.major", Label: "Required Java", Section: "Java", Kind: components.FieldInt,
			Value: strconv.Itoa(c.Java.Major), Min: 0, Max: 99, Note: s.requiredNote(),
			Desc: "Minimum JDK detection will accept. 0 derives it from the Minecraft version."},
		{Key: "java.path", Label: "Java binary", Section: "Java", Value: c.Java.Path, Note: s.javaNote(),
			Desc: "Leave empty to detect automatically from JAVA_HOME, PATH and the nix store."},
		{Key: "java.detect", Label: "Re-detect Java", Section: "Java", Kind: components.FieldAction,
			Note: "run detection", Desc: "Drop the cached result and search for a JDK again."},

		{Key: "runtime.autoRestart", Label: "Auto restart", Section: "Runtime", Kind: components.FieldBool,
			Value: boolText(c.Runtime.AutoRestart), Desc: "Bring the server back up after a crash."},
		{Key: "runtime.stopCommand", Label: "Stop command", Section: "Runtime", Value: c.Runtime.StopCommand,
			Desc: "Console command sent for a graceful stop before any signal is used.", Validate: notEmpty},
		{Key: "runtime.stopTimeout", Label: "Stop timeout (s)", Section: "Runtime", Kind: components.FieldInt,
			Value: strconv.Itoa(c.Runtime.StopTimeoutSec), Min: 5, Max: 3600,
			Desc: "How long to wait after the stop command before escalating to SIGTERM."},

		{Key: "ui.theme", Label: "Theme", Section: "Interface", Kind: components.FieldEnum,
			Value: u.Theme, Options: theme.Names(), Desc: "Applied the next time irori starts."},
		{Key: "ui.mouse", Label: "Mouse support", Section: "Interface", Kind: components.FieldBool,
			Value: boolText(u.Mouse), Desc: "Click tabs, buttons and list rows. Disable to keep terminal text selection."},
		{Key: "ui.consoleLines", Label: "Console scrollback", Section: "Interface", Kind: components.FieldInt,
			Value: strconv.Itoa(u.ConsoleLines), Min: 200, Max: 100000,
			Desc: "Lines kept in the console buffer."},
		{Key: "ui.editor", Label: "External editor", Section: "Interface", Value: u.Editor,
			Note: "$VISUAL / $EDITOR / nano", Desc: "Command used to open files from the Files tab."},
	}

	for i := range fields {
		if v, ok := edited[fields[i].Key]; ok && !fields[i].ReadOnly && fields[i].Kind != components.FieldAction {
			fields[i].Value = v
		}
		fields[i].Dirty = s.isDirty(fields[i])
	}
	s.fields.SetFields(fields)
}

func (s *Settings) isDirty(f components.Field) bool {
	c, u := s.cfg, s.user
	switch f.Key {
	case "server.name":
		return f.Value != c.Server.Name
	case "server.jar":
		return f.Value != c.Server.Jar
	case "java.xms":
		return f.Value != c.Java.Xms
	case "java.xmx":
		return f.Value != c.Java.Xmx
	case "java.preset":
		return f.Value != c.Java.Preset
	case "java.extraFlags":
		return f.Value != strings.Join(c.Java.ExtraFlags, " ")
	case "java.major":
		return f.Value != strconv.Itoa(c.Java.Major)
	case "java.path":
		return f.Value != c.Java.Path
	case "runtime.autoRestart":
		return f.Value != boolText(c.Runtime.AutoRestart)
	case "runtime.stopCommand":
		return f.Value != c.Runtime.StopCommand
	case "runtime.stopTimeout":
		return f.Value != strconv.Itoa(c.Runtime.StopTimeoutSec)
	case "ui.theme":
		return f.Value != u.Theme
	case "ui.mouse":
		return f.Value != boolText(u.Mouse)
	case "ui.consoleLines":
		return f.Value != strconv.Itoa(u.ConsoleLines)
	case "ui.editor":
		return f.Value != u.Editor
	}
	return false
}

func (s *Settings) value(key string) string {
	for _, f := range s.fields.All() {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

func (s *Settings) dirty() bool {
	for _, f := range s.fields.All() {
		if s.isDirty(f) {
			return true
		}
	}
	return false
}

func notEmpty(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("cannot be empty")
	}
	return nil
}

func validMem(v string) error {
	mb, err := launch.ParseMemMB(v)
	if err != nil {
		return err
	}
	if mb < 512 {
		return fmt.Errorf("at least 512M is required")
	}
	return nil
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func (s *Settings) javaNote() string {
	switch {
	case s.jdkErr != nil:
		return "not found"
	case s.jdk.Path == "":
		return "detecting…"
	case s.cfg.JavaMajor() > 0 && s.jdk.Major < s.cfg.JavaMajor():
		return "auto: " + s.jdk.Display() + " — too old"
	default:
		return "auto: " + s.jdk.Display() + " · " + s.jdk.Source
	}
}

func (s *Settings) requiredNote() string {
	if s.cfg.Java.Major > 0 {
		return "from " + s.cfg.Server.Type.Display() + " " + s.cfg.Server.MCVersion
	}
	if guess := java.RequiredFor(s.cfg.Server.MCVersion); guess > 0 {
		return fmt.Sprintf("auto: %d (from Minecraft %s)", guess, s.cfg.Server.MCVersion)
	}
	return "no requirement known"
}

func (s *Settings) updateNote() string {
	switch {
	case s.installing:
		return "installing…"
	case s.checking:
		return "checking…"
	case s.newBuild != nil && s.newBuild.Name != s.cfg.Server.Build:
		return s.newBuild.Name + " available — Enter to install"
	case s.newBuild != nil:
		return "up to date"
	default:
		return "check for updates"
	}
}

func (s *Settings) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case msgs.StatusMsg:
		s.running = m.Status.State.IsUp()
		return s, nil

	case msgs.LinkDownMsg:
		s.running = false
		return s, nil

	case msgs.ConfigChangedMsg:
		s.rebuild()
		return s, nil

	case javaDetectedMsg:
		s.jdk, s.jdkErr = m.jdk, m.err
		s.rebuild()
		return s, nil

	case buildCheckedMsg:
		s.checking = false
		if m.err != nil {
			s.rebuild()
			return s, errToast("Update check", m.err)
		}
		s.newBuild = &m.build
		s.rebuild()
		if m.build.Name == s.cfg.Server.Build {
			return s, toast("already on the newest build", models.LevelIrori)
		}
		return s, toast("build "+m.build.Name+" is available", models.LevelIrori)

	case installBuildMsg:
		if s.newBuild == nil {
			return s, nil
		}
		s.installing = true
		s.rebuild()
		return s, s.installBuild(*s.newBuild)

	case buildInstalledMsg:
		s.installing = false
		if m.err != nil {
			s.rebuild()
			return s, errToast("Install", m.err)
		}
		s.cfg.Server.Build = m.build.Name
		s.cfg.Server.Jar = m.build.CoreJar()
		s.newBuild = nil
		if err := s.cfg.Save(); err != nil {
			return s, errToast("Config", err)
		}
		s.rebuild()
		return s, toast("installed build "+m.build.Name, models.LevelIrori)

	case tea.KeyMsg:
		return s.handleKey(m)

	case tea.MouseMsg:
		cmd, changed := s.fields.Update(msg)
		if changed {
			s.saved = false
			s.rebuild()
		}
		return s, cmd
	}

	if s.fields.Editing() {
		cmd, _ := s.fields.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s *Settings) handleKey(k tea.KeyMsg) (Screen, tea.Cmd) {
	if s.fields.Editing() {
		cmd, changed := s.fields.Update(k)
		if changed {
			s.saved = false
			s.rebuild()
		}
		return s, cmd
	}

	switch k.String() {
	case "ctrl+s":
		return s, s.save()
	case "u", "esc":
		if s.dirty() {
			s.reset()
			return s, toast("changes discarded", models.LevelIrori)
		}
		return s, nil
	case "enter", " ":
		if f, ok := s.fields.Selected(); ok && f.Kind == components.FieldAction {
			return s, s.action(f.Key)
		}
	}

	cmd, changed := s.fields.Update(k)
	if changed {
		s.saved = false
		s.rebuild()
	}
	return s, cmd
}

func (s *Settings) action(key string) tea.Cmd {
	switch key {
	case "java.detect":
		_ = java.ClearCache(s.cfg.StateDir())
		s.jdk, s.jdkErr = java.JDK{}, nil
		s.rebuild()
		return s.detectJava()

	case "server.update":
		if s.installing || s.checking {
			return nil
		}
		if config.Sealed() {
			return toast("sealed mode: the core is pinned by Nix", models.LevelWarn)
		}
		if s.newBuild != nil && s.newBuild.Name != s.cfg.Server.Build {
			if s.running {
				return toast("stop the server before replacing the core", models.LevelWarn)
			}
			build := *s.newBuild
			return func() tea.Msg {
				return msgs.ConfirmMsg{
					Title: "Install build " + build.Name,
					Body: fmt.Sprintf("%s %s will be re-downloaded from mcjars into this directory.\nWorlds and plugins are untouched. Continue?",
						s.cfg.Server.Type.Display(), s.cfg.Server.MCVersion),
					OnYes: installBuildMsg{},
				}
			}
		}
		s.checking = true
		s.rebuild()
		return s.checkBuild()
	}
	return nil
}

func (s *Settings) detectJava() tea.Cmd {
	dir, explicit, major := s.cfg.StateDir(), s.cfg.Java.Path, s.cfg.JavaMajor()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		jdk, err := java.ResolveCached(ctx, dir, explicit, major)
		return javaDetectedMsg{jdk: jdk, err: err}
	}
}

func (s *Settings) checkBuild() tea.Cmd {
	t, v := s.cfg.Server.Type, s.cfg.Server.MCVersion
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		b, err := mcjars.New().LatestBuild(ctx, t, v)
		return buildCheckedMsg{build: b, err: err}
	}
}

func (s *Settings) installBuild(b mcjars.Build) tea.Cmd {
	dir := s.cfg.Dir()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		err := mcjars.New().Install(ctx, b, dir, nil)
		return buildInstalledMsg{build: b, err: err}
	}
}

func (s *Settings) save() tea.Cmd {
	if !s.dirty() {
		return toast("nothing to save", models.LevelWarn)
	}

	c, u := s.cfg, s.user
	c.Server.Name = strings.TrimSpace(s.value("server.name"))
	c.Server.Jar = strings.TrimSpace(s.value("server.jar"))
	c.Java.Xms = strings.TrimSpace(s.value("java.xms"))
	c.Java.Xmx = strings.TrimSpace(s.value("java.xmx"))
	c.Java.Preset = s.value("java.preset")
	c.Java.ExtraFlags = strings.Fields(s.value("java.extraFlags"))
	majorChanged := s.value("java.major") != strconv.Itoa(c.Java.Major)
	c.Java.Major = atoiOr(s.value("java.major"), c.Java.Major)
	c.Java.Path = strings.TrimSpace(s.value("java.path"))
	c.Runtime.AutoRestart = s.value("runtime.autoRestart") == "true"
	c.Runtime.StopCommand = strings.TrimSpace(s.value("runtime.stopCommand"))
	c.Runtime.StopTimeoutSec = atoiOr(s.value("runtime.stopTimeout"), c.Runtime.StopTimeoutSec)

	if xms, err := launch.ParseMemMB(c.Java.Xms); err == nil {
		if xmx, err := launch.ParseMemMB(c.Java.Xmx); err == nil && xms > xmx {
			c.Java.Xms = c.Java.Xmx
		}
	}

	u.Theme = s.value("ui.theme")
	u.Mouse = s.value("ui.mouse") == "true"
	u.ConsoleLines = atoiOr(s.value("ui.consoleLines"), u.ConsoleLines)
	u.Editor = strings.TrimSpace(s.value("ui.editor"))

	if err := c.Save(); err != nil {
		return errToast("Config", err)
	}
	if err := u.Save(); err != nil {
		return errToast("User settings", err)
	}

	s.saved = true
	s.reset()

	text := "settings saved"
	if s.running {
		text += " — restart the server to apply"
	}
	cmds := []tea.Cmd{
		toast(text, models.LevelIrori),
		func() tea.Msg { return msgs.ConfigChangedMsg{} },
	}
	// The memo is keyed on the required major, but a stale entry would still be
	// shown until it expires, so raising the floor has to re-run detection.
	if majorChanged {
		_ = java.ClearCache(s.cfg.StateDir())
		s.jdk, s.jdkErr = java.JDK{}, nil
		cmds = append(cmds, s.detectJava())
	}
	return tea.Batch(cmds...)
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func (s *Settings) Hints() []components.Hint {
	if s.fields.Editing() {
		return []components.Hint{
			{Key: "Enter", Desc: "commit"},
			{Key: "Esc", Desc: "cancel"},
		}
	}
	return []components.Hint{
		{Key: "↑↓", Desc: "move"},
		{Key: "Space/←→", Desc: "change"},
		{Key: "Enter", Desc: "edit / run"},
		{Key: "Ctrl+S", Desc: "save"},
		{Key: "u", Desc: "discard"},
	}
}

func (s *Settings) View() string {
	preview, previewHeight := s.preview()
	formHeight := s.Height - previewHeight
	if formHeight < 5 {
		formHeight = s.Height
	}

	badge := ""
	switch {
	case s.dirty():
		badge = "unsaved"
	case s.saved:
		badge = "saved"
	}

	panel := components.Panel{
		Title:   "irori settings",
		Badge:   badge,
		Focused: true,
		Width:   s.Width,
		Height:  formHeight,
	}

	inner := panel.ContentHeight()
	if e := s.fields.Err(); e != "" {
		inner--
	}
	s.fields.SetSize(panel.ContentWidth(), inner)

	body := components.PadLines(s.fields.View(), inner)
	if e := s.fields.Err(); e != "" {
		body += "\n" + s.S.Error.Render(components.Truncate("! "+e, panel.ContentWidth()))
	}

	form := panel.Render(body, s.S)
	if formHeight == s.Height {
		return form
	}
	return lipgloss.JoinVertical(lipgloss.Left, form, preview)
}

func (s *Settings) preview() (string, int) {
	draft := *s.cfg
	draft.Server.Jar = s.value("server.jar")
	draft.Java.Xms = s.value("java.xms")
	draft.Java.Xmx = s.value("java.xmx")
	draft.Java.Preset = s.value("java.preset")
	draft.Java.ExtraFlags = strings.Fields(s.value("java.extraFlags"))

	javaPath := s.value("java.path")
	if javaPath == "" {
		javaPath = s.jdk.Path
	}
	if javaPath == "" {
		javaPath = "java"
	}

	panel := components.Panel{Title: "launch command", Width: s.Width}

	var lines []string
	spec, err := launch.Build(&draft, javaPath)
	if err != nil {
		lines = []string{s.S.Error.Render(components.Truncate(err.Error(), panel.ContentWidth()))}
	} else {
		for _, l := range components.WrapWords(spec.String(), panel.ContentWidth()) {
			lines = append(lines, s.S.Subtext.Render(l))
		}
	}

	panel.Height = clampInt(len(lines)+2, previewRows, max(previewRows, s.Height/2))
	return panel.Render(
		components.PadLines(strings.Join(lines, "\n"), panel.ContentHeight()), s.S), panel.Height
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
