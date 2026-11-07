//go:build unix

package integration

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestAdminUninstallInteractiveDependencyReview(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	installPluginFixtureIn(t, env, "a", "admin-suite")
	for _, args := range [][]string{{"repository", "policy", "add", "admin-suite@a/clone"}} {
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, "plugin", "uninstall", "admin-suite@a", "--yes")
	c.expect("Remove policy references")
	c.expect("admin-suite@a/clone")
	c.send("n")
	if code := c.wait(); code != 20 {
		t.Fatal(code, c.screen())
	}
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("dependency refusal changed home")
	}
	c = newConsoleIn(t, dir, bin, env, "plugin", "uninstall", "admin-suite@a")
	c.expect("Remove policy references")
	c.expect("admin-suite@a/clone")
	c.send("y")
	c.expect("Remove plugins")
	c.expect("admin-team")
	c.send("n")
	if code := c.wait(); code != 20 {
		t.Fatal(code, c.screen())
	}
	if strings.Contains(c.screen(), "work: removed") || !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("uninstall refusal changed home")
	}
	c = newConsoleIn(t, dir, bin, env, "plugin", "uninstall", "admin-suite@a")
	c.expect("Remove policy references")
	c.send("y")
	c.expect("Remove plugins")
	c.send("y")
	c.expect("work: removed admin-suite@a")
	if code := c.wait(); code != 0 {
		t.Fatal(code, c.screen())
	}
}
