package cli

import (
	"context"
	"fmt"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/pluginadmin"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconfig"
	"github.com/spf13/cobra"
	"slices"
	"strings"
)

func runRepositoryHub(cmd *cobra.Command) error {
	for {
		home, snapshot, err := readAdministration(cmd.Context())
		if err != nil {
			return err
		}
		cfg, reg := snapshot.Config, snapshot.Registry
		var description strings.Builder
		description.WriteString("Repository resolution policy:\n")
		entries := repoconfig.ListPolicy(cfg, reg)
		policy := []string{}
		locators := []string{}
		if len(entries) == 0 {
			description.WriteString("(empty)\n")
		}
		for _, e := range entries {
			reason := "available"
			if e.UnavailableReason != nil {
				reason = *e.UnavailableReason
			}
			fmt.Fprintf(&description, "%d  %s  %s\n", e.Position, pluginadmin.SafeText(e.Ref), reason)
			policy = append(policy, e.Ref)
		}
		for _, l := range repoconfig.ListLocators(cfg, reg) {
			locators = append(locators, l.Ref)
		}
		fmt.Fprintf(&description, "Search roots (independent of workspace location): %s\n", pluginadmin.SafeText(joinOrNone(cfg.RepositoryRoots)))
		options := []present.Option[string]{
			{Value: "locator-list", Primary: "List installed Locators", Secondary: "work repository locator list [--json]"},
			{Value: "policy-list", Primary: "List policy", Secondary: "work repository policy list [--json]"},
			{Value: "policy-add", Primary: "Add policy Locator", Secondary: "work repository policy add LOCATOR [--before REF | --after REF]"},
			{Value: "policy-remove", Primary: "Remove policy Locators", Secondary: "work repository policy remove LOCATOR..."},
			{Value: "policy-move", Primary: "Move policy Locator", Secondary: "work repository policy move LOCATOR --before REF | --after REF"},
			{Value: "policy-replace", Primary: "Replace policy order", Secondary: "work repository policy replace LOCATOR..."},
			{Value: "root-list", Primary: "List search roots", Secondary: "work repository root list [--json]"},
			{Value: "root-add", Primary: "Add search roots", Secondary: "work repository root add PATH..."},
			{Value: "root-remove", Primary: "Remove search roots", Secondary: "work repository root remove PATH..."},
			{Value: "root-replace", Primary: "Replace search roots", Secondary: "work repository root replace PATH..."},
			{Value: "", Primary: "Leave unchanged / Exit"},
		}
		action, err := hubChoice(cmd, "Repositories", description.String(), options)
		if err != nil {
			return err
		}
		if action == "" {
			return nil
		}
		family, verb, _ := strings.Cut(action, "-")
		if verb == "list" {
			if err = hubDirect(cmd, "repository", family, "list"); err != nil {
				return err
			}
			continue
		}
		edit := repositoryEdit{Kind: action, Reviewed: slices.Clone(policy)}
		// A nonnil empty review distinguishes an empty collection from no review.
		if edit.Reviewed == nil {
			edit.Reviewed = []string{}
		}
		if family == "root" {
			edit.Reviewed = append([]string{}, cfg.RepositoryRoots...)
		}
		switch {
		case verb == "remove":
			choices := policy
			if family == "root" {
				choices = cfg.RepositoryRoots
			}
			edit.Values, err = hubTargets(cmd, "Remove "+family+" entries", choices)
			if err != nil {
				return err
			}
			if len(edit.Values) == 0 {
				continue
			}
		case action == "policy-add" || action == "policy-move":
			choices := locators
			if verb == "move" {
				choices = policy
			}
			opts := stringOptions(choices)
			opts = append(opts, present.Option[string]{Value: "", Primary: "Back"})
			var selected string
			description := "Disabled installed Locators can be added. Retained unavailable entries can be moved."
			if len(choices) == 0 {
				description = "No Locators to select. Install a provider or configure policy first."
			}
			selected, err = hubChoice(cmd, "Select policy Locator", description, opts)
			if err != nil {
				return err
			}
			if selected == "" {
				continue
			}
			edit.Values = []string{selected}
			positions := []present.Option[string]{}
			if verb == "add" {
				positions = append(positions, present.Option[string]{Value: "end", Primary: "Append at end"})
			}
			for _, ref := range policy {
				if ref != selected {
					positions = append(positions, present.Option[string]{Value: "before:" + ref, Primary: "Before " + ref}, present.Option[string]{Value: "after:" + ref, Primary: "After " + ref})
				}
			}
			positions = append(positions, present.Option[string]{Value: "", Primary: "Back"})
			var pos string
			pos, err = hubChoice(cmd, "Policy position", "Position translates to --before / --after.", positions)
			if err != nil {
				return err
			}
			if pos == "" {
				continue
			}
			if strings.HasPrefix(pos, "before:") {
				edit.Before = strings.TrimPrefix(pos, "before:")
			}
			if strings.HasPrefix(pos, "after:") {
				edit.After = strings.TrimPrefix(pos, "after:")
			}
		default:
			var back bool
			edit.Values, back, err = hubRepositoryCollection(cmd, cfg, reg, edit, locators)
			if err != nil {
				return err
			}
			if back {
				continue
			}
		}
		preview, err := previewRepositoryEdit(cfg, reg, edit)
		if err != nil {
			return err
		}
		args := []string{"repository", family, verb}
		args = append(args, edit.Values...)
		if edit.Before != "" {
			args = append(args, "--before", edit.Before)
		}
		if edit.After != "" {
			args = append(args, "--after", edit.After)
		}
		// Direct remove expresses clearing an entire collection, including its last entry.
		if verb == "replace" && len(edit.Values) == 0 {
			if len(edit.Reviewed) == 0 {
				continue
			}
			args = []string{"repository", family, "remove"}
			args = append(args, edit.Reviewed...)
		}
		result := preview.RepositoryResolution.Locators
		if family == "root" {
			result = preview.RepositoryRoots
		}
		accepted, err := present.Confirm(cmd.Context(), present.IO{In: cmd.InOrStdin(), UI: cmd.ErrOrStderr()}, present.ConfirmSpec{Title: "Review repository change", Impact: safeCommand(args) + "\nResult: " + pluginadmin.SafeText(joinOrNone(result)), Accept: "Apply", Reject: "Back"})
		if err != nil {
			return err
		}
		if !accepted {
			continue
		}
		if err = mutationPreflight(cmd, home); err != nil {
			return err
		}
		final, err := applyRepositoryEdit(cmd.Context(), home, edit)
		if err != nil {
			return err
		}
		result = final.RepositoryResolution.Locators
		label := "policy"
		if family == "root" {
			result = final.RepositoryRoots
			label = "roots"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "work: %s now %s\nwork: (equivalent: %s)\n", label, pluginadmin.SafeText(joinOrNone(result)), safeCommand(args))
	}
}

