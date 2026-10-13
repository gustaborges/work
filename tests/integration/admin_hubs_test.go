//go:build unix

package integration

import (
	"fmt"
	"github.com/creack/pty"
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"net/url"
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
func hubExit(c *console, menu string) {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		screen := c.screen()
		lines := strings.Split(screen, "\n")
		if len(lines) > 1 && lines[1] == menu && strings.Contains(screen, "↑/↓ move") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if time.Now().After(deadline) {
		c.t.Fatalf("hub menu did not return:\n%s", c.screen())
	}
	c.send("/Leave unchanged\r")
	if code := c.wait(); code != 0 {
		c.t.Fatal(code, c.screen())
	}
	c.mu.Lock()
	raw := string(c.raw)
	c.mu.Unlock()
	if enters, leaves := strings.Count(raw, "\x1b[?1049h"), strings.Count(raw, "\x1b[?1049l"); enters != 1 || leaves != 1 {
		c.t.Fatalf("hub restarted its terminal session: %d enters, %d leaves", enters, leaves)
	}
}

func TestAdminPluginHubToggleUpdateRemoval(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	src := fixtures.Prepare(t, "admin-suite")
	hubRun(t, bin, env, dir, "plugin", "install", src, "--as", "a")
	hubRun(t, bin, env, dir, "repository", "policy", "add", "a/clone")
	c := newConsoleIn(t, dir, bin, env, "plugin")
	managerScreen(c, "Installed (1)", "Enabled")
	if screen := c.screen(); strings.Contains(screen, "release-blue") || strings.Contains(screen, "local-pinned") {
		t.Fatal(screen)
	}
	c.send(" ")
	managerScreen(c, "a disabled.", "Disabled")
	before := adminTree(t, home)
	hubRun(t, bin, env, dir, "plugin", "disable", "a")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("hub/direct disable differed")
	}
	fixtures.Revise(t, src, "green-hub", nil)
	c.send("\r")
	managerScreen(c, "Plugins › a", "Check for updates")
	c.send("\x1b[B\r")
	managerScreen(c, "Review update", "green-hub")
	// Source changes after review must never replace the frozen candidate.
	fixtures.Revise(t, src, "not-reviewed", nil)
	c.send("\x1b[B\r")
	managerScreen(c, "Updated a.", "green-hub")
	c.send("\x1b[B\r")
	managerScreen(c, "Review uninstall", "a/clone")
	c.send("\x1b")
	managerScreen(c, "Plugins › a", "Uninstall")
	if got := hubRun(t, bin, env, dir, "plugin", "list"); !strings.Contains(got, "green-hub") {
		t.Fatal(got)
	}
	c.send("\r")
	managerScreen(c, "Review uninstall", "a/clone")
	c.send("\x1b[B\x1b[B\r")
	managerScreen(c, "Authorize the listed", "Review uninstall")
	c.send("\x1b[A \x1b[B\r")
	managerScreen(c, "Uninstalled a.", "Installed (0)")
	managerExit(c)
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
	for _, surface := range []struct{ name, title string }{{"repository", "Repositories"}, {"convention", "Convention"}} {
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
	managerScreen(c, "Installed (0)", "No plugins installed")
	c.send("i")
	managerScreen(c, "Source:", "Review installation")
	c.send(src + "\rbad/alias\r\r\r")
	managerScreen(c, "Enter a valid plugin alias", "Alias:")
	c.send("\x15a\r\r\r")
	managerScreen(c, "Plugins › Review installation", "Plugin   a")
	c.send("\x1b[B\r")
	managerScreen(c, "Installed a.", "Installed (1)")
	if strings.Contains(c.screen(), "bad/alias") {
		t.Fatal("rejected alias survived", c.screen())
	}
	c.send(" ")
	managerScreen(c, "a disabled.", "Disabled")
	c.send(" ")
	managerScreen(c, "a enabled.", "Enabled")
	before := adminTree(t, home)
	c.send("\r\x1b[B\r")
	managerScreen(c, "Update check", "unchanged")
	c.send("\r")
	managerScreen(c, "Plugins › a", "Check for updates")
	c.send("\x1b")
	managerScreen(c, "Installed (1)", "Esc close")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("pure check changed state")
	}
	c.send("\x1b[C")
	managerScreen(c, "[Official]", "Available")
	c.send("\r")
	managerScreen(c, "Plugins › work-reference", "Install bundled package")
	c.send("\r")
	managerScreen(c, "Install bundled package", "offline")
	c.send("\x1b[B\r")
	managerScreen(c, "Installed work-reference.", "Installed (2)")
	c.send("\x1b")
	managerScreen(c, "[Official]", "Enabled", "Esc close")
	managerExit(c)
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
	managerScreen(c, "Installed (0)", "Esc close")
	c.send("\x1b[C")
	managerScreen(c, "[Official]", "Available")
	managerExit(c)
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

func TestAdminHubReadResultsRemainVisibleUntilBack(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	repo := filepath.Join(dir, "repo")
	makeRepo(t, repo)
	hubRun(t, bin, env, dir, "plugin", "install", fixtures.Prepare(t, "admin-suite"), "--as", "visible")
	hubRun(t, bin, env, dir, "repository", "policy", "add", "visible/clone")
	hubRun(t, bin, env, repo, "convention", "set", "admin-team")
	before := adminTree(t, home)
	for _, tc := range []struct{ hub, action, title, result string }{
		{"repository", "List policy", "Repository policy", "visible/clone  (available)"},
		{"repository", "List installed Locators", "Installed Locators", "visible/clone"},
		{"repository", "List search roots", "Repository search roots", "no repository search roots"},
		{"convention", "Inspect remembered convention", "Remembered convention", "work: convention admin-team"},
	} {
		t.Run(tc.hub+"/"+tc.action, func(t *testing.T) {
			c := newConsoleIn(t, repo, bin, env, tc.hub)
			c.expect("Leave unchanged")
			c.send("/" + tc.action + "\r")
			c.expect("enter Back")
			screen := c.screen()
			if !strings.Contains(screen, tc.title) || !strings.Contains(screen, tc.result) || strings.Contains(screen, "Leave unchanged") {
				t.Fatalf("read result was covered by the menu:\n%s", screen)
			}
			c.send("\r")
			hubExit(c, map[string]string{"plugin": "Plugins", "repository": "Repositories", "convention": "Convention"}[tc.hub])
		})
	}
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("read navigation changed state")
	}
}

