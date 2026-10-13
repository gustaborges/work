// Package bootstrap installs and repairs the offline reference package through
// the shared administrative journal. Lifecycle history preserves explicit
// policy edits and removal; the registry alone owns package enablement.
package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/seed"
)

// Alias is the local package alias the seed is installed under.
const Alias = plugininstall.ReferenceAlias

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
// install() sweeps stale ones on its next run. Delegated to
// internal/plugininstall so the staging/swap sequence is literally the same
// code `work plugin install` uses (ADR-0003 "same pipeline", research R1).
var stagePrefix = plugininstall.StagePrefix(Alias)

// backupSuffix names the previous package while a replacement is in progress.
// A restart restores it when the replacement did not reach its destination.
const backupSuffix = plugininstall.BackupSuffix

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
// but take and release the common administration lock.
func EnsureSeed(h workhome.Home) error {
	return EnsureSeedContext(context.Background(), h)
}
func EnsureSeedContext(ctx context.Context, h workhome.Home) error {
	err := adminstate.WithWriteOptions(ctx, h, "work plugin enable work-reference", func(s *adminstate.Session) error { return EnsureSeedLocked(h, s) }, adminstate.WriteOptions{Checkpoint: func(point string) error {
		switch point {
		case "move-backup:" + Alias:
			return checkpoint("dest-backed-up")
		case "move-publish:" + Alias:
			return checkpoint("renamed")
		case "before-commit":
			return checkpoint("registered")
		}
		return nil
	}})
	var typed *diag.Error
	if err != nil && !errors.As(err, &typed) {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot prepare the embedded reference package")
	}
	return err
}

// EnsureSeedLocked is the non-recursive bootstrap path. User input and source
// retrieval never call the common-lock public entry point while holding it.
func EnsureSeedLocked(h workhome.Home, s *adminstate.Session) error {
	if s.Lifecycle.Removed {
		return nil
	}
	digest, err := seed.ContentDigest()
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "no embedded reference package for this platform")
	}
	oldMeta, _ := readInstallMeta(filepath.Join(h.PluginsDir(), Alias, ".install-meta.json"))
	isCurrent, err := current(h, s.Registry, digest, oldMeta)
	if err != nil {
		return err
	}
	_, lifeErr := os.Stat(h.ReferencePackageFile())
	raw, err := registry.Load(h.RegistryFile())
	if err != nil {
		return err
	}
	_, owned := raw.PackageByAlias(Alias)
	if isCurrent {
		if !owned {
			s.Persist("registry")
		}
		if os.IsNotExist(lifeErr) {
			s.Lifecycle.Initialized = true
			s.Lifecycle.PolicySeeded = true
			s.Persist("lifecycle")
		}
		return nil
	}
	// A pre-existing config is explicit user state, including an empty policy.
	_, configErr := os.Stat(h.ConfigFile())
	genuinelyFresh := !s.Lifecycle.Initialized && os.IsNotExist(configErr) && len(s.Registry.Components) == 0 && !existsSeed(h)
	return installLocked(h, s, digest, genuinelyFresh)
}

