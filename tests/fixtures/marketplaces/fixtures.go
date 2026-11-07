// Package marketplaces provides static marketplace source fixtures for tests:
// valid single, multiple and homonymous catalogs plus invalid-shape catalogs.
// Every fixture is copied into a disposable directory before use, so tests can
// mutate, move or delete their "author" source freely.
package marketplaces

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Copy copies the named fixture into a fresh temporary directory and returns
// the copy's path. Executable bits are preserved.
func Copy(t testing.TB, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), name)
	dst := filepath.Join(t.TempDir(), name)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("copying fixture %q: %v", name, err)
	}
	return dst
}
