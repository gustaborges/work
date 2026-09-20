package extension

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

func TestLinkerValueIsStoredWithProvenance(t *testing.T) {
	h := newHarness(t, "linker-value")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 0 {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	if got := h.snapshot().Links["github.pull_request"]; got != "https://example.test/pr/212-from-linker" {
		t.Errorf("link = %q", got)
	}
	if len(h.idx.entries) != 1 {
		t.Fatalf("provenance = %+v", h.idx.entries)
	}
	p := h.idx.entries[0]
	if p.Section != "links" || p.Key != "github.pull_request" || p.SourceComponent != "context-suite/linker" ||
		p.SourceOperation != "discover" || p.WorkID != h.ctx.State.Work.ID || p.RecordedAt == "" {
		t.Errorf("provenance = %+v", p)
	}
	// linker2 stays silent by default, so it only produces a Running.
	if !slices.Equal(h.rec.kinds(), []EventKind{Running, Linked, Running}) {
		t.Errorf("events = %v", h.rec.kinds())
	}
	linked := h.rec.events[1]
	if linked.Key != "github.pull_request" || linked.Operation != OpDiscover {
		t.Errorf("linked event = %+v", linked)
	}
}

func TestLinkerReceivesOnlyDeclaredInputs(t *testing.T) {
	h := newHarness(t, "linker-value")
	Run(context.Background(), h.ctx)

	ls := h.launches()
	if len(ls) == 0 || ls[0].Role != "linker" {
		t.Fatalf("launches = %+v", ls)
	}
	inputs, _ := ls[0].Stdin["inputs"].(map[string]any)
	if len(ls[0].Stdin) != 1 || len(inputs) != 1 || inputs["worktree_path"] != h.ctx.WorktreePath {
		t.Errorf("stdin = %v, want only inputs.worktree_path", ls[0].Stdin)
	}
}

func TestLinkerWithoutValueIsSilent(t *testing.T) {
	h := newHarness(t, "linker-none,linker2-none")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 0 || len(h.idx.entries) != 0 {
		t.Errorf("warnings=%+v provenance=%+v", rep.Warnings, h.idx.entries)
	}
	if len(h.snapshot().Links) != 0 {
		t.Errorf("links = %v", h.snapshot().Links)
	}
	for _, e := range h.rec.events {
		if e.Kind == Linked || e.Kind == Warned {
			t.Errorf("unexpected event %+v", e)
		}
	}
}

func TestLinkerEmptyValueIsAnInvalidResponse(t *testing.T) {
	h := newHarness(t, "linker-empty-value")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 1 || rep.Warnings[0].Token != diag.WarnExtensionResponseInvalid {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	if len(h.snapshot().Links) != 0 || len(h.idx.entries) != 0 {
		t.Errorf("something was persisted: %v %v", h.snapshot().Links, h.idx.entries)
	}
}

func TestSameKeyLinkersLastInOrderWins(t *testing.T) {
	h := newHarness(t, "linker-value,linker2-value")
	Run(context.Background(), h.ctx)

	if got := h.snapshot().Links["github.pull_request"]; got != "https://example.test/pr/212-from-linker2" {
		t.Errorf("link = %q, want linker2's value", got)
	}
	if n := len(h.idx.entries); n != 2 || h.idx.entries[1].SourceComponent != "context-suite/linker2" {
		t.Errorf("provenance = %+v", h.idx.entries)
	}
}

func TestLinkerValueReplacesStarterPublishedLink(t *testing.T) {
	h := newHarness(t, "linker-value")
	h.ctx.State.Links["github.pull_request"] = "https://example.test/from-starter"
	st := h.snapshot()
	st.Links["github.pull_request"] = "https://example.test/from-starter"
	if err := workWrite(h, st); err != nil {
		t.Fatal(err)
	}
	Run(context.Background(), h.ctx)

	if got := h.snapshot().Links["github.pull_request"]; got != "https://example.test/pr/212-from-linker" {
		t.Errorf("link = %q", got)
	}
}

func TestPersistFailureIsAWarningAndKeepsEarlierResults(t *testing.T) {
	h := newHarness(t, "linker-value,linker2-value")
	h.ctx.SnapshotPath = filepath.Join(h.ctx.WorkDir, "missing", "work-state.json")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 2 {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	for _, w := range rep.Warnings {
		if w.Token != diag.WarnExtensionPersistFailed || w.Operation != OpDiscover || w.Work == "" {
			t.Errorf("warning = %+v", w)
		}
	}
	for _, e := range h.rec.events {
		if e.Kind == Linked {
			t.Errorf("a link was reported although nothing persisted: %+v", e)
		}
	}
}

func TestProvenanceFailureDoesNotFailTheLinker(t *testing.T) {
	h := newHarness(t, "linker-value")
	h.idx.err = errors.New("disk full")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 0 {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	if h.snapshot().Links["github.pull_request"] == "" {
		t.Error("link was not persisted")
	}
	if len(rep.Diagnostics) != 1 {
		t.Errorf("diagnostics = %v, want one debug note", rep.Diagnostics)
	}
	var sawLinked bool
	for _, e := range h.rec.events {
		sawLinked = sawLinked || e.Kind == Linked
	}
	if !sawLinked {
		t.Error("no Linked event")
	}
}

func TestRunOrderAndResultAreIdenticalAcrossRepeats(t *testing.T) {
	h := newHarness(t, "linker-value,linker2-value")
	fresh := h.snapshot()

	for i := range 20 {
		if err := workWrite(h, fresh); err != nil {
			t.Fatal(err)
		}
		h.rec.events, h.idx.entries = nil, nil
		Run(context.Background(), h.ctx)

		var order []string
		for _, e := range h.rec.events {
			if e.Kind == Running {
				order = append(order, e.Component)
			}
		}
		if !slices.Equal(order, []string{"context-suite/linker", "context-suite/linker2"}) {
			t.Fatalf("run %d: order = %v", i, order)
		}
		if got := h.snapshot().Links["github.pull_request"]; got != "https://example.test/pr/212-from-linker2" {
			t.Fatalf("run %d: link = %q", i, got)
		}
		if n := len(h.idx.entries); n != 2 || h.idx.entries[1].SourceComponent != "context-suite/linker2" {
			t.Fatalf("run %d: provenance = %+v", i, h.idx.entries)
		}
	}
}
