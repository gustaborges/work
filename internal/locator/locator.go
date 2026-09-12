// Package locator resolves a transient Repository Reference to a local Git
// clone by walking the Repository Resolution Policy as a chain of
// responsibility (ADR-0014, ADR-0015, ADR-0016; ADD §7.1). It has no CLI or
// internal/present dependency, so it is directly unit-testable against fake
// Locator entrypoints (tests/fixtures/locators).
//
// A reference carrying a path never reaches this package — the caller
// validates it directly with reporef.ValidatePath (ADR-0014). Resolve is for
// every other reference shape: name, git_fetch_urls, query, or any
// combination.
package locator

import (
	"context"
	"strings"

	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/registry"
)

// Reference is the transient object a Starter produced (ADR-0016), minus the
// path field, which the caller handles separately. It is never persisted
// (FR-005).
type Reference struct {
	GitFetchURLs []string
	Name         string
	Query        string
}

// hasField reports whether ref carries at least one non-empty locatable
// field. A reference with none can never be eligible for any Locator.
func (r Reference) hasField() bool {
	return len(r.GitFetchURLs) > 0 || r.Name != "" || r.Query != ""
}

// Deps is everything Resolve needs, injected for testability — mirrors how
// starter.Invoke and create.Run already take explicit dependencies rather
// than loading config themselves.
type Deps struct {
	// PluginsDir is the root under which registered components' entrypoints
	// live (workhome.Home.PluginsDir()).
	PluginsDir string
	// Policy is config.RepositoryResolution.Locators, in traversal order.
	Policy []string
	// Roots is config.RepositoryRoots, absolute, projected verbatim to every
	// executed Locator (FR-021).
	Roots []string
	// Registry supplies each policy entry's availability, role, and accepts
	// vocabulary.
	Registry *registry.Registry
}

// Candidate is one validated, deduplicated local repository path from the
// Locator that ended traversal.
type Candidate struct {
	// Path is the absolute, symlink-resolved repository path.
	Path string
	// Remote is the picker's secondary identifying line: the first remote
	// fetch URL, else the parent directory name (research R15). It is
	// computed only when there are >= 2 candidates.
	Remote string
}

// Outcome is the result of a successful Resolve call. Exactly one of Resolved
// or Candidates is populated.
type Outcome struct {
	// Resolved is set on the single-match success.
	Resolved string
	// Candidates holds >= 2 validated, deduped candidates on the ambiguous
	// success; nil otherwise.
	Candidates []Candidate
}

// Resolve walks Deps.Policy as a chain of responsibility to find the local
// clone(s) matching ref (ADR-0015, ADD §7.1). All failures are *diag.Error
// values from the categories no-eligible-locator (27), no-repository-found
// (26), repository-candidate-invalid (28), or locator-failed (29).
// repository-ambiguous (30) is not constructed here: an ambiguous Outcome is
// a success from this package's perspective — the caller (internal/cli)
// decides whether to open a picker or fail non-interactively (FR-011,
// FR-033).
func Resolve(ctx context.Context, d Deps, ref Reference) (Outcome, error) {
	if !ref.hasField() {
		return Outcome{}, errNoEligibleLocator()
	}

	tr, err := traverse(ctx, d, ref)
	if err != nil {
		return Outcome{}, err
	}
	if len(tr.matches) == 0 {
		if tr.anyEligible {
			return Outcome{}, errNoRepositoryFound()
		}
		return Outcome{}, errNoEligibleLocator()
	}

	valid := validateAndDedup(tr.matches)
	switch len(valid) {
	case 0:
		return Outcome{}, errCandidateInvalid(tr.matches)
	case 1:
		return Outcome{Resolved: valid[0]}, nil
	default:
		return Outcome{Candidates: buildCandidates(valid)}, nil
	}
}

// eligible reports whether comp participates in resolution for ref — its
// accepts list intersects ref's present fields (R3) — and if so the
// projection to send it: only the accepted-and-present fields, nothing else
// (R4, FR-007, FR-008, SC-004). Eligibility and projection are computed from
// static data only; no subprocess is started to decide (FR-006, FR-044).
func eligible(comp registry.Component, ref Reference) (ipc.RepositoryReference, bool) {
	var proj ipc.RepositoryReference
	var matched bool
	for _, f := range comp.Accepts {
		switch f {
		case "name":
			if ref.Name != "" {
				proj.Name = ref.Name
				matched = true
			}
		case "git_fetch_urls":
			if len(ref.GitFetchURLs) > 0 {
				proj.GitFetchURLs = ref.GitFetchURLs
				matched = true
			}
		case "query":
			if ref.Query != "" {
				proj.Query = ref.Query
				matched = true
			}
		}
	}
	return proj, matched
}

// availableComponent resolves a "<alias>/<name>" policy entry to a registered
// repository-locator component. A policy entry that does not parse, or names
// nothing registered with that role, is unavailable and is skipped by the
// caller (R12, FR-025).
func availableComponent(reg *registry.Registry, ref string) (registry.Component, bool) {
	if reg == nil {
		return registry.Component{}, false
	}
	alias, name, ok := strings.Cut(ref, "/")
	if !ok {
		return registry.Component{}, false
	}
	for _, c := range reg.Components {
		if c.Alias == alias && c.Name == name && c.Role == registry.RoleRepositoryLocator {
			return c, true
		}
	}
	return registry.Component{}, false
}
