//go:build unix

package integration

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"

	fixtures "github.com/gustaborges/work/tests/fixtures/plugins"
)

func TestAdminUpdateInteractiveReviewAndSelection(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, home := ptyEnv(t)
	source := fixtures.Prepare(t, "admin-suite")
	cmd := exec.Command(bin, "plugin", "install", source, "--as", "a")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	fixtures.Revise(t, source, "reviewed-green", nil)
	before := adminTree(t, home)
	c := newConsoleIn(t, dir, bin, env, "plugin", "update", "a")
	c.expect("Apply plugin updates")
	c.expect("reviewed-green")
	// Freeze before changing the source: accepting must install reviewed-green.
	fixtures.Revise(t, source, "after-review", nil)
	c.send("y")
	c.expect("work: update a updated")
	if code := c.wait(); code != 0 {
		t.Fatalf("%d %s", code, c.screen())
	}
	cmd = exec.Command(bin, "plugin", "list", "--json")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "reviewed-green") || strings.Contains(string(out), "after-review") {
		t.Fatal(string(out), err)
	}
	if reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("accepted update did nothing")
	}
	before = adminTree(t, home)
	c = newConsoleIn(t, dir, bin, env, "plugin", "update")
	c.expect("Update plugins")
	c.send(" ")
	c.send("\r")
	c.expect("Apply plugin updates")
	c.expect("after-review")
	c.send("n")
	if code := c.wait(); code != 20 {
		t.Fatalf("%d %s", code, c.screen())
	}
	if !reflect.DeepEqual(before, adminTree(t, home)) {
		t.Fatal("declined update changed home")
	}
}
