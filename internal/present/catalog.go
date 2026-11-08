package present

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// CatalogProperty is one labeled metadata line of an item's details.
type CatalogProperty struct{ Label, Value string }

// CatalogSection is a titled block of an item's details.
type CatalogSection struct{ Title, Content string }

// CatalogItem is a display-safe row. ID is the opaque identity handed back to
// callbacks; Source is the filter key shared by installed and available rows
// (the ID of the matching source row). Badge, when set, replaces the computed
// state column.
type CatalogItem struct {
	ID, Name, Details, Source, Badge string
	Installed, Enabled               bool
	Actions                          []CatalogAction
	Properties                       []CatalogProperty
	Sections                         []CatalogSection
}

// CatalogAction is something the user can run. Direct actions run immediately
// through CatalogSpec.Run; the others are reviewed through Prepare. Go opens a
// child collection instead ("available" or "installed", filtered to the current
// item when it is a source) and Form opens the registration form.
type CatalogAction struct {
	ID, Label, Go string
	Direct, Form  bool
}

// CatalogData is one coherent read of everything the manager shows. Installed
// and Sources are complete: search and filters never change their totals.
type CatalogData struct{ Installed, Sources, Available []CatalogItem }

// CatalogField is one editable registration field. Visible, when set, decides
// from the other values whether the field is shown.
type CatalogField struct {
	Key, Label, Hint string
	Secret           bool
	Validate         func(string) error
	Choices          []string
	Visible          func(values map[string]string) bool
}

// CatalogRequest names the targets of an action by opaque identity.
type CatalogRequest struct {
	IDs    []string
	Action string
	Values map[string]string
}

// CatalogReview is a frozen proposal. Close runs on every exit path and Apply
// is nil for purely informational reviews.
type CatalogReview struct {
	Title, Content, Accept string
	Destructive            bool
	Dependencies           []string
	Apply                  func(context.Context, bool) (string, error)
	Close                  func()
}

// CatalogSpec is the domain-free description of a persistent two-destination
// manager (Installed and Marketplaces). Callbacks own all domain rules and
// return display-safe text; no callback writes to the terminal.
type CatalogSpec struct {
	Title, Empty string
	StartSources bool
	Fields       []CatalogField
	FormAction   string
	FormTitle    string
	// InstalledActions and SourceActions apply to the selection set, or to every
	// row currently shown when nothing is selected.
	InstalledActions, SourceActions []CatalogAction
	Load                            func(context.Context) (CatalogData, error)
	Run                             func(context.Context, CatalogRequest) (string, error)
	Prepare                         func(context.Context, CatalogRequest) (*CatalogReview, error)
	ErrorText                       func(error) string
}

type catalogScreen int

const (
	catalogList catalogScreen = iota
	catalogDetails
	catalogForm
	catalogReview
	catalogMenu
	catalogResult
)

type catalogView int

const (
	viewInstalled catalogView = iota
	viewSources
	viewAvailable
)

type catalogResultMsg struct {
	seq    int
	data   *CatalogData
	review *CatalogReview
	text   string
	done   bool
	err    error
}

type catalogNav struct {
	view                                    catalogView
	screen                                  catalogScreen
	selected, source, query                 string
	cursor, offset, action, scroll, control int
}

type catalogModel struct {
	baseFrame
	ctx                                  context.Context
	spec                                 CatalogSpec
	data                                 CatalogData
	view                                 catalogView
	screen                               catalogScreen
	cursor, offset, action, scroll       int
	field, inputAt, control, origin, seq int
	query, source, selected, result      string
	working                              string
	searching                            bool
	picked                               map[string]bool
	values                               map[string]string
	stack                                []catalogNav
	review                               *CatalogReview
	authorized, busy                     bool
	notice, problem                      string
	cancel                               context.CancelFunc
}

func newCatalog(ctx context.Context, pio IO, spec CatalogSpec) *catalogModel {
	m := &catalogModel{baseFrame: newBaseFrame(pio), ctx: ctx, spec: spec, values: map[string]string{}, picked: map[string]bool{}}
	if spec.StartSources {
		m.view = viewSources
	}
	return m
}

func (m *catalogModel) Init() tea.Cmd { return tea.Batch(tea.RequestBackgroundColor, m.load()) }

func (m *catalogModel) load() tea.Cmd {
	m.busy = true
	m.working = "Loading…"
	m.seq++
	seq := m.seq
	return func() tea.Msg {
		data, err := m.spec.Load(m.ctx)
		if err != nil {
			return catalogResultMsg{seq: seq, err: err}
		}
		return catalogResultMsg{seq: seq, data: &data}
	}
}

