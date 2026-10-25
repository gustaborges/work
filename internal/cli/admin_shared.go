package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	title := hubReadTitle(args)
	var result bytes.Buffer
	if title != "" {
		root.SetOut(io.MultiWriter(cmd.OutOrStdout(), &result))
		root.SetErr(io.MultiWriter(cmd.ErrOrStderr(), &result))
	} else {
		root.SetOut(cmd.OutOrStdout())
		root.SetErr(cmd.ErrOrStderr())
	}
	root.SetArgs(args)
	executed, err := root.ExecuteContextC(cmd.Context())
	if err != nil {
		if title != "" && result.Len() > 0 {
			if viewErr := hubReadResult(cmd, title, result.String(), args); viewErr != nil {
				return viewErr
			}
		}
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
	if title != "" {
		return hubReadResult(cmd, title, result.String(), equivalent)
	}
	return nil
}
func hubReadTitle(args []string) string {
	if len(args) < 2 {
		return ""
	}
	switch {
	case args[0] == "plugin" && args[1] == "list":
		return "Installed plugins"
	case args[0] == "plugin" && args[1] == "update" && slices.Contains(args, "--check"):
		return "Plugin update check"
	case args[0] == "convention" && args[1] == "show":
		return "Remembered convention"
	case args[0] == "repository" && len(args) == 3 && args[2] == "list":
		switch args[1] {
		case "locator":
			return "Installed Locators"
		case "policy":
			return "Repository policy"
		case "root":
			return "Repository search roots"
		}
	}
	return ""
}
func hubReadResult(cmd *cobra.Command, title, content string, args []string) error {
	if strings.TrimSpace(content) == "" {
		content = "No entries."
	}
	content = strings.TrimSpace(content) + "\n\nEquivalent: " + safeCommand(args)
	_, err := present.Wizard(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, present.WizardSpec{Title: title, Steps: []present.Step{
		present.TextStep("result", func(present.Answers) (present.TextSpec, error) {
			return present.TextSpec{Title: title, Content: content}, nil
		}),
	}})
	return err
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

// A hub publishes stable output only after leaving its one full-screen session.
// Live notices keep accepted operation results visible during navigation.
type hubSessionWriter struct {
	ctx    context.Context
	output io.Writer
}

func (w hubSessionWriter) Write(data []byte) (int, error) {
	n, err := w.output.Write(data)
	if n > 0 {
		present.SessionNotice(w.ctx, string(data[:n]))
	}
	return n, err
}
func runHub(cmd *cobra.Command, run func(*cobra.Command) error) error {
	originalCtx := cmd.Context()
	out, ui := cmd.OutOrStdout(), cmd.ErrOrStderr()
	var results, notices bytes.Buffer
	err := present.WithSession(originalCtx, present.IO{In: cmd.InOrStdin(), UI: ui}, func(ctx context.Context) error {
		cmd.SetContext(ctx)
		cmd.SetOut(hubSessionWriter{ctx: ctx, output: &results})
		cmd.SetErr(hubSessionWriter{ctx: ctx, output: &notices})
		defer func() { cmd.SetContext(originalCtx); cmd.SetOut(out); cmd.SetErr(ui) }()
		return run(cmd)
	})
	_, _ = io.Copy(ui, &notices)
	_, _ = io.Copy(out, &results)
	return err
}
