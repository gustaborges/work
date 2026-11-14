package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
)

type pluginUpdateOutput struct {
	Updates []pluginadmin.UpdateResult `json:"updates"`
}

// newPluginUpdateCmd registers `work plugin update`. Candidates always come
// from each plugin's recorded source, never from discovery state or a name, and
// versions are compared only for equality.
func newPluginUpdateCmd() *cobra.Command {
	var check, all, yes, removePolicy bool
	var mkt string
	cmd := &cobra.Command{
		Use:           "update [PLUGIN[@MARKETPLACE]...] [-m MARKETPLACE]",
		Short:         "Check or apply reviewed plugin updates",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut := cmd.Flags().Changed("json")
			switch {
			case check && (all || yes || removePolicy):
				return diag.New(diag.Usage, "--check excludes --all, --yes and --remove-policy-references")
			case all && (len(args) > 0 || mkt != ""):
				return diag.New(diag.Usage, "--all excludes plugin targets and --marketplace")
			case jsonOut && !check:
				return diag.New(diag.Usage, "--json requires --check")
			}
			var explicit []pluginid.Provider
			if len(args) > 0 {
				var err error
				if explicit, err = parseSelectors(args, mkt); err != nil {
					return err
				}
			}
			interactive := present.IsInteractive()
			if !check && !interactive && (!yes || (!all && len(explicit) == 0)) {
				return diag.New(diag.Usage, "non-interactive update requires explicit plugins or --all, and --yes")
			}
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if check {
				return runUpdateCheck(cmd, explicit, mkt, jsonOut)
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			targets := explicit
			if all || len(explicit) == 0 {
				_, s, err := readAdministration(cmd.Context())
				if err != nil {
					return err
				}
				targets = pluginadmin.InstalledProviders(s, mkt)
				if len(targets) == 0 {
					return diag.New(diag.Usage, "no installed plugins to update")
				}
				if !all {
					if targets, err = selectUpdateTargets(cmd, targets); err != nil {
						return err
					}
				}
			}
			return runUpdateApply(cmd, home, targets, yes, removePolicy)
		},
	}
	addMarketplaceFlag(cmd, &mkt)
	cmd.Flags().BoolVar(&check, "check", false, "inspect candidates without changing anything")
	cmd.Flags().BoolVar(&all, "all", false, "update every installed plugin")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm applying the reviewed updates (does not select targets or approve policy removal)")
	cmd.Flags().BoolVar(&removePolicy, "remove-policy-references", false, "approve removing exactly the reviewed policy references an update withdraws")
	usageFlags(cmd, "run `work plugin update [--check] [<plugin>@<marketplace>...] [--all] [--yes]`")
	return cmd
}

func runUpdateCheck(cmd *cobra.Command, explicit []pluginid.Provider, mkt string, jsonOut bool) error {
	targets := explicit
	if len(targets) == 0 {
		_, s, err := readAdministration(cmd.Context())
		if err != nil {
			return err
		}
		if targets = pluginadmin.InstalledProviders(s, mkt); len(targets) == 0 {
			if jsonOut {
				return printJSON(cmd, pluginUpdateOutput{Updates: []pluginadmin.UpdateResult{}})
			}
			return nil
		}
	}
	home, err := workhome.Resolve()
	if err != nil {
		return err
	}
	rows, checkErr := pluginadmin.Check(cmd.Context(), home, targets)
	if rows == nil {
		return checkErr
	}
	if jsonOut {
		if err := printJSON(cmd, pluginUpdateOutput{Updates: rows}); err != nil {
			return err
		}
		return checkErr
	}
	for _, r := range rows {
		fmt.Fprintln(cmd.OutOrStdout(), pluginadmin.UpdateSummary(r))
		if len(r.PolicyReferencesRemoved) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  policy references withdrawn: %s\n", strings.Join(r.PolicyReferencesRemoved, ", "))
		}
		if r.Error != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", r.Error.Token, r.Error.Message)
		}
	}
	return checkErr
}

