package theme

import "github.com/charmbracelet/lipgloss"

// Catppuccin is the Mocha flavour. The other three are deliberately not here:
// what separates them is the background ramp, which irori never paints, and
// their foregrounds sit within 36/255 per channel of these — on screen they
// render as the same theme.
func Catppuccin() Theme {
	return Theme{
		Name: "catppuccin",

		Mantle:  lipgloss.Color("#181825"),
		Surface: lipgloss.Color("#313244"),
		Crust:   lipgloss.Color("#11111b"),

		Text:    lipgloss.Color("#cdd6f4"),
		Subtext: lipgloss.Color("#a6adc8"),
		Muted:   lipgloss.Color("#6c7086"),

		Border:        lipgloss.Color("#45475a"),
		BorderFocused: lipgloss.Color("#89b4fa"),
		Selection:     lipgloss.Color("#313244"),

		Accent:    lipgloss.Color("#89b4fa"),
		Secondary: lipgloss.Color("#cba6f7"),

		Success: lipgloss.Color("#a6e3a1"),
		Warning: lipgloss.Color("#f9e2af"),
		Error:   lipgloss.Color("#f38ba8"),
		Info:    lipgloss.Color("#89dceb"),

		Running:  lipgloss.Color("#a6e3a1"),
		Starting: lipgloss.Color("#f9e2af"),
		Stopping: lipgloss.Color("#fab387"),
		Stopped:  lipgloss.Color("#6c7086"),
		Crashed:  lipgloss.Color("#f38ba8"),

		LogInfo:  lipgloss.Color("#cdd6f4"),
		LogWarn:  lipgloss.Color("#f9e2af"),
		LogError: lipgloss.Color("#f38ba8"),
		LogDebug: lipgloss.Color("#6c7086"),
		LogTime:  lipgloss.Color("#585b70"),
		LogChat:  lipgloss.Color("#94e2d5"),

		Dir:     lipgloss.Color("#89b4fa"),
		Jar:     lipgloss.Color("#fab387"),
		Archive: lipgloss.Color("#cba6f7"),
		Config:  lipgloss.Color("#a6e3a1"),
		Exec:    lipgloss.Color("#f38ba8"),
	}
}
