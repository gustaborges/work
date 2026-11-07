package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

// loadPluginContext is pure and keeps inventory on one coherent snapshot.
func loadPluginContext(ctx context.Context) (workhome.Home, *adminstate.Snapshot, error) {
	return readAdministration(ctx)
}

// parseSelectors resolves PLUGIN[@MARKETPLACE] operands to exact providers
// before any state is read or any source is retrieved. A bare name always needs
// the shared -m/--marketplace, even when it would be unique; a qualified
// selector carries exactly one '@' and cannot be combined with -m, even if the
// values agree; a batch is all qualified or all bare. Duplicates collapse and
// the result is sorted, so the target set is deterministic.
func parseSelectors(args []string, marketplaceFlag string) ([]pluginid.Provider, error) {
	if len(args) == 0 {
		return nil, diag.New(diag.Usage, "supply at least one plugin as plugin@marketplace, or plugin names with --marketplace")
	}
	qualified := 0
	for _, a := range args {
		if strings.Contains(a, "@") {
			qualified++
		}
	}
	switch {
	case qualified != 0 && qualified != len(args):
		return nil, diag.New(diag.Usage, "do not mix qualified plugin@marketplace selectors with bare names")
	case qualified != 0 && marketplaceFlag != "":
		return nil, diag.New(diag.Usage, "a qualified plugin@marketplace selector cannot be combined with --marketplace")
	case qualified == 0 && marketplaceFlag == "":
		return nil, diag.New(diag.Usage, "name the marketplace: use plugin@marketplace or --marketplace")
	}
	out := make([]pluginid.Provider, 0, len(args))
	for _, a := range args {
		var p pluginid.Provider
		var err error
		if qualified != 0 {
			p, err = pluginid.ParseProvider(a)
		} else {
			p, err = pluginid.NewProvider(a, marketplaceFlag)
		}
		if err != nil {
			return nil, diag.New(diag.Usage, "supply plugins as plugin@marketplace; paths, URLs and manifest files are not plugin selectors")
		}
		out = append(out, p)
	}
	return pluginadmin.SortedProviders(out), nil
}

// parseOneSelector is parseSelectors for commands that take exactly one target.
func parseOneSelector(args []string, marketplaceFlag string) (pluginid.Provider, error) {
	if len(args) != 1 {
		return pluginid.Provider{}, diag.New(diag.Usage, "supply exactly one plugin as plugin@marketplace")
	}
	ps, err := parseSelectors(args, marketplaceFlag)
	if err != nil {
		return pluginid.Provider{}, err
	}
	return ps[0], nil
}

// addMarketplaceFlag registers the shared -m/--marketplace selector flag.
func addMarketplaceFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVarP(target, "marketplace", "m", "", "marketplace that the bare plugin names belong to")
}

// usageFlags turns flag-parse failures, including retired flags, into plain
// usage errors without translating them.
func usageFlags(cmd *cobra.Command, hint string) {
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return diag.Wrap(diag.Usage, err, "invalid flags; "+hint)
	})
}

// valueOrUnknown renders a nullable inventory string.
func valueOrUnknown(v *string) string {
	if v == nil {
		return "unknown"
	}
	return *v
}
