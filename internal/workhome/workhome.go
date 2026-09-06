// Package workhome resolves the location of the Work home directory (the tree
// rooted at ~/.work, or wherever WORK_HOME points) and creates its fixed
// subdirectory layout.
package workhome

import (
	"fmt"
	"os"
	"path/filepath"
)

// Home is a resolved Work home root. The zero value is not usable; obtain one
// from Resolve.
type Home struct {
	root string
}

// Resolve determines the Work home root: the absolute form of $WORK_HOME when
// set and non-empty, otherwise ~/.work.
func Resolve() (Home, error) {
	if v := os.Getenv("WORK_HOME"); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return Home{}, fmt.Errorf("resolving WORK_HOME %q: %w", v, err)
		}
		return Home{root: abs}, nil
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return Home{}, fmt.Errorf("locating home directory: %w", err)
	}
	return Home{root: filepath.Join(dir, ".work")}, nil
}

// At returns a Home rooted at an explicit path. Intended for tests.
func At(root string) Home { return Home{root: root} }

// Root is the absolute Work home directory.
func (h Home) Root() string { return h.root }

// ConfigDir holds human-editable configuration.
func (h Home) ConfigDir() string { return filepath.Join(h.root, "config") }

// ConfigFile is the path to work.json.
func (h Home) ConfigFile() string { return filepath.Join(h.ConfigDir(), "work.json") }

// PluginsDir holds installed plugin packages, including the embedded seed.
func (h Home) PluginsDir() string { return filepath.Join(h.root, "plugins") }

// StateDir holds generated state that is never hand-edited.
func (h Home) StateDir() string { return filepath.Join(h.root, "state") }

// RegistryFile is the path to the generated component registry.
func (h Home) RegistryFile() string { return filepath.Join(h.StateDir(), "registry.json") }

// DBFile is the path to the SQLite projection database.
func (h Home) DBFile() string { return filepath.Join(h.StateDir(), "work.db") }

// LocksDir holds advisory lockfiles.
func (h Home) LocksDir() string { return filepath.Join(h.StateDir(), "locks") }

// LockPath returns the lockfile path for a given logical name. The ".lock"
// suffix is appended automatically.
func (h Home) LockPath(name string) string {
	return filepath.Join(h.LocksDir(), name+".lock")
}

// EnsureLayout creates every directory in the fixed layout. It is idempotent.
func (h Home) EnsureLayout() error {
	for _, dir := range []string{h.ConfigDir(), h.PluginsDir(), h.StateDir(), h.LocksDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return nil
}
