package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gustaborges/work/internal/diag"
)

// Every malformed or retired form fails as usage (exit 2) before any state,
// source or network is touched.
func TestPluginSelectorGrammarFailsBeforeAnyState(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	t.Setenv("WORK_HOME", home)
	cases := map[string][][]string{
		"remove": {
			{}, {"--yes"}, {"a"}, {"a@m", "--yes", "--json", "x"}, {"a/clone", "--yes"}, {"a@m", "--all", "--yes"},
			{"a@m@n"}, {"a b@m"}, {"a@m", "b"}, {"a@m", "-m", "m"}, {"a", "b@m", "-m", "m"}, {"./plugin", "-m", "m"},
		},
		"enable":  {{}, {"a"}, {"a@m", "-m", "m"}, {"a@m", "b"}},
		"disable": {{}, {"a"}, {"a@m", "-m", "m"}},
		"add": {
			{}, {"a"}, {"a@m", "b@m"}, {"a@m", "-m", "m"}, {"./local/dir"}, {"https://example.test/p.git"},
			{"plugin.json"}, {"a@m", "--as", "x"}, {"a@m", "--link"}, {"a@m", "--embedded"}, {"a@m@n"},
		},
	}
	for sub, list := range cases {
		for _, args := range list {
			cmd := newRootCmd()
			var out, ui bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&ui)
			cmd.SetArgs(append([]string{"plugin", sub}, args...))
			if err := cmd.Execute(); diag.ExitCode(err) != 2 || out.Len() != 0 {
				t.Fatalf("plugin %s %v: exit %d (%v), stdout %q", sub, args, diag.ExitCode(err), err, out.String())
			}
			if _, err := os.Stat(home); !os.IsNotExist(err) {
				t.Fatalf("plugin %s %v created state: %v", sub, args, err)
			}
		}
	}
}

func TestRetiredPluginCommandsAreAbsent(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	t.Setenv("WORK_HOME", home)
	for _, args := range [][]string{
		{"plugin", "install", "a@m"}, {"plugin", "uninstall", "a@m"}, {"marketplace", "add", "src"}, {"plugin", "update", "a@m"},
	} {
		cmd := newRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); diag.ExitCode(err) != 2 {
			t.Fatalf("%v: exit %d (%v)", args, diag.ExitCode(err), err)
		}
		if _, err := os.Stat(home); !os.IsNotExist(err) {
			t.Fatalf("%v created state", args)
		}
	}
}

func TestPluginHelpShowsTheQualifiedGrammar(t *testing.T) {
	for sub, words := range map[string][]string{
		"add":    {"PLUGIN[@MARKETPLACE]", "--marketplace", "--yes", "--remove-policy-references"},
		"remove": {"PLUGIN[@MARKETPLACE]", "--marketplace", "--yes", "--remove-policy-references"},
		"list":   {"--marketplace", "--available"},
		"enable": {"PLUGIN[@MARKETPLACE]", "--marketplace"},
	} {
		cmd := newRootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"plugin", sub, "--help"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, w := range words {
			if !strings.Contains(out.String(), w) {
				t.Errorf("plugin %s --help lacks %q:\n%s", sub, w, out.String())
			}
		}
	}
}