func (m *catalogModel) closeReview() {
	if m.review != nil && m.review.Close != nil {
		m.review.Close()
	}
	m.review = nil
	m.authorized = false
}

func (m *catalogModel) errorText(err error) string {
	if m.spec.ErrorText != nil {
		return m.spec.ErrorText(err)
	}
	return "The operation could not be completed. Try again."
}

func (m *catalogModel) list() []CatalogItem {
	switch m.view {
	case viewSources:
		return m.data.Sources
	case viewAvailable:
		return m.data.Available
	}
	return m.data.Installed
}

// items applies the source filter and search to the current collection.
func (m *catalogModel) items() []CatalogItem {
	out := []CatalogItem{}
	q := strings.ToLower(m.query)
	for _, e := range m.list() {
		if m.view != viewSources && m.source != "" && e.Source != m.source {
			continue
		}
		if strings.Contains(strings.ToLower(e.Name), q) {
			out = append(out, e)
		}
	}
	return out
}

func (m *catalogModel) item() (CatalogItem, bool) {
	for _, e := range m.list() {
		if e.ID == m.selected {
			return e, true
		}
	}
	return CatalogItem{}, false
}

func (m *catalogModel) selectable() bool { return m.view != viewAvailable }

// prune drops selections and filters whose targets disappeared.
func (m *catalogModel) prune() {
	for id := range m.picked {
		if !slices.ContainsFunc(m.list(), func(e CatalogItem) bool { return e.ID == id }) {
			delete(m.picked, id)
		}
	}
	if m.source != "" && !slices.ContainsFunc(m.data.Sources, func(e CatalogItem) bool { return e.ID == m.source }) {
		m.source = ""
	}
}

func (m *catalogModel) sources() []string {
	seen := []string{}
	for _, e := range m.list() {
		if e.Source != "" && !slices.Contains(seen, e.Source) {
			seen = append(seen, e.Source)
		}
	}
	slices.Sort(seen)
	return seen
}

func (m *catalogModel) cycleSource() {
	all := append([]string{""}, m.sources()...)
	at := slices.Index(all, m.source)
	m.source = all[(at+1)%len(all)]
	m.cursor, m.offset = 0, 0
}

func (m *catalogModel) snapshot() catalogNav {
	return catalogNav{view: m.view, screen: m.screen, selected: m.selected, source: m.source, query: m.query,
		cursor: m.cursor, offset: m.offset, action: m.action, scroll: m.scroll, control: m.control}
}

func (m *catalogModel) restore(n catalogNav) {
	m.view, m.screen, m.selected, m.source, m.query = n.view, n.screen, n.selected, n.source, n.query
	m.cursor, m.offset, m.action, m.scroll, m.control = n.cursor, n.offset, n.action, n.scroll, n.control
	m.searching = false
}

func (m *catalogModel) push() { m.stack = append(m.stack, m.snapshot()) }

func (m *catalogModel) pop() bool {
	if len(m.stack) == 0 {
		return false
	}
	m.restore(m.stack[len(m.stack)-1])
	m.stack = m.stack[:len(m.stack)-1]
	return true
}

// descend opens an action surface above the current one. An action started
// from a collection or details remembers where to return once it completes.
func (m *catalogModel) descend(to catalogScreen) {
	if m.screen == catalogList || m.screen == catalogDetails {
		m.origin = len(m.stack)
	}
	m.push()
	m.screen = to
	m.scroll, m.control, m.action = 0, 0, 0
	m.problem, m.notice = "", ""
}

// unwind returns to the surface that started the finished action, dropping any
// target that disappeared.
func (m *catalogModel) unwind() {
	if m.origin < len(m.stack) {
		m.restore(m.stack[m.origin])
		m.stack = m.stack[:m.origin]
	}
	m.valid()
}

func (m *catalogModel) valid() {
	if m.screen == catalogDetails {
		if _, ok := m.item(); !ok {
			if !m.pop() {
				m.screen = catalogList
			}
		}
	}
}

func (m *catalogModel) back() {
	m.closeReview()
	m.seq++
	m.problem = ""
	if !m.pop() {
		m.screen = catalogList
	}
	m.valid()
}

