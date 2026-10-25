package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/workhome"
)

// newPluginInstallCmd registers `work plugin install <SOURCE> [--link]
// [--as <ALIAS>]` (contracts/cli-work-plugin.md).
func newPluginInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [SOURCE] [--embedded]",
		Short: "Install a plugin from a source or the offline bundled package",
		Args: func(cmd *cobra.Command, args []string) error {
			embedded, _ := cmd.Flags().GetBool("embedded")
			if embedded {
				if len(args) != 0 || cmd.Flags().Changed("link") || cmd.Flags().Changed("as") {
					return diag.New(diag.Usage, "--embedded accepts no SOURCE, --link or --as")
				}
				return nil
			}
			if len(args) != 1 {
				return diag.New(diag.Usage, "missing SOURCE argument: run `work plugin install <source> [--link] [--as <alias>]`")
			}
			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on `work plugin install` (it is a mutation)")
			}
			embedded, _ := cmd.Flags().GetBool("embedded")
			if embedded {
				home, err := workhome.Resolve()
				if err != nil {
					return err
				}
				if err = mutationPreflight(cmd, home); err != nil {
					return err
				}
				res, err := pluginadmin.InstallEmbedded(cmd.Context(), home)
				if err != nil {
					return err
				}

				printPluginInstall(cmd, res)
				return nil
			}
			link, _ := cmd.Flags().GetBool("link")
			as, _ := cmd.Flags().GetString("as")
			source := strings.TrimSpace(args[0])
			opts := plugininstall.Options{Link: link, Alias: as}
			if err := plugininstall.ValidateRequest(source, opts); err != nil {
				return err
			}

			home, err := workhome.Resolve()
			if err != nil {
				return err
			}
			if err = mutationPreflight(cmd, home); err != nil {
				return err
			}
			prepared, err := plugininstall.Prepare(cmd.Context(), source, opts)
			if err != nil {
				return err
			}
			defer prepared.Close()
			res, err := pluginadmin.InstallPrepared(cmd.Context(), home, prepared)
			if err != nil {
				return err
			}

			printPluginInstall(cmd, res)
			return nil
		},
	}
	cmd.Flags().Bool("embedded", false, "install the bundled reference package offline without adding policy entries")
	cmd.Flags().Bool("link", false, "install a local SOURCE by reference (a symlink) instead of copying")
	cmd.Flags().String("as", "", "override the default alias (the manifest's name)")
	return cmd
}

func printPluginInstall(cmd *cobra.Command, res plugininstall.Result) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "work: installed %s (%s)\n", res.Package.Alias, res.Package.Origin)
	if len(res.Components) > 0 {
		parts := make([]string, len(res.Components))
		for i, c := range res.Components {
			parts[i] = fmt.Sprintf("%s (%s)", c.Name, c.Role)
		}
		fmt.Fprintf(out, "work: components: %s\n", strings.Join(parts, ", "))
	}
}
