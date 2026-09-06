package plugin

import (
	"strings"
	"testing"
)

const seedManifest = `{
  "name": "work-reference",
  "version": "1.0.0",
  "components": [
    { "name": "local-path-starter", "role": "starter", "entrypoint": "starter" },
    {
      "name": "filesystem-repository-locator",
      "role": "repository-locator",
      "display_name": "Filesystem repositories",
      "description": "Finds local Git repositories in configured search roots",
      "accepts": ["name", "git_fetch_urls", "query"],
      "entrypoint": "locator"
    }
  ],
  "conventions": [
    { "name": "freeform", "prefixes": ["{slug}"] }
  ]
}`

func TestParseSeedManifest(t *testing.T) {
	m, err := Parse([]byte(seedManifest))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Name != "work-reference" || m.Version != "1.0.0" {
		t.Errorf("top level = %+v", m)
	}
	if len(m.Components) != 2 {
		t.Fatalf("components = %d, want 2", len(m.Components))
	}
	if !m.Components[0].IsFallbackStarter() {
		t.Errorf("first component should be a fallback starter")
	}
	loc := m.Components[1]
	if loc.Role != RoleRepositoryLocator || len(loc.Accepts) != 3 {
		t.Errorf("locator = %+v", loc)
	}
	if len(m.Conventions) != 1 || m.Conventions[0].Name != "freeform" || m.Conventions[0].Prefixes[0] != "{slug}" {
		t.Errorf("conventions = %+v", m.Conventions)
	}
}

func TestParseValid(t *testing.T) {
	cases := map[string]string{
		"specific starter with pattern": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"s","role":"starter","entrypoint":"s","pattern":"^https://","runtime":"python3"}]}`,
		"importer with on": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"i","role":"importer","entrypoint":"i","on":["clone"]}]}`,
		"importer with manual": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"i","role":"importer","entrypoint":"i","manual":true,"inputs":["work:slug","meta:key:optional"]}]}`,
		"linker with discover": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"l","role":"linker","key":"pr","entrypoint":"l","discover":["x"]}]}`,
		"empty components and conventions": `{"name":"p","version":"1","components":[],"conventions":[]}`,
	}
	for name, in := range cases {
		if _, err := Parse([]byte(in)); err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string
	}{
		"unknown top-level field": {
			`{"name":"p","version":"1","components":[],"conventions":[],"extra":1}`, "unknown top-level field",
		},
		"missing conventions key": {
			`{"name":"p","version":"1","components":[]}`, `missing required field "conventions"`,
		},
		"empty name": {
			`{"name":"","version":"1","components":[],"conventions":[]}`, "name",
		},
		"unknown component field": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"s","role":"starter","entrypoint":"s","priority":1}]}`, "unknown field",
		},
		"invocation field rejected": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"s","role":"starter","entrypoint":"s","invocation":"x"}]}`, "unknown field",
		},
		"starter with accepts": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"s","role":"starter","entrypoint":"s","accepts":["name"]}]}`, `forbids field "accepts"`,
		},
		"locator without accepts": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"l","role":"repository-locator","entrypoint":"l"}]}`, `requires field "accepts"`,
		},
		"locator with empty accepts": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"l","role":"repository-locator","entrypoint":"l","accepts":[]}]}`, "non-empty accepts",
		},
		"locator accepts path": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"l","role":"repository-locator","entrypoint":"l","accepts":["path"]}]}`, "not one of",
		},
		"locator with pattern": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"l","role":"repository-locator","entrypoint":"l","accepts":["name"],"pattern":"x"}]}`, `forbids field "pattern"`,
		},
		"importer without on or manual": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"i","role":"importer","entrypoint":"i"}]}`, "at least one of",
		},
		"linker without key": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"l","role":"linker","entrypoint":"l","discover":["x"]}]}`, `requires field "key"`,
		},
		"unknown role": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"x","role":"frobnicator","entrypoint":"x"}]}`, "unknown role",
		},
		"bad inputs grammar": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"i","role":"importer","entrypoint":"i","manual":true,"inputs":["bogus"]}]}`, "does not match",
		},
		"convention with role field": {
			`{"name":"p","version":"1","components":[],"conventions":[{"name":"x","prefixes":["a"],"role":"starter"}]}`, "conventions carry only",
		},
		"convention without prefixes": {
			`{"name":"p","version":"1","components":[],"conventions":[{"name":"x"}]}`, "missing prefixes",
		},
	}
	for name, c := range cases {
		_, err := Parse([]byte(c.in))
		if err == nil {
			t.Errorf("%s: want error, got nil", name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not contain %q", name, err.Error(), c.want)
		}
	}
}
