package pluginadmin

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

type removalTarget struct {
	packageEntry registry.Package
	components   []registry.Component
	digest       string
}

// UninstallPlan retains private reviewed facts. Accessors return independent
// views so presentation cannot expand the set or dependency authorization.
type UninstallPlan struct {
	aliases, references, names []string
	targets                    []removalTarget
	providers                  []registry.Package
	conventions                []registry.Convention
	effects                    []ConventionEffect
	lifecycle                  adminstate.Lifecycle
}

func (p *UninstallPlan) Aliases() []string          { return slices.Clone(p.aliases) }
func (p *UninstallPlan) PolicyReferences() []string { return slices.Clone(p.references) }
func (p *UninstallPlan) ConventionEffects() []ConventionEffect {
	out := slices.Clone(p.effects)
	for i := range out {
		out[i].OwnersBefore = slices.Clone(out[i].OwnersBefore)
		out[i].OwnersAfter = slices.Clone(out[i].OwnersAfter)
	}
	return out
}
func removalReferences(s *adminstate.Snapshot, aliases []string) []string {
	out := []string{}
	for _, ref := range s.Config.RepositoryResolution.Locators {
		alias, _, ok := strings.Cut(ref, "/")
		if ok && slices.Contains(aliases, alias) {
			out = append(out, ref)
		}
	}
	return out
}

// PlanUninstall reads a coherent catalog and owned roots without retrieving or
// invoking any source. Every alias is resolved before inspecting any target.
func PlanUninstall(ctx context.Context, h workhome.Home, aliases []string) (*UninstallPlan, error) {
	targets := slices.Clone(aliases)
	slices.Sort(targets)
	targets = slices.Compact(targets)
	if len(targets) == 0 {
		return nil, diag.New(diag.Usage, "supply at least one installed plugin alias")
	}
	for _, a := range targets {
		if plugin.ValidateAlias(a) != nil {
			return nil, diag.New(diag.Usage, "supply exact installed plugin aliases")
		}
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	for _, a := range targets {
		if _, ok := s.Registry.PackageByAlias(a); !ok {
			return nil, diag.Newf(diag.Usage, "plugin alias %q is not installed", a)
		}
	}
	p := &UninstallPlan{aliases: targets, references: removalReferences(s, targets), lifecycle: *s.Lifecycle}
	after := *s.Registry
	after.Packages = slices.Clone(s.Registry.Packages)
	after.Components = slices.Clone(s.Registry.Components)
	after.Conventions = slices.Clone(s.Registry.Conventions)
	for _, a := range targets {
		entry, _ := s.Registry.PackageByAlias(a)
		digest, err := adminstate.OwnedPackageDigest(h, a)
		if err != nil {
			return nil, diag.Wrap(diag.PluginUninstallFailed, err, "cannot inspect the owned plugin root")
		}
		p.targets = append(p.targets, removalTarget{packageEntry: entry, components: aliasComponents(s.Registry, a), digest: digest})
		p.names = append(p.names, entry.Conventions...)
		after.RemoveAlias(a)
	}
	slices.Sort(p.names)
	p.names = slices.Compact(p.names)
	p.providers, p.conventions = providerFacts(s.Registry, p.names)
	for _, name := range p.names {
		e := ConventionEffect{Name: SafeText(name), OwnersBefore: []string{}, OwnersAfter: []string{}}
		_, e.AvailableBefore = s.Registry.Effective().ConventionByName(name)
		_, e.AvailableAfter = after.Effective().ConventionByName(name)
		for _, entry := range s.Registry.ListPackages() {
			if s.Registry.PackageEnabled(entry.Alias) && slices.Contains(entry.Conventions, name) {
				e.OwnersBefore = append(e.OwnersBefore, SafeText(entry.Alias))
			}
		}
		for _, entry := range after.ListPackages() {
			if after.PackageEnabled(entry.Alias) && slices.Contains(entry.Conventions, name) {
				e.OwnersAfter = append(e.OwnersAfter, SafeText(entry.Alias))
			}
		}
		if e.AvailableBefore != e.AvailableAfter || !slices.Equal(e.OwnersBefore, e.OwnersAfter) {
			p.effects = append(p.effects, e)
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
	return p, nil
}
func verifyRemoval(h workhome.Home, s *adminstate.Snapshot, p *UninstallPlan) error {
	conflict := func() error {
		return diag.New(diag.AdministrationConflict, "reviewed plugins or dependencies changed; review removal again")
	}
	if !slices.Equal(removalReferences(s, p.aliases), p.references) {
		return conflict()
	}
	owners, conventions := providerFacts(s.Registry, p.names)
	if !reflect.DeepEqual(owners, p.providers) || !reflect.DeepEqual(conventions, p.conventions) {
		return conflict()
	}
	if slices.Contains(p.aliases, adminstate.ReferenceAlias) && *s.Lifecycle != p.lifecycle {
		return conflict()
	}
	for _, t := range p.targets {
		current, ok := s.Registry.PackageByAlias(t.packageEntry.Alias)
		if !ok || !reflect.DeepEqual(current, t.packageEntry) || !reflect.DeepEqual(aliasComponents(s.Registry, t.packageEntry.Alias), t.components) {
			return conflict()
		}
		digest, err := adminstate.OwnedPackageDigest(h, t.packageEntry.Alias)
		if err != nil || digest != t.digest {
			return conflict()
		}
	}
	return nil
}

func Uninstall(ctx context.Context, h workhome.Home, p *UninstallPlan, removePolicy bool) ([]string, error) {
	return UninstallWithOptions(ctx, h, p, removePolicy, adminstate.WriteOptions{})
}

// UninstallWithOptions commits the complete set. Only committed outcomes carry receipts,
// including a committed cleanup error; precommit failures carry none.
func UninstallWithOptions(ctx context.Context, h workhome.Home, p *UninstallPlan, removePolicy bool, opts adminstate.WriteOptions) ([]string, error) {
	if p == nil || len(p.aliases) == 0 {
		return nil, diag.New(diag.Usage, "review installed plugins before removal")
	}
	if len(p.references) > 0 && !removePolicy {
		return nil, diag.New(diag.PluginPolicyDependency, "uninstall removes policy Locators; explicitly approve --remove-policy-references")
	}
	operation := "work plugin uninstall " + strings.Join(p.aliases, " ") + " --yes"
	if removePolicy {
		operation += " --remove-policy-references"
	}
	err := adminstate.WithWriteOptions(ctx, h, operation, func(s *adminstate.Session) error {
		if err := verifyRemoval(h, s.Snapshot, p); err != nil {
			return err
		}
		for _, a := range p.aliases {
			if err := s.Remove(a); err != nil {
				return err
			}
			s.Registry.RemoveAlias(a)
		}
		s.Config.RepositoryResolution.Locators = slices.DeleteFunc(slices.Clone(s.Config.RepositoryResolution.Locators), func(ref string) bool { return slices.Contains(p.references, ref) })
		if slices.Contains(p.aliases, adminstate.ReferenceAlias) {
			s.Lifecycle.MarkRemoved()
			s.Persist("lifecycle")
		}
		return nil
	}, opts)
	if err == nil || diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code {
		return p.Aliases(), err
	}
	var typed *diag.Error
	if errors.As(err, &typed) {
		return nil, err
	}
	return nil, diag.Wrap(diag.PluginUninstallFailed, err, "plugin removal failed; the complete selected set was restored")
}
