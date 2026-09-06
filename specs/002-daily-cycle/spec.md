# Feature Specification: Daily Cycle — Resume and Archive

**Feature Branch**: `feature/002-daily-cycle-p0-specs`

**Created**: 2026-09-06

**Status**: Draft

**Input**: User description: "`docs/roadmap.md` — F2 only — Ciclo diário: retomar e arquivar Works. Works stop being disposable: they can be resumed by recency and archived while preserving their canonical snapshot."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Resume a Work by recency (Priority: P1)

A person who created one or more Works earlier wants to return to one of them. They run `work resume`, see their Works ordered from most recently accessed to least recently accessed, pick one, and their terminal session lands inside that Work's checkout. If they already know which Work they want, they name it on the command line and skip the list entirely.

**Why this priority**: Without a way back in, every Work created by F1 is a dead end. Resume is the single capability that turns "create a Work" into "work across days". It is the smallest slice that delivers standalone value on top of F1.

**Independent Test**: With two or more Works created via F1, run `work resume`, confirm the ordering reflects last access, select the least-recently-accessed one, and verify the session is repositioned into its worktree and that its recent-access position moves to the top. Repeat with an explicit target and confirm no list is shown.

**Acceptance Scenarios**:

1. **Given** two Works A and B where B was accessed more recently than A, **When** the user runs `work resume` in an interactive terminal, **Then** the Works are listed with B before A, each distinguishable by its identity, and choosing a Work repositions the session into that Work's worktree.
2. **Given** the user selects Work A from the list, **When** the resume completes, **Then** A's recent-access position is updated so a subsequent `work resume` lists A before B, and the canonical snapshot and the projection agree on the new access time.
3. **Given** an explicit target that is the opaque id of one Work, **When** the user runs `work resume <id>`, **Then** no selection list is shown, the session is repositioned into that Work, and the recent-access position is updated exactly as in the interactive path.
4. **Given** an explicit target that is not the id of any Work, **When** the user runs `work resume <id>`, **Then** the command fails with an actionable message and changes no state.
5. **Given** the current environment cannot fulfill the terminal-repositioning contract, **When** a resume otherwise succeeds, **Then** the recent-access update is still applied, the real worktree path is reported unambiguously, and the result states that the session was not repositioned, with actionable guidance — matching F1's behavior for the same condition.

---

### User Story 2 - Archive Works while preserving their context (Priority: P2)

A person is done with one or more Works and wants them out of the active list without losing what was recorded about them. They run `work archive`, select one or more active Works, confirm, and the tool destroys those Works' worktrees while keeping each Work's canonical snapshot (and any imported artifacts) in an archived area. Naming explicit targets skips the selection step but never skips the confirmation.

**Why this priority**: Archiving keeps the active list meaningful as Works accumulate, and it is the operation that makes the "preserve the snapshot" guarantee observable. It depends on nothing from Story 1 and can ship independently, but resuming is the more frequent daily action, so it ranks second.

**Independent Test**: With several Works created via F1, run `work archive`, multi-select a subset, confirm, and verify those worktrees are gone, their snapshots remain readable under the archived area, the still-active Works are untouched, and the projection no longer lists the archived Works as active. Repeat with explicit targets and confirm the confirmation prompt still appears.

**Acceptance Scenarios**:

1. **Given** three active Works, **When** the user runs `work archive` interactively, **Then** all three are listed for multi-selection, none is selected by default, and nothing changes until the user confirms.
2. **Given** the user has selected two Works and confirms, **When** the archive completes, **Then** exactly those two worktrees are destroyed, each of the two Works' `work-state.json` (and any non-core files a plugin added) is preserved under the archived area, the third Work is unchanged, and each archived Work's persisted state reflects that it is archived.
3. **Given** explicit targets that are the opaque ids of one or more Works, **When** the user runs `work archive <ids>`, **Then** the selection list is skipped but the applicable confirmation is still required, and declining the confirmation changes nothing.
4. **Given** a non-interactive invocation with explicit target ids and an explicit approval flag, **When** `work archive` runs, **Then** it archives the named Works without prompting and returns a stable exit code; without the approval flag it fails without archiving anything.
5. **Given** a selected Work whose worktree has uncommitted or untracked changes, **When** `work archive` reaches that Work, **Then** it does not destroy that worktree until the user gives an extra explicit acknowledgement (interactively) or passes the corresponding flag (non-interactively); without it, that Work is left active and the rest of the batch is unaffected.
6. **Given** an archive operation that fails partway (for example, a worktree cannot be removed), **When** the failure occurs, **Then** already-archived Works in the batch stay consistently archived, the failing Work is left either fully active or fully archived — never half-moved — and the projection matches the on-disk reality after the run.
7. **Given** a target id that names a Work that is already archived or does not exist, **When** `work archive <id>` runs, **Then** the command reports the condition without altering any other Work in the batch.

