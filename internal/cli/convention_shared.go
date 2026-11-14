package cli

import (
	"context"
	"os"
	"reflect"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconv"
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

// resolveConventionRef maps user input to one enabled convention. A fully
// qualified reference names its owner exactly; a bare name is accepted only
// when exactly one enabled provider offers it, and the stored result is always
// fully qualified.
func resolveConventionRef(reg *registry.Registry, input string) (pluginid.Ref, error) {
	eff := reg.Effective()
	if strings.ContainsAny(input, "@/") {
		ref, err := pluginid.ParseRef(input)
		if err == nil {
			if _, ok := eff.ConventionByRef(ref); ok {
				return ref, nil
			}
		}
		return pluginid.Ref{}, diag.New(diag.ConventionUnknown, "the selected branch convention is unavailable; install or enable its provider, then run work convention set <plugin>@<marketplace>/<name>")
	}
	matches := eff.ConventionsNamed(input)
	switch len(matches) {
	case 0:
		return pluginid.Ref{}, diag.New(diag.ConventionUnknown, "the selected branch convention is unavailable; install or enable its provider, then run work convention set <name>")
	case 1:
		return matches[0].Ref(), nil
	}
	return pluginid.Ref{}, diag.New(diag.Usage, "more than one provider offers that convention; use <plugin>@<marketplace>/<name>")
}

// setConvention takes a review snapshot even for direct commands, so a writer
// cannot silently replace facts that changed while waiting for the lock.
func setConvention(ctx context.Context, h workhome.Home, identity, input string) error {
	reviewed, err := adminstate.Read(ctx, h)
	if err != nil {
		return err
	}
	ref, err := resolveConventionRef(reviewed.Registry, input)
	if err != nil {
		return err
	}
	return setReviewedConvention(ctx, h, identity, ref.String(), reviewed)
}

// conventionFacts is what a reviewed convention choice depends on: the
// convention record and the installed provider (including its source instance).
func conventionFacts(reg *registry.Registry, ref pluginid.Ref) (registry.Convention, registry.Package, bool) {
	eff := reg.Effective()
	c, cok := eff.ConventionByRef(ref)
	p, pok := eff.PackageByProvider(ref.Provider)
	return c, p, cok && pok
}

// setReviewedConvention records the qualified reference bound to its source
// registration instance after proving the reviewed facts still hold.
func setReviewedConvention(ctx context.Context, h workhome.Home, identity, qualified string, reviewed *adminstate.Snapshot) error {
	ref, err := pluginid.ParseRef(qualified)
	if err != nil {
		return diag.New(diag.ConventionUnknown, "the selected branch convention is not a qualified reference")
	}
	return adminstate.WithWrite(ctx, h, "work convention set", func(s *adminstate.Session) error {
		old, had := reviewed.Memory.Get(identity)
		current, has := s.Memory.Get(identity)
		bc, bp, bok := conventionFacts(reviewed.Registry, ref)
		ac, ap, aok := conventionFacts(s.Registry, ref)
		if old != current || had != has || bok != aok || !reflect.DeepEqual(bc, ac) || !reflect.DeepEqual(bp, ap) {
			return diag.New(diag.AdministrationConflict, "reviewed convention choice or provider changed; review again")
		}
		if !aok {
			return diag.New(diag.ConventionUnknown, "the selected branch convention is unavailable; install or enable its provider, then run work convention set <name>")
		}
		s.Memory.Set(identity, repoconv.NewReference(ap.Provenance.RegistrationID, ref))
		return nil
	})
}
