# Feature Specification: New Origins via Plugin

**Feature Branch**: `f4-plugins`

**Created**: 2026-09-12

**Status**: Draft

**Input**: User description: "`docs/roadmap.md` — advance to F4 — New origins via plugin. Install a plugin and start Works using a new syntax/origin, including contribution and fork modes." Authorities: `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md`, ADR-0000, ADR-0002, ADR-0003, ADR-0004, ADR-0006, ADR-0011, ADR-0012.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install a Plugin and See It Registered (Priority: P1)

A developer has a plugin package that teaches Work to recognize a new kind of origin — for example, a pull-request link. They install it from a local path while developing it (`--link`, no copy) and, separately, install a released plugin from a remote source (a pinned, fixed reference). Afterward they list installed plugins and see each one's origin and the exact reference that was pinned.

**Why this priority**: Nothing else in this slice is reachable without an installed plugin. Installation is also independently valuable and testable on its own: it either registers a working, validated component set or it fails cleanly, before anything ever tries to use it.

**Independent Test**: Install a fixture plugin from a local path with `--link`, confirm `work plugin list` shows it linked (not copied) with its declared components; separately install the same fixture as a plain local install (no `--link`) and confirm it is copied and pinned; confirm a manifest that violates role-based field rules is rejected with nothing registered.

**Acceptance Scenarios**:

1. **Given** a local plugin fixture directory with a valid `plugin.json`, **When** the user runs `work plugin install <path> --link`, **Then** Work registers the plugin by reference (no copy of the source), records its declared components and conventions in the generated registry, and `work plugin list` reports it as linked to that path.
2. **Given** a plugin published at a remote source, **When** the user runs `work plugin install <source>`, **Then** Work fixes a specific content reference at install time, copies the pinned content into local plugin storage, and `work plugin list` reports that pinned reference.
3. **Given** an already-installed alias, **When** the user installs a different origin whose manifest `name` collides with that alias, **Then** installation fails explicitly naming the conflicting alias and origin, registers nothing, and the existing plugin is unchanged; installing the same origin again under the same alias succeeds idempotently.
4. **Given** a manifest whose `components[]` includes a field disallowed for its declared `role` (for example, a `starter` entry with a `key` field), **When** installation runs, **Then** it fails before registering anything, and no partial component entry is written to the registry.

---

### User Story 2 - Create a New Work from a Plugin-Provided Origin (Priority: P1)

Once a plugin is installed, the developer runs `work start <plugin-specific-argument>` — an argument shaped for that plugin's origin, not a path or a name the reference package would recognize. Work matches the installed Starter, obtains a Repository Reference from it, resolves that reference into exactly one local repository through the same policy used for any other origin, and — because the Starter returned no start modes — continues into the ordinary new-Work journey: slug, convention, prefix, and a materialized worktree.

**Why this priority**: This is F4's core value proposition — the architectural claim that domain logic for a new origin lives entirely outside the core, proven by an end-to-end journey that produces the same guarantees F1 already ships. Without this, plugin installation would be inert.

**Independent Test**: With a fixture Starter installed that recognizes a distinctive argument shape and returns a Repository Reference (with a resolvable local path or identifiers) and no `start_modes`, run `work start <argument>` and confirm the resulting worktree, `work-state.json`, and index entry are indistinguishable in shape and guarantees from an F1 direct-path creation.

**Acceptance Scenarios**:

1. **Given** an installed Starter whose `pattern` uniquely matches the argument, **When** the user runs `work start <argument>`, **Then** Work invokes that Starter with only `{ "arg": <argument> }`, receives a Repository Reference, and resolves it through the same direct-path-or-Locator-chain pipeline used for any other Starter.
2. **Given** a Starter response with no `start_modes`, **When** resolution yields exactly one repository, **Then** Work proceeds through slug, convention, and prefix selection exactly as it would for `work start <path>`, and persists `work.start_mode: "new"`.
3. **Given** the completed creation, **When** the user inspects the result, **Then** the worktree, directory, and `work-state.json` are created with the same transactional guarantees as F1, and the terminal is repositioned into the new worktree.
4. **Given** the Starter response omits a base branch, **When** the new-Work journey reaches that step, **Then** Work prompts for a local or remote branch exactly as it does when no plugin is involved.

---

### User Story 3 - Contribute to a Resolved Reference Without the New-Work Steps (Priority: P2)

