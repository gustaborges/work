// Package starter selects and invokes the Starter component that turns the
// user's SOURCE argument into a repository reference. F1 has exactly one
// enabled Starter: the seed's fallback local-path Starter. F4 adds Match,
// which widens selection to every registered *specific* Starter (a non-empty
// Pattern), falling back to the single registered fallback only when no
// specific pattern matches (ADR-0004). The core reads only the typed fields
// of the response — nothing a Starter emits can influence core-governed Work
// state (FR-018).
package starter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/workhome"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/semconv"
	"github.com/gustaborges/work/internal/work"
)

// LogicalName is the stable name recorded in work.starter for the seed Starter.
const LogicalName = "local-path-starter"

// Select returns the Starter that will handle this run. In F1 that is always
// the single fallback-layer Starter registered by bootstrap.
func Select(reg *registry.Registry) (registry.Component, error) {
	c, ok := reg.Effective().StarterFallback()
	if !ok {
		return registry.Component{}, diag.New(diag.BootstrapFailed,
			"no Starter is registered; run `work plugin add work-reference@builtin` or install and enable a Starter")
	}
	return c, nil
}

// Outcome carries a Starter-pattern collision (ADR-0004): populated iff two
// or more registered specific Starters' patterns matched the argument, in
// which case Match's own Component return is the zero value and the caller
// must choose among Ambiguous itself — interactively (a present.Select step)
// or by failing starter-ambiguous (36) non-interactively. The choice is never
// memoized anywhere on disk (FR-012, SC-006). This mirrors
// internal/locator.Outcome's Resolved/Candidates split for the structurally
// identical repository-ambiguity problem (research R7).
type Outcome struct {
	Ambiguous []registry.Component
}

