// Package plugin parses and validates plugin.json. F1 only runs this against
// the embedded seed, but the validator implements the rule-sets for all four
// component roles so later slices need no change.
package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Manifest is a parsed, validated plugin.json.
type Manifest struct {
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Components  []Component  `json:"components"`
	Conventions []Convention `json:"conventions"`
}

// Component is one entry of components[].
type Component struct {
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Entrypoint  string   `json:"entrypoint"`
	Runtime     string   `json:"runtime,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Accepts     []string `json:"accepts,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Description string   `json:"description,omitempty"`
	On          []string `json:"on,omitempty"`
	Manual      bool     `json:"manual,omitempty"`
	Inputs      []string `json:"inputs,omitempty"`
	Key         string   `json:"key,omitempty"`
	Discover    []string `json:"discover,omitempty"`
}

// Convention is one entry of conventions[].
type Convention struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

// IsFallbackStarter reports whether c is a fallback-layer starter (role starter
// with no pattern).
func (c Component) IsFallbackStarter() bool {
	return c.Role == RoleStarter && strings.TrimSpace(c.Pattern) == ""
}

// Roles.
const (
	RoleStarter           = "starter"
	RoleRepositoryLocator = "repository-locator"
	RoleImporter          = "importer"
	RoleLinker            = "linker"
)

var validRoles = []string{RoleStarter, RoleRepositoryLocator, RoleImporter, RoleLinker}

// allComponentKeys is every key any role may carry. A component key outside this
// set is invalid regardless of role (this is where invocation, type,
// capabilities, hooks, priority, score, and typos are rejected).
var allComponentKeys = []string{
	"name", "role", "entrypoint", "runtime", "pattern", "accepts",
	"display_name", "description", "on", "manual", "inputs", "key", "discover",
}

var topLevelKeys = []string{"name", "version", "components", "conventions"}

var conventionKeys = []string{"name", "prefixes"}

var acceptsVocab = []string{"git_fetch_urls", "name", "query"}

type roleRule struct {
	required  []string
	allowed   []string // beyond required
	forbidden []string
	anyOf     [][]string // at least one key from each group must be present
}

var roleRules = map[string]roleRule{
	RoleStarter: {
		required:  []string{"name", "role", "entrypoint"},
		allowed:   []string{"pattern", "runtime"},
		forbidden: []string{"accepts", "display_name", "description", "on", "manual", "inputs", "key", "discover"},
	},
	RoleRepositoryLocator: {
		required:  []string{"name", "role", "entrypoint", "accepts"},
		allowed:   []string{"runtime", "display_name", "description"},
		forbidden: []string{"pattern", "on", "manual", "inputs", "key", "discover"},
	},
	RoleImporter: {
		required:  []string{"name", "role", "entrypoint"},
		allowed:   []string{"on", "manual", "inputs", "runtime"},
		forbidden: []string{"pattern", "accepts", "display_name", "description", "key", "discover"},
		anyOf:     [][]string{{"on", "manual"}},
	},
	RoleLinker: {
		required:  []string{"name", "role", "key", "entrypoint"},
		allowed:   []string{"discover", "manual", "runtime"},
		forbidden: []string{"pattern", "accepts", "display_name", "description", "on", "inputs"},
		anyOf:     [][]string{{"discover", "manual"}},
	},
}

var inputsGrammar = regexp.MustCompile(`^(work|meta|link):[^:]+(:optional)?$`)

