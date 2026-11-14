package starter

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

func TestDecodeModesAcceptsValidShapes(t *testing.T) {
	for _, raw := range []string{``, `null`, `{}`, `{"new":{}}`, `{"fork":{"links":null,"meta":null}}`, `{"contribution":{"meta":{"github.pull_request.number":null}}}`} {
		if _, err := decodeModes(json.RawMessage(raw)); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

func TestDecodeModesRejectsWrongShapes(t *testing.T) {
	for _, raw := range []string{`[]`, `true`, `1`, `"secret"`, `{"unknown":{}}`, `{"fork":null}`, `{"fork":[]}`, `{"fork":{"links":[]}}`, `{"fork":{"meta":4}}`, `{"fork":{"links":{"github.pull_request":null}}}`, `{"fork":{"links":{"github.pull_request":42}}}`} {
		for range 3 {
			_, err := decodeModes(json.RawMessage(raw))
			if diag.Token(err) != diag.StarterResponseInvalid.Token || strings.Contains(err.Error(), "secret") {
				t.Errorf("%s: %v", raw, err)
				continue
			}
			// Same summary and hint as the other structural Starter failures.
			if de, ok := errors.AsType[*diag.Error](err); !ok || de.Summary == "" || de.Hint == "" {
				t.Errorf("%s: missing summary/hint: %#v", raw, err)
			}
		}
	}
}

func TestDecodeModesQuotesStarterControlledText(t *testing.T) {
	for _, raw := range []string{
		"{\"\\u001b[31mred\\nforged\":{}}",
		"{\"fork\":{\"links\":{\"\\u001b[31mk\\nforged\":null}}}",
	} {
		_, err := decodeModes(json.RawMessage(raw))
		if err == nil {
			t.Fatalf("%s: accepted", raw)
		}
		msg := diag.Format(err)
		if strings.ContainsAny(msg, "\x1b") || strings.Count(strings.TrimRight(msg, "\n"), "\n") != 0 {
			t.Errorf("%s: raw control characters reached the diagnostic: %q", raw, msg)
		}
		if !strings.Contains(msg, `\x1b`) || !strings.Contains(msg, `\n`) {
			t.Errorf("%s: expected quoted control characters: %q", raw, msg)
		}
	}
}

func TestValidateResponseModeEntries(t *testing.T) {
	for _, modes := range []map[string]ModeContext{
		nil,
		{"new": {}},
		{"fork": {Links: map[string]string{"github.pull_request": "https://example.test/pr"}}},
		{"contribution": {Meta: map[string]any{"github.pull_request.number": nil}}},
	} {
		if err := ValidateResponse(Reference{Modes: modes}, "sample"); err != nil {
			t.Errorf("%v: %v", modes, err)
		}
	}
	for _, tc := range []struct {
		name  string
		modes map[string]ModeContext
	}{
		{"bad link key", map[string]ModeContext{"fork": {Links: map[string]string{"Bad.Key": "secret"}}}},
		{"foreign private meta", map[string]ModeContext{"fork": {Meta: map[string]any{"plugin.foreign.x": "secret"}}}},
		{"empty link value", map[string]ModeContext{"fork": {Links: map[string]string{"github.pull_request": ""}}}},
		{"unselected mode", map[string]ModeContext{"new": {}, "contribution": {Links: map[string]string{"Bad.Key": "secret"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 3 {
				err := ValidateResponse(Reference{Modes: tc.modes}, "sample")
				if diag.Token(err) != diag.StarterResponseInvalid.Token || strings.Contains(err.Error(), "secret") {
					t.Fatalf("%v", err)
				}
				// The border prints only the diagnostic's own text, so the
				// offending mode must be in it, not in a wrapper.
				var mode string
				for name := range tc.modes {
					if len(tc.modes[name].Links)+len(tc.modes[name].Meta) > 0 {
						mode = name
					}
				}
				if out := diag.Format(err); !strings.Contains(out, `mode "`+mode+`"`) {
					t.Fatalf("mode %q not named in %q", mode, out)
				}
			}
		})
	}
}

func TestMergeModesCopiesAndReplacesShallowly(t *testing.T) {
	for _, mode := range []string{"new", "contribution", "fork"} {
		ref := Reference{
			Path: "repo", BaseBranch: "main",
			Links: map[string]string{"github.pull_request": "base", "github.issue": "shared"},
			Meta:  map[string]any{"github.pull_request.number": map[string]any{"old": true}},
			Modes: map[string]ModeContext{mode: {
				Links: map[string]string{"github.pull_request": "selected"},
				Meta:  map[string]any{"github.pull_request.number": nil},
			}},
		}
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
	ref := Reference{
		Meta:  map[string]any{"github.pull_request.number": map[string]any{"old": true}},
		Modes: map[string]ModeContext{"new": {Meta: map[string]any{"github.pull_request.number": map[string]any{"new": true}}}},
	}
	if got := MergeMode(ref, "new"); !reflect.DeepEqual(got.Meta["github.pull_request.number"], map[string]any{"new": true}) {
		t.Fatal(got)
	}
}

func TestImplicitNewMode(t *testing.T) {
	ref := Reference{Modes: map[string]ModeContext{"new": {Links: map[string]string{"github.pull_request": "selected"}}}}
	if got := MergeMode(ref, ""); got.Links["github.pull_request"] != "selected" {
		t.Fatal(got)
	}
}
