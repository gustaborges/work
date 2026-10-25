package present

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"strings"
	"testing"
)

func TestTextWaitsForBackAndScrolls(t *testing.T) {
	var content strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&content, "result %02d\n", i)
	}
	m := newTextModel(IO{}, TextSpec{Title: "Installed plugins", Content: content.String()})
	m.th = offTheme()
	m.w = 40
	m.h = 12
	body := m.body(40, 12)
	if m.status().done || !strings.Contains(body, "result 00") || strings.Contains(body, "result 49") || !strings.Contains(body, "enter Back") {
		t.Fatal(body, m.status())
	}
	next, _ := m.Update(press("end"))
	m = next.(textModel)
	if body = m.body(40, 12); !strings.Contains(body, "result 49") || strings.Contains(body, "result 00") {
		t.Fatal(body)
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	m = next.(textModel)
	if body = m.body(30, 8); len(strings.Split(body, "\n")) > 8 || !strings.Contains(body, "enter Back") {
		t.Fatal(body)
	}
	next, _ = m.Update(press("enter"))
	if !next.status().done || next.status().cancelled {
		t.Fatal(next.status())
	}
}
func TestTextWrapsAndCancellationHasNoReceipt(t *testing.T) {
	m := newTextModel(IO{}, TextSpec{Title: "Result", Content: strings.Repeat("wide ", 20)})
	m.th = offTheme()
	body := m.body(24, 10)
	for _, line := range strings.Split(body, "\n") {
		if DisplayWidth(line) > 24 {
			t.Fatal(line)
		}
	}
	if !strings.Contains(body, "wide") {
		t.Fatal(body)
	}
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		next, _ := m.Update(press(key))
		if !next.status().cancelled || next.status().receipt != "" {
			t.Fatal(key, next.status())
		}
	}
}

func TestTextWrapsLongPathsWithoutLosingContent(t *testing.T) {
	content := "/" + strings.Repeat("long-path/", 20)
	m := newTextModel(IO{}, TextSpec{Content: content})
	m.w = 24
	lines := m.lines()
	for _, line := range lines {
		if DisplayWidth(line) > m.w {
			t.Fatal("path overflowed the viewport:", line)
		}
	}
	if strings.Join(lines, "") != content {
		t.Fatal("wrapping lost part of the path")
	}
}
