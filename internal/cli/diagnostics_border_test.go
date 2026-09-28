package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/extension"
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
	t.Setenv("WORK_DEBUG", "")
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

	t.Setenv("WORK_DEBUG", "")
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

func TestRenderDiagnosticWorkDebugShowsSingleNonDiagError(t *testing.T) {
	t.Setenv("WORK_DEBUG", "1")
	var b bytes.Buffer
	renderDiagnostic(&b, strings.NewReader(""), true, errors.New("kaboom"))
	if !strings.Contains(b.String(), "kaboom") {
		t.Errorf("WORK_DEBUG hid an unwrapped non-diag error: %q", b.String())
	}
}

func TestRenderDiagnosticWorkDebugDoesNotDuplicateBareDiagError(t *testing.T) {
	t.Setenv("WORK_DEBUG", "1")
	var b bytes.Buffer
	renderDiagnostic(&b, strings.NewReader(""), true, diag.New(diag.TargetNotFound, "no Work has that id"))
	if b.String() != "✘ no Work has that id\n" {
		t.Errorf("bare diag error changed under WORK_DEBUG: %q", b.String())
	}
}

func sampleWarning() diag.Warning {
	return diag.NewWarning(diag.WarnExtensionFailed, "01J9WORK", "acme/pr-linker", "discover",
		"it exited unsuccessfully; no link was recorded").WithHint("Run it again with WORK_DEBUG=1 to see why.")
}

func TestRenderWarningNonInteractiveIsFrozenLine(t *testing.T) {
	t.Setenv("WORK_DEBUG", "")
	var b bytes.Buffer
	renderWarning(&b, strings.NewReader(""), false, sampleWarning())

	want := "warning: extension-failed: acme/pr-linker (discover) for Work 01J9WORK: it exited unsuccessfully; no link was recorded\n"
	if b.String() != want {
		t.Errorf("line = %q, want %q", b.String(), want)
	}
}

func TestRenderWarningInteractiveShowsSummaryAndHint(t *testing.T) {
	t.Setenv("WORK_DEBUG", "")
	t.Setenv("NO_COLOR", "1")
	var b bytes.Buffer
	renderWarning(&b, strings.NewReader(""), true, sampleWarning())

	out := b.String()
	if !strings.Contains(out, "acme/pr-linker: it exited unsuccessfully; no link was recorded") ||
		!strings.Contains(out, "Run it again with WORK_DEBUG=1") {
		t.Errorf("interactive warning = %q", out)
	}
	if strings.Contains(out, "warning: extension-failed") {
		t.Errorf("interactive warning used the frozen line: %q", out)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("control sequences with NO_COLOR set: %q", out)
	}
}

func TestRenderWarningDebugAppendsExitStatusAndStderr(t *testing.T) {
	w := sampleWarning().WithCause(&extension.Failure{ExitCode: 3, Stderr: "boom\nsecond line\n", Err: errors.New("exited 3")})

	t.Setenv("WORK_DEBUG", "")
	var quiet bytes.Buffer
	renderWarning(&quiet, strings.NewReader(""), false, w)
	if strings.Contains(quiet.String(), "boom") || strings.Contains(quiet.String(), "exit status") {
		t.Errorf("debug detail leaked without WORK_DEBUG: %q", quiet.String())
	}

	t.Setenv("WORK_DEBUG", "1")
	var loud bytes.Buffer
	renderWarning(&loud, strings.NewReader(""), false, w)
	for _, want := range []string{"exit status: 3", "boom", "second line"} {
		if !strings.Contains(loud.String(), want) {
			t.Errorf("WORK_DEBUG output %q lacks %q", loud.String(), want)
		}
	}
}

func TestProgressObserverLinesAreFrozen(t *testing.T) {
	var b bytes.Buffer
	o := newProgressObserver(&b, strings.NewReader(""), false)
	o.OnEvent(extension.Event{Kind: extension.Running, Component: "acme/pr-linker", Operation: extension.OpDiscover})
	o.OnEvent(extension.Event{Kind: extension.Linked, Component: "acme/pr-linker", Operation: extension.OpDiscover, Key: "github.pull_request"})
	o.OnEvent(extension.Event{Kind: extension.Running, Component: "acme/importer", Operation: extension.OpImport})
	o.OnEvent(extension.Event{Kind: extension.Imported, Component: "acme/importer", Operation: extension.OpImport, Count: 2})

	want := "work: running acme/pr-linker (discover)\n" +
		"work: acme/pr-linker: linked github.pull_request\n" +
		"work: running acme/importer (import)\n" +
		"work: acme/importer: imported 2 item(s)\n"
	if b.String() != want {
		t.Errorf("progress =\n%q\nwant\n%q", b.String(), want)
	}
}

func TestProgressObserverNoControlSequencesWhenNotATerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var b bytes.Buffer
	o := newProgressObserver(&b, strings.NewReader(""), true)
	o.OnEvent(extension.Event{Kind: extension.Running, Component: "acme/l", Operation: extension.OpDiscover})
	if strings.Contains(b.String(), "\x1b") || b.String() != "work: running acme/l (discover)\n" {
		t.Errorf("output = %q", b.String())
	}
}
