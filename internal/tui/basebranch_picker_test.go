package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func bbPress(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case "left":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft})
	case "right":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyRight})
	case "tab":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	case "backspace":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	case "ctrl+c":
		return tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})
	default:
		return tea.KeyPressMsg(tea.Key{Code: []rune(s)[0], Text: s})
	}
}

func bbStep(m tea.Model, keys ...string) (baseBranchModel, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = m.Update(bbPress(k))
	}
	return m.(baseBranchModel), cmd
}

func mixedItems() []BaseBranchItem {
	return []BaseBranchItem{
		{Label: "main            aaaaaaa", Remote: false},
		{Label: "feature/x       bbbbbbb", Remote: false},
		{Label: "origin/main     ccccccc", Remote: true},
		{Label: "origin/dev      ddddddd", Remote: true},
	}
}

func TestBaseBranchDefaultTabIsRemote(t *testing.T) {
	m := newBaseBranchModel(mixedItems())
	if len(m.tabs) != 2 || m.tabs[0].name != "Remote" || m.tabs[1].name != "Local" {
		t.Fatalf("tabs = %+v, want [Remote Local]", m.tabs)
	}
	if m.active != 0 {
		t.Errorf("active tab = %d, want 0 (Remote)", m.active)
	}
	// Enter on the untouched Remote tab selects its first row (origin/main, index 2).
	got, cmd := bbStep(m, "enter")
	if !isQuit(cmd) {
		t.Error("enter did not quit")
	}
	if got.selected != 2 {
		t.Errorf("selected = %d, want 2 (origin/main)", got.selected)
	}
}

func TestBaseBranchSwitchTabAndSelect(t *testing.T) {
	for _, key := range []string{"right", "tab", "left"} {
		// Any single tab move lands on Local (2 tabs, wrap-around); down -> feature/x (index 1).
		got, _ := bbStep(newBaseBranchModel(mixedItems()), key, "down", "enter")
		if got.selected != 1 {
			t.Errorf("%s then down+enter: selected = %d, want 1 (feature/x)", key, got.selected)
		}
	}
}

func TestBaseBranchSwitchTabResetsCursorAndFilter(t *testing.T) {
	m := newBaseBranchModel(mixedItems())
	m, _ = bbStep(m, "down")             // cursor 1 on Remote
	m, _ = bbStep(m, "/", "d", "e", "v") // filter "dev" -> origin/dev only
	if len(m.visible) != 1 {
		t.Fatalf("filtered visible = %d, want 1", len(m.visible))
	}
	m, _ = bbStep(m, "right") // to Local
	if m.filtering || m.filter != "" {
		t.Errorf("tab switch left filter active: filtering=%v filter=%q", m.filtering, m.filter)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d after tab switch, want 0", m.cursor)
	}
	if len(m.visible) != 2 {
		t.Errorf("Local visible = %d, want 2", len(m.visible))
	}
}

func TestBaseBranchFilterNarrowsAndSelects(t *testing.T) {
	m := newBaseBranchModel(mixedItems())
	m, _ = bbStep(m, "/", "d", "e", "v") // Remote tab, "dev"
	if len(m.visible) != 1 || m.items[m.visible[0]].Label[:10] != "origin/dev" {
		t.Fatalf("visible = %v", m.visible)
	}
	got, cmd := bbStep(m, "enter")
	if !isQuit(cmd) || got.selected != 3 {
		t.Errorf("selected = %d, want 3 (origin/dev)", got.selected)
	}

	// Backspace widens the filter again.
	m2 := newBaseBranchModel(mixedItems())
	m2, _ = bbStep(m2, "/", "x", "backspace")
	if m2.filter != "" || len(m2.visible) != 2 {
		t.Errorf("after backspace: filter=%q visible=%d, want empty/2", m2.filter, len(m2.visible))
	}
}

func TestBaseBranchFilterEscClears(t *testing.T) {
	m := newBaseBranchModel(mixedItems())
	m, _ = bbStep(m, "/", "z", "z", "z") // no match
	if len(m.visible) != 0 {
		t.Fatalf("visible = %d, want 0", len(m.visible))
	}
	got, cmd := bbStep(m, "enter") // no selectable row -> ignored
	if isQuit(cmd) || got.selected != -1 {
		t.Errorf("enter on empty filter selected %d / quit=%v", got.selected, isQuit(cmd))
	}
	m, _ = bbStep(m, "esc") // clears filter, stays in picker
	if m.filtering || m.filter != "" || len(m.visible) != 2 {
		t.Errorf("esc did not clear filter: filtering=%v filter=%q visible=%d", m.filtering, m.filter, len(m.visible))
	}
}

func TestBaseBranchEmptyFilterEnterKeepsFiltering(t *testing.T) {
	m := newBaseBranchModel(mixedItems())
	m, _ = bbStep(m, "/", "z", "z", "z")
	got, cmd := bbStep(m, "enter")
	if isQuit(cmd) || !got.filtering || got.selected != -1 {
		t.Errorf("enter on empty filter: quit=%v filtering=%v selected=%d", isQuit(cmd), got.filtering, got.selected)
	}

	got, cmd = bbStep(got, "q")
	if isQuit(cmd) || got.filter != "zzzq" {
		t.Errorf("q after empty-filter enter: quit=%v filter=%q", isQuit(cmd), got.filter)
	}

	got, _ = bbStep(got, "esc")
	if got.filtering || got.filter != "" || len(got.visible) != 2 {
		t.Errorf("esc after empty-filter enter: filtering=%v filter=%q visible=%d", got.filtering, got.filter, len(got.visible))
	}
}

