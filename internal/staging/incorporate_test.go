package staging

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAppliedRetainsUndoAndTerminalState(t *testing.T) {
	stage, dest := t.TempDir(), t.TempDir()
	write(t, dest, "notes/old", "old")
	write(t, stage, "notes/new", "new")
	before := treeHash(t, dest)
	p, err := Build(stage, dest)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.ApplyContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.Created != 1 {
		t.Fatal(a.Created)
	}
	if err := a.Rollback(); err != nil {
		t.Fatal(err)
	}
	if treeHash(t, dest) != before {
		t.Fatal("rollback changed preexisting tree")
	}
	if err := a.Rollback(); err != nil {
		t.Fatal(err)
	}
	a, err = p.ApplyContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := a.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := a.Rollback(); err == nil {
		t.Fatal("rollback after commit accepted")
	}
}

func TestApplyCancelledAndFailedUndo(t *testing.T) {
	stage, dest := t.TempDir(), t.TempDir()
	write(t, stage, "a/file", "data")
	p, _ := Build(stage, dest)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a, err := p.ApplyContext(ctx)
	if err == nil {
		t.Fatal("cancel accepted")
	}
	if err := a.Rollback(); err != nil {
		t.Fatal(err)
	}
	a, err = p.ApplyContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// An independent entry prevents deleting our directory. Retain it for retry.
	write(t, dest, "a/foreign", "keep")
	if err := a.Rollback(); err == nil {
		t.Fatal("lost removal failure")
	}
	if len(a.Unresolved()) != 1 {
		t.Fatal(a.Unresolved())
	}
	os.Remove(filepath.Join(dest, "a/foreign"))
	if err := a.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRejectsChangedDestination(t *testing.T) {
	stage, dest := t.TempDir(), t.TempDir()
	write(t, stage, "a/file", "data")
	os.Mkdir(filepath.Join(dest, "a"), 0755)
	p, _ := Build(stage, dest)
	os.Rename(filepath.Join(dest, "a"), filepath.Join(dest, "old"))
	os.Mkdir(filepath.Join(dest, "a"), 0755)
	a, err := p.ApplyContext(context.Background())
	if err == nil {
		t.Fatal("changed ancestor accepted")
	}
	a.Rollback()
}

// Cancelling on a deterministic context check exercises the bounded copy loop
// after exclusive destination creation, without scheduling or timing races.
type copyCancel struct {
	context.Context
	calls  int
	cancel context.CancelFunc
}

func (c *copyCancel) Err() error {
	c.calls++
	if c.calls == 4 {
		c.cancel()
	}
	return c.Context.Err()
}
func TestCancellationDuringCopy(t *testing.T) {
	stage, dest := t.TempDir(), t.TempDir()
	write(t, stage, "large", string(make([]byte, 256*1024)))
	p, _ := Build(stage, dest)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &copyCancel{Context: base, cancel: cancel}
	a, err := p.ApplyContext(ctx)
	if err == nil || a.Created != 1 {
		t.Fatal(a, err)
	}
	if err := a.Rollback(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 0 {
		t.Fatal(entries)
	}
}

func TestRollbackPreservesReplacedEntry(t *testing.T) {
	stage, dest := t.TempDir(), t.TempDir()
	write(t, stage, "file", "ours")
	p, _ := Build(stage, dest)
	a, err := p.ApplyContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dest, "file"), filepath.Join(dest, "moved")); err != nil {
		t.Fatal(err)
	}
	write(t, dest, "file", "foreign")
	if err := a.Rollback(); err == nil {
		t.Fatal("replacement was not reported")
	}
	data, _ := os.ReadFile(filepath.Join(dest, "file"))
	if string(data) != "foreign" {
		t.Fatal("removed replacement")
	}
}
