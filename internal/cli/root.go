// Package cli assembles the `work` command tree and owns process exit.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newRootCmd builds the top-level `work` command.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "work",
		Short:         "Isolated, reproducible units of work backed by git worktrees",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Only read commands honor --json; mutating commands reject it.
	root.PersistentFlags().Bool("json", false, "emit machine-readable output (read commands only)")

	return root
}

// Execute runs the root command and terminates the process with the
// appropriate exit code.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "work:", err)
		os.Exit(1)
	}
}