func (m *catalogModel) goTo(view catalogView, source string) {
	if m.screen == catalogMenu {
		m.pop()
	}
	m.push()
	m.view, m.source, m.query = view, source, ""
	m.screen = catalogList
	m.cursor, m.offset, m.scroll = 0, 0, 0
	m.problem, m.notice = "", ""
}

func (m *catalogModel) switchTab() {
	if len(m.stack) > 0 || m.view == viewAvailable {
		return
	}
	next := viewInstalled
	if m.view == viewInstalled {
		next = viewSources
	}
	m.view, m.source, m.query = next, "", ""
	m.cursor, m.offset = 0, 0
	m.problem, m.notice = "", ""
}

// targets is the selection set, or every row currently shown.
func (m *catalogModel) targets() []string {
	ids := []string{}
	for _, e := range m.list() {
		if m.picked[e.ID] {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) > 0 {
		return ids
	}
	for _, e := range m.items() {
		ids = append(ids, e.ID)
	}
	return ids
}

func (m *catalogModel) bulk() []CatalogAction {
	switch m.view {
	case viewSources:
		return m.spec.SourceActions
	case viewAvailable:
		return nil
	}
	return m.spec.InstalledActions
}

func (m *catalogModel) prepare(req CatalogRequest, label string) tea.Cmd {
	m.closeReview()
	m.busy = true
	m.working = label
	m.problem, m.notice = "", ""
	m.seq++
	seq := m.seq
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	return func() tea.Msg {
		review, err := m.spec.Prepare(ctx, req)
		return catalogResultMsg{seq: seq, review: review, err: err}
	}
}

func (m *catalogModel) run(req CatalogRequest, label string) tea.Cmd {
	m.busy = true
	m.working = label
	m.problem, m.notice = "", ""
	m.seq++
	seq := m.seq
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	return func() tea.Msg {
		text, err := m.spec.Run(ctx, req)
		return m.reload(seq, text, err)
	}
}

func (m *catalogModel) reload(seq int, text string, err error) tea.Msg {
	data, readErr := m.spec.Load(m.ctx)
	if err == nil {
		err = readErr
	}
	if readErr != nil {
		return catalogResultMsg{seq: seq, text: text, err: err, done: true}
	}
	return catalogResultMsg{seq: seq, data: &data, text: text, err: err, done: true}
}

func (m *catalogModel) apply() tea.Cmd {
	m.busy = true
	m.working = "Applying " + strings.ToLower(m.review.Accept) + "…"
	m.problem = ""
	m.seq++
	seq := m.seq
	r, authorized := m.review, m.authorized
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	return func() tea.Msg {
		text, err := r.Apply(ctx, authorized)
		return m.reload(seq, text, err)
	}
}

func (m *catalogModel) activate(a CatalogAction, ids []string) tea.Cmd {
	switch {
	case a.Go != "":
		view, source := viewAvailable, ""
		if a.Go == "installed" {
			view = viewInstalled
		}
		if item, ok := m.item(); ok && m.screen == catalogDetails && m.view == viewSources {
			source = item.ID
		}
		m.goTo(view, source)
		return nil
	case a.Form:
		if m.screen == catalogMenu {
			m.pop()
		}
		m.descend(catalogForm)
		m.field = 0
		m.inputAt = len([]rune(m.fieldValue()))
		return nil
	case len(ids) == 0:
		m.problem = "Nothing to act on."
		return nil
	case a.Direct:
		if m.screen == catalogList || m.screen == catalogDetails {
			m.origin = len(m.stack)
		}
		return m.run(CatalogRequest{IDs: ids, Action: a.ID}, a.Label+"…")
	}
	return m.prepare(CatalogRequest{IDs: ids, Action: a.ID}, a.Label+"…")
}

func (m *catalogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.absorb(msg) {
		return m, nil
	}
	switch v := msg.(type) {
	case catalogResultMsg:
		if v.seq != m.seq {
			if v.review != nil && v.review.Close != nil {
				v.review.Close()
			}
			return m, nil
		}
		m.busy = false
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if v.data != nil {
			m.data = *v.data
			m.prune()
		}
		if v.done {
			m.closeReview()
			m.unwind()
		}
		if v.err != nil {
			if v.review != nil && v.review.Close != nil {
				v.review.Close()
			}
			m.problem = m.errorText(v.err)
			m.notice = v.text
			if v.done && strings.Contains(v.text, "\n") {
				m.notice = ""
				m.showResult(v.text)
			}
			return m, nil
		}
		m.problem = ""
		if v.review != nil {
			m.review = v.review
			m.descend(catalogReview)
		}
		if strings.Contains(v.text, "\n") {
			m.showResult(v.text)
		} else if v.text != "" {
			m.notice = v.text
		}
		return m, nil
	case tea.PasteMsg:
		if !m.busy {
			if m.screen == catalogForm {
				m.insert(v.Content)
			} else if m.searching {
				m.query += collapseToLine(v.Content)
				m.cursor, m.offset = 0, 0
			}
		}
	case tea.KeyPressMsg:
		return m.key(v)
	}
	return m, nil
}

func (m *catalogModel) showResult(text string) {
	m.result = text
	m.push()
	m.screen = catalogResult
	m.scroll, m.control = 0, 0
}

func (m *catalogModel) key(v tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := v.String()
	if m.busy {
		if key == "esc" || key == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
			}
		}
		return m, nil
	}
	if key == "ctrl+c" {
		m.closeReview()
		return m, tea.Quit
	}
	switch m.screen {
	case catalogForm:
		return m, m.formKey(v)
	case catalogList:
		return m.listKey(v, key)
	case catalogDetails:
		return m.detailsKey(key)
	case catalogMenu:
		return m.menuKey(key)
	case catalogResult:
		switch key {
		case "esc", "enter":
			m.back()
		case "pgdown":
			m.scroll += max(1, m.h/2)
		case "pgup":
			m.scroll = max(0, m.scroll-max(1, m.h/2))
		case "home":
			m.scroll = 0
		case "end":
			m.scroll = 1 << 20
		}
	case catalogReview:
		return m.reviewKey(key)
	}
	return m, nil
}

