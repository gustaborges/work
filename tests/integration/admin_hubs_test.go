//go:build unix

package integration

import (
	"github.com/creack/pty"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func hubRun(t *testing.T, bin string, env []string, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(args, err, string(out))
	}
	return string(out)
}
func expectHubRefresh(c *console, from int, text string) {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s := c.snapshot()
		if len(s) > from && strings.Contains(s[from:], text) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("no refreshed %q: %s", text, c.snapshot())
}
func hubExit(c *console, menu string) {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		screen := c.screen()
		if strings.Contains(screen, menu) && strings.Contains(screen, "Leave unchanged") && strings.Contains(screen, "↑/↓ move") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.send("/Leave unchanged\r")
	if code := c.wait(); code != 0 {
		c.t.Fatal(code, c.screen())
	}
	c.mu.Lock()
	raw := string(c.raw)
	c.mu.Unlock()
	if !strings.Contains(raw, "\x1b[?1049l") {
		c.t.Fatal("alternate screen was not restored")
	}
}

func TestAdminPluginHubToggleUpdateRemoval(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	src := fixtures.Prepare(t, "admin-suite")
	hubRun(t, bin, env, dir, "plugin", "install", src, "--as", "a")
	hubRun(t, bin, env, dir, "repository", "policy", "add", "a/clone")
	c := newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("Plugins")
	c.send("/Disable plugins\r")
	c.expect("Select plugins to disable")
	c.send(" \r")
	c.expect("work: disabled a")
	c.expect("enabled: false")
	hubExit(c, "Plugins")
	before := adminTree(t, home)
	hubRun(t, bin, env, dir, "plugin", "disable", "a")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("hub/direct disable differed")
	}
	fixtures.Revise(t, src, "green-hub", nil)
	c = newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("Plugins")
	c.send("/Apply updates\r")
	c.expect("Select plugins to update")
	c.send(" \r")
	c.expect("Apply plugin updates")
	c.expect("green-hub")
	c.send("y")
	c.expect("work: update a updated")
	c.expect("green-hub  enabled: false")
	hubExit(c, "Plugins")
	c = newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("Plugins")
	c.send("/Uninstall plugins\r")
	c.expect("Select plugins to uninstall")
	c.send(" \r")
	c.expect("Remove policy references")
	c.send("n")
	if code := c.wait(); code != 20 {
		t.Fatal(code, c.screen())
	}
	c = newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("Plugins")
	c.send("/Uninstall plugins\r")
	c.expect("Select plugins to uninstall")
	c.send(" \r")
	c.expect("Remove policy references")
	c.send("y")
	c.expect("Uninstall plugins: a")
	c.send("y")
	c.expect("work: uninstalled a")
	c.expect("No plugins installed")
	hubExit(c, "Plugins")
}
func TestAdminRepositoryHubRootCorrectionAndClear(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	c := newConsoleIn(t, dir, bin, env, "repository")
	c.expect("Repositories")
	c.send("/Add search roots\r")
	c.expect("Choose repository search directories")
	c.send("/Enter directory path\r")
	c.expect("Search root directory")
	c.send("/nonexistent-admin-root\r")
	c.expect("does not exist")
	c.send("\x15" + dir + "\r")
	c.expect("Selected: " + dir)
	c.send("/Review selected entries\r")
	c.expect("Review repository change")
	c.send("y")
	c.expect("work: roots now " + dir)
	c.expect("Search roots (independent of workspace location): " + dir)
	hubExit(c, "Repositories")
	before := adminTree(t, home)
	hubRun(t, bin, env, dir, "repository", "root", "add", dir)
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("hub/direct roots differed")
	}
	if strings.Contains(c.screen(), "nonexistent-admin-root") {
		t.Fatal("rejected field survived", c.screen())
	}
	c = newConsoleIn(t, dir, bin, env, "repository")
	c.expect("Repositories")
	c.send("/Replace search roots\r")
	c.expect("Choose repository search directories")
	c.send("/Review selected entries\r")
	c.expect("Review repository change")
	c.expect("work repository root remove")
	c.send("y")
	c.expect("work: roots now (none)")
	c.expect("location): (none)")
	hubExit(c, "Repositories")
}
func TestAdminHubEmptyNavigationCancellationAndViewport(t *testing.T) {
	bin := buildWorkBin(t)
	for _, surface := range []struct{ name, title string }{{"plugin", "Plugins"}, {"repository", "Repositories"}, {"convention", "Convention"}} {
		for _, term := range []string{"xterm-256color", "dumb"} {
			t.Run(surface.name+"/"+term, func(t *testing.T) {
				env, dir, home := ptyEnv(t)
				makeRepo(t, filepath.Join(dir, "repo"))
				dir = filepath.Join(dir, "repo")
				env = append(env, "TERM="+term, "NO_COLOR=1")
				before := adminTree(t, home)
				c := newConsoleSize(t, pty.Winsize{Rows: 24, Cols: 80}, bin, env, surface.name)
				// Convention needs a repository working directory.
				if surface.name == "convention" {
					_ = c.cmd.Process.Kill()
					_ = c.wait()
					c = newConsoleIn(t, dir, bin, env, surface.name)
				}
				c.expect(surface.title)
				_ = pty.Setsize(c.f, &pty.Winsize{Rows: 50, Cols: 120})
				c.send("\x03")
				if code := c.wait(); code != 20 {
					t.Fatal(code, c.screen())
				}
				if !reflect.DeepEqual(before, adminTree(t, home)) {
					t.Fatal("navigation mutated home")
				}
				if strings.Count(c.screen(), "Operation cancelled") != 1 {
					t.Fatal("expected one error", c.screen())
				}
			})
		}
	}
}

