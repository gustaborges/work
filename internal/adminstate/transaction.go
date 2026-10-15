package adminstate

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/workhome"
)

type Image struct {
	Exists bool   `json:"exists"`
	Bytes  []byte `json:"bytes"`
	Digest string `json:"digest"`
}
type FileChange struct {
	Name   string `json:"name"`
	Before Image  `json:"before"`
	After  Image  `json:"after"`
}
type Move struct {
	Alias        string `json:"alias"`
	Remove       bool   `json:"remove,omitempty"`
	Stage        string `json:"stage"`
	BeforeExists bool   `json:"before_exists"`
	BeforeDigest string `json:"before_digest"`
	AfterDigest  string `json:"after_digest"`
}

// OwnedPackageDigest inspects only the owned alias root; symlinks are hashed
// as link text and never followed. Missing roots have an empty digest.
func OwnedPackageDigest(h workhome.Home, alias string) (string, error) {
	if plugin.ValidateAlias(alias) != nil {
		return "", diag.New(diag.Usage, "supply exact installed plugin aliases")
	}
	root := filepath.Join(h.PluginsDir(), alias)
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return treeDigest(root)
}

// Remove transfers only the owned root to the journal backup. Cleanup unlinks
// linked roots without traversing their external sources.
func (s *Session) Remove(alias string) error {
	for _, m := range s.moves {
		if m.Alias == alias {
			return errors.New("duplicate package move")
		}
	}
	digest, err := OwnedPackageDigest(s.home, alias)
	if err != nil {
		return err
	}
	s.moves = append(s.moves, Move{Alias: alias, Remove: true, BeforeExists: digest != "", BeforeDigest: digest})
	return nil
}

type Journal struct {
	Schema    int          `json:"schema"`
	ID        string       `json:"id"`
	Operation string       `json:"operation"`
	Phase     string       `json:"phase"`
	Files     []FileChange `json:"files"`
	Moves     []Move       `json:"moves"`
}

// Session is valid only inside WithWrite. Replace transfers ownership of a
// same-filesystem prepared package root to the session, including on failure.
type Session struct {
	*Snapshot
	home      workhome.Home
	moves     []Move
	persist   map[string]bool
	Recovered []string
}

func (s *Session) Replace(alias, stage string) error {
	if plugin.ValidateAlias(alias) != nil {
		return diag.New(diag.PluginInvalid, "invalid package alias")
	}
	for _, m := range s.moves {
		if m.Alias == alias {
			return errors.New("duplicate package replacement")
		}
	}
	if filepath.Dir(stage) != s.home.PluginsDir() || !strings.HasPrefix(filepath.Base(stage), ".admin-stage-") {
		return errors.New("stage is not transaction-owned")
	}
	info, err := os.Lstat(stage)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid package stage")
	}
	m := Move{Alias: alias, Stage: filepath.Base(stage)}
	dest := filepath.Join(s.home.PluginsDir(), alias)
	if _, err := os.Lstat(dest); err == nil {
		m.BeforeExists = true
		m.BeforeDigest, err = treeDigest(dest)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.AfterDigest, err = treeDigest(stage)
	if err != nil {
		return err
	}
	s.moves = append(s.moves, m)
	return nil
}
func acquire(ctx context.Context, h workhome.Home, timeout time.Duration) (func(), error) {
	if ctx.Err() != nil {
		return nil, cancelled(ctx)
	}
	release, err := lockfile.AcquireContext(ctx, h.AdministrationLock(), timeout)
	if ctx.Err() != nil {
		if release != nil {
			release()
		}
		return nil, cancelled(ctx)
	}
	if errors.Is(err, lockfile.ErrTimeout) {
		return nil, diag.Wrap(diag.AdministrationBusy, err, "administration is busy; retry when the running component or operation finishes")
	}
	return release, err
}

func WithWrite(ctx context.Context, h workhome.Home, operation string, fn func(*Session) error) error {
	return withWrite(ctx, h, operation, fn, nil)
}

