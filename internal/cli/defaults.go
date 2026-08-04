package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/spf13/cobra"
)

func defaultsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defaults [file...]",
		Short: "Show or restore the configs the core ships with",
		Long: "Without arguments this lists the configuration files the installed build\n" +
			"provides. Given one or more paths it writes those files back to their\n" +
			"pristine contents, which is the quickest way out of a config a server\n" +
			"refuses to start from.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			if config.Sealed() {
				return fmt.Errorf("sealed mode is on, irori may not fetch the shipped configs")
			}

			configs, err := mcjars.New().Configs(cmd.Context(), cfg.Server.BuildID)
			if err != nil {
				return err
			}
			if len(configs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "this build ships no configuration files")
				return nil
			}

			out := cmd.OutOrStdout()
			if len(args) == 0 {
				fmt.Fprintf(out, "%s %s %s ships:\n\n",
					cfg.Server.Type.Display(), cfg.Server.MCVersion, cfg.Server.Build)
				for _, c := range configs {
					fmt.Fprintf(out, "  %-34s %-11s %s\n", c.Location, strings.ToLower(c.Format), status(cfg, c))
				}
				fmt.Fprintf(out, "\nRestore one with: irori defaults %s\n", configs[0].Location)
				return nil
			}

			force, _ := cmd.Flags().GetBool("force")
			h := host.NewLocal(cfg.Dir())
			for _, name := range args {
				c, ok := mcjars.FindConfig(configs, name)
				if !ok {
					return fmt.Errorf("%s is not one of this build's configuration files", name)
				}
				if !force {
					if _, err := h.Stat(c.Location); err == nil {
						backup := c.Location + ".irori.bak"
						if err := h.Copy(c.Location, backup); err != nil {
							return err
						}
						fmt.Fprintf(out, "kept a copy at %s\n", backup)
					}
				}
				if err := h.WriteFile(c.Location, []byte(c.Value), 0o644); err != nil {
					return err
				}
				fmt.Fprintf(out, "restored %s\n", c.Location)
			}
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite without keeping a .irori.bak copy")
	return cmd
}

func status(cfg *config.Config, c mcjars.Config) string {
	path := filepath.Join(cfg.Dir(), filepath.FromSlash(c.Location))
	raw, err := os.ReadFile(path)
	switch {
	case err != nil:
		return "not present"
	case string(raw) == c.Value:
		return "unchanged"
	default:
		return "modified"
	}
}
