package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/pluginadmin"
)

// newPluginListCmd registers `work plugin list [--json]`: read-only, mutates
// nothing (FR-034).
func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List installed plugin packages",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, snapshot, err := loadPluginContext(cmd.Context())
			if err != nil {
				return err
			}
			entries := pluginadmin.Inventory(snapshot)

			if cmd.Flags().Changed("json") {
				return printJSON(cmd, entries)
			}

			out := cmd.OutOrStdout()
			for _, e := range entries {
				version := "unknown"
				if e.Version != nil {
					version = *e.Version
				}
				state := "enabled"
				if !e.Enabled {
					state = "disabled"
				}
				fmt.Fprintf(out, "%s  %s  %s  version: %s  %s  %s\n", e.Alias, e.Origin, e.Reference, version, state, e.InspectionStatus)
				for _, c := range e.Components {
					fmt.Fprintf(out, "  %s (%s)\n", c.Name, c.Role)
				}
			}
			return nil
		},
	}
}
