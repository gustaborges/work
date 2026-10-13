package cli

import (
	"fmt"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/spf13/cobra"
)

func newConventionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "convention", Short: "Inspect and change the remembered branch convention for this repository", SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := hubGuard(cmd, "run `work convention show` or `work convention set <convention>`; see `work --help`"); err != nil {
			return err
		}
		return runConventionHub(cmd)
	}}
	cmd.AddCommand(newConventionShowCmd(), newConventionSetCmd())
	return cmd
}
func runConventionHub(cmd *cobra.Command) error {
	for {
		home, snapshot, err := loadConventionContext(cmd.Context())
		if err != nil {
			return err
		}
		identity, err := resolveConventionIdentity()
		if err != nil {
			return err
		}
		view := snapshot.Memory.View(identity, snapshot.Registry)
		current := "not set"
		if view.Convention != nil {
			current = pluginadmin.SafeText(*view.Convention)
		}
		status := "available"
		if view.UnavailableReason != nil {
			status = *view.UnavailableReason
		}
		description := fmt.Sprintf("Repository: %s\nCurrent: %s (%s)\nwork convention show [--json]", pluginadmin.SafeSource(identity), current, status)
		options := []present.Option[string]{{Value: "show", Primary: "Inspect remembered convention", Secondary: "work convention show [--json]"}, {Value: "set", Primary: "Set available convention", Secondary: "work convention set CONVENTION"}, {Value: "", Primary: "Leave unchanged"}}
		action, err := hubChoice(cmd, "Convention", description, options)
		if err != nil {
			return err
		}
		if action == "" {
			return nil
		}
		if action == "show" {
			if err = hubDirect(cmd, "convention", "show"); err != nil {
				return err
			}
			continue
		}
		names := conventionNames(snapshot.Registry)
		opts := stringOptions(names)
		opts = append(opts, present.Option[string]{Value: "", Primary: "Back / Leave unchanged"})
		guidance := "Choose an explicit replacement. work convention set CONVENTION"
		if len(names) == 0 {
			guidance = "No available convention. Install or enable a provider first."
		}
		chosen, err := hubChoice(cmd, "Available conventions", guidance, opts)
		if err != nil {
			return err
		}
		if chosen == "" {
			continue
		}
		if view.Convention != nil && chosen == *view.Convention && view.Available {
			continue
		}
		if err = mutationPreflight(cmd, home); err != nil {
			return err
		}
		if err = setReviewedConvention(cmd.Context(), home, identity, chosen, snapshot); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "work: convention set to %s\nwork: (equivalent: %s)\n", pluginadmin.SafeText(chosen), safeCommand([]string{"convention", "set", chosen}))
	}
}
