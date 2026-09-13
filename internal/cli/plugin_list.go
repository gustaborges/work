package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/registry"
)

// pluginListEntry is the `work plugin list --json` shape for one package
// (contracts/cli-work-plugin.md §work plugin list).
type pluginListEntry struct {
	Alias       string                `json:"alias"`
	Origin      string                `json:"origin"`
	Reference   string                `json:"reference"`
	Components  []pluginListComponent `json:"components"`
	Conventions []string              `json:"conventions"`
}

type pluginListComponent struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

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
			_, reg, err := loadPluginContext()
			if err != nil {
				return err
			}

			pkgs := reg.ListPackages()
			entries := make([]pluginListEntry, len(pkgs))
			for i, p := range pkgs {
				comps := componentsByAlias(reg, p.Alias)
				jsonComps := make([]pluginListComponent, len(comps))
				for j, c := range comps {
					jsonComps[j] = pluginListComponent{Name: c.Name, Role: c.Role}
				}
				conventions := p.Conventions
				if conventions == nil {
					conventions = []string{}
				}
				entries[i] = pluginListEntry{
					Alias: p.Alias, Origin: p.Origin, Reference: p.Reference,
					Components: jsonComps, Conventions: conventions,
				}
			}

			if cmd.Flags().Changed("json") {
				return printJSON(cmd, entries)
			}

			out := cmd.OutOrStdout()
			for _, e := range entries {
				fmt.Fprintf(out, "%s  %s  %s\n", e.Alias, e.Origin, e.Reference)
				for _, c := range e.Components {
					fmt.Fprintf(out, "  %s (%s)\n", c.Name, c.Role)
				}
			}
			return nil
		},
	}
}

// componentsByAlias returns every component registered by alias, in
// manifest order.
func componentsByAlias(reg *registry.Registry, alias string) []registry.Component {
	var out []registry.Component
	for _, c := range reg.Components {
		if c.Alias == alias {
			out = append(out, c)
		}
	}
	return out
}
