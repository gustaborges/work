package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
)

// newPluginMarketplaceCmd registers `work plugin marketplace`. Registration and
// refresh only change discovery and never touch installed plugins or policy;
// removal is the one whole-source operation and is reviewed as a set.
func newPluginMarketplaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "marketplace",
		Short:         "Register, list, refresh and remove plugin marketplaces",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          noArgsUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hubGuard(cmd, "run `work plugin marketplace add <source>`, `list`, `upgrade [name]` or `remove <name>`; see `work plugin marketplace --help`"); err != nil {
				return err
			}
			return runPluginHub(cmd, true)
		},
	}
	cmd.AddCommand(newMarketplaceAddCmd(), newMarketplaceListCmd(), newMarketplaceUpgradeCmd(), newMarketplaceRemoveCmd())
	return cmd
}

type marketplaceAddOutput struct {
	Name      string `json:"name"`
	Result    string `json:"result"`
	Revision  string `json:"revision"`
	Committed bool   `json:"committed"`
}

func newMarketplaceAddCmd() *cobra.Command {
	var branch string
	cmd := &cobra.Command{
		Use:           "add SOURCE [--branch BRANCH]",
		Short:         "Register a Git or local marketplace (installs nothing)",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return diag.New(diag.Usage, "supply exactly one marketplace SOURCE: a Git URL or a local directory")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Grammar first: nothing below touches state or the network on a usage error.
			spec, err := marketplaceadmin.Classify(args[0], branch)
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
			res, err := marketplaceadmin.Add(cmd.Context(), home, spec)
			if res.Name != "" && (err == nil || res.Committed) {
				if cmd.Flags().Changed("json") {
					if perr := printJSON(cmd, marketplaceAddOutput{Name: res.Name, Result: res.Result, Revision: res.Revision, Committed: res.Committed}); perr != nil && err == nil {
						err = perr
					}
				} else if res.Result == "unchanged" {
					fmt.Fprintf(cmd.OutOrStdout(), "work: unchanged %s\n", pluginadmin.SafeText(res.Name))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "work: registered %s (%s)\n", pluginadmin.SafeText(res.Name), pluginadmin.SafeText(res.Revision))
				}
			}
			return err
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "Git branch to follow (default: the remote's default branch, recorded at registration)")
	return cmd
}

func newMarketplaceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List known marketplaces with local installed totals",
		Args:          noArgsUsage,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, snapshot, err := loadPluginContext(cmd.Context())
			if err != nil {
				return err
			}
			rows := marketplaceadmin.List(snapshot)
			if cmd.Flags().Changed("json") {
				return printJSON(cmd, rows)
			}
			out := cmd.OutOrStdout()
			for _, r := range rows {
				name := r.Name
				if r.DisplayName != nil {
					name += " (" + *r.DisplayName + ")"
				}
				plugins := "unknown"
				if r.PluginCount != nil {
					plugins = fmt.Sprint(*r.PluginCount)
				}
				fmt.Fprintf(out, "%s  %s  %s  catalog: %s  plugins: %s  installed: %d  enabled: %d\n", name, r.SourceKind, r.Source, r.CatalogStatus, plugins, r.InstalledCount, r.EnabledCount)
			}
			return nil
		},
	}
}

type marketplaceUpgradeOutput struct {
	Results []marketplaceadmin.UpgradeResult `json:"results"`
}

func newMarketplaceUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "upgrade [MARKETPLACE]",
		Short:         "Refresh discovery from one or every marketplace (installed plugins are untouched)",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			rows, err := marketplaceadmin.Upgrade(cmd.Context(), home, args)
			if cmd.Flags().Changed("json") {
				if len(rows) > 0 || err == nil {
					if perr := printJSON(cmd, marketplaceUpgradeOutput{Results: nonNilRows(rows)}); perr != nil && err == nil {
						err = perr
					}
				}
				return err
			}
			out := cmd.OutOrStdout()
			for _, r := range rows {
				line := fmt.Sprintf("work: %s %s", r.Status, pluginadmin.SafeText(r.Name))
				if r.Status != marketplaceadmin.StatusFailed && r.Revision != nil {
					line += " (" + pluginadmin.SafeText(*r.Revision) + ")"
				}
				if r.Error != nil {
					line += " " + strings.TrimSpace(pluginadmin.SafeText(r.Error.Token))
				}
				fmt.Fprintln(out, line)
			}
			return err
		},
	}
}

