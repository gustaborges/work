package cli

import (
	"context"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/workhome"
)

// loadPluginContext is pure and keeps inventory on one coherent snapshot.
func loadPluginContext(ctx context.Context) (workhome.Home, *adminstate.Snapshot, error) {
	return readAdministration(ctx)
}
