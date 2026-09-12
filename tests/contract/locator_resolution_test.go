// Core-side resolution contract: pins the projected ipc.LocatorInput payload
// for every reference shape in
// specs/004-local-clone-locator/contracts/repository-reference.md §Tests, and
// the outcome table in
// specs/004-local-clone-locator/contracts/repository-locator.md
// §"Outcome → exit code" against fake Locators
// (tests/fixtures/locators/locator_test.go stays green and untouched).
package contract

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/locator"
	"github.com/gustaborges/work/internal/registry"
	fixtures "github.com/gustaborges/work/tests/fixtures/locators"
)

func wantLocatorToken(t *testing.T, err error, c diag.Category) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error with token %q, got nil", c.Token)
	}
	if got := diag.Token(err); got != c.Token {
		t.Fatalf("token = %q, want %q (%v)", got, c.Token, err)
	}
}

// TestLocatorResolutionProjection pins the serialised LocatorInput.repository
// / repository_roots for every reference shape in
// contracts/repository-reference.md §Tests: only the accepted-and-present
// fields reach the Locator, plus repository_roots verbatim — never the raw
// argument, base_branch, start_modes, meta, or links (which is structurally
// guaranteed: ipc.RepositoryReference has no such fields at all). The "ok"
// fixture self-checks the payload it actually received and exits non-zero on
// any mismatch (see tests/fixtures/locators/ok/main.go).
func TestLocatorResolutionProjection(t *testing.T) {
	roots := []string{"/roots/a", "/roots/b"}

	cases := []struct {
		name     string
		accepts  []string
		ref      locator.Reference
		wantRepo map[string]any
		wantElig bool // false => never invoked; assert no-eligible-locator
	}{
		{
			name:     "name only, locator accepts name",
			accepts:  []string{"name"},
			ref:      locator.Reference{Name: "payments"},
			wantRepo: map[string]any{"name": "payments"},
			wantElig: true,
		},
		{
			name:     "git_fetch_urls only, locator accepts git_fetch_urls",
			accepts:  []string{"git_fetch_urls"},
			ref:      locator.Reference{GitFetchURLs: []string{"https://example.com/p.git", "git@example.com:p.git"}},
			wantRepo: map[string]any{"git_fetch_urls": []any{"https://example.com/p.git", "git@example.com:p.git"}},
			wantElig: true,
		},
		{
			name:     "query only, locator accepts query",
			accepts:  []string{"query"},
			ref:      locator.Reference{Query: "pay"},
			wantRepo: map[string]any{"query": "pay"},
			wantElig: true,
		},
		{
			name:     "mixed reference, locator accepts only name: query is present but not projected",
			accepts:  []string{"name"},
			ref:      locator.Reference{Name: "payments", Query: "pay"},
			wantRepo: map[string]any{"name": "payments"},
			wantElig: true,
		},
		{
			name:     "accepted field absent on the reference is omitted",
			accepts:  []string{"name", "query"},
			ref:      locator.Reference{Name: "payments"},
			wantRepo: map[string]any{"name": "payments"},
			wantElig: true,
		},
		{
			name:     "git_fetch_urls present, locator accepts name only: ineligible, never invoked",
			accepts:  []string{"name"},
			ref:      locator.Reference{GitFetchURLs: []string{"https://example.com/x.git"}},
			wantElig: false,
		},
		{
			name:     "no field at all",
			accepts:  []string{"name", "git_fetch_urls", "query"},
			ref:      locator.Reference{},
			wantElig: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plugins := t.TempDir()
			reg := &registry.Registry{}
			fixtures.Install(t, reg, plugins, "fixture", "ok", "ok", tc.accepts)
			d := locator.Deps{PluginsDir: plugins, Policy: []string{"fixture/ok"}, Roots: roots, Registry: reg}

			if !tc.wantElig {
				_, err := locator.Resolve(context.Background(), d, tc.ref)
				wantLocatorToken(t, err, diag.NoEligibleLocator)
				return
			}

			repoJSON, err := json.Marshal(tc.wantRepo)
			if err != nil {
				t.Fatal(err)
			}
			rootsJSON, err := json.Marshal(roots)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("LOCATOR_FIXTURE_EXPECT_REPOSITORY", string(repoJSON))
			t.Setenv("LOCATOR_FIXTURE_EXPECT_ROOTS", string(rootsJSON))
			t.Setenv("LOCATOR_FIXTURE_MATCHES", "[]") // matches:[] is enough; only the payload matters here

			_, err = locator.Resolve(context.Background(), d, tc.ref)
			// A payload mismatch makes the fixture exit 1, which Resolve
			// reports as locator-failed — any other outcome here proves the
			// projection matched what the fixture expected.
			if diag.Token(err) == diag.LocatorFailed.Token {
				t.Fatalf("projected payload did not match contract: %v", err)
			}
		})
	}
}

