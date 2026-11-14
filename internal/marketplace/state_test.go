package marketplace

import "testing"

func TestDecodeStateRejectsTrailingData(t *testing.T) {
	if _, err := DecodeState([]byte(`{"schema":1,"catalogs":[]}`)); err != nil {
		t.Fatalf("valid state rejected: %v", err)
	}
	for name, doc := range map[string]string{
		"second document": `{"schema":1,"catalogs":[]} {}`,
		"stray brace":     `{"schema":1,"catalogs":[]}}`,
		"stray bracket":   `{"schema":1,"catalogs":[]}]`,
	} {
		if _, err := DecodeState([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
