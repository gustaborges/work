---
description: "Task list for F3 — Find the Local Clone"
---

# Tasks: Find the Local Clone (F3)

**Input**: Design documents from `specs/004-local-clone-locator/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md` (R1–R21), `data-model.md`,
`contracts/` (`cli-work-start.md`, `cli-work-repository.md`, `repository-reference.md`,
`repository-locator.md`, `resolution-policy.md`), `quickstart.md` (S1–S13)

**Tests**: Included — the plan and spec mandate contract, unit, `testscript`, and
PTY coverage. Test tasks are first-class here.

**Organization**: Phases follow the plan's Branching Strategy (one
`feature/004-local-clone-locator-p<n>-*` branch per phase, stacked on the prior
phase tip). User-story order in the phase numbering matches the plan (US3/US5
before US4, because US4 hardens the pipeline the earlier stories build), not the
raw P1→P3 priority.

## Path & module conventions

- Single Go module `github.com/gustaborges/work`, Go 1.26.
- New packages: `internal/locator/`, `internal/repoconfig/`.
- New CLI files: `internal/cli/repository.go`, `repository_policy.go`,
  `repository_root.go`, `repository_test.go`.
- Edits: `internal/cli/start.go`, `internal/starter/starter.go`,
  `internal/diag/diag.go`, `internal/config/*` (no key change),
  `internal/workspace/*` (one validation clause), `seed/starter/main.go`.
- Test fixtures: `tests/fixtures/locators/` (compiled fake Locator entrypoints,
  built like `internal/starter/testdata/rogue`).
- **No persisted-schema change**: `work-state.json` stays schema 2, `work.db`
  stays `user_version = 2`, `config/work.json` gains no key.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Scaffolding both new packages and the fake-Locator fixtures need
before any logic lands. Branch: `feature/004-local-clone-locator-p1-foundational`
(shared with Phase 2).

- [X] T001 Confirm base/target branch with the user, then
      `git switch feature/004-local-clone-locator-specs-00 && git pull && git switch -c feature/004-local-clone-locator-p1-foundational`.
- [X] T002 [P] Create `internal/locator/` package skeleton with `locator.go`
      holding the package doc, `Reference`, `Deps`, `Outcome`, `Candidate` type
      declarations and the `Resolve(ctx context.Context, d Deps, ref Reference) (Outcome, error)`
      signature (empty body returning a not-implemented error), per data-model.md
      §1–§3 and research.md R2.
- [X] T003 [P] Create `internal/repoconfig/` package skeleton with `roots.go` and
      `policy.go` package docs and exported-function stubs (`ListRoots`, `AddRoots`,
      `RemoveRoots`, `ReplaceRoots`, `NeedsSetup`, `ValidateRoot`; `ListPolicy`,
      `AddPolicy`, `RemovePolicy`, `MovePolicy`, `ReplacePolicy`) per data-model.md
      §4–§5 and contracts/resolution-policy.md §Operation semantics.
- [X] T004 [P] Create `tests/fixtures/locators/` with `go:build ignore`-style
      buildable entrypoints `ok/` (one match), `empty/` (matches: []), `two/`
      (two matches), `dupe/` (same repo twice via a relative + symlinked path),
      `invalid/` (a match that is not a git repo), `boom/` (exit 1), each a
      `main.go` reading `ipc.LocatorInput` and writing `ipc.LocatorResponse`,
      following `internal/starter/testdata/rogue` and `seed/locator/main.go`.
- [X] T005 [P] Add a fixture-Locator registry helper (test-only) that registers a
      component with a chosen `alias/name`, `role: repository-locator`, `accepts`
      list, and entrypoint path — used by `internal/locator`, `internal/repoconfig`,
      and `tests/contract` to install fake Locators without a plugin install.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The resolution engine, the config-operations package, the Starter
reference extension, and the diagnostic categories — everything the user stories
wire together. **No CLI wiring in this phase.** Same branch as Phase 1.

**⚠️ CRITICAL**: No user-story phase can begin until this phase is complete and
`make lint` + `go test ./...` are green.

### Diagnostics

- [X] T006 Add categories `NoRepositoryFound` (`no-repository-found`, 26),
      `NoEligibleLocator` (`no-eligible-locator`, 27), `RepositoryCandidateInvalid`
      (`repository-candidate-invalid`, 28), `LocatorFailed` (`locator-failed`, 29),
      `RepositoryAmbiguous` (`repository-ambiguous`, 30) to `internal/diag/diag.go`,
      appended to `All` after `SnapshotUnreadable`, per data-model.md §7.
