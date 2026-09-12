# Implementation Plan: Find the Local Clone (F3)

**Branch**: spec/plan/doc work on `feature/004-local-clone-locator-specs-00`; F3
implementation uses one `feature/004-local-clone-locator-p<n>-*` branch per phase
(see **Branching Strategy**) | **Date**: 2026-09-07 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/004-local-clone-locator/spec.md`;
authorities `docs/prd.md` (FR-39/40/42/43/44/45/46/49, NFR-8/9),
`docs/add/add-0001-work-system-architecture.md` §7.1–§7.2, ADR-0014, ADR-0015,
ADR-0016.

## Summary

F3 is the first slice that expands the domain since F2. It makes
`work start <name-or-reference>` resolve a local clone through the **Repository
Resolution Policy** instead of demanding an absolute path, and it ships the
non-interactive `work repository` command surface for inspecting and editing that
policy and the repository search roots. The interactive `repository` hub of
ADR-0019 is **deferred** past F3 (see the architectural-authority gates); the
`work repository` parent command prints grouped help and exits 0.

The groundwork already exists. F1 deliberately pre-built the whole location
substrate and left it dormant:

- `config.Config` already carries `repository_roots` and
  `repository_resolution.locators`; `config.Load/Save` already round-trip them.
- `bootstrap` already extracts, registers, and **adds the seed
  `filesystem-repository-locator` to the policy** (`bootstrap.LocatorPolicyEntry`).
- The seed Locator binary (`seed/locator`) is built, embedded, and
  contract-tested (`tests/contract/locator_test.go`,
  `specs/001-first-local-work/contracts/ipc-repository-locator.md`).
- `ipc.RepositoryReference` already has `Path` / `GitFetchURLs` / `Name` / `Query`;
  `ipc.InvokeLocator` and `ipc.LocatorInput/LocatorResponse` already exist.
- `reporef.ValidatePath` is the core's path→usable-repo authority and is reused
  verbatim to validate Locator candidates.

So F3 is **wiring plus one new command surface**, not new infrastructure. Six
things change:

- **A resolution engine (`internal/locator`).** Given a transient Repository
  Reference with no `path`, it walks the policy as a chain of responsibility:
  eligibility by `accepts` ∩ present-fields, projection of only accepted+present
  fields plus the configured roots, `ipc.InvokeLocator`, then core-side candidate
  validation (`reporef.ValidatePath`) and deduplication. First Locator to return
  candidates ends the chain; `matches:[]` advances; an operational failure halts
  (ADR-0015, ADD §7.1). It returns one of two success shapes — **resolved(one
  path)** or **ambiguous(candidates)** — or one of five typed failures.

- **`work start` learns to locate.** The single source step now classifies the
  Starter's reference: a `path` still goes straight to `reporef.ValidatePath`
  (unchanged F1 behaviour); a name/url/query reference goes through
  `internal/locator`. A single resolved repository continues the identical F1
  creation journey. Multiple valid repositories insert one `present.Select` step
  (interactive) or fail with `repository-ambiguous` (non-interactive / explicit
  argv). No hidden heuristic, no ranking.

- **The seed Starter classifies its argument.** `local-path-starter` currently
  always emits `repository.path`. It now emits `path` only when the argument
  looks like a filesystem path and otherwise emits `repository.name`, so a bare
  token reaches the Locator chain (FR-032).

- **The `work repository` surface.** A new command tree under `Administration`:
  `work repository locator list`, `work repository policy list|add|remove|move|replace`,
  `work repository root list|add|remove|replace`, all with the F2.5 non-interactive
  contract (reads take `--json` and are pure; mutations have stable output and
  exit codes). `work repository` with no subcommand is an ordinary Cobra parent —
  it prints grouped help and exits 0 in every stream configuration. The
  interactive `repository` hub of ADR-0019 (and its equivalent-command echo) is
  **deferred**; a later slice ships the full-screen hub.

- **First-run setup at `work start`.** On an interactive `work start`, before any
  other question, the wizard ensures a workspace root and at least one repository
  search root are configured: it prompts for each with a stated purpose,
  validates, and persists them before resolution. Once both are set it never asks
  again (unchanged F1 behaviour for the workspace step on later runs). A search
  root may not overlap the workspace root in either direction — rejected at setup
  and at every `work repository root` mutation. Non-interactive mode runs no
  setup: `work start <path>` still needs no root; `work start <name>` with no
  roots fails `no-repository-found` (26) with guidance.

- **Five typed outcomes.** `internal/diag` gains `no-repository-found` (26),
  `no-eligible-locator` (27), `repository-candidate-invalid` (28),
  `locator-failed` (29), and `repository-ambiguous` (30) so "add a root", "that
  folder is broken", "your Locator errored", and "be more specific" never blur
  together (FR-019, spec US4).

Technical approach: no new dependency, no new binary beyond what the seed already
ships, **no persisted-schema change** (`work-state.json` stays schema 2, `work.db`
stays `user_version = 2`, `work.starter` still records `local-path-starter`).
`config/work.json` gains no new *keys* — F1 already declared them — only the
`work repository` commands and F3 wiring start writing them past bootstrap. The
F2.5 `internal/present` API (`Wizard`, `InputStep`, `SelectStep`, `StepResolved`,
`Fatal`) is a hard prerequisite and is consumed as-is.

## Technical Context

**Language/Version**: Go 1.26 (toolchain go1.26.x). Single module
`github.com/gustaborges/work`. No new language features.

**Primary Dependencies** (all already vendored by F1/F2/F2.5 — F3 adds none):
- `github.com/spf13/cobra` — the `work repository` command tree; `policy`/`root`
  parents get `GroupID` under the existing `admin` group (`internal/cli/help.go`).
- `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2` — only via `internal/present`;
  F3 writes no new Bubble Tea model, it composes `present.Select` / `present.Wizard`.
- System `git` — the seed Locator shells out to `git remote` / `git remote get-url`
  for `git_fetch_urls` matching (already implemented, ADR-0016); the core adds no
  new git subcommand (candidate validation reuses `reporef`/`gitx`).
- `modernc.org/sqlite` — untouched; F3 issues no new projection query.

**Storage**: **No schema change.**
- `config/work.json`: F3 reads and now also writes `workspace`-sibling keys
  `repository_roots` (`[]string`, absolute) and
  `repository_resolution.locators` (`[]string` of `"<alias>/<component>"`), both
  already present in `config.Config` and defaulted to `[]` since F1. Writes stay
  atomic via `config.Save` (`atomicfile`).
- `state/registry.json`: read-only in F3 — the Locator `accepts`, `role`,
  `display_name`, `description` are already recorded by `bootstrap`.
- `work-state.json` / `work.db`: untouched. A Work created from a located clone is
  byte-for-byte identical to one created from the same path typed directly
  (spec SC-008).

**Testing**:
- `internal/locator` unit tests: eligibility (accepts ∩ fields), projection
  (only accepted+present fields + roots reach `LocatorInput`; never the raw arg,
  base, modes, meta, links — SC-004), chain traversal (`matches:[]` advances;
  first non-empty ends; operational failure halts — no fallback), candidate
  validation + dedup (path variants / symlinks collapse — SC-009), the five
  outcome classifications. Driven by small fake Locator entrypoints (compiled
  test binaries, like `internal/starter/testdata/rogue`).
- `internal/repoconfig` unit tests: `policy add --before/--after`, `move`
  (exactly one of `--before`/`--after`), `remove`, `replace`; unknown-locator
  rejection; `policy list` marks an entry whose component is absent from the
  registry as unavailable rather than dropping it (FR-025); root add/remove/replace
  validation, absolutization, dedup.
- `tests/contract/`: extend `locator_test.go` (the seed binary contract is
  already green) with a **core-side resolution** contract asserting the
  projection payload and the five outcomes against fake Locators.
- `tests/integration/` `testscript`: `work repository locator list [--json]`,
  `policy list [--json]`, `root list [--json]` output shapes and purity;
  `policy add/remove/move/replace` and `root add/remove/replace` mutation output +
  exit codes; `root add`/`replace` rejects a path overlapping the workspace (and
  the workspace-set path rejects overlap with a root); installing a second
  (fixture) Locator does **not** change the policy (SC-005); `work repository`
  with no subcommand prints grouped help and exits 0 in every stream
  configuration; `NO_COLOR`/`TERM=dumb`/piped output carries no ANSI (F2.5 parity).
- `tests/integration/` PTY (`//go:build unix`, existing `console` harness):
  `work start <name>` with one match resolves silently and completes the wizard;
  with two matches the `present.Select` repository step appears, identifies each
  clone, and the chosen one is materialised; on a fresh install the first two
  wizard prompts are the workspace root and a repository search root (each with a
  purpose line), and neither is asked when both are already configured.
