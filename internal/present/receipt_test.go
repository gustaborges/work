package present

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

func TestReceiptColourOff(t *testing.T) {
	got := Receipt(offTheme(), "Local repository path", MarkSuccess, "/home/u/src")
	want := "Local repository path\n  ✔ /home/u/src\n\n"
	if got != want {
		t.Fatalf("Receipt = %q, want %q", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("colour-off receipt carries an escape byte: %q", got)
	}
}

func TestReceiptColourOn(t *testing.T) {
	got := Receipt(onTheme(), "Slug", MarkSuccess, "my-work")
	// The ✔ is styled with the Success token; the rest is plain.
	if !strings.Contains(got, "Slug\n  ") || !strings.HasSuffix(got, " my-work\n\n") {
		t.Errorf("receipt shape wrong: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;2;45;190;140m✔\x1b[m") {
		t.Errorf("✔ not styled with the Success token: %q", got)
	}
}

func TestCancelNotice(t *testing.T) {
	if got := CancelNotice(offTheme()); got != "✘ Operation cancelled\n" {
		t.Errorf("CancelNotice = %q", got)
	}
	on := CancelNotice(onTheme())
	if !strings.Contains(on, "\x1b[38;2;255;92;122m✘\x1b[m Operation cancelled\n") {
		t.Errorf("CancelNotice colour-on = %q", on)
	}
}

func TestConfirmReceipt(t *testing.T) {
	if got := ConfirmReceipt(offTheme(), "Create Work"); got != "✔ Create Work confirmed\n" {
		t.Errorf("ConfirmReceipt = %q", got)
	}
}

func TestAsciiMarkFallback(t *testing.T) {
	if got := Receipt(asciiTheme(), "T", MarkSuccess, "v"); got != "T\n  ok v\n\n" {
		t.Errorf("ascii receipt = %q", got)
	}
	if got := CancelNotice(asciiTheme()); got != "x Operation cancelled\n" {
		t.Errorf("ascii cancel = %q", got)
	}
}
