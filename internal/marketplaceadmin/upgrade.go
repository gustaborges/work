package marketplaceadmin

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

// Upgrade statuses.
const (
	StatusUpgraded  = "upgraded"
	StatusUnchanged = "unchanged"
	StatusFailed    = "failed"
)

// RowError is the credential-safe failure of one source.
type RowError struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

// UpgradeResult is the outcome of refreshing one source. Committed is true once
// the refreshed offer is published, even if cleanup afterwards reported an
// error; CleanupPending says that cleanup remains.
type UpgradeResult struct {
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Revision       *string   `json:"revision"`
	Error          *RowError `json:"error"`
	Committed      bool      `json:"committed"`
	CleanupPending bool      `json:"cleanup_pending,omitempty"`
}

func failure(name string, err error) UpgradeResult {
	err = classify(err)
	r := UpgradeResult{Name: name, Status: StatusFailed}
	var typed *diag.Error
	if errors.As(err, &typed) {
		r.Error = &RowError{Token: typed.Category.Token, Message: typed.Msg}
	} else {
		r.Error = &RowError{Token: "error", Message: "the marketplace could not be refreshed"}
	}
	return r
}

// Upgrade refreshes discovery for the named sources, or every known source
// when names is empty. Sources publish independently: a failure keeps earlier
// successes and later sources still run. Cancellation and recovery or cleanup
// failures stop the remaining work and outrank the aggregate. Installed
// plugins, enablement and policy are never touched.
//
// The returned error is nil only when every source succeeded or was unchanged;
// otherwise it is the highest-precedence failure and the rows retain
// per-source detail.
func Upgrade(ctx context.Context, h workhome.Home, names []string) ([]UpgradeResult, error) {
	state, err := adminstate.Read(ctx, h)
	if err != nil {
		return nil, err
	}
	targets, err := resolveTargets(state, names)
	if err != nil {
		return nil, err
	}
	rows := []UpgradeResult{}
	var fatal error
	failed := 0
	for _, t := range targets {
		row, err := refreshOne(ctx, h, t)
		rows = append(rows, row)
		if row.Status == StatusFailed {
			failed++
		}
		switch code := diag.ExitCode(err); {
		case err == nil:
		case code == diag.Cancelled.Code, code == diag.AdministrationRecoveryRequired.Code, code == diag.AdministrationCleanupFailed.Code:
			fatal = err
		}
		if fatal != nil {
			break
		}
	}
	switch {
	case fatal != nil:
		return rows, fatal
	case failed > 0:
		return rows, diag.Newf(diag.MarketplaceUpgradeFailed, "%d of %d marketplaces could not be refreshed", failed, len(rows))
	}
	return rows, nil
}

// target is one source to refresh; builtin has no registration.
type target struct {
	name    string
	builtin bool
	reg     config.Marketplace
}

func resolveTargets(s *adminstate.Snapshot, names []string) ([]target, error) {
	all := []target{{name: pluginid.Builtin, builtin: true}}
	for _, m := range s.Config.Marketplaces {
		all = append(all, target{name: m.Name, reg: m})
	}
	slices.SortFunc(all, func(a, b target) int { return strings.Compare(a.name, b.name) })
	if len(names) == 0 {
		return all, nil
	}
	var out []target
	for _, n := range names {
		i := slices.IndexFunc(all, func(t target) bool { return t.name == n })
		if i < 0 {
			return nil, diag.Newf(diag.Usage, "unknown marketplace %q", n)
		}
		if !slices.ContainsFunc(out, func(t target) bool { return t.name == n }) {
			out = append(out, all[i])
		}
	}
	return out, nil
}

// refreshOne prepares one source outside the lock and publishes it in its own
// transaction. The returned error is the raw classified error (for aggregate
// precedence); the row carries the safe projection.
func refreshOne(ctx context.Context, h workhome.Home, t target) (UpgradeResult, error) {
	if t.builtin {
		// The included offer is a pure function of the binary: there is nothing
		// stored to refresh, so it is always current.
		snap, err := marketplace.BuiltinSnapshot()
		if err != nil {
			e := classify(err)
			return failure(t.name, e), e
		}
		rev := snap.Revision
		return UpgradeResult{Name: t.name, Status: StatusUnchanged, Revision: &rev}, nil
	}
	if ctx.Err() != nil {
		e := classify(ctx.Err())
		return failure(t.name, e), e
	}
	var (
		snap  *marketplace.Snapshot
		lease *adminstate.Lease
		stage string
		err   error
	)
	switch t.reg.Kind {
	case pluginid.KindLocal:
		if lease, stage, err = adminstate.NewCaptureStage(ctx, h); err == nil {
			snap, err = marketplace.CaptureLocal(ctx, t.reg.Source, stage, marketplace.CaptureOptions{})
		}
	default:
		snap, err = marketplace.OpenGit(ctx, t.reg.Source, t.reg.Branch, "", os.TempDir())
	}
	defer func() {
		if snap != nil {
			snap.Close()
		}
		lease.Release()
	}()
	if err != nil {
		e := classify(err)
		return failure(t.name, e), e
	}
	if snap.Offer.Name != t.name {
		e := diag.Newf(diag.MarketplaceNameConflict, "the source now publishes %q instead of %q; remove and register it again to change its namespace", snap.Offer.Name, t.name)
		return failure(t.name, e), e
	}

	changed := false
	err = adminstate.WithWrite(ctx, h, opUpgrade+" "+t.name, func(s *adminstate.Session) error {
		cur, ok := findRegistration(s.Config, t.reg.RegistrationID)
		if !ok || cur != t.reg {
			return diag.New(diag.AdministrationConflict, "the marketplace registration changed during refresh; retry")
		}
		if old, ok := s.Catalogs.ByRegistration(t.reg.RegistrationID); ok && old.CatalogDigest == snap.Offer.CatalogDigest {
			// Semantically identical offer: keep the published revision and bytes.
			return nil
		}
		if snap.Source.Kind == pluginid.KindLocal {
			if err := s.PublishCapture(stage, snap.Offer.CaptureDigest()); err != nil {
				return err
			}
			snap.Detach()
		}
		s.Catalogs.Upsert(marketplace.StoredFrom(t.reg.RegistrationID, snap))
		changed = true
		return nil
	})
	rev := snap.Revision
	row := UpgradeResult{Name: t.name, Status: StatusUnchanged}
	if err == nil || diag.ExitCode(err) == diag.AdministrationCleanupFailed.Code {
		if changed {
			row.Status, row.Revision, row.Committed = StatusUpgraded, &rev, true
		} else if state, rerr := adminstate.Read(ctx, h); rerr == nil {
			if c, ok := state.Catalogs.ByRegistration(t.reg.RegistrationID); ok {
				r := c.Revision
				row.Revision = &r
			}
		}
		if err != nil {
			row.CleanupPending = true
			e := classify(err)
			row.Error = &RowError{Token: diag.Token(e), Message: "the refresh was committed but cleanup is pending"}
			return row, e
		}
		return row, nil
	}
	e := classify(err)
	return failure(t.name, e), e
}

func findRegistration(cfg *config.Config, id string) (config.Marketplace, bool) {
	for _, m := range cfg.Marketplaces {
		if m.RegistrationID == id {
			return m, true
		}
	}
	return config.Marketplace{}, false
}
