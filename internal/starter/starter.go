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
	"regexp"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/ipc"
	"github.com/gustaborges/work/internal/registry"
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
			// A malformed pattern can only reach the registry through
			// internal/plugin, which does not validate regex syntax; treat it
			// as never matching rather than failing every `work start` call.
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
// case as no-eligible-locator (FR-005, research R8). BaseBranch and
// StartModes are F4 additions, already present-but-unread on
// ipc.StarterResponse (research R8); Meta/Links stay off this type
// deliberately — persisting them is F5/F6 scope, and keeping them off the
// type (not merely unused by convention) is what makes "F4 does not consume
// them" a type constraint rather than a promise.
type Reference struct {
	Path         string
	GitFetchURLs []string
	Name         string
	Query        string
	BaseBranch   string
	StartModes   []string
}

// Invoke runs the Starter entrypoint with arg and returns the repository
// reference it produced. A non-zero exit or a structurally invalid response is
// reported as an unusable repository.
func Invoke(pluginsDir string, c registry.Component, arg string) (Reference, error) {
	entrypoint := c.EntrypointPath(pluginsDir)
	resp, err := ipc.InvokeStarter(entrypoint, ipc.StarterInput{Arg: arg})
	if err != nil {
		return Reference{}, diag.Wrap(diag.UnusableRepo, err,
			"the Starter could not resolve the given source")
	}
	// Only the typed reference fields are consumed; meta, links, and any
	// extra keys the subprocess emitted are deliberately ignored (FR-018
	// trust boundary).
	return Reference{
		Path:         resp.Repository.Path,
		GitFetchURLs: resp.Repository.GitFetchURLs,
		Name:         resp.Repository.Name,
		Query:        resp.Repository.Query,
		BaseBranch:   resp.BaseBranch,
		StartModes:   resp.StartModes,
	}, nil
}