func (m *catalogModel) listKey(v tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	items := m.items()
	if m.searching {
		switch key {
		case "esc":
			m.query = ""
			m.searching = false
		case "enter", "down":
			m.searching = false
		case "backspace":
			r := []rune(m.query)
			if len(r) > 0 {
				m.query = string(r[:len(r)-1])
			}
		default:
			if v.Text != "" {
				m.query += v.Text
			}
		}
		m.cursor, m.offset = 0, 0
		return m, nil
	}
	switch key {
	case "esc":
		if m.query != "" {
			m.query = ""
			m.cursor, m.offset = 0, 0
			return m, nil
		}
		if len(m.stack) > 0 {
			m.back()
			return m, nil
		}
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case "left", "shift+tab":
		m.switchTab()
	case "right", "tab":
		m.switchTab()
	case "r":
		m.notice, m.problem = "", ""
		return m, m.load()
	case "/":
		m.searching = true
	case "f":
		m.cycleSource()
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(max(0, len(items)-1), m.cursor+1)
	case "pgdown":
		m.cursor = min(max(0, len(items)-1), m.cursor+max(1, m.h-10))
	case "pgup":
		m.cursor = max(0, m.cursor-max(1, m.h-10))
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = max(0, len(items)-1)
	case "enter":
		if len(items) > 0 {
			m.cursor = min(m.cursor, len(items)-1)
			m.push()
			m.selected = items[m.cursor].ID
			m.screen = catalogDetails
			m.action, m.scroll = 0, 0
			m.notice, m.problem = "", ""
		}
	case "space":
		if m.selectable() && len(items) > 0 {
			id := items[min(m.cursor, len(items)-1)].ID
			if m.picked[id] {
				delete(m.picked, id)
			} else {
				m.picked[id] = true
			}
		}
	case "s":
		if m.selectable() {
			all := len(items) > 0
			for _, e := range items {
				all = all && m.picked[e.ID]
			}
			for _, e := range items {
				if all {
					delete(m.picked, e.ID)
				} else {
					m.picked[e.ID] = true
				}
			}
		}
	case "a":
		if len(m.bulk()) > 0 {
			m.descend(catalogMenu)
		}
	}
	return m, nil
}

func (m *catalogModel) detailsKey(key string) (tea.Model, tea.Cmd) {
	item, ok := m.item()
	if !ok {
		m.back()
		return m, nil
	}
	switch key {
	case "esc":
		m.back()
	case "up", "k":
		m.action = max(0, m.action-1)
	case "down", "j":
		m.action = min(max(0, len(item.Actions)-1), m.action+1)
	case "enter":
		if len(item.Actions) > 0 {
			a := item.Actions[min(m.action, len(item.Actions)-1)]
			return m, m.activate(a, []string{item.ID})
		}
	case "pgdown":
		m.scroll += max(1, m.h/2)
	case "pgup":
		m.scroll = max(0, m.scroll-max(1, m.h/2))
	case "home":
		m.scroll = 0
	case "end":
		m.scroll = 1 << 20
	}
	return m, nil
}