func TestAdminHubInstallBackKeepsOneTerminalSession(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, "plugin")
	managerScreen(c, "Installed (0)", "Esc close")
	c.send("i")
	managerScreen(c, "Source:", "Review installation")
	c.send("/unused-source\rremembered-alias\r")
	managerFormBack(c)
	managerScreen(c, "Installed (0)", "Esc close")
	c.send("i")
	managerScreen(c, "Alias: remembered-alias", "Review installation")
	managerFormBack(c)
	managerScreen(c, "Installed (0)", "Esc close")
	managerExit(c)
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("Back changed state")
	}
}

// Assert reconstructed screens, not old text still present in the byte stream.
func managerScreen(c *console, texts ...string) {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		screen := c.screen()
		ok := true
		for _, text := range texts {
			if !strings.Contains(screen, text) {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("manager screen missing %q:\n%s", texts, c.screen())
}
func managerExit(c *console) { managerClose(c, "\x1b") }
func managerClose(c *console, key string) {
	c.t.Helper()
	c.send(key)
	if code := c.wait(); code != 0 {
		c.t.Fatal(code, c.screen())
	}
	c.mu.Lock()
	raw := string(c.raw)
	c.mu.Unlock()
	if strings.Count(raw, "\x1b[?1049h") != 1 || strings.Count(raw, "\x1b[?1049l") != 1 {
		c.t.Fatal("terminal restarted", raw)
	}
	tail := raw[strings.LastIndex(raw, "\x1b[?1049l")+len("\x1b[?1049l"):]
	for _, forbidden := range []string{"Operation cancelled", "work:", "✔", "Equivalent:", "equivalent:"} {
		if strings.Contains(tail, forbidden) {
			c.t.Fatal("manager dumped transcript", tail)
		}
	}
	if strings.Count(tail, "Changes saved successfully.") != 1 {
		c.t.Fatal("missing single acknowledgment", tail)
	}
}

func TestAdminPluginManagerErrorRecoveryAndViewport(t *testing.T) {
	bin := buildWorkBin(t)
	for _, term := range []string{"xterm-256color", "dumb"} {
		t.Run(term, func(t *testing.T) {
			env, dir, home := ptyEnv(t)
			env = append(env, "TERM="+term, "NO_COLOR=1")
			before := adminTree(t, home)
			c := newConsoleIn(t, dir, bin, env, "plugin")
			managerScreen(c, "Installed (0)", "Esc close")
			_ = pty.Setsize(c.f, &pty.Winsize{Rows: 16, Cols: 64})
			c.rows, c.cols = 16, 64
			c.send("i")
			managerScreen(c, "Source:", "Esc tabs")
			c.send("/missing-plugin\r\r\r\r")
			managerScreen(c, "plugin content", "Review installation")
			managerFormBack(c)
			managerScreen(c, "Installed (0)", "Esc close")
			_ = pty.Setsize(c.f, &pty.Winsize{Rows: 50, Cols: 120})
			c.rows, c.cols = 50, 120
			c.send("\x1b[C")
			managerScreen(c, "[Official]", "Available")
			c.send("\r\r")
			managerScreen(c, "Install bundled package", "offline")
			c.send("\x1b")
			managerScreen(c, "Plugins › work-reference", "Install bundled package")
			c.send("\x1b")
			managerScreen(c, "[Official]", "Esc close")
			managerExit(c)
			if !reflect.DeepEqual(before, adminTree(t, home)) {
				t.Fatal("error or declined review changed state")
			}
		})
	}
}

func TestAdminPluginManagerRemoteInstallAndSearch(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, _ := ptyEnv(t)
	src := fixtures.Prepare(t, "admin-suite")
	adminCommitSource(t, src)
	source := (&url.URL{Scheme: "file", Path: filepath.ToSlash(src)}).String()
	c := newConsoleIn(t, dir, bin, env, "plugin")
	managerScreen(c, "Installed (0)", "Esc close")
	c.send("i")
	managerScreen(c, "Source:", "Esc tabs")
	c.send(source + "\rremote-plugin\r\r\r")
	managerScreen(c, "Review installation", "remote-pinned")
	c.send("\x1b[B\r")
	managerScreen(c, "Installed remote-plugin.", "Installed (1)")
	c.send("/REMOTE\r")
	managerScreen(c, "remote-plugin", "Enabled")
	c.send("\r")
	managerScreen(c, "Plugins › remote-plugin", "Origin   remote-pinned")
	c.send("\x1b")
	managerScreen(c, "/ REMOTE", "Esc close")
	c.send("\x1b")
	managerScreen(c, "/ Search plugins", "Esc close")
	managerExit(c)
}

func TestAdminPluginManagerLongInventoryAndRepeatedBack(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	src := fixtures.Prepare(t, "admin-suite")
	for i := 0; i < 28; i++ {
		hubRun(t, bin, env, dir, "plugin", "install", src, "--as", fmt.Sprintf("plugin-%02d-with-a-long-but-distinct-name", i))
	}
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, "plugin")
	managerScreen(c, "Installed (28)", "Esc close")
	_ = pty.Setsize(c.f, &pty.Winsize{Rows: 24, Cols: 80})
	c.rows, c.cols = 24, 80
	c.send("\x1b[F")
	managerScreen(c, "plugin-27-with-a-long-but-distinct-name", "28 of 28")
	for range 3 {
		c.send("\r")
		managerScreen(c, "Plugins › plugin-27", "Check for updates")
		c.send("\x1b")
		managerScreen(c, "plugin-27-with-a-long-but-distinct-name", "Esc close")
	}
	c.send("/plugin-04\r")
	managerScreen(c, "/ plugin-04", "plugin-04-with-a-long-but-distinct-name")
	if strings.Contains(c.screen(), "plugin-27-with-a-long-but-distinct-name") {
		t.Fatal("search retained unrelated row", c.screen())
	}
	c.send("\x1b")
	managerScreen(c, "/ Search plugins", "Esc close")
	managerExit(c)
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("navigation changed state")
	}
}

