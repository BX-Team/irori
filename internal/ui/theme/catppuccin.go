package theme

import "github.com/charmbracelet/lipgloss"

func CatppuccinMocha() Theme {
	return Theme{
		Name: "catppuccin-mocha",

		Base:    lipgloss.Color("#1e1e2e"),
		Mantle:  lipgloss.Color("#181825"),
		Crust:   lipgloss.Color("#11111b"),
		Surface: lipgloss.Color("#313244"),
		Overlay: lipgloss.Color("#6c7086"),

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

func CatppuccinLatte() Theme {
	return Theme{
		Name: "catppuccin-latte",

		Base:    lipgloss.Color("#eff1f5"),
		Mantle:  lipgloss.Color("#e6e9ef"),
		Crust:   lipgloss.Color("#dce0e8"),
		Surface: lipgloss.Color("#ccd0da"),
		Overlay: lipgloss.Color("#9ca0b0"),

		Text:    lipgloss.Color("#4c4f69"),
		Subtext: lipgloss.Color("#6c6f85"),
		Muted:   lipgloss.Color("#9ca0b0"),

		Border:        lipgloss.Color("#bcc0cc"),
		BorderFocused: lipgloss.Color("#1e66f5"),
		Selection:     lipgloss.Color("#ccd0da"),

		Accent:    lipgloss.Color("#1e66f5"),
		Secondary: lipgloss.Color("#8839ef"),

		Success: lipgloss.Color("#40a02b"),
		Warning: lipgloss.Color("#df8e1d"),
		Error:   lipgloss.Color("#d20f39"),
		Info:    lipgloss.Color("#04a5e5"),

		Running:  lipgloss.Color("#40a02b"),
		Starting: lipgloss.Color("#df8e1d"),
		Stopping: lipgloss.Color("#fe640b"),
		Stopped:  lipgloss.Color("#9ca0b0"),
		Crashed:  lipgloss.Color("#d20f39"),

		LogInfo:  lipgloss.Color("#4c4f69"),
		LogWarn:  lipgloss.Color("#df8e1d"),
		LogError: lipgloss.Color("#d20f39"),
		LogDebug: lipgloss.Color("#9ca0b0"),
		LogTime:  lipgloss.Color("#acb0be"),
		LogChat:  lipgloss.Color("#179299"),

		Dir:     lipgloss.Color("#1e66f5"),
		Jar:     lipgloss.Color("#fe640b"),
		Archive: lipgloss.Color("#8839ef"),
		Config:  lipgloss.Color("#40a02b"),
		Exec:    lipgloss.Color("#d20f39"),
	}
}
