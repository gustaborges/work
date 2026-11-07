//go:build unix

package integration

import (
	"github.com/creack/pty"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/workhome"
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
	expectWrappedHubText(c, "Selected: "+dir)
	c.send("/Review selected entries\r")
	c.expect("Review repository change")
	c.send("y")
	// Receipts truncate long paths; the refreshed inventory retains the full root.
	c.expect("work: roots now ")
	expectWrappedHubText(c, "Search roots (independent of workspace location): "+dir)
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

// Long temporary paths can wrap at spaces, hyphens or the terminal edge.
// Compare the current screen's text independently of those line breaks.
func expectWrappedHubText(c *console, text string) {
	c.t.Helper()
	compact := func(s string) string { return strings.Join(strings.Fields(s), "") }
	want := compact(text)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(compact(c.screen()), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("hub screen missing wrapped text %q:\n%s", text, c.screen())
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
	if err := bootstrap.EnsureSeed(workhome.At(home)); err != nil {
		t.Fatal(err)
	}
	installPluginFixtureIn(t, env, "a", "admin-suite")
	hubRun(t, bin, env, repo, "convention", "set", "admin-team")
	hubRun(t, bin, env, dir, "plugin", "disable", "admin-suite@a")
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
	assertConventionShow(t, bin, env, repo, "work: convention work-reference@builtin/freeform")
	// A selected provider changing after convention acceptance fails before Work
	// materialization, even when no executable declaration changed.
	hubRun(t, bin, env, repo, "convention", "set", "freeform")
	args = []string{"start", repo, "--base", "main", "--slug", "drift", "--prefix", "{slug}"}
	c = newConsoleIn(t, dir, bin, env, args...)
	c.expect("Create Work")
	hubRun(t, bin, env, dir, "plugin", "disable", "work-reference@builtin")
	c.send("y")
	if code := c.wait(); code != 53 {
		t.Fatal(code, c.screen())
	}
}

func TestAdminRepositoryHubPolicyOperations(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	for _, alias := range []string{"a", "b"} {
		installPluginFixtureIn(t, env, alias, "admin-suite")
	}
	hubRun(t, bin, env, dir, "plugin", "disable", "admin-suite@b")
	for _, alias := range []string{"a", "b"} {
		c := newConsoleIn(t, dir, bin, env, "repository")
		c.expect("Repositories")
		c.send("/Add policy Locator\r")
		c.expect("Select policy Locator")
		c.send("/admin-suite@" + alias + "/clone\r")
		c.expect("Policy position")
		c.send("\r")
		c.expect("Review repository change")
		c.send("y")
		c.expect("work: policy now")
		c.expect("1  admin-suite@a/clone")
		if alias == "b" {
			c.expect("package-disabled")
		}
		hubExit(c, "Repositories")
	}
	c := newConsoleIn(t, dir, bin, env, "repository")
	c.expect("Repositories")
	c.send("/Move policy Locator\x1b[B\r")
	c.expect("Select policy Locator")
	c.send("/admin-suite@b/clone\r")
	c.expect("Policy position")
	c.send("\r")
	c.expect("Review repository change")
	c.expect("--before admin-suite@a/clone")
	c.send("y")
	c.expect("work: policy now admin-suite@b/clone, admin-suite@a/clone")
	c.expect("1  admin-suite@b/clone")
	hubExit(c, "Repositories")
	before := adminTree(t, home)
	hubRun(t, bin, env, dir, "repository", "policy", "replace", "admin-suite@b/clone", "admin-suite@a/clone")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("hub/direct order differed")
	}
	c = newConsoleIn(t, dir, bin, env, "repository")
	c.expect("Repositories")
	c.send("/Replace policy order\r")
	c.expect("Policy order")
	c.send("/admin-suite@a/clone\r")
	c.expect("Selected: admin-suite@a/clone")
	c.send("/admin-suite@b/clone\r")
	c.expect("Selected: admin-suite@a/clone, admin-suite@b/clone")
	c.send("/Review selected entries\r")
	c.expect("Review repository change")
	c.send("y")
	c.expect("work: policy now admin-suite@a/clone, admin-suite@b/clone")
	c.expect("1  admin-suite@a/clone")
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
	installPluginFixtureIn(t, env, "visible", "admin-suite")
	hubRun(t, bin, env, dir, "repository", "policy", "add", "admin-suite@visible/clone")
	hubRun(t, bin, env, repo, "convention", "set", "admin-team")
	before := adminTree(t, home)
	for _, tc := range []struct{ hub, action, title, result string }{
		{"repository", "List policy", "Repository policy", "admin-suite@visible/clone  (available)"},
		{"repository", "List installed Locators", "Installed Locators", "admin-suite@visible/clone"},
		{"repository", "List search roots", "Repository search roots", "no repository search roots"},
		{"convention", "Inspect remembered convention", "Remembered convention", "work: convention admin-suite@visible/admin-team"},
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

// The manager lists installed plugins by their composed identity, toggles one
// without leaving the session, and reviews removal with its policy dependency.
func TestAdminPluginManagerToggleAndRemoval(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	installPluginFixtureIn(t, env, "a", "admin-suite")
	hubRun(t, bin, env, dir, "repository", "policy", "add", "admin-suite@a/clone")
	c := newConsoleIn(t, dir, bin, env, "plugin")
	managerScreen(c, "Installed (1)", "admin-suite@a", "Enabled")
	if screen := c.screen(); strings.Contains(screen, "local-pinned") || strings.Contains(screen, "Install plugin") || strings.Contains(screen, "Official") {
		t.Fatal("the manager still advertises retired destinations:\n" + screen)
	}
	c.send(" ")
	managerScreen(c, "admin-suite@a disabled.", "Disabled")
	before := adminTree(t, home)
	hubRun(t, bin, env, dir, "plugin", "disable", "admin-suite@a")
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("hub/direct disable differed")
	}
	c.send("\r")
	managerScreen(c, "Plugins › admin-suite@a", "Remove")
	c.send("\x1b[B\r")
	managerScreen(c, "Review removal", "admin-suite@a/clone")
	c.send("\x1b[B\x1b[B\r")
	managerScreen(c, "Authorize the listed", "Review removal")
	c.send("\x1b[A \x1b[B\r")
	managerScreen(c, "Removed admin-suite@a.", "Installed (0)")
	managerExit(c)
}
