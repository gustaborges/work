package marketplaceadmin

import (
	"context"
	"errors"
	"slices"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

const opRemove = "work plugin marketplace remove"

// ReasonDistributionProvided is the stable usage reason for refusing to remove
// the code-owned provider; its plugins are removable one by one instead.
const ReasonDistributionProvided = "distribution-provided-marketplace"

// RemoveResult is the outcome of a whole-source removal. Committed is true once
// the removal is published, even when cleanup afterwards reported an error.
type RemoveResult struct {
	Name           string
	Result         string // removed
	Removed        []pluginid.Provider
	Committed      bool
	CleanupPending bool
}

// RemovePlan is one reviewed whole-source removal. Everything in it was read
// from local state: removing a source never contacts its origin, so it works
// offline and for sources that are gone.
type RemovePlan struct {
	home      workhome.Home
	source    config.Marketplace
	uninstall *pluginadmin.UninstallPlan
}

// Name is the namespace being removed.
func (p *RemovePlan) Name() string { return p.source.Name }

// Source is the registration being removed, in its stored (not display) form.
func (p *RemovePlan) Source() config.Marketplace { return p.source }

// Providers are every installed plugin copied from the source, including
// disabled ones and ones its offer no longer lists.
func (p *RemovePlan) Providers() []pluginid.Provider { return p.uninstall.Providers() }

// PolicyReferences are the repository-resolution Locators owned by those
// plugins. Removing them needs authorization separate from confirming the
// removal itself.
func (p *RemovePlan) PolicyReferences() []string { return p.uninstall.PolicyReferences() }

// ConventionEffects lists conventions whose availability the removal changes.
func (p *RemovePlan) ConventionEffects() []pluginadmin.ConventionEffect {
	return p.uninstall.ConventionEffects()
}

// sourceTargets are the installed plugins that belong to one source instance.
// Membership is by registration, not by namespace text, so a namespace reused
// by a different registration is never swept in.
func sourceTargets(s *adminstate.Snapshot, registrationID string) []pluginid.Provider {
	out := []pluginid.Provider{}
	for _, pkg := range s.Registry.Packages {
		if pkg.Provenance.RegistrationID == registrationID {
			out = append(out, pkg.Provider)
		}
	}
	return pluginadmin.SortedProviders(out)
}

// CheckRemovable rejects, without touching any state, a name that can never be
// removed as a source: the code-owned provider. Commands call it before
// recovery or locking so a refused request leaves no trace.
func CheckRemovable(name string) error {
	if pluginid.IsReserved(name) {
		return diag.Newf(diag.Usage, "%s: the included marketplace %q cannot be removed; remove its plugins individually", ReasonDistributionProvided, name)
	}
	return nil
}

// PrepareRemove freezes the complete removal of the named source from one
// coherent local read. The included provider is refused by the domain, not by
// the command line: callers cannot compose their way around it.
func PrepareRemove(ctx context.Context, h workhome.Home, name string) (*RemovePlan, error) {
	if err := CheckRemovable(name); err != nil {
		return nil, err
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	src, ok := s.Config.Marketplace(name)
	if !ok {
		return nil, diag.Newf(diag.Usage, "unknown marketplace %q", name)
	}
	uninstall, err := pluginadmin.PlanUninstallSet(ctx, h, sourceTargets(s, src.RegistrationID))
	if err != nil {
		return nil, err
	}
	return &RemovePlan{home: h, source: src, uninstall: uninstall}, nil
}

// Apply publishes the reviewed removal as one transaction: the registration,
// its stored offer, every installed copy and the authorized policy references
// commit together or not at all. Reviewed facts that changed since
// preparation, including a plugin installed from the source in the meantime,
// conflict instead of widening what was authorized.
func (p *RemovePlan) Apply(ctx context.Context, removePolicy bool) (RemoveResult, error) {
	return p.ApplyWithOptions(ctx, removePolicy, adminstate.WriteOptions{})
}

// ApplyWithOptions is Apply with transaction checkpoint hooks for tests.
func (p *RemovePlan) ApplyWithOptions(ctx context.Context, removePolicy bool, opts adminstate.WriteOptions) (RemoveResult, error) {
	res := RemoveResult{Name: p.source.Name, Result: "removed"}
	if len(p.uninstall.PolicyReferences()) > 0 && !removePolicy {
		return res, diag.New(diag.PluginPolicyDependency, "removal drops policy Locators; explicitly approve --remove-policy-references")
	}
	if ctx.Err() != nil {
		return res, classify(ctx.Err())
	}
	operation := opRemove + " " + p.source.Name + " --yes"
	if removePolicy {
		operation += " --remove-policy-references"
	}
	err := adminstate.WithWriteOptions(ctx, p.home, operation, func(s *adminstate.Session) error {
		cur, ok := findRegistration(s.Config, p.source.RegistrationID)
		if !ok || cur != p.source {
			return diag.New(diag.AdministrationConflict, "the marketplace registration changed; review the removal again")
		}
		if !slices.Equal(sourceTargets(s.Snapshot, p.source.RegistrationID), p.uninstall.Providers()) {
			return diag.New(diag.AdministrationConflict, "the plugins installed from this marketplace changed; review the removal again")
		}
		if err := p.uninstall.ApplyIn(p.home, s); err != nil {
			return err
		}
		s.Config.Marketplaces = slices.DeleteFunc(slices.Clone(s.Config.Marketplaces), func(m config.Marketplace) bool {
			return m.RegistrationID == p.source.RegistrationID
		})
		s.Catalogs.Remove(p.source.RegistrationID)
		return nil
	}, opts)
	switch {
	case err == nil:
		res.Committed, res.Removed = true, p.uninstall.Providers()
	case diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code:
		res.Committed, res.CleanupPending, res.Removed = true, true, p.uninstall.Providers()
	}
	if err != nil {
		if _, typed := errors.AsType[*diag.Error](err); !typed {
			err = diag.Wrap(diag.PluginUninstallFailed, err, "marketplace removal failed; the previous state was restored")
		}
		return res, classify(err)
	}
	return res, nil
}
