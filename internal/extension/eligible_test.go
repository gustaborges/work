package extension

import (
	"slices"
	"testing"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/work"
)

var starterComp = registry.Component{Alias: "acme", Name: "pr-starter", Role: registry.RoleStarter}

func subject(mode string, links map[string]string, meta map[string]any) Subject {
	st := &work.State{
		Work: work.WorkSection{
			ID: "W1", Slug: "s", StartMode: mode, Branch: "b", BaseBranch: "main",
		},
		Meta:  meta,
		Links: links,
	}
	if mode == work.StartModeContribution {
		st.Work.Slug = ""
	}
	if st.Meta == nil {
		st.Meta = map[string]any{}
	}
	if st.Links == nil {
		st.Links = map[string]string{}
	}
	return Subject{State: st, WorktreePath: "/w/worktree", Starter: starterComp}
}

func linker(name string, automatic bool, on []plugin.Subscription, inputs ...string) registry.Component {
	return registry.Component{
		Alias: "acme", Name: name, Role: plugin.RoleLinker, Key: "github.pull_request",
		Discover: &plugin.Discover{Automatic: automatic, On: on}, Inputs: inputs,
	}
}

func importer(name string, on []plugin.Subscription, inputs ...string) registry.Component {
	return registry.Component{Alias: "acme", Name: name, Role: plugin.RoleImporter, On: on, Inputs: inputs}
}

var startFinalized = []plugin.Subscription{{Event: plugin.EventStartFinalized}}

func names(ds []Decision) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Component.QualifiedName())
	}
	return out
}

func TestEligibleTruthTable(t *testing.T) {
	cases := []struct {
		name string
		comp registry.Component
		subj Subject
		want bool
	}{
		{"automatic linker, no restriction", linker("l", true, startFinalized, "work:worktree_path"), subject("new", nil, nil), true},
		{"linker not automatic", linker("l", false, startFinalized), subject("new", nil, nil), false},
		{"linker automatic without on", linker("l", true, nil), subject("new", nil, nil), false},
		{"linker without a key is inert", registry.Component{Alias: "acme", Name: "l", Role: plugin.RoleLinker,
			Discover: &plugin.Discover{Automatic: true, On: startFinalized}}, subject("new", nil, nil), false},
		{"legacy linker without activation data", registry.Component{Alias: "acme", Name: "l", Role: plugin.RoleLinker}, subject("new", nil, nil), false},
		{"other event", linker("l", true, []plugin.Subscription{{Event: "start:other"}}), subject("new", nil, nil), false},
		{"restricted by bare name, matches", importer("i", []plugin.Subscription{{Event: plugin.EventStartFinalized, Starters: []string{"pr-starter"}}}), subject("new", nil, nil), true},
		{"restricted by alias/name, matches", importer("i", []plugin.Subscription{{Event: plugin.EventStartFinalized, Starters: []string{"acme/pr-starter"}}}), subject("new", nil, nil), true},
		{"restricted by alias/name, other alias", importer("i", []plugin.Subscription{{Event: plugin.EventStartFinalized, Starters: []string{"other/pr-starter"}}}), subject("new", nil, nil), false},
		{"restricted to another starter", importer("i", []plugin.Subscription{{Event: plugin.EventStartFinalized, Starters: []string{"else"}}}), subject("new", nil, nil), false},
		{"any listed starter matches", importer("i", []plugin.Subscription{{Event: plugin.EventStartFinalized, Starters: []string{"else", "pr-starter"}}}), subject("new", nil, nil), true},
		{"importer without on", importer("i", nil), subject("new", nil, nil), false},
		{"required link present", importer("i", startFinalized, "link:github.pull_request"), subject("new", map[string]string{"github.pull_request": "u"}, nil), true},
		{"required link absent", importer("i", startFinalized, "link:github.pull_request"), subject("new", nil, nil), false},
		{"optional link absent", importer("i", startFinalized, "link:github.pull_request:optional"), subject("new", nil, nil), true},
		{"required meta present", importer("i", startFinalized, "meta:github.pull_request.number"), subject("new", nil, map[string]any{"github.pull_request.number": 1}), true},
		{"required meta absent", importer("i", startFinalized, "meta:github.pull_request.number"), subject("new", nil, nil), false},
		{"required slug in new mode", importer("i", startFinalized, "work:slug"), subject("new", nil, nil), true},
		{"required slug in contribution mode", importer("i", startFinalized, "work:slug"), subject("contribution", nil, nil), false},
		{"optional slug in contribution mode", importer("i", startFinalized, "work:slug:optional"), subject("contribution", nil, nil), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := &registry.Registry{Components: []registry.Component{tc.comp}}
			got := Eligible(reg, tc.comp.Role, plugin.EventStartFinalized, tc.subj)
			if (len(got) == 1) != tc.want {
				t.Fatalf("eligible = %v, want %v", names(got), tc.want)
			}
		})
	}
}

