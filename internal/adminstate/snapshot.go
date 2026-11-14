// Package adminstate owns the finite administrative epoch and journal protocol.
// Reads never create locks, layouts, state, or a Work projection database.
package adminstate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/workhome"
)

// ControlSchema is the only supported administration control/journal schema.
const ControlSchema = 1

// Control is the stable/pending epoch record.
type Control struct {
	Schema        int     `json:"schema"`
	Epoch         uint64  `json:"epoch"`
	Status        string  `json:"status"`
	TransactionID *string `json:"transaction_id"`
}

// Snapshot is one coherent administrative state. Inspection maps a provider's
// public ID to its installed-content status.
type Snapshot struct {
	Epoch      uint64
	Config     *config.Config
	Registry   *registry.Registry
	Memory     *repoconv.Store
	Lifecycle  *Lifecycle
	Catalogs   *marketplace.State
	Inspection map[string]string
}

func recoveryError(err error) error {
	return diag.Wrap(diag.AdministrationRecoveryRequired, err, "administration needs recovery; rerun the interrupted mutating command with its required flags")
}
func cancelled(ctx context.Context) error {
	return diag.Wrap(diag.Cancelled, ctx.Err(), "administration cancelled")
}
func loadControl(h workhome.Home) (Control, error) {
	c := Control{Schema: ControlSchema, Status: "stable"}
	b, err := os.ReadFile(h.AdministrationFile())
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, recoveryError(err)
	}
	c = Control{}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, recoveryError(err)
	}
	if c.Schema != ControlSchema || (c.Status != "stable" && c.Status != "pending") || (c.Status == "stable" && c.TransactionID != nil) || (c.Status == "pending" && c.TransactionID == nil) {
		return c, recoveryError(errors.New("invalid administration control"))
	}
	if c.TransactionID != nil {
		if _, err = h.TransactionDir(*c.TransactionID); err != nil {
			return c, recoveryError(err)
		}
	}
	return c, nil
}

// Read accepts equal stable epochs only, even when an intermediate load failed.
func Read(ctx context.Context, h workhome.Home) (*Snapshot, error) { return read(ctx, h, nil) }
func read(ctx context.Context, h workhome.Home, between func()) (*Snapshot, error) {
	for range 3 {
		if ctx.Err() != nil {
			return nil, cancelled(ctx)
		}
		a, err := loadControl(h)
		if err != nil {
			return nil, err
		}
		if a.Status != "stable" {
			return nil, pendingReadError(h, a)
		}
		s, loadErr := load(h)
		if between != nil {
			between()
		}
		b, err := loadControl(h)
		if err != nil {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, cancelled(ctx)
		}
		if b.Status != "stable" {
			return nil, pendingReadError(h, b)
		}
		if !reflect.DeepEqual(a, b) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		s.Epoch = b.Epoch
		return s, nil
	}
	return nil, diag.New(diag.AdministrationConflict, "administration changed during inspection; retry the command")
}

func load(h workhome.Home) (*Snapshot, error) {
	cfg, err := config.Load(h.ConfigFile())
	if err != nil {
		return nil, err
	}
	reg, err := registry.Load(h.RegistryFile())
	if err != nil {
		return nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}
	mem, err := repoconv.Load(h.BranchConventionsFile())
	if err != nil {
		return nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read branch convention memory")
	}
	cats, err := marketplace.LoadState(h.CatalogsFile())
	if err != nil {
		return nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the marketplace catalogs")
	}
	life, err := loadLifecycle(h)
	if err != nil {
		return nil, err
	}
	inspection := map[string]string{}
	for _, p := range reg.Packages {
		status := "ready"
		src, err := h.PackageSource(p.Provider)
		if err != nil {
			return nil, recoveryError(err)
		}
		if info, e := os.Stat(filepath.Join(src, "plugin.json")); e != nil || !info.Mode().IsRegular() {
			status = "content-unavailable"
		}
		for _, c := range reg.ProviderComponents(p.Provider) {
			if info, e := os.Stat(c.EntrypointPath(h.PluginsDir())); e != nil || !info.Mode().IsRegular() {
				status = "content-unavailable"
			}
		}
		inspection[p.ID()] = status
	}
	annotateRevisions(reg)
	return &Snapshot{Config: cfg, Registry: reg, Memory: mem, Lifecycle: life, Catalogs: cats, Inspection: inspection}, nil
}
