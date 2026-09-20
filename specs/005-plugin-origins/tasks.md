---
description: "Task list for F4 — New Origins via Plugin"
---

# Tasks: New Origins via Plugin (F4)

**Input**: Design documents from `specs/005-plugin-origins/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md` (R1–R19), `data-model.md`,
`contracts/` (`cli-work-plugin.md`, `cli-work-convention.md`, `cli-work-start.md`,
`starter-protocol.md`, `work-state.schema.json`), `quickstart.md` (S1–S14)

**Tests**: Included — the plan and spec mandate unit, contract, `testscript`, and
PTY coverage for every new package and command. Test tasks are first-class here.

**Organization**: Phases follow the plan's Branching Strategy (one
`feature/005-plugin-origins-p<n>-*` branch per phase, stacked on the prior phase
tip). Phase order matches the plan, not raw spec priority: US4 (Starter
collision, P3) is built before US5 (branch convention, P3) because collision
hardens US2's pipeline while convention memory needs at least one completed
fork-mode creation from US3 to be independently testable.

## Path & module conventions

- Single Go module `github.com/gustaborges/work`, Go 1.26.
- New packages: `internal/plugininstall/`, `internal/repoidentity/`,
  `internal/repoconv/`.
- New CLI files: `internal/cli/plugin.go`, `plugin_install.go`, `plugin_list.go`,
  `convention.go`, `convention_show.go`, `convention_set.go`, plus their
  `*_test.go`.
- Edits: `internal/registry/registry.go` (+`Package`), `internal/diag/diag.go`
  (+31–38), `internal/gitx/gitx.go` (+`Remotes`/`RemoteURL`/`RootCommits`/
  `IsShallow`/`DiscoverRepoRoot`/`Clone`), `internal/workhome/workhome.go`
  (+`BranchConventionsFile`), `internal/work/state.go` (schema → 3),
  `internal/create/create.go` (+`Params.StartMode`), `internal/starter/starter.go`
  (+`Match`, widened `Reference`), `internal/bootstrap/bootstrap.go` (delegates
  staging/swap to `plugininstall`), `internal/cli/start.go` (Starter match +
  collision + start-modes + generalized convention step), `internal/cli/root.go`
  (register `plugin`/`convention`).
- Test fixtures: `tests/fixtures/plugins/` (installable fixture packages —
  `plugin.json` + a compiled Go entrypoint — mirroring the pattern
  `tests/fixtures/locators/` already established for fake Locators).
