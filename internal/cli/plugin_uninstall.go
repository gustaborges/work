package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

func newPluginUninstallCmd() *cobra.Command {
	var yes, remove bool
	cmd := &cobra.Command{Use: "uninstall <PLUGIN@MARKETPLACE...>", Short: "Remove a reviewed set of installed plugins", SilenceErrors: true, SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			_, err := parseProviders(args)
			return err
		}, RunE: func(cmd *cobra.Command, args []string) error {
			providers, err := parseProviders(args)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on plugin uninstall")
			}
			interactive := present.IsInteractive()
			if !interactive && !yes {
				return diag.New(diag.Usage, "noninteractive removal requires explicit plugins and --yes")
			}
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			plan, err := pluginadmin.PlanUninstall(cmd.Context(), home, providers)
			if err != nil {
				return err
			}
			pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
			dependencies := plan.PolicyReferences()
			if len(dependencies) > 0 && !remove {
				if !interactive {
					return diag.New(diag.PluginPolicyDependency, "removal drops policy Locators; explicitly approve --remove-policy-references")
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
				fmt.Fprintf(&impact, "Remove plugins: %s\n", strings.Join(pluginadmin.ProviderNames(plan.Providers()), ", "))
				if len(dependencies) > 0 {
					fmt.Fprintf(&impact, "Policy references removed:\n%s\n", safeRemovalReferences(dependencies))
				}
				for _, effect := range plan.ConventionEffects() {
					fmt.Fprintf(&impact, "Convention %s: available %t → %t\n", effect.Name, effect.AvailableBefore, effect.AvailableAfter)
				}
				for _, p := range plan.Providers() {
					root, _ := home.PackageRoot(p)
					fmt.Fprintf(&impact, "Owned content removed: %s\n", pluginadmin.SafeText(filepath.Clean(root)))
				}
				equivalent := append([]string{"plugin", "uninstall"}, pluginadmin.ProviderNames(plan.Providers())...)
				equivalent = append(equivalent, "--yes")
				if remove {
					equivalent = append(equivalent, "--remove-policy-references")
				}
				fmt.Fprintf(&impact, "Equivalent: %s\n", safeCommand(equivalent))
				impact.WriteString("Remembered conventions and existing Works are retained. The marketplace source and sibling plugins are untouched.")
				accepted, err := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Uninstall plugins", Impact: impact.String(), Accept: "Uninstall", Reject: "Cancel"})
				if err != nil {
					return err
				}
				if !accepted {
					return diag.New(diag.Cancelled, "plugin uninstall declined")
				}
			}
			removed, err := pluginadmin.Uninstall(cmd.Context(), home, plan, remove)
			for _, p := range removed {
				fmt.Fprintf(cmd.OutOrStdout(), "work: removed %s\n", pluginadmin.SafeText(p.String()))
			}
			return err
		}}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm uninstalling the selected plugins (does not approve dependency removal)")
	cmd.Flags().BoolVar(&remove, "remove-policy-references", false, "approve removing exactly the reviewed policy references owned by the selected plugins")
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return diag.Wrap(diag.Usage, err, "invalid uninstall flags; run `work plugin uninstall <plugin@marketplace...> [--yes] [--remove-policy-references]`")
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
