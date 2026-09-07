package present

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/gustaborges/work/internal/present/theme"
)

// baseFrame carries the theme, terminal size, and background signal shared by
// every primitive model. Width/height start at a sane default so a model driven
// without a WindowSizeMsg (unit tests) still renders.
type baseFrame struct {
	th   theme.Theme
	cap  theme.Capability
	dark bool
	w, h int
}

func newBaseFrame(io IO) baseFrame {
	cap := theme.Detect(io.UI, io.In)
	return baseFrame{cap: cap, dark: true, th: theme.New(cap, true), w: 80, h: 24}
}

// absorb handles the messages every model treats identically — terminal resize
// and the background-colour reply — and reports whether it consumed msg.
func (f *baseFrame) absorb(msg tea.Msg) bool {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		f.w, f.h = m.Width, m.Height
		return true
	case tea.BackgroundColorMsg:
		f.dark = m.IsDark()
		f.th = theme.New(f.cap, f.dark)
		return true
	}
	return false
}

// rule is a full-width horizontal line in the Primary token, drawn across the
// top of every wizard screen (contracts/interaction.md §1).
func (f baseFrame) rule() string {
	w := f.w
	if w < 1 {
		w = 1
	}
	return f.th.Primary.Render(strings.Repeat("─", w))
}

// clamp bounds content to the viewport: every line is truncated to f.w display
// cells and the frame is capped at f.h lines, so no active frame ever wraps the
// body or scrolls past the alternate-screen buffer — content that scrolls off
// cannot be reclaimed (FR-002, FR-006, SC-004).
func (f baseFrame) clamp(content string) tea.View {
	lines := strings.Split(content, "\n")
	if f.w > 0 {
		for i, ln := range lines {
			lines[i] = TruncTail(ln, f.w)
		}
	}
	if f.h > 0 && len(lines) > f.h {
		lines = lines[:f.h]
	}
	return tea.NewView(strings.Join(lines, "\n"))
}
