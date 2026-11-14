# PRD — Work

**A context orchestrator for local agentic development.**

**Status:** Draft v4 — marketplace distribution requirements

**Author:** Gustavo Carvalho

**Date:** 2026-11-07
**Supersedes:** plugin distribution and identity requirements in v3; unrelated requirements remain in force.

**Delivery status:** Target contract for feature 009; marketplace behavior is planned, not implemented. The CLI is unreleased, so breaking development plugins/state is permitted and backward-compatibility scaffolding is out of scope.

***

## 1. Executive summary

Work is a CLI/TUI that orchestrates the start, resumption, and closure of development work units. Each Work gets an isolated Git worktree, a local state snapshot, and its own area for artifacts that plugins can append to. The core knows Git and the Work lifecycle; all origin-specific, integration-specific, or enrichment-specific logic is provided by external plugins.

In addition to resolving a Work's origin, plugins can publish metadata and links, discover external relationships, and import artifacts. Work statically evaluates when each extension may run, provides only the data it declares, and preserves control over Work state, lifecycle, and files.

***

## 2. Problem and motivation

People who work with local AI agents across multiple repositories, branches, and pull requests repeat mechanical work: creating worktrees, finding the correct base, gathering context, and keeping artifacts out of the tracked tree. Flows such as starting a feature, contributing to a PR, and resuming work have different origins, but they all need the same deterministic environment.

Work reduces that friction with a single extensible entry point. A plugin can recognize a PR link, report the base, and publish the corresponding link; other components in the same package can discover complementary relationships or import context without the core needing to know GitHub, GitLab, Jira, or any other specific integration.

***

## 3. Goals

* Prepare a work environment deterministically, without depending on AI inference.
* Isolate every Work in its own Git worktree, without affecting the main checkout.
* Allow plugins to add context and external relationships without controlling the Work lifecycle.
* Keep the core small, auditable, and independent of specific integrations.
* Offer extensibility through installable packages without changing Work's code.
* Make every journey discoverable through a branded entry point and full help, without requiring deep CLI memorization.

***

## 4. Non-goals

* Work is not an AI agent and does not make development decisions.
* Work does not host repositories or replace Git.
* The core does not implement forge, issue tracker, or other external tool-specific integrations.
* Importer staging is not a security sandbox: plugins remain processes run with the user's permissions.
* Work does not impose a single branch convention; the choice remains per repository and up to the user.

***

## 5. Target audience

* Developers who switch between tasks and repositories using local AI agents.
* Maintainers and contributors of open source projects who review or take over third-party PRs.
* Teams that use Git worktrees but currently prepare them manually.
* Authors of plugins for new origins, integrations, and ways to enrich a Work.

***

## 6. Core concepts

| Term | Definition |
| --- | --- |
| **Work** | A unit of work associated with a worktree, branch, dedicated directory, and local state snapshot. |
| **Marketplace** | An explicitly published, named source offering independently selectable plugins; a valid catalog may also be empty. |
| **Catalog snapshot** | A fixed offer obtained from a marketplace, browsable without contacting its source. |
| **Plugin identity** | Plugin name together with marketplace namespace; plugin names may repeat across marketplaces. |
| **Installation provenance** | Actual source, offered entry, revision, and content used; name reuse does not establish continuity. |
| **Plugin** | An installable package that declares zero or more executable components and/or branch conventions. It is installed, updated, enabled, and removed as a unit. |
| **Component** | An executable contribution declared in `components[]`. Its `role` determines the contract, manifest fields, and available operations. |
| **Starter** | A component that interprets the `work start` argument, resolves the Repository Reference, and any other start data it can determine. |
| **Importer** | A component that produces files and directories to be incorporated into the Work. |
| **Linker** | A component responsible for a link key; it can discover its value and/or allow manual association. |
| **Repository Reference** | A transient description of the repository resolved by a Starter: it may contain a local path or data that allows it to be located. |
| **Repository Locator** | A component that attempts to resolve a Repository Reference into one or more local Git repositories, without interpreting the Work origin or defining its lifecycle. |
| **Repository Resolution Policy** | A global ordered sequence of Repository Locators chosen by the user for the current machine. |
| **Repository Search Root** | A configured directory where discovery strategies may look for clones; it is distinct from the Work materialization root. |
| **Branch convention** | A declarative contribution in `conventions[]`, with names and prefixes; it is not an executable component. |
| **Event** | A lifecycle point defined and published by Work, which Importers and Linker discoveries may subscribe to. |
| **`work` state** | Domain state governed and versioned by the core, selectively exposed as `work:<key>`. |
| **Link** | A first-class external relationship between a Work and a resource, identified by a semantic key such as `github.pull_request`. |
| **Metadata** | Extensible information published by components, distinct from Work state and links. |
| **Input** | Data that a component declares it wants to receive, in the form `work:<key>`, `link:<key>`, or `meta:<key>`, optionally with the `:optional` suffix. |
| **Semantic Convention** | A public contract that defines the namespace, key, meaning, and representation of an interoperable `link` or `meta`. |
| **Worktree** | An isolated Git worktree created for the Work. |
| **Slug** | A short, readable identifier used in the branch and Work name. |

