package theme

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Theme is the one semantic token set every renderer draws from. Each token is
// a ready-to-use lipgloss.Style: with colour enabled it carries the token's
// foreground (downsampled to the terminal's colour profile) and, for Primary,
// bold; with colour disabled every token is a plain style that emits no escape
// bytes at all. No renderer defines its own colour (contracts/theme.md).
type Theme struct {
	Primary   lipgloss.Style // titles, focused option, active group/tab, brand start
	Secondary lipgloss.Style // key hints, secondary emphasis, brand end
	Success   lipgloss.Style // the ✔ receipt mark
	Warning   lipgloss.Style // non-fatal notices
	Danger    lipgloss.Style // the ✘ mark, validation errors, destructive emphasis
	Muted     lipgloss.Style // descriptions, secondary metadata, scroll position line
	Text      lipgloss.Style // body text — always the terminal foreground, never recoloured

	// Cap is the capability probe this theme was built from.
	Cap Capability
}

// tokenPalette is one token's dark and light hex values. The values are the
// contract's starting point (research R13); light variants are contrast-checked
// during implementation.
type tokenPalette struct{ dark, light string }

var palette = map[string]tokenPalette{
	"primary":   {"#11A8CD", "#087C99"},
	"secondary": {"#8B7CF6", "#5D4CC9"},
	"success":   {"#2DBE8C", "#167A5B"},
	"warning":   {"#D9A521", "#8A6D1A"},
	"danger":    {"#FF5C7A", "#B4233F"},
	"muted":     {"#8A8A8A", "#6B6B6B"},
}

// New builds the active theme from a capability probe and a dark/light signal.
// When cap.ColorEnabled is false every token degrades to a plain style (zero
// styling bytes, SC-008); the textual ❯ / [x] / ✔ / ✘ marks carry state and
// focus meaning without colour.
func New(cap Capability, dark bool) Theme {
	t := Theme{Cap: cap}
	if !cap.ColorEnabled {
		plain := lipgloss.NewStyle()
		t.Primary, t.Secondary, t.Success = plain, plain, plain
		t.Warning, t.Danger, t.Muted, t.Text = plain, plain, plain, plain
		return t
	}

	fg := func(name string) lipgloss.Style {
		p := palette[name]
		hex := p.dark
		if !dark {
			hex = p.light
		}
		return lipgloss.NewStyle().Foreground(cap.Profile.Convert(lipgloss.Color(hex)))
	}

	t.Primary = fg("primary").Bold(true)
	t.Secondary = fg("secondary")
	t.Success = fg("success")
	t.Warning = fg("warning")
	t.Danger = fg("danger")
	t.Muted = fg("muted")
	t.Text = lipgloss.NewStyle()
	return t
}

// NewForProfile is a convenience for callers and tests that have a profile in
// hand: it builds a colour-enabled capability at that profile.
func NewForProfile(p colorprofile.Profile, dark bool) Theme {
	return New(Capability{ColorEnabled: true, Interactive: true, Profile: p}, dark)
}
