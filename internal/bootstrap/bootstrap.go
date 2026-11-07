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
	"slices"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/plugininstall"
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
// copy and registers it as enabled. It resolves the code-owned offer and runs
// the same validation, copy and registration every marketplace entry goes
// through, so the default has no installation path of its own.
func installLocked(h workhome.Home, s *adminstate.Session, snap *marketplace.Snapshot) error {
	entry, err := snap.Resolve(pluginid.BuiltinRegistrationID, Provider.Plugin)
	if err != nil {
		return errors.New("the included offer does not contain the reference package")
	}
	defer entry.Close()
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
	prepared, err := plugininstall.Prepare(entry, stage)
	if err != nil {
		return err
	}
	if err = checkpoint("staged-complete"); err != nil {
		return err
	}
	if _, err = prepared.Publish(s, true); err != nil {
		return err
	}
	transferred = true
	return nil
}
