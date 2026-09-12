package cli

import (
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// loadRepositoryContext resolves the Work home, ensures the seed is bootstrapped
// (so a fresh install's default policy and registry exist — quickstart S1),
// and loads the current config and registry. Every `work repository`
// subcommand shares this precondition, exactly like `work start`.
func loadRepositoryContext() (workhome.Home, *config.Config, *registry.Registry, error) {
	home, err := workhome.Resolve()
	if err != nil {
		return workhome.Home{}, nil, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	if err := bootstrap.EnsureSeed(home); err != nil {
		return workhome.Home{}, nil, nil, err
	}
	cfg, err := config.Load(home.ConfigFile())
	if err != nil {
		return workhome.Home{}, nil, nil, err
	}
	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		return workhome.Home{}, nil, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}
	return home, cfg, reg, nil
}

// saveRepositoryConfig persists cfg atomically after a successful mutation.
func saveRepositoryConfig(home workhome.Home, cfg *config.Config) error {
	return config.Save(home.ConfigFile(), cfg)
}
