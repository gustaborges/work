package workcontext

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/work"
)

// PathHint supplies an untrusted snapshot location, or empty when unavailable.
type PathHint func(context.Context, string) string

// ByID scans immediate configured lifecycle children before consulting a hint.
// IDs are opaque exact strings; they are never interpolated into file paths.
func ByID(ctx context.Context, workspace, id string, hints PathHint) (Context, error) {
	if err := cancelled(ctx); err != nil {
		return Context{}, err
	}
	var found Context
	if workspace != "" {
		for _, area := range []string{work.StatusInProgress, work.StatusArchived} {
			entries, err := os.ReadDir(filepath.Join(workspace, area))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return Context{}, unreadable(err)
			}
			for _, entry := range entries {
				if err := cancelled(ctx); err != nil {
					return Context{}, err
				}
				if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
					continue
				}
				dir := filepath.Join(workspace, area, entry.Name())
				c, err := Read(dir)
				if err != nil {
					// Decode only the identity to distinguish unrelated damaged entries.
					// This is never used as displayed or authoritative state.
					b, _ := os.ReadFile(filepath.Join(dir, "work-state.json"))
					var identity struct {
						Work struct {
							ID string `json:"id"`
						} `json:"work"`
					}
					if json.Unmarshal(b, &identity) == nil && identity.Work.ID == id {
						return Context{}, err
					}
					continue
				}
				if c.State.Work.ID != id {
					continue
				}
				if found.State != nil && found.WorkDir != c.WorkDir {
					return Context{}, unreadable(diag.New(diag.SnapshotUnreadable, "duplicate Work identity"))
				}
				found = c
			}
		}
	}
	if found.State != nil {
		return found, nil
	}
	if hints != nil {
		path := hints(ctx, id)
		if err := cancelled(ctx); err != nil {
			return Context{}, err
		}
		if filepath.IsAbs(path) && filepath.Base(path) == "work-state.json" {
			c, err := Read(filepath.Dir(path))
			if err == nil && c.State.Work.ID == id {
				return c, nil
			}
		}
	}
	return Context{}, notFound()
}
func cancelled(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return diag.Wrap(diag.Cancelled, err, "status cancelled")
	}
	return nil
}
