# Implementation Plan: New Origins via Plugin (F4)

**Branch**: spec/plan/doc work on `feature/005-plugin-origins-specs-00`; F4
implementation uses one `feature/005-plugin-origins-p<n>-*` branch per phase (see
**Branching Strategy**) | **Date**: 2026-09-12 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/005-plugin-origins/spec.md`;
authorities `docs/prd.md` (FR-1, FR-3, FR-6, FR-16, FR-17, FR-21–FR-23, FR-25,
RNF-2), `docs/add/add-0001-work-system-architecture.md` §4, §5, §7, §8,
ADR-0000, ADR-0002, ADR-0003, ADR-0004, ADR-0006, ADR-0011, ADR-0012.

## Summary

F4 makes the core's domain surface genuinely pluggable: a package installed
entirely outside the binary can teach `work start` a brand-new argument syntax,
and the resulting journey — including the two modes only a plugin can produce,
`contribution` and `fork` — ends in the exact same worktree/snapshot guarantees
as an F1 direct-path creation. This is the roadmap's stated architectural proof
point for F4 ("Extensible beta"): after this slice, adding an origin is a
packaging problem, not a core-code problem.

Three earlier slices left more groundwork dormant than usual, and reading it
narrows F4 to four real verticals:

- `internal/plugin` **already validates every role** (`starter`,
  `repository-locator`, `importer`, `linker`) against the ADD §4.1 field table —
  F1 built the general validator to seed the reference package and left
  `importer`/`linker` rules unused on purpose (its own doc comment says so).
  F4 does not touch this validator; it only reaches it from a new public
  command instead of only from bootstrap.
- `internal/registry` and `internal/convention` are already written against a
  general multi-entry catalog (component identity is `(alias, name)`; the
  convention catalog is loaded from the registry, not hardcoded) — F1 seeds
  exactly one of each on purpose, so a second installed package needs no
  change to either package, only new data in them.
- `gitx.WorktreeAddExisting` already exists — built for the F2 archive
  compensation path, but it is exactly the primitive contribution mode needs
  to check out an already-existing branch without creating one.
- `work-state.json`'s own F1 schema comments say, verbatim, "`'contribution' /
  'fork'` arrive with F4" for `start_mode`, and "omitted only in contribution
  mode (unreachable in F1)" for `branch_convention` — the persisted-state shape
  this slice needs was specified two slices ago and never used until now.
- `work.db`'s `branch_convention` column is already nullable with no CHECK
  constraint — F4 needs **no projection migration** at all.

So F4 is genuinely new logic in four places, each independently testable:

1. **A plugin install pipeline** (`internal/plugininstall`), reusing the exact
   staging/atomic-swap primitives `internal/bootstrap` already uses to install
   the embedded seed (ADR-0002's "same pipeline" is implemented literally, not
   just conceptually) — `work plugin install <source> [--link] [--as <alias>]`
   and `work plugin list [--json]`. Local development linking is a directory
   symlink (junction on Windows) at `plugins/<alias>/source`, not a new code
   path through `registry.Component.EntrypointPath` — every existing invocation
   call site (`internal/starter`, `internal/locator/chain.go`) needs zero
   change.
2. **Starter pattern matching and collision** (`internal/starter.Match`),
   replacing F1/F3's "always the one fallback" with the two-layer resolution
   ADR-0004 already specified: zero/one/many pattern matches against every
   registered specific Starter, falling back to the single registered fallback,
   surfacing a collision the same shape as F3's repository ambiguity (a
   `present.Select` step interactively, an actionable non-interactive failure
   otherwise) — never persisted, asked again every time.
3. **Start modes and contribution/fork journeys**, widening `work-state.json`
   to schema 3 (`start_mode` gains `contribution`/`fork`; `slug` and
   `branch_convention` both become required-unless-contribution, mirroring the
   `archived_at` conditional the schema-2 contract already established for F2)
   and teaching
   `internal/create` a second materialization path — checkout of an
   already-existing branch, whose rollback must never delete that branch,
   because contribution mode never owns it.
4. **A branch convention catalog with per-repository memory** (ADR-0011): a new
   `internal/repoidentity` (the three-layer identity key: remote URL, then
   sorted root-commit hashes, then absolute path for a remote-less shallow
   clone) and a new `internal/repoconv` (the generated, never-hand-edited
   memoization store), wired into `work start`'s existing prefix-style
   selection step, plus the first genuinely interactive **hub** this codebase
   ships — `work convention` — because, unlike `work plugin` and `work
   repository`, ADR-0011/FR-030 do not defer it.

No dependency, binary, or IPC transport is new (ADR-0000/ADR-0006 are consumed
exactly as documented). `config/work.json` and `registry.json` need no new
top-level keys beyond `registry.json`'s additive `packages` array (unversioned,
regenerated state). `work.db` stays at `user_version = 2`. Only
`work-state.json` moves to schema 3 — an additive superset of schema 2, exactly
as schema 2 was an additive superset of schema 1.

## Technical Context

**Language/Version**: Go 1.26 (toolchain go1.26.x). Single module
`github.com/gustaborges/work`. No new language features.

**Primary Dependencies** (all already vendored by F1/F2/F2.5/F3 — F4 adds
none):
- `github.com/spf13/cobra` — `work plugin install|list` and `work convention
  show|set` join the existing command tree; `plugin`'s `GroupID` is
  `groupAdmin` (mirrors `repository`); `convention` is also `groupAdmin`.
- `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2` — only via
  `internal/present`; F4 composes `present.Select`/`present.Wizard` for the
  Starter-collision step, the start-mode choice, the convention step, and the
  new `work convention` hub. No new Bubble Tea model.
- System `git` — invoked via `internal/gitx` for: remote plugin install
  (`git clone`, shallow), repository identity (`git remote get-url`, `git
  rev-list --max-parents=0 HEAD`, shallow detection), and contribution-mode
  checkout (`git worktree add <dir> <branch>`, no `-b`, via the already-existing
  `WorktreeAddExisting`). No new git-adjacent subprocess.
- `modernc.org/sqlite` — untouched; F4 issues no new projection query shape
  (existing `branch_convention`/`start_mode` columns are already unconstrained
  TEXT).

**Storage**:
- `work-state.json`: **schema 3** (additive over schema 2). `work.start_mode`
  enum widens from `{"new"}` to `{"new","contribution","fork"}`.
  `work.slug` and `work.branch_convention` both become conditionally
  required — required unless `start_mode == "contribution"`, and MUST be
  absent exactly then (contribution mode never runs the slug or convention
  step, FR-020; the same `if/then/else` shape schema 2 already uses for
  `archived_at`). `work.base_branch` stays required in every mode: in
  contribution mode it
  carries the same value as `work.branch` (the Starter-resolved branch is
  checked out directly; there is no separate "base" to record — ADD §7's PR
  example reuses one `base_branch` field for both the fork-mode base and the
  contribution-mode target). `Read` accepts schema 1–3; `Write` always emits 3.
- `work.db`: **no migration.** `SchemaVersion` stays 2 — `branch_convention` is
  already a nullable, unconstrained column and `start_mode` carries no CHECK
  constraint (`internal/projection/projection.go`, migrations table). F4 adds
  no query shape.
- `registry.json`: additive, unversioned (regenerated from installed
  manifests, never hand-edited). Gains `packages: []Package{alias, origin,
  reference}` — the per-package record `work plugin list` reads (FR-007);
  `components[]`/`conventions[]` are unchanged in shape, just populated by more
  than one package after F4.
- `config/work.json`: **no new keys.** Repository policy/roots are untouched by
  F4 (a plugin-provided Repository Locator is exercised through F3's existing,
  unchanged contract — Out of Scope).
- New generated file `~/.work/state/branch_conventions.json`
  (`workhome.Home.BranchConventionsFile()`): the per-repository-identity
  convention memory (`internal/repoconv`). Lives beside `registry.json`, not
  inside it — it is per-repository *choice* state, not the *catalog* the
  registry already models, and it is not derivable by reconciling `work.db`
  from snapshots (a Work's own snapshot records only its own chosen
  convention, never the repository's identity key).

**Testing**:
- `internal/plugin` unit tests: unchanged (already exercises all four roles);
  F4 adds no new manifest-parsing rule.
- `internal/plugininstall` unit tests: local pinned copy vs. `--link` symlink
  (content never duplicated on disk in the linked case, SC-003); remote pinned
  install records the cloned `HEAD` commit as the reference; alias default vs.
  `--as`; alias collision (different origin → fail, nothing registered — SC-004)
  vs. idempotent reinstall (same origin, same alias); fallback-Starter conflict
  at install time (edge case, FR-011); manifest violating role rules rejects
  the whole install with zero partial registration (SC-005); `--link` with a
  remote source rejected before anything runs (FR-002).
- `internal/starter` unit tests: `Match` against fixture patterns — zero
  matches → the fallback; one → direct invoke; many → an `Ambiguous` outcome
  listing exactly the colliding components; no fallback and no match →
  `starter-not-matched`; the choice is never memoized across two calls with the
  same argument (SC-006).
- `internal/repoidentity` unit tests: the three-layer key against `gittest`
  fixtures — origin-remote repos, no-remote repos (root-commit hash, including
  a merged-unrelated-histories composite key sorted deterministically), and a
  shallow clone with no remote (path fallback); moving/recloning a repository
  with a stable remote or root commit yields the same key.
- `internal/repoconv` unit tests: get/set/idempotent-load against a temp state
  file; unknown identity returns "unset", never an error.
- `internal/work` unit tests: `Validate` accepts `contribution` with
  `branch_convention` absent and rejects it present; accepts `fork`/`new` with
  `branch_convention` required; schema 1/2 documents still `Decode` cleanly
  (regression).
- `internal/create` unit tests: the contribution-mode path calls
  `WorktreeAddExisting`, not `WorktreeAdd`; its rollback compensator removes
  the worktree but never deletes the branch (the critical divergence from
  fork/new); the fork/new path is byte-identical to F1/F3 behaviour.
- `tests/contract/`: a starter-match contract exercising the same fixture
  binaries pattern F3 established for locators (`tests/fixtures/plugins/`); a
  plugin-manifest contract confirming the F1 validator rejects the four
  documented violation shapes with zero registration.
- `tests/integration/` `testscript`: `work plugin install [--link] [--as]`
  output/exit codes across local/remote/link/alias-conflict/fallback-conflict/
  manifest-invalid; `work plugin list [--json]` shape and purity;
  `work convention show [--json]` purity; `work convention set <name>` and its
  rejection of an unknown name (nothing persisted); `work plugin`/`work
  repository` unchanged help-only-parent behaviour; `NO_COLOR`/`TERM=dumb`/piped
  output for every new command.
- `tests/integration/` PTY (`//go:build unix`): `work start <plugin-arg>` with
  a single-pattern-match fixture Starter completing the full new-Work journey
  end-to-end (US2); the same fixture with `start_modes: ["contribution",
  "fork"]` — contribution skips slug/convention/prefix, fork runs the full
  sequence (US3); two colliding fixture Starters presenting an explicit
  `present.Select` before either runs (US4); first fork-mode use of a
  repository with 2+ enabled conventions prompting once and reusing the choice
  from a second clone (US5); the new `work convention` hub.
