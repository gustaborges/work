package work

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"missing id":          func(s *State) { s.Work.ID = "" },
		"missing slug":        func(s *State) { s.Work.Slug = "" },
		"missing starter":     func(s *State) { s.Work.Starter = "" },
		"missing branch":      func(s *State) { s.Work.Branch = "" },
		"missing base_branch": func(s *State) { s.Work.BaseBranch = "" },
		"missing convention":  func(s *State) { s.Work.BranchConvention = "" },
		"missing created_at":  func(s *State) { s.Work.CreatedAt = "" },
		"bad status":          func(s *State) { s.Work.Status = "archived" },
		"bad start_mode":      func(s *State) { s.Work.StartMode = "contribution" },
		"bad timestamp":       func(s *State) { s.Work.CreatedAt = "yesterday" },
		"wrong schema":        func(s *State) { s.Schema = 2 },
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
	if out.Work != in.Work || out.Schema != 1 {
		t.Errorf("round trip mismatch: %+v vs %+v", out.Work, in.Work)
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
