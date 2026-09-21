package diagrender

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/gustaborges/work/internal/present/theme"
)

func offTheme() theme.Theme { return theme.New(theme.Capability{ColorEnabled: false}, true) }
func onTheme() theme.Theme  { return theme.NewForProfile(colorprofile.TrueColor, true) }
func asciiTheme() theme.Theme {
	return theme.New(theme.Capability{ColorEnabled: false, AsciiMarks: true}, true)
}

func TestHumanColourOff(t *testing.T) {
	got := Human(offTheme(), "the workspace root does not exist", "")
	if got != "✘ the workspace root does not exist\n" {
		t.Fatalf("Human = %q", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("colour-off diagnostic carries an escape byte: %q", got)
	}
}

func TestHumanWithHint(t *testing.T) {
	got := Human(offTheme(), "that Work's worktree has uncommitted changes", "run with --force-dirty to archive it anyway")
	want := "✘ that Work's worktree has uncommitted changes\n" +
		"  → run with --force-dirty to archive it anyway\n"
	if got != want {
		t.Fatalf("Human = %q, want %q", got, want)
	}
}

func TestHumanColourOn(t *testing.T) {
	got := Human(onTheme(), "boom", "")
	if !strings.Contains(got, "\x1b[38;2;255;92;122m✘\x1b[m boom\n") {
		t.Errorf("✘ not styled with the Danger token: %q", got)
	}
}

func TestCancel(t *testing.T) {
	if got := Cancel(offTheme()); got != "✘ Operation cancelled\n" {
		t.Errorf("Cancel = %q", got)
	}
}

func TestUnexpected(t *testing.T) {
	got := Unexpected(offTheme())
	want := "✘ something went wrong\n  → run with WORK_DEBUG=1 for details\n"
	if got != want {
		t.Errorf("Unexpected = %q, want %q", got, want)
	}
}

func TestAsciiFallback(t *testing.T) {
	got := Human(asciiTheme(), "nope", "try again")
	want := "x nope\n  -> try again\n"
	if got != want {
		t.Errorf("ascii Human = %q, want %q", got, want)
	}
}

func TestWarn(t *testing.T) {
	got := Warn(offTheme(), "gh/linker could not run", "Run with WORK_DEBUG=1 for details.")
	want := "⚠ gh/linker could not run\n  → Run with WORK_DEBUG=1 for details.\n"
	if got != want {
		t.Errorf("Warn = %q, want %q", got, want)
	}
	if got := Warn(offTheme(), "just a summary", ""); got != "⚠ just a summary\n" {
		t.Errorf("Warn without hint = %q", got)
	}
}

func TestWarnAsciiFallbackAndColour(t *testing.T) {
	if got := Warn(asciiTheme(), "careful", "retry"); got != "! careful\n  -> retry\n" {
		t.Errorf("ASCII Warn = %q", got)
	}
	if got := Warn(onTheme(), "careful", ""); !strings.Contains(got, "\x1b[38;2;217;165;33m⚠\x1b[m careful\n") {
		t.Errorf("⚠ not styled with the Warning token: %q", got)
	}
}
