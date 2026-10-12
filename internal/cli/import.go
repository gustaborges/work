package cli

import (
	"fmt"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/extension"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/registry"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "import [IMPORTER]", Short: "Import artifacts into the current Work", SilenceErrors: true, SilenceUsage: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return diag.New(diag.Usage, "run `work import [IMPORTER]` with at most one argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("json") {
				return diag.New(diag.Usage, "--json is not accepted on `work import` (it is a mutation)")
			}
			if len(args) == 0 && !present.IsInteractive() {
				return diag.New(diag.Usage, "an Importer is required non-interactively; run `work import IMPORTER`")
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
				selected, err = extension.ResolveManual(reg, plugin.RoleImporter, args[0])
				if err != nil {
					return err
				}
			} else {
				choices := extension.ManualCandidates(reg, plugin.RoleImporter, extension.Subject{State: current.State, WorktreePath: current.WorktreePath})
				if len(choices) == 0 {
					return diag.New(diag.NoEligibleComponent, "no manual Importer has its required inputs; associate the required links or install an eligible provider")
				}
				answers, err := present.Wizard(cmd.Context(), contextIO(cmd), present.WizardSpec{Title: "Import into a Work", Steps: []present.Step{
					present.SelectStep("provider", func(present.Answers) (present.SelectSpec[registry.Component], error) {
						options := make([]present.Option[registry.Component], len(choices))
						for i, d := range choices {
							c := d.Component
							options[i] = present.Option[registry.Component]{Value: c, Primary: c.QualifiedName(), Secondary: c.Manual.DisplayName + " — " + c.Manual.Description}
						}
						return present.SelectSpec[registry.Component]{Title: "Choose an Importer", Options: options, Filterable: true, Receipt: func(o present.Option[registry.Component]) string { return o.Value.QualifiedName() }}, nil
					}),
				}})
				if err != nil {
					return err
				}
				selected = answers.Value("provider").(registry.Component)
			}
			progress := newProgressObserver(cmd.ErrOrStderr(), cmd.InOrStdin(), present.IsInteractive())
			progress.OnEvent(extension.Event{Kind: extension.Running, Component: selected.QualifiedName(), Operation: extension.OpImport})
			result, err := extension.ImportOne(cmd.Context(), extension.ImportParams{Home: home, Expected: current, Component: selected, Reload: func() (*registry.Registry, error) {
				reg, err := registry.Load(home.RegistryFile())
				if err != nil {
					return nil, diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
				}
				return reg, nil
			}})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "work: imported %s %s %d\n", current.State.Work.ID, selected.QualifiedName(), result.Count)
			return nil
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return diag.Wrap(diag.Usage, err, err.Error()) })
	return cmd
}
