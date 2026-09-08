//go:build unix

package integration

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// TestVTAlternateScreen: entering the alt buffer hides the primary buffer,
// writes there leave no scrollback, and leaving it restores the primary buffer
// unchanged — the reprint a full-screen program does after exit then lands in
// the restored primary buffer.
func TestVTAlternateScreen(t *testing.T) {
	v := newVT(4, 20)
	v.write([]byte("primary line\r\n"))
	v.write([]byte("\x1b[?1049h"))
	v.write([]byte("full screen frame\r\nsecond frame line\r\n"))
	if got := v.String(); strings.Contains(got, "primary line") {
		t.Errorf("primary buffer leaked into the alt screen:\n%s", got)
	}
	if !strings.Contains(v.String(), "full screen frame") {
		t.Errorf("alt-screen content not shown:\n%s", v.String())
	}
	v.write([]byte("\x1b[?1049l"))
	after := v.String()
	if !strings.Contains(after, "primary line") {
		t.Errorf("primary buffer not restored after exiting the alt screen:\n%s", after)
	}
	for _, gone := range []string{"full screen frame", "second frame line"} {
		if strings.Contains(after, gone) {
			t.Errorf("alt-screen content %q survived into the primary buffer:\n%s", gone, after)
		}
	}
	// A post-exit reprint lands in the restored primary buffer.
	v.write([]byte("receipt reprint\r\n"))
	if !strings.Contains(v.String(), "receipt reprint") {
		t.Errorf("post-exit reprint missing:\n%s", v.String())
	}
}

// vt is a deliberately small terminal emulator: just enough of the cursor,
// erase, scroll, and alternate-screen semantics to reconstruct what bubbletea's
// renderer leaves visible after a run. It is not a conformance-grade VT — it
// handles the sequences the renderer actually emits (CUU/CUD/CUF/CUB/CHA/VPA,
// ED, EL, CR, LF, TAB, BS, and the DECSET 1049/1047/47 alt-screen toggle) and
// ignores styling and the other private-mode toggles.
type vt struct {
	rows, cols int
	grid       [][]rune
	scroll     []string // lines that scrolled above the viewport
	cx, cy     int

	// Alternate-screen buffer (DECSET 1049/1047/47). A full-screen program
	// enters it on start and leaves it on exit; the primary buffer and its
	// scrollback are saved on enter and restored on exit, so nothing the
	// program painted survives (ADR-0021).
	alt       bool
	savedCx   int
	savedCy   int
	savedGrid [][]rune
}

func newVT(rows, cols int) *vt {
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}
	v := &vt{rows: rows, cols: cols}
	v.grid = make([][]rune, rows)
	for i := range v.grid {
		v.grid[i] = blankRow(cols)
	}
	return v
}

func blankRow(cols int) []rune {
	r := make([]rune, cols)
	for i := range r {
		r[i] = ' '
	}
	return r
}

func (v *vt) write(p []byte) {
	s := string(p)
	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case ch == 0x1b:
			i += v.escape(s[i:])
		case ch == '\r':
			v.cx = 0
			i++
		case ch == '\n':
			v.lineFeed()
			i++
		case ch == '\t':
			v.cx = min((v.cx/8+1)*8, v.cols-1)
			i++
		case ch == 0x08:
			if v.cx > 0 {
				v.cx--
			}
			i++
		case ch < 0x20:
			i++ // other C0 controls: ignore
		default:
			r, sz := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && sz <= 1 {
				i++
				continue
			}
			v.put(r)
			i += sz
		}
	}
}