func TestAdminConventionStartExplicitSoleReplacement(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	repo := filepath.Join(dir, "src", "demo")
	makeRepo(t, repo)
	hubRun(t, bin, env, dir, "plugin", "install", "--embedded")
	hubRun(t, bin, env, dir, "plugin", "install", fixtures.Prepare(t, "admin-suite"), "--as", "a")
	hubRun(t, bin, env, repo, "convention", "set", "admin-team")
	hubRun(t, bin, env, dir, "plugin", "disable", "a")
	writeRepositoryRoots(t, home, filepath.Dir(repo))
	ws := filepath.Join(dir, "ws")
	args := []string{"start", repo, "--workspace", ws, "--base", "main", "--slug", "replace", "--prefix", "{slug}", "--yes"}
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 38 || !strings.Contains(string(out), "convention-unknown") {
		t.Fatal(err, string(out))
	}
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, args...)
	c.expect("Convention")
	c.expect("freeform")
	c.send("\x03")
	if code := c.wait(); code != 20 {
		t.Fatal(code, c.screen())
	}
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("replacement cancellation changed memory/home")
	}
	c = newConsoleIn(t, dir, bin, env, args...)
	c.expect("Convention")
	c.expect("freeform")
	c.send("\r")
	c.expect("work: created")
	if code := c.wait(); code != 0 {
		t.Fatal(code, c.screen())
	}
	assertConventionShow(t, bin, env, repo, "work: convention freeform")
	// A selected provider changing after convention acceptance fails before Work
	// materialization, even when no executable declaration changed.
	hubRun(t, bin, env, repo, "convention", "set", "freeform")
	args = []string{"start", repo, "--base", "main", "--slug", "drift", "--prefix", "{slug}"}
	c = newConsoleIn(t, dir, bin, env, args...)
	c.expect("Create Work")
	hubRun(t, bin, env, dir, "plugin", "disable", "work-reference")
	c.send("y")
	if code := c.wait(); code != 53 {
		t.Fatal(code, c.screen())
	}
}

