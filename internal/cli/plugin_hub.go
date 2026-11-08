package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gustaborges/work/internal/adminstate"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/marketplaceadmin"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/pluginid"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/spf13/cobra"
)

// runPluginHub opens the one plugin manager. `work plugin` starts on
// Installed and `work plugin marketplace` on Marketplaces; both reach the same
// destinations and print exactly one acknowledgment after the session ends.
func runPluginHub(cmd *cobra.Command, startSources bool) error {
	home, err := workhome.Resolve()
	if err != nil {
		return err
	}
	spec := pluginCatalog(home)
	spec.StartSources = startSources
	if err = present.Catalog(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, spec); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Changes saved successfully.")
	return nil
}

func sourceKindLabel(kind string) string {
	switch kind {
	case pluginid.KindBuiltin:
		return "Included with Work"
	case pluginid.KindGit:
		return "Git repository"
	case pluginid.KindLocal:
		return "Local directory"
	}
	return "Source"
}

func pluginCatalog(home workhome.Home) present.CatalogSpec {
	return present.CatalogSpec{
		Title: "Plugins", Empty: "No plugins installed. Open Marketplaces to register a source and add a plugin.",
		ErrorText: pluginManagerError, FormAction: "add-source", FormTitle: "Add marketplace",
		Fields: []present.CatalogField{
			{Key: "source", Label: "Source", Hint: "Git URL or local directory", Validate: func(v string) error {
				_, err := marketplaceadmin.Classify(v, "")
				return err
			}},
			{Key: "branch", Label: "Branch", Hint: "Optional; defaults to the remote's default branch", Visible: func(values map[string]string) bool {
				spec, err := marketplaceadmin.Classify(values["source"], "")
				return err == nil && spec.Kind == pluginid.KindGit
			}},
		},
		InstalledActions: []present.CatalogAction{
			{ID: "enable", Label: "Enable", Direct: true}, {ID: "disable", Label: "Disable", Direct: true},
			{ID: "check", Label: "Check for updates"}, {ID: "update", Label: "Update"}, {ID: "uninstall", Label: "Remove"},
		},
		SourceActions: []present.CatalogAction{
			{ID: "add-source", Label: "Add marketplace", Form: true},
			{ID: "available", Label: "Browse available plugins", Go: "available"},
			{ID: "upgrade", Label: "Upgrade catalogs", Direct: true},
		},
		Load: func(ctx context.Context) (present.CatalogData, error) { return loadManagerData(ctx, home) },
		Run: func(ctx context.Context, r present.CatalogRequest) (string, error) {
			return runManagerAction(ctx, home, r)
		},
		Prepare: func(ctx context.Context, r present.CatalogRequest) (*present.CatalogReview, error) {
			return prepareManagerAction(ctx, home, r)
		},
	}
}

