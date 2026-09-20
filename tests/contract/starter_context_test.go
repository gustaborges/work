package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/starter"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

// TestStarterPublishedContextContract runs the context-suite Starter through
// the same Invoke + ValidateResponse pair `work start` uses, for each shape of
// the Starter response the contract distinguishes.
func TestStarterPublishedContextContract(t *testing.T) {
	dir := fixtures.Prepare(t, "context-suite")
	comp := registry.Component{
		Alias: "context-suite", Name: "starter", Role: registry.RoleStarter, Entrypoint: "starter",
	}
	// Target resolves <pluginsDir>/<alias>/source/<entrypoint>.
	pluginsDir := t.TempDir()
	src := filepath.Join(pluginsDir, comp.Alias, "source")
	copyDir(t, dir, src)

	cases := []struct {
		mode      string
		wantErr   string // diag token; empty = valid
		wantLinks int
		wantMeta  int
	}{
		{mode: "", wantLinks: 0, wantMeta: 0},
		{mode: "starter-context", wantLinks: 1, wantMeta: 1},
		{mode: "starter-bad-key", wantErr: diag.StarterResponseInvalid.Token},
		{mode: "starter-foreign-private", wantErr: diag.StarterResponseInvalid.Token},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("WORK_FIXTURE_MODE", tc.mode)
			ref, err := starter.Invoke(pluginsDir, comp, "demo-ctx-1")
			if err != nil {
				t.Fatalf("Invoke: %v", err)
			}
			err = starter.ValidateResponse(ref, "context-suite")
			if tc.wantErr != "" {
				if diag.Token(err) != tc.wantErr {
					t.Fatalf("err = %v, want %s", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateResponse: %v", err)
			}
			if len(ref.Links) != tc.wantLinks || len(ref.Meta) != tc.wantMeta {
				t.Errorf("links=%v meta=%v", ref.Links, ref.Meta)
			}
		})
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