- [X] T007 Extend the `diag` table test in `internal/diag/diag_test.go` to assert
      codes 26–30 and their tokens, and that codes 0/2/10–25 and every existing
      token are unchanged (FR-036).

### Starter reference extension

- [X] T008 Extend `starter.Reference` in `internal/starter/starter.go` with
      `GitFetchURLs []string`, `Name string`, `Query string`; make `Invoke` return
      all typed reference fields and **remove** the "the Starter returned no
      repository path" guard (its intent moves to `internal/locator`), per
      research.md R8. Keep `start_modes`/`base_branch`/`meta`/`links` ignored.
- [X] T009 [P] Update `internal/starter/starter_test.go` for the widened
      `Reference` (path-only, name-only, and mixed responses) and the removed
      no-path error.

### Resolution engine (`internal/locator`)

- [X] T010 Implement eligibility in `internal/locator/locator.go`: a policy entry
      participates iff it resolves to a registered `role == "repository-locator"`
      component **and** `accepts ∩ {non-empty git_fetch_urls|name|query} ≠ ∅`;
      `path` is never in `accepts`; computed from static data only, no subprocess
      (FR-006, research.md R3).
- [X] T011 Implement projection in `internal/locator/locator.go`: build
      `ipc.LocatorInput` with only the accepted-and-present reference fields plus
      `Deps.Roots` — never the raw arg, base branch, start modes, meta, or links
      (FR-007, FR-008, SC-004, research.md R4).
- [X] T012 Implement chain traversal in `internal/locator/chain.go`: walk
      `Deps.Policy` in order; skip unavailable, skip ineligible; `InvokeLocator`
      transport/non-zero-exit/unparseable-stdout → **halt** `locator-failed` (29);
      `matches: []` → continue; non-empty `matches` → end traversal and evaluate
      that Locator only. No aggregation, no fallback after results or after an
      error (FR-009–FR-015, ADR-0015, research.md R6).
- [X] T013 Implement candidate validation + dedup in `internal/locator/candidate.go`:
      `reporef.ValidatePath` per `match.repo_path`; dedup key = resolved
      (`EvalSymlinks`) absolute path; drop invalid silently; all-invalid →
      `repository-candidate-invalid` (28) (FR-016–FR-018, SC-009, research.md R7).
- [X] T014 Implement outcome classification in `internal/locator/outcome.go`:
      1 valid → `Outcome.Resolved`; ≥2 valid → `Outcome.Candidates`; traversal
      ended with no candidates and some Locator was eligible → `no-repository-found`
      (26); none ever eligible (empty policy / no accepted field) →
      `no-eligible-locator` (27). All errors are `*diag.Error` with `Summary` +
      `Hint` (data-model.md §3, research.md R5).
- [X] T015 [P] Compute the picker secondary line for the ambiguous case in
      `internal/locator/candidate.go`: first remote fetch URL
      (`git -C <path> remote get-url <first>`) else parent directory name;
      computed only when `len(Candidates) >= 2` (research.md R15).
- [X] T016 [P] Unit tests `internal/locator/locator_test.go` driving `Resolve`
      against the Phase 1 fixture Locators: eligibility (accepts ∩ fields),
      projection payload (only accepted+present + roots; never arg/base/modes/
      meta/links), traversal order, `matches:[]` advances, first non-empty ends,
      operational failure halts with no fallback, dedup collapses path/symlink
      variants, and each of the five outcome classifications.

### Config operations (`internal/repoconfig`)

- [X] T017 Implement `internal/repoconfig/roots.go`: `ListRoots` (stored order,
      `[]` never an error); `AddRoots`/`ReplaceRoots` (expand `~`, `filepath.Abs`,
      store plain absolute; require existing readable dir else `usage` 2; reject
      overlap with `config.Workspace` in **either** direction on a path-segment
      boundary using `workspace` canonical helpers, error names both paths + reason,
      `usage` 2; canonical dedup); `RemoveRoots` (canonical match, absent = no-op)
      (FR-020, FR-022, research.md R13).
- [X] T018 Implement `internal/repoconfig/roots.go` exports for the CLI first-run
      steps: `NeedsSetup(cfg) (wantWorkspace, wantRoot bool)` and
      `ValidateRoot(cfg, path) (abs string, err error)` — no `present` import
      (data-model.md §5, research.md R21).
