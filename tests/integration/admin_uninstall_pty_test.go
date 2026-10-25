//go:build unix

package integration

import (
	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestAdminUninstallInteractiveDependencyReview(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	src := fixtures.Prepare(t, "admin-suite")
	for _, args := range [][]string{{"plugin", "install", src, "--as", "a"}, {"repository", "policy", "add", "a/clone"}} {
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, "plugin", "uninstall", "a", "--yes")
	c.expect("Remove policy references")
	c.expect("a/clone")
	c.send("n")
	if code := c.wait(); code != 20 {
		t.Fatal(code, c.screen())
	}
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("dependency refusal changed home")
	}
	c = newConsoleIn(t, dir, bin, env, "plugin", "uninstall", "a")
	c.expect("Remove policy references")
	c.expect("a/clone")
	c.send("y")
	c.expect("Uninstall plugins")
	c.expect("admin-team")
	c.send("n")
	if code := c.wait(); code != 20 {
		t.Fatal(code, c.screen())
	}
	if strings.Contains(c.screen(), "work: uninstalled") || !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("uninstall refusal changed home")
	}
	c = newConsoleIn(t, dir, bin, env, "plugin", "uninstall", "a")
	c.expect("Remove policy references")
	c.send("y")
	c.expect("Uninstall plugins")
	c.send("y")
	c.expect("work: uninstalled a")
	if code := c.wait(); code != 0 {
		t.Fatal(code, c.screen())
	}
}
