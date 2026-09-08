# Feature Specification: Find the Local Clone

**Feature Branch**: `feature/004-local-clone-locator-specs-00`

**Created**: 2026-09-07

**Status**: Draft

**Input**: User description: "`docs/roadmap.md` — advance to F3 — Find the local clone. Start a Work by name or reference using configured search roots and an explicit Locator policy, instead of requiring the absolute path." Authorities: `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md`, ADR-0014, ADR-0015, ADR-0016.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start a Work Using Only a Repository Name (Priority: P1)

A developer has several clones organized under a couple of directories on their machine. They run `work start <repository-name>` without knowing or typing the clone's absolute path. Work interprets the argument, searches the configured roots with the default discovery strategy, finds exactly one matching clone, and continues into the familiar creation flow — base branch, slug, convention, prefix — ending in a ready worktree.

**Why this priority**: This is the entire point of the slice. Until a Work can start from a name, every `work start` demands the user already know the exact filesystem location of the clone, which is the friction F3 removes. A single-match name is the smallest end-to-end slice that delivers this value.

**Independent Test**: Configure one search root containing a single clone whose name matches the argument, run `work start <name>` with no path, and confirm resolution produces that clone's path and the rest of the F1 creation journey completes unchanged into a materialized Work.

**Acceptance Scenarios**:

1. **Given** one configured search root containing exactly one clone named `payments`, **When** the user runs `work start payments`, **Then** Work resolves the single local repository, does not prompt for a path, and proceeds to base-branch/slug/convention selection.
2. **Given** the resolved repository, **When** the creation journey completes, **Then** the worktree, directory, and `work-state.json` are created exactly as they are for `work start <path>` and the terminal is repositioned into the new worktree.
3. **Given** an argument that already carries a local path, **When** the user runs `work start /abs/path/to/repo`, **Then** Work validates that path directly and runs no Locator, and the downstream journey is identical to the name-based journey.

---

### User Story 2 - Choose the Right Repository When Several Clones Match (Priority: P2)

The same developer keeps two clones of the same project — one for day-to-day work and one dedicated to reviewing forks. Both match the name they typed. Work does not guess: it presents both resolved repositories and asks the developer to pick the intended one, then continues creation with that choice.

**Why this priority**: Ambiguity is common on real machines and silently picking one clone would materialize a Work against the wrong repository. Explicit selection is required for the feature to be trustworthy, but it builds directly on the P1 pipeline.

**Independent Test**: Place two matching clones in the configured roots, run `work start <name>` interactively, confirm both are offered with enough identifying detail to tell them apart, select one, and verify the Work is created against the selected repository and not the other.

**Acceptance Scenarios**:

1. **Given** two configured roots that each contain a clone matching `payments`, **When** the user runs `work start payments`, **Then** Work presents both resolved repositories for explicit selection and does not begin creation until one is chosen.
2. **Given** the selection prompt, **When** the user chooses the second repository, **Then** creation proceeds against that repository's path and the chain stops — no further Locator is consulted.
3. **Given** a non-interactive invocation where more than one repository matches, **When** resolution would need a choice, **Then** the command fails with actionable output explaining how to narrow the reference or adjust roots, and opens no selector.

---

### User Story 3 - Configure Repository Search Roots (Priority: P2)

Before locating anything by name, the developer tells Work where their clones live. On the first interactive `work start`, Work asks — up front, before any other question — for the workspace directory and for at least one repository search root, explaining what each is for; the search roots are kept separate from the workspace where Works and their worktrees are materialized. Afterward the developer can list, add, remove, and replace those roots from the command line, and a script can read the persisted configuration.

**Why this priority**: The default Locator can only find clones inside roots the user has declared, so root configuration is a precondition for P1 in any real setup. Because a name cannot resolve without a root, first-run setup must establish one rather than letting the first `work start` fail.

**Independent Test**: On a fresh install, run `work start <name>` interactively and confirm the wizard's first two prompts are the workspace root and a repository search root, each with an explanation of its purpose; then add a second root via `work repository root add`, confirm `work repository root list` reports both, remove one, replace the set, and confirm each change is reflected in the persisted configuration and in a subsequent `work start <name>` search scope.

