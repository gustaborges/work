package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// declaredGroupIDs is the set help.go registers; the correspondence test holds
// the command tree to exactly these (contracts/cli-help.md, SC-005).
func declaredGroupIDs() map[string]bool {
	m := map[string]bool{}
	for _, g := range helpGroups {
		m[g.ID] = true
	}
	return m
}

func renderHelp(t *testing.T) string {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("`work --help` returned an error: %v", err)
	}
	return out.String()
}

func TestHelpEveryCommandIsGrouped(t *testing.T) {
	declared := declaredGroupIDs()
	for _, c := range newRootCmd().Commands() {
		if c.Hidden || !c.IsAvailableCommand() {
			continue
		}
		if c.GroupID == "" {
			t.Errorf("command %q has no GroupID", c.Name())
			continue
		}
		if !declared[c.GroupID] {
			t.Errorf("command %q has GroupID %q, not one of the declared groups", c.Name(), c.GroupID)
		}
	}
}

func TestHelpRenderedGroupsAreNonEmptyAndOrdered(t *testing.T) {
	help := renderHelp(t)

	// A group title renders only with ≥1 command line beneath it, and titles
	// appear in the declared order.
	lastIdx := -1
	cmdLineRe := regexp.MustCompile(`^  \S`)
	for _, g := range helpGroups {
		members := commandsInGroup(newRootCmd(), g.ID)
		idx := strings.Index(help, g.Title+":\n")
		if len(members) == 0 {
			if idx != -1 {
				t.Errorf("empty group %q rendered a title", g.Title)
			}
			continue
		}
		if idx == -1 {
			t.Errorf("non-empty group %q did not render", g.Title)
			continue
		}
		if idx < lastIdx {
			t.Errorf("group %q rendered out of declared order", g.Title)
		}
		lastIdx = idx

		// The line immediately after the title is a command line.
		after := help[idx+len(g.Title)+2:]
		if !cmdLineRe.MatchString(after) {
			t.Errorf("group %q title is not followed by a command line:\n%s", g.Title, after)
		}
	}
}

func TestHelpCommandLineCountMatchesInventory(t *testing.T) {
	help := renderHelp(t)

	var want []string
	for _, c := range newRootCmd().Commands() {
		if c.IsAvailableCommand() {
			want = append(want, c.Name())
		}
	}

	for _, name := range want {
		re := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(name) + `\s`)
		if n := len(re.FindAllString(help, -1)); n != 1 {
			t.Errorf("command %q appears %d times in help, want exactly 1", name, n)
		}
	}

	// Total command lines == inventory size (no extra, no missing).
	lineRe := regexp.MustCompile(`(?m)^  [a-z][a-z-]*\s{2,}\S`)
	if got := len(lineRe.FindAllString(help, -1)); got != len(want) {
		t.Errorf("help has %d command lines, inventory has %d\n%s", got, len(want), help)
	}
}

func TestHelpIsPlainAndExitZeroWhenPiped(t *testing.T) {
	out, errb, code := runWork(t, "--help")
	if code != 0 {
		t.Fatalf("`work --help` exit = %d, want 0\n%s", code, errb)
	}
	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("piped `work --help` contains an escape sequence:\n%q", out)
	}
	if !strings.Contains(out, "WORK") || !strings.Contains(out, "Daily Commands:") {
		t.Errorf("piped help missing brand/group:\n%s", out)
	}
}

func TestHelpForSubcommandIsDefault(t *testing.T) {
	// `work start --help` still uses Cobra's default renderer, not the root view.
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"start", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("`work start --help`: %v", err)
	}
	s := out.String()
	if strings.Contains(s, "Daily Commands:") {
		t.Errorf("subcommand help leaked the root group view:\n%s", s)
	}
	if !strings.Contains(s, "Usage:") || !strings.Contains(s, "start") {
		t.Errorf("subcommand help missing its own usage:\n%s", s)
	}
}

func TestHelpCompletionGroupedHelpHidden(t *testing.T) {
	root := newRootCmd()
	var completion, help *cobra.Command
	for _, c := range root.Commands() {
		switch c.Name() {
		case "completion":
			completion = c
		case "help":
			help = c
		}
	}
	if completion == nil || completion.GroupID != groupSetup {
		t.Errorf("completion command is not in the Setup group: %+v", completion)
	}
	if help == nil || !help.Hidden {
		t.Errorf("auto help command should be hidden: %+v", help)
	}
}
