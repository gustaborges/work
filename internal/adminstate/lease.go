package adminstate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gustaborges/work/internal/atomicfile"
	"github.com/gustaborges/work/internal/lockfile"
	"github.com/gustaborges/work/internal/marketplace"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/workhome"
)

// Lease kinds name what a lease protects from orphan collection.
const (
	// LeasePackageStage protects a transaction-owned package stage directory
	// (plugins/.admin-stage-<id>) that a plan prepares outside the lock.
	LeasePackageStage = "package-stage"
	// LeaseCaptureStage protects a private capture under construction
	// (state/marketplace-snapshots/.stage-<id>).
	LeaseCaptureStage = "capture-stage"
	// LeaseCapture protects a published capture (a 64-hex directory) that a
	// plan reads, even after a refresh stops referencing it.
	LeaseCapture = "capture"
)

const leaseSchema = 1

var (
	leaseID           = regexp.MustCompile(`^[0-9a-f]{32}$`)
	captureStagePrefx = ".stage-"
)

type leaseRecord struct {
	Schema int    `json:"schema"`
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Object string `json:"object"`
}

func (r leaseRecord) valid() bool {
	if r.Schema != leaseSchema || !leaseID.MatchString(r.ID) {
		return false
	}
	switch r.Kind {
	case LeasePackageStage:
		return r.Object == workhome.StagePrefix+r.ID
	case LeaseCaptureStage:
		return r.Object == captureStagePrefx+r.ID
	case LeaseCapture:
		return pluginid.ValidateHexDigest(r.Object) == nil
	}
	return false
}

// Lease is a per-plan, process-held claim on a prepared object. Liveness is
// the OS lock on the lease's lock file, never age or PID: a crashed process
// releases the lock, so its abandoned object becomes collectable at once.
type Lease struct {
	home   workhome.Home
	rec    leaseRecord
	unlock func()
	once   sync.Once
}

// Object is the protected object name (a directory base name or hex digest).
func (l *Lease) Object() string { return l.rec.Object }

// ID is the lease identity.
func (l *Lease) ID() string { return l.rec.ID }

func leasePaths(h workhome.Home, id string) (record, lock string) {
	return filepath.Join(h.LeasesDir(), id+".json"), filepath.Join(h.LeasesDir(), id+".lock")
}

func newLeaseID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}

// acquireLease writes the record and takes the process-held lock under a short
// administration lock, so a concurrent collector either sees the live lock or
// runs before the object exists.
func acquireLease(ctx context.Context, h workhome.Home, kind, object string, rec leaseRecord, prepare func() error) (*Lease, error) {
	release, err := acquire(ctx, h, lockfile.DefaultTimeout)
	if err != nil {
		return nil, err
	}
	defer release()
	rec.Schema, rec.Kind, rec.Object = leaseSchema, kind, object
	if !rec.valid() {
		return nil, errors.New("invalid lease request")
	}
	recPath, lockPath := leasePaths(h, rec.ID)
	unlock, err := lockfile.AcquireContext(ctx, lockPath, lockfile.DefaultTimeout)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(rec)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(recPath), 0o700)
	}
	if err == nil {
		err = atomicfile.WriteFile(recPath, append(data, '\n'))
	}
	if err == nil && prepare != nil {
		err = prepare()
	}
	if err != nil {
		unlock()
		_ = os.Remove(recPath)
		_ = os.Remove(lockPath)
		return nil, err
	}
	return &Lease{home: h, rec: rec, unlock: unlock}, nil
}

// NewPackageStage creates a leased package stage directory outside any
// journal. The caller fills it and hands it to Session.Replace (or removes it)
// and then releases the lease.
func NewPackageStage(ctx context.Context, h workhome.Home) (*Lease, string, error) {
	id, err := newLeaseID()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Join(h.PluginsDir(), workhome.StagePrefix+id)
	l, err := acquireLease(ctx, h, LeasePackageStage, workhome.StagePrefix+id, leaseRecord{ID: id}, func() error {
		if err := os.MkdirAll(h.PluginsDir(), 0o700); err != nil {
			return err
		}
		return os.Mkdir(dir, 0o700)
	})
	if err != nil {
		return nil, "", err
	}
	return l, dir, nil
}

// NewCaptureStage leases a private capture location. The directory is not
// created: the capture routine requires a path that does not exist yet.
func NewCaptureStage(ctx context.Context, h workhome.Home) (*Lease, string, error) {
	id, err := newLeaseID()
	if err != nil {
		return nil, "", err
	}
	l, err := acquireLease(ctx, h, LeaseCaptureStage, captureStagePrefx+id, leaseRecord{ID: id}, func() error {
		return os.MkdirAll(h.SnapshotsDir(), 0o700)
	})
	if err != nil {
		return nil, "", err
	}
	return l, filepath.Join(h.SnapshotsDir(), captureStagePrefx+id), nil
}

// HoldCapture leases a published capture for a plan that will read it.
func HoldCapture(ctx context.Context, h workhome.Home, hexDigest string) (*Lease, error) {
	id, err := newLeaseID()
	if err != nil {
		return nil, err
	}
	return acquireLease(ctx, h, LeaseCapture, hexDigest, leaseRecord{ID: id}, func() error {
		dir, err := h.SnapshotDir(hexDigest)
		if err != nil {
			return err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return errors.New("capture is unavailable")
		}
		return nil
	})
}