**Acceptance Scenarios**:

1. **Given** a fresh install with no workspace and no search roots, **When** the user runs `work start <name>` in an interactive terminal, **Then** before any other question Work asks for the workspace root and for at least one repository search root, each prompt stating what the value is used for, and persists both choices before resolution begins.
2. **Given** an install where the workspace and at least one search root are already configured, **When** the user runs `work start <name>`, **Then** neither is asked again and the journey proceeds directly.
3. **Given** configured roots, **When** the user runs `work repository root add <dir-a> <dir-b>` then `work repository root list --json`, **Then** both directories are persisted as search roots distinct from the workspace root and the list command prints them in a stable machine-readable form and changes no state.
4. **Given** a candidate search root that is equal to or nested within the workspace root (including its in-progress or archived area), or a workspace root equal to or nested within a configured search root, **When** the user tries to configure it (at first-run setup or via `work repository root add`/`replace`), **Then** Work rejects it with a message naming both paths and the reason, and persists nothing.
5. **Given** the default strategy runs, **When** it searches, **Then** it only ever looks inside the configured search roots and never treats Work worktrees or archived Works as a repository catalog.

---

### User Story 4 - Distinguish No Result, Invalid Candidate, and Locator Failure (Priority: P3)

The developer runs `work start <name>` in three broken situations: nothing matches anywhere, something matches but is not actually a usable Git repository, and a configured Locator itself errors while running. Each produces a distinct, understandable outcome — never a silent fallback and never one failure disguised as another.

**Why this priority**: Determinism and auditability are cross-cutting gates for the slice. If these three outcomes blur together, the user cannot tell "I need to add a root" from "that folder is broken" from "my index tool is down". It is P3 because it hardens the P1/P2 pipeline rather than adding a new journey.

**Independent Test**: Drive resolution into each of the three states with fixtures — empty roots, a non-repository directory named like the target, and a Locator stub that exits with an error — and confirm three different messages, no cross-conversion, and no partial Work creation in any case.

**Acceptance Scenarios**:

1. **Given** configured roots with no matching clone, **When** resolution finishes, **Then** Work reports "no repository found" with guidance to check the reference or roots, and no Work is created.
2. **Given** a directory that matches the reference but is not an accessible, usable Git repository, **When** the core validates candidates, **Then** Work reports an invalid-candidate outcome identifying what was rejected, distinct from "no repository found".
3. **Given** a Locator in the policy that fails operationally while resolving, **When** the failure occurs, **Then** resolution halts with a diagnostic attributing the failure to that Locator, and Work does not silently continue to the next Locator.
4. **Given** any of the three outcomes, **When** the command exits, **Then** no branch, worktree, directory, snapshot, or index entry has been created.

---

### User Story 5 - Inspect and Manage the Resolution Policy (Priority: P3)

The developer wants to see and control which discovery mechanisms Work uses and in what order. They list installed Locators, list the effective policy, and add, remove, move, or replace entries with non-interactive commands. Installing a plugin never rewrites this order behind their back.

**Why this priority**: A single default Locator is enough for the P1 demo, but the ordered-policy model is the architectural core of the slice and must be built and exercised now, even before third-party Locators exist. It is P3 because the default policy already works without the user touching it.

**Independent Test**: With the seed Locator present, run `work repository locator list` and `work repository policy list`, then add a second (fixture) Locator to the policy with explicit positioning, move it, remove it, and replace the whole sequence, confirming each command produces the expected persisted, script-readable configuration.

**Acceptance Scenarios**:

1. **Given** a freshly bootstrapped installation, **When** the user runs `work repository policy list`, **Then** it shows an ordered sequence containing the default filesystem Locator.
2. **Given** an installed Locator that is not in the policy, **When** a plugin providing it is installed or enabled, **Then** the effective policy is unchanged until the user adds it explicitly.
3. **Given** a policy with two Locators, **When** the user runs `work repository policy move <locator> --after <other>`, **Then** the persisted order changes accordingly and resolution traverses them in the new order.
4. **Given** a Locator reference in the policy that cannot currently run, **When** the user runs `work repository policy list`, **Then** that reference is still shown and marked as unavailable rather than hidden.
5. **Given** `work repository` invoked with no subcommand, **When** the process runs (in any stream configuration), **Then** it prints the grouped help listing the `locator`, `policy`, and `root` subcommands and exits 0, and opens no interactive menu.

