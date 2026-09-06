// Package cli assembles the `work` command tree and owns process exit. Command
// implementations return errors (diag.Error where the outcome is
// user-meaningful); Execute maps them to the stable exit codes.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/tui"
)

// newRootCmd builds the top-level `work` command with its subcommands
// registered.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "work",
		Short:         "Isolated, reproducible units of work backed by git worktrees",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHome(cmd)
		},
	}

	// Only read commands honor --json; mutating commands reject it. It stays
	// hidden until F1's first read command needs it, so it never appears in the
	// help for `work start` (a mutation) or the bare `work` home.
	root.PersistentFlags().Bool("json", false, "emit machine-readable output (read commands only)")
	_ = root.PersistentFlags().MarkHidden("json")

	root.AddCommand(newStartCmd())
	root.AddCommand(newResumeCmd())
	root.AddCommand(newShellInitCmd())

	return root
}

// runHome handles `work` with no subcommand: in an interactive terminal it
// opens the TUI home and dispatches the chosen journey; otherwise it prints a
// one-line command summary and exits 2 without rendering any TUI (RF-51,
// contracts/cli-work-home.md).
func runHome(cmd *cobra.Command) error {
	if !tui.IsInteractive() {
		return diag.New(diag.Usage,
			"run `work start <path>` to create a Work or `work resume` to return to one; see `work --help` for all commands")
	}
	choice, err := tui.RunHome(cmd.Context())
	if err != nil {
		return err
	}
	switch choice {
	case tui.HomeStartWork:
		return runStart(cmd, "", startFlags{})
	case tui.HomeResumeWork:
		return runResume(cmd, "", false)
	default:
		// Left the home without choosing anything.
		return nil
	}
}

// Execute runs the root command and terminates the process with the exit code
// for whatever error it returns. An interrupt (Ctrl-C) cancels the command's
// context so an in-flight `work start` unwinds its partial state and exits with
// the "cancelled" code rather than leaving orphans (FR-021, S8).
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := newRootCmd().ExecuteContext(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, diag.Format(err))
	}
	os.Exit(diag.ExitCode(err))
}
