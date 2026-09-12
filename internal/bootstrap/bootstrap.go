// Package bootstrap installs the embedded reference package into the Work home
// on first use, through the same pipeline any plugin would take. It is
// idempotent and self-repairing: identity is the logical component name plus a
// content digest, never a "seeded" flag, so any number of concurrent or
// interrupted runs converge to exactly one usable record of each component.
package bootstrap

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/seed"
)

// Alias is the local package alias the seed is installed under.
const Alias = "work-reference"

// LocatorPolicyEntry is the seed Locator's identifier in
// repository_resolution.locators.
const LocatorPolicyEntry = Alias + "/filesystem-repository-locator"

const (
	componentStarter = "local-path-starter"
	componentLocator = "filesystem-repository-locator"
	conventionName   = "freeform"
	installOrigin    = "embedded-seed"
)

// stagePrefix names the temp directories install() stages the package in
// before the atomic swap. A process killed mid-install can leave one behind;
// install() sweeps stale ones on its next run.
const stagePrefix = ".work-reference-"

// backupSuffix names the previous package while a replacement is in progress.
// A restart restores it when the replacement did not reach its destination.
const backupSuffix = ".old"

// installCheckpoint, when non-nil, is invoked at each named phase of install()
// so tests can simulate an interruption. It is always nil in production.
var installCheckpoint func(phase string) error

func checkpoint(phase string) error {
	if installCheckpoint == nil {
		return nil
	}
	return installCheckpoint(phase)
}

type installMeta struct {
	Origin        string `json:"origin"`
	ContentDigest string `json:"content_digest"`
	InstalledAt   string `json:"installed_at"`
	// PolicySeeded records that the one-time "add the seed Locator to the
	// resolution policy" action has completed at least once. It is an
	// installation-history fact, not a mirror of the live policy: once true
	// it stays true across repairs and digest updates even if the user later
	// runs `work repository policy remove` (FR-027, ADR-0015 — reinstalling
	// never edits the policy). It only distinguishes that history from a
	// genuinely fresh or interrupted-before-seeding install, which must still
	// self-heal.
	PolicySeeded bool `json:"policy_seeded,omitempty"`
}

// EnsureSeed makes the reference package present and registered. It is safe to
// call on every command; when the install is already current it does nothing
// but take and release the bootstrap lock.
func EnsureSeed(h workhome.Home) error {
	if err := h.EnsureLayout(); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot create the Work home layout")
	}

	release, err := lockfile.Acquire(h.LockPath("bootstrap"))
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot acquire the bootstrap lock")
	}
	defer release()

	digest, err := seed.ContentDigest()
	if err != nil {
		return diag.Wrapf(diag.BootstrapFailed, err,
			"no embedded reference package for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	reg, err := registry.Load(h.RegistryFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}

	if err := recoverInterruptedSwap(h); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot recover the previous reference package")
	}

	metaPath := filepath.Join(h.PluginsDir(), Alias, ".install-meta.json")
	oldMeta, _ := readInstallMeta(metaPath) // zero value (PolicySeeded: false) if unreadable

	isCurrent, err := current(h, reg, digest, oldMeta)
	if err != nil {
		return err // already a diag error
	}
	if isCurrent {
		return nil
	}
	return install(h, digest, !oldMeta.PolicySeeded)
}

// current reports whether every seed record is registered, the installed
// digest matches the embedded one, the one-time policy-seed action has
// completed, and the plugin directory is intact. Deliberately not part of
// this check: whether the seed Locator is still *in* the live policy — that
// is user-editable state past the first install (`work repository policy
// remove`), not an installation-integrity signal (FR-027, ADR-0015).
func current(h workhome.Home, reg *registry.Registry, digest string, meta installMeta) (bool, error) {
	if !reg.HasComponent(Alias, componentStarter) ||
		!reg.HasComponent(Alias, componentLocator) {
		return false, nil
	}
	if _, ok := reg.ConventionByName(conventionName); !ok {
		return false, nil
	}
	if meta.ContentDigest != digest || !meta.PolicySeeded {
		return false, nil
	}

	src := filepath.Join(h.PluginsDir(), Alias, "source")
	for _, name := range binaryNames() {
		if _, err := os.Stat(filepath.Join(src, name)); err != nil {
			return false, nil
		}
	}
	return true, nil
}

