// Package cli assembles the `work` command tree and owns process exit. Command
// implementations return errors (diag.Error where the outcome is
// user-meaningful); Execute maps them to the stable exit codes.
package cli

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/present/brand"
	"github.com/gustaborges/work/internal/present/theme"
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
			return runBareWork(cmd)
		},
	}

	// Only read commands honor --json; mutating commands reject it. It stays
	// hidden until F1's first read command needs it, so it never appears in the
	// help for `work start` (a mutation) or the bare `work` brand.
	root.PersistentFlags().Bool("json", false, "emit machine-readable output (read commands only)")
	_ = root.PersistentFlags().MarkHidden("json")

	start := newStartCmd()
	resume := newResumeCmd()
	archive := newArchiveCmd()
	shellInit := newShellInitCmd()
	repository := newRepositoryCmd()

	start.GroupID = groupDaily
	resume.GroupID = groupDaily
	archive.GroupID = groupDaily
	shellInit.GroupID = groupSetup
	repository.GroupID = groupAdmin

	root.AddCommand(start, resume, archive, shellInit, repository)

	installHelp(root)

	return root
}

// runBareWork handles `work` with no subcommand. In an interactive terminal it
// prints the static WORK brand to stdout and exits 0 — no selector, no Bubble
// Tea program (contracts/cli-work-home.md, ADR-0019). Non-interactively it keeps
// the F1/F2 behaviour exactly: a one-line usage summary on stderr, exit 2.
func runBareWork(cmd *cobra.Command) error {
	if !present.IsInteractive() {
		return diag.New(diag.Usage,
			"run `work start <path>` to create a Work, `work resume` to return to one, or `work archive` to close one; see `work --help` for all commands")
	}

	out := cmd.OutOrStdout()
	probe := theme.Detect(out, cmd.InOrStdin())
	cmd.Print(brand.Render(terminalWidth(), probe.Profile, probe.ColorEnabled))
	return nil
}

// terminalWidth is the current stdout column count, or 80 when it cannot be
// determined (the brand renderer's documented fallback).
func terminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

// Execute runs the root command and terminates the process with the exit code
// for whatever error it returns. An interrupt (Ctrl-C) cancels the command's
// context so an in-flight `work start` unwinds its partial state and exits with
// the "cancelled" code rather than leaving orphans (FR-021, S8). Any failure is
// rendered exactly once, here, by renderDiagnostic (contracts/diagnostics.md).
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := newRootCmd()
	err := root.ExecuteContext(ctx)
	os.Exit(renderDiagnostic(root.ErrOrStderr(), root.InOrStdin(), present.IsInteractive(), err))
}
