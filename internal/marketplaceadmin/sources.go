// Package marketplaceadmin owns marketplace source policy: registering and
// refreshing Git and local sources, and the pure projection of sources and
// their local installed totals. A registration only publishes discovery; it
// never installs a plugin, executes plugin code or changes repository policy.
package marketplaceadmin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

// Operation labels recorded in journals; recovery maps them back to commands.
const (
	opAdd     = "work plugin marketplace add"
	opUpgrade = "work plugin marketplace upgrade"
)

var (
	schemeURL = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
	scpLike   = regexp.MustCompile(`^[A-Za-z0-9._~-]+@[A-Za-z0-9._-]+:[^\\]`)
	drivePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

// Spec is a normalized source request.
type Spec struct {
	Kind   string
	Source string
	Branch string // git only; empty means discover the default at registration
}

// Classify normalizes raw user input without touching the network or state. A
// URL or scp-style address selects Git transport; anything else is a local
// directory path captured live (a checkout is not treated as Git, and
// `file://` explicitly selects committed Git content). There is no hosting
// shorthand.
func Classify(source, branch string) (Spec, error) {
	source = strings.TrimSpace(source)
	branch = strings.TrimSpace(branch)
	if source == "" {
		return Spec{}, diag.New(diag.Usage, "supply a marketplace SOURCE: a Git URL or a local directory")
	}
	gitLike := schemeURL.MatchString(source) || (scpLike.MatchString(source) && !drivePath.MatchString(source))
	if gitLike {
		if strings.ContainsAny(source, "\x00\n\r") {
			return Spec{}, diag.New(diag.Usage, "the marketplace SOURCE is not a valid Git address")
		}
		if branch != "" && (strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, "\x00\n\r ")) {
			return Spec{}, diag.New(diag.Usage, "--branch is not a valid branch name")
		}
		return Spec{Kind: pluginid.KindGit, Source: source, Branch: branch}, nil
	}
	if branch != "" {
		return Spec{}, diag.New(diag.Usage, "--branch applies to Git sources only")
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return Spec{}, diag.Wrap(diag.Usage, err, "the marketplace SOURCE is not a usable path")
	}
	return Spec{Kind: pluginid.KindLocal, Source: filepath.Clean(abs)}, nil
}

// Classification of failures to the contracted exit categories.
func classify(err error) error {
	var typed *diag.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &typed):
		return err
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return diag.Wrap(diag.Cancelled, err, "marketplace operation cancelled")
	case errors.Is(err, marketplace.ErrInvalid):
		return diag.Wrap(diag.MarketplaceInvalid, err, safeInvalid(err))
	case errors.Is(err, marketplace.ErrChanged):
		return diag.Wrap(diag.MarketplaceSourceUnavailable, err, "the marketplace source changed while it was being captured; retry when it is stable")
	case errors.Is(err, marketplace.ErrSourceUnavailable):
		return diag.Wrap(diag.MarketplaceSourceUnavailable, err, "the marketplace source, branch or revision is unavailable")
	}
	return err
}

// safeInvalid keeps validation detail (paths and names inside the catalog) but
// drops any wrapped transport cause.
func safeInvalid(err error) string {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, marketplace.ErrInvalid.Error()+": ")
	return "the marketplace catalog is invalid: " + msg
}

func newRegistrationID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sameDescriptor(m config.Marketplace, s Spec) bool {
	if m.Kind != s.Kind || m.Source != s.Source {
		return false
	}
	return s.Branch == "" || m.Branch == s.Branch
}

// AddResult is the outcome of a registration.
type AddResult struct {
	Name      string
	Result    string // registered | unchanged
	Revision  string
	Committed bool
}

// AddPlan is a prepared registration: a validated fixed offer and, for local
// sources, a leased private capture. Close releases both and is mandatory on
// every exit path; it is idempotent.
type AddPlan struct {
	home    workhome.Home
	spec    Spec
	reused  *config.Marketplace // exact existing registration, when unchanged
	snap    *marketplace.Snapshot
	lease   *adminstate.Lease
	stage   string
	baseEpo uint64
	once    sync.Once
}

// Name is the published namespace the plan would register.
func (p *AddPlan) Name() string {
	if p.reused != nil {
		return p.reused.Name
	}
	return p.snap.Offer.Name
}

// Offer is the fixed offer the registration would publish; nil when unchanged.
func (p *AddPlan) Offer() *marketplace.Offer {
	if p.snap == nil {
		return nil
	}
	return p.snap.Offer
}

// Close releases the lease and any scratch content. It is idempotent.
func (p *AddPlan) Close() {
	p.once.Do(func() {
		if p.snap != nil {
			p.snap.Close()
		}
		if p.lease != nil {
			p.lease.Release()
		}
	})
}

