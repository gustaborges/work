package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
)

func linkContext(t *testing.T) (workhome.Home, string, *work.State) {
	t.Helper()
	home := workhome.At(filepath.Join(t.TempDir(), "home"))
	t.Setenv("WORK_HOME", home.Root())
	reg := &registry.Registry{Components: []registry.Component{{Alias: "manual", Name: "linker", Role: plugin.RoleLinker, Key: "github.pull_request", Runtime: "missing-runtime", Entrypoint: "does-not-exist", Manual: &plugin.Manual{DisplayName: "PR"}, Inputs: []string{"meta:missing.discovery"}}}}
	if err := registry.Save(home.RegistryFile(), reg); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "in-progress", "repo_branch")
	os.MkdirAll(dir, 0755)
	s := &work.State{Work: work.WorkSection{ID: "opaque-id", Status: "in-progress", StartMode: "contribution", Starter: "test", Branch: "branch", BaseBranch: "main", CreatedAt: "2026-01-01T00:00:00Z", LastAccessedAt: "2026-01-02T00:00:00Z"}, Links: map[string]string{"other.link": "untouched", "github.pull_request": "starter"}, Meta: map[string]any{"test.number": float64(7)}}
	path := filepath.Join(dir, "work-state.json")
	if err := work.Write(path, s); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return home, path, s
}
func TestLinkDirectExactReceiptAndProvenance(t *testing.T) {
	home, path, before := linkContext(t)
	value := " exact $;\n\t\x1btext "
	cmd := newRootCmd()
	var out, ui bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&ui)
	cmd.SetArgs([]string{"link", "manual/linker", "--", value})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out.String() != "work: linked opaque-id github.pull_request manual/linker\n" || ui.Len() != 0 {
		t.Fatal(out.String(), ui.String())
	}
	after, err := work.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Links["github.pull_request"] != value || after.Links["other.link"] != "untouched" || !reflect.DeepEqual(before.Work, after.Work) || !reflect.DeepEqual(before.Meta, after.Meta) {
		t.Fatal(after)
	}
	db, err := projection.Open(home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ps, err := db.Provenance("opaque-id")
	if err != nil || len(ps) != 1 || ps[0].SourceOperation != "link" || ps[0].SourceComponent != "manual/linker" {
		t.Fatal(ps, err)
	}
	row, found, err := db.Get("opaque-id")
	if err != nil || !found || row.LastAccessedAt != before.Work.LastAccessedAt {
		t.Fatal(row, err)
	}
}
func TestLinkAutomationUsageNeverMutates(t *testing.T) {
	_, path, _ := linkContext(t)
	before, _ := os.ReadFile(path)
	for _, args := range [][]string{{"link"}, {"link", "manual/linker"}, {"link", "manual/linker", ""}, {"link", "manual/linker", "value", "--json=false"}, {"link", "manual/linker", "value", "extra"}, {"link", "manual/linker", "value", "--yes"}} {
		root := newRootCmd()
		root.SetArgs(args)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		err := root.Execute()
		if diag.ExitCode(err) != 2 || strings.Contains(out.String(), "linked") {
			t.Fatal(args, err, out.String())
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatal("usage mutated snapshot")
		}
	}
}

func TestLinkSelectedRowPreservesOtherRowsAndProvenance(t *testing.T) {
	home, path, _ := linkContext(t)
	db, err := projection.Open(home.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	other := projection.Work{ID: "unrelated", DirPath: "unrelated-dir", SnapshotPath: "unrelated-snapshot", LastAccessedAt: "2026-01-01T00:00:00Z"}
	otherSource := projection.Provenance{WorkID: other.ID, Section: "links", Key: "other.link", SourceComponent: "a/source", SourceOperation: "start", RecordedAt: "2026-01-01T00:00:00Z"}
	if err := db.UpsertWithProvenance(other, []projection.Provenance{otherSource}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"link", "manual/linker", "value"}, {"link", "manual/linker", "replacement"}} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	row, found, err := db.Get(other.ID)
	if err != nil || !found || row != other {
		t.Fatal(row, err)
	}
	ps, err := db.Provenance(other.ID)
	if err != nil || len(ps) != 1 || ps[0] != otherSource {
		t.Fatal(ps, err)
	}
	ps, err = db.Provenance("opaque-id")
	if err != nil || len(ps) != 1 || ps[0].SourceOperation != "link" {
		t.Fatal(ps, err)
	}
	s, _ := work.Read(path)
	if s.Links["github.pull_request"] != "replacement" {
		t.Fatal(s)
	}
}

func TestLinkIndexFailureIsCommittedSuccess(t *testing.T) {
	home, path, _ := linkContext(t)
	if err := os.WriteFile(home.DBFile(), []byte("corrupt index"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORK_DEBUG", "1")
	root := newRootCmd()
	var out, ui bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&ui)
	root.SetArgs([]string{"link", "manual/linker", "secret-value"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "work: linked") || !strings.Contains(ui.String(), "provenance") || strings.Contains(ui.String(), "secret-value") {
		t.Fatal(out.String(), ui.String())
	}
	s, _ := work.Read(path)
	if s.Links["github.pull_request"] != "secret-value" {
		t.Fatal(s)
	}
}
