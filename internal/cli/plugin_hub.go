package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/plugin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/plugininstall"
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
		Title: "Plugins", Empty: "No plugins installed. Choose Official or Install plugin.",
		Fields: []present.CatalogField{
			{Key: "source", Label: "Source", Hint: "Local path or git URL", Validate: validateHubSource},
			{Key: "alias", Label: "Alias", Hint: "Optional; defaults to the plugin name", Validate: func(v string) error {
				if v == "" {
					return nil
				}
				if plugin.ValidateAlias(v) != nil {
					return diag.New(diag.Usage, "Enter a valid plugin alias (letters, numbers, dots, underscores or hyphens).")
				}
				return nil
			}},
			{Key: "mode", Label: "Mode", Hint: "Link is available for local sources only", Choices: []string{"Copy fixed source", "Link local source"}},
		},
		ErrorText: pluginManagerError,
		Load: func(ctx context.Context) (present.CatalogData, error) {
			s, err := adminstate.Read(ctx, home)
			if err != nil {
				return present.CatalogData{}, err
			}
			data := present.CatalogData{}
			official := present.CatalogItem{ID: bootstrap.Alias, Name: bootstrap.Alias, Details: "Official bundled reference package\n\nProvides local-path startup, filesystem repository discovery, and the freeform convention.\n\nAvailable offline.", Actions: []present.CatalogAction{{ID: "embedded", Label: "Install bundled package"}}}
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
				item := present.CatalogItem{ID: e.Alias, Name: e.Alias, Installed: true, Enabled: e.Enabled, Details: state,
					Properties: []present.CatalogProperty{{Label: "Version", Value: updateValue(e.Version)}, {Label: "Origin", Value: e.Origin}, {Label: "Source", Value: e.Reference}},
					Sections:   []present.CatalogSection{{Title: "Components", Content: strings.Join(components, "\n")}, {Title: "Conventions", Content: "  " + strings.Join(conventions, "\n  ")}},
					Actions:    []present.CatalogAction{{ID: "toggle", Label: verb}, {ID: "update", Label: "Check for updates"}, {ID: "uninstall", Label: "Uninstall"}}}

				data.Installed = append(data.Installed, item)
				if e.Alias == bootstrap.Alias {
					official = item
					official.Details = "Official bundled reference package\n\n" + official.Details
				}
			}
			data.Official = []present.CatalogItem{official}
			return data, nil
		},
		Toggle: func(ctx context.Context, id string, enabled bool) error {
			if _, err := adminstate.Preflight(ctx, home); err != nil {
				return err
			}
			_, err := pluginadmin.SetEnabled(ctx, home, []string{id}, enabled)
			return err
		},
		Prepare: func(ctx context.Context, r present.CatalogRequest) (*present.CatalogReview, error) {
			switch r.Action {
			case "install":
				return prepareManagerInstall(ctx, home, r.Values)
			case "embedded":
				return &present.CatalogReview{Title: "Install bundled package", Content: "Install work-reference offline.\n\nProvides local-path startup, filesystem repository discovery, and the freeform convention.\n\nRepository policy membership is unchanged.", Accept: "Install", Apply: func(ctx context.Context, _ bool) (string, error) {
					if _, err := adminstate.Preflight(ctx, home); err != nil {
						return "", err
					}
					_, err := pluginadmin.InstallEmbedded(ctx, home)
					if err != nil {
						return "", err
					}
					return "Installed " + bootstrap.Alias + ".", nil
				}}, nil
			case "update":
				return prepareManagerUpdate(ctx, home, r.ID)
			case "uninstall":
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
func prepareManagerInstall(ctx context.Context, home workhome.Home, values map[string]string) (*present.CatalogReview, error) {
	source := strings.TrimSpace(values["source"])
	opts := plugininstall.Options{Alias: strings.TrimSpace(values["alias"]), Link: values["mode"] == "Link local source"}
	if err := plugininstall.ValidateRequest(source, opts); err != nil {
		return nil, err
	}
	p, err := plugininstall.Prepare(ctx, source, opts)
	if err != nil {
		return nil, err
	}
	s, err := adminstate.Read(ctx, home)
	if err == nil {
		err = p.ValidateCatalog(s.Registry)
	}
	if err != nil {
		p.Close()
		return nil, err
	}
	var content strings.Builder
	fmt.Fprintf(&content, "Plugin   %s\nVersion  %s\nSource   %s\nMode     %s\n", pluginadmin.SafeText(p.Alias), pluginadmin.SafeText(p.Manifest.Version), pluginadmin.SafeSource(source), p.Origin)
	if _, exists := s.Registry.PackageByAlias(p.Alias); exists {
		content.WriteString("\nReplaces the existing installation; enablement is retained.\n")
	}
	content.WriteString("\nComponents\n")
	for _, c := range p.Manifest.Components {
		fmt.Fprintf(&content, "  %s (%s)\n", pluginadmin.SafeText(c.Name), pluginadmin.SafeText(c.Role))
	}
	return &present.CatalogReview{Title: "Review installation", Content: content.String(), Accept: "Install", Close: p.Close, Apply: func(ctx context.Context, _ bool) (string, error) {
		if _, err := adminstate.Preflight(ctx, home); err != nil {
			return "", err
		}
		_, err := pluginadmin.InstallPrepared(ctx, home, p)
		if err != nil {
			return "", err
		}
		return "Installed " + pluginadmin.SafeText(p.Alias) + ".", nil
	}}, nil
}
func prepareManagerUpdate(ctx context.Context, home workhome.Home, id string) (*present.CatalogReview, error) {
	plan, err := pluginadmin.PrepareUpdates(ctx, home, []string{id}, false)
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	refs := []string{}
	available := false
	for _, r := range plan.Results {
		fmt.Fprintf(&content, "%s: %s\nVersion    %s → %s\nReference  %s → %s\nDigest     %s → %s\n", r.Alias, r.Status, updateValue(r.CurrentVersion), updateValue(r.CandidateVersion), updateValue(r.CurrentReference), updateValue(r.CandidateReference), updateValue(r.CurrentDigest), updateValue(r.CandidateDigest))
		if r.Error != nil {
			fmt.Fprintf(&content, "\n%s\n", r.Error.Message)
		}
		added, withdrawn := plan.DeclarationChanges(r.Alias)
		if len(added) > 0 {
			fmt.Fprintf(&content, "\nAdded: %s\n", strings.Join(added, ", "))
		}
		if len(withdrawn) > 0 {
			fmt.Fprintf(&content, "Withdrawn: %s\n", strings.Join(withdrawn, ", "))
		}
		if modified := plan.ModifiedDeclarations(r.Alias); len(modified) > 0 {
			fmt.Fprintf(&content, "Changed: %s\n", strings.Join(modified, ", "))
		}
		managerEffects(&content, plan.ConventionEffects(r.Alias))
		if r.Status == "update-available" {
			available = true
			refs = append(refs, r.PolicyReferencesRemoved...)
		}
	}
	review := &present.CatalogReview{Title: "Update check", Content: content.String(), Dependencies: refs, Close: plan.Close}
	if available {
		review.Title = "Review update"
		review.Accept = "Apply update"
		review.Apply = func(ctx context.Context, remove bool) (string, error) {
			if _, err := adminstate.Preflight(ctx, home); err != nil {
				return "", err
			}
			rows, err := pluginadmin.Apply(ctx, home, plan, remove)
			for _, row := range rows {
				if row.Status == "updated" {
					return "Updated " + row.Alias + ".", err
				}
			}
			return "", err
		}
	}
	return review, nil
}
func prepareManagerUninstall(ctx context.Context, home workhome.Home, id string) (*present.CatalogReview, error) {
	plan, err := pluginadmin.PlanUninstall(ctx, home, []string{id})
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	fmt.Fprintf(&content, "Uninstall %s?\n\nOwned installation removed:\n%s\n\nLinked sources, remembered conventions and existing Works are retained.\n", pluginadmin.SafeText(id), pluginadmin.SafeText(filepath.Join(home.PluginsDir(), id)))
	managerEffects(&content, plan.ConventionEffects())
	return &present.CatalogReview{Title: "Review uninstall", Destructive: true, Content: content.String(), Accept: "Uninstall", Dependencies: plan.PolicyReferences(), Apply: func(ctx context.Context, remove bool) (string, error) {
		if _, err := adminstate.Preflight(ctx, home); err != nil {
			return "", err
		}
		aliases, err := pluginadmin.Uninstall(ctx, home, plan, remove)
		if len(aliases) > 0 {
			return "Uninstalled " + pluginadmin.SafeText(id) + ".", err
		}
		return "", err
	}}, nil
}
func managerEffects(b *strings.Builder, effects []pluginadmin.ConventionEffect) {
	for _, e := range effects {
		fmt.Fprintf(b, "\nConvention %s: available %t → %t\nEnabled owners: %s → %s\n", e.Name, e.AvailableBefore, e.AvailableAfter, joinOrNone(e.OwnersBefore), joinOrNone(e.OwnersAfter))
	}
}
func validateHubSource(v string) error {
	if strings.TrimSpace(v) == "" {
		return diag.New(diag.Usage, "Enter a source directory or URL.")
	}
	return nil
}
