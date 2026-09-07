//go:build unix

package integration

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// vt is a deliberately small terminal emulator: just enough of the cursor,
// erase, and scroll semantics to reconstruct what bubbletea's inline renderer
// leaves visible after a run. It is not a conformance-grade VT — it handles the
// sequences the renderer actually emits (CUU/CUD/CUF/CUB/CHA, ED, EL, CR, LF,
// TAB, BS) and ignores styling and private-mode toggles.
type vt struct {
	rows, cols int
	grid       [][]rune
	scroll     []string // lines that scrolled above the viewport
	cx, cy     int
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

func (v *vt) csi(final byte, params string) {
	if strings.ContainsAny(params, "?><$") {
		return // private modes / DECRQM: not visible content
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
	case 'G':
		v.cx = clamp(n(1)-1, 0, v.cols-1)
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
		v.scroll = append(v.scroll, rowString(v.grid[0]))
		copy(v.grid, v.grid[1:])
		v.grid[v.rows-1] = blankRow(v.cols)
		return
	}
	v.cy++
}

func (v *vt) String() string {
	lines := append([]string(nil), v.scroll...)
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
