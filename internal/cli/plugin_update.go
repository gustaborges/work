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

func newPluginUpdateCmd() *cobra.Command {
	var check, all, yes, remove bool
	cmd := &cobra.Command{Use: "update [PLUGIN...]", Short: "Check or apply reviewed plugin updates", SilenceErrors: true, SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			for _, a := range args {
				if plugin.ValidateAlias(a) != nil {
					return diag.New(diag.Usage, "supply exact installed plugin aliases")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if check && (all || yes || remove) || all && len(args) > 0 || !check && cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--check excludes --all, --yes and --remove-policy-references; --all excludes aliases; --json requires --check")
			}
			interactive := present.IsInteractive()
			if !check && !interactive && (!yes || !all && len(args) == 0) {
				return diag.New(diag.Usage, "noninteractive update requires explicit aliases or --all, and --yes")
			}
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if check {
				rows, checkErr := pluginadmin.Check(cmd.Context(), home, args)
				if rows == nil {
					return checkErr
				}
				if cmd.Flags().Changed("json") {
					if err := printJSON(cmd, map[string]any{"updates": rows}); err != nil {
						return err
					}
				} else {
					printUpdateChecks(cmd, rows)
				}
				return checkErr
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
			if len(args) == 0 && !all {
				_, s, err := readAdministration(cmd.Context())
				if err != nil {
					return err
				}
				options := []present.Option[string]{}
				for _, p := range s.Registry.ListPackages() {
					options = append(options, present.Option[string]{Value: p.Alias, Primary: pluginadmin.SafeText(p.Alias)})
				}
				if len(options) == 0 {
					return diag.New(diag.Usage, "no installed plugins to select; install a plugin first")
				}
				args, err = present.MultiSelect(cmd.Context(), pio, present.MultiSelectSpec[string]{Title: "Update plugins", Options: options, Filterable: true})
				if err != nil {
					return err
				}
				if len(args) == 0 {
					return diag.New(diag.Cancelled, "no plugins selected")
				}
			}
			plan, err := pluginadmin.PrepareUpdates(cmd.Context(), home, args, all)
			if err != nil {
				return err
			}
			defer plan.Close()
			if len(plan.Results) == 0 {
				return nil
			}
			dependencies := []string{}
			var impact strings.Builder
			for _, r := range plan.Results {
				fmt.Fprintf(&impact, "%s: %s\n  version: %s → %s\n  reference: %s → %s\n  digest: %s → %s\n", r.Alias, r.Status, updateValue(r.CurrentVersion), updateValue(r.CandidateVersion), updateValue(r.CurrentReference), updateValue(r.CandidateReference), updateValue(r.CurrentDigest), updateValue(r.CandidateDigest))
				for _, effect := range plan.ConventionEffects(r.Alias) {
					fmt.Fprintf(&impact, "  convention %s: available %t → %t; enabled owners: %s → %s\n", effect.Name, effect.AvailableBefore, effect.AvailableAfter, joinOrNone(effect.OwnersBefore), joinOrNone(effect.OwnersAfter))
				}
				added, withdrawn := plan.DeclarationChanges(r.Alias)
				if modified := plan.ModifiedDeclarations(r.Alias); len(modified) > 0 {
					fmt.Fprintf(&impact, "  changed: %s\n", strings.Join(modified, ", "))
				}
				if len(added) > 0 {
					fmt.Fprintf(&impact, "  added: %s\n", strings.Join(added, ", "))
				}
				if len(withdrawn) > 0 {
					fmt.Fprintf(&impact, "  withdrawn: %s\n", strings.Join(withdrawn, ", "))
				}
				if r.Status == "update-available" {
					dependencies = append(dependencies, r.PolicyReferencesRemoved...)
				}
			}
			if len(dependencies) > 0 && !remove {
				if !interactive {
					return diag.New(diag.PluginPolicyDependency, "update withdraws policy Locators; explicitly approve --remove-policy-references")
				}
				approved, err := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Remove policy references", Impact: strings.Join(dependencies, "\n"), Accept: "Remove references", Reject: "Cancel"})
				if err != nil {
					return err
				}
				if !approved {
					return diag.New(diag.Cancelled, "policy reference removal declined")
				}
				remove = true
			}
			if !yes {
				if len(dependencies) > 0 {
					fmt.Fprintf(&impact, "Policy references removed:\n%s\n", strings.Join(dependencies, "\n"))
				}
				equivalent := []string{"plugin", "update"}
				for _, result := range plan.Results {
					equivalent = append(equivalent, result.Alias)
				}
				equivalent = append(equivalent, "--yes")
				if remove {
					equivalent = append(equivalent, "--remove-policy-references")
				}
				fmt.Fprintf(&impact, "Equivalent: %s\n", safeCommand(equivalent))
				accepted, err := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Apply plugin updates", Impact: impact.String(), Accept: "Apply", Reject: "Cancel"})
				if err != nil {
					return err
				}
				if !accepted {
					return diag.New(diag.Cancelled, "plugin update declined")
				}
			}
			rows, applyErr := pluginadmin.Apply(cmd.Context(), home, plan, remove)
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "work: update %s %s\n", r.Alias, r.Status)
				if r.Error != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "work: update-error %s %s\n", r.Alias, r.Error.Token)
				}
			}
			return applyErr
		}}
	cmd.Flags().BoolVar(&check, "check", false, "inspect candidates without changing installed state")
	cmd.Flags().BoolVar(&all, "all", false, "apply to the complete installed plugin set")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm applying updates (does not select targets or approve dependency removal)")
	cmd.Flags().BoolVar(&remove, "remove-policy-references", false, "approve removing exactly the reviewed withdrawn Locator references")
	return cmd
}
func updateValue(v *string) string {
	if v == nil {
		return "unknown"
	}
	return *v
}
func printUpdateChecks(cmd *cobra.Command, rows []pluginadmin.UpdateResult) {
	for _, r := range rows {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  version: %s → %s  reference: %s → %s  digest: %s → %s\n", r.Alias, r.Origin, r.Status, updateValue(r.CurrentVersion), updateValue(r.CandidateVersion), updateValue(r.CurrentReference), updateValue(r.CandidateReference), updateValue(r.CurrentDigest), updateValue(r.CandidateDigest))
		if len(r.PolicyReferencesRemoved) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  policy references removed: %s\n", strings.Join(r.PolicyReferencesRemoved, ", "))
		}
		if r.Error != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", r.Error.Token, r.Error.Message)
		}
	}
}