- [X] T019 Implement `internal/repoconfig/policy.go`: `ListPolicy` (every entry in
      order with an `available` flag from the registry — `role == "repository-locator"`;
      unavailable entries kept + marked, never dropped); `AddPolicy` (`ref` must
      resolve to a registered repository-locator else `usage` 2; append or
      `--before`/`--after`; present `ref` = no-op); `RemovePolicy` (drop each,
      absent = no-op, component stays installed); `MovePolicy` (exactly one of
      `--before`/`--after` else `usage` 2; `ref` and anchor must be present);
      `ReplacePolicy` (validate whole list then atomic write). Accept a bare
      `<component>` when unambiguous and echo it back qualified (FR-023–FR-028,
      research.md R12, contracts/resolution-policy.md).
- [X] T020 [P] Add a `Registered Locator` view helper for `locator list` in
      `internal/repoconfig/policy.go`: project `registry.Component` where
      `role == "repository-locator"` to `{ref, display_name, description, accepts,
      in_policy}` (data-model.md §6, research.md R14).
- [X] T021 [P] Unit tests `internal/repoconfig/repoconfig_test.go`: every policy
      operation incl. unknown-locator rejection, positioning, bare-component
      resolution, availability marker on `list`; every root operation incl.
      absolutization, canonical dedup, non-dir/unreadable rejection, and
      workspace-overlap rejection in both directions; `NeedsSetup`/`ValidateRoot`.

### Contract test (core-side resolution)

- [X] T022 Add `tests/contract/locator_resolution_test.go`: assert the serialised
      `LocatorInput` payload for every reference shape in
      `contracts/repository-reference.md` §Tests, and the six outcome rows in
      `contracts/repository-locator.md` §"Outcome → exit code" against fake
      Locators. `tests/contract/locator_test.go` (seed binary) stays untouched and
      green.

**Checkpoint**: `internal/locator` and `internal/repoconfig` fully unit- and
contract-tested; `diag` table extended; `starter.Reference` widened. No behaviour
change to any existing command. `make lint` + `go test ./...` green on the CI
matrix, F1/F2/F2.5 suites unchanged. Merge forward.

---

## Phase 3: User Story 1 — Start a Work Using Only a Repository Name (Priority: P1) 🎯 MVP

**Goal**: `work start <name>` resolves a single matching local clone through the
policy and continues the identical F1 creation journey; `work start <path>` is
unchanged.

**Independent Test**: One search root with one clone named `payments`; run
`work start payments` (no path); resolution yields that clone's path and the F1
journey completes into a materialised Work (quickstart S3, S4).

Branch: `feature/004-local-clone-locator-p2-us1-start-by-name`, cut from Phase 2
tip.

### Tests for User Story 1

- [X] T023 [P] [US1] PTY test `tests/integration/start_by_name_test.go`: single
      match — `work start payments` in an 80×24 PTY resolves `$R1/payments`
      silently, no path prompt, no picker; wizard continues at prefix/slug/base;
      confirm prints the three F1 stdout lines and repositions the terminal;
      `internal/work/verify.Check` passes (quickstart S3).
- [X] T024 [P] [US1] Head-to-head test (`tests/integration/`): `work start <path>`
      vs `work start <name>` for the same repo produce identical `work-state.json`
      fields (normalising slug/branch/timestamps) and identical terminal
      repositioning (SC-008, quickstart S4).
- [X] T025 [P] [US1] `testscript` `tests/integration/start_by_name_non_interactive.txtar`:
      one match proceeds; explicit-argv single match proceeds; missing `SOURCE`
      still exit 2; `--json` still rejected on `work start`.
- [X] T026 [P] [US1] Extend `seed/starter/main_test.go` for argument
      classification (path-looking vs bare token).

### Implementation for User Story 1

- [X] T027 [US1] Teach `seed/starter/main.go` to classify the argument: emit
      `repository.path` (absolutised) when it contains a path separator, starts
      with `.`/`..`/`~`/a drive letter, or names an existing filesystem entry;
      otherwise emit `repository.name = <arg>`. Never emit `git_fetch_urls`/`query`
      (contracts/cli-work-start.md §Argument classification, FR-032).
- [X] T028 [US1] In `internal/cli/start.go`, restructure the Source step to
      classify the `starter.Reference`: `ref.Path != ""` → `reporef.ValidatePath`
      (unchanged F1 path, `invalid-path`/`unusable-repo` still in-frame
      recoverable); otherwise call `locator.Resolve` with `Deps` built from the
      loaded `config.Config` (policy, roots, registry, plugins dir) (research.md
      R9, contracts/cli-work-start.md §Interactive flow).
