package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/present/brand"
	"github.com/gustaborges/work/internal/present/theme"
)

// Command groups for `work --help`. The order here is the render order; a group
// with no available command is not printed (contracts/cli-help.md, SC-005 — zero
// empty groups). Future commands attach by setting their GroupID to one of these
// (data-model.md §6).
const (
	groupDaily  = "daily"
	groupInWork = "in-work"
	groupAdmin  = "admin"
	groupSetup  = "setup"
)

var helpGroups = []*cobra.Group{
	{ID: groupDaily, Title: "Daily Commands"},
	{ID: groupInWork, Title: "Inside a Work"},
	{ID: groupAdmin, Title: "Administration"},
	{ID: groupSetup, Title: "Setup"},
}

// installHelp registers the command groups, assigns every command a GroupID so
// nothing renders ungrouped, and installs the custom help renderer. The command
// inventory is Cobra's live tree — there is no hand-written command list, so a
// command that is not registered cannot appear (contracts/cli-help.md,
// research R15).
func installHelp(root *cobra.Command) {
	for _, g := range helpGroups {
		root.AddGroup(g)
	}

	// Cobra's auto-generated helpers: `completion` is useful setup plumbing and
	// stays visible under Setup; `help` is hidden so it never renders as a
	// command line (the -h/--help flag and the direction footer cover it).
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()
	for _, c := range root.Commands() {
		switch c.Name() {
		case "completion":
			c.GroupID = groupSetup
		case "help":
			c.Hidden = true
		}
	}

	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c != root {
			defaultHelp(c, args)
			return
		}
		renderRootHelp(c)
	})
}

// renderRootHelp writes the `work --help` view: the compact brand header, the
// usage line, one block per non-empty group in declared order, the local flags,
// and the per-command direction footer. It always exits via Cobra with code 0.
// Colour (group titles in Primary) is used only when the output stream is a
// colour-capable TTY; piped output carries no escape sequences (FR-025).
func renderRootHelp(root *cobra.Command) {
	out := root.OutOrStdout()
	probe := theme.Detect(out, root.InOrStdin())
	th := theme.New(probe, true)

	fmt.Fprint(out, brand.Header(probe.ColorEnabled))
	fmt.Fprintf(out, "\nUsage:\n  %s [command]\n", root.CommandPath())

	width := commandNameWidth(root)
	for _, g := range helpGroups {
		cmds := commandsInGroup(root, g.ID)
		if len(cmds) == 0 {
			continue
		}
		fmt.Fprintf(out, "\n%s:\n", th.Primary.Render(g.Title))
		for _, c := range cmds {
			fmt.Fprintf(out, "  %-*s  %s\n", width, c.Name(), c.Short)
		}
	}

	root.InitDefaultHelpFlag()
	if usages := strings.TrimRight(root.LocalFlags().FlagUsages(), "\n"); usages != "" {
		fmt.Fprintf(out, "\nFlags:\n%s\n", usages)
	}

	fmt.Fprintf(out, "\nUse \"%s [command] --help\" for more information about a command.\n", root.CommandPath())
}

// commandsInGroup returns the available (non-hidden, non-deprecated) subcommands
// of root whose GroupID is id, sorted by name.
func commandsInGroup(root *cobra.Command, id string) []*cobra.Command {
	var cmds []*cobra.Command
	for _, c := range root.Commands() {
		if c.GroupID == id && c.IsAvailableCommand() {
			cmds = append(cmds, c)
		}
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name() < cmds[j].Name() })
	return cmds
}

// commandNameWidth is the column width for the command name in every group
// block: the longest available subcommand name.
func commandNameWidth(root *cobra.Command) int {
	w := 0
	for _, c := range root.Commands() {
		if c.IsAvailableCommand() && len(c.Name()) > w {
			w = len(c.Name())
		}
	}
	return w
}
