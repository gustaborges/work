package worklist

import (
	"time"

	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/reltime"
)

// WorkRow is the presentation model the resume and archive pickers render and
// the unit an explicit command-line target resolves to. It is derived from a
// projection row; it is not persisted.
type WorkRow struct {
	ID             string    // work.id (ULID)
	RepoName       string    // source repository name
	Slug           string    // user-chosen slug (not unique)
	Branch         string    // real branch name (may contain '/')
	Status         string    // "in-progress" | "archived"
	LastAccessedAt time.Time // parsed from the snapshot-authoritative column
	DirPath        string    // Work directory (in-progress or archived area)
	WorktreePath   string    // worktree path; "" for an archived Work
	SnapshotPath   string    // work-state.json path
	// DisplayName is "<repo>  <slug>", with "  (<id[:6]>)" appended only when
	// another row collides on (repo, slug, branch).
	DisplayName string
	// RelativeTime is a coarse "3 hours ago"-style phrase for LastAccessedAt.
	RelativeTime string
}

// Outcome is the result of resolving an explicit id.
type Outcome string

const (
	// Resolved: the id names an in-progress Work.
	Resolved Outcome = "resolved"
	// NotFound: the id matches no Work.
	NotFound Outcome = "not-found"
	// Archived: the id names an archived Work (a valid Work, but not resumable).
	Archived Outcome = "archived"
)

// List returns the Work rows for the pickers, most-recently-accessed first with
// the ULID id breaking ties (a total order). includeArchived controls whether
// archived Works are included; the resume/archive pickers pass false, an
// explicit-target resolution passes true so Resolve can tell "archived" apart
// from "not found". RelativeTime is computed against the current time.
func List(db *projection.DB, includeArchived bool) ([]WorkRow, error) {
	var (
		works []projection.Work
		err   error
	)
	if includeArchived {
		works, err = db.List()
	} else {
		works, err = db.ListActive()
	}
	if err != nil {
		return nil, err
	}
	return Rows(works, time.Now().UTC()), nil
}

// Rows builds the presentation rows from projection rows, computing relative
// times against now and appending the short-id disambiguation suffix where
// (repo, slug, branch) is not unique. The input order is preserved.
func Rows(works []projection.Work, now time.Time) []WorkRow {
	type key struct{ repo, slug, branch string }
	counts := make(map[key]int, len(works))
	for _, w := range works {
		counts[key{w.RepoName, w.Slug, w.Branch}]++
	}

	out := make([]WorkRow, 0, len(works))
	for _, w := range works {
		last, _ := time.Parse(time.RFC3339, w.LastAccessedAt)
		name := w.RepoName + "  " + w.Slug
		if counts[key{w.RepoName, w.Slug, w.Branch}] > 1 && len(w.ID) >= 6 {
			name += "  (" + w.ID[:6] + ")"
		}
		out = append(out, WorkRow{
			ID:             w.ID,
			RepoName:       w.RepoName,
			Slug:           w.Slug,
			Branch:         w.Branch,
			Status:         w.Status,
			LastAccessedAt: last,
			DirPath:        w.DirPath,
			WorktreePath:   w.WorktreePath,
			SnapshotPath:   w.SnapshotPath,
			DisplayName:    name,
			RelativeTime:   reltime.Format(now.Sub(last)),
		})
	}
	return out
}

// Resolve matches rawID against work.id only — never a slug, branch, or path
// (FR-026). rows should be the full list (List with includeArchived true) so an
// archived target is reported as Archived rather than NotFound.
func Resolve(rows []WorkRow, rawID string) (WorkRow, Outcome) {
	for _, r := range rows {
		if r.ID == rawID {
			if r.Status == "archived" {
				return r, Archived
			}
			return r, Resolved
		}
	}
	return WorkRow{}, NotFound
}
