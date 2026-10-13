package cli

import (
	"context"
	"os"
	"reflect"
	"slices"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/registry"
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

// setConvention takes a review snapshot even for direct commands, so a writer
// cannot silently replace facts that changed while waiting for the lock.
func setConvention(ctx context.Context, h workhome.Home, identity, name string) error {
	reviewed, err := adminstate.Read(ctx, h)
	if err != nil {
		return err
	}
	return setReviewedConvention(ctx, h, identity, name, reviewed)
}
func conventionProviders(reg *registry.Registry, name string) []registry.Package {
	owners := []registry.Package{}
	for _, p := range reg.ListPackages() {
		if slices.Contains(p.Conventions, name) {
			owners = append(owners, p)
		}
	}
	return owners
}
func setReviewedConvention(ctx context.Context, h workhome.Home, identity, name string, reviewed *adminstate.Snapshot) error {
	return adminstate.WithWrite(ctx, h, "work convention set", func(s *adminstate.Session) error {
		old, had := reviewed.Memory.Get(identity)
		current, has := s.Memory.Get(identity)
		before, bok := reviewed.Registry.Effective().ConventionByName(name)
		after, aok := s.Registry.Effective().ConventionByName(name)
		if old != current || had != has || bok != aok || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(conventionProviders(reviewed.Registry, name), conventionProviders(s.Registry, name)) {
			return diag.New(diag.AdministrationConflict, "reviewed convention choice or provider changed; review again")
		}
		if !aok {
			return diag.New(diag.ConventionUnknown, "the selected branch convention is unavailable; install or enable its provider, then run work convention set <name>")
		}
		s.Memory.Set(identity, name)
		return nil
	})
}
