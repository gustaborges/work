package adminstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/workhome"
)

// Preflight is called only after mutating grammar and required flags have
// been validated. It never performs semantic lookup for the new operation.
func Preflight(ctx context.Context, h workhome.Home) ([]string, error) {
	release, err := acquire(ctx, h, lockfile.DefaultTimeout)
	if err != nil {
		return nil, err
	}
	defer release()
	recovered, err := recoverLocked(h)
	reportRecovery(ctx, recovered)
	return recovered, err
}
func loadJournal(h workhome.Home, id string) (*Journal, error) {
	dir, err := h.TransactionDir(id)
	if err != nil {
		return nil, recoveryError(err)
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, recoveryError(errors.New("invalid transaction directory"))
	}
	path := filepath.Join(dir, "journal.json")
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, recoveryError(errors.New("invalid transaction journal"))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, recoveryError(err)
	}
	var j Journal
	if json.Unmarshal(b, &j) != nil || j.Schema != 1 || j.ID != id || (j.Phase != "prepared" && j.Phase != "committed") || j.Operation == "" || len(j.Files)+len(j.Moves) == 0 {
		return nil, recoveryError(errors.New("malformed transaction journal"))
	}
	names := map[string]bool{}
	for _, f := range j.Files {
		if statePath(h, f.Name) == "" || names[f.Name] {
			return nil, recoveryError(errors.New("unsafe journal image"))
		}
		names[f.Name] = true
		for _, i := range []Image{f.Before, f.After} {
			if (i.Exists && (digest(i.Bytes) != i.Digest || !json.Valid(i.Bytes))) || (!i.Exists && (len(i.Bytes) != 0 || i.Digest != "")) {
				return nil, recoveryError(errors.New("corrupt journal image"))
			}
		}
	}
	names = map[string]bool{}
	for _, m := range j.Moves {
		validStage := filepath.Base(m.Stage) == m.Stage && strings.HasPrefix(m.Stage, ".admin-stage-") && m.AfterDigest != ""
		if m.Remove {
			validStage = m.Stage == "" && m.AfterDigest == ""
		}
		if plugin.ValidateAlias(m.Alias) != nil || names[m.Alias] || !validStage || (m.BeforeExists != (m.BeforeDigest != "")) {
			return nil, recoveryError(errors.New("unsafe package journal"))
		}
		names[m.Alias] = true
	}
	return &j, nil
}
func recoverLocked(h workhome.Home) ([]string, error) {
	c, err := loadControl(h)
	if err != nil {
		return nil, err
	}
	var recovered []string
	if c.Status == "pending" {
		j, err := loadJournal(h, *c.TransactionID)
		if err != nil {
			return nil, err
		}
		if j.Phase == "prepared" {
			err = restoreBefore(h, j)
		} else {
			err = restoreAfter(h, j)
		}
		if err != nil {
			return nil, recoveryError(err)
		}
		if err = saveJSON(h.AdministrationFile(), Control{Schema: 1, Epoch: c.Epoch + 1, Status: "stable"}); err != nil {
			return nil, recoveryError(err)
		}
		recovered = append(recovered, j.Operation)
		if err = cleanup(h, j); err != nil {
			return recovered, committedError(err)
		}
	}
	// Unreferenced evidence is cleanup-only. It can never restore live state.
	entries, err := os.ReadDir(h.AdministrationDir())
	if errors.Is(err, os.ErrNotExist) {
		return recovered, sweepOrphanStages(h)
	}
	if err != nil {
		return recovered, recoveryError(err)
	}
	for _, e := range entries {
		// A process can die before its prepared journal is published. With no
		// pending reference, an owned directory without a journal is abandoned.
		dir, pathErr := h.TransactionDir(e.Name())
		if pathErr != nil || !e.IsDir() {
			return recovered, recoveryError(errors.New("unsafe recovery directory"))
		}
		info, statErr := os.Lstat(dir)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return recovered, recoveryError(errors.New("unsafe recovery directory"))
		}
		if _, statErr = os.Lstat(filepath.Join(dir, "journal.json")); errors.Is(statErr, os.ErrNotExist) {
			if err := os.RemoveAll(dir); err != nil {
				return recovered, recoveryError(err)
			}
			continue
		}
		j, err := loadJournal(h, e.Name())
		if err != nil {
			return recovered, err
		}
		if err = cleanup(h, j); err != nil {
			return recovered, committedError(err)
		}
		recovered = append(recovered, j.Operation)
	}
	return recovered, sweepOrphanStages(h)
}
func exists(path string) bool { _, err := os.Lstat(path); return err == nil }
func verifyTree(path, want string) error {
	got, err := treeDigest(path)
	if err != nil {
		return err
	}
	if got != want {
		return errors.New("package recovery evidence does not match journal")
	}
	return nil
}
func restoreBefore(h workhome.Home, j *Journal) error {
	ctx, cancel := context.WithTimeout(context.Background(), lockfile.DefaultTimeout)
	defer cancel()
	return restoreBeforeContext(ctx, h, j)
}
func restoreBeforeContext(ctx context.Context, h workhome.Home, j *Journal) error {
	for i := len(j.Moves) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		m := j.Moves[i]
		dest := filepath.Join(h.PluginsDir(), m.Alias)
		backup := backupPath(h, j.ID, m.Alias)
		if m.BeforeExists {
			if exists(backup) {
				if err := verifyTree(backup, m.BeforeDigest); err != nil {
					return err
				}
				if exists(dest) {
					if err := verifyTree(dest, m.AfterDigest); err != nil {
						return err
					}
					if err := os.RemoveAll(dest); err != nil {
						return err
					}
				}
				if err := os.Rename(backup, dest); err != nil {
					return err
				}
			} else if err := verifyTree(dest, m.BeforeDigest); err != nil {
				return err
			}
		} else if exists(dest) {
			if err := verifyTree(dest, m.AfterDigest); err != nil {
				return err
			}
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
		}
	}
	for _, f := range j.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeImage(statePath(h, f.Name), f.Before); err != nil {
			return err
		}
	}
	return nil
}
func restoreAfter(h workhome.Home, j *Journal) error {
	ctx, cancel := context.WithTimeout(context.Background(), lockfile.DefaultTimeout)
	defer cancel()
	return restoreAfterContext(ctx, h, j)
}
func restoreAfterContext(ctx context.Context, h workhome.Home, j *Journal) error {
	for _, m := range j.Moves {
		if err := ctx.Err(); err != nil {
			return err
		}
		dest := filepath.Join(h.PluginsDir(), m.Alias)
		if m.Remove {
			if _, err := os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
				return errors.New("committed removal has unexpected package evidence")
			}
			continue
		}
		if err := verifyTree(dest, m.AfterDigest); err != nil {
			return fmt.Errorf("committed package evidence missing: %w", err)
		}
	}
	for _, f := range j.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeImage(statePath(h, f.Name), f.After); err != nil {
			return err
		}
	}
	return nil
}
func cleanup(h workhome.Home, j *Journal) error {
	for _, m := range j.Moves {
		paths := []string{backupPath(h, j.ID, m.Alias)}
		if !m.Remove {
			paths = append(paths, filepath.Join(h.PluginsDir(), m.Stage))
		}
		for _, p := range paths {
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		}
	}
	dir, err := h.TransactionDir(j.ID)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

type recoveryObserverKey struct{}

func WithRecoveryObserver(ctx context.Context, observe func(string)) context.Context {
	return context.WithValue(ctx, recoveryObserverKey{}, observe)
}
func reportRecovery(ctx context.Context, operations []string) {
	if observe, ok := ctx.Value(recoveryObserverKey{}).(func(string)); ok {
		for _, op := range operations {
			observe(op)
		}
	}
}

func sweepOrphanStages(h workhome.Home) error {
	entries, err := os.ReadDir(h.PluginsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return recoveryError(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".admin-stage-") {
			if err := os.RemoveAll(filepath.Join(h.PluginsDir(), e.Name())); err != nil {
				return recoveryError(err)
			}
		}
	}
	return nil
}

// recoveryCommand maps a journal operation label to the command the user can
// rerun. Labels that are not direct user commands (automatic setup, start)
// fall back to generic wording rather than naming a command nobody typed.
func recoveryCommand(operation string) string {
	words := strings.Fields(operation)
	if len(words) >= 3 && words[0] == "work" {
		switch words[1] + " " + words[2] {
		case "plugin install", "plugin enable", "plugin disable", "plugin update", "plugin uninstall", "repository policy", "repository root", "convention set":
			return "work " + words[1] + " " + words[2]
		}
	}
	return "the interrupted administrative command"
}

// Only known command words enter the public recovery hint. Journal contents,
// source credentials and filesystem evidence remain private diagnostics.
func pendingReadError(h workhome.Home, c Control) error {
	j, err := loadJournal(h, *c.TransactionID)
	if err != nil {
		return err
	}
	command := recoveryCommand(j.Operation)
	return diag.New(diag.AdministrationRecoveryRequired, "administration needs recovery; rerun "+command+" with its original arguments and required flags")
}
