package marketplaceadmin

import (
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
)

// Catalog statuses of a listed source.
const (
	CatalogReady   = "ready"
	CatalogMissing = "missing"
)

// Source is the credential-safe row of one known marketplace. Totals are local
// counts from the same administrative snapshot: they include withdrawn
// entries, satisfy EnabledCount <= InstalledCount and never depend on catalog
// availability or on any filter.
type Source struct {
	Name           string  `json:"name"`
	DisplayName    *string `json:"display_name"`
	SourceKind     string  `json:"source_kind"`
	Source         string  `json:"source"`
	Branch         *string `json:"branch"`
	Revision       *string `json:"revision"`
	PluginCount    *int    `json:"plugin_count"`
	CatalogStatus  string  `json:"catalog_status"`
	InstalledCount int     `json:"installed_count"`
	EnabledCount   int     `json:"enabled_count"`
}

// List projects every known source from one coherent snapshot without any
// network access, home creation or catalog refresh. The included provider is
// projected from the binary, so it is listed even in a home that does not
// exist yet; the online `work` source appears only after explicit
// registration.
func List(s *adminstate.Snapshot) []Source {
	out := []Source{}
	builtin := Source{
		Name: pluginid.Builtin, SourceKind: pluginid.KindBuiltin, Source: pluginid.Builtin, CatalogStatus: CatalogMissing,
	}
	if snap, err := marketplace.BuiltinSnapshot(); err == nil {
		n, rev := len(snap.Offer.Entries), snap.Revision
		builtin.PluginCount, builtin.Revision, builtin.CatalogStatus = &n, &rev, CatalogReady
		builtin.DisplayName = display(snap.Offer.DisplayName)
	}
	builtin.InstalledCount, builtin.EnabledCount = totals(s, pluginid.BuiltinRegistrationID)
	out = append(out, builtin)

	for _, m := range s.Config.Marketplaces {
		row := Source{
			Name: pluginadmin.SafeText(m.Name), SourceKind: m.Kind, Source: pluginadmin.SafeSource(m.Source), CatalogStatus: CatalogMissing,
		}
		if m.Branch != "" {
			b := pluginadmin.SafeText(m.Branch)
			row.Branch = &b
		}
		if c, ok := s.Catalogs.ByRegistration(m.RegistrationID); ok && storedMatches(c, m) {
			n, rev := len(c.Entries), pluginadmin.SafeText(c.Revision)
			row.PluginCount, row.Revision, row.CatalogStatus = &n, &rev, CatalogReady
			row.DisplayName = display(c.DisplayName)
		}
		row.InstalledCount, row.EnabledCount = totals(s, m.RegistrationID)
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b Source) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// storedMatches ties stored discovery to the exact registered descriptor, so a
// hand-edited or mismatched catalog is unavailable rather than silently usable
// for a different source.
func storedMatches(c *marketplace.Stored, m config.Marketplace) bool {
	return c.Namespace == m.Name && c.Source.Kind == m.Kind && c.Source.Source == m.Source && c.Source.Branch == m.Branch
}

func display(v string) *string {
	if v == "" {
		return nil
	}
	v = pluginadmin.SafeText(v)
	return &v
}

// totals counts installed and enabled plugins that came from one source
// instance, whether or not their entry is still offered.
func totals(s *adminstate.Snapshot, registrationID string) (installed, enabled int) {
	for _, p := range s.Registry.Packages {
		if p.Provenance.RegistrationID != registrationID {
			continue
		}
		installed++
		if p.Enabled {
			enabled++
		}
	}
	return installed, enabled
}
