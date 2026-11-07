// Package workhome resolves the location of the Work home directory (the tree
// rooted at ~/.work, or wherever WORK_HOME points) and creates its fixed
// subdirectory layout.
package workhome

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gustaborges/work/internal/pluginid"
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

// PluginsDir holds installed plugin packages under plugins/<marketplace>/<plugin>.
func (h Home) PluginsDir() string { return filepath.Join(h.root, "plugins") }

// StateDir holds generated state that is never hand-edited.
func (h Home) StateDir() string { return filepath.Join(h.root, "state") }

// RegistryFile is the path to the generated component registry.
func (h Home) RegistryFile() string { return filepath.Join(h.StateDir(), "registry.json") }

// BranchConventionsFile is the path to the generated, per-repository-identity
// branch convention memory (internal/repoconv, ADR-0011).
func (h Home) BranchConventionsFile() string {
	return filepath.Join(h.StateDir(), "branch_conventions.json")
}

// DBFile is the path to the SQLite projection database.
func (h Home) DBFile() string { return filepath.Join(h.StateDir(), "work.db") }

// LocksDir holds advisory lockfiles.
func (h Home) LocksDir() string { return filepath.Join(h.StateDir(), "locks") }

// LockPath returns the lockfile path for a given logical name. The ".lock"
// suffix is appended automatically.
func (h Home) LockPath(name string) string {
	return filepath.Join(h.LocksDir(), name+".lock")
}

// WorkLockPath is the advisory lockfile that serializes every mutation of one
// Work (resume, archive, and extension results) by its id. The name is
// sha256(id) in hex, scoped per Work rather than per repository and branch, so
// all writers must derive it here to actually exclude each other.
func (h Home) WorkLockPath(id string) string {
	sum := sha256.Sum256([]byte(id))
	return h.LockPath(hex.EncodeToString(sum[:]))
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

// AdministrationFile is the stable/pending epoch control; absent means epoch 0.
func (h Home) AdministrationFile() string { return filepath.Join(h.StateDir(), "administration.json") }

// LifecycleFile records initialization, policy handling and default removal
// outside any removable package directory.
func (h Home) LifecycleFile() string      { return filepath.Join(h.StateDir(), "lifecycle.json") }
func (h Home) AdministrationDir() string  { return filepath.Join(h.StateDir(), "administration") }
func (h Home) AdministrationLock() string { return h.LockPath("administration") }

var transactionIDGrammar = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)

func (h Home) TransactionDir(id string) (string, error) {
	if !transactionIDGrammar.MatchString(id) {
		return "", fmt.Errorf("invalid administrative transaction id")
	}
	return filepath.Join(h.AdministrationDir(), id), nil
}

// CatalogsFile holds the stored discovery offers of every registered source.
func (h Home) CatalogsFile() string { return filepath.Join(h.StateDir(), "marketplace-catalogs.json") }

// SnapshotsDir holds complete fixed captures, one directory per validated
// 64-hex digest payload. A capture is never an execution root.
func (h Home) SnapshotsDir() string { return filepath.Join(h.StateDir(), "marketplace-snapshots") }

// LeasesDir holds the per-plan lease records and their process-held locks. It
// is deliberately outside AdministrationDir, whose children are all treated as
// transaction directories by recovery.
func (h Home) LeasesDir() string { return filepath.Join(h.StateDir(), "administration-leases") }

// SnapshotDir resolves a capture directory from the bare hex digest payload.
// The algorithm prefix is never a path segment because ':' is not portable.
func (h Home) SnapshotDir(hexDigest string) (string, error) {
	if err := pluginid.ValidateHexDigest(hexDigest); err != nil {
		return "", err
	}
	return filepath.Join(h.SnapshotsDir(), hexDigest), nil
}

// PackageRoot is the owned wrapper directory of one installed provider:
// plugins/<marketplace>/<plugin>. Both segments are validated separately, so a
// display string can never select a path.
func (h Home) PackageRoot(p pluginid.Provider) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	return filepath.Join(h.PluginsDir(), p.Marketplace, p.Plugin), nil
}

// PackageSource is the managed copy that components execute from.
func (h Home) PackageSource(p pluginid.Provider) (string, error) {
	root, err := h.PackageRoot(p)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "source"), nil
}

// MarketplaceDir is the namespace directory that parents package roots.
func (h Home) MarketplaceDir(marketplace string) (string, error) {
	if err := pluginid.ValidateName(marketplace); err != nil {
		return "", err
	}
	return filepath.Join(h.PluginsDir(), marketplace), nil
}

// StagePrefix marks transaction-owned package stages directly under PluginsDir.
// No installed marketplace can begin with a dot, so stages never collide with
// a namespace directory.
const StagePrefix = ".admin-stage-"

// IsStageName reports whether base names a transaction-owned stage directory.
func IsStageName(base string) bool {
	return filepath.Base(base) == base && strings.HasPrefix(base, StagePrefix) && len(base) > len(StagePrefix)
}

// BackupPath is where a transaction parks a provider's previous root. It
// encodes the typed address (plugin@marketplace is unambiguous because '@' is
// not a legal name byte) rather than nesting under the live namespace.
func (h Home) BackupPath(transactionID string, p pluginid.Provider) (string, error) {
	if !transactionIDGrammar.MatchString(transactionID) {
		return "", fmt.Errorf("invalid administrative transaction id")
	}
	if err := p.Validate(); err != nil {
		return "", err
	}
	return filepath.Join(h.PluginsDir(), ".admin-"+transactionID+"-"+p.Plugin+"@"+p.Marketplace+"-before"), nil
}
