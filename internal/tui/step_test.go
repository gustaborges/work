package tui

import (
	"regexp"
	"strings"
	"testing"
)

var stepANSI = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestStepDone(t *testing.T) {
	var b strings.Builder
	StepDone(&b, "Slug", "tenantizacao")
	out := stepANSI.ReplaceAllString(b.String(), "")

	if !strings.HasPrefix(out, "Slug\n") {
		t.Errorf("title is not the first line:\n%q", out)
	}
	if !strings.Contains(out, "✓ tenantizacao") {
		t.Errorf("missing the checked value:\n%q", out)
	}
	// Exactly one blank line trails the block so the next prompt is separated
	// from it by a single empty row.
	if !strings.HasSuffix(out, "\n\n") || strings.HasSuffix(out, "\n\n\n") {
		t.Errorf("want exactly one trailing blank line:\n%q", out)
	}
}
