// Package starter selects and invokes the Starter component that turns the
// user's SOURCE argument into a repository reference. F1 has exactly one
// enabled Starter: the seed's fallback local-path Starter. The core reads only
// the typed fields of the response — nothing a Starter emits can influence
// core-governed Work state (FR-018).
package starter

import (
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

// Reference is the subset of a Starter response the core acts on (ADR-0016).
// All four fields are independent and optional; a reference with none of them
// set is not rejected here — internal/locator classifies that case as
// no-eligible-locator (FR-005, research R8).
type Reference struct {
	Path         string
	GitFetchURLs []string
	Name         string
	Query        string
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
	// Only the typed reference fields are consumed; every other field of the
	// response (start_modes, base_branch, meta, links) and any extra keys the
	// subprocess emitted are deliberately ignored (FR-018 trust boundary).
	return Reference{
		Path:         resp.Repository.Path,
		GitFetchURLs: resp.Repository.GitFetchURLs,
		Name:         resp.Repository.Name,
		Query:        resp.Repository.Query,
	}, nil
}
