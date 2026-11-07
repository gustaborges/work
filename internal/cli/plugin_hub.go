package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

func runPluginHub(cmd *cobra.Command) error {
	home, err := workhome.Resolve()
	if err != nil {
		return err
	}
	spec := pluginCatalog(home)
	if err = present.Catalog(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, spec); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Changes saved successfully.")
	return nil
}

func pluginCatalog(home workhome.Home) present.CatalogSpec {
	return present.CatalogSpec{
		Title: "Plugins", Empty: "No plugins installed. Register a marketplace with `work plugin marketplace add`.",
		ErrorText: pluginManagerError,
		Load: func(ctx context.Context) (present.CatalogData, error) {
			s, err := adminstate.Read(ctx, home)
			if err != nil {
				return present.CatalogData{}, err
			}
			data := present.CatalogData{}
			for _, e := range pluginadmin.Inventory(s) {
				state := "Disabled"
				verb := "Enable"
				if e.Enabled {
					state = "Enabled"
					verb = "Disable"
				}
				components := []string{}
				for _, c := range e.Components {
					components = append(components, "  "+c.Name+" ("+c.Role+")")
				}
				if len(components) == 0 {
					components = append(components, "  None")
				}
				conventions := e.Conventions
				if len(conventions) == 0 {
					conventions = []string{"None"}
				}
				data.Installed = append(data.Installed, present.CatalogItem{
					ID: e.ID, Name: e.ID, Installed: true, Enabled: e.Enabled, Details: state,
					Properties: []present.CatalogProperty{{Label: "Version", Value: valueOrUnknown(e.Version)}, {Label: "Marketplace", Value: e.Marketplace}, {Label: "Source", Value: e.Source}},
					Sections:   []present.CatalogSection{{Title: "Components", Content: strings.Join(components, "\n")}, {Title: "Conventions", Content: "  " + strings.Join(conventions, "\n  ")}},
					Actions:    []present.CatalogAction{{ID: "toggle", Label: verb}, {ID: "uninstall", Label: "Remove"}},
				})
			}
			return data, nil
		},
		Toggle: func(ctx context.Context, id string, enabled bool) error {
			p, err := pluginid.ParseProvider(id)
			if err != nil {
				return diag.New(diag.Usage, "Choose an installed plugin.")
			}
			if _, err := adminstate.Preflight(ctx, home); err != nil {
				return err
			}
			_, err = pluginadmin.SetEnabled(ctx, home, []pluginid.Provider{p}, enabled)
			return err
		},
		Prepare: func(ctx context.Context, r present.CatalogRequest) (*present.CatalogReview, error) {
			if r.Action == "uninstall" {
				return prepareManagerUninstall(ctx, home, r.ID)
			}
			return nil, diag.New(diag.Usage, "Choose an available plugin action.")
		},
	}
}

func pluginManagerError(err error) string {
	var d *diag.Error
	if errors.As(err, &d) {
		if d.Summary != "" {
			return pluginadmin.SafeText(d.Summary)
		}
		return pluginadmin.SafeText(d.Msg)
	}
	return "The operation could not be completed. Check the source and try again."
}

func prepareManagerUninstall(ctx context.Context, home workhome.Home, id string) (*present.CatalogReview, error) {
	p, err := pluginid.ParseProvider(id)
	if err != nil {
		return nil, diag.New(diag.Usage, "Choose an installed plugin.")
	}
	plan, err := pluginadmin.PlanUninstall(ctx, home, []pluginid.Provider{p})
	if err != nil {
		return nil, err
	}
	root, _ := home.PackageRoot(p)
	var content strings.Builder
	fmt.Fprintf(&content, "Remove %s?\n\nOwned installation removed:\n%s\n\nRemembered conventions and existing Works are retained.\n", pluginadmin.SafeText(id), pluginadmin.SafeText(root))
	managerEffects(&content, plan.ConventionEffects())
	return &present.CatalogReview{Title: "Review removal", Destructive: true, Content: content.String(), Accept: "Remove", Dependencies: plan.PolicyReferences(), Apply: func(ctx context.Context, remove bool) (string, error) {
		if _, err := adminstate.Preflight(ctx, home); err != nil {
			return "", err
		}
		removed, err := pluginadmin.Uninstall(ctx, home, plan, remove)
		if len(removed) > 0 {
			return "Removed " + pluginadmin.SafeText(id) + ".", err
		}
		return "", err
	}}, nil
}

func managerEffects(b *strings.Builder, effects []pluginadmin.ConventionEffect) {
	for _, e := range effects {
		fmt.Fprintf(b, "\nConvention %s: available %t → %t\n", e.Name, e.AvailableBefore, e.AvailableAfter)
	}
}
