package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type User struct {
	Theme        string `json:"theme"`
	Mouse        bool   `json:"mouse"`
	Editor       string `json:"editor,omitempty"`
	ConsoleLines int    `json:"consoleLines"`
}

func DefaultUser() *User {
	return &User{
		Theme:        "catppuccin-mocha",
		Mouse:        true,
		ConsoleLines: 5000,
	}
}

func userPath() string { return filepath.Join(UserConfigDir(), "config.json") }

func LoadUser() *User {
	u := DefaultUser()
	raw, err := os.ReadFile(userPath())
	if err != nil {
		return u
	}
	_ = json.Unmarshal(raw, u)
	if u.ConsoleLines <= 0 {
		u.ConsoleLines = 5000
	}
	if u.Theme == "" {
		u.Theme = "catppuccin-mocha"
	}
	return u
}

func (u *User) Save() error {
	if err := os.MkdirAll(UserConfigDir(), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(userPath(), append(raw, '\n'), 0o644)
}

// EditorCommand resolves the external editor, preferring the user setting then
// the usual environment variables.
func (u *User) EditorCommand() string {
	for _, c := range []string{u.Editor, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if c != "" {
			return c
		}
	}
	return "nano"
}
