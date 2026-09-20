package repoconv

import (
	"path/filepath"
	"testing"
)

func TestGetUnknownIdentity(t *testing.T) {
	s := &Store{}
	conv, ok := s.Get("nope")
	if ok || conv != "" {
		t.Errorf("Get(unknown) = %q, %v, want \"\", false", conv, ok)
	}
}

func TestSetThenGet(t *testing.T) {
	s := &Store{}
	s.Set("repo-a", "gitflow")
	conv, ok := s.Get("repo-a")
	if !ok || conv != "gitflow" {
		t.Errorf("Get(repo-a) = %q, %v, want gitflow, true", conv, ok)
	}
}

func TestSetOverwritesExistingEntry(t *testing.T) {
	s := &Store{}
	s.Set("repo-a", "gitflow")
	s.Set("repo-a", "freeform")
	if len(s.Entries) != 1 {
		t.Fatalf("entries = %d, want 1 (Set must upsert, not append)", len(s.Entries))
	}
	conv, ok := s.Get("repo-a")
	if !ok || conv != "freeform" {
		t.Errorf("Get(repo-a) after overwrite = %q, %v, want freeform, true", conv, ok)
	}
}

func TestLoadMissingFileYieldsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "branch_conventions.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load(missing): %v", err)
	}
	if len(s.Entries) != 0 {
		t.Errorf("Load(missing) = %+v, want empty", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "branch_conventions.json")
	in := &Store{}
	in.Set("repo-a", "gitflow")
	in.Set("repo-b", "freeform")
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if conv, ok := out.Get("repo-a"); !ok || conv != "gitflow" {
		t.Errorf("round trip repo-a = %q, %v", conv, ok)
	}
	if conv, ok := out.Get("repo-b"); !ok || conv != "freeform" {
		t.Errorf("round trip repo-b = %q, %v", conv, ok)
	}
}

func TestIdempotentReloadAfterSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "branch_conventions.json")
	in := &Store{}
	in.Set("repo-a", "gitflow")
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	first, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(path, first); err != nil {
		t.Fatalf("Save (2nd, unchanged): %v", err)
	}
	second, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Entries) != 1 || second.Entries[0] != (Entry{Identity: "repo-a", Convention: "gitflow"}) {
		t.Errorf("reload after re-save drifted: %+v", second.Entries)
	}
}
