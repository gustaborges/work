package extension

import (
	"slices"
	"testing"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/semconv"
	"github.com/gustaborges/work/internal/work"
	"github.com/gustaborges/work/tests/fixtures/installfx"
)

func prov(plugin, marketplace string) pluginid.Provider {
	return pluginid.Provider{Plugin: plugin, Marketplace: marketplace}
}

func starterOf(p pluginid.Provider, name string) registry.Component {
	return registry.Component{Provider: p, Name: name, Role: registry.RoleStarter, Entrypoint: "s"}
}

func subscriber(p pluginid.Provider, name string, starters ...string) registry.Component {
	return registry.Component{Provider: p, Name: name, Role: plugin.RoleImporter, Entrypoint: "i",
		On: []plugin.Subscription{{Event: plugin.EventStartFinalized, Starters: starters}}}
}

func subjectFrom(starter registry.Component) Subject {
	s := subject("new", nil, nil)
	s.Starter = starter
	return s
}

// regOf builds a registry whose packages carry distinct source instances per
// marketplace, as two real registrations would.
func regOf(comps ...registry.Component) *registry.Registry { return installfx.Reg(nil, comps...) }

func TestStarterFilterSelfPackageBindsToItsOwnProvider(t *testing.T) {
	alpha, beta := prov("sample", "alpha"), prov("sample", "beta")
	sa, sb := starterOf(alpha, "pr"), starterOf(beta, "pr")
	reg := regOf(sa, sb, subscriber(alpha, "own", "pr"), subscriber(beta, "other", "pr"))

	// The Work came from alpha's Starter: only alpha's subscriber, which names
	// a Starter its own package declares, runs. Beta's homonym never matches.
	got := names(Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sa)))
	if !slices.Equal(got, []string{"sample@alpha/own"}) {
		t.Fatalf("alpha Work activated %v", got)
	}
	got = names(Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sb)))
	if !slices.Equal(got, []string{"sample@beta/other"}) {
		t.Fatalf("beta Work activated %v", got)
	}
}

func TestStarterFilterExactExternalReference(t *testing.T) {
	alpha, beta, watcher := prov("sample", "alpha"), prov("sample", "beta"), prov("watcher", "team")
	sa, sb := starterOf(alpha, "pr"), starterOf(beta, "pr")
	reg := regOf(sa, sb, subscriber(watcher, "w", "sample@beta/pr"))
	if got := Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sa)); len(got) != 0 {
		t.Fatalf("a filter naming beta ran for alpha's Work: %v", names(got))
	}
	if got := names(Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sb))); !slices.Equal(got, []string{"watcher@team/w"}) {
		t.Fatalf("exact external reference did not match: %v", got)
	}
}

func TestStarterFilterUniqueGlobalBareName(t *testing.T) {
	alpha, watcher := prov("sample", "alpha"), prov("watcher", "team")
	sa := starterOf(alpha, "pr")
	reg := regOf(sa, subscriber(watcher, "w", "pr"))
	if got := names(Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sa))); !slices.Equal(got, []string{"watcher@team/w"}) {
		t.Fatalf("unique global bare name did not bind: %v", got)
	}
}

func TestStarterFilterAmbiguousGlobalBareNameIsWithheld(t *testing.T) {
	alpha, beta, watcher := prov("sample", "alpha"), prov("sample", "beta"), prov("watcher", "team")
	sa, sb := starterOf(alpha, "pr"), starterOf(beta, "pr")
	reg := regOf(sa, sb, subscriber(watcher, "w", "pr"))
	decisions, ambiguous := EligibleDiagnosed(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sa))
	if len(decisions) != 0 {
		t.Fatalf("an ambiguous filter must not run its subscriber: %v", names(decisions))
	}
	if len(ambiguous) != 1 || ambiguous[0].Component.QualifiedName() != "watcher@team/w" || ambiguous[0].Filter != "pr" {
		t.Fatalf("ambiguity not reported: %+v", ambiguous)
	}

	// Disabling one of the two makes the remaining one unique again.
	installfx.SetPackageEnabled(reg, "sample", true)
	pkg, _ := reg.PackageByProvider(beta)
	pkg.Enabled = false
	reg.UpsertPackage(pkg)
	if got := names(Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sa))); !slices.Equal(got, []string{"watcher@team/w"}) {
		t.Fatalf("filter did not bind once unique: %v", got)
	}
}

func TestStarterFilterWithoutInstalledStarterIsInactive(t *testing.T) {
	watcher := prov("watcher", "team")
	reg := regOf(subscriber(watcher, "w", "missing"))
	decisions, ambiguous := EligibleDiagnosed(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(starterOf(prov("sample", "alpha"), "pr")))
	if len(decisions) != 0 || len(ambiguous) != 0 {
		t.Fatalf("an uninstalled Starter is inactive, not an error: %v %v", decisions, ambiguous)
	}
}

func TestNoStarterFilterKeepsAnyStarterBehaviour(t *testing.T) {
	alpha, watcher := prov("sample", "alpha"), prov("watcher", "team")
	sa := starterOf(alpha, "pr")
	reg := regOf(sa, subscriber(watcher, "w"))
	if got := names(Eligible(reg, plugin.RoleImporter, plugin.EventStartFinalized, subjectFrom(sa))); !slices.Equal(got, []string{"watcher@team/w"}) {
		t.Fatalf("subscription without a filter must follow any Starter: %v", got)
	}
}

// privateWork is a Work whose snapshot holds the same local name stored by two
// homonymous providers, each under its own source instance.
func privateWork(alpha, beta pluginid.Provider, idA, idB string) Subject {
	meta := map[string]any{
		semconv.StorageKey(alpha, idA, "build_id"): "from-alpha",
		semconv.StorageKey(beta, idB, "build_id"):  "from-beta",
	}
	s := subject("new", nil, meta)
	return s
}

