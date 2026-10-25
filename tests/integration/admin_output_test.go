package integration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestAdminOutputMixedBatchAndReceipts(t *testing.T) {
	h := newAdminHarness(t)
	home := workhome.At(adminHome(h))
	h.run(t, h.root, 0, "plugin", "install", "--embedded")
	good, bad := fixtures.Prepare(t, "admin-suite"), fixtures.Prepare(t, "admin-suite")
	for _, test := range []struct{ alias, source string }{{"good", good}, {"bad", bad}} {
		out, _ := h.run(t, h.root, 0, "plugin", "install", test.source, "--as", test.alias)
		if !strings.HasPrefix(out, "work: installed "+test.alias+" (local-pinned)\n") {
			t.Fatal(out)
		}
	}
	fixtures.Revise(t, good, "green\n\x1b[31m", nil)
	if err := os.WriteFile(filepath.Join(bad, "plugin.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	before := adminTree(t, home.Root())
	out, ui := h.run(t, h.root, 50, "plugin", "update", "--check", "--json")
	var data struct {
		Updates []pluginadmin.UpdateResult `json:"updates"`
	}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatal(err, out)
	}
	if len(data.Updates) != 3 || data.Updates[0].Alias != "bad" || data.Updates[0].Status != "failed" || data.Updates[1].Alias != "good" || data.Updates[1].Status != "update-available" || data.Updates[2].Alias != "work-reference" || data.Updates[2].Status != "not-applicable" {
		t.Fatal(out)
	}
	if strings.Count(ui, "error: plugin-update-failed:") != 1 || strings.ContainsAny(out+ui, "\x1b\t") {
		t.Fatal(out, ui)
	}
	if !reflect.DeepEqual(before, adminTree(t, home.Root())) {
		t.Fatal("failed batch check mutated home")
	}
	out, ui = h.run(t, h.root, 50, "plugin", "update", "--all", "--yes")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 || lines[0] != "work: update bad failed" || !strings.HasPrefix(lines[1], "work: update-error bad ") || lines[2] != "work: update good updated" || lines[3] != "work: update work-reference not-applicable" || strings.Count(ui, "error:") != 1 || strings.ContainsAny(out+ui, "\x1b\t") {
		t.Fatal(out, ui)
	}
	out, ui = h.run(t, h.root, 0, "plugin", "disable", "good", "good")
	if out != "work: disabled good\n" || ui != "" {
		t.Fatal(out, ui)
	}
	out, ui = h.run(t, h.root, 0, "plugin", "enable", "good")
	if out != "work: enabled good\n" || ui != "" {
		t.Fatal(out, ui)
	}
}

func TestAdminOutputCommittedCleanup(t *testing.T) {
	h := newAdminHarness(t)
	home := workhome.At(adminHome(h))
	source := fixtures.Prepare(t, "admin-suite")
	h.run(t, h.root, 0, "plugin", "install", source, "--as", "a")
	fixtures.Revise(t, source, "green", nil)
	plan, err := pluginadmin.PrepareUpdates(context.Background(), home, []string{"a"}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	rows, err := pluginadmin.ApplyWithOptions(context.Background(), home, plan, false, adminstate.WriteOptions{Checkpoint: func(point string) error {
		if point == "cleanup" {
			return errors.New("private cleanup cause")
		}
		return nil
	}})
	public := diag.Format(err)
	if diag.ExitCode(err) != 55 || len(rows) != 1 || rows[0].Status != "updated" || !strings.Contains(public, "committed") || strings.Contains(public, "private cleanup cause") {
		t.Fatal(rows, err, public)
	}
	// A repeat mutation completes cleanup without undoing the committed update.
	_, ui := h.run(t, h.root, 0, "plugin", "enable", "a")
	if !strings.Contains(ui, "recovered interrupted administrative operation") {
		t.Fatal(ui)
	}
	s, err := adminstate.Read(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Registry.PackageByAlias("a")
	if p.Version != "green" {
		t.Fatal(p)
	}
}
