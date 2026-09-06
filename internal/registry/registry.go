// Package registry reads and writes registry.json, the generated component
// index built from installed plugin manifests. It is never hand-edited. Writes
// are atomic and entry upserts are idempotent.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/gustaborges/work/internal/atomicfile"
)

// Component is one registered component. Identity is (Alias, Name).
type Component struct {
	Alias        string   `json:"alias"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Entrypoint   string   `json:"entrypoint"`
	Runtime      string   `json:"runtime,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`
	StarterLayer string   `json:"starter_layer,omitempty"`
	Accepts      []string `json:"accepts,omitempty"`
	DisplayName  string   `json:"display_name,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// EntrypointPath resolves the component's executable inside a plugins
// directory: <pluginsDir>/<alias>/source/<entrypoint>, with a ".exe" suffix on
// Windows. It mirrors the layout bootstrap writes.
func (c Component) EntrypointPath(pluginsDir string) string {
	p := filepath.Join(pluginsDir, c.Alias, "source", c.Entrypoint)
	if runtime.GOOS == "windows" {
		p += ".exe"
	}
	return p
}

// Convention is one registered branch convention. Identity is Name.
type Convention struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

// Registry is the whole registry.json document.
type Registry struct {
	Components  []Component  `json:"components"`
	Conventions []Convention `json:"conventions"`
}

// Roles / layers used by queries.
const (
	RoleStarter           = "starter"
	RoleRepositoryLocator = "repository-locator"
	LayerFallback         = "fallback"
)

// Load reads registry.json. A missing file yields an empty Registry.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Registry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry: reading %s: %w", path, err)
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("registry: %s is not valid JSON: %w", path, err)
	}
	return &r, nil
}

// Save writes r to path atomically.
func Save(path string, r *Registry) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshaling: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("registry: creating state dir: %w", err)
	}
	if err := atomicfile.WriteFile(path, data); err != nil {
		return fmt.Errorf("registry: writing %s: %w", path, err)
	}
	return nil
}

// UpsertComponent inserts or replaces the entry keyed by (Alias, Name).
func (r *Registry) UpsertComponent(c Component) {
	for i := range r.Components {
		if r.Components[i].Alias == c.Alias && r.Components[i].Name == c.Name {
			r.Components[i] = c
			return
		}
	}
	r.Components = append(r.Components, c)
}

// UpsertConvention inserts or replaces the entry keyed by Name.
func (r *Registry) UpsertConvention(c Convention) {
	for i := range r.Conventions {
		if r.Conventions[i].Name == c.Name {
			r.Conventions[i] = c
			return
		}
	}
	r.Conventions = append(r.Conventions, c)
}

// ByRole returns every component with the given role.
func (r *Registry) ByRole(role string) []Component {
	var out []Component
	for _, c := range r.Components {
		if c.Role == role {
			out = append(out, c)
		}
	}
	return out
}

// ListConventions returns a copy of the registered conventions.
func (r *Registry) ListConventions() []Convention {
	return slices.Clone(r.Conventions)
}

// ConventionByName returns the named convention.
func (r *Registry) ConventionByName(name string) (Convention, bool) {
	for _, c := range r.Conventions {
		if c.Name == name {
			return c, true
		}
	}
	return Convention{}, false
}

// StarterFallback returns the single fallback-layer starter, if one is
// registered.
func (r *Registry) StarterFallback() (Component, bool) {
	for _, c := range r.Components {
		if c.Role == RoleStarter && c.StarterLayer == LayerFallback {
			return c, true
		}
	}
	return Component{}, false
}

// HasComponent reports whether an entry keyed by (alias, name) exists.
func (r *Registry) HasComponent(alias, name string) bool {
	for _, c := range r.Components {
		if c.Alias == alias && c.Name == name {
			return true
		}
	}
	return false
}