// Preview uses the direct domain validators on detached collections.
func previewRepositoryEdit(cfg *config.Config, reg *registry.Registry, edit repositoryEdit) (*config.Config, error) {
	clone := *cfg
	clone.RepositoryResolution.Locators = slices.Clone(cfg.RepositoryResolution.Locators)
	clone.RepositoryRoots = slices.Clone(cfg.RepositoryRoots)
	var err error
	switch edit.Kind {
	case "policy-add":
		err = repoconfig.AddPolicy(&clone, reg, edit.Values[0], edit.Before, edit.After)
	case "policy-remove":
		err = repoconfig.RemovePolicy(&clone, edit.Values)
	case "policy-move":
		err = repoconfig.MovePolicy(&clone, edit.Values[0], edit.Before, edit.After)
	case "policy-replace":
		err = repoconfig.ReplacePolicy(&clone, reg, edit.Values)
	case "root-add":
		err = repoconfig.AddRoots(&clone, edit.Values)
	case "root-remove":
		err = repoconfig.RemoveRoots(&clone, edit.Values)
	case "root-replace":
		err = repoconfig.ReplaceRoots(&clone, edit.Values)
	}
	return &clone, err
}

// Build replacements by accepting entries in order, using the same selectors
// and validated path input as ordinary single-entry actions.
func hubRepositoryCollection(cmd *cobra.Command, cfg *config.Config, reg *registry.Registry, edit repositoryEdit, locators []string) ([]string, bool, error) {
	values := []string{}
	isRoot := strings.HasPrefix(edit.Kind, "root-")
	for {
		choices := slices.Clone(edit.Reviewed)
		if !isRoot {
			for _, ref := range locators {
				if !slices.Contains(choices, ref) {
					choices = append(choices, ref)
				}
			}
		}
		options := []present.Option[string]{}
		for _, value := range choices {
			if !slices.Contains(values, value) {
				options = append(options, present.Option[string]{Value: "entry:" + value, Primary: pluginadmin.SafeText(value)})
			}
		}
		if isRoot {
			options = append(options, present.Option[string]{Value: "input", Primary: "Enter directory path"})
		}
		options = append(options, present.Option[string]{Value: "review", Primary: "Review selected entries"}, present.Option[string]{Value: "back", Primary: "Back"})
		title := "Policy order"
		description := "Choose entries in traversal order."
		if isRoot {
			title = "Search roots"
			description = "Choose repository search directories, independent of workspace location."
		}
		selected, err := hubChoice(cmd, title, description+"\nSelected: "+pluginadmin.SafeText(joinOrNone(values)), options)
		if err != nil {
			return nil, false, err
		}
		switch selected {
		case "back":
			return nil, true, nil
		case "review":
			return values, false, nil
		case "input":
			path, err := hubInput(cmd, present.InputSpec{Title: "Search root directory", Description: "Enter an existing directory. Leave empty to return to selected entries.", Validate: func(_ context.Context, v string) error {
				if v == "" {
					return nil
				}
				candidate := edit
				candidate.Values = append(slices.Clone(values), v)
				_, e := previewRepositoryEdit(cfg, reg, candidate)
				return e
			}})
			if err != nil {
				return nil, false, err
			}
			if path != "" {
				values = append(values, path)
			}
		default:
			values = append(values, strings.TrimPrefix(selected, "entry:"))
		}
	}
}