func (m *catalogModel) menuKey(key string) (tea.Model, tea.Cmd) {
	actions := m.bulk()
	switch key {
	case "esc":
		m.back()
	case "up", "k":
		m.action = max(0, m.action-1)
	case "down", "j":
		m.action = min(max(0, len(actions)-1), m.action+1)
	case "enter":
		if len(actions) > 0 {
			return m, m.activate(actions[min(m.action, len(actions)-1)], m.targets())
		}
	}
	return m, nil
}

func (m *catalogModel) reviewKey(key string) (tea.Model, tea.Cmd) {
	if m.review == nil {
		m.back()
		return m, nil
	}
	count := 2
	if m.review.Apply == nil {
		count = 1
	}
	if len(m.review.Dependencies) > 0 {
		count++
	}
	switch key {
	case "esc":
		m.back()
	case "up":
		m.control = max(0, m.control-1)
	case "down", "tab":
		m.control = (m.control + 1) % count
	case "space", "enter":
		if m.control == 0 {
			m.back()
			return m, nil
		}
		if len(m.review.Dependencies) > 0 && m.control == 1 {
			m.authorized = !m.authorized
			return m, nil
		}
		if len(m.review.Dependencies) > 0 && !m.authorized {
			m.problem = "Authorize the listed policy-reference removal first."
			return m, nil
		}
		if m.review.Apply != nil {
			return m, m.apply()
		}
	case "pgdown":
		m.scroll += max(1, m.h/2)
	case "pgup":
		m.scroll = max(0, m.scroll-max(1, m.h/2))
	case "home":
		m.scroll = 0
	case "end":
		m.scroll = 1 << 20
	}
	return m, nil
}

func (m *catalogModel) visibleFields() []CatalogField {
	out := []CatalogField{}
	for _, f := range m.spec.Fields {
		if f.Visible == nil || f.Visible(m.values) {
			out = append(out, f)
		}
	}
	return out
}

func (m *catalogModel) fieldValue() string {
	fields := m.visibleFields()
	if m.field >= len(fields) {
		return ""
	}
	return m.values[fields[m.field].Key]
}

func (m *catalogModel) insert(text string) {
	fields := m.visibleFields()
	if m.field >= len(fields) {
		return
	}
	f := fields[m.field]
	if len(f.Choices) > 0 {
		return
	}
	r := []rune(m.fieldValue())
	at := min(m.inputAt, len(r))
	p := []rune(collapseToLine(text))
	m.values[f.Key] = string(append(append(append([]rune{}, r[:at]...), p...), r[at:]...))
	m.inputAt = at + len(p)
	m.problem = ""
}

func (m *catalogModel) formKey(key tea.KeyPressMsg) tea.Cmd {
	fields := m.visibleFields()
	n := len(fields)
	m.field = min(m.field, n)
	switch key.String() {
	case "esc":
		m.back()
		return nil
	case "tab", "down":
		m.field = (m.field + 1) % (n + 1)
		m.inputAt = len([]rune(m.fieldValue()))
		return nil
	case "shift+tab", "up":
		m.field = (m.field + n) % (n + 1)
		m.inputAt = len([]rune(m.fieldValue()))
		return nil
	case "enter":
		if m.field < n {
			m.field++
			m.inputAt = len([]rune(m.fieldValue()))
			return nil
		}
		for i, f := range fields {
			if f.Validate != nil {
				if err := f.Validate(m.values[f.Key]); err != nil {
					m.problem = m.errorText(err)
					m.field = i
					m.inputAt = len([]rune(m.fieldValue()))
					return nil
				}
			}
		}
		values := map[string]string{}
		for _, f := range fields {
			values[f.Key] = m.values[f.Key]
		}
		return m.prepare(CatalogRequest{Action: m.spec.FormAction, Values: values}, "Checking the source…")
	}
	if m.field >= n {
		return nil
	}
	f := fields[m.field]
	if len(f.Choices) > 0 {
		if key.String() == "space" || key.String() == "right" || key.String() == "left" {
			i := 0
			for j, c := range f.Choices {
				if c == m.values[f.Key] {
					i = j
				}
			}
			d := 1
			if key.String() == "left" {
				d = len(f.Choices) - 1
			}
			m.values[f.Key] = f.Choices[(i+d)%len(f.Choices)]
		}
		return nil
	}
	r := []rune(m.fieldValue())
	m.inputAt = min(m.inputAt, len(r))
	switch key.String() {
	case "left":
		m.inputAt = max(0, m.inputAt-1)
	case "right":
		m.inputAt = min(len(r), m.inputAt+1)
	case "home", "ctrl+a":
		m.inputAt = 0
	case "end", "ctrl+e":
		m.inputAt = len(r)
	case "ctrl+u":
		r = nil
		m.inputAt = 0
	case "ctrl+k":
		r = r[:m.inputAt]
	case "backspace":
		if m.inputAt > 0 {
			r = append(r[:m.inputAt-1], r[m.inputAt:]...)
			m.inputAt--
		}
	case "delete":
		if m.inputAt < len(r) {
			r = append(r[:m.inputAt], r[m.inputAt+1:]...)
		}
	default:
		if key.Text != "" {
			m.insert(key.Text)
			return nil
		}
	}
	m.values[f.Key] = string(r)
	m.problem = ""
	return nil
}

