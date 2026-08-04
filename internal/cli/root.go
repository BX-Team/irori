package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/bx-team/irori/internal/config"
	"github.com/spf13/cobra"
)

var version = "dev"

func SetVersion(v string) { version = v }

func Execute() error {
	root := &cobra.Command{
		Use:           "irori",
		Short:         "TUI for managing a Minecraft server",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          func(cmd *cobra.Command, _ []string) error { return runTUI(cmd) },
	}
	root.PersistentFlags().StringP("dir", "C", "", "server directory (default: search upwards from the working directory)")

	root.AddCommand(
		daemonCmd(),
		startCmd(),
		stopCmd(),
		restartCmd(),
		killCmd(),
		statusCmd(),
		sendCmd(),
		logsCmd(),
		applyCmd(),
		nixCmd(),
		defaultsCmd(),
	)
	return root.Execute()
}

// loadConfig resolves the server directory from --dir or by walking up from the
// working directory.
func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return config.LoadFrom(wd)
	}
	return config.Load(dir)
}

func mustConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := loadConfig(cmd)
	if errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("%s not found. Run irori inside a server directory to set one up", config.FileName)
	}
	return cfg, err
}
