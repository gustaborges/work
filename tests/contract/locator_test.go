package contract

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func makeRepo(t *testing.T, parent, name string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

type locatorResp struct {
	Matches []map[string]any `json:"matches"`
}

func decodeLocator(t *testing.T, stdout []byte) locatorResp {
	t.Helper()
	var r locatorResp
	if err := json.Unmarshal(stdout, &r); err != nil {
		t.Fatalf("stdout not JSON: %q", stdout)
	}
	for _, m := range r.Matches {
		for k := range m {
			if k != "repo_path" {
				t.Errorf("match carries ranking/extra key %q: %v", k, m)
			}
		}
	}
	return r
}

func TestLocatorContract(t *testing.T) {
	bin := seedBin(t, "locator")

	t.Run("name match, one repo", func(t *testing.T) {
		root := t.TempDir()
		want := makeRepo(t, root, "project")
		in := `{"repository":{"name":"project"},"repository_roots":["` + root + `"]}`
		res := runBin(t, bin, in)
		if res.exitCode != 0 {
			t.Fatalf("exit %d: %s", res.exitCode, res.stderr)
		}
		r := decodeLocator(t, res.stdout)
		if len(r.Matches) != 1 || r.Matches[0]["repo_path"] != want {
			t.Fatalf("matches = %v, want [%s]", r.Matches, want)
		}
	})

	t.Run("name match, two repos, no ranking keys", func(t *testing.T) {
		root := t.TempDir()
		makeRepo(t, filepath.Join(root, "a"), "project")
		makeRepo(t, filepath.Join(root, "b"), "project")
		in := `{"repository":{"name":"project"},"repository_roots":["` + root + `"]}`
		res := runBin(t, bin, in)
		if res.exitCode != 0 {
			t.Fatalf("exit %d", res.exitCode)
		}
		r := decodeLocator(t, res.stdout)
		if len(r.Matches) != 2 {
			t.Fatalf("matches = %v", r.Matches)
		}
	})

	t.Run("no match", func(t *testing.T) {
		root := t.TempDir()
		makeRepo(t, root, "project")
		in := `{"repository":{"name":"nope"},"repository_roots":["` + root + `"]}`
		res := runBin(t, bin, in)
		if res.exitCode != 0 {
			t.Fatalf("exit %d", res.exitCode)
		}
		r := decodeLocator(t, res.stdout)
		if len(r.Matches) != 0 {
			t.Fatalf("matches = %v, want []", r.Matches)
		}
	})

	t.Run("empty roots", func(t *testing.T) {
		in := `{"repository":{"name":"project"},"repository_roots":[]}`
		res := runBin(t, bin, in)
		if res.exitCode != 0 {
			t.Fatalf("exit %d", res.exitCode)
		}
		r := decodeLocator(t, res.stdout)
		if len(r.Matches) != 0 {
			t.Fatalf("matches = %v", r.Matches)
		}
	})

	t.Run("fetch-url match on non-origin remote", func(t *testing.T) {
		root := t.TempDir()
		clone := makeRepo(t, root, "some-checkout")
		target := "https://example.com/team/widget.git"
		git(t, clone, "remote", "add", "upstream", target)
		in := `{"repository":{"git_fetch_urls":["` + target + `"]},"repository_roots":["` + root + `"]}`
		res := runBin(t, bin, in)
		if res.exitCode != 0 {
			t.Fatalf("exit %d: %s", res.exitCode, res.stderr)
		}
		r := decodeLocator(t, res.stdout)
		if len(r.Matches) != 1 || r.Matches[0]["repo_path"] != clone {
			t.Fatalf("matches = %v, want [%s]", r.Matches, clone)
		}
	})

	t.Run("garbage stdin", func(t *testing.T) {
		res := runBin(t, bin, "xxx")
		if res.exitCode == 0 {
			t.Fatalf("want non-zero exit; stdout %q", res.stdout)
		}
		if len(res.stdout) != 0 {
			t.Errorf("want no stdout, got %q", res.stdout)
		}
	})

	t.Run("unreadable root", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("chmod 0000 does not deny directory reads on Windows")
		}
		if os.Getuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		parent := t.TempDir()
		denied := filepath.Join(parent, "denied")
		if err := os.Mkdir(denied, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(denied, 0o755) })
		in := `{"repository":{"name":"x"},"repository_roots":["` + filepath.Join(denied, "sub") + `"]}`
		res := runBin(t, bin, in)
		if res.exitCode == 0 {
			t.Fatalf("want non-zero exit for unreadable root")
		}
	})
}
