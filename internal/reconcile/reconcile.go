package reconcile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/work"
)

// SkippedSnapshot records a snapshot that could not be turned into a row. It is
// a diagnostic, never fatal (FR-024).
type SkippedSnapshot struct {
	Path   string
	Reason string
}

// Report is the outcome of a Rebuild or Reconcile pass.
type Report struct {
	Indexed int               // rows written from readable snapshots
	Dropped int               // rows removed for Works no longer on disk (Reconcile only)
	Skipped []SkippedSnapshot // snapshots that could not be read/parsed/validated
	// SnapshotsWritten is always 0: the snapshots are authoritative and this
	// package never writes one (FR-023, SC-006). A test asserts it.
	SnapshotsWritten int
}

// areas maps a workspace subdirectory to whether the Works under it are
// archived.
var areas = []struct {
	dir      string
	archived bool
}{
	{"in-progress", false},
	{"archived", true},
}

// Rebuild recreates the works table from empty and indexes one row per
// readable, schema-valid snapshot found directly under
// <workspaceRoot>/in-progress/*/ and <workspaceRoot>/archived/*/. An unreadable
// snapshot is skipped and collected, never fatal. It opens and closes its own
// connection to dbPath, creating the file if absent.
func Rebuild(workspaceRoot, dbPath string) (Report, error) {
	db, err := openDB(dbPath, true)
	if err != nil {
		return Report{}, err
	}
	defer db.Close()

	if err := db.Reset(); err != nil {
		return Report{}, err
	}

	rows, skipped := scan(workspaceRoot)
	rep := Report{Skipped: skipped}
	for _, r := range rows {
		if err := db.Upsert(r); err != nil {
			return rep, fmt.Errorf("reconcile: rebuild: %w", err)
		}
		rep.Indexed++
	}
	return rep, nil
}

// Reconcile brings an existing, openable projection into agreement with the
// on-disk snapshots: every snapshot's row is upserted (snapshot values win,
// fixing a stale last_accessed_at, a wrong status, or a missing row), and every
// row whose snapshot file no longer exists is dropped. It never writes a
// snapshot.
func Reconcile(db *projection.DB, workspaceRoot string) (Report, error) {
	rows, skipped := scan(workspaceRoot)
	rep := Report{Skipped: skipped}
	for _, r := range rows {
		if err := db.Upsert(r); err != nil {
			return rep, fmt.Errorf("reconcile: %w", err)
		}
		rep.Indexed++
	}

	existing, err := db.List()
	if err != nil {
		return rep, err
	}
	for _, r := range existing {
		if _, err := os.Stat(r.SnapshotPath); errors.Is(err, fs.ErrNotExist) {
			if err := db.Delete(r.ID); err != nil {
				return rep, err
			}
			rep.Dropped++
		}
	}
	return rep, nil
}

// scan walks the two Work areas non-recursively and returns one projection row
// per readable, valid snapshot plus a skip entry for each snapshot that could
// not be read, parsed, or validated.
func scan(workspaceRoot string) (rows []projection.Work, skipped []SkippedSnapshot) {
	for _, a := range areas {
		areaDir := filepath.Join(workspaceRoot, a.dir)
		entries, err := os.ReadDir(areaDir)
		if err != nil {
			continue // a missing area is normal (e.g. nothing archived yet)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dirPath := filepath.Join(areaDir, e.Name())
			snapPath := filepath.Join(dirPath, "work-state.json")
			if _, err := os.Stat(snapPath); err != nil {
				continue // a directory without a snapshot is not our concern
			}

			snap, err := work.Read(snapPath)
			if err != nil {
				skipped = append(skipped, SkippedSnapshot{snapPath, reasonFor(err)})
				continue
			}
			if err := snap.Validate(); err != nil {
				skipped = append(skipped, SkippedSnapshot{snapPath, err.Error()})
				continue
			}

			w := snap.Work
			archived := w.Status == work.StatusArchived
			row := projection.Work{
				ID:               w.ID,
				Slug:             w.Slug,
				Status:           w.Status,
				StartMode:        w.StartMode,
				Starter:          w.Starter,
				Branch:           w.Branch,
				BaseBranch:       w.BaseBranch,
				BranchConvention: w.BranchConvention,
				RepoName:         deriveRepoName(e.Name(), w.Branch, a.archived),
				DirPath:          dirPath,
				SnapshotPath:     snapPath,
				CreatedAt:        w.CreatedAt,
				LastAccessedAt:   w.LastAccessedAt,
				ArchivedAt:       w.ArchivedAt,
			}
			if !archived {
				row.WorktreePath = filepath.Join(dirPath, "worktree")
			}
			rows = append(rows, row)
		}
	}
	return rows, skipped
}

// reasonFor turns a read error into a short skip reason.
func reasonFor(err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return "snapshot missing"
	}
	return err.Error()
}

// deriveRepoName recovers the source repository name from a Work directory name.
// F1 names an active Work directory "<repo>_<branch-with-'/'->'-'>"; an archived
// one adds a "<yyyymmdd>-" prefix and, on a same-day collision, a "-<n>"
// suffix. Knowing the branch from the snapshot lets us strip the exact suffix
// (the inverse of F1's derivation); a "prefix before the last '_'" is the
// fallback when the name does not fit that shape.
func deriveRepoName(dirBase, branch string, archived bool) string {
	name := dirBase
	if archived {
		if i := strings.IndexByte(name, '-'); i == 8 && isDigits(name[:8]) {
			name = name[i+1:]
		}
	}
	sanitized := strings.ReplaceAll(branch, "/", "-")
	if repo, ok := strings.CutSuffix(name, "_"+sanitized); ok {
		return repo
	}
	// Archived collision suffix: "<repo>_<sanitized>-<n>".
	if marker := "_" + sanitized + "-"; strings.Contains(name, marker) {
		if idx := strings.LastIndex(name, marker); idx >= 0 && isDigits(name[idx+len(marker):]) {
			return name[:idx]
		}
	}
	if i := strings.LastIndexByte(name, '_'); i >= 0 {
		return name[:i]
	}
	return name
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
