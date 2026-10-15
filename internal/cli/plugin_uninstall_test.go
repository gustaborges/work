package cli

import (
	"bytes"
	"github.com/gustaborges/work/internal/diag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminUninstallGrammar(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	t.Setenv("WORK_HOME", home)
	for _, args := range [][]string{{}, {"--yes"}, {"a"}, {"a", "--yes", "--json"}, {"a/clone", "--yes"}, {"a", "--all", "--yes"}} {
		cmd := newRootCmd()
		var out, ui bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&ui)
		cmd.SetArgs(append([]string{"plugin", "uninstall"}, args...))
		if err := cmd.Execute(); diag.ExitCode(err) != 2 || out.Len() != 0 {
			t.Fatal(args, err, out.String())
		}
		if _, err := os.Stat(home); !os.IsNotExist(err) {
			t.Fatal("invalid grammar created state", args, err)
		}
	}
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"plugin", "uninstall", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"PLUGIN", "--yes", "--remove-policy-references"} {
		if !strings.Contains(out.String(), word) {
			t.Fatal(word, out.String())
		}
	}
}
