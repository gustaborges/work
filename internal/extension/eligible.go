package extension

import (
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/semconv"
)

// Eligible returns the installed components of role (plugin.RoleLinker or
// plugin.RoleImporter) that run automatically for event, each with the inputs
// they are sent, ordered by qualified name.
//
// The decision is static: nothing is started to make it, and a component that
// is not eligible leaves no trace. A component runs only when it is set up for
// automatic use, subscribes to event for the Starter that produced the Work,
// and every input it does not mark optional resolves. The order is bytewise on
// "<plugin>@<marketplace>/<name>", so it does not depend on registry order; it
// is reproducible, not a contract authors may rely on.
func Eligible(reg *registry.Registry, role, event string, s Subject) []Decision {
	decisions, _ := EligibleDiagnosed(reg, role, event, s)
	return decisions
}

// Ambiguity names a subscriber that was withheld because one of its Starter
// filters matched more than one enabled Starter and none was singled out.
type Ambiguity struct {
	Component registry.Component
	Filter    string
}

// EligibleDiagnosed is Eligible plus the subscribers it refused to run because
// a bare Starter name was ambiguous. Such a subscriber never executes.
func EligibleDiagnosed(reg *registry.Registry, role, event string, s Subject) ([]Decision, []Ambiguity) {
	var out []Decision
	var ambiguous []Ambiguity
	for _, c := range reg.Effective().Extensions(role) {
		ok, filter := subscribed(reg, c, event, s.Starter)
		if filter != "" {
			ambiguous = append(ambiguous, Ambiguity{Component: c, Filter: filter})
		}
		if !ok {
			continue
		}
		inputs, missing, err := ResolveInputs(reg, c.Provider, c.Inputs, s)
		if err != nil || len(missing) != 0 {
			continue
		}
		out = append(out, Decision{Component: c, Inputs: inputs})
	}
	slices.SortFunc(out, func(a, b Decision) int {
		return strings.Compare(a.Component.QualifiedName(), b.Component.QualifiedName())
	})
	slices.SortFunc(ambiguous, func(a, b Ambiguity) int {
		return strings.Compare(a.Component.QualifiedName(), b.Component.QualifiedName())
	})
	return out, ambiguous
}

// subscriptions returns the subscriptions that make c run automatically. A
// Linker without a key, or one not marked automatic, has none; a component
// registered before activation data existed has none either.
func subscriptions(c registry.Component) []plugin.Subscription {
	switch c.Role {
	case plugin.RoleLinker:
		if c.Key == "" || c.Discover == nil || !c.Discover.Automatic {
			return nil
		}
		return c.Discover.On
	case plugin.RoleImporter:
		return c.On
	}
	return nil
}

// subscribed reports whether c runs for event when starter produced the Work.
// A non-empty second result is a bare Starter filter that was ambiguous, which
// withholds the subscription.
func subscribed(reg *registry.Registry, c registry.Component, event string, starter registry.Component) (bool, string) {
	var ambiguous string
	for _, sub := range subscriptions(c) {
		if sub.Event != event {
			continue
		}
		allowed, filter := starterAllowed(reg, c, sub.Starters, starter)
		if allowed {
			return true, ""
		}
		if filter != "" && ambiguous == "" {
			ambiguous = filter
		}
	}
	return false, ambiguous
}

// starterAllowed applies a subscription's Starter restriction. No filter means
// any Starter. Otherwise each filter is, in order:
//
//   - "plugin@marketplace/name": exactly that component;
//   - a bare name the subscribing package itself declares as a Starter: that
//     package's own Starter, never a homonym from another provider;
//   - any other bare name: the one enabled Starter with that name. With none
//     the filter is inactive; with several it is ambiguous and the second
//     result names it, so the subscriber is withheld rather than guessed.
func starterAllowed(reg *registry.Registry, subscriber registry.Component, starters []string, s registry.Component) (bool, string) {
	if len(starters) == 0 {
		return true, ""
	}
	ambiguous := ""
	for _, ref := range starters {
		if strings.Contains(ref, "/") {
			if ref == s.QualifiedName() {
				return true, ""
			}
			continue
		}
		if own, ok := ownStarter(reg, subscriber.Provider, ref); ok {
			if own.QualifiedName() == s.QualifiedName() {
				return true, ""
			}
			continue
		}
		if ref != s.Name {
			continue
		}
		var matching []registry.Component
		for _, c := range reg.Effective().ByRole(registry.RoleStarter) {
			if c.Name == ref {
				matching = append(matching, c)
			}
		}
		switch len(matching) {
		case 1:
			if matching[0].QualifiedName() == s.QualifiedName() {
				return true, ""
			}
		default:
			if len(matching) > 1 && ambiguous == "" {
				ambiguous = ref
			}
		}
	}
	return false, ambiguous
}