***

## 7. Use cases / journeys

### 7.1 Start a new development

#### First use and workspace root

On the first use that requires creating a Work, if no workspace root is configured, Work suggests a default directory and allows the user to choose another directory. The choice is persisted and reused in later commands. The user can change the configured root later.

The user runs `work start [repo name or reference]`. With no argument, the TUI collects the origin; with an argument, the Starter interprets it and returns a Repository Reference. If it already contains a local path, Work validates the repository directly; otherwise, it uses the configured Repository Resolution Policy to locate a local clone. Once exactly one valid repository is resolved, the Work creation flow continues normally. Because the Starter does not return `start_modes`, Work creates a new Work. The user chooses a slug, convention, and branch prefix and receives a ready worktree.

### 7.2 Contribute to or fork a pull request

The user runs `work start <PR link>`. The Starter resolves the repository and base and returns `start_modes: ["contribution", "fork"]`. Work offers those modes. In contribution mode, it checks out the resolved branch without asking for slug or convention; in fork mode, it follows the dedicated branch journey. The Starter may publish the PR link and useful metadata; in `start:finalized`, eligible components can discover links and import context.

### 7.3 Resume and archive work

With `work resume`, the user selects a Work in most-recently-accessed order; `work resume <work>` skips selection. With `work archive`, the user selects active works, confirms, and Work destroys the worktrees and archives the remaining files and state snapshot; explicit targets in `work archive <work...>` skip only selection, not confirmation.

### 7.4 Import context manually

Inside a Work, the user runs `work import`, chooses an available manual Importer, and Work runs it only if all required inputs are present. `work import <importer>` skips selection. The produced files are incorporated after collision validation.

### 7.5 Associate a link manually

Inside a Work, the user runs `work link`, chooses an available manual Linker, and provides a value. In direct use, `work link <linker> <value>` supplies both. Work persists the value under the Linker key.

### 7.6 Inspect the Work

Inside a Work, the user runs `work status` to inspect identity, state, branch, location, and persisted links without running extensions or changing state. Outside a Work, `work status <work>` inspects an explicit target.

### 7.7 Manage plugins, discovery, and conventions

The user registers a marketplace, browses its catalog, and selects one plugin to install as an independent managed copy. Installed plugins and marketplaces are separate management areas. Refreshing an offer does not update installations. Removing a user-managed marketplace removes its plugins after exact impact review. All plugin and marketplace operations have equivalent TUI and direct-command effects and authorization. The plugin manager retains feedback within its session and emits one acknowledgment on normal exit. Repository and convention hubs retain direct-command guidance.

***

## 8. Functional requirements

### `work start [source]`