func (m *catalogModel) focused(text string, active bool) string {
	if !active {
		return "  " + text
	}
	line := TruncTail("› "+text, max(1, m.w-4))
	if m.cap.ColorEnabled {
		return m.th.Primary.Reverse(true).Width(max(1, m.w-4)).Render(line)
	}
	return line
}

func (m *catalogModel) contentWindow(text string, height int) string {
	lines := strings.Split(ansi.Hardwrap(text, max(1, m.w-4), false), "\n")
	m.scroll = min(max(0, m.scroll), max(0, len(lines)-height))
	end := min(len(lines), m.scroll+height)
	return strings.Join(lines[m.scroll:end], "\n")
}

func (m *catalogModel) itemDetails(item CatalogItem) string {
	var b strings.Builder
	if ansi.StringWidth(item.Name) > max(1, m.w-15) {
		b.WriteString(item.Name + "\n\n")
	}
	b.WriteString(item.Details)
	labelWidth := 9
	for _, p := range item.Properties {
		labelWidth = max(labelWidth, ansi.StringWidth(p.Label)+1)
	}
	for _, p := range item.Properties {
		pad := strings.Repeat(" ", labelWidth-ansi.StringWidth(p.Label))
		b.WriteString("\n" + m.th.Muted.Render(p.Label+pad) + p.Value)
	}
	for _, section := range item.Sections {
		b.WriteString("\n\n" + m.th.Text.Bold(true).Render(section.Title) + "\n" + section.Content)
	}
	return b.String()
}

// totals states complete local counts. They ignore search and the source filter.
func (m *catalogModel) totals() string {
	switch m.view {
	case viewSources:
		return fmt.Sprintf("%d marketplaces", len(m.data.Sources))
	case viewAvailable:
		have, total := 0, 0
		for _, e := range m.data.Available {
			if m.source != "" && e.Source != m.source {
				continue
			}
			total++
			if e.Installed {
				have++
			}
		}
		return fmt.Sprintf("%d available · %d installed", total, have)
	}
	enabled := 0
	for _, e := range m.data.Installed {
		if e.Enabled {
			enabled++
		}
	}
	return fmt.Sprintf("%d installed · %d enabled", len(m.data.Installed), enabled)
}

func (m *catalogModel) viewName() string {
	switch m.view {
	case viewSources:
		return "Marketplaces"
	case viewAvailable:
		return "Available plugins"
	}
	return "Installed"
}

