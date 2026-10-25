package pluginadmin

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// UpdateResult is the public, credential-safe outcome of one resolved target.
type UpdateResult struct {
	Alias                   string       `json:"alias"`
	Origin                  string       `json:"origin"`
	Status                  string       `json:"status"`
	CurrentVersion          *string      `json:"current_version"`
	CandidateVersion        *string      `json:"candidate_version"`
	CurrentReference        *string      `json:"current_reference"`
	CandidateReference      *string      `json:"candidate_reference"`
	CurrentDigest           *string      `json:"current_digest"`
	CandidateDigest         *string      `json:"candidate_digest"`
	PolicyReferencesRemoved []string     `json:"policy_references_removed"`
	Error                   *UpdateError `json:"error"`
}
type UpdateError struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

// ConventionEffect records enabled ownership before and after one candidate.
type ConventionEffect struct {
	Name                            string
	AvailableBefore, AvailableAfter bool
	OwnersBefore, OwnersAfter       []string
}
type updateCandidate struct {
	baseline         registry.Package
	components       []registry.Component
	providers        []registry.Package
	conventions      []registry.Convention
	prepared         *plugininstall.Prepared
	result           UpdateResult
	added, withdrawn []string
	modified         []string
	effects          []ConventionEffect
}

// UpdatePlan owns frozen source copies across review. Close is idempotent and
// mandatory on every normal exit. Nothing is persisted during preparation.
type UpdatePlan struct {
	Results    []UpdateResult
	candidates []*updateCandidate
	once       sync.Once
	closed     bool
}

func (p *UpdatePlan) Close() {
	p.once.Do(func() {
		p.closed = true
		for _, c := range p.candidates {
			if c.prepared != nil {
				c.prepared.Close()
			}
		}
	})
}
func updateFailure(r *UpdateResult, err error) {
	r.Status = "failed"
	msg := "cannot prepare or apply this plugin; inspect its source and reinstall if needed"
	if d, ok := errors.AsType[*diag.Error](err); ok {
		msg = d.Msg
	}
	r.Error = &UpdateError{Token: diag.Token(err), Message: SafeText(msg)}
}
func updateAggregate(rows []UpdateResult) error {
	for _, r := range rows {
		if r.Status == "failed" {
			return diag.New(diag.PluginUpdateFailed, "one or more plugin updates failed; inspect the per-plugin outcomes")
		}
	}
	return nil
}
func aliasComponents(reg *registry.Registry, alias string) []registry.Component {
	out := []registry.Component{}
	for _, c := range reg.Components {
		if c.Alias == alias {
			out = append(out, c)
		}
	}
	return out
}
func providerFacts(reg *registry.Registry, names []string) ([]registry.Package, []registry.Convention) {
	owners := []registry.Package{}
	conventions := []registry.Convention{}
	for _, p := range reg.ListPackages() {
		for _, n := range names {
			if slices.Contains(p.Conventions, n) {
				enabled := reg.PackageEnabled(p.Alias)
				relevant := []string{}
				for _, name := range names {
					if slices.Contains(p.Conventions, name) {
						relevant = append(relevant, name)
					}
				}
				owners = append(owners, registry.Package{Alias: p.Alias, Enabled: &enabled, Conventions: relevant})
				break
			}
		}
	}
	for _, n := range names {
		if c, ok := reg.ConventionByName(n); ok {
			conventions = append(conventions, c)
		}
	}
	return owners, conventions
}
func affectedReferences(s *adminstate.Snapshot, c *updateCandidate) []string {
	out := []string{}
	for _, ref := range s.Config.RepositoryResolution.Locators {
		alias, name, ok := strings.Cut(ref, "/")
		if !ok || alias != c.baseline.Alias {
			continue
		}
		// Updates withdraw registered Locators only, retaining unrelated stale refs.
		oldLocator := false
		for _, old := range c.components {
			if old.Name == name && old.Role == registry.RoleRepositoryLocator {
				oldLocator = true
			}
		}
		retained := false
		for _, next := range c.prepared.Manifest.Components {
			if next.Name == name && next.Role == registry.RoleRepositoryLocator {
				retained = true
			}
		}
		if oldLocator && !retained {
			out = append(out, ref)
		}
	}
	return out
}
func candidateNames(c *updateCandidate) []string {
	names := slices.Clone(c.baseline.Conventions)
	if c.prepared != nil {
		for _, cv := range c.prepared.Manifest.Conventions {
			if !slices.Contains(names, cv.Name) {
				names = append(names, cv.Name)
			}
		}
	}
	slices.Sort(names)
	return names
}
func declarationChanges(c *updateCandidate) {
	old, next := []string{}, []string{}
	for _, v := range c.components {
		old = append(old, "component "+v.Name+" ("+v.Role+")")
	}
	for _, v := range c.baseline.Conventions {
		old = append(old, "convention "+v)
	}
	for _, v := range c.prepared.Manifest.Components {
		next = append(next, "component "+v.Name+" ("+v.Role+")")
	}
	for _, v := range c.prepared.Manifest.Conventions {
		next = append(next, "convention "+v.Name)
	}
	for _, v := range next {
		if !slices.Contains(old, v) {
			c.added = append(c.added, SafeText(v))
		}
	}
	for _, v := range old {
		if !slices.Contains(next, v) {
			c.withdrawn = append(c.withdrawn, SafeText(v))
		}
	}
	for _, next := range c.prepared.Manifest.Components {
		for _, old := range c.components {
			if old.Name == next.Name && old.Role == next.Role {
				old.Revision = ""
				if !reflect.DeepEqual(old, plugininstall.ComponentEntry(c.baseline.Alias, next)) {
					c.modified = append(c.modified, SafeText("component "+next.Name))
				}
			}
		}
	}
	for _, next := range c.prepared.Manifest.Conventions {
		for _, old := range c.conventions {
			if old.Name == next.Name && !slices.Equal(old.Prefixes, next.Prefixes) {
				c.modified = append(c.modified, SafeText("convention "+next.Name))
			}
		}
	}
}

