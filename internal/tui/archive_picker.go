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

// apVisibleRows is the fixed number of Work rows the archive picker renders, for
// the same stable-frame-height reason as the resume picker.
const apVisibleRows = 6

// archiveStage is which view the archive picker is showing.
type archiveStage int

const (
	apList    archiveStage = iota // the multi-select list
	apConfirm                     // the confirmation view for the checked Works
)

// archiveModel is the multi-select picker for `work archive`. It renders the
// shared two-line Work row with a checkbox, and on Enter moves to a confirmation
// view that spells out the destructive consequences before anything happens.
type archiveModel struct {
	rows          []worklist.WorkRow
	workspaceRoot string

	stage archiveStage

	filtering bool
	filter    string
	visible   []int // indices into rows, narrowed by the filter
	cursor    int   // index into visible
	offset    int   // index into visible of the first shown row

	checked map[int]bool // indices into rows that are selected

	confirmed bool // Enter pressed on the confirmation view
	cancelled bool // Ctrl-C at any point
	hasDarkBg bool
}

func newArchiveModel(rows []worklist.WorkRow, workspaceRoot string) archiveModel {
	m := archiveModel{
		rows:          rows,
		workspaceRoot: workspaceRoot,
		checked:       make(map[int]bool, len(rows)),
		hasDarkBg:     true,
	}
	m.refilter()
	return m
}

func (m *archiveModel) refilter() {
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

func (m *archiveModel) clampView() {
	n := len(m.visible)
	m.cursor = min(max(m.cursor, 0), max(0, n-1))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+apVisibleRows {
		m.offset = m.cursor - apVisibleRows + 1
	}
	m.offset = min(max(m.offset, 0), max(0, n-apVisibleRows))
}

// checkedIndices returns the checked rows in the original list order.
func (m archiveModel) checkedIndices() []int {
	var out []int
	for i := range m.rows {
		if m.checked[i] {
			out = append(out, i)
		}
	}
	return out
}

func (m archiveModel) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m archiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.hasDarkBg = msg.IsDark()
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m archiveModel) handleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := key.String()

	// Ctrl-C cancels the whole operation from any view.
	if s == "ctrl+c" {
		m.cancelled = true
		return m, tea.Quit
	}

	if m.stage == apConfirm {
		switch s {
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		case "esc", "q":
			m.stage = apList
		}
		return m, nil
	}

	if m.filtering {
		switch s {
		case "esc":
			m.filter = ""
			m.filtering = false
			m.refilter()
		case "enter":
			return m.advance()
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
		case "space":
			m.toggle()
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
		m.cancelled = true
		return m, tea.Quit
	case "/":
		m.filtering = true
	case "space":
		m.toggle()
	case "enter":
		return m.advance()
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	}
	return m, nil
}

func (m *archiveModel) moveCursor(delta int) {
	m.cursor += delta
	m.clampView()
}

func (m *archiveModel) toggle() {
	if m.cursor < len(m.visible) {
		idx := m.visible[m.cursor]
		if m.checked[idx] {
			delete(m.checked, idx)
		} else {
			m.checked[idx] = true
		}
	}
}

// advance moves from the list to the confirmation view. Enter with nothing
// checked stays on the list (a no-op).
func (m archiveModel) advance() (tea.Model, tea.Cmd) {
	if len(m.checkedIndices()) == 0 {
		return m, nil
	}
	m.filtering = false
	m.stage = apConfirm
	return m, nil
}

