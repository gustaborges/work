package cli

import (
	"fmt"

	"github.com/gustaborges/work/internal/diag"
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
	return &cobra.Command{Use: verb + " <PLUGIN@MARKETPLACE...>", Short: verb + " installed plugins", SilenceErrors: true, SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			_, err := parseProviders(args)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on plugin enable/disable mutations")
			}
			providers, err := parseProviders(args)
			if err != nil {
				return err
			}
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			done, err := pluginadmin.SetEnabled(cmd.Context(), home, providers, enabled)
			if err != nil {
				return err
			}
			state := "disabled"
			if enabled {
				state = "enabled"
			}
			for _, p := range done {
				fmt.Fprintf(cmd.OutOrStdout(), "work: %s %s\n", state, pluginadmin.SafeText(p.String()))
			}
			return nil
		}}
}
