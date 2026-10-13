package cli

import (
	"github.com/spf13/cobra"
)

// Bare administration commands open pure, discoverable interactive hubs.
func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "plugin",
		Short:         "Install, inspect, enable, disable, update and uninstall plugins",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hubGuard(cmd, "run `work plugin list`, `work plugin install <source>` or `work plugin --help`"); err != nil {
				return err
			}
			return runHub(cmd, runPluginHub)
		},
	}
	cmd.AddCommand(newPluginInstallCmd(), newPluginListCmd(), newPluginEnableCmd(), newPluginDisableCmd(), newPluginUpdateCmd(), newPluginUninstallCmd())
	return cmd
}
