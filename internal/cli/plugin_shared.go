package cli

import (
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// loadPluginContext resolves the Work home and loads the current registry.
// Unlike `work repository`/`work start`, `work plugin` subcommands do not
// call bootstrap.EnsureSeed themselves — install/list must work before the
// embedded reference package has ever been bootstrapped (a plugin is a
// first-class origin in its own right, not conditioned on the seed).
func loadPluginContext() (workhome.Home, *registry.Registry, error) {
	home, err := workhome.Resolve()
	if err != nil {
		return workhome.Home{}, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	if err := home.EnsureLayout(); err != nil {
		return workhome.Home{}, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot create the Work home layout")
	}
	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		return workhome.Home{}, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}
	return home, reg, nil
}
