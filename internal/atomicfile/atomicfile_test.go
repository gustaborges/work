package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWriteFileCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.json")
	if err := WriteFile(path, []byte("hello")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("contents = %q, want %q", got, "hello")
	}
}

func TestWriteFileReplacesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("old-and-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}
}

func TestWriteFileNoTempLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := WriteFile(path, []byte("x")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir has %d entries %v, want just the target", len(entries), names)
	}
}

func TestWriteFileErrorLeavesTargetUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	// filepath.Dir(path) does not exist -> CreateTemp fails, target must survive.
	bad := filepath.Join(dir, "missing-subdir", "f")
	if err := WriteFile(bad, []byte("nope")); err == nil {
		t.Fatal("WriteFile into missing dir: want error, got nil")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "original" {
		t.Errorf("target changed: %q", got)
	}
}

func TestConcurrentWritersNeverYieldPartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	payloadA := bytes.Repeat([]byte("A"), 4096)
	payloadB := bytes.Repeat([]byte("B"), 4096)
	if err := os.WriteFile(path, payloadA, 0o644); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := payloadA
			if i%2 == 1 {
				p = payloadB
			}
			if err := WriteFile(path, p); err != nil {
				t.Errorf("WriteFile: %v", err)
			}
		}(i)
	}
	wg.Wait()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payloadA) && !bytes.Equal(got, payloadB) {
		t.Errorf("file is neither complete payload: len=%d", len(got))
	}
}