// DeclarationChanges returns the determined added/withdrawn declarations.
func (p *UpdatePlan) DeclarationChanges(alias string) (added, withdrawn []string) {
	for _, c := range p.candidates {
		if c.baseline.Alias == alias {
			return slices.Clone(c.added), slices.Clone(c.withdrawn)
		}
	}
	return nil, nil
}
func (p *UpdatePlan) ModifiedDeclarations(alias string) []string {
	for _, c := range p.candidates {
		if c.baseline.Alias == alias {
			return slices.Clone(c.modified)
		}
	}
	return nil
}
func (p *UpdatePlan) ConventionEffects(alias string) []ConventionEffect {
	for _, c := range p.candidates {
		if c.baseline.Alias == alias {
			return slices.Clone(c.effects)
		}
	}
	return nil
}
func conventionEffects(reg *registry.Registry, c *updateCandidate) []ConventionEffect {
	after := *reg
	after.Packages = slices.Clone(reg.Packages)
	after.Components = slices.Clone(reg.Components)
	after.Conventions = slices.Clone(reg.Conventions)
	c.prepared.Register(&after)
	out := []ConventionEffect{}
	for _, name := range candidateNames(c) {
		effect := ConventionEffect{Name: SafeText(name), OwnersBefore: []string{}, OwnersAfter: []string{}}
		_, effect.AvailableBefore = reg.Effective().ConventionByName(name)
		_, effect.AvailableAfter = after.Effective().ConventionByName(name)
		for _, p := range reg.ListPackages() {
			if reg.PackageEnabled(p.Alias) && slices.Contains(p.Conventions, name) {
				effect.OwnersBefore = append(effect.OwnersBefore, SafeText(p.Alias))
			}
		}
		for _, p := range after.ListPackages() {
			if after.PackageEnabled(p.Alias) && slices.Contains(p.Conventions, name) {
				effect.OwnersAfter = append(effect.OwnersAfter, SafeText(p.Alias))
			}
		}
		if effect.AvailableBefore != effect.AvailableAfter || !slices.Equal(effect.OwnersBefore, effect.OwnersAfter) {
			out = append(out, effect)
		}
	}
	return out
}
func recordedSource(p registry.Package) string {
	if p.Source != "" {
		return p.Source
	}
	if p.Origin == registry.OriginRemotePinned {
		if at := strings.LastIndex(p.Reference, "@"); at >= 0 {
			return p.Reference[:at]
		}
	}
	return p.Reference
}

