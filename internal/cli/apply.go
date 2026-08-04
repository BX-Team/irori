package cli

import (
	"fmt"

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
	return cmd
}
