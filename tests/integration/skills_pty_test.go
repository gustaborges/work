//go:build unix

package integration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsInstallConfirmation(t *testing.T) {
	bin := buildWorkBin(t)
	for _, scope := range []string{"local", "user"} {
		t.Run(scope, func(t *testing.T) {
			env, dir, _ := ptyEnv(t)
			userHome := filepath.Join(dir, "skill-user-home")
			env = append(env, "HOME="+userHome, "USERPROFILE="+userHome, "CLAUDE_CONFIG_DIR=")
			base := dir
			if scope == "user" {
				base = userHome
			}
			target := filepath.Join(base, ".agents", "skills", "work-cli-plugin-builder")
			c := newConsoleIn(t, dir, bin, env, "skills", "install", "--vendor=codex", "--scope="+scope)
			c.expect("Install Work skills")
			c.expect("work-cli-plugin-builder")
			c.send("n")
			if code := c.wait(); code != 20 {
				t.Fatal(code, c.screen())
			}
			if _, err := os.Stat(filepath.Join(base, ".agents")); !os.IsNotExist(err) {
				t.Fatal("cancel created target or coordination files", err)
			}
			c = newConsoleIn(t, dir, bin, env, "skills", "install", "--vendor=codex", "--scope="+scope)
			c.expect("Install Work skills")
			c.send("y")
			if code := c.wait(); code != 0 {
				t.Fatal(code, c.screen())
			}
			if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
