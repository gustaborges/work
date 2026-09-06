package tui

import "testing"

func TestValidateSlug(t *testing.T) {
	good := []string{"add-retry", "fix123", "a", "feature-x-y"}
	for _, s := range good {
		if err := ValidateSlug(s); err != nil {
			t.Errorf("ValidateSlug(%q) = %v", s, err)
		}
	}
	bad := []string{"", "  ", "has space", "tab\there", "a..b", "..", "-lead", "ctrl\x01char"}
	for _, s := range bad {
		if err := ValidateSlug(s); err == nil {
			t.Errorf("ValidateSlug(%q) = nil, want error", s)
		}
	}
}