- `tests/integration/` non-interactive: `work start <name>` single match proceeds;
  two matches → `repository-ambiguous` exit 30, no Work; `no-repository-found`
  exit 26; a failing fixture Locator → `locator-failed` exit 29.
- Regression: F2 (S1–S13) and F2.5 (Q1–Q12) stay green unchanged; F1 (S1–S12)
  stays green with only the interactive fresh-install scenarios adding the two
  up-front setup answers to their pty scripts — no stdout/exit-code/snapshot
  change (spec SC-011, SC-013). Terminal sizes 40×10, 80×24, 160×50 for the new
  picker and the first-run prompts.
- CI matrix unchanged: `ubuntu-latest`, `macos-latest`, `windows-latest`; PTY
  tests stay `unix`-tagged; Windows keeps non-interactive + manual smoke.

**Target Platform**: Same as F1/F2/F2.5 — Linux (amd64, arm64), macOS (arm64,
amd64), Windows (amd64). No change to the supported terminal / shell matrix.

**Project Type**: Single-project CLI tool (unchanged). Two new `internal/`
packages (`locator`, `repoconfig`), one new `internal/cli` command family
(`repository*`), and edits to `internal/cli/start.go`, `internal/starter`,
`internal/diag`, `seed/starter`.

**Performance Goals**: Not latency-critical. SC-012 sets the one budget: resolving
a single-match name over a search tree of ≥500 repositories completes within 2 s
on the reference machine. The seed Locator walks each root breadth-limited to
depth 6 and stops descending at a `.git`; the core then validates only the
returned candidates. No caching or indexing (spec Out of Scope).

