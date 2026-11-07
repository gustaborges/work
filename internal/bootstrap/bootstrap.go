// Package bootstrap performs first-use setup of the included reference package
// through the shared administrative journal. Lifecycle history preserves
// explicit policy decisions and deliberate removal: setup runs once, never
// repairs or replaces an installed copy because the binary changed, and never
// resurrects a removed default.
package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workhome"
)

// Provider is the identity the reference package installs under.
var Provider = adminstate.ReferenceProvider

// LocatorPolicyEntry is the seed Locator's fully qualified identifier in
// repository_resolution.locators.
var LocatorPolicyEntry = pluginid.Ref{Provider: Provider, Name: "filesystem-repository-locator"}.String()

// installCheckpoint, when non-nil, is invoked at each named phase of setup so
// tests can simulate an interruption. It is always nil in production.
var installCheckpoint func(phase string) error

func checkpoint(phase string) error {
	if installCheckpoint == nil {
		return nil
	}
	return installCheckpoint(phase)
}

// EnsureSeed performs first-use setup when it has never completed. It is safe
// to call on every command: once the lifecycle records initialization it does
// nothing but take and release the common administration lock.
func EnsureSeed(h workhome.Home) error {
	return EnsureSeedContext(context.Background(), h)
}

// EnsureSeedContext is EnsureSeed with cancellation.
func EnsureSeedContext(ctx context.Context, h workhome.Home) error {
	err := adminstate.WithWriteOptions(ctx, h, "work (automatic setup)", func(s *adminstate.Session) error { return EnsureSeedLocked(h, s) }, adminstate.WriteOptions{Checkpoint: func(point string) error {
		switch point {
		case "move-backup:" + Provider.String():
			return checkpoint("dest-backed-up")
		case "move-publish:" + Provider.String():
			return checkpoint("renamed")
		case "before-commit":
			return checkpoint("registered")
		}
		return nil
	}})
	var typed *diag.Error
	if err != nil && !errors.As(err, &typed) {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot prepare the included reference package")
	}
	return err
}

// EnsureSeedLocked is the non-recursive setup path. User input and source
// retrieval never call the common-lock public entry point while holding it.
func EnsureSeedLocked(h workhome.Home, s *adminstate.Session) error {
	if s.Lifecycle.Initialized || s.Lifecycle.Removed {
		return nil
	}
	snap, err := marketplace.BuiltinSnapshot()
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "no included reference package for this platform")
	}
	if err = installLocked(h, s, snap); err != nil {
		return err
	}
	if !s.Lifecycle.PolicyHandled {
		if !slices.Contains(s.Config.RepositoryResolution.Locators, LocatorPolicyEntry) {
			s.Config.RepositoryResolution.Locators = append(s.Config.RepositoryResolution.Locators, LocatorPolicyEntry)
		}
		s.Lifecycle.PolicyHandled = true
	}
	s.Lifecycle.Initialized = true
	s.Persist("lifecycle")
	return nil
}

// installLocked publishes the builtin reference entry as an independent managed
// copy and registers it as enabled. The copy comes from the code-owned offer
// through the same tree and registration steps every marketplace entry uses.
func installLocked(h workhome.Home, s *adminstate.Session, snap *marketplace.Snapshot) error {
	entry, ok := snap.Offer.Entry(Provider.Plugin)
	if !ok {
		return errors.New("the included offer does not contain the reference package")
	}
	tree, err := marketplace.BuiltinView().PackageTree(entry.Path)
	if err != nil {
		return err
	}
	raw, err := tree.ReadFile("plugin.json")
	if err != nil {
		return err
	}
	manifest, err := plugin.Parse(raw)
	if err != nil {
		return err
	}
	spec := plugininstall.Spec{
		Provider: Provider, Manifest: manifest, Enabled: true, ContentDigest: entry.PackageDigest,
		Provenance: registry.Provenance{
			RegistrationID: pluginid.BuiltinRegistrationID, Kind: pluginid.KindBuiltin, Source: pluginid.Builtin,
			MarketplaceName: Provider.Marketplace, PluginName: Provider.Plugin, Path: entry.Path,
			Revision: snap.Revision, CatalogDigest: snap.Offer.CatalogDigest,
		},
	}
	stage, err := s.NewStage()
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
	if err = tree.CopyTo(filepath.Join(stage, "source")); err != nil {
		return err
	}
	if err = plugininstall.WriteMeta(stage, spec); err != nil {
		return err
	}
	if err = checkpoint("staged-complete"); err != nil {
		return err
	}
	if err = s.Replace(Provider, stage); err != nil {
		return err
	}
	transferred = true
	_, err = plugininstall.Register(s.Registry, spec)
	return err
}
