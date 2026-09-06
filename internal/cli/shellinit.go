package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/shellintegration"
)

// newShellInitCmd registers `work shell-init <shell>`. It prints the wrapper
// snippet for the named shell to stdout and exits 0. It is read-only.
func newShellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "shell-init <bash|zsh|fish|powershell>",
		Short:         "Print the shell snippet that repositions the terminal after `work start`",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return diag.Newf(diag.Usage,
					"specify a shell: %s", strings.Join(shellintegration.Supported, ", "))
			}
			snippet, err := shellintegration.Snippet(args[0])
			if err != nil {
				return err
			}
			cmd.OutOrStdout().Write([]byte(snippet))
			return nil
		},
	}
}
