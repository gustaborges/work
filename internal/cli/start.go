package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
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
	"github.com/gustaborges/work/internal/locator"
	"github.com/gustaborges/work/internal/present"
	"github.com/gustaborges/work/internal/registry"
	"github.com/gustaborges/work/internal/repoconfig"
	"github.com/gustaborges/work/internal/repoconv"
	"github.com/gustaborges/work/internal/repoidentity"
	"github.com/gustaborges/work/internal/reporef"
	"github.com/gustaborges/work/internal/shellintegration"
	"github.com/gustaborges/work/internal/starter"
	"github.com/gustaborges/work/internal/work"
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
	interactive := present.IsInteractive()
	pio := present.IO{In: cmd.InOrStdin(), UI: errOut}

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
	// EnsureSeed may have just appended the seed Locator to the policy (first
	// run only, bootstrap.LocatorPolicyEntry); reload so locatorDeps below
	// reflects it instead of the pre-bootstrap snapshot.
	cfg, err = config.Load(home.ConfigFile())
	if err != nil {
		return err
	}
	if err := gitx.Preflight(); err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "git is required but not usable")
	}

	reg, err := registry.Load(home.RegistryFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot read the component registry")
	}

	catalog := convention.Load(reg)

	// convStore is the per-repository-identity branch-convention memory
	// (ADR-0011, research R14): loaded once, read/written by resolveConvention
	// and the interactive Convention step below.
	convStore, err := repoconv.Load(home.BranchConventionsFile())
	if err != nil {
		return diag.Wrap(diag.BootstrapFailed, err, "cannot read the branch convention memory")
	}

	// First-run setup (interactive only, before any other step): ensure a
	// workspace root and at least one repository search root exist, each with
	// a purpose line. First-run only — once both are configured neither
	// prompt appears again and an already-configured install is
	// byte-identical to F1/F2.5. Persisted as a single config.Save here,
	// before the eager SOURCE-argv resolution below, so an explicit
	// `work start <name>` also resolves through the just-configured root
	// (contracts/cli-work-start.md §First-run setup, research R21).
	if interactive {
		if err := runFirstRunSetup(ctx, pio, home, cfg, f); err != nil {
			return err
		}
	}

	// Values the flow collects. The validation closures side-effect these as
	// each step is accepted, so the interactive wizard and the non-interactive
	// resolution below share exactly the same rules (FR-005, FR-029).
	var (
		repoPath         string
		repoName         string
		prefix           string
		slug, branch     string
		base             basebranch.Choice
		baseResolved     bool
		workspaceRoot    string
		candidates       []locator.Candidate
		ambiguityPending bool
		// wizardActive is true once the interactive wizard runs: a nested
		// present.Select cannot open then, so validatePath reuses the collision
		// choice already made for pickedFor instead of asking again.
		wizardActive bool
		pickedFor    string
		// initialSourceErr is why the argv SOURCE did not resolve before the
		// wizard opened; the path step shows it instead of failing silently.
		initialSourceErr error
		// chosenStarter is the Starter internal/starter.Match resolved for
		// SOURCE (F4) — the reference fallback in F1/F3, or a plugin-provided
		// specific Starter once one is installed and matches.
		chosenStarter registry.Component
		// starterBaseBranch and starterStartModes are the F4 fields of the
		// matched Starter's response (contracts/starter-protocol.md),
		// populated once validatePath's Invoke succeeds. Consumed by
		// resolveBase/resolveContribution and the interactive Mode step below.
		starterBaseBranch string
		starterStartModes []string
		// starterMeta and starterLinks are the validated context the chosen
		// Starter published; they seed the Work's first snapshot.
		starterMeta  map[string]any
		starterLinks map[string]string
		// startMode is the resolved work.start_mode: left empty (create.Run
		// defaults it to "new") when the Starter offered no start_modes, or
		// set to the Mode step's answer otherwise (FR-021/FR-022).
		startMode string
		// conventionValue is the resolved work.branch_convention (F4, research
		// R15): stays empty in contribution mode (never consulted); resolved
		// eagerly once the repository path is known outside contribution mode,
		// or by the interactive Convention step otherwise.
		conventionValue string
		// repoIdentityKey is the ADR-0011 identity computed the first time
		// resolveConvention runs with a known repository path; reused by the
		// interactive Convention step to persist an explicit pick without
		// recomputing it.
		repoIdentityKey string
	)

	// locatorDeps feeds internal/locator.Resolve from the loaded config: the
	// policy and search roots are read-only here, exactly like cfg itself.
	locatorDeps := locator.Deps{
		PluginsDir: home.PluginsDir(),
		Policy:     cfg.RepositoryResolution.Locators,
		Roots:      cfg.RepositoryRoots,
		Registry:   reg,
	}

	// 1+2. SOURCE → Starter → a Repository Reference, then either direct path
	// validation or resolution through the policy. The validation runs inside
	// the interactive field: an invalid/unusable path or an unresolved name is
	// shown in the live frame and replaced on the next attempt, so a rejected
	// value never reaches terminal history and the journey is not restarted
	// (FR-005, S4). A path reference short-circuits to reporef.ValidatePath and
	// never reaches internal/locator (ADR-0014); every other shape resolves
	// through the policy (contracts/cli-work-start.md §Interactive flow).
	//
	// The Starter to invoke is resolved fresh from SOURCE itself (F4, research
	// R7): a collision (>= 2 pattern matches) is resolved by an explicit,
	// unmemoized present.Select interactively, or fails starter-ambiguous (36)
	// non-interactively (ADR-0004, US4).
	validatePath := func(ctx context.Context, s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return errors.New("a path or name is required")
		}
		comp, outcome, merr := starter.Match(reg, s)
		if merr != nil {
			return present.Fatal(merr) // starter-not-matched: no input text fixes a missing fallback
		}
		if outcome.Ambiguous != nil {
			// Non-interactively (or an explicit SOURCE with no TTY), a collision
			// is always fatal — no selector opened, no Work created (FR-012's
			// non-interactive clause, US4 AC4). Interactively, present.Select
			// (the same standalone picker internal/cli/resume.go uses) resolves
			// it before SOURCE's own validation continues; the choice is never
			// memoized (FR-012, SC-006). This runs safely only for an
			// argv-provided SOURCE, resolved eagerly before any present.Wizard
			// is constructed — validatePath is also reused as the interactive
			// path-prompt's own Validate, where a nested full-screen picker
			// would conflict with the already-running wizard program.
			if !interactive {
				return present.Fatal(diag.Newf(diag.StarterAmbiguous,
					"%d Starters matched this argument", len(outcome.Ambiguous)).
					WithSummary("More than one Starter matched this argument.").
					WithHint("Pick one interactively, or narrow the argument so only one Starter's pattern matches."))
			}
			if wizardActive {
				if s != pickedFor || chosenStarter.Name == "" {
					return fmt.Errorf("%d Starters match %q; run `work start %s` to choose one", len(outcome.Ambiguous), s, s)
				}
				comp = chosenStarter
			} else {
				chosen, serr := present.Select(ctx, pio, starterSelectSpec(outcome.Ambiguous))
				if serr != nil {
					return present.Fatal(serr) // Ctrl-C/Esc at the picker -> diag.Cancelled
				}
				comp = chosen
				pickedFor = s
			}
		}
		chosenStarter = comp

		ref, err := starter.Invoke(home.PluginsDir(), comp, s)
		if err != nil {
			if retryable(err, diag.InvalidPath, diag.UnusableRepo) {
				return err // shown in-frame; the user can correct it
			}
			return present.Fatal(err)
		}
		if verr := starter.ValidateResponse(ref, reg.PluginNameOf(comp.Alias)); verr != nil {
			return present.Fatal(verr)
		}
		starterBaseBranch = ref.BaseBranch
		starterStartModes = ref.StartModes
		starterMeta = ref.Meta
		starterLinks = ref.Links

		if ref.Path != "" {
			normalized, verr := reporef.ValidatePath(ref.Path)
			if verr != nil {
				if retryable(verr, diag.InvalidPath, diag.UnusableRepo) {
					return verr
				}
				return present.Fatal(verr)
			}
			repoPath = normalized
			repoName = filepath.Base(normalized)
			return nil
		}

		out, rerr := locator.Resolve(ctx, locatorDeps, locator.Reference{
			GitFetchURLs: ref.GitFetchURLs,
			Name:         ref.Name,
			Query:        ref.Query,
		})
		if rerr != nil {
			// no-repository-found is the one outcome the interactive step
			// recovers from in-frame (research R5, R9); the other categories
			// (no-eligible-locator, repository-candidate-invalid,
			// locator-failed) are configuration/operational and terminal.
			if retryable(rerr, diag.NoRepositoryFound) {
				return rerr
			}
			return present.Fatal(rerr)
		}
		if out.Resolved != "" {
			repoPath = out.Resolved
			repoName = filepath.Base(out.Resolved)
			return nil
		}
		// Ambiguous outcome (>= 2 candidates): accept the step and stash the
		// candidates. The caller decides what happens next: interactively, the
		// conditional Repository present.Select step below renders them;
		// non-interactively (or an explicit-argv SOURCE with no TTY), the
		// caller maps this to repository-ambiguous (30) with no selector
		// opened (FR-011, FR-013, FR-033, research R9).
		candidates = out.Candidates
		ambiguityPending = true
		return nil
	}

	// ambiguousErr is the repository-ambiguous (30) diagnostic for the
	// non-interactive / explicit-argv path (contracts/cli-work-start.md
	// §Non-interactive flow, FR-033).
	ambiguousErr := func() error {
		return diag.New(diag.RepositoryAmbiguous,
			fmt.Sprintf("%d repositories matched this reference", len(candidates))).
			WithSummary("More than one local clone matched what you typed.").
			WithHint("Pass a more specific reference, or adjust repository_roots.")
	}

	// 4+5. Slug → derived branch name, validated and collision-checked before
	// any mutation. The check runs inside the interactive field, so an invalid
	// name or a collision is shown in-frame and replaced on the next attempt
	// without restarting the journey (FR-005, S5, S6).
	validateSlug := func(_ context.Context, s string) error {
		s = strings.TrimSpace(s)
		if err := branchname.ValidateSlug(s); err != nil {
			return err
		}
		derived, err := catalog.DeriveName(conventionValue, prefix, s)
		if err == nil {
			err = branchname.Validate(derived)
		}
		if err == nil {
			err = branchname.DetectCollision(gitx.Open(repoPath), derived)
		}
		if err != nil {
			if retryable(err, diag.InvalidBranchName, diag.BranchCollision) {
				return err
			}
			return present.Fatal(err)
		}
		slug, branch = s, derived
		return nil
	}

	// 7. Workspace root: reuse a configured root silently; on a machine with
	// none, validate the supplied/prompted value and persist it after the run.
	validateWorkspace := func(_ context.Context, raw string) error {
		a, verr := workspace.Validate(raw, cfg.RepositoryRoots)
		if verr != nil {
			return verr // a path typo the user can correct in-frame
		}
		workspaceRoot = a
		return nil
	}

	// resolveBase turns --base, or a Starter-supplied base_branch (F4,
	// FR-023/FR-024), into a Choice once the repository path is known: an
	// explicit flag takes priority (unchanged F1/F3 behaviour); absent that, a
	// Starter's base_branch is used directly so the base-branch step never
	// renders. Neither present leaves base unresolved for the interactive step
	// to prompt for. The explicit state marker, rather than a property of
	// Choice, is the sole resolution guard; an empty refname can therefore
	// never cause a valid choice to be resolved again — safe to call
	// speculatively at multiple points in the flow.
	resolveBase := func() error {
		if baseResolved || repoPath == "" {
			return nil
		}
		refname := ""
		switch {
		case f.baseSet:
			refname = f.base
		case starterBaseBranch != "":
			refname = starterBaseBranch
		default:
			return nil
		}
		choices, err := basebranch.List(gitx.Open(repoPath))
		if err != nil {
			return err
		}
		if len(choices) == 0 {
			return diag.New(diag.NoBaseBranch, "the repository has no selectable base branch")
		}
		base, err = basebranch.Resolve(choices, refname)
		if err == nil {
			baseResolved = true
		}
		return err
	}

	// resolveContribution finalizes slug/branch/base once the Mode step
	// resolves to "contribution" (research R12, FR-020): branch is exactly the
	// branch the Starter resolved — base_branch doubles as the checkout
	// target, ADD §7's single field serving both purposes — slug stays empty,
	// and no prefix/convention step ever renders. branch is the local name of
	// the resolved choice, so a remote-only branch is checked out as a local
	// tracking branch. It always resolves from
	// starterBaseBranch, never from --base: the checked-out branch in this
	// mode comes from the Starter, not the user, even if --base was also
	// given (contracts/starter-protocol.md).
	resolveContribution := func() error {
		choices, err := basebranch.List(gitx.Open(repoPath))
		if err != nil {
			return err
		}
		if len(choices) == 0 {
			return diag.New(diag.NoBaseBranch, "the repository has no selectable base branch")
		}
		b, err := basebranch.Resolve(choices, starterBaseBranch)
		if err != nil {
			return err
		}
		base, baseResolved = b, true
		slug, branch = "", base.LocalName()
		return nil
	}

	// resolveConvention determines work.branch_convention once the repository
	// path is known, outside contribution mode (research R15, ADR-0011): a
	// memoized per-repository choice if one exists; the catalog's sole entry,
	// silently adopted and memoized, if there is exactly one; otherwise left
	// unresolved for the interactive Convention step to prompt for (there is
	// no non-interactive convention flag in F4, mirroring the start_modes
	// gap). A no-op once resolved, or before the repository path is known —
	// safe to call speculatively at multiple points in the flow, exactly like
	// resolveBase.
	resolveConvention := func() error {
		if conventionValue != "" || repoPath == "" {
			return nil
		}
		id, err := repoidentity.Identify(gitx.Open(repoPath))
		if err != nil {
			return err
		}
		repoIdentityKey = id
		if v, ok := convStore.Get(id); ok {
			conventionValue = v
			return nil
		}
		names := conventionNames(reg)
		if len(names) != 1 {
			return nil
		}
		conventionValue = names[0]
		convStore.Set(id, conventionValue)
		return repoconv.Save(home.BranchConventionsFile(), convStore)
	}

	// Resolve every value an explicit flag or the SOURCE argument already
	// supplies; the interactive wizard then prompts only for what is missing.
	if source != "" {
		if err := validatePath(ctx, source); err != nil {
			if underlying, fatal := present.IsFatal(err); fatal {
				return underlying
			}
			initialSourceErr = err
			if !interactive {
				return err
			}
		}
		// Non-interactive (or an explicit SOURCE with no TTY to prompt on)
		// cannot open the Repository selector: an ambiguous match fails here
		// with no Work, branch, worktree, or config write (FR-033, SC-010).
		if ambiguityPending && !interactive {
			return ambiguousErr()
		}
		// start_modes presence has no non-interactive mode-selection flag in
		// F4 (Out of Scope, contracts/cli-work-start.md §New clause: start
		// modes) — a non-interactive invocation against such a Starter
		// response fails with an actionable usage error rather than silently
		// defaulting to a mode.
		if len(starterStartModes) > 0 && !interactive {
			return diag.New(diag.Usage,
				"this Starter offers a start-mode choice (contribution/fork); no non-interactive flag selects one yet").
				WithHint("Run `work start` in an interactive terminal to choose a mode.")
		}
		// Outside contribution mode (guaranteed here: start_modes is absent,
		// so the mode is unambiguously "new" — contribution/fork only ever
		// arise from a Starter's start_modes), the convention can be resolved
		// as soon as the repository path is known, the same eager-resolution
		// window F1/F3 already use for slug/base (research R15). F4 adds no
		// non-interactive flag to pick among 2+ unmemoized conventions,
		// mirroring the start_modes gap above.
		if len(starterStartModes) == 0 && !ambiguityPending {
			if err := resolveConvention(); err != nil {
				return err
			}
			if !interactive && conventionValue == "" {
				return diag.New(diag.Usage,
					"more than one branch convention is enabled and none is memoized for this repository").
					WithHint("run `work start` in an interactive terminal to choose one, or `work convention set <convention>` beforehand")
			}
		}
	}

	switch {
	case f.prefixSet:
		prefix = f.prefix
	case conventionValue != "":
		if names, perr := catalog.Prefixes(conventionValue); perr == nil && len(names) == 1 {
			// A convention with a single prefix is not a choice.
			prefix = names[0]
		}
	}

	// Slug validation needs the repository (for the collision check), so it can
	// only run eagerly once the path is known — otherwise the slug becomes a
	// wizard step whose build runs the same closure after the path step. When
	// the Starter offered start_modes, the mode itself isn't resolved yet (it
	// is a wizard step below) and contribution mode never runs slug at all —
	// so eager validation is deferred to the slug step's own build closure,
	// which re-checks the resolved mode first (FR-020).
	if f.slugSet && repoPath != "" && len(starterStartModes) == 0 && conventionValue != "" {
		if err := validateSlug(ctx, f.slug); err != nil {
			if underlying, fatal := present.IsFatal(err); fatal {
				return underlying
			}
			if !interactive {
				return err
			}
		}
	}

	if err := resolveBase(); err != nil {
		return err
	}

	persistWorkspace := false
	switch {
	case strings.TrimSpace(cfg.Workspace) != "":
		if f.workspaceSet {
			return diag.New(diag.Usage,
				"a workspace root is already configured; edit ~/.work/config/work.json to change it")
		}
		workspaceRoot = cfg.Workspace
	case f.workspaceSet:
		a, verr := workspace.Validate(f.workspace, cfg.RepositoryRoots)
		if verr != nil {
			return verr
		}
		workspaceRoot, persistWorkspace = a, true
	}

	// 8. Confirmation is the last collected step; it is also required (with
	// --yes) for a non-interactive run.
	needConfirm := !f.yes

	if interactive {
		var steps []present.Step
		wizardActive = true

		if repoPath == "" && !ambiguityPending {
			steps = append(steps, present.InputStep("path", func(present.Answers) (present.InputSpec, error) {
				return present.InputSpec{
					Title:        "Local repository path",
					Initial:      strings.TrimSpace(source),
					Validate:     validatePath,
					InitialError: initialSourceErrText(initialSourceErr),
					Receipt: func(accepted string) string {
						if repoPath != "" {
							return repoPath
						}
						return accepted // ambiguous: the step shows what was typed
					},
				}, nil
			}))
		}

		// A conditional Repository step: skipped without rendering unless the
		// Source step just resolved to >= 2 candidates (research R9,
		// contracts/cli-work-start.md §Interactive flow). It is always present
		// in the step list — never resumed, never re-added — because the
		// ambiguity is only known once the Source step above has run.
		steps = append(steps, present.SelectStep("repository", func(present.Answers) (present.SelectSpec[locator.Candidate], error) {
			if !ambiguityPending {
				return present.SelectSpec[locator.Candidate]{}, present.StepResolved(locator.Candidate{Path: repoPath})
			}
			return candidateSelectSpec(candidates), nil
		}))

		// A conditional Mode step (F4, contracts/cli-work-start.md §New
		// clause: start modes): rendered only when the resolved Starter
		// offered start_modes, listing exactly those values and no others
		// (ADR-0004). Absent start_modes, it silently resolves to "new" — the
		// unchanged F1/F3 journey — with no step shown.
		steps = append(steps, present.SelectStep("mode", func(present.Answers) (present.SelectSpec[string], error) {
			if len(starterStartModes) == 0 {
				return present.SelectSpec[string]{}, present.StepResolved(work.StartModeNew)
			}
			return present.SelectSpec[string]{
				Title:   "Mode",
				Options: stringOptions(starterStartModes),
			}, nil
		}))

		// A conditional Convention step (F4, contracts/cli-work-start.md §New
		// clause: convention resolution generalizes, research R15): skipped
		// entirely in contribution mode; resolved silently via
		// resolveConvention when memoized or the catalog has exactly one
		// entry; otherwise a present.SelectStep offering the enabled catalog.
		// Always present in the step list — its build decides whether to
		// render, exactly like the Repository/Mode steps above.
		steps = append(steps, present.SelectStep("convention", func(a present.Answers) (present.SelectSpec[string], error) {
			if rv, ok := a.Value("repository").(locator.Candidate); ok && rv.Path != "" {
				repoPath, repoName = rv.Path, filepath.Base(rv.Path)
			}
			if a.String("mode") == work.StartModeContribution {
				return present.SelectSpec[string]{}, present.StepResolved("")
			}
			if err := resolveConvention(); err != nil {
				return present.SelectSpec[string]{}, present.Fatal(err)
			}
			if conventionValue != "" {
				return present.SelectSpec[string]{}, present.StepResolved(conventionValue)
			}
			return present.SelectSpec[string]{
				Title:   "Convention",
				Options: stringOptions(conventionNames(reg)),
			}, nil
		}))

		// The Prefix step is always present too: its own build decides whether
		// a chosen convention needs a prefix choice, and — the first time a
		// repository's convention was just picked interactively above — it is
		// where that pick is persisted, before the wizard advances past this
		// step (contract's "before the wizard advances" rule): the Convention
		// step's build cannot itself persist a fresh interactive pick, since
		// its own build runs before the user has chosen anything.
		steps = append(steps, present.SelectStep("prefix", func(a present.Answers) (present.SelectSpec[string], error) {
			if a.String("mode") == work.StartModeContribution {
				return present.SelectSpec[string]{}, present.StepResolved("")
			}
			if conventionValue == "" {
				conventionValue = a.String("convention")
				convStore.Set(repoIdentityKey, conventionValue)
				if err := repoconv.Save(home.BranchConventionsFile(), convStore); err != nil {
					return present.SelectSpec[string]{}, present.Fatal(err)
				}
			}
			if prefix != "" {
				return present.SelectSpec[string]{}, present.StepResolved(prefix)
			}
			prefixes, err := catalog.Prefixes(conventionValue)
			if err != nil {
				return present.SelectSpec[string]{}, present.Fatal(err)
			}
			if len(prefixes) == 1 {
				return present.SelectSpec[string]{}, present.StepResolved(prefixes[0])
			}
			return present.SelectSpec[string]{
				Title:   "Branch prefix",
				Options: stringOptions(prefixes),
			}, nil
		}))

		if branch == "" {
			steps = append(steps, present.InputStep("slug", func(a present.Answers) (present.InputSpec, error) {
				// The Repository step (immediately before Mode) resolves an
				// ambiguous match; sync it here — before this step's own build
				// runs — so the collision check below (and resolveContribution,
				// which needs repoPath) sees the chosen repository, not the
				// pre-selection empty repoPath.
				if rv, ok := a.Value("repository").(locator.Candidate); ok && rv.Path != "" {
					repoPath, repoName = rv.Path, filepath.Base(rv.Path)
				}
				// Contribution mode never runs slug/convention/prefix (FR-020):
				// branch is exactly the Starter-resolved branch, checked out
				// directly.
				if a.String("mode") == work.StartModeContribution {
					if err := resolveContribution(); err != nil {
						return present.InputSpec{}, present.Fatal(err)
					}
					return present.InputSpec{}, present.StepResolved(slug)
				}
				if p := a.String("prefix"); p != "" {
					prefix = p
				}
				// --slug can only be resolved here (not eagerly) when SOURCE was
				// prompted: the collision check needs the repository path.
				if f.slugSet {
					switch err := validateSlug(ctx, f.slug); {
					case err == nil:
						return present.InputSpec{}, present.StepResolved(slug)
					default:
						if underlying, fatal := present.IsFatal(err); fatal {
							return present.InputSpec{}, underlying
						}
						// a retryable error (bad name / collision): fall through
						// and let the user correct the prefilled value in-frame.
					}
				}
				return present.InputSpec{
					Title:       "Slug",
					Description: "a short identifier for this Work",
					Initial:     strings.TrimSpace(f.slug),
					Validate:    validateSlug,
					Receipt:     func(string) string { return slug },
				}, nil
			}))
		}

		if base.Refname == "" {
			steps = append(steps, present.SelectStep("base", func(present.Answers) (present.SelectSpec[basebranch.Choice], error) {
				// A Starter-supplied base_branch is used directly, same as
				// --base: no prompt (FR-024). Contribution mode never reaches
				// here unresolved — starterBaseBranch is always non-empty by
				// the time a Starter can offer it, so this branch already
				// covers that mode with no separate check.
				if f.baseSet || starterBaseBranch != "" {
					if err := resolveBase(); err != nil {
						return present.SelectSpec[basebranch.Choice]{}, err
					}
					return present.SelectSpec[basebranch.Choice]{}, present.StepResolved(base)
				}
				choices, err := basebranch.List(gitx.Open(repoPath))
				if err != nil {
					return present.SelectSpec[basebranch.Choice]{}, present.Fatal(err)
				}
				if len(choices) == 0 {
					return present.SelectSpec[basebranch.Choice]{}, present.Fatal(
						diag.New(diag.NoBaseBranch, "the repository has no selectable base branch"))
				}
				return baseBranchSpec(choices), nil
			}))
		}

		if strings.TrimSpace(cfg.Workspace) == "" && !f.workspaceSet {
			suggested, serr := workspace.SuggestDefault()
			if serr != nil {
				return diag.Wrap(diag.Usage, serr, "cannot suggest a workspace root")
			}
			steps = append(steps, present.InputStep("workspace", func(present.Answers) (present.InputSpec, error) {
				return present.InputSpec{
					Title:       "Workspace root",
					Description: "where materialized Works are kept",
					Initial:     suggested,
					Validate:    validateWorkspace,
					Receipt:     func(string) string { return workspaceRoot },
				}, nil
			}))
			persistWorkspace = true
		}

		if needConfirm {
			steps = append(steps, present.ConfirmStep("confirm", func(a present.Answers) (present.ConfirmSpec, error) {
				mode := a.String("mode")
				if mode == work.StartModeContribution {
					if err := resolveContribution(); err != nil {
						return present.ConfirmSpec{}, err
					}
				} else {
					if bv, ok := a.Value("base").(basebranch.Choice); ok {
						base = bv
					}
					if err := resolveBase(); err != nil {
						return present.ConfirmSpec{}, err
					}
				}
				return present.ConfirmSpec{
					Title:  "Create Work",
					Impact: startImpact(repoPath, repoName, base, branch, workspaceRoot, mode),
					Accept: "Create",
					Reject: "Cancel",
				}, nil
			}))
		}

		if len(steps) > 0 {
			ans, err := present.Wizard(ctx, pio, present.WizardSpec{Title: "Start a Work", Steps: steps})
			if err != nil {
				return err
			}
			if mv := ans.String("mode"); mv != "" {
				startMode = mv
			}
			if bv, ok := ans.Value("base").(basebranch.Choice); ok {
				base = bv
				baseResolved = true
			}
			if needConfirm && !ans.Bool("confirm") {
				return diag.New(diag.Cancelled, "creation declined at the confirmation prompt")
			}
		}
	}

	if startMode == work.StartModeContribution {
		if err := resolveContribution(); err != nil {
			return err
		}
	} else if err := resolveBase(); err != nil {
		return err
	}

	// The workspace root is persisted only once every source- and name-level
	// check has passed, so a rejected run never records a root (SC-004).
	if persistWorkspace {
		if err := workspace.Persist(home, workspaceRoot); err != nil {
			return err
		}
	}

	// conventionValue is already resolved by this point: eagerly, by the
	// Convention/Prefix wizard steps, or left empty in contribution mode
	// (FR-020) — work-state schema 3 requires branch_convention to be absent
	// exactly then.

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
		Convention:      conventionValue,
		Starter:         starterLogicalName(reg, chosenStarter),
		StartMode:       startMode,

		Meta:             starterMeta,
		Links:            starterLinks,
		StarterComponent: chosenStarter.QualifiedName(),
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

