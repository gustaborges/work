package cli

import (
	"context"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

// loadPluginContext is pure and keeps inventory on one coherent snapshot.
func loadPluginContext(ctx context.Context) (workhome.Home, *adminstate.Snapshot, error) {
	return readAdministration(ctx)
}

// parseProviders validates every operand as plugin@marketplace before any
// state is read: there is no global alias and no implicit lookup.
func parseProviders(args []string) ([]pluginid.Provider, error) {
	if len(args) == 0 {
		return nil, diag.New(diag.Usage, "supply at least one installed plugin as plugin@marketplace")
	}
	out := make([]pluginid.Provider, 0, len(args))
	for _, a := range args {
		p, err := pluginid.ParseProvider(a)
		if err != nil {
			return nil, diag.New(diag.Usage, "supply exact installed plugins as plugin@marketplace")
		}
		out = append(out, p)
	}
	return out, nil
}

// valueOrUnknown renders a nullable inventory string.
func valueOrUnknown(v *string) string {
	if v == nil {
		return "unknown"
	}
	return *v
}
