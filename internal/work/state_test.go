package work

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// schemaExample mirrors the example in contracts/work-state.schema.json.
const schemaExample = `{
  "schema": 1,
  "work": {
    "id": "01JB0K3M7Q8ZC4X2N6R9WFD5AE",
    "slug": "add-retry-logic",
    "status": "in-progress",
    "start_mode": "new",
    "starter": "local-path-starter",
    "branch": "add-retry-logic",
    "base_branch": "main",
    "branch_convention": "freeform",
    "created_at": "2026-09-05T14:03:11Z",
    "last_accessed_at": "2026-09-05T14:03:11Z"
  },
  "meta": {},
  "links": {}
}`

func TestDecodeAndValidateExample(t *testing.T) {
	s, err := Decode([]byte(schemaExample))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if s.Work.Slug != "add-retry-logic" || s.Work.BranchConvention != "freeform" {
		t.Errorf("decoded wrong: %+v", s.Work)
	}
}

func TestDecodeRejectsUnknownField(t *testing.T) {
	bad := strings.Replace(schemaExample, `"meta": {}`, `"meta": {}, "extra": 1`, 1)
	if _, err := Decode([]byte(bad)); err == nil {
		t.Fatal("Decode: want error on unknown top-level field")
	}
	badWork := strings.Replace(schemaExample, `"slug": "add-retry-logic",`, `"slug": "x", "repo_path": "/p",`, 1)
	if _, err := Decode([]byte(badWork)); err == nil {
		t.Fatal("Decode: want error on unknown work.* field")
	}
}

func TestValidateRejects(t *testing.T) {
	base := func() *State {
		s, _ := Decode([]byte(schemaExample))
		return s
	}

	cases := map[string]func(*State){
		"missing id":                          func(s *State) { s.Work.ID = "" },
		"missing slug":                        func(s *State) { s.Work.Slug = "" },
		"missing starter":                     func(s *State) { s.Work.Starter = "" },
		"missing branch":                      func(s *State) { s.Work.Branch = "" },
		"missing base_branch":                 func(s *State) { s.Work.BaseBranch = "" },
		"missing convention":                  func(s *State) { s.Work.BranchConvention = "" },
		"missing created_at":                  func(s *State) { s.Work.CreatedAt = "" },
		"unknown status":                      func(s *State) { s.Work.Status = "paused" },
		"archived on schema 1":                func(s *State) { s.Work.Status = "archived" },
		"bad start_mode":                      func(s *State) { s.Work.StartMode = "contribution" },
		"bad timestamp":                       func(s *State) { s.Work.CreatedAt = "yesterday" },
		"unsupported schema":                  func(s *State) { s.Schema = 3 },
		"archived without archived_at":        func(s *State) { s.Schema = 2; s.Work.Status = "archived" },
		"archived_at without archived status": func(s *State) { s.Schema = 2; s.Work.ArchivedAt = "2026-09-06T18:22:00Z" },
	}
	for name, mutate := range cases {
		s := base()
		mutate(s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: Validate returned nil, want error", name)
		}
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work-state.json")
	in, _ := Decode([]byte(schemaExample))
	if err := Write(path, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	// A schema-1 document is upgraded to schema 2 in place on the first write.
	if out.Work != in.Work || out.Schema != 2 {
		t.Errorf("round trip mismatch: %+v vs %+v (schema %d)", out.Work, in.Work, out.Schema)
	}

	// On-disk form is valid JSON with the closed shape.
	raw, _ := os.ReadFile(path)
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("on-disk not JSON: %v", err)
	}
	for _, k := range []string{"schema", "work", "meta", "links"} {
		if _, ok := generic[k]; !ok {
			t.Errorf("on-disk missing %q", k)
		}
	}
}

func TestDecodeSchemaRules(t *testing.T) {
	// schema 1 carrying archived_at is corrupt.
	bad := strings.Replace(schemaExample,
		`"last_accessed_at": "2026-09-05T14:03:11Z"`,
		`"last_accessed_at": "2026-09-05T14:03:11Z", "archived_at": "2026-09-06T00:00:00Z"`, 1)
	if _, err := Decode([]byte(bad)); err == nil {
		t.Error("Decode: want error on schema-1 document with archived_at")
	}

	// schema 1 that is archived is corrupt.
	badStatus := strings.Replace(schemaExample, `"status": "in-progress"`, `"status": "archived"`, 1)
	if _, err := Decode([]byte(badStatus)); err == nil {
		t.Error("Decode: want error on schema-1 archived document")
	}

	// Unsupported schema version.
	future := strings.Replace(schemaExample, `"schema": 1`, `"schema": 3`, 1)
	if _, err := Decode([]byte(future)); err == nil {
		t.Error("Decode: want error on schema 3")
	}
}

func TestArchivedRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work-state.json")
	s, _ := Decode([]byte(schemaExample))
	now := time.Date(2026, 9, 6, 18, 22, 0, 0, time.UTC)
	s.Archive(now)
	if err := Write(path, s); err != nil {
		t.Fatalf("Write(archived): %v", err)
	}
	out, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if out.Schema != 2 || out.Work.Status != StatusArchived {
		t.Fatalf("archived snapshot: schema %d status %q", out.Schema, out.Work.Status)
	}
	if out.Work.ArchivedAt != "2026-09-06T18:22:00Z" || out.Work.LastAccessedAt != "2026-09-06T18:22:00Z" {
		t.Errorf("archived timestamps: archived_at %q last_accessed_at %q", out.Work.ArchivedAt, out.Work.LastAccessedAt)
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `"archived_at"`) {
		t.Errorf("on-disk form missing archived_at:\n%s", raw)
	}
}

func TestTouchBumpsLastAccessed(t *testing.T) {
	s, _ := Decode([]byte(schemaExample))
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	s.Touch(now)
	if s.Work.LastAccessedAt != "2026-10-01T09:00:00Z" {
		t.Errorf("Touch: last_accessed_at = %q", s.Work.LastAccessedAt)
	}
	if s.Work.Status != StatusInProgress || s.Work.ArchivedAt != "" {
		t.Errorf("Touch changed more than last_accessed_at: %+v", s.Work)
	}
	if err := s.Validate(); err != nil {
		t.Errorf("Validate after Touch: %v", err)
	}
}

func TestActiveSnapshotOmitsArchivedAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work-state.json")
	s, _ := Decode([]byte(schemaExample))
	if err := Write(path, s); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "archived_at") {
		t.Errorf("active snapshot emits archived_at:\n%s", raw)
	}
}

func TestWriteRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work-state.json")
	in, _ := Decode([]byte(schemaExample))
	in.Work.Status = "archived"
	if err := Write(path, in); err == nil {
		t.Fatal("Write(invalid): want error")
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("invalid state was written to disk")
	}
}
