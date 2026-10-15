package pluginadmin

import (
	"context"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/workhome"
)

// SetEnabled validates the complete exact-alias set before changing any
// package. A satisfied request does not rewrite its bytes or publish an epoch.
func SetEnabled(ctx context.Context, h workhome.Home, aliases []string, enabled bool) ([]string, error) {
	targets := slices.Clone(aliases)
	slices.Sort(targets)
	targets = slices.Compact(targets)
	if len(targets) == 0 {
		return nil, diag.New(diag.Usage, "supply at least one installed plugin alias")
	}
	for _, alias := range targets {
		if plugin.ValidateAlias(alias) != nil {
			return nil, diag.New(diag.Usage, "supply exact installed plugin aliases")
		}
	}
	verb := "disable"
	if enabled {
		verb = "enable"
	}
	err := adminstate.WithWrite(ctx, h, "work plugin "+verb+" "+strings.Join(targets, " "), func(s *adminstate.Session) error {
		for _, alias := range targets {
			if _, ok := s.Registry.PackageByAlias(alias); !ok {
				return diag.Newf(diag.Usage, "plugin alias %q is not installed", alias)
			}
		}
		for _, alias := range targets {
			if s.Registry.PackageEnabled(alias) == enabled {
				continue
			}
			p, _ := s.Registry.PackageByAlias(alias)
			value := enabled
			p.Enabled = &value
			s.Registry.UpsertPackage(p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return targets, nil
}
