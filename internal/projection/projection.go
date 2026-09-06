// Package projection is the SQLite lookup database derived from Work snapshots.
// It is a projection only (ADR-0013): every row can be rebuilt from the
// canonical work-state.json files, and nothing reads it as an authority.
package projection

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the current PRAGMA user_version.
const SchemaVersion = 1

// Work is one row of the works table.
type Work struct {
	ID               string
	Slug             string
	Status           string
	StartMode        string
	Starter          string
	Branch           string
	BaseBranch       string
	BranchConvention string
	RepoName         string
	DirPath          string
	WorktreePath     string
	SnapshotPath     string
	CreatedAt        string
	LastAccessedAt   string
}

// DB wraps the projection database handle.
type DB struct {
	sql *sql.DB
}

// Open opens (creating if absent) the projection database at path.
func Open(path string) (*DB, error) {
	h, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("projection: open %s: %w", path, err)
	}
	// A single connection sidesteps cross-connection locking for this
	// single-user, low-traffic database.
	h.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := h.Exec(pragma); err != nil {
			h.Close()
			return nil, fmt.Errorf("projection: %s: %w", pragma, err)
		}
	}
	return &DB{sql: h}, nil
}

// Close releases the database handle.
func (d *DB) Close() error { return d.sql.Close() }

// Migrate creates the schema if the database is new. It is idempotent.
func (d *DB) Migrate() error {
	var version int
	if err := d.sql.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("projection: reading user_version: %w", err)
	}
	if version >= SchemaVersion {
		return nil
	}
	const ddl = `
CREATE TABLE IF NOT EXISTS works (
	id                TEXT PRIMARY KEY,
	slug              TEXT NOT NULL,
	status            TEXT NOT NULL,
	start_mode        TEXT NOT NULL,
	starter           TEXT NOT NULL,
	branch            TEXT NOT NULL,
	base_branch       TEXT NOT NULL,
	branch_convention TEXT,
	repo_name         TEXT NOT NULL,
	dir_path          TEXT NOT NULL UNIQUE,
	worktree_path     TEXT NOT NULL,
	snapshot_path     TEXT NOT NULL,
	created_at        TEXT NOT NULL,
	last_accessed_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS works_last_accessed ON works(last_accessed_at DESC);
`
	if _, err := d.sql.Exec(ddl); err != nil {
		return fmt.Errorf("projection: creating schema: %w", err)
	}
	if _, err := d.sql.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		return fmt.Errorf("projection: setting user_version: %w", err)
	}
	return nil
}

// Upsert inserts w or replaces the existing row with the same id.
func (d *DB) Upsert(w Work) error {
	const q = `
INSERT INTO works (id, slug, status, start_mode, starter, branch, base_branch,
	branch_convention, repo_name, dir_path, worktree_path, snapshot_path,
	created_at, last_accessed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	slug=excluded.slug, status=excluded.status, start_mode=excluded.start_mode,
	starter=excluded.starter, branch=excluded.branch, base_branch=excluded.base_branch,
	branch_convention=excluded.branch_convention, repo_name=excluded.repo_name,
	dir_path=excluded.dir_path, worktree_path=excluded.worktree_path,
	snapshot_path=excluded.snapshot_path, created_at=excluded.created_at,
	last_accessed_at=excluded.last_accessed_at
`
	_, err := d.sql.Exec(q, w.ID, w.Slug, w.Status, w.StartMode, w.Starter, w.Branch,
		w.BaseBranch, w.BranchConvention, w.RepoName, w.DirPath, w.WorktreePath,
		w.SnapshotPath, w.CreatedAt, w.LastAccessedAt)
	if err != nil {
		return fmt.Errorf("projection: upsert %s: %w", w.ID, err)
	}
	return nil
}

const selectColumns = `id, slug, status, start_mode, starter, branch, base_branch,
	branch_convention, repo_name, dir_path, worktree_path, snapshot_path,
	created_at, last_accessed_at`

func scanWork(s interface{ Scan(...any) error }) (Work, error) {
	var w Work
	err := s.Scan(&w.ID, &w.Slug, &w.Status, &w.StartMode, &w.Starter, &w.Branch,
		&w.BaseBranch, &w.BranchConvention, &w.RepoName, &w.DirPath, &w.WorktreePath,
		&w.SnapshotPath, &w.CreatedAt, &w.LastAccessedAt)
	return w, err
}

// Get returns the row with the given id. The bool is false when no row exists.
func (d *DB) Get(id string) (Work, bool, error) {
	row := d.sql.QueryRow("SELECT "+selectColumns+" FROM works WHERE id = ?", id)
	w, err := scanWork(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Work{}, false, nil
	}
	if err != nil {
		return Work{}, false, fmt.Errorf("projection: get %s: %w", id, err)
	}
	return w, true, nil
}

// Delete removes the row with the given id. Deleting a missing row is not an
// error.
func (d *DB) Delete(id string) error {
	if _, err := d.sql.Exec("DELETE FROM works WHERE id = ?", id); err != nil {
		return fmt.Errorf("projection: delete %s: %w", id, err)
	}
	return nil
}

// List returns every row, most-recently-accessed first.
func (d *DB) List() ([]Work, error) {
	rows, err := d.sql.Query("SELECT " + selectColumns + " FROM works ORDER BY last_accessed_at DESC")
	if err != nil {
		return nil, fmt.Errorf("projection: list: %w", err)
	}
	defer rows.Close()

	var out []Work
	for rows.Next() {
		w, err := scanWork(rows)
		if err != nil {
			return nil, fmt.Errorf("projection: scan: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