- **Persisted-schema changes**: `work-state.json` → schema 3 (additive).
  `registry.json` gains the additive, unversioned `packages[]` array. A new
  generated file, `~/.work/state/branch_conventions.json`. **`work.db` needs no
  migration** (`SchemaVersion` stays 2 — confirmed against the live
  `internal/projection/projection.go` DDL: `branch_convention` already has no
  `NOT NULL`, and Go's zero-value `string` for `Slug`/`BranchConvention` inserts
  as SQL `''`, not `NULL`, so the existing `slug TEXT NOT NULL` column is
  unaffected by contribution mode's empty values).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Fixture plugin packages every later phase's tests build on.
Branch: `feature/005-plugin-origins-p1-foundational` (shared with Phase 2), cut
from `feature/005-plugin-origins-specs-00` tip.

- [X] T001 Confirm base/target branch with the user, then
      `git switch feature/005-plugin-origins-specs-00 && git pull && git switch -c feature/005-plugin-origins-p1-foundational`.
      (User confirmed: `feature/005-plugin-origins-specs-00` does not exist in
      this worktree; work directly on `f4-plugins` instead of cutting a new
      phase branch.)
- [X] T002 [P] Create `tests/fixtures/plugins/specific-starter/` and
      `tests/fixtures/plugins/colliding-starter/`: each a `plugin.json` +
      `main.go` Starter entrypoint reading `{"arg":"<token>"}` on stdin and
      writing the extended response shape from `contracts/starter-protocol.md`
      (`repository`, `base_branch`, `start_modes`). `specific-starter` matches a
      distinctive literal token (e.g. `demo-pr-1`), returns
      `start_modes: ["contribution","fork"]` and a `base_branch`, and declares a
      `gitflow` convention in its manifest (so the catalog has 2 entries
      alongside the reference package's `freeform`, per quickstart S13).
      `colliding-starter` declares an overlapping `pattern` matching the same
      token, for US4. Neither declares `accepts` or a `key` (those belong to
      other roles — `internal/plugin`'s role-field rules still apply to
      fixtures).
- [X] T003 [P] Create `tests/fixtures/plugins/invalid-manifest/` (a `plugin.json`
      with one `starter` component carrying a `key` field — forbidden for that
      role per `internal/plugin`'s `roleRules`, spec US1 AC4 — never meant to
      build or run) and `tests/fixtures/plugins/fallback-starter-a/` +
      `fallback-starter-b/` (each a valid, buildable `role: starter` component
      with an empty `pattern`, for FR-011/quickstart S6).
- [X] T004 [P] Add `tests/fixtures/plugins/plugins.go`: `Prepare(t *testing.T,
      fixture string) string` — copies `tests/fixtures/plugins/<fixture>` into a
      `t.TempDir()`, `go build`s its `main.go` into the entrypoint name its
      `plugin.json` declares (`.exe` suffix on Windows), and returns the
      directory path ready to hand to `work plugin install <dir>`; mirrors
      `tests/fixtures/locators/locators.go`'s `Build` pattern but stages a whole
      installable directory rather than registering a component directly.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Every primitive the four verticals share — diagnostics, the
registry's package record, the new `gitx`/`repoidentity`/`repoconv` layers,
schema 3, and `internal/create`'s existing-branch path. **No CLI wiring in this
phase.** Same branch as Phase 1.

**⚠️ CRITICAL**: No user-story phase can begin until this phase is complete and
`make lint` + `go test ./...` are green.

### Diagnostics

- [X] T005 Add categories `PluginInvalid` (`plugin-invalid`, 31),
      `PluginAliasConflict` (`plugin-alias-conflict`, 32),
      `PluginFallbackConflict` (`plugin-fallback-conflict`, 33),
      `PluginInstallFailed` (`plugin-install-failed`, 34),
      `StarterNotMatched` (`starter-not-matched`, 35),
      `StarterAmbiguous` (`starter-ambiguous`, 36),
      `StarterResponseInvalid` (`starter-response-invalid`, 37),
      `ConventionUnknown` (`convention-unknown`, 38) to
      `internal/diag/diag.go`, appended to `All` after `RepositoryAmbiguous`,
      per data-model.md §11 / research R18.
- [X] T006 [P] Extend the `diag` table test in `internal/diag/diag_test.go` to
      assert codes 31–38 and their tokens, and that codes 0/2/10–30 and every
      existing token are unchanged.

### Registry

- [X] T007 [P] Add `Package{Alias, Origin, Reference}` and a `Packages
      []Package` field to `Registry` in `internal/registry/registry.go`, plus
      `Origin` constants (`OriginLocalLinked`, `OriginLocalPinned`,
      `OriginRemotePinned`), `UpsertPackage` (keyed by `Alias`),
      `PackageByAlias`, and `ListPackages` (sorted by alias), per data-model.md
      §2, research R5.
- [X] T008 [P] Extend `internal/registry/registry_test.go`: `Packages`
      upsert/lookup/sorted-list, JSON round-trip, and that existing
      `Components`/`Conventions` behaviour is unchanged.

### `gitx` additions

- [X] T009 [P] Add to `internal/gitx/gitx.go`: `Repo.Remotes() ([]string,
      error)` (`git remote`); `Repo.RemoteURL(name string) (string, bool,
      error)` (`git remote get-url <name>`, `ok=false` on a clean non-zero
      exit); `Repo.RootCommits() ([]string, error)` (`git rev-list
      --max-parents=0 HEAD`, one hash per line); `Repo.IsShallow() (bool,
      error)` (`git rev-parse --is-shallow-repository`); package-level
      `DiscoverRepoRoot(cwd string) (string, error)` (`git -C <cwd> rev-parse
      --show-toplevel`); package-level `Clone(source, dest string) (headSHA
      string, err error)` (`git clone --depth 1 <source> <dest>`, then `git -C
      <dest> rev-parse HEAD`), per research R13/R3.
- [X] T010 [P] Unit tests in `internal/gitx/gitx_test.go` for
      `Remotes`/`RemoteURL` (present/absent), `RootCommits` (single commit, and
      a merged-unrelated-histories fixture yielding two roots),
      `IsShallow`/`DiscoverRepoRoot`, and `Clone` (against a local `file://`
      remote — no network), using `internal/gittest` fixtures.

### `internal/repoidentity`

- [X] T011 [P] Create `internal/repoidentity/repoidentity.go`:
      `Identify(repo gitx.Repo) (string, error)` implementing ADR-0011's three
      ordered layers — (1) the `origin` remote's fetch URL if configured; (2)
      else `RootCommits()`, sorted lexicographically and `+`-joined, if any
      exist; (3) else the absolute, symlink-resolved `repo.Dir` — per
      data-model.md §9, research R13.
- [X] T012 [P] Unit tests `internal/repoidentity/repoidentity_test.go`:
      origin-remote repos; no-remote repos (single root commit, and a
      merged-unrelated-histories composite key sorted deterministically
      regardless of merge order); a shallow clone with no remote (path
      fallback); moving/recloning a repository with a stable remote or root
      commit yields the same key.

### `internal/repoconv`

- [X] T013 [P] Add `Home.BranchConventionsFile() string` (`<root>/state/
      branch_conventions.json`) to `internal/workhome/workhome.go`.
- [X] T014 [P] Create `internal/repoconv/repoconv.go`: `Entry{Identity,
      Convention}`; `Load`/`Save` (atomic via `internal/atomicfile`,
      missing-file-yields-empty, mirroring `registry.Load`/`Save`);
      `Get(identity string) (string, bool)`; `Set(identity, convention
      string)` upserting by `Identity`, per data-model.md §8, research R14.
- [X] T015 [P] Unit tests `internal/repoconv/repoconv_test.go`: get/set against
      a temp state file; an unknown identity returns `("", false)`, never an
      error; `Set` overwrites an existing entry for the same identity;
      idempotent reload after `Save`.

### `work-state.json` schema 3

- [X] T016 In `internal/work/state.go`: bump `Schema` to `3`; add
      `StartModeFork = "fork"` and `StartModeContribution = "contribution"`
      alongside the existing `StartModeNew`; give `WorkSection.Slug` and
      `.BranchConvention` `omitempty` JSON tags (both may now be legitimately
      absent); update the package doc comment for the schema-2→3 change per
      data-model.md §6, research R10.
- [X] T017 Update `Validate()` in `internal/work/state.go`: `start_mode` ∈
      `{"new","contribution","fork"}`; `slug` and `branch_convention` both
      required + non-empty **unless** `start_mode == "contribution"`, in which
      case both **must be absent**; `base_branch` stays required in every mode;
      the schema-1/2 `archived_at` conditional rule is unchanged.
- [X] T018 [P] Extend `internal/work/state_test.go`: `Validate` accepts
      `contribution` with `slug`/`branch_convention` absent and rejects it with
      either present; accepts `fork`/`new` with both required; schema 1/2
      documents still `Decode` cleanly (regression); `Write` always emits
      schema 3.

### `internal/create` existing-branch path

- [X] T019 Add `Params.StartMode string` to `internal/create/create.go`. In
      `Run`'s step 3, when `StartMode == "contribution"`, call
      `repo.WorktreeAddExisting(worktreePath, p.Branch)` (no new branch) instead
      of `WorktreeAdd`, and push a compensator that calls only
      `repo.WorktreeRemove(worktreePath)` — **never** `repo.BranchDelete` — per
      research R12; every other `StartMode` value keeps the existing
      `WorktreeAdd` + worktree-then-branch-delete compensator.
- [X] T020 Update `build()` in `internal/create/create.go`: set
      `ws.StartMode` from `p.StartMode` (defaulting to `work.StartModeNew` when
      empty, preserving today's behaviour); leave `ws.Slug` and
      `ws.BranchConvention` empty when `p.StartMode == "contribution"`
      (`p.Slug`/`p.Convention` are expected empty from the caller in that mode —
      `create` does not itself decide the mode, only materializes it, FR-022).
- [X] T021 [P] Unit tests `internal/create/create_test.go`: the
      contribution-mode path calls `WorktreeAddExisting`, not `WorktreeAdd`; a
      simulated failure after that step unwinds the worktree but leaves the
      pre-existing branch intact; checking out an already-existing branch
      succeeds (contribution's whole premise — `git worktree add -b` would
      reject it); the fork/new path is byte-identical to F1/F3 behaviour.

**Checkpoint**: `internal/registry`, `internal/gitx`, `internal/repoidentity`,
`internal/repoconv`, `internal/work`, and `internal/create` all carry F4's new
surface with full unit coverage; `diag.All` carries 31–38. No behaviour change
to any existing command — `work start <path>` is still byte-identical.
`make lint` + `go test ./...` green on the CI matrix, F1/F2/F2.5/F3 suites
unchanged. Merge forward.

---

## Phase 3: User Story 1 — Install a Plugin and See It Registered (Priority: P1) 🎯 MVP

**Goal**: `work plugin install <source> [--link] [--as <alias>]` installs a
local (pinned or linked) or remote (pinned) plugin package through the same
staged-atomic-swap pipeline `bootstrap.install` uses for the embedded seed;
`work plugin list [--json]` reports what is installed.

**Independent Test**: Install a fixture plugin from a local path with
`--link`, confirm `work plugin list` shows it linked (not copied); separately
install the same fixture as a plain local install and confirm it is copied and
pinned; confirm a manifest violating role-based field rules is rejected with
nothing registered.

Branch: `feature/005-plugin-origins-p2-us1-install`, cut from Phase 2 tip.

### Tests for User Story 1

- [X] T022 [P] [US1] `internal/plugininstall/plugininstall_test.go`: local
      pinned copy vs. `--link` symlink (content never duplicated on disk in the
      linked case, SC-003); remote pinned install records the cloned `HEAD`
      commit as the reference; default alias vs. `--as`; alias collision
      (different origin → fail, nothing registered, SC-004) vs. idempotent
      reinstall (same origin, same alias); fallback-Starter conflict at install
      time (FR-011); a manifest violating role rules rejects the whole install
      with zero partial registration (SC-005); `--link` with a remote source
      rejected before any I/O (FR-002).
- [X] T023 [P] [US1] `tests/integration/plugin_install.txtar`: local
      pinned/linked, remote pinned (a local `file://` remote, no network),
      `--link`+remote (exit 2), alias conflict (exit 32) then idempotent
      reinstall (exit 0), fallback conflict (exit 33), invalid manifest
      (exit 31) — every case and exit code from
      `contracts/cli-work-plugin.md` §Contract tests.
- [X] T024 [P] [US1] `tests/integration/plugin_list.txtar`: empty list (before
      any install); list after install, text and `--json` shapes; sorted by
      alias; purity (no mutation, exit 0 always).
- [X] T025 [P] [US1] `internal/cli/plugin_test.go`: `work plugin` bare-command
      help-only-parent behaviour (`Args: cobra.NoArgs`, grouped help, exit 0 in
      every stream configuration) structurally identical to `repository_test.go`
      (research R17); `--json` on the bare command rejected as usage (exit 2).

### Implementation for User Story 1

- [X] T026 [US1] Extract `stagePrefix`/`backupSuffix`/`moveDestinationAside`/
      `restoreDestination`/`recoverInterruptedSwap`/`sweepStaleStaging` from
      `internal/bootstrap/bootstrap.go` into `internal/plugininstall/swap.go`,
      generalized over an arbitrary alias/plugins-dir instead of hardcoding
      `bootstrap.Alias` (research R1).
- [X] T027 [US1] Update `internal/bootstrap/bootstrap.go`'s `install()` to call
      `internal/plugininstall`'s extracted staging/swap primitives instead of
      owning its own copy; `EnsureSeed`'s public behaviour and every existing
      `bootstrap_test.go`/`stress_test.go` case stay green unchanged (ADR-0003
      "same pipeline," made literal).
- [X] T028 [US1] `internal/plugininstall/install.go`: source classification
      duplicating `seed/starter/main.go`'s `looksLikePath` heuristic (path
      separator, `.`/`..`/`~`/drive-letter prefix, or an existing filesystem
      entry ⇒ local; else remote) — an independent, self-contained copy per
      research R4, not a shared package.
- [X] T029 [US1] `internal/plugininstall/link.go`: local `--link` install
      creates `plugins/<alias>/source` as a directory symlink (junction on
      Windows) to the original path — no copy, no `EntrypointPath` change
      anywhere (research R2).
- [X] T030 [US1] `internal/plugininstall/install.go`: local non-`--link`
      install copies the source tree into staging (a pinned copy); remote
      install runs `gitx.Clone` (shallow) into a temp dir, records its `HEAD`
      SHA as the pinned reference, and stages the tree minus `.git` (research
      R3).
- [X] T031 [US1] `internal/plugininstall/install.go`: alias resolution
      (`--as`, else manifest `name`) and collision detection — compare the
      resulting alias's existing `Package.Reference`'s *origin identity*
      (absolute local path, or remote source URL ignoring the pinned SHA); a
      differing origin fails `plugin-alias-conflict` (32) with the plugin-name
      or `--as`-alias message from the contract, never showing the existing
      path; an
      identical origin under the same alias is an idempotent reinstall
      (research R5).
- [X] T032 [US1] `internal/plugininstall/install.go`: fallback-Starter
      uniqueness — a manifest declaring a fallback Starter (`role: starter`,
      empty `pattern`) fails `plugin-fallback-conflict` (33) when
      `registry.StarterFallback()` already returns a component from a
      *different* alias; reinstalling the same alias's own fallback is not a
      conflict (research R6).
- [X] T033 [US1] `internal/plugininstall/install.go`: assemble the full
      pipeline — classify `SOURCE`; reject `--link`+remote before any I/O
      (`usage`, 2); obtain content (T028–T030); `plugin.Parse` + full
      validation, failing `plugin-invalid` (31) with nothing registered on any
      violation; resolve alias + fallback checks (T031–T032); stage-then-swap
      via T026's primitives; register one `registry.Package` + one
      `registry.Component` per manifest component + one `registry.Convention`
      per manifest convention; any other I/O/clone/staging failure →
      `plugin-install-failed` (34).
- [X] T034 [US1] `internal/cli/plugin.go`: `work plugin` help-only parent
      (`GroupID = groupAdmin`, `Args: cobra.NoArgs`, `--json` rejected,
      `RunE` calls `cmd.Help()`), structurally identical to
      `internal/cli/repository.go` (research R17); register its children
      (`install`, `list`) and wire `newPluginCmd()` into
      `internal/cli/root.go` with `GroupID = groupAdmin`.
- [X] T035 [US1] `internal/cli/plugin_install.go`: `work plugin install
      <SOURCE> [--link] [--as <ALIAS>]`, delegating to `internal/plugininstall`;
      stdout `work: installed <alias> (<origin>)` +
      `work: components: <name> (<role>)[, ...]`; exit codes 0/2/31/32/33/34
      per `contracts/cli-work-plugin.md`.
- [X] T036 [US1] `internal/cli/plugin_list.go`: `work plugin list [--json]`,
      read-only, sorted-by-alias text block (`<alias>  <origin>  <reference>` +
      indented component lines) and the documented `--json` array shape;
      mutates nothing (FR-034).

**Checkpoint**: A plugin package written entirely outside the core can be
installed (local pinned/linked, remote pinned) and listed; alias/fallback
conflicts fail deterministically with zero partial registration; `bootstrap`
now runs through the shared pipeline. F1–F3 suites green. Merge forward.

---

## Phase 4: User Story 2 — Create a New Work from a Plugin-Provided Origin (Priority: P1)

**Goal**: `work start <plugin-specific-argument>` matches the installed
Starter, resolves a Repository Reference through the unchanged F3 pipeline,
and — absent `start_modes` — completes the ordinary new-Work journey with the
same guarantees as an F1 direct-path creation.

**Independent Test**: With `specific-starter` installed and matching a
distinctive argument, run `work start <argument>` and confirm the resulting
worktree, `work-state.json`, and index entry are indistinguishable in shape
and guarantees from an F1 direct-path creation.

Branch: `feature/005-plugin-origins-p3-us2-start`, cut from Phase 3 tip.

### Tests for User Story 2

- [X] T037 [P] [US2] `tests/contract/starter_match_test.go` (NEW): `Match`
      against fixture Starters — one specific match invoked directly with no
      selection step; zero matches fall back to the reference Starter;
      no match and no fallback → `starter-not-matched` (35). (Ambiguous/
      collision cases land in Phase 6.)
- [X] T038 [P] [US2] Extend `internal/starter/starter_test.go` for the widened
      `Reference`: `BaseBranch`/`StartModes` are read from the wire response but
      not yet consumed by any caller in this phase (consumption lands in
      Phase 5).
- [X] T039 [P] [US2] `tests/integration/start_by_plugin_starter_test.go` (PTY):
      with `specific-starter` installed and matching `demo-pr-1`, `work start
      demo-pr-1` completes the full new-Work journey; the specific Starter was
      invoked, not the reference fallback; `work.starter` names it (qualified
      `<alias>/<name>` if it would otherwise collide with another enabled
      Starter's bare name); the resulting worktree/`work-state.json`/index
      entry are indistinguishable in shape from an F1 direct-path creation
      (quickstart S7, SC-002).

### Implementation for User Story 2

- [X] T040 [US2] `internal/starter/starter.go`: add `Match(reg
      *registry.Registry, arg string) (registry.Component, Outcome, error)` —
      compile (`regexp.MustCompile`) and evaluate every registered `starter`
      component's non-empty `Pattern` against `arg`; zero matches →
      `registry.StarterFallback()` (`starter-not-matched`, 35, if none); exactly
      one match → return it directly; ≥2 matches → an `Outcome{Ambiguous:
      [...]}` for the caller to handle (research R7; the caller-side collision
      UI is Phase 6's job — this phase only needs the zero/one-match path
      wired end to end).
- [X] T041 [US2] `internal/starter/starter.go`: widen `Reference` (or the
      `Invoke` return type) with `BaseBranch string` and `StartModes
      []string`, both already present-but-unread on `ipc.StarterResponse`;
      `Meta`/`Links` stay unexposed (research R8).
- [X] T042 [US2] `internal/cli/start.go`: replace the `starter.Select(reg)`
      call in the SOURCE step with `starter.Match(reg, source)`; wire the
      zero/one-match outcomes into the existing `validatePath` closure
      unchanged in every other respect; qualify `work.starter` as
      `<alias>/<name>` only when the bare name would collide with another
      enabled Starter (quickstart S7).
- [X] T043 [US2] Regression check: with only the reference package installed
      (no specific Starters), `Match` always falls back to
      `registry.StarterFallback()` with zero specific patterns evaluated —
      `work start <path>` stays byte-identical to F3 (contract test + existing
      `f1_regression_test.go`/`start_by_name_test.go` suites unchanged).

**Checkpoint**: A plugin-provided Starter can drive a full new-Work creation
end to end with F1-identical guarantees. F1–F3 suites green. Merge forward.

---

## Phase 5: User Story 3 — Contribute to a Resolved Reference Without the New-Work Steps (Priority: P2)

**Goal**: When a Starter response carries `start_modes`, `work start` offers
exactly those values; **contribution** checks out the Starter-resolved branch
directly with no slug/convention/prefix steps; **fork** runs the identical
new-Work journey using the Starter's repository/base branch.

**Independent Test**: With `specific-starter` returning
`start_modes: ["contribution", "fork"]` and a base branch, run `work start
<argument>`, select contribution mode, and confirm Work checks out that exact
branch with no slug/convention/prefix prompts and no `branch_convention`
persisted.

Branch: `feature/005-plugin-origins-p4-us3-modes`, cut from Phase 4 tip.

### Tests for User Story 3

- [X] T044 [P] [US3] `tests/integration/start_modes_test.go` (PTY): with
      `specific-starter` returning `start_modes:["contribution","fork"]` +
      `base_branch`, the Mode step offers exactly those two; selecting **fork**
      skips the base-branch prompt (already supplied) then runs the full
      slug/convention/prefix sequence, persisting `start_mode: "fork"`;
      selecting **contribution** shows no slug/convention/prefix step, checks
      out the existing branch, and persists `start_mode: "contribution"` with
      no `branch_convention` key in `work-state.json` (quickstart S8, S9).
- [X] T045 [P] [US3] Extend the same test file for S10: contribution mode
      cancelled mid-confirmation leaves no worktree/dir/snapshot/index entry,
      and the Starter-resolved branch still exists in the source repository.
- [X] T046 [P] [US3] `tests/contract/starter_match_test.go`: an unrecognized
      `start_modes` value, and `"contribution"` present with no `base_branch`,
      both → `starter-response-invalid` (37); a response carrying `meta`/
      `links` still creates a Work whose `work-state.json` `meta`/`links` stay
      `{}` (unconsumed, research R8).
- [X] T047 [P] [US3] Extend `internal/create/create_test.go`: in contribution
      mode, `base_branch` in the built snapshot equals `branch` (the
      Starter-resolved branch checked out directly — there is no separate
      base); confirm (by reading, not editing) that
      `internal/work/verify.Check`'s plain-string field comparisons already
      treat an empty `Slug`/`BranchConvention` as coherent when both the
      snapshot and the projection row agree — no code change expected there.

### Implementation for User Story 3

- [X] T048 [US3] `internal/cli/start.go` (research R9): immediately after
      `Invoke`, validate the raw response shape before any Work
      materialization — `start_modes` containing a value outside
      `{"contribution","fork"}`, or containing `"contribution"` with no
      `base_branch`, both fail `starter-response-invalid` (37).
- [X] T049 [US3] `internal/cli/start.go`: once the repository is resolved
      (and disambiguated, if needed), when the Starter response carries
      `start_modes`, add a `present.SelectStep` ("Mode") offering exactly those
      values and no others; when `base_branch` is present, use it directly and
      skip the base-branch prompt (FR-024); when absent, prompt as today
      (FR-023) — but only in fork/new modes (contribution's missing
      `base_branch` is already rejected by T048).
- [X] T050 [US3] `internal/cli/start.go`: branch the journey on the
      selected/implied mode. `"new"`/`"fork"`: unchanged
      slug/convention/prefix/confirmation sequence; `work.start_mode` set to
      the resolved value. `"contribution"`: skip slug, convention, and prefix
      entirely; the confirmation preview names the branch being checked out,
      not a base+new-branch pair; `Branch` and `BaseBranchShort` in
      `create.Params` both equal the Starter-resolved branch.
- [X] T051 [US3] `internal/cli/start.go` non-interactive path: `start_modes`
      presence has no non-interactive mode-selection flag in F4 (Out of
      Scope) — a non-interactive invocation against such a Starter response
      fails with an actionable usage error rather than silently defaulting to
      a mode (documented gap, `contracts/cli-work-start.md` §New clause: start
      modes).
- [X] T052 [US3] `internal/cli/start.go`: pass the resolved `StartMode` into
      `create.Params.StartMode` (Phase 2's `WorktreeAddExisting` branch/
      compensator, T019, is now exercised end to end); verify via T045 that a
      failure/cancellation never deletes a contribution-mode branch (SC-009,
      research R12).

**Checkpoint**: Contribution and fork modes both work end to end with the
correct persisted shape and rollback guarantees. F1–F3 suites green. Merge
forward.

---

## Phase 6: User Story 4 — Resolve a Starter Collision Explicitly (Priority: P3)

**Goal**: Two Starters matching the same argument never auto-resolve: an
interactive `present.Select` names both before either runs; a non-interactive
invocation fails `starter-ambiguous` (36) with no selector.

**Independent Test**: Install two fixture Starters whose patterns both match
one argument; confirm both are listed for explicit selection with no default,
the chosen Starter (and only that one) is invoked, and a repeat run asks
again rather than remembering.

Branch: `feature/005-plugin-origins-p5-us4-collision`, cut from Phase 5 tip.

### Tests for User Story 4

- [X] T053 [P] [US4] `tests/contract/starter_match_test.go`: `specific-starter`
      + `colliding-starter` both matching one argument → `Outcome.Ambiguous`
      naming both, neither invoked yet; running the same collision twice with
      different choices shows both runs ask again (no memoization anywhere on
      disk, SC-006).
- [X] T054 [P] [US4] `tests/integration/starter_collision_test.go` (PTY): with
      both fixtures installed and matching, interactive `work start
      demo-pr-1` shows a Starter selector (no ranking) **before** the SOURCE
      step's own validation runs; only the chosen component is invoked;
      running it twice may pick a different Starter each time (quickstart
      S11).
- [X] T055 [P] [US4] Non-interactive collision case (extend
      `tests/integration/starter_collision_test.go` or a new txtar): explicit
      `SOURCE` matching both fixtures → exit 36 (`starter-ambiguous`) naming
      both colliding Starters, no selector opened, no Work created (US4 AC4).
- [X] T056 [P] [US4] Non-interactive no-match case: no fallback registered and
      no pattern matches → exit 35 (`starter-not-matched`), actionable message,
      no Work created (quickstart S12).

### Implementation for User Story 4

- [X] T057 [US4] `internal/cli/start.go`: on `starter.Match` returning an
      `Ambiguous` outcome, interactively call `present.Select[registry.Component]`
      ("Starter", listing alias/display name, no ranking) **before** the
      SOURCE step's own path/name validation runs; invoke exactly the chosen
      component; the choice is not remembered for a later step or invocation
      (FR-012, SC-006).
- [X] T058 [US4] `internal/cli/start.go`: non-interactively (or an explicit
      `SOURCE` with no TTY), map an `Ambiguous` outcome to `starter-ambiguous`
      (36) naming the colliding Starters, opening no selector and creating no
      Work (FR-012's non-interactive clause, US4 AC4).
- [X] T059 [US4] `internal/cli/start.go`: no pattern match and no registered
      fallback maps to `starter-not-matched` (35) with an actionable "enable or
      install a Starter" message, no Work created (FR-013).

**Checkpoint**: Starter collisions are always an explicit, unmemoized choice;
non-interactive collisions fail deterministically. F1–F3 suites green. Merge
forward.

---

## Phase 7: User Story 5 — Inspect and Change the Remembered Branch Convention (Priority: P3)

**Goal**: The convention step generalizes from hardcoded `freeform` to the
full catalog, memoized per repository identity (ADR-0011) and reusable from
any clone; `work convention show|set` and the interactive hub manage that
memory directly.

**Independent Test**: Create a fork-mode Work against a repository, choosing
a convention; from a second clone of the same repository, `work convention
show` reports the same choice; `work convention set <other>` changes it, and
a subsequent fork-mode Work against either clone uses the new choice without
asking again.

Branch: `feature/005-plugin-origins-p6-us5-convention`, cut from Phase 6 tip.

### Tests for User Story 5

- [X] T060 [P] [US5] `tests/integration/convention_memory_test.go` (PTY): with
      `specific-starter`'s `gitflow` convention installed alongside the
      reference `freeform` (2 enabled), the first fork-mode `work start`
      against a repository shows a Convention step once and memoizes the
      choice; a second clone of the same repository shows no step and reuses
      the memoized choice via `work convention show` (quickstart S13).
- [X] T061 [P] [US5] `tests/integration/convention_show_set.txtar`: `show`
      unset (text `work: convention not set` / `--json`
      `{"convention":null}`) and set (after T060, from any clone); `set nope`
      → exit 38, nothing persisted; `set gitflow` → exit 0, subsequent `show`
      reflects it; every subcommand outside a git repository → exit 2, no
      crash; `show` never mutates state (FR-028).
- [X] T062 [P] [US5] `tests/integration/convention_hub_test.go` (PTY): `work
      convention` with no subcommand, interactive, inside a repository with
      2+ enabled conventions — shows the current choice, changing it persists
      immediately and prints the `work: (equivalent: \`work convention set
      <name>\`)` receipt; leaving it unchanged persists nothing; non-interactive
      bare command fails usage (exit 2), opening no selector (quickstart S14).
- [X] T063 [P] [US5] Extend `internal/cli/start_test.go` (or a new file) for
      the generalized convention step: a repository with exactly one enabled
      convention shows no step and silently memoizes it; 2+ enabled and
      unmemoized shows one `present.SelectStep` before the prefix step;
      already-memoized shows no step; contribution mode never reaches this
      step at all.

### Implementation for User Story 5

- [X] T064 [US5] `internal/cli/start.go`: generalize the convention step off
      the hardcoded `convention.Freeform` (research R15) — build the catalog
      via `convention.Load(reg)`; outside contribution mode, resolve the
      current repository's identity (`repoidentity.Identify`) and look up a
      memoized choice (`repoconv.Get`); memoized → use it, no step; unmemoized
      + exactly one entry → silently adopt and memoize, no step (mirrors the
      existing single-prefix collapse); unmemoized + 2+ entries → a
      `present.SelectStep` ("Convention") before the existing prefix step,
      memoizing the accepted choice (`repoconv.Set`) before the wizard
      advances. Contribution mode skips this block entirely (unchanged from
      Phase 5).
- [X] T065 [US5] `internal/cli/convention.go`: the `work convention` parent —
      shared repository-identity resolution (`gitx.DiscoverRepoRoot` +
      `repoidentity.Identify`), failing "not inside a git repository" (usage,
      2) outside a repo; interactive no-subcommand form opens a
      `present.Wizard` hub (read-only current-choice line, a `SelectStep`
      offering every enabled convention plus "leave unchanged"); on a change,
      persists it and prints the equivalent `work convention set <name>`
      receipt (ADR-0019); non-interactive no-subcommand form fails usage
      (exit 2), opening no hub.
- [X] T066 [US5] `internal/cli/convention_show.go`: `work convention show
      [--json]` — read-only; stdout `work: convention <name>` or `work:
      convention not set`; `--json` shape `{"identity":"<key>",
      "convention":"<name>"|null}`.
- [X] T067 [US5] `internal/cli/convention_set.go`: `work convention set
      <CONVENTION>` — looks up `CONVENTION` across every installed package's
      registered conventions; not found → `convention-unknown` (38), nothing
      persisted; found → `repoconv.Set`, stdout `work: convention set to
      <name>`.
- [X] T068 [US5] Register `newConventionCmd()` in `internal/cli/root.go` with
      `GroupID = groupAdmin`.

**Checkpoint**: Branch convention choice is a first-class, per-repository,
cross-clone memory with a full show/set/hub surface. All five user stories are
independently functional. F1–F3 suites green. Merge forward.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Compatibility sweep, presentation parity, docs. Branch:
`feature/005-plugin-origins-p7-polish`, cut from Phase 7 tip.

- [X] T069 [P] Presentation parity sweep: `NO_COLOR=1`/`TERM=dumb`/redirected
      streams carry zero ANSI for `work plugin install/list`, `work convention
      show/set`, and every new `present.Select`/`present.SelectStep` inside
      `work start` (extend `tests/integration/no_color_env.txtar`/
      `no_ansi_when_piped.txtar`/`stream_separation_test.go`).
- [X] T070 [P] `tests/integration/plugin_failures.txtar`: categories 31–38 each
      reachable as a distinct, actionable, no-partial-state exit
      (`plugin-invalid`, `plugin-alias-conflict`, `plugin-fallback-conflict`,
      `plugin-install-failed`, `starter-not-matched`, `starter-ambiguous`,
      `starter-response-invalid`, `convention-unknown`) in one script-readable
      sweep.
- [X] T071 [P] Extend `tests/integration/help_inventory.txtar` (or its `_test.go`
      companion): `work plugin` and `work convention` each appear exactly once,
      under **Administration**.
- [X] T072 Full regression run: F1 `quickstart.md` S1–S12, F2 S1–S13, F2.5
      Q1–Q12, F3 S1–S13 all green with no stdout/token/mutation/exit-code diff
      (SC-010); `tests/contract/starter_test.go` (the seed binary contract)
      stays untouched and green.
- [X] T073 [P] Run this spec's `quickstart.md` S1–S14 end to end and record any
      deviation.
- [X] T074 [P] CI matrix sanity: the `--link` directory-symlink path exercised
      on `ubuntu-latest`, `macos-latest`, `windows-latest`; confirm the
      Windows-Developer-Mode constraint is documented in code comments
      (research R2) rather than silently failing.
- [X] T075 [P] Refresh the managed Spec Kit agent-context section via
      `/speckit-agent-context-update` (or the skill) so `AGENTS.md`/`CLAUDE.md`
      reflect the shipped `internal/plugininstall`, `internal/repoidentity`,
      `internal/repoconv`, `work plugin`, and `work convention` surface.
- [X] T076 [P] Update `docs/add/add-0001-work-system-architecture.md`
      cross-references and any `docs/` command inventory to list `work plugin`
      and `work convention`; confirm ADR-0000/0002/0003/0004/0006/0011/0012
      need no text edit (plan Phase 0 note).
- [X] T077 Code-quality pass on `internal/plugininstall`, `internal/repoidentity`,
      `internal/repoconv`, `internal/starter`, `internal/create`, and the new
      `internal/cli/plugin*.go`/`convention*.go` (comments preserve *why* per
      CLAUDE.md; no process metadata in comments; public API documented).

---

## Phase 9: Review Corrections (PR #39 triage)

**Purpose**: Bring the code to the spec amendments made after PR review
(FR-004a, FR-004b, FR-006 reserved alias, FR-006a, FR-020, FR-020a, FR-033).
Each task is test-first and lands as its own commit. Independent of one
another except where noted.

- [X] T078 [US3] Contribution mode on a remote-only branch (FR-020, FR-033):
      failing tests first — fresh clone with the branch only under `origin/`
      yields an attached HEAD on a local tracking branch and a snapshot whose
      `branch` and `base_branch` are the local name; cancel/failure after the
      worktree step removes the created tracking branch; a branch that already
      existed locally is never deleted. Then `internal/cli/start.go`
      (`resolveContribution`) derives the local name from the remote-tracking
      `basebranch.Choice`, and `internal/create/create.go` (contribution
      branch) checks it out via `WorktreeAddExisting` and compensates by
      deleting the branch only when this run created it.
- [ ] T079 [P] [US1] Reserved reference alias (FR-006): failing tests first —
      `--as work-reference` and a manifest named `work-reference` exit 32 with
      the seed's directory and registry entries unchanged. Then
      `internal/plugininstall/install.go` treats an alias that already owns
      registered components (with no `Package`) as a conflict via the existing
      `aliasConflict`.
- [ ] T080 [P] [US1] Reinstall replaces exactly (FR-006a): failing tests first
      — reinstall dropping a component and a convention leaves neither in
      `work plugin list` or the catalog; a convention shared with another
      package stays. Then a `registry.Registry` helper removes an alias's
      components and its previous `Package`'s conventions (`Package.Conventions`)
      before `internal/plugininstall/install.go` registers the new manifest.
- [ ] T081 [P] [US1] Starter `pattern` validity (FR-004a): failing test first —
      a fixture whose `pattern` is `(unclosed` exits 31 with nothing
      registered. Then `internal/plugin/manifest.go` compiles the pattern in
      `parseComponent`; refresh the stale "only reaches the registry through
      internal/plugin" comment in `internal/starter/starter.go`.
- [ ] T082 [P] [US1] Alias grammar (FR-004b): failing tests first — `--as ..`,
      `--as a/b`, `--as x.old`, and a manifest named `..` exit 31 before
      `plugins/` is touched. Then `internal/plugin/manifest.go` validates
      `name` and `internal/plugininstall/install.go` validates `--as` against
      the one grammar, sharing a single validator.
- [ ] T083 [P] [US3] Slug-less Work identity (FR-020a): failing test first —
      two contribution Works in one repository on different branches render
      distinct, non-blank names. Then `internal/worklist/worklist.go` falls
      back to `Branch` when `Slug` is empty, in both `DisplayName` and the
      disambiguation key.
- [ ] T084 Add the `contracts/*.md` contract-test rows (S5b, S6b, S6c and the
      plugin-install rows) to the integration suites, then rerun T072/T073
      (full regression, quickstart) to confirm F1–F3 suites are unchanged
      (SC-010).

---

## Dependencies & Execution Order

### Phase dependencies

- **Phase 1 (Setup)**: no dependencies.
- **Phase 2 (Foundational)**: depends on Phase 1 — **blocks Phases 3–8**.
- **Phase 3 (US1)**: depends on Phase 2. MVP.
- **Phase 4 (US2)**: depends on Phase 3 (`internal/starter.Match`'s zero/one
  match path needs at least one installed specific Starter, i.e. `work plugin
  install` working).
- **Phase 5 (US3)**: depends on Phase 4 (widens the same Starter response and
  the same `start.go` region US2 wired).
- **Phase 6 (US4)**: depends on Phase 5 in the plan's stacked-branch model (it
  hardens the same `Match`/`start.go` pipeline all of US2/US3 touch), though
  its own collision logic only strictly needs Phase 4's `Match`.
- **Phase 7 (US5)**: depends on Phase 6 in the stacked-branch model; the
  convention memory itself only needs Phase 2 (`repoidentity`/`repoconv`) plus
  at least one working fork-mode journey from Phase 5.
- **Phase 8 (Polish)**: depends on Phases 3–7.

### User-story independence

- **US1** is independently testable once Phase 2 is done (install/list, no
  `work start` involvement).
- **US2** builds on US1 (needs an installed Starter) but its own match/resolve
  logic is independently testable via the contract test.
- **US3** builds on US2's pipeline; its mode-branching and rollback rule are
  independently testable once a Starter can return `start_modes`.
- **US4** is a hardening layer over US2's `Match` — testable once two fixture
  Starters can be installed and matched.
- **US5** is independently testable once Phase 2's `repoidentity`/`repoconv`
  exist; it only needs *some* fork-mode creation to have run once (from US2 or
  US3) to have something to remember.

### Within each phase

- Test tasks marked [P] can be written in parallel and should fail before the
  implementation tasks land.
- `internal/plugininstall` files: `swap.go` (T026) before `install.go`'s
  classification (T028) before link/remote (T029–T030) before alias/fallback
  rules (T031–T032) before the assembled pipeline (T033).
- `start.go` edits within a phase are sequential (same file); edits across
  Phases 4/5/6/7 build on each other in that order (Match → modes →
  collision → convention), all in the same file region.

### Parallel opportunities

- **Phase 1**: T002, T003, T004 all [P].
- **Phase 2**: T006, T008, T010, T012, T015, T018, T021 (tests) [P]; the
  registry/gitx/repoidentity/repoconv/work-state streams are independent of
  each other and of `internal/create`.
- **Phase 3–7**: all test tasks within a phase marked [P]; within Phase 3,
  T026–T030 form one sequential stream while T034 (the CLI parent) can start
  once T033's pipeline signature is stable.
- **Phase 8**: T069, T070, T071, T073, T074, T075, T076 all [P].

---

## Parallel Example: Phase 2 Foundational

```bash
# Registry + diagnostics stream (one dev):
Task T005 – diag categories 31-38
Task T007 – registry.Package
Task T008 – registry_test.go

# git/identity/convention-memory stream (in parallel, another dev):
Task T009 – gitx additions
Task T011 – internal/repoidentity
Task T013 – workhome.BranchConventionsFile
Task T014 – internal/repoconv

# Schema + create stream (in parallel, a third dev):
Task T016 – work-state.json schema 3
Task T017 – Validate conditional rule
Task T019 – create.Params.StartMode + existing-branch path
```

---

## Implementation Strategy

### MVP (Phases 1–3)

1. Phase 1 Setup → Phase 2 Foundational (registry, diag, gitx, repoidentity,
   repoconv, schema 3, create's existing-branch path).
2. Phase 3 US1: `work plugin install`/`work plugin list` fully working.
3. **STOP and VALIDATE**: quickstart S1–S6; F1–F3 suites green.
4. Demo: install a plugin from a local path (pinned and linked) and from a
   remote source, and list it back.

### Incremental delivery

- + Phase 4 (US2): a plugin Starter drives a full new-Work creation → demo
  quickstart S7.
- + Phase 5 (US3): contribution and fork modes → demo quickstart S8–S10.
- + Phase 6 (US4): explicit, unmemoized Starter-collision choice → demo
  quickstart S11.
- + Phase 7 (US5): branch convention catalog + per-repository memory + hub →
  demo quickstart S13–S14.
- + Phase 8: compatibility sweep → release `release/0.5`.

Each phase merges forward only after its Checkpoint is met and `make lint` +
`go test ./...` are green on `ubuntu-latest`, `macos-latest`, `windows-latest`,
with every prior slice's demo (F1 S1–S12, F2 S1–S13, F2.5 Q1–Q12, F3 S1–S13,
earlier F4 phases) still green.

---

## Notes

- [P] = different files, no ordering dependency.
- [Story] label maps a task to a spec user story (US1–US5) for traceability.
- `work.db` needs no migration anywhere in F4 — if a task seems to require a
  projection column or DDL change, stop and re-read data-model.md §6 / research
  R11.
- The seed binary contract test `tests/contract/starter_test.go` must stay
  green and untouched.
- Branch names use one hyphenated segment under `feature/`
  (`feature/005-plugin-origins-p<n>-<short>`), never a nested path.
- Confirm base/target branch with the user at the start of every phase (per
  plan Branching Strategy — prior phases were stacked rather than targeting
  `develop`).
