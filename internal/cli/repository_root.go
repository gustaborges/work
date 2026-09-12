package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/repoconfig"
)

// newRepositoryRootCmd registers `work repository root list|add|remove|replace`
// (contracts/cli-work-repository.md, resolution-policy.md).
func newRepositoryRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "root",
		Short: "Manage repository search roots",
	}
	cmd.AddCommand(
		newRepositoryRootListCmd(),
		newRepositoryRootAddCmd(),
		newRepositoryRootRemoveCmd(),
		newRepositoryRootReplaceCmd(),
	)
	return cmd
}

func newRepositoryRootListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List configured repository search roots",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, _, err := loadRepositoryContext()
			if err != nil {
				return err
			}
			roots := repoconfig.ListRoots(cfg)

			if cmd.Flags().Changed("json") {
				return printJSON(cmd, map[string]any{"roots": roots})
			}
			if len(roots) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: no repository search roots configured")
				return nil
			}
			out := cmd.OutOrStdout()
			for _, r := range roots {
				fmt.Fprintln(out, r)
			}
			return nil
		},
	}
}

func newRepositoryRootAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "add <PATH...>",
		Short:         "Add one or more repository search roots",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutateRoots(cmd, args, repoconfig.AddRoots)
		},
	}
}

func newRepositoryRootRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "remove <PATH...>",
		Short:         "Remove one or more repository search roots",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutateRoots(cmd, args, repoconfig.RemoveRoots)
		},
	}
}

func newRepositoryRootReplaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "replace <PATH...>",
		Short:         "Replace the whole set of repository search roots",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutateRoots(cmd, args, repoconfig.ReplaceRoots)
		},
	}
}

// mutateRoots runs op against the loaded config and, on success, saves it and
// prints the one stable `work: roots now …` line (resolution-policy.md
// §Mutation output). A rejected mutation writes nothing.
func mutateRoots(cmd *cobra.Command, paths []string, op func(*config.Config, []string) error) error {
	if cmd.Flags().Changed("json") {
		return diag.New(diag.Usage, "--json is not accepted on a `work repository root` mutation")
	}
	home, cfg, _, err := loadRepositoryContext()
	if err != nil {
		return err
	}
	if err := op(cfg, paths); err != nil {
		return err
	}
	if err := saveRepositoryConfig(home, cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "work: roots now %s\n", joinOrNone(cfg.RepositoryRoots))
	return nil
}

// joinOrNone renders a stable mutation-result list, e.g. "a, b" or "(none)".
func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

// printJSON writes v as indented JSON to cmd's stdout. `--json` output is
// pure and always exits 0, even for an empty collection.
func printJSON(cmd *cobra.Command, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
