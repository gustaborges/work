package reconcile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/gustaborges/work/internal/projection"
)

// openDB opens the projection database at path. When create is true, a file
// that cannot be opened as a database (corrupt, truncated) is deleted and
// recreated so a rebuild can proceed.
func openDB(path string, create bool) (*projection.DB, error) {
	db, err := projection.Open(path)
	if err == nil {
		return db, nil
	}
	if !create {
		return nil, err
	}
	if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("reconcile: replacing unopenable projection %s: %w", path, rmErr)
	}
	return projection.Open(path)
}

// Open returns a projection database that agrees with the on-disk snapshots.
// It rebuilds the database from the snapshots when the file is absent,
// unopenable, or below the current schema version, and otherwise runs a
// lightweight reconcile pass. The returned Report carries any skipped-snapshot
// diagnostics for the caller to surface; the caller owns the returned *DB and
// must Close it.
func Open(workspaceRoot, dbPath string) (*projection.DB, Report, error) {
	needRebuild := false
	if _, err := os.Stat(dbPath); errors.Is(err, fs.ErrNotExist) {
		needRebuild = true
	}

	if !needRebuild {
		db, err := projection.Open(dbPath)
		if err != nil {
			needRebuild = true
		} else if v, verr := db.UserVersion(); verr != nil || v < projection.SchemaVersion {
			db.Close()
			needRebuild = true
		} else {
			rep, rerr := Reconcile(db, workspaceRoot)
			if rerr != nil {
				db.Close()
				return nil, rep, rerr
			}
			return db, rep, nil
		}
	}

	rep, err := Rebuild(workspaceRoot, dbPath)
	if err != nil {
		return nil, rep, err
	}
	db, err := openDB(dbPath, false)
	if err != nil {
		return nil, rep, err
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, rep, err
	}
	return db, rep, nil
}