// PrepareAdd resolves and validates a source outside the administration lock.
// An exact existing registration returns an unchanged plan without any
// retrieval. Nothing is persisted.
func PrepareAdd(ctx context.Context, h workhome.Home, spec Spec) (_ *AddPlan, err error) {
	state, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	for i := range state.Config.Marketplaces {
		if m := state.Config.Marketplaces[i]; sameDescriptor(m, spec) {
			return &AddPlan{home: h, spec: spec, reused: &m, baseEpo: state.Epoch}, nil
		}
	}
	plan := &AddPlan{home: h, spec: spec, baseEpo: state.Epoch}
	defer func() {
		if err != nil {
			plan.Close()
			err = classify(err)
		}
	}()
	switch spec.Kind {
	case pluginid.KindLocal:
		var stage string
		if plan.lease, stage, err = adminstate.NewCaptureStage(ctx, h); err != nil {
			return nil, err
		}
		plan.stage = stage
		if plan.snap, err = marketplace.CaptureLocal(ctx, spec.Source, stage, marketplace.CaptureOptions{}); err != nil {
			return nil, err
		}
	case pluginid.KindGit:
		branch := spec.Branch
		if branch == "" {
			if branch, err = marketplace.DiscoverBranch(ctx, spec.Source); err != nil {
				return nil, err
			}
		}
		if plan.snap, err = marketplace.OpenGit(ctx, spec.Source, branch, "", os.TempDir()); err != nil {
			return nil, err
		}
		plan.spec.Branch = branch
	default:
		return nil, diag.New(diag.Usage, "unsupported marketplace source kind")
	}
	if err = checkNamespace(state.Config, plan.snap.Offer.Name); err != nil {
		return nil, err
	}
	return plan, nil
}

// checkNamespace rejects a namespace that is reserved for the included
// provider, invalid, or already published by another registered source.
func checkNamespace(cfg *config.Config, name string) error {
	if pluginid.IsReserved(name) {
		return diag.New(diag.MarketplaceNameConflict, "the namespace \"builtin\" is reserved for the included provider")
	}
	if err := pluginid.ValidateRegisteredMarketplace(name); err != nil {
		return diag.Wrap(diag.MarketplaceInvalid, err, "the marketplace name is invalid: "+err.Error())
	}
	for _, m := range cfg.Marketplaces {
		if pluginid.Fold(m.Name) == pluginid.Fold(name) {
			return diag.Newf(diag.MarketplaceNameConflict, "the namespace %q is already published by another registered source", name)
		}
	}
	return nil
}

// Apply registers the reviewed offer. The registration, its first catalog and
// the capture reference commit together; the default repository-resolution
// policy is not touched. Facts that drifted since preparation conflict
// instead of expanding what was reviewed.
func (p *AddPlan) Apply(ctx context.Context) (AddResult, error) {
	if p.reused != nil {
		return AddResult{Name: p.reused.Name, Result: "unchanged"}, nil
	}
	if ctx.Err() != nil {
		return AddResult{}, classify(ctx.Err())
	}
	id, err := newRegistrationID()
	if err != nil {
		return AddResult{}, err
	}
	err = adminstate.WithWrite(ctx, p.home, opAdd+" "+p.spec.Source, func(s *adminstate.Session) error {
		for _, m := range s.Config.Marketplaces {
			if sameDescriptor(m, p.spec) {
				return diag.New(diag.AdministrationConflict, "the source was registered concurrently; review again")
			}
		}
		if err := checkNamespace(s.Config, p.snap.Offer.Name); err != nil {
			return err
		}
		if p.snap.Source.Kind == pluginid.KindLocal {
			if err := s.PublishCapture(p.stage, p.snap.Offer.CaptureDigest()); err != nil {
				return err
			}
			p.snap.Detach()
		}
		s.Config.Marketplaces = append(s.Config.Marketplaces, config.Marketplace{
			RegistrationID: id, Name: p.snap.Offer.Name, Kind: p.spec.Kind, Source: p.spec.Source, Branch: p.spec.Branch,
		})
		s.Catalogs.Upsert(marketplace.StoredFrom(id, p.snap))
		return nil
	})
	committed := err == nil || diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code
	res := AddResult{Name: p.snap.Offer.Name, Result: "registered", Revision: p.snap.Revision, Committed: committed}
	if err != nil {
		return res, classify(err)
	}
	return res, nil
}

// Add prepares and applies a registration in one step.
func Add(ctx context.Context, h workhome.Home, spec Spec) (AddResult, error) {
	plan, err := PrepareAdd(ctx, h, spec)
	if err != nil {
		return AddResult{}, err
	}
	defer plan.Close()
	return plan.Apply(ctx)
}