func TestAdminPluginManagerNormalQuit(t *testing.T) {
	bin := buildWorkBin(t)
	for name, key := range map[string]string{"Esc": "\x1b", "q": "q", "CtrlC": "\x03"} {
		t.Run(name, func(t *testing.T) {
			env, dir, home := ptyEnv(t)
			before := adminTree(t, home)
			c := newConsoleIn(t, dir, bin, env, "plugin")
			managerScreen(c, "Installed (0)", "Esc close")
			managerClose(c, key)
			if !reflect.DeepEqual(before, adminTree(t, home)) {
				t.Fatal("exit initialized state")
			}
		})
	}
}

func TestAdminPluginManagerVisibleSource(t *testing.T) {
	bin := buildWorkBin(t)
	for name, source := range map[string]string{
		"local":   "/tmp/local plugin source",
		"https":   "https://github.com/example/work-plugin.git",
		"ssh":     "git@github.com:example/work-plugin.git",
		"ssh-url": "ssh://git@github.com/example/work-plugin.git",
	} {
		t.Run(name, func(t *testing.T) {
			env, dir, home := ptyEnv(t)
			before := adminTree(t, home)
			c := newConsoleIn(t, dir, bin, env, "plugin")
			managerScreen(c, "Installed (0)", "Esc close")
			c.send("i")
			managerScreen(c, "Source:", "Local path or git URL")
			c.send(source)
			managerScreen(c, "Source: "+source, "Esc tabs")
			if strings.Contains(c.screen(), "••••") {
				t.Fatal("source was masked", c.screen())
			}
			managerFormBack(c)
			managerScreen(c, "Installed (0)", "Esc close")
			managerExit(c)
			if !reflect.DeepEqual(before, adminTree(t, home)) {
				t.Fatal("source editing/Back changed state")
			}
		})
	}
}