func TestEligibleOnlyReturnsTheAskedRole(t *testing.T) {
	reg := &registry.Registry{Components: []registry.Component{
		linker("l", true, startFinalized), importer("i", startFinalized), starterComp,
	}}
	if got := names(Eligible(reg, plugin.RoleLinker, plugin.EventStartFinalized, subject("new", nil, nil))); !slices.Equal(got, []string{"acme/l"}) {
		t.Errorf("linkers = %v", got)
	}
	if got := names(Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subject("new", nil, nil))); !slices.Equal(got, []string{"acme/i"}) {
		t.Errorf("importers = %v", got)
	}
	if got := Eligible(reg, plugin.RoleStarter, plugin.EventStartFinalized, subject("new", nil, nil)); len(got) != 0 {
		t.Errorf("starters = %v, want none", names(got))
	}
}

func TestEligibleOrderIsBytewiseAndIgnoresRegistryOrder(t *testing.T) {
	mk := func(alias, name string) registry.Component {
		c := linker(name, true, startFinalized)
		c.Alias = alias
		return c
	}
	comps := []registry.Component{mk("b", "z"), mk("a", "linker2"), mk("a", "linker"), mk("B", "x")}
	want := []string{"B/x", "a/linker", "a/linker2", "b/z"}
	for range 3 {
		reg := &registry.Registry{Components: slices.Clone(comps)}
		if got := names(Eligible(reg, plugin.RoleLinker, plugin.EventStartFinalized, subject("new", nil, nil))); !slices.Equal(got, want) {
			t.Fatalf("order = %v, want %v", got, want)
		}
		slices.Reverse(comps)
	}
}

func TestEligibleDeliversOnlyResolvedInputsByBareKey(t *testing.T) {
	c := importer("i", startFinalized,
		"link:github.pull_request", "work:start_mode:optional", "work:slug:optional",
		"meta:github.pull_request.number:optional", "meta:missing.key:optional")
	reg := &registry.Registry{Components: []registry.Component{c}}
	subj := subject("contribution", map[string]string{"github.pull_request": "https://x"}, map[string]any{"github.pull_request.number": float64(7)})

	got := Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subj)
	if len(got) != 1 {
		t.Fatalf("eligible = %v", names(got))
	}
	want := map[string]any{
		"github.pull_request":        "https://x",
		"start_mode":                 "contribution",
		"github.pull_request.number": float64(7),
	}
	if len(got[0].Inputs) != len(want) {
		t.Fatalf("inputs = %v, want %v", got[0].Inputs, want)
	}
	for k, v := range want {
		if got[0].Inputs[k] != v {
			t.Errorf("inputs[%q] = %v, want %v", k, got[0].Inputs[k], v)
		}
	}
}

func TestEligibleWorkFacts(t *testing.T) {
	c := linker("l", true, startFinalized,
		"work:worktree_path", "work:start_mode", "work:branch", "work:base_branch", "work:slug")
	reg := &registry.Registry{Components: []registry.Component{c}}
	got := Eligible(reg, plugin.RoleLinker, plugin.EventStartFinalized, subject("new", nil, nil))
	if len(got) != 1 {
		t.Fatal("not eligible")
	}
	want := map[string]any{"worktree_path": "/w/worktree", "start_mode": "new", "branch": "b", "base_branch": "main", "slug": "s"}
	for k, v := range want {
		if got[0].Inputs[k] != v {
			t.Errorf("inputs[%q] = %v, want %v", k, got[0].Inputs[k], v)
		}
	}
}

func TestEligibleIsPure(t *testing.T) {
	c := importer("i", startFinalized, "link:github.pull_request")
	subj := subject("new", map[string]string{"github.pull_request": "u"}, nil)
	reg := &registry.Registry{Components: []registry.Component{c}}
	Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subj)
	if len(subj.State.Links) != 1 || len(reg.Components) != 1 {
		t.Errorf("Eligible mutated its arguments")
	}
}