* **FR-1.** Work must statically select, among enabled Starters, the one whose `pattern` recognizes the argument; multiple matches must be presented to the user, without memorizing the choice.
* **FR-2.** The chosen Starter must receive the argument and return a Repository Reference, along with any other start data it can resolve. The reference may contain the path to a local repository directly or enough identifiers for a later resolution attempt. Before any Work materialization, Work must resolve the reference to exactly one valid and accessible local Git repository.
* **FR-3.** When the Starter response contains `start_modes`, Work must offer exactly the returned modes. The absence of this field means new Work creation. In contribution mode, Work checks out the resolved branch and does not run the slug, convention, and prefix steps; in fork mode, the flow follows the new Work path.
* **FR-4.** Outside contribution mode, Work must prompt for a slug.
* **FR-5.** Outside contribution mode, Work must present the resolved branch convention prefixes for selection.
* **FR-6.** If the Starter does not provide a base branch, Work must allow selecting a remote or local branch.
* **FR-7.** Work must create the worktree, its directory, and `work-state.json`; plugins may add other files and directories, which are not structure created or interpreted by the core.
* **FR-8.** After materializing the structure and core state, Work must publish `start:finalized`, execute subscribed and eligible Linkers and Importers in the order defined by the architecture, and indicate to the user which extensions are running.
* **FR-9.** When complete, the command must move the terminal to the directory of the newly created worktree.
* **FR-10.** On first required setup, Work must make available, without network access or manual installation, the components and minimum configuration needed to start a Work from a locally discoverable repository using the default strategy. Later setup must preserve disablement, deliberate removal, and user-edited or empty policy.
* **FR-23.** On first use of a repository outside contribution mode, if more than one convention is enabled, the user must choose one and the choice must be remembered for that repository.
* **FR-24.** On first use, Work must provide at least one branch convention by default.
* **FR-36.** When no workspace root is configured, Work must prompt for it before the first Work creation, offer a suggested default value, persist the choice, and allow it to be changed later.
* **FR-37.** Outside contribution mode, before creating a branch or worktree, Work must ensure that the resulting branch name is valid for Git and does not collide with an existing branch incompatible with the new creation. If the name is rejected, the user must be able to correct the choice that produced it and try again.

### Work extensions

* **FR-26.** Before executing an Importer operation or Linker discovery, Work must statically evaluate enablement, activation mode, event subscription, Starter filter, and required inputs; an ineligible component must not be executed.
* **FR-27.** Work must resolve inputs in the `work`, `meta`, and `link` namespaces and project to the subprocess only the inputs declared by the component; missing optional inputs must be omitted.
* **FR-28.** For each Importer execution, Work must provide a unique temporary directory, validate all collisions before changing the Work, and never silently overwrite files.
* **FR-29.** An automatic Linker or Importer failure after Work creation must complete `work start` with a warning and diagnostic, without rolling back the Work. A successful discovery with no value is not an error.
* **FR-30.** Inside a Work, `work import` must offer Importers that declare `manual` and are eligible, and run the selected `import` operation; an explicit Importer must skip selection without skipping eligibility validation.
* **FR-31.** Inside a Work, `work link` must offer Linkers that declare `manual`, prompt for a non-empty value, and persist that value under the Linker key; an explicit Linker and value must allow the same operation without the TUI.
* **FR-32.** Work state, metadata, and links must be persisted in semantically distinct sections in the same canonical snapshot. Each link key keeps a single value; Starter publication, discovery, and manual association perform upserts, and the last source wins.
* **FR-33.** The `work-state.json` snapshot must be updated atomically, versioned by schema, and controlled exclusively by the core; plugins interact with state only through public inputs and outputs.
* **FR-34.** Work must keep `work.db` as a queryable global projection and be able to rebuild or reconcile it from Work snapshots.
* **FR-35.** Public metadata and link keys must follow Semantic Conventions; private keys must use an explicit provider namespace that distinguishes plugins from different marketplaces. Public key identity must not include its producer. Removing and re-registering a marketplace must not transfer ownership of its retained private values to a replacement source.
* **FR-38.** `work status [work]` must present identity, state, branch, applicable location, and links currently persisted in the canonical snapshot. With no target, it must use the Work associated with the current directory. The operation is read-only, does not run extensions, and does not modify the Work or its recent access.
* **FR-39.** Work must allow plugins to provide Repository Locators independently of Starters. A Starter should not need to know which Repository Locators are installed or configured on the machine.
* **FR-40.** The user must be able to keep multiple Repository Locators installed and enabled and explicitly define which of them participate in the Repository Resolution Policy and in what order.
* **FR-41.** Installing or enabling a plugin must not silently insert its Repository Locators into the existing Repository Resolution Policy. In interactive flow, Work may offer the user immediate configuration of the new Locators.
* **FR-42.** During resolution, Work must consider the Repository Locators in the policy order. A Locator with no result allows resolution to continue to the next eligible Locator. A single valid result ends resolution. Multiple valid results must be presented to the user for explicit choice.
* **FR-43.** An operational failure of a Repository Locator is distinct from no result. In v1, failure of a Locator explicitly selected in the Repository Resolution Policy must stop resolution with a diagnostic, without silent fallback to the next Locator.
* **FR-44.** Work must evaluate a Repository Locator's eligibility before executing it, using only the statically declared information from the component and the identifiers present in the Repository Reference.
* **FR-45.** The user must be able to inspect and manage the Repository Resolution Policy through the TUI and equivalent non-interactive commands, including adding, removing, and reordering Locators. They must also be able to list installed Locators and manage repository search roots. Persisted configuration must remain inspectable and usable by automation.
* **FR-46.** Work must allow repository search roots to be configured independently from the root where Works and their worktrees are materialized, and project them explicitly to the Repository Locators that are executed.
* **FR-47.** When a plugin is disabled, its Repository Locators must remain referenced in the Repository Resolution Policy, but be unavailable while the plugin is disabled. The interface must make this state visible.
* **FR-48.** Removing plugins, directly or through marketplace removal, must identify all affected Repository Resolution Policy references and require explicit authorization to remove them in the same consistent change. Operation confirmation alone does not authorize dependency removal.
* **FR-49.** The default locator strategy must be able to discover Git repositories in configured search roots without relying on external services. When more than one repository matches the provided reference, the user must be able to explicitly select the desired one.