---

### User Story 3 - Rebuild the lookup index from canonical snapshots (Priority: P3)

A person's global lookup index (`work.db`) is deleted, truncated, or becomes inconsistent with what is on disk. They can rebuild it entirely from the Works' canonical snapshots, and afterwards `work resume` and `work archive` behave exactly as before.

**Why this priority**: This is the guarantee that the SQLite projection is genuinely disposable. It is essential to the slice's exit criteria but is exercised by a recovery/maintenance path rather than the daily journey, so it ranks last.

**Independent Test**: Create and access several Works (some archived), delete or corrupt `work.db`, trigger a rebuild, and verify the resulting index lists every Work with the correct status and recent-access order, and that no canonical snapshot was modified in the process.

**Acceptance Scenarios**:

1. **Given** a populated set of active and archived Works and a `work.db` that has been deleted, **When** the index is rebuilt, **Then** every Work discoverable on disk appears in the index with status, identity, branch, and timestamps matching its snapshot.
2. **Given** a `work.db` whose rows disagree with the snapshots (stale access time, missing row, row for a Work no longer on disk), **When** reconciliation runs, **Then** the index is brought into agreement with the snapshots, which are treated as authoritative and are never rewritten to match the index.
3. **Given** a rebuild has completed, **When** the user runs `work resume`, **Then** the recency ordering is identical to what it was before the projection was lost.
4. **Given** a snapshot that cannot be read or parsed during a rebuild, **When** the rebuild runs, **Then** it reports that Work as skipped with an actionable diagnostic and still indexes the remaining Works.

---

### Edge Cases

- The user runs `work resume` or `work archive` when no Works exist yet: the command explains that there is nothing to resume/archive and exits without error state.
- The user runs `work resume` from inside a Work's worktree with no target: the current Work is still listed and selectable; selecting it is a valid no-op reposition that still updates recent access.
- The user archives the Work whose worktree is the current working directory: the worktree is still destroyed and the session is repositioned out of it (to a safe, reported location), rather than leaving the shell stranded in a deleted directory.
- Two Works share the same slug (same repository, different branches, or different repositories): the interactive picker still shows them as distinct rows; explicit targeting is by opaque id, so the ambiguity never reaches a mutation.
- A Work's worktree was manually deleted outside the tool before `work archive`: archiving still moves the snapshot to the archived area and records the archived state; the missing worktree is not treated as a hard failure.
- A Work's worktree has uncommitted changes or untracked files when it is archived: the archive stops short of destroying that worktree until the user gives an extra explicit acknowledgement (interactive) or passes the dedicated flag (non-interactive).
- The archived-area destination directory already exists (same repository, same branch, archived twice on the same day): the collision is resolved deterministically without overwriting a previously archived Work.
- `work resume` is given a target that matches only an archived Work: the command reports that the Work is archived and does not attempt to reposition into a destroyed worktree (un-archiving is out of scope for this slice).
- The recent-access update or the archive move is interrupted mid-write: a later inspection sees either the old state or the new state, never a torn snapshot or an index that contradicts every snapshot.
- Concurrent `work resume`/`work archive` invocations target the same Work: at most one mutation wins; the other fails cleanly without corrupting state.

## Requirements *(mandatory)*

### Functional Requirements

#### Resume

- **FR-001**: The product MUST offer `work resume [target]` and an equivalent journey in the interface opened by `work` with no arguments.
- **FR-002**: With no target in an interactive terminal, `work resume` MUST list existing resumable Works ordered from most recently accessed to least recently accessed, with each Work presented so that Works sharing a slug remain distinguishable.
- **FR-003**: Selecting a Work in `work resume` (interactively or by explicit target) MUST update that Work's recent-access marker to "now" in the canonical snapshot and reflect the same value in the projection.
- **FR-004**: On success, `work resume` MUST reposition the terminal session into the selected Work's worktree, using the same shell-integration contract and the same fallback behavior F1 defined for `work start` when that contract cannot be honored.
- **FR-005**: An explicit target MUST skip the selection list but MUST NOT skip identity resolution or the recent-access update; a target that resolves to no Work MUST fail with an actionable message and no state change.
- **FR-006**: `work resume` MUST NOT modify any Work other than the one being resumed, and MUST NOT run plugins or extension logic.
- **FR-007**: In non-interactive input, `work resume` with no resolvable target MUST fail with actionable guidance and a stable exit code without opening a TUI.

#### Archive

