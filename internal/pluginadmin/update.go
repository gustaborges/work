package pluginadmin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// Update statuses. Checks report the first four; applying reports updated or
// not-applied for the targets it touched.
const (
	UpdateUnchanged  = "unchanged"
	UpdateAvailable  = "update-available"
	UpdateWithdrawn  = "withdrawn"
	UpdateFailed     = "failed"
	UpdateUpdated    = "updated"
	UpdateNotApplied = "not-applied"
)

// UpdateError is the credential-safe failure of one target.
type UpdateError struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

// UpdateResult is the public outcome of one installed plugin. Versions are
// opaque: they are compared only for equality, never ordered.
type UpdateResult struct {
	ID                      string       `json:"id"`
	Name                    string       `json:"name"`
	Marketplace             string       `json:"marketplace"`
	Status                  string       `json:"status"`
	CurrentVersion          *string      `json:"current_version"`
	CandidateVersion        *string      `json:"candidate_version"`
	CurrentDigest           *string      `json:"current_digest"`
	CandidateDigest         *string      `json:"candidate_digest"`
	CandidateRevision       *string      `json:"candidate_revision"`
	Added                   []string     `json:"declarations_added"`
	Withdrawn               []string     `json:"declarations_withdrawn"`
	Changed                 []string     `json:"declarations_changed"`
	PolicyReferencesRemoved []string     `json:"policy_references_removed"`
	Committed               bool         `json:"committed"`
	CleanupPending          bool         `json:"cleanup_pending,omitempty"`
	Error                   *UpdateError `json:"error"`
}

// updateTarget is the frozen reviewed candidate of one installed plugin: the
// baseline it was judged against and, for an available update, the staged copy
// that Apply publishes without retrieving anything again.
type updateTarget struct {
	provider    pluginid.Provider
	prior       registry.Package
	components  []registry.Component
	conventions []registry.Convention
	source      *config.Marketplace // nil for the builtin provider
	manifest    *plugin.Manifest
	entry       *marketplace.ResolvedEntry
	prepared    *plugininstall.Prepared
	lease       *adminstate.Lease
	dropped     []string
	effects     []ConventionEffect
	result      UpdateResult
	transferred bool
}

// UpdatePlan owns the frozen candidates across review. Close is mandatory on
// every exit path and idempotent; nothing is persisted while preparing.
type UpdatePlan struct {
	home    workhome.Home
	Results []UpdateResult
	targets []*updateTarget
	epoch   uint64
	once    sync.Once
	closed  bool
}

// Close releases staged copies, resolved content and leases.
func (p *UpdatePlan) Close() {
	p.once.Do(func() {
		p.closed = true
		for _, t := range p.targets {
			t.close()
		}
	})
}

func (t *updateTarget) close() {
	if t.entry != nil {
		t.entry.Close()
	}
	if t.prepared != nil && !t.transferred {
		_ = os.RemoveAll(t.prepared.Stage)
	}
	t.lease.Release()
}

// ConventionEffects lists conventions of the target whose availability the
// update changes.
func (p *UpdatePlan) ConventionEffects(id string) []ConventionEffect {
	for _, t := range p.targets {
		if t.result.ID == SafeText(id) {
			return slices.Clone(t.effects)
		}
	}
	return nil
}

// InstalledProviders lists every installed plugin, optionally limited to one
// marketplace, from a coherent snapshot.
func InstalledProviders(s *adminstate.Snapshot, marketplaceName string) []pluginid.Provider {
	out := []pluginid.Provider{}
	for _, p := range s.Registry.ListPackages() {
		if marketplaceName == "" || p.Provider.Marketplace == marketplaceName {
			out = append(out, p.Provider)
		}
	}
	return out
}

func failResult(r *UpdateResult, err error) {
	r.Status = UpdateFailed
	r.Error = &UpdateError{Token: diag.Token(err), Message: "this plugin could not be prepared or applied; its installed copy is unchanged"}
	if d, ok := errors.AsType[*diag.Error](err); ok {
		r.Error.Message = SafeText(d.Msg)
	}
}

// groupKey identifies the source and branch whose single current revision
// every target of the group is judged against.
func groupKey(p registry.Provenance) marketplace.Source {
	return marketplace.Source{Kind: p.Kind, Source: p.Source, Branch: p.Branch}
}