func ownStarter(reg *registry.Registry, p pluginid.Provider, name string) (registry.Component, bool) {
	for _, c := range reg.ProviderComponents(p) {
		if c.Role == registry.RoleStarter && c.Name == name {
			return c, true
		}
	}
	return registry.Component{}, false
}

// ResolveInputs projects declared inputs without coercing their values. Missing
// required declarations are sorted; absent optional inputs are omitted, while
// present null metadata remains present. owner is the provider whose component
// declared them: its private keys resolve against its own source instance,
// and an explicitly qualified key against the currently registered instance of
// exactly that provider. The delivered key is always the declared spelling.
func ResolveInputs(reg *registry.Registry, owner pluginid.Provider, declared []string, s Subject) (map[string]any, []string, error) {
	inputs := make(map[string]any, len(declared))
	var missing []string
	for _, raw := range declared {
		in, err := plugin.ParseInput(raw)
		if err != nil {
			return nil, nil, err
		}
		v, found := lookup(reg, owner, in, s)
		if found {
			inputs[in.Key] = v
		} else if !in.Optional {
			missing = append(missing, raw)
		}
	}
	slices.Sort(missing)
	return inputs, missing, nil
}

// storedKey maps a declared meta or link key to the key the Work stores. Public
// keys are shared. A private key is bound to the registration instance of its
// provider, so a replacement source with equal public names never reads it.
func storedKey(reg *registry.Registry, owner pluginid.Provider, key string) (string, bool) {
	if !semconv.IsPrivate(key) {
		return key, true
	}
	k, err := semconv.ParsePrivate(key)
	if err != nil || k.Stored() {
		return "", false
	}
	provider := k.Provider()
	if k.Relative() {
		if k.Plugin != owner.Plugin {
			return "", false
		}
		provider = owner
	}
	pkg, ok := reg.PackageByProvider(provider)
	if !ok {
		return "", false
	}
	return semconv.StorageKey(provider, pkg.Provenance.RegistrationID, k.Local), true
}

func lookup(reg *registry.Registry, owner pluginid.Provider, in plugin.Input, s Subject) (any, bool) {
	switch in.Source {
	case plugin.SourceWork:
		return fact(in.Key, s)
	case plugin.SourceLink:
		key, ok := storedKey(reg, owner, in.Key)
		if !ok {
			return nil, false
		}
		v, ok := s.State.Links[key]
		return v, ok
	case plugin.SourceMeta:
		key, ok := storedKey(reg, owner, in.Key)
		if !ok {
			return nil, false
		}
		v, ok := s.State.Meta[key]
		return v, ok
	}
	return nil, false
}

// The work facts an input may name; they mirror semconv.Facts.
const (
	factWorktreePath = "worktree_path"
	factStartMode    = "start_mode"
	factBranch       = "branch"
	factBaseBranch   = "base_branch"
	factSlug         = "slug"
)

// fact resolves one of the exposed work facts. slug is the only one that can
// be absent: a contribution Work has none.
func fact(key string, s Subject) (any, bool) {
	w := s.State.Work
	switch key {
	case factWorktreePath:
		return s.WorktreePath, s.WorktreePath != ""
	case factStartMode:
		return w.StartMode, w.StartMode != ""
	case factBranch:
		return w.Branch, w.Branch != ""
	case factBaseBranch:
		return w.BaseBranch, w.BaseBranch != ""
	case factSlug:
		return w.Slug, w.Slug != ""
	}
	return nil, false
}

// Applicable reports whether anything would run automatically for s: some
// Linker or Importer is eligible right now. Importers that become eligible only
// through a Linker's result need an eligible Linker, so this is enough to skip
// the whole pipeline — and everything around it — when nothing is installed
// for it.
func Applicable(reg *registry.Registry, event string, s Subject) bool {
	return len(Eligible(reg, plugin.RoleLinker, event, s)) > 0 ||
		len(Eligible(reg, plugin.RoleImporter, event, s)) > 0
}
