package repoconfig

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
)

// errNotFound marks a policy-entry lookup that found nothing, so callers can
// tell "absent" (a no-op for remove) apart from "ambiguous" (always a usage
// error).
var errNotFound = errors.New("not found")

// PolicyEntry is one row of `work repository policy list` (data-model.md §4).
type PolicyEntry struct {
	// Ref is the qualified "<alias>/<component>" string as stored.
	Ref string
	// Position is the 1-based traversal order.
	Position int
	// Available is true iff a registered component with that alias+name has
	// role == repository-locator (research R12).
	Available bool
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
	InPolicy bool
}

// ListPolicy returns every policy entry in order, each marked with its
// current availability. Unavailable entries are never dropped (FR-025).
func ListPolicy(cfg *config.Config, reg *registry.Registry) []PolicyEntry {
	list := cfg.RepositoryResolution.Locators
	out := make([]PolicyEntry, len(list))
	for i, ref := range list {
		out[i] = PolicyEntry{Ref: ref, Position: i + 1, Available: isAvailable(reg, ref)}
	}
	return out
}

// ListLocators returns every registered repository-locator component,
// regardless of policy membership, each marked whether it is currently in the
// policy.
func ListLocators(cfg *config.Config, reg *registry.Registry) []RegisteredLocator {
	comps := reg.ByRole(registry.RoleRepositoryLocator)
	out := make([]RegisteredLocator, len(comps))
	for i, c := range comps {
		ref := c.Alias + "/" + c.Name
		out[i] = RegisteredLocator{
			Ref:         ref,
			DisplayName: c.DisplayName,
			Description: c.Description,
			Accepts:     slices.Clone(c.Accepts),
			InPolicy:    slices.Contains(cfg.RepositoryResolution.Locators, ref),
		}
	}
	return out
}

// AddPolicy appends ref to the policy, or positions it relative to an anchor
// given by exactly one of before/after (both empty is a plain append). ref
// must resolve to a registered repository-locator component; a bare
// <component> is accepted when unambiguous and resolved to its qualified
// form. Adding an already-present ref is a no-op success (data-model.md §4).
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

// ReplacePolicy validates every ref against the registry, then atomically
// replaces the whole policy. Duplicate refs collapse to one entry at their
// first position.
func ReplacePolicy(cfg *config.Config, reg *registry.Registry, refs []string) error {
	next := []string{}
	for _, ref := range refs {
		qualified, err := resolveAgainstRegistry(reg, ref)
		if err != nil {
			return err
		}
		if !slices.Contains(next, qualified) {
			next = append(next, qualified)
		}
	}
	cfg.RepositoryResolution.Locators = next
	return nil
}

// isAvailable reports whether ref names a registered repository-locator
// component (research R12).
func isAvailable(reg *registry.Registry, ref string) bool {
	alias, name, ok := strings.Cut(ref, "/")
	if !ok {
		return false
	}
	return hasRepositoryLocator(reg, alias, name)
}

func hasRepositoryLocator(reg *registry.Registry, alias, name string) bool {
	for _, c := range reg.Components {
		if c.Alias == alias && c.Name == name && c.Role == registry.RoleRepositoryLocator {
			return true
		}
	}
	return false
}

// resolveAgainstRegistry resolves ref (qualified or a bare component name)
// against installed repository-locator components, for operations that must
// only ever name something currently runnable (`add`, `replace`).
func resolveAgainstRegistry(reg *registry.Registry, ref string) (string, error) {
	if alias, name, ok := strings.Cut(ref, "/"); ok {
		if hasRepositoryLocator(reg, alias, name) {
			return ref, nil
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
		return matches[0].Alias + "/" + matches[0].Name, nil
	default:
		return "", diag.Newf(diag.Usage,
			"%q matches more than one installed repository locator; use the qualified <alias>/<component> form", ref)
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
		if _, name, ok := strings.Cut(entry, "/"); ok && name == ref {
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
			"%q matches more than one policy entry; use the qualified <alias>/<component> form", ref)
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