// escape consumes one escape sequence starting at s[0]=='\x1b' and returns how
// many bytes it spanned.
func (v *vt) escape(s string) int {
	if len(s) < 2 {
		return len(s)
	}
	switch s[1] {
	case '[': // CSI
		j := 2
		for j < len(s) && (s[j] == '?' || s[j] == '>' || s[j] == '<' || s[j] == ';' || s[j] == '$' || (s[j] >= '0' && s[j] <= '9')) {
			j++
		}
		if j >= len(s) {
			return len(s)
		}
		final := s[j]
		params := s[2:j]
		v.csi(final, params)
		return j + 1
	case ']': // OSC — runs to BEL or ST
		j := 2
		for j < len(s) {
			if s[j] == 0x07 {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return len(s)
	case 'M': // RI — reverse index: cursor up one line, scroll down at the top
		if v.cy > 0 {
			v.cy--
		} else {
			copy(v.grid[1:], v.grid[:v.rows-1])
			v.grid[0] = blankRow(v.cols)
		}
		return 2
	case 'D': // IND — index: cursor down one line, scroll at the bottom
		v.lineFeed()
		return 2
	case 'E': // NEL — next line
		v.cx = 0
		v.lineFeed()
		return 2
	default:
		return 2 // two-byte escape (e.g. ESC =, ESC >)
	}
}

// altModes are the DECSET/DECRST parameters that toggle the alternate screen.
var altModes = map[string]bool{"?1049": true, "?1047": true, "?47": true}

func (v *vt) csi(final byte, params string) {
	if altModes[params] {
		switch final {
		case 'h':
			v.enterAlt()
		case 'l':
			v.exitAlt()
		}
		return
	}
	if strings.ContainsAny(params, "?><$") {
		return // other private modes / DECRQM: not visible content
	}
	n := func(def int) int {
		if params == "" {
			return def
		}
		first := params
		if k := strings.IndexByte(params, ';'); k >= 0 {
			first = params[:k]
		}
		x := 0
		for _, c := range first {
			x = x*10 + int(c-'0')
		}
		if x == 0 {
			return def
		}
		return x
	}
	switch final {
	case 'A':
		v.cy = max(v.cy-n(1), 0)
	case 'B':
		v.cy = min(v.cy+n(1), v.rows-1)
	case 'C':
		v.cx = min(v.cx+n(1), v.cols-1)
	case 'D':
		v.cx = max(v.cx-n(1), 0)
	case 'G', '`': // CHA / HPA — column position absolute
		v.cx = clamp(n(1)-1, 0, v.cols-1)
	case 'd': // VPA — line position absolute (bubbletea's per-line frame diff)
		v.cy = clamp(n(1)-1, 0, v.rows-1)
	case 'e': // VPR — line position relative
		v.cy = clamp(v.cy+n(1), 0, v.rows-1)
	case 'H', 'f':
		row, col := 1, 1
		if k := strings.IndexByte(params, ';'); k >= 0 {
			row = atoi(params[:k], 1)
			col = atoi(params[k+1:], 1)
		} else {
			row = atoi(params, 1)
		}
		v.cy = clamp(row-1, 0, v.rows-1)
		v.cx = clamp(col-1, 0, v.cols-1)
	case 'J':
		switch n(0) {
		case 0:
			v.clearLine(v.cy, v.cx, v.cols)
			for r := v.cy + 1; r < v.rows; r++ {
				v.grid[r] = blankRow(v.cols)
			}
		case 1:
			v.clearLine(v.cy, 0, v.cx+1)
			for r := 0; r < v.cy; r++ {
				v.grid[r] = blankRow(v.cols)
			}
		case 2:
			for r := 0; r < v.rows; r++ {
				v.grid[r] = blankRow(v.cols)
			}
		}
	case 'K':
		switch n(0) {
		case 0:
			v.clearLine(v.cy, v.cx, v.cols)
		case 1:
			v.clearLine(v.cy, 0, v.cx+1)
		case 2:
			v.clearLine(v.cy, 0, v.cols)
		}
	}
}

func (v *vt) clearLine(row, from, to int) {
	if row < 0 || row >= v.rows {
		return
	}
	for c := max(from, 0); c < min(to, v.cols); c++ {
		v.grid[row][c] = ' '
	}
}

func (v *vt) put(r rune) {
	if v.cx >= v.cols {
		v.cx = 0
		v.lineFeed()
	}
	w := ansi.StringWidth(string(r))
	if w == 0 {
		return
	}
	v.grid[v.cy][v.cx] = r
	for k := 1; k < w && v.cx+k < v.cols; k++ {
		v.grid[v.cy][v.cx+k] = 0
	}
	v.cx += w
}

func (v *vt) lineFeed() {
	if v.cy >= v.rows-1 {
		if !v.alt {
			// The alternate screen has no scrollback: content that scrolls off
			// the top is gone, not captured (DECSET 1049).
			v.scroll = append(v.scroll, rowString(v.grid[0]))
		}
		copy(v.grid, v.grid[1:])
		v.grid[v.rows-1] = blankRow(v.cols)
		return
	}
	v.cy++
}

// enterAlt switches to the alternate screen: the primary grid and cursor are
// saved, the working grid is cleared, and the cursor homes.
func (v *vt) enterAlt() {
	if v.alt {
		return
	}
	v.savedGrid = v.grid
	v.savedCx, v.savedCy = v.cx, v.cy
	v.grid = make([][]rune, v.rows)
	for i := range v.grid {
		v.grid[i] = blankRow(v.cols)
	}
	v.cx, v.cy, v.alt = 0, 0, true
}

// exitAlt restores the primary screen exactly as it was before enterAlt.
func (v *vt) exitAlt() {
	if !v.alt {
		return
	}
	if v.savedGrid != nil {
		v.grid = v.savedGrid
	}
	v.cx, v.cy = v.savedCx, v.savedCy
	v.savedGrid, v.alt = nil, false
}

func (v *vt) String() string {
	var lines []string
	if !v.alt {
		lines = append(lines, v.scroll...)
	}
	for _, row := range v.grid {
		lines = append(lines, rowString(row))
	}
	// Drop trailing blank lines so assertions read cleanly.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// deSoftWrap rejoins lines the terminal hard-wrapped at the right margin: a
// content line whose display width is exactly cols is glued to the line below
// it. bubbletea's inline renderer keeps its own lines (titles, prompts,
// receipts) well short of the margin, so this only ever repairs a wrapped long
// value such as an absolute path.
func deSoftWrap(screen string, cols int) string {
	if cols <= 0 {
		return screen
	}
	var out []string
	for _, ln := range strings.Split(screen, "\n") {
		if n := len(out); n > 0 && ansi.StringWidth(out[n-1]) == cols {
			out[n-1] += ln
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func rowString(row []rune) string {
	var b strings.Builder
	for _, r := range row {
		if r == 0 {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " ")
}

func atoi(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	x := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		x = x*10 + int(c-'0')
	}
	return x
}

func clamp(x, lo, hi int) int { return max(lo, min(x, hi)) }