// maintenanceSource proves that the recorded provenance still belongs to a
// registered source instance (or the code-owned builtin) and returns the
// source to resolve. Discovery state is deliberately not consulted: a missing
// stored offer never prevents a check, but an absent or different registration
// always does, so a name can never redirect maintenance to another origin.
func maintenanceSource(cfg *config.Config, pkg registry.Package) (*config.Marketplace, error) {
	pv := pkg.Provenance
	if pv.Kind == pluginid.KindBuiltin {
		if pv.RegistrationID != pluginid.BuiltinRegistrationID {
			return nil, diag.New(diag.MarketplaceEntryUnavailable, "the recorded source of this plugin is inconsistent")
		}
		return nil, nil
	}
	for _, m := range cfg.Marketplaces {
		if m.RegistrationID != pv.RegistrationID {
			continue
		}
		if m.Name != pv.MarketplaceName || m.Kind != pv.Kind || m.Source != pv.Source || m.Branch != pv.Branch {
			break
		}
		return &m, nil
	}
	return nil, diag.Newf(diag.MarketplaceEntryUnavailable, "the marketplace %q this plugin was copied from is no longer registered as it was; register it again and reinstall to change its source", pv.MarketplaceName)
}

// PrepareUpdates judges every provider against the current offer of its
// recorded source, resolving each source and branch once, and freezes the
// candidates of available updates. When stage is false (a check) nothing is
// copied: every candidate is still fully validated, even at an equal version.
//
// Nothing is persisted and no administration lock is held while sources are
// retrieved. Every provider is resolved before any source is contacted.
func PrepareUpdates(ctx context.Context, h workhome.Home, providers []pluginid.Provider, stage bool) (_ *UpdatePlan, err error) {
	targets := SortedProviders(providers)
	if len(targets) == 0 {
		return nil, diag.New(diag.Usage, "no installed plugins to inspect")
	}
	s, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	plan := &UpdatePlan{home: h, Results: []UpdateResult{}, epoch: s.Epoch}
	defer func() {
		if err != nil {
			plan.Close()
		}
	}()
	byGroup := map[marketplace.Source][]*updateTarget{}
	var order []marketplace.Source
	for _, p := range targets {
		if p.Validate() != nil {
			return nil, diag.New(diag.Usage, "supply exact installed plugins as plugin@marketplace")
		}
		pkg, ok := s.Registry.PackageByProvider(p)
		if !ok {
			return nil, diag.Newf(diag.Usage, "plugin %q is not installed", p)
		}
		t := &updateTarget{provider: p, prior: pkg, components: s.Registry.ProviderComponents(p), conventions: s.Registry.ProviderConventions(p)}
		t.result = UpdateResult{
			ID: SafeText(p.String()), Name: SafeText(p.Plugin), Marketplace: SafeText(p.Marketplace),
			CurrentVersion: nullable(pkg.Version), CurrentDigest: nullable(pkg.ContentDigest),
			Added: []string{}, Withdrawn: []string{}, Changed: []string{}, PolicyReferencesRemoved: []string{},
		}
		plan.targets = append(plan.targets, t)
		plan.Results = append(plan.Results, t.result)
		if t.source, err = maintenanceSource(s.Config, pkg); err != nil {
			failResult(&t.result, err)
			err = nil
			continue
		}
		key := groupKey(pkg.Provenance)
		if _, seen := byGroup[key]; !seen {
			order = append(order, key)
		}
		byGroup[key] = append(byGroup[key], t)
	}
	for _, key := range order {
		if err = ctx.Err(); err != nil {
			return nil, ClassifySourceError(err)
		}
		plan.judgeGroup(ctx, s, key, byGroup[key], stage)
	}
	if err = ctx.Err(); err != nil {
		return nil, ClassifySourceError(err)
	}
	plan.simulateEffects(s)
	for i, t := range plan.targets {
		plan.Results[i] = t.result
	}
	return plan, nil
}

// judgeGroup opens one candidate for the group and judges each member.
func (p *UpdatePlan) judgeGroup(ctx context.Context, s *adminstate.Snapshot, key marketplace.Source, members []*updateTarget, stage bool) {
	cand, err := marketplace.OpenCandidate(ctx, key, os.TempDir())
	if err != nil {
		err = ClassifySourceError(err)
		for _, t := range members {
			failResult(&t.result, err)
		}
		return
	}
	defer cand.Close()
	rev := SafeText(cand.Revision())
	for _, t := range members {
		if ctx.Err() != nil {
			return
		}
		if cand.Offer().Name != t.prior.Provenance.MarketplaceName {
			failResult(&t.result, diag.Newf(diag.MarketplaceNameConflict, "the source now publishes %q instead of %q; remove and register it again to change its namespace", cand.Offer().Name, t.prior.Provenance.MarketplaceName))
			continue
		}
		t.result.CandidateRevision = &rev
		p.judge(ctx, s, cand, t, stage)
	}
}