// TestLocatorResolutionOutcomeTable exercises every row of
// contracts/repository-locator.md §"Outcome → exit code" through the real
// Resolve pipeline against fake Locators.
func TestLocatorResolutionOutcomeTable(t *testing.T) {
	t.Run("one valid repo -> resolved", func(t *testing.T) {
		repo := makeRepo(t, t.TempDir(), "project")
		t.Setenv("LOCATOR_FIXTURE_MATCHES", mustJSON(t, []string{repo}))
		plugins := t.TempDir()
		reg := &registry.Registry{}
		fixtures.Install(t, reg, plugins, "fixture", "ok", "ok", []string{"name"})
		d := locator.Deps{PluginsDir: plugins, Policy: []string{"fixture/ok"}, Registry: reg}

		out, err := locator.Resolve(context.Background(), d, locator.Reference{Name: "project"})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if out.Resolved == "" {
			t.Fatal("want Resolved set")
		}
	})

	t.Run(">=2 valid repos -> ambiguous", func(t *testing.T) {
		root := t.TempDir()
		a := makeRepo(t, root, "a")
		b := makeRepo(t, root, "b")
		t.Setenv("LOCATOR_FIXTURE_MATCHES", mustJSON(t, []string{a, b}))
		plugins := t.TempDir()
		reg := &registry.Registry{}
		fixtures.Install(t, reg, plugins, "fixture", "two", "two", []string{"name"})
		d := locator.Deps{PluginsDir: plugins, Policy: []string{"fixture/two"}, Registry: reg}

		out, err := locator.Resolve(context.Background(), d, locator.Reference{Name: "project"})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(out.Candidates) < 2 {
			t.Fatalf("Candidates = %v, want >= 2", out.Candidates)
		}
	})

	t.Run("every eligible locator returned [] -> no-repository-found (26)", func(t *testing.T) {
		plugins := t.TempDir()
		reg := &registry.Registry{}
		fixtures.Install(t, reg, plugins, "fixture", "empty", "empty", []string{"name"})
		d := locator.Deps{PluginsDir: plugins, Policy: []string{"fixture/empty"}, Registry: reg}

		_, err := locator.Resolve(context.Background(), d, locator.Reference{Name: "project"})
		wantLocatorToken(t, err, diag.NoRepositoryFound)
	})

	t.Run("empty policy -> no-eligible-locator (27)", func(t *testing.T) {
		d := locator.Deps{Registry: &registry.Registry{}}
		_, err := locator.Resolve(context.Background(), d, locator.Reference{Name: "project"})
		wantLocatorToken(t, err, diag.NoEligibleLocator)
	})

	t.Run("ending locator's candidates all invalid -> repository-candidate-invalid (28)", func(t *testing.T) {
		notARepo := t.TempDir()
		t.Setenv("LOCATOR_FIXTURE_MATCHES", mustJSON(t, []string{notARepo}))
		plugins := t.TempDir()
		reg := &registry.Registry{}
		fixtures.Install(t, reg, plugins, "fixture", "invalid", "invalid", []string{"name"})
		d := locator.Deps{PluginsDir: plugins, Policy: []string{"fixture/invalid"}, Registry: reg}

		_, err := locator.Resolve(context.Background(), d, locator.Reference{Name: "project"})
		wantLocatorToken(t, err, diag.RepositoryCandidateInvalid)
	})

	t.Run("a locator errored -> locator-failed (29), no fallback", func(t *testing.T) {
		repo := makeRepo(t, t.TempDir(), "project")
		t.Setenv("LOCATOR_FIXTURE_MATCHES", mustJSON(t, []string{repo}))
		plugins := t.TempDir()
		reg := &registry.Registry{}
		fixtures.Install(t, reg, plugins, "fixtureA", "boom", "boom", []string{"name"})
		fixtures.Install(t, reg, plugins, "fixtureB", "ok", "ok", []string{"name"})
		d := locator.Deps{PluginsDir: plugins, Policy: []string{"fixtureA/boom", "fixtureB/ok"}, Registry: reg}

		_, err := locator.Resolve(context.Background(), d, locator.Reference{Name: "project"})
		wantLocatorToken(t, err, diag.LocatorFailed)
	})
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