- [X] T029 [US1] Handle the single-match outcome in `internal/cli/start.go`:
      `Outcome.Resolved` → stash `repoPath`, accept the step, continue the
      identical F1 creation journey (no divergence in snapshot, projection, or
      repositioning — FR-004, FR-035, research.md R16). Defer `Ambiguous` and the
      error outcomes to Phases 4/7 (a temporary `present.Fatal`/error return is
      acceptable within this phase).
- [X] T030 [US1] Wire the same classify-then-`locator.Resolve` closure into the
      non-interactive / explicit-argv path in `internal/cli/start.go` so a single
      resolved match proceeds without a prompt (FR-012).
- [X] T031 [US1] Update `internal/cli/start_test.go` for the restructured Source
      step (path classification branch + single-match resolution branch).

**Checkpoint**: `work start <name>` (single match) reaches a ready worktree with
zero paths typed; `work start <path>` byte-identical to F1. F1
`create_happy.txtar` / `invalid_path.txtar` / `rollback.txtar` and all F1/F2/F2.5
non-interactive suites green. Merge forward.

---

## Phase 4: User Story 2 — Choose the Right Repository When Several Clones Match (Priority: P2)

**Goal**: Multiple valid matches → an explicit `present.Select` step
(interactive) or `repository-ambiguous` exit 30 (non-interactive / explicit
argv); never an auto-pick, chain never resumed.

**Independent Test**: Two matching clones in configured roots; interactive
`work start <name>` offers both with identifying detail, selecting one materialises
the Work against that repo and not the other; the non-interactive form exits 30
with no Work (quickstart S5, S6).

Branch: `feature/004-local-clone-locator-p3-us2-disambiguate`, cut from Phase 3
tip.

### Tests for User Story 2

- [X] T032 [P] [US2] PTY test in `tests/integration/start_by_name_test.go`: two
      matches — a Repository select step appears with primary = absolute path,
      secondary = remote URL or parent dir, bounded/filterable/`❯` marker;
      choosing `$R2/payments` materialises the Work against it; step collapses to a
      `payments (…/mirror)` receipt; chain not resumed (quickstart S5).
- [X] T033 [P] [US2] Selector-geometry assertions for the Repository step at
      40×10 / 80×24 / 160×50 (extend `tests/integration/selector_geometry_test.go`
      or a new file), per plan Testing.
- [X] T034 [P] [US2] `testscript` / non-interactive test
      (`tests/integration/*ambiguity*` or extend an existing txtar): explicit
      `SOURCE` resolving to ≥2 repos → exit 30 `repository-ambiguous` with the
      narrow-the-reference hint, **no selector**, no branch/worktree/dir/snapshot/
      row, config unchanged (quickstart S6, SC-010).

### Implementation for User Story 2

- [X] T035 [US2] In `internal/cli/start.go`, on `Outcome.Candidates` from the
      Source step: stash the deduped candidates, set `ambiguityPending`, accept the
      step (research.md R9).
- [X] T036 [US2] Add the conditional `repository` `present.SelectStep` in
      `internal/cli/start.go`, shown only when `ambiguityPending`: options are the
      candidates (primary = resolved path, secondary = remote/parent per T015);
      on accept `repoPath` is the chosen path and the chain is not resumed
      (FR-011, FR-013, contracts/cli-work-start.md).
- [X] T037 [US2] In the non-interactive / explicit-argv path in
      `internal/cli/start.go`, map `Outcome.Candidates` to `repository-ambiguous`
      (exit 30) with a `*diag.Error` carrying the "pass a more specific reference
      or adjust repository_roots" hint; open no selector (FR-033, research.md R5).
- [X] T038 [US2] Verify (test + code) that every ambiguity/failure exit from the
      Source or Repository step returns before `create.Run`, leaving zero branch,
      worktree, Work dir, snapshot, or `works` row (FR-037, SC-010).

**Checkpoint**: Interactive ambiguity always requires a choice (0% auto-select);
non-interactive ambiguity is exit 30 with no Work. Merge forward.

---

## Phase 5: User Story 3 — Configure Repository Search Roots (Priority: P2)

**Goal**: `work repository root list|add|remove|replace` with validation and
bidirectional workspace/root overlap rejection; interactive `work start` first-run
setup prompts for the workspace root and one search root, up front, first run
only.

**Independent Test**: Fresh install — interactive `work start <name>` first two
prompts are workspace root + search root with purpose lines; `work repository root
add`/`list --json`/`remove`/`replace` round-trip through `work.json`; overlap
configs are rejected before any write (quickstart S2, S13, SC-013).