The developer runs `work start <PR-link>` against a plugin that recognizes pull-request links. The Starter resolves the repository, provides a base branch, and returns `start_modes: ["contribution", "fork"]`. The developer picks contribution mode: Work checks out the already-resolved branch directly, skipping slug, convention, and prefix, and materializes a Work around that checkout.

**Why this priority**: Contribution mode is the journey that most clearly could not exist without a plugin providing `start_modes`, and it changes the shape of Work creation (no new branch). It depends on User Story 2's resolution pipeline, so it follows it in priority.

**Independent Test**: With a fixture Starter that returns `start_modes: ["contribution", "fork"]` and a base branch for an existing (non-new) branch, run `work start <argument>`, select contribution mode, and confirm Work checks out that exact branch with no slug/convention/prefix prompts and no `branch_convention` persisted.

**Acceptance Scenarios**:

1. **Given** a Starter response containing `start_modes: ["contribution", "fork"]`, **When** `work start <argument>` runs, **Then** Work offers exactly those two modes and no others.
2. **Given** the user selects contribution mode, **When** creation proceeds, **Then** Work checks out the Starter-resolved branch directly, does not prompt for slug, convention, or prefix, and does not require that branch to be new.
3. **Given** contribution mode completes, **When** the snapshot is inspected, **Then** `work.start_mode` is `"contribution"`, the branch matches the Starter's resolved branch, and no `branch_convention` value is present.
4. **Given** the same Starter response, **When** the user selects fork mode instead, **Then** Work follows the identical journey as User Story 2 (slug, convention, prefix, new dedicated branch) using the Starter-provided repository and base branch.

---

### User Story 4 - Resolve a Starter Collision Explicitly (Priority: P3)

Two installed plugins each declare a specific recognition pattern, and both happen to match the same argument the developer types. Work does not guess or apply hidden priority: it tells the developer that multiple Starters can handle this argument, lists them, and lets the developer choose which one runs.

**Why this priority**: Determinism under multiple installed plugins is a cross-cutting gate, not a new capability — it hardens User Story 2's pipeline for the realistic case of an ecosystem with more than one plugin installed. It is lower priority because it requires two colliding fixtures to exercise.

**Independent Test**: Install two fixture Starters whose patterns both match one crafted argument, run `work start <argument>`, confirm both are listed for explicit selection with no default, and confirm the chosen Starter (and only that one) is invoked; repeat the same argument and confirm the question is asked again rather than remembered.

**Acceptance Scenarios**:

1. **Given** two enabled Starters whose patterns both match the argument, **When** the user runs `work start <argument>`, **Then** Work presents both candidates for explicit selection before invoking either.
2. **Given** the selection prompt, **When** the user picks one Starter, **Then** only that Starter is invoked, and the pipeline continues exactly as in User Story 2 from that point.
3. **Given** the same collision, **When** the user runs `work start <argument>` again in a later invocation, **Then** Work asks the same question again; the earlier choice is not remembered or reused.
4. **Given** a non-interactive invocation that hits a collision, **When** resolution would need a choice, **Then** the command fails with actionable output naming the colliding Starters, and opens no selector.
5. **Given** a collision already resolved for the argument, **When** the Starter's reference cannot be located and the user is left at the interactive SOURCE field, **Then** the field shows why resolution failed, and re-submitting shows a field error instead of asking the Starter question again.

---

### User Story 5 - Inspect and Change the Remembered Branch Convention (Priority: P3)

A repository already has a remembered branch convention from an earlier fork-mode Work. From any clone of that same repository, the developer checks which convention is remembered and switches it to a different one enabled on the machine; the new choice applies the next time a fork-mode Work is created against that repository, regardless of which clone is used.

**Why this priority**: This closes the loop on convention management introduced by the plugin-supplied convention catalog, but it is a management operation on state that only exists once User Stories 2/3 have created at least one convention choice — hence lowest priority.

**Independent Test**: Create a fork-mode Work against a repository, choosing a convention; from a second clone of the same repository, run `work convention show` and confirm it reports the same choice; run `work convention set <other-convention>` and confirm a subsequent fork-mode Work against either clone uses the new choice without asking again.

**Acceptance Scenarios**:

