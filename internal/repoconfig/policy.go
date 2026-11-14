package repoconfig

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
)

// errNotFound marks a policy-entry lookup that found nothing, so callers can
// tell "absent" (a no-op for remove) apart from "ambiguous" (always a usage
// error).
var errNotFound = errors.New("not found")

// PolicyEntry is one row of `work repository policy list` (data-model.md §4).
type PolicyEntry struct {
	// Ref is the fully qualified "<plugin>@<marketplace>/<component>" string as stored.
	Ref string
	// Position is the 1-based traversal order.
	Position int
	// Available is true iff a registered component with that alias+name has
	// role == repository-locator (research R12).
	Available         bool
	UnavailableReason *string
	Installed         bool
	Enabled           bool
}

// RegisteredLocator is one row of `work repository locator list": a
// projection of registry.Component restricted to the repository-locator role
// (data-model.md §6).
type RegisteredLocator struct {
	Ref         string
	DisplayName string
	Description string
	Accepts     []string
	// InPolicy is true iff Ref is a member of
	// cfg.RepositoryResolution.Locators.
	InPolicy          bool
	Enabled           bool
	State             string
	UnavailableReason *string
}

// ListPolicy returns every policy entry in order, each marked with its
// current availability. Unavailable entries are never dropped (FR-025).
func ListPolicy(cfg *config.Config, reg *registry.Registry) []PolicyEntry {
	list := cfg.RepositoryResolution.Locators
	out := make([]PolicyEntry, len(list))
	for i, ref := range list {
		state, reason := LocatorState(reg, ref)
		installed := reason == nil || (*reason != "package-missing" && *reason != "reference-invalid")
		enabled := false
		if r, err := pluginid.ParseRef(ref); err == nil {
			enabled = installed && reg.PackageEnabled(r.Provider)
		}
		out[i] = PolicyEntry{Ref: ref, Position: i + 1, Available: state == "available", UnavailableReason: reason, Installed: installed, Enabled: enabled}
	}
	return out
}

// ListLocators returns every registered repository-locator component,
// regardless of policy membership, each marked whether it is currently in the
// policy.
func ListLocators(cfg *config.Config, reg *registry.Registry) []RegisteredLocator {
	comps := reg.ByRole(registry.RoleRepositoryLocator)
	sort.Slice(comps, func(i, j int) bool { return comps[i].QualifiedName() < comps[j].QualifiedName() })
	out := make([]RegisteredLocator, len(comps))
	for i, c := range comps {
		ref := c.QualifiedName()
		out[i] = RegisteredLocator{
			Ref:         ref,
			DisplayName: c.DisplayName,
			Description: c.Description,
			Accepts:     slices.Clone(c.Accepts),
			InPolicy:    slices.Contains(cfg.RepositoryResolution.Locators, ref),
			Enabled:     reg.PackageEnabled(c.Provider),
		}
		out[i].State, out[i].UnavailableReason = LocatorState(reg, ref)
	}
	return out
}

// AddPolicy appends ref to the policy, or positions it relative to an anchor
// given by exactly one of before/after (both empty is a plain append). ref
// must resolve to a registered repository-locator component; a bare
// <component> is accepted when unambiguous and always stored fully qualified. Adding an already-present ref is a no-op success (data-model.md §4).
func AddPolicy(cfg *config.Config, reg *registry.Registry, ref, before, after string) error {
	if before != "" && after != "" {
		return diag.New(diag.Usage, "specify only one of --before or --after")
	}
	qualified, err := resolveAgainstRegistry(reg, ref)
	if err != nil {
		return err
	}
	list := cfg.RepositoryResolution.Locators
	if slices.Contains(list, qualified) {
		return nil
	}
	next := slices.Clone(list)
	switch {
	case before != "":
		anchor, err := resolveAgainstPolicy(next, before)
		if err != nil {
			return notFoundAsUsage(before, err)
		}
		next = slices.Insert(next, slices.Index(next, anchor), qualified)
	case after != "":
		anchor, err := resolveAgainstPolicy(next, after)
		if err != nil {
			return notFoundAsUsage(after, err)
		}
		next = slices.Insert(next, slices.Index(next, anchor)+1, qualified)
	default:
		next = append(next, qualified)
	}
	cfg.RepositoryResolution.Locators = next
	return nil
}

// RemovePolicy drops each ref from the policy; the referenced component stays
// installed and enabled (ADR-0015). A ref that is not in the policy is a
// no-op success; an ambiguous bare name is still a usage error.
func RemovePolicy(cfg *config.Config, refs []string) error {
	list := slices.Clone(cfg.RepositoryResolution.Locators)
	for _, ref := range refs {
		qualified, err := resolveAgainstPolicy(list, ref)
		if err != nil {
			if errors.Is(err, errNotFound) {
				continue
			}
			return err
		}
		list = slices.DeleteFunc(list, func(s string) bool { return s == qualified })
	}
	cfg.RepositoryResolution.Locators = list
	return nil
}