// WriteOptions provides an injected service checkpoint for deterministic fault
// tests. Production does not select failures through environment variables.
type WriteOptions struct{ Checkpoint func(string) error }

func WithWriteOptions(ctx context.Context, h workhome.Home, operation string, fn func(*Session) error, opts WriteOptions) error {
	return withWrite(ctx, h, operation, fn, opts.Checkpoint)
}
func withWrite(ctx context.Context, h workhome.Home, operation string, fn func(*Session) error, checkpoint func(string) error) error {
	release, err := acquire(ctx, h, lockfile.DefaultTimeout)
	if err != nil {
		return err
	}
	defer release()
	recovered, err := recoverLocked(h)
	reportRecovery(ctx, recovered)
	if err != nil {
		return err
	}
	c, err := loadControl(h)
	if err != nil {
		return err
	}
	before, err := load(h)
	if err != nil {
		return err
	}
	// Compare typed state, not serialization of the original file. No-op keeps
	// original formatting and unknown fields byte-for-byte.
	models, err := modelBytes(before)
	if err != nil {
		return err
	}
	s := &Session{Snapshot: before, home: h, Recovered: recovered}
	defer func() {
		control, e := loadControl(h)
		if e != nil || control.Status == "pending" {
			return
		}
		for _, m := range s.moves {
			if !m.Remove {
				_ = os.RemoveAll(filepath.Join(h.PluginsDir(), m.Stage))
			}
		}
	}()
	if err = fn(s); err != nil {
		return err
	}
	after, err := modelBytes(s.Snapshot)
	if err != nil {
		return err
	}
	var files []FileChange
	for i, name := range []string{"config", "registry", "memory", "lifecycle"} {
		if bytes.Equal(models[i], after[i]) && !s.persist[name] {
			continue
		}
		path := statePath(h, name)
		old, err := readImage(path)
		if err != nil {
			return err
		}
		files = append(files, FileChange{Name: name, Before: old, After: newImage(after[i])})
	}
	if len(files) == 0 && len(s.moves) == 0 {
		return nil
	}
	if ctx.Err() != nil {
		return cancelled(ctx)
	}
	id, err := newID()
	if err != nil {
		return err
	}
	j := Journal{Schema: 1, ID: id, Operation: operation, Phase: "prepared", Files: files, Moves: s.moves}
	dir, _ := h.TransactionDir(id)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err = saveJSON(filepath.Join(dir, "journal.json"), &j); err != nil {
		return err
	}
	point := func(p string) error {
		if ctx.Err() != nil {
			return cancelled(ctx)
		}
		if checkpoint != nil {
			return checkpoint(p)
		}
		return nil
	}
	if err = point("prepared"); err != nil {
		_ = cleanup(h, &j)
		return err
	}
	pending := Control{Schema: 1, Epoch: c.Epoch + 1, Status: "pending", TransactionID: &id}
	if err = saveJSON(h.AdministrationFile(), pending); err != nil {
		_ = cleanup(h, &j)
		return err
	}
	rollback := func(cause error) error {
		if err := restoreBefore(h, &j); err != nil {
			return recoveryError(errors.Join(cause, err))
		}
		if err := saveJSON(h.AdministrationFile(), Control{Schema: 1, Epoch: pending.Epoch + 1, Status: "stable"}); err != nil {
			return recoveryError(errors.Join(cause, err))
		}
		if err := cleanup(h, &j); err != nil {
			return recoveryError(errors.Join(cause, err))
		}
		return cause
	}
	if err = point("pending"); err != nil {
		return rollback(err)
	}
	for _, m := range j.Moves {
		dest := filepath.Join(h.PluginsDir(), m.Alias)
		backup := backupPath(h, id, m.Alias)
		if m.BeforeExists {
			if err = os.Rename(dest, backup); err != nil {
				return rollback(err)
			}
		}
		if err = point("move-backup:" + m.Alias); err != nil {
			return rollback(err)
		}
		if !m.Remove {
			if err = os.Rename(filepath.Join(h.PluginsDir(), m.Stage), dest); err != nil {
				return rollback(err)
			}
		}
		if err = point("move-publish:" + m.Alias); err != nil {
			return rollback(err)
		}
	}
	for _, f := range j.Files {
		if err = writeImage(statePath(h, f.Name), f.After); err != nil {
			return rollback(err)
		}
		if err = point("json:" + f.Name); err != nil {
			return rollback(err)
		}
	}
	if err = point("before-commit"); err != nil {
		return rollback(err)
	}
	j.Phase = "committed"
	if err = saveJSON(filepath.Join(dir, "journal.json"), &j); err != nil {
		j.Phase = "prepared"
		return rollback(err)
	}
	// Cancellation after the marker cannot turn a committed operation into a rollback.
	if checkpoint != nil {
		if err = checkpoint("committed"); err != nil {
			return committedError(err)
		}
	}
	if err = saveJSON(h.AdministrationFile(), Control{Schema: 1, Epoch: pending.Epoch + 1, Status: "stable"}); err != nil {
		return committedError(err)
	}
	if checkpoint != nil {
		if err = checkpoint("stable"); err != nil {
			return committedError(err)
		}
		if err = checkpoint("cleanup"); err != nil {
			return committedError(err)
		}
	}
	if err = cleanup(h, &j); err != nil {
		return committedError(err)
	}
	return nil
}
func committedError(err error) error {
	return diag.Wrap(diag.AdministrationCleanupFailed, err, "administrative changes were committed; rerun the mutating command to finish recovery and cleanup")
}
func modelBytes(s *Snapshot) ([][]byte, error) {
	values := []any{s.Config, s.Registry, s.Memory, s.Lifecycle}
	result := make([][]byte, len(values))
	for i, v := range values {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			return nil, e
		}
		result[i] = append(b, '\n')
	}
	return result, nil
}
func statePath(h workhome.Home, name string) string {
	switch name {
	case "config":
		return h.ConfigFile()
	case "registry":
		return h.RegistryFile()
	case "memory":
		return h.BranchConventionsFile()
	case "lifecycle":
		return h.ReferencePackageFile()
	}
	return ""
}
func newID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func digest(b []byte) string  { v := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(v[:]) }
func newImage(b []byte) Image { return Image{Exists: true, Bytes: b, Digest: digest(b)} }
func readImage(path string) (Image, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Image{}, nil
	}
	if err != nil {
		return Image{}, err
	}
	return newImage(b), nil
}
func writeImage(path string, i Image) error {
	if !i.Exists {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return atomicfile.WriteFile(path, i.Bytes)
}
func saveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeImage(path, newImage(append(b, '\n')))
}
func backupPath(h workhome.Home, id, alias string) string {
	return filepath.Join(h.PluginsDir(), ".admin-"+id+"-"+alias+"-before")
}

// treeDigest hashes owned paths without following symlinks, including a linked
// source root. File modes are reduced to executability for cross-OS stability.
func treeDigest(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		kind := "file"
		var b []byte
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			kind = "link"
			var v string
			v, err = os.Readlink(path)
			b = []byte(v)
		case d.IsDir():
			kind = "dir"
		case info.Mode().IsRegular():
			b, err = os.ReadFile(path)
		default:
			return errors.New("unsupported package file")
		}
		if err != nil {
			return err
		}
		exec := info.Mode().Perm()&0111 != 0
		_, err = fmt.Fprintf(hash, "%q %s %t %q\n", filepath.ToSlash(rel), kind, exec, b)
		return err
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// Persist records an authorized legacy adoption even if its projection is unchanged.
func (s *Session) Persist(name string) {
	if s.persist == nil {
		s.persist = map[string]bool{}
	}
	s.persist[name] = true
}
