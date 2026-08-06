package theme

import "charm.land/lipgloss/v2"

func TokyoNight() Theme {
	return Theme{
		Name: "tokyonight",

		Mantle:  lipgloss.Color("#16161e"),
		Surface: lipgloss.Color("#292e42"),
		Crust:   lipgloss.Color("#1a1b26"),

		Text:    lipgloss.Color("#c0caf5"),
		Subtext: lipgloss.Color("#a9b1d6"),
		Muted:   lipgloss.Color("#565f89"),

		Border:        lipgloss.Color("#3b4261"),
		BorderFocused: lipgloss.Color("#7aa2f7"),
		Selection:     lipgloss.Color("#283457"),

		Accent:    lipgloss.Color("#7aa2f7"),
		Secondary: lipgloss.Color("#bb9af7"),

		Success: lipgloss.Color("#9ece6a"),
		Warning: lipgloss.Color("#e0af68"),
		Error:   lipgloss.Color("#f7768e"),
		Info:    lipgloss.Color("#7dcfff"),

		Running:  lipgloss.Color("#9ece6a"),
		Starting: lipgloss.Color("#e0af68"),
		Stopping: lipgloss.Color("#ff9e64"),
		Stopped:  lipgloss.Color("#565f89"),
		Crashed:  lipgloss.Color("#f7768e"),

		LogInfo:  lipgloss.Color("#c0caf5"),
		LogWarn:  lipgloss.Color("#e0af68"),
		LogError: lipgloss.Color("#f7768e"),
		LogDebug: lipgloss.Color("#565f89"),
		LogTime:  lipgloss.Color("#414868"),
		LogChat:  lipgloss.Color("#73daca"),

		Dir:     lipgloss.Color("#7aa2f7"),
		Jar:     lipgloss.Color("#ff9e64"),
		Archive: lipgloss.Color("#bb9af7"),
		Config:  lipgloss.Color("#9ece6a"),
		Exec:    lipgloss.Color("#f7768e"),
	}
}

// Kanagawa is Hokusai's wave rendered as a terminal palette: warm ink base,
// muted autumn accents. The closest thing on this list to what "irori" means.
func Kanagawa() Theme {
	return Theme{
		Name: "kanagawa",

		Mantle:  lipgloss.Color("#16161d"),
		Surface: lipgloss.Color("#2a2a37"),
		Crust:   lipgloss.Color("#1f1f28"),

		Text:    lipgloss.Color("#dcd7ba"),
		Subtext: lipgloss.Color("#c8c093"),
		Muted:   lipgloss.Color("#727169"),

		Border:        lipgloss.Color("#363646"),
		BorderFocused: lipgloss.Color("#7e9cd8"),
		Selection:     lipgloss.Color("#2d4f67"),

		Accent:    lipgloss.Color("#7e9cd8"),
		Secondary: lipgloss.Color("#957fb8"),

		Success: lipgloss.Color("#98bb6c"),
		Warning: lipgloss.Color("#dca561"),
		Error:   lipgloss.Color("#e46876"),
		Info:    lipgloss.Color("#7fb4ca"),

		Running:  lipgloss.Color("#98bb6c"),
		Starting: lipgloss.Color("#dca561"),
		Stopping: lipgloss.Color("#ffa066"),
		Stopped:  lipgloss.Color("#727169"),
		Crashed:  lipgloss.Color("#e46876"),

		LogInfo:  lipgloss.Color("#dcd7ba"),
		LogWarn:  lipgloss.Color("#dca561"),
		LogError: lipgloss.Color("#e46876"),
		LogDebug: lipgloss.Color("#727169"),
		LogTime:  lipgloss.Color("#54546d"),
		LogChat:  lipgloss.Color("#7aa89f"),

		Dir:     lipgloss.Color("#7e9cd8"),
		Jar:     lipgloss.Color("#ffa066"),
		Archive: lipgloss.Color("#957fb8"),
		Config:  lipgloss.Color("#98bb6c"),
		Exec:    lipgloss.Color("#e46876"),
	}
}

func GruvboxDark() Theme {
	return Theme{
		Name: "gruvbox-dark",

		Mantle:  lipgloss.Color("#1d2021"),
		Surface: lipgloss.Color("#3c3836"),
		Crust:   lipgloss.Color("#282828"),

		Text:    lipgloss.Color("#ebdbb2"),
		Subtext: lipgloss.Color("#d5c4a1"),
		Muted:   lipgloss.Color("#928374"),

		Border:        lipgloss.Color("#504945"),
		BorderFocused: lipgloss.Color("#83a598"),
		Selection:     lipgloss.Color("#3c3836"),

		Accent:    lipgloss.Color("#83a598"),
		Secondary: lipgloss.Color("#d3869b"),

		Success: lipgloss.Color("#b8bb26"),
		Warning: lipgloss.Color("#fabd2f"),
		Error:   lipgloss.Color("#fb4934"),
		Info:    lipgloss.Color("#8ec07c"),

		Running:  lipgloss.Color("#b8bb26"),
		Starting: lipgloss.Color("#fabd2f"),
		Stopping: lipgloss.Color("#fe8019"),
		Stopped:  lipgloss.Color("#928374"),
		Crashed:  lipgloss.Color("#fb4934"),

		LogInfo:  lipgloss.Color("#ebdbb2"),
		LogWarn:  lipgloss.Color("#fabd2f"),
		LogError: lipgloss.Color("#fb4934"),
		LogDebug: lipgloss.Color("#928374"),
		LogTime:  lipgloss.Color("#665c54"),
		LogChat:  lipgloss.Color("#8ec07c"),

		Dir:     lipgloss.Color("#83a598"),
		Jar:     lipgloss.Color("#fe8019"),
		Archive: lipgloss.Color("#d3869b"),
		Config:  lipgloss.Color("#b8bb26"),
		Exec:    lipgloss.Color("#fb4934"),
	}
}

