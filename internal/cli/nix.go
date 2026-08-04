package cli

import (
	"fmt"
	"os"

	"github.com/bx-team/irori/internal/lock"
	"github.com/bx-team/irori/internal/nixgen"
	"github.com/spf13/cobra"
)

func nixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nix",
		Short: "Render the lock file as a Nix expression",
		Long: "Turns every URL and checksum in " + lockFileHint + " into fixed-output\n" +
			"derivations, so a NixOS host can build the server directory instead of\n" +
			"letting irori download anything at runtime.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			lf, err := lock.Load(cfg.LockPath())
			if err != nil {
				return err
			}

			out, warnings := nixgen.Generate(cfg, lf)
			for _, w := range warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %s\n", w.Target, w.Reason)
			}

			target, _ := cmd.Flags().GetString("output")
			if target == "" || target == "-" {
				_, err := fmt.Fprint(cmd.OutOrStdout(), out)
				return err
			}
			if err := os.WriteFile(target, []byte(out), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "-", "file to write to, or - for stdout")
	return cmd
}

const lockFileHint = ".irori.lock.json"
