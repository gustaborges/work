package cli

import (
	"os"

	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoidentity"
	"github.com/gustaborges/work/internal/workhome"
)

// loadConventionContext resolves the Work home, ensures the seed is
// bootstrapped (so the reference package's "freeform" convention exists even
// before any `work start` has run), and loads the current registry. Every
// `work convention` subcommand and the hub share this precondition, exactly
// like `work repository`.
func loadConventionContext() (workhome.Home, *registry.Registry, error) {
	home, err := workhome.Resolve()
	if err != nil {
		return workhome.Home{}, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	if err := bootstrap.EnsureSeed(home); err != nil {
		return workhome.Home{}, nil, err
	}
	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		return workhome.Home{}, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}
	return home, reg, nil
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