// runFirstRunSetup asks for the workspace root and/or a repository search
// root when either is still unconfigured (contracts/cli-work-start.md
// §First-run setup, research R21). It mutates cfg in place and persists both
// values in a single config.Save before returning; a cancelled prompt
// (Ctrl-C/Esc) returns diag.Cancelled with cfg unchanged on disk. A no-op
// when both are already configured, or when a --workspace flag covers the
// only missing piece.
func runFirstRunSetup(ctx context.Context, pio present.IO, home workhome.Home, cfg *config.Config, f startFlags) error {
	wantWorkspace, wantRoot := repoconfig.NeedsSetup(cfg)
	wantWorkspace = wantWorkspace && !f.workspaceSet
	if !wantWorkspace && !wantRoot {
		return nil
	}

	var setupWorkspace, setupRoot string
	var steps []present.Step

	if wantWorkspace {
		suggested, serr := workspace.SuggestDefault()
		if serr != nil {
			return diag.Wrap(diag.Usage, serr, "cannot suggest a workspace root")
		}
		steps = append(steps, present.InputStep("setup-workspace", func(present.Answers) (present.InputSpec, error) {
			return present.InputSpec{
				Title: "Workspace root",
				Description: "where Work stores and organises worktrees — in-progress and " +
					"archived Works live here, kept separate from your source clones",
				Initial: suggested,
				Validate: func(_ context.Context, raw string) error {
					a, verr := workspace.Validate(raw, cfg.RepositoryRoots)
					if verr != nil {
						return verr // shown in-frame; re-promptable
					}
					// Lay out in-progress/archived now (not deferred to the
					// final persist below) so a search root typed at the next
					// prompt can be checked for overlap against a real
					// directory (quickstart S13: rejecting $WS/in-progress).
					if err := workspace.EnsureLayout(a); err != nil {
						return present.Fatal(err)
					}
					setupWorkspace = a
					return nil
				},
				Receipt: func(string) string { return setupWorkspace },
			}, nil
		}))
	}

	if wantRoot {
		steps = append(steps, present.InputStep("setup-root", func(present.Answers) (present.InputSpec, error) {
			return present.InputSpec{
				Title: "Repository search root",
				Description: "a directory that holds your Git clones, so `work start <name>` can find " +
					"them without a full path — add more later with `work repository root add`",
				Validate: func(_ context.Context, raw string) error {
					probe := *cfg
					if setupWorkspace != "" {
						probe.Workspace = setupWorkspace
					}
					a, verr := repoconfig.ValidateRoot(&probe, raw)
					if verr != nil {
						return verr // shown in-frame; re-promptable (overlap included)
					}
					setupRoot = a
					return nil
				},
				Receipt: func(string) string { return setupRoot },
			}, nil
		}))
	}

	if _, err := present.Wizard(ctx, pio, present.WizardSpec{Title: "Set up Work", Steps: steps}); err != nil {
		return err // Ctrl-C/Esc -> diag.Cancelled (exit 20); nothing persisted
	}

	if setupWorkspace != "" {
		cfg.Workspace = setupWorkspace
	}
	if setupRoot != "" {
		cfg.RepositoryRoots = append(cfg.RepositoryRoots, setupRoot)
	}
	return config.Save(home.ConfigFile(), cfg)
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

// retryable reports whether err is a diag.Error whose category is one the
// interactive flow can recover from by re-prompting.
func retryable(err error, cats ...diag.Category) bool {
	var d *diag.Error
	if !errors.As(err, &d) {
		return false
	}
	return slices.Contains(cats, d.Category)
}

// starterLogicalName is the value persisted to work.starter: c's bare Name,
// unless another registered Starter shares that same bare name, in which
// case it is qualified as "<alias>/<name>" to stay unambiguous (F4, quickstart
// S7). The reference fallback's name never collides in practice, so this is a
// no-op for F1/F3 callers.
func starterLogicalName(reg *registry.Registry, c registry.Component) string {
	count := 0
	for _, other := range reg.ByRole(registry.RoleStarter) {
		if other.Name == c.Name {
			count++
		}
	}
	if count > 1 {
		return c.Alias + "/" + c.Name
	}
	return c.Name
}

// conventionNames returns the enabled branch-convention catalog's names, in
// registration order (the reference package's "freeform" first, then every
// plugin-declared convention in install order).
func conventionNames(reg *registry.Registry) []string {
	convs := reg.ListConventions()
	names := make([]string, len(convs))
	for i, c := range convs {
		names[i] = c.Name
	}
	return names
}

// stringOptions wraps plain strings as single-line select options.
func stringOptions(vals []string) []present.Option[string] {
	opts := make([]present.Option[string], len(vals))
	for i, v := range vals {
		opts[i] = present.Option[string]{Value: v, Primary: v}
	}
	return opts
}

// candidateSelectSpec builds the Repository selector shown only when the
// Source step resolved to >= 2 candidates: primary line the resolved absolute
// path (what actually disambiguates two clones of the same project),
// secondary line the first remote fetch URL or the parent directory
// (locator.Candidate.Remote, research R15). The list collapses to a
// "name (secondary)" receipt on accept (contracts/cli-work-start.md
// §Interactive flow, quickstart S5).
func candidateSelectSpec(candidates []locator.Candidate) present.SelectSpec[locator.Candidate] {
	opts := make([]present.Option[locator.Candidate], len(candidates))
	for i, c := range candidates {
		opts[i] = present.Option[locator.Candidate]{Value: c, Primary: c.Path, Secondary: c.Remote}
	}
	return present.SelectSpec[locator.Candidate]{
		Title:      "Repository",
		Filterable: true,
		Options:    opts,
		Receipt: func(o present.Option[locator.Candidate]) string {
			return fmt.Sprintf("%s (%s)", filepath.Base(o.Value.Path), o.Value.Remote)
		},
	}
}

// starterSelectSpec builds the Starter collision picker (US4, ADR-0004): no
// ranking, primary line the qualified "<alias>/<name>" (the same form
// starterLogicalName persists when a bare name would otherwise collide), no
// secondary line. The choice is never memoized — Select is called fresh on
// every collision (FR-012, SC-006).
func starterSelectSpec(candidates []registry.Component) present.SelectSpec[registry.Component] {
	opts := make([]present.Option[registry.Component], len(candidates))
	for i, c := range candidates {
		opts[i] = present.Option[registry.Component]{Value: c, Primary: c.Alias + "/" + c.Name}
	}
	return present.SelectSpec[registry.Component]{
		Title:   "Starter",
		Options: opts,
		Receipt: func(o present.Option[registry.Component]) string { return o.Value.Alias + "/" + o.Value.Name },
	}
}

// baseBranchSpec builds the base-branch selector: the choices grouped into
// Local / Remote tabs (the tab bar is hidden when only one scope is present),
// filterable, and hash-free — the short object name is deliberately not shown as
// a secondary line (it is noise for the common case). The list collapses to its
// short-name receipt on accept (FR-007, FR-008).
func baseBranchSpec(choices []basebranch.Choice) present.SelectSpec[basebranch.Choice] {
	opts := make([]present.Option[basebranch.Choice], len(choices))
	for i, c := range choices {
		group := "Local"
		if c.Scope == basebranch.ScopeRemoteTracking {
			group = "Remote"
		}
		opts[i] = present.Option[basebranch.Choice]{
			Value:   c,
			Primary: c.Short,
			Group:   group,
		}
	}
	return present.SelectSpec[basebranch.Choice]{
		Title:      "Base branch",
		Grouped:    true,
		Filterable: true,
		Options:    opts,
		Receipt:    func(o present.Option[basebranch.Choice]) string { return o.Value.Short },
	}
}

// startImpact is the confirmation preview: the resolved repository, base
// branch (short name only), branch, workspace root, and target directory. In
// contribution mode there is no separate base to show — the preview names the
// existing branch being checked out instead of a new one being created from a
// base (contracts/cli-work-start.md §New clause: start modes).
func startImpact(repoPath, repoName string, base basebranch.Choice, branch, workspaceRoot, mode string) string {
	dirPath := filepath.Join(workspaceRoot, "in-progress", repoName+"_"+strings.ReplaceAll(branch, "/", "-"))
	if mode == work.StartModeContribution {
		return fmt.Sprintf(
			"  repository: %s\n  branch:     %s (existing, checked out)\n  workspace:  %s\n  directory:  %s",
			repoPath, branch, workspaceRoot, dirPath)
	}
	return fmt.Sprintf(
		"  repository: %s\n  base:       %s\n  branch:     %s\n  workspace:  %s\n  directory:  %s",
		repoPath, base.Short, branch, workspaceRoot, dirPath)
}

func initialSourceErrText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
