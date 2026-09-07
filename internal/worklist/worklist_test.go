package worklist

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/projection"
)

func openDB(t *testing.T) *projection.DB {
	t.Helper()
	db, err := projection.Open(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func work(id, slug, branch, status, last string) projection.Work {
	w := projection.Work{
		ID: id, Slug: slug, Status: status, StartMode: "new", Starter: "local-path-starter",
		Branch: branch, BaseBranch: "main", BranchConvention: "freeform", RepoName: "demo",
		DirPath:      "/ws/in-progress/demo_" + branch + "-" + id,
		WorktreePath: "/ws/in-progress/demo_" + branch + "-" + id + "/worktree",
		SnapshotPath: "/ws/in-progress/demo_" + branch + "-" + id + "/work-state.json",
		CreatedAt:    "2026-01-01T00:00:00Z", LastAccessedAt: last,
	}
	if status == "archived" {
		w.ArchivedAt = last
		w.WorktreePath = ""
	}
	return w
}

func TestListRecencyOrderAndTieBreak(t *testing.T) {
	db := openDB(t)
	// alpha and bravo share a timestamp; charlie is newer.
	must(t, db.Upsert(work("01ALPHA0000000000000000000", "alpha", "alpha", "in-progress", "2026-06-01T00:00:00Z")))
	must(t, db.Upsert(work("01BRAVO0000000000000000000", "bravo", "bravo", "in-progress", "2026-06-01T00:00:00Z")))
	must(t, db.Upsert(work("01CHARLIE00000000000000000", "charlie", "charlie", "in-progress", "2026-06-02T00:00:00Z")))

	rows, err := List(db, false)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{rows[0].Slug, rows[1].Slug, rows[2].Slug}
	// charlie newest; alpha/bravo tie broken by id DESC -> bravo before alpha.
	want := []string{"charlie", "bravo", "alpha"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestListExcludesArchivedUnlessAsked(t *testing.T) {
	db := openDB(t)
	must(t, db.Upsert(work("01ACT00000000000000000000A", "act", "act", "in-progress", "2026-06-01T00:00:00Z")))
	must(t, db.Upsert(work("01ARC00000000000000000000A", "arc", "arc", "archived", "2026-06-05T00:00:00Z")))

	active, _ := List(db, false)
	if len(active) != 1 || active[0].Slug != "act" {
		t.Errorf("active list = %+v", active)
	}
	all, _ := List(db, true)
	if len(all) != 2 {
		t.Errorf("full list = %+v", all)
	}
}

func TestResolveIDOnly(t *testing.T) {
	rows := Rows([]projection.Work{
		work("01AAAAAAAAAAAAAAAAAAAAAAAAA", "alpha", "alpha", "in-progress", "2026-06-01T00:00:00Z"),
		work("01BBBBBBBBBBBBBBBBBBBBBBBBB", "bravo", "bravo", "archived", "2026-06-02T00:00:00Z"),
	}, time.Now())

	if r, o := Resolve(rows, "01AAAAAAAAAAAAAAAAAAAAAAAAA"); o != Resolved || r.Slug != "alpha" {
		t.Errorf("resolve active: %v %v", r, o)
	}
	if _, o := Resolve(rows, "01BBBBBBBBBBBBBBBBBBBBBBBBB"); o != Archived {
		t.Errorf("resolve archived: got %v, want archived", o)
	}
	if _, o := Resolve(rows, "nonexistent"); o != NotFound {
		t.Errorf("resolve unknown: got %v, want not-found", o)
	}
	// A slug is never a valid target.
	if _, o := Resolve(rows, "alpha"); o != NotFound {
		t.Errorf("resolve by slug: got %v, want not-found", o)
	}
}

func TestDisambiguationSuffix(t *testing.T) {
	now := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	// Same slug, different branch -> no suffix (branch on line 2 separates them).
	diffBranch := Rows([]projection.Work{
		work("01AAAAAAAAAAAAAAAAAAAAAAAAA", "fix", "fix-a", "in-progress", "2026-06-01T00:00:00Z"),
		work("01BBBBBBBBBBBBBBBBBBBBBBBBB", "fix", "fix-b", "in-progress", "2026-06-01T00:00:00Z"),
	}, now)
	for _, r := range diffBranch {
		if r.DisplayName != "demo  fix" {
			t.Errorf("different branch should not get a suffix: %q", r.DisplayName)
		}
	}

	// Same (repo, slug, branch) -> both get the short-id suffix.
	collide := Rows([]projection.Work{
		work("01AAAAAAAAAAAAAAAAAAAAAAAAA", "fix", "fix", "in-progress", "2026-06-01T00:00:00Z"),
		work("01BBBBBBBBBBBBBBBBBBBBBBBBB", "fix", "fix", "in-progress", "2026-06-01T00:00:00Z"),
	}, now)
	if collide[0].DisplayName != "demo  fix  (01AAAA)" || collide[1].DisplayName != "demo  fix  (01BBBB)" {
		t.Errorf("collision suffix wrong: %q / %q", collide[0].DisplayName, collide[1].DisplayName)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	rows := Rows([]projection.Work{
		work("01AAAAAAAAAAAAAAAAAAAAAAAAA", "x", "x", "in-progress", "2026-06-10T09:00:00Z"),
	}, now)
	if rows[0].RelativeTime != "3 hours ago" {
		t.Errorf("relative time = %q, want %q", rows[0].RelativeTime, "3 hours ago")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
