//go:build unix

package integration

import (
	"os/exec"
	"testing"

	"github.com/creack/pty"
	"github.com/gustaborges/work/tests/fixtures/marketplaces"
)

// S01/S03/S05: the manager stays usable at the reduced logical viewport, after
// resize down and up, and in no-color and dumb terminals.
func TestMarketplaceManagerGeometryAndTerminalCapabilities(t *testing.T) {
	bin := buildWorkBin(t)
	for name, extra := range map[string][]string{"color": nil, "no-color": {"NO_COLOR=1"}, "dumb": {"TERM=dumb"}} {
		t.Run(name, func(t *testing.T) {
			env, dir, _ := ptyEnv(t)
			env = append(env, extra...)
			local := marketplaces.Copy(t, "multiple")
			hubRun(t, bin, env, dir, "plugin", "marketplace", "add", local)
			hubRun(t, bin, env, dir, "plugin", "add", "alpha@demo")

			for _, size := range []pty.Winsize{{Rows: 18, Cols: 60}, {Rows: 30, Cols: 100}} {
				cmd := exec.Command(bin, "plugin")
				cmd.Env, cmd.Dir = env, dir
				c := startConsole(t, size, cmd)
				managerScreen(c, "Installed (1)", "alpha@demo", "Esc close")
				assertNoLineExceeds(t, c.screen(), int(size.Cols))
				c.send("\r")
				managerScreen(c, "alpha@demo", "Remove")
				assertNoLineExceeds(t, c.screen(), int(size.Cols))
				c.send("\x1b")
				managerScreen(c, "Space mark")
				c.send("\t")
				managerScreen(c, "Marketplaces (", "Included with Work", "Local directory")
				assertNoLineExceeds(t, c.screen(), int(size.Cols))
				managerExit(c)
			}
			// Dynamic resize down and up must leave the session responsive.
			cmd := exec.Command(bin, "plugin")
			cmd.Env, cmd.Dir = env, dir
			c := startConsole(t, pty.Winsize{Rows: 30, Cols: 100}, cmd)
			managerScreen(c, "alpha@demo")
			for _, size := range []pty.Winsize{{Rows: 18, Cols: 60}, {Rows: 30, Cols: 100}} {
				if err := pty.Setsize(c.f, &size); err != nil {
					t.Fatal(err)
				}
			}
			c.send("/alpha")
			c.expect("/ alpha")
			c.send("\x1b")
			managerScreen(c, "/ Search…")
			managerExit(c)
		})
	}
}
