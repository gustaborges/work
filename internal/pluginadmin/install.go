package pluginadmin

import (
	"context"
	"errors"
	"os"
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

// Install results.
const (
	ResultInstalled   = "installed"
	ResultReinstalled = "reinstalled"
	ResultUnchanged   = "unchanged"
)

// InstallResult is the outcome of an add. Committed distinguishes an applied
// transaction from a failure before commitment when an error is also returned.
type InstallResult struct {
	Provider      pluginid.Provider
	Revision      string
	ContentDigest string
	Result        string
	Committed     bool
}

// InstallPlan is one reviewed installation: the fixed offered entry, its staged
// and validated copy, and the registry facts it was prepared against. Close is
// mandatory on every exit path and idempotent.
//
// Planning runs outside the administration lock, so a slow Git retrieval or
// copy never blocks other commands; Apply revalidates exactly the facts it
// depends on under the lock and conflicts instead of widening what was
// reviewed.
type InstallPlan struct {
	home     workhome.Home
	provider pluginid.Provider
	result   string

	entry    *marketplace.ResolvedEntry
	prepared *plugininstall.Prepared
	source   *config.Marketplace // nil for the builtin provider
	prior    *registry.Package
	dropped  []string // policy references the replacement would orphan

	stageLease   *adminstate.Lease
	captureLease *adminstate.Lease
	transferred  bool
	once         sync.Once
}

// Provider is the plugin the plan installs.
func (p *InstallPlan) Provider() pluginid.Provider { return p.provider }

// Result says what Apply will do: installed, reinstalled or unchanged.
func (p *InstallPlan) Result() string { return p.result }

// PolicyReferences are the repository-resolution Locators that a replacement
// would orphan because the new package no longer declares them. Reinstalling
// over them needs separate, explicit authorization.
func (p *InstallPlan) PolicyReferences() []string { return slices.Clone(p.dropped) }

// Replacing describes the installed copy a reinstall overwrites.
func (p *InstallPlan) Replacing() (registry.Package, bool) {
	if p.prior == nil {
		return registry.Package{}, false
	}
	return *p.prior, true
}

// Offered is the fixed entry the plan would install.
func (p *InstallPlan) Offered() marketplace.Entry { return p.entry.Entry }

// Close releases the staged copy, the resolved content and both leases.
func (p *InstallPlan) Close() {
	p.once.Do(func() {
		if p.entry != nil {
			p.entry.Close()
		}
		if p.prepared != nil && !p.transferred {
			_ = os.RemoveAll(p.prepared.Stage)
		}
		p.stageLease.Release()
		p.captureLease.Release()
	})
}

// PrepareInstall resolves plugin in marketplaceName from its stored, fixed
// offer and stages a validated copy. Nothing is persisted. The offer may be
// browsed offline; the content comes from the fixed revision or capture and is
// never replaced by a newer tip or by whatever the source directory holds now.
func PrepareInstall(ctx context.Context, h workhome.Home, pluginName, marketplaceName string) (_ *InstallPlan, err error) {
	provider, perr := pluginid.NewProvider(pluginName, marketplaceName)
	if perr != nil {
		return nil, diag.New(diag.Usage, "supply the plugin as plugin@marketplace")
	}
	snap, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	plan := &InstallPlan{home: h, provider: provider}
	defer func() {
		if err != nil {
			plan.Close()
			err = ClassifySourceError(err)
		}
	}()
	if err = plan.resolve(ctx, snap); err != nil {
		return nil, err
	}
	if cur, ok := snap.Registry.PackageByProvider(provider); ok {
		plan.prior = &cur
	}
	plan.result = ResultInstalled
	if plan.prior != nil {
		plan.result = ResultReinstalled
		if plan.prior.ContentDigest == plan.entry.Entry.PackageDigest &&
			plan.prior.Provenance.RegistrationID == plan.entry.RegistrationID &&
			snap.Inspection[provider.String()] == "ready" {
			plan.result = ResultUnchanged
			return plan, nil
		}
	}
	if plan.stageLease, err = plan.stage(ctx); err != nil {
		return nil, err
	}
	if err = fallbackConflict(snap.Registry, provider, plan.prepared.Manifest.Components); err != nil {
		return nil, err
	}
	plan.dropped = droppedReferences(snap.Config, provider, plan.prepared)
	return plan, nil
}

// stage copies the validated package into a leased stage outside any journal.
func (p *InstallPlan) stage(ctx context.Context) (*adminstate.Lease, error) {
	lease, dir, err := adminstate.NewPackageStage(ctx, p.home)
	if err != nil {
		return nil, err
	}
	if p.prepared, err = plugininstall.Prepare(p.entry, dir); err != nil {
		_ = os.RemoveAll(dir)
		lease.Release()
		return nil, err
	}
	return lease, nil
}

// resolve opens the fixed content of the requested entry.
func (p *InstallPlan) resolve(ctx context.Context, s *adminstate.Snapshot) error {
	if pluginid.IsReserved(p.provider.Marketplace) {
		if p.provider.Marketplace != pluginid.Builtin {
			return diag.Newf(diag.Usage, "marketplace %q is not registered", p.provider.Marketplace)
		}
		snap, err := marketplace.BuiltinSnapshot()
		if err != nil {
			return diag.Wrap(diag.BootstrapFailed, err, "the included offer is unavailable")
		}
		return p.bind(snap, pluginid.BuiltinRegistrationID)
	}
	src, ok := s.Config.Marketplace(p.provider.Marketplace)
	if !ok {
		return diag.Newf(diag.Usage, "marketplace %q is not registered; register it with `work plugin marketplace add`", p.provider.Marketplace)
	}
	p.source = &src
	stored, ok := s.Catalogs.ByRegistration(src.RegistrationID)
	if !ok || stored.Namespace != src.Name || stored.Source.Kind != src.Kind || stored.Source.Source != src.Source || stored.Source.Branch != src.Branch {
		return diag.Newf(diag.MarketplaceEntryUnavailable, "the stored offer of marketplace %q is missing or does not match its source; run `work plugin marketplace upgrade %s`", src.Name, src.Name)
	}
	if _, ok := stored.StoredEntry(p.provider.Plugin); !ok {
		return diag.Newf(diag.Usage, "marketplace %q does not offer a plugin named %q", src.Name, p.provider.Plugin)
	}
	var snap *marketplace.Snapshot
	var err error
	switch stored.Source.Kind {
	case pluginid.KindLocal:
		hexDigest, valid := marketplace.HexPayload(valueOf(stored.CaptureDigest))
		if !valid {
			return diag.New(diag.MarketplaceEntryUnavailable, "the stored offer has no usable capture")
		}
		if p.captureLease, err = adminstate.HoldCapture(ctx, p.home, hexDigest); err != nil {
			return diag.Wrap(diag.MarketplaceSourceUnavailable, err, "the captured marketplace content is unavailable")
		}
		dir, derr := p.home.SnapshotDir(hexDigest)
		if derr != nil {
			return derr
		}
		snap, err = marketplace.OpenCapture(dir, src.Source)
	case pluginid.KindGit:
		snap, err = marketplace.OpenGit(ctx, src.Source, src.Branch, stored.Revision, os.TempDir())
	default:
		return diag.New(diag.MarketplaceEntryUnavailable, "the stored offer has an unsupported source")
	}
	if err != nil {
		return err
	}
	if !snap.MatchesStored(*stored) {
		snap.Close()
		return diag.Newf(diag.MarketplaceSourceUnavailable, "the fixed content of marketplace %q no longer matches its stored offer", src.Name)
	}
	return p.bind(snap, src.RegistrationID)
}

func valueOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (p *InstallPlan) bind(snap *marketplace.Snapshot, registrationID string) error {
	entry, err := snap.Resolve(registrationID, p.provider.Plugin)
	if err != nil {
		snap.Close()
		if errors.Is(err, marketplace.ErrNoEntry) {
			return diag.Newf(diag.Usage, "marketplace %q does not offer a plugin named %q", p.provider.Marketplace, p.provider.Plugin)
		}
		return err
	}
	p.entry = entry
	return nil
}

// fallbackConflict enforces the single fallback Starter: a second provider may
// not register one, and the error names both complete providers.
func fallbackConflict(reg *registry.Registry, p pluginid.Provider, incoming []plugin.Component) error {
	for _, c := range incoming {
		if !c.IsFallbackStarter() {
			continue
		}
		if fb, ok := reg.StarterFallback(); ok && fb.Provider != p {
			return diag.Newf(diag.PluginFallbackConflict, "a fallback Starter is already registered by %s; installing %s would give two", fb.Provider, p)
		}
	}
	return nil
}

// droppedReferences lists policy Locators owned by provider that the prepared
// package no longer declares as repository locators.
func droppedReferences(cfg *config.Config, p pluginid.Provider, prep *plugininstall.Prepared) []string {
	still := map[string]bool{}
	for _, c := range prep.Manifest.Components {
		if c.Role == registry.RoleRepositoryLocator {
			still[pluginid.Ref{Provider: p, Name: c.Name}.String()] = true
		}
	}
	out := []string{}
	for _, ref := range cfg.RepositoryResolution.Locators {
		if r, err := pluginid.ParseRef(ref); err == nil && r.Provider == p && !still[ref] {
			out = append(out, ref)
		}
	}
	return out
}

// Apply publishes the reviewed copy and registers its declarations in one
// administrative transaction. An unchanged plan commits nothing. Facts the plan
// depended on that changed since preparation conflict (exit 53); unrelated
// concurrent changes merge.
func (p *InstallPlan) Apply(ctx context.Context, removePolicy bool) (InstallResult, error) {
	return p.ApplyWithOptions(ctx, removePolicy, adminstate.WriteOptions{})
}

// ApplyWithOptions is Apply with transaction checkpoint hooks for tests.
func (p *InstallPlan) ApplyWithOptions(ctx context.Context, removePolicy bool, opts adminstate.WriteOptions) (InstallResult, error) {
	res := InstallResult{
		Provider: p.provider, Revision: p.entry.Revision, ContentDigest: p.entry.Entry.PackageDigest, Result: p.result,
	}
	if p.result == ResultUnchanged {
		return res, nil
	}
	if ctx.Err() != nil {
		return InstallResult{}, ClassifySourceError(ctx.Err())
	}
	if len(p.dropped) > 0 && !removePolicy {
		return InstallResult{}, diag.New(diag.PluginPolicyDependency, "reinstalling drops policy Locators; explicitly approve --remove-policy-references")
	}
	operation := "work plugin add " + p.provider.String()
	err := adminstate.WithWriteOptions(ctx, p.home, operation, func(s *adminstate.Session) error {
		if err := p.verify(s.Snapshot); err != nil {
			return err
		}
		enabled := p.prior == nil || p.prior.Enabled
		if _, err := p.prepared.Publish(s, enabled); err != nil {
			return err
		}
		p.transferred = true
		if p.provider == adminstate.ReferenceProvider && !(s.Lifecycle.Initialized && s.Lifecycle.PolicyHandled) {
			// Choosing the reference explicitly settles first-use setup and the
			// default policy: a later setup must neither reinstall it nor seed
			// the default Locator the user did not ask for.
			s.Lifecycle.Initialized = true
			s.Lifecycle.MarkPolicyHandled()
			s.Persist("lifecycle")
		}
		if len(p.dropped) > 0 {
			s.Config.RepositoryResolution.Locators = slices.DeleteFunc(slices.Clone(s.Config.RepositoryResolution.Locators),
				func(ref string) bool { return slices.Contains(p.dropped, ref) })
		}
		return nil
	}, opts)
	res.Committed = err == nil || diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code
	if err != nil {
		var typed *diag.Error
		if errors.As(err, &typed) {
			return res, err
		}
		return res, diag.Wrap(diag.PluginInstallFailed, err, "plugin installation failed; the previous state was restored")
	}
	return res, nil
}

// verify compares exactly the facts the plan depended on with the current
// state, under the administration lock.
func (p *InstallPlan) verify(s *adminstate.Snapshot) error {
	conflict := func(why string) error {
		return diag.Newf(diag.AdministrationConflict, "%s; review the installation again", why)
	}
	if p.source != nil {
		cur, ok := s.Config.Marketplace(p.source.Name)
		if !ok || cur != *p.source {
			return conflict("the marketplace registration changed")
		}
		stored, ok := s.Catalogs.ByRegistration(cur.RegistrationID)
		if !ok {
			return conflict("the marketplace offer is no longer stored")
		}
		offered, ok := stored.StoredEntry(p.provider.Plugin)
		if !ok || offered.PackageDigest != p.entry.Entry.PackageDigest || offered.Path != p.entry.Entry.Path {
			return conflict("the offered plugin changed")
		}
	}
	cur, installed := s.Registry.PackageByProvider(p.provider)
	switch {
	case installed != (p.prior != nil):
		return conflict("the plugin was installed or removed concurrently")
	case installed && cur != *p.prior:
		return conflict("the installed plugin changed")
	}
	if err := fallbackConflict(s.Registry, p.provider, p.prepared.Manifest.Components); err != nil {
		return err
	}
	if !slices.Equal(droppedReferences(s.Config, p.provider, p.prepared), p.dropped) {
		return conflict("the policy references that depend on this plugin changed")
	}
	return nil
}
