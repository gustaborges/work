package jsonx

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTrailingData(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`{"a":1}`, false},
		{"{\"a\":1}\n \t\n", false},
		{`{"a":1}}`, true},
		{`{"a":1}]`, true},
		{`{"a":1} x`, true},
		{`{"a":1} {"b":2}`, true},
		{`{"a":1} 7`, true},
	}
	for _, c := range cases {
		dec := json.NewDecoder(strings.NewReader(c.in))
		var v map[string]any
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got := TrailingData(dec); got != c.want {
			t.Errorf("TrailingData(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
