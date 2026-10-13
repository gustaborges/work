package cli

import (
	"context"
	"fmt"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

func readAdministration(ctx context.Context) (workhome.Home, *adminstate.Snapshot, error) {
	h, err := workhome.Resolve()
	if err != nil {
		return h, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	s, err := adminstate.Read(ctx, h)
	return h, s, err
}
func mutationPreflight(cmd *cobra.Command, h workhome.Home) error {
	ctx := adminstate.WithRecoveryObserver(cmd.Context(), func(string) { fmt.Fprintln(cmd.ErrOrStderr(), "work: recovered interrupted administrative operation") })
	cmd.SetContext(ctx)
	_, err := adminstate.Preflight(ctx, h)
	return err
}

// setConvention merges only this repository identity against current memory.
func setConvention(ctx context.Context, h workhome.Home, identity, name string) error {
	return adminstate.WithWrite(ctx, h, "work convention set", func(s *adminstate.Session) error {
		if _, ok := s.Registry.Effective().ConventionByName(name); !ok {
			return diag.New(diag.ConventionUnknown, "the selected branch convention is unavailable; install or enable its provider")
		}
		s.Memory.Set(identity, name)
		return nil
	})
}
