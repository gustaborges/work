//go:build unix

package integration

import (
	"strings"
	"testing"

	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

// Both entry points open the same persistent manager: one terminal session,
// one acknowledgment, no per-action transcript.
func TestMarketplaceManagerStartsOnMarketplacesAndRegistersSource(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, _ := ptyEnv(t)
	local := marketplaces.Copy(t, "multiple")

	c := newConsoleIn(t, dir, bin, env, "plugin", "marketplace")
	managerScreen(c, "Marketplaces (", "(builtin)", "Included with Work")
	if strings.Contains(c.screen(), "[Installed") {
		t.Fatal("marketplace command must start on Marketplaces:\n" + c.screen())
	}
	c.send("a")
	managerScreen(c, "Add marketplace")
	c.send("\r")
	managerScreen(c, "Source:")
	c.send(local + "\r\r")
	managerScreen(c, "Review marketplace", "Register demo", "installs nothing")
	c.send("\x1b[B\r")
	managerScreen(c, "registered demo.", "Demo Plugins")
	managerExit(c)

	out := hubRun(t, bin, env, dir, "plugin", "marketplace", "list")
	if !strings.Contains(out, "demo") {
		t.Fatal(out)
	}
}

func TestMarketplaceManagerBackAndAbandonedReviewKeepSession(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, _ := ptyEnv(t)
	local := marketplaces.Copy(t, "multiple")

	c := newConsoleIn(t, dir, bin, env, "plugin", "marketplace")
	managerScreen(c, "(builtin)")
	c.send("a")
	managerScreen(c, "Add marketplace")
	c.send("\r")
	managerScreen(c, "Source:")
	c.send(local + "\r\r")
	managerScreen(c, "Review marketplace")
	c.send("\x1b")
	managerScreen(c, "Review registration")
	c.send("\x1b")
	managerScreen(c, "Marketplaces (", "Space mark")
	managerExit(c)
	if got := hubRun(t, bin, env, dir, "plugin", "marketplace", "list"); strings.Contains(got, "demo") {
		t.Fatal("abandoned review registered a source:\n" + got)
	}
}