### Edge Cases

- The Repository Reference carries both a path and other identifiers: the path wins, is validated directly, and the other fields are not consulted as fallback.
- The Repository Reference carries no path and no field that any eligible Locator accepts: resolution cannot start and Work reports an actionable error naming what identifiers would be needed.
- The Repository Resolution Policy is empty: there is nothing to traverse; Work reports that no discovery mechanism is configured.
- A Locator returns several candidates that point to the same repository (path variants, symlinks): the user sees that repository at most once.
- A Locator returns a mix of valid and invalid candidates: invalid ones are dropped; if one valid candidate remains, resolution succeeds; if none remains, resolution fails with an invalid-candidate diagnostic.
- Two Locators would each locate a different repository for the same reference: the first Locator to return candidates ends the chain; there is no arbitration between Locators.
- A configured search root does not exist or is not readable: the default strategy surfaces this without aborting the whole resolution as a hard Locator failure, unless the strategy itself defines the missing root as an operational error.
- A matching clone is nested deeper than the default strategy's scan depth: it is not found, and the outcome is "no repository found", not a failure.
- The same name matches a clone in a configured root and a worktree under the workspace: only the configured roots are searched; the workspace is never an implicit catalog.
- A non-interactive `work start <name>` resolves to exactly one repository: it proceeds without any prompt.
- A non-interactive `work start <name>` on an install with no configured search roots: it cannot run first-run setup, so it fails with the no-result outcome and guidance to configure a root — it does not hang or open a prompt. A non-interactive `work start <path>` is unaffected and still needs no root.
- First-run setup is declined or cancelled at the workspace or search-root prompt: nothing is persisted and no Work is created, consistent with cancelling any wizard step.
- The user points a search root at the workspace root, or sets the workspace root inside an existing search root: the configuration is rejected before it is written, whether attempted at first-run setup or through `work repository root add`/`replace`.
- `NO_COLOR`, `TERM=dumb`, or redirected streams during a repository management command: output stays stable and script-readable, consistent with the F2.5 presentation contract.

## Requirements *(mandatory)*

### Functional Requirements

**Repository Reference and resolution pipeline**

- **FR-001**: `work start` MUST accept an argument that is not a local path and, through the matched Starter, obtain a transient Repository Reference that may carry Git fetch endpoints, a name, and/or opaque query text instead of a resolved path.
- **FR-002**: When the Repository Reference carries a local path, Work MUST validate that path directly as an existing, accessible, usable Git repository and MUST NOT execute any Repository Locator; the reference's other fields MUST NOT act as fallback for an invalid path.
- **FR-003**: When the Repository Reference carries no local path, Work MUST resolve it by traversing the global Repository Resolution Policy in its declared order, before any Work materialization.
- **FR-004**: Resolution MUST produce exactly one valid, accessible local Git repository before the creation journey continues; that resolved path MUST feed the identical F1/F2 creation, snapshot, and terminal-repositioning behavior used for `work start <path>`.
- **FR-005**: The Repository Reference MUST remain transient — it MUST NOT be promoted into the `work`, `meta`, or `links` sections of the snapshot.

**Locator eligibility and data projection**

- **FR-006**: Before executing a Locator, Work MUST evaluate its eligibility using only the Locator's statically declared accepted fields and the identifiers present in the Repository Reference; an ineligible Locator MUST be skipped without execution.
- **FR-007**: Work MUST project to an executed Locator only the reference fields that the Locator declares as accepted and that are present, together with the configured repository search roots — and nothing else.
- **FR-008**: A Locator MUST NOT receive the raw `work start` argument, the start mode, the base branch, published metadata, or links.

**Chain-of-responsibility traversal**

