package cli

import (
	"os"

	"context"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/repoidentity"
	"github.com/gustaborges/work/internal/workhome"
)

// loadConventionContext projects current enabled providers without bootstrap.
func loadConventionContext(ctx context.Context) (workhome.Home, *adminstate.Snapshot, error) {
	h, s, err := readAdministration(ctx)
	if err != nil {
		return h, nil, err
	}
	return h, s, nil
}

// resolveConventionIdentity discovers the git repository rooted at or above
// the current working directory and computes its ADR-0011 identity key.
// Outside a git repository (or with git unusable), every `work convention`
// form fails identically (contracts/cli-work-convention.md §Repository
// identity resolution).
func resolveConventionIdentity() (string, error) {
	if err := gitx.Preflight(); err != nil {
		return "", diag.Wrap(diag.BootstrapFailed, err, "git is required but not usable")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", diag.Wrap(diag.BootstrapFailed, err, "cannot determine the current directory")
	}
	root, err := gitx.DiscoverRepoRoot(cwd)
	if err != nil {
		return "", diag.New(diag.Usage, "not inside a git repository; run from any clone")
	}
	identity, err := repoidentity.Identify(gitx.Open(root))
	if err != nil {
		return "", diag.Wrap(diag.BootstrapFailed, err, "cannot compute the repository identity")
	}
	return identity, nil
}
