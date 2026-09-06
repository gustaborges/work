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
const SchemaVersion = 2

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
	// ArchivedAt mirrors work.archived_at: RFC 3339 UTC for an archived Work,
	// empty for an active one.
	ArchivedAt string
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

// migrations holds the forward-only, version-stepped DDL. Index i upgrades a
// database from user_version i to user_version i+1. A fresh database runs every
// step in order; an F1 database at user_version 1 runs only the last.
var migrations = []string{
	// 0 -> 1: the F1 works table.
	`
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
`,
	// 1 -> 2 (F2): archived Works. status already TEXT, so 'archived' needs no
	// DDL; add the nullable archived_at column and the status filter index.
	`
ALTER TABLE works ADD COLUMN archived_at TEXT;
CREATE INDEX IF NOT EXISTS works_status ON works(status);
`,
}

// Migrate steps the database forward to SchemaVersion. It is idempotent and
// safe on a fresh database (runs every step) and on an F1 database (runs only
// the 1 -> 2 step, preserving existing rows).
func (d *DB) Migrate() error {
	var version int
	if err := d.sql.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("projection: reading user_version: %w", err)
	}
	for version < SchemaVersion {
		if _, err := d.sql.Exec(migrations[version]); err != nil {
			return fmt.Errorf("projection: migrating %d -> %d: %w", version, version+1, err)
		}
		version++
		if _, err := d.sql.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			return fmt.Errorf("projection: setting user_version: %w", err)
		}
	}
	return nil
}

// Reset drops the works table and recreates it empty at the current
// SchemaVersion. It backs a full index rebuild (internal/reconcile); the
// canonical snapshots are the authority and are never touched.
func (d *DB) Reset() error {
	if _, err := d.sql.Exec("DROP TABLE IF EXISTS works"); err != nil {
		return fmt.Errorf("projection: reset: drop: %w", err)
	}
	if _, err := d.sql.Exec("PRAGMA user_version = 0"); err != nil {
		return fmt.Errorf("projection: reset: user_version: %w", err)
	}
	return d.Migrate()
}

// UserVersion returns the database's PRAGMA user_version.
func (d *DB) UserVersion() (int, error) {
	var v int
	if err := d.sql.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("projection: reading user_version: %w", err)
	}
	return v, nil
}

// Upsert inserts w or replaces the existing row with the same id.
func (d *DB) Upsert(w Work) error {
	const q = `
INSERT INTO works (id, slug, status, start_mode, starter, branch, base_branch,
	branch_convention, repo_name, dir_path, worktree_path, snapshot_path,
	created_at, last_accessed_at, archived_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	slug=excluded.slug, status=excluded.status, start_mode=excluded.start_mode,
	starter=excluded.starter, branch=excluded.branch, base_branch=excluded.base_branch,
	branch_convention=excluded.branch_convention, repo_name=excluded.repo_name,
	dir_path=excluded.dir_path, worktree_path=excluded.worktree_path,
	snapshot_path=excluded.snapshot_path, created_at=excluded.created_at,
	last_accessed_at=excluded.last_accessed_at, archived_at=excluded.archived_at
`
	_, err := d.sql.Exec(q, w.ID, w.Slug, w.Status, w.StartMode, w.Starter, w.Branch,
		w.BaseBranch, w.BranchConvention, w.RepoName, w.DirPath, w.WorktreePath,
		w.SnapshotPath, w.CreatedAt, w.LastAccessedAt, nullIfEmpty(w.ArchivedAt))
	if err != nil {
		return fmt.Errorf("projection: upsert %s: %w", w.ID, err)
	}
	return nil
}

const selectColumns = `id, slug, status, start_mode, starter, branch, base_branch,
	branch_convention, repo_name, dir_path, worktree_path, snapshot_path,
	created_at, last_accessed_at, archived_at`

func scanWork(s interface{ Scan(...any) error }) (Work, error) {
	var w Work
	var archivedAt sql.NullString
	err := s.Scan(&w.ID, &w.Slug, &w.Status, &w.StartMode, &w.Starter, &w.Branch,
		&w.BaseBranch, &w.BranchConvention, &w.RepoName, &w.DirPath, &w.WorktreePath,
		&w.SnapshotPath, &w.CreatedAt, &w.LastAccessedAt, &archivedAt)
	w.ArchivedAt = archivedAt.String
	return w, err
}

// nullIfEmpty maps "" to a SQL NULL so an active Work's archived_at column is
// NULL rather than an empty string.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
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

// recencyOrder is the total, deterministic ordering shared by every list query:
// most-recently-accessed first, ties broken by the ULID id (creation order).
const recencyOrder = " ORDER BY last_accessed_at DESC, id DESC"

// List returns every row, most-recently-accessed first.
func (d *DB) List() ([]Work, error) {
	return d.query("SELECT "+selectColumns+" FROM works"+recencyOrder, "list")
}

// ListActive returns the in-progress rows only, most-recently-accessed first.
// It is the source for the resume and archive pickers.
func (d *DB) ListActive() ([]Work, error) {
	return d.query("SELECT "+selectColumns+" FROM works WHERE status = 'in-progress'"+recencyOrder, "list-active")
}

func (d *DB) query(q, label string) ([]Work, error) {
	rows, err := d.sql.Query(q)
	if err != nil {
		return nil, fmt.Errorf("projection: %s: %w", label, err)
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

// SetAccessed bumps a Work's last_accessed_at. It is the post-commit projection
// trailer of a successful `work resume`; a missing row is not an error (the
// snapshot is authoritative and the next reconcile re-adds the row).
func (d *DB) SetAccessed(id, ts string) error {
	if _, err := d.sql.Exec("UPDATE works SET last_accessed_at = ? WHERE id = ?", ts, id); err != nil {
		return fmt.Errorf("projection: set-accessed %s: %w", id, err)
	}
	return nil
}

// MarkArchived flips a Work's row to archived: status, archived_at, the new
// archived-area dir_path / snapshot_path, and an empty worktree_path. It is the
// post-commit projection trailer of `work archive`. The caller supplies the
// already-relocated paths and the archival timestamp on row.
func (d *DB) MarkArchived(id string, row Work) error {
	const q = `UPDATE works
	SET status = 'archived', archived_at = ?, dir_path = ?, snapshot_path = ?, worktree_path = ''
	WHERE id = ?`
	if _, err := d.sql.Exec(q, nullIfEmpty(row.ArchivedAt), row.DirPath, row.SnapshotPath, id); err != nil {
		return fmt.Errorf("projection: mark-archived %s: %w", id, err)
	}
	return nil
}
