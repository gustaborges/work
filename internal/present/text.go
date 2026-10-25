package present

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// TextSpec describes a read-only result that remains visible until Back.
// Content must already be safe for terminal display; callers own redaction.
type TextSpec struct {
	Title   string
	Content string
}

type textModel struct {
	baseFrame
	spec            TextSpec
	offset          int
	done, cancelled bool
}

func newTextModel(io IO, spec TextSpec) textModel {
	return textModel{baseFrame: newBaseFrame(io), spec: spec}
}
func (m textModel) Init() tea.Cmd                   { return nil }
func (m textModel) withFrame(f baseFrame) stepModel { m.baseFrame = f; return m }
func (m textModel) lines() []string {
	return strings.Split(ansi.Wrap(strings.TrimSpace(m.spec.Content), max(m.w, 1), ""), "\n")
}
func (m textModel) help() string  { return Wrap("↑/↓ scroll · enter Back · q quit", max(m.w, 1)) }
func (m textModel) rows() int     { return max(m.h-3-lineCount(m.help()), 1) }
func (m *textModel) clampOffset() { m.offset = max(0, min(m.offset, len(m.lines())-m.rows())) }
func (m textModel) Update(msg tea.Msg) (stepModel, tea.Cmd) {
	if m.absorb(msg) {
		m.clampOffset()
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "enter":
		m.done = true
	case "q", "esc", "ctrl+c":
		m.cancelled = true
	case "up", "k":
		m.offset--
	case "down", "j":
		m.offset++
	case "pgup":
		m.offset -= m.rows()
	case "pgdown":
		m.offset += m.rows()
	case "home":
		m.offset = 0
	case "end":
		m.offset = len(m.lines())
	}
	m.clampOffset()
	return m, nil
}
func (m textModel) body(width, height int) string {
	m.w, m.h = width, height
	m.clampOffset()
	lines := m.lines()
	end := min(m.offset+m.rows(), len(lines))
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", m.th.Primary.Render(m.spec.Title))
	b.WriteString(strings.Join(lines[m.offset:end], "\n"))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%s\n", m.th.Muted.Render(fmt.Sprintf("%d–%d of %d", m.offset+1, end, len(lines))))
	b.WriteString(m.th.Muted.Render(m.help()))
	return b.String()
}
func (m textModel) status() stepStatus              { return stepStatus{done: m.done, cancelled: m.cancelled} }
func (m textModel) cursorPos() (tea.Position, bool) { return tea.Position{}, false }

// TextStep uses the wizard's full-screen frame and shared terminal lifecycle.
func TextStep(key string, build func(Answers) (TextSpec, error)) Step {
	return Step{key: key, make: func(a Answers, io IO, _ context.Context) (stepModel, error) {
		spec, err := build(a)
		if err != nil {
			return nil, err
		}
		return newTextModel(io, spec), nil
	}}
}
