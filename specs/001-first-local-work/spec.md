# Feature Specification: First Local Work

**Feature Branch**: N/A — no branch was created by the specification flow

**Created**: 2026-09-04

**Status**: Draft

**Input**: User description: "`docs/roadmap.md` — F1 only — First local Work"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create the first Work from a local clone (Priority: P1)

A person with an already-cloned Git repository provides its local path, chooses where Works will be stored, selects the base branch, and defines the name of the new work. In the end, they get their own branch in an isolated worktree and can already work in the new checkout.

**Why this priority**: This is the product's minimum value proposition and the vertical demonstration that validates the core of Work creation.

**Independent Test**: On a clean installation with no network, run `work start <path>` against a valid Git clone, complete the requested choices, and confirm that the new checkout is ready, isolated, and represented by coherent canonical state.

**Acceptance Scenarios**:

1. **Given** a clean installation, a compatible interactive terminal, and a path to a valid local Git clone, **When** the user runs `work start <path>`, accepts or changes the suggested workspace root, selects a base branch, provides a slug, and chooses the prefix of the `freeform` convention, **Then** a Work in `in-progress` state is created with its own branch, isolated worktree, canonical snapshot, and coherent lookup record, and the session ends positioned in the new checkout.
2. **Given** a clean installation with no network access, **When** the first `work start <path>` is started, **Then** the minimum official set required by the local flow is made available automatically and creation can be completed without additional manual installation.
3. **Given** a selected local or remote base branch, **When** the Work is materialized, **Then** the new branch starts exactly from the selected revision and the repository's original checkout remains unchanged.
4. **Given** a successfully created Work, **When** its snapshot is reopened and compared to the checkout and the lookup record, **Then** identity, status, slug, branch, base, convention, and locations describe the same Work.

---

### User Story 2 - Start the same flow through the guided interface (Priority: P2)

A person who doesn't remember the command syntax opens `work`, navigates the home interface, and starts creating a local Work. They can also run `work start` with no source and provide the path when prompted.

**Why this priority**: Guided discovery reduces friction on first contact and establishes the interactive surface that later slices will expand.

**Independent Test**: Open `work` in an interactive terminal, choose the start-Work action, provide a local path, and complete the same journey as story P1 using only the keyboard.

**Acceptance Scenarios**:

1. **Given** an interactive terminal, **When** the user runs `work` with no arguments, **Then** the home interface offers the available journey to start a Work and lets the user go through it by keyboard.
2. **Given** an interactive terminal, **When** the user runs `work start` with no source, **Then** the repository path is requested and, once provided, the flow converges to the same validations and guarantees as `work start <path>`.
3. **Given** an already-configured workspace root, **When** another Work is started, **Then** the root is reused without asking again, unless the user chooses to change it.
4. **Given** that the user changes the configured workspace root, **When** they create later Works, **Then** the new root is used without moving or re-identifying existing Works.
5. **Given** a non-interactive input missing a required value, **When** the command is run, **Then** it exits with actionable guidance and a stable exit code, without opening the interactive interface or creating Work state.

---

### User Story 3 - Recover from errors without leaving partial state (Priority: P3)

A person who provides an invalid path, produces an invalid branch name, or tries to use a name incompatible with an existing branch gets a clear explanation and can fix the choice before any definitive materialization.

**Why this priority**: Trust in the product depends on predictable Git and filesystem operations, especially on first use.

**Independent Test**: Exercise invalid paths, rejected names, branch collisions, cancellations, and induced failures at each materialization boundary; verify that no orphan branch, Work directory, snapshot, or record remains.

**Acceptance Scenarios**:

1. **Given** a path that does not exist, is inaccessible, or does not represent a usable Git repository, **When** it is provided, **Then** the problem is identified before materialization and the interactive flow lets the user provide another path.
2. **Given** a slug or prefix that results in an invalid branch name, **When** the name is validated, **Then** the cause is presented and the user can fix the choice in the same flow without any branch or Work having been created.
3. **Given** a resulting name that collides with an incompatible local or remote branch, **When** the collision is detected, **Then** the conflicting branch is identified and the user can choose another name without partial state.
4. **Given** a failure during materialization, **When** the operation cannot publish a coherent Work, **Then** all new artifacts of the attempt are rolled back and no incomplete snapshot or orphan record remains visible.
5. **Given** that the user cancels before final confirmation, **When** the flow ends, **Then** no branch, worktree, Work folder, snapshot, or index entry is created by the attempt.

### Edge Cases

