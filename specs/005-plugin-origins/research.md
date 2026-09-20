# Phase 0 Research: New Origins via Plugin (F4)

Authorities: `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md`
§4/§5/§7/§8, ADR-0000, ADR-0002, ADR-0003, ADR-0004, ADR-0006, ADR-0011,
ADR-0012, `specs/005-plugin-origins/spec.md`. Each decision below resolves one
NEEDS-CLARIFICATION-shaped question the spec leaves to planning (per its own
Assumptions section) or one implementation choice the architecture leaves open.

## R1 — Install pipeline is literally shared with bootstrap, not merely similar

**Decision**: Extract the staging/atomic-swap primitives currently private to
`internal/bootstrap` (`stagePrefix`, `backupSuffix`, `moveDestinationAside`,
`restoreDestination`, `recoverInterruptedSwap`, `sweepStaleStaging`, the
temp-dir-then-rename sequence) into `internal/plugininstall`, generalized over
an arbitrary alias instead of hardcoding `bootstrap.Alias`. `bootstrap.install`
becomes a caller of `plugininstall`'s primitives instead of owning a parallel
copy.

**Rationale**: ADR-0002/ADR-0003 both say the reference package installs
through "exactly the same installation mechanism/pipeline" as any third-party
plugin. F3's plan already treats such statements as testable, not aspirational
(see its ADR-0016 gate). Sharing the actual code is the only way "same
pipeline" is a checkable fact rather than a design intention two
implementations could quietly drift apart from.

**Alternatives considered**: Leave `bootstrap.install` as its own copy and
have `plugininstall` duplicate the pattern. Rejected — two independently
evolving atomic-swap implementations is exactly the kind of drift ADR-0002's
own "single source of truth" consequence warns against, and the F1 author
already wrote `bootstrap.go`'s interruption-recovery logic carefully enough
(checkpoints, stale-staging sweep) that re-deriving it is pure risk for zero
benefit.

## R2 — `--link` is a directory symlink, not a new entrypoint-resolution path

**Decision**: `work plugin install <path> --link` materializes
`plugins/<alias>/source` as a directory symlink (a junction on Windows)
pointing at the original path, instead of copying files into it.
`registry.Component.EntrypointPath(pluginsDir)` and every existing call site
(`internal/starter.Invoke`, `internal/locator/chain.go`) need **zero** code
change — they already resolve through `<pluginsDir>/<alias>/source/
<entrypoint>`, and a symlink at that location is transparent to `os.Stat`/
`exec.Command`.

