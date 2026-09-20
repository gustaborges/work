// Package plugininstall is the shared install pipeline `work plugin install`
// and internal/bootstrap's embedded-seed install both run through, so ADR-0002
// and ADR-0003's "same pipeline" claim is a checkable fact rather than two
// independently evolving copies (research R1). It stages a package's content
// in a temp directory under the destination plugins directory, then swaps it
// into place with a recoverable rename sequence: a process killed mid-install
// leaves either the previous package or the staged replacement fully intact,
// never a half-written destination.
package plugininstall

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// BackupSuffix names the previous package while a replacement is in
// progress. A restart restores it when the replacement did not reach its
// destination.
const BackupSuffix = ".old"

// StagePrefix names the temp directories StageThenSwap stages alias's content
// in before the atomic swap, scoped per alias so concurrent installs of
// different packages never collide. A process killed mid-install can leave
// one behind; SweepStaleStaging removes it on a later run.
func StagePrefix(alias string) string {
	return "." + alias + "-"
}

// MoveDestinationAside renames dest to backup, if dest exists. It fails if
// backup already exists — a sign an earlier swap never completed.
func MoveDestinationAside(dest, backup string) error {
	if _, err := os.Lstat(dest); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := os.Lstat(backup); err == nil {
		return errors.New("previous package backup already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(dest, backup)
}

// RestoreDestination renames backup back to dest, if backup exists.
func RestoreDestination(backup, dest string) error {
	if _, err := os.Lstat(backup); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Rename(backup, dest)
}

// RecoverInterruptedSwap restores the previous package at pluginsDir/alias
// when a process died after moving it aside but before the staged
// replacement reached its destination. A leftover backup beside a valid
// destination is safe to discard.
func RecoverInterruptedSwap(pluginsDir, alias string) error {
	dest := filepath.Join(pluginsDir, alias)
	backup := dest + BackupSuffix

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

// SweepStaleStaging removes staging directories orphaned by a killed install
// of alias. Best-effort: anything it cannot remove is retried on the next
// run.
func SweepStaleStaging(pluginsDir, alias string) {
	prefix := StagePrefix(alias)
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			_ = os.RemoveAll(filepath.Join(pluginsDir, e.Name()))
		}
	}
}

// StageThenSwap stages alias's content under pluginsDir by calling
// writeContent with a not-yet-existing staging directory to populate, then
// atomically swaps the result into pluginsDir/<alias>. Any interruption left
// by a previous install of the same alias is recovered and swept first, so a
// process killed mid-install never accumulates partial state (SC-005,
// SC-007). On failure the previous destination (if any) is left intact.
func StageThenSwap(pluginsDir, alias string, writeContent func(stagingDir string) error) error {
	if err := RecoverInterruptedSwap(pluginsDir, alias); err != nil {
		return err
	}
	SweepStaleStaging(pluginsDir, alias)

	staging, err := os.MkdirTemp(pluginsDir, StagePrefix(alias)+"*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := writeContent(staging); err != nil {
		return err
	}

	dest := filepath.Join(pluginsDir, alias)
	backup := dest + BackupSuffix
	if err := MoveDestinationAside(dest, backup); err != nil {
		return err
	}
	if err := os.Rename(staging, dest); err != nil {
		if restoreErr := RestoreDestination(backup, dest); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	return os.RemoveAll(backup)
}
