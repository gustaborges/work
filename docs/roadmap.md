# Vertical Delivery Roadmap — Work v1

**Status:** Proposal

**Date:** 2026-09-04

**Sources:** `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md`, and accepted ADRs

***

## 1. Goal

This roadmap turns the scope of v1 into vertical deliveries: each slice ends in an executable, demonstrable, and useful journey for someone. A slice includes the minimum required CLI/TUI, domain, Git, persistence, plugin contracts, error messages, and tests; isolated technical components are not treated as product milestones.

The roadmap defines order and exit criteria, not dates. Dates depend on team capacity, cadence, and the desired coverage level. The sequence reduces risk first in the core value proposition — creating a deterministic worktree — and then expands the same flow to localization, extensions, and full operation.

***

## 2. Delivery line

| Slice | Usable outcome | Milestone |
| --- | --- | --- |
| F1 — First local Work | Create and enter a Work from a local Git path | Walking skeleton |
| F2 — Daily cycle | Resume and archive Works without losing the snapshot | Local alpha |
| F2.5 — Terminal experience | Finish interactive journeys with a clean history and discover commands through a brand entry point and full help | Consolidated alpha |
| F3 — Find the clone | Start by name or reference using roots and Locator policy | Local beta |
| F4 — New origins via plugin | Install a plugin and start Works using a new syntax/origin | Extensible beta |
| F5 — Automatic context | Discover links and import artifacts when `work start` completes | v1 feature preview |
| F6 — On-demand context | Inspect links and run manual Linkers/Importers | v1 feature complete |
| F7 — Sustainable operation | Manage plugins, Locators, policy, and conventions safely | v1 release candidate |

The slices are cumulative. No slice may break the demonstrable journeys of the previous ones.

***

## 3. Vertical slices

### F1 — First local Work

**User outcome:** `work start /path/to/repo` produces a valid branch, an isolated worktree, and a canonical snapshot; at the end, the user can work in the new checkout.

**End-to-end demonstration:**

1. On a clean installation, run `work start <path>`.
2. Set the workspace root on first use.
3. Choose the base branch, slug, and `freeform` prefix provided by the reference package.
4. See the Work materialize in `in-progress`, with a coherent `worktree/` and `work-state.json`.
5. Confirm that an invalid name or incompatible branch can be corrected without leaving partial state behind.

**Includes:** initial Go/Cobra binary structure; initial `work` home TUI; `work start [source]` with interactive collection or direct source; workspace configuration; minimum install and registration pipeline needed for offline bootstrap; reference package with local Starter and `freeform` convention; Starter invocation via subprocess; direct `repository.path` validation; base/slug/prefix selection; native Git ref validation; branch and worktree creation; atomic snapshot write; initial SQLite projection; shell integration needed to position the terminal.

**Exit criterion:** the journey works on a clean installation and is transactional until materialization. Failures before creation leave no branch, Work directory, snapshot, or orphaned registration behind. The created state can be reopened and validated by integration test.

**Requirements covered:** RF-2 (direct path), RF-4 to RF-7, RF-9, RF-10, RF-24, RF-33, RF-36, RF-37; basis for RF-50 to RF-52 and RNF-1, RNF-3, RNF-5, RNF-6, RNF-7, and RNF-10.

### F2 — Daily cycle: resume and archive

**User outcome:** Works stop being disposable; they can be resumed by recency and archived while preserving context.

**End-to-end demonstration:**

1. Create two Works with F1.
2. Run `work resume`, select the less recent one, and confirm the access order update; repeat with `work resume <work>` without selection.
3. Run `work archive`, select one or more Works, and confirm the operation; repeat with explicit targets without removing the confirmation.
4. See the worktrees removed, the snapshots preserved in `archived`, and the index coherent.
5. Delete or corrupt the test projection and rebuild it from snapshots.

**Includes:** home TUI expansion; listing by `last_accessed_at`; TUI selection and direct targets; safe resolution of the current Work; atomic access update; multi-selection and archive confirmation; worktree removal via Git; movement of the remaining files; reconciliation/rebuild of `work.db`.

**Exit criterion:** create → resume → archive is a repeatable journey, and losing the SQLite projection does not mean losing the canonical state.

**Requirements covered:** RF-11 to RF-15 and RF-34.

**Milestone:** at the end of F2 there is a useful **local alpha** without third-party plugins.

### F2.5 — Coherent terminal experience

**User outcome:** the journeys already delivered operate inline, preserve only compact receipts of accepted choices, present errors and cancellations once, and offer a responsive brand entry point that forwards to full help.

**End-to-end demonstration:**

1. Run `work start`, enter the wrong path and slug repeatedly, and finish by seeing only accepted values in history.
2. Select the base branch, resume, and archive Works without jumps in lines or columns; confirm that expanded controls collapse at the end.
3. Cancel selection and confirmation and observe a single concise result, without mutation and with the exit code preserved.
4. Run `work` in a wide terminal and see the `WORK` wordmark in terminal art with a gradient between the primary and secondary accents; repeat in a narrow terminal or without color and see the compact readable form.
5. Run `work --help` and locate all available commands grouped by context; repeat with redirected streams and confirm flat output and stable contracts.

