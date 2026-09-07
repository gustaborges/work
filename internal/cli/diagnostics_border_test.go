package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

// The diagnostic border is the one place a failure is rendered
// (contracts/diagnostics.md). These cases pin the renderer-selection table.
func TestRenderDiagnosticNonInteractiveIsFrozenFormat(t *testing.T) {
	var b bytes.Buffer
	err := diag.New(diag.DirtyWorktree, "that Work's worktree has uncommitted or untracked changes")
	code := renderDiagnostic(&b, strings.NewReader(""), false, err)

	if got := strings.TrimRight(b.String(), "\n"); got != "error: dirty-worktree: that Work's worktree has uncommitted or untracked changes" {
		t.Errorf("non-interactive line = %q", got)
	}
	if code != 23 {
		t.Errorf("exit code = %d, want 23", code)
	}
	if strings.Contains(b.String(), "✘") {
		t.Errorf("non-interactive output used the interactive mark: %q", b.String())
	}
}

func TestRenderDiagnosticNonInteractiveByteIdenticalToFormat(t *testing.T) {
	for _, err := range []error{
		diag.New(diag.TargetNotFound, "no Work has that id"),
		diag.New(diag.TargetArchived, "that Work is archived and cannot be resumed").WithHint("pick another"),
		diag.Wrap(diag.BootstrapFailed, errors.New("disk gone"), "cannot open the lookup index"),
		errors.New("bare error"),
	} {
		var b bytes.Buffer
		renderDiagnostic(&b, strings.NewReader(""), false, err)
		if got, want := strings.TrimRight(b.String(), "\n"), diag.Format(err); got != want {
			t.Errorf("non-interactive render = %q, want diag.Format %q", got, want)
		}
	}
}

func TestRenderDiagnosticInteractiveCancelled(t *testing.T) {
	var b bytes.Buffer
	code := renderDiagnostic(&b, strings.NewReader(""), true, diag.New(diag.Cancelled, "cancelled"))
	if b.String() != "✘ Operation cancelled\n" {
		t.Errorf("interactive cancel = %q", b.String())
	}
	if code != 20 {
		t.Errorf("exit code = %d, want 20", code)
	}
	if strings.Contains(b.String(), "error:") {
		t.Errorf("interactive cancel leaked the non-interactive form: %q", b.String())
	}
}

func TestRenderDiagnosticInteractiveHuman(t *testing.T) {
	var b bytes.Buffer
	err := diag.New(diag.DirtyWorktree, "internal phrasing").
		WithSummary("that Work has uncommitted changes").
		WithHint("run with --force-dirty to archive it anyway")
	code := renderDiagnostic(&b, strings.NewReader(""), true, err)

	want := "✘ that Work has uncommitted changes\n" +
		"  → run with --force-dirty to archive it anyway\n"
	if b.String() != want {
		t.Errorf("interactive human = %q, want %q", b.String(), want)
	}
	if code != 23 {
		t.Errorf("exit code = %d, want 23", code)
	}
	if strings.Contains(b.String(), "error:") || strings.Contains(b.String(), "internal phrasing") {
		t.Errorf("interactive human leaked internal text: %q", b.String())
	}
}

func TestRenderDiagnosticInteractiveHumanFallsBackToMsg(t *testing.T) {
	var b bytes.Buffer
	renderDiagnostic(&b, strings.NewReader(""), true, diag.New(diag.TargetNotFound, "no Work has that id"))
	if b.String() != "✘ no Work has that id\n" {
		t.Errorf("interactive human (no summary) = %q", b.String())
	}
}

func TestRenderDiagnosticInteractiveNonDiag(t *testing.T) {
	var b bytes.Buffer
	code := renderDiagnostic(&b, strings.NewReader(""), true, errors.New("kaboom"))
	want := "✘ something went wrong\n  → run with WORK_DEBUG=1 for details\n"
	if b.String() != want {
		t.Errorf("interactive non-diag = %q, want %q", b.String(), want)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if strings.Contains(b.String(), "kaboom") {
		t.Errorf("interactive non-diag leaked the internal message: %q", b.String())
	}
}

func TestRenderDiagnosticWorkDebugAppendsCauseChain(t *testing.T) {
	err := diag.Wrap(diag.BootstrapFailed, errors.New("permission denied"), "cannot open the lookup index")

	var quiet bytes.Buffer
	renderDiagnostic(&quiet, strings.NewReader(""), true, err)
	if strings.Contains(quiet.String(), "permission denied") {
		t.Errorf("cause chain shown without WORK_DEBUG: %q", quiet.String())
	}

	t.Setenv("WORK_DEBUG", "1")
	var loud bytes.Buffer
	renderDiagnostic(&loud, strings.NewReader(""), true, err)
	if !strings.Contains(loud.String(), "cannot open the lookup index") ||
		!strings.Contains(loud.String(), "permission denied") {
		t.Errorf("WORK_DEBUG did not append the cause chain: %q", loud.String())
	}
	// The human line still comes first and is unchanged.
	if !strings.HasPrefix(loud.String(), "✘ cannot open the lookup index\n") {
		t.Errorf("WORK_DEBUG changed the human line: %q", loud.String())
	}
}