- `tests/integration/` non-interactive: every new failure category (31–38)
  reachable with the documented exit code and no partial state; a
  non-interactive Starter collision fails without opening a selector (US4 AC4).
- Regression: F1 (S1–S12), F2 (S1–S13), F2.5 (Q1–Q12), F3 (S1–S13) stay green
  unchanged — `work start <path>` and the reference Starter's fallback-only
  behaviour are byte-identical; no existing stdout line, error token, or exit
  code shifts (SC-010).
- CI matrix unchanged: `ubuntu-latest`, `macos-latest`, `windows-latest`. The
  `--link` symlink path is exercised on all three; Windows without Developer
  Mode / admin is documented as a known constraint of a development-only flag,
  not a supported-platform regression (see Constraints).

**Target Platform**: Same as F1–F3 — Linux (amd64, arm64), macOS (arm64,
amd64), Windows (amd64). No change to the supported terminal/shell matrix.

**Project Type**: Single-project CLI tool (unchanged). Four new `internal/`
packages (`plugininstall`, `repoidentity`, `repoconv`, plus `starter` and
`registry` gaining new exported surface rather than new packages), two new
`internal/cli` command families (`plugin*`, `convention*`), and edits to
`internal/cli/start.go`, `internal/create`, `internal/work`, `internal/diag`,
`internal/gitx`, `internal/bootstrap` (delegates its staging/swap to the
extracted primitives).

