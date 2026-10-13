package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func newAdminHarness(t *testing.T) *f6Harness {
	t.Helper()
	root := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	h := &f6Harness{root: root, bin: filepath.Join(root, "work"+suffix)}
	if out, err := exec.Command("go", "build", "-o", h.bin, "github.com/gustaborges/work/cmd/work").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	h.env = append(os.Environ(), "WORK_HOME="+filepath.Join(root, "work home ação"), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	return h
}
func adminHome(h *f6Harness) string { return filepath.Join(h.root, "work home ação") }
func TestAdminPureFreshReads(t *testing.T) {
	h := newAdminHarness(t)
	before := adminTree(t, adminHome(h))
	for range 20 {
		for _, args := range [][]string{{"plugin", "list", "--json"}, {"repository", "locator", "list", "--json"}, {"repository", "policy", "list", "--json"}, {"repository", "root", "list", "--json"}} {
			h.run(t, h.root, 0, args...)
		}
	}
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("read created state")
	}
}