// Match resolves which Starter handles arg (ADR-0004, contracts/
// starter-protocol.md §Selection): every registered starter component with a
// non-empty Pattern is compiled as a Go-syntax regular expression and
// evaluated against arg, locally, before any subprocess runs. Zero matches
// falls back to registry.StarterFallback (starter-not-matched, 35, if none is
// registered); exactly one match is returned directly; two or more return an
// Outcome carrying every colliding component, with a zero Component — no
// score, rank, specificity, or installation-order tiebreak decides among
// them.
func Match(reg *registry.Registry, arg string) (registry.Component, Outcome, error) {
	var matches []registry.Component
	for _, c := range reg.Effective().ByRole(registry.RoleStarter) {
		pattern := strings.TrimSpace(c.Pattern)
		if pattern == "" {
			continue // the fallback layer is never pattern-matched
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			// internal/plugin rejects a non-compiling pattern at install, so
			// only a hand-edited registry can hold one; treat it as never
			// matching rather than failing every `work start` call.
			continue
		}
		if re.MatchString(arg) {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		fb, ok := reg.Effective().StarterFallback()
		if !ok {
			return registry.Component{}, Outcome{}, diag.New(diag.StarterNotMatched,
				"no Starter matches this argument; run `work plugin add work-reference@builtin` or install and enable a matching Starter").
				WithSummary("Nothing recognizes this argument.").
				WithHint("Run `work plugin add work-reference@builtin`, add a matching Starter, or enable a fallback with `work plugin enable <plugin>@<marketplace>`.")
		}
		return fb, Outcome{}, nil
	case 1:
		return matches[0], Outcome{}, nil
	default:
		return registry.Component{}, Outcome{Ambiguous: matches}, nil
	}
}

// ModeContext contains the contextual overrides for one creation mode.
type ModeContext struct {
	Links map[string]string `json:"links"`
	Meta  map[string]any    `json:"meta"`
}

// Reference is the subset of a Starter response the core acts on (ADR-0016).
// The four repository fields are independent and optional; a reference with
// none of them set is not rejected here — internal/locator classifies that
// case as no-eligible-locator. Meta and Links carry the
// context the Starter publishes for the Work; they are checked by
// ValidateResponse before anything is materialized. Nothing else a Starter
// emits is reachable through this type.
//
// Modes holds the per-creation-mode overrides already decoded by Invoke: names
// are limited to new, contribution and fork and every entry is well formed.
// Their keys are not yet checked against the publishing plugin; that is
// ValidateResponse's job.
type Reference struct {
	Modes        map[string]ModeContext
	Path         string
	GitFetchURLs []string
	Name         string
	Query        string
	BaseBranch   string
	StartModes   []string
	Meta         map[string]any
	Links        map[string]string
}

// Invoke runs the Starter entrypoint with arg and returns the repository
// reference it produced. A non-zero exit or unparseable output is reported as
// an unusable repository. A malformed "modes" field is reported as
// starter-response-invalid instead, so a Starter bug is not mistaken for a
// source the user can correct and retry.
func Invoke(pluginsDir string, c registry.Component, arg string) (Reference, error) {
	resp, err := ipc.InvokeStarter(c.Target(pluginsDir), ipc.StarterInput{Arg: arg})
	if err != nil {
		return Reference{}, diag.Wrap(diag.UnusableRepo, err,
			"the Starter could not resolve the given source")
	}
	modes, err := decodeModes(resp.Modes)
	if err != nil {
		return Reference{}, err
	}
	// Only the typed fields are consumed; any extra key the subprocess
	// emitted is deliberately ignored: nothing a Starter says can influence
	// core-governed Work state.
	return Reference{
		Modes:        modes,
		Path:         resp.Repository.Path,
		GitFetchURLs: resp.Repository.GitFetchURLs,
		Name:         resp.Repository.Name,
		Query:        resp.Repository.Query,
		BaseBranch:   resp.BaseBranch,
		StartModes:   resp.StartModes,
		Meta:         resp.Meta,
		Links:        resp.Links,
	}, nil
}

// ValidateResponse enforces the structural rules starter-protocol.md places on
// a Starter's response beyond what Invoke's JSON decoding already checks
// (research R9): every StartModes value must be work.StartModeFork or
// work.StartModeContribution, and StartModes containing StartModeContribution
// requires a non-empty BaseBranch — contribution mode never prompts for one,
// so an absent base branch here can never be filled in later. Both violations
// are starter-response-invalid (37), the same failure shape as a malformed
// response, checked before any Work materialization.
//
// owner is the publishing plugin's manifest name: every Meta and Links key
// must be a valid Semantic Conventions key that owner may publish, and every
// link value a non-empty string. The error names the offending key, never a
// value.
func ValidateResponse(ref Reference, owner string) error {
	hasContribution := false
	for _, m := range ref.StartModes {
		switch m {
		case work.StartModeFork:
		case work.StartModeContribution:
			hasContribution = true
		default:
			return structurallyInvalid("the Starter returned an unrecognized start mode %q", m)
		}
	}
	if hasContribution && ref.BaseBranch == "" {
		return structurallyInvalid("the Starter offered contribution mode with no base branch to check out")
	}
	if err := validatePublished(ref, owner, ""); err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(ref.Modes)) {
		entry := ref.Modes[name]
		if err := validatePublished(Reference{Links: entry.Links, Meta: entry.Meta}, owner, name); err != nil {
			return err
		}
	}
	return nil
}

// structurallyInvalid is the one place a malformed Starter response gets its
// code, summary and hint, so every such failure reads the same at the CLI border.
func structurallyInvalid(format string, args ...any) *diag.Error {
	return diag.Newf(diag.StarterResponseInvalid, format, args...).
		WithSummary("The Starter's response is structurally invalid.").
		WithHint("This is a bug in the installed Starter, not something fixable from the command line.")
}

