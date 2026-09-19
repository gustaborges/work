package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/repoconv"
)

// conventionShowResult is the `work convention show --json` shape
// (contracts/cli-work-convention.md).
type conventionShowResult struct {
	Identity   string  `json:"identity"`
	Convention *string `json:"convention"`
}

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
			home, _, err := loadConventionContext()
			if err != nil {
				return err
			}
			identity, err := resolveConventionIdentity()
			if err != nil {
				return err
			}
			store, err := repoconv.Load(home.BranchConventionsFile())
			if err != nil {
				return diag.Wrap(diag.BootstrapFailed, err, "cannot read the branch convention memory")
			}
			name, ok := store.Get(identity)

			if cmd.Flags().Changed("json") {
				var namePtr *string
				if ok {
					namePtr = &name
				}
				return printJSON(cmd, conventionShowResult{Identity: identity, Convention: namePtr})
			}

			out := cmd.OutOrStdout()
			if !ok {
				fmt.Fprintln(out, "work: convention not set")
				return nil
			}
			fmt.Fprintf(out, "work: convention %s\n", name)
			return nil
		},
	}
}
