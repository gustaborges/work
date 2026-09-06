package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/worklist"
)

// rpVisibleRows is the fixed number of Work rows the resume picker renders. It
// pads when there are fewer and scrolls when there are more, so the frame height
// is stable between keystrokes (Bubble Tea's inline renderer leaves stale lines
// otherwise).
const rpVisibleRows = 6

// resumeModel is the single-select recency picker for `work resume`. It renders
// the shared two-line Work row (name on top, relative access time and branch
// beneath) and returns the chosen Work id.
type resumeModel struct {
	rows []worklist.WorkRow

	filtering bool
	filter    string
	visible   []int // indices into rows, narrowed by the filter
	cursor    int   // index into visible
	offset    int   // index into visible of the first shown row

	selected  int // index into rows, or -1 while unresolved / on cancel
	hasDarkBg bool
}

func newResumeModel(rows []worklist.WorkRow) resumeModel {
	m := resumeModel{rows: rows, selected: -1, hasDarkBg: true}
	m.refilter()
	return m
}

// refilter recomputes visible from the current filter, then re-clamps the
// cursor and scroll offset.
func (m *resumeModel) refilter() {
	m.visible = m.visible[:0]
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	for i, r := range m.rows {
		hay := strings.ToLower(r.DisplayName + " " + r.Branch)
		if needle == "" || strings.Contains(hay, needle) {
			m.visible = append(m.visible, i)
		}
	}
	m.clampView()
}

// clampView keeps cursor within the visible rows and offset within a window
// that keeps the cursor on screen.
func (m *resumeModel) clampView() {
	n := len(m.visible)
	m.cursor = min(max(m.cursor, 0), max(0, n-1))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rpVisibleRows {
		m.offset = m.cursor - rpVisibleRows + 1
	}
	m.offset = min(max(m.offset, 0), max(0, n-rpVisibleRows))
}

func (m resumeModel) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m resumeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.hasDarkBg = msg.IsDark()
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m resumeModel) handleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := key.String()

	if s == "ctrl+c" {
		m.selected = -1
		return m, tea.Quit
	}

	if m.filtering {
		switch s {
		case "esc":
			m.filter = ""
			m.filtering = false
			m.refilter()
		case "enter":
			return m.choose()
		case "backspace":
			if r := []rune(m.filter); len(r) > 0 {
				m.filter = string(r[:len(r)-1])
				m.cursor, m.offset = 0, 0
				m.refilter()
			}
		case "up":
			m.moveCursor(-1)
		case "down":
			m.moveCursor(1)
		default:
			if r := []rune(s); len(r) == 1 && unicode.IsPrint(r[0]) {
				m.filter += s
				m.cursor, m.offset = 0, 0
				m.refilter()
			}
		}
		return m, nil
	}

	switch s {
	case "q", "esc":
		m.selected = -1
		return m, tea.Quit
	case "/":
		m.filtering = true
	case "enter":
		return m.choose()
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	}
	return m, nil
}

func (m *resumeModel) moveCursor(delta int) {
	m.cursor += delta
	m.clampView()
}

func (m resumeModel) choose() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.visible) {
		m.selected = m.visible[m.cursor]
		return m, tea.Quit
	}
	return m, nil // empty filter result: ignore
}

func (m resumeModel) View() tea.View {
	t := huh.ThemeCharm(m.hasDarkBg).Focused
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#F780E2")).Bold(true)

	var lines []string
	lines = append(lines, t.Title.Render("Resume a Work"), "")

	end := min(m.offset+rpVisibleRows, len(m.visible))
	switch {
	case len(m.visible) == 0:
		lines = append(lines, dim.Render("  no Work matches the filter"))
		for range rpVisibleRows*2 - 1 {
			lines = append(lines, "")
		}
	default:
		for i := m.offset; i < end; i++ {
			r := m.rows[m.visible[i]]
			second := "  " + r.RelativeTime + " • " + r.Branch
			if i == m.cursor {
				lines = append(lines,
					t.SelectSelector.Render("")+t.SelectedOption.Render(r.DisplayName),
					"  "+dim.Render(second))
			} else {
				lines = append(lines,
					"  "+t.Option.Render(r.DisplayName),
					"  "+dim.Render(second))
			}
		}
		for i := end - m.offset; i < rpVisibleRows; i++ {
			lines = append(lines, "", "")
		}
	}

	if len(m.visible) > rpVisibleRows {
		lines = append(lines, dim.Render(fmt.Sprintf("  %d–%d of %d", m.offset+1, end, len(m.visible))))
	} else {
		lines = append(lines, "")
	}

	if m.filtering {
		lines = append(lines,
			accent.Render("/")+m.filter+dim.Render("█"),
			dim.Render("enter accept · esc clear · ↑/↓ move"))
	} else {
		lines = append(lines, "", dim.Render("↑/↓ move · / filter · enter select · q quit"))
	}

	return tea.NewView(t.Base.Render(strings.Join(lines, "\n")))
}

// SelectResume opens the recency picker over rows and returns the chosen Work
// id. It requires an interactive terminal; callers gate on tui.IsInteractive
// first. Cancelling (q / Esc / Ctrl-C) returns a diag.Cancelled error. rows must
// be non-empty.
func SelectResume(ctx context.Context, rows []worklist.WorkRow) (string, error) {
	if len(rows) == 0 {
		return "", diag.New(diag.Usage, "there are no Works to resume")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	final, err := tea.NewProgram(newResumeModel(rows), tea.WithContext(ctx)).Run()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", diag.New(diag.Cancelled, "cancelled")
		}
		return "", diag.Wrap(diag.Usage, err, "the resume picker could not be shown")
	}
	m, ok := final.(resumeModel)
	if !ok {
		return "", diag.New(diag.Usage, "the resume picker ended in an unexpected state")
	}
	if m.selected < 0 {
		return "", diag.New(diag.Cancelled, "cancelled")
	}
	return m.rows[m.selected].ID, nil
}
