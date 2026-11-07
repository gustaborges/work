package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/pluginadmin"
)

// inventoryEnvelope is the one plugin-list JSON shape. All three arrays are
// always present and non-null; discovery rows arrive with the available view.
type inventoryEnvelope struct {
	Installed     []pluginadmin.Entry `json:"installed"`
	Available     []any               `json:"available"`
	CatalogErrors []any               `json:"catalog_errors"`
}

// newPluginListCmd registers `work plugin list [--json]`: read-only, mutates
// nothing.
func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List installed plugins",
		Args:          noArgsUsage,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, snapshot, err := loadPluginContext(cmd.Context())
			if err != nil {
				return err
			}
			entries := pluginadmin.Inventory(snapshot)

			if cmd.Flags().Changed("json") {
				return printJSON(cmd, inventoryEnvelope{Installed: entries, Available: []any{}, CatalogErrors: []any{}})
			}

			out := cmd.OutOrStdout()
			for _, e := range entries {
				state := "enabled"
				if !e.Enabled {
					state = "disabled"
				}
				fmt.Fprintf(out, "%s  %s  %s  version: %s  %s  %s\n", e.ID, e.SourceKind, e.Source, valueOrUnknown(e.Version), state, e.InspectionStatus)
				for _, c := range e.Components {
					fmt.Fprintf(out, "  %s (%s)\n", c.Name, c.Role)
				}
			}
			return nil
		},
	}
}