func (m archiveModel) View() tea.View {
	t := huh.ThemeCharm(m.hasDarkBg).Focused
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#F780E2")).Bold(true)

	if m.stage == apConfirm {
		return tea.NewView(t.Base.Render(m.confirmView(t, dim)))
	}

	var lines []string
	lines = append(lines, t.Title.Render("Archive Works"), "")

	end := min(m.offset+apVisibleRows, len(m.visible))
	switch {
	case len(m.visible) == 0:
		lines = append(lines, dim.Render("  no Work matches the filter"))
		for range apVisibleRows*2 - 1 {
			lines = append(lines, "")
		}
	default:
		for i := m.offset; i < end; i++ {
			idx := m.visible[i]
			r := m.rows[idx]
			box := "[ ] "
			if m.checked[idx] {
				box = "[x] "
			}
			second := "      " + r.RelativeTime + " • " + r.Branch
			if i == m.cursor {
				lines = append(lines,
					t.SelectSelector.Render("")+t.SelectedOption.Render(box+r.DisplayName),
					dim.Render(second))
			} else {
				lines = append(lines,
					"  "+t.Option.Render(box+r.DisplayName),
					dim.Render(second))
			}
		}
		for i := end - m.offset; i < apVisibleRows; i++ {
			lines = append(lines, "", "")
		}
	}

	if len(m.visible) > apVisibleRows {
		lines = append(lines, dim.Render(fmt.Sprintf("  %d–%d of %d", m.offset+1, end, len(m.visible))))
	} else {
		lines = append(lines, "")
	}

	if m.filtering {
		lines = append(lines,
			accent.Render("/")+m.filter+dim.Render("█"),
			dim.Render("space toggle · enter continue · esc clear · ↑/↓ move"))
	} else {
		lines = append(lines, "", dim.Render("space toggle · ↑/↓ move · / filter · enter continue · q quit"))
	}

	return tea.NewView(t.Base.Render(strings.Join(lines, "\n")))
}

func (m archiveModel) confirmView(t huh.FieldStyles, dim lipgloss.Style) string {
	picked := m.checkedIndices()
	var lines []string
	lines = append(lines, t.Title.Render("Archive Works"), "")
	for _, idx := range picked {
		r := m.rows[idx]
		lines = append(lines, "  "+t.Option.Render(r.DisplayName), dim.Render("      "+r.Branch))
	}
	lines = append(lines, "",
		fmt.Sprintf("%d worktree(s) will be destroyed.", len(picked)),
		"Snapshots move to "+archivedDir(m.workspaceRoot),
		"Branches are kept.",
		"",
		dim.Render("enter confirm · esc back"))
	return strings.Join(lines, "\n")
}

func archivedDir(workspaceRoot string) string {
	if workspaceRoot == "" {
		return "the archived area"
	}
	return workspaceRoot + "/archived/"
}

// SelectArchive opens the multi-select archive picker over rows and returns the
// ids the user checked and confirmed, in list order. It requires an interactive
// terminal; callers gate on tui.IsInteractive first. Cancelling (q / Esc at the
// list, Ctrl-C anywhere) returns a diag.Cancelled error. A confirmed selection
// with no rows checked cannot happen (Enter is a no-op then). rows must be
// non-empty.
func SelectArchive(ctx context.Context, rows []worklist.WorkRow, workspaceRoot string) ([]string, error) {
	if len(rows) == 0 {
		return nil, diag.New(diag.Usage, "there are no Works to archive")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	final, err := tea.NewProgram(newArchiveModel(rows, workspaceRoot), tea.WithContext(ctx)).Run()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, diag.New(diag.Cancelled, "cancelled")
		}
		return nil, diag.Wrap(diag.Usage, err, "the archive picker could not be shown")
	}
	m, ok := final.(archiveModel)
	if !ok {
		return nil, diag.New(diag.Usage, "the archive picker ended in an unexpected state")
	}
	if m.cancelled || !m.confirmed {
		return nil, diag.New(diag.Cancelled, "cancelled")
	}
	picked := m.checkedIndices()
	ids := make([]string, 0, len(picked))
	for _, idx := range picked {
		ids = append(ids, m.rows[idx].ID)
	}
	if len(ids) == 0 {
		return nil, diag.New(diag.Cancelled, "cancelled")
	}
	return ids, nil
}