func (m *catalogModel) View() tea.View {
	title := m.spec.Title
	switch m.screen {
	case catalogDetails:
		if item, ok := m.item(); ok {
			title += " › " + m.viewName() + " › " + item.Name
		}
	case catalogReview:
		if m.review != nil {
			title += " › " + m.review.Title
		}
	case catalogMenu:
		title += " › " + m.viewName() + " › Actions"
	case catalogForm:
		title += " › " + m.spec.FormTitle
	case catalogResult:
		title += " › Results"
	default:
		if m.view == viewAvailable {
			title += " › " + m.viewName()
		}
	}
	labels := []string{fmt.Sprintf("Installed (%d)", len(m.data.Installed)), fmt.Sprintf("Marketplaces (%d)", len(m.data.Sources))}
	active := 0
	if m.view != viewInstalled {
		active = 1
	}
	if m.view == viewInstalled && m.source != "" && len(m.stack) > 0 {
		active = 0
	}
	tabs := []string{}
	for i, label := range labels {
		if i == active {
			tabs = append(tabs, m.th.Primary.Bold(true).Underline(true).Render("["+label+"]"))
		} else {
			tabs = append(tabs, m.th.Muted.Render(label))
		}
	}
	header := m.th.Text.Bold(true).Render(title) + "\n\n" + strings.Join(tabs, "   ") + "\n\n"
	footer := ""
	var body string
	statusText := m.notice
	if m.problem != "" {
		statusText = strings.TrimSpace(m.notice + " " + m.problem)
	}
	statusText = ansi.Hardwrap(statusText, max(1, m.w-4), false)
	statusLines := strings.Split(statusText, "\n")
	if len(statusLines) > 2 {
		statusLines = statusLines[:2]
		statusLines[1] = TruncTail(statusLines[1]+"…", max(1, m.w-4))
	}
	statusText = strings.Join(statusLines, "\n")
	height := max(1, m.h-10-max(0, len(statusLines)-1))
	switch m.screen {
	case catalogList:
		body, footer = m.listBody(height)
	case catalogDetails:
		footer = "↑↓ action · Enter select\nPgUp/PgDn details · Esc back"
		if item, ok := m.item(); ok {
			actions := []string{}
			for i, a := range item.Actions {
				actions = append(actions, m.focused(a.Label, i == m.action))
			}
			body = m.contentWindow(m.itemDetails(item), max(1, height-len(actions)-2)) + "\n\n" + strings.Join(actions, "\n")
		}
	case catalogMenu:
		footer = "↑↓ action · Enter select\nEsc back"
		scope := fmt.Sprintf("Applies to all %d shown", len(m.items()))
		if n := len(m.picked); n > 0 {
			scope = fmt.Sprintf("Applies to %d selected", n)
		}
		lines := []string{m.th.Muted.Render(scope), ""}
		for i, a := range m.bulk() {
			lines = append(lines, m.focused(a.Label, i == m.action))
		}
		body = strings.Join(lines, "\n")
	case catalogForm:
		footer = "Tab/Shift+Tab field · ←→ cursor · Space mode\nEnter next/review · Esc back"
		body = m.formBody(height)
	case catalogResult:
		footer = "PgUp/PgDn results · Enter or Esc back"
		body = m.contentWindow(m.result, max(1, height-3)) + "\n\n" + m.focused("Back", true)
	case catalogReview:
		footer = "↑↓ action · Enter select\nPgUp/PgDn review · Esc back"
		if m.review != nil {
			body, footer = m.reviewBody(height)
		}
	}
	status := m.th.Success.Render(statusText)
	if m.problem != "" {
		status = m.th.Danger.Render(statusText)
	}
	if m.busy {
		status = m.th.Muted.Render(m.working)
		footer = "Please wait; Esc stops the work when it is still safe."
	}
	bodyLines := strings.Split(body, "\n")
	if len(bodyLines) > height {
		bodyLines = bodyLines[:height]
	}
	for len(bodyLines) < height {
		bodyLines = append(bodyLines, "")
	}
	text := header + strings.Join(bodyLines, "\n") + "\n\n" + status + "\n" + m.th.Muted.Render(footer)
	view := m.clamp(text)
	view.AltScreen = true
	return view
}

