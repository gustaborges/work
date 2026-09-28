package staging

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func wantRefusal(t *testing.T, err error, rel string, reason RefusalReason) {
	t.Helper()
	var ref *Refusal
	if !errors.As(err, &ref) {
		t.Fatalf("err = %v, want a *Refusal", err)
	}
	if ref.Rel != rel || ref.Reason != reason {
		t.Errorf("refusal = %q/%s, want %q/%s", ref.Rel, ref.Reason, rel, reason)
	}
}

func TestBuildRefusesEveryUnsafeShapeAndLeavesTheWorkUntouched(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, stage, work string)
		rel    string
		reason RefusalReason
	}{
		{"file destination exists", func(t *testing.T, stage, work string) {
			write(t, work, "notes/pr.md", "old")
			write(t, stage, "notes/pr.md", "new")
		}, "notes/pr.md", Exists},
		{"file destination is a directory", func(t *testing.T, stage, work string) {
			if err := os.MkdirAll(filepath.Join(work, "notes"), 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, stage, "notes", "a file where a directory is")
		}, "notes", Exists},
		{"directory destination is a file", func(t *testing.T, stage, work string) {
			write(t, work, "notes", "a file")
			write(t, stage, "notes/pr.md", "x")
		}, "notes", Exists},
		{"worktree itself", func(t *testing.T, stage, work string) {
			write(t, stage, "worktree", "x")
		}, "worktree", ReservedPath},
		{"inside the worktree", func(t *testing.T, stage, work string) {
			write(t, stage, "worktree/injected.md", "x")
		}, "worktree", ReservedPath},
		{"snapshot", func(t *testing.T, stage, work string) {
			write(t, stage, "work-state.json", "{}")
		}, "work-state.json", ReservedPath},
		{"worktree, other case", func(t *testing.T, stage, work string) {
			write(t, stage, "Worktree/x.md", "x")
		}, "Worktree", ReservedPath},
		{"snapshot, other case", func(t *testing.T, stage, work string) {
			write(t, stage, "WORK-STATE.JSON", "{}")
		}, "WORK-STATE.JSON", ReservedPath},
		{"symlink", func(t *testing.T, stage, work string) {
			write(t, stage, "notes/real.md", "x")
			if err := os.Symlink("real.md", filepath.Join(stage, "notes", "link.md")); err != nil {
				t.Skipf("cannot create symlinks: %v", err)
			}
		}, "notes/link.md", NotRegular},
		{"destination symlink leaving the Work", func(t *testing.T, stage, work string) {
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(work, "out")); err != nil {
				t.Skipf("cannot create symlinks: %v", err)
			}
			write(t, stage, "out/x.md", "x")
		}, "out", EscapesWork},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stage, work := t.TempDir(), t.TempDir()
			write(t, work, "existing.md", "keep")
			tc.setup(t, stage, work)
			before := treeHash(t, work)

			_, err := Build(stage, work)
			wantRefusal(t, err, tc.rel, tc.reason)
			if treeHash(t, work) != before {
				t.Error("a refusal changed the Work directory")
			}
		})
	}
}

func TestBuildRefusesTheWholeExecutionForOneBadEntry(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	write(t, stage, "notes/clean.md", "clean")
	write(t, stage, "work-state.json", "{}")

	plan, err := Build(stage, work)
	wantRefusal(t, err, "work-state.json", ReservedPath)
	if len(plan.Items) != 0 {
		t.Errorf("plan = %+v, want nothing usable", plan.Items)
	}
}

func TestRefusalMessageNamesThePathNotContent(t *testing.T) {
	r := &Refusal{Rel: "notes/pr.md", Reason: Exists}
	if got := r.Error(); got != `staging: "notes/pr.md" already exists` {
		t.Errorf("message = %q", got)
	}
}