- **FR-009**: Work MUST traverse eligible Locators one at a time in policy order.
- **FR-010**: A Locator that returns no candidates MUST allow resolution to continue to the next eligible Locator.
- **FR-011**: A Locator that returns candidates MUST end the traversal; later Locators MUST NOT be consulted once a Locator has produced results.
- **FR-012**: When traversal ends with exactly one valid candidate repository, resolution MUST complete with that repository and MUST NOT prompt.
- **FR-013**: When traversal ends with more than one valid candidate repository, Work MUST present them for explicit user selection and MUST NOT begin creation until one is chosen.
- **FR-014**: Work MUST NOT apply any score, rank, confidence, declared priority, installation-order precedence, or cross-Locator aggregation when resolving.
- **FR-015**: An operational or protocol failure of a Locator MUST halt resolution with a diagnostic attributing the failure to that Locator; Work MUST NOT silently fall back to the next Locator.

**Candidate validation**

- **FR-016**: Work MUST validate every candidate returned by a Locator as an existing, accessible, usable Git repository, and Work — not the Locator — MUST be the final authority on that validity.
- **FR-017**: Work MUST deduplicate candidates that resolve to the same repository so the user never sees the same repository twice.
- **FR-018**: When every candidate returned by the Locator that ended the traversal is invalid, resolution MUST fail with a diagnostic identifying the rejected candidates, distinct from a no-result outcome.
- **FR-019**: Work MUST expose three distinguishable resolution outcomes to the user — no result, invalid candidate(s), and Locator failure — and MUST NOT convert one into another.

**Repository search roots**

- **FR-020**: Repository search roots MUST be configurable independently from the workspace root where Works and their worktrees are materialized. Work MUST reject any configuration in which a search root is equal to or nested within the workspace root, or the workspace root is equal to or nested within a search root — at first-run setup and at every `work repository root` mutation — with a message naming both paths and the reason, persisting nothing.
- **FR-021**: Resolution and Locators MUST NOT treat the workspace's in-progress or archived areas as an implicit repository catalog.
- **FR-022**: Users MUST be able to list, add, remove, and replace repository search roots through non-interactive commands, and the persisted root configuration MUST remain inspectable and usable by automation.
- **FR-022a**: On the first interactive `work start`, before presenting any other choice, Work MUST ensure a workspace root and at least one repository search root are configured — prompting for each with a stated purpose, validating, and persisting them before resolution — and MUST NOT ask again on later runs once both are set. In non-interactive mode Work MUST NOT run this setup; a non-interactive `work start` that needs a search root and has none MUST fail with the no-result outcome and guidance, while `work start <path>` MUST remain usable with no search root configured.

**Repository Resolution Policy**

- **FR-023**: Work MUST maintain a single global, ordered, declarative Repository Resolution Policy whose entries are qualified Locator references.
- **FR-024**: Users MUST be able to list the effective policy, add an entry (optionally positioned relative to another), remove entries, move an entry relative to another, and replace the whole sequence, through non-interactive commands.
- **FR-025**: `work repository policy list` MUST include policy entries that cannot currently run and mark them as unavailable rather than omitting them.
- **FR-026**: Users MUST be able to list all installed Locators and their state via `work repository locator list`.
- **FR-027**: Installing or enabling a plugin that provides Locators MUST NOT insert those Locators into the Repository Resolution Policy.
- **FR-028**: Removing a Locator from the policy MUST leave the Locator installed and enabled, only excluded from the resolution strategy; it MUST NOT create a separate per-Locator enablement state.
- **FR-029**: `work repository` invoked with no subcommand MUST print the grouped help for its subcommands and exit 0, in every stream configuration, and MUST NOT open an interactive menu. (The interactive `repository` hub named in ADR-0019 — and the equivalent-command echo it would carry — is deferred beyond F3; see Out of Scope.)

**Bootstrap and default strategy**

