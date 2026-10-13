package integration

import (
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
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
		for _, args := range [][]string{{"plugin", "update", "--check", "--json"}, {"plugin", "list", "--json"}, {"repository", "locator", "list", "--json"}, {"repository", "policy", "list", "--json"}, {"repository", "root", "list", "--json"}} {
			h.run(t, h.root, 0, args...)
		}
	}
	if !reflect.DeepEqual(before, adminTree(t, adminHome(h))) {
		t.Fatal("read created state")
	}
}

func TestAdminUpdateRepeatedCheckPurity(t *testing.T) {
	h := newAdminHarness(t)
	temp := t.TempDir()
	h.env = append(h.env, "TMPDIR="+temp, "TMP="+temp, "TEMP="+temp)
	source := fixtures.Prepare(t, "admin-suite")
	h.run(t, h.root, 0, "plugin", "install", source, "--as", "a")
	log := filepath.Join(h.root, "invocations")
	h.env = append(h.env, "WORK_ADMIN_LOG="+log)
	for _, kind := range []string{"unchanged", "newer", "invalid", "inaccessible"} {
		t.Run(kind, func(t *testing.T) {
			code := 0
			switch kind {
			case "newer":
				fixtures.Revise(t, source, "green", nil)
			case "invalid":
				if err := os.WriteFile(filepath.Join(source, "plugin.json"), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
				code = 50
			case "inaccessible":
				if err := os.Rename(source, source+"-moved"); err != nil {
					t.Fatal(err)
				}
				defer os.Rename(source+"-moved", source)
				code = 50
			}
			homeBefore := adminTree(t, adminHome(h))
			tempBefore := adminTree(t, temp)
			for range 20 {
				h.run(t, h.root, code, "plugin", "update", "--check", "a", "--json")
			}
			if !reflect.DeepEqual(homeBefore, adminTree(t, adminHome(h))) || !reflect.DeepEqual(tempBefore, adminTree(t, temp)) {
				t.Fatal("explicit check leaked persistent or temporary state")
			}
		})
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("check executed a component")
	}
}