func nonNilRows(r []marketplaceadmin.UpgradeResult) []marketplaceadmin.UpgradeResult {
	if r == nil {
		return []marketplaceadmin.UpgradeResult{}
	}
	return r
}

type marketplaceRemoveOutput struct {
	Name           string   `json:"name"`
	Result         string   `json:"result"`
	Removed        []string `json:"removed"`
	Committed      bool     `json:"committed"`
	CleanupPending bool     `json:"cleanup_pending,omitempty"`
}

// newMarketplaceRemoveCmd registers `work plugin marketplace remove`. It never
// contacts the source: the reviewed set is every plugin copied from it plus the
// registration and stored offer, all committed together.
func newMarketplaceRemoveCmd() *cobra.Command {
	var yes, removePolicy bool
	cmd := &cobra.Command{
		Use:           "remove MARKETPLACE",
		Short:         "Remove a marketplace and every plugin copied from it",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return diag.New(diag.Usage, "supply exactly one MARKETPLACE to remove")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := marketplaceadmin.CheckRemovable(args[0]); err != nil {
				return err
			}
			interactive := present.IsInteractive()
			if !interactive && !yes {
				return diag.New(diag.Usage, "non-interactive removal requires --yes")
			}
			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			plan, err := marketplaceadmin.PrepareRemove(cmd.Context(), home, args[0])
			if err != nil {
				return err
			}
			pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
			dependencies := plan.PolicyReferences()
			if len(dependencies) > 0 && !removePolicy {
				if !interactive {
					return diag.New(diag.PluginPolicyDependency, "removal drops policy Locators; explicitly approve --remove-policy-references")
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
				ok, cerr := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Remove marketplace", Impact: marketplaceRemovalImpact(plan, removePolicy), Accept: "Remove", Reject: "Cancel"})
				if cerr != nil {
					return cerr
				}
				if !ok {
					return diag.New(diag.Cancelled, "marketplace removal declined")
				}
			}
			res, err := plan.Apply(cmd.Context(), removePolicy)
			if !res.Committed {
				return err
			}
			ids := make([]string, 0, len(res.Removed))
			for _, p := range res.Removed {
				ids = append(ids, pluginadmin.SafeText(p.String()))
			}
			if cmd.Flags().Changed("json") {
				if perr := printJSON(cmd, marketplaceRemoveOutput{Name: pluginadmin.SafeText(res.Name), Result: res.Result, Removed: ids, Committed: true, CleanupPending: res.CleanupPending}); perr != nil && err == nil {
					err = perr
				}
				return err
			}
			for _, id := range ids {
				fmt.Fprintf(cmd.OutOrStdout(), "work: removed %s\n", id)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "work: removed marketplace %s\n", pluginadmin.SafeText(res.Name))
			return err
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm removing the reviewed marketplace and plugins (does not approve policy removal)")
	cmd.Flags().BoolVar(&removePolicy, "remove-policy-references", false, "approve removing exactly the reviewed policy references owned by those plugins")
	usageFlags(cmd, "run `work plugin marketplace remove <name> [--yes] [--remove-policy-references]`")
	return cmd
}

func marketplaceRemovalImpact(plan *marketplaceadmin.RemovePlan, removePolicy bool) string {
	var b strings.Builder
	src := plan.Source()
	fmt.Fprintf(&b, "Remove marketplace %s (%s)\n", pluginadmin.SafeText(src.Name), pluginadmin.SafeSource(src.Source))
	if ids := pluginadmin.ProviderNames(plan.Providers()); len(ids) > 0 {
		fmt.Fprintf(&b, "Plugins removed (including disabled or withdrawn): %s\n", pluginadmin.SafeText(strings.Join(ids, ", ")))
	} else {
		b.WriteString("No plugins are installed from this marketplace.\n")
	}
	if refs := plan.PolicyReferences(); len(refs) > 0 {
		fmt.Fprintf(&b, "Policy references removed:\n%s\n", safeRemovalReferences(refs))
	}
	for _, e := range plan.ConventionEffects() {
		fmt.Fprintf(&b, "Convention %s: available %t → %t\n", e.Name, e.AvailableBefore, e.AvailableAfter)
	}
	equivalent := []string{"plugin", "marketplace", "remove", src.Name, "--yes"}
	if removePolicy {
		equivalent = append(equivalent, "--remove-policy-references")
	}
	fmt.Fprintf(&b, "Equivalent: %s\n", safeCommand(equivalent))
	b.WriteString("Removal works offline. Existing Works and remembered conventions are retained.")
	return b.String()
}
