package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconfig"
)

// newRepositoryPolicyCmd registers
// `work repository policy list|add|remove|move|replace`
// (contracts/cli-work-repository.md, resolution-policy.md).
func newRepositoryPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage the repository resolution policy",
	}
	cmd.AddCommand(
		newRepositoryPolicyListCmd(),
		newRepositoryPolicyAddCmd(),
		newRepositoryPolicyRemoveCmd(),
		newRepositoryPolicyMoveCmd(),
		newRepositoryPolicyReplaceCmd(),
	)
	return cmd
}

// newRepositoryLocatorCmd registers `work repository locator list`.
func newRepositoryLocatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "locator",
		Short: "List registered repository locators",
	}
	cmd.AddCommand(&cobra.Command{
		Use:           "list",
		Short:         "List every registered repository-locator component",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, reg, err := loadRepositoryContext()
			if err != nil {
				return err
			}
			locators := repoconfig.ListLocators(cfg, reg)

			if cmd.Flags().Changed("json") {
				type jsonLocator struct {
					Ref         string   `json:"ref"`
					DisplayName string   `json:"display_name"`
					Description string   `json:"description"`
					Accepts     []string `json:"accepts"`
					InPolicy    bool     `json:"in_policy"`
				}
				out := make([]jsonLocator, len(locators))
				for i, l := range locators {
					out[i] = jsonLocator{
						Ref: l.Ref, DisplayName: l.DisplayName, Description: l.Description,
						Accepts: nonNil(l.Accepts), InPolicy: l.InPolicy,
					}
				}
				return printJSON(cmd, map[string]any{"locators": out})
			}
			if len(locators) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: no repository locators are registered")
				return nil
			}
			w := cmd.OutOrStdout()
			for _, l := range locators {
				marker := ""
				if l.InPolicy {
					marker = "  (in policy)"
				}
				fmt.Fprintf(w, "%s  %s%s\n", l.Ref, l.DisplayName, marker)
			}
			return nil
		},
	})
	return cmd
}

func newRepositoryPolicyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "List the repository resolution policy in traversal order",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, reg, err := loadRepositoryContext()
			if err != nil {
				return err
			}
			entries := repoconfig.ListPolicy(cfg, reg)

			if cmd.Flags().Changed("json") {
				type jsonEntry struct {
					Ref       string `json:"ref"`
					Position  int    `json:"position"`
					Available bool   `json:"available"`
				}
				out := make([]jsonEntry, len(entries))
				for i, e := range entries {
					out[i] = jsonEntry{Ref: e.Ref, Position: e.Position, Available: e.Available}
				}
				return printJSON(cmd, map[string]any{"policy": out})
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: the repository resolution policy is empty")
				return nil
			}
			w := cmd.OutOrStdout()
			for _, e := range entries {
				state := "available"
				if !e.Available {
					state = "unavailable"
				}
				fmt.Fprintf(w, "%d  %s  (%s)\n", e.Position, e.Ref, state)
			}
			return nil
		},
	}
}

func newRepositoryPolicyAddCmd() *cobra.Command {
	var before, after string
	cmd := &cobra.Command{
		Use:           "add <LOCATOR>",
		Short:         "Add a locator to the resolution policy",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutatePolicy(cmd, func(cfg *config.Config, reg *registry.Registry) error {
				return repoconfig.AddPolicy(cfg, reg, args[0], before, after)
			})
		},
	}
	cmd.Flags().StringVar(&before, "before", "", "position the locator immediately before this one")
	cmd.Flags().StringVar(&after, "after", "", "position the locator immediately after this one")
	return cmd
}

func newRepositoryPolicyRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "remove <LOCATOR...>",
		Short:         "Remove one or more locators from the resolution policy",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutatePolicy(cmd, func(cfg *config.Config, reg *registry.Registry) error {
				return repoconfig.RemovePolicy(cfg, args)
			})
		},
	}
}

func newRepositoryPolicyMoveCmd() *cobra.Command {
	var before, after string
	cmd := &cobra.Command{
		Use:           "move <LOCATOR>",
		Short:         "Reposition a locator in the resolution policy",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutatePolicy(cmd, func(cfg *config.Config, reg *registry.Registry) error {
				return repoconfig.MovePolicy(cfg, args[0], before, after)
			})
		},
	}
	cmd.Flags().StringVar(&before, "before", "", "move immediately before this locator")
	cmd.Flags().StringVar(&after, "after", "", "move immediately after this locator")
	return cmd
}

func newRepositoryPolicyReplaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "replace <LOCATOR...>",
		Short:         "Replace the whole resolution policy",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mutatePolicy(cmd, func(cfg *config.Config, reg *registry.Registry) error {
				return repoconfig.ReplacePolicy(cfg, reg, args)
			})
		},
	}
}

// mutatePolicy runs op against the loaded config/registry and, on success,
// saves the config and prints the one stable `work: policy now …` line
// (resolution-policy.md §Mutation output). A rejected mutation writes nothing.
func mutatePolicy(cmd *cobra.Command, op func(*config.Config, *registry.Registry) error) error {
	if cmd.Flags().Changed("json") {
		return diag.New(diag.Usage, "--json is not accepted on a `work repository policy` mutation")
	}
	home, cfg, reg, err := loadRepositoryContext()
	if err != nil {
		return err
	}
	if err := op(cfg, reg); err != nil {
		return err
	}
	if err := saveRepositoryConfig(home, cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "work: policy now %s\n", joinOrNone(cfg.RepositoryResolution.Locators))
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