1. **Given** a repository with a previously remembered convention, **When** the user runs `work convention show` from any clone of that repository, **Then** Work reports the currently remembered convention without changing it or any other state.
2. **Given** more than one convention enabled on the machine, **When** the user runs `work convention set <convention>`, **Then** Work replaces the remembered choice for that repository's identity, independent of which clone the command ran from.
3. **Given** the convention has just been changed, **When** a later fork-mode `work start` targets that repository from a different clone, **Then** it uses the newly set convention and does not prompt for a choice.
4. **Given** an interactive terminal and no subcommand, **When** the user runs `work convention`, **Then** Work opens a hub showing the current repository's remembered choice and an action to change it, and after that action reveals the equivalent direct command.

### Edge Cases

- A plugin manifest declares two components with `role: starter` and no `pattern` on either (two fallbacks): installation (or, if installed independently, enabling the second) is rejected — a machine may have at most one enabled fallback Starter.
- A `--link` install is attempted against a remote source: rejected before anything is registered.
- A Starter subprocess exits non-zero or writes structurally invalid output: `work start` fails, and no Work, branch, or partial registry state results.
- A Starter's Repository Reference resolves to zero, more than one, or an invalid candidate through the F3 pipeline: the same distinct no-result / ambiguous / invalid-candidate / Locator-failure outcomes apply, unchanged by which Starter produced the reference.
- A Starter returns `start_modes` containing a value Work does not recognize: treated as a structurally invalid response, failing the same way as a malformed Starter output.
- Contribution mode is selected but the Starter did not resolve a base branch: contribution mode still requires a resolved branch to check out; its absence is a structurally invalid response, not an implicit prompt (contribution mode never asks for a branch).
- A repository has no remembered convention yet and only one convention is enabled on the machine: no prompt is needed, and that convention is used and remembered without a selection step.
- A repository has no remembered convention and more than one is enabled: fork-mode creation requires the explicit choice described in User Story 2, and only after that choice does `work convention show` have something to report.
- `work convention set <convention>` is given a name that is not currently enabled: rejected, with nothing persisted.
- Two plugins declare a convention with the same `name`: both are visible in the catalog; conventions are declarative data, not executable components, so this is not treated as the alias-collision failure that applies to installed plugin packages.
- Uninstalling, enabling, or disabling a plugin: out of scope for this slice's commands (see Out of Scope); a plugin installed under F4 is simply available once installed.
- `NO_COLOR`, `TERM=dumb`, or redirected streams during `work plugin install`, `work plugin list`, `work convention show`, or `work convention set`: output stays stable and script-readable, consistent with the F2.5 presentation contract.

## Requirements *(mandatory)*

### Functional Requirements

**Plugin installation and registry**

