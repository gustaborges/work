package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestTokensDarkTrueColor(t *testing.T) {
	th := NewForProfile(colorprofile.TrueColor, true)
	want := map[string]string{
		"Primary":   "\x1b[1;38;2;17;168;205mx\x1b[m",
		"Secondary": "\x1b[38;2;139;124;246mx\x1b[m",
		"Success":   "\x1b[38;2;45;190;140mx\x1b[m",
		"Warning":   "\x1b[38;2;217;165;33mx\x1b[m",
		"Danger":    "\x1b[38;2;255;92;122mx\x1b[m",
		"Muted":     "\x1b[38;2;138;138;138mx\x1b[m",
		"Text":      "x",
	}
	assertTokens(t, th, want)
}

func TestTokensLightTrueColor(t *testing.T) {
	th := NewForProfile(colorprofile.TrueColor, false)
	want := map[string]string{
		"Primary":   "\x1b[1;38;2;8;124;153mx\x1b[m",
		"Secondary": "\x1b[38;2;93;76;201mx\x1b[m",
		"Success":   "\x1b[38;2;22;122;91mx\x1b[m",
		"Warning":   "\x1b[38;2;138;109;26mx\x1b[m",
		"Danger":    "\x1b[38;2;180;35;63mx\x1b[m",
		"Muted":     "\x1b[38;2;107;107;107mx\x1b[m",
		"Text":      "x",
	}
	assertTokens(t, th, want)
}

func TestTokensANSI16(t *testing.T) {
	th := NewForProfile(colorprofile.ANSI, true)
	// 4-bit downsample: colour survives, Primary keeps bold.
	want := map[string]string{
		"Primary":   "\x1b[1;94mx\x1b[m",
		"Secondary": "\x1b[94mx\x1b[m",
		"Text":      "x",
	}
	assertTokens(t, th, want)
	for _, s := range []string{th.Primary.Render("x"), th.Secondary.Render("x"), th.Danger.Render("x")} {
		if strings.Contains(s, "38;2;") || strings.Contains(s, "38;5;") {
			t.Errorf("ANSI16 token still carries a 24-bit / 256 colour: %q", s)
		}
	}
}

func TestTokensColourOffEmitsNoEscapes(t *testing.T) {
	th := New(Capability{ColorEnabled: false}, true)
	for name, style := range map[string]interface{ Render(...string) string }{
		"Primary":   th.Primary,
		"Secondary": th.Secondary,
		"Success":   th.Success,
		"Warning":   th.Warning,
		"Danger":    th.Danger,
		"Muted":     th.Muted,
		"Text":      th.Text,
	} {
		got := style.Render("x")
		if got != "x" {
			t.Errorf("%s colour-off render = %q, want %q", name, got, "x")
		}
		if strings.Contains(got, "\x1b") {
			t.Errorf("%s colour-off render contains an escape byte: %q", name, got)
		}
	}
}

func assertTokens(t *testing.T, th Theme, want map[string]string) {
	t.Helper()
	got := map[string]string{
		"Primary":   th.Primary.Render("x"),
		"Secondary": th.Secondary.Render("x"),
		"Success":   th.Success.Render("x"),
		"Warning":   th.Warning.Render("x"),
		"Danger":    th.Danger.Render("x"),
		"Muted":     th.Muted.Render("x"),
		"Text":      th.Text.Render("x"),
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %q, want %q", name, got[name], w)
		}
	}
}
