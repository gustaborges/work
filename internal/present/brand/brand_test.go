package brand

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

func TestArtDimensionsStable(t *testing.T) {
	if artWidth != 35 || artHeight != 6 {
		t.Fatalf("art is %dx%d, want 35x6 (contracts/brand.md — regenerate goldens if the glyph design changed)", artWidth, artHeight)
	}
	for i, l := range artLines {
		if w := lipgloss.Width(l); w != artWidth {
			t.Errorf("art line %d display width = %d, want %d (%q)", i, w, artWidth, l)
		}
	}
}

func TestRenderWideGradient(t *testing.T) {
	out := Render(120, colorprofile.TrueColor, true)

	if !strings.Contains(out, "\x1b[") {
		t.Fatal("wide true-colour brand has no styling")
	}
	if !strings.Contains(out, tagline) || !strings.Contains(out, direction) {
		t.Errorf("brand missing tagline/direction:\n%s", out)
	}
	// The gradient sweeps primary -> secondary: the first coloured column is
	// #11A8CD, the last is #8B7CF6 (per-column interpolation endpoints).
	if !strings.Contains(out, "38;2;17;168;205") {
		t.Errorf("gradient does not start at #11A8CD (17;168;205):\n%q", out)
	}
	if !strings.Contains(out, "38;2;139;124;246") {
		t.Errorf("gradient does not end at #8B7CF6 (139;124;246):\n%q", out)
	}
	// Intermediate columns differ from both endpoints.
	stops := lipgloss.Blend1D(artWidth, lipgloss.Color(gradientStart), lipgloss.Color(gradientEnd))
	if len(stops) != artWidth {
		t.Fatalf("Blend1D returned %d stops, want %d", len(stops), artWidth)
	}
}

func TestRenderWidePlainNoColor(t *testing.T) {
	out := Render(120, colorprofile.NoTTY, false)

	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("colour-off brand contains an escape sequence:\n%q", out)
	}
	for _, l := range artLines {
		if !strings.Contains(out, strings.TrimRight(l, " ")) {
			t.Errorf("colour-off brand missing art line %q:\n%s", l, out)
		}
	}
	if !strings.Contains(out, tagline) || !strings.Contains(out, direction) {
		t.Errorf("colour-off brand missing tagline/direction:\n%s", out)
	}
}

func TestRenderShallowProfileIsPlain(t *testing.T) {
	// Colour on but only 16 colours: no per-column gradient, but bold is allowed.
	out := Render(120, colorprofile.ANSI, true)
	if strings.Contains(out, "38;2;") || strings.Contains(out, "38;5;") {
		t.Errorf("ANSI16 brand carries a 24-bit/256 colour:\n%q", out)
	}
}

func TestRenderNarrowCompact(t *testing.T) {
	for _, w := range []int{10, 20, 34} {
		out := Render(w, colorprofile.TrueColor, true)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if lines[0] != "WORK" && !strings.Contains(lines[0], "WORK") {
			t.Errorf("width %d: first line is not the compact WORK word: %q", w, lines[0])
		}
		for _, l := range lines {
			if strings.Contains(l, "██") {
				t.Errorf("width %d: compact form still contains block art: %q", w, l)
			}
		}
	}
}

func TestRenderNarrowCompactNoColorIsPlain(t *testing.T) {
	out := Render(20, colorprofile.NoTTY, false)
	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("colour-off compact brand contains an escape sequence:\n%q", out)
	}
}

func TestHeaderIsCompactAndDirectionless(t *testing.T) {
	h := Header(false)
	if strings.ContainsRune(h, '\x1b') {
		t.Errorf("colour-off header contains an escape sequence: %q", h)
	}
	if strings.Contains(h, "██") {
		t.Errorf("help header must be the compact word, not the art: %q", h)
	}
	if strings.Contains(h, direction) {
		t.Errorf("help header must not carry the `work --help` direction: %q", h)
	}
	if !strings.Contains(h, "WORK") || !strings.Contains(h, tagline) {
		t.Errorf("help header missing WORK/tagline: %q", h)
	}
}

func TestRenderDefaultsWidthWhenUnknown(t *testing.T) {
	if Render(0, colorprofile.TrueColor, true) != Render(80, colorprofile.TrueColor, true) {
		t.Error("width 0 should behave as width 80")
	}
}