// validatePublished checks the published keys in sorted order so the key a
// failure names does not depend on map iteration. mode is the creation mode
// whose override is being checked, or "" for the base maps; it goes into the
// message itself because the CLI border shows only the diagnostic's own text,
// not the text of any error wrapped around it.
func validatePublished(ref Reference, owner, mode string) error {
	where := ""
	if mode != "" {
		where = fmt.Sprintf(" in mode %q", mode)
	}
	invalidKey := func(section, key string) error {
		return structurallyInvalid("the Starter returned a %s key that is not valid%s: %q", section, where, key)
	}
	for _, key := range slices.Sorted(maps.Keys(ref.Meta)) {
		if semconv.ValidatePublished(owner, key) != nil {
			return invalidKey("metadata", key)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(ref.Links)) {
		if semconv.ValidatePublished(owner, key) != nil {
			return invalidKey("link", key)
		}
		if semconv.ValidLinkValue(ref.Links[key]) != nil {
			return structurallyInvalid("the Starter returned an empty value for the link %q%s", key, where)
		}
	}
	return nil
}

// Bind returns ref with every private key the Starter published rewritten to
// the stored form of the publishing source instance, so homonymous providers
// and re-registered sources never share or inherit each other's private
// values. Public keys are shared semantic keys and are unchanged. ref must
// already have passed ValidateResponse for c; the maps are copied.
func Bind(ref Reference, c registry.Component, reg *registry.Registry) Reference {
	pkg, ok := reg.PackageByProvider(c.Provider)
	if !ok {
		return ref
	}
	id := pkg.Provenance.RegistrationID
	if ref.Meta != nil {
		meta := make(map[string]any, len(ref.Meta))
		for k, v := range ref.Meta {
			meta[semconv.BindPublished(c.Provider, id, k)] = v
		}
		ref.Meta = meta
	}
	if ref.Links != nil {
		links := make(map[string]string, len(ref.Links))
		for k, v := range ref.Links {
			links[semconv.BindPublished(c.Provider, id, k)] = v
		}
		ref.Links = links
	}
	return ref
}

// InvokeGuarded protects the selected package revision through IPC completion.
func InvokeGuarded(ctx context.Context, h workhome.Home, c registry.Component, arg string) (Reference, error) {
	var ref Reference
	err := adminstate.WithInvocation(ctx, h, c, func() error { var err error; ref, err = Invoke(h.PluginsDir(), c, arg); return err })
	return ref, err
}

// decodeModes turns the wire "modes" payload into typed mode context. Shape
// failures stay in the Starter diagnostic domain (code 37) rather than the
// UnusableRepo class Invoke gives IPC errors. Names and keys come from the
// Starter, so they are always quoted.
func decodeModes(raw json.RawMessage) (map[string]ModeContext, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	if raw[0] != '{' {
		return nil, structurallyInvalid("the Starter returned invalid modes")
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return nil, structurallyInvalid("the Starter returned invalid modes")
	}
	out := make(map[string]ModeContext, len(entries))
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		if name != "new" && name != "contribution" && name != "fork" {
			return nil, structurallyInvalid("the Starter returned invalid mode name %q", name)
		}
		value := bytes.TrimSpace(entries[name])
		if len(value) == 0 || value[0] != '{' {
			return nil, structurallyInvalid("the Starter returned invalid mode %q", name)
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(value, &fields) != nil {
			return nil, structurallyInvalid("the Starter returned invalid mode %q", name)
		}
		var entry ModeContext
		if json.Unmarshal(value, &entry) != nil {
			return nil, structurallyInvalid("the Starter returned invalid maps in mode %q", name)
		}
		// encoding/json accepts null for string values; link values must be strings.
		if links := fields["links"]; len(links) > 0 && !bytes.Equal(bytes.TrimSpace(links), []byte("null")) {
			var values map[string]json.RawMessage
			if json.Unmarshal(links, &values) != nil {
				return nil, structurallyInvalid("the Starter returned invalid links in mode %q", name)
			}
			for _, key := range slices.Sorted(maps.Keys(values)) {
				v := bytes.TrimSpace(values[key])
				if len(v) == 0 || v[0] != '"' {
					return nil, structurallyInvalid("the Starter returned an invalid value for the link %q in mode %q", key, name)
				}
			}
		}
		out[name] = entry
	}
	return out, nil
}

// MergeMode copies the published maps and shallowly overlays the selected mode.
// Call ValidateResponse first; invalid responses must never be materialized.
func MergeMode(ref Reference, mode string) Reference {
	if mode == "" {
		mode = work.StartModeNew
	}
	entry := ref.Modes[mode]
	ref.Links = maps.Clone(ref.Links)
	ref.Meta = maps.Clone(ref.Meta)
	if len(entry.Links) > 0 && ref.Links == nil {
		ref.Links = make(map[string]string)
	}
	if len(entry.Meta) > 0 && ref.Meta == nil {
		ref.Meta = make(map[string]any)
	}
	maps.Copy(ref.Links, entry.Links)
	maps.Copy(ref.Meta, entry.Meta)
	return ref
}
