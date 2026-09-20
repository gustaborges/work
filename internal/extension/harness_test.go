package extension

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/internal/workhome"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

type recorder struct{ events []Event }

func (r *recorder) OnEvent(e Event) { r.events = append(r.events, e) }

func (r *recorder) kinds() []EventKind {
	var out []EventKind
	for _, e := range r.events {
		out = append(out, e.Kind)
	}
	return out
}

type fakeIndex struct {
	entries []projection.Provenance
	err     error
}

func (f *fakeIndex) RecordProvenance(e ...projection.Provenance) error {
	if f.err != nil {
		return f.err
	}
	f.entries = append(f.entries, e...)
	return nil
}

// harness is a Work home with context-suite installed and one committed Work
// whose snapshot lives in a real directory.
type harness struct {
	t   *testing.T
	ctx Context
	rec *recorder
	idx *fakeIndex
	log string
}

// newHarness installs the context-suite fixture, writes a Work created by its
// Starter, and selects the fixture modes ("linker-value,importer-ok", …).
func newHarness(t *testing.T, mode string) *harness {
	t.Helper()
	home := workhome.At(filepath.Join(t.TempDir(), "dotwork"))
	if err := home.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	reg := &registry.Registry{}
	if _, err := plugininstall.Install(home.PluginsDir(), reg, fixtures.Prepare(t, "context-suite"), plugininstall.Options{}); err != nil {
		t.Fatalf("install context-suite: %v", err)
	}
	starter, ok := findComponent(reg, "starter")
	if !ok {
		t.Fatal("context-suite starter not registered")
	}

	workDir := filepath.Join(t.TempDir(), "in-progress", "repo_branch")
	if err := os.MkdirAll(filepath.Join(workDir, "worktree"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := &work.State{
		Work: work.WorkSection{
			ID: "01J9TESTWORK0000000000000A", Slug: "s", Status: work.StatusInProgress, StartMode: work.StartModeNew,
			Starter: "starter", Branch: "s", BaseBranch: "main", BranchConvention: "freeform",
			CreatedAt: "2026-02-03T04:05:06Z", LastAccessedAt: "2026-02-03T04:05:06Z",
		},
		Meta:  map[string]any{},
		Links: map[string]string{},
	}
	snapshot := filepath.Join(workDir, "work-state.json")
	if err := work.Write(snapshot, st); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(t.TempDir(), "launches.log")
	t.Setenv("WORK_FIXTURE_MODE", mode)
	t.Setenv("WORK_FIXTURE_LOG", logPath)

	h := &harness{t: t, rec: &recorder{}, idx: &fakeIndex{}, log: logPath}
	h.ctx = Context{
		Home: home, Registry: reg, State: st, Starter: starter,
		WorktreePath: filepath.Join(workDir, "worktree"), WorkDir: workDir, SnapshotPath: snapshot,
		Index: h.idx, Observer: h.rec,
	}
	return h
}

func findComponent(reg *registry.Registry, name string) (registry.Component, bool) {
	for _, c := range reg.Components {
		if c.Name == name {
			return c, true
		}
	}
	return registry.Component{}, false
}

func (h *harness) snapshot() *work.State {
	h.t.Helper()
	st, err := work.Read(h.ctx.SnapshotPath)
	if err != nil {
		h.t.Fatal(err)
	}
	return st
}

// launches returns what each launched fixture component received, in order.
func (h *harness) launches() []struct {
	Role  string
	Stdin map[string]any
} {
	h.t.Helper()
	f, err := os.Open(h.log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	var out []struct {
		Role  string
		Stdin map[string]any
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var l struct {
			Role  string
			Stdin map[string]any
		}
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			h.t.Fatal(err)
		}
		out = append(out, l)
	}
	return out
}

func workWrite(h *harness, st *work.State) error { return work.Write(h.ctx.SnapshotPath, st) }