**Performance Goals**: Not latency-critical. Plugin install is a one-shot,
user-initiated operation (network/clone latency is the plugin author's/source
host's concern, not a budget this plan sets). Starter pattern matching against
an installed set is local regexp evaluation with no measurable SC target (the
same "not a hot loop" reasoning ADR-0000 already gives for subprocess spawns
applies unchanged).

**Constraints**:
- The chosen Starter receives only `{ "arg": <argument> }` — no start mode,
  base branch, metadata, links, or other Work-derived data (FR-014, unchanged
  trust boundary from F1's `ipc-starter.md`).
- A Starter's `meta`/`links` output fields are read by the wire format but
  **deliberately still discarded** in F4 — only `repository`, `base_branch`,
  and `start_modes` are consumed. Persisting `meta`/`links` from a Starter
  response is F5 scope (`start:finalized`); F4 must not implement it early.
- Repository Reference resolution is unchanged: a plugin-provided reference
  goes through the identical direct-path-or-Locator-chain pipeline as the
  reference Starter's, with no Starter-specific or plugin-specific branch
  anywhere in `internal/locator` (FR-017, FR-018).
- No score, rank, confidence, or install-order precedence for Starter
  matching, mirroring the existing rule for Locators (ADR-0004, NFR-9): exactly
  one pattern match wins; zero falls back; more than one is always an explicit
  choice, never memoized (FR-012, SC-006).
- At most one enabled fallback Starter at a time; a second is rejected at
  install time, not silently layered (FR-011). There is no enable/disable
  surface in F4 (deferred to F7) — "enabled" reduces to "registered" for every
  rule in this slice.
- `--link` never copies content and is rejected outright against a remote
  source (FR-002); a plain local or remote install always fixes a pinned
  reference at install time (FR-003, SC-003).
- An alias conflict at install time fails before registering anything, with no
  silent overwrite, rename, or auto-suffix; the same origin under the same
  alias is idempotent (FR-006, SC-004).
- A manifest violating role-based field validation is rejected before
  registering any of its components — zero partial registration (FR-004,
  SC-005). The validator itself (`internal/plugin`) is unchanged; only its
  install-time caller is new.
- Contribution mode never runs slug/convention/prefix and never requires the
  checked-out branch to be new; its rollback never deletes that branch — it
  was never created by this Work (FR-020, SC-007, SC-009).
- Fork mode and the absent-`start_modes` path are the identical new-Work
  journey used without a plugin, ending in `work.start_mode` = the Starter's
  actual mode, or `"new"` when `start_modes` was absent — never inferred or
  overridden (FR-021, FR-022).
- Outside contribution mode, on a repository's first use, an explicit
  convention choice is required only when more than one is enabled; the choice
  is memoized per repository identity and reused from any clone (FR-025,
  FR-026, ADR-0011).
- `work convention set <name>` rejects a name that is not currently enabled,
  persisting nothing (FR-029). `work convention show` is read-only in every
  respect (FR-028).
- Every F1/F2/F2.5/F3 command grammar, transaction guarantee, stable stdout
  line, error token, and exit code is preserved; `work start <path>` and the
  reference Starter's resolution are unchanged (FR-032, SC-010).
- A `work start` invocation that fails or is cancelled at any point — Starter
  match, Starter execution, resolution, mode/branch/convention/slug selection,
  branch-name validation — leaves no branch, worktree, Work directory,
  snapshot, or index entry (FR-033, SC-009).
- `work plugin install`/`work plugin list` have stable, script-usable output
  and exit codes; `list` accepts `--json` and mutates nothing (FR-034).
- A directory-symlink `--link` install requires Developer Mode (or an
  elevated process) on Windows to create the link; this is an accepted,
  documented constraint of a development-only flag, not a regression of the
  supported-platform matrix — the plain local and remote install paths need no
  such privilege on any platform.

**Scale/Scope**: Single user, single machine. Code surface: `internal/
plugininstall` (≈4 files), `internal/repoidentity` (≈2 files), `internal/
repoconv` (≈2 files), `internal/cli/plugin*.go` + `internal/cli/convention*.go`
(≈5 files), edits to `starter.go`, `registry.go`, `work/state.go`,
`create/create.go`, `diag/diag.go`, `gitx/gitx.go`, `bootstrap/bootstrap.go`,
`start.go`. One new fixture tree, `tests/fixtures/plugins/`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unratified template, exactly as
for F1–F3. As before, this plan is held to the **cross-cutting gates in
`docs/roadmap.md` §4** and the spec's **Success Criteria**, treated as binding.

| Gate (roadmap §4) | How this plan satisfies it | Verified by |
|---|---|---|
| Determinism | Starter matching has no score/rank/priority: exactly one pattern match wins, zero falls back to the single registered fallback, more than one is always an explicit unmemoized choice (ADR-0004). Convention choice is either forced (exactly one enabled) or explicitly chosen and memoized by repository identity, never inferred from repository content. Alias/fallback conflicts fail deterministically at install time, never by installation order. | `internal/starter` unit tests; SC-006; `internal/plugininstall` unit tests; SC-004, SC-005 |
| Integrity | Plugin install is staged-then-atomically-swapped exactly like `bootstrap.install` (shared primitives); a failed or interrupted install leaves the previous state intact and registers nothing partial. `work start` failures at any new stage (match, invoke, mode/branch/convention selection) unwind before `create.Run`'s commit point exactly as F1's pipeline already does; contribution mode's compensator is the one new rollback rule (never deletes the pre-existing branch). | `internal/plugininstall` interruption tests (mirroring `bootstrap_test.go`/`stress_test.go`); `internal/create` compensator tests; SC-005, SC-009 |
| Process contract | `work plugin install`/`list` and `work convention show`/`set` follow the established stable-stdout / `error: <token>: <message>` / fixed-exit-code contract; 8 new categories (31–38) appended to `diag.All` with fixed codes (table test extended, mirroring F3's 26–30 addition). `work plugin list` and `work convention show` honour `--json`; both mutate nothing. | `diag` table test; `testscript` suite; SC-004, SC-005, SC-009 |
| Portability | No new IPC transport; plugin subprocess invocation reuses `ipc.Run`/`InvokeStarter` unchanged (ADR-0000/ADR-0006). Remote install and repository identity add `git` subcommands only, via `internal/gitx`, already cross-platform. `--link`'s directory symlink is the one genuinely OS-sensitive addition; it is documented as a Windows-Developer-Mode constraint of a dev-only flag rather than treated as a portability regression. Same 3-OS CI. | CI matrix; `internal/plugininstall` per-OS link tests; `internal/repoidentity` cross-platform `gittest` fixtures |
| Auditability | Each new failure category names what failed in user vocabulary with a next action (`diag.Error.Summary`/`Hint`), exactly like F1–F3's categories; a colliding-Starter or unknown-convention diagnostic never prints subprocess stderr or repository contents in normal output — only in the existing `WORK_DEBUG` cause channel. `work plugin list` never leaks a remote source's credentials (it prints only the resolved reference). | `diag` renderer tests; FR-018 (Starter trust boundary); SC-003 |
| UX | `work plugin` stays a help-only parent (mirrors `work repository`'s F3 precedent verbatim — `install`/`list` are its only children); `work convention` is this codebase's first genuinely interactive hub, showing the current choice and revealing the equivalent direct command after a change, exactly as ADR-0019/FR-030 require and exactly as F3 explicitly deferred for `work repository`. Starter collision and the fork/contribution choice reuse the existing bounded, keyboard-navigable `present.Select`. Non-interactive ambiguity and every new failure are actionable, never a hang or a silently-opened selector. | help-inventory test (unchanged shape, two more admin-group entries); PTY hub + collision tests; `testscript`; SC-002, SC-006, SC-007 |
| Regression | F1 S1–S12, F2 S1–S13, F2.5 Q1–Q12, F3 S1–S13 stay green unchanged; `work start <path>` and the reference Starter's fallback-only journey keep their exact contract (stdout, exit codes, transaction, snapshot shape) as the compatibility baseline — a schema-3 writer still satisfies every schema-1/2 reader assumption those suites depend on (`start_mode: "new"`, `branch_convention` present). | CI; SC-010 |

**Architectural-authority gates (PRD → ADR → ADD):**

- **PRD**: FR-1, FR-3, FR-6, FR-16, FR-17, FR-21–FR-23, FR-25, and RNF-2 are the
  product authority the spec cites; F4 delivers them fully within its stated
  scope (plugin enable/disable/update/uninstall and the full `work plugin` hub
  remain F7, per the roadmap). ✅
- **ADR-0000** (accepted): every plugin component still runs as an external
  subprocess via the unchanged `internal/ipc` contract; the core never loads
  plugin code in-process. Remote install's `git clone` is the one
  core-in-process operation this slice adds, and it is `git` — the sole
  exception ADR-0000 already carves out. ✅
- **ADR-0002** (accepted): local pinned/linked and remote pinned install share
  one pipeline; alias defaults to the manifest `name`, collision fails
  explicitly with no auto-suffix, `--as` resolves it, reinstalling the same
  origin under the same alias is idempotent; the registry stays generated,
  never hand-edited; installing a Locator never touches the resolution policy
  (unchanged F3 behaviour — this slice adds no Locator-policy interaction).
  `internal/plugininstall` and the extended `registry.Package` implement
  exactly this. ✅
- **ADR-0003** (accepted): the embedded seed still installs through the
  identical pipeline as any third-party plugin — this plan makes that literal
  by having `bootstrap.install` call the same staging/swap primitives
  `plugininstall` exposes, rather than merely resembling them. ✅
- **ADR-0004** (accepted): two-layer Starter resolution (fallback + specific),
  no manual ordering, collision resolved by explicit unmemoized choice reusing
  the same selection component as mode disambiguation. `internal/starter.Match`
  implements exactly this, with zero score/priority/specificity comparison. ✅
- **ADR-0006** (accepted): no language/stack restriction on a plugin component;
  runtime declared explicitly and checked at install time (already implemented
  by `internal/plugin`); no shebang/exec-bit reliance (unchanged `ipc.Run`
  direct-invocation model). F4 adds no new invocation mechanism — it only
  reaches more of what already exists from a public command. ✅
- **ADR-0011** (accepted): the three-layer repository identity key, interview
  + memoization convention model, and the dedicated `work convention` surface
  (not folded into `work plugin`, not a `work start` flag) are delivered
  verbatim by `internal/repoidentity` + `internal/repoconv` + `internal/cli/
  convention*.go`, including the one interactive hub this slice actually
  builds. ✅
- **ADR-0012** (accepted): component role is the sole semantic discriminator;
  `conventions[]` stays separate, non-executable, and outside role validation;
  no `invocation`/`type`/`capabilities`/`hooks`/`priority`/`score` field
  anywhere. `internal/plugin` already encodes this precisely — F4 changes
  nothing about the manifest model, only who can trigger its validation. ✅
- **Historical contracts**: `specs/001-first-local-work/contracts/
  ipc-starter.md` describes only the fallback Starter's response shape (path
  only, `start_modes` always absent). F4's `contracts/starter-protocol.md`
  **extends, not supersedes**, it: every rule in the F1 contract still holds
  for the fallback Starter; the extension documents what a *specific* Starter
  may additionally return. `specs/001-first-local-work/contracts/
  plugin-manifest.md` is authoritative and unchanged — this plan's
  `contracts/cli-work-plugin.md` documents only the new install-time caller
  around it (alias/`--link`/`--as`/collision), not a new validation rule.
  `specs/002-daily-cycle/contracts/work-state.schema.json` (schema 2) is
  superseded by this slice's schema 3 exactly as schema 1 was superseded by
  schema 2 — additively, with both older documents left intact as the
  historical record. ✅

**Gate result: PASS.** No violations requiring Complexity Tracking.

### Post-design re-check (after Phase 1)

Re-evaluated after `research.md`, `data-model.md`, `contracts/`, and
`quickstart.md`. Still **PASS**:

- No new dependency, no new IPC transport, no new binary beyond the fixture
  plugins under `tests/fixtures/plugins/` (test-only, never shipped).
- `work.db` needs no migration (confirmed against the live `projection.go`
  migrations table — `branch_convention` is already nullable, unconstrained
  TEXT); `config/work.json` gains no key; `registry.json`'s `packages` array is
  additive and unversioned.
- `contracts/work-state.schema.json` (schema 3) makes the
  `branch_convention`-required-unless-contribution rule a mechanical JSON
  Schema `if/then/else` assertion, the same pattern schema 2 already
  established for `archived_at` — not a new kind of rule, a third instance of
  one.
- `contracts/starter-protocol.md`, `contracts/cli-work-plugin.md`,
  `contracts/cli-work-convention.md`, and the `cli-work-start.md` amendment
  each pin one of the four verticals in the Summary to a concrete, testable
  shape.
- `quickstart.md` re-runs the F1+F2+F2.5+F3 validation set as the
  compatibility proof (SC-010) and adds S1–S14 for F4's five user stories plus
  the new failure categories.
- All seven roadmap §4 gates have a concrete contract or test harness (table
  above → `contracts/` and `research.md`).

## Project Structure

### Documentation (this feature)

```text
specs/005-plugin-origins/
├── plan.md                       # This file
├── spec.md                       # Feature specification
├── research.md                   # Phase 0 output — decisions R1–R19
├── data-model.md                 # Phase 1 output — plugin/registry/starter/work/convention entities
├── quickstart.md                 # Phase 1 output — validation scenarios S1–S14 + F1/F2/F2.5/F3 regression
├── contracts/                    # Phase 1 output
│   ├── cli-work-plugin.md        # `work plugin install|list` grammar, registry Package shape, exit codes
│   ├── cli-work-convention.md    # `work convention show|set` + the interactive hub, exit codes
│   ├── cli-work-start.md         # F4 amendment: Starter matching/collision, start_modes, contribution/fork
│   ├── starter-protocol.md       # F4 extension of specs/001's ipc-starter.md: pattern, start_modes, base_branch
│   └── work-state.schema.json    # schema 3 — additive over specs/002's schema 2
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

Additions and edits to the F1–F3 tree:

```text
internal/
├── plugininstall/                # NEW — the shared install pipeline (ADR-0002, ADR-0003)
│   ├── install.go                #   InstallLocal(pinned|linked) / InstallRemote(pinned); alias resolution
│   ├── swap.go                   #   staging + atomic-swap primitives, extracted so bootstrap.install calls them too
│   ├── link.go                   #   --link: directory symlink/junction at plugins/<alias>/source, no copy
│   └── plugininstall_test.go
├── repoidentity/                 # NEW — ADR-0011 three-layer repository identity key
│   ├── repoidentity.go           #   Identify(repo gitx.Repo) (string, error): origin -> root commits -> path
│   └── repoidentity_test.go
├── repoconv/                     # NEW — persisted per-repository convention memory (generated state)
│   ├── repoconv.go               #   Load/Save/Get/Set against state/branch_conventions.json
│   └── repoconv_test.go
├── plugin/
│   └── manifest.go               # unchanged — already validates all four roles (no F4 edit)
├── registry/
│   └── registry.go               # + Package{Alias, Origin, Reference}; Packages field; UpsertPackage/PackageByAlias
├── starter/
│   └── starter.go                # + Match(reg, arg): pattern eval, fallback, Ambiguous outcome (ADR-0004);
│                                  #   Reference/response gains BaseBranch + StartModes (meta/links still discarded)
├── work/
│   └── state.go                  # Schema -> 3; StartModeFork/StartModeContribution; conditional branch_convention
├── create/
│   └── create.go                 # Params.StartMode; existing-branch checkout path; no-branch-delete compensator
├── gitx/
│   └── gitx.go                   # + Clone, Remotes/RemoteURL, RootCommits, IsShallow, DiscoverRepoRoot
├── bootstrap/
│   └── bootstrap.go               # staging/swap delegated to internal/plugininstall (ADR-0003 "same pipeline")
├── diag/
│   └── diag.go                    # + 8 categories 31-38 (plugin-invalid .. convention-unknown)
└── cli/
    ├── start.go                   # Starter match + collision step; start_modes offer; contribution/fork branch;
    │                               # convention step now reads/writes repoidentity+repoconv instead of hardcoded freeform
    ├── plugin.go                  # NEW — `work plugin` parent (help-only, mirrors repository.go), GroupID admin
    ├── plugin_install.go          # NEW — `work plugin install <source> [--link] [--as <alias>]`
    ├── plugin_list.go             # NEW — `work plugin list [--json]`
    ├── convention.go              # NEW — `work convention` parent: interactive hub, or help outside a TTY
    ├── convention_show.go         # NEW — `work convention show [--json]`
    ├── convention_set.go          # NEW — `work convention set <CONVENTION>`
    └── *_test.go

tests/
├── contract/
│   └── starter_match_test.go     # NEW — core-side pattern-match contract against fixture Starters
├── integration/
│   ├── plugin_install.txtar          # NEW — local/link/remote/alias-conflict/fallback-conflict/manifest-invalid
│   ├── plugin_list.txtar             # NEW — list [--json] shape + purity
│   ├── convention_show_set.txtar     # NEW — show/set + unknown-name rejection
│   ├── start_by_plugin_starter_test.go   # NEW (pty) — single-pattern-match fixture Starter, full US2 journey
│   ├── start_modes_test.go               # NEW (pty) — contribution vs. fork from the same fixture response
│   ├── starter_collision_test.go         # NEW (pty) — two colliding fixture Starters, explicit selection
│   ├── convention_memory_test.go         # NEW (pty) — first-use choice memoized, reused from a second clone
│   ├── convention_hub_test.go            # NEW (pty) — `work convention` interactive hub
│   └── plugin_failures.txtar             # NEW — categories 31-38 as distinct, actionable exits
└── fixtures/
    └── plugins/                          # NEW — tiny fixture plugin packages
        ├── specific-starter/             #   pattern match, start_modes contribution+fork, base_branch
        ├── colliding-starter/            #   pattern overlapping specific-starter's, for US4
        └── invalid-manifest/             #   role-rule violations, for FR-004 (never registered)
```

**Structure Decision**: Single Go project (unchanged). `internal/plugininstall`
owns the install pipeline and imports only `plugin`, `registry`, `diag`,
`atomicfile`, `gitx` — no CLI or `present` dependency, unit-testable in
isolation exactly like `internal/locator` in F3. `internal/repoidentity` and
`internal/repoconv` are similarly presentation-free. `internal/cli` keeps every
journey decision: `start.go` owns Starter matching, the collision step, mode
offering, and the convention step; `plugin*.go`/`convention*.go` own their
command grammars (the plugin parent stays help-only; the convention parent is
the one hub). `internal/starter`, `internal/work`, `internal/create`,
`internal/registry`, `internal/gitx`, and `internal/diag` gain fields/exported
functions/categories but no behavioural change to any existing call path that
does not opt into the new ones. `internal/bootstrap` changes only which
package supplies its staging/swap helpers — its own public behaviour
(`EnsureSeed`) is unchanged. No other package changes; `internal/locator`,
`internal/repoconfig`, and the F3 `work repository` surface are untouched.

## Branching Strategy

F4 follows the same git-flow shape as F1–F3: one short-lived
`feature/005-plugin-origins-p<n>-*` branch per `tasks.md` phase, each cut from
the previous phase's tip, merged forward at the phase **Checkpoint** once
`make lint` + `go test ./...` are green on the CI matrix.

> **Prerequisite**: F3 must have merged (the roadmap gate — F4 begins only once
> the Repository Resolution Policy and Locator chain are in place, since every
> plugin-provided reference resolves through them unchanged).

> **Merge cadence is the user's call.** Per the standing project note, prior
> phase branches were stacked (phase N+1's PR targets phase N's branch, not
> `develop`). Before starting a phase, `/speckit-implement` MUST confirm with
> the user which branch to base the new phase branch on and which branch its PR
> targets — do not assume `develop`.

| Phase (tasks.md) | Feature branch | Cut from |
|---|---|---|
| 0 — Specs & doc (this spec set; confirm ADR-0000/2/3/4/6/11/12 need no edit) | `feature/005-plugin-origins-specs-00` | F3 tip |
| 1 — Foundational: `registry.Package`; `diag` categories 31–38; `gitx` additions (Clone, Remotes/RemoteURL, RootCommits, IsShallow, DiscoverRepoRoot); `internal/repoidentity`; `internal/repoconv`; `work-state.json` schema 3 + `Validate` conditional rule; `internal/create` existing-branch path + compensator; no CLI wiring | `feature/005-plugin-origins-p1-foundational` | phase 0 tip |
| 2 — US1 Install a plugin 🎯 MVP: `internal/plugininstall` (local pinned/linked, remote pinned, alias/fallback rules); `bootstrap.install` delegates to it; `work plugin install`/`work plugin list` | `feature/005-plugin-origins-p2-us1-install` | phase 1 tip |
| 3 — US2 Start from a plugin Starter: `internal/starter.Match` (pattern eval + fallback, no collision UI yet); wire into `start.go`'s source step; full new-Work journey via a fixture specific Starter | `feature/005-plugin-origins-p3-us2-start` | phase 2 tip |
| 4 — US3 Contribution & fork modes: `start_modes` offering (`present.Select`), contribution-mode branching in `start.go` + `create.Run`, base-branch consumption when the Starter supplies one | `feature/005-plugin-origins-p4-us3-modes` | phase 3 tip |
| 5 — US4 Starter collision: interactive `present.Select` among colliding specific Starters; non-interactive `starter-ambiguous`; `starter-not-matched`; `starter-response-invalid` | `feature/005-plugin-origins-p5-us4-collision` | phase 4 tip |
| 6 — US5 Branch convention catalog & memory: convention step reads/writes `repoidentity`+`repoconv`; `work convention show|set` + the interactive hub | `feature/005-plugin-origins-p6-us5-convention` | phase 5 tip |
| 7 — Polish: full F1–F3 regression sweep; `NO_COLOR`/`TERM=dumb`/non-interactive parity for every new command; SC checks; docs | `feature/005-plugin-origins-p7-polish` | phase 6 tip |

Rules (identical spirit to F1–F3):

- **Naming**: `feature/005-plugin-origins-p<n>-<short>` — the phase identifier
  stays in one hyphen-separated segment under `feature/` (no nested path).
- **First action of each phase**: confirm base/target with the user, then
  `git switch <base> && git pull && git switch -c <feature-branch>`.
- **Merge gate**: a phase merges forward only after its **Checkpoint** in
  `tasks.md` is met and `make lint` + `go test ./...` are green on all three
  OSes, with every prior slice's automated demo (F1 S1–S12, F2 S1–S13,
  F2.5 Q1–Q12, F3 S1–S13, and earlier F4 phases) still green.
- **Release**: when phase 7 merges, F4 ships via `release/0.5` cut from
  `develop`, merged to `master` and tagged (standard git-flow release). The
  release step is the hand-off, not a task.

Spec/plan/contract/doc edits stay on the p0 branch; the per-phase feature-branch
rule covers implementation code only.

## Complexity Tracking

No Constitution Check violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
