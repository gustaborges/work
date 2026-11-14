package adminstate

import "testing"

func TestDecodeLifecycleRejectsTrailingData(t *testing.T) {
	const good = `{"schema":1,"initialized":true,"policy_handled":true,"removed":false}`
	if _, err := decodeLifecycle([]byte(good)); err != nil {
		t.Fatalf("valid lifecycle rejected: %v", err)
	}
	for name, doc := range map[string]string{
		"second document": good + ` {}`,
		"stray brace":     good + `}`,
		"stray bracket":   good + `]`,
	} {
		if _, err := decodeLifecycle([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