func managerFormBack(c *console) {
	c.t.Helper()
	c.send("\x1b")
	managerScreen(c, "Enter/Tab edit form", "Esc back")
	c.send("\x1b")
}

func TestAdminPluginManagerTabNavigationThroughInstall(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, "plugin")
	managerScreen(c, "[Installed (0)]", "Esc close")
	for _, step := range []struct{ key, screen, hint string }{
		{"\x1b[C", "[Official]", "Esc close"},
		{"\x1b[C", "[Install plugin]", "Enter/Tab edit form"},
		{"\x1b[C", "[Installed (0)]", "Esc close"},
		{"\x1b[D", "[Install plugin]", "Enter/Tab edit form"},
		{"\x1b[D", "[Official]", "Esc close"},
		{"\x1b[D", "[Installed (0)]", "Esc close"},
		{"\x1b[D", "[Install plugin]", "Enter/Tab edit form"},
	} {
		c.send(step.key)
		managerScreen(c, step.screen, step.hint)
	}
	c.send("\r")
	managerScreen(c, "Source: █", "Esc tabs")
	c.send("draft-path")
	managerScreen(c, "Source: draft-path█", "Esc tabs")
	c.send("\x1b")
	managerScreen(c, "Source: draft-path", "Enter/Tab edit form")
	c.send("\x1b[C")
	managerScreen(c, "[Installed (0)]", "Esc close")
	c.send("\x1b[D")
	managerScreen(c, "Source: draft-path", "Enter/Tab edit form")
	c.send("\t")
	managerScreen(c, "Source: draft-path█", "Esc tabs")
	managerFormBack(c)
	managerScreen(c, "[Installed (0)]", "Esc close")
	managerExit(c)
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("tab navigation or draft editing changed state")
	}
}