// install stages and swaps in the embedded reference package and registers
// its components. seedPolicy is true unless the policy-seed action already
// completed in a prior install of this package (research: install-meta
// PolicySeeded) — only then does it also append the seed Locator to the
// resolution policy; either way the completed fact is (re)recorded in the
// swapped-in install-meta so a later repair does not re-seed it.
func install(h workhome.Home, digest string, seedPolicy bool) error {
	manifestBytes := seed.ManifestJSON()
	manifest, err := plugin.Parse(manifestBytes)
	if err != nil {
		return diag.Wrapf(diag.BootstrapFailed, err, "the embedded plugin.json is invalid")
	}

	starterBin, locatorBin, err := seed.HostAssets()
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot read the embedded reference binaries")
	}

	// Sweep any staging dir a previously-killed install left behind, so an
	// interrupted run never accumulates partial directories (SC-007, FR-005).
	sweepStaleStaging(h)

	// Stage the whole package in a temp dir, then replace the destination through
	// a recoverable rename sequence.
	staging, err := os.MkdirTemp(h.PluginsDir(), stagePrefix+"*")
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot stage the reference package")
	}
	defer os.RemoveAll(staging)
	if err := checkpoint("staged"); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "interrupted while staging the reference package")
	}

	srcDir := filepath.Join(staging, "source")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot stage the reference package")
	}
	names := binaryNames()
	for i, blob := range [][]byte{starterBin, locatorBin} {
		if err := os.WriteFile(filepath.Join(srcDir, names[i]), blob, 0o755); err != nil {
			return diag.Wrap(diag.BootstrapFailed, err, "cannot write a reference binary")
		}
	}
	if err := os.WriteFile(filepath.Join(staging, "plugin.json"), manifestBytes, 0o644); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot write plugin.json")
	}
	meta := installMeta{Origin: installOrigin, ContentDigest: digest, InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(staging, ".install-meta.json"), metaBytes, 0o644); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot write install metadata")
	}

	if err := checkpoint("staged-complete"); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "interrupted before installing the reference package")
	}

	dest := filepath.Join(h.PluginsDir(), Alias)
	backup := dest + backupSuffix
	if err := moveDestinationAside(dest, backup); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot prepare the existing reference package for replacement")
	}
	if err := checkpoint("dest-backed-up"); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "interrupted while replacing the reference package")
	}
	if err := os.Rename(staging, dest); err != nil {
		if restoreErr := restoreDestination(backup, dest); restoreErr != nil {
			return diag.Wrapf(diag.BootstrapFailed, err,
				"cannot install the reference package (and cannot restore the previous package: %v)", restoreErr)
		}
		return diag.Wrap(diag.BootstrapFailed, err, "cannot install the reference package")
	}
	if err := os.RemoveAll(backup); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot remove the replaced reference package")
	}
	if err := checkpoint("renamed"); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "interrupted before registering the reference package")
	}

	if err := registerComponents(h, manifest); err != nil {
		return err
	}
	if err := checkpoint("registered"); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "interrupted before updating the resolution policy")
	}
	if seedPolicy {
		if err := addLocatorToPolicy(h); err != nil {
			return err
		}
	}
	// Record the policy-seed action as done, whether it just ran or had
	// already completed in a prior install of this package — the meta file
	// this install just swapped in otherwise reads PolicySeeded: false, which
	// would look like an interrupted install and re-trigger addLocatorToPolicy
	// on the very next command, undoing an explicit `policy remove`.
	if err := setPolicySeeded(filepath.Join(dest, ".install-meta.json")); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot record the resolution policy as seeded")
	}
	return nil
}

// setPolicySeeded marks the install-meta file at path as PolicySeeded: true.
func setPolicySeeded(path string) error {
	meta, err := readInstallMeta(path)
	if err != nil {
		return err
	}
	meta.PolicySeeded = true
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// recoverInterruptedSwap restores the previous package when a process died
// after moving it aside but before the staged replacement reached dest. A
// leftover backup beside a valid destination is safe to discard.
func recoverInterruptedSwap(h workhome.Home) error {
	dest := filepath.Join(h.PluginsDir(), Alias)
	backup := dest + backupSuffix

	_, destErr := os.Lstat(dest)
	_, backupErr := os.Lstat(backup)
	switch {
	case errors.Is(destErr, os.ErrNotExist) && backupErr == nil:
		return os.Rename(backup, dest)
	case destErr == nil && backupErr == nil:
		return os.RemoveAll(backup)
	case destErr != nil && !errors.Is(destErr, os.ErrNotExist):
		return destErr
	case backupErr != nil && !errors.Is(backupErr, os.ErrNotExist):
		return backupErr
	}
	return nil
}

func moveDestinationAside(dest, backup string) error {
	if _, err := os.Lstat(dest); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := os.Lstat(backup); err == nil {
		return errors.New("previous-package backup already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(dest, backup)
}

func restoreDestination(backup, dest string) error {
	if _, err := os.Lstat(backup); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Rename(backup, dest)
}

// sweepStaleStaging removes staging directories orphaned by a killed install.
// A best-effort cleanup: anything it cannot remove is retried on the next run.
func sweepStaleStaging(h workhome.Home) {
	entries, err := os.ReadDir(h.PluginsDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), stagePrefix) {
			_ = os.RemoveAll(filepath.Join(h.PluginsDir(), e.Name()))
		}
	}
}

func registerComponents(h workhome.Home, manifest *plugin.Manifest) error {
	reg, err := registry.Load(h.RegistryFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}
	for _, c := range manifest.Components {
		entry := registry.Component{
			Alias:       Alias,
			Name:        c.Name,
			Role:        c.Role,
			Entrypoint:  c.Entrypoint,
			Runtime:     c.Runtime,
			Pattern:     c.Pattern,
			Accepts:     c.Accepts,
			DisplayName: c.DisplayName,
			Description: c.Description,
		}
		if c.IsFallbackStarter() {
			entry.StarterLayer = registry.LayerFallback
		}
		reg.UpsertComponent(entry)
	}
	for _, cv := range manifest.Conventions {
		reg.UpsertConvention(registry.Convention{Name: cv.Name, Prefixes: cv.Prefixes})
	}
	if err := registry.Save(h.RegistryFile(), reg); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot write the component registry")
	}
	return nil
}

func addLocatorToPolicy(h workhome.Home) error {
	cfg, err := config.Load(h.ConfigFile())
	if err != nil {
		return err // already a diag error
	}
	if slices.Contains(cfg.RepositoryResolution.Locators, LocatorPolicyEntry) {
		return nil
	}
	cfg.RepositoryResolution.Locators = append(cfg.RepositoryResolution.Locators, LocatorPolicyEntry)
	if err := config.Save(h.ConfigFile(), cfg); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot update the repository resolution policy")
	}
	return nil
}

func binaryNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"starter.exe", "locator.exe"}
	}
	return []string{"starter", "locator"}
}

func readInstallMeta(path string) (installMeta, error) {
	var m installMeta
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, nil
}
