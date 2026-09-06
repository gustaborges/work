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

func TestMigrateIdempotent(t *testing.T) {
	db := openTest(t)
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate (2nd): %v", err)
	}
	var v int
	if err := db.sql.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != SchemaVersion {
		t.Errorf("user_version = %d, want %d", v, SchemaVersion)
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
