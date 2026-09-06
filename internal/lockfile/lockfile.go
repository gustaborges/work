// Package lockfile provides advisory, process-scoped file locks used to
// serialize concurrent bootstrap and Work-creation attempts. Locks are
// exclusive and blocking with a timeout; releasing is idempotent.
package lockfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultTimeout bounds how long Acquire waits for a contended lock before
// giving up.
const DefaultTimeout = 30 * time.Second

// ErrTimeout is returned when the lock cannot be acquired within the timeout.
var ErrTimeout = errors.New("lockfile: timed out waiting for lock")

// Acquire takes an exclusive lock on path, creating the file and its parent
// directory if needed, and returns a release function. It blocks up to
// DefaultTimeout.
func Acquire(path string) (release func(), err error) {
	return AcquireContext(context.Background(), path, DefaultTimeout)
}

// AcquireContext is Acquire with an explicit context and timeout. The context
// cancels the wait; the timeout bounds it.
func AcquireContext(ctx context.Context, path string, timeout time.Duration) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("lockfile: creating lock dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lockfile: opening %s: %w", path, err)
	}

	deadline := time.Now().Add(timeout)
	for {
		ok, err := tryLock(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lockfile: locking %s: %w", path, err)
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, ErrTimeout
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = unlock(f)
			_ = f.Close()
		})
	}, nil
}
