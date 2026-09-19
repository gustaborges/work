package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugininstall"
	"github.com/gustaborges/work/internal/registry"
)

// newPluginInstallCmd registers `work plugin install <SOURCE> [--link]
// [--as <ALIAS>]` (contracts/cli-work-plugin.md).
func newPluginInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <SOURCE>",
		Short: "Install a plugin package from a local path or a remote source",
		Args: func(cmd *cobra.Command, args []string) error {
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
			link, _ := cmd.Flags().GetBool("link")
			as, _ := cmd.Flags().GetString("as")

			home, reg, err := loadPluginContext()
			if err != nil {
				return err
			}

			res, err := plugininstall.Install(home.PluginsDir(), reg, strings.TrimSpace(args[0]), plugininstall.Options{
				Link:  link,
				Alias: as,
			})
			if err != nil {
				return err
			}
			if err := registry.Save(home.RegistryFile(), reg); err != nil {
				return diag.Wrap(diag.PluginInstallFailed, err, "cannot write the component registry")
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "work: installed %s (%s)\n", res.Package.Alias, res.Package.Origin)
			if len(res.Components) > 0 {
				parts := make([]string, len(res.Components))
				for i, c := range res.Components {
					parts[i] = fmt.Sprintf("%s (%s)", c.Name, c.Role)
				}
				fmt.Fprintf(out, "work: components: %s\n", strings.Join(parts, ", "))
			}
			return nil
		},
	}
	cmd.Flags().Bool("link", false, "install a local SOURCE by reference (a symlink) instead of copying")
	cmd.Flags().String("as", "", "override the default alias (the manifest's name)")
	return cmd
}
