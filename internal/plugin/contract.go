package plugin

import "sort"

// RoleContract is a detached description of the rules used by Parse.
type RoleContract struct {
	Required, Allowed, Forbidden []string
	AnyOf                        [][]string
}
type Descriptor struct {
	TopLevel, Component, Convention, Roles, Accepts, Events []string
	InputsGrammar                                           string
	Rules                                                   map[string]RoleContract
}

func sortedCopy(v []string) []string { r := append([]string{}, v...); sort.Strings(r); return r }

// Contract returns independent, sorted copies; callers cannot alter validation.
func Contract() Descriptor {
	d := Descriptor{sortedCopy(topLevelKeys), sortedCopy(allComponentKeys), sortedCopy(conventionKeys), sortedCopy(validRoles), sortedCopy(acceptsVocab), sortedCopy(CoreEvents), inputsGrammar.String(), map[string]RoleContract{}}
	for name, r := range roleRules {
		groups := [][]string{}
		for _, g := range r.anyOf {
			groups = append(groups, sortedCopy(g))
		}
		d.Rules[name] = RoleContract{sortedCopy(r.required), sortedCopy(r.allowed), sortedCopy(r.forbidden), groups}
	}
	return d
}
