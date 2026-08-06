package cli

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/ui"
	"github.com/bx-team/irori/internal/ui/wizard"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/spf13/cobra"
)

func runTUI(cmd *cobra.Command) error {
	cfg, err := loadConfig(cmd)
	if errors.Is(err, config.ErrNotFound) {
		return runSetup(cmd)
	}
	if err != nil {
		return err
	}
	return runApp(cfg)
}

func runApp(cfg *config.Config) error {
	if err := cfg.EnsureStateDir(); err != nil {
		return err
	}
	user := config.LoadUser()

	zone.NewGlobal()
	app := ui.New(cfg, user)
	defer app.Close()

	_, err := tea.NewProgram(app).Run()
	return err
}

func runSetup(cmd *cobra.Command) error {
	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = wd
	}

	user := config.LoadUser()
	cfg, err := wizard.Run(dir, user)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil
	}
	fmt.Printf("created %s\n", cfg.Path())
	return runApp(cfg)
}