Branch: `feature/004-local-clone-locator-p4-us3-roots`, cut from Phase 4 tip.

### Tests for User Story 3

- [X] T039 [P] [US3] `testscript` `tests/integration/repository_root.txtar`:
      `root list` (human + `--json` `{"roots":[…]}`, empty prints a `note:` to
      stderr exit 0); `root add` multiple, absolute, in order, re-add no-op;
      `root remove`; `root replace`; non-dir / unreadable rejected `usage` 2;
      workspace-overlap rejected in both directions with both paths named; ANSI
      absent on non-TTY (quickstart S2, contracts/cli-work-repository.md).
- [X] T040 [P] [US3] PTY test `tests/integration/start_first_run_test.go`: fresh
      `WORK_HOME` + unset `workspace` — first prompt = workspace root (purpose
      line), second = search root (purpose line); both persisted in a single
      `config.Save` **before** the first journey step; a second `work start`
      shows neither; search-root prompt rejects `$WS/in-progress` in-frame,
      re-promptable; Ctrl-C at either prompt → exit 20, nothing written, no Work
      (quickstart S13, SC-013).
- [X] T041 [P] [US3] Non-interactive coverage in
      `tests/integration/start_by_name_non_interactive.txtar`: `work start <name>`
      on a fresh home with no roots → exit 26 `no-repository-found` + hint to
      `work repository root add`, no prompt/hang; `work start "$R1/payments"` on the
      same fresh home proceeds (no root needed) (contracts/cli-work-start.md
      §First-run setup, FR-022a).
- [X] T042 [P] [US3] Update the F1 interactive fresh-install PTY scripts
      (`tests/integration/` F1 scenarios / `f1_regression_test.go`) to answer the
      two up-front setup prompts — **no** change to stdout, exit codes, or the
      resulting snapshot (SC-011, SC-013).
- [X] T043 [P] [US3] `internal/workspace` test for the new validation clause:
      workspace root equal to / inside / containing a configured repository root
      is rejected; a workspace root merely enclosed by an unrelated Git repo is
      still accepted (research.md R13).

### Implementation for User Story 3

- [X] T044 [US3] Add `internal/cli/repository.go`: the `work repository` parent
      command with `GroupID = "admin"` and no `Run` (Cobra prints grouped help,
      exit 0). Register it under `internal/cli/root.go`.
- [X] T045 [US3] Add `internal/cli/repository_root.go`: `work repository root`
      parent (`GroupID = "admin"`) + `list` (`--json`, pure), `add <PATH...>`,
      `remove <PATH...>`, `replace <PATH...>` delegating to `internal/repoconfig`;
      mutations reject `--json` (exit 2), print one stable
      `work: roots now <a>, <b>` line; single atomic `config.Save`
      (contracts/cli-work-repository.md, resolution-policy.md §Mutation output).
- [X] T046 [US3] Add the workspace↔root overlap clause to
      `internal/workspace/Validate` (mirror of the `repoconfig` root rule),
      reusing the existing canonicalisation helpers; keep the "not rejected merely
      because a Git repo encloses it" behaviour (data-model.md §8, research.md R13).
- [X] T047 [US3] Add first-run setup to `internal/cli/start.go` (interactive
      only), before the Source step: when `repoconfig.NeedsSetup` reports
      `wantWorkspace`, prompt for the workspace root (the F1 FR-006 step moved to
      the front, with a purpose line); when `wantRoot`, prompt for one search root
      (`repoconfig.ValidateRoot`, purpose line, overlap rejected in-frame &
      re-promptable). Persist both in a single `config.Save` before resolution;
      cancel → exit 20, nothing persisted (contracts/cli-work-start.md, research.md
      R21).
- [X] T048 [US3] Ensure non-interactive `work start` runs **no** setup: a
      name/reference needing resolution with `RepositoryRoots` empty →
      `no-repository-found` (exit 26) + hint; `work start <path>` still root-free
      (FR-022a).
- [X] T049 [US3] Add `internal/cli/repository_test.go` cases for `repository root`
      grammar, `--json` purity, mutation output, and exit codes.

**Checkpoint**: Roots are fully manageable non-interactively and round-trip through
`work.json`; fresh-install interactive `work start` establishes workspace + root
up front and never re-asks; overlap is rejected before any write. F1/F2/F2.5
suites green (F1 fresh-install PTY scripts updated with the two answers only).
Merge forward.

---

## Phase 6: User Story 5 — Inspect and Manage the Resolution Policy (Priority: P3)

