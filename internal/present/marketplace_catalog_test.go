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

func managerFixture() *catalogModel {
	m := newCatalog(context.Background(), IO{}, CatalogSpec{
		Title: "Plugins", Empty: "No plugins installed.", ErrorText: func(e error) string { return e.Error() },
		FormAction: "add-source", FormTitle: "Add marketplace",
		InstalledActions: []CatalogAction{{ID: "enable", Label: "Enable", Direct: true}, {ID: "check", Label: "Check"}, {ID: "uninstall", Label: "Remove"}},
		SourceActions:    []CatalogAction{{ID: "add-source", Label: "Add marketplace", Form: true}, {ID: "available", Label: "Browse", Go: "available"}, {ID: "upgrade", Label: "Upgrade", Direct: true}},
	})
	for i := 0; i < 50; i++ {
		src := "alpha"
		if i%2 == 1 {
			src = "beta"
		}
		m.data.Installed = append(m.data.Installed, CatalogItem{
			ID: fmt.Sprintf("p%02d@%s", i, src), Name: fmt.Sprintf("plugin-%02d@%s", i, src), Source: src, Installed: true, Enabled: i%2 == 0,
			Details: strings.Repeat("long detail content\n", 50) + "LAST DETAIL",
			Actions: []CatalogAction{{ID: "disable", Label: "Disable", Direct: true}, {ID: "check", Label: "Check"}, {ID: "uninstall", Label: "Remove"}},
		})
	}
	m.data.Sources = []CatalogItem{
		{ID: "alpha", Name: "Alpha (alpha)", Badge: "Git repository", Actions: []CatalogAction{{ID: "available", Label: "Browse", Go: "available"}, {ID: "installed", Label: "Installed", Go: "installed"}, {ID: "remove-source", Label: "Remove marketplace"}}},
		{ID: "beta", Name: "beta", Badge: "Local directory", Actions: []CatalogAction{{ID: "available", Label: "Browse", Go: "available"}}},
	}
	m.data.Available = []CatalogItem{
		{ID: "x@alpha", Name: "x@alpha", Source: "alpha", Actions: []CatalogAction{{ID: "add", Label: "Add"}}},
		{ID: "y@beta", Name: "y@beta", Source: "beta", Installed: true, Actions: []CatalogAction{{ID: "add", Label: "Reinstall"}}},
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

func TestManagerInstalledGeometryScrollSearchAndTotals(t *testing.T) {
	m := managerFixture()
	for _, size := range []tea.WindowSizeMsg{{Width: 100, Height: 30}, {Width: 60, Height: 18}, {Width: 48, Height: 16}, {Width: 100, Height: 30}} {
		m.Update(size)
		for i := 0; i < 49; i++ {
			catalogPress(m, "down")
		}
		view := m.View()
		if !view.AltScreen || !strings.Contains(view.Content, "plugin-49") || !strings.Contains(view.Content, "Esc close") {
			t.Fatal(size, view.Content)
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
	if !strings.Contains(m.View().Content, "50 installed · 25 enabled") {
		t.Fatal(m.View().Content)
	}
	catalogPress(m, "/")
	catalogType(m, "PLUGIN-23")
	catalogPress(m, "enter")
	if got := m.items(); len(got) != 1 || got[0].ID != "p23@beta" {
		t.Fatal(got)
	}
	if !strings.Contains(m.View().Content, "50 installed") {
		t.Fatal("totals followed the search")
	}
	catalogPress(m, "enter")
	if m.selected != "p23@beta" || m.screen != catalogDetails {
		t.Fatal("details not selected")
	}
	catalogPress(m, "esc")
	if m.query != "PLUGIN-23" || m.screen != catalogList {
		t.Fatal("Back lost search")
	}
	catalogPress(m, "esc")
	if m.query != "" {
		t.Fatal("Esc must clear search first")
	}
	if _, ok := catalogPress(m, "esc")().(tea.QuitMsg); !ok {
		t.Fatal("normal exit did not quit")
	}
}

func TestManagerSourceFilterAndSelectionSurviveDetailsAndPrune(t *testing.T) {
	m := managerFixture()
	catalogPress(m, "f")
	if m.source != "alpha" || len(m.items()) != 25 {
		t.Fatal(m.source, len(m.items()))
	}
	catalogPress(m, "space")
	catalogPress(m, "down")
	catalogPress(m, "space")
	if len(m.picked) != 2 {
		t.Fatal(m.picked)
	}
	catalogPress(m, "enter")
	catalogPress(m, "esc")
	if m.source != "alpha" || len(m.picked) != 2 || m.cursor != 1 {
		t.Fatal("details lost filter, selection or position")
	}
	catalogPress(m, "s")
	if len(m.picked) != 25 {
		t.Fatal("select all shown", len(m.picked))
	}
	m.data.Installed = m.data.Installed[:2]
	m.prune()
	if len(m.picked) != 1 {
		t.Fatal("pruned selection should keep only surviving targets", m.picked)
	}
	m.data.Sources = m.data.Sources[1:]
	m.prune()
	if m.source != "" {
		t.Fatal("vanished source filter kept")
	}
}

func TestManagerTabsAndSourceNavigationBackStack(t *testing.T) {
	m := managerFixture()
	catalogPress(m, "right")
	if m.view != viewSources || len(m.items()) != 2 {
		t.Fatal("Marketplaces tab")
	}
	if !strings.Contains(m.View().Content, "Git repository") || !strings.Contains(m.View().Content, "2 marketplaces") {
		t.Fatal(m.View().Content)
	}
	catalogPress(m, "enter")
	catalogPress(m, "enter") // Browse
	if m.view != viewAvailable || m.source != "alpha" || len(m.items()) != 1 {
		t.Fatal("source entries not filtered", m.view, m.source)
	}
	catalogPress(m, "right")
	if m.view != viewAvailable {
		t.Fatal("tab key left a nested collection")
	}
	catalogPress(m, "esc")
	if m.view != viewSources || m.screen != catalogDetails || m.selected != "alpha" {
		t.Fatal("Back did not return to source details")
	}
	catalogPress(m, "down")
	catalogPress(m, "enter") // Installed here
	if m.view != viewInstalled || m.source != "alpha" || len(m.items()) != 25 {
		t.Fatal("installed source filter")
	}
	catalogPress(m, "esc")
	catalogPress(m, "esc")
	if m.view != viewSources || m.screen != catalogList {
		t.Fatal("stack did not unwind to the collection")
	}
}

func TestManagerAllSourcesCatalogHasNoBulkInstall(t *testing.T) {
	m := managerFixture()
	catalogPress(m, "right")
	catalogPress(m, "a")
	if m.screen != catalogMenu {
		t.Fatal("actions menu")
	}
	catalogPress(m, "down")
	catalogPress(m, "enter")
	if m.view != viewAvailable || m.source != "" || len(m.items()) != 2 {
		t.Fatal("all-source catalog")
	}
	catalogPress(m, "space")
	catalogPress(m, "a")
	if len(m.picked) != 0 || m.screen != catalogList {
		t.Fatal("available entries must not be selectable or have bulk actions")
	}
}

func TestManagerBulkTargetsSelectionOrShown(t *testing.T) {
	m := managerFixture()
	var got []CatalogRequest
	m.spec.Run = func(_ context.Context, r CatalogRequest) (string, error) { got = append(got, r); return "Done.", nil }
	m.spec.Load = func(context.Context) (CatalogData, error) { return m.data, nil }
	catalogPress(m, "f")
	catalogPress(m, "a")
	if !strings.Contains(m.View().Content, "Applies to all 25 shown") {
		t.Fatal(m.View().Content)
	}
	catalogRun(m, catalogPress(m, "enter"))
	if len(got) != 1 || len(got[0].IDs) != 25 || got[0].Action != "enable" {
		t.Fatal(got)
	}
	if m.screen != catalogList || !strings.Contains(m.View().Content, "Done.") {
		t.Fatal("did not return with refreshed state", m.View().Content)
	}
	catalogPress(m, "space")
	catalogPress(m, "a")
	if !strings.Contains(m.View().Content, "Applies to 1 selected") {
		t.Fatal(m.View().Content)
	}
	catalogRun(m, catalogPress(m, "enter"))
	if len(got) != 2 || len(got[1].IDs) != 1 {
		t.Fatal(got)
	}
}

func TestManagerReviewAuthorizationBackApplyAndCleanup(t *testing.T) {
	m := managerFixture()
	closed, applied := 0, 0
	m.selected = "p00@alpha"
	m.screen = catalogDetails
	review := func() *CatalogReview {
		return &CatalogReview{Title: "Review removal", Content: "Owned installation", Dependencies: []string{"p00/clone"}, Accept: "Remove", Destructive: true, Close: func() { closed++ }, Apply: func(_ context.Context, remove bool) (string, error) {
			if !remove {
				t.Fatal("missing authorization")
			}
			applied++
			return "Removed p00.", nil
		}}
	}
	m.spec.Prepare = func(context.Context, CatalogRequest) (*CatalogReview, error) { return review(), nil }
	m.spec.Load = func(context.Context) (CatalogData, error) { return m.data, nil }
	start := func() {
		m.action = 2
		catalogRun(m, catalogPress(m, "enter"))
	}
	start()
	m.control = 2
	catalogPress(m, "enter")
	if applied != 0 || m.problem == "" {
		t.Fatal("dependency gate missing")
	}
	catalogPress(m, "esc")
	if closed != 1 || m.screen != catalogDetails {
		t.Fatal("Back ended manager or leaked plan", closed, m.screen)
	}
	start()
	m.control = 1
	catalogPress(m, "space")
	m.control = 2
	catalogRun(m, catalogPress(m, "enter"))
	if applied != 1 || closed != 2 || m.screen != catalogDetails || m.review != nil {
		t.Fatal("application did not return to the originating surface", applied, closed, m.screen)
	}
	if !strings.Contains(m.View().Content, "Removed p00.") {
		t.Fatal(m.View().Content)
	}
}

func TestManagerFailuresAndInformationalReviewsStayOpen(t *testing.T) {
	m := managerFixture()
	m.selected = "p00@alpha"
	m.screen = catalogDetails
	m.spec.Prepare = func(context.Context, CatalogRequest) (*CatalogReview, error) {
		return nil, errors.New("Source unavailable; check its path.")
	}
	m.action = 1
	catalogRun(m, catalogPress(m, "enter"))
	if m.screen != catalogDetails || !strings.Contains(m.View().Content, "Source unavailable") {
		t.Fatal(m.View().Content)
	}
	closed := 0
	m.spec.Prepare = func(context.Context, CatalogRequest) (*CatalogReview, error) {
		return &CatalogReview{Title: "Update check", Content: "unchanged", Close: func() { closed++ }}, nil
	}
	catalogRun(m, catalogPress(m, "enter"))
	if m.screen != catalogReview || m.problem != "" {
		t.Fatal("review not shown or stale error kept", m.screen, m.problem)
	}
	catalogPress(m, "enter")
	if m.screen != catalogDetails || closed != 1 {
		t.Fatal("informational review Back", m.screen, closed)
	}
}

func TestManagerStaleAsyncCompletionIsDiscardedAndClosed(t *testing.T) {
	m := managerFixture()
	m.spec.Prepare = func(context.Context, CatalogRequest) (*CatalogReview, error) { return nil, nil }
	m.selected = "p00@alpha"
	m.screen = catalogDetails
	closed := 0
	cmd := m.prepare(CatalogRequest{Action: "check", IDs: []string{"p00@alpha"}}, "x")
	stale := catalogResultMsg{seq: m.seq - 1, review: &CatalogReview{Title: "old", Close: func() { closed++ }}}
	m.Update(stale)
	if closed != 1 || m.review != nil || !m.busy {
		t.Fatal("stale review adopted", closed)
	}
	_ = cmd
	m.busy = false
	m.Update(catalogResultMsg{seq: m.seq, review: &CatalogReview{Title: "new", Content: "c", Close: func() { closed++ }}})
	if m.review == nil || m.screen != catalogReview {
		t.Fatal("current review ignored")
	}
}

func TestManagerMultiLineResultsGetResultSurfaceAndBack(t *testing.T) {
	m := managerFixture()
	m.spec.Run = func(context.Context, CatalogRequest) (string, error) {
		return "upgraded a (r1)\nunchanged b", errors.New("one marketplace failed")
	}
	m.spec.Load = func(context.Context) (CatalogData, error) { return m.data, nil }
	catalogPress(m, "right")
	catalogPress(m, "a")
	catalogPress(m, "down")
	catalogPress(m, "down")
	catalogRun(m, catalogPress(m, "enter"))
	view := m.View().Content
	if m.screen != catalogResult || !strings.Contains(view, "upgraded a (r1)") || !strings.Contains(view, "one marketplace failed") {
		t.Fatal(view)
	}
	catalogPress(m, "esc")
	if m.screen != catalogList || m.view != viewSources {
		t.Fatal("results Back")
	}
}

func TestManagerFormVisibilityMaskValidationAndDraft(t *testing.T) {
	m := managerFixture()
	m.spec.Fields = []CatalogField{
		{Key: "source", Label: "Source", Secret: true, Validate: func(s string) error {
			if s == "" {
				return errors.New("Enter a source")
			}
			return nil
		}},
		{Key: "branch", Label: "Branch", Visible: func(v map[string]string) bool { return strings.HasPrefix(v["source"], "https://") }},
	}
	catalogPress(m, "right")
	catalogPress(m, "a")
	catalogPress(m, "enter")
	if m.screen != catalogForm {
		t.Fatal("form not opened")
	}
	if len(m.visibleFields()) != 1 {
		t.Fatal("branch shown for a non-Git source")
	}
	catalogPress(m, "tab")
	catalogPress(m, "enter")
	if m.field != 0 || m.problem == "" {
		t.Fatal("validation missing", m.field)
	}
	catalogType(m, "https://user:secret@example.test/repo")
	if strings.Contains(m.View().Content, "secret") {
		t.Fatal("credential leaked")
	}
	if len(m.visibleFields()) != 2 {
		t.Fatal("branch hidden for Git source")
	}
	catalogPress(m, "ctrl+u")
	catalogType(m, "local")
	catalogPress(m, "left")
	catalogType(m, "Q")
	if m.values["source"] != "locaQl" {
		t.Fatal(m.values)
	}
	catalogPress(m, "esc")
	if m.screen != catalogList || m.view != viewSources {
		t.Fatal("form Back")
	}
	catalogPress(m, "a")
	catalogPress(m, "enter")
	if m.values["source"] != "locaQl" {
		t.Fatal("draft lost")
	}
}

func TestManagerLongInputKeepsCursorVisibleAtSmallSize(t *testing.T) {
	m := managerFixture()
	m.spec.Fields = []CatalogField{{Key: "name", Label: "Name"}}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 16})
	catalogPress(m, "right")
	catalogPress(m, "a")
	catalogPress(m, "enter")
	catalogType(m, strings.Repeat("name", 30))
	if !strings.Contains(m.View().Content, "█") {
		t.Fatal("cursor scrolled out of field", m.View().Content)
	}
}

func TestManagerLongUnicodeNamesKeepFocusAndActionsReachable(t *testing.T) {
	m := managerFixture()
	m.data.Installed[0].Name = strings.Repeat("長い名前", 40)
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 16})
	if !strings.Contains(m.View().Content, "Enabled") {
		t.Fatal(m.View().Content)
	}
	catalogPress(m, "enter")
	catalogPress(m, "down")
	catalogPress(m, "down")
	if !strings.Contains(m.View().Content, "› Remove") {
		t.Fatal("focused action hidden", m.View().Content)
	}
	m.scroll = 1 << 20
	if !strings.Contains(m.View().Content, "LAST DETAIL") {
		t.Fatal("details inaccessible", m.View().Content)
	}
}

func TestManagerStartsOnMarketplacesAndRemovesObsoleteControls(t *testing.T) {
	m := newCatalog(context.Background(), IO{}, CatalogSpec{Title: "Plugins", StartSources: true})
	view := m.View().Content
	for _, banned := range []string{"Official", "Install plugin", "alias", "embedded"} {
		if strings.Contains(view, banned) {
			t.Fatal("obsolete control", banned, view)
		}
	}
	if m.view != viewSources {
		t.Fatal("marketplace command must start on Marketplaces")
	}
}

func TestManagerDestructiveActionIsRecognizableWithoutColor(t *testing.T) {
	m := managerFixture()
	m.review = &CatalogReview{Title: "Review removal", Content: "c", Accept: "Remove", Destructive: true, Apply: func(context.Context, bool) (string, error) { return "", nil }}
	m.screen = catalogReview
	m.control = 1
	m.cap.ColorEnabled = false
	if !strings.Contains(m.View().Content, "› Remove") {
		t.Fatal(m.View().Content)
	}
}
