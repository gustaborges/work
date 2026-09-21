package staging

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// write creates rel (slash-separated) under dir with content, making parents.
func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// treeHash summarises every path, type and file content under dir, so a test
// can prove a directory is byte-identical before and after.
func treeHash(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		h.Write([]byte(rel + "|" + d.Type().String() + "|"))
		if d.Type().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write(b)
		}
		h.Write([]byte("\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(h.Sum(nil))
}

func relOf(p Plan) []string {
	var out []string
	for _, it := range p.Items {
		out = append(out, it.Rel)
	}
	return out
}

func TestNewStageIsPrivateUniqueAndRemovable(t *testing.T) {
	a, err := NewStage()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewStage()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Remove()
	defer b.Remove()

	if a.Dir == b.Dir {
		t.Fatalf("two stages share %s", a.Dir)
	}
	if want := filepath.Join(os.TempDir(), "work"); filepath.Dir(a.Dir) != want || !strings.HasPrefix(filepath.Base(a.Dir), "import-") {
		t.Errorf("stage %s is not under %s/import-*", a.Dir, want)
	}
	info, err := os.Stat(a.Dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("stage missing: %v", err)
	}
	if entries, _ := os.ReadDir(a.Dir); len(entries) != 0 {
		t.Errorf("stage not empty: %v", entries)
	}
	if info.Mode().Perm()&0o077 != 0 && filepath.Separator == '/' {
		t.Errorf("stage mode = %v, want 0700", info.Mode().Perm())
	}
	write(t, a.Dir, "x/y.txt", "y")
	if err := a.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.Dir); !os.IsNotExist(err) {
		t.Errorf("stage still exists after Remove: %v", err)
	}
	if err := a.Remove(); err != nil {
		t.Errorf("second Remove: %v", err)
	}
}

func TestBuildCleanPathListsDirectoriesBeforeFiles(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	write(t, stage, "notes/context.md", "c")
	write(t, stage, "a.txt", "a")
	write(t, stage, "notes/deep/x.md", "x")
	before := treeHash(t, work)

	plan, err := Build(stage, work)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := []string{"notes", "notes/deep", "a.txt", "notes/context.md", "notes/deep/x.md"}
	if got := relOf(plan); !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if treeHash(t, work) != before {
		t.Error("Build modified the Work directory")
	}
	if plan.Items[0].Dest != filepath.Join(work, "notes") {
		t.Errorf("dest = %s", plan.Items[0].Dest)
	}
}

func TestBuildEmptyStageIsAnEmptyPlan(t *testing.T) {
	plan, err := Build(t.TempDir(), t.TempDir())
	if err != nil || len(plan.Items) != 0 {
		t.Fatalf("plan = %+v, err = %v", plan, err)
	}
}

func TestBuildMergesIntoExistingDirectoryWhenNothingCollides(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	write(t, work, "notes/old.md", "old")
	write(t, stage, "notes/new.md", "new")

	plan, err := Build(stage, work)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	n, err := plan.Incorporate()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("created = %d, want 1 (the directory already existed)", n)
	}
	for _, rel := range []string{"notes/old.md", "notes/new.md"} {
		if _, err := os.Stat(filepath.Join(work, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s: %v", rel, err)
		}
	}
}

func TestIncorporateCreatesTreeAndPreservesRelativePaths(t *testing.T) {
	stage, work := t.TempDir(), t.TempDir()
	write(t, stage, "notes/context.md", "hello")
	write(t, stage, "notes/deep/x.md", "deep")

	plan, err := Build(stage, work)
	if err != nil {
		t.Fatal(err)
	}
	n, err := plan.Incorporate()
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("created = %d, want 4 (2 dirs + 2 files)", n)
	}
	for rel, want := range map[string]string{"notes/context.md": "hello", "notes/deep/x.md": "deep"} {
		got, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v", rel, got, err)
		}
	}
	// Copy, not move: the stage is untouched and can be removed on its own.
	if _, err := os.Stat(filepath.Join(stage, "notes", "context.md")); err != nil {
		t.Errorf("stage file gone after incorporation: %v", err)
	}
}

func TestIncorporateEmptyPlanCreatesNothing(t *testing.T) {
	n, err := Plan{}.Incorporate()
	if n != 0 || err != nil {
		t.Errorf("n=%d err=%v", n, err)
	}
}
