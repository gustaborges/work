package integration

import (
	"context"
	"encoding/json"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAdminBatchOutcomesAndAuthorization(t *testing.T) {
	h := newAdminHarness(t)
	a := fixtures.Prepare(t, "admin-suite")
	b := fixtures.Prepare(t, "admin-suite")
	h.run(t, h.root, 0, "plugin", "install", a, "--as", "a")
	h.run(t, h.root, 0, "plugin", "install", b, "--as", "b")
	fixtures.Revise(t, a, "green", nil)
	_ = os.WriteFile(filepath.Join(b, "plugin.json"), []byte("invalid"), 0600)
	home := workhome.At(adminHome(h))
	before := adminTree(t, home.Root())
	h.run(t, h.root, 2, "plugin", "update", "a", "b")
	h.run(t, h.root, 2, "plugin", "update", "a", "missing", "--yes")
	if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
		t.Fatal("unconfirmed update changed home")
	}
	out, ui := h.run(t, h.root, 50, "plugin", "update", "--check", "--json")
	var data struct{ Updates []pluginadmin.UpdateResult }
	if json.Unmarshal([]byte(out), &data) != nil || len(data.Updates) != 2 || strings.Count(ui, "error:") != 1 {
		t.Fatal(out, ui)
	}
	out, ui = h.run(t, h.root, 50, "plugin", "update", "--all", "--yes")
	if !strings.Contains(out, "work: update a updated") || !strings.Contains(out, "work: update b failed") || strings.Count(ui, "error:") != 1 {
		t.Fatal(out, ui)
	}
	fixtures.Revise(t, a, "withdrawn", func(m map[string]any) { m["components"] = []any{} })
	h.run(t, h.root, 0, "repository", "policy", "add", "a/clone")
	before = adminTree(t, home.Root())
	h.run(t, h.root, 52, "plugin", "update", "a", "--yes")
	if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
		t.Fatal("dependency refusal changed home")
	}
	h.run(t, h.root, 0, "plugin", "update", "a", "--yes", "--remove-policy-references")
}
func TestAdminBatchCancellationAndRecovery(t *testing.T) {
	for _, kind := range []string{"cancel", "cleanup"} {
		t.Run(kind, func(t *testing.T) {
			h := newAdminHarness(t)
			source := fixtures.Prepare(t, "admin-suite")
			for _, a := range []string{"a", "b"} {
				h.run(t, h.root, 0, "plugin", "install", source, "--as", a)
			}
			fixtures.Revise(t, source, "green", nil)
			home := workhome.At(adminHome(h))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			plan, err := pluginadmin.PrepareUpdates(ctx, home, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			rows, err := pluginadmin.ApplyWithOptions(ctx, home, plan, false, adminstate.WriteOptions{Checkpoint: func(point string) error {
				if point == "cleanup" {
					if kind == "cancel" {
						cancel()
					} else {
						return diag.New(diag.AdministrationCleanupFailed, "injected cleanup")
					}
				}
				return nil
			}})
			expected := 20
			if kind == "cleanup" {
				expected = 55
			}
			if diag.ExitCode(err) != expected || len(rows) != 2 || rows[0].Status != "updated" || rows[1].Status != "not-applied" {
				t.Fatal(rows, err)
			}
			if _, err = adminstate.Preflight(context.Background(), home); err != nil {
				t.Fatal(err)
			}
			s, err := adminstate.Read(context.Background(), home)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := s.Registry.PackageByAlias("a")
			b, _ := s.Registry.PackageByAlias("b")
			if a.Version != "green" || b.Version != "release-blue" {
				t.Fatal(a, b)
			}
		})
	}
}
