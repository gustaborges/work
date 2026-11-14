package skillinstall

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	skills "github.com/gustaborges/work/ai-skills"
	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/lockfile"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

var errInterrupted = errors.New("interrupted checkpoint")

// Engine supplies deterministic failure seams to tests, never environment flags.
type Engine struct{ Checkpoint func(string) error }

func (e Engine) checkpoint(point string) error {
	if e.Checkpoint != nil {
		return e.Checkpoint(point)
	}
	return nil
}
func Install(ctx context.Context, ds []Destination, force bool) []Outcome {
	return (Engine{}).Install(ctx, ds, force)
}
func (e Engine) Install(ctx context.Context, ds []Destination, force bool) []Outcome {
	plans, out, ok := planAll(ds, force)
	if !ok {
		return out
	}
	unique := map[string]Destination{}
	for _, d := range ds {
		unique[d.Path] = d
	}
	keys := []string{}
	for k := range unique {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	releases := []func(){}
	defer func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}()
	fail := func(idx int, token string) []Outcome { out[idx].Action = "failed"; out[idx].Error = token; return out }
	for _, key := range keys {
		d := unique[key]
		idx := 0
		for i, x := range ds {
			if x.Path == key {
				idx = i
				break
			}
		}
		if err := safePath(d); err != nil {
			return fail(idx, "skill-target-conflict")
		}
		if err := os.MkdirAll(filepath.Dir(d.Path), 0755); err != nil {
			return fail(idx, "skill-io-failed")
		}
		if err := safePath(d); err != nil {
			return fail(idx, "skill-target-conflict")
		}
		// Reject malicious lock/record links before any write-capable open.
		lp := sibling(d, ".lock")
		if info, err := os.Lstat(lp); err == nil {
			if !info.Mode().IsRegular() {
				return fail(idx, "skill-target-conflict")
			}
		} else if !os.IsNotExist(err) {
			return fail(idx, "skill-io-failed")
		}
		release, err := lockfile.AcquireContext(ctx, lp, lockfile.DefaultTimeout)
		if err != nil {
			token := "skill-io-failed"
			if errors.Is(err, lockfile.ErrTimeout) {
				token = "skill-install-busy"
			}
			if ctx.Err() != nil {
				token = "cancelled"
			}
			return fail(idx, token)
		}
		releases = append(releases, release)
	}
	for i, p := range plans {
		if p.Before.Result.State == "recovery_required" {
			if err := e.recoverTarget(p.Destination); err != nil {
				return fail(i, "skill-recovery-required")
			}
			digest, _ := skills.Digest(p.Destination.Skill)
			plans[i].Before = Observe(p.Destination, digest)
		} else {
			current := Observe(p.Destination, p.Before.Result.Embedded)
			if !sameObservation(p.Before, current) {
				return fail(i, "skill-target-conflict")
			}
			plans[i].Before = current
		}
	}
	// All locked destinations are revalidated before staging any payload.
	for i, p := range plans {
		r := p.Before.Result
		if r.Error != "" {
			return fail(i, r.Error)
		}
		if !force && (r.State == "foreign" || r.State == "modified") {
			return fail(i, "skill-target-conflict")
		}
	}
	done := map[string]Outcome{}
	for i, p := range plans {
		if previous, ok := done[p.Destination.Path]; ok {
			previous.Destination = p.Destination
			out[i] = previous
			continue
		}
		out[i] = e.apply(ctx, p)
		done[p.Destination.Path] = out[i]
		if out[i].Error != "" {
			break
		}
	}
	return out
}
func (e Engine) apply(ctx context.Context, p Plan) (out Outcome) {
	d := p.Destination
	out = Outcome{Destination: d, Action: "failed", Commit: "none"}
	iofail := func() Outcome { out.Error = "skill-io-failed"; return out }
	if ctx.Err() != nil {
		out.Error = "cancelled"
		return out
	}
	if p.Before.Result.State == "current" {
		out.Action = "unchanged"
		return out
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return iofail()
	}
	op := hex.EncodeToString(random)
	stem := ".work-skill-" + d.Skill + "." + op
	r := Record{Schema: 1, Operation: op, Skill: d.Skill, Stage: stem + ".stage", Backup: stem + ".backup", Phase: "prepared", OldExists: p.Before.Result.State != "absent", OldFingerprint: p.Before.Fingerprint}
	stage, backup := recordPaths(d, r)
	if err := safeParents(d, p.Before); err != nil {
		out.Error = "skill-target-conflict"
		return out
	}
	if err := os.Mkdir(stage, 0755); err != nil {
		return iofail()
	}
	if err := e.checkpoint("stage-created"); err != nil {
		_ = os.RemoveAll(stage)
		return iofail()
	}
	recorded := false
	defer func() {
		if !recorded && safeParents(d, p.Before) == nil {
			_ = os.RemoveAll(stage)
		}
	}()
	tree, err := skills.Tree(d.Skill)
	if err != nil {
		return iofail()
	}
	err = fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		dest := filepath.Join(stage, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		b, err := fs.ReadFile(tree, name)
		if err != nil {
			return err
		}
		if err = os.WriteFile(dest, b, 0644); err != nil {
			return err
		}
		f, err := os.OpenFile(dest, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		err = f.Sync()
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		return e.checkpoint("file:" + name)
	})
	if err != nil {
		return iofail()
	}
	marker := Marker{1, d.Skill, skills.Algorithm, p.Before.Result.Embedded}
	b, err := json.Marshal(marker)
	if err != nil {
		return iofail()
	}
	if err = atomicfile.WriteFile(filepath.Join(stage, MarkerName), b); err != nil {
		return iofail()
	}
	got, err := skills.TreeDigest(os.DirFS(stage))
	if err != nil || got != marker.Digest {
		return iofail()
	}
	r.NewDigest = marker.Digest
	markerHash := sha256.Sum256(b)
	r.NewMarker = hex.EncodeToString(markerHash[:])
	r.NewFingerprint, err = fingerprint(stage)
	if err != nil {
		return iofail()
	}
	if err = e.checkpoint("stage"); err != nil {
		return iofail()
	}
	current := Observe(d, marker.Digest)
	if !sameObservation(p.Before, current) {
		out.Error = "skill-target-conflict"
		return out
	}
	writeRecord := func() error {
		if err := safeParents(d, p.Before); err != nil {
			return err
		}
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		return atomicfile.WriteFileContext(ctx, sibling(d, ".transaction.json"), b)
	}
	if err = syncDirectory(stage); err != nil {
		return iofail()
	}
	if err = writeRecord(); err != nil {
		return iofail()
	}
	recorded = true
	if err = syncDirectory(filepath.Dir(d.Path)); err != nil {
		out.Commit = "recovery-required"
		out.Error = "skill-recovery-required"
		return out
	}
	pending := func() Outcome { out.Commit = "recovery-required"; out.Error = "skill-recovery-required"; return out }
	if err = e.checkpoint("prepared"); err != nil {
		return pending()
	}
	// Record existence intentionally prevents Observe here; compare full evidence.
	exists, fp, err := treeEvidence(d.Path)
	if err != nil || exists != r.OldExists || fp != r.OldFingerprint {
		return pending()
	}
	if err = safeParents(d, p.Before); err != nil {
		return pending()
	}
	if r.OldExists {
		if err = os.Rename(d.Path, backup); err != nil {
			return pending()
		}
	}
	if err = syncDirectory(filepath.Dir(d.Path)); err != nil {
		return pending()
	}
	if err = e.checkpoint("old-moved"); err != nil {
		return pending()
	}
	if err = safeParents(d, p.Before); err != nil {
		return pending()
	}
	if err = os.Rename(stage, d.Path); err != nil {
		return pending()
	}
	if err = syncDirectory(filepath.Dir(d.Path)); err != nil {
		return pending()
	}
	if err = e.checkpoint("new-moved"); err != nil {
		return pending()
	}
	exists, fp, err = treeEvidence(d.Path)
	if err != nil || !exists || fp != r.NewFingerprint {
		return pending()
	}
	r.Phase = "committed"
	if err = writeRecord(); err != nil {
		return pending()
	}
	out.Action = "installed"
	if r.OldExists {
		out.Action = "updated"
	}
	out.Commit = "committed"
	if err = e.checkpoint("committed"); err != nil {
		out.Commit = "cleanup-pending"
		out.Error = "skill-cleanup-pending"
		return out
	}
	if err = e.recoverTarget(d); err != nil {
		out.Commit = "cleanup-pending"
		out.Error = "skill-cleanup-pending"
	}
	return out
}

// Error returns only a safe operational summary; causes are not user content.
func (o Outcome) String() string {
	return fmt.Sprintf("%s vendor=%s skill=%s path=%q commit=%s", o.Action, o.Destination.Vendor, o.Destination.Skill, o.Destination.Path, o.Commit)
}