**Constraints**:
- `path` in a reference short-circuits to direct validation; other fields are
  never a fallback for an invalid path (FR-002, ADR-0014).
- A Locator receives only its declared-and-present `accepts` fields plus
  `repository_roots` — nothing else of core state (FR-007, FR-008, SC-004).
- Eligibility uses only the static `accepts` list and the reference fields; an
  ineligible Locator is never executed (FR-006).
- Chain of responsibility: `matches:[]` advances; the first Locator to return
  candidates ends traversal; operational failure halts with a diagnostic and no
  silent fallback (FR-009–FR-015, ADR-0015).
- No score / rank / confidence / priority / install-order precedence / cross-Locator
  aggregation anywhere (FR-014, NFR-9).
- The core is the final authority on candidate validity; candidates are
  deduplicated to one entry per repository (FR-016, FR-017).
- Installing or enabling a plugin never edits the policy (FR-027, ADR-0015).
- `policy list` shows entries that cannot currently run, marked unavailable
  (FR-025).
- Resolution and Locators never treat `workspace/in-progress` or
  `workspace/archived` as a catalog — they only ever see `repository_roots`
  (FR-021, ADD §7.2).
- Non-interactive ambiguity fails (`repository-ambiguous`, exit 30) and opens no
  selector (FR-033).
- Reads under `work repository` take `--json` and mutate nothing; mutations have
  stable output and exit codes; no interactive session is required for automation
  (FR-034, F2.5 parity). `work repository` with no subcommand prints grouped help
  and exits 0 (the ADR-0019 hub is deferred past F3).
- Interactive `work start` runs first-run setup (workspace root + ≥1 search root,
  each with a purpose line) before any other step, and only while either is unset;
  non-interactive mode runs no setup and keeps `work start <path>` root-free
  (FR-022a). A search root and the workspace root may not be nested in each other
  in either direction — rejected at setup and at every `work repository root`
  mutation (FR-020).
- Every F1/F2/F2.5 command grammar, transaction guarantee, stable stdout line,
  error token, and exit code is preserved; `work start <path>` is unchanged
  (FR-036, SC-011).
- A resolution ending in failure or unresolved ambiguity leaves zero branch,
  worktree, Work directory, snapshot, or index entry (FR-037, SC-010).

