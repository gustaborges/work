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
			{"name":"i","role":"importer","entrypoint":"i","on":[{"event":"start:finalized"}]}]}`,
		"importer with manual": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"i","role":"importer","entrypoint":"i","manual":{"display_name":"I"},"inputs":["work:slug","meta:x.key:optional"]}]}`,
		"linker with discover": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"l","role":"linker","key":"a.pr","entrypoint":"l","discover":{"automatic":true,"on":[{"event":"start:finalized"}]}}]}`,
		"automatic without on is accepted": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"l","role":"linker","key":"a.pr","entrypoint":"l","discover":{"automatic":true}}]}`,
		"starters by name and alias/name": `{"name":"p","version":"1","conventions":[],"components":[
			{"name":"i","role":"importer","entrypoint":"i","on":[{"event":"start:finalized","starters":["s","other/s"]}]}]}`,
		"own private linker key": `{"name":"acme-tools","version":"1","conventions":[],"components":[
			{"name":"l","role":"linker","key":"plugin.acme-tools.build_id","entrypoint":"l","manual":{"display_name":"L"}}]}`,
		"dotted plugin publishing a public key": `{"name":"foo.bar","version":"1","conventions":[],"components":[
			{"name":"l","role":"linker","key":"github.pull_request","entrypoint":"l","manual":{"display_name":"L"}}]}`,
		"empty components and conventions": `{"name":"p","version":"1","components":[],"conventions":[]}`,
	}
	for name, in := range cases {
		if _, err := Parse([]byte(in)); err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
	}
}

func imp(fields string) string {
	return `{"name":"p","version":"1","conventions":[],"components":[{"name":"i","role":"importer","entrypoint":"i",` + fields + `}]}`
}