func (m *catalogModel) listBody(height int) (string, string) {
	footer := "↑↓ select · Enter details · Space mark · a actions · f source\n/ search · r refresh · ←→ view · Esc close"
	switch m.view {
	case viewAvailable:
		footer = "↑↓ select · Enter details · f source\n/ search · r refresh · Esc back"
	case viewSources:
		footer = "↑↓ select · Enter details · Space mark · a actions\n/ search · r refresh · ←→ view · Esc close"
	}
	if len(m.stack) > 0 {
		footer = strings.Replace(footer, " · ←→ view", "", 1)
		footer = strings.Replace(footer, "Esc close", "Esc back", 1)
	}
	search := "/ Search…"
	if m.query != "" || m.searching {
		search = "/ " + m.query
		if m.searching {
			search += "█"
		}
	}
	stats := m.totals()
	if m.source != "" {
		stats += " · Source: " + m.source
	}
	if n := len(m.picked); n > 0 && m.selectable() {
		stats += fmt.Sprintf(" · %d selected", n)
	}
	body := m.th.Muted.Render(search) + "\n" + m.th.Muted.Render(TruncTail(stats, max(1, m.w-4))) + "\n"
	items := m.items()
	rows := max(1, height-2)
	m.cursor, m.offset = clampScroll(m.cursor, m.offset, len(items), rows)
	if len(items) == 0 {
		switch {
		case m.query != "" || m.source != "":
			body += "No matching entries."
		case m.view == viewAvailable:
			body += "No plugins are offered. Refresh the marketplace catalog."
		case m.view == viewSources:
			body += "No marketplaces are known."
		default:
			body += m.spec.Empty
		}
	}
	nameWidth := max(1, m.w-30)
	for i := m.offset; i < min(len(items), m.offset+rows); i++ {
		e := items[i]
		state := e.Badge
		if state == "" {
			state = "Available"
			if e.Installed {
				state = "Disabled"
				if e.Enabled {
					state = "Enabled"
				}
			}
		}
		mark := "   "
		if m.selectable() {
			mark = "[ ]"
			if m.picked[e.ID] {
				mark = "[x]"
			}
		}
		name := TruncTail(e.Name, nameWidth)
		padding := strings.Repeat(" ", max(1, nameWidth-ansi.StringWidth(name)+2))
		if i != m.cursor {
			if e.Installed && e.Enabled {
				state = m.th.Success.Render(state)
			} else {
				state = m.th.Muted.Render(state)
			}
		}
		body += m.focused(mark+" "+name+padding+state, i == m.cursor) + "\n"
	}
	if len(items) > rows {
		footer = fmt.Sprintf("%d–%d of %d · PgUp/PgDn scroll\n", m.offset+1, min(len(items), m.offset+rows), len(items)) + footer
	}
	return body, footer
}

func (m *catalogModel) formBody(height int) string {
	fields := m.visibleFields()
	lines := []string{}
	for i, f := range fields {
		value := m.values[f.Key]
		editing := i == m.field
		if len(f.Choices) > 0 {
			if value == "" {
				value = f.Choices[0]
			}
			value = "‹ " + value + " ›"
		} else if f.Secret {
			value = strings.Repeat("•", len([]rune(value)))
		}
		if editing && len(f.Choices) == 0 {
			r := []rune(value)
			at := min(m.inputAt, len(r))
			available := max(2, m.w-ansi.StringWidth(f.Label)-8)
			before := string(r[:at])
			if ansi.StringWidth(before) >= available {
				before = ansi.TruncateLeft(before, ansi.StringWidth(before)-available+2, "…")
			}
			value = before + "█" + string(r[at:])
		}
		lines = append(lines, m.focused(f.Label+": "+value, editing), "  "+m.th.Muted.Render(f.Hint), "")
	}
	lines = append(lines, m.focused("Review registration", m.field >= len(fields)))
	at := min(m.field*3, max(0, len(lines)-1))
	offset := max(0, at-height+1)
	return strings.Join(lines[offset:min(len(lines), offset+height)], "\n")
}

func (m *catalogModel) reviewBody(height int) (string, string) {
	footer := "↑↓ action · Enter select\nPgUp/PgDn review · Esc back"
	controls := []string{m.focused("Back", m.control == 0)}
	content := m.review.Content
	if len(m.review.Dependencies) > 0 {
		footer = "↑↓ action · Enter select · Space authorize\nPgUp/PgDn review · Esc back"
		content += "\n\nPolicy references to remove:\n" + strings.Join(m.review.Dependencies, "\n")
		mark := "[ ]"
		if m.authorized {
			mark = "[x]"
		}
		controls = append(controls, m.focused(mark+" Authorize removal of listed references", m.control == 1))
	}
	if m.review.Apply != nil {
		label := m.focused(m.review.Accept, m.control == len(controls))
		if m.review.Destructive && m.cap.ColorEnabled {
			style := m.th.Danger.Bold(true)
			text := "  " + m.review.Accept
			if m.control == len(controls) {
				style = style.Reverse(true).Width(max(1, m.w-4))
				text = "› " + m.review.Accept
			}
			label = style.Render(text)
		}
		controls = append(controls, label)
	}
	return m.contentWindow(content, max(1, height-len(controls)-2)) + "\n\n" + strings.Join(controls, "\n"), footer
}

// Catalog runs one terminal lifecycle and never prints receipts. Its caller
// owns the single normal-exit acknowledgment, distinct from direct CLI output.
func Catalog(ctx context.Context, pio IO, spec CatalogSpec) error {
	m := newCatalog(ctx, pio, spec)
	defer m.closeReview()
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(pio.in()), tea.WithOutput(pio.ui())).Run()
	return err
}
