package cli

import (
	"context"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

func loadRepositoryContext() (workhome.Home, *config.Config, *registry.Registry, error) {
	h, s, err := readAdministration(context.Background())
	if err != nil {
		return h, nil, nil, err
	}
	return h, s.Config, s.Registry, nil
}