### `work resume` and `work archive`

* **FR-11.** `work resume` must list existing works from most recently accessed to oldest and open the selected one; with an explicit target, it must open it directly.
* **FR-12.** Selection in `work resume` must update recent access and move the terminal to the worktree.
* **FR-13.** `work archive` must list active works with multi-selection; with explicit targets, it must skip selection and still require the applicable confirmation.
* **FR-14.** After confirmation, it must destroy the selected worktrees and move the remaining files to the archived area.
* **FR-15.** Persisted state must reflect archiving.

### Plugin and convention management

* **FR-16.** Every new plugin installation, reinstallation, or content replacement must select an explicitly published marketplace entry and produce an independent managed copy. Standalone installation and new linked installations are not supported.
* **FR-17.** The user must be able to inspect installed marketplace plugins, their qualified identities, enablement, versions, and safe provenance.
* **FR-18.** The user must be able to enable or disable a plugin without uninstalling it.
* **FR-19.** The user must be able to check and explicitly apply updates to selected plugins or all eligible plugins, applying the exact reviewed candidate. Versions are opaque; only a different version offers an update. Ordinary Work commands never check updates automatically.
* **FR-20.** The user must be able to remove selected installed plugins without removing their marketplace or siblings, preserving existing Works and remembered convention choices.
* **FR-21.** Marketplace names must be unique among registered sources and plugin names unique within a marketplace. Conflicting marketplace provenance must fail without overwrite or silent rename; homonymous plugins from distinct marketplaces must be independently usable.
* **FR-25.** The user must be able to inspect and change the remembered branch convention from any clone of the repository.

### Marketplace distribution

