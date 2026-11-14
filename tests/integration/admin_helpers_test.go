package integration

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/repoidentity"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/tests/fixtures/installfx"
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

// home resolves the Work home the harness's binary runs against.
func (h *f6Harness) home() workhome.Home {
	root := ""
	for _, e := range h.env {
		if v, ok := strings.CutPrefix(e, "WORK_HOME="); ok {
			root = v
		}
	}
	return workhome.At(root)
}

// install publishes the prepared plugin directory as <manifest-name>@<marketplace>
// through the production administrative journal, standing in for "installed from
// some earlier marketplace add". It returns the provider it installed as.
func (h *f6Harness) install(t *testing.T, marketplace, dir string) pluginid.Provider {
	t.Helper()
	return installfx.InstallAs(t, h.home(), marketplace, dir, true)
}

// mustIdentity computes the ADR-0011 repository identity of the clone at dir.
func mustIdentity(t *testing.T, dir string) string {
	t.Helper()
	id, err := repoidentity.Identify(gitx.Open(dir))
	if err != nil {
		t.Fatal(err)
	}
	return id
}
