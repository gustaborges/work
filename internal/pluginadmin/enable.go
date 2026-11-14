package pluginadmin

import (
	"context"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

// SortedProviders returns a deterministic, duplicate-free copy of targets.
func SortedProviders(targets []pluginid.Provider) []pluginid.Provider {
	out := slices.Clone(targets)
	slices.SortFunc(out, func(a, b pluginid.Provider) int {
		if c := strings.Compare(a.Marketplace, b.Marketplace); c != 0 {
			return c
		}
		return strings.Compare(a.Plugin, b.Plugin)
	})
	return slices.Compact(out)
}

// ProviderNames renders providers as plugin@marketplace.
func ProviderNames(ps []pluginid.Provider) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.String()
	}
	return out
}

// SetEnabled validates the complete exact-provider set before changing any
// package. A satisfied request does not rewrite its bytes or publish an epoch.
func SetEnabled(ctx context.Context, h workhome.Home, providers []pluginid.Provider, enabled bool) ([]pluginid.Provider, error) {
	targets := SortedProviders(providers)
	if len(targets) == 0 {
		return nil, diag.New(diag.Usage, "supply at least one installed plugin as plugin@marketplace")
	}
	for _, p := range targets {
		if p.Validate() != nil {
			return nil, diag.New(diag.Usage, "supply exact installed plugins as plugin@marketplace")
		}
	}
	verb := "disable"
	if enabled {
		verb = "enable"
	}
	err := adminstate.WithWrite(ctx, h, "work plugin "+verb+" "+strings.Join(ProviderNames(targets), " "), func(s *adminstate.Session) error {
		for _, p := range targets {
			if _, ok := s.Registry.PackageByProvider(p); !ok {
				return diag.Newf(diag.Usage, "plugin %q is not installed", p)
			}
		}
		for _, p := range targets {
			if s.Registry.PackageEnabled(p) == enabled {
				continue
			}
			pkg, _ := s.Registry.PackageByProvider(p)
			pkg.Enabled = enabled
			s.Registry.UpsertPackage(pkg)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return targets, nil
}
