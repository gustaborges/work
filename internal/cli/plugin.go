package cli

import (
	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
)

// newPluginCmd registers the `work plugin` parent: the non-interactive
// surface for installing and listing plugin packages installed outside the
// core binary (ADR-0002, ADR-0003). With no subcommand it prints grouped
// help for install/list and exits 0 in every stream configuration —
// structurally identical to `work repository`'s F3 precedent (research R17):
// `enable`/`disable`/`update`/`uninstall` and the interactive `work plugin`
// hub are deferred to F7 (spec Out of Scope).
func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "plugin",
		Short:         "Install and list plugin packages",
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
	cmd.AddCommand(newPluginInstallCmd(), newPluginListCmd())
	return cmd
}
