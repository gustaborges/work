package cli

import (
	"github.com/spf13/cobra"
)

// Bare administration commands open pure, discoverable interactive hubs.
func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "plugin",
		Short:         "Register marketplaces and add, inspect, enable, disable and remove plugins",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          noArgsUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hubGuard(cmd, "run `work plugin list`, `work plugin marketplace add <source>` or `work plugin --help`"); err != nil {
				return err
			}
			return runPluginHub(cmd, false)
		},
	}
	cmd.AddCommand(newPluginAddCmd(), newPluginListCmd(), newPluginRemoveCmd(), newPluginUpdateCmd(), newPluginEnableCmd(), newPluginDisableCmd(), newPluginMarketplaceCmd())
	return cmd
}
