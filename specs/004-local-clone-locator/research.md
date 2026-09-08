# Phase 0 Research: Find the Local Clone (F3)

**Feature**: `specs/004-local-clone-locator/` · **Plan**: [plan.md](./plan.md) · **Date**: 2026-09-07

This document resolves every unknown in the plan's Technical Context. Each entry is
**Decision / Rationale / Alternatives considered**. The spec carries **no
`NEEDS CLARIFICATION` markers** (checklist iteration 1 passed on 2026-09-07); the
one deliberately deferred boundary (missing search root = absence vs failure) is
resolved in R11.

Governing sources: `docs/prd.md` FR-39/40/42/43/44/45/46/49, NFR-8/NFR-9;
`docs/adr/adr-0014-origin-resolution-local-repository-location.md`;
`docs/adr/adr-0015-ordered-local-repository-locators.md`;
`docs/adr/adr-0016-repository-reference-git-endpoints.md`;
`docs/add/add-0001-work-system-architecture.md` §7.1–§7.2 & §11;
`docs/roadmap.md` §4 + F3; the F1/F2/F2.5 design sets; the current Go
implementation.

---

## R1 — F3 is wiring, not new infrastructure (roadmap F3 "Includes")

**Decision.** Build on the dormant substrate F1 already shipped rather than
introducing new packages for config, bootstrap, or IPC. Concretely reuse:

| Already present | Reused for |
|---|---|
| `config.Config.RepositoryRoots`, `.RepositoryResolution.Locators` + `Load/Save` | the policy & roots store — no new key, no schema bump |
| `bootstrap.LocatorPolicyEntry` + `addLocatorToPolicy` | the seed Locator is already in the default policy at first run |
| `seed/locator` binary + `tests/contract/locator_test.go` + `specs/001-first-local-work/contracts/ipc-repository-locator.md` | the default filesystem Locator — built, embedded, contract-green |
| `ipc.RepositoryReference{Path,GitFetchURLs,Name,Query}`, `ipc.LocatorInput/LocatorResponse/Match`, `ipc.InvokeLocator` | the core↔Locator wire protocol |
| `reporef.ValidatePath` | candidate validation (dir + readable + non-bare git + ≥1 commit) |
| `registry.Component{Role,Accepts,DisplayName,Description}` + `ByRole` | Locator lookup and `accepts` for eligibility |
| `present.Wizard / SelectStep / InputStep / StepResolved / Fatal` (F2.5) | the ambiguity picker and the first-run setup prompts (workspace root + search root) at `work start` |

**Rationale.** The F1 plan explicitly pre-built and contract-tested this path
"so F3 activates it with no new bootstrap work" (F1 `ipc-repository-locator.md`
scope note). Re-deriving any of it would risk diverging from a tested contract.

**Alternatives considered.**
- *A fresh `internal/resolution` owning its own config file* — rejected: splits the
  single human-editable `work.json` (ADD §3, config package doc) and duplicates
  atomic-write handling.
- *Teach the seed Starter to locate* — rejected by ADR-0014 (couples origin
  interpretation to local clone organisation; defeats N+M composition).

---

## R2 — `internal/locator` package shape

**Decision.** One new package, no CLI or `present` import:

```go
package locator

// Reference is the transient object a Starter produced (ADR-0016). Mirror of the
// consumed subset of ipc.RepositoryReference; never persisted.
type Reference struct {
    Path         string
    GitFetchURLs []string
    Name         string
    Query        string
}

// Deps is everything Resolve needs, injected for testability.
type Deps struct {
    PluginsDir string
    Policy     []string           // config.RepositoryResolution.Locators, in order
    Roots      []string           // config.RepositoryRoots, absolute
    Registry   *registry.Registry // for accepts / entrypoint / availability
}

type Outcome struct {
    Resolved   string      // set on the single-match success
    Candidates []Candidate // len >= 2 on the ambiguous success; empty otherwise
}

func Resolve(ctx context.Context, d Deps, ref Reference) (Outcome, error)
```

`Resolve` is the sole entry point. `ref.Path != ""` never reaches `locator` — the
CLI calls `reporef.ValidatePath` directly (ADR-0014, ADD §7.1). Failures are
`*diag.Error` with the R5 categories.

**Rationale.** A struct of dependencies keeps `Resolve` a pure function of its
inputs and lets unit tests point `PluginsDir` at a fixture tree of fake Locator
binaries. Matches how `starter.Invoke` / `create.Run` already take explicit deps.

