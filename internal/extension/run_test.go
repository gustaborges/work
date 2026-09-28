package extension

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gustaborges/work/internal/diag"
)

func TestFailureClassesMapToStableWarnings(t *testing.T) {
	cases := []struct {
		mode      string
		op        string
		component string
		token     string
	}{
		{"linker-exit1,linker2-none", OpDiscover, "context-suite/linker", diag.WarnExtensionFailed},
		{"linker-garbage,linker2-none", OpDiscover, "context-suite/linker", diag.WarnExtensionResponseInvalid},
		{"linker-empty-value,linker2-none", OpDiscover, "context-suite/linker", diag.WarnExtensionResponseInvalid},
		{"linker-value,importer-exit1,importer2-empty", OpImport, "context-suite/importer", diag.WarnExtensionFailed},
		{"linker-value,importer-garbage,importer2-empty", OpImport, "context-suite/importer", diag.WarnExtensionResponseInvalid},
		{"linker-value,importer-mixed,importer2-empty", OpImport, "context-suite/importer", diag.WarnExtensionOutputRefused},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			h := newHarness(t, tc.mode)
			rep := Run(context.Background(), h.ctx)

			if len(rep.Warnings) != 1 {
				t.Fatalf("warnings = %+v, want exactly one", rep.Warnings)
			}
			w := rep.Warnings[0]
			if w.Token != tc.token || w.Component != tc.component || w.Operation != tc.op || w.Work != h.ctx.State.Work.ID {
				t.Errorf("warning = %+v", w)
			}
			if w.Summary == "" {
				t.Error("warning has no summary")
			}
			if strings.Contains(w.Summary+w.Hint, "example.test") {
				t.Errorf("warning leaks a link value: %q %q", w.Summary, w.Hint)
			}
			if _, err := os.Stat(h.ctx.SnapshotPath); err != nil {
				t.Errorf("the Work's snapshot is gone: %v", err)
			}
		})
	}
}

func TestStartFailureWhenTheEntrypointIsMissing(t *testing.T) {
	h := newHarness(t, "linker2-none")
	entry := filepath.Join(h.ctx.Home.PluginsDir(), "context-suite", "source", "linker")
	if runtime.GOOS == "windows" {
		entry += ".exe"
	}
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 1 || rep.Warnings[0].Token != diag.WarnExtensionStartFailed {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	if rep.Warnings[0].Component != "context-suite/linker" {
		t.Errorf("component = %q", rep.Warnings[0].Component)
	}
}

func TestFailedLinkerMakesDependentImporterIneligibleWithoutASecondWarning(t *testing.T) {
	h := newHarness(t, "linker-exit1,linker2-none,importer-ok")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want just the Linker's", rep.Warnings)
	}
	for _, l := range h.launches() {
		if l.Role == "importer" {
			t.Error("the dependent Importer ran without its link")
		}
	}
}

func TestOtherComponentsStillRunAfterAFailure(t *testing.T) {
	h := newHarness(t, "linker-exit1,linker2-value,importer-ok,importer2-empty")
	rep := Run(context.Background(), h.ctx)

	if len(rep.Warnings) != 1 || rep.Warnings[0].Component != "context-suite/linker" {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	if got := h.snapshot().Links["github.pull_request"]; got != "https://example.test/pr/212-from-linker2" {
		t.Errorf("link = %q, want the healthy Linker's", got)
	}
	if _, err := os.Stat(filepath.Join(h.ctx.WorkDir, "notes", "context.md")); err != nil {
		t.Errorf("the healthy Importer's output is missing: %v", err)
	}
}

func TestFailedComponentLeavesNoEffect(t *testing.T) {
	h := newHarness(t, "linker-value,importer-exit1,importer2-empty")
	before, _ := os.ReadDir(h.ctx.WorkDir)
	Run(context.Background(), h.ctx)
	after, _ := os.ReadDir(h.ctx.WorkDir)

	if len(before) != len(after) {
		t.Errorf("Work directory changed: %d -> %d entries", len(before), len(after))
	}
}

func TestFailureCarriesDebugDetailOnlyInTheCause(t *testing.T) {
	h := newHarness(t, "linker-exit1,linker2-none")
	rep := Run(context.Background(), h.ctx)

	w := rep.Warnings[0]
	f, ok := w.Cause.(*Failure)
	if !ok {
		t.Fatalf("cause = %T, want *Failure", w.Cause)
	}
	if f.ExitCode != 1 || !strings.Contains(f.Stderr, "simulated failure") {
		t.Errorf("failure = %+v", f)
	}
	if strings.Contains(w.Summary+w.Hint, "simulated failure") {
		t.Error("the extension's own text reached the human summary")
	}
}

// waitForLaunch blocks until the fixture log shows a launch of role.
func waitForLaunch(t *testing.T, h *harness, role string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, l := range h.launches() {
			if l.Role == role {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never started", role)
}

func TestInterruptDuringALinkerStopsEverythingAfterIt(t *testing.T) {
	h := newHarness(t, "linker-hang,linker2-value,importer-ok")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Report, 1)
	go func() { done <- Run(ctx, h.ctx) }()

	waitForLaunch(t, h, "linker")
	cancel()
	var rep Report
	select {
	case rep = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return after the interrupt")
	}

	if len(rep.Warnings) != 1 || rep.Warnings[0].Token != diag.WarnExtensionInterrupted {
		t.Fatalf("warnings = %+v, want one interrupted", rep.Warnings)
	}
	if rep.Warnings[0].Component != "context-suite/linker" || rep.Warnings[0].Operation != OpDiscover {
		t.Errorf("warning = %+v", rep.Warnings[0])
	}
	for _, l := range h.launches() {
		if l.Role != "linker" {
			t.Errorf("%s started after the interrupt", l.Role)
		}
	}
	if len(h.snapshot().Links) != 0 {
		t.Errorf("links = %v, nothing may persist after an interrupt", h.snapshot().Links)
	}
}

func TestInterruptDuringAnImporterDiscardsItsOutputAndRemovesTheStage(t *testing.T) {
	h := newHarness(t, "linker-value,importer-hang,importer2-ok")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Report, 1)
	go func() { done <- Run(ctx, h.ctx) }()

	waitForLaunch(t, h, "importer")
	cancel()
	var rep Report
	select {
	case rep = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return after the interrupt")
	}

	if len(rep.Warnings) != 1 || rep.Warnings[0].Token != diag.WarnExtensionInterrupted {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	if rep.Warnings[0].Operation != OpImport {
		t.Errorf("operation = %q", rep.Warnings[0].Operation)
	}
	if _, err := os.Stat(filepath.Join(h.ctx.WorkDir, "notes")); !os.IsNotExist(err) {
		t.Errorf("partial output was incorporated: %v", err)
	}
	for _, d := range stageDirsFrom(h) {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("stage %s survived the interrupt", d)
		}
	}
	if slices.ContainsFunc(h.launches(), func(l struct {
		Role  string
		Stdin map[string]any
	}) bool {
		return l.Role == "importer2"
	}) {
		t.Error("importer2 started after the interrupt")
	}
	if got := h.snapshot().Links["github.pull_request"]; got == "" {
		t.Error("the link persisted before the interrupt was lost")
	}
}

func TestAnAlreadyCancelledContextStartsNothing(t *testing.T) {
	h := newHarness(t, "linker-value")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := Run(ctx, h.ctx)

	if len(rep.Warnings) != 1 || rep.Warnings[0].Token != diag.WarnExtensionInterrupted {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
	if len(h.launches()) != 0 {
		t.Errorf("launches = %+v, want none", h.launches())
	}
}
