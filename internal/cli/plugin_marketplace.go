package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/workhome"
)

// newPluginMarketplaceCmd registers `work plugin marketplace`. Only registration,
// listing and refresh exist: registration installs nothing, and refresh never
// touches installed plugins or policy.
func newPluginMarketplaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "marketplace",
		Short:         "Register, list and refresh plugin marketplaces",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          noArgsUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			return hubGuard(cmd, "run `work plugin marketplace add <source>`, `list` or `upgrade [name]`; see `work plugin marketplace --help`")
		},
	}
	cmd.AddCommand(newMarketplaceAddCmd(), newMarketplaceListCmd(), newMarketplaceUpgradeCmd())
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