- **FR-008**: The product MUST offer `work archive [targets...]` and an equivalent journey in the interface opened by `work` with no arguments.
- **FR-009**: With no targets in an interactive terminal, `work archive` MUST list active Works with multi-selection and MUST NOT preselect any Work.
- **FR-010**: `work archive` MUST require an applicable confirmation before performing any destructive action, whether the Works were chosen from the list or named as explicit targets.
- **FR-011**: In non-interactive input, `work archive` MUST require an explicit approval flag to proceed; without it the command MUST fail without archiving anything, and it MUST never open a TUI.
- **FR-012**: On confirmation, `work archive` MUST, for each selected Work: destroy the Work's worktree via Git, move the Work's remaining files (canonical snapshot and any non-core artifacts) into the archived area, and update the canonical snapshot so its status reflects that the Work is archived.
- **FR-013**: Before destroying a selected Work's worktree, `work archive` MUST detect whether that worktree has uncommitted or untracked changes and, if so, MUST NOT destroy it until the user gives an extra explicit acknowledgement (interactively) or passes a dedicated flag (non-interactively); an un-acknowledged dirty Work is left active and does not fail the rest of the batch.
- **FR-014**: The underlying Git branch a Work created in the source repository MUST be left intact when the Work is archived; branch deletion is out of scope for this slice.
- **FR-015**: The archived location MUST preserve the canonical snapshot intact except for the core-governed status transition and MUST NOT require the source repository or any plugin to be present to read it later.
- **FR-016**: Archiving MUST leave every non-selected Work, all configuration, and all bootstrap state untouched.
- **FR-017**: Each per-Work archive step MUST be transactional: a Work is either fully active or fully archived after the run, never partially moved, and a failure on one Work in a batch MUST NOT roll back Works already archived in that batch.
- **FR-018**: After a successful or partial `work archive`, the projection MUST agree with on-disk reality (archived Works no longer listed as active; still-active Works unchanged).
- **FR-019**: `work archive` MUST report — without failing the whole batch — any target that names a Work that does not exist or is already archived.
- **FR-020**: The set of Works eligible for `work resume` and for `work archive` MUST be defined explicitly by Work status; archived Works MUST NOT appear in the `work archive` selection list, and MUST NOT be repositioned into by `work resume`.

#### Canonical state and projection

- **FR-021**: `work-state.json` MUST remain the sole authority for a Work's status and recent-access marker; every change in this slice (recent-access bump, archive transition) MUST be written to the snapshot atomically and only by the core.
- **FR-022**: The product MUST be able to rebuild `work.db` entirely from the canonical snapshots of all Works (active and archived) such that resume ordering and archive eligibility are identical to before the projection was lost.
- **FR-023**: The product MUST be able to reconcile an existing `work.db` against the snapshots, resolving disagreements in favor of the snapshots and never rewriting a snapshot to match the projection.
- **FR-024**: A rebuild or reconciliation MUST skip and diagnose any unreadable or unparseable snapshot without aborting the indexing of the remaining Works.
- **FR-025**: Losing, deleting, or corrupting `work.db` MUST NOT cause the loss of any Work's canonical state, and MUST NOT block a subsequent rebuild.

#### Cross-cutting

- **FR-026**: `work resume` and `work archive` MUST accept an explicit target only as a Work's opaque id (the `id` recorded in its canonical snapshot); no other identifier form is accepted for targeting, so an explicit target is never ambiguous. Human selection among same-slug Works is served by the interactive picker, which MUST render them as distinct rows.
- **FR-027**: Diagnostics MUST distinguish at least: nothing to resume/archive, target id not found, target already archived, dirty worktree not acknowledged, worktree removal failure, archive move failure, snapshot write failure, unreadable snapshot during rebuild, and incompatible shell.
- **FR-028**: Success and failure results of `work resume` and `work archive` MUST have messages and exit codes stable enough for scripting, including unambiguous identification of the affected Work(s) and their locations.
- **FR-029**: All operations in this slice MUST be deterministic, work without AI, and not require network access.
- **FR-030**: The `work` home interface MUST reach the resume and archive journeys by keyboard, in addition to the F1 journeys it already exposes.

### Key Entities

- **Resumable Work**: An existing Work in the active state, identified by its canonical snapshot, with a recent-access marker used to order the resume list.
- **Recent-access marker**: The `last_accessed_at` field of the canonical snapshot; the single value that determines resume ordering and that a successful resume updates.
- **Archive operation**: A bounded, confirmed operation over one or more selected Works that destroys worktrees and relocates snapshots, with per-Work transactional boundaries.
- **Archived Work**: A Work whose worktree has been destroyed and whose canonical snapshot (plus any non-core artifacts) lives in the archived area, with a status that records the archival.
- **Archived area**: The location under the workspace root where archived Works' files are kept, separate from the active area, readable without the source repository or any plugin.
- **Work id**: The opaque, sortable identifier assigned at creation (F1) and recorded in the snapshot and the projection; the only accepted form of an explicit command-line target in this slice.
- **Global lookup projection**: `work.db`; a derived, rebuildable index over all Works used to list, order, and filter them; never a competing source of truth.
- **Index rebuild / reconciliation**: A maintenance operation that reconstructs or corrects the projection from the canonical snapshots without modifying them.

