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
		Schema:               Schema,
		Workspace:            "/home/u/work",
		RepositoryRoots:      []string{"/src"},
		RepositoryResolution: RepositoryResolution{Locators: []string{"work-reference@builtin/filesystem-repository-locator"}},
		Marketplaces:         []Marketplace{{RegistrationID: "0123456789abcdef0123456789abcdef", Name: "team", Kind: KindGit, Source: "https://example.org/t.git", Branch: "main"}},
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
	if err := Save(path, &Config{Schema: Schema, Workspace: "/x", RepositoryRoots: []string{}, RepositoryResolution: RepositoryResolution{Locators: []string{}}}); err != nil {
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

func TestStrictCurrentSchema(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	valid := `{"schema":1,"workspace":"","repository_roots":[],"repository_resolution":{"locators":[]},"marketplaces":[]}`
	if _, err := Decode([]byte(valid)); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := map[string]string{
		"missing schema":    `{"workspace":"","repository_roots":[],"repository_resolution":{"locators":[]}}`,
		"future schema":     `{"schema":2,"workspace":""}`,
		"unknown field":     `{"schema":1,"workspace":"","extra":true}`,
		"trailing":          valid + `{}`,
		"reserved name":     `{"schema":1,"marketplaces":[{"registration_id":"` + id + `","name":"BuiltIn","kind":"local","source":"/x"}]}`,
		"builtin id":        `{"schema":1,"marketplaces":[{"registration_id":"builtin","name":"team","kind":"local","source":"/x"}]}`,
		"bad kind":          `{"schema":1,"marketplaces":[{"registration_id":"` + id + `","name":"team","kind":"svn","source":"/x"}]}`,
		"git without brach": `{"schema":1,"marketplaces":[{"registration_id":"` + id + `","name":"team","kind":"git","source":"/x"}]}`,
		"local with branch": `{"schema":1,"marketplaces":[{"registration_id":"` + id + `","name":"team","kind":"local","source":"/x","branch":"main"}]}`,
		"case duplicate":    `{"schema":1,"marketplaces":[{"registration_id":"` + id + `","name":"team","kind":"local","source":"/x"},{"registration_id":"fedcba9876543210fedcba9876543210","name":"TEAM","kind":"local","source":"/y"}]}`,
		"id duplicate":      `{"schema":1,"marketplaces":[{"registration_id":"` + id + `","name":"a","kind":"local","source":"/x"},{"registration_id":"` + id + `","name":"b","kind":"local","source":"/y"}]}`,
	}
	for name, doc := range cases {
		if _, err := Decode([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	path := filepath.Join(t.TempDir(), "work.json")
	if err := os.WriteFile(path, []byte(cases["unknown field"]), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); diag.ExitCode(err) != diag.BootstrapFailed.Code {
		t.Errorf("Load(unknown field) exit = %d", diag.ExitCode(err))
	}
}
