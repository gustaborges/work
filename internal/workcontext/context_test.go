package workcontext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/work"
)

func snapshot(t *testing.T, dir, id string, schema int, archived bool) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := work.State{Schema: schema, Work: work.WorkSection{ID: id, Slug: "topic", Status: work.StatusInProgress, StartMode: "new", Starter: "test", Branch: "topic", BaseBranch: "main", BranchConvention: "freeform", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-01T00:00:00Z"}, Meta: map[string]any{}, Links: map[string]string{}}
	if archived {
		s.Work.Status = work.StatusArchived
		s.Work.ArchivedAt = s.Work.CreatedAt
	}
	b, _ := json.Marshal(s)
	p := filepath.Join(dir, "work-state.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestCurrent(t *testing.T) {
	for _, tc := range []struct {
		schema     int
		area, desc string
	}{{1, "in-progress", ""}, {2, "archived", "notes"}, {3, "in-progress", "worktree/src"}, {3, "in-progress", "notes"}} {
		t.Run(tc.area+tc.desc+string(rune('0'+tc.schema)), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), tc.area, "work")
			snapshot(t, dir, "exact/id", tc.schema, tc.area == "archived")
			cwd := filepath.Join(dir, tc.desc)
			os.MkdirAll(cwd, 0o755)
			if tc.desc != "" {
				os.WriteFile(filepath.Join(cwd, "work-state.json"), []byte("broken"), 0o644)
			}
			c, err := Current(cwd)
			if err != nil {
				t.Fatal(err)
			}
			real, _ := filepath.EvalSymlinks(dir)
			if c.State.Work.ID != "exact/id" || c.WorkDir != real {
				t.Fatalf("%+v", c)
			}
			if tc.area == "archived" && c.WorktreePath != "" {
				t.Fatal(c.WorktreePath)
			}
		})
	}
}
func TestCurrentInvalidStopsAtNearest(t *testing.T) {
	outer := filepath.Join(t.TempDir(), "in-progress", "outer")
	snapshot(t, outer, "outer", 3, false)
	inner := filepath.Join(outer, "in-progress", "inner")
	p := snapshot(t, inner, "inner", 3, false)
	os.WriteFile(p, []byte(`{"schema":3,"work":{"id":"inner"}}`), 0o644)
	_, err := Current(inner)
	if diag.ExitCode(err) != 25 {
		t.Fatalf("%v", err)
	}
}
func TestCurrentSymlinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "in-progress", "work")
	p := snapshot(t, dir, "id", 3, false)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Skip(err)
	}
	if _, err := Current(alias); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "snapshot")
	b, _ := os.ReadFile(p)
	os.WriteFile(target, b, 0o644)
	os.Remove(p)
	os.Symlink(target, p)
	if _, err := Current(dir); diag.ExitCode(err) != 25 {
		t.Fatalf("%v", err)
	}
}
func TestFullValidation(t *testing.T) {
	for _, bad := range []string{`{"schema":4}`, `{"schema":3,"work":{"id":"id"}}`, `not json`} {
		dir := filepath.Join(t.TempDir(), "in-progress", "work")
		p := snapshot(t, dir, "id", 3, false)
		os.WriteFile(p, []byte(bad), 0o644)
		if _, err := Current(dir); diag.ExitCode(err) != 25 {
			t.Fatalf("%v", err)
		}
	}
}

func TestCanonicalRequiredFieldsAndDates(t *testing.T) {
	for _, field := range []string{"id", "status", "start_mode", "starter", "branch", "base_branch", "created_at", "last_accessed_at", "slug", "branch_convention"} {
		t.Run(field, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "in-progress", "w")
			p := snapshot(t, dir, "id", 3, false)
			b, _ := os.ReadFile(p)
			var v map[string]any
			json.Unmarshal(b, &v)
			delete(v["work"].(map[string]any), field)
			b, _ = json.Marshal(v)
			os.WriteFile(p, b, 0o600)
			if _, err := Current(dir); diag.ExitCode(err) != 25 {
				t.Fatal(err)
			}
		})
	}
	for _, field := range []string{"created_at", "last_accessed_at", "status", "start_mode"} {
		dir := filepath.Join(t.TempDir(), "in-progress", "w")
		p := snapshot(t, dir, "id", 3, false)
		b, _ := os.ReadFile(p)
		var v map[string]any
		json.Unmarshal(b, &v)
		v["work"].(map[string]any)[field] = "invalid"
		b, _ = json.Marshal(v)
		os.WriteFile(p, b, 0o600)
		if _, err := Current(dir); diag.ExitCode(err) != 25 {
			t.Fatalf("%s: %v", field, err)
		}
	}
}
