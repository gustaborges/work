package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/reconcile"
	"github.com/gustaborges/work/internal/resume"
	"github.com/gustaborges/work/internal/shellintegration"
	"github.com/gustaborges/work/internal/tui"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/internal/worklist"
)

// newResumeCmd registers `work resume [WORK]`.
func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "resume [WORK]",
		Short:         "Resume an existing Work, most recently accessed first",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var target string
			if len(args) == 1 {
				target = args[0]
			}
			return runResume(cmd, target, cmd.Flags().Changed("json"))
		},
	}
}

func runResume(cmd *cobra.Command, target string, jsonSet bool) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	interactive := present.IsInteractive()
	target = strings.TrimSpace(target)

	// --json is a read-only flag; resume is a mutation (ADR-0017).
	if jsonSet {
		return diag.New(diag.Usage, "--json is not accepted on `work resume` (it is a mutation)")
	}

	home, err := workhome.Resolve()
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	if err := gitx.Preflight(); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "git is required but not usable")
	}

	cfg, err := config.Load(home.ConfigFile())
	if err != nil {
		return err
	}
	workspaceRoot := strings.TrimSpace(cfg.Workspace)

	// With no workspace configured there are no Works: settle the three flows
	// without opening (or rebuilding) a projection database.
	if workspaceRoot == "" {
		return resumeNoWorks(errOut, target, interactive)
	}

	db, report, err := reconcile.Open(workspaceRoot, home.DBFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot open the lookup index")
	}
	defer db.Close()
	printSkipped(errOut, report)

	// Resolve the Work to resume: an explicit id skips the list entirely.
	var row worklist.WorkRow
	if target != "" {
		rows, err := worklist.List(db, true)
		if err != nil {
			return diag.Wrap(diag.BootstrapFailed, err, "cannot read the lookup index")
		}
		r, outcome := worklist.Resolve(rows, target)
		switch outcome {
		case worklist.NotFound:
			return diag.New(diag.TargetNotFound, "no Work has that id")
		case worklist.Archived:
			return diag.New(diag.TargetArchived, "that Work is archived and cannot be resumed")
		}
		row = r
	} else {
		rows, err := worklist.List(db, false)
		if err != nil {
			return diag.Wrap(diag.BootstrapFailed, err, "cannot read the lookup index")
		}
		if len(rows) == 0 {
			fmt.Fprintln(errOut, "note: no Works to resume")
			return nil
		}
		if !interactive {
			return diag.New(diag.Usage,
				"pass a Work id to resume; run `work resume` in a terminal to pick from the list")
		}
		id, err := tui.SelectResume(ctx, rows)
		if err != nil {
			return err
		}
		row = rowByID(rows, id)
	}

	res, err := resume.Run(ctx, resume.Params{
		Home:         home,
		DB:           db,
		ID:           row.ID,
		SnapshotPath: row.SnapshotPath,
		WorktreePath: row.WorktreePath,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "work: resumed %s\n", row.ID)
	fmt.Fprintf(out, "work: path %s\n", res.WorktreePath)
	if res.IndexStale {
		fmt.Fprintln(errOut, "note: the lookup index could not be updated; it will self-heal on the next `work` command")
	}

	if shellintegration.Active() {
		if err := shellintegration.WriteTargetPath(res.WorktreePath); err != nil {
			return err
		}
	} else {
		shellintegration.ReportNoIntegration(errOut, res.WorktreePath, shellintegration.DetectShell())
	}
	return nil
}

// resumeNoWorks handles the three flows when no workspace root is configured and
// therefore no Work can exist.
func resumeNoWorks(errOut io.Writer, target string, interactive bool) error {
	if target != "" {
		return diag.New(diag.TargetNotFound, "no Work has that id")
	}
	if !interactive {
		return diag.New(diag.Usage,
			"pass a Work id to resume; run `work resume` in a terminal to pick from the list")
	}
	fmt.Fprintln(errOut, "note: no Works to resume")
	return nil
}

// printSkipped surfaces each unreadable snapshot the index rebuild/reconcile
// skipped, so the user learns a Work is missing from the list without the
// command failing (FR-024).
func printSkipped(errOut io.Writer, r reconcile.Report) {
	for _, s := range r.Skipped {
		fmt.Fprintf(errOut, "note: %s: %s: %s\n", diag.SnapshotUnreadable.Token, s.Path, s.Reason)
	}
}

func rowByID(rows []worklist.WorkRow, id string) worklist.WorkRow {
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	return worklist.WorkRow{}
}
