package workcontext

import (
	"context"
	"github.com/gustaborges/work/internal/diag"
	"os"
	"path/filepath"
	"testing"
)

func TestByID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "in-progress", "work")
	snapshot(t, dir, "opaque-ID", 3, false)
	bad := filepath.Join(root, "in-progress", "bad")
	os.MkdirAll(bad, 0o755)
	os.WriteFile(filepath.Join(bad, "work-state.json"), []byte("garbage"), 0o644)
	for _, id := range []string{"opaque-ID", "opaque", "OPAQUE-ID", "../work"} {
		c, err := ByID(context.Background(), root, id, nil)
		if id == "opaque-ID" {
			if err != nil || c.State.Work.ID != id {
				t.Fatalf("%+v %v", c, err)
			}
		} else if diag.ExitCode(err) != 21 {
			t.Fatalf("%s %v", id, err)
		}
	}
	snapshot(t, filepath.Join(root, "archived", "duplicate"), "opaque-ID", 2, true)
	if _, err := ByID(context.Background(), root, "opaque-ID", nil); diag.ExitCode(err) != 25 {
		t.Fatal(err)
	}
}
func TestByIDMatchingInvalidAndInaccessibleRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "in-progress", "work")
	p := snapshot(t, dir, "id", 3, false)
	os.WriteFile(p, []byte(`{"schema":3,"work":{"id":"id"}}`), 0o644)
	if _, err := ByID(context.Background(), root, "id", nil); diag.ExitCode(err) != 25 {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := ByID(context.Background(), file, "id", nil); diag.ExitCode(err) != 25 {
		t.Fatal(err)
	}
}
func TestHintsAreUntrusted(t *testing.T) {
	old := filepath.Join(t.TempDir(), "archived", "old")
	p := snapshot(t, old, "id", 2, true)
	for _, tc := range []struct {
		path, id string
		code     int
	}{{p, "id", 0}, {p, "other", 21}, {"relative/work-state.json", "id", 21}, {filepath.Dir(p), "id", 21}} {
		_, err := ByID(context.Background(), t.TempDir(), tc.id, func(context.Context, string) string { return tc.path })
		if diag.ExitCode(err) != tc.code {
			t.Fatalf("%+v %v", tc, err)
		}
	}
	root := t.TempDir()
	snapshot(t, filepath.Join(root, "in-progress", "new"), "id", 3, false)
	c, err := ByID(context.Background(), root, "id", func(context.Context, string) string { t.Fatal("hint used before canonical scan"); return p })
	if err != nil || c.State.Work.Status != "in-progress" {
		t.Fatalf("%+v %v", c, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ByID(ctx, root, "id", nil); diag.ExitCode(err) != 21 && diag.ExitCode(err) != 20 {
		t.Fatal(err)
	}
}