**Goal**: `work repository policy list|add|remove|move|replace` +
`work repository locator list`, the availability model, and the help-only
`work repository` parent; installing a Locator never edits the policy.

**Independent Test**: With the seed Locator present, `locator list` and
`policy list` show the default; add/move/remove/replace a second fixture Locator
with explicit positioning, each producing the expected persisted script-readable
config; a second-Locator install leaves the policy byte-identical
(quickstart S1, S9, S10, S11, S12, SC-005, SC-006).

Branch: `feature/004-local-clone-locator-p5-us5-policy`, cut from Phase 5 tip.

### Tests for User Story 5

- [X] T050 [P] [US5] `testscript` `tests/integration/repository_policy.txtar`:
      `policy list` (human + `--json` `{"policy":[{ref,position,available}]}`);
      `add` with/without `--before`/`--after`; `move` (neither/both →
      exit 2); `remove` (component stays installed, `in_policy:false` after);
      `replace`; unknown `<LOCATOR>` → exit 2; unavailable entry from a
      hand-edited `work.json` shown marked `(unavailable)` and skipped at
      resolution (quickstart S9, S11).
- [X] T051 [P] [US5] `testscript` `tests/integration/repository_locator_list.txtar`:
      `--json` and human shapes, `in_policy` flag, exit codes, purity
      (quickstart S1, contracts/resolution-policy.md §`--json` shapes).
- [X] T052 [P] [US5] `testscript` `tests/integration/repository_help.txtar`:
      `work repository` with no subcommand → grouped `locator`/`policy`/`root`
      help on stdout, exit 0, in interactive and redirected streams; no menu; no
      ANSI on non-TTY; `work repository --json` → `error: usage:` exit 2
      (quickstart S12).
- [X] T053 [P] [US5] No-auto-insert test (`tests/integration/` or
      `internal/` — extend an existing bootstrap/plugin test): registering /
      installing a second Locator leaves `repository_resolution.locators`
      byte-identical; the new Locator appears in `locator list` with
      `in_policy:false` (SC-005, quickstart S10).
- [X] T054 [P] [US5] Extend `tests/integration/help_inventory.*`: `work
      repository` appears exactly once, under **Administration**.

### Implementation for User Story 5

- [X] T055 [US5] Add `internal/cli/repository_policy.go`: `work repository policy`
      parent (`GroupID = "admin"`) + `list` (`--json`, pure, marks unavailable),
      `add <LOCATOR> [--before|--after]`, `remove <LOCATOR...>`,
      `move <LOCATOR> (--before|--after)`, `replace <LOCATOR...>`, plus
      `work repository locator list [--json]`, all delegating to
      `internal/repoconfig`. Mutations reject `--json`, print one stable
      `work: policy now <ref>, <ref>` line, single atomic `config.Save`
      (contracts/cli-work-repository.md, resolution-policy.md).
- [X] T056 [US5] Ensure the chain (`internal/locator/chain.go`) skips unavailable
      policy entries and continues — verify against the S11 hand-edited-`work.json`
      case (a `work start <name>` run skips the ghost entry and resolves via the
      seed Locator, no `locator-failed`) (research.md R12, quickstart S11).
- [X] T057 [US5] Bare-`<component>` acceptance: `policy add`/`move`/`remove` and
      `locator list` accept an unqualified component when unambiguous across
      installed Locators and echo it back qualified (contracts/resolution-policy.md).
- [X] T058 [US5] Extend `internal/cli/repository_test.go` for the `policy` and
      `locator` grammar, positioning flags, and exit codes.

**Checkpoint**: The full ordered-policy model is inspectable and editable
non-interactively; plugin install never mutates the policy; unavailable entries
are visible and skipped. Merge forward.

---

## Phase 7: User Story 4 — Distinguish No Result, Invalid Candidate, and Locator Failure (Priority: P3)

**Goal**: The five resolution outcomes surface as five distinct diagnostics with
`Summary`/`Hint`, never cross-converted, never a silent fallback, never a partial
Work; `no-repository-found` is recoverable in-frame in the interactive Source step.

**Independent Test**: Drive resolution into empty-roots, non-repo-directory, and
erroring-Locator states with fixtures; confirm exits 26/28/29 are distinct with no
cross-conversion and no partial Work in any case (quickstart S7, S8, SC-003,
SC-010).

Branch: `feature/004-local-clone-locator-p6-polish`, cut from Phase 6 tip
(shared with Phase 8).

### Tests for User Story 4