* **FR-65. Independent selection.** A marketplace may offer one or many independent plugins; selecting one installs no siblings.
* **FR-66. Mandatory offer.** An origin without an explicit valid marketplace catalog must not be installed or implicitly converted to a marketplace.
* **FR-67. Source lifecycle.** Users must be able to register, inspect, explicitly refresh, and remove user-managed marketplaces without silently replacing a conflicting source.
* **FR-68. Scoped names.** Homonymous plugins from different marketplaces must coexist and be executed and maintained independently, subject to genuine exclusive-contribution constraints.
* **FR-69. Explicit selection.** Installation and maintenance must explicitly identify plugin and marketplace; no global implicit name lookup or plugin alias is offered.
* **FR-70. Independent copies.** Changing or losing the original source must not alter installed bytes or prevent execution of a managed copy.
* **FR-71. Independent discovery.** An obtained offer must be browsable offline without state changes; refreshing it must not change installed content, enablement, or policy.
* **FR-72. Provenance continuity.** Installations must retain fixed content and actual provenance sufficient for independent execution and maintenance from the recorded source, without depending on the discovery snapshot or redirecting by name reuse.
* **FR-73. Explicit maintenance.** Updates must preserve opaque version semantics and apply precisely the reviewed candidate to the selected plugin; sibling changes must not update it.
* **FR-74. Withdrawal resilience.** Withdrawing an offered entry must not remove or disable its installed copy. Failed maintenance must preserve the previous copy.
* **FR-75. Whole-source removal.** Removing a user-managed marketplace must remove its registration, offer, and all installed plugins, including disabled and withdrawn entries, as one recoverable publication after exact impact review and dependency authorization.
* **FR-76. Surface parity.** Every plugin and marketplace administrative capability must be available in CLI and TUI, including filtering, multiple installed targets, check without apply, update all, and refresh all catalogs. Reads are pure; explicit checks may retrieve temporary candidates without persisting state.
* **FR-77. Recoverable administration.** Administrative publication must be coherent and recoverable. Cancellation or failure before commitment preserves earlier state; failure after commitment reports applied change rather than claiming rollback.
* **FR-78. Included offer.** The distribution-provided marketplace remains available offline while Work is installed. Its plugins use the same selection, copy, enablement, and individual removal rules. Initial setup installs only an entry from this offer and honors durable removal and one-time policy initialization.
* **FR-79. Clean pre-release cutover.** Work has not been released. This feature must define one purposeful marketplace-only contract and remove superseded installation, identity, state-format, and plugin paths. Backward compatibility with earlier development plugins, state, or CLI behavior is not required; no adapters, dual readers, migration commands, or adoption flows are included.
* **FR-80. Official online offer.** The project must offer an official online marketplace with publications independent of Work releases. Explicit registration, refresh, and removal, including removal of installed plugins, follow ordinary user-managed source rules.
* **FR-81. Display names.** A marketplace may publish an optional display name independent of identity. Absence uses its namespace; equal display names remain distinguishable; changing it changes no identity or reference.
* **FR-82. Local totals.** Marketplace inspection must show nonnegative installed and enabled totals from local inventory, including withdrawn entries, even with an unavailable catalog. Enabled never exceeds installed; totals do not depend on search filters and refresh after mutations.
* **FR-83. Presentation continuity.** New screens must preserve the existing visual hierarchy, semantic colors, spacing, expressive focus and selection, and recognizable destructive actions. Focus, selection, state, and actions remain distinguishable without color and in reduced terminals.

FR-65 through FR-83 correspond, in order, to proposal MKT-01 through MKT-19. A marketplace organizes distribution, not execution trust. Central aggregation, automatic discovery, inter-plugin dependency installation, whole-catalog installation, signing, payment, and protection against same-user file modification remain out of scope.

### Related marketplace decisions

ADR-0022 through ADR-0030 record catalog, snapshot, identity, commands, removal, administrative publication, included offer, and official online offer decisions. They replace earlier decisions only in their declared scopes; implementation is tracked in feature 009.

### Extensibility

* **FR-22.** Any behavior beyond orchestrating worktrees via Git must be delegated to external plugins, discovered and installed by the user.

### CLI/TUI surface