// Parse decodes data as plugin.json and fully validates it.
func Parse(data []byte) (*Manifest, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("plugin.json is not a JSON object: %w", err)
	}
	for k := range top {
		if !slices.Contains(topLevelKeys, k) {
			return nil, fmt.Errorf("plugin.json: unknown top-level field %q", k)
		}
	}
	for _, k := range topLevelKeys {
		if _, ok := top[k]; !ok {
			return nil, fmt.Errorf("plugin.json: missing required field %q", k)
		}
	}

	var m Manifest
	if err := json.Unmarshal(top["name"], &m.Name); err != nil || strings.TrimSpace(m.Name) == "" {
		return nil, fmt.Errorf("plugin.json: %q must be a non-empty string", "name")
	}
	if err := json.Unmarshal(top["version"], &m.Version); err != nil || strings.TrimSpace(m.Version) == "" {
		return nil, fmt.Errorf("plugin.json: %q must be a non-empty string", "version")
	}

	var rawComponents []map[string]json.RawMessage
	if err := json.Unmarshal(top["components"], &rawComponents); err != nil {
		return nil, fmt.Errorf("plugin.json: components must be an array")
	}
	for i, rc := range rawComponents {
		c, err := parseComponent(rc)
		if err != nil {
			return nil, fmt.Errorf("plugin.json: components[%d]: %w", i, err)
		}
		m.Components = append(m.Components, c)
	}

	var rawConventions []map[string]json.RawMessage
	if err := json.Unmarshal(top["conventions"], &rawConventions); err != nil {
		return nil, fmt.Errorf("plugin.json: conventions must be an array")
	}
	for i, rv := range rawConventions {
		cv, err := parseConvention(rv)
		if err != nil {
			return nil, fmt.Errorf("plugin.json: conventions[%d]: %w", i, err)
		}
		m.Conventions = append(m.Conventions, cv)
	}

	return &m, nil
}

func parseComponent(raw map[string]json.RawMessage) (Component, error) {
	var c Component

	for k := range raw {
		if !slices.Contains(allComponentKeys, k) {
			return c, fmt.Errorf("unknown field %q", k)
		}
	}

	var role string
	if err := json.Unmarshal(raw["role"], &role); err != nil {
		return c, fmt.Errorf("role must be a string")
	}
	if !slices.Contains(validRoles, role) {
		return c, fmt.Errorf("unknown role %q", role)
	}
	rule := roleRules[role]

	for _, k := range rule.required {
		if _, ok := raw[k]; !ok {
			return c, fmt.Errorf("role %q requires field %q", role, k)
		}
	}
	for _, k := range rule.forbidden {
		if _, ok := raw[k]; ok {
			return c, fmt.Errorf("role %q forbids field %q", role, k)
		}
	}
	for _, group := range rule.anyOf {
		if !slices.ContainsFunc(group, func(k string) bool { _, ok := raw[k]; return ok }) {
			return c, fmt.Errorf("role %q requires at least one of %v", role, group)
		}
	}

	// Decode into the typed struct now that keys are known-good.
	blob, _ := json.Marshal(raw)
	dec := json.NewDecoder(strings.NewReader(string(blob)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, err
	}

	if strings.TrimSpace(c.Name) == "" {
		return c, fmt.Errorf("name must be non-empty")
	}
	if strings.TrimSpace(c.Entrypoint) == "" {
		return c, fmt.Errorf("entrypoint must be non-empty")
	}

	if role == RoleRepositoryLocator {
		if len(c.Accepts) == 0 {
			return c, fmt.Errorf("repository-locator requires a non-empty accepts")
		}
		for _, a := range c.Accepts {
			if !slices.Contains(acceptsVocab, a) {
				return c, fmt.Errorf("repository-locator accepts %q is not one of %v", a, acceptsVocab)
			}
		}
	}
	for _, in := range c.Inputs {
		if !inputsGrammar.MatchString(in) {
			return c, fmt.Errorf("inputs entry %q does not match <work|meta|link>:<key>[:optional]", in)
		}
	}

	return c, nil
}

func parseConvention(raw map[string]json.RawMessage) (Convention, error) {
	var cv Convention
	for k := range raw {
		if !slices.Contains(conventionKeys, k) {
			return cv, fmt.Errorf("unknown field %q (conventions carry only name and prefixes)", k)
		}
	}
	if err := json.Unmarshal(raw["name"], &cv.Name); err != nil || strings.TrimSpace(cv.Name) == "" {
		return cv, fmt.Errorf("name must be a non-empty string")
	}
	if _, ok := raw["prefixes"]; !ok {
		return cv, fmt.Errorf("missing prefixes")
	}
	if err := json.Unmarshal(raw["prefixes"], &cv.Prefixes); err != nil || len(cv.Prefixes) == 0 {
		return cv, fmt.Errorf("prefixes must be a non-empty array of strings")
	}
	return cv, nil
}