- [X] T059 [P] [US4] `testscript` `tests/integration/resolution_outcomes.txtar`:
      policy `[invalid]` → exit 28 `repository-candidate-invalid` naming the
      rejected path; policy `[boom]` → exit 29 `locator-failed` naming the
      Locator, a following policy entry **never consulted**; empty roots → exit 26
      `no-repository-found`; assert 26 ≠ 27 ≠ 28 ≠ 29 and no Work in any case
      (quickstart S7, S8).
- [X] T060 [P] [US4] PTY variant in `tests/integration/start_by_name_test.go`: at
      the Source prompt, `nonesuch` shows the `no-repository-found` error
      **in-frame** and the field is re-promptable; then `payments` continues the
      journey (quickstart S7).
- [X] T061 [P] [US4] Test that `no-eligible-locator` (27) — empty policy or a
      reference carrying no accepted field, and the `{}` / unaccepted-`query`
      cases from `contracts/repository-reference.md` §Tests — is terminal and
      distinct from 26.

### Implementation for User Story 4

- [X] T062 [US4] Finalise `Summary`/`Hint` text for all five categories in
      `internal/locator/outcome.go` (and the `repository-ambiguous` construction
      in `start.go`): "add a root" (26), "policy not set up" (27), "that folder is
      broken" (28), "your Locator errored" (29), "be more specific" (30); the
      Locator's stderr is retained as the internal cause only, never printed in
      normal output (research.md R5, plan Auditability gate).
- [X] T063 [US4] In `internal/cli/start.go`, wire the interactive Source-step
      outcome handling: `no-repository-found` (26) shown in-frame & re-promptable;
      `no-eligible-locator` (27) / `repository-candidate-invalid` (28) /
      `locator-failed` (29) → `present.Fatal(err)` to the diagnostic border
      (contracts/cli-work-start.md §Interactive flow, research.md R9).
- [X] T064 [US4] Confirm non-interactive Source-step handling returns the exact
      token/exit for each of 26/27/28/29 (no re-prompt) with the transaction
      guarantee (no branch/worktree/dir/snapshot/row; config unchanged; workspace
      root not persisted) (contracts/cli-work-start.md §Non-interactive flow,
      FR-037).

**Checkpoint**: Three (five) failure modes produce distinct messages in 100% of
fixtures with zero cross-conversion and zero partial Work. Merge forward.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Compatibility sweep, presentation parity, performance, docs. Same
branch as Phase 7.

- [X] T065 [P] Presentation parity sweep: `NO_COLOR` / `TERM=dumb` / redirected
      streams carry zero ANSI for every `work repository` subcommand and the
      new `work start` steps (extend `tests/integration/no_color_env.txtar` /
      `no_ansi_when_piped.txtar` / `stream_separation_test.go`) (F2.5 parity,
      spec Edge Cases).
- [X] T066 [P] Performance test (`tests/integration/performance_test.go`): a
      500-repo search tree, non-interactive `work start <unique-name>` completes
      < 2 s on the CI reference runner (SC-012, research.md R18).
- [X] T067 Full regression run: F1 `quickstart.md` S1–S12 (non-interactive
      unchanged; interactive fresh-install scripts carry only the two added setup
      answers), F2 S1–S13, F2.5 Q1–Q12, `tests/contract/locator_test.go`, and the
      `diag` table test — all green with no stdout / token / mutation / exit-code
      diff (SC-011).
- [X] T068 [P] Run the F3 `quickstart.md` scenarios S1–S13 end to end and record
      any deviation.
- [X] T069 [P] Refresh the managed Spec Kit agent-context section via
      `/speckit-agent-context-update` (or the skill) so `AGENTS.md` / `CLAUDE.md`
      reflect the shipped `internal/locator`, `internal/repoconfig`, and
      `work repository` surface.
- [X] T070 [P] Update `docs/add/add-0001-work-system-architecture.md` §7.1–§7.2
      cross-references and any `docs/` command inventory to list `work repository`;
      confirm ADR-0014/0015/0016/0019 need no text edit (plan Phase 0 note).
- [X] T071 Code-quality pass on `internal/locator`, `internal/repoconfig`, and the
      new `internal/cli/repository*.go` (comments preserve *why* per CLAUDE.md; no
      process metadata in comments; public API documented).

---

## Dependencies & Execution Order

### Phase dependencies

- **Phase 1 (Setup)**: no dependencies.
- **Phase 2 (Foundational)**: depends on Phase 1 — **blocks Phases 3–8**.
- **Phase 3 (US1)**: depends on Phase 2. MVP.
- **Phase 4 (US2)**: depends on Phase 3 (reuses the Source-step restructure and
  the candidate/secondary-line logic).