* **FR-50.** In an interactive terminal, `work` with no arguments must show a static branded entry with guidance to `work --help`, without opening a selector. `work --help` must show usage, options, and exactly the available public commands, grouped by context. `work plugin`, `work repository`, and `work convention` must continue opening the hubs for their respective domains.
* **FR-51.** In an interactive terminal, omitted selection values must be collected by the TUI and explicit values must skip only the corresponding selection. In non-interactive stdin, a missing required value must fail with actionable usage without attempting to open the TUI.
* **FR-52.** Read commands must accept `--json` and not alter state; mutations must have stable output and exit codes. `--yes` may confirm already-determined impacts, but not choose targets or values.
* **FR-53.** Interactive collection and selection must run in one full-screen terminal session per journey and restore the primary buffer once. Interview journeys retain compact accepted-step receipts; the persistent plugin manager replaces feedback within its session and prints only its normal-exit acknowledgment. Sensitive values must be redacted or omitted.
* **FR-54.** A recoverable failure attributable to the active field must appear inside that field and be replaced by the next attempt. At most one current error may remain visible, and rejected attempts must not accumulate in the terminal history.
* **FR-55.** Active controls must respect the available terminal dimensions. Moving focus, checking options, filtering, or switching groups must not shift columns or change the height of unchanged lines.
* **FR-56.** Selector focus must be identifiable by bold text and by a reserved-width textual indicator; color may reinforce it, but cannot be the only signal. Single and multi selectors must offer consistent movement keys, filtering where available, confirmation, and cancellation, with visible help.
* **FR-57.** Cancellation must preserve the contracted category and exit code and never confirm a mutation. Interview cancellation leaves one concise result; Back or cancellation of a plugin-manager action returns within the manager without accumulating cancellation receipts.
* **FR-58.** Non-recoverable failures must be shown once, in user language, with a next step when known. Technical causes must remain available for explicit diagnosis, without being part of the normal human-facing output.
* **FR-59.** The primary brand must render `WORK` as terminal art and apply, when true color and sufficient contrast are available, a gradient between primary `#11A8CD` and secondary `#8B7CF6`. Narrow or colorless terminals must receive the compact `WORK` form without broken wrapping.
* **FR-60.** Presentation must use a single semantic theme: brand colors do not replace the meanings of success, warning, or failure; body text uses the terminal's normal color; light backgrounds, limited palettes, and monochrome mode receive readable alternatives.
* **FR-61.** Color must be disabled in non-interactive output when `NO_COLOR` is not empty or when `TERM=dumb`. Output must remain understandable without color and must not contain control sequences in those cases.
* **FR-62.** Interactive presentation and human diagnostics must use the configured interface channel; stable command results remain on stdout. Visual refresh must not change existing arguments, flags, error tokens, or exit codes without an explicit later contract.
* **FR-63.** `work` with no arguments must exit successfully after the brand entry in an interactive terminal. In non-interactive use, it must preserve the existing usage failure; `work --help` must exit successfully in both modes.
* **FR-64.** Confirmations preceding mutations must show the exact impact. Interview confirmations reduce to compact receipts; plugin-manager confirmations allow Back before commitment and show results within the same session. Authorization never expands to newly discovered targets or dependencies.

***

## 9. Non-functional requirements

* **NFR-1. Radical core simplicity.** Work remains small and auditable; domain logic lives in plugins.
* **NFR-2. Frictionless extensibility.** A new plugin does not require changes to Work's code.
* **NFR-3. Determinism.** Preparation and eligibility are predictable and do not depend on AI.
* **NFR-4. Zero AI reasoning cost.** No CLI journey requires an LLM.
* **NFR-5. State auditability.** The canonical local snapshot preserves Work, metadata, links, and artifacts traceably; the global index can be rebuilt from it.
* **NFR-6. Portability.** Behavior is consistent across supported operating systems.
* **NFR-7. Minimal necessary trust.** The plugin origin is explicitly chosen by the user and the core never loads plugin code into its own process.
* **NFR-8. Extension composability.** Starters and repository location mechanisms must be able to evolve independently. Creating a new Starter must not require a custom implementation of the local location strategies already available to the user.
* **NFR-9. Explicit local policy.** Precedence between location mechanisms belongs to user configuration and must not depend on installation order, plugin-declared priority, or hidden core heuristics.
* **NFR-10. Progressive and consistent CLI.** Every journey must be discoverable from `work` and `work --help`; everyday commands remain shallow, while direct automation commands follow a regular grammar, have stable output and exit codes, and never depend on the TUI.
* **NFR-11. Accessible and portable presentation.** Interactions must remain understandable without color, adapt to available space, and preserve stable geometry across supported operating systems and terminals.

***

## 10. User experience