func (p *UpdatePlan) judge(ctx context.Context, s *adminstate.Snapshot, cand *marketplace.Candidate, t *updateTarget, stage bool) {
	entry, err := cand.Resolve(t.prior.Provenance.RegistrationID, t.provider.Plugin)
	if errors.Is(err, marketplace.ErrNoEntry) {
		// Withdrawal changes the offer, never the installed copy.
		t.result.Status = UpdateWithdrawn
		return
	}
	if err != nil {
		failResult(&t.result, ClassifySourceError(err))
		return
	}
	t.entry = entry
	t.result.CandidateVersion = nullable(entry.Entry.Version)
	t.result.CandidateDigest = nullable(entry.Entry.PackageDigest)
	available := entry.Entry.Version != t.prior.Version
	if available && stage {
		lease, dir, lerr := adminstate.NewPackageStage(ctx, p.home)
		if lerr != nil {
			failResult(&t.result, lerr)
			return
		}
		t.lease = lease
		if t.prepared, err = plugininstall.Prepare(entry, dir); err != nil {
			_ = os.RemoveAll(dir)
			failResult(&t.result, ClassifySourceError(err))
			return
		}
		t.manifest = t.prepared.Manifest
	} else {
		_, manifest, verr := plugininstall.Validate(entry)
		if verr != nil {
			failResult(&t.result, ClassifySourceError(verr))
			return
		}
		t.manifest = manifest
	}
	if !available {
		// Equal versions stay unchanged even when valid, changed bytes exist;
		// replacing them is an explicit reinstall from a refreshed offer.
		t.result.Status = UpdateUnchanged
		return
	}
	if err = fallbackConflict(s.Registry, t.provider, t.manifest.Components); err != nil {
		failResult(&t.result, err)
		return
	}
	t.result.Status = UpdateAvailable
	t.dropped = withdrawnReferences(s.Config, t.provider, t.components, t.manifest)
	t.result.PolicyReferencesRemoved = slices.Clone(t.dropped)
	t.declarationChanges()
}

// withdrawnReferences lists policy Locators owned by provider that the old
// copy registered as repository locators and the candidate no longer declares.
// Unrelated stale references are retained.
func withdrawnReferences(cfg *config.Config, p pluginid.Provider, old []registry.Component, next *plugin.Manifest) []string {
	registered := map[string]bool{}
	for _, c := range old {
		if c.Role == registry.RoleRepositoryLocator {
			registered[c.Name] = true
		}
	}
	kept := map[string]bool{}
	for _, c := range next.Components {
		if c.Role == registry.RoleRepositoryLocator {
			kept[c.Name] = true
		}
	}
	out := []string{}
	for _, ref := range cfg.RepositoryResolution.Locators {
		if r, err := pluginid.ParseRef(ref); err == nil && r.Provider == p && registered[r.Name] && !kept[r.Name] {
			out = append(out, ref)
		}
	}
	return out
}

