package cli

import (
	"context"
	"os"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workcontext"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

// contextIO uses the caller's streams; domain operations never own rendering.
func contextIO(cmd *cobra.Command) present.IO {
	return present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}
}

// loadContextGlobals reads only; unlike setup commands it never bootstraps.
func loadContextGlobals() (workhome.Home, *config.Config, *registry.Registry, error) {
	home, err := workhome.Resolve()
	if err != nil {
		return workhome.Home{}, nil, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	cfg, err := config.Load(home.ConfigFile())
	if err != nil {
		return home, nil, nil, err
	}
	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		return home, nil, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}
	return home, cfg, reg, nil
}

// resolveContext deliberately bypasses all global reads for current-directory
// status, so it also works in old workspaces and with no Work home at all.
func resolveContext(ctx context.Context, target string) (workcontext.Context, error) {
	if err := ctx.Err(); err != nil {
		return workcontext.Context{}, diag.Wrap(diag.Cancelled, err, "status cancelled")
	}
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return workcontext.Context{}, diag.Wrap(diag.SnapshotUnreadable, err, "cannot determine the current Work directory")
		}
		return workcontext.Current(cwd)
	}
	home, err := workhome.Resolve()
	if err != nil {
		return workcontext.Context{}, diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	cfg, err := config.Load(home.ConfigFile())
	if err != nil {
		return workcontext.Context{}, err
	}
	return workcontext.ByID(ctx, cfg.Workspace, target, func(ctx context.Context, id string) string { return projection.LookupPathHint(ctx, home.DBFile(), id) })
}

// requireActiveContext captures the real directory and identity once. Later
// mutations must reread and compare this expected context under their lock.
func requireActiveContext(ctx context.Context) (workcontext.Context, error) {
	c, err := resolveContext(ctx, "")
	if err != nil {
		return c, err
	}
	if c.State.Work.Status != work.StatusInProgress {
		return workcontext.Context{}, diag.New(diag.TargetArchived, "that Work is archived; enrich an active Work instead")
	}
	return c, nil
}

// reloadContextProvider revalidates the selected identity against the latest
// retained registry state. It never substitutes another installed provider.
func reloadContextProvider(home workhome.Home, selected registry.Component) (registry.Component, error) {
	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		return registry.Component{}, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}
	for _, c := range reg.Effective().Components {
		if c.Alias == selected.Alias && c.Name == selected.Name && c.Role == selected.Role && c.Manual != nil && c.Key == selected.Key {
			return c, nil
		}
	}
	return registry.Component{}, diag.New(diag.ManualComponentUnavailable, "the selected manual component is no longer available")
}
