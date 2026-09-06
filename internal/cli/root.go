// Package cli assembles the `work` command tree and owns process exit. Command
// implementations return errors (diag.Error where the outcome is
// user-meaningful); Execute maps them to the stable exit codes.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
)

// newRootCmd builds the top-level `work` command with its subcommands
// registered.
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

	root.AddCommand(newStartCmd())
	root.AddCommand(newShellInitCmd())

	return root
}

// Execute runs the root command and terminates the process with the exit code
// for whatever error it returns.
func Execute() {
	err := newRootCmd().Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, diag.Format(err))
	}
	os.Exit(diag.ExitCode(err))
}
