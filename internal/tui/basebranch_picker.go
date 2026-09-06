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
)

// bbVisibleRows is the fixed height of the branch list. The picker always
// renders exactly this many list lines (padding when there are fewer, scrolling
// when there are more) so the frame height never changes between keystrokes —
// Bubble Tea's inline renderer leaves stale lines behind otherwise.
const bbVisibleRows = 8

// BaseBranchItem is one row offered by the base-branch picker. Remote marks a
// remote-tracking ref; the picker groups items into a Remote tab and a Local
// tab. Label is shown verbatim.
type BaseBranchItem struct {
	Label  string
	Remote bool
}

// baseBranchTab is one selectable group. rows holds indices into the original
// item slice so a selection maps straight back to the caller's ordering.
type baseBranchTab struct {
	name string
	rows []int
}

// baseBranchModel is the Bubble Tea model for the two-phase base-branch picker:
// pick a source tab (Remote / Local), then a branch within it, with "/" to
// filter the active tab. A tab with no rows is not created; when only one tab
// exists the tab bar is hidden. A future slice may add an "Other work" source —
// deliberately absent here.
type baseBranchModel struct {
	items  []BaseBranchItem
	tabs   []baseBranchTab
	active int

	filtering bool
	filter    string
	visible   []int // indices into items, = active tab's rows narrowed by filter
	cursor    int   // index into visible
	offset    int   // index into visible of the first shown row

	selected  int // index into items, or -1 while unresolved / on abort
	hasDarkBg bool
}

func newBaseBranchModel(items []BaseBranchItem) baseBranchModel {
	var remote, local baseBranchTab
	remote.name, local.name = "Remote", "Local"
	for i, it := range items {
		if it.Remote {
			remote.rows = append(remote.rows, i)
		} else {
			local.rows = append(local.rows, i)
		}
	}
	m := baseBranchModel{items: items, selected: -1, hasDarkBg: true}
	if len(remote.rows) > 0 {
		m.tabs = append(m.tabs, remote)
	}
	if len(local.rows) > 0 {
		m.tabs = append(m.tabs, local)
	}
	m.refilter()
	return m
}

// refilter recomputes visible from the active tab and the current filter, then
// re-clamps the cursor and scroll offset.
func (m *baseBranchModel) refilter() {
	m.visible = m.visible[:0]
	if len(m.tabs) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	for _, row := range m.tabs[m.active].rows {
		if needle == "" || strings.Contains(strings.ToLower(m.items[row].Label), needle) {
			m.visible = append(m.visible, row)
		}
	}
	m.clampView()
}

// clampView keeps cursor within the visible rows and offset within a window
// that keeps the cursor on screen.
func (m *baseBranchModel) clampView() {
	n := len(m.visible)
	m.cursor = min(max(m.cursor, 0), max(0, n-1))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+bbVisibleRows {
		m.offset = m.cursor - bbVisibleRows + 1
	}
	m.offset = min(max(m.offset, 0), max(0, n-bbVisibleRows))
}

func (m baseBranchModel) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m baseBranchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.hasDarkBg = msg.IsDark()
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m baseBranchModel) handleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := key.String()

	// Ctrl-C always aborts.
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
		case "left", "right", "tab", "shift+tab":
			m.switchTab(s)
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
	case "left", "h", "right", "l", "tab", "shift+tab":
		m.switchTab(s)
	}
	return m, nil
}

func (m *baseBranchModel) moveCursor(delta int) {
	m.cursor += delta
	m.clampView()
}

func (m *baseBranchModel) switchTab(key string) {
	if len(m.tabs) < 2 {
		return
	}
	switch key {
	case "left", "h", "shift+tab":
		m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
	default:
		m.active = (m.active + 1) % len(m.tabs)
	}
	m.filter = ""
	m.filtering = false
	m.cursor, m.offset = 0, 0
	m.refilter()
}

func (m baseBranchModel) choose() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.visible) {
		m.selected = m.visible[m.cursor]
		return m, tea.Quit
	}
	return m, nil // empty filter result: ignore
}

func (m baseBranchModel) View() tea.View {
	t := huh.ThemeCharm(m.hasDarkBg).Focused
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	// Matches the fuchsia accent huh uses for its select cursor.
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#F780E2")).Bold(true)

	var lines []string
	lines = append(lines, t.Title.Render("Base branch"), "")

	if len(m.tabs) > 1 {
		var bar strings.Builder
		for i, tab := range m.tabs {
			if i > 0 {
				bar.WriteString("  ")
			}
			if i == m.active {
				bar.WriteString(accent.Render("[ " + tab.name + " ]"))
			} else {
				bar.WriteString(dim.Render("  " + tab.name + "  "))
			}
		}
		lines = append(lines, bar.String(), "")
	}

	// List region: always exactly bbVisibleRows lines.
	end := min(m.offset+bbVisibleRows, len(m.visible))
	switch {
	case len(m.visible) == 0:
		lines = append(lines, dim.Render("  no branch matches the filter"))
		for range bbVisibleRows - 1 {
			lines = append(lines, "")
		}
	default:
		for i := m.offset; i < end; i++ {
			label := m.items[m.visible[i]].Label
			if i == m.cursor {
				lines = append(lines, t.SelectSelector.Render("")+t.SelectedOption.Render(label))
			} else {
				lines = append(lines, "  "+t.Option.Render(label))
			}
		}
		for i := end - m.offset; i < bbVisibleRows; i++ {
			lines = append(lines, "")
		}
	}

	// Scroll position (one line, blank when the whole list fits).
	if len(m.visible) > bbVisibleRows {
		lines = append(lines, dim.Render(fmt.Sprintf("  %d–%d of %d", m.offset+1, end, len(m.visible))))
	} else {
		lines = append(lines, "")
	}

	// Footer: always two lines — filter input (blank when not filtering), then help.
	if m.filtering {
		lines = append(lines, accent.Render("/")+m.filter+dim.Render("█"),
			dim.Render("enter accept · esc clear · ↑/↓ move"))
	} else {
		help := "↑/↓ move · / filter · enter select · q quit"
		if len(m.tabs) > 1 {
			help = "←/→ switch · " + help
		}
		lines = append(lines, "", dim.Render(help))
	}

	return tea.NewView(t.Base.Render(strings.Join(lines, "\n")))
}

// SelectBaseBranch opens the two-phase base-branch picker and returns the index
// into items of the chosen row. It requires an interactive terminal; callers
// gate on tui.IsInteractive first. Aborting (q / Esc / Ctrl-C) returns a
// diag.Cancelled error. items must be non-empty.
func SelectBaseBranch(ctx context.Context, items []BaseBranchItem) (int, error) {
	if len(items) == 0 {
		return 0, diag.New(diag.NoBaseBranch, "the repository has no selectable base branch")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	final, err := tea.NewProgram(newBaseBranchModel(items), tea.WithContext(ctx)).Run()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, diag.New(diag.Cancelled, "cancelled")
		}
		return 0, diag.Wrap(diag.Usage, err, "the base-branch picker could not be shown")
	}
	m, ok := final.(baseBranchModel)
	if !ok {
		return 0, diag.New(diag.Usage, "the base-branch picker ended in an unexpected state")
	}
	if m.selected < 0 {
		return 0, diag.New(diag.Cancelled, "cancelled")
	}
	return m.selected, nil
}
