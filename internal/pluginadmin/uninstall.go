package pluginadmin

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// ConventionEffect records whether a removed provider's convention stops being
// available. Conventions are owned by their provider, so removing a provider
// can only withdraw its own references, never another provider's homonym.
type ConventionEffect struct {
	Name                            string
	AvailableBefore, AvailableAfter bool
}

type removalTarget struct {
	packageEntry registry.Package
	components   []registry.Component
	conventions  []registry.Convention
	digest       string
}

// UninstallPlan retains private reviewed facts. Accessors return independent
// views so presentation cannot expand the set or dependency authorization.
type UninstallPlan struct {
	providers  []pluginid.Provider
	references []string
	targets    []removalTarget
	effects    []ConventionEffect
	lifecycle  adminstate.Lifecycle
}

// Providers is the exact reviewed set.
func (p *UninstallPlan) Providers() []pluginid.Provider { return slices.Clone(p.providers) }

// PolicyReferences are the repository-resolution Locators that depend on the set.
func (p *UninstallPlan) PolicyReferences() []string { return slices.Clone(p.references) }

// ConventionEffects lists conventions whose availability changes.
func (p *UninstallPlan) ConventionEffects() []ConventionEffect { return slices.Clone(p.effects) }

// removalReferences are the policy Locators owned by any of the providers.
func removalReferences(s *adminstate.Snapshot, providers []pluginid.Provider) []string {
	out := []string{}
	for _, ref := range s.Config.RepositoryResolution.Locators {
		if r, err := pluginid.ParseRef(ref); err == nil && slices.Contains(providers, r.Provider) {
			out = append(out, ref)
		}
	}
	return out
}

func factsOf(reg *registry.Registry, p pluginid.Provider) ([]registry.Component, []registry.Convention) {
	return reg.ProviderComponents(p), reg.ProviderConventions(p)
}

// PlanUninstall reads a coherent catalog and owned roots without retrieving or
// invoking any source. Every provider is resolved before inspecting any target.
func PlanUninstall(ctx context.Context, h workhome.Home, providers []pluginid.Provider) (*UninstallPlan, error) {
	targets := SortedProviders(providers)
	if len(targets) == 0 {
		return nil, diag.New(diag.Usage, "supply at least one installed plugin as plugin@marketplace")
	}
	for _, p := range targets {
		if p.Validate() != nil {
			return nil, diag.New(diag.Usage, "supply exact installed plugins as plugin@marketplace")
		}
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	for _, p := range targets {
		if _, ok := s.Registry.PackageByProvider(p); !ok {
			return nil, diag.Newf(diag.Usage, "plugin %q is not installed", p)
		}
	}
	plan := &UninstallPlan{providers: targets, references: removalReferences(s, targets), lifecycle: *s.Lifecycle}
	before := s.Registry.Effective()
	after := registry.New()
	*after = *s.Registry
	after.Packages = slices.Clone(s.Registry.Packages)
	after.Components = slices.Clone(s.Registry.Components)
	after.Conventions = slices.Clone(s.Registry.Conventions)
	for _, p := range targets {
		entry, _ := s.Registry.PackageByProvider(p)
		digest, err := adminstate.OwnedPackageDigest(h, p)
		if err != nil {
			return nil, diag.Wrap(diag.PluginUninstallFailed, err, "cannot inspect the owned plugin root")
		}
		comps, convs := factsOf(s.Registry, p)
		plan.targets = append(plan.targets, removalTarget{packageEntry: entry, components: comps, conventions: convs, digest: digest})
		after.RemoveProvider(p)
	}
	afterEffective := after.Effective()
	for _, t := range plan.targets {
		for _, c := range t.conventions {
			_, was := before.ConventionByRef(c.Ref())
			_, will := afterEffective.ConventionByRef(c.Ref())
			if was != will {
				plan.effects = append(plan.effects, ConventionEffect{Name: SafeText(c.Ref().String()), AvailableBefore: was, AvailableAfter: will})
			}
		}
	}
	// Root inspection is outside the epoch reader; reject any concurrent admin
	// transaction before presenting facts from more than one catalog revision.
	latest, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	if latest.Epoch != s.Epoch {
		return nil, diag.New(diag.AdministrationConflict, "plugin catalog changed; review removal again")
	}
	return plan, nil
}

func verifyRemoval(h workhome.Home, s *adminstate.Snapshot, p *UninstallPlan) error {
	conflict := func() error {
		return diag.New(diag.AdministrationConflict, "reviewed plugins or dependencies changed; review removal again")
	}
	if !slices.Equal(removalReferences(s, p.providers), p.references) {
		return conflict()
	}
	if slices.Contains(p.providers, adminstate.ReferenceProvider) && *s.Lifecycle != p.lifecycle {
		return conflict()
	}
	for _, t := range p.targets {
		pid := t.packageEntry.Provider
		current, ok := s.Registry.PackageByProvider(pid)
		comps, convs := factsOf(s.Registry, pid)
		if !ok || !reflect.DeepEqual(current, t.packageEntry) || !reflect.DeepEqual(comps, t.components) || !reflect.DeepEqual(convs, t.conventions) {
			return conflict()
		}
		digest, err := adminstate.OwnedPackageDigest(h, pid)
		if err != nil || digest != t.digest {
			return conflict()
		}
	}
	return nil
}

// Uninstall commits the reviewed set.
func Uninstall(ctx context.Context, h workhome.Home, p *UninstallPlan, removePolicy bool) ([]pluginid.Provider, error) {
	return UninstallWithOptions(ctx, h, p, removePolicy, adminstate.WriteOptions{})
}

// UninstallWithOptions commits the complete set. Only committed outcomes carry receipts,
// including a committed cleanup error; precommit failures carry none.
func UninstallWithOptions(ctx context.Context, h workhome.Home, p *UninstallPlan, removePolicy bool, opts adminstate.WriteOptions) ([]pluginid.Provider, error) {
	if p == nil || len(p.providers) == 0 {
		return nil, diag.New(diag.Usage, "review installed plugins before removal")
	}
	if len(p.references) > 0 && !removePolicy {
		return nil, diag.New(diag.PluginPolicyDependency, "removal drops policy Locators; explicitly approve --remove-policy-references")
	}
	operation := "work plugin uninstall " + strings.Join(ProviderNames(p.providers), " ") + " --yes"
	if removePolicy {
		operation += " --remove-policy-references"
	}
	err := adminstate.WithWriteOptions(ctx, h, operation, func(s *adminstate.Session) error {
		if err := verifyRemoval(h, s.Snapshot, p); err != nil {
			return err
		}
		for _, pid := range p.providers {
			if err := s.Remove(pid); err != nil {
				return err
			}
			s.Registry.RemoveProvider(pid)
		}
		s.Config.RepositoryResolution.Locators = slices.DeleteFunc(slices.Clone(s.Config.RepositoryResolution.Locators), func(ref string) bool { return slices.Contains(p.references, ref) })
		if slices.Contains(p.providers, adminstate.ReferenceProvider) {
			s.Lifecycle.MarkRemoved()
			s.Persist("lifecycle")
		}
		return nil
	}, opts)
	if err == nil || diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code {
		return p.Providers(), err
	}
	var typed *diag.Error
	if errors.As(err, &typed) {
		return nil, err
	}
	return nil, diag.Wrap(diag.PluginUninstallFailed, err, "plugin removal failed; the complete selected set was restored")
}
