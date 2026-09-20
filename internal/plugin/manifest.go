// Package plugin parses and validates plugin.json for all four component roles,
// including the Importer and Linker activation declarations (events, Starter
// restrictions, manual availability and inputs).
package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/semconv"
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
	Name        string         `json:"name"`
	Role        string         `json:"role"`
	Entrypoint  string         `json:"entrypoint"`
	Runtime     string         `json:"runtime,omitempty"`
	Pattern     string         `json:"pattern,omitempty"`
	Accepts     []string       `json:"accepts,omitempty"`
	DisplayName string         `json:"display_name,omitempty"`
	Description string         `json:"description,omitempty"`
	On          []Subscription `json:"on,omitempty"`
	Manual      *Manual        `json:"manual,omitempty"`
	Inputs      []string       `json:"inputs,omitempty"`
	Key         string         `json:"key,omitempty"`
	Discover    *Discover      `json:"discover,omitempty"`
}

// Subscription subscribes a component to a core event. Starters, when
// present, restricts it to Works produced by the named Starters: each entry is
// a bare component name (any installed Starter with that name) or
// "<alias>/<name>" (exactly that component). A Starter that is not installed
// is not an error.
type Subscription struct {
	Event    string   `json:"event"`
	Starters []string `json:"starters,omitempty"`
}

// Manual makes a component available for manual invocation under
// DisplayName. It is validated and recorded but only acted on by a later
// slice.
type Manual struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
}

// Discover is a Linker's automatic-discovery declaration. Automatic without
// On is accepted and never runs on its own.
type Discover struct {
	Automatic bool           `json:"automatic"`
	On        []Subscription `json:"on,omitempty"`
	Inputs    []string       `json:"inputs,omitempty"`
}

// Input is a parsed "<work|meta|link>:<key>[:optional]" declaration.
type Input struct {
	Source   string
	Key      string
	Optional bool
}

// Input sources.
const (
	SourceWork = "work"
	SourceMeta = "meta"
	SourceLink = "link"
)

// EventStartFinalized is published once per `work start`, after the Work is
// fully created.
const EventStartFinalized = "start:finalized"

// CoreEvents are the events a component may subscribe to.
var CoreEvents = []string{EventStartFinalized}

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

// aliasGrammar is a single path segment: an alias names a directory under
// plugin storage, so it must not be able to leave it (separators, "." and
// "..") or be mistaken for a leftover dot-prefixed staging directory.
var aliasGrammar = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateAlias reports why alias cannot name an installed package. A ".old"
// suffix is refused because that is the backup name of a package being
// replaced, so alias "x.old" would collide with a replacement of "x".
func ValidateAlias(alias string) error {
	if !aliasGrammar.MatchString(alias) || strings.HasSuffix(alias, ".old") {
		return fmt.Errorf("%q is not a valid plugin alias: use letters, digits, '.', '_' and '-', starting with a letter or digit, and not ending in \".old\"", alias)
	}
	return nil
}

var inputsGrammar = regexp.MustCompile(`^(work|meta|link):([^:]+)(:optional)?$`)

// ParseInput parses one inputs entry and checks the key against the source:
// a work input must name an exposed fact, meta and link inputs must be valid
// Semantic Conventions keys.
func ParseInput(s string) (Input, error) {
	m := inputsGrammar.FindStringSubmatch(s)
	if m == nil {
		return Input{}, fmt.Errorf("inputs entry %q does not match <work|meta|link>:<key>[:optional]", s)
	}
	in := Input{Source: m[1], Key: m[2], Optional: m[3] != ""}
	if in.Source == SourceWork {
		if !slices.Contains(semconv.Facts, in.Key) {
			return Input{}, fmt.Errorf("inputs entry %q: %q is not an exposed work fact (%s)", s, in.Key, strings.Join(semconv.Facts, ", "))
		}
		return in, nil
	}
	if err := semconv.ValidKey(in.Key); err != nil {
		return Input{}, fmt.Errorf("inputs entry %q: %w", s, err)
	}
	return in, nil
}

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
	if err := ValidateAlias(m.Name); err != nil {
		return nil, fmt.Errorf("plugin.json: name: %w", err)
	}
	if err := json.Unmarshal(top["version"], &m.Version); err != nil || strings.TrimSpace(m.Version) == "" {
		return nil, fmt.Errorf("plugin.json: %q must be a non-empty string", "version")
	}

	var rawComponents []map[string]json.RawMessage
	if err := json.Unmarshal(top["components"], &rawComponents); err != nil {
		return nil, fmt.Errorf("plugin.json: components must be an array")
	}
	for i, rc := range rawComponents {
		c, err := parseComponent(rc, m.Name)
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

func parseComponent(raw map[string]json.RawMessage, pluginName string) (Component, error) {
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
	if role == RoleStarter && c.Pattern != "" {
		if _, err := regexp.Compile(c.Pattern); err != nil {
			return c, fmt.Errorf("pattern %q is not a valid regular expression: %w", c.Pattern, err)
		}
	}
	if err := validateActivation(c, pluginName); err != nil {
		return c, err
	}

	return c, nil
}

// validateActivation checks the Importer/Linker declarations: events,
// Starter restrictions, manual display name, inputs and Linker key ownership.
func validateActivation(c Component, pluginName string) error {
	if c.Role == RoleLinker {
		if err := semconv.ValidatePublished(pluginName, c.Key); err != nil {
			return fmt.Errorf("linker key: %w", err)
		}
	}
	if err := validateSubscriptions("on", c.On, c.On != nil); err != nil {
		return err
	}
	if c.Manual != nil && strings.TrimSpace(c.Manual.DisplayName) == "" {
		return fmt.Errorf("manual.display_name must be a non-empty string")
	}
	if err := validateInputs(c.Inputs); err != nil {
		return err
	}
	if d := c.Discover; d != nil {
		if err := validateSubscriptions("discover.on", d.On, d.On != nil); err != nil {
			return err
		}
		if err := validateInputs(d.Inputs); err != nil {
			return fmt.Errorf("discover: %w", err)
		}
	}
	return nil
}

func validateSubscriptions(field string, subs []Subscription, present bool) error {
	if present && len(subs) == 0 {
		return fmt.Errorf("%s must be a non-empty array of subscriptions", field)
	}
	for i, sub := range subs {
		if !slices.Contains(CoreEvents, sub.Event) {
			return fmt.Errorf("%s[%d]: %q is not a core event (%s)", field, i, sub.Event, strings.Join(CoreEvents, ", "))
		}
		if sub.Starters != nil && len(sub.Starters) == 0 {
			return fmt.Errorf("%s[%d]: starters must be non-empty when present", field, i)
		}
		for _, st := range sub.Starters {
			if !validStarterRef(st) {
				return fmt.Errorf("%s[%d]: starters entry %q must be <name> or <alias>/<name>", field, i, st)
			}
		}
	}
	return nil
}

func validStarterRef(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	alias, name, qualified := strings.Cut(s, "/")
	if !qualified {
		return true
	}
	return alias != "" && name != "" && !strings.Contains(name, "/")
}

// validateInputs parses every entry and rejects a key repeated in any
// namespace: the delivered document is keyed by the bare key.
func validateInputs(inputs []string) error {
	seen := map[string]string{}
	for _, raw := range inputs {
		in, err := ParseInput(raw)
		if err != nil {
			return err
		}
		if prev, dup := seen[in.Key]; dup {
			return fmt.Errorf("inputs entries %q and %q name the same key %q", prev, raw, in.Key)
		}
		seen[in.Key] = raw
	}
	return nil
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