func TestBaseBranchEmptyModelIsSafe(t *testing.T) {
	for _, items := range [][]BaseBranchItem{nil, []BaseBranchItem{}} {
		m := newBaseBranchModel(items)
		if len(m.tabs) != 0 || len(m.visible) != 0 || m.cursor != 0 || m.offset != 0 {
			t.Errorf("empty model = %+v", m)
		}
		if got, cmd := bbStep(m, "enter"); isQuit(cmd) || got.selected != -1 {
			t.Errorf("empty model enter: quit=%v selected=%d", isQuit(cmd), got.selected)
		}
	}
}

func TestBaseBranchCursorClamps(t *testing.T) {
	m := newBaseBranchModel(mixedItems())
	m, _ = bbStep(m, "up", "up") // already at top
	if m.cursor != 0 {
		t.Errorf("cursor = %d after up past start, want 0", m.cursor)
	}
	m, _ = bbStep(m, "down", "down", "down", "down") // only 2 rows on Remote
	if m.cursor != 1 {
		t.Errorf("cursor = %d after down past end, want 1", m.cursor)
	}
}

func TestBaseBranchSingleTabHidesBar(t *testing.T) {
	local := []BaseBranchItem{
		{Label: "main    aaaaaaa", Remote: false},
		{Label: "dev     bbbbbbb", Remote: false},
	}
	m := newBaseBranchModel(local)
	if len(m.tabs) != 1 || m.tabs[0].name != "Local" {
		t.Fatalf("tabs = %+v, want [Local]", m.tabs)
	}
	view := m.View().Content
	if strings.Contains(view, "[ Local ]") || strings.Contains(view, "Remote") {
		t.Errorf("single-tab view shows a tab bar:\n%s", view)
	}
	if strings.Contains(view, "switch") {
		t.Errorf("single-tab view offers tab switching:\n%s", view)
	}
	// left/right/tab are no-ops with one tab; enter still selects.
	got, _ := bbStep(m, "right", "tab", "down", "enter")
	if got.selected != 1 {
		t.Errorf("selected = %d, want 1", got.selected)
	}
}

func TestBaseBranchAbortLeavesNoSelection(t *testing.T) {
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		got, cmd := bbStep(newBaseBranchModel(mixedItems()), key)
		if got.selected != -1 {
			t.Errorf("%s: selected = %d, want -1", key, got.selected)
		}
		if !isQuit(cmd) {
			t.Errorf("%s: did not quit", key)
		}
	}
	// While filtering, "q" types into the filter and does not quit.
	got, cmd := bbStep(newBaseBranchModel(mixedItems()), "/", "q")
	if isQuit(cmd) || got.selected != -1 || got.filter != "q" {
		t.Errorf("q while filtering: quit=%v filter=%q", isQuit(cmd), got.filter)
	}
}

func TestBaseBranchViewHeightIsConstant(t *testing.T) {
	// A long Local list and a short Remote list: the frame must stay the same
	// height across tab switches, filtering, and cursor moves, or Bubble Tea's
	// inline renderer leaves stale header lines behind.
	items := []BaseBranchItem{{Label: "origin/main  aaa", Remote: true}}
	for i := range 20 {
		items = append(items, BaseBranchItem{Label: fmt.Sprintf("branch-%02d  b%02d", i, i), Remote: false})
	}
	m := newBaseBranchModel(items)
	want := strings.Count(m.View().Content, "\n")

	check := func(label string, mm baseBranchModel) {
		if got := strings.Count(mm.View().Content, "\n"); got != want {
			t.Errorf("%s: view has %d newlines, want %d", label, got, want)
		}
	}
	m2, _ := bbStep(m, "tab") // -> Local (20 rows)
	check("local tab", m2)
	m3, _ := bbStep(m2, "down", "down", "down", "down", "down", "down", "down", "down", "down", "down")
	check("scrolled down", m3)
	m4, _ := bbStep(m3, "/", "b", "r")
	check("filtering", m4)
	m5, _ := bbStep(m4, "esc")
	check("filter cleared", m5)
	m6, _ := bbStep(m5, "tab") // back to Remote (1 row)
	check("remote tab", m6)
}

func TestBaseBranchScrollsToKeepCursorVisible(t *testing.T) {
	items := []BaseBranchItem{}
	for i := range 20 {
		items = append(items, BaseBranchItem{Label: fmt.Sprintf("b%02d", i), Remote: false})
	}
	m := newBaseBranchModel(items)
	for range 15 {
		m, _ = bbStep(m, "down")
	}
	if m.cursor != 15 {
		t.Fatalf("cursor = %d, want 15", m.cursor)
	}
	if m.cursor < m.offset || m.cursor >= m.offset+bbVisibleRows {
		t.Errorf("cursor %d outside window [%d,%d)", m.cursor, m.offset, m.offset+bbVisibleRows)
	}
	if !strings.Contains(m.View().Content, "b15") {
		t.Errorf("cursor row b15 not shown:\n%s", m.View().Content)
	}
	if strings.Contains(m.View().Content, "b00") {
		t.Errorf("scrolled view still shows b00:\n%s", m.View().Content)
	}
}

func TestBaseBranchViewHasBothTabsAndNoOtherWork(t *testing.T) {
	view := newBaseBranchModel(mixedItems()).View().Content
	if !strings.Contains(view, "Remote") || !strings.Contains(view, "Local") {
		t.Errorf("view missing a tab label:\n%s", view)
	}
	if strings.Contains(strings.ToLower(view), "other work") {
		t.Errorf("view exposes the reserved 'Other work' source:\n%s", view)
	}
}
