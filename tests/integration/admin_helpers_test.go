package integration

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// adminTree uses Lstat/WalkDir and never traverses linked installations or
// opens databases. Nil is distinct from an existing empty directory.
func adminTree(t *testing.T, root string) map[string]string {
	t.Helper()
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	}
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			s, e := os.Readlink(path)
			out[rel] = "link:" + s
			return e
		}
		b, e := os.ReadFile(path)
		out[rel] = "file:" + string(b)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
