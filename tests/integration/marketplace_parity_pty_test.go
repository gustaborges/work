//go:build unix

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

func installedState(t *testing.T, bin string, env []string, dir string) map[string]bool {
	t.Helper()
	raw := hubRun(t, bin, env, dir, "plugin", "list", "--json")
	var env2 struct {
		Plugins []struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		} `json:"installed"`
	}
	if err := json.Unmarshal([]byte(raw), &env2); err != nil {
		t.Fatal(err, raw)
	}
	out := map[string]bool{}
	for _, p := range env2.Plugins {
		out[p.ID] = p.Enabled
	}
	return out
}

// P07/P08/P09/P12: browse one source's offers, add through a reviewed
// selection, toggle several at once and remove them as one reviewed set, with
// the same effective state the direct commands produce.
func TestMarketplaceManagerParityAddEnableDisableRemove(t *testing.T) {
	bin := buildWorkBin(t)
	env, dir, _ := ptyEnv(t)
	local := marketplaces.Copy(t, "multiple")
	hubRun(t, bin, env, dir, "plugin", "marketplace", "add", local)

	c := newConsoleIn(t, dir, bin, env, "plugin", "marketplace")
	managerScreen(c, "demo")
	// Source details → Browse available plugins (one-source catalog).
	c.send("\x1b[B\r")
	managerScreen(c, "Marketplaces › Demo Plugins (demo)", "Browse available plugins")
	c.send("\r")
	managerScreen(c, "Available plugins", "alpha@demo", "beta@demo", "gamma@demo", "3 available · 0 installed")
	c.send("\r")
	managerScreen(c, "alpha@demo", "Add")
	c.send("\r")
	managerScreen(c, "Review installation", "Add alpha@demo")
	c.send("\x1b[B\r")
	managerScreen(c, "installed alpha@demo.")
	if got := installedState(t, bin, env, dir); len(got) != 1 || !got["alpha@demo"] {
		t.Fatal("manager add differs from direct add", got)
	}
	managerClose(c, "\x03")

	hubRun(t, bin, env, dir, "plugin", "add", "beta@demo")
	c = newConsoleIn(t, dir, bin, env, "plugin")
	managerScreen(c, "Installed (2)", "alpha@demo", "beta@demo")
	c.send(" \x1b[B ")
	managerScreen(c, "2 selected")
	c.send("a")
	managerScreen(c, "Applies to 2 selected")
	c.send("\x1b[B\r") // Disable
	managerScreen(c, "Disabled 2 plugin(s).")
	got := installedState(t, bin, env, dir)
	if got["alpha@demo"] || got["beta@demo"] {
		t.Fatal("multi-target disable differs from direct disable", got)
	}
	c.send("a")
	managerScreen(c, "Applies to 2 selected", "Remove")
	c.send("\x1b[B\x1b[B\x1b[B\x1b[B\r")
	managerScreen(c, "Review removal", "2 plugin(s)", "marketplace source is untouched")
	c.send("\x1b[B\r")
	managerScreen(c, "Removed 2 plugin(s).", "Installed (0)")
	managerExit(c)
	if got := installedState(t, bin, env, dir); len(got) != 0 {
		t.Fatal(got)
	}
	if out := hubRun(t, bin, env, dir, "plugin", "marketplace", "list"); !strings.Contains(out, "demo") {
		t.Fatal("removing plugins removed the source")
	}
}