func loadManagerData(ctx context.Context, home workhome.Home) (present.CatalogData, error) {
	s, err := adminstate.Read(ctx, home)
	if err != nil {
		return present.CatalogData{}, err
	}
	data := present.CatalogData{}
	for _, e := range pluginadmin.Inventory(s) {
		state, verb := "Disabled", "Enable"
		if e.Enabled {
			state, verb = "Enabled", "Disable"
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
		props := []present.CatalogProperty{{Label: "Version", Value: valueOrUnknown(e.Version)}, {Label: "Marketplace", Value: e.Marketplace}, {Label: "Source", Value: e.Source}}
		if e.Revision != nil {
			props = append(props, present.CatalogProperty{Label: "Revision", Value: *e.Revision})
		}
		if e.ContentDigest != nil {
			props = append(props, present.CatalogProperty{Label: "Digest", Value: *e.ContentDigest})
		}
		actions := []present.CatalogAction{
			{ID: strings.ToLower(verb), Label: verb, Direct: true}, {ID: "check", Label: "Check for updates"},
			{ID: "update", Label: "Update"}, {ID: "uninstall", Label: "Remove"},
		}
		data.Installed = append(data.Installed, present.CatalogItem{
			ID: e.ID, Name: e.ID, Source: e.Marketplace, Installed: true, Enabled: e.Enabled, Details: state,
			Properties: props, Actions: actions,
			Sections: []present.CatalogSection{{Title: "Components", Content: strings.Join(components, "\n")}, {Title: "Conventions", Content: "  " + strings.Join(conventions, "\n  ")}},
		})
	}
	for _, r := range marketplaceadmin.List(s) {
		name := r.Name
		if r.DisplayName != nil && *r.DisplayName != r.Name {
			name = *r.DisplayName + " (" + r.Name + ")"
		}
		plugins := "unknown"
		if r.PluginCount != nil {
			plugins = fmt.Sprint(*r.PluginCount)
		}
		props := []present.CatalogProperty{
			{Label: "Namespace", Value: r.Name}, {Label: "Kind", Value: sourceKindLabel(r.SourceKind)}, {Label: "Origin", Value: r.Source},
			{Label: "Catalog", Value: r.CatalogStatus}, {Label: "Offered", Value: plugins},
			{Label: "Installed", Value: fmt.Sprint(r.InstalledCount)}, {Label: "Enabled", Value: fmt.Sprint(r.EnabledCount)},
		}
		if r.Branch != nil {
			props = append(props, present.CatalogProperty{Label: "Branch", Value: *r.Branch})
		}
		if r.Revision != nil {
			props = append(props, present.CatalogProperty{Label: "Revision", Value: *r.Revision})
		}
		actions := []present.CatalogAction{
			{ID: "available", Label: "Browse available plugins", Go: "available"},
			{ID: "installed", Label: "Show installed plugins", Go: "installed"},
			{ID: "upgrade", Label: "Upgrade catalog", Direct: true},
		}
		if r.SourceKind != pluginid.KindBuiltin {
			actions = append(actions, present.CatalogAction{ID: "remove-source", Label: "Remove marketplace"})
		}
		data.Sources = append(data.Sources, present.CatalogItem{
			ID: r.Name, Name: name, Badge: sourceKindLabel(r.SourceKind), Properties: props, Actions: actions,
			Details: fmt.Sprintf("%s · catalog %s", sourceKindLabel(r.SourceKind), r.CatalogStatus),
		})
	}
	offers, _ := pluginadmin.Available(s, "")
	for _, o := range offers {
		components := []string{}
		for _, c := range o.Components {
			components = append(components, "  "+c.Name+" ("+c.Role+")")
		}
		if len(components) == 0 {
			components = append(components, "  None")
		}
		label := "Add"
		if o.Installed {
			label = "Reinstall"
		}
		item := present.CatalogItem{
			ID: o.ID, Name: o.ID, Source: o.Marketplace, Installed: o.Installed, Details: o.Description,
			Properties: []present.CatalogProperty{{Label: "Version", Value: o.Version}, {Label: "Revision", Value: o.Revision}},
			Sections:   []present.CatalogSection{{Title: "Contributions", Content: strings.Join(components, "\n")}},
			Actions:    []present.CatalogAction{{ID: "add", Label: label}},
		}
		if o.Enabled != nil {
			item.Enabled = *o.Enabled
		}
		data.Available = append(data.Available, item)
	}
	return data, nil
}

func parseManagerIDs(ids []string) ([]pluginid.Provider, error) {
	out := make([]pluginid.Provider, 0, len(ids))
	for _, id := range ids {
		p, err := pluginid.ParseProvider(id)
		if err != nil {
			return nil, diag.New(diag.Usage, "Choose installed plugins.")
		}
		out = append(out, p)
	}
	return pluginadmin.SortedProviders(out), nil
}

func runManagerAction(ctx context.Context, home workhome.Home, r present.CatalogRequest) (string, error) {
	if _, err := adminstate.Preflight(ctx, home); err != nil {
		return "", err
	}
	switch r.Action {
	case "enable", "disable":
		ps, err := parseManagerIDs(r.IDs)
		if err != nil {
			return "", err
		}
		done, err := pluginadmin.SetEnabled(ctx, home, ps, r.Action == "enable")
		if err != nil {
			return "", err
		}
		verb := "Enabled"
		if r.Action == "disable" {
			verb = "Disabled"
		}
		return fmt.Sprintf("%s %d plugin(s).", verb, len(done)), nil
	case "upgrade":
		rows, err := marketplaceadmin.Upgrade(ctx, home, r.IDs)
		var b strings.Builder
		for _, row := range rows {
			line := fmt.Sprintf("%s %s", row.Status, pluginadmin.SafeText(row.Name))
			if row.Status != marketplaceadmin.StatusFailed && row.Revision != nil {
				line += " (" + pluginadmin.SafeText(*row.Revision) + ")"
			}
			if row.Error != nil {
				line += " " + pluginadmin.SafeText(row.Error.Token)
			}
			b.WriteString(line + "\n")
		}
		text := strings.TrimSuffix(b.String(), "\n")
		if len(rows) == 1 {
			text += "."
		}
		return text, err
	}
	return "", diag.New(diag.Usage, "Choose an available action.")
}

func prepareManagerAction(ctx context.Context, home workhome.Home, r present.CatalogRequest) (*present.CatalogReview, error) {
	switch r.Action {
	case "add-source":
		return prepareManagerAddSource(ctx, home, r.Values)
	case "uninstall":
		return prepareManagerUninstall(ctx, home, r.IDs)
	case "remove-source":
		if len(r.IDs) != 1 {
			return nil, diag.New(diag.Usage, "Choose one marketplace.")
		}
		return prepareManagerRemoveSource(ctx, home, r.IDs[0])
	case "add":
		if len(r.IDs) != 1 {
			return nil, diag.New(diag.Usage, "Choose one plugin to add.")
		}
		return prepareManagerInstall(ctx, home, r.IDs[0])
	case "check":
		return prepareManagerCheck(ctx, home, r.IDs)
	case "update":
		return prepareManagerUpdate(ctx, home, r.IDs)
	}
	return nil, diag.New(diag.Usage, "Choose an available action.")
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

func prepareManagerAddSource(ctx context.Context, home workhome.Home, values map[string]string) (*present.CatalogReview, error) {
	spec, err := marketplaceadmin.Classify(values["source"], values["branch"])
	if err != nil {
		return nil, err
	}
	if _, err := adminstate.Preflight(ctx, home); err != nil {
		return nil, err
	}
	plan, err := marketplaceadmin.PrepareAdd(ctx, home, spec)
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	fmt.Fprintf(&content, "Register %s\n\nOrigin: %s\n", pluginadmin.SafeText(plan.Name()), pluginadmin.SafeSource(spec.Source))
	if spec.Branch != "" {
		fmt.Fprintf(&content, "Branch: %s\n", pluginadmin.SafeText(spec.Branch))
	}
	if offer := plan.Offer(); offer != nil {
		fmt.Fprintf(&content, "Offers %d plugin(s).\n", len(offer.Entries))
	} else {
		content.WriteString("This exact source is already registered.\n")
	}
	content.WriteString("\nRegistering installs nothing and leaves repository policy unchanged.")
	return &present.CatalogReview{Title: "Review marketplace", Content: content.String(), Accept: "Register", Close: plan.Close,
		Apply: func(ctx context.Context, _ bool) (string, error) {
			if _, err := adminstate.Preflight(ctx, home); err != nil {
				return "", err
			}
			res, err := plan.Apply(ctx)
			if err == nil || res.Committed {
				return fmt.Sprintf("%s %s.", res.Result, pluginadmin.SafeText(res.Name)), err
			}
			return "", err
		}}, nil
}

func prepareManagerInstall(ctx context.Context, home workhome.Home, id string) (*present.CatalogReview, error) {
	p, err := pluginid.ParseProvider(id)
	if err != nil {
		return nil, diag.New(diag.Usage, "Choose an offered plugin.")
	}
	if _, err := adminstate.Preflight(ctx, home); err != nil {
		return nil, err
	}
	plan, err := pluginadmin.PrepareInstall(ctx, home, p.Plugin, p.Marketplace)
	if err != nil {
		return nil, err
	}
	offered := plan.Offered()
	var content strings.Builder
	title, accept := "Review installation", "Add"
	if plan.Result() == pluginadmin.ResultReinstalled {
		title, accept = "Review reinstall", "Reinstall"
		prior, _ := plan.Replacing()
		fmt.Fprintf(&content, "Replace %s: version %s → %s\n\nEnablement is kept.\n", pluginadmin.SafeText(id), pluginadmin.SafeText(prior.Version), pluginadmin.SafeText(offered.Version))
	} else {
		fmt.Fprintf(&content, "Add %s (version %s)\n\nAn independent copy is stored; the marketplace is not modified.\n", pluginadmin.SafeText(id), pluginadmin.SafeText(offered.Version))
	}
	for _, c := range offered.Components {
		fmt.Fprintf(&content, "  %s (%s)\n", pluginadmin.SafeText(c.Name), pluginadmin.SafeText(c.Role))
	}
	return &present.CatalogReview{Title: title, Content: content.String(), Accept: accept, Dependencies: plan.PolicyReferences(), Close: plan.Close,
		Apply: func(ctx context.Context, remove bool) (string, error) {
			if _, err := adminstate.Preflight(ctx, home); err != nil {
				return "", err
			}
			res, err := plan.Apply(ctx, remove)
			if err == nil || res.Committed {
				return fmt.Sprintf("%s %s.", res.Result, pluginadmin.SafeText(res.Provider.String())), err
			}
			return "", err
		}}, nil
}

func prepareManagerCheck(ctx context.Context, home workhome.Home, ids []string) (*present.CatalogReview, error) {
	ps, err := parseManagerIDs(ids)
	if err != nil {
		return nil, err
	}
	rows, err := pluginadmin.Check(ctx, home, ps)
	if rows == nil {
		return nil, err
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(pluginadmin.SafeText(pluginadmin.UpdateSummary(r)) + "\n")
		if len(r.PolicyReferencesRemoved) > 0 {
			fmt.Fprintf(&b, "  policy references withdrawn: %s\n", pluginadmin.SafeText(strings.Join(r.PolicyReferencesRemoved, ", ")))
		}
		if r.Error != nil {
			fmt.Fprintf(&b, "  %s: %s\n", pluginadmin.SafeText(r.Error.Token), pluginadmin.SafeText(r.Error.Message))
		}
	}
	b.WriteString("\nChecking changes nothing. Choose Update to review and apply.")
	return &present.CatalogReview{Title: "Update check", Content: b.String()}, nil
}

func prepareManagerUpdate(ctx context.Context, home workhome.Home, ids []string) (*present.CatalogReview, error) {
	ps, err := parseManagerIDs(ids)
	if err != nil {
		return nil, err
	}
	if _, err := adminstate.Preflight(ctx, home); err != nil {
		return nil, err
	}
	plan, err := pluginadmin.PrepareUpdates(ctx, home, ps, true)
	if err != nil {
		return nil, err
	}
	var available []pluginadmin.UpdateResult
	var dependencies []string
	var b strings.Builder
	for _, r := range plan.Results {
		b.WriteString(pluginadmin.SafeText(pluginadmin.UpdateSummary(r)) + "\n")
		if r.Error != nil {
			fmt.Fprintf(&b, "  %s: %s\n", pluginadmin.SafeText(r.Error.Token), pluginadmin.SafeText(r.Error.Message))
		}
		if r.Status == pluginadmin.UpdateAvailable {
			available = append(available, r)
			dependencies = append(dependencies, r.PolicyReferencesRemoved...)
			for _, e := range plan.ConventionEffects(r.ID) {
				fmt.Fprintf(&b, "  convention %s: available %t → %t\n", e.Name, e.AvailableBefore, e.AvailableAfter)
			}
		}
	}
	review := &present.CatalogReview{Title: "Review updates", Close: plan.Close, Dependencies: dependencies}
	if len(available) == 0 {
		b.WriteString("\nNo update is available.")
		review.Content = b.String()
		return review, nil
	}
	b.WriteString("\nEnablement is kept. Remembered conventions and existing Works are retained.")
	review.Content, review.Accept = b.String(), "Apply updates"
	review.Apply = func(ctx context.Context, remove bool) (string, error) {
		rows, err := pluginadmin.ApplyUpdates(ctx, plan, remove)
		var out strings.Builder
		for _, r := range rows {
			fmt.Fprintf(&out, "%s %s\n", r.Status, pluginadmin.SafeText(r.ID))
		}
		return strings.TrimSuffix(out.String(), "\n"), err
	}
	return review, nil
}

func prepareManagerRemoveSource(ctx context.Context, home workhome.Home, name string) (*present.CatalogReview, error) {
	if err := marketplaceadmin.CheckRemovable(name); err != nil {
		return nil, err
	}
	if _, err := adminstate.Preflight(ctx, home); err != nil {
		return nil, err
	}
	plan, err := marketplaceadmin.PrepareRemove(ctx, home, name)
	if err != nil {
		return nil, err
	}
	return &present.CatalogReview{Title: "Review marketplace removal", Destructive: true, Content: marketplaceRemovalImpact(plan, false),
		Accept: "Remove marketplace", Dependencies: plan.PolicyReferences(),
		Apply: func(ctx context.Context, remove bool) (string, error) {
			if _, err := adminstate.Preflight(ctx, home); err != nil {
				return "", err
			}
			res, err := plan.Apply(ctx, remove)
			if res.Committed {
				return "Removed marketplace " + pluginadmin.SafeText(res.Name) + ".", err
			}
			return "", err
		}}, nil
}

func prepareManagerUninstall(ctx context.Context, home workhome.Home, ids []string) (*present.CatalogReview, error) {
	ps, err := parseManagerIDs(ids)
	if err != nil {
		return nil, err
	}
	plan, err := pluginadmin.PlanUninstallSet(ctx, home, ps)
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	fmt.Fprintf(&content, "Remove %d plugin(s):\n", len(ps))
	for _, p := range ps {
		root, _ := home.PackageRoot(p)
		fmt.Fprintf(&content, "  %s\n    %s\n", pluginadmin.SafeText(p.String()), pluginadmin.SafeText(root))
	}
	content.WriteString("\nThe marketplace source is untouched. Remembered conventions and existing Works are retained.\n")
	managerEffects(&content, plan.ConventionEffects())
	return &present.CatalogReview{Title: "Review removal", Destructive: true, Content: content.String(), Accept: "Remove", Dependencies: plan.PolicyReferences(),
		Apply: func(ctx context.Context, remove bool) (string, error) {
			if _, err := adminstate.Preflight(ctx, home); err != nil {
				return "", err
			}
			removed, err := pluginadmin.Uninstall(ctx, home, plan, remove)
			if len(removed) > 0 {
				return fmt.Sprintf("Removed %d plugin(s).", len(removed)), err
			}
			return "", err
		}}, nil
}

func managerEffects(b *strings.Builder, effects []pluginadmin.ConventionEffect) {
	for _, e := range effects {
		fmt.Fprintf(b, "\nConvention %s: available %t → %t\n", e.Name, e.AvailableBefore, e.AvailableAfter)
	}
}
