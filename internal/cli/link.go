package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/extension"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/projection"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/workcontext"
	"github.com/spf13/cobra"
)

func newLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "link [LINKER] [VALUE]", Short: "Associate a link with the current Work",
		SilenceErrors: true, SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return diag.New(diag.Usage, "run `work link [LINKER] [VALUE]` with at most two arguments")
			}
			if len(args) == 2 && args[1] == "" {
				return diag.New(diag.Usage, "enter a non-empty value; run `work link LINKER VALUE`")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on `work link` (it is a mutation)")
			}
			if len(args) < 2 && !present.IsInteractive() {
				return diag.New(diag.Usage, "both arguments are required non-interactively; run `work link LINKER VALUE`")
			}
			current, err := requireActiveContext(cmd.Context())
			if err != nil {
				return err
			}
			home, _, reg, err := loadContextGlobals()
			if err != nil {
				return err
			}
			var selected registry.Component
			if len(args) > 0 {
				selected, err = extension.ResolveManual(reg, plugin.RoleLinker, args[0])
				if err != nil {
					return err
				}
			}
			value := ""
			if len(args) == 2 {
				value = args[1]
			} else {
				choices := extension.ManualCandidates(reg, plugin.RoleLinker, extension.Subject{State: current.State, WorktreePath: current.WorktreePath})
				if len(choices) == 0 {
					return diag.New(diag.NoEligibleComponent, "no manual Linker is available; install or enable a package with a manual Linker")
				}
				answers, err := present.Wizard(cmd.Context(), contextIO(cmd), present.WizardSpec{Title: "Link a Work", Steps: []present.Step{
					present.SelectStep("provider", func(present.Answers) (present.SelectSpec[registry.Component], error) {
						if len(args) > 0 {
							return present.SelectSpec[registry.Component]{}, present.StepResolved(selected)
						}
						options := make([]present.Option[registry.Component], len(choices))
						for i, d := range choices {
							c := d.Component
							options[i] = present.Option[registry.Component]{Value: c, Primary: c.QualifiedName(), Secondary: c.Manual.DisplayName + " — " + c.Manual.Description}
						}
						return present.SelectSpec[registry.Component]{Title: "Choose a Linker", Options: options, Filterable: true, Receipt: func(o present.Option[registry.Component]) string { return o.Value.QualifiedName() }}, nil
					}),
					present.InputStep("value", func(a present.Answers) (present.InputSpec, error) {
						c := a.Value("provider").(registry.Component)
						return present.InputSpec{Title: "Link value", Description: c.Key, Validate: func(_ context.Context, s string) error {
							if s == "" {
								return fmt.Errorf("enter a non-empty value")
							}
							return nil
						}, Receipt: func(string) string { return "value supplied" }}, nil
					}),
				}})
				if err != nil {
					return err
				}
				selected = answers.Value("provider").(registry.Component)
				value = answers.String("value")
			}
			result, err := extension.PublishLink(cmd.Context(), extension.PublishParams{Home: home, Expected: current, Component: selected, Operation: extension.OpLink, Value: value,
				Reload: func() (*registry.Registry, error) {
					r, err := registry.Load(home.RegistryFile())
					if err != nil {
						return nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
					}
					return r, nil
				},
				Record: func(c workcontext.Context, p projection.Provenance) error { return recordContextLink(home, c, p) },
			})
			if err != nil {
				return err
			}
			if os.Getenv("WORK_DEBUG") != "" {
				for _, note := range result.Diagnostics {
					fmt.Fprintln(cmd.ErrOrStderr(), "debug: "+note)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "work: linked %s %s %s\n", result.State.Work.ID, selected.Key, selected.QualifiedName())
			return nil
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return diag.Wrap(diag.Usage, err, err.Error()) })
	return cmd
}