**Includes:** inline lifecycle with final receipts; recoverable validation in the active control; selectors limited by the viewport and with stable geometry; perceptible focus without relying on color; concise human cancellation and diagnostics; semantic theme and color opt-out; static brand home; help grouped by the real command inventory; generic boundary between journey and presentation; non-interactive compatibility.

**Exit criterion:** the F1/F2 journeys preserve their contracts and mutations, no automation test regresses, active controls fit in the supported viewports, and the terminal history after completion contains only receipts and durable results.

**Requirements covered:** RF-50 to RF-64; RNF-10 and RNF-11.

**Milestone:** at the end of F2.5 there is a **consolidated alpha** with the terminal experience stabilized before the domain expands in F3.

### F3 — Find the local clone

**User outcome:** the absolute path is no longer required; `work start <name-or-reference>` finds a clone in the configured roots according to an explicit policy.

**End-to-end demonstration:**

1. Configure two search roots separate from the workspace.
2. Start a Work using only a repository name.
3. Observe the default filesystem Locator find a single clone and complete the F1 flow.
4. Repeat with two matches and explicitly choose the correct repository.
5. Demonstrate `no result`, `invalid candidate`, and `Locator failure` as distinct outcomes.

**Includes:** transient Repository Reference; Repository Locator contract and invocation; configurable roots; minimum ordered policy; eligibility evaluation by `accepts`; projection of accepted fields only; candidate validation and deduplication; short-circuiting; ambiguity selection; offline filesystem Locator from the reference package; `work repository` TUI hub; minimal non-interactive commands `work repository locator list`, `work repository policy list|add|remove|move|replace`, and `work repository root list|add|remove|replace`.

**Exit criterion:** direct path and policy-based location converge on the same creation journey, without hidden heuristics and without using the Work's own worktrees as an implicit catalog.

**Requirements covered:** RF-39, RF-42 to RF-44, RF-46, and RF-49; first delivery of RF-40 and RF-45; RNF-8 and RNF-9.

**Milestone:** at the end of F3 there is a **local beta** that supports starting work without requiring the user to know the clone path.

### F4 — New origins via plugin

**User outcome:** an origin unknown to the core can be installed and used to create a complete Work, including contribution and fork modes.

**End-to-end demonstration:**

1. Install a local plugin fixture with `work plugin install <path> --link` and a package from a remote repository with `work plugin install <source>`.
2. Run `work start <specific-arg>` and select the Starter when two patterns collide.
3. Resolve the Repository Reference through the F3 pipeline.
4. Choose `contribution` and create a checkout of the resolved branch without slug/convention.
5. Repeat in `fork`, choose a convention/prefix, and create a dedicated branch.
6. List the origin and the fixed reference of the installed plugin.

**Includes:** complete manifest and role-based validation; pinned or linked local install and remote install with fixed reference; alias and collision handling; generated registry; portable runtime/entrypoint; Starter matching and fallback; collision without hidden priority; Starter output protocol; `start_modes`; provided or selected base branch; convention catalog; choice and persistence by repository identity; `work convention` hub and `work convention show|set` commands.

**Exit criterion:** a test plugin written outside the core adds a new origin without changing the binary, and all success paths reach the same worktree and snapshot guarantees from F1.

**Requirements covered:** RF-1, RF-3, RF-6, RF-16, RF-17, RF-21 to RF-23, and RF-25; completes the separation required by RF-2; RNF-2.

**Milestone:** at the end of F4 there is an **extensible beta**, and the main architectural assumption — domain outside the core — is proven by integration.

### F5 — Automatic context on start

**User outcome:** a Work created by an origin can start with links and useful artifacts, without manual steps and without allowing an extension to corrupt the already created Work.

**End-to-end demonstration:**

1. A Starter publishes metadata and a link in the initial snapshot.
2. In `start:finalized`, an eligible Linker discovers another value and the core performs an upsert.
3. An Importer consumes the newly discovered link, writes to staging, and incorporates the artifacts without collision.
4. A second Importer attempts to collide with an existing file and none of its output is incorporated.
5. An automatic extension fails; `work start` ends with a warning, keeping the Work usable.

**Includes:** `meta` and `links` sections; Semantic Conventions and private namespace; `start:finalized` event; filtering by Starter; resolution of required/optional inputs; minimal data projection; Linker → persistence → Importer phases; upsert and provenance; exclusive staging; preflight of all collisions; cleanup; diagnostics and non-destructive failure semantics.

**Exit criterion:** the automatic pipeline is deterministic and tested in both success and failure. Plugins receive only the declared inputs, never write directly to the snapshot, and no Importer can produce partial incorporation.