// InstallEmbeddedLocked is explicit offline installation, including after a
// deliberate removal. It marks initialization handled without seeding policy.
// Existing registry enablement survives a reinstall.
func InstallEmbeddedLocked(h workhome.Home, s *adminstate.Session) (plugininstall.Result, error) {
	if fallback, ok := s.Registry.StarterFallback(); ok && fallback.Alias != Alias {
		return plugininstall.Result{}, diag.Newf(diag.PluginFallbackConflict, "a fallback Starter is already registered by %q", fallback.Alias)
	}
	digest, err := seed.ContentDigest()
	if err != nil {
		return plugininstall.Result{}, diag.Wrap(diag.BootstrapFailed, err, "no embedded reference package for this platform")
	}
	if err = installLocked(h, s, digest, false); err != nil {
		return plugininstall.Result{}, err
	}
	m, err := plugin.Parse(seed.ManifestJSON())
	if err != nil {
		return plugininstall.Result{}, err
	}
	p, _ := s.Registry.PackageByAlias(Alias)
	return plugininstall.Result{Package: p, Components: m.Components}, nil
}
func existsSeed(h workhome.Home) bool {
	_, err := os.Lstat(filepath.Join(h.PluginsDir(), Alias))
	return err == nil
}
func current(h workhome.Home, reg *registry.Registry, digest string, meta installMeta) (bool, error) {
	if !reg.HasComponent(Alias, componentStarter) || !reg.HasComponent(Alias, componentLocator) {
		return false, nil
	}
	if _, ok := reg.ConventionByName(conventionName); !ok {
		return false, nil
	}
	if meta.ContentDigest != digest {
		return false, nil
	}
	starter, locator, err := seed.HostAssets()
	if err != nil {
		return false, err
	}
	for i, name := range binaryNames() {
		path := filepath.Join(h.PluginsDir(), Alias, "source", name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false, nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return false, nil
		}
		expected := starter
		if i == 1 {
			expected = locator
		}
		if sha256.Sum256(b) != sha256.Sum256(expected) {
			return false, nil
		}
	}
	path := filepath.Join(h.PluginsDir(), Alias, "plugin.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(b) != sha256.Sum256(seed.ManifestJSON()) {
		return false, nil
	}

	return true, nil
}
func installLocked(h workhome.Home, s *adminstate.Session, digest string, seedPolicy bool) error {
	manifestBytes := seed.ManifestJSON()
	manifest, err := plugin.Parse(manifestBytes)
	if err != nil {
		return err
	}
	starterBin, locatorBin, err := seed.HostAssets()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(h.PluginsDir(), 0700); err != nil {
		return err
	}
	// Recover the old pre-journal swap format only while already holding the
	// shared lock, before staging new journal-owned content.
	if err = plugininstall.RecoverInterruptedSwap(h.PluginsDir(), Alias); err != nil {
		return err
	}
	plugininstall.SweepStaleStaging(h.PluginsDir(), Alias)
	stage, err := os.MkdirTemp(h.PluginsDir(), ".admin-stage-")
	if err != nil {
		return err
	}
	transferred := false
	defer func() {
		if !transferred {
			_ = os.RemoveAll(stage)
		}
	}()
	if err = checkpoint("staged"); err != nil {
		return err
	}
	src := filepath.Join(stage, "source")
	if err = os.MkdirAll(src, 0755); err != nil {
		return err
	}
	for i, blob := range [][]byte{starterBin, locatorBin} {
		if err = os.WriteFile(filepath.Join(src, binaryNames()[i]), blob, 0755); err != nil {
			return err
		}
	}
	if err = os.WriteFile(filepath.Join(stage, "plugin.json"), manifestBytes, 0644); err != nil {
		return err
	}
	meta := installMeta{Origin: installOrigin, ContentDigest: digest, InstalledAt: time.Now().UTC().Format(time.RFC3339), PolicySeeded: true}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, ".install-meta.json"), data, 0644); err != nil {
		return err
	}
	if err = checkpoint("staged-complete"); err != nil {
		return err
	}

	if err = s.Replace(Alias, stage); err != nil {
		return err
	}
	transferred = true
	old, _ := s.Registry.PackageByAlias(Alias)
	names := []string{}
	for _, cv := range manifest.Conventions {
		names = append(names, cv.Name)
	}
	s.Registry.RemoveAlias(Alias)
	s.Registry.UpsertPackage(registry.Package{Alias: Alias, PluginName: manifest.Name, Origin: registry.OriginEmbedded, Reference: "embedded", Version: manifest.Version, ContentDigest: digest, Conventions: names, Enabled: old.Enabled})
	for _, c := range manifest.Components {
		s.Registry.UpsertComponent(plugininstall.ComponentEntry(Alias, c))
	}
	for _, cv := range manifest.Conventions {
		s.Registry.UpsertConvention(registry.Convention{Name: cv.Name, Prefixes: cv.Prefixes})
	}
	if seedPolicy && !slices.Contains(s.Config.RepositoryResolution.Locators, LocatorPolicyEntry) {
		s.Config.RepositoryResolution.Locators = append(s.Config.RepositoryResolution.Locators, LocatorPolicyEntry)
	}
	s.Lifecycle.Initialized = true
	s.Lifecycle.PolicySeeded = true
	s.Lifecycle.Removed = false
	s.Persist("lifecycle")
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
