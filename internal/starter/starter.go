// Package starter selects and invokes the Starter component that turns the
// user's SOURCE argument into a repository reference. F1 has exactly one
// enabled Starter: the seed's fallback local-path Starter. The core reads only
// the typed fields of the response — nothing a Starter emits can influence
// core-governed Work state (FR-018).
package starter

import (
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

// Reference is the subset of a Starter response the core acts on. Only path is
// populated by the F1 seed Starter.
type Reference struct {
	Path string
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
	if strings.TrimSpace(resp.Repository.Path) == "" {
		return Reference{}, diag.New(diag.UnusableRepo,
			"the Starter returned no repository path for the given source")
	}
	// Only repository.path is consumed in F1; every other field of the response
	// (and any extra keys the subprocess emitted) is deliberately ignored.
	return Reference{Path: resp.Repository.Path}, nil
}
