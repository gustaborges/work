package pluginadmin

import (
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
)

// Offered is a stored offer projected for browsing: the fixed entry of a
// marketplace together with whether and how it is installed locally. It
// describes the stored offer, not current network truth.
type Offered struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Marketplace string            `json:"marketplace"`
	Version     string            `json:"version"`
	Revision    string            `json:"revision"`
	Description string            `json:"description"`
	Components  []Component       `json:"components"`
	Conventions []string          `json:"conventions"`
	Installed   bool              `json:"installed"`
	Enabled     *bool             `json:"enabled"`
	Entry       marketplace.Entry `json:"-"`
	Provider    pluginid.Provider `json:"-"`
}

// CatalogError reports a source whose stored offer is missing or inconsistent.
// The valid rows of other sources are still returned.
type CatalogError struct {
	Marketplace string `json:"marketplace"`
	Error       string `json:"error"`
	Message     string `json:"message"`
}

// Available projects every stored offer of the snapshot, optionally limited to
// one marketplace, with no network access and no write. The binary-owned
// provider is projected from the binary. A source whose stored catalog is
// missing or does not match its registration yields a CatalogError instead of
// rows and is never repaired or fetched here.
func Available(s *adminstate.Snapshot, only string) ([]Offered, []CatalogError) {
	return available(s, only, marketplace.BuiltinSnapshot)
}

// available is Available with the binary-owned provider injected, so the
// failure of the included offer can be exercised.
func available(s *adminstate.Snapshot, only string, builtin func() (*marketplace.Snapshot, error)) ([]Offered, []CatalogError) {
	rows := []Offered{}
	errs := []CatalogError{}
	add := func(namespace, revision string, entries []marketplace.Entry) {
		for _, e := range entries {
			p := pluginid.Provider{Marketplace: namespace, Plugin: e.Name}
			row := Offered{
				ID: SafeText(p.String()), Name: SafeText(e.Name), Marketplace: SafeText(namespace), Version: SafeText(e.Version),
				Revision: SafeText(revision), Description: SafeText(e.Description),
				Components: []Component{}, Conventions: []string{}, Entry: e, Provider: p,
			}
			for _, c := range e.Components {
				ref := pluginid.Ref{Provider: p, Name: c.Name}
				row.Components = append(row.Components, Component{ID: SafeText(ref.String()), Name: SafeText(c.Name), Role: SafeText(c.Role)})
			}
			for _, c := range e.Conventions {
				row.Conventions = append(row.Conventions, SafeText(pluginid.Ref{Provider: p, Name: c.Name}.String()))
			}
			if pkg, ok := s.Registry.PackageByProvider(p); ok {
				enabled := pkg.Enabled
				row.Installed, row.Enabled = true, &enabled
			}
			rows = append(rows, row)
		}
	}
	if only == "" || only == pluginid.Builtin {
		if snap, err := builtin(); err == nil {
			add(pluginid.Builtin, snap.Revision, snap.Offer.Entries)
		} else {
			// A binary built without its included offer is a defect to surface,
			// not an empty provider; marketplaceadmin.List reports it as missing.
			errs = append(errs, CatalogError{
				Marketplace: pluginid.Builtin, Error: "marketplace-entry-unavailable",
				Message: "the included offer cannot be read from this binary",
			})
		}
	}
	for _, m := range s.Config.Marketplaces {
		if only != "" && m.Name != only {
			continue
		}
		c, ok := s.Catalogs.ByRegistration(m.RegistrationID)
		if !ok || !StoredMatches(c, m) {
			errs = append(errs, CatalogError{
				Marketplace: SafeText(m.Name), Error: "marketplace-entry-unavailable",
				Message: "the stored offer is missing or does not match its source; run `work plugin marketplace upgrade " + SafeText(m.Name) + "`",
			})
			continue
		}
		add(m.Name, c.Revision, c.Entries)
	}
	slices.SortStableFunc(rows, func(a, b Offered) int {
		if c := strings.Compare(a.Marketplace, b.Marketplace); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return rows, errs
}

// StoredMatches ties stored discovery to the exact registered descriptor.
func StoredMatches(c *marketplace.Stored, m config.Marketplace) bool {
	return c.Namespace == m.Name && c.Source.Kind == m.Kind && c.Source.Source == m.Source && c.Source.Branch == m.Branch
}

// KnownNamespace reports whether name can be inspected: the included provider,
// a registered source, or a namespace that still has installed plugins even if
// its registration is gone.
func KnownNamespace(s *adminstate.Snapshot, name string) bool {
	if name == pluginid.Builtin {
		return true
	}
	if _, ok := s.Config.Marketplace(name); ok {
		return true
	}
	for _, p := range s.Registry.Packages {
		if p.Provider.Marketplace == name {
			return true
		}
	}
	return false
}

// FilterInstalled limits an inventory to one marketplace; an empty name keeps all.
func FilterInstalled(entries []Entry, name string) []Entry {
	if name == "" {
		return entries
	}
	out := []Entry{}
	for _, e := range entries {
		if e.Marketplace == name {
			out = append(out, e)
		}
	}
	return out
}