- The path contains spaces, non-ASCII characters, relative segments, or symbolic links; resolution must be unambiguous and consistent across the supported platforms.
- The path points to a Git worktree, to a repository with no commits, to a bare repository, or to a directory that can be read but on which the required operation is not permitted.
- The base branch exists only as a remote reference, or equal local and remote names point to different revisions; the displayed choice must distinguish source and revision.
- The slug is empty, contains separators, spaces, reserved characters, or combinations rejected by Git's reference rules.
- The computed branch name already exists locally, remotely, or is associated with another worktree.
- The Work's destination directory already exists, is partially occupied, or is not writable.
- Two concurrent attempts compute the same branch or the same Work directory; at most one may complete and the other must fail without changing the winning Work.
- The process is interrupted between creating the branch, the worktree, the snapshot, and the lookup record; a new inspection must not observe a partially published Work.
- The official bootstrap has already completed, was previously interrupted, or finds existing valid records; repetition must converge to a single usable state without duplication.
- Integration with the current shell is not available; the result must not claim the session changed directory and must report the real worktree path and the guidance defined by the support contract.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The product MUST offer `work start [source]` and an equivalent action in the interface opened by `work` with no arguments.
- **FR-002**: In this slice, `source` MUST be interpreted exclusively as a direct path to a local Git repository; lookup by name, remote reference, or configured roots MUST NOT be started as a fallback.
- **FR-003**: Before materializing the Work, the product MUST validate that the resolved path exists, is accessible, and represents a usable Git repository with at least one selectable base branch.
- **FR-004**: On a clean installation, the product MUST make the minimum official package available automatically and without network, containing the local Starter and the `freeform` branch convention, through the same controlled registry that will serve the other packages.
- **FR-005**: The official bootstrap MUST be idempotent: repeated runs or runs resumed after interruption must not duplicate components, conventions, or records.
- **FR-006**: When no workspace root is configured, the product MUST suggest a value, allow changing it, validate its fitness to hold Works, and persist the choice before the first creation.
- **FR-007**: The user MUST be able to later change the workspace root for new creations, without moving or modifying existing Works.
- **FR-008**: Aside from a value already provided validly, required choices in an interactive terminal MUST be collected by the guided interface; each explicit value MUST skip only its corresponding collection, without skipping validations.
- **FR-009**: The product MUST list local and remote base branches so that homonymous or divergent references can be distinguished and an exact revision can be selected.
- **FR-010**: The product MUST request a non-empty slug and present the prefixes of the resolved branch convention; the official package MUST make the `freeform` convention available on first use.
- **FR-011**: The final branch name MUST be derived deterministically from the chosen convention, prefix, and slug, and MUST be shown to the user before materialization.
- **FR-012**: Before creating any branch or worktree, the product MUST validate the final name against Git's rules and detect incompatible collisions in local references, remote references, and existing worktrees.
- **FR-013**: When a name or collision is rejected in interactive mode, the product MUST explain the offending choice and allow correcting it in the same flow, repeating all affected validations.
- **FR-014**: After successful validation, the product MUST create a branch from the chosen base revision and associate it with an isolated worktree, without changing files, the current branch, or uncommitted changes of the source checkout.
- **FR-015**: Each completed Work MUST occupy its own directory under the `in-progress` area of the workspace root and contain exactly the core structure required in this slice: `worktree/` and `work-state.json`.
- **FR-016**: `work-state.json` MUST be the canonical, self-contained, versioned snapshot of the Work and MUST record, at minimum, slug, `in-progress` status, `new` mode, Starter, branch, base branch, convention, and creation and access timestamps.
- **FR-017**: The snapshot MUST be published as a complete unit; interruption or write failure MUST NOT expose partial or invalid content as canonical state.
- **FR-018**: The core MUST be the sole authority for governing and changing the Work's state fields; behavior provided by the official package MUST remain outside the core's trust boundary.
- **FR-019**: After publishing the snapshot, the product MUST create or update a global lookup record coherent with it; that record MUST be treated as a projection, never as a competing canonical source.
- **FR-020**: Creation MUST be transactional up to full publication: if any step fails, the attempt MUST remove its new branch, worktree, Work folder, snapshot, and global record, without removing previously valid configuration or bootstrap.
- **FR-021**: Cancellation before full publication MUST obey the same guarantees of no orphan artifacts as a failure.
- **FR-022**: On successful completion in an officially supported interactive shell, the session MUST be positioned in the new worktree's directory and the result MUST clearly identify the Work, the branch, and its path.
- **FR-023**: If the current environment cannot fulfill the documented terminal-positioning contract, the product MUST report that condition without claiming the session changed directory, unambiguously report the real worktree path when it exists, and present actionable instruction to enable or use the correct integration.
- **FR-024**: In non-interactive input, any missing source or required choice MUST cause an actionable failure without attempting to open a TUI and without performing Work mutations.
- **FR-025**: Success and failure results of `work start` MUST have messages and exit codes stable enough for use by scripts, including the unambiguous identification of the created directory on success.
- **FR-026**: All choices, validations, and effects MUST be deterministic, work without AI, and not depend on network access for this slice's local journey.
- **FR-027**: Diagnostics MUST distinguish at least invalid path, unusable repository, missing base branch, invalid name, branch collision, occupied destination, bootstrap failure, materialization failure, and incompatible shell, without exposing unnecessary repository content.
- **FR-028**: A created Work MUST be reopenable and validatable by an integration check that compares snapshot, worktree, Git branch, and global record without requiring F2's resume features.
- **FR-029**: This slice's TUI home MUST allow reaching every public journey available in F1 by keyboard and need not expose actions reserved for later slices.

