package projection

import (
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func sampleWork(id string) Work {
	return Work{
		ID: id, Slug: "add-retry", Status: "in-progress", StartMode: "new",
		Starter: "local-path-starter", Branch: "add-retry", BaseBranch: "main",
		BranchConvention: "freeform", RepoName: "demo",
		DirPath:      "/ws/in-progress/demo_add-retry-" + id,
		WorktreePath: "/ws/in-progress/demo_add-retry-" + id + "/worktree",
		SnapshotPath: "/ws/in-progress/demo_add-retry-" + id + "/work-state.json",
		CreatedAt:    "2026-09-05T14:03:11Z", LastAccessedAt: "2026-09-05T14:03:11Z",
	}
}

func userVersion(t *testing.T, db *DB) int {
	t.Helper()
	var v int
	if err := db.sql.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func archivedWork(id string) Work {
	w := sampleWork(id)
	w.Status = "archived"
	w.ArchivedAt = "2026-09-06T18:22:00Z"
	w.LastAccessedAt = "2026-09-06T18:22:00Z"
	w.DirPath = "/ws/archived/20260906-demo_add-retry-" + id
	w.SnapshotPath = w.DirPath + "/work-state.json"
	w.WorktreePath = ""
	return w
}

func TestMigrateIdempotent(t *testing.T) {
	db := openTest(t)
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate (2nd): %v", err)
	}
	if v := userVersion(t, db); v != SchemaVersion {
		t.Errorf("user_version = %d, want %d", v, SchemaVersion)
	}
}

func TestFreshDatabaseOpensAtV2(t *testing.T) {
	db := openTest(t)
	if v := userVersion(t, db); v != 2 {
		t.Fatalf("fresh db user_version = %d, want 2", v)
	}
	// archived_at and the status index exist.
	if err := db.Upsert(archivedWork("z")); err != nil {
		t.Fatalf("Upsert archived: %v", err)
	}
	got, _, _ := db.Get("z")
	if got.Status != "archived" || got.ArchivedAt != "2026-09-06T18:22:00Z" {
		t.Errorf("archived row round-trip: %+v", got)
	}
}

func TestMigrateV1toV2InPlace(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "work.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// Stand up an F1 database: the 0->1 migration, user_version 1, one row with
	// the F1 column set only.
	if _, err := db.sql.Exec(migrations[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec(`INSERT INTO works
		(id, slug, status, start_mode, starter, branch, base_branch, branch_convention,
		 repo_name, dir_path, worktree_path, snapshot_path, created_at, last_accessed_at)
		VALUES ('f1','s','in-progress','new','local-path-starter','b','main','freeform',
		 'demo','/ws/in-progress/demo_b','/ws/in-progress/demo_b/worktree',
		 '/ws/in-progress/demo_b/work-state.json','2026-01-01T00:00:00Z','2026-01-02T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate v1->v2: %v", err)
	}
	if v := userVersion(t, db); v != 2 {
		t.Fatalf("after migrate user_version = %d, want 2", v)
	}
	// Idempotent.
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate (again): %v", err)
	}

	// The pre-existing row survived, with archived_at NULL (empty in Go).
	got, ok, err := db.Get("f1")
	if err != nil || !ok {
		t.Fatalf("Get(f1) after migrate: %v, %v", ok, err)
	}
	if got.Slug != "s" || got.LastAccessedAt != "2026-01-02T00:00:00Z" || got.ArchivedAt != "" {
		t.Errorf("row not preserved: %+v", got)
	}
}

func TestUpsertGetDelete(t *testing.T) {
	db := openTest(t)
	w := sampleWork("a")

	if err := db.Upsert(w); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, ok, err := db.Get("a")
	if err != nil || !ok {
		t.Fatalf("Get: %v, %v", ok, err)
	}
	if got != w {
		t.Errorf("Get mismatch:\n got %+v\nwant %+v", got, w)
	}

	// Upsert same id again -> still one row, updated field.
	w.Slug = "renamed"
	if err := db.Upsert(w); err != nil {
		t.Fatal(err)
	}
	list, _ := db.List()
	if len(list) != 1 || list[0].Slug != "renamed" {
		t.Errorf("after re-upsert: %+v", list)
	}

	if err := db.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := db.Get("a"); ok {
		t.Errorf("row still present after Delete")
	}
	// Deleting again is fine.
	if err := db.Delete("a"); err != nil {
		t.Errorf("Delete(missing): %v", err)
	}
}

func TestGetMissing(t *testing.T) {
	db := openTest(t)
	if _, ok, err := db.Get("nope"); err != nil || ok {
		t.Errorf("Get(missing) = %v, %v; want false, nil", ok, err)
	}
}

func TestDirPathUnique(t *testing.T) {
	db := openTest(t)
	a := sampleWork("a")
	b := sampleWork("b")
	b.DirPath = a.DirPath // collide
	if err := db.Upsert(a); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert(b); err == nil {
		t.Errorf("Upsert with duplicate dir_path: want UNIQUE violation")
	}
}

func TestListOrderedByLastAccessed(t *testing.T) {
	db := openTest(t)
	older := sampleWork("old")
	older.LastAccessedAt = "2026-01-01T00:00:00Z"
	newer := sampleWork("new")
	newer.LastAccessedAt = "2026-12-31T00:00:00Z"
	if err := db.Upsert(older); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert(newer); err != nil {
		t.Fatal(err)
	}
	list, err := db.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "new" {
		t.Errorf("List order = %v", []string{list[0].ID, list[1].ID})
	}
}

func TestListOrderIsTotalOnEqualTimestamps(t *testing.T) {
	db := openTest(t)
	// Same timestamp; the ULID id is the tie-break, DESC.
	for _, id := range []string{"01AAA", "01BBB", "01CCC"} {
		w := sampleWork(id)
		w.LastAccessedAt = "2026-06-01T00:00:00Z"
		if err := db.Upsert(w); err != nil {
			t.Fatal(err)
		}
	}
	list, err := db.List()
	if err != nil {
		t.Fatal(err)
	}
	got := []string{list[0].ID, list[1].ID, list[2].ID}
	want := []string{"01CCC", "01BBB", "01AAA"}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("tie-break order = %v, want %v", got, want)
	}
}

func TestListActive(t *testing.T) {
	db := openTest(t)
	active := sampleWork("act")
	active.LastAccessedAt = "2026-05-01T00:00:00Z"
	archived := archivedWork("arc")
	if err := db.Upsert(active); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert(archived); err != nil {
		t.Fatal(err)
	}
	list, err := db.ListActive()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "act" {
		t.Errorf("ListActive = %+v, want just [act]", list)
	}
}

func TestSetAccessed(t *testing.T) {
	db := openTest(t)
	w := sampleWork("a")
	if err := db.Upsert(w); err != nil {
		t.Fatal(err)
	}
	if err := db.SetAccessed("a", "2027-01-01T00:00:00Z"); err != nil {
		t.Fatalf("SetAccessed: %v", err)
	}
	got, _, _ := db.Get("a")
	if got.LastAccessedAt != "2027-01-01T00:00:00Z" {
		t.Errorf("last_accessed_at = %q", got.LastAccessedAt)
	}
	// A missing row is not an error.
	if err := db.SetAccessed("nope", "2027-01-01T00:00:00Z"); err != nil {
		t.Errorf("SetAccessed(missing): %v", err)
	}
}

func TestMarkArchived(t *testing.T) {
	db := openTest(t)
	w := sampleWork("a")
	if err := db.Upsert(w); err != nil {
		t.Fatal(err)
	}
	row := w
	row.ArchivedAt = "2026-09-06T18:22:00Z"
	row.LastAccessedAt = "2026-09-06T18:22:00Z"
	row.DirPath = "/ws/archived/20260906-demo_add-retry-a"
	row.SnapshotPath = row.DirPath + "/work-state.json"
	if err := db.MarkArchived("a", row); err != nil {
		t.Fatalf("MarkArchived: %v", err)
	}
	got, _, _ := db.Get("a")
	if got.Status != "archived" || got.WorktreePath != "" || got.ArchivedAt != "2026-09-06T18:22:00Z" {
		t.Errorf("archived row: %+v", got)
	}
	if got.LastAccessedAt != "2026-09-06T18:22:00Z" {
		t.Errorf("archived row last_accessed_at = %q, want the archival time", got.LastAccessedAt)
	}
	if got.DirPath != row.DirPath || got.SnapshotPath != row.SnapshotPath {
		t.Errorf("archived paths: dir %q snap %q", got.DirPath, got.SnapshotPath)
	}
	if act, _ := db.ListActive(); len(act) != 0 {
		t.Errorf("archived Work still listed active: %+v", act)
	}
}
