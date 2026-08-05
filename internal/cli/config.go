package cli

import (
	"fmt"

	"github.com/bx-team/irori/internal/confdiff"
	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/host"
	"github.com/bx-team/irori/internal/mcjars"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Compare the server's configs with the ones the core ships",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(configDiffCmd(), configImportCmd())
	return cmd
}

func configDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "List the keys that differ from the shipped defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, res, err := compareConfigs(cmd)
			if err != nil {
				return err
			}
			report(cmd, cfg, res)
			return nil
		},
	}
}

func configImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Declare those keys in " + config.FileName,
		Long: "Writes every key that differs from the core's own defaults into the\n" +
			"configs section of " + config.FileName + ", so `irori apply` — and a NixOS\n" +
			"host — put the same values back on top of a freshly generated config.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, res, err := compareConfigs(cmd)
			if err != nil {
				return err
			}
			report(cmd, cfg, res)
			if dry, _ := cmd.Flags().GetBool("dry-run"); dry || res.Empty() {
				return nil
			}
			confdiff.Declare(cfg, res.Changes)
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\ndeclared %d key(s) in %s\n", len(res.Changes), config.FileName)
			return nil
		},
	}
	cmd.Flags().Bool("dry-run", false, "print what would be declared without writing anything")
	return cmd
}

func compareConfigs(cmd *cobra.Command) (*config.Config, confdiff.Result, error) {
	cfg, err := mustConfig(cmd)
	if err != nil {
		return nil, confdiff.Result{}, err
	}
	if config.Sealed() {
		return nil, confdiff.Result{}, fmt.Errorf("sealed mode is on, irori may not fetch the shipped configs")
	}
	shipped, err := mcjars.New().Configs(cmd.Context(), cfg.Server.BuildID)
	if err != nil {
		return nil, confdiff.Result{}, err
	}
	return cfg, confdiff.Compare(host.NewLocal(cfg.Dir()), shipped), nil
}

func report(cmd *cobra.Command, cfg *config.Config, res confdiff.Result) {
	out := cmd.OutOrStdout()
	if len(res.Compared) == 0 {
		fmt.Fprintln(out, "none of the files this build ships are in the directory yet")
		return
	}
	if res.Empty() {
		fmt.Fprintf(out, "%d file(s) compared, everything matches what %s ships\n",
			len(res.Compared), cfg.Server.Type.Display())
	}
	for _, c := range res.Changes {
		declared := ""
		if cfg.HasOverride(c.File, c.Key) {
			declared = "  (already declared)"
		}
		fmt.Fprintf(out, "%-30s %-44s %s → %s%s\n", c.File, c.Key, or(c.Default, "«unset»"), c.Value, declared)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(out, "skipped %s %s: %s\n", s.File, s.Key, s.Reason)
	}
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
