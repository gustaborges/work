package present

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func catalogFixture() *catalogModel {
	m := newCatalog(context.Background(), IO{}, CatalogSpec{Title: "Plugins", Empty: "No plugins installed.", ErrorText: func(e error) string { return e.Error() }})
	for i := 0; i < 50; i++ {
		m.data.Installed = append(m.data.Installed, CatalogItem{ID: fmt.Sprintf("p%02d", i), Name: fmt.Sprintf("plugin-%02d", i), Installed: true, Enabled: i%2 == 0, Details: strings.Repeat("long detail content\n", 50) + "LAST DETAIL", Actions: []CatalogAction{{ID: "toggle", Label: "Disable"}, {ID: "update", Label: "Check updates"}, {ID: "uninstall", Label: "Uninstall"}}})
	}
	return m
}
func catalogPress(m *catalogModel, key string) tea.Cmd { _, cmd := m.Update(press(key)); return cmd }
func catalogRun(m *catalogModel, cmd tea.Cmd) {
	if cmd != nil {
		m.Update(cmd())
	}
}
func catalogType(m *catalogModel, text string) {
	for _, r := range text {
		catalogPress(m, string(r))
	}
}
func TestCatalogInventorySearchScrollAndResize(t *testing.T) {
	m := catalogFixture()
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 48, Height: 16}, {Width: 120, Height: 50}} {
		m.Update(size)
		for i := 0; i < 49; i++ {
			catalogPress(m, "down")
		}
		view := m.View()
		if !view.AltScreen || !strings.Contains(view.Content, "plugin-49") || !strings.Contains(view.Content, "Esc close") {
			t.Fatal(view.Content)
		}
		if strings.Contains(view.Content, "Version") || strings.Contains(view.Content, "local-linked") {
			t.Fatal("metadata crowded inventory", view.Content)
		}
		lines := strings.Split(view.Content, "\n")
		if len(lines) > size.Height {
			t.Fatal("overflow", len(lines))
		}
		for _, l := range lines {
			if ansi.StringWidth(l) > size.Width {
				t.Fatal("line overflow", l)
			}
		}
	}
	catalogPress(m, "/")
	catalogType(m, "PLUGIN-23")
	catalogPress(m, "enter")
	if got := m.items(); len(got) != 1 || got[0].ID != "p23" {
		t.Fatal(got)
	}
	catalogPress(m, "enter")
	if m.selected != "p23" || m.screen != catalogDetails {
		t.Fatal("details not selected")
	}
	catalogPress(m, "esc")
	if m.query != "PLUGIN-23" {
		t.Fatal("Back lost search")
	}
	catalogPress(m, "esc")
	if m.query != "" {
		t.Fatal("Esc must clear search first")
	}
	cmd := catalogPress(m, "esc")
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("normal exit did not quit")
	}
}
func TestCatalogLongNamesAndDetailsKeepActionsReachable(t *testing.T) {
	m := catalogFixture()
	m.data.Installed[0].Name = strings.Repeat("長い名前", 40)
	if !strings.Contains(m.View().Content, "Enabled") {
		t.Fatal(m.View().Content)
	}
	catalogPress(m, "enter")
	catalogPress(m, "down")
	catalogPress(m, "down")
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 16})
	if !strings.Contains(m.View().Content, "› Uninstall") {
		t.Fatal("focused action hidden", m.View().Content)
	}
	m.scroll = 1 << 20
	if !strings.Contains(m.View().Content, "LAST DETAIL") {
		t.Fatal("details inaccessible", m.View().Content)
	}
}
func TestCatalogToggleRefreshesWithoutReceiptHistory(t *testing.T) {
	m := catalogFixture()
	calls := 0
	m.spec.Toggle = func(_ context.Context, id string, enabled bool) error {
		calls++
		m.data.Installed[0].Enabled = enabled
		return nil
	}
	m.spec.Load = func(context.Context) (CatalogData, error) { return m.data, nil }
	catalogRun(m, catalogPress(m, "space"))
	if calls != 1 || m.data.Installed[0].Enabled || m.screen != catalogList {
		t.Fatal("toggle failed")
	}
	if !strings.Contains(m.View().Content, "plugin-00 disabled.") {
		t.Fatal(m.View().Content)
	}
	catalogRun(m, catalogPress(m, "space"))
	view := m.View().Content
	if calls != 2 || !strings.Contains(view, "plugin-00 enabled.") || strings.Contains(view, "disabled.") || strings.Contains(view, "✔") {
		t.Fatal(view)
	}
}
func TestCatalogReviewAuthorizationBackAndApply(t *testing.T) {
	m := catalogFixture()
	closed, applied := 0, 0
	m.selected = "p00"
	m.screen = catalogDetails
	review := func() *CatalogReview {
		return &CatalogReview{Title: "Review uninstall", Content: "Owned installation", Dependencies: []string{"p00/clone"}, Accept: "Uninstall", Close: func() { closed++ }, Apply: func(_ context.Context, remove bool) (string, error) {
			if !remove {
				t.Fatal("missing authorization")
			}
			applied++
			return "Uninstalled p00.", nil
		}}
	}
	m.spec.Prepare = func(context.Context, CatalogRequest) (*CatalogReview, error) { return review(), nil }
	m.spec.Load = func(context.Context) (CatalogData, error) { return m.data, nil }
	catalogRun(m, m.start(CatalogRequest{Action: "uninstall"}))
	m.control = 2
	catalogPress(m, "enter")
	if applied != 0 || m.problem == "" {
		t.Fatal("dependency gate missing")
	}
	catalogPress(m, "esc")
	if closed != 1 || m.screen != catalogDetails {
		t.Fatal("Back ended manager or leaked plan")
	}
	catalogRun(m, m.start(CatalogRequest{Action: "uninstall"}))
	m.control = 1
	catalogPress(m, "space")
	m.control = 2
	catalogRun(m, catalogPress(m, "enter"))
	if applied != 1 || closed != 2 || m.screen != catalogDetails || m.review != nil {
		t.Fatal("application did not return to manager")
	}
	if !strings.Contains(m.View().Content, "Uninstalled p00.") {
		t.Fatal(m.View().Content)
	}
}
func TestCatalogRecoverableFailuresAndDeclinesStayOpen(t *testing.T) {
	m := catalogFixture()
	m.selected = "p00"
	m.screen = catalogDetails
	m.spec.Prepare = func(context.Context, CatalogRequest) (*CatalogReview, error) {
		return nil, errors.New("Source unavailable; check its path.")
	}
	catalogRun(m, m.start(CatalogRequest{Action: "update"}))
	if m.screen != catalogDetails || !strings.Contains(m.View().Content, "Source unavailable") {
		t.Fatal(m.View().Content)
	}
	m.spec.Prepare = func(context.Context, CatalogRequest) (*CatalogReview, error) {
		return &CatalogReview{Title: "Check", Content: "unchanged"}, nil
	}
	catalogRun(m, m.start(CatalogRequest{Action: "update"}))
	catalogPress(m, "enter")
	if m.screen != catalogDetails || m.problem != "" {
		t.Fatal("decline terminated manager")
	}
}
func TestCatalogFormEditingMaskValidationAndBack(t *testing.T) {
	m := catalogFixture()
	m.spec.Fields = []CatalogField{{Key: "source", Label: "Source", Secret: true, Validate: func(s string) error {
		if s == "" {
			return errors.New("Enter a source")
		}
		return nil
	}}, {Key: "alias", Label: "Alias"}, {Key: "mode", Label: "Mode", Choices: []string{"Copy", "Link"}}}
	m.selectTab(2)
	catalogPress(m, "enter")
	m.field = 3
	catalogPress(m, "enter")
	if m.field != 0 || m.problem == "" {
		t.Fatal("source validation missing")
	}
	catalogType(m, "https://user:secret@example.test/repo")
	if strings.Contains(m.View().Content, "secret") {
		t.Fatal("credential leaked")
	}
	catalogPress(m, "ctrl+u")
	catalogType(m, "local")
	catalogPress(m, "left")
	catalogType(m, "Q")
	if m.values["source"] != "locaQl" {
		t.Fatal(m.values)
	}
	catalogPress(m, "enter")
	catalogType(m, "q-is-valid")
	catalogPress(m, "enter")
	catalogPress(m, "space")
	if m.values["mode"] != "Link" {
		t.Fatal(m.values)
	}
	catalogPress(m, "esc")
	m.selectTab(2)
	if m.values["alias"] != "q-is-valid" {
		t.Fatal("Back lost fields")
	}
}

