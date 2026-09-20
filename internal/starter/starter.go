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
	c, ok := reg.StarterFallback()
	if !ok {
		return registry.Component{}, diag.New(diag.BootstrapFailed,
			"no Starter is registered; the reference package is not installed")
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
	for _, c := range reg.ByRole(registry.RoleStarter) {
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
		fb, ok := reg.StarterFallback()
		if !ok {
			return registry.Component{}, Outcome{}, diag.New(diag.StarterNotMatched,
				"no Starter matches this argument, and none is registered as a fallback").
				WithSummary("Nothing recognizes this argument.").
				WithHint("Install a Starter that matches it, or enable a fallback Starter.")
		}
		return fb, Outcome{}, nil
	case 1:
		return matches[0], Outcome{}, nil
	default:
		return registry.Component{}, Outcome{Ambiguous: matches}, nil
	}
}

// Reference is the subset of a Starter response the core acts on (ADR-0016).
// The four repository fields are independent and optional; a reference with
// none of them set is not rejected here — internal/locator classifies that
// case as no-eligible-locator (FR-005, research R8). Meta and Links carry the
// context the Starter publishes for the Work; they are checked by
// ValidateResponse before anything is materialized. Nothing else a Starter
// emits is reachable through this type.
type Reference struct {
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
// reference it produced. A non-zero exit or a structurally invalid response is
// reported as an unusable repository.
func Invoke(pluginsDir string, c registry.Component, arg string) (Reference, error) {
	resp, err := ipc.InvokeStarter(c.Target(pluginsDir), ipc.StarterInput{Arg: arg})
	if err != nil {
		return Reference{}, diag.Wrap(diag.UnusableRepo, err,
			"the Starter could not resolve the given source")
	}
	// Only the typed fields are consumed; any extra key the subprocess
	// emitted is deliberately ignored (FR-018 trust boundary).
	return Reference{
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
			return diag.Newf(diag.StarterResponseInvalid,
				"the Starter returned an unrecognized start mode %q", m).
				WithSummary("The Starter's response is structurally invalid.").
				WithHint("This is a bug in the installed Starter, not something fixable from the command line.")
		}
	}
	if hasContribution && ref.BaseBranch == "" {
		return diag.New(diag.StarterResponseInvalid,
			"the Starter offered contribution mode with no base branch to check out").
			WithSummary("The Starter's response is structurally invalid.").
			WithHint("This is a bug in the installed Starter, not something fixable from the command line.")
	}
	return validatePublished(ref, owner)
}

// validatePublished checks the published keys in sorted order so the key a
// failure names does not depend on map iteration.
func validatePublished(ref Reference, owner string) error {
	invalidKey := func(section, key string) error {
		return diag.Newf(diag.StarterResponseInvalid,
			"the Starter returned a %s key that is not valid: %q", section, key).
			WithSummary("The Starter's response is structurally invalid.").
			WithHint("This is a bug in the installed Starter, not something fixable from the command line.")
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
			return diag.Newf(diag.StarterResponseInvalid,
				"the Starter returned an empty value for the link %q", key).
				WithSummary("The Starter's response is structurally invalid.").
				WithHint("This is a bug in the installed Starter, not something fixable from the command line.")
		}
	}
	return nil
}
