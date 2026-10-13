package cli

import (
	"fmt"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

func newPluginUninstallCmd() *cobra.Command {
	var yes, remove bool
	cmd := &cobra.Command{Use: "uninstall <PLUGIN...>", Short: "Uninstall a reviewed set of plugin packages", SilenceErrors: true, SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return diag.New(diag.Usage, "supply at least one exact installed plugin alias")
			}
			for _, a := range args {
				if plugin.ValidateAlias(a) != nil {
					return diag.New(diag.Usage, "supply exact installed plugin aliases")
				}
			}
			return nil
		}, RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on plugin uninstall")
			}
			interactive := present.IsInteractive()
			if !interactive && !yes {
				return diag.New(diag.Usage, "noninteractive uninstall requires explicit aliases and --yes")
			}
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			plan, err := pluginadmin.PlanUninstall(cmd.Context(), home, args)
			if err != nil {
				return err
			}
			pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
			dependencies := plan.PolicyReferences()
			if len(dependencies) > 0 && !remove {
				if !interactive {
					return diag.New(diag.PluginPolicyDependency, "uninstall removes policy Locators; explicitly approve --remove-policy-references")
				}
				approved, err := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Remove policy references", Impact: safeRemovalReferences(dependencies), Accept: "Remove references", Reject: "Cancel"})
				if err != nil {
					return err
				}
				if !approved {
					return diag.New(diag.Cancelled, "policy reference removal declined")
				}
				remove = true
			}
			if !yes {
				var impact strings.Builder
				fmt.Fprintf(&impact, "Uninstall plugins: %s\n", strings.Join(plan.Aliases(), ", "))
				if len(dependencies) > 0 {
					fmt.Fprintf(&impact, "Policy references removed:\n%s\n", safeRemovalReferences(dependencies))
				}
				for _, effect := range plan.ConventionEffects() {
					fmt.Fprintf(&impact, "Convention %s: available %t → %t; enabled owners: %s → %s\n", effect.Name, effect.AvailableBefore, effect.AvailableAfter, joinOrNone(effect.OwnersBefore), joinOrNone(effect.OwnersAfter))
				}
				impact.WriteString("Remembered conventions and existing Works are retained. Linked sources are retained.")
				accepted, err := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Uninstall plugins", Impact: impact.String(), Accept: "Uninstall", Reject: "Cancel"})
				if err != nil {
					return err
				}
				if !accepted {
					return diag.New(diag.Cancelled, "plugin uninstall declined")
				}
			}
			aliases, err := pluginadmin.Uninstall(cmd.Context(), home, plan, remove)
			for _, a := range aliases {
				fmt.Fprintf(cmd.OutOrStdout(), "work: uninstalled %s\n", a)
			}
			return err
		}}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm uninstalling the selected plugins (does not approve dependency removal)")
	cmd.Flags().BoolVar(&remove, "remove-policy-references", false, "approve removing exactly the reviewed policy references owned by the selected plugins")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return diag.Wrap(diag.Usage, err, "invalid uninstall flags; run `work plugin uninstall <alias...> [--yes] [--remove-policy-references]`")
	})
	return cmd
}
func safeRemovalReferences(refs []string) string {
	out := make([]string, len(refs))
	for i, ref := range refs {
		out[i] = pluginadmin.SafeText(ref)
	}
	return strings.Join(out, "\n")
}