func selectUpdateTargets(cmd *cobra.Command, installed []pluginid.Provider) ([]pluginid.Provider, error) {
	options := make([]present.Option[string], 0, len(installed))
	for _, p := range installed {
		options = append(options, present.Option[string]{Value: p.String(), Primary: pluginadmin.SafeText(p.String())})
	}
	pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
	chosen, err := present.MultiSelect(cmd.Context(), pio, present.MultiSelectSpec[string]{Title: "Update plugins", Options: options, Filterable: true})
	if err != nil {
		return nil, err
	}
	if len(chosen) == 0 {
		return nil, diag.New(diag.Cancelled, "no plugins selected")
	}
	out := make([]pluginid.Provider, 0, len(chosen))
	for _, id := range chosen {
		p, perr := pluginid.ParseProvider(id)
		if perr != nil {
			return nil, diag.New(diag.Usage, "choose installed plugins")
		}
		out = append(out, p)
	}
	return pluginadmin.SortedProviders(out), nil
}

func runUpdateApply(cmd *cobra.Command, home workhome.Home, targets []pluginid.Provider, yes, removePolicy bool) error {
	plan, err := pluginadmin.PrepareUpdates(cmd.Context(), home, targets, true)
	if err != nil {
		return err
	}
	defer plan.Close()

	var available []pluginadmin.UpdateResult
	var dependencies []string
	for _, r := range plan.Results {
		if r.Status == pluginadmin.UpdateAvailable {
			available = append(available, r)
			dependencies = append(dependencies, r.PolicyReferencesRemoved...)
		}
	}
	if len(available) > 0 {
		interactive := present.IsInteractive()
		pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
		if len(dependencies) > 0 && !removePolicy {
			if !interactive {
				return diag.New(diag.PluginPolicyDependency, "update withdraws policy Locators; explicitly approve --remove-policy-references")
			}
			ok, cerr := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Remove policy references", Impact: safeRemovalReferences(dependencies), Accept: "Remove references", Reject: "Cancel"})
			if cerr != nil {
				return cerr
			}
			if !ok {
				return diag.New(diag.Cancelled, "policy reference removal declined")
			}
			removePolicy = true
		}
		if !yes {
			ok, cerr := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Apply plugin updates", Impact: updateImpact(plan, available, dependencies, removePolicy), Accept: "Apply", Reject: "Cancel"})
			if cerr != nil {
				return cerr
			}
			if !ok {
				return diag.New(diag.Cancelled, "plugin update declined")
			}
		}
	}
	rows, applyErr := pluginadmin.ApplyUpdates(cmd.Context(), plan, removePolicy)
	if rows != nil {
		printUpdateResults(cmd, rows)
	}
	return applyErr
}

func updateImpact(plan *pluginadmin.UpdatePlan, available []pluginadmin.UpdateResult, dependencies []string, removePolicy bool) string {
	var b strings.Builder
	equivalent := []string{"plugin", "update"}
	for _, r := range available {
		b.WriteString(pluginadmin.UpdateSummary(r) + "\n")
		for _, group := range []struct {
			label string
			items []string
		}{{"added", r.Added}, {"withdrawn", r.Withdrawn}, {"changed", r.Changed}} {
			if len(group.items) > 0 {
				fmt.Fprintf(&b, "  %s: %s\n", group.label, strings.Join(group.items, ", "))
			}
		}
		for _, e := range plan.ConventionEffects(r.ID) {
			fmt.Fprintf(&b, "  convention %s: available %t → %t\n", e.Name, e.AvailableBefore, e.AvailableAfter)
		}
		equivalent = append(equivalent, r.ID)
	}
	if len(dependencies) > 0 {
		fmt.Fprintf(&b, "Policy references removed:\n%s\n", safeRemovalReferences(dependencies))
	}
	equivalent = append(equivalent, "--yes")
	if removePolicy {
		equivalent = append(equivalent, "--remove-policy-references")
	}
	fmt.Fprintf(&b, "Equivalent: %s\nEnablement is kept. Remembered conventions and existing Works are retained.", safeCommand(equivalent))
	return b.String()
}

func printUpdateResults(cmd *cobra.Command, rows []pluginadmin.UpdateResult) {
	for _, r := range rows {
		fmt.Fprintf(cmd.OutOrStdout(), "work: %s %s\n", r.Status, r.ID)
		if r.Error != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "work: %s %s %s\n", "update-error", r.ID, r.Error.Token)
		}
	}
}