In an interactive terminal, `work` with no arguments shows a static entry: the `WORK` wordmark in terminal art, colored with a gradient between the primary and secondary accents when supported, followed by the tagline and the `work --help` hint. Help groups only the commands that are actually available by usage context and keeps all public journeys discoverable. The empty command does not open a selector. In non-interactive mode, it preserves the existing usage failure; `work --help` works in both modes.

The everyday commands are `work start [source]`, `work resume [work]`, `work archive [work...]`, `work status [work]`, `work import [importer]`, and `work link [linker] [value]`. Missing targets use the TUI when the operation requires a choice; an explicit target skips that selection. In non-interactive stdin, a missing required value fails with actionable usage instead of trying to open the TUI.

In the `work start` flow, Work resolves and presents the needed choices, creates the Work, materializes its canonical state from the Starter response, and executes post-creation extensions. While Linkers and Importers run, Work shows which extension is currently running; automatic failures are reported as warnings with diagnostics, because Work is already available.

All choices — competing Starters, start mode, convention, prefix, base branch, resumption, archiving, manual Importer, and manual Linker — use consistent, full-screen, keyboard-navigable controls. While active, recoverable errors are replaced in the field itself and selectors respect the viewport without geometry jumps; when accepted, they leave compact receipts with the chosen values. Cancellations and final failures appear once, without exposing technical chains in the normal human-facing output. `work status` uses the current Work when no explicit Work is provided; `work import` and `work link` always operate on the current Work. When that context cannot be resolved, they fail with a clear message.

`work plugin`, `work repository`, and `work convention` are TUI hubs. In the direct API, the verb comes after a singular resource path: `plugin add|list|enable|disable|update|remove` and `plugin marketplace add|list|upgrade|remove`, `repository policy list|add|remove|move|replace`, `repository locator list`, `repository root list|add|remove|replace`, and `convention show|set`. Removing a Locator from the policy only stops using it in the strategy; disabling remains a plugin-only operation. The marketplace surface is governed by ADR-0025; ADR-0019 continues to govern unchanged discovery and command semantics; the separation between journeys and interactive presentation is governed by ADR-0020, and the full-screen rendering structure by ADR-0021.

Read commands accept `--json` and do not alter recent access, configuration, checkout, or provenance. `--yes` confirms already-determined impacts, but never chooses a target, mode, or value. There are no command, resource, or global plugin aliases. Marketplace namespaces remain visible independently of display names. The distribution-provided and official online offers have distinct identities and lifecycles.

***

## 11. Success metrics

* Time between `work start <arg>` and a ready environment, including post-creation extensions.
* Number of community-created plugins.
* Reduction in accidentally committed AI/spec artifacts.
* Recurring use of `work resume`.
* Rate of extensions that become eligible and complete without manual intervention.
* Completion rate of interactive journeys without residue from rejected attempts, old errors, or expanded selectors in terminal history.
* Time for a person to find, through the branded entry point and help, the right command for a public journey.

***

## 12. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Malicious or faulty plugin | Explicit origin, external processes, and no implied additional permissions. |
| Starter ambiguity | Explicit choice on every collision, with no hidden priority. |
| Destructive import | Staging and detection of all collisions before incorporation. |
| Extension without enough data | Static eligibility; the component is not executed just to discover missing input. |
| Context loss during archiving | Archiving preserves metadata, links, and artifacts; only the worktree is destroyed. |
| Plugin installation unexpectedly changes how repositories are found | New Locators do not automatically enter the existing policy. |
| Two mechanisms locate different repositories for the same reference | The policy is ordered and uses short-circuiting; there is no implicit arbitration between results from different Locators. |
| Configured Locator becomes unavailable | The state is exposed explicitly; operational failure is not treated as a missing match. |
| Default scanner finds worktrees created by Work itself | Repository search roots are configured separately from the Work materialization root. |

***

## 13. Roadmap / out of scope for v1

The v1 sequencing in vertical slices, with a demo and exit criteria per delivery, is in [`docs/roadmap.md`](roadmap.md).

**Out of scope for v1:** integration plugins beyond the minimum reference set; multiple simultaneous conventions per repository; synchronization between machines; a graphical interface; a security sandbox for plugins.

**Future candidates:** optional local telemetry; official forge integrations; an executable component to automatically detect branch convention; new operations and lifecycle events defined by the core.