func registered(p pluginid.Provider, id string) registry.Package {
	pkg := installfx.Package(p, true)
	pkg.Provenance.RegistrationID = id
	return pkg
}

const (
	idAlpha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	idBeta  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	idNew   = "cccccccccccccccccccccccccccccccc"
)

func TestPrivateInputsReadOnlyTheDeclaringSourceInstance(t *testing.T) {
	alpha, beta := prov("sample", "alpha"), prov("sample", "beta")
	reg := registry.New()
	reg.UpsertPackage(registered(alpha, idAlpha))
	reg.UpsertPackage(registered(beta, idBeta))
	s := privateWork(alpha, beta, idAlpha, idBeta)

	got, missing, err := ResolveInputs(reg, alpha, []string{"meta:plugin.sample.build_id"}, s)
	if err != nil || len(missing) != 0 || got["plugin.sample.build_id"] != "from-alpha" {
		t.Fatalf("alpha read %v, missing %v, %v", got, missing, err)
	}
	got, _, _ = ResolveInputs(reg, beta, []string{"meta:plugin.sample.build_id"}, s)
	if got["plugin.sample.build_id"] != "from-beta" {
		t.Fatalf("beta read %v", got)
	}
	// The subprocess still sees the declared, provider-independent key.
	if _, ok := got[semconv.StorageKey(beta, idBeta, "build_id")]; ok {
		t.Fatal("the core storage key leaked into the subprocess document")
	}
}

func TestQualifiedInputBindsToExactlyThatProvider(t *testing.T) {
	alpha, beta, reader := prov("sample", "alpha"), prov("sample", "beta"), prov("reader", "team")
	reg := registry.New()
	reg.UpsertPackage(registered(alpha, idAlpha))
	reg.UpsertPackage(registered(beta, idBeta))
	reg.UpsertPackage(registered(reader, idNew))
	s := privateWork(alpha, beta, idAlpha, idBeta)

	got, missing, err := ResolveInputs(reg, reader, []string{"meta:plugin.sample@beta.build_id"}, s)
	if err != nil || len(missing) != 0 || got["plugin.sample@beta.build_id"] != "from-beta" {
		t.Fatalf("qualified read %v, missing %v, %v", got, missing, err)
	}
	// A relative key of another plugin never reads, even when it is installed.
	_, missing, _ = ResolveInputs(reg, reader, []string{"meta:plugin.sample.build_id"}, s)
	if len(missing) != 1 {
		t.Fatalf("a foreign relative key must not resolve: %v", missing)
	}
}

func TestReRegisteredSourceDoesNotInheritPrivateValues(t *testing.T) {
	alpha := prov("sample", "alpha")
	s := subject("new", nil, map[string]any{semconv.StorageKey(alpha, idAlpha, "build_id"): "old"})

	// Same public names, a new source instance: the earlier value is invisible.
	reg := registry.New()
	reg.UpsertPackage(registered(alpha, idNew))
	got, missing, _ := ResolveInputs(reg, alpha, []string{"meta:plugin.sample.build_id"}, s)
	if len(got) != 0 || len(missing) != 1 {
		t.Fatalf("replacement source read the earlier instance's value: %v %v", got, missing)
	}
	reg = registry.New()
	reg.UpsertPackage(registered(alpha, idAlpha))
	if got, _, _ = ResolveInputs(reg, alpha, []string{"meta:plugin.sample.build_id"}, s); got["plugin.sample.build_id"] != "old" {
		t.Fatalf("the original instance lost its value: %v", got)
	}
}

func TestOptionalPrivateInputIsOmittedWhenAbsent(t *testing.T) {
	alpha := prov("sample", "alpha")
	reg := registry.New()
	reg.UpsertPackage(registered(alpha, idAlpha))
	got, missing, err := ResolveInputs(reg, alpha, []string{"meta:plugin.sample.nope:optional", "link:plugin.sample@gone.x:optional"}, subject("new", nil, nil))
	if err != nil || len(got) != 0 || len(missing) != 0 {
		t.Fatalf("optional private inputs must be omitted: %v %v %v", got, missing, err)
	}
}

func TestDottedProviderPrivateInput(t *testing.T) {
	dotted := prov("acme.tools", "team.dev")
	reg := registry.New()
	reg.UpsertPackage(registered(dotted, idAlpha))
	s := subject("new", nil, map[string]any{semconv.StorageKey(dotted, idAlpha, "x"): 7})
	got, missing, err := ResolveInputs(reg, dotted, []string{"meta:plugin.acme%2Etools.x", "meta:plugin.acme%2Etools@team%2Edev.x"}, s)
	if err != nil || len(missing) != 0 || got["plugin.acme%2Etools.x"] != 7 {
		t.Fatalf("dotted provider read %v, missing %v, %v", got, missing, err)
	}
}

func TestLinkInputsAreBoundLikeMetaInputs(t *testing.T) {
	alpha, beta := prov("sample", "alpha"), prov("sample", "beta")
	reg := registry.New()
	reg.UpsertPackage(registered(alpha, idAlpha))
	reg.UpsertPackage(registered(beta, idBeta))
	links := map[string]string{
		semconv.StorageKey(alpha, idAlpha, "url"): "https://a.example/1",
		semconv.StorageKey(beta, idBeta, "url"):   "https://b.example/1",
	}
	s := subject("new", links, nil)
	s.State.Work.StartMode = work.StartModeNew
	got, _, _ := ResolveInputs(reg, beta, []string{"link:plugin.sample.url"}, s)
	if got["plugin.sample.url"] != "https://b.example/1" {
		t.Fatalf("beta read %v", got)
	}
}