// RosePineMoon has no green at all, so foam (its cyan) carries "running" and
// pine carries "info" — swapping in an off-palette green is what makes ports of
// this theme look wrong.
func RosePineMoon() Theme {
	return Theme{
		Name: "rose-pine-moon",

		Mantle:  lipgloss.Color("#2a273f"),
		Surface: lipgloss.Color("#393552"),
		Crust:   lipgloss.Color("#232136"),

		Text:    lipgloss.Color("#e0def4"),
		Subtext: lipgloss.Color("#908caa"),
		Muted:   lipgloss.Color("#6e6a86"),

		Border:        lipgloss.Color("#44415a"),
		BorderFocused: lipgloss.Color("#c4a7e7"),
		Selection:     lipgloss.Color("#393552"),

		Accent:    lipgloss.Color("#c4a7e7"),
		Secondary: lipgloss.Color("#ea9a97"),

		Success: lipgloss.Color("#9ccfd8"),
		Warning: lipgloss.Color("#f6c177"),
		Error:   lipgloss.Color("#eb6f92"),
		Info:    lipgloss.Color("#3e8fb0"),

		Running:  lipgloss.Color("#9ccfd8"),
		Starting: lipgloss.Color("#f6c177"),
		Stopping: lipgloss.Color("#ea9a97"),
		Stopped:  lipgloss.Color("#6e6a86"),
		Crashed:  lipgloss.Color("#eb6f92"),

		LogInfo:  lipgloss.Color("#e0def4"),
		LogWarn:  lipgloss.Color("#f6c177"),
		LogError: lipgloss.Color("#eb6f92"),
		LogDebug: lipgloss.Color("#6e6a86"),
		LogTime:  lipgloss.Color("#56526e"),
		LogChat:  lipgloss.Color("#3e8fb0"),

		// Pine is dark enough to read as dimmed text in a file list, so it stays
		// on Info and foam takes the config files.
		Dir:     lipgloss.Color("#c4a7e7"),
		Jar:     lipgloss.Color("#f6c177"),
		Archive: lipgloss.Color("#ea9a97"),
		Config:  lipgloss.Color("#9ccfd8"),
		Exec:    lipgloss.Color("#eb6f92"),
	}
}

func Nord() Theme {
	return Theme{
		Name: "nord",

		Mantle:  lipgloss.Color("#3b4252"),
		Surface: lipgloss.Color("#434c5e"),
		Crust:   lipgloss.Color("#242933"),

		Text:    lipgloss.Color("#eceff4"),
		Subtext: lipgloss.Color("#d8dee9"),
		Muted:   lipgloss.Color("#616e88"),

		Border:        lipgloss.Color("#4c566a"),
		BorderFocused: lipgloss.Color("#88c0d0"),
		Selection:     lipgloss.Color("#434c5e"),

		Accent:    lipgloss.Color("#88c0d0"),
		Secondary: lipgloss.Color("#b48ead"),

		Success: lipgloss.Color("#a3be8c"),
		Warning: lipgloss.Color("#ebcb8b"),
		Error:   lipgloss.Color("#bf616a"),
		Info:    lipgloss.Color("#8fbcbb"),

		Running:  lipgloss.Color("#a3be8c"),
		Starting: lipgloss.Color("#ebcb8b"),
		Stopping: lipgloss.Color("#d08770"),
		Stopped:  lipgloss.Color("#616e88"),
		Crashed:  lipgloss.Color("#bf616a"),

		LogInfo:  lipgloss.Color("#eceff4"),
		LogWarn:  lipgloss.Color("#ebcb8b"),
		LogError: lipgloss.Color("#bf616a"),
		LogDebug: lipgloss.Color("#616e88"),
		LogTime:  lipgloss.Color("#4c566a"),
		LogChat:  lipgloss.Color("#8fbcbb"),

		Dir:     lipgloss.Color("#81a1c1"),
		Jar:     lipgloss.Color("#d08770"),
		Archive: lipgloss.Color("#b48ead"),
		Config:  lipgloss.Color("#a3be8c"),
		Exec:    lipgloss.Color("#bf616a"),
	}
}
