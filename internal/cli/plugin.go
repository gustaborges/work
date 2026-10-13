package cli

import (
	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
)

// newPluginCmd exposes the direct M3 package controls. The full interactive
// lifecycle hub is delivered by the later hub milestone.
func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "plugin",
		Short:         "Install, inspect, enable, disable and update plugin packages",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on `work plugin` (it prints help)")
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(newPluginInstallCmd(), newPluginListCmd(), newPluginEnableCmd(), newPluginDisableCmd(), newPluginUpdateCmd())
	return cmd
}
