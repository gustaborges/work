package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
)

// inventoryEnvelope is the one plugin-list JSON shape. All three arrays are
// always present and non-null; Available is populated only for --available.
type inventoryEnvelope struct {
	Installed     []pluginadmin.Entry        `json:"installed"`
	Available     []pluginadmin.Offered      `json:"available"`
	CatalogErrors []pluginadmin.CatalogError `json:"catalog_errors"`
}

// newPluginListCmd registers `work plugin list`: a pure read of one coherent
// snapshot. It never contacts a source, repairs a catalog or writes state.
func newPluginListCmd() *cobra.Command {
	var mkt string
	var available bool
	cmd := &cobra.Command{
		Use:           "list [-m MARKETPLACE] [--available --json]",
		Short:         "List installed plugins",
		Args:          noArgsUsage,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut := cmd.Flags().Changed("json")
			if available && !jsonOut {
				return diag.New(diag.Usage, "--available requires --json")
			}
			_, snapshot, err := loadPluginContext(cmd.Context())
			if err != nil {
				return err
			}
			if mkt != "" && !pluginadmin.KnownNamespace(snapshot, mkt) {
				return diag.Newf(diag.Usage, "marketplace %q is not known", mkt)
			}
			entries := pluginadmin.FilterInstalled(pluginadmin.Inventory(snapshot), mkt)

			if jsonOut {
				env := inventoryEnvelope{Installed: entries, Available: []pluginadmin.Offered{}, CatalogErrors: []pluginadmin.CatalogError{}}
				if available {
					env.Available, env.CatalogErrors = pluginadmin.Available(snapshot, mkt)
				}
				if err := printJSON(cmd, env); err != nil {
					return err
				}
				if len(env.CatalogErrors) > 0 {
					return diag.New(diag.MarketplaceEntryUnavailable, "a marketplace's stored offer is missing or inconsistent; see catalog_errors")
				}
				return nil
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
	addMarketplaceFlag(cmd, &mkt)
	cmd.Flags().BoolVar(&available, "available", false, "include offered plugins from stored catalogs (requires --json)")
	usageFlags(cmd, "run `work plugin list [-m MARKETPLACE] [--available --json]`")
	return cmd
}
