package brand

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

const (
	tagline   = "Isolated work, ready when you are."
	direction = "Run 'work --help' to get started."

	// Gradient endpoints: Primary at the left edge, Secondary at the right
	// (theme.md). Fixed regardless of the light/dark variant — the brand is the
	// one deliberate exception to the semantic theme (research R12/R13).
	gradientStart = "#11A8CD"
	gradientEnd   = "#8B7CF6"
)

// ArtWidth is the fixed display width of the full wordmark art. A caller with a
// known terminal width uses it to decide whether the full form fits.
func ArtWidth() int { return artWidth }

// Render returns the static WORK brand block shown by bare interactive `work`:
// the wordmark, a blank line, the tagline, a blank line, and the `work --help`
// direction, terminated by a newline. It is a pure function of the terminal
// width, its colour profile, and whether colour is enabled (contracts/brand.md):
//
//   - full gradient — width >= ArtWidth, colour enabled, profile has at least
//     256 colours: the art with a per-column primary->secondary gradient sweeping
//     through all four glyphs.
//   - full plain — width >= ArtWidth otherwise: the same art with no colour
//     (an optional bold when colour is on but the profile is shallow).
//   - compact — width < ArtWidth: the single word WORK plus the tagline and the
//     direction, each on its own line.
//
// When colorEnabled is false the returned string contains no ANSI/OSC control
// sequences at all, not even bold (FR-025, SC-008).
func Render(width int, profile colorprofile.Profile, colorEnabled bool) string {
	if width <= 0 {
		width = 80
	}
	if width < artWidth {
		return compact(colorEnabled) + "\n\n" + tagline + "\n\n" + direction + "\n"
	}

	var art []string
	if colorEnabled && profile >= colorprofile.ANSI256 {
		art = gradientLines(profile)
	} else {
		art = plainLines(colorEnabled)
	}
	return strings.Join(art, "\n") + "\n\n" +
		center(tagline) + "\n\n" +
		center(direction) + "\n"
}

// Header is the compact brand shown as the `work --help` heading regardless of
// terminal width (contracts/cli-help.md): the word WORK and the tagline, without
// the `work --help` direction. Plain unless colour is enabled.
func Header(colorEnabled bool) string {
	return compact(colorEnabled) + "\n" + tagline + "\n"
}

// compact is the single styled word WORK — bold only when colour is enabled, so
// the colour-off output stays escape-free.
func compact(colorEnabled bool) string {
	if colorEnabled {
		return lipgloss.NewStyle().Bold(true).Render("WORK")
	}
	return "WORK"
}

// plainLines is the art with no per-column colour; the whole block is bolded
// when colour is enabled but the profile is too shallow for the gradient.
func plainLines(colorEnabled bool) []string {
	if !colorEnabled {
		return artLines
	}
	bold := lipgloss.NewStyle().Bold(true)
	out := make([]string, len(artLines))
	for i, l := range artLines {
		out[i] = bold.Render(l)
	}
	return out
}

// gradientLines colours each terminal column of the art by linear interpolation
// from gradientStart to gradientEnd across artWidth, downsampled to profile.
// Blank columns are left unstyled so the block carries only the colour it needs.
func gradientLines(profile colorprofile.Profile) []string {
	stops := lipgloss.Blend1D(artWidth, lipgloss.Color(gradientStart), lipgloss.Color(gradientEnd))
	out := make([]string, len(artLines))
	for i, line := range artLines {
		var b strings.Builder
		for col, r := range []rune(line) {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			c := stops[len(stops)-1]
			if col < len(stops) {
				c = stops[col]
			}
			b.WriteString(lipgloss.NewStyle().Foreground(profile.Convert(c)).Render(string(r)))
		}
		out[i] = b.String()
	}
	return out
}

// center left-pads s to sit under the centre of the art block. It never
// negative-pads: a line wider than the art is returned unchanged.
func center(s string) string {
	if pad := (artWidth - lipgloss.Width(s)) / 2; pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}