func TestIncorporateFailureAtEveryItemLeavesTheTreeByteIdentical(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	write(t, work, "notes/existing.md", "keep me")
	write(t, stage, "notes/added.md", "a")
	write(t, stage, "docs/deep/x.md", "x")
	write(t, stage, "docs/y.md", "y")

	plan, err := Build(stage, work)
	if err != nil {
		t.Fatal(err)
	}
	before := treeHash(t, work)

	for n := range len(plan.Items) {
		t.Run("fail at "+strconv.Itoa(n), func(t *testing.T) {
			t.Setenv("WORK_FAIL_AT", "incorporate:"+strconv.Itoa(n))
			created, err := plan.Incorporate()
			if err == nil {
				t.Fatal("Incorporate succeeded, want the injected failure")
			}
			if created != 0 {
				t.Errorf("created = %d after a failure, want 0", created)
			}
			if treeHash(t, work) != before {
				t.Error("the Work directory changed although incorporation failed")
			}
			if got, _ := os.ReadFile(filepath.Join(work, "notes", "existing.md")); string(got) != "keep me" {
				t.Errorf("existing file = %q", got)
			}
		})
	}

	// With the injection lifted the same plan still incorporates cleanly.
	t.Setenv("WORK_FAIL_AT", "")
	if _, err := plan.Incorporate(); err != nil {
		t.Fatalf("Incorporate after the failures: %v", err)
	}
}

func TestIncorporateNeverOverwritesAndRollsBackOnACollisionBetweenItems(t *testing.T) {
	// Two items that reach the same file stand in for two names that differ
	// only in case on a case-insensitive volume: exclusive create is what stops
	// the second, and the first must be rolled back.
	stage, work := t.TempDir(), t.TempDir()
	write(t, stage, "a.md", "first")
	write(t, stage, "b.md", "second")
	write(t, work, "keep.md", "keep")
	before := treeHash(t, work)

	dest := filepath.Join(work, "same.md")
	plan := Plan{Items: []Item{
		{Rel: "same.md", Src: filepath.Join(stage, "a.md"), Dest: dest},
		{Rel: "SAME.md", Src: filepath.Join(stage, "b.md"), Dest: dest},
	}}
	if _, err := plan.Incorporate(); err == nil {
		t.Fatal("Incorporate succeeded, want a collision")
	}
	if treeHash(t, work) != before {
		t.Error("the first item was not rolled back")
	}
}

func TestIncorporateDoesNotRemoveAPreExistingFileItFailedToCreate(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	write(t, stage, "x.md", "new")
	write(t, work, "x.md", "precious")

	plan := Plan{Items: []Item{{Rel: "x.md", Src: filepath.Join(stage, "x.md"), Dest: filepath.Join(work, "x.md")}}}
	if _, err := plan.Incorporate(); err == nil {
		t.Fatal("Incorporate overwrote or ignored an existing file")
	}
	if got, _ := os.ReadFile(filepath.Join(work, "x.md")); string(got) != "precious" {
		t.Errorf("x.md = %q, want it untouched", got)
	}
}

func TestIncorporateRollbackKeepsAMergedDirectoryItDidNotCreate(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	write(t, work, "notes/old.md", "old")
	write(t, stage, "notes/new.md", "new")
	write(t, stage, "z.md", "z")

	plan, err := Build(stage, work)
	if err != nil {
		t.Fatal(err)
	}
	// Items: notes (merged), notes/new.md, z.md — fail on the last.
	t.Setenv("WORK_FAIL_AT", "incorporate:"+strconv.Itoa(len(plan.Items)-1))
	if _, err := plan.Incorporate(); err == nil {
		t.Fatal("want the injected failure")
	}
	if _, err := os.Stat(filepath.Join(work, "notes", "old.md")); err != nil {
		t.Errorf("the merged directory or its file was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "notes", "new.md")); !os.IsNotExist(err) {
		t.Errorf("notes/new.md survived the rollback: %v", err)
	}
}