- **Phase 5 (US3)**: depends on Phase 4 in the plan's stacked-branch model; the
  `work repository root` surface itself only needs Phase 2, but first-run setup
  edits the same `start.go` Source region as Phases 3–4.
- **Phase 6 (US5)**: depends on Phase 5 (shares `internal/cli/repository.go` and
  the `repository_test.go` file; `repoconfig` policy ops are from Phase 2).
- **Phase 7 (US4)**: depends on Phases 3–6 (hardens the Source-step outcome
  handling all of them touch).
- **Phase 8 (Polish)**: depends on Phases 3–7.

### User-story independence

- **US1** is independently testable once Phase 2 is done (single-match resolution
  + `work start <path>` regression).
- **US2** builds on US1's pipeline but its picker / exit-30 behaviour is
  independently testable.
- **US3** (root management + first-run setup) is independently testable; the
  `work repository root` CLI needs only Phase 2.
- **US5** (policy management) is independently testable; needs only Phase 2 for
  the engine, Phase 5 for the shared parent command file.
- **US4** is a hardening layer over US1/US2/US3/US5 — testable once those exist.

### Within each phase

- Test tasks marked [P] can be written in parallel and should fail before the
  implementation tasks land.
- `internal/locator` files: `locator.go` (types + eligibility + projection)
  before `chain.go` before `candidate.go`/`outcome.go`.
- `internal/repoconfig`: `roots.go` and `policy.go` are independent ([P]).
- `start.go` edits within a phase are sequential (same file).

### Parallel opportunities

- **Phase 1**: T002, T003, T004, T005 all [P].
- **Phase 2**: T007, T009, T016, T021 (tests) [P]; T015, T020 [P]; engine vs
  `repoconfig` implementation streams are independent.
- **Phase 3–7**: all test tasks within a phase marked [P]; `seed/starter` (T027)
  is independent of the `start.go` edits.
- **Phase 8**: T065, T066, T068, T069, T070 all [P].

---

## Parallel Example: Phase 2 Foundational

```bash
# Engine stream (one dev):
Task T010 – eligibility in internal/locator/locator.go
Task T011 – projection in internal/locator/locator.go
Task T012 – chain traversal in internal/locator/chain.go
Task T013 – candidate validation + dedup in internal/locator/candidate.go
Task T014 – outcome classification in internal/locator/outcome.go

# Config stream (in parallel, another dev):
Task T017 – internal/repoconfig/roots.go
Task T019 – internal/repoconfig/policy.go

# Cross-cutting, parallel from the start:
Task T006 – diag categories 26–30
Task T008 – starter.Reference extension
```

---

## Implementation Strategy

### MVP (Phases 1–3)

1. Phase 1 Setup → Phase 2 Foundational (engine + repoconfig + diag + starter).
2. Phase 3 US1: `work start <name>` single match + `work start <path>` regression.
3. **STOP and VALIDATE**: quickstart S3 + S4; F1 non-interactive suite green.
4. Demo: start a Work with zero paths typed.

### Incremental delivery

- + Phase 4 (US2): disambiguation picker + exit 30 → demo two-clone choice.
- + Phase 5 (US3): `work repository root` + first-run setup → demo fresh install.
- + Phase 6 (US5): `work repository policy` / `locator` → demo policy editing.
- + Phase 7 (US4): the five distinct diagnostics → demo the failure taxonomy.
- + Phase 8: compatibility + performance sweep → release `release/0.4`.

Each phase merges forward only after its Checkpoint is met and `make lint` +
`go test ./...` are green on `ubuntu-latest`, `macos-latest`, `windows-latest`,
with every prior slice's demo (F1 S1–S12, F2 S1–S13, F2.5 Q1–Q12, earlier F3
phases) still green.

---

## Notes

- [P] = different files, no ordering dependency.
- [Story] label maps a task to a spec user story (US1–US5) for traceability.
- No persisted-schema change anywhere in F3 — if a task seems to require one,
  stop and re-read `data-model.md` §8.
- The seed binary contract test `tests/contract/locator_test.go` must stay green
  and untouched.
- Branch names use one hyphenated segment under `feature/`
  (`feature/004-local-clone-locator-p<n>-<short>`), never a nested path.
- Confirm base/target branch with the user at the start of every phase (per plan
  Branching Strategy — F1/F2 phases stacked rather than targeting `develop`).
