package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/repoconv"
)

// newConventionSetCmd registers `work convention set <CONVENTION>`: replaces
// the memoized choice for this repository's identity, independent of which
// clone the command runs from (FR-027).
func newConventionSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <CONVENTION>",
		Short: "Set the remembered branch convention for this repository",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return diag.New(diag.Usage, "missing CONVENTION argument: run `work convention set <convention>`")
			}
			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on `work convention set` (it is a mutation)")
			}
			name := strings.TrimSpace(args[0])

			home, reg, err := loadConventionContext()
			if err != nil {
				return err
			}
			if _, ok := reg.ConventionByName(name); !ok {
				return diag.Newf(diag.ConventionUnknown, "%q is not a currently enabled branch convention", name).
					WithHint("run `work convention show` to see what is remembered, or `work plugin list` for the enabled catalog")
			}
			identity, err := resolveConventionIdentity()
			if err != nil {
				return err
			}
			store, err := repoconv.Load(home.BranchConventionsFile())
			if err != nil {
				return diag.Wrap(diag.BootstrapFailed, err, "cannot read the branch convention memory")
			}
			store.Set(identity, name)
			if err := repoconv.Save(home.BranchConventionsFile(), store); err != nil {
				return diag.Wrap(diag.BootstrapFailed, err, "cannot persist the branch convention memory")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "work: convention set to %s\n", name)
			return nil
		},
	}
}
