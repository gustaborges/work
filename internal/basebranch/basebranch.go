// Package basebranch lists a repository's local and remote-tracking branches
// as base-branch choices and resolves a --base flag value against them. The
// picker distinguishes homonyms and divergent refs by short object name so the
// selected revision is unambiguous (FR-009).
package basebranch

import (
	"fmt"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
)

// Scope is where a base-branch choice lives.
type Scope string

const (
	ScopeLocal          Scope = "local"
	ScopeRemoteTracking Scope = "remote-tracking"
)

// Choice is one row of the base-branch picker.
type Choice struct {
	// Refname is the fully-qualified ref, used verbatim as the worktree base.
	Refname string
	// Short is the display/stored name: "main" or "origin/main".
	Short string
	// Scope is local or remote-tracking.
	Scope Scope
	// ObjectShort is the short object name, disambiguating homonyms.
	ObjectShort string
}

// List returns every selectable base branch: local heads and remote-tracking
// branches (excluding refs/remotes/*/HEAD).
func List(repo gitx.Repo) ([]Choice, error) {
	refs, err := repo.ForEachRef("refs/heads", "refs/remotes")
	if err != nil {
		return nil, diag.Wrap(diag.UnusableRepo, err, "cannot list branches")
	}
	var choices []Choice
	for _, r := range refs {
		scope := ScopeLocal
		if strings.HasPrefix(r.Refname, "refs/remotes/") {
			scope = ScopeRemoteTracking
		}
		choices = append(choices, Choice{
			Refname:     r.Refname,
			Short:       r.Short,
			Scope:       scope,
			ObjectShort: r.ObjectShort,
		})
	}
	return choices, nil
}

// Resolve picks the choice matching flagValue. flagValue may be a bare short
// name ("main") or a qualified remote form ("origin/main"). An empty flagValue
// is a programming error — callers gate on it. An ambiguous bare name that
// matches both a local and a remote-tracking branch is a usage error.
func Resolve(choices []Choice, flagValue string) (Choice, error) {
	if len(choices) == 0 {
		return Choice{}, diag.New(diag.NoBaseBranch, "the repository has no selectable base branch")
	}
	v := strings.TrimSpace(flagValue)
	if v == "" {
		return Choice{}, diag.New(diag.Usage, "no base branch was selected")
	}

	// Exact short-name match wins ("main", "origin/main").
	for _, c := range choices {
		if c.Short == v {
			return c, nil
		}
	}
	// Otherwise a bare name may still identify a remote-tracking branch
	// ("main" -> "origin/main"), but only if it does so unambiguously.
	var byTail []Choice
	for _, c := range choices {
		if c.Scope == ScopeRemoteTracking && strings.HasSuffix(c.Short, "/"+v) {
			byTail = append(byTail, c)
		}
	}
	switch len(byTail) {
	case 1:
		return byTail[0], nil
	case 0:
		return Choice{}, diag.Newf(diag.Usage, "no base branch matches %q", v)
	default:
		return Choice{}, diag.Newf(diag.Usage,
			"base branch %q is ambiguous; qualify it as <remote>/<name>", v)
	}
}

// Format renders a choice for a confirm summary.
func (c Choice) Format() string {
	return fmt.Sprintf("%s @ %s", c.Short, c.ObjectShort)
}
