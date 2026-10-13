package cli

import (
	"github.com/spf13/cobra"
)

// Bare administration commands open pure, discoverable interactive hubs.
func newRepositoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "repository",
		Short:         "Inspect and manage repository search roots and the resolution policy",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hubGuard(cmd, "run `work repository locator list`, `work repository policy list`, `work repository root list` or `work repository --help`"); err != nil {
				return err
			}
			return runRepositoryHub(cmd)
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
