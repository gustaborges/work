package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gustaborges/work/internal/basebranch"
	"github.com/gustaborges/work/internal/bootstrap"
	"github.com/gustaborges/work/internal/branchname"
	"github.com/gustaborges/work/internal/config"
	"github.com/gustaborges/work/internal/convention"
	"github.com/gustaborges/work/internal/create"
	"github.com/gustaborges/work/internal/diag"
	"github.com/gustaborges/work/internal/gitx"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/reporef"
	"github.com/gustaborges/work/internal/shellintegration"
	"github.com/gustaborges/work/internal/starter"
	"github.com/gustaborges/work/internal/tui"
	"github.com/gustaborges/work/internal/workhome"
	"github.com/gustaborges/work/internal/workspace"
)

type startFlags struct {
	workspace    string
	base         string
	slug         string
	prefix       string
	yes          bool
	workspaceSet bool
	baseSet      bool
	slugSet      bool
	prefixSet    bool
	jsonSet      bool
}

// newStartCmd registers `work start [SOURCE]`.
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "start [SOURCE]",
		Short:         "Create a new Work from a local git repository",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			f := startFlags{}
			f.workspace, _ = cmd.Flags().GetString("workspace")
			f.base, _ = cmd.Flags().GetString("base")
			f.slug, _ = cmd.Flags().GetString("slug")
			f.prefix, _ = cmd.Flags().GetString("prefix")
			f.yes, _ = cmd.Flags().GetBool("yes")
			f.workspaceSet = cmd.Flags().Changed("workspace")
			f.baseSet = cmd.Flags().Changed("base")
			f.slugSet = cmd.Flags().Changed("slug")
			f.prefixSet = cmd.Flags().Changed("prefix")
			f.jsonSet = cmd.Flags().Changed("json")

			var source string
			if len(args) == 1 {
				source = args[0]
			}
			return runStart(cmd, source, f)
		},
	}
	cmd.Flags().String("workspace", "", "workspace root for this run")
	cmd.Flags().String("base", "", "base branch to start from")
	cmd.Flags().String("slug", "", "short identifier for the Work")
	cmd.Flags().String("prefix", "", "branch-convention prefix")
	cmd.Flags().Bool("yes", false, "skip the confirmation prompt")
	return cmd
}

func runStart(cmd *cobra.Command, source string, f startFlags) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	interactive := tui.IsInteractive()

	if f.jsonSet {
		return diag.New(diag.Usage, "--json is not accepted on `work start` (it is a mutation)")
	}

	home, err := workhome.Resolve()
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot locate the Work home directory")
	}

	// Config is read-only here; a missing file yields defaults with no write.
	cfg, err := config.Load(home.ConfigFile())
	if err != nil {
		return err
	}

	// Non-interactive runs must supply every required value up front: a missing
	// one fails before any mutation, bootstrap, or config write (FR-024, S9).
	if !interactive {
		if err := requireNonInteractiveFlags(source, cfg, f); err != nil {
			return err
		}
	}

	// Preconditions: seed present, git usable.
	if err := bootstrap.EnsureSeed(home); err != nil {
		return err
	}
	if err := gitx.Preflight(); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "git is required but not usable")
	}

	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}

	// 1. SOURCE.
	if strings.TrimSpace(source) == "" {
		if !interactive {
			return diag.New(diag.Usage, "no SOURCE given: pass a path to a local git repository")
		}
		source, err = tui.InputPath()
		if err != nil {
			return err
		}
	}

	// 2. Resolve via the Starter, then validate the path directly.
	starterComp, err := starter.Select(reg)
	if err != nil {
		return err
	}
	ref, err := starter.Invoke(home.PluginsDir(), starterComp, source)
	if err != nil {
		return err
	}
	repoPath, err := reporef.ValidatePath(ref.Path)
	if err != nil {
		return err
	}
	repo := gitx.Open(repoPath)
	repoName := filepath.Base(repoPath)

	// 3. Workspace root.
	workspaceRoot, err := resolveWorkspace(home, cfg, f, interactive)
	if err != nil {
		return err
	}

	// 4. Base branch.
	choices, err := basebranch.List(repo)
	if err != nil {
		return err
	}
	if len(choices) == 0 {
		return diag.New(diag.NoBaseBranch, "the repository has no selectable base branch")
	}
	var base basebranch.Choice
	if f.baseSet {
		base, err = basebranch.Resolve(choices, f.base)
	} else if interactive {
		base, err = selectBase(choices)
	} else {
		return diag.New(diag.Usage, "missing --base: name a base branch")
	}
	if err != nil {
		return err
	}

	// 5. Prefix (freeform convention).
	catalog := convention.Load(reg)
	prefixes, err := catalog.Prefixes(convention.Freeform)
	if err != nil {
		return err
	}
	var prefix string
	switch {
	case f.prefixSet:
		prefix = f.prefix
	case interactive:
		prefix, err = tui.SelectPrefix(prefixes)
		if err != nil {
			return err
		}
	default:
		return diag.New(diag.Usage, "missing --prefix: name a branch prefix")
	}

	// 6. Slug.
	var slug string
	switch {
	case f.slugSet:
		slug = strings.TrimSpace(f.slug)
	case interactive:
		slug, err = tui.InputSlug()
		if err != nil {
			return err
		}
	default:
		return diag.New(diag.Usage, "missing --slug: name the Work")
	}

	// 7. Derive + validate the branch name (before any mutation).
	branch, err := catalog.DeriveName(convention.Freeform, prefix, slug)
	if err != nil {
		return err
	}
	if err := branchname.Validate(branch); err != nil {
		return err
	}
	if err := branchname.DetectCollision(repo, branch); err != nil {
		return err
	}

	// 8. Confirm.
	dirPath := filepath.Join(workspaceRoot, "in-progress", repoName+"_"+strings.ReplaceAll(branch, "/", "-"))
	summary := fmt.Sprintf(
		"Create Work\n  repository: %s\n  base:       %s\n  branch:     %s\n  workspace:  %s\n  directory:  %s",
		repoPath, base.Format(), branch, workspaceRoot, dirPath)
	if !f.yes {
		if !interactive {
			return diag.New(diag.Usage, "missing --yes: confirm the creation non-interactively with --yes")
		}
		ok, err := tui.ConfirmCreate(summary)
		if err != nil {
			return err
		}
		if !ok {
			return diag.New(diag.Cancelled, "creation declined at the confirmation prompt")
		}
	}

	// 9. Materialize.
	res, err := create.Run(ctx, create.Params{
		Home:            home,
		SourceRepo:      repoPath,
		RepoName:        repoName,
		WorkspaceRoot:   workspaceRoot,
		Slug:            slug,
		Branch:          branch,
		BaseRefname:     base.Refname,
		BaseBranchShort: base.Short,
		Convention:      convention.Freeform,
		Starter:         starter.LogicalName,
	})
	if err != nil {
		return err
	}

	// 10. Report.
	baseObj := res.BaseObject
	if baseObj == "" {
		baseObj = base.ObjectShort
	}
	fmt.Fprintf(out, "work: created %s\n", res.WorkID)
	fmt.Fprintf(out, "work: branch %s  (from %s @ %s)\n", branch, base.Short, baseObj)
	fmt.Fprintf(out, "work: path %s\n", res.WorktreePath)

	if shellintegration.Active() {
		if err := shellintegration.WriteTargetPath(res.WorktreePath); err != nil {
			return err
		}
	} else {
		shellintegration.ReportNoIntegration(errOut, res.WorktreePath, shellintegration.DetectShell())
	}
	return nil
}

