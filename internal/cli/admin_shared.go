package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

func readAdministration(ctx context.Context) (workhome.Home, *adminstate.Snapshot, error) {
	h, err := workhome.Resolve()
	if err != nil {
		return h, nil, diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}
	s, err := adminstate.Read(ctx, h)
	return h, s, err
}
func mutationPreflight(cmd *cobra.Command, h workhome.Home) error {
	ctx := adminstate.WithRecoveryObserver(cmd.Context(), func(string) { fmt.Fprintln(cmd.ErrOrStderr(), "work: recovered interrupted administrative operation") })
	cmd.SetContext(ctx)
	_, err := adminstate.Preflight(ctx, h)
	return err
}

// hubChoice and hubInput use the existing full-screen wizard/keymap. A fresh
// command tree for accepted direct actions keeps flags and closures per action.
func hubChoice(cmd *cobra.Command, title, description string, options []present.Option[string]) (string, error) {
	a, err := present.Wizard(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, present.WizardSpec{Title: title, Steps: []present.Step{
		present.SelectStep("choice", func(present.Answers) (present.SelectSpec[string], error) {
			return present.SelectSpec[string]{Title: title, Description: description, Options: options, Filterable: true}, nil
		}),
	}})
	if err != nil {
		return "", err
	}
	return a.String("choice"), nil
}
func hubInput(cmd *cobra.Command, spec present.InputSpec) (string, error) {
	a, err := present.Wizard(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, present.WizardSpec{Title: spec.Title, Steps: []present.Step{
		present.InputStep("value", func(present.Answers) (present.InputSpec, error) { return spec, nil }),
	}})
	if err != nil {
		return "", err
	}
	return a.String("value"), nil
}
func hubTargets(cmd *cobra.Command, title string, values []string) ([]string, error) {
	if len(values) == 0 {
		_, err := hubChoice(cmd, title, "Nothing to select. Install or configure an entry first.", []present.Option[string]{{Value: "", Primary: "Back"}})
		return nil, err
	}
	a, err := present.Wizard(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, present.WizardSpec{Title: title, Steps: []present.Step{
		present.MultiSelectStep("targets", func(present.Answers) (present.MultiSelectSpec[string], error) {
			return present.MultiSelectSpec[string]{Title: title, Description: "Select targets; accept an empty selection to go Back.", Options: stringOptions(values), Filterable: true}, nil
		}),
	}})
	if err != nil {
		return nil, err
	}
	targets, _ := a.Value("targets").([]string)
	return targets, nil
}
func hubDirect(cmd *cobra.Command, args ...string) error {
	root := newRootCmd()
	root.SetIn(cmd.InOrStdin())
	root.SetOut(cmd.OutOrStdout())
	root.SetErr(cmd.ErrOrStderr())
	root.SetArgs(args)
	executed, err := root.ExecuteContextC(cmd.Context())
	if err != nil {
		return err
	}
	equivalent := slices.Clone(args)
	if len(args) > 1 && args[0] == "plugin" && (args[1] == "update" || args[1] == "uninstall") {
		check, _ := executed.Flags().GetBool("check")
		if !check {
			remove, _ := executed.Flags().GetBool("remove-policy-references")
			if remove {
				equivalent = append(equivalent, "--remove-policy-references")
			}
			equivalent = append(equivalent, "--yes")
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "work: (equivalent: %s)\n", safeCommand(equivalent))
	return nil
}
func safeCommand(args []string) string {
	parts := []string{"work"}
	for i, arg := range args {
		if len(args) > 2 && args[0] == "plugin" && args[1] == "install" && i == 2 && arg != "--embedded" {
			arg = pluginadmin.SafeSource(arg)
		} else {
			arg = pluginadmin.SafeText(arg)
		}
		if strings.ContainsAny(arg, " \t'\"`$;&|<>()[]{}*?!\\") {
			arg = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		}
		parts = append(parts, arg)
	}
	return strings.Join(parts, " ")
}
func hubGuard(cmd *cobra.Command, guidance string) error {
	if cmd.Flags().Changed("json") {
		return diag.New(diag.Usage, "--json requires a concrete read action; "+guidance)
	}
	if !present.IsInteractive() {
		return diag.New(diag.Usage, guidance)
	}
	return nil
}