### Key Entities

- **Local source**: Transient reference provided by the user; contains the resolved path of the clone and exists only to drive validation and creation.
- **Workspace configuration**: Persisted preference that defines where new active Works will be materialized; later changes do not alter existing Works.
- **Official reference package**: Self-contained set made available on first use, with its own identity and registry, that provides local-path interpretation and the `freeform` convention without network.
- **Branch convention**: Named rule that offers prefixes and, together with the slug, deterministically produces the candidate branch name.
- **Work**: Active unit of work relating slug, status, start mode, Starter, branch, base branch, convention, timestamps, directory, and worktree.
- **Canonical snapshot**: Versioned, self-contained representation of the Work in `work-state.json`; it is the authority over that unit's persisted state.
- **Global lookup record**: Projection derived from the snapshot used to locate and query Works; it must remain coherent but does not replace the snapshot as the source of truth.
- **Creation attempt**: Bounded operation that gathers choices, validations, and mutations and that ends in full publication or rollback of the new artifacts.

### Scope Boundaries

**Included in F1**:

- Clean installation and minimum offline bootstrap of the official package.
- Direct local Git path, provided by command or by the interactive interface.
- Initial configuration and change of the workspace root.
- Selection of base branch, slug, and prefix of the `freeform` convention.
- Transactional creation of branch, worktree, snapshot, and global record.
- Initial TUI home, basic non-interactive behavior, and the shell integration needed to enter the checkout.

**Explicitly excluded from F1**:

- Resuming or archiving Works and fully rebuilding the global record (F2).
- Finding clones by name, reference, search roots, or Repository Locators (F3).
- Installing or administering third-party plugins, resolving collisions between Starters, and using `contribution` or `fork` modes (F4).
- Publishing links or domain metadata and running automatic or manual Linkers or Importers (F5 and F6).
- Complete administrative surfaces for plugins, Locators, policies, roots, and conventions (F7).
- Formal plugin signing, telemetry, cross-machine synchronization, graphical interface, progress streaming, core-imposed timeout, automatic convention detection, and additional official integrations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 90% of test users can go from a clean installation to a ready worktree within 3 minutes, without consulting external documentation.
- **SC-002**: 100% of acceptance scenarios with valid local repositories complete with mutually coherent branch, worktree, snapshot, and global record on every officially supported combination of operating system and shell.
- **SC-003**: 100% of failures and interruptions injected before full publication leave zero orphan branches, worktrees, Work directories, snapshots, or global records belonging to the attempt.
- **SC-004**: 100% of invalid-name or incompatible-collision cases are rejected before the first Git or Work mutation and allow correction in the same interactive flow.
- **SC-005**: 100% of first-use runs planned for F1 complete without network access and without requiring manual installation beyond the declared prerequisites.
- **SC-006**: On 100% of officially supported shells, a successful creation leaves the session in the new checkout; in incompatible environments, 100% of results correctly state that no repositioning happened and provide an actionable path and guidance when applicable.
- **SC-007**: After 100 repeated bootstrap runs and 20 interruptions at distinct points, there is exactly one usable record of each official component, with no duplication or partial state.
- **SC-008**: In usability testing, at least 90% of participants identify the cause and complete the correction of an invalid path or name on the first attempt, without restarting the command.

## Assumptions

- The user already has Git installed and accessible, a writable local clone, and credentials or permissions sufficient to query their references and create a branch/worktree.
- A repository usable in F1 has at least one commit and one selectable local or remote branch; empty and bare repositories are rejected with actionable diagnostics.
- The official `freeform` convention produces the name from the slug via its default prefix; multiple conventions and remembering the choice per repository identity belong to later slices.
- The official matrix of operating systems and shells will be finalized in the implementation plan; this specification's criteria apply in full to each combination declared in that matrix.
- The contract by which a supported shell session enters the worktree will be decided and documented in the plan, preserving the observable result of FR-022 and FR-023.
- The self-contained distribution format of the official package will be decided in the plan and must prove the absence of additional runtime and of network on all supported platforms.
- The suggested workspace root may vary by platform, as long as it is explicit, writable, persisted, and separate from the source clones.
- In F1, changing the workspace root for later creations is done by editing `workspace` in the persisted configuration directly; an in-product command to change it belongs to a later slice. FR-007's guarantee — existing Works are neither moved nor re-identified — applies to that manual change.
- Valid configuration and bootstrap completed before a Work attempt may remain after cancellation or failure; the transactional guarantee covers all artifacts belonging to the creation attempt.
