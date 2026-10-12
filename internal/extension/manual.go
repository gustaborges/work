package extension

import (
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/semconv"
)

func manualProviders(reg *registry.Registry, role string) []registry.Component {
	var out []registry.Component
	if role != plugin.RoleLinker && role != plugin.RoleImporter {
		return out
	}
	for _, c := range reg.Effective().Extensions(role) {
		if c.Manual == nil || strings.TrimSpace(c.Manual.DisplayName) == "" {
			continue
		}
		if role == plugin.RoleLinker && semconv.ValidatePublished(reg.PluginNameOf(c.Alias), c.Key) != nil {
			continue
		}
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b registry.Component) int { return strings.Compare(a.QualifiedName(), b.QualifiedName()) })
	return out
}

// ManualCandidates returns static manual choices. Link association never uses
// discovery inputs, runtime, event subscriptions or Starter restrictions.
func ManualCandidates(reg *registry.Registry, role string, s Subject) []Decision {
	var out []Decision
	for _, c := range manualProviders(reg, role) {
		d := Decision{Component: c}
		if role == plugin.RoleImporter {
			inputs, missing, err := ResolveInputs(c.Inputs, s)
			if err != nil || len(missing) != 0 {
				continue
			}
			d.Inputs = inputs
		}
		out = append(out, d)
	}
	return out
}

// ResolveManual resolves identity before any importer input filtering, so a
// bare ambiguous name never silently selects the provider whose inputs exist.
func ResolveManual(reg *registry.Registry, role, reference string) (registry.Component, error) {
	var matches []registry.Component
	for _, c := range manualProviders(reg, role) {
		if reference == c.QualifiedName() || (!strings.Contains(reference, "/") && reference == c.Name) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := make([]string, len(matches))
		for i, c := range matches {
			names[i] = c.QualifiedName()
		}
		return registry.Component{}, diag.Newf(diag.ManualComponentAmbiguous, "manual component %q is ambiguous; choose %s", reference, strings.Join(names, ", "))
	}
	return registry.Component{}, diag.Newf(diag.ManualComponentUnavailable, "manual %s %q is unavailable; install or enable a package declaring that manual component", role, reference)
}