- **FR-030**: On first use, Work MUST make available — with no manual installation and no network access — a filesystem-based Repository Locator and a minimal Repository Resolution Policy that includes it.
- **FR-031**: The default filesystem Locator MUST search only the configured repository search roots, to a bounded depth defined by the Locator, offline, and MUST return all matches without selecting among them.
- **FR-032**: The reference fallback Starter MUST be able to turn a bare name-or-reference argument into a Repository Reference so that such an argument reaches the Locator chain.

**Non-interactive and contract preservation**

- **FR-033**: In non-interactive mode, a resolution that would require a user choice among multiple valid candidates MUST fail with actionable output and MUST NOT open a selector.
- **FR-034**: Read commands under `work repository` (`locator list`, `policy list`, `root list`) MUST accept `--json`, MUST NOT alter state, and MUST NOT require an interactive session for automation.
- **FR-035**: The direct-path journey and the policy-located journey MUST converge on identical Work creation behavior downstream of resolution, with no divergence in snapshot content, index projection, or terminal repositioning.
- **FR-036**: All F1, F2, and F2.5 command grammar, transaction guarantees, stable stdout lines, error tokens, and exit codes MUST be preserved; `work start <path>` behavior MUST NOT change.
- **FR-037**: A resolution that ends in any failure or ambiguity that the user does not resolve MUST leave no branch, worktree, Work directory, snapshot, or index entry behind.

### Key Entities

- **Repository Reference**: A transient, per-invocation description of the target repository produced by a Starter. Independent optional fields: a resolved local path, known Git fetch endpoints, a possibly-ambiguous name, and opaque query text. Not persisted.
- **Repository Locator**: An installed executable component that turns a Repository Reference into zero or more candidate local repositories. Declares which reference fields it accepts. Does not interpret the origin, choose among candidates, set the base branch or mode, publish metadata or links, or create worktrees.
- **Repository Resolution Policy**: The global ordered list of qualified Locator references the user has chosen for this machine. Traversed as a chain of responsibility. Never modified by plugin installation.
- **Repository Search Root**: A configured directory where discovery strategies may look for clones. Distinct from the workspace/Work-materialization root, and required to be non-overlapping with it in both directions (neither may sit inside the other).
- **Candidate Repository**: A local repository path returned by a Locator, pending core validation and deduplication.
- **Resolution Outcome**: The result of traversal — resolved (one repository), ambiguous (user selection required), no result, invalid candidate(s), or Locator failure — each surfaced distinctly.

### Scope and Dependencies

- Builds on delivered F1 (`specs/001-first-local-work/`), F2 (`specs/002-daily-cycle/`), and F2.5 (`specs/003-terminal-ux-revamp/`); the roadmap requires F2.5's interactive and non-interactive contracts to be in place before this slice begins. The new selection prompt uses the F2.5 presentation boundary.
- Product authority is `docs/prd.md` FR-39, FR-40 (first delivery), FR-42, FR-43, FR-44, FR-45 (first delivery), FR-46, FR-49, and NFR-8, NFR-9. ADR-0014 governs the origin/location separation, ADR-0015 the ordered policy, ADR-0016 the Repository Reference and Git-endpoint semantics; `docs/add/add-0001-work-system-architecture.md` Sections 7.1 and 7.2 describe their realization.
- The seed/reference package gains a filesystem Locator and a default policy entry, installed through the existing offline bootstrap pipeline.
- Every earlier slice's automated end-to-end demonstration must remain green.

### Out of Scope

