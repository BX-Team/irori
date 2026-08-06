package theme

import "github.com/charmbracelet/lipgloss"

// Theme is a set of foregrounds, not a colour scheme for a window. irori never
// paints a page background — it draws over whatever the terminal already is —
// which is why every palette here is a dark one: a light palette would be
// unreadable in anything but a light terminal.
type Theme struct {
	Name string

	// Mantle backs modals, Surface backs the bars, and Crust is the text drawn
	// on top of an Accent or state-coloured chip. Nothing here covers the whole
	// screen.
	Mantle  lipgloss.Color
	Surface lipgloss.Color
	Crust   lipgloss.Color

	Text    lipgloss.Color
	Subtext lipgloss.Color
	Muted   lipgloss.Color

	Border        lipgloss.Color
	BorderFocused lipgloss.Color
	Selection     lipgloss.Color

	Accent    lipgloss.Color
	Secondary lipgloss.Color

	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
	Info    lipgloss.Color

	Running  lipgloss.Color
	Starting lipgloss.Color
	Stopping lipgloss.Color
	Stopped  lipgloss.Color
	Crashed  lipgloss.Color

	LogInfo  lipgloss.Color
	LogWarn  lipgloss.Color
	LogError lipgloss.Color
	LogDebug lipgloss.Color
	LogTime  lipgloss.Color
	LogChat  lipgloss.Color

	Dir     lipgloss.Color
	Jar     lipgloss.Color
	Archive lipgloss.Color
	Config  lipgloss.Color
	Exec    lipgloss.Color
}

var all = []Theme{
	Catppuccin(),
	TokyoNight(),
	Kanagawa(),
	GruvboxDark(),
	RosePineMoon(),
	Nord(),
}

func Names() []string {
	out := make([]string, len(all))
	for i, t := range all {
		out[i] = t.Name
	}
	return out
}

// GetTheme falls back to the default rather than erroring: a user config still
// naming a theme that has since been dropped should not stop the TUI opening.
func GetTheme(name string) Theme {
	for _, t := range all {
		if t.Name == name {
			return t
		}
	}
	return all[0]
}