func lnk(fields string) string {
	return `{"name":"p","version":"1","conventions":[],"components":[{"name":"l","role":"linker","entrypoint":"l",` + fields + `}]}`
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
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"l","role":"linker","entrypoint":"l","discover":{"automatic":true}}]}`, `requires field "key"`,
		},
		"unknown role": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"x","role":"frobnicator","entrypoint":"x"}]}`, "unknown role",
		},
		"bad inputs grammar": {
			`{"name":"p","version":"1","conventions":[],"components":[{"name":"i","role":"importer","entrypoint":"i","manual":{"display_name":"I"},"inputs":["bogus"]}]}`, "does not match",
		},
		"placeholder on strings rejected": {
			imp(`"on":["clone"]`), "cannot unmarshal",
		},
		"placeholder manual true rejected": {
			imp(`"manual":true`), "cannot unmarshal",
		},
		"placeholder discover list rejected": {
			lnk(`"key":"a.pr","discover":["x"]`), "cannot unmarshal",
		},
		"undefined event": {
			imp(`"on":[{"event":"work:archived"}]`), "not a core event",
		},
		"empty on": {
			imp(`"on":[]`), "non-empty array",
		},
		"empty starters": {
			imp(`"on":[{"event":"start:finalized","starters":[]}]`), "starters must be non-empty",
		},
		"blank starters entry": {
			imp(`"on":[{"event":"start:finalized","starters":[" "]}]`), "starters entry",
		},
		"malformed alias/name starters entry": {
			imp(`"on":[{"event":"start:finalized","starters":["a/"]}]`), "starters entry",
		},
		"empty manual display_name": {
			imp(`"manual":{"display_name":" "}`), "display_name",
		},
		"manual without display_name": {
			imp(`"manual":{"description":"d"}`), "display_name",
		},
		"malformed input": {
			imp(`"on":[{"event":"start:finalized"}],"inputs":["link-x.y"]`), "does not match",
		},
		"work fact outside the exposed set": {
			imp(`"on":[{"event":"start:finalized"}],"inputs":["work:head_sha"]`), "not an exposed work fact",
		},
		"input key with bad grammar": {
			imp(`"on":[{"event":"start:finalized"}],"inputs":["link:GitHub"]`), "inputs entry",
		},
		"duplicate key across namespaces": {
			imp(`"on":[{"event":"start:finalized"}],"inputs":["meta:x.y","link:x.y"]`), "same key",
		},
		"linker discover inputs are checked too": {
			lnk(`"key":"a.pr","discover":{"automatic":true,"inputs":["work:nope"]}`), "not an exposed work fact",
		},
		"linker key with bad grammar": {
			lnk(`"key":"pr","manual":{"display_name":"L"}`), "linker key",
		},
		"linker key private to another plugin": {
			lnk(`"key":"plugin.other.pr","manual":{"display_name":"L"}`), "private to plugin",
		},
		"dotted plugin cannot own a private key": {
			`{"name":"foo.bar","version":"1","conventions":[],"components":[{"name":"l","role":"linker","entrypoint":"l","key":"plugin.foo.bar.x","manual":{"display_name":"L"}}]}`, "dot in its name",
		},
		"unknown field inside on": {
			imp(`"on":[{"event":"start:finalized","priority":1}]`), "unknown field",
		},
		"unknown field inside manual": {
			imp(`"manual":{"display_name":"I","icon":"x"}`), "unknown field",
		},
		"unknown field inside discover": {
			lnk(`"key":"a.pr","discover":{"automatic":true,"score":1}`), "unknown field",
		},
		"top-level inputs on a linker": {
			lnk(`"key":"a.pr","manual":{"display_name":"L"},"inputs":["work:slug"]`), `forbids field "inputs"`,
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

func TestParseRejectsStarterPatternThatDoesNotCompile(t *testing.T) {
	in := `{"name":"p","version":"1","conventions":[],"components":[
		{"name":"s","role":"starter","entrypoint":"s","pattern":"(unclosed"}]}`
	_, err := Parse([]byte(in))
	if err == nil || !strings.Contains(err.Error(), "pattern") {
		t.Fatalf("err = %v, want a pattern error", err)
	}
}

func TestParseRejectsNameThatIsNotAValidAlias(t *testing.T) {
	for _, name := range []string{"..", ".", "a/b", `a\b`, ".hidden", "x.old", "with space", "-lead"} {
		in := `{"name":"` + strings.ReplaceAll(name, `\`, `\\`) + `","version":"1","conventions":[],"components":[]}`
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("name %q accepted, want an alias error", name)
		}
	}
}

func TestValidateAlias(t *testing.T) {
	for _, ok := range []string{"a", "work-reference", "github_plugin", "p1.2", "A-b.c", "x.olds"} {
		if err := ValidateAlias(ok); err != nil {
			t.Errorf("ValidateAlias(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", " ", "..", ".", "a/b", "a b", ".x", "_x", "x.old", "é"} {
		if err := ValidateAlias(bad); err == nil {
			t.Errorf("ValidateAlias(%q) = nil, want an error", bad)
		}
	}
}

// The manifest shown in ADD §4 must parse into exactly the declared shapes.
func TestParseADDExample(t *testing.T) {
	const in = `{"name":"github-plugin","version":"1.0.0","conventions":[],"components":[
		{"name":"github-pull-request-starter","role":"starter","entrypoint":"starter.py","runtime":"python3","pattern":"^https://github.com/"},
		{"name":"github-pull-request-importer","role":"importer","entrypoint":"importer.py","runtime":"python3",
		 "on":[{"event":"start:finalized","starters":["github-pull-request-starter"]}],
		 "manual":{"display_name":"Pull Request Context","description":"Imports the artifacts associated with the pull request"},
		 "inputs":["link:github.pull_request","work:start_mode:optional"]},
		{"name":"github-pull-request-linker","role":"linker","key":"github.pull_request","entrypoint":"linker.py","runtime":"python3",
		 "discover":{"automatic":true,"on":[{"event":"start:finalized","starters":["github-pull-request-starter"]}],"inputs":["work:worktree_path"]},
		 "manual":{"display_name":"GitHub Pull Request","description":"Links the Work to a GitHub pull request"}}]}`
	m, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	imp, lnk := m.Components[1], m.Components[2]
	if imp.On[0].Event != EventStartFinalized || imp.On[0].Starters[0] != "github-pull-request-starter" {
		t.Errorf("importer on = %+v", imp.On)
	}
	if imp.Manual == nil || imp.Manual.DisplayName != "Pull Request Context" {
		t.Errorf("importer manual = %+v", imp.Manual)
	}
	if lnk.Discover == nil || !lnk.Discover.Automatic || lnk.Discover.Inputs[0] != "work:worktree_path" {
		t.Errorf("linker discover = %+v", lnk.Discover)
	}
}

func TestParseInput(t *testing.T) {
	got, err := ParseInput("work:start_mode:optional")
	if err != nil || got != (Input{Source: SourceWork, Key: "start_mode", Optional: true}) {
		t.Errorf("ParseInput = %+v, %v", got, err)
	}
	got, err = ParseInput("link:github.pull_request")
	if err != nil || got != (Input{Source: SourceLink, Key: "github.pull_request"}) {
		t.Errorf("ParseInput = %+v, %v", got, err)
	}
	for _, bad := range []string{"", "slug", "work:", "x:y", "work:slug:required", "meta:Bad"} {
		if _, err := ParseInput(bad); err == nil {
			t.Errorf("ParseInput(%q) accepted", bad)
		}
	}
}