- The interactive `work repository` hub named in ADR-0019 — deferred beyond F3. F3 ships the direct `work repository` grammar and a `work repository` parent that prints grouped help and exits 0; a later slice delivers the full-screen hub. ADR-0019's text is unchanged; this is a scoped deferral, tracked in the plan.
- Non-filesystem Locators (alias tools, corporate indexes, service-backed discovery) — delivered later by plugins.
- Typed provider identifiers, a canonical Git URL identity, SSH/HTTPS equivalence inference, or use of push URLs.
- The full plugin lifecycle for Locators — enable/disable of a plugin's Locators and uninstall-time dependency handling for policy references (F7).
- Caching or persistent indexing of discovery scans.
- Starter-returned `start_modes`, contribution and fork journeys, and new origins via plugin (F4).
- Automatic or on-demand context: link discovery and artifact import (F5, F6).
- Changing the `work start <path>` grammar or any other established command contract.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can start a Work by supplying only a repository name — zero absolute paths typed — and reach a ready worktree whenever exactly one matching clone exists in the configured roots.
- **SC-002**: When more than one clone matches, 100% of interactive resolutions require an explicit user choice and 0% auto-select; the materialized Work always targets the chosen repository.
- **SC-003**: The three failure modes — no result, invalid candidate, Locator failure — produce three distinct messages in 100% of test fixtures, with zero cases of one outcome being reported as another.
- **SC-004**: Across every Repository Reference shape tested, an executed Locator receives only its declared-and-present fields plus the configured roots, with zero leakage of the argument, start mode, base branch, metadata, or links.
- **SC-005**: Installing or enabling a plugin that provides a Locator changes the effective Repository Resolution Policy in 0 cases.
- **SC-006**: Every policy and search-root operation (list, add, remove, move, replace) is achievable through a non-interactive command, and the persisted configuration is re-readable by a script in every case.
- **SC-013**: On a fresh install, an interactive `work start <name>` asks for the workspace root and a repository search root as its first two prompts in 100% of runs, and asks for neither on any run where both are already configured; a configuration that overlaps workspace and search root is rejected before any write in 100% of test fixtures.
- **SC-007**: The default filesystem Locator completes discovery with zero network calls and returns zero results drawn from the workspace's in-progress or archived areas.
- **SC-008**: In head-to-head tests, `work start <path>` and `work start <name>` (single match) produce identical snapshot fields and identical terminal repositioning.
- **SC-009**: When Locators return duplicate or path-variant references to one repository, the user is presented that repository exactly once.
- **SC-010**: A resolution ending in failure or unresolved ambiguity leaves zero branches, worktrees, Work directories, snapshots, or index entries in 100% of tests.
- **SC-011**: All existing F1, F2, and F2.5 non-interactive contract tests remain green with no changes to arguments, flags, stable stdout, error tokens, or exit codes.
- **SC-012**: Resolving a single-match name over a configured search tree of at least 500 repositories completes within 2 seconds on a supported reference machine.

## Assumptions

- F2.5 (Terminal UX Revamp) is delivered; the ambiguity selection prompt is built on its wizard/presentation boundary and inherits its geometry, accessibility, cancellation, and non-interactive rules.
- "Minimum ordered policy" means a single default Locator ships enabled in the policy; the ordering and chain-of-responsibility semantics are fully implemented and tested now even though third-party Locators arrive in later slices.
- The default filesystem Locator owns its own scan-depth default and configuration; the core does not define traversal depth.
- A missing or unreadable configured search root is handled by the default strategy as an absence of matches in that root rather than a hard resolution failure, unless the strategy explicitly classifies it as operational failure; the precise boundary is settled during planning.
- Error tokens and exit codes for the new no-result, invalid-candidate, and Locator-failure outcomes are defined during planning, consistent with the existing error taxonomy and the cross-cutting process contract.
- `git` invoked as a subprocess is the authority for whether a candidate directory is a usable repository; the core does not reimplement repository detection.
- Non-interactive resolution has no automatic disambiguation: the user must narrow the reference or adjust roots and rerun.
- The persisted global configuration file already holds the workspace root (from F1); this slice adds the search-roots list and the resolution-policy sequence to the same file.
- F1's rule that a workspace root is not rejected merely because a Git repository encloses it still holds; F3 adds only the specific mutual-exclusion between the workspace root and the configured repository search roots. The rejection is a configuration-time check reusing F1's path-canonicalisation helpers.
- "Up front" for first-run setup means the workspace and search-root prompts precede the prefix/slug/base-branch questions; on installs where both are already configured the wizard is unchanged from F1/F2.5.
- Disabled-plugin and uninstall interactions with policy references are represented only as an "unavailable" display state in this slice; their management is F7.
- No critical product, security, or privacy decision is left unresolved for specification; planning-stage prototyping may refine the Locator IPC payload and diagnostics without weakening these outcomes.
