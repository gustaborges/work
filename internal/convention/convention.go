// Package convention resolves a branch-naming convention and its prefix into a
// concrete branch name. A convention is always addressed by its fully qualified
// reference (plugin@marketplace/name): equal bare names from different
// providers are independent conventions with their own prefixes, so a bare name
// is never looked up here.
package convention

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
)

// Catalog is the set of known conventions, loaded from the component registry.
type Catalog struct {
	conventions []registry.Convention
}

// Load builds a Catalog from the registry.
func Load(reg *registry.Registry) Catalog {
	return Catalog{conventions: reg.Effective().ListConventions()}
}

// Prefixes returns the prefixes offered by the convention with the given
// qualified reference.
func (c Catalog) Prefixes(conventionRef string) ([]string, error) {
	for _, cv := range c.conventions {
		if cv.Ref().String() == conventionRef {
			return cv.Prefixes, nil
		}
	}
	return nil, diag.Newf(diag.BootstrapFailed, "branch convention %q is not registered", conventionRef)
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