**Requirements covered:** RF-8, RF-26 to RF-29, RF-32, and RF-35.

### F6 — On-demand context

**User outcome:** after creation, it is possible to inspect relationships and manually add context to the current Work.

**End-to-end demonstration:**

1. Inside a Work, run `work status` and present state and links without changing the snapshot, recent access, or provenance; repeat outside it with an explicit target.
2. Run `work link`, select only eligible manual Linkers, and associate a non-empty value.
3. Run `work import`, select only eligible manual Importers, and incorporate their artifacts through the same F5 staging.
4. Run the three commands outside a Work and receive an actionable error with no state change.

**Includes:** current Work detection and explicit target resolution; presentation of only available and eligible operations; manual association without a subprocess; manual import reusing F5 eligibility/staging; `status` strictly read-only and with `--json`; messages for missing candidates and inputs.

**Exit criterion:** the manual flows reuse the same contracts and guarantees as the automatic flow and do not create a second state or execution model.

**Requirements covered:** RF-30, RF-31, and RF-38.

**Milestone:** at the end of F6, v1 is **feature complete** in the Work and enrichment journeys.

### F7 — Sustainable platform operation

**User outcome:** the ecosystem can evolve without installation, update, disablement, or removal silently changing repository resolution.

**End-to-end demonstration:**

1. Enable and disable a plugin while preserving the unavailable reference of its Locator in the policy.
2. Install a new Locator and confirm that it does not enter the policy automatically.
3. Add, remove, and reorder Locators through commands and the TUI, with the same effective configuration.
4. Run `work plugin update --check` without changing the installation and apply `work plugin update <plugin>` or `work plugin update --all` explicitly after validating the new manifest.
5. Attempt to remove a plugin referenced by the policy and require explicit dependency handling.

**Includes:** `plugin list|install|enable|disable|update|uninstall`; `update --check` validation; consistent uninstall; states for installed, enabled, policy-included, and unavailable Locators; full policy and root operations; parity between TUI and non-interactive commands; interactive confirmation and explicit flags for automation; transactional validation of registry/configuration.

**Exit criterion:** every administrative mutation is explicit, inspectable, and recoverable by reinstalling/configuration; no installation changes precedence and no removal leaves a broken reference silently.

**Requirements covered:** RF-18 to RF-20, RF-40, RF-41, RF-45, RF-47, and RF-48.

**Milestone:** at the end of F7 there is a **v1 release candidate**.

***

## 4. Cross-cutting gates

These items do not form a separate horizontal slice. They enter the definition of done for every slice that touches the corresponding surface:

* **Determinism:** no hidden priority, fallback, or implicit policy mutation.
* **Integrity:** operations that change snapshot, index, filesystem, and Git have failure tests and leave no avoidable partial state.
* **Process contract:** stdin/stdout, exit codes, stderr, and invalid responses have contract tests with self-contained fixtures.
* **Portability:** paths, runtime execution, atomic rename, and shell integration are tested on the officially supported operating systems.
* **Auditability:** important decisions and diagnostics identify the Work, component, and operation without recording secrets or unnecessary content.
* **UX:** every journey is discoverable through the brand entry point and grouped help; hubs show the equivalent direct command; interactive paths are inline, viewport-limited, and keyboard-navigable; commands intended for automation have `--json` on reads, stable output and exit codes, and never try to open a TUI.
* **Regression:** the automated demonstration of each previous slice remains green.

The release candidate also undergoes clean installation, upgrade between supported versions, interruption/cancellation at mutation points, reindexing from snapshots, and tests with real Git repositories containing ambiguous local/remote branches.

***

## 5. Decisions needed before starting F1

Three contracts must be closed in the implementation plan for the first slice:

1. **Shell integration for RF-9 and RF-12.** A child process does not change the parent shell directory. It is necessary to choose and document the contract, for example a shell function, an initialization command that emits `cd`, or a managed subshell.
2. **Matrix of supported operating systems and shells.** RNF-6 requires portability, but the supported set must be explicit to define CI and the behavior of paths, symlinks, and execution.
3. **Self-contained format of the reference package.** RF-10 forbids external dependency on first use; therefore, its executables cannot assume a missing runtime. Packaging must guarantee that property on all supported platforms.

Before F5, there must also be a first published version of the Semantic Conventions used by the plugin fixtures; without that, validation of public keys is underspecified.

***

## 6. What should not be pulled forward into v1

Formal plugin signatures, telemetry, machine-to-machine sync, graphical interface, progress streaming, core-imposed timeout, automatic convention detection, and additional official integrations remain outside these slices. Integration fixtures may simulate GitHub, GitLab, or an issue tracker, but they do not turn an official integration into a dependency for proving the core contracts.

***

## 7. Next step

Plan and implement F2.5 from `specs/003-terminal-ux-revamp/` before starting F3. The expansion to clone location only begins after the interactive and non-interactive contracts of F1/F2 are preserved by the new presentation boundary.
