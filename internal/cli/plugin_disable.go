package cli

import "github.com/spf13/cobra"

func newPluginDisableCmd() *cobra.Command { return newPluginToggleCmd(false) }
