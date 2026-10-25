package present

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Catalog is a persistent inventory interface, independent of Wizard receipts.
// Callbacks own domain rules and return display-safe text. Review resources live
// until application or Back; no callback writes to the terminal.
type CatalogProperty struct{ Label, Value string }
type CatalogSection struct{ Title, Content string }
type CatalogItem struct {
	ID, Name, Details  string
	Installed, Enabled bool
	Actions            []CatalogAction
	Properties         []CatalogProperty
	Sections           []CatalogSection
}
type CatalogAction struct{ ID, Label string }
type CatalogData struct{ Installed, Official []CatalogItem }
type CatalogField struct {
	Key, Label, Hint string
	Secret           bool
	Validate         func(string) error
	Choices          []string
}
type CatalogRequest struct {
	ID, Action string
	Values     map[string]string
}
type CatalogReview struct {
	Title, Content, Accept string
	Destructive            bool
	Dependencies           []string
	Apply                  func(context.Context, bool) (string, error)
	Close                  func()
}
type CatalogSpec struct {
	Title, Empty string
	Fields       []CatalogField
	Load         func(context.Context) (CatalogData, error)
	Toggle       func(context.Context, string, bool) error
	Prepare      func(context.Context, CatalogRequest) (*CatalogReview, error)
	ErrorText    func(error) string
}

type catalogScreen int

const (
	catalogList catalogScreen = iota
	catalogDetails
	catalogForm
	catalogReview
)

type catalogResultMsg struct {
	data    *CatalogData
	review  *CatalogReview
	text    string
	applied bool
	err     error
}
type catalogModel struct {
	baseFrame
	ctx                                                          context.Context
	spec                                                         CatalogSpec
	data                                                         CatalogData
	screen                                                       catalogScreen
	tab, cursor, offset, action, scroll, field, inputAt, control int
	query                                                        string
	searching                                                    bool
	editingForm                                                  bool
	selected                                                     string
	values                                                       map[string]string
	review                                                       *CatalogReview
	authorized, busy                                             bool
	notice, problem                                              string
	cancel                                                       context.CancelFunc
}

