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
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/workhome"
)

type Control struct {
	Schema        int     `json:"schema"`
	Epoch         uint64  `json:"epoch"`
	Status        string  `json:"status"`
	TransactionID *string `json:"transaction_id"`
}
type Lifecycle struct {
	Schema       int  `json:"schema"`
	Initialized  bool `json:"initialized"`
	PolicySeeded bool `json:"policy_seeded"`
	Removed      bool `json:"removed"`
}
type Snapshot struct {
	Epoch      uint64
	Config     *config.Config
	Registry   *registry.Registry
	Memory     *repoconv.Store
	Lifecycle  *Lifecycle
	Inspection map[string]string
}

func recoveryError(err error) error {
	return diag.Wrap(diag.AdministrationRecoveryRequired, err, "administration needs recovery; rerun the interrupted mutating command with its required flags")
}
func cancelled(ctx context.Context) error {
	return diag.Wrap(diag.Cancelled, ctx.Err(), "administration cancelled")
}
func loadControl(h workhome.Home) (Control, error) {
	c := Control{Schema: 1, Status: "stable"}
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
	if c.Schema != 1 || (c.Status != "stable" && c.Status != "pending") || (c.Status == "stable" && c.TransactionID != nil) || (c.Status == "pending" && c.TransactionID == nil) {
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
	life, err := loadLifecycle(h, reg)
	if err != nil {
		return nil, err
	}
	// Backfill only from installed managed content. Never consult a linked
	// developer source for a version that was not registered at install time.
	for i := range reg.Packages {
		p := &reg.Packages[i]
		if plugin.ValidateAlias(p.Alias) != nil {
			return nil, recoveryError(errors.New("unsafe installed alias"))
		}
		if p.Version == "" && (p.Origin == registry.OriginLocalPinned || p.Origin == registry.OriginRemotePinned) {
			root := filepath.Join(h.PluginsDir(), p.Alias, "source")
			// Refuse linked installation roots and escaping manifest symlinks.
			manifest := filepath.Join(root, "plugin.json")
			resolved, e := filepath.EvalSymlinks(manifest)
			base, e2 := filepath.EvalSymlinks(root)
			aliasInfo, e3 := os.Lstat(filepath.Dir(root))
			if e == nil && e2 == nil && e3 == nil && aliasInfo.Mode()&os.ModeSymlink == 0 {
				rel, e := filepath.Rel(base, resolved)
				if e == nil && filepath.IsLocal(rel) {
					if b, e := os.ReadFile(resolved); e == nil {
						if m, e := plugin.Parse(b); e == nil {
							p.Version = m.Version
						}
					}
				}
			}
		}
	}
	inspection := map[string]string{}
	for _, p := range reg.Packages {
		status := "ready"
		if p.Version == "" || p.ContentDigest == "" {
			status = "metadata-unknown"
		}
		root := filepath.Join(h.PluginsDir(), p.Alias, "source")
		manifest := filepath.Join(root, "plugin.json")
		if p.Origin == registry.OriginEmbedded {
			manifest = filepath.Join(h.PluginsDir(), p.Alias, "plugin.json")
		}
		if info, e := os.Stat(manifest); e != nil || !info.Mode().IsRegular() {
			status = "content-unavailable"
		}
		for _, c := range reg.Components {
			if c.Alias == p.Alias {
				if info, e := os.Stat(c.EntrypointPath(h.PluginsDir())); e != nil || !info.Mode().IsRegular() {
					status = "content-unavailable"
				}
			}
		}
		inspection[p.Alias] = status
	}
	annotateRevisions(reg)
	return &Snapshot{Config: cfg, Registry: reg, Memory: mem, Lifecycle: life, Inspection: inspection}, nil
}