func TestCatalogLongInputKeepsCursorVisible(t *testing.T) {
	m := catalogFixture()
	m.spec.Fields = []CatalogField{{Key: "name", Label: "Name"}}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 16})
	m.selectTab(2)
	catalogType(m, strings.Repeat("name", 30))
	if !strings.Contains(m.View().Content, "█") {
		t.Fatal("cursor scrolled out of field", m.View().Content)
	}
}

func TestCatalogInstallTabKeepsNavigationFocusAndDraft(t *testing.T) {
	m := catalogFixture()
	m.spec.Fields = []CatalogField{{Key: "source", Label: "Source"}}
	for _, key := range []string{"right", "right", "right", "left", "left", "left"} {
		catalogPress(m, key)
		if m.editingForm {
			t.Fatal("tab navigation entered editing")
		}
	}
	if m.tab != 0 {
		t.Fatal("tabs did not wrap both ways", m.tab)
	}
	catalogPress(m, "left")
	if m.tab != 2 || !strings.Contains(m.View().Content, "Enter/Tab edit form") {
		t.Fatal(m.View().Content)
	}
	catalogPress(m, "enter")
	catalogType(m, "draft-path")
	catalogPress(m, "left")
	catalogType(m, "Q")
	if m.tab != 2 || m.values["source"] != "draft-patQh" {
		t.Fatal("editing arrow changed tabs", m.values)
	}
	catalogPress(m, "esc")
	if m.editingForm || m.tab != 2 {
		t.Fatal("Esc did not restore tab navigation")
	}
	catalogPress(m, "right")
	if m.tab != 0 {
		t.Fatal("install tab trapped navigation")
	}
	catalogPress(m, "left")
	if m.values["source"] != "draft-patQh" || m.editingForm {
		t.Fatal("draft lost", m.values)
	}
	catalogPress(m, "tab")
	if !m.editingForm {
		t.Fatal("Tab did not enter form")
	}
}