// requireNonInteractiveFlags enforces that a non-interactive run carries every
// value the flow needs, naming the first missing one.
func requireNonInteractiveFlags(source string, cfg *config.Config, f startFlags) error {
	if strings.TrimSpace(source) == "" {
		return diag.New(diag.Usage, "no SOURCE given: pass a path to a local git repository")
	}
	if strings.TrimSpace(cfg.Workspace) == "" && !f.workspaceSet {
		return diag.New(diag.Usage, "missing --workspace: no workspace root is configured yet")
	}
	if !f.baseSet {
		return diag.New(diag.Usage, "missing --base: name a base branch")
	}
	if !f.prefixSet {
		return diag.New(diag.Usage, "missing --prefix: name a branch prefix")
	}
	if !f.slugSet {
		return diag.New(diag.Usage, "missing --slug: name the Work")
	}
	if !f.yes {
		return diag.New(diag.Usage, "missing --yes: confirm the creation non-interactively with --yes")
	}
	return nil
}

// resolveWorkspace applies the F1 workspace-root rule: reuse a configured root
// silently; on a machine with none, persist the supplied/prompted value.
func resolveWorkspace(home workhome.Home, cfg *config.Config, f startFlags, interactive bool) (string, error) {
	if strings.TrimSpace(cfg.Workspace) != "" {
		if f.workspaceSet {
			return "", diag.New(diag.Usage,
				"a workspace root is already configured; edit ~/.work/config/work.json to change it")
		}
		return cfg.Workspace, nil
	}

	var raw string
	switch {
	case f.workspaceSet:
		raw = f.workspace
	case interactive:
		suggested, err := workspace.SuggestDefault()
		if err != nil {
			return "", diag.Wrap(diag.Usage, err, "cannot suggest a workspace root")
		}
		raw, err = tui.EditWorkspaceRoot(suggested)
		if err != nil {
			return "", err
		}
	default:
		return "", diag.New(diag.Usage, "missing --workspace: no workspace root is configured yet")
	}

	abs, err := workspace.Validate(raw, cfg.RepositoryRoots)
	if err != nil {
		return "", err
	}
	if err := workspace.Persist(home, abs); err != nil {
		return "", err
	}
	return abs, nil
}

func selectBase(choices []basebranch.Choice) (basebranch.Choice, error) {
	labels := make([]string, len(choices))
	for i, c := range choices {
		scope := "local"
		if c.Scope == basebranch.ScopeRemoteTracking {
			scope = "remote"
		}
		labels[i] = fmt.Sprintf("%-24s %s  [%s]", c.Short, c.ObjectShort, scope)
	}
	idx, err := tui.SelectBaseBranch(labels)
	if err != nil {
		return basebranch.Choice{}, err
	}
	return choices[idx], nil
}
