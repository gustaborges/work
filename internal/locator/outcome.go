package locator

import (
	"strings"

	"github.com/gustaborges/work/internal/diag"
)

// errNoRepositoryFound is returned when the policy was fully traversed and
// every eligible Locator returned matches:[] (26). The interactive Source
// step recovers from this one in-frame by re-prompting for a different
// reference (research R5, R9).
func errNoRepositoryFound() error {
	return diag.New(diag.NoRepositoryFound,
		"no configured repository search root has a clone matching the given reference").
		WithSummary("No local clone matched what you typed.").
		WithHint("Check the name, or add a search root with `work repository root add <dir>`.")
}

// errNoEligibleLocator is returned when resolution could not even start: the
// policy is empty, the reference carries no field, or no policy entry accepts
// any field the reference carries (27). Terminal — retyping will not help.
func errNoEligibleLocator() error {
	return diag.New(diag.NoEligibleLocator,
		"no repository locator is configured to resolve this reference").
		WithSummary("Nothing is set up to locate a repository from what you typed.").
		WithHint("Run `work repository policy list` to check the configured locators, or pass a direct path.")
}

// errCandidateInvalid is returned when the Locator that ended traversal
// returned candidates but every one failed reporef.ValidatePath (28). It
// names the rejected path(s) so the user can find the broken directory
// (quickstart S8).
func errCandidateInvalid(rejected []string) error {
	return diag.Newf(diag.RepositoryCandidateInvalid,
		"the repository locator's candidates are not usable git repositories: %s", strings.Join(rejected, ", ")).
		WithSummary("The repository locator found something, but it isn't a usable Git repository.").
		WithHint("Check the search roots for a broken or non-git directory, or adjust the reference.")
}

// errLocatorFailed is returned when a Locator transport-failed, exited
// non-zero, or produced unparseable stdout (29). Traversal halts with no
// fallback to a later policy entry (ADR-0015).
func errLocatorFailed(ref string, cause error) error {
	return diag.Wrapf(diag.LocatorFailed, cause, "repository locator %s failed", ref).
		WithSummary("A repository locator errored while searching for a match.").
		WithHint("Check `work repository policy list` and the locator's installation, then retry.")
}
