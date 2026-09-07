package tui

import (
	"errors"
	"fmt"
	"strings"

	huh "charm.land/huh/v2"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/worklist"
)

// abort maps a huh abort into a diag cancelled error; other errors pass through.
func abort(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return diag.New(diag.Cancelled, "cancelled")
	}
	return err
}

func run(field huh.Field) error {
	return abort(huh.NewForm(huh.NewGroup(field)).Run())
}

// ConfirmArchive shows the Works about to be archived and the destructive
// consequences, and asks for a yes/no confirmation (default No). It is the
// explicit-target equivalent of the archive picker's confirmation view.
func ConfirmArchive(rows []worklist.WorkRow, workspaceRoot string) (bool, error) {
	var b strings.Builder
	b.WriteString("Archive Works\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s  (%s)\n", r.DisplayName, r.Branch)
	}
	fmt.Fprintf(&b, "\n%d worktree(s) will be destroyed.\nSnapshots move to %s\nBranches are kept.",
		len(rows), archivedDir(workspaceRoot))
	var ok bool
	err := run(huh.NewConfirm().
		Title(b.String()).
		Affirmative("Archive").
		Negative("Cancel").
		Value(&ok))
	if err != nil {
		return false, err
	}
	return ok, nil
}

// AckDirtyWork asks whether to archive one Work whose worktree has uncommitted
// or untracked changes (default No). Declining leaves that Work active.
func AckDirtyWork(row worklist.WorkRow) (bool, error) {
	var ok bool
	err := run(huh.NewConfirm().
		Title(row.DisplayName + "  (" + row.Branch + ")\n\nThis worktree has uncommitted or untracked changes.\nArchive it anyway? The changes in the worktree will be lost.").
		Affirmative("Archive anyway").
		Negative("Keep active").
		Value(&ok))
	if err != nil {
		return false, err
	}
	return ok, nil
}
