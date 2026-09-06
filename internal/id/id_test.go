package id

import (
	"strings"
	"testing"
	"time"
)

func TestNewIsSortableByTime(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	earlier := New(base)
	later := New(base.Add(time.Hour))
	if earlier >= later {
		t.Errorf("expected %s < %s", earlier, later)
	}
}

func TestNewLengthAndAlphabet(t *testing.T) {
	got := New(time.Now())
	if len(got) != 26 {
		t.Fatalf("len = %d, want 26 (%q)", len(got), got)
	}
	for _, r := range got {
		if !strings.ContainsRune(crockford, r) {
			t.Errorf("char %q not in Crockford alphabet", r)
		}
	}
}

func TestNewIsUnique(t *testing.T) {
	now := time.Now()
	seen := map[string]bool{}
	for range 1000 {
		v := New(now)
		if seen[v] {
			t.Fatalf("duplicate id %s", v)
		}
		seen[v] = true
	}
}
