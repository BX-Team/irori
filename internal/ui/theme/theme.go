package theme

import "image/color"

// Theme is a set of foregrounds, not a colour scheme for a window. irori never
// paints a page background — it draws over whatever the terminal already is —
// which is why every palette here is a dark one: a light palette would be
// unreadable in anything but a light terminal.
type Theme struct {
	Name string

	// Mantle backs modals, Surface backs the bars, and Crust is the text drawn
	// on top of an Accent or state-coloured chip. Nothing here covers the whole
	// screen.
	Mantle  color.Color
	Surface color.Color
	Crust   color.Color

	Text    color.Color
	Subtext color.Color
	Muted   color.Color

	Border        color.Color
	BorderFocused color.Color
	Selection     color.Color

	Accent    color.Color
	Secondary color.Color

	Success color.Color
	Warning color.Color
	Error   color.Color
	Info    color.Color

	Running  color.Color
	Starting color.Color
	Stopping color.Color
	Stopped  color.Color
	Crashed  color.Color

	LogInfo  color.Color
	LogWarn  color.Color
	LogError color.Color
	LogDebug color.Color
	LogTime  color.Color
	LogChat  color.Color

	Dir     color.Color
	Jar     color.Color
	Archive color.Color
	Config  color.Color
	Exec    color.Color
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
