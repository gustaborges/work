package archive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveDirNoCollision(t *testing.T) {
	ws := t.TempDir()
	date := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	got := ArchiveDir(ws, "demo", "alpha", date)
	want := filepath.Join(ws, "archived", "20260906-demo_alpha")
	if got != want {
		t.Fatalf("ArchiveDir = %q, want %q", got, want)
	}
}

func TestArchiveDirSuffixesOnSameDayRepeat(t *testing.T) {
	ws := t.TempDir()
	date := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	archived := filepath.Join(ws, "archived")

	mkdir(t, filepath.Join(archived, "20260906-demo_alpha"))
	if got, want := ArchiveDir(ws, "demo", "alpha", date), filepath.Join(archived, "20260906-demo_alpha-2"); got != want {
		t.Fatalf("first repeat = %q, want %q", got, want)
	}

	mkdir(t, filepath.Join(archived, "20260906-demo_alpha-2"))
	if got, want := ArchiveDir(ws, "demo", "alpha", date), filepath.Join(archived, "20260906-demo_alpha-3"); got != want {
		t.Fatalf("second repeat = %q, want %q", got, want)
	}
}

func TestArchiveDirLeavesUnrelatedDirAlone(t *testing.T) {
	ws := t.TempDir()
	date := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	// A same-day archive of a *different* branch does not collide.
	mkdir(t, filepath.Join(ws, "archived", "20260906-demo_beta"))
	got := ArchiveDir(ws, "demo", "alpha", date)
	want := filepath.Join(ws, "archived", "20260906-demo_alpha")
	if got != want {
		t.Fatalf("ArchiveDir = %q, want %q", got, want)
	}
}

func TestSanitizeBranch(t *testing.T) {
	if got := SanitizeBranch("feature/x/y"); got != "feature-x-y" {
		t.Fatalf("SanitizeBranch = %q, want %q", got, "feature-x-y")
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