**Scale/Scope**: Single user, single machine. Code surface: `internal/locator`
(≈4 files), `internal/repoconfig` (≈3 files), `internal/cli/repository*.go`
(≈4 files), edits to `start.go` / `starter.go` / `diag.go` /
`seed/starter/main.go`. No `seed/manifest/plugin.json` change (the Locator entry
is already correct), no new flag on `work start`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unratified template (placeholder
principles, no ratified version), so there are no project-specific constitutional
gates. As in F1/F2/F2.5, this plan is held to the **cross-cutting gates in
`docs/roadmap.md` §4** and the spec's **Success Criteria**, treated as binding.

| Gate (roadmap §4) | How this plan satisfies it | Verified by |
|---|---|---|
| Determinism | Policy order is the only precedence; eligibility is a pure function of static `accepts` and reference fields; traversal is a fixed chain of responsibility with no scoring or aggregation; `matches:[]` advances, first non-empty ends, operational failure halts. No install-order or manifest-declared priority anywhere. | `internal/locator` unit tests; `tests/contract` core-side resolution; SC-004, SC-005 |
| Integrity | Resolution is read-only until the existing F1 `create.Run` commit. Every failure/ambiguity path returns before `create.Run`, leaving no branch/worktree/dir/snapshot/row (F1's transaction contract is unchanged and its rollback tests still guard it). First-run setup persists only `workspace` + `repository_roots` via atomic `config.Save` (the same pre-`create.Run` config write F1 already does for the workspace root) — cancelling before it completes writes nothing. `work repository` mutations are single atomic `config.Save` writes. | F1 rollback tests (unchanged); new non-interactive outcome + first-run cancel tests; SC-010 |
| Process contract | stdout for `work start` success is byte-identical to F1; `error: <token>: <message>` unchanged; five new categories appended to `diag.All` with fixed codes 26–30 (table test extended). `work repository` reads honour `--json`; mutations print a stable one-line result. | `diag` table test; `testscript` suite; `start` golden stdout; SC-008, SC-011 |
| Portability | No new subprocess from the core; the seed Locator's `git remote` usage already ships and is contract-tested cross-platform. Path handling reuses `reporef`/`workspace` canonicalisation (symlink-, 8.3-, `/var`→`/private/var`-safe). PTY tests `unix`-tagged; Windows keeps non-interactive coverage + manual smoke. Same 3-OS CI. | CI matrix; contract tests; SC-009 |
| Auditability | Each resolution failure names what failed in user vocabulary with a next action (`diag.Error.Summary`/`Hint`); the Locator's stderr is retained as the internal cause only (never printed in normal output). Diagnostics quote the reference the user gave and candidate paths, never remote contents or secrets (reuses the `reporef` rule). `policy list` surfaces unavailable entries instead of hiding drift. | `diag` renderer tests; FR-015/FR-019/FR-025; SC-003 |
| UX | `work repository` is discoverable from `work --help` → `Administration`; with no subcommand it prints grouped help and exits 0 (ADR-0019 hub deferred, FR-029); reads take `--json`, never open a TUI. First-run `work start` asks for the workspace root and a search root up front, each with a purpose line, and never re-asks once set (FR-022a). The `work start` picker uses the F2.5 bounded, keyboard-navigable, receipt-collapsing `present.Select`. Non-interactive ambiguity is actionable, not a hang. | help-inventory test; PTY picker + first-run geometry tests; `testscript`; SC-002, SC-006, SC-013 |
| Regression | F2 S1–S13, F2.5 Q1–Q12 and all non-interactive F1 demos stay green unchanged; `work start <path>` keeps its exact contract (stdout, exit codes, transaction) and is the compatibility baseline. Interactive F1 scenarios that start from a *fresh* install now answer two extra up-front prompts (workspace root — which F1 already asked, just later — and one search root); their pty input scripts gain those answers, with no change to stdout, exit codes, or the resulting snapshot. F1 non-interactive contract tests configure no search root and are untouched (FR-022a). | CI; SC-011, SC-013 |

**Architectural-authority gates (PRD → ADR → ADD):**

- **PRD**: FR-39/40/42/43/44/45/46/49 and NFR-8/NFR-9 are the product authority
  the spec cites; F3 delivers them fully except the parts the roadmap defers to
  F7 (plugin enable/disable and uninstall dependency handling for policy
  references). ✅
- **ADR-0014** (accepted): origin resolution is separate from clone location; a
  `path` reference is validated directly, Locators otherwise. `internal/locator`
  is that phase; the seed Starter stays a Starter and never locates. ✅
- **ADR-0015** (accepted): one global ordered declarative policy, chain of
  responsibility, no aggregation, no `continue_on_locator_error`, install/enable
  does not modify it, removal ≠ disable. `internal/repoconfig` + the
  `work repository` grammar (`policy list|add|remove|move|replace`,
  `locator list`, `root list|add|remove|replace`) implement exactly this. ✅
- **ADR-0016** (accepted): the transient Repository Reference with independent
  optional `path` / `git_fetch_urls` / `name` / `query`; `git_fetch_urls` is not
  canonical identity; Locators compare across all remotes with no normalisation.
  Realised by `ipc.RepositoryReference` (already correct) + the eligibility and
  projection rules. ✅
- **ADD §7.1–§7.2** (already written): the resolution pipeline, the projected
  `LocatorInput`, the outcome table, the policy/roots config shape, and the
  `work repository` command semantics. This plan's `internal/locator` and
  `internal/repoconfig` match them; no ADD edit is required. ✅
- **ADR-0019** (accepted): the static home, grouped help, and the direct
  `work repository` grammar are delivered verbatim. ADR-0019 also lists an
  interactive `repository` hub among the preserved human surface; **F3 defers
  that hub** and ships `work repository` as a help-printing parent instead. This
  is a scoped deferral, not a reversal — ADR-0019's text is left intact, the
  direct grammar it fixes is fully implemented, and a later slice delivers the
  full-screen hub (tracked here and in `spec.md` Out of Scope). FR-029 is
  restated as the help-on-no-subcommand behaviour. ⚠️ tracked deferral.
- **Historical contracts**: `specs/001-first-local-work/contracts/cli-work-start.md`
  says "F1: a direct path only … no name/URL/root lookup fallback". F3's
  `contracts/cli-work-start.md` **supersedes that clause only**; every other part
  of the F1 `work start` contract (preconditions, transaction, stdout, exit
  codes 0/2/10–17/20) stays authoritative. `ipc-repository-locator.md` is
  promoted from "informational, never executed" to the live F3 contract. ✅
- **⚠️ tracked, not a violation**: the standing F2 note that `work archive` also
  writes `WORK_CD_FILE` (ADR-0018 amendment) is untouched by F3.

**Gate result: PASS.** No violations requiring Complexity Tracking.

### Post-design re-check (after Phase 1)

Re-evaluated after `research.md`, `data-model.md`, `contracts/`, and
`quickstart.md`. Still **PASS**:

- No new dependency, no new binary, no new persisted key, no schema touch; the
  `diag` category table grows by five fixed codes and nothing else moves.
- `contracts/repository-reference.md` and `contracts/repository-locator.md` make
  the projection ban and the five outcomes mechanical assertions, not review
  conventions.
- `contracts/cli-work-start.md` changes the SOURCE clause (may be a
  name/reference) and adds first-run setup (workspace root + search root, up
  front, first run only) plus the workspace/root overlap rejection; the
  transaction, stdout, and pre-F3 exit codes are quoted forward unchanged, and
  the wizard on an already-configured install is unchanged.
- `contracts/cli-work-repository.md` and `contracts/resolution-policy.md` pin the
  grammar, `--json` shapes, availability states, and the no-auto-insert rule.
- `quickstart.md` re-runs the entire F1 + F2 + F2.5 validation set as the
  compatibility proof (SC-011) and adds S1–S13 for F3.
- All seven roadmap §4 gates have a concrete contract or test harness (table
  above → `contracts/` and `research.md`).

## Project Structure

### Documentation (this feature)

```text
specs/004-local-clone-locator/
├── plan.md                       # This file
├── spec.md                       # Feature specification
├── research.md                   # Phase 0 output — decisions R1–R21
├── data-model.md                 # Phase 1 output — reference, candidate, outcome, policy, roots (all transient / config)
├── quickstart.md                 # Phase 1 output — validation scenarios S1–S13 + F1/F2/F2.5 regression
├── contracts/                    # Phase 1 output
│   ├── cli-work-start.md         # F3 amendment: SOURCE may be a name/reference; resolution pipeline; exit codes 26–30
│   ├── cli-work-repository.md     # `work repository` grammar, --json shapes, help-on-no-subcommand, exit codes (hub deferred)
│   ├── repository-reference.md   # the transient Reference: fields, who sets them, transience, path-wins, eligibility
│   ├── repository-locator.md     # F3 IPC contract (promotes the F1 informational one): projection, output, the five outcomes, dedup authority
│   └── resolution-policy.md      # config/work.json shape, ordering semantics, availability states, add/remove/move/replace semantics, no-auto-insert
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

Additions and edits to the F1/F2/F2.5 tree:

```text
internal/
├── locator/                      # NEW — the Repository Reference → local repo resolution engine (ADR-0014/0015/0016, ADD §7.1)
│   ├── locator.go                #   Reference type; Resolve(ctx, Deps, Reference) (Outcome, error); eligibility; projection into ipc.LocatorInput
│   ├── chain.go                  #   policy traversal: skip ineligible, matches:[] advances, first non-empty ends, operational failure halts
│   ├── candidate.go              #   Candidate; validate via reporef.ValidatePath; dedup by symlink-resolved abs path
│   ├── outcome.go                #   Outcome (Resolved path | Ambiguous []Candidate); the five failure → diag.Category mappings
│   └── locator_test.go
├── repoconfig/                   # NEW — policy & search-root operations over config.Config (ADR-0015, ADD §7.2)
│   ├── roots.go                  #   list / add / remove / replace; absolutize, validate (dir + readable), reject workspace↔root overlap (both directions), dedup
│   ├── policy.go                 #   list(availability) / add(--before|--after) / remove / move(--before|--after) / replace; ref-exists check against the registry
│   └── repoconfig_test.go        #   (roots.go also exports NeedsSetup + ValidateRoot for the CLI first-run steps — no present import here)
├── starter/
│   └── starter.go                # Reference gains GitFetchURLs/Name/Query; Invoke returns them (no longer requires Path); the "nothing to resolve" guard moves to internal/locator
├── diag/
│   └── diag.go                   # + no-repository-found (26), no-eligible-locator (27), repository-candidate-invalid (28), locator-failed (29), repository-ambiguous (30); All table extended
└── cli/
    ├── start.go                  # first-run setup steps (workspace root + search root, up front, first run only); source step classifies the reference; path → reporef (unchanged); name/url/query → locator.Resolve; ambiguity → present.Select step / repository-ambiguous
    ├── repository.go             # NEW — `work repository` parent (GroupID admin); no Run → Cobra prints grouped help, exit 0 (ADR-0019 hub deferred)
    ├── repository_policy.go      # NEW — `work repository policy list|add|remove|move|replace` (+ locator list)
    ├── repository_root.go        # NEW — `work repository root list|add|remove|replace`
    └── repository_test.go

seed/
└── starter/main.go               # classify arg: filesystem-looking → {"repository":{"path":...}} (F1 behaviour); bare token → {"repository":{"name":...}}

tests/
├── contract/
│   └── locator_resolution_test.go   # NEW — core-side resolution against fake Locator bins: projection payload, five outcomes, dedup
├── integration/
│   ├── start_by_name_test.go        # NEW (pty) — name → single match completes; name → ambiguity picker; chosen repo materialised
│   ├── start_first_run_test.go      # NEW (pty) — fresh install: first two prompts are workspace root + search root (with purpose lines); already-configured install skips both
│   ├── repository_help.txtar        # NEW — `work repository` with no subcommand prints grouped help, exit 0, no ANSI on non-TTY
│   ├── repository_locator_list.txtar # NEW — locator list [--json]
│   ├── repository_policy.txtar       # NEW — policy list/add/remove/move/replace + no-auto-insert
│   ├── repository_root.txtar         # NEW — root list/add/remove/replace + workspace-overlap rejection (both directions)
│   ├── start_by_name_non_interactive.txtar  # NEW — single match proceeds; missing/ambiguous fail with the right token
│   └── resolution_outcomes.txtar     # NEW — no-repository-found / candidate-invalid / locator-failed as distinct exits
└── fixtures/
    └── locators/                     # NEW — tiny fake Locator entrypoints (ok / empty / two-matches / invalid-candidate / boom)
```

**Structure Decision**: Single Go project (unchanged). The new logic is two
role-focused packages. `internal/locator` is the resolution engine and imports
only `ipc`, `registry`, `config`, `reporef`, `gitx`, `diag` — it has no CLI or
`present` dependency and is unit-testable against fake Locator binaries.
`internal/repoconfig` is pure config manipulation over `config.Config` and the
registry, with no I/O beyond `config.Load/Save` and no `present` import (it
exports the root validation and the "needs first-run setup" predicate the CLI
calls). `internal/cli` keeps every journey decision: `start.go` owns the
first-run setup steps, the reference classification, and the ambiguity step;
`repository*.go` own the command grammar (the no-subcommand parent just prints
help). `internal/starter`
and `internal/diag` gain fields/categories but no behavioural change to existing
paths. `seed/starter` gains one classification branch. No other package changes;
`bootstrap`, `create`, `work-state.json`, `work.db`, and the shell-integration
contract are untouched.

## Branching Strategy

F3 follows the same git-flow shape as F1/F2/F2.5: one short-lived
`feature/004-local-clone-locator-p<n>-*` branch per `tasks.md` phase, each cut
from the previous phase's tip, merged forward at the phase **Checkpoint** once
`make lint` + `go test ./...` are green on the CI matrix.

> **Prerequisite**: F2.5 must have merged (the roadmap gate — F3 begins only once
> the F2.5 interactive and non-interactive contracts are in place). The current
> tree already carries `internal/present` with `Wizard`/`SelectStep`; confirm it
> is on the intended base branch before cutting phase 1.

> **Merge cadence is the user's call.** Per the standing project note the F1/F2
> phase branches were in practice **stacked** (phase N+1's PR targets phase N's
> branch, not `develop`). Before starting a phase, `/speckit-implement` MUST
> confirm with the user which branch to base the new phase branch on and which
> branch its PR targets — do not assume `develop`.

| Phase (tasks.md) | Feature branch | Cut from |
|---|---|---|
| 0 — Specs & doc (this spec set; confirm ADR-0014/15/16 + ADD §7 need no edit) | `feature/004-local-clone-locator-specs-00` | F2.5 tip |
| 1 — Foundational: `internal/locator` engine + `internal/repoconfig` + `starter.Reference` extension + `diag` categories 26–30 + contract/unit tests (no CLI wiring) | `feature/004-local-clone-locator-p1-foundational` | phase 0 tip |
| 2 — US1 Start by name 🎯 MVP: seed Starter classification; wire `locator.Resolve` into `work start` for the single-match path; F1 `work start <path>` regression | `feature/004-local-clone-locator-p2-us1-start-by-name` | phase 1 tip |
| 3 — US2 Disambiguation: the `present.Select` repository step; non-interactive / argv `repository-ambiguous`; candidate identification in the picker | `feature/004-local-clone-locator-p3-us2-disambiguate` | phase 2 tip |
| 4 — US3 Search roots: `work repository root list|add|remove|replace`; root validation + workspace↔root overlap rejection; interactive `work start` first-run setup (workspace root + search root, up front) | `feature/004-local-clone-locator-p4-us3-roots` | phase 3 tip |
| 5 — US5 Policy management: `work repository policy list|add|remove|move|replace` + `locator list` + availability model + the `work repository` help-only parent; no-auto-insert test | `feature/004-local-clone-locator-p5-us5-policy` | phase 4 tip |
| 6 — US4 Outcome hardening + compatibility sweep: the five outcomes as distinct diagnostics with Summary/Hint; non-interactive/`NO_COLOR`/`TERM=dumb` sweep; full F1/F2/F2.5 regression; SC-012 perf check | `feature/004-local-clone-locator-p6-polish` | phase 5 tip |

Rules (identical spirit to F1/F2/F2.5):

- **Naming**: `feature/004-local-clone-locator-p<n>-<short>` — the phase
  identifier stays in one hyphen-separated segment under `feature/` (no nested
  path).
- **First action of each phase**: confirm base/target with the user, then
  `git switch <base> && git pull && git switch -c <feature-branch>`.
- **Merge gate**: a phase merges forward only after its **Checkpoint** in
  `tasks.md` is met and `make lint` + `go test ./...` are green on all three
  OSes, with every prior slice's automated demo (F1 S1–S12, F2 S1–S13,
  F2.5 Q1–Q12, and earlier F3 phases) still green.
- **Release**: when phase 6 merges, F3 ships via `release/0.4` cut from `develop`,
  merged to `master` and tagged (standard git-flow release). The release step is
  the hand-off, not a task.

Spec/plan/contract/doc edits stay on the p0 branch; the per-phase feature-branch
rule covers implementation code only.

## Complexity Tracking

No Constitution Check violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