func newCatalog(ctx context.Context, pio IO, spec CatalogSpec) *catalogModel {
	return &catalogModel{baseFrame: newBaseFrame(pio), ctx: ctx, spec: spec, values: map[string]string{}}
}
func (m *catalogModel) Init() tea.Cmd { return tea.Batch(tea.RequestBackgroundColor, m.load()) }
func (m *catalogModel) load() tea.Cmd {
	m.busy = true
	return func() tea.Msg {
		data, err := m.spec.Load(m.ctx)
		if err != nil {
			return catalogResultMsg{err: err}
		}
		return catalogResultMsg{data: &data}
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
func (m *catalogModel) items() []CatalogItem {
	entries := m.data.Installed
	if m.tab == 1 {
		entries = m.data.Official
	}
	out := []CatalogItem{}
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name), strings.ToLower(m.query)) {
			out = append(out, e)
		}
	}
	return out
}
func (m *catalogModel) item() (CatalogItem, bool) {
	for _, entries := range [][]CatalogItem{m.data.Installed, m.data.Official} {
		for _, e := range entries {
			if e.ID == m.selected {
				return e, true
			}
		}
	}
	return CatalogItem{}, false
}
func (m *catalogModel) selectTab(tab int) {
	m.closeReview()
	m.tab = (tab + 3) % 3
	m.screen = catalogList
	if m.tab == 2 {
		m.screen = catalogForm
	}
	m.cursor, m.offset, m.scroll, m.field, m.inputAt = 0, 0, 0, 0, 0
	m.query = ""
	m.searching = false
	m.editingForm = false
	m.problem = ""
	m.notice = ""
	m.inputAt = len([]rune(m.fieldValue()))
}
func (m *catalogModel) start(request CatalogRequest) tea.Cmd {
	m.closeReview()
	m.busy = true
	m.problem = ""
	m.notice = ""
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	return func() tea.Msg {
		review, err := m.spec.Prepare(ctx, request)
		return catalogResultMsg{review: review, err: err}
	}
}
func (m *catalogModel) toggle(item CatalogItem) tea.Cmd {
	m.busy = true
	m.problem = ""
	m.notice = ""
	return func() tea.Msg {
		err := m.spec.Toggle(m.ctx, item.ID, !item.Enabled)
		data, readErr := m.spec.Load(m.ctx)
		if err == nil {
			err = readErr
		}
		state := "enabled"
		if item.Enabled {
			state = "disabled"
		}
		text := ""
		if readErr == nil {
			for _, entry := range data.Installed {
				if entry.ID == item.ID && entry.Enabled == !item.Enabled {
					text = item.Name + " " + state + "."
				}
			}
		}
		if readErr != nil {
			return catalogResultMsg{err: err, text: text}
		}
		return catalogResultMsg{data: &data, err: err, text: text}
	}
}
func (m *catalogModel) back() {
	m.closeReview()
	m.problem = ""
	m.scroll = 0
	m.control = 0
	switch m.screen {
	case catalogDetails:
		m.screen = catalogList
	case catalogReview:
		if m.tab == 2 {
			m.screen = catalogForm
			m.editingForm = true
		} else if _, ok := m.item(); ok {
			m.screen = catalogDetails
		} else {
			m.screen = catalogList
		}
	case catalogForm:
		m.selectTab(0)
	}
}
func (m *catalogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.absorb(msg) {
		return m, nil
	}
	switch v := msg.(type) {
	case catalogResultMsg:
		m.busy = false
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if v.data != nil {
			m.data = *v.data
		}
		if v.applied {
			m.closeReview()
			m.scroll = 0
			m.control = 0
			if m.tab == 2 {
				m.selectTab(0)
			} else {
				m.screen = catalogDetails
			}
			if _, ok := m.item(); !ok {
				m.screen = catalogList
			}
		}
		if v.err != nil {
			if v.review != nil && v.review.Close != nil {
				v.review.Close()
			}
			m.closeReview()
			m.problem = m.errorText(v.err)
			if v.text != "" {
				m.notice = v.text
			}
			return m, nil
		}
		m.problem = ""
		if v.review != nil {
			m.review = v.review
			m.screen = catalogReview
			m.scroll = 0
			m.control = 0
		}
		if v.text != "" {
			m.notice = v.text
		}
		if m.screen == catalogDetails {
			if _, ok := m.item(); !ok {
				m.screen = catalogList
			}
		}
		return m, nil
	case tea.PasteMsg:
		if !m.busy {
			if m.screen == catalogForm {
				m.editingForm = true
				m.insert(v.Content)
			} else if m.searching {
				m.query += collapseToLine(v.Content)
				m.cursor = 0
				m.offset = 0
			}
		}
	case tea.KeyPressMsg:
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
		if m.screen == catalogForm {
			if !m.editingForm {
				switch key {
				case "left", "shift+tab":
					m.selectTab(m.tab - 1)
				case "right":
					m.selectTab(m.tab + 1)
				case "enter", "tab", "down":
					m.editingForm = true
				case "esc":
					m.back()
				default:
					if v.Text != "" {
						m.editingForm = true
						return m, m.formKey(v)
					}
				}
				return m, nil
			}
			if key == "esc" {
				m.editingForm = false
				m.problem = ""
				return m, nil
			}
			return m, m.formKey(v)
		}
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
		if key == "esc" {
			if m.screen == catalogList {
				if m.query != "" {
					m.query = ""
					m.cursor = 0
					m.offset = 0
					return m, nil
				}
				return m, tea.Quit
			}
			m.back()
			return m, nil
		}
		if m.screen == catalogList {
			items := m.items()
			switch key {
			case "q":
				return m, tea.Quit
			case "left", "shift+tab":
				m.selectTab(m.tab - 1)
			case "right", "tab":
				m.selectTab(m.tab + 1)
			case "r":
				m.notice = ""
				m.problem = ""
				return m, m.load()
			case "i":
				m.selectTab(2)
				m.editingForm = true
			case "/":
				m.searching = true
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
					m.selected = items[m.cursor].ID
					m.screen = catalogDetails
					m.action = 0
					m.scroll = 0
					m.notice = ""
					m.problem = ""
				}
			case "space":
				if len(items) > 0 {
					item := items[min(m.cursor, len(items)-1)]
					if item.Installed {
						return m, m.toggle(item)
					}
				}
			}
			return m, nil
		}
		if m.screen == catalogDetails {
			item, ok := m.item()
			if !ok {
				m.screen = catalogList
				return m, nil
			}
			switch key {
			case "up", "k":
				m.action = max(0, m.action-1)
			case "down", "j":
				m.action = min(max(0, len(item.Actions)-1), m.action+1)
			case "space":
				if item.Installed {
					return m, m.toggle(item)
				}
			case "enter":
				if len(item.Actions) > 0 {
					a := item.Actions[min(m.action, len(item.Actions)-1)]
					if a.ID == "toggle" {
						return m, m.toggle(item)
					}
					return m, m.start(CatalogRequest{ID: item.ID, Action: a.ID})
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
		if m.screen == catalogReview && m.review != nil {
			count := 2
			if m.review.Apply == nil {
				count = 1
			}
			if len(m.review.Dependencies) > 0 {
				count++
			}
			switch key {
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
					m.busy = true
					m.problem = ""
					r, authorized := m.review, m.authorized
					ctx, cancel := context.WithCancel(m.ctx)
					m.cancel = cancel
					return m, func() tea.Msg {
						text, err := r.Apply(ctx, authorized)
						data, readErr := m.spec.Load(m.ctx)
						if err == nil {
							err = readErr
						}
						if readErr != nil {
							return catalogResultMsg{text: text, err: err, applied: true}
						}
						return catalogResultMsg{data: &data, text: text, err: err, applied: true}
					}
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
		}
	}
	return m, nil
}
func (m *catalogModel) fieldValue() string {
	if m.field >= len(m.spec.Fields) {
		return ""
	}
	return m.values[m.spec.Fields[m.field].Key]
}
func (m *catalogModel) insert(text string) {
	if m.field >= len(m.spec.Fields) {
		return
	}
	f := m.spec.Fields[m.field]
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
	n := len(m.spec.Fields)
	if key.String() == "esc" {
		m.back()
		return nil
	}
	switch key.String() {
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
		for i, f := range m.spec.Fields {
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
		for k, v := range m.values {
			values[k] = v
		}
		return m.start(CatalogRequest{Action: "install", Values: values})
	}
	if m.field >= n {
		return nil
	}
	f := m.spec.Fields[m.field]
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
	for _, p := range item.Properties {
		b.WriteString("\n" + m.th.Muted.Render(fmt.Sprintf("%-9s", p.Label)) + p.Value)
	}
	for _, section := range item.Sections {
		b.WriteString("\n\n" + m.th.Text.Bold(true).Render(section.Title) + "\n" + section.Content)
	}
	return b.String()
}

func (m *catalogModel) View() tea.View {
	title := m.spec.Title
	if m.screen == catalogDetails {
		if item, ok := m.item(); ok {
			title += " › " + item.Name
		}
	}
	if m.screen == catalogReview && m.review != nil {
		title += " › " + m.review.Title
	}
	tabs := []string{fmt.Sprintf("Installed (%d)", len(m.data.Installed)), "Official", "Install plugin"}
	for i, s := range tabs {
		if i == m.tab {
			tabs[i] = m.th.Primary.Bold(true).Underline(true).Render("[" + s + "]")
		} else {
			tabs[i] = m.th.Muted.Render(s)
		}
	}
	header := m.th.Text.Bold(true).Render(title) + "\n\n" + strings.Join(tabs, "   ") + "\n\n"
	footer := "↑↓ select · Enter details · Space toggle · ←→ tabs\n/ search · i install · r refresh · Esc close"
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
		search := "/ Search plugins…"
		if m.query != "" || m.searching {
			search = "/ " + m.query
			if m.searching {
				search += "█"
			}
		}
		body = m.th.Muted.Render(search) + "\n\n"
		items := m.items()
		rows := max(1, height-2)
		m.cursor, m.offset = clampScroll(m.cursor, m.offset, len(items), rows)
		if len(items) == 0 {
			if m.query != "" {
				body += "No matching plugins."
			} else if m.tab == 1 {
				body += "No official plugins available."
			} else {
				body += m.spec.Empty
			}
		}
		nameWidth := max(1, m.w-22)
		for i := m.offset; i < min(len(items), m.offset+rows); i++ {
			e := items[i]
			state := "Available"
			mark := "[ ]"
			if e.Installed {
				state = "Disabled"
				if e.Enabled {
					state = "Enabled"
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
	case catalogDetails:
		footer = "↑↓ action · Enter select · Space toggle\nPgUp/PgDn details · Esc back"
		if item, ok := m.item(); ok {
			if !item.Installed {
				footer = "↑↓ action · Enter select\nPgUp/PgDn details · Esc back"
			}
			actions := []string{}
			for i, a := range item.Actions {
				actions = append(actions, m.focused(a.Label, i == m.action))
			}
			body = m.contentWindow(m.itemDetails(item), max(1, height-len(actions)-2)) + "\n\n" + strings.Join(actions, "\n")
		}
	case catalogForm:
		footer = "←→ tabs · Enter/Tab edit form\nEsc back"
		if m.editingForm {
			footer = "Tab/Shift+Tab field · ←→ cursor · Space mode\nEnter next/review · Esc tabs"
		}
		lines := []string{}
		for i, f := range m.spec.Fields {
			value := m.values[f.Key]
			if len(f.Choices) > 0 {
				if value == "" {
					value = f.Choices[0]
				}
				value = "‹ " + value + " ›"
			} else if f.Secret {
				value = strings.Repeat("•", len([]rune(value)))
			}
			if m.editingForm && i == m.field && len(f.Choices) == 0 {
				r := []rune(value)
				at := min(m.inputAt, len(r))
				available := max(2, m.w-ansi.StringWidth(f.Label)-8)
				before := string(r[:at])
				if ansi.StringWidth(before) >= available {
					before = ansi.TruncateLeft(before, ansi.StringWidth(before)-available+2, "…")
				}
				value = before + "█" + string(r[at:])
			}
			lines = append(lines, m.focused(f.Label+": "+value, m.editingForm && i == m.field), "  "+m.th.Muted.Render(f.Hint), "")
		}
		lines = append(lines, m.focused("Review installation", m.editingForm && m.field == len(m.spec.Fields)))
		// Keep the focused field and the review control visible in small terminals.
		at := min(m.field*3, max(0, len(lines)-1))
		offset := max(0, at-height+1)
		body = strings.Join(lines[offset:min(len(lines), offset+height)], "\n")
	case catalogReview:
		footer = "↑↓ action · Enter select\nPgUp/PgDn review · Esc back"
		if m.review != nil {
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
			body = m.contentWindow(content, max(1, height-len(controls)-2)) + "\n\n" + strings.Join(controls, "\n")
		}
	}
	status := m.th.Success.Render(statusText)
	if m.problem != "" {
		status = m.th.Danger.Render(statusText)
	}
	if m.busy {
		status = m.th.Muted.Render("Working…")
		footer = "Please wait; Esc stops preparation when possible."
	}
	// Allocate fixed footer space so errors and Back stay visible after resize.
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

// Catalog runs one terminal lifecycle and never prints receipts. Its caller
// owns the single normal-exit acknowledgment, distinct from direct CLI output.
func Catalog(ctx context.Context, pio IO, spec CatalogSpec) error {
	m := newCatalog(ctx, pio, spec)
	defer m.closeReview()
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(pio.in()), tea.WithOutput(pio.ui())).Run()
	return err
}
