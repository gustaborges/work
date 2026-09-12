package cli

import (
	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
)

// newRepositoryCmd registers the `work repository` parent: the non-
// interactive surface for inspecting and editing the Repository Resolution
// Policy and the repository search roots (ADR-0015, ADR-0019, ADD §7.2).
// With no subcommand it prints grouped help for locator/policy/root and exits
// 0 in every stream configuration — an ordinary Cobra parent, not a menu; the
// interactive `repository` hub of ADR-0019 is deferred past F3
// (contracts/cli-work-repository.md §Scope note).
func newRepositoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "repository",
		Short:         "Inspect and manage repository search roots and the resolution policy",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// The only command in this family whose no-arg form exits 0
			// rather than 2 (it is informational, not a missing mandatory
			// value) — but --json is still a mutation-shaped rejection.
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on `work repository` (it prints help)")
			}
			return cmd.Help()
		},
	}
	// locator/policy/root are children of this command, not of the root `work`
	// command, so they carry no GroupID: Cobra only validates/renders groups
	// declared on a command's direct parent, and only `work`'s own help uses
	// the custom grouped renderer (installHelp) — this subtree's own --help
	// prints Cobra's plain "Available Commands" listing, which already
	// satisfies "grouped help for locator/policy/root" (contracts/
	// cli-work-repository.md).
	cmd.AddCommand(newRepositoryLocatorCmd(), newRepositoryPolicyCmd(), newRepositoryRootCmd())
	return cmd
}