func (t *updateTarget) declarationChanges() {
	old, next := map[string]bool{}, map[string]bool{}
	for _, c := range t.components {
		old["component "+c.Name+" ("+c.Role+")"] = true
	}
	for _, c := range t.conventions {
		old["convention "+c.Name] = true
	}
	for _, c := range t.manifest.Components {
		next["component "+c.Name+" ("+c.Role+")"] = true
	}
	for _, c := range t.manifest.Conventions {
		next["convention "+c.Name] = true
	}
	for _, k := range sortedKeys(next) {
		if !old[k] {
			t.result.Added = append(t.result.Added, SafeText(k))
		}
	}
	for _, k := range sortedKeys(old) {
		if !next[k] {
			t.result.Withdrawn = append(t.result.Withdrawn, SafeText(k))
		}
	}
	for _, c := range t.manifest.Components {
		for _, prev := range t.components {
			if prev.Name != c.Name || prev.Role != c.Role {
				continue
			}
			prev.Revision = ""
			if !reflect.DeepEqual(prev, plugininstall.ComponentEntry(t.provider, c)) {
				t.result.Changed = append(t.result.Changed, SafeText("component "+c.Name))
			}
		}
	}
	for _, c := range t.manifest.Conventions {
		for _, prev := range t.conventions {
			if prev.Name == c.Name && !slices.Equal(prev.Prefixes, c.Prefixes) {
				t.result.Changed = append(t.result.Changed, SafeText("convention "+c.Name))
			}
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// simulateEffects previews the whole successful batch in commit order, so each
// reviewed convention effect reflects the targets applied before it.
func (p *UpdatePlan) simulateEffects(s *adminstate.Snapshot) {
	sim := cloneRegistry(s.Registry)
	for _, t := range p.targets {
		if t.result.Status != UpdateAvailable {
			continue
		}
		before := sim.Effective()
		spec := plugininstall.Spec{
			Provider: t.provider, Manifest: t.manifest, Enabled: t.prior.Enabled,
			ContentDigest: t.result.valueDigest(), Provenance: plugininstall.ProvenanceOf(t.entry),
		}
		if _, err := plugininstall.Register(sim, spec); err != nil {
			continue
		}
		after := sim.Effective()
		names := map[string]bool{}
		for _, c := range t.conventions {
			names[c.Name] = true
		}
		for _, c := range t.manifest.Conventions {
			names[c.Name] = true
		}
		for _, name := range sortedKeys(names) {
			ref := pluginid.Ref{Provider: t.provider, Name: name}
			_, was := before.ConventionByRef(ref)
			_, will := after.ConventionByRef(ref)
			if was != will {
				t.effects = append(t.effects, ConventionEffect{Name: SafeText(ref.String()), AvailableBefore: was, AvailableAfter: will})
			}
		}
	}
}

func (r UpdateResult) valueDigest() string {
	if r.CandidateDigest == nil {
		return ""
	}
	return *r.CandidateDigest
}

func cloneRegistry(r *registry.Registry) *registry.Registry {
	c := *r
	c.Packages = slices.Clone(r.Packages)
	c.Components = slices.Clone(r.Components)
	c.Conventions = slices.Clone(r.Conventions)
	return &c
}

// Check prepares candidates for providers and reports them without staging or
// persisting anything. The rows are exposed only if the administrative state
// they were judged against is still current, so one result never mixes two
// catalog revisions.
func Check(ctx context.Context, h workhome.Home, providers []pluginid.Provider) ([]UpdateResult, error) {
	plan, err := PrepareUpdates(ctx, h, providers, false)
	if err != nil {
		return nil, err
	}
	defer plan.Close()
	latest, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	if latest.Epoch != plan.epoch {
		return nil, diag.New(diag.AdministrationConflict, "installed plugins changed while checking; check again")
	}
	return plan.Results, updateAggregate(plan.Results)
}

// aggregatePrecedence ranks the outcomes that outrank the generic update
// failure when a batch has several: a conflict or busy state tells the user
// more than "failed".
var aggregatePrecedence = []diag.Category{
	diag.AdministrationConflict, diag.AdministrationBusy, diag.PluginPolicyDependency,
}

func updateAggregate(rows []UpdateResult) error {
	failed, best := false, len(aggregatePrecedence)
	for _, r := range rows {
		if r.Status != UpdateFailed {
			continue
		}
		failed = true
		if r.Error == nil {
			continue
		}
		for rank, c := range aggregatePrecedence {
			if r.Error.Token == c.Token && rank < best {
				best = rank
			}
		}
	}
	const summary = "one or more plugin updates failed; inspect the per-plugin outcomes"
	switch {
	case best < len(aggregatePrecedence):
		return diag.New(aggregatePrecedence[best], summary)
	case failed:
		return diag.New(diag.PluginUpdateFailed, summary)
	}
	return nil
}

// verify compares exactly the facts the candidate depended on with the
// current state, under the administration lock.
func (t *updateTarget) verify(s *adminstate.Snapshot) error {
	conflict := func(why string) error {
		return diag.Newf(diag.AdministrationConflict, "%s; review the update again", why)
	}
	cur, ok := s.Registry.PackageByProvider(t.provider)
	if !ok || !reflect.DeepEqual(cur, t.prior) {
		return conflict("the reviewed plugin changed")
	}
	if !reflect.DeepEqual(s.Registry.ProviderComponents(t.provider), t.components) ||
		!reflect.DeepEqual(s.Registry.ProviderConventions(t.provider), t.conventions) {
		return conflict("the reviewed declarations changed")
	}
	if t.source != nil {
		reg, ok := registrationByID(s.Config, t.source.RegistrationID)
		if !ok || reg != *t.source {
			return conflict("the marketplace registration changed")
		}
	}
	if !slices.Equal(withdrawnReferences(s.Config, t.provider, t.components, t.manifest), t.dropped) {
		return conflict("the policy references that depend on this plugin changed")
	}
	return fallbackConflict(s.Registry, t.provider, t.manifest.Components)
}

func registrationByID(cfg *config.Config, id string) (config.Marketplace, bool) {
	for _, m := range cfg.Marketplaces {
		if m.RegistrationID == id {
			return m, true
		}
	}
	return config.Marketplace{}, false
}

// ApplyUpdates publishes each reviewed available update in its own
// transaction. The exact frozen candidate is applied: nothing is retrieved
// again, and a target whose reviewed facts drifted fails alone without
// widening what was authorized. Independent successes survive sibling
// failures; cancellation, recovery and cleanup failures stop the remainder.
func ApplyUpdates(ctx context.Context, plan *UpdatePlan, removePolicy bool) ([]UpdateResult, error) {
	return ApplyUpdatesWithOptions(ctx, plan, removePolicy, adminstate.WriteOptions{})
}

// ApplyUpdatesWithOptions is ApplyUpdates with transaction checkpoint hooks for tests.
func ApplyUpdatesWithOptions(ctx context.Context, plan *UpdatePlan, removePolicy bool, opts adminstate.WriteOptions) ([]UpdateResult, error) {
	if plan == nil || plan.closed {
		return nil, diag.New(diag.AdministrationConflict, "update candidates were closed; review the update again")
	}
	for _, t := range plan.targets {
		if t.result.Status == UpdateAvailable && len(t.dropped) > 0 && !removePolicy {
			return nil, diag.New(diag.PluginPolicyDependency, "update withdraws policy Locators; explicitly approve --remove-policy-references")
		}
	}
	rows := slices.Clone(plan.Results)
	var stop error
	for i, t := range plan.targets {
		if stop == nil && ctx.Err() != nil {
			stop = ClassifySourceError(ctx.Err())
		}
		if stop != nil {
			if t.result.Status == UpdateAvailable {
				rows[i].Status = UpdateNotApplied
				rows[i].Error = &UpdateError{Token: diag.Token(stop), Message: "the update was not applied"}
			}
			continue
		}
		if t.result.Status != UpdateAvailable {
			continue
		}
		operation := "work plugin update " + t.provider.String() + " --yes"
		err := adminstate.WithWriteOptions(ctx, plan.home, operation, func(s *adminstate.Session) error {
			if err := t.verify(s.Snapshot); err != nil {
				return err
			}
			if _, err := t.prepared.Publish(s, t.prior.Enabled); err != nil {
				return err
			}
			t.transferred = true
			s.Config.RepositoryResolution.Locators = slices.DeleteFunc(slices.Clone(s.Config.RepositoryResolution.Locators),
				func(ref string) bool { return slices.Contains(t.dropped, ref) })
			return nil
		}, opts)
		committed := err == nil || diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code
		switch {
		case err == nil:
			rows[i].Status, rows[i].Committed = UpdateUpdated, true
		case committed:
			rows[i].Status, rows[i].Committed, rows[i].CleanupPending = UpdateUpdated, true, true
			rows[i].Error = &UpdateError{Token: diag.Token(err), Message: "the update was committed but cleanup is pending"}
		default:
			if _, typed := errors.AsType[*diag.Error](err); !typed {
				err = diag.Wrap(diag.PluginUpdateFailed, err, "plugin update failed; the previous copy was restored")
			}
			failResult(&rows[i], err)
		}
		switch diag.ExitCode(err) {
		case diag.Cancelled.Code, diag.AdministrationRecoveryRequired.Code, diag.AdministrationCleanupFailed.Code:
			stop = err
		}
	}
	if stop != nil {
		return rows, stop
	}
	return rows, updateAggregate(rows)
}

// UpdateSummary renders one result's version change for human review.
func UpdateSummary(r UpdateResult) string {
	value := func(v *string) string {
		if v == nil {
			return "unknown"
		}
		return *v
	}
	return fmt.Sprintf("%s: %s  version %s → %s  digest %s → %s", r.ID, r.Status,
		value(r.CurrentVersion), value(r.CandidateVersion), value(r.CurrentDigest), value(r.CandidateDigest))
}
