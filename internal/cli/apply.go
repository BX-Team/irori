package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bx-team/irori/internal/apply"
	"github.com/bx-team/irori/internal/config"
	"github.com/spf13/cobra"
)

func applyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Bring the directory in line with .irori.json",
		Long: "Installs the declared core and plugins, removes plugins that are no longer\n" +
			"declared, and writes the declared configuration keys into their files.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			update, _ := cmd.Flags().GetBool("update")
			sealed, _ := cmd.Flags().GetBool("sealed")

			opts := apply.Options{DryRun: dryRun, Update: update, Sealed: sealed || config.Sealed()}
			if only, _ := cmd.Flags().GetString("only"); only != "" {
				switch apply.Kind(only) {
				case apply.KindCore, apply.KindPlugin, apply.KindConfig:
					opts.Only = apply.Kind(only)
				default:
					return fmt.Errorf("--only takes core, plugin or config, not %q", only)
				}
			}
			if from, _ := cmd.Flags().GetString("overrides"); from != "" {
				values, err := readOverrides(from)
				if err != nil {
					return err
				}
				opts.Configs = values
			}
			res, err := apply.Run(cmd.Context(), cfg, opts, func(s apply.Step) {
				fmt.Fprintln(cmd.OutOrStdout(), s)
			})

			switch {
			case res.Changed == 0 && res.Failed == 0:
				fmt.Fprintln(cmd.OutOrStdout(), "already up to date")
			case dryRun:
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d change(s) would be made\n", res.Changed)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d change(s) applied\n", res.Changed)
			}
			return err
		},
	}
	cmd.Flags().Bool("dry-run", false, "print what would change without touching anything")
	cmd.Flags().Bool("update", false, "re-resolve plugins that are declared without a pinned version")
	cmd.Flags().Bool("sealed", false, "fail instead of downloading, for Nix-managed deployments")
	cmd.Flags().String("only", "", "run one kind of step only: core, plugin or config")
	cmd.Flags().String("overrides", "", "JSON file of config keys to enforce instead of the ones in "+config.FileName)
	return cmd
}

func readOverrides(path string) (map[string]map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}
