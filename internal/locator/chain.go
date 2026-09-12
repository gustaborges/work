package locator

import (
	"context"

	"github.com/gustaborges/work/internal/ipc"
)

// traverseResult is the raw outcome of walking the policy, before candidate
// validation and dedup.
type traverseResult struct {
	// matches holds the raw repo_path values from the Locator that ended
	// traversal. Empty when no eligible Locator ever returned a match.
	matches []string
	// anyEligible is true when at least one policy entry was available and
	// eligible for ref, distinguishing no-repository-found (26, "the policy
	// ran and found nothing") from no-eligible-locator (27, "nothing could
	// even run") when matches is empty.
	anyEligible bool
}

// traverse walks Deps.Policy in order (ADR-0015): an unavailable entry is
// skipped; an ineligible entry is skipped; an eligible entry is invoked, and
//   - a transport/exit/parse failure halts traversal with locator-failed (29)
//     and no fallback to the next entry (FR-015, ADR-0015);
//   - "matches": [] continues to the next entry (FR-010);
//   - a non-empty "matches" ends traversal — later entries are never consulted
//     (FR-011); only this entry's candidates are ever evaluated (no
//     aggregation across Locators, ADR-0015).
func traverse(ctx context.Context, d Deps, ref Reference) (traverseResult, error) {
	var anyEligible bool
	for _, policyRef := range d.Policy {
		comp, ok := availableComponent(d.Registry, policyRef)
		if !ok {
			continue
		}
		proj, ok := eligible(comp, ref)
		if !ok {
			continue
		}
		anyEligible = true

		input := ipc.LocatorInput{Repository: proj, RepositoryRoots: d.Roots}
		resp, err := ipc.InvokeLocator(comp.EntrypointPath(d.PluginsDir), input)
		if err != nil {
			return traverseResult{}, errLocatorFailed(policyRef, err)
		}
		if len(resp.Matches) == 0 {
			continue
		}

		paths := make([]string, len(resp.Matches))
		for i, m := range resp.Matches {
			paths[i] = m.RepoPath
		}
		return traverseResult{matches: paths, anyEligible: true}, nil
	}
	return traverseResult{anyEligible: anyEligible}, nil
}
