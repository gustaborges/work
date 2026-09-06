package lockfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.lock")
	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	release() // idempotent

	// Reacquire after release.
	release2, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire (2nd): %v", err)
	}
	release2()
}

func TestSecondAcquireBlocksUntilRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.lock")
	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// While held, a bounded acquire times out.
	_, err = AcquireContext(context.Background(), path, 100*time.Millisecond)
	if err != ErrTimeout {
		t.Fatalf("contended Acquire: err = %v, want ErrTimeout", err)
	}

	// Release, then a waiter should succeed.
	done := make(chan error, 1)
	go func() {
		r, e := AcquireContext(context.Background(), path, 2*time.Second)
		if e == nil {
			r()
		}
		done <- e
	}()
	time.Sleep(50 * time.Millisecond)
	release()

	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("waiter failed after release: %v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter did not acquire after release")
	}
}

func TestStaleLockfileIsReusable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.lock")
	// Simulate a leftover file from a crashed process.
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire over stale file: %v", err)
	}
	release()
}

func TestContextCancels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.lock")
	release, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err = AcquireContext(ctx, path, 10*time.Second)
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