### Scope Boundaries

**Included in F2**:

- `work resume [id]`: recency-ordered listing, interactive selection, explicit target by Work id, recent-access update, terminal repositioning.
- `work archive [id...]`: multi-selection, explicit targets by Work id, mandatory confirmation, non-interactive approval flag, dirty-worktree acknowledgement, worktree destruction, snapshot relocation to the archived area, status transition.
- Expansion of the `work` home interface to reach both journeys.
- Full rebuild and reconciliation of `work.db` from canonical snapshots, including archived Works.
- Failure and interruption handling for the recent-access update and the archive move.

**Explicitly excluded from F2**:

- Un-archiving or otherwise returning an archived Work to the active state.
- Deleting a Work permanently (removing its snapshot).
- Deleting the Git branch a Work created in the source repository.
- Targeting a Work by slug, branch name, or directory name on the command line (interactive picker only for human disambiguation).
- Finding clones by name, reference, roots, or Repository Locators (F3).
- Installing or administering plugins, resolving Starter collisions, and `contribution`/`fork` modes (F4).
- Publishing links or metadata and running Linkers or Importers, automatic or manual (F5, F6).
- `work status` and other read-only inspection surfaces (F6).
- Administrative surfaces for plugins, Locators, policy, roots, and conventions (F7).
- Any change to how F1 creates a Work.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user who has created two or more Works can resume a chosen one and land in its checkout in under 30 seconds, without consulting external documentation, on every officially supported operating-system/shell combination.
- **SC-002**: In 100% of resume operations, the resumed Work's recent-access position moves ahead of every Work not accessed since, and a repeated `work resume` reflects the new order.
- **SC-003**: In 100% of archive operations, exactly the selected Works are archived: their worktrees are gone, their snapshots are readable in the archived area, their persisted status is archived, and no non-selected Work changes.
- **SC-004**: 100% of `work archive` invocations perform no destructive action before an explicit confirmation (interactive) or an explicit approval flag (non-interactive).
- **SC-005**: 100% of failures and interruptions injected during the recent-access update or the archive move leave the affected Work fully in one state (active or archived) with a snapshot that is readable and internally consistent.
- **SC-006**: After deleting `work.db` and rebuilding it from snapshots, the active/archived classification and the resume ordering are identical to before, in 100% of trials, and 0% of canonical snapshots are modified by the rebuild.
- **SC-007**: The full F1 demonstration (create a first local Work) continues to pass unchanged alongside the F2 journeys.
- **SC-008**: In usability testing, at least 90% of participants successfully resume the correct Work on the first attempt when choosing from a list of at least three.
- **SC-009**: In 100% of archive attempts where a selected Work's worktree has uncommitted or untracked changes, no such worktree is destroyed without the extra acknowledgement (interactive) or dedicated flag (non-interactive).

## Assumptions

- "Resumable" means a Work whose status is the F1 active state (`in-progress`); archived Works are excluded from `work resume` targeting and listing. Un-archiving is a later concern.
- The archived area is `archived/` under the workspace root, as described in the architecture document, with per-Work directories prefixed by the archival date; the worktree is not copied there.
- The terminal-repositioning contract and its unavailable-environment fallback are exactly those established for F1's `work start`; F2 reuses them for `work resume` and does not redefine them.
- Non-interactive `work archive` uses the same explicit-approval convention F1 established for non-interactive `work start` (an explicit flag; absence is a hard failure, not an implied "yes"). A separate dedicated flag is required to archive a Work whose worktree is dirty.
- Explicit command-line targeting is by the Work's opaque `id` (already present in the F1 snapshot and projection). The `id` is what `work resume` / `work archive` echo on success and what scripts capture, so a Work is never targeted by a mutable or shared attribute.
- Archiving does not touch the Git branch: the worktree is removed with `git worktree remove` (or equivalent) but the branch ref stays in the source repository. A user who wants the branch gone deletes it themselves.
- `work resume` does not require or resolve a "current Work": with no target it always lists; from inside a worktree the current Work is simply one of the listed rows.
- Changing the configured workspace root does not move already-materialized Works, active or archived (carried over from F1); F2 operates on Works where they already live.
- The projection may gain columns or a table to record status and to support archived Works, but it remains derived and rebuildable; no new canonical store is introduced.
- Git is available and the source repository is reachable for worktree removal in the normal path; a missing worktree or missing source repository degrades to preserving the snapshot and recording the archived state rather than failing hard.
- Concurrency between `work` invocations is guarded by the same advisory-lock mechanism F1 introduced.
