package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/archive"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/reconcile"
	"github.com/gustaborges/work/internal/shellintegration"
	"github.com/gustaborges/work/internal/tui"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/internal/worklist"
)

// newArchiveCmd registers `work archive [WORK...]`.
func newArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "archive [WORK...]",
		Short:         "Archive one or more Works, preserving their context",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			forceDirty, _ := cmd.Flags().GetBool("force-dirty")
			return runArchive(cmd, args, archiveFlags{
				yes:        yes,
				forceDirty: forceDirty,
				jsonSet:    cmd.Flags().Changed("json"),
			})
		},
	}
	cmd.Flags().Bool("yes", false, "confirm the archival non-interactively")
	cmd.Flags().Bool("force-dirty", false, "archive even a Work whose worktree has uncommitted or untracked changes")
	return cmd
}

type archiveFlags struct {
	yes        bool
	forceDirty bool
	jsonSet    bool
}

func runArchive(cmd *cobra.Command, args []string, f archiveFlags) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	interactive := tui.IsInteractive()

	if f.jsonSet {
		return diag.New(diag.Usage, "--json is not accepted on `work archive` (it is a mutation)")
	}

	// Non-interactive runs need explicit targets and --yes up front, before any
	// bootstrap or index work (FR-011).
	if !interactive {
		if len(args) == 0 {
			return diag.New(diag.Usage,
				"pass one or more Work ids to archive; run `work archive` in a terminal to pick from the list")
		}
		if !f.yes {
			return diag.New(diag.Usage, "missing --yes: confirm the archival non-interactively with --yes")
		}
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
	if workspaceRoot == "" {
		// No workspace configured means no Work can exist.
		for _, id := range args {
			fmt.Fprintf(errOut, "note: %s: not found\n", id)
		}
		if len(args) == 0 {
			fmt.Fprintln(errOut, "note: no active Works to archive")
		}
		return nil
	}

	db, report, err := reconcile.Open(workspaceRoot, home.DBFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot open the lookup index")
	}
	defer db.Close()
	printSkipped(errOut, report)

	// Resolve the selection: explicit ids skip the picker; no ids opens it.
	var rows []worklist.WorkRow
	explicit := len(args) > 0
	if explicit {
		all, err := worklist.List(db, true)
		if err != nil {
			return diag.Wrap(diag.BootstrapFailed, err, "cannot read the lookup index")
		}
		var unknown, archivedIDs int
		seen := map[string]bool{}
		for _, id := range args {
			if seen[id] {
				continue
			}
			seen[id] = true
			r, outcome := worklist.Resolve(all, id)
			switch outcome {
			case worklist.NotFound:
				fmt.Fprintf(errOut, "note: %s: not found\n", id)
				unknown++
			case worklist.Archived:
				fmt.Fprintf(errOut, "note: %s: already archived\n", id)
				archivedIDs++
			default:
				rows = append(rows, r)
			}
		}
		if len(rows) == 0 {
			// A single explicit target that is already archived is the whole
			// job and is fatal (contract exit codes); everything else settles
			// at exit 0 with the notes already printed.
			if len(args) == 1 && archivedIDs == 1 {
				return diag.New(diag.TargetArchived, "that Work is already archived")
			}
			fmt.Fprintln(out, "work: archived 0 of 0")
			return nil
		}
	} else {
		active, err := worklist.List(db, false)
		if err != nil {
			return diag.Wrap(diag.BootstrapFailed, err, "cannot read the lookup index")
		}
		if len(active) == 0 {
			fmt.Fprintln(errOut, "note: no active Works to archive")
			return nil
		}
		ids, err := tui.SelectArchive(ctx, active, workspaceRoot)
		if err != nil {
			return err
		}
		rows = rowsByIDs(active, ids)
	}

	// Confirmation (FR-010): the picker already carries it for the interactive
	// no-target path; every other path needs it here.
	if explicit {
		if interactive {
			ok, err := tui.ConfirmArchive(rows, workspaceRoot)
			if err != nil {
				return err
			}
			if !ok {
				return diag.New(diag.Cancelled, "archival declined at the confirmation prompt")
			}
		} else if !f.yes {
			return diag.New(diag.Usage, "missing --yes: confirm the archival non-interactively with --yes")
		}
	}

	// Non-interactive single-target dirty guard is fatal (exit 23) only when
	// that one target is the whole job; in a batch it is a per-Work note.
	if !interactive && !f.forceDirty && len(rows) == 1 && len(args) == 1 {
		if wt := rows[0].WorktreePath; wt != "" {
			if dirty, derr := gitx.Open(wt).IsDirty(); derr == nil && dirty {
				return diag.New(diag.DirtyWorktree,
					"that Work's worktree has uncommitted or untracked changes; pass --force-dirty to archive it anyway")
			}
		}
	}

	cwd, _ := os.Getwd()
	var ackDirty func(projection.Work) (bool, error)
	if interactive && !f.forceDirty {
		ackDirty = func(w projection.Work) (bool, error) {
			return tui.AckDirtyWork(rowFor(rows, w.ID))
		}
	}

	rep, runErr := archive.Run(ctx, archive.Params{
		Home:          home,
		DB:            db,
		WorkspaceRoot: workspaceRoot,
		Rows:          projectionRows(db, rows),
		ForceDirty:    f.forceDirty,
		AckDirty:      ackDirty,
		CallerCWD:     cwd,
	})

	repositionOut := false
	for _, o := range rep.Outcomes {
		switch o.State {
		case archive.StateArchived:
			fmt.Fprintf(out, "work: archived %s  (%s)\n", o.ID, o.ArchivedDir)
			if o.Note != "" {
				fmt.Fprintf(errOut, "note: %s: %s\n", o.ID, o.Note)
			}
		case archive.StateLeftActive:
			msg := o.Note
			if msg == "" {
				msg = "worktree has uncommitted or untracked changes — left active (use --force-dirty)"
			}
			fmt.Fprintf(errOut, "note: %s: %s\n", o.ID, msg)
		case archive.StateFailed:
			fmt.Fprintf(errOut, "note: %s: archive step %q failed after the snapshot was committed; the index will self-heal\n", o.ID, o.FailedStep)
		}
		if o.WasCWD && o.State == archive.StateArchived {
			repositionOut = true
		}
	}
	fmt.Fprintf(out, "work: archived %d of %d\n", rep.Archived(), len(rep.Outcomes))

	if repositionOut {
		root := workspaceRoot
		if shellintegration.Active() {
			if err := shellintegration.WriteTargetPath(root); err != nil {
				return err
			}
		} else {
			fmt.Fprintln(errOut, "note: this shell session is now inside a directory that was archived away.")
			fmt.Fprintln(errOut, "note: cd to the workspace root:")
			fmt.Fprintln(errOut, root)
		}
	}

	return runErr
}

// projectionRows fetches the current projection row for each selected Work so
// the orchestrator has every column it needs; a row that vanished between
// listing and now is skipped.
func projectionRows(db *projection.DB, rows []worklist.WorkRow) []projection.Work {
	out := make([]projection.Work, 0, len(rows))
	for _, r := range rows {
		w, ok, err := db.Get(r.ID)
		if err != nil || !ok {
			continue
		}
		out = append(out, w)
	}
	return out
}

func rowsByIDs(rows []worklist.WorkRow, ids []string) []worklist.WorkRow {
	out := make([]worklist.WorkRow, 0, len(ids))
	for _, id := range ids {
		for _, r := range rows {
			if r.ID == id {
				out = append(out, r)
			}
		}
	}
	return out
}

func rowFor(rows []worklist.WorkRow, id string) worklist.WorkRow {
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	return worklist.WorkRow{ID: id}
}