// releaseWait bounds how long Release waits for administration. Record cleanup
// is housekeeping: a dead record is collected by the next mutation, so a busy
// administration lock (for example during a long component invocation) must
// never stall a closing plan.
const releaseWait = 250 * time.Millisecond

// Release drops the process-held lock first and only then briefly takes
// administration to delete the record, so a releasing plan never waits for
// administration while holding the lease. It is idempotent and never fails: if
// administration is busy the dead record is collected later.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		l.unlock()
		ctx, cancel := context.WithTimeout(context.Background(), releaseWait)
		defer cancel()
		release, err := acquire(ctx, l.home, releaseWait)
		if err != nil {
			return
		}
		defer release()
		rec, lock := leasePaths(l.home, l.rec.ID)
		_ = os.Remove(rec)
		_ = os.Remove(lock)
	})
}

// liveSet is the set of objects protected by leases whose locks are held.
type liveSet map[string]bool

// collectLeases classifies every lease under administration. A record whose
// lock can be taken without blocking belongs to a dead holder and is removed;
// a held lock is live and its object is reported as protected.
func collectLeases(h workhome.Home) (liveSet, error) {
	live := liveSet{}
	entries, err := os.ReadDir(h.LeasesDir())
	if errors.Is(err, os.ErrNotExist) {
		return live, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		id, ok := strings.CutSuffix(name, ".json")
		if !ok {
			id, ok = strings.CutSuffix(name, ".lock")
		}
		if !ok || !leaseID.MatchString(id) {
			continue // not a lease artifact; never touch arbitrary paths
		}
		recPath, lockPath := leasePaths(h, id)
		unlock, err := lockfile.AcquireContext(context.Background(), lockPath, 0)
		if err != nil {
			if errors.Is(err, lockfile.ErrTimeout) {
				if rec, ok := readLease(recPath); ok && rec.ID == id {
					live[rec.Object] = true
				}
				continue
			}
			return nil, err
		}
		_ = os.Remove(recPath)
		unlock()
		_ = os.Remove(lockPath)
	}
	return live, nil
}

func readLease(path string) (leaseRecord, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return leaseRecord{}, false
	}
	var r leaseRecord
	if json.Unmarshal(b, &r) != nil || !r.valid() {
		return leaseRecord{}, false
	}
	return r, true
}

// collectOrphans removes only unreferenced, unleased, owned objects:
// transaction stages directly under the plugins directory and captures or
// capture stages directly under the snapshots directory. It is an internal
// mutation performed under administration, never a list or check side effect.
func collectOrphans(h workhome.Home) error {
	live, err := collectLeases(h)
	if err != nil {
		return recoveryError(err)
	}
	if err := collectStages(h, live); err != nil {
		return recoveryError(err)
	}
	if err := collectCaptures(h, live); err != nil {
		return recoveryError(err)
	}
	return nil
}

func collectStages(h workhome.Home, live liveSet) error {
	entries, err := os.ReadDir(h.PluginsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if workhome.IsStageName(e.Name()) && !live[e.Name()] {
			if err := os.RemoveAll(filepath.Join(h.PluginsDir(), e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectCaptures(h workhome.Home, live liveSet) error {
	entries, err := os.ReadDir(h.SnapshotsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// Without a valid reference set nothing can be proven unreferenced, so an
	// unreadable catalog state keeps every published capture.
	referenced := map[string]bool{}
	cats, catErr := marketplace.LoadState(h.CatalogsFile())
	if catErr == nil {
		for _, c := range cats.Catalogs {
			if c.CaptureDigest != nil {
				if hexDigest, ok := marketplace.HexPayload(*c.CaptureDigest); ok {
					referenced[hexDigest] = true
				}
			}
		}
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case pluginid.ValidateHexDigest(name) == nil:
			if catErr != nil || referenced[name] || live[name] {
				continue
			}
		case strings.HasPrefix(name, captureStagePrefx) && leaseID.MatchString(strings.TrimPrefix(name, captureStagePrefx)):
			if live[name] {
				continue
			}
		default:
			continue
		}
		if err := os.RemoveAll(filepath.Join(h.SnapshotsDir(), name)); err != nil {
			return err
		}
	}
	return nil
}

// PublishCapture moves a completed private capture to its fixed hex-only
// directory. It must run inside a write session so publication and the
// catalog reference that follows share one administration lock: a crash can
// leave an unreferenced complete capture, never a reference to partial bytes.
// An already-published capture with the same digest is kept and the stage is
// discarded.
func (s *Session) PublishCapture(stage, captureDigest string) error {
	hexDigest, ok := marketplace.HexPayload(captureDigest)
	if !ok {
		return errors.New("invalid capture digest")
	}
	dest, err := s.home.SnapshotDir(hexDigest)
	if err != nil {
		return err
	}
	if filepath.Dir(stage) != s.home.SnapshotsDir() || !strings.HasPrefix(filepath.Base(stage), captureStagePrefx) {
		return errors.New("capture stage is not transaction-owned")
	}
	if info, err := os.Lstat(dest); err == nil {
		if !info.IsDir() {
			return errors.New("capture path is not a directory")
		}
		return os.RemoveAll(stage)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(stage, dest)
}
