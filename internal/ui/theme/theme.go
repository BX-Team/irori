package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name string

	Base    lipgloss.Color
	Mantle  lipgloss.Color
	Crust   lipgloss.Color
	Surface lipgloss.Color
	Overlay lipgloss.Color

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

func Names() []string { return []string{"catppuccin-mocha", "catppuccin-latte"} }

func GetTheme(name string) Theme {
	switch name {
	case "catppuccin-latte", "latte":
		return CatppuccinLatte()
	default:
		return CatppuccinMocha()
	}
}
