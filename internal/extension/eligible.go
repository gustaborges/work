package extension

import (
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
)

// Eligible returns the installed components of role (plugin.RoleLinker or
// plugin.RoleImporter) that run automatically for event, each with the inputs
// they are sent, ordered by qualified name.
//
// The decision is static: nothing is started to make it, and a component that
// is not eligible leaves no trace. A component runs only when it is set up for
// automatic use, subscribes to event for the Starter that produced the Work,
// and every input it does not mark optional resolves. The order is bytewise on
// "<alias>/<name>", so it does not depend on registry order; it is
// reproducible, not a contract authors may rely on.
func Eligible(reg *registry.Registry, role, event string, s Subject) []Decision {
	var out []Decision
	for _, c := range reg.Extensions(role) {
		if !subscribed(c, event, s.Starter) {
			continue
		}
		inputs, ok := resolveInputs(c.Inputs, s)
		if !ok {
			continue
		}
		out = append(out, Decision{Component: c, Inputs: inputs})
	}
	slices.SortFunc(out, func(a, b Decision) int {
		return strings.Compare(a.Component.QualifiedName(), b.Component.QualifiedName())
	})
	return out
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

func subscribed(c registry.Component, event string, starter registry.Component) bool {
	for _, sub := range subscriptions(c) {
		if sub.Event == event && starterAllowed(sub.Starters, starter) {
			return true
		}
	}
	return false
}

// starterAllowed applies a subscription's Starter restriction: a bare name
// matches any Starter with that name, "<alias>/<name>" matches exactly one.
func starterAllowed(starters []string, s registry.Component) bool {
	if len(starters) == 0 {
		return true
	}
	for _, ref := range starters {
		if strings.Contains(ref, "/") {
			if ref == s.QualifiedName() {
				return true
			}
		} else if ref == s.Name {
			return true
		}
	}
	return false
}

// resolveInputs looks up every declared input. It reports false when an input
// that is not optional has no value; optional ones that are absent are left
// out of the result rather than sent empty.
func resolveInputs(declared []string, s Subject) (map[string]any, bool) {
	inputs := make(map[string]any, len(declared))
	for _, raw := range declared {
		in, err := plugin.ParseInput(raw)
		if err != nil {
			return nil, false
		}
		v, found := lookup(in, s)
		switch {
		case found:
			inputs[in.Key] = v
		case !in.Optional:
			return nil, false
		}
	}
	return inputs, true
}

func lookup(in plugin.Input, s Subject) (any, bool) {
	switch in.Source {
	case plugin.SourceWork:
		return fact(in.Key, s)
	case plugin.SourceLink:
		v, ok := s.State.Links[in.Key]
		return v, ok
	case plugin.SourceMeta:
		v, ok := s.State.Meta[in.Key]
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