// MovePolicy repositions ref immediately before or after anchor. Exactly one
// of before/after must be given, and both ref and the anchor must already be
// present in the policy.
func MovePolicy(cfg *config.Config, ref, before, after string) error {
	if (before == "") == (after == "") {
		return diag.New(diag.Usage, "specify exactly one of --before or --after")
	}
	anchorRaw := before
	if anchorRaw == "" {
		anchorRaw = after
	}

	list := slices.Clone(cfg.RepositoryResolution.Locators)
	qualified, err := resolveAgainstPolicy(list, ref)
	if err != nil {
		return notFoundAsUsage(ref, err)
	}
	anchor, err := resolveAgainstPolicy(list, anchorRaw)
	if err != nil {
		return notFoundAsUsage(anchorRaw, err)
	}
	if anchor == qualified {
		return diag.Newf(diag.Usage, "cannot move %s relative to itself", qualified)
	}

	list = slices.DeleteFunc(list, func(s string) bool { return s == qualified })
	idx := slices.Index(list, anchor)
	if before != "" {
		list = slices.Insert(list, idx, qualified)
	} else {
		list = slices.Insert(list, idx+1, qualified)
	}
	cfg.RepositoryResolution.Locators = list
	return nil
}

// ReplacePolicy resolves every operand before assigning the requested order.
func ReplacePolicy(cfg *config.Config, reg *registry.Registry, refs []string) error {
	next := []string{}
	for _, ref := range refs {
		qualified, err := resolveReplacement(cfg.RepositoryResolution.Locators, reg, ref)
		if err != nil {
			return err
		}
		if slices.Contains(next, qualified) {
			return diag.Newf(diag.Usage, "duplicate repository locator %q", qualified)
		}
		next = append(next, qualified)
	}
	cfg.RepositoryResolution.Locators = next
	return nil
}

func qualifiedForm(ref string) bool {
	_, err := pluginid.ParseRef(ref)
	return err == nil
}

func resolveReplacement(previous []string, reg *registry.Registry, ref string) (string, error) {
	if qualifiedForm(ref) && slices.Contains(previous, ref) {
		return ref, nil
	}
	matches := []string{}
	for _, c := range reg.ByRole(registry.RoleRepositoryLocator) {
		q := c.QualifiedName()
		if q == ref || (!qualifiedForm(ref) && c.Name == ref) {
			matches = append(matches, q)
		}
	}
	if !qualifiedForm(ref) {
		for _, q := range previous {
			if r, err := pluginid.ParseRef(q); err == nil && r.Name == ref && !slices.Contains(matches, q) {
				matches = append(matches, q)
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", diag.Newf(diag.Usage, "ambiguous repository locator %q; use <plugin>@<marketplace>/<component>", ref)
	}
	return "", diag.Newf(diag.Usage, "unknown repository locator %q", ref)
}

func hasRepositoryLocator(reg *registry.Registry, r pluginid.Ref) bool {
	for _, c := range reg.Components {
		if c.Provider == r.Provider && c.Name == r.Name && c.Role == registry.RoleRepositoryLocator {
			return true
		}
	}
	return false
}

// resolveAgainstRegistry resolves ref (fully qualified or a bare component
// name) against installed repository-locator components, for operations that
// must name an installed Locator (`add`, `replace`). The result is always the
// fully qualified form.
func resolveAgainstRegistry(reg *registry.Registry, ref string) (string, error) {
	if strings.ContainsAny(ref, "/@") {
		r, err := pluginid.ParseRef(ref)
		if err == nil && hasRepositoryLocator(reg, r) {
			return r.String(), nil
		}
		return "", diag.Newf(diag.Usage, "unknown repository locator %q", ref)
	}
	var matches []registry.Component
	for _, c := range reg.ByRole(registry.RoleRepositoryLocator) {
		if c.Name == ref {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 0:
		return "", diag.Newf(diag.Usage, "unknown repository locator %q", ref)
	case 1:
		return matches[0].QualifiedName(), nil
	default:
		return "", diag.Newf(diag.Usage,
			"%q matches more than one installed repository locator; use the qualified <plugin>@<marketplace>/<component> form", ref)
	}
}

// resolveAgainstPolicy resolves ref (qualified or a bare component name)
// against the entries already in list, for operations on entries that may no
// longer be registered (`remove`, `move`).
func resolveAgainstPolicy(list []string, ref string) (string, error) {
	if slices.Contains(list, ref) {
		return ref, nil
	}
	var matches []string
	for _, entry := range list {
		if r, err := pluginid.ParseRef(entry); err == nil && r.Name == ref {
			matches = append(matches, entry)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%w: %s", errNotFound, ref)
	case 1:
		return matches[0], nil
	default:
		return "", diag.Newf(diag.Usage,
			"%q matches more than one policy entry; use the qualified <plugin>@<marketplace>/<component> form", ref)
	}
}

// notFoundAsUsage turns a resolveAgainstPolicy "not found" into the usage
// error required at call sites (move, add's --before/--after anchor) where
// absence is not a no-op.
func notFoundAsUsage(ref string, err error) error {
	if errors.Is(err, errNotFound) {
		return diag.Newf(diag.Usage, "unknown policy entry %q", ref)
	}
	return err
}

// LocatorState distinguishes retained references in requirement precedence.
func LocatorState(reg *registry.Registry, ref string) (string, *string) {
	r, err := pluginid.ParseRef(ref)
	if err != nil {
		reason := "reference-invalid"
		return "unavailable", &reason
	}
	_, found := reg.PackageByProvider(r.Provider)
	var component *registry.Component
	for i, c := range reg.Components {
		if c.Provider == r.Provider && c.Name == r.Name {
			component = &reg.Components[i]
		}
	}
	reason := ""
	switch {
	case !found:
		reason = "package-missing"
	case component == nil:
		reason = "component-missing"
	case component.Role != registry.RoleRepositoryLocator:
		reason = "role-changed"
	case !reg.PackageEnabled(r.Provider):
		reason = "package-disabled"
	}
	if reason != "" {
		return "unavailable", &reason
	}
	return "available", nil
}