func TestAdminPluginHubInstallCheckAndEnable(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	src := fixtures.Prepare(t, "admin-suite")
	c := newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("No plugins installed")
	c.send("/Install from source\r")
	c.expect("Plugin source")
	c.send(src + "\r")
	c.expect("Plugin alias")
	c.send("bad/alias\r")
	c.expect("not a valid plugin alias")
	c.send("\x15a\r")
	c.expect("Installation mode")
	c.send("\r")
	c.expect("work: installed a")
	c.expect("a  release-blue")
	hubExit(c, "Plugins")
	if strings.Contains(c.screen(), "bad/alias") {
		t.Fatal("rejected alias survived", c.screen())
	}
	hubRun(t, bin, env, dir, "plugin", "disable", "a")
	c = newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("enabled: false")
	c.send("/Enable plugins\r")
	c.expect("Select plugins to enable")
	c.send(" \r")
	c.expect("work: enabled a")
	c.expect("enabled: true")
	hubExit(c, "Plugins")
	before := adminTree(t, home)
	c = newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("Plugins")
	c.send("/Check updates\r")
	c.expect("Select plugins to check")
	offset := len(c.snapshot())
	c.send(" \r")
	c.expect("unchanged")
	expectHubRefresh(c, offset, "Plugins")
	hubExit(c, "Plugins")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("hub check mutated home")
	}
	c = newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("Plugins")
	c.send("/Install bundled package\r")
	c.expect("Installs offline")
	c.send("y")
	c.expect("work: installed work-reference")
	c.expect("embedded-seed")
	hubExit(c, "Plugins")
}
func TestAdminRepositoryHubPolicyOperations(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	src := fixtures.Prepare(t, "admin-suite")
	for _, alias := range []string{"a", "b"} {
		hubRun(t, bin, env, dir, "plugin", "install", src, "--as", alias)
	}
	hubRun(t, bin, env, dir, "plugin", "disable", "b")
	for _, alias := range []string{"a", "b"} {
		c := newConsoleIn(t, dir, bin, env, "repository")
		c.expect("Repositories")
		c.send("/Add policy Locator\r")
		c.expect("Select policy Locator")
		c.send("/" + alias + "/clone\r")
		c.expect("Policy position")
		c.send("\r")
		c.expect("Review repository change")
		c.send("y")
		c.expect("work: policy now")
		c.expect("1  a/clone")
		if alias == "b" {
			c.expect("package-disabled")
		}
		hubExit(c, "Repositories")
	}
	c := newConsoleIn(t, dir, bin, env, "repository")
	c.expect("Repositories")
	c.send("/Move policy Locator\x1b[B\r")
	c.expect("Select policy Locator")
	c.send("/b/clone\r")
	c.expect("Policy position")
	c.send("\r")
	c.expect("Review repository change")
	c.expect("--before a/clone")
	c.send("y")
	c.expect("work: policy now b/clone, a/clone")
	c.expect("1  b/clone")
	hubExit(c, "Repositories")
	before := adminTree(t, home)
	hubRun(t, bin, env, dir, "repository", "policy", "replace", "b/clone", "a/clone")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("hub/direct order differed")
	}
	c = newConsoleIn(t, dir, bin, env, "repository")
	c.expect("Repositories")
	c.send("/Replace policy order\r")
	c.expect("Policy order")
	c.send("/a/clone\r")
	c.expect("Selected: a/clone")
	c.send("/b/clone\r")
	c.expect("Selected: a/clone, b/clone")
	c.send("/Review selected entries\r")
	c.expect("Review repository change")
	c.send("y")
	c.expect("work: policy now a/clone, b/clone")
	c.expect("1  a/clone")
	hubExit(c, "Repositories")
	c = newConsoleIn(t, dir, bin, env, "repository")
	c.expect("Repositories")
	c.send("/Remove policy Locators\r")
	c.expect("Remove policy entries")
	c.send(" \x1b[B \r")
	c.expect("Review repository change")
	c.send("y")
	c.expect("work: policy now (none)")
	c.expect("(empty)")
	hubExit(c, "Repositories")
}

func TestAdminHubEmptyBackAndIdentitySafety(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	repo := filepath.Join(dir, "repo")
	makeRepo(t, repo)
	gitIn(t, repo, "remote", "add", "origin", "https://private-user:private-password@example.test/project?token=private-token#private-fragment")
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, "plugin")
	c.expect("No plugins installed")
	c.send("/Disable plugins\r")
	c.expect("Nothing to select")
	c.send("\r")
	hubExit(c, "Plugins")
	c = newConsoleIn(t, repo, bin, env, "convention")
	c.expect("Repository: https://example.test/project")
	c.send("/Set available\r")
	c.expect("No available convention")
	c.send("\r")
	hubExit(c, "Convention")
	for _, secret := range []string{"private-user", "private-password", "private-token", "private-fragment"} {
		if strings.Contains(c.snapshot(), secret) {
			t.Fatal("hub leaked identity", secret)
		}
	}
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("Back/Leave unchanged initialized home")
	}
}
