package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/repoconv"
)

// newConventionCmd registers the `work convention` family: `show`/`set` are
// ordinary read/mutate commands; the bare form is this codebase's first
// genuinely interactive hub — unlike `work plugin`/`work repository`,
// ADR-0011/FR-030 do not defer it to F7 (research R16).
func newConventionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "convention",
		Short:         "Inspect and change the remembered branch convention for this repository",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on `work convention` (it opens the interactive hub)")
			}
			if !present.IsInteractive() {
				return diag.New(diag.Usage,
					"run `work convention show` or `work convention set <convention>`; see `work --help`")
			}
			return runConventionHub(cmd)
		},
	}
	cmd.AddCommand(newConventionShowCmd(), newConventionSetCmd())
	return cmd
}

// runConventionHub opens the interactive hub (contracts/cli-work-convention.md
// §`work convention` (no subcommand)): a read-only display of the current
// choice, a single-choice step over the enabled catalog plus "leave
// unchanged", and — on an actual change — the equivalent direct command as
// the receipt (ADR-0019). Cancellation (Esc/q/Ctrl-C) returns diag.Cancelled
// (exit 20) via present.Select; leaving the choice unchanged (explicitly
// re-picking the current value, or picking "leave unchanged") persists
// nothing and prints no receipt.
func runConventionHub(cmd *cobra.Command) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	pio := present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}

	home, reg, err := loadConventionContext()
	if err != nil {
		return err
	}
	identity, err := resolveConventionIdentity()
	if err != nil {
		return err
	}
	store, err := repoconv.Load(home.BranchConventionsFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot read the branch convention memory")
	}
	current, hasCurrent := store.Get(identity)

	const leaveUnchanged = ""
	opts := []present.Option[string]{{Value: leaveUnchanged, Primary: "(leave unchanged)"}}
	for _, name := range conventionNames(reg) {
		primary := name
		if hasCurrent && name == current {
			primary = name + " (current)"
		}
		opts = append(opts, present.Option[string]{Value: name, Primary: primary})
	}

	desc := "current: not set"
	if hasCurrent {
		desc = "current: " + current
	}

	chosen, err := present.Select(ctx, pio, present.SelectSpec[string]{
		Title:       "Convention",
		Description: desc,
		Options:     opts,
		Receipt: func(o present.Option[string]) string {
			if o.Value == leaveUnchanged {
				return "unchanged"
			}
			return o.Value
		},
	})
	if err != nil {
		return err
	}

	if chosen == leaveUnchanged || chosen == current {
		return nil
	}

	store.Set(identity, chosen)
	if err := repoconv.Save(home.BranchConventionsFile(), store); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot persist the branch convention memory")
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "work: convention set to %s\n", chosen)
	fmt.Fprintf(out, "work: (equivalent: `work convention set %s`)\n", chosen)
	return nil
}