**Rationale**: The alternative — threading a "resolved source directory" concept
through every component-invocation call site — touches `internal/starter` and
`internal/locator`, two packages F3 explicitly kept free of anything but the
Repository Reference pipeline. A symlink satisfies FR-002 ("no copy... for
active development": editing the original files is immediately reflected)
with a one-line addition (`os.Symlink`) at exactly one place: the installer.

**Alternatives considered**: Store a `SourceDir` per package in the registry
and have `EntrypointPath` take it as a parameter. Rejected — it is a wider
blast radius (two more packages' signatures change) for a feature that a
symlink already solves completely, and it would make `--link` behave
differently from every other install kind at the one boundary
(`registry.Component`) that is supposed to be uniform.

**Constraint accepted**: creating a directory symlink on Windows requires
Developer Mode or an elevated process. This is documented as a known
constraint of a development-only flag (Technical Context, Constraints), not a
portability regression — the plain local and remote install paths need no such
privilege on any platform.

## R3 — Remote install pins to the cloned commit SHA; no `--ref` flag in F4

**Decision**: `work plugin install <source>` (no `--link`, source not a local
path) runs `git clone --depth 1 <source> <tmp>` via a new `gitx.Clone`, reads
`git rev-parse HEAD` from the clone as the pinned reference, then stages the
working tree (minus `.git`) through the same `plugininstall` staging/swap
primitives R1 extracts. There is no `--ref`/`--tag`/`--branch` flag in this
slice.

**Rationale**: The spec's Out of Scope section explicitly limits F4's flag
surface to `--link` and `--as`, calling any further flag interaction "a
planning-stage detail, not a product-level scope change" — i.e., deferred, not
silently invented here. Cloning the default branch and recording the resulting
commit satisfies FR-003 ("fixing a specific content reference at install
time") without inventing ungoverned surface.

**Alternatives considered**: Accept `<source>@<ref>` inline syntax. Rejected
for F4 — nothing in the spec's acceptance scenarios, FRs, or the ADD's direct
API list names it, and adding informal syntax outside the documented flag
surface risks a grammar F7 (or a dedicated update slice) would then have to
either formalize or break.

## R4 — Local-vs-remote source classification duplicates the seed's heuristic; no shared package

**Decision**: `work plugin install <source>` classifies `<source>` as a local
path using the same rule the seed Starter already applies to `work start`'s
argument (`seed/starter/main.go`'s `looksLikePath`: separator present, a
`.`/`..`/`~`/drive-letter prefix, or the token names an existing filesystem
entry). The core-side copy lives in `internal/plugininstall`; it is not
extracted into a shared package with the seed.

**Rationale**: The seed Starter is a deliberately self-contained subprocess —
it is meant to model what an arbitrary third-party plugin binary looks like
(ADR-0000/ADR-0003), so it does not import `internal/...` packages even though
Go's build would allow it. Re-implementing ~15 lines of classification twice
is cheaper than creating a shared package two call sites use for a rule simple
enough that drift risk is negligible (both are exercised by contract tests
that pin the exact heuristic).

**Alternatives considered**: A shared `internal/pathlike` package imported by
both. Rejected as premature abstraction for two call sites of a short,
independently-tested rule — exactly the kind of "three similar lines" the
project's own simplicity guidance accepts over an abstraction.

## R5 — Alias identity and collision detection

**Decision**: `registry.Registry` gains `Packages []Package{Alias, Origin,
Reference}` (`Origin` ∈ `local-linked | local-pinned | remote-pinned`;
`Reference` is the absolute source path for local kinds, `<source>@<sha>` for
remote). Default alias = manifest `name`; `--as <alias>` overrides it.
Collision rule: if `Packages` already has an entry for the resulting alias
whose `Reference`'s *origin identity* — the absolute local path for a local
install, the source URL (ignoring the pinned SHA) for a remote one — differs
from the one being installed, installation fails naming the alias and the
conflicting origin, and registers nothing. An identical origin identity under
the same alias is treated as a reinstall: idempotent, re-running the same
staging/registration pipeline.

**Rationale**: This is FR-006 and ADR-0002 read literally — uniqueness is
local to the registry, not global; reinstalling the same source is
idempotent, not a conflict; there is no auto-suffix.

**Amendment (triage of PR #39)**: the alias is also a directory name under
`plugins/`, so it is constrained to `^[A-Za-z0-9][A-Za-z0-9._-]*$` and may not
end in `.old` (the swap backup suffix); anything else is `plugin-invalid`
(FR-004b). The reference package's alias (`work-reference`) is reserved by
name (FR-006): the seed is registered by bootstrap as components only, with no
`Package`, and `work plugin install` deliberately works before bootstrap has
ever run, so no registry state could be relied on to protect it. A same-origin reinstall replaces the alias's components
and previously declared conventions rather than merging into them (FR-006a).

**Alternatives considered**: Key collision purely on `Alias` with no origin
comparison (any second install under an existing alias is a hard conflict,
even the same source). Rejected — it would make `work plugin install <path>`
non-idempotent for a developer re-running it, which ADR-0002 explicitly calls
out as required behaviour ("reinstalling the same source under the same alias
is idempotent").

## R6 — Fallback-Starter uniqueness is an install-time check, not a runtime one

**Decision**: At install time, after manifest validation, if the manifest
declares a component with `role: starter` and an empty `pattern`, and
`registry.StarterFallback()` already returns a component from a **different**
alias, installation fails (`plugin-fallback-conflict`, category 33) before
registering anything. Reinstalling the same alias's own existing fallback (an
idempotent reinstall, R5) is not a conflict with itself.

**Rationale**: FR-011 and the spec's first edge case both frame this as a
property enforced "at install time (or, if installed independently, enabling
the second)". Since `work plugin enable/disable` does not exist in F4 (Out of
Scope, deferred to F7), "enabled" reduces to "registered" for every rule in
this slice — there is no separate runtime state to check.

**Alternatives considered**: Defer the check to `internal/starter.Match` at
`work start` time (silently pick one when two fallbacks are somehow
registered). Rejected — it would let a second fallback install silently
succeed and only surface confusing behaviour later, the opposite of "rejected
rather than silently allowed and later ignored" (FR-011's own wording).

## R7 — Starter matching: pattern evaluation, fallback, and a first-class Ambiguous outcome

**Decision**: `internal/starter` gains `Match(reg *registry.Registry, arg
string) (registry.Component, Outcome, error)`, following the exact shape
`internal/locator.Resolve` already established for repository ambiguity:
every registered `starter` component with a non-empty `Pattern` is compiled
(`regexp.MustCompile`, no caching needed at this scale) and evaluated against
`arg`; zero matches falls back to `registry.StarterFallback()` (error
`starter-not-matched`, category 35, if none is registered); exactly one match
returns it directly; two or more return an `Ambiguous` outcome carrying the
colliding components, for the CLI to render a `present.Select` step
interactively or fail `starter-ambiguous` (category 36) non-interactively —
mirroring `locator.Outcome`'s `Resolved`/`Candidates` split precisely.

**Rationale**: ADR-0004 specifies exactly this two-layer resolution with "no
manual ordering list" and reuse of "the same TUI selection component already
planned for contribution-vs-fork disambiguation" — F3 already built and
proved the identical shape (compute locally, no subprocess, ambiguous-is-not-
an-error, the caller decides interactive-vs-fail) for a structurally identical
problem. Reusing the shape (not the code — different domain types) keeps the
two ambiguity-handling call sites in `start.go` symmetric and easy to read
side by side.

**Alternatives considered**: A "most specific pattern wins" heuristic.
Rejected — ADR-0004 explicitly rejects it (Alternative B) as reintroducing
non-determinism for free-form patterns; this plan does not reopen a closed
ADR decision.

## R8 — Starter response consumption widens to `base_branch` and `start_modes`; `meta`/`links` stay discarded

**Decision**: The type `internal/starter` returns from `Invoke` gains
`BaseBranch string` and `StartModes []string` (both already present, unread,
on `ipc.StarterResponse`). `Meta`/`Links` remain **not** surfaced by this
type in F4.

**Rationale**: FR-023/FR-024 (base branch) and FR-019/FR-021/FR-022 (start
modes) require the core to act on these fields for the first time. `Meta`/
`Links` persistence is explicitly `start:finalized`/Linker/Importer machinery
(ADD §9–§11), which the spec's own Out of Scope section reserves for F5/F6 —
consuming them now would be scope creep the spec explicitly warns against
("Automatic or on-demand context... is unaffected by and not required for
F4").

**Alternatives considered**: Surface the full `ipc.StarterResponse` to
`internal/cli` and let `start.go` decide what to ignore. Rejected — it would
weaken the "only typed reference fields are consumed" trust-boundary comment
already in `starter.go` (FR-018) by making it a convention instead of a type
constraint; keeping `meta`/`links` off the returned type is what makes "F4
does not implement them" mechanically true rather than a promise.

## R9 — Structurally invalid Starter responses: one category, three trigger shapes

**Decision**: `starter-response-invalid` (category 37) covers three shapes,
checked immediately after `Invoke` returns and before any Work
materialization: (a) `start_modes` containing a value outside `{"contribution",
"fork"}`; (b) `start_modes` containing `"contribution"` with no resolved base
branch; (c) — already covered by the existing `unusable-repo`/subprocess-
failure path from F1/F3, listed here only for completeness, not re-implemented.

**Rationale**: The spec's edge cases treat an unrecognized `start_modes` value
and a base-branch-less contribution response as "the same way as a malformed
Starter output" — i.e., one failure shape, not two, and not folded into the
unrelated repository-resolution categories (26–30).

## R10 — `work-state.json` schema 3: conditional `branch_convention`, widened `start_mode`

**Decision**: Bump `work.Schema` to 3. `WorkSection.StartMode` accepts
`{"new","contribution","fork"}`. `Validate` requires `slug` and
`branch_convention` non-empty unless `start_mode == "contribution"`, in which
case both MUST be empty (contribution mode runs neither the slug nor the
convention step — FR-020). `base_branch` stays required in every mode; in
contribution mode it carries the same value as `branch` (the Starter-resolved
branch checked out directly — there is no separate "base" to record, and ADD
§7's PR example reuses one `base_branch` field for both purposes). `Read`
accepts schema 1–3; `Write` always emits 3, upgrading an older document in
place on next write exactly as the 1→2 upgrade already works.

**Rationale**: This is not new design — it is executing what F1's own schema-1
JSON Schema comment already commits to: `start_mode` "F1 only ever writes
'new'. 'contribution'/'fork' arrive with F4", and `branch_convention` "Omitted
only in contribution mode (unreachable in F1), hence required here." F2 set
the precedent for *how* such a widening is expressed: an additive schema bump
plus an `if/then/else` conditional-required rule (there, `archived_at` iff
`status == "archived"`). Reusing that exact shape for `branch_convention` iff
`start_mode != "contribution"` keeps the schema evolution pattern uniform
across slices instead of inventing a new one.

**Alternatives considered**: Keep schema at 2 and just relax `Validate`'s Go
code without bumping the JSON Schema version. Rejected — the persisted
document's *shape* genuinely changes (a field that was always required can now
be legitimately absent), which is precisely the kind of change F2 treated as
schema-worthy; leaving the version number frozen while the accepted shape
moves would make `schema: 2` stop meaning one fixed shape, undermining the
whole point of versioning it.

## R11 — `work.db` needs no migration

**Decision**: `projection.SchemaVersion` stays 2. No new migration step is
added.

**Rationale**: Read directly from `internal/projection/projection.go`'s
migrations table: the `works` table's `branch_convention` column already has
no `NOT NULL` constraint (unlike every sibling column), and `start_mode TEXT
NOT NULL` carries no `CHECK` constraint restricting its values. Both of F4's
schema-3 changes are already representable in the existing DDL. This is a
verified fact about the current tree, not an assumption — confirmed by reading
the live file before writing this plan.

## R12 — Contribution mode's rollback must never delete the checked-out branch

**Decision**: `internal/create.Run` gains `Params.StartMode`. For
`"contribution"`, step 3 calls `repo.WorktreeAddExisting(worktreePath,
p.Branch)` (no new branch created) instead of `WorktreeAdd`, and its
compensator calls only `repo.WorktreeRemove(worktreePath)` — it does **not**
call `repo.BranchDelete(p.Branch)` the way the fork/new path's compensator
does. Fork mode and the absent-`start_modes` path are unchanged: `WorktreeAdd`
plus the existing worktree-then-branch-delete compensator.

**Rationale**: This is the one correctness-sensitive divergence the whole
contribution-mode feature turns on. SC-009 requires that a failed/cancelled
`work start` "leaves zero branches... behind" — but in contribution mode the
branch was never created by this Work; it pre-existed (it is "the already-
resolved branch" from the PR/reference, per US3). Deleting it on rollback
would destroy a contributor's real, pre-existing branch — the opposite of
integrity, not a preservation of it. SC-009 is satisfied here by never
creating what would need to be deleted, not by the fork/new path's delete-on-
rollback rule, which only applies to a branch this Work itself brought into
existence.

**Amendment (triage of PR #39)**: "the already-resolved branch" is not
always a local branch. On a fresh clone the Starter's `base_branch` is often
only a remote-tracking ref (`origin/feature/x`); passing that to `git worktree
add` yields a detached HEAD and persists `origin/feature/x` as the Work's
branch. Contribution mode therefore checks out the *local* branch name
(`git worktree add <dir> <name>` lets git create a local branch tracking the
single matching remote-tracking branch), persists that local name as both
`branch` and `base_branch`, and its compensator deletes the local branch only
when this run created it. The "never delete" rule above is scoped to a branch
that existed locally before the run (spec FR-020, FR-033).

**Alternatives considered**: Reuse `WorktreeAdd` and immediately reset it to
track the existing branch. Rejected — `git worktree add -b <branch>` fails
outright if `<branch>` already exists, which contribution mode's whole premise
requires to be the normal case (FR-020: "MUST NOT require that branch to be
new").

## R13 — Repository identity: a new package layered on new `gitx` primitives

**Decision**: `internal/repoidentity.Identify(repo gitx.Repo) (string, error)`
implements ADR-0011's three ordered layers: (1) the `origin` remote's fetch
URL, if configured; (2) failing that, `git rev-list --max-parents=0 HEAD`'s
result set, sorted lexicographically and joined, if any root commit exists;
(3) failing that (a shallow clone with no remote), the absolute, symlink-
resolved repository path. `gitx` gains `Remotes() ([]string, error)`,
`RemoteURL(name string) (string, bool, error)`, `RootCommits() ([]string,
error)`, and `IsShallow() (bool, error)` to back it — all new, additive
methods on the existing `Repo` type, following the same `run`/`exitOK`
plumbing every other `gitx` method already uses. A new package-level
`gitx.DiscoverRepoRoot(cwd string) (string, error)` (`git rev-parse
--show-toplevel`) resolves "the repository associated with the current
directory" for `work convention show|set|` and the hub, distinct from
`SourceRepoOf` (which resolves a Work's *worktree* to its *source* repo, a
different relationship).

**Rationale**: ADR-0011 specifies the three layers, their order, and the
sorted-composite-key rule for merged unrelated histories precisely; this is a
direct implementation, not a design choice. Reusing `gitx`'s existing
`Repo`/`run` plumbing rather than shelling out ad hoc from a new package keeps
every git invocation in the one package the architecture already designates
for it.

## R14 — Convention memory is new generated state, not folded into the registry or `work.db`

**Decision**: A new file, `~/.work/state/branch_conventions.json`
(`workhome.Home.BranchConventionsFile()`), holds the per-repository-identity
memoized choice: `{"entries":[{"identity":"<key>","convention":"<name>"}]}`.
A new package `internal/repoconv` provides `Load`/`Save`/`Get(identity)
(string, bool)`/`Set(identity, convention)`, atomic-written via the existing
`internal/atomicfile` helper, following `registry.Load`/`Save`'s exact
missing-file-yields-empty pattern.

**Rationale**: This state is neither the component/convention *catalog*
(`registry.json` already models that, and is regenerated from manifests — a
per-repository *choice* has no manifest to regenerate from) nor derivable from
`work.db`/snapshots (a Work's own `work-state.json` records only the
convention *it* used, never the repository's identity key, and a repository
can have a memoized choice before any Work has ever been created against it —
the spec's edge cases explicitly allow a single-enabled-convention repository
to have "no prompt... and remembered without a selection step", i.e. before
confirmation). It needs its own small, generated, never-hand-edited file,
exactly the boundary ADR-0002/ADR-0013 already draw between configuration,
generated state, and canonical snapshots.

**Alternatives considered**: A table in `work.db`. Rejected — `work.db` is
documented as a pure *projection*, rebuildable from snapshots alone
(`internal/reconcile`); a convention-memory table would violate that
invariant, since no snapshot carries a repository identity key to reconcile
it from.

## R15 — `work start`'s convention step generalizes from hardcoded `freeform`

**Decision**: `start.go`'s convention handling changes from "always
`convention.Freeform`" to: build the full catalog via the already-generalized
`convention.Load(reg)` (unchanged); outside contribution mode, compute the
current repository's identity (R13) and look up a memoized choice (R14); if
none and the catalog has exactly one entry, silently adopt and memoize it (no
prompt — mirrors the existing "a convention with a single prefix is not a
choice" rule one level up); if none and the catalog has more than one, insert
a `present.SelectStep` (same shape as the existing prefix step) and memoize
the accepted choice; if already memoized, use it directly with no step.

**Rationale**: This is FR-025/FR-026 read together with the edge cases, and it
reuses the exact "collapse to no-op when there is only one option" pattern
`start.go` already applies to `prefixes` — no new UI idiom, just one more
instance of an idiom that already exists in this file.

## R16 — `work convention` is this codebase's first interactive hub

**Decision**: `work convention` with no subcommand, in an interactive
terminal, opens a small `present.Wizard`: a read-only display of the current
repository's memoized choice (or "not set" if none/only one exists), and a
single step to change it among the enabled catalog; after a change, it prints
the equivalent direct command (`work convention set <name>`) as its receipt,
per ADR-0019's "each TUI makes the equivalent direct command visible for
automation" rule. Non-interactively, the bare command fails the same way
`work start` already does with no argument in non-interactive mode — an
actionable usage error, never a silently-opened hub.

**Rationale**: Unlike `work plugin`/`work repository`, which the spec/roadmap
explicitly scope as help-only parents in this slice (their full hubs are F7
and were already deferred once, for `work repository`, in F3), FR-030 and
ADR-0011 both require the `work convention` hub *in this slice* — the spec's
Out of Scope section names only `work plugin`'s hub as deferred, and does not
mention `work convention`. It is small enough (one repository, one choice) to
build now without pulling forward any of F7's plugin-lifecycle scope.

## R17 — `work plugin` stays a help-only parent, mirroring `work repository`'s F3 precedent exactly

**Decision**: `internal/cli/plugin.go`'s no-subcommand handler is structurally
identical to `internal/cli/repository.go`'s: `Args: cobra.NoArgs`, `--json`
rejected as a usage error (it prints help, not data), `RunE` calls
`cmd.Help()`. Its children are only `install` and `list`.

**Rationale**: The spec's Out of Scope section states this explicitly:
"`work plugin` with no subcommand in this slice prints grouped help... exits
0, opening no interactive menu, mirroring F3's scoped deferral of the `work
repository` hub." Reusing the already-shipped, already-tested pattern instead
of writing a new one is both faster and keeps the two deferred-hub parents
behaviourally identical, which the help-inventory test can then assert once
for both shapes.

## R18 — New diagnostic categories: codes 31–38, continuing directly after F3's 26–30

**Decision**:

| Code | Token | Trigger |
|---|---|---|
| 31 | `plugin-invalid` | Manifest fails role-based field validation (FR-004); nothing registered |
| 32 | `plugin-alias-conflict` | Alias resolves to a different existing origin (FR-006) |
| 33 | `plugin-fallback-conflict` | A second enabled fallback Starter would result (FR-011) |
| 34 | `plugin-install-failed` | Generic install-time I/O/clone/staging failure (mirrors `bootstrap-failed`, scoped to `work plugin install`) |
| 35 | `starter-not-matched` | No pattern match and no registered fallback (FR-013) |
| 36 | `starter-ambiguous` | Non-interactive Starter pattern collision (FR-012/US4 AC4) |
| 37 | `starter-response-invalid` | Unrecognized `start_modes` value, or contribution mode with no resolved base branch (FR-019, edge cases) |
| 38 | `convention-unknown` | `work convention set <name>` names a convention that is not enabled (FR-029) |

**Rationale**: The spec's Assumptions section explicitly defers exact error
tokens/codes to planning, "consistent with the existing error taxonomy." F3
established the pattern of one fixed code per distinguishable,
script-relevant outcome, appended after the previous slice's highest code with
no renumbering. This table is that pattern applied to F4's eight
distinguishable new outcomes — granular enough that a script can branch on
"which specific thing went wrong" without parsing message text, matching every
prior slice's contract.

## R19 — Fixture plugins live under `tests/fixtures/plugins/`, mirroring F3's `tests/fixtures/locators/`

**Decision**: Three tiny fixture packages: `specific-starter` (a distinctive
`pattern`, returns `start_modes: ["contribution","fork"]` and a `base_branch`,
for US2/US3), `colliding-starter` (a pattern overlapping `specific-starter`'s,
for US4), and `invalid-manifest` (a manifest violating one role rule per test
case, for FR-004 — never successfully installed, used only to assert
zero-partial-registration). A fixture package "may declare an `importer` or
`linker` component for manifest-validation coverage" per the spec's own
Assumptions, without F4 executing that role.

**Rationale**: Directly follows the spec's Independent Test descriptions for
US1–US4 and its Assumptions section's explicit allowance; mirrors the already-
proven `tests/fixtures/locators/` shape (tiny compiled Go binaries acting as
fake plugin entrypoints) so the contract-test harness pattern F3 built needs
no reinvention.
