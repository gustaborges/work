package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
)

type pluginAddOutput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Marketplace   string `json:"marketplace"`
	Revision      string `json:"revision"`
	ContentDigest string `json:"content_digest"`
	Result        string `json:"result"`
	Committed     bool   `json:"committed"`
}

// newPluginAddCmd registers `work plugin add`. It copies exactly one offered
// entry of a registered marketplace; there is no way to name a path, URL or
// manifest here, because the only publisher is a resolved marketplace entry.
func newPluginAddCmd() *cobra.Command {
	var mkt string
	var yes, removePolicy bool
	cmd := &cobra.Command{
		Use:           "add <PLUGIN[@MARKETPLACE]> [-m MARKETPLACE]",
		Short:         "Copy one offered plugin into independent managed storage",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Grammar first: no state, source or network is touched on a usage error.
			provider, err := parseOneSelector(args, mkt)
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
			plan, err := pluginadmin.PrepareInstall(cmd.Context(), home, provider.Plugin, provider.Marketplace)
			if err != nil {
				return err
			}
			defer plan.Close()

			removePolicy, err = confirmReinstall(cmd, plan, yes, removePolicy)
			if err != nil {
				return err
			}
			res, err := plan.Apply(cmd.Context(), removePolicy)
			if err == nil || res.Committed {
				if perr := printAddResult(cmd, res); perr != nil && err == nil {
					err = perr
				}
			}
			return err
		},
	}
	addMarketplaceFlag(cmd, &mkt)
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm replacing an installed copy (does not approve policy removal)")
	cmd.Flags().BoolVar(&removePolicy, "remove-policy-references", false, "approve dropping the reviewed policy references a replacement would orphan")
	usageFlags(cmd, "run `work plugin add <plugin>@<marketplace> [--yes] [--remove-policy-references]`")
	return cmd
}

// confirmReinstall reviews a replacement. A first installation or an unchanged
// copy needs no confirmation; replacing an installed copy needs --yes or an
// interactive acceptance, and dropping policy references needs its own
// explicit authorization that --yes never grants.
func confirmReinstall(cmd *cobra.Command, plan *pluginadmin.InstallPlan, yes, removePolicy bool) (bool, error) {
	if plan.Result() != pluginadmin.ResultReinstalled {
		return removePolicy, nil
	}
	interactive := present.IsInteractive()
	if !interactive && !yes {
		return false, diag.New(diag.Usage, "replacing an installed plugin requires --yes in a non-interactive session")
	}
	pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
	if deps := plan.PolicyReferences(); len(deps) > 0 && !removePolicy {
		if !interactive {
			return false, diag.New(diag.PluginPolicyDependency, "reinstalling drops policy Locators; explicitly approve --remove-policy-references")
		}
		ok, err := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Remove policy references", Impact: safeRemovalReferences(deps), Accept: "Remove references", Reject: "Cancel"})
		if err != nil {
			return false, err
		}
		if !ok {
			return false, diag.New(diag.Cancelled, "policy reference removal declined")
		}
		removePolicy = true
	}
	if yes {
		return removePolicy, nil
	}
	var impact strings.Builder
	prior, _ := plan.Replacing()
	offered := plan.Offered()
	fmt.Fprintf(&impact, "Replace %s: version %s → %s\n", pluginadmin.SafeText(plan.Provider().String()), pluginadmin.SafeText(prior.Version), pluginadmin.SafeText(offered.Version))
	impact.WriteString("Enablement is kept. The marketplace source and sibling plugins are untouched.")
	ok, err := present.Confirm(cmd.Context(), pio, present.ConfirmSpec{Title: "Reinstall plugin", Impact: impact.String(), Accept: "Reinstall", Reject: "Cancel"})
	if err != nil {
		return false, err
	}
	if !ok {
		return false, diag.New(diag.Cancelled, "plugin reinstall declined")
	}
	return removePolicy, nil
}

func printAddResult(cmd *cobra.Command, r pluginadmin.InstallResult) error {
	if cmd.Flags().Changed("json") {
		return printJSON(cmd, pluginAddOutput{
			ID: pluginadmin.SafeText(r.Provider.String()), Name: pluginadmin.SafeText(r.Provider.Plugin),
			Marketplace: pluginadmin.SafeText(r.Provider.Marketplace), Revision: pluginadmin.SafeText(r.Revision),
			ContentDigest: pluginadmin.SafeText(r.ContentDigest), Result: r.Result, Committed: r.Committed || r.Result == pluginadmin.ResultUnchanged,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "work: %s %s\n", r.Result, pluginadmin.SafeText(r.Provider.String()))
	return nil
}
