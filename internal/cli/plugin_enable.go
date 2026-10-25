package cli

import (
	"fmt"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

func newPluginEnableCmd() *cobra.Command { return newPluginToggleCmd(true) }
func newPluginToggleCmd(enabled bool) *cobra.Command {
	verb := "disable"
	if enabled {
		verb = "enable"
	}
	return &cobra.Command{Use: verb + " <PLUGIN...>", Short: verb + " installed plugin packages", SilenceErrors: true, SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return diag.New(diag.Usage, "supply at least one installed plugin alias")
			}
			for _, alias := range args {
				if plugin.ValidateAlias(alias) != nil {
					return diag.New(diag.Usage, "supply exact installed plugin aliases")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on plugin enable/disable mutations")
			}
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			aliases, err := pluginadmin.SetEnabled(cmd.Context(), home, args, enabled)
			if err != nil {
				return err
			}
			state := "disabled"
			if enabled {
				state = "enabled"
			}
			for _, alias := range aliases {
				fmt.Fprintf(cmd.OutOrStdout(), "work: %s %s\n", state, alias)
			}
			return nil
		}}
}
