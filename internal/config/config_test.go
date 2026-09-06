package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "work.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Workspace != "" || len(c.RepositoryRoots) != 0 || len(c.RepositoryResolution.Locators) != 0 {
		t.Errorf("default not empty: %+v", c)
	}
	if c.RepositoryRoots == nil || c.RepositoryResolution.Locators == nil {
		t.Errorf("default slices must be non-nil for shape stability: %+v", c)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "work.json")
	in := &Config{
		Workspace:            "/home/u/work",
		RepositoryRoots:      []string{"/src"},
		RepositoryResolution: RepositoryResolution{Locators: []string{"work-reference/filesystem-repository-locator"}},
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Workspace != in.Workspace ||
		len(out.RepositoryRoots) != 1 || out.RepositoryRoots[0] != "/src" ||
		len(out.RepositoryResolution.Locators) != 1 {
		t.Errorf("round trip mismatch: %+v", out)
	}
}

func TestLoadMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(malformed): want error")
	}
	if diag.ExitCode(err) != diag.BootstrapFailed.Code {
		t.Errorf("exit code = %d, want %d", diag.ExitCode(err), diag.BootstrapFailed.Code)
	}
	if got := err.Error(); !contains(got, "work.json") {
		t.Errorf("message %q does not name the file", got)
	}
}

func TestSaveIsAtomicNoPartialOnMarshalPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work.json")
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	// Overwrite with a valid new config; the directory must hold exactly one file.
	if err := Save(path, &Config{Workspace: "/x", RepositoryRoots: []string{}, RepositoryResolution: RepositoryResolution{Locators: []string{}}}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1", len(entries))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
