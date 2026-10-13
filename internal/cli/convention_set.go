package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/workhome"
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

			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			identity, err := resolveConventionIdentity()
			if err != nil {
				return err
			}
			if err = setConvention(cmd.Context(), home, identity, name); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "work: convention set to %s\n", name)
			return nil
		},
	}
}