// PrepareUpdates resolves every alias before retrieving anything, then freezes
// one candidate per managed target outside all administrative locks.
func PrepareUpdates(ctx context.Context, h workhome.Home, aliases []string, all bool) (_ *UpdatePlan, err error) {
	if all && len(aliases) > 0 {
		return nil, diag.New(diag.Usage, "--all excludes explicit aliases")
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	targets := slices.Clone(aliases)
	if all || len(targets) == 0 {
		for _, p := range s.Registry.ListPackages() {
			targets = append(targets, p.Alias)
		}
	}
	slices.Sort(targets)
	targets = slices.Compact(targets)
	for _, a := range targets {
		if plugin.ValidateAlias(a) != nil {
			return nil, diag.New(diag.Usage, "supply exact installed plugin aliases")
		}
		if _, ok := s.Registry.PackageByAlias(a); !ok {
			return nil, diag.Newf(diag.Usage, "plugin alias %q is not installed", a)
		}
	}
	plan := &UpdatePlan{Results: []UpdateResult{}}
	defer func() {
		if err != nil {
			plan.Close()
		}
	}()
	for _, alias := range targets {
		baseline, _ := s.Registry.PackageByAlias(alias)
		c := &updateCandidate{baseline: baseline, components: aliasComponents(s.Registry, alias)}
		r := UpdateResult{Alias: SafeText(alias), Origin: SafeText(baseline.Origin), CurrentVersion: nullable(baseline.Version), CurrentReference: nullable(SafeReference(baseline.Reference, baseline.Origin)), CurrentDigest: nullable(baseline.ContentDigest), PolicyReferencesRemoved: []string{}}
		if ctx.Err() != nil {
			r.Status = "not-applied"
			r.Error = &UpdateError{Token: diag.Cancelled.Token, Message: "update preparation cancelled"}
		} else {
			switch baseline.Origin {
			case registry.OriginLocalLinked:
				r.Status = "source-managed"
			case registry.OriginEmbedded:
				r.Status = "not-applicable"
			case registry.OriginLocalPinned, registry.OriginRemotePinned:
				if baseline.Version == "" {
					updateFailure(&r, diag.New(diag.PluginInvalid, "installed version is unknown; explicitly reinstall this plugin to establish a baseline"))
					break
				}
				c.prepared, err = plugininstall.Prepare(ctx, recordedSource(baseline), plugininstall.Options{Alias: alias})
				if err != nil {
					updateFailure(&r, err)
					err = nil
					break
				}
				if e := c.prepared.ValidateCatalog(s.Registry); e != nil {
					updateFailure(&r, e)
					break
				}
				r.CandidateVersion = nullable(c.prepared.Manifest.Version)
				r.CandidateReference = nullable(SafeReference(c.prepared.Reference, c.prepared.Origin))
				r.CandidateDigest = nullable(c.prepared.Digest)
				r.Status = "unchanged"
				if baseline.Version != c.prepared.Manifest.Version {
					r.Status = "update-available"
				}
				r.PolicyReferencesRemoved = affectedReferences(s, c)
			default:
				updateFailure(&r, diag.New(diag.PluginInvalid, "plugin origin cannot be updated; explicitly reinstall it"))
			}
		}
		c.providers, c.conventions = providerFacts(s.Registry, candidateNames(c))
		if c.prepared != nil && r.Status != "failed" {
			declarationChanges(c)
		}
		c.result = r
		plan.candidates = append(plan.candidates, c)
		plan.Results = append(plan.Results, r)
	}
	// Preview the complete successful batch in commit order so two withdrawals
	// cannot both imply that the other's convention ownership will remain.
	simulated := *s.Registry
	simulated.Packages = slices.Clone(s.Registry.Packages)
	simulated.Components = slices.Clone(s.Registry.Components)
	simulated.Conventions = slices.Clone(s.Registry.Conventions)
	for _, candidate := range plan.candidates {
		if candidate.result.Status == "update-available" {
			candidate.effects = conventionEffects(&simulated, candidate)
			candidate.prepared.Register(&simulated)
		}
	}
	return plan, nil
}
func verifyCandidate(s *adminstate.Snapshot, c *updateCandidate) error {
	current, ok := s.Registry.PackageByAlias(c.baseline.Alias)
	if !ok || !reflect.DeepEqual(current, c.baseline) || !reflect.DeepEqual(aliasComponents(s.Registry, c.baseline.Alias), c.components) {
		return diag.New(diag.AdministrationConflict, "reviewed plugin changed; review the update again")
	}
	owners, conventions := providerFacts(s.Registry, candidateNames(c))
	if !reflect.DeepEqual(owners, c.providers) || !reflect.DeepEqual(conventions, c.conventions) {
		return diag.New(diag.AdministrationConflict, "reviewed convention providers changed; review the update again")
	}
	if c.prepared != nil && c.result.Status != "failed" {
		if !slices.Equal(affectedReferences(s, c), c.result.PolicyReferencesRemoved) {
			return diag.New(diag.AdministrationConflict, "reviewed policy dependencies changed; review the update again")
		}
		if c.result.Status != "failed" {
			if err := c.prepared.ValidateCatalog(s.Registry); err != nil {
				return diag.New(diag.AdministrationConflict, "catalog constraints changed; review the update again")
			}
		}
	}
	return nil
}

// Check retrieves only explicitly, buffers exhaustive rows, and revalidates
// target facts against a coherent snapshot before exposing any result.
func Check(ctx context.Context, h workhome.Home, aliases []string) ([]UpdateResult, error) {
	return check(ctx, h, aliases, nil)
}
func check(ctx context.Context, h workhome.Home, aliases []string, afterPrepare func()) ([]UpdateResult, error) {
	plan, err := PrepareUpdates(ctx, h, aliases, false)
	if err != nil {
		return nil, err
	}
	defer plan.Close()
	if afterPrepare != nil {
		afterPrepare()
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	for _, c := range plan.candidates {
		if err = verifyCandidate(s, c); err != nil {
			return nil, err
		}
	}
	return plan.Results, updateAggregate(plan.Results)
}

// Apply commits each reviewed package independently. Authorization is checked
// across the entire batch before the first package is staged.
func Apply(ctx context.Context, h workhome.Home, plan *UpdatePlan, removePolicy bool) ([]UpdateResult, error) {
	return ApplyWithOptions(ctx, h, plan, removePolicy, adminstate.WriteOptions{})
}
func ApplyWithOptions(ctx context.Context, h workhome.Home, plan *UpdatePlan, removePolicy bool, opts adminstate.WriteOptions) ([]UpdateResult, error) {
	if plan.closed {
		return nil, diag.New(diag.AdministrationConflict, "update candidates have been closed; review again")
	}
	for _, c := range plan.candidates {
		if c.result.Status == "update-available" && len(c.result.PolicyReferencesRemoved) > 0 && !removePolicy {
			return nil, diag.New(diag.PluginPolicyDependency, "update withdraws policy Locators; explicitly approve --remove-policy-references")
		}
	}
	rows := make([]UpdateResult, len(plan.candidates))
	for i, c := range plan.candidates {
		rows[i] = c.result
	}
	var stop error
	for i, c := range plan.candidates {
		if stop != nil || ctx.Err() != nil {
			if stop == nil {
				stop = diag.Wrap(diag.Cancelled, ctx.Err(), "update batch cancelled")
			}
			rows[i].Status = "not-applied"
			rows[i].Error = &UpdateError{Token: diag.Token(stop), Message: "update was not applied"}
			continue
		}
		if c.result.Status != "update-available" {
			snapshot, err := adminstate.Read(ctx, h)
			if err == nil {
				err = verifyCandidate(snapshot, c)
			}
			if err != nil {
				updateFailure(&rows[i], err)
				// A concurrent-change conflict only affects this package; recovery,
				// cleanup and cancellation make further targets unsafe to attempt.
				switch diag.ExitCode(err) {
				case diag.AdministrationRecoveryRequired.Code, diag.AdministrationCleanupFailed.Code, diag.Cancelled.Code:
					stop = err
				}
			}
			continue
		}
		type facts struct {
			providers   []registry.Package
			conventions []registry.Convention
		}
		refreshed := map[*updateCandidate]facts{}
		err := adminstate.WithWriteOptions(ctx, h, "work plugin update "+c.baseline.Alias+" --yes", func(s *adminstate.Session) error {
			if err := verifyCandidate(s.Snapshot, c); err != nil {
				return err
			}
			digest, err := plugininstall.ContentDigest(c.prepared.ContentDir)
			if err != nil || digest != c.prepared.Digest {
				return diag.New(diag.AdministrationConflict, "prepared content changed; review the update again")
			}
			stage, err := c.prepared.Stage(h.PluginsDir())
			if err != nil {
				return diag.Wrap(diag.PluginUpdateFailed, err, "cannot stage plugin update")
			}
			if err = s.Replace(c.baseline.Alias, stage); err != nil {
				_ = os.RemoveAll(stage)
				return err
			}
			// Only advance remaining provider facts that still match under this lock.
			// This recognizes this batch's own approved changes without accepting
			// unrelated provider drift that occurred since review.
			eligible := []*updateCandidate{}
			for _, next := range plan.candidates[i+1:] {
				owners, conventions := providerFacts(s.Registry, candidateNames(next))
				if reflect.DeepEqual(owners, next.providers) && reflect.DeepEqual(conventions, next.conventions) {
					eligible = append(eligible, next)
				}
			}
			c.prepared.Register(s.Registry)
			for _, next := range eligible {
				owners, conventions := providerFacts(s.Registry, candidateNames(next))
				refreshed[next] = facts{owners, conventions}
			}
			refs := c.result.PolicyReferencesRemoved
			s.Config.RepositoryResolution.Locators = slices.DeleteFunc(slices.Clone(s.Config.RepositoryResolution.Locators), func(ref string) bool { return slices.Contains(refs, ref) })
			return nil
		}, opts)
		if err == nil || diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code {
			for next, f := range refreshed {
				next.providers = f.providers
				next.conventions = f.conventions
			}
		}
		if err == nil {
			rows[i].Status = "updated"
			continue
		}
		if diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code {
			rows[i].Status = "updated"
			rows[i].Error = &UpdateError{Token: diag.Token(err), Message: "update committed; cleanup requires recovery"}
		} else {
			updateFailure(&rows[i], err)
		}
		// Same stop set as candidate verification above.
		switch diag.ExitCode(err) {
		case diag.AdministrationRecoveryRequired.Code, diag.AdministrationCleanupFailed.Code, diag.Cancelled.Code:
			stop = err
		}
	}
	if stop != nil {
		return rows, stop
	}
	return rows, updateAggregate(rows)
}
