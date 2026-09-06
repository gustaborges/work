// Package convention resolves a branch-naming convention and its prefix into a
// concrete branch name. F1 seeds exactly one convention, "freeform", whose sole
// prefix is "{slug}", so a derived name equals the slug; the package is written
// against the general catalog so later slices add conventions without changing
// callers.
package convention

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
)

// Freeform is the name of the convention F1 always resolves to.
const Freeform = "freeform"

// Catalog is the set of known conventions, loaded from the component registry.
type Catalog struct {
	conventions []registry.Convention
}

// Load builds a Catalog from the registry.
func Load(reg *registry.Registry) Catalog {
	return Catalog{conventions: reg.ListConventions()}
}

// Prefixes returns the prefixes offered by the named convention.
func (c Catalog) Prefixes(conventionName string) ([]string, error) {
	for _, cv := range c.conventions {
		if cv.Name == conventionName {
			return cv.Prefixes, nil
		}
	}
	return nil, diag.Newf(diag.BootstrapFailed, "branch convention %q is not registered", conventionName)
}

// Interpolate expands a prefix template against a slug. The only token is
// "{slug}".
func Interpolate(prefix, slug string) string {
	return strings.ReplaceAll(prefix, "{slug}", slug)
}

// DeriveName resolves a convention + prefix + slug to a branch name. The prefix
// must be one the convention offers.
func (c Catalog) DeriveName(conventionName, prefix, slug string) (string, error) {
	prefixes, err := c.Prefixes(conventionName)
	if err != nil {
		return "", err
	}
	if !slices.Contains(prefixes, prefix) {
		return "", diag.Newf(diag.Usage,
			"prefix %q is not offered by the %q convention (offered: %s)",
			prefix, conventionName, strings.Join(prefixes, ", "))
	}
	name := Interpolate(prefix, slug)
	if strings.TrimSpace(name) == "" {
		return "", diag.New(diag.InvalidBranchName, "the derived branch name is empty")
	}
	return name, nil
}

// DefaultPrefix returns the single prefix for a convention that has exactly one
// (F1: freeform -> "{slug}"). It errors if the convention offers several.
func (c Catalog) DefaultPrefix(conventionName string) (string, error) {
	prefixes, err := c.Prefixes(conventionName)
	if err != nil {
		return "", err
	}
	if len(prefixes) != 1 {
		return "", fmt.Errorf("convention %q offers %d prefixes; a choice is required", conventionName, len(prefixes))
	}
	return prefixes[0], nil
}
