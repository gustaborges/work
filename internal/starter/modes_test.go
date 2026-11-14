package starter

import (
	"encoding/json"
	"github.com/gustaborges/work/internal/diag"
	"reflect"
	"strings"
	"testing"
)

func TestModeValidation(t *testing.T) {
	for _, raw := range []string{``, `null`, `{}`, `{"new":{}}`, `{"fork":{"links":null,"meta":null}}`, `{"contribution":{"meta":{"github.pull_request.number":null}}}`} {
		if err := ValidateResponse(Reference{Modes: json.RawMessage(raw)}, "sample"); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{`[]`, `true`, `1`, `"secret"`, `{"unknown":{}}`, `{"fork":null}`, `{"fork":[]}`, `{"fork":{"links":[]}}`, `{"fork":{"meta":4}}`, `{"fork":{"links":{"github.pull_request":null}}}`, `{"fork":{"links":{"github.pull_request":42}}}`, `{"fork":{"links":{"github.pull_request":""}}}`, `{"fork":{"links":{"Bad.Key":"secret"}}}`, `{"fork":{"meta":{"plugin.foreign.x":"secret"}}}`} {
		for range 3 {
			err := ValidateResponse(Reference{Modes: json.RawMessage(raw)}, "sample")
			if diag.Token(err) != diag.StarterResponseInvalid.Token || strings.Contains(err.Error(), "secret") {
				t.Errorf("%s: %v", raw, err)
			}
		}
	}
}

func TestMergeModesCopiesAndReplacesShallowly(t *testing.T) {
	for _, mode := range []string{"new", "contribution", "fork"} {
		ref := Reference{Path: "repo", BaseBranch: "main", Links: map[string]string{"github.pull_request": "base", "github.issue": "shared"}, Meta: map[string]any{"github.pull_request.number": map[string]any{"old": true}}, Modes: json.RawMessage(`{"` + mode + `":{"links":{"github.pull_request":"selected"},"meta":{"github.pull_request.number":null}}}`)}
		got := MergeMode(ref, mode)
		if got.Path != "repo" || got.BaseBranch != "main" || got.Links["github.pull_request"] != "selected" || got.Links["github.issue"] != "shared" || got.Meta["github.pull_request.number"] != nil {
			t.Fatal(got)
		}
		got.Links["github.issue"] = "mutated"
		if ref.Links["github.issue"] != "shared" || ref.Meta["github.pull_request.number"] == nil {
			t.Fatal("mutated response")
		}
		if other := MergeMode(ref, "missing"); !reflect.DeepEqual(other.Links, ref.Links) {
			t.Fatal(other)
		}
	}
	ref := Reference{Meta: map[string]any{"github.pull_request.number": map[string]any{"old": true}}, Modes: json.RawMessage(`{"new":{"meta":{"github.pull_request.number":{"new":true}}}}`)}
	if got := MergeMode(ref, "new"); !reflect.DeepEqual(got.Meta["github.pull_request.number"], map[string]any{"new": true}) {
		t.Fatal(got)
	}
}

func TestImplicitNewMode(t *testing.T) {
	ref := Reference{Modes: json.RawMessage(`{"new":{"links":{"github.pull_request":"selected"}}}`)}
	if got := MergeMode(ref, ""); got.Links["github.pull_request"] != "selected" {
		t.Fatal(got)
	}
}
