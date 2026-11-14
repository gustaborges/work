// Package pluginadmin implements package administration without presentation.
package pluginadmin

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/gustaborges/work/internal/adminstate"
)

// Component is one installed component with its full provider reference.
type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Entry is the credential-safe inventory row of one installed plugin. Every
// row has a real marketplace identity and actual provenance.
type Entry struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Marketplace      string      `json:"marketplace"`
	Enabled          bool        `json:"enabled"`
	Version          *string     `json:"version"`
	ContentDigest    *string     `json:"content_digest"`
	InspectionStatus string      `json:"inspection_status"`
	SourceKind       string      `json:"source_kind"`
	Source           string      `json:"source"`
	Revision         *string     `json:"revision"`
	Components       []Component `json:"components"`
	Conventions      []string    `json:"conventions"`
}

func nullable(v string) *string {
	if v == "" {
		return nil
	}
	v = SafeText(v)
	return &v
}

// Inventory projects the installed packages from one coherent snapshot,
// sorted by marketplace then plugin, with contributions in manifest order.
func Inventory(s *adminstate.Snapshot) []Entry {
	out := []Entry{}
	for _, p := range s.Registry.ListPackages() {
		e := Entry{
			ID: SafeText(p.ID()), Name: SafeText(p.Provider.Plugin), Marketplace: SafeText(p.Provider.Marketplace),
			Enabled: p.Enabled, Version: nullable(p.Version), ContentDigest: nullable(p.ContentDigest),
			InspectionStatus: s.Inspection[p.ID()], SourceKind: SafeText(p.Provenance.Kind),
			Source: SafeSource(p.Provenance.Source), Revision: nullable(p.Provenance.Revision),
			Components: []Component{}, Conventions: []string{},
		}
		if e.InspectionStatus == "" {
			e.InspectionStatus = "content-unavailable"
		}
		for _, c := range s.Registry.ProviderComponents(p.Provider) {
			e.Components = append(e.Components, Component{ID: SafeText(c.QualifiedName()), Name: SafeText(c.Name), Role: SafeText(c.Role)})
		}
		for _, c := range s.Registry.ProviderConventions(p.Provider) {
			e.Conventions = append(e.Conventions, SafeText(c.Ref().String()))
		}
		out = append(out, e)
	}
	return out
}

// SafeText keeps author-controlled text on one terminal line, escaping controls.
func SafeText(v string) string {
	var b strings.Builder
	for _, r := range v {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SafeSource omits URL credentials, query and fragment, including opaque URLs.
func SafeSource(v string) string {
	if strings.Contains(v, "://") {
		u, err := url.Parse(v)
		if err != nil {
			return "<source>"
		}
		u.User = nil
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		u.RawFragment = ""
		v = u.String()
	} else if at := strings.Index(v, "@"); at >= 0 && strings.Contains(v[at+1:], ":") {
		v = v[at+1:]
	}
	return SafeText(v)
}
