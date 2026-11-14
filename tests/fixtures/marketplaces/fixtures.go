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

// WriteEntrypoint writes an executable fixture entrypoint. On Windows it also
// writes the ".exe" sibling that installation requires of native entrypoints.
func WriteEntrypoint(path, body string) error {
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return os.WriteFile(path+".exe", []byte(body), 0o755)
	}
	return nil
}

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
		if err := os.WriteFile(to, b, info.Mode().Perm()); err != nil {
			return err
		}
		// Installation requires a native entrypoint to carry ".exe" on Windows,
		// while the shared fixtures ship one POSIX script per entrypoint.
		if runtime.GOOS == "windows" && filepath.Base(filepath.Dir(rel)) == "bin" && filepath.Ext(rel) == "" {
			return os.WriteFile(to+".exe", b, 0o644)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("copying fixture %q: %v", name, err)
	}
	return dst
}
