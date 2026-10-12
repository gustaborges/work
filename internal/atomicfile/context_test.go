package atomicfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Cancellation at the second check occurs after preparation, immediately
// before rename. This avoids a timing-dependent fsync/cancellation race.
type cancelAtReplacement struct {
	context.Context
	checks int
}

func (c *cancelAtReplacement) Err() error {
	c.checks++
	if c.checks >= 2 {
		return context.Canceled
	}
	return nil
}

func TestCancellationBeforeReplacementPreservesDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "saved")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAtReplacement{Context: context.Background()}
	if err := WriteFileContext(ctx, path, []byte("replacement")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "old" {
		t.Fatal(string(b), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
}
