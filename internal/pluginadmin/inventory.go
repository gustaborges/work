// Package pluginadmin implements package administration without presentation.
package pluginadmin

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/registry"
)

type Component struct {
	Name string `json:"name"`
	Role string `json:"role"`
}
type Entry struct {
	Alias            string      `json:"alias"`
	Origin           string      `json:"origin"`
	Reference        string      `json:"reference"`
	Components       []Component `json:"components"`
	Conventions      []string    `json:"conventions"`
	Enabled          bool        `json:"enabled"`
	Version          *string     `json:"version"`
	ContentDigest    *string     `json:"content_digest"`
	InspectionStatus string      `json:"inspection_status"`
}

func nullable(v string) *string {
	if v == "" {
		return nil
	}
	v = SafeText(v)
	return &v
}
func Inventory(s *adminstate.Snapshot) []Entry {
	out := []Entry{}
	for _, p := range s.Registry.ListPackages() {
		e := Entry{Alias: SafeText(p.Alias), Origin: SafeText(p.Origin), Reference: SafeReference(p.Reference, p.Origin), Enabled: s.Registry.PackageEnabled(p.Alias), Version: nullable(p.Version), ContentDigest: nullable(p.ContentDigest), Components: []Component{}, Conventions: []string{}, InspectionStatus: s.Inspection[p.Alias]}
		if e.InspectionStatus == "" {
			e.InspectionStatus = "metadata-unknown"
		}
		for _, c := range s.Registry.Components {
			if c.Alias == p.Alias {
				e.Components = append(e.Components, Component{Name: SafeText(c.Name), Role: SafeText(c.Role)})
			}
		}
		for _, name := range p.Conventions {
			e.Conventions = append(e.Conventions, SafeText(name))
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
func SafeReference(v, origin string) string {
	if origin == registry.OriginRemotePinned {
		if at := strings.LastIndex(v, "@"); at >= 0 {
			return SafeSource(v[:at]) + "@" + SafeText(v[at+1:])
		}
	}
	return SafeSource(v)
}