**Alternatives considered.**
- *Method on a `Resolver` struct constructed from `workhome.Home`* — rejected:
  pulls `workhome` and config-loading into the engine; the CLI already has the
  loaded `config.Config`.
- *Return a typed enum instead of `Outcome{Resolved, Candidates}`* — equivalent;
  the two-field struct reads better at the call site (`if o.Resolved != ""`).

---

## R3 — Eligibility: `accepts` ∩ present reference fields (ADR-0016, ADD §7.1)

**Decision.** A policy entry participates iff **all** of:

1. its `"<alias>/<name>"` resolves to a `registry` component with
   `role == "repository-locator"` (availability — R12);
2. at least one of `git_fetch_urls`, `name`, `query` is both in the component's
   `accepts` list **and** non-empty on the `Reference`.

`path` is never in `accepts` (enforced by `plugin.Parse`, `acceptsVocab`) and
never makes a Locator eligible. Eligibility is computed from static data only —
no subprocess is started to ask (FR-006, FR-044).

**Rationale.** Verbatim ADD §7.1 ("A Locator participates only if it still
exists, its plugin is enabled, and at least one field from `accepts` is present
in the reference"). Plugin enable/disable is F7, so "enabled" collapses to
"registered" in F3 (R12).

**Alternatives considered.**
- *Run every policy Locator and let it self-filter* — rejected by ADR-0015
  (increases cost, makes the result sensitive to installed plugins) and FR-006.

---

## R4 — Projection: only accepted-and-present fields + roots (FR-007, FR-008)

**Decision.** For each eligible Locator, build `ipc.LocatorInput` containing:

- `repository`: exactly the intersection from R3 — an accepted-and-present field
  is copied; anything else (including accepted-but-absent) is omitted;
- `repository_roots`: the configured absolute roots, unchanged.

Nothing else is projected: not the raw `work start` argument, not `start_modes`,
`base_branch`, `meta`, or `links`. A contract test (`tests/contract`) asserts the
serialised payload for every reference shape (SC-004).

**Rationale.** ADD §7.2 shows exactly this payload. `ipc.LocatorInput` already has
only these two fields, so the guarantee is largely structural; the test pins it
against future drift.

---

## R5 — Five outcomes → five `diag` categories (FR-019, spec US4)

**Decision.** Append to `diag` (codes continue after F2's 25):

| Token | Code | Meaning | In `work start`, interactive |
|---|---|---|---|
| `no-repository-found` | 26 | policy fully traversed, every eligible Locator returned `matches:[]` | **recoverable in-frame** — re-enter a different reference |
| `no-eligible-locator` | 27 | resolution cannot start: the policy is empty, or no policy Locator `accepts` any field the reference carries | terminal (configuration) |
| `repository-candidate-invalid` | 28 | the Locator that ended traversal returned candidates but `reporef.ValidatePath` rejected every one | terminal |
| `locator-failed` | 29 | a Locator exited non-zero, emitted unparseable stdout, or a projected root was unreadable by it | terminal |
| `repository-ambiguous` | 30 | ≥2 valid candidates and the run cannot prompt (non-interactive, or an explicit argv SOURCE non-interactively) | n/a — interactive resolves via the picker |

`no-repository-found` is the only one the interactive source step can recover
from by re-prompting, because the others are fixed by editing configuration or
the environment, not by retyping the name. The others return
`present.Fatal(err)` from the resolution step. All five carry `Summary`/`Hint`
for the F2.5 interactive diagnostic renderer.

**Rationale.** Spec US4 requires three *distinguishable* outcomes (no result /
invalid / failure) and the edge-case list adds empty-policy, no-accepted-field,
and non-interactive ambiguity. Five tokens keep "add a root" (`26`), "policy not
set up" (`27`), "that folder is broken" (`28`), "your Locator errored" (`29`),
and "be more specific" (`30`) individually greppable. `diag.All` and its table
test are extended.

**Alternatives considered.**
- *Fold `no-eligible-locator` into `usage` (2)* — rejected: it is a resolution
  outcome the user hits mid-journey, not a CLI misuse; a distinct token lets a
  script tell it apart from a bad flag.
- *Fold `repository-ambiguous` into `usage`* — rejected for the same reason and
  because FR-033 frames it as "fail with actionable output".
- *One `resolution-failed` category with a sub-reason string* — rejected: breaks
  the "scripts branch on the exit code" contract (`diag` package doc).

---

## R6 — Chain-of-responsibility semantics (ADR-0015, ADD §7.1)

**Decision.** Traverse `Deps.Policy` in order. For each entry:

- **not available** (R12) → skip, continue;
- **ineligible** (R3) → skip, continue;
- **eligible** → `ipc.InvokeLocator`:
  - transport/exit/parse error → **halt**, `locator-failed` (29);
  - `matches: []` → continue to the next entry;
  - `matches` non-empty → **end traversal** and evaluate this Locator's candidates
    only (validate + dedup, R7): 1 valid → `Resolved`; ≥2 valid → `Ambiguous`;
    0 valid → `repository-candidate-invalid` (28).

If traversal ends with every entry skipped-or-empty: `no-repository-found` (26),
unless *no* entry was ever eligible and the reason is structural (empty policy /
no accepted field) → `no-eligible-locator` (27).

**Rationale.** This is the F1 `ipc-repository-locator.md` outcome table and ADD
§7.1 ("`matches: []` continues to the next eligible Locator … One valid candidate
completes resolution; multiple valid candidates are shown to the user and stop the
chain. If all candidates from a Locator are invalid, resolution fails"). A Locator
that *returns results* is terminal for the chain — later Locators are never
consulted once one produces candidates (FR-011).

**Alternatives considered.**
- *Accumulate valid candidates across Locators* — rejected by ADR-0015 (no
  aggregation, no arbitration between Locators).
- *`continue_on_locator_error`* — explicitly rejected by ADR-0015 for v1.

---

## R7 — Candidate validation and deduplication (FR-016, FR-017, SC-009)

**Decision.** For each `match.repo_path`:

1. validate with `reporef.ValidatePath` — it already returns the absolute,
   `filepath.EvalSymlinks`-resolved path and rejects non-dir / unreadable /
   non-git / bare / zero-commit;
2. key the dedup set on that resolved path;
3. drop invalid matches silently; if **all** are invalid → `28`.

The user-facing candidate list is the deduplicated set of *resolved* paths.

**Rationale.** `reporef.ValidatePath` is already the core's single path→repo
authority (its package doc says so) and its symlink resolution gives a natural
canonical dedup key, so two matches that are the same repo via a symlinked
ancestor or a `./x` vs `x` path collapse to one entry (SC-009). ADR-0014: "The
core remains the final authority to validate every path used in the Git
lifecycle."

**Alternatives considered.**
- *Dedup on raw `repo_path` strings* — rejected: misses symlink / relative
  variants the spec edge cases call out.
- *Dedup on `git rev-parse --git-common-dir`* — more correct for worktree edge
  cases but F3 candidates are clones, not worktrees; deferred unless a real case
  appears.

---

## R8 — `starter.Reference` extension; still no `start_modes` (FR-032, F4 boundary)

**Decision.** `starter.Reference` gains `GitFetchURLs []string`, `Name string`,
`Query string`. `starter.Invoke` stops requiring `Path` and returns whatever the
Starter emitted (still reading only the typed fields — FR trust boundary). The old
"the Starter returned no repository path" error is **removed from `starter`** and
its intent moves to `internal/locator`: a reference with neither `path` nor any
locatable field is `no-eligible-locator` (27).

`start_modes`, `base_branch`, `meta`, `links` from the Starter response stay
ignored by the core in F3 exactly as in F1/F2 — they are the F4/F5 payload.

**Rationale.** Minimal surface: the reference fields already exist on
`ipc.RepositoryReference`; `starter.Reference` just stops throwing away three of
them. Keeping `start_modes` out preserves the F1/F2 "absence means new Work"
behaviour and the roadmap slice boundary.

---

## R9 — Ambiguity is a wizard step, not an in-closure sub-prompt (FR-013, FR-033)

**Decision.** Restructure `work start`'s source phase into up to two steps:

- **`source`** (`InputStep`, shown when `SOURCE` is omitted; the closure also runs
  inline for an explicit argv SOURCE and for non-interactive): starter-invoke +
  classify. If `ref.Path != ""` → `reporef.ValidatePath` (unchanged F1 path,
  `invalid-path`/`unusable-repo` recoverable in-frame). Else → `locator.Resolve`:
  - `Resolved` → stash `repoPath`, accept the step;
  - `Ambiguous` → stash the candidates, set `ambiguityPending`, accept the step;
  - `no-repository-found` (26) → return the error (recoverable in-frame);
  - `27`/`28`/`29` → `present.Fatal(err)`.
- **`repository`** (`SelectStep`, shown only when `ambiguityPending`): options are
  the deduped candidates; on accept, `repoPath` is the chosen path. Uses the F2.5
  bounded, filterable, receipt-collapsing selector.

Non-interactive / explicit-argv resolution runs the same closure inline; an
`Ambiguous` outcome there becomes `repository-ambiguous` (30) with a hint to pass
a more specific reference or narrow the roots.

**Rationale.** `present` validation closures cannot spawn a nested selector, and
the F2.5 lifecycle is one bounded step at a time. A follow-on conditional step is
exactly how `start.go` already handles `base` (`SelectStep` returning
`StepResolved` / `Fatal` from its build closure), so this reuses a proven shape.
`no-repository-found` stays in-frame recoverable to match F1's invalid-path
ergonomics; the config-shaped failures are fatal because retyping won't help.

**Alternatives considered.**
- *Resolve fully inside the `source` closure and print candidates as text, asking
  the user to re-run with `--repo <path>`* — rejected: adds a flag, worse UX,
  and still needs the non-interactive story.
- *A dedicated `present.Resolve` primitive* — rejected: over-engineered; the
  `SelectStep` + `StepResolved` combo already covers it.

---

## R10 — `work repository` command grammar (ADR-0019 direct API, ADD §7.2)

**Decision.** Implement exactly the surface ADD §12.5 / ADR-0019 already fix:

```
work repository                                    # prints grouped help, exit 0 (ADR-0019 hub deferred past F3 — see R21)
work repository locator list                        [--json]
work repository policy list                         [--json]
work repository policy add <LOCATOR> [--before <LOCATOR> | --after <LOCATOR>]
work repository policy remove <LOCATOR...>
work repository policy move <LOCATOR> (--before <LOCATOR> | --after <LOCATOR>)
work repository policy replace <LOCATOR...>
work repository root list                           [--json]
work repository root add <PATH...>
work repository root remove <PATH...>
work repository root replace <PATH...>
```

`<LOCATOR>` is a `"<alias>/<component>"` qualified reference (the same string
stored in `repository_resolution.locators`); a bare `<component>` is accepted when
it is unambiguous across installed Locators and echoed back qualified. Parent
commands `repository`, `repository policy`, `repository root` carry `GroupID =
groupAdmin`. `add` takes an optional `--before`/`--after`; `move` requires exactly
one; passing both, or neither to `move`, is `usage` (2). Reads honour the existing
hidden persistent `--json` flag and reject nothing; mutations reject `--json`
(consistent with `work start`/`resume`).

**Rationale.** ADR-0019 and ADD §7.2/§12.5 already spell the grammar out
verbatim; F3 has no latitude to invent verbs. "Verb after a singular resource
path" matches `plugin list|install|...` and `convention show|set`.

The one deviation: ADR-0019 also preserves an interactive `repository` hub for
the no-subcommand form. F3 **defers** that hub (R21) and makes `work repository`
a Cobra parent with no `Run`, so it prints grouped help and exits 0. This is a
scoped deferral confirmed with the product owner — ADR-0019's text is untouched,
the direct grammar is fully delivered, and a later slice ships the full-screen
hub.

**Alternatives considered.**
- *Ship a minimal `present.Select` hub now* — rejected by the product owner: for
  a family this shallow (list/add/remove/move/replace over two collections) a
  menu that just re-invokes the direct commands adds a pty test and maintenance
  with no capability gain; the real value is a rich, stay-resident TUI, which is
  its own slice.
- *Amend ADR-0019 to drop the hub entirely* — rejected: the `plugin` and
  `convention` hubs are out of F3's scope; a narrow deferral note keeps the ADR
  coherent until a slice revisits all three.

---

## R11 — Missing / unreadable search root: absence, not failure (spec Assumption)

**Decision.** A configured root that does not exist or is not readable is **not**
a hard resolution failure at the core level. The core passes the configured roots
through unchanged; the **seed Locator already** returns exit-non-zero on an
unreadable root (`os.ReadDir` check in `seed/locator/main.go`), which the core
classifies as `locator-failed` (29) for *that* Locator per R6. A root that is
simply absent yields no matches from that Locator and traversal continues.

`work repository root add` **does** validate at configuration time (R13): you
cannot add a path that isn't a readable directory. So the only way to reach the
unreadable-root case at resolution time is a root deleted or permissions-changed
after it was added — correctly surfaced as a Locator operational failure, not
silently ignored.

**Rationale.** The spec assumption allows either reading; this one keeps the core
dumb (it does not stat roots) and lets each Locator decide, which is where
`accepts` and depth limits already live. It also means the seed Locator's
existing, contract-tested "unreadable root → exit ≠ 0" behaviour is the F3
behaviour with no change.

**Alternatives considered.**
- *Core pre-filters roots to existing-readable ones* — rejected: hides
  configuration drift the user should see, and duplicates a check the Locator
  already does.

---

## R12 — Policy entry availability model (FR-025, FR-027, F7 boundary)

**Decision.** A policy entry `"<alias>/<name>"` is **available** iff the registry
contains a component with that alias+name and `role == "repository-locator"`.
`work repository policy list` renders every entry in order; an unavailable entry
is shown with an `(unavailable)` marker and still occupies its position.
Resolution skips unavailable entries (R6). Installing/enabling a plugin never
appends to `repository_resolution.locators` — only `bootstrap` (seed, once) and
`work repository policy add` write it.

Plugin **disable** (which would make an available Locator temporarily
unavailable while keeping the policy reference) and **uninstall** dependency
handling (removing policy references on confirmation) are **F7** — F3 only needs
the "component absent from the registry ⇒ unavailable" case, which arises today if
a user hand-edits `work.json` to reference a Locator that was never installed.

**Rationale.** ADR-0015: "Removing a Locator from the policy simply stops using
it; it does not create an individual enablement state. Disabling the plugin makes
its Locators unavailable, but preserves their positions." F3 delivers the
inspect/add/remove/move/replace half (roadmap: "first delivery of RF-40 and
RF-45"); F7 delivers the lifecycle half.

**Alternatives considered.**
- *Drop unavailable entries from `policy list`* — rejected by FR-025 (must be
  shown, marked).
- *Auto-add a newly installed Locator when the policy is empty* — rejected by
  ADR-0015 / FR-027 (no silent insertion, ever).

---

## R13 — Search-root validation, absolutization, dedup, workspace isolation (FR-020, FR-022)

**Decision.** `work repository root add <PATH...>` (and `replace`, and the
first-run setup prompt — R21):

- expand a leading `~`, `filepath.Abs`, and store the **plain absolute** form
  (not symlink-resolved — mirrors `workspace.absPath`, so the user recognises it);
- reject a path that is not an existing readable directory (`usage`, 2);
- **reject a path that overlaps the workspace root in either direction** — the
  root is equal to or nested within `config.Workspace`, or `config.Workspace` is
  equal to or nested within the root. Comparison is canonical
  (`workspace.canonical`, `/var`→`/private/var`- and 8.3-safe) on a
  path-segment boundary. The error names both paths and the reason (`usage`, 2);
- dedup against existing roots on the same canonical basis — adding an
  already-configured root is a no-op success.

`remove` matches canonically; removing an absent root is a no-op success.
`replace` validates the whole new set (incl. workspace overlap) and writes it
atomically. The symmetric check also guards a workspace-root change: F1's
`workspace.Validate` gains "not equal to / inside / containing any configured
repository root" alongside its existing repo-enclosure rule.

**Rationale.** The product owner's call: the workspace is Work-managed territory
(`in-progress`, `archived`, and whatever special directories later slices add).
Work cannot promise the safety of user files placed under it, and treating a
clone as workspace-managed — or a Work as a clone — corrupts both the catalog and
materialisation models. F1's FR-006 already required "separation … from
configured source-clone locations"; F3 makes that concrete and bidirectional.
F1's other FR-006 rule is untouched: a workspace root is still **not** rejected
merely because an unrelated Git repository encloses it — only overlap with a
*configured search root* is rejected. Reuses the existing `workspace`
canonicalisation helpers, so no second implementation.

**Alternatives considered.**
- *Allow overlap, rely on FR-021 (only configured roots are projected)* — the
  earlier F3 decision, now **reversed**: it lets a user keep a misconfigured root
  that silently scans Works, and offers no guard as workspace-owned
  subdirectories multiply.
- *Reject only roots inside the workspace, not the reverse* — rejected: a
  workspace inside a search root is worse (materialisation writes into a scanned
  tree); both directions must fail.
- *Store symlink-resolved roots* — rejected: `workspace` stores the plain form;
  matching that keeps `work.json` readable.

---

## R14 — `--json` shapes for the three read commands (FR-034)

**Decision.**

```jsonc
// work repository locator list --json
{ "locators": [
  { "ref": "work-reference/filesystem-repository-locator",
    "display_name": "Filesystem repositories",
    "description": "Finds local Git repositories in configured search roots",
    "accepts": ["name", "git_fetch_urls", "query"],
    "in_policy": true }
] }

// work repository policy list --json
{ "policy": [
  { "ref": "work-reference/filesystem-repository-locator", "position": 1, "available": true }
] }

// work repository root list --json
{ "roots": ["/home/user/src", "/home/user/projects"] }
```

Human (non-`--json`) output is one line per row, stable and greppable, matching
the F2.5 read-command style. All three exit 0 with an empty collection when
nothing is configured (`{"roots":[]}` etc.), never an error.

**Rationale.** Keys mirror the config file and the registry so a script can round
-trip. `in_policy` on `locator list` answers "is this one already used?" without a
second call; `available` on `policy list` is the R12 marker.

---

## R15 — Identifying candidate repositories in the picker (FR-013, spec US2)

**Decision.** In the `present.Select` repository step, each option is:

- **primary**: the resolved absolute repository path (the thing that
  disambiguates two clones of the same project);
- **secondary**: the first remote fetch URL if the repo has one
  (`git -C <path> remote get-url <first>`), else the immediate parent directory
  name — enough to tell "the work copy" from "the review copy" apart.

The receipt collapses to the basename + short parent (`project (~/src/acme)`),
consistent with `reporef.RepositoryReference.Format`.

**Rationale.** Spec acceptance scenario US2-1 requires "enough identifying detail
to tell them apart". The path alone already disambiguates; the remote URL helps
when two roots mirror the same layout. One extra `git` call per candidate, only
in the already-rare ambiguous case, is well within budget.

**Alternatives considered.**
- *Show only the path* — usually fine, but two roots with identical sub-layouts
  (`~/src/acme/api`, `~/mirror/acme/api`) read better with the remote.
- *Show the last-commit date / branch* — more git calls, not more disambiguating.

---

## R16 — No snapshot / projection / provenance change (spec SC-008, FR-035)

**Decision.** A Work created from a located clone records exactly what F1 records:
`work.starter = "local-path-starter"`, `work-state.json` schema 2, a `works` row
in `work.db` (`user_version = 2`). The located path becomes `create.Params.
SourceRepo` just as a typed path does. The Repository Reference is not written
anywhere (FR-005). No new provenance field, no `work.repository_locator` key.

**Rationale.** The reference is transient by ADR-0016; ADD §3 keeps `work` to
core-governed state only. F3's job ends when `repoPath` is known — from there the
journey is byte-identical to F1, which SC-008 asserts with a head-to-head test.

---

## R17 — Testing strategy

**Decision.**
- **`internal/locator` unit tests** drive `Resolve` against a fixture tree of
  compiled fake Locator entrypoints (`tests/fixtures/locators/*`, built like
  `internal/starter/testdata/rogue`): `ok` (one match), `empty`, `two` (two
  matches), `dupe` (same repo twice via a symlink), `invalid` (a match that isn't
  a repo), `boom` (exit 1). Assert eligibility, projection payload, traversal
  order, the five outcomes, and dedup.
- **`tests/contract/locator_resolution_test.go`** pins the serialised
  `LocatorInput` for every reference shape (SC-004) and the outcome table.
- **`internal/repoconfig` unit tests** cover every policy/root operation and the
  availability marker.
- **`testscript`** covers the whole `work repository` surface and non-interactive
  `work start <name>` (single match, ambiguous, not-found).
- **PTY** covers the interactive single-match flow, the ambiguity picker
  (geometry at 40×10 / 80×24 / 160×50), and first-run setup (fresh install shows
  the two up-front prompts with purpose lines; configured install shows neither;
  cancel → exit 20).
- **Regression**: F2 S1–S13, F2.5 Q1–Q12 and non-interactive F1 unchanged
  (SC-011); interactive F1 fresh-install pty scripts gain the two setup answers
  with no stdout/exit/snapshot change; `tests/contract/locator_test.go` (seed
  binary) stays green untouched.

**Rationale.** Mirrors the F1/F2/F2.5 split (contract for wire shapes, unit for
logic, `testscript` for CLI grammar, PTY for interaction) so F3 slots into the
same CI job with no new harness.

---

## R18 — Performance: SC-012 (single match over ≥500 repos < 2 s)

**Decision.** Accept the seed Locator's existing depth-6 breadth walk with no
caching. The walk stops descending at a `.git`, so a root of 500 sibling clones
is 500 `os.Stat` + one `os.ReadDir` per intermediate dir. The core then runs
`reporef.ValidatePath` (a handful of `git` calls) on **only the returned
matches** — for a unique name that is one repo. A benchmark test builds a
500-repo tree and asserts wall time < 2 s on the CI reference runner.

**Rationale.** The expensive part (per-candidate `git` inspection) scales with
*matches*, not *repos*, and a name lookup returns one. The walk itself is
filesystem-bound and well under budget at this scale. Caching/indexing is
explicit spec Out of Scope.

**Alternatives considered.**
- *Parallelise the root walk* — unnecessary at this scale; adds nondeterminism to
  match ordering (which must stay stable for the picker).

---

## R19 — CI / platform posture (unchanged)

**Decision.** Same 3-OS matrix (`ubuntu-latest`, `macos-latest`,
`windows-latest`). PTY tests `//go:build unix`; Windows runs the `testscript` +
unit + contract coverage plus a manual smoke of `work start <name>` and
`work repository`. The seed Locator's `git remote` calls already run on Windows
in the F1 contract test.

**Rationale.** F3 adds no platform-specific syscall; path handling reuses the
cross-platform `reporef`/`workspace` helpers.

---

## R20 — Branching strategy

**Decision.** Stacked per-phase `feature/004-local-clone-locator-p<n>-*` branches
cut from the prior phase tip, as in F1/F2/F2.5, with base/target confirmed with
the user at the start of each phase (the F1 phases stacked in practice rather
than targeting `develop`). Phase table in [plan.md](./plan.md#branching-strategy).

**Rationale.** Matches the standing project note and every prior slice; keeps each
user story an independently reviewable, independently green increment.

---

## R21 — First-run setup at `work start`; `work repository` hub deferred

**Decision (product owner, 2026-09-07).**

1. **The interactive `work start` wizard runs first-run setup before any other
   step.** When `config.Workspace` is unset it asks for the workspace root (the
   F1 FR-006 step, moved to the front); when `config.RepositoryRoots` is empty it
   then asks for one repository search root. Each prompt carries a one-line
   purpose statement. Both are validated (search root: existing readable dir, no
   workspace overlap — R13) and persisted via `config.Save` before resolution.
   Once both are configured neither prompt ever appears again — an
   already-set-up wizard is byte-identical to F1/F2.5. Cancelling either →
   exit 20, nothing persisted, no Work.

2. **Non-interactive `work start` runs no setup.** `work start <path>` still needs
   no search root (F1 unchanged). `work start <name>`/reference with no roots →
   `no-repository-found` (26) with a hint to `work repository root add <dir>` or
   to run interactively. This keeps every F1 non-interactive contract test green
   (SC-011) — they configure no root and are untouched.

3. **`work repository` with no subcommand prints grouped help and exits 0.** The
   interactive `repository` hub of ADR-0019 is deferred past F3 (see R10). A
   later slice ships a full, stay-resident TUI.

**Rationale.** A name cannot resolve without a search root, so the first
`work start <name>` on a fresh install would otherwise always fail once before
the user discovers `work repository root add`. Establishing the root during the
same guided flow that already establishes the workspace root removes that
failure and gives the two location settings a single, explained home. Restricting
it to the interactive wizard avoids turning a search root into a hard
precondition for the path-only, script-driven `work start <path>` journey, which
never touches a Locator (ADR-0014) — that restriction is what keeps SC-011
intact. Deferring the hub is covered in R10.

**Alternatives considered.**
- *Lazily prompt for a root only when a name reference first needs one* —
  rejected by the product owner: the workspace and the root should be asked
  together, up front, as "the two things Work needs to know about your machine".
- *Make a root a hard precondition for every `work start`, interactive or not* —
  rejected: it breaks F1 non-interactive `work start <path>` contract tests
  (SC-011) for no benefit to the path journey.
- *Seed a default search root at bootstrap (e.g. `~/src`)* — rejected: guessing
  the user's clone directory and silently scanning it violates the "no implicit
  catalog" posture (FR-021) and ADR-0015's no-silent-configuration spirit.
