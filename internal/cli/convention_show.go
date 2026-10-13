package cli

import (
	"fmt"

	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/spf13/cobra"
)

// newConventionShowCmd registers `work convention show [--json]`: read-only in
// every respect (FR-028) — it never creates, persists, or otherwise touches
// the memoized choice, including when none exists yet.
func newConventionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "show",
		Short:         "Show the remembered branch convention for this repository",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, snapshot, err := loadConventionContext(cmd.Context())
			if err != nil {
				return err
			}
			identity, err := resolveConventionIdentity()
			if err != nil {
				return err
			}
			view := snapshot.Memory.View(identity, snapshot.Registry)
			view.Identity = pluginadmin.SafeSource(identity)
			if view.Convention != nil {
				safe := pluginadmin.SafeText(*view.Convention)
				view.Convention = &safe
			}
			if cmd.Flags().Changed("json") {
				return printJSON(cmd, view)
			}

			out := cmd.OutOrStdout()
			if view.Convention == nil {
				fmt.Fprintln(out, "work: convention not set")
				return nil
			}
			if !view.Available {
				fmt.Fprintf(out, "work: convention %s (unavailable)\n", *view.Convention)
				fmt.Fprintln(out, "work: install or enable its provider, or use `work convention set <name>`")
				return nil
			}
			fmt.Fprintf(out, "work: convention %s\n", *view.Convention)
			return nil
		},
	}
}
