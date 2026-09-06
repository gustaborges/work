package cli

import (
	"github.com/spf13/cobra"
)

// newShellInitCmd registers `work shell-init <shell>`. It prints a shell
// wrapper snippet to stdout; the implementation lands in a later slice.
func newShellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "shell-init <bash|zsh|fish|powershell>",
		Short:         "Print the shell snippet that repositions the terminal after `work start`",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
