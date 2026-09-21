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
const SchemaVersion = 3

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
	// 2 -> 3 (F5): operational provenance of meta/links keys. Index-only: the
	// snapshot holds every value, and a rebuild does not restore this table.
	`
CREATE TABLE IF NOT EXISTS work_provenance (
	work_id          TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
	section          TEXT NOT NULL CHECK (section IN ('meta', 'links')),
	key              TEXT NOT NULL,
	source_component TEXT NOT NULL,
	source_operation TEXT NOT NULL,
	recorded_at      TEXT NOT NULL,
	PRIMARY KEY (work_id, section, key)
);
`,
}

// Migrate steps the database forward to SchemaVersion. It is idempotent and
// safe on a fresh database (runs every step) and on an older one (runs only
// the missing steps, preserving existing rows).
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

// Reset drops the works and provenance tables and recreates them empty at the
// current SchemaVersion. It backs a full index rebuild (internal/reconcile);
// the canonical snapshots are the authority and are never touched. Provenance
// goes first because it references works.
func (d *DB) Reset() error {
	if _, err := d.sql.Exec("DROP TABLE IF EXISTS work_provenance"); err != nil {
		return fmt.Errorf("projection: reset: drop provenance: %w", err)
	}
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

// execer is the part of *sql.DB and *sql.Tx that writes need, so one upsert
// serves both a bare call and a transaction.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// Upsert inserts w or replaces the existing row with the same id. It never
// deletes the row first, so a Work's provenance is not cascaded away.
func (d *DB) Upsert(w Work) error {
	return upsertWork(d.sql, w)
}

func upsertWork(x execer, w Work) error {
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
	_, err := x.Exec(q, w.ID, w.Slug, w.Status, w.StartMode, w.Starter, w.Branch,
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

// MarkArchived flips a Work's row to archived: status, archived_at,
// last_accessed_at (bumped to the archival time, mirroring the snapshot), the
// new archived-area dir_path / snapshot_path, and an empty worktree_path. It is
// the post-commit projection trailer of `work archive`. The caller supplies the
// already-relocated paths and the archival timestamps on row.
func (d *DB) MarkArchived(id string, row Work) error {
	const q = `UPDATE works
	SET status = 'archived', archived_at = ?, last_accessed_at = ?,
		dir_path = ?, snapshot_path = ?, worktree_path = ''
	WHERE id = ?`
	if _, err := d.sql.Exec(q, nullIfEmpty(row.ArchivedAt), row.LastAccessedAt,
		row.DirPath, row.SnapshotPath, id); err != nil {
		return fmt.Errorf("projection: mark-archived %s: %w", id, err)
	}
	return nil
}

// Provenance records which component last supplied a meta or links key of a
// Work and how. Only the current source is kept: a later publication replaces
// it (last source wins). The value itself lives only in the snapshot.
type Provenance struct {
	WorkID string
	// Section is "meta" or "links".
	Section string
	Key     string
	// SourceComponent is the component's "<alias>/<name>".
	SourceComponent string
	// SourceOperation is "start" for a Starter's publication and "discover"
	// for a Linker's.
	SourceOperation string
	// RecordedAt is RFC 3339 UTC.
	RecordedAt string
}

const upsertProvenanceSQL = `
INSERT INTO work_provenance (work_id, section, key, source_component, source_operation, recorded_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(work_id, section, key) DO UPDATE SET
	source_component=excluded.source_component,
	source_operation=excluded.source_operation,
	recorded_at=excluded.recorded_at
`

func recordProvenance(x execer, entries []Provenance) error {
	for _, p := range entries {
		if _, err := x.Exec(upsertProvenanceSQL, p.WorkID, p.Section, p.Key,
			p.SourceComponent, p.SourceOperation, p.RecordedAt); err != nil {
			return fmt.Errorf("projection: provenance %s %s.%s: %w", p.WorkID, p.Section, p.Key, err)
		}
	}
	return nil
}

// UpsertWithProvenance writes the Work row and its provenance entries in one
// transaction, so no Work is indexed without the provenance of the context it
// was created with. An entry that fails leaves no row behind.
func (d *DB) UpsertWithProvenance(w Work, entries []Provenance) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return fmt.Errorf("projection: upsert %s: %w", w.ID, err)
	}
	defer tx.Rollback()
	if err := upsertWork(tx, w); err != nil {
		return err
	}
	if err := recordProvenance(tx, entries); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("projection: upsert %s: %w", w.ID, err)
	}
	return nil
}

// RecordProvenance upserts entries, replacing the previous source of each
// (work, section, key).
func (d *DB) RecordProvenance(entries ...Provenance) error {
	return recordProvenance(d.sql, entries)
}

// Provenance returns a Work's provenance rows ordered by section then key.
func (d *DB) Provenance(workID string) ([]Provenance, error) {
	rows, err := d.sql.Query(`SELECT work_id, section, key, source_component, source_operation, recorded_at
FROM work_provenance WHERE work_id = ? ORDER BY section, key`, workID)
	if err != nil {
		return nil, fmt.Errorf("projection: provenance %s: %w", workID, err)
	}
	defer rows.Close()

	var out []Provenance
	for rows.Next() {
		var p Provenance
		if err := rows.Scan(&p.WorkID, &p.Section, &p.Key, &p.SourceComponent, &p.SourceOperation, &p.RecordedAt); err != nil {
			return nil, fmt.Errorf("projection: scan provenance: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
