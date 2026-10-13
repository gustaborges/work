package cli

import (
	"context"
	"fmt"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/spf13/cobra"
	"strings"
)

func runPluginHub(cmd *cobra.Command) error {
	for {
		_, snapshot, err := readAdministration(cmd.Context())
		if err != nil {
			return err
		}
		var description strings.Builder
		packages := pluginadmin.Inventory(snapshot)
		aliases := []string{}
		if len(packages) == 0 {
			description.WriteString("No plugins installed. Choose an explicit source or bundled installation.\n")
		}
		for _, p := range packages {
			fmt.Fprintf(&description, "%s  %s  enabled: %t  %s\n", p.Alias, updateValue(p.Version), p.Enabled, p.Origin)
			aliases = append(aliases, p.Alias)
		}
		action, err := hubChoice(cmd, "Plugins", description.String(), []present.Option[string]{
			{Value: "list", Primary: "List plugins", Secondary: "work plugin list [--json]"},
			{Value: "install", Primary: "Install from source", Secondary: "work plugin install SOURCE [--as ALIAS] [--link]"},
			{Value: "embedded", Primary: "Install bundled package", Secondary: "work plugin install --embedded"},
			{Value: "enable", Primary: "Enable plugins", Secondary: "work plugin enable ALIAS..."},
			{Value: "disable", Primary: "Disable plugins", Secondary: "work plugin disable ALIAS..."},
			{Value: "check", Primary: "Check updates", Secondary: "work plugin update --check [ALIAS...] [--json]"},
			{Value: "update", Primary: "Apply updates", Secondary: "work plugin update ALIAS... --yes [--remove-policy-references]"},
			{Value: "uninstall", Primary: "Uninstall plugins", Secondary: "work plugin uninstall ALIAS... --yes [--remove-policy-references]"},
			{Value: "", Primary: "Leave unchanged / Exit"},
		})
		if err != nil {
			return err
		}
		switch action {
		case "":
			return nil
		case "list":
			err = hubDirect(cmd, "plugin", "list")
		case "embedded":
			var yes bool
			yes, err = present.Confirm(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, present.ConfirmSpec{Title: "Install bundled package", Impact: "work plugin install --embedded\nInstalls offline and preserves explicit repository policy.", Accept: "Install", Reject: "Back"})
			if err == nil && yes {
				err = hubDirect(cmd, "plugin", "install", "--embedded")
			}
		case "install":
			err = hubInstall(cmd)
		default:
			var targets []string
			targets, err = hubTargets(cmd, "Select plugins to "+action, aliases)
			if err == nil && len(targets) > 0 {
				args := []string{"plugin", action}
				if action == "check" {
					args = []string{"plugin", "update", "--check"}
				}
				args = append(args, targets...)
				err = hubDirect(cmd, args...)
			}
		}
		if err != nil {
			return err
		}
	}
}
func hubInstall(cmd *cobra.Command) error {
	source, err := hubInput(cmd, present.InputSpec{Title: "Plugin source", Description: "Local directory or remote source. Source credentials are omitted from receipts.", Secret: true, Validate: func(_ context.Context, v string) error { return validateHubSource(v) }})
	if err != nil {
		return err
	}
	alias, err := hubInput(cmd, present.InputSpec{Title: "Plugin alias", Description: "Optional; leave empty to use the manifest name.", Validate: func(_ context.Context, v string) error {
		if v == "" {
			return nil
		}
		return plugin.ValidateAlias(v)
	}})
	if err != nil {
		return err
	}
	mode, err := hubChoice(cmd, "Installation mode", "Linked installation is for a local source managed outside Work.", []present.Option[string]{{Value: "pinned", Primary: "Copy fixed source"}, {Value: "linked", Primary: "Link local source"}, {Value: "", Primary: "Back"}})
	if err != nil || mode == "" {
		return err
	}
	args := []string{"plugin", "install", source}
	if alias != "" {
		args = append(args, "--as", alias)
	}
	if mode == "linked" {
		args = append(args, "--link")
	}
	return hubDirect(cmd, args...)
}
func validateHubSource(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("enter a source directory or URL")
	}
	return nil
}
