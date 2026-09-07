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
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/reconcile"
	"github.com/gustaborges/work/internal/shellintegration"
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
	interactive := present.IsInteractive()
	pio := present.IO{In: cmd.InOrStdin(), UI: errOut}

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
		if !interactive {
			return diag.New(diag.Usage, "pass one or more Work ids to archive; run `work archive` in a terminal to pick from the list")
		}
	}

	var declinedDirty []worklist.WorkRow
	requestedCount := len(rows)
	if interactive {
		initial := present.Answers{}
		var steps []present.Step
		if explicit {
			initial["rows"] = rows
		} else {
			active, err := worklist.List(db, false)
			if err != nil {
				return diag.Wrap(diag.BootstrapFailed, err, "cannot read the lookup index")
			}
			opts := make([]present.Option[worklist.WorkRow], len(active))
			for i, r := range active {
				opts[i] = present.Option[worklist.WorkRow]{Value: r, Primary: r.DisplayName, Secondary: r.RelativeTime + " • " + r.Branch}
			}
			steps = append(steps, present.MultiSelectStep("rows", func(present.Answers) (present.MultiSelectSpec[worklist.WorkRow], error) {
				return present.MultiSelectSpec[worklist.WorkRow]{Title: "Archive Works", Filterable: true, Options: opts}, nil
			}))
		}
		var dirtyRows []worklist.WorkRow
		steps = append(steps,
			present.ConfirmStep("confirm", func(a present.Answers) (present.ConfirmSpec, error) {
				selected, _ := a.Value("rows").([]worklist.WorkRow)
				return present.ConfirmSpec{Title: "Archive Works", Impact: archiveConfirmImpact(selected, workspaceRoot), Accept: "Archive", Reject: "Cancel"}, nil
			}),
			present.ConfirmSequenceStep("dirty", func(a present.Answers) ([]present.ConfirmSpec, error) {
				if !a.Bool("confirm") || f.forceDirty {
					return nil, nil
				}
				selected, _ := a.Value("rows").([]worklist.WorkRow)
				dirtyRows = dirtyArchiveRows(selected)
				specs := make([]present.ConfirmSpec, 0, len(dirtyRows))
				for _, r := range dirtyRows {
					specs = append(specs, present.ConfirmSpec{Title: r.DisplayName + "  (" + r.Branch + ")", Impact: "This worktree has uncommitted or untracked changes.\nArchiving it anyway will lose those changes.", Accept: "Archive anyway", Reject: "Keep active"})
				}
				return specs, nil
			}),
		)
		ans, err := present.Wizard(ctx, pio, present.WizardSpec{Title: "Archive Works", Initial: initial, Steps: steps})
		if err != nil {
			return err
		}
		if !ans.Bool("confirm") {
			return diag.New(diag.Cancelled, "archival declined at the confirmation prompt")
		}
		rows, _ = ans.Value("rows").([]worklist.WorkRow)
		requestedCount = len(rows)
		dirtyAnswers, _ := ans.Value("dirty").([]bool)
		declinedDirty = declinedDirtyRows(dirtyRows, dirtyAnswers)
		rows = filterDirtyDeclines(rows, dirtyRows, dirtyAnswers)
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

	rep, runErr := archive.Run(ctx, archive.Params{
		Home:          home,
		DB:            db,
		WorkspaceRoot: workspaceRoot,
		Rows:          projectionRows(db, rows),
		ForceDirty:    f.forceDirty,
		// A Work that becomes dirty after the preflight is left active. Asking a
		// fresh question here would open another full-screen session.
		AckDirty:  nil,
		CallerCWD: cwd,
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
	for _, r := range declinedDirty {
		fmt.Fprintf(errOut, "note: %s: worktree has uncommitted or untracked changes — left active\n", r.ID)
	}
	fmt.Fprintf(out, "work: archived %d of %d\n", rep.Archived(), requestedCount)

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

func dirtyArchiveRows(rows []worklist.WorkRow) []worklist.WorkRow {
	var dirty []worklist.WorkRow
	for _, r := range rows {
		if r.WorktreePath == "" {
			continue
		}
		if d, err := gitx.Open(r.WorktreePath).IsDirty(); err == nil && d {
			dirty = append(dirty, r)
		}
	}
	return dirty
}

func filterDirtyDeclines(rows, dirty []worklist.WorkRow, answers []bool) []worklist.WorkRow {
	approved := make(map[string]bool, len(dirty))
	for i, r := range dirty {
		approved[r.ID] = i < len(answers) && answers[i]
	}
	out := make([]worklist.WorkRow, 0, len(rows))
	for _, r := range rows {
		if accepted, isDirty := approved[r.ID]; !isDirty || accepted {
			out = append(out, r)
		}
	}
	return out
}

func declinedDirtyRows(dirty []worklist.WorkRow, answers []bool) []worklist.WorkRow {
	var declined []worklist.WorkRow
	for i, r := range dirty {
		if i >= len(answers) || !answers[i] {
			declined = append(declined, r)
		}
	}
	return declined
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

// archiveConsequences is the destructive-effects block every archive
// confirmation states (F2 contract cli-work-archive.md §3): the worktrees are
// removed, the snapshots relocate, the branches survive.
func archiveConsequences(n int, workspaceRoot string) string {
	dir := "the archived area"
	if workspaceRoot != "" {
		dir = workspaceRoot + "/archived/"
	}
	return fmt.Sprintf(
		"%d worktree(s) will be destroyed.\nSnapshots move to %s\nBranches are kept.",
		n, dir)
}

// archiveConfirmImpact is the preview for the explicit-target confirmation: the
// Works about to be archived, then the shared consequences block.
func archiveConfirmImpact(rows []worklist.WorkRow, workspaceRoot string) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s  (%s)\n", r.DisplayName, r.Branch)
	}
	b.WriteString("\n")
	b.WriteString(archiveConsequences(len(rows), workspaceRoot))
	return b.String()
}