- **FR-001**: Work MUST allow installing a plugin package from a local path, using the same installation pipeline as the reference-package bootstrap, producing a pinned copy of that path's content.
- **FR-002**: `--link` MUST install a local path by reference, without copying, for active development, and MUST be rejected when combined with a remote source.
- **FR-003**: Work MUST allow installing a plugin package from a remote source, fixing the installed content to one specific reference at install time.
- **FR-004**: Installation MUST validate every entry in the manifest's `components[]` against its declared `role`'s required, allowed, and invalid fields, and MUST reject the whole installation — registering nothing — when any entry violates those rules.
- **FR-005**: Installation MUST register, per component, at least: package alias, component name, role, entrypoint, declared runtime (if any), and the role-specific fields relevant to matching and invocation (for example, a Starter's `pattern`); it MUST register declared `conventions[]` (name and prefixes) into the convention catalog, separately from executable components.
- **FR-006**: By default, the installed alias MUST be the manifest's `name`. When that alias already maps to a different origin in the local registry, installation MUST fail explicitly and MUST NOT overwrite, rename, or silently auto-suffix it. The failure message MUST name the plugin being installed and explain that installation failed because its name collides with the name of a plugin already installed; when the conflicting alias was proposed with `--as`, it MUST instead say that the proposed alias conflicts with an existing plugin's alias. In both cases it MUST offer choosing a different `--as` alias, and in the name-collision case it MUST also offer uninstalling the existing plugin. It MUST NOT expose the existing plugin's installation path or origin; reinstalling the same origin under the same existing alias MUST succeed idempotently. An explicit alternative alias MUST let the user resolve the conflict at install time.
- **FR-007**: Users MUST be able to list installed plugins, and each entry MUST show its origin (local linked, local pinned, or remote pinned) and its currently pinned reference.
- **FR-008**: A component's declared runtime MUST be checked at install time; a component with no declared runtime MUST be treated as a self-contained executable entrypoint.

**Starter matching and collision**

- **FR-009**: For `work start <arg>`, Work MUST evaluate every enabled Starter that declares a non-empty `pattern` against the argument locally, without invoking a subprocess for matching.
- **FR-010**: Exactly one pattern match MUST invoke that Starter directly, with no selection step.
- **FR-011**: No pattern match MUST fall back to the single enabled Starter with no pattern (the fallback); a machine MUST NOT have more than one enabled fallback Starter at a time, and enabling a second one MUST be rejected rather than silently allowed and later ignored.
- **FR-012**: More than one pattern match MUST present the user with the colliding Starters for explicit selection before any of them is invoked; the choice MUST NOT be persisted or reused on a later invocation with the same or a different argument.
- **FR-013**: No pattern match and no enabled fallback MUST fail `work start` with an actionable message to enable or install a Starter, and MUST NOT create a Work.
- **FR-014**: The invoked Starter MUST receive only `{ "arg": <argument> }`; Work MUST NOT pass start mode, base branch, metadata, links, or any other Work-derived data into a Starter's input.
- **FR-015**: A Starter subprocess failure or a structurally invalid response MUST fail `work start` with no Work materialized; there is no partial-match result distinct from success or failure.

**Repository Reference to resolution**

- **FR-016**: A successful Starter response MUST be interpreted as, at most, a transient Repository Reference (`path`, `git_fetch_urls`, `name`, `query`), plus optional `base_branch`, `start_modes`, `meta`, and `links`; only the `repository` fields feed resolution.
- **FR-017**: The Repository Reference from any Starter — reference-package or plugin-provided — MUST be resolved through the identical resolution pipeline (direct-path validation, or the ordered Repository Resolution Policy) with no Starter-specific or plugin-specific resolution path.
- **FR-017a**: When a Starter-provided reference cannot be resolved, an interactive `work start` MUST show why in the field that asks for a replacement; it MUST NOT fall back to that prompt silently, and it MUST NOT reopen the Starter selector from within that prompt.
- **FR-018**: Before any Work materialization, resolution MUST produce exactly one valid, accessible local Git repository; any resolution outcome other than a single valid repository MUST end `work start` the same way regardless of which Starter produced the reference, and MUST leave no partial state.

**Start modes**

- **FR-019**: When the Starter response includes `start_modes`, Work MUST offer exactly the returned modes and MUST NOT offer any mode the Starter did not return; an unrecognized mode value MUST be treated as a structurally invalid Starter response.
- **FR-020**: In contribution mode, Work MUST check out the Starter-resolved branch directly, MUST NOT run the slug, convention, or prefix steps, MUST NOT require that branch to be new, and MUST persist the start mode and branch without a `branch_convention` value.
- **FR-021**: In fork mode, or when the Starter response omits `start_modes`, Work MUST follow the same new-Work journey used when no plugin is involved — slug, convention, prefix, branch-name validation — ending in a dedicated branch, and MUST persist `work.start_mode` as `"new"` when `start_modes` was absent.
- **FR-022**: The persisted `work.start_mode` MUST reflect exactly what the Starter returned and, where applicable, what the user selected; Work MUST NOT infer or override it.

**Base branch**

- **FR-023**: When the Starter response omits a base branch, Work MUST prompt for selection among local and remote branches, identically to the reference-Starter journey.
- **FR-024**: When the Starter response includes a base branch, Work MUST use it directly and MUST NOT prompt for one.

**Branch convention catalog and persistence**

- **FR-025**: Outside contribution mode, the selectable convention catalog MUST include the reference package's default convention plus every convention declared by an enabled plugin.
- **FR-026**: Outside contribution mode, on first use of a given repository (identified as in ADR-0011), when more than one convention is enabled, Work MUST require an explicit choice and persist it keyed to that repository's identity; on a later use of the same repository from any clone, Work MUST reuse the persisted choice without asking again.
- **FR-027**: Users MUST be able to inspect (`work convention show`) and change (`work convention set <convention>`) the remembered convention for the repository associated with the current clone, and this MUST work identically from any clone of that repository.
- **FR-028**: `work convention show` MUST be read-only and MUST NOT alter the persisted choice, recent access, or any other state.
- **FR-029**: `work convention set <convention>` MUST reject a convention name that is not currently enabled, persisting nothing.
- **FR-030**: In an interactive terminal, `work convention` with no subcommand MUST open a hub presenting the current repository's remembered convention and an action to change it, and MUST reveal the equivalent direct command after that action; in a non-interactive context, it MUST NOT attempt to open that hub.

**Determinism and contract preservation**

- **FR-031**: Every plugin-driven success path (contribution and fork) MUST end in the same worktree, `work-state.json`, and `work.db` guarantees as an F1 direct-path creation, with no divergence in snapshot shape, transaction guarantee, or terminal-repositioning behavior based on which Starter or origin produced the Work.
- **FR-032**: All F1, F2, F2.5, and F3 command grammar, transaction guarantees, stable stdout lines, error tokens, and exit codes MUST be preserved; `work start <path>` and reference-Starter resolution MUST NOT change behavior.
- **FR-033**: A `work start` invocation that fails or is cancelled at any point (Starter match, Starter execution, resolution, mode/branch/convention/slug selection, branch-name validation) MUST leave no branch, worktree, Work directory, snapshot, or index entry behind.
- **FR-034**: `work plugin install` and `work plugin list` MUST have stable, script-usable output and exit codes; `work plugin list` MUST accept `--json` and MUST NOT alter state.

### Key Entities

- **Plugin Package**: An installable, versioned unit containing a manifest (`plugin.json`), zero or more executable components, and zero or more declarative branch conventions. Installed, listed, and identified by a locally unique alias; update, enable/disable, and uninstall are out of scope for this slice.
- **Plugin Registry Entry**: The generated (never hand-edited) local record of an installed plugin: alias, origin (local-linked, local-pinned, or remote-pinned), pinned reference, and its registered components and conventions.
- **Starter (Component)**: A component with `role: starter` that interprets a `work start` argument and returns a Repository Reference plus optional base branch, start modes, metadata, and links. Declares either a recognition `pattern` ("specific") or none ("fallback"); at most one fallback Starter may be enabled at a time.
- **Repository Reference**: The transient, per-invocation description of the target repository a Starter produces (unchanged from F3): optional local path, Git fetch endpoints, name, and/or opaque query text. Not persisted; resolved through the existing F3 pipeline regardless of which Starter produced it.
- **Start Mode**: One of the values a Starter may return in `start_modes` (`contribution`, `fork`); its absence means new-Work creation. Persisted verbatim in `work.start_mode`.
- **Branch Convention**: A declarative, non-executable contribution (`name` + `prefixes[]`) from the reference package or an enabled plugin, added to the global selectable catalog.
- **Convention Choice**: The convention remembered for a specific repository identity, set on first fork-mode use (when ambiguous) or via `work convention set`, and reusable from any clone of that repository.

### Scope and Dependencies

- Builds on delivered F1 (`specs/001-first-local-work/`), F2 (`specs/002-daily-cycle/`), F2.5 (`specs/003-terminal-ux-revamp/`), and F3 (`specs/004-local-clone-locator/`); Starter-produced Repository References resolve through F3's Repository Resolution Policy and Repository Locator chain unchanged.
- Product authority is `docs/prd.md` FR-1, FR-3, FR-6, FR-16, FR-17, FR-21 through FR-23, FR-25, and RNF-2; this slice also completes the Starter/Locator separation required by FR-2 (first delivered by F3). ADR-0000 governs the external-subprocess execution model; ADR-0002 governs installation, the generated registry, and alias collision; ADR-0003 governs the bootstrap pipeline this slice's install path reuses; ADR-0004 governs Starter matching, fallback, and collision; ADR-0006 governs component runtime/invocation; ADR-0011 governs branch convention resolution and repository identity; ADR-0012 governs the component model and manifest, including role-based field validation. `docs/add/add-0001-work-system-architecture.md` Sections 4, 5, 7, and 8 describe their realization.
- Every earlier slice's automated end-to-end demonstration must remain green.

### Out of Scope

- `work plugin enable|disable|update|uninstall`, and the full interactive `work plugin` hub with parity across all plugin lifecycle operations — delivered by F7 alongside the rest of sustainable plugin operation. `work plugin` with no subcommand in this slice prints grouped help for the subcommands that exist (`install`, `list`) and exits 0, opening no interactive menu, mirroring F3's scoped deferral of the `work repository` hub.
- Repository Locators contributed by plugins — this slice concerns Starters and conventions; a plugin-provided Locator is exercised through F3's existing, unchanged Locator contract and policy, with no new locator-specific behavior introduced here.
- Formal plugin signing (ADR-0008) — origins remain explicitly user-chosen and pinned, but content authenticity verification is deferred.
- Automatic or on-demand context: link discovery and artifact import via Importers/Linkers (F5, F6) — a plugin installed in this slice may declare a `starter` role and `conventions[]` only for the purposes of this slice's demonstration; Importer/Linker execution semantics are unaffected by and not required for F4.
- `work plugin install --as <alias>` combined with `--link` and other flag interactions beyond alias-conflict resolution and the local/remote `--link` restriction — any additional flag surface is a planning-stage detail, not a product-level scope change.
- Changing the `work start <path>` or `work start <name-or-reference>` grammar established by F1/F3, or any other established command contract.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A plugin written and installed entirely outside the core binary can drive `work start` to a materialized Work — zero core code changes required to add a new origin.
- **SC-002**: 100% of successful plugin-driven Work creations (contribution and fork) produce a snapshot and worktree indistinguishable in required shape and transactional guarantees from an equivalent F1 direct-path creation.
- **SC-003**: A local-path install with `--link` never copies the source content in 100% of test runs; a plain local or remote install always fixes a pinned reference at install time.
- **SC-004**: An alias conflict at install time fails in 100% of cases with zero silent overwrite, rename, or auto-suffix; reinstalling the same origin under the same alias succeeds idempotently in 100% of cases.
- **SC-005**: A manifest violating role-based field validation is rejected before registering any of its components in 100% of test fixtures — zero partial registrations.
- **SC-006**: When multiple installed Starters match one argument, 100% of interactive resolutions require explicit selection and 0% auto-select a candidate; the same collision is asked again (never remembered) on every subsequent invocation.
- **SC-007**: In contribution mode, 0% of runs prompt for slug, convention, or prefix; in fork mode, 100% of runs follow the identical selection sequence used without a plugin.
- **SC-008**: A repository's remembered branch convention, once set, is reported identically by `work convention show` and reused by fork-mode creation from every clone of that repository, in 100% of test fixtures.
- **SC-009**: Any `work start` invocation that fails or is cancelled at any stage leaves zero branches, worktrees, Work directories, snapshots, or index entries, in 100% of tests.
- **SC-010**: All existing F1, F2, F2.5, and F3 non-interactive contract tests remain green with no changes to arguments, flags, stable stdout, error tokens, or exit codes.

## Assumptions

- F3 (Find the Local Clone) is delivered; plugin-provided Repository References are resolved with no changes to the Repository Resolution Policy, Locator eligibility, or candidate-validation behavior already shipped.
- "Complete manifest and role-based validation" means the installer enforces the required/allowed/invalid field table for every role (`starter`, `importer`, `linker`, `repository-locator`) even though this slice exercises only `starter` end-to-end; `importer`/`linker` validation is built now so later slices (F5/F6) do not need a second installation-validation pass.
- A fixture plugin used for testing may declare an `importer` or `linker` component for manifest-validation coverage, but this slice does not execute those roles; their runtime behavior is delivered in F5/F6.
- Repository identity for convention memory follows ADR-0011 (origin URL, then root commits, then absolute path for a remote-less shallow clone) and reuses the identity mechanism already implied by F1's `freeform`-only convention handling, now generalized to a multi-convention catalog.
- The registry's per-component fields (alias, name, role, entrypoint, runtime, pattern, `accepts`, events/filters, inputs, Linker key, discovery, manual presentation) are all recorded at install time as described in `add-0001` §5, even for roles not yet executed, so update/uninstall in later slices operate on complete records.
- Non-interactive collision resolution and non-interactive `work convention` sub-behavior follow the same "fail with actionable output, never open a selector" rule already established for F3's non-interactive contracts.
- Error tokens and exit codes for new failure modes introduced here (manifest validation failure, alias conflict, missing fallback, Starter collision in non-interactive mode, structurally invalid Starter response, unset convention name) are defined during planning, consistent with the existing error taxonomy.
- Plugin runtime execution (subprocess invocation, `<runtime> <entrypoint>` command construction, stdin/stdout JSON contract, exit-code interpretation) follows ADR-0006 and ADR-0000 exactly as documented; no new IPC transport is introduced by this slice.
- No critical product, security, or privacy decision is left unresolved for specification; planning-stage prototyping may refine exact manifest validation error messages and registry field layout without weakening these outcomes.
