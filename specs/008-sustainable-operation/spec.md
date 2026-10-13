# Feature Specification: Sustainable Platform Operation (F7)

**Feature Branch**: `feature/sustainable-operation`

**Created**: 2026-10-12

**Status**: Draft

**Input**: User description: "next roadmap milestone M7" — interpreted as F7, Sustainable Platform Operation, in `docs/roadmap.md`, the cumulative v1 release-candidate milestone after F6.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect and Control Installed Plugins (Priority: P1)

A developer inspects every installed package and temporarily disables or reenables it. They can distinguish installed, enabled, policy-included, and unavailable Locators. Disablement preserves configuration so reactivation restores participation at the same position.

**Why this priority**: Safe maintenance requires visible state and reversible control over execution without accidentally changing repository preference.

**Independent Test**: Install two fixture packages, include their Locators in a chosen order, disable one, inspect the inventories and policy, then reenable it. Verify only enabled providers execute and the policy never changes.

**Acceptance Scenarios**:

1. **Given** enabled and disabled installed packages, **When** `work plugin list` or its `--json` form runs, **Then** both appear with aliases, origins, installed references, declared versions, enablement, components, and conventions; inspection changes no persisted state.
2. **Given** a plugin with a policy-included Locator, **When** `work plugin disable <PLUGIN>` runs, **Then** its executable components and solely provided conventions become unavailable while its installation, policy references and positions, remembered repository choices, and existing Works remain unchanged.
3. **Given** that disabled plugin, **When** `work plugin enable <PLUGIN>` runs, **Then** valid declarations become available again and its Locator resumes participation at the retained position without a policy rewrite.
4. **Given** an enabled plugin whose Locator is outside the policy, **When** it is disabled and reenabled, **Then** the Locator stays outside the policy and never executes during policy-based resolution.
5. **Given** several requested aliases including an unknown alias, **When** enable or disable is requested, **Then** the complete request fails before changing any package; repeating a valid already-satisfied request succeeds without changing unrelated state.

---

### User Story 2 - Manage Repository Resolution Explicitly (Priority: P1)

A developer edits the ordered Locator strategy and search roots through direct commands or the repository hub. Installing or enabling a Locator never gives it implicit precedence. Unavailable references remain visible and manageable.

**Why this priority**: Repository selection must remain a deliberate choice as the installed ecosystem grows or becomes temporarily unavailable.

**Independent Test**: Install another Locator, verify the policy is unchanged, explicitly add and reorder it, and demonstrate the resulting resolution order. Repeat configuration through the hub and compare effective policy and roots.

**Acceptance Scenarios**:

1. **Given** an existing policy, **When** a package is installed, reinstalled, enabled, or updated to add a Locator, **Then** the policy stays unchanged until a separate explicit policy action includes it.
2. **Given** installed Locators, a disabled provider, and a stale policy reference, **When** Locators and policy are listed, **Then** installation, enablement, membership, and order are distinguishable, with an explanation for every unavailable policy entry.
3. **Given** valid Locator references, **When** the user adds, removes, moves, or replaces entries, **Then** the resulting sequence matches the requested order; removal from policy does not disable or uninstall the provider.
4. **Given** an unavailable entry already in policy, **When** it is moved or removed, **Then** the operation succeeds without invoking its provider; new entries require installed Locators, and replacement may explicitly retain existing unavailable references.
5. **Given** roots separate from the Work workspace, **When** roots are listed, added, removed, or replaced, **Then** direct commands and the hub produce the same validated, deduplicated roots without changing the workspace root or existing Works.
6. **Given** invalid positioning, duplicates in a policy replacement, an unknown new Locator, or an invalid root, **When** the change is submitted, **Then** the entire request fails without altering the previous collection.

---

### User Story 3 - Check and Apply Updates Deliberately (Priority: P1)

A developer checks for updates without disturbing installed packages, reviews candidate versions and effects, and explicitly updates named packages or all eligible packages. Invalid candidates never replace usable installations.

**Why this priority**: Explicit, validated updates let users maintain integrations while keeping daily Work behavior reproducible.

**Independent Test**: Install a pinned fixture, publish another version at its recorded source, check repeatedly, and compare installed content and configuration before and after checks. Apply the update and verify new declarations, a fixed reference, retained enablement, and unchanged policy order. Repeat with invalid content and a withdrawn policy-referenced Locator.

**Acceptance Scenarios**:

1. **Given** installed packages and newer source content, **When** `work plugin update --check [PLUGIN...]` runs, **Then** named packages, or all installed packages when unnamed, each receive an outcome: unchanged, update available, source-managed/not applicable, or failed. Available current/candidate versions and references and validation failures are visible; installed content and persisted application state remain unchanged.
2. **Given** different source content with the same declared version, **When** updates are checked or applied, **Then** a changed content reference alone does not constitute a release; versions are opaque values without assumed ordering.
3. **Given** a valid candidate with a different version, **When** its update is explicitly applied and confirmed, **Then** the exact reviewed candidate replaces the installation, obsolete declarations disappear, new declarations register, and alias, source identity, enablement, and unrelated configuration are retained.
4. **Given** an invalid manifest, unavailable required runtime, inaccessible source, or incompatible declaration, **When** update is checked or applied, **Then** the affected package reports the reason and stays usable at its previous reference.
5. **Given** an update that removes or changes the role of a policy-referenced Locator, **When** application is requested, **Then** Work identifies affected references and requires explicit approval of their removal before changing package and policy together; refusal changes neither.
6. **Given** multiple named targets or `--all`, **When** the resolved update set is confirmed, **Then** each target receives an outcome, independent successful updates remain applied, failures do not silently suppress remaining targets, and any failed target makes the overall command unsuccessful.
7. **Given** untargeted `work plugin update`, **When** used interactively, **Then** it offers selection and impact review; without interactive input it fails with actionable usage. Combining `--check` with `--all`, or explicit targets with `--all`, fails before mutation.
8. **Given** ordinary start, resume, archive, status, link, or import use, **When** it runs, **Then** no plugin update check or source update occurs.
9. **Given** interruption during package replacement, **When** administration resumes, **Then** the installation and registrations describe one consistent approved revision, or Work identifies unresolved recovery and refuses to present the mixed installation as usable.

---

### User Story 4 - Uninstall with Explicit Dependency Handling (Priority: P1)

A developer removes a package after reviewing which policy entries will disappear. Removal preserves existing Works and developer-owned linked sources. The bundled reference package can be removed durably like another package.

**Why this priority**: Maintenance requires complete, understandable removal without silently broken dependencies or resurrected defaults.

**Independent Test**: Remove a plugin whose Locators occupy several policy positions, first refusing then approving dependency removal. Verify installation, registrations, and references change together. Repeat with linked sources and the bundled package, then run later commands to verify source preservation and durable removal.

**Acceptance Scenarios**:

1. **Given** a policy-referenced plugin, **When** interactive uninstall is requested, **Then** Work lists the package and affected references and confirms their removal together; cancellation leaves everything unchanged.
2. **Given** the same dependency non-interactively, **When** uninstall lacks explicit dependency-removal authorization, **Then** it fails without changes even with `--yes`. With explicit dependency handling and required confirmation, only the determined package and references are removed.
3. **Given** a linked installation, **When** uninstall succeeds, **Then** Work removes its own installation and registrations without deleting or modifying the external source directory.
4. **Given** existing Works and remembered conventions from the package, **When** it is removed, **Then** snapshots, links, artifacts, branches, and checkouts remain unchanged; remembered choices remain inspectable and indicate unavailability if no enabled provider remains.
5. **Given** the bundled reference package was deliberately disabled or uninstalled, **When** later commands require plugin state, **Then** bootstrap respects that decision, does not restore the package or policy membership, and explains missing capabilities. Explicit reinstallation restores availability without automatically restoring policy membership.
6. **Given** a failed or cancelled removal before commitment, **When** the command terminates normally, **Then** the package and policy retain their previous usable state; failed recovery is reported with affected state and a concrete recovery action.
7. **Given** several uninstall targets including an unknown alias or a recoverable failure while removing the selected set, **When** the request fails, **Then** no selected package or policy reference is left partially removed.
8. **Given** two administrative operations affecting shared configuration, **When** their execution overlaps, **Then** they commit against current state or report a conflict without overwriting unrelated changes; inspections after commitment show matching installation, declarations, enablement, and policy.

---

### User Story 5 - Complete Administration Through Discoverable Hubs (Priority: P2)

A developer manages plugins, repository resolution, and repository conventions with the keyboard. Every hub shows equivalent direct commands, current state, and pending impact. Automation achieves the same results without interactive controls.

**Why this priority**: Hubs complete the promised administrative surface; direct commands independently deliver and verify the underlying operations.

**Independent Test**: Perform enablement, a checked update, policy reordering, root replacement, and a convention change through the three hubs and compare each with the direct equivalent. Repeat cancellations and incomplete non-interactive requests.

**Acceptance Scenarios**:

1. **Given** an interactive terminal, **When** `work plugin`, `work repository`, or `work convention` runs, **Then** the hub presents current state and applicable actions with equivalent direct-command guidance and supports complete keyboard operation.
2. **Given** a remembered convention with no enabled provider, **When** the choice is inspected, **Then** its stored value and unavailability are visible without replacement; creation requiring that convention requests an explicit available choice or fails with guidance non-interactively.
3. **Given** an available replacement, **When** the choice is explicitly changed through the hub or `work convention set <CONVENTION>`, **Then** it is remembered across clones of that repository without rewriting existing Works.
4. **Given** cancelled or rejected interactive input, **When** the journey exits, **Then** no unconfirmed change occurs, the terminal is restored, and the result appears once without rejected values or expanded controls in history.
5. **Given** redirected streams or missing required automation values, **When** a hub or incomplete mutation runs, **Then** no interactive controls start and actionable usage explains the direct form. Fully specified commands work without a terminal.
6. **Given** administrative success, cancellation, or failure with a credential-bearing source, **When** results are presented, **Then** stable results and exit outcomes remain distinguishable from interface output, failures appear once, and normal output does not reveal credentials.

### Edge Cases

- Empty inventories, policies, and root sets remain inspectable; explicit removals can empty collections without a new clearing command.
- Unknown aliases in multi-target requests fail before execution; repeated aliases are handled once, and package aliases never change implicitly.
- Multiple packages may provide one convention name; disabling/removing one leaves it available while another enabled provider remains, under existing catalog rules.
- Updates may drop, rename, or change the role of Locators. Affected policy references require the same dependency handling as uninstall; unaffected references retain relative order.
- Disabled packages can be checked, updated, or removed and remain disabled after update/reinstall unless explicitly enabled.
- Linked sources may change externally or disappear. Inspection reports source problems without silently refreshing registrations or claiming to pin or undo developer edits.
- A source can change after check/review; application uses the reviewed fixed candidate or requires another review, never substitutes a different revision silently.
- Source failures, interruption during replacement, and concurrent administration must not leave content, registrations, enablement, and policy describing different committed states.
- Reads on a fresh home must not bootstrap defaults or create configuration, registry, index, or lock files. Temporary retrieval for explicit checks is removed on normal termination.
- Deliberate bundled-package removal differs from accidental installation damage. Repair may recover damage but must honor user removal and policy edits.
- Missing Starters, Locators, or conventions produce install/enable/configure guidance without silently changing choices.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Work MUST expose `work plugin list|install|enable|disable|update|uninstall`, using installed aliases as management targets and retaining existing installation and alias-conflict behavior.
- **FR-002**: Plugin inventory MUST show enabled and disabled installations, origins, installed references or linked locations, declared versions, component roles, and conventions. Legacy installations lacking newly exposed information MUST remain inspectable without a write.
- **FR-003**: Enablement MUST apply to the whole package across Starters, Locators, Linkers, Importers, and conventions, with existing installations enabled unless explicitly disabled.
- **FR-004**: Disablement MUST retain installation, policy references/order, remembered choices, and existing Works. Enablement MUST restore availability without inserting Locators into policy.
- **FR-005**: Enable/disable MUST validate all aliases before committing, handle repeated targets once, and succeed idempotently when already satisfied.
- **FR-006**: Locator inventory MUST distinguish installed, enabled, policy-included, and available states. Policy views MUST preserve traversal order and explain unavailability, including missing declarations and changed roles.
- **FR-007**: Install, reinstall, enable, and update MUST NOT add or reorder policy references implicitly. Initial offline bootstrap remains the sole default-policy seeding exception, only for genuinely new setup.
- **FR-008**: Policy `list|add|remove|move|replace` MUST retain existing positioning syntax, reject duplicates and invalid positioning, and require installed Locators for new references. Existing unavailable references MUST remain inspectable, removable, repositionable, and explicitly retainable in replacement.
- **FR-009**: Root `list|add|remove|replace` MUST preserve existing validation and path deduplication independently of the workspace root. Rejected policy/root changes MUST preserve the prior collection in full.
- **FR-010**: Resolution MUST use available, eligible Locators in declared order and preserve distinctions between ineligibility, no result, ambiguity, invalid candidates, and operational failure, without hidden fallback or individual Locator enablement.
- **FR-011**: `work plugin update --check [PLUGIN...]` MUST inspect named packages, or all installations when unnamed, report every outcome, and leave installed content and persisted application state unchanged. Temporary retrieval MUST be cleaned on normal termination.
- **FR-012**: Update availability MUST depend on a different opaque manifest version at the recorded source, without version ordering or commit-count heuristics. Results MUST identify available current/candidate versions and references and distinguish invalid candidates, source failures, and source-managed/not-applicable origins.
- **FR-013**: Work MUST support named updates, `update --all`, and interactive selection for untargeted `update`. `--check` with `--all`, and targets with `--all`, MUST fail before mutation. `--yes` MUST NOT choose targets.
- **FR-014**: Before commitment, update MUST validate the candidate using the same applicable package, manifest, declaration, and runtime rules as install, without executing plugin components to validate or preview it.
- **FR-015**: Update review MUST identify targets, old/candidate versions and references, added/withdrawn declarations, and affected policy dependencies. Application MUST use the exact reviewed candidate and retain alias, source identity, enablement, and unrelated settings.
- **FR-016**: Update MUST replace package content and registrations consistently, withdraw obsolete declarations, retain declarations still provided by other packages under existing catalog rules, and leave the prior installation usable on failure before commitment.
- **FR-017**: An update withdrawing a policy-referenced Locator MUST require explicit dependency handling and remove approved references in the same consistent change. Unaffected references MUST retain relative order.
- **FR-018**: Multi-package update MUST report each outcome with per-package commitment: independent successes remain applied, failures do not silently suppress remaining targets, and any failed target causes overall failure. Unknown explicit aliases MUST fail before applying any target.
- **FR-019**: Ordinary Work journeys and administrative reads other than explicit update operations MUST NOT check for or apply plugin source updates.
- **FR-020**: Uninstall MUST remove selected managed installations and registrations without deleting linked sources, other packages, or existing Work content/history.
- **FR-021**: Uninstall MUST identify all affected policy references, display their removal impact, and remove approved references together with their packages. Refusal or missing dependency authorization MUST leave both unchanged.
- **FR-022**: Non-interactive update/uninstall removing policy dependencies MUST require an explicit dependency-removal option alongside applicable confirmation. `--yes` alone MUST NOT authorize unspecified dependency changes; interactive review MUST name affected references before acceptance.
- **FR-023**: Multi-target uninstall MUST validate and review the complete removal set before committing it as one consistent change. Unknown targets, refusal, or recoverable failure before commitment MUST preserve the whole requested set and policy.
- **FR-024**: Deliberate bundled-package disablement/removal MUST survive later commands and supported application upgrades. Bootstrap/repair MUST NOT reverse it or restore removed policy entries; explicit offline reinstallation MUST be available without automatic policy reinsertion.
- **FR-025**: Maintenance MUST preserve remembered repository conventions. Inspection MUST show unavailable choices without replacing them; a journey needing an unavailable choice MUST request an explicit available replacement or fail with guidance. Another enabled provider keeps a shared name available under existing catalog semantics.
- **FR-026**: `work convention show|set` and its hub MUST operate on the same remembered choice from any clone of that repository. Explicit change MUST affect future choices without rewriting existing snapshots or branches.
- **FR-027**: Plugin, repository, and convention hubs MUST expose the same applicable operations, validation, dependency handling, and resulting state as direct commands and show equivalent direct-command guidance.
- **FR-028**: All administrative reads, including update checks, MUST accept `--json` and be pure: no bootstrap, repair, recency changes, persistent cache writes, or application-state creation. Human/structured views MUST agree; source retrieval for explicit checks is permitted.
- **FR-029**: Required confirmations MUST show determined impact before acceptance. Explicit targets skip only selection; `--yes` confirms determined impacts only. Incomplete non-interactive requests MUST fail with actionable usage without opening controls.
- **FR-030**: Interactive administration MUST follow current full-screen behavior, keyboard navigation, supported viewport constraints, semantic color rules, safe receipts, and cancellation semantics. Cancellation before commitment MUST cause no unconfirmed mutation.
- **FR-031**: Mutations MUST have stable results and exit outcomes, identify affected packages/references in diagnostics, and render terminal failures once. Human interface output MUST remain separate from stable results; normal output MUST NOT expose source credentials.
- **FR-032**: Administrative mutations MUST preserve consistent committed content, registrations, enablement, and configuration under recoverable failures and concurrent administration, without overwriting unrelated concurrent changes. Failed recovery MUST identify unresolved state and recovery actions rather than claim success.
- **FR-033**: Every administrative journey MUST appear in grouped help and its applicable hub. Existing direct grammar and stable contracts MUST remain compatible except for additive maintenance commands/state and the already-promised interactive hubs.
- **FR-034**: Previous slice demonstrations MUST remain successful. Release-candidate validation MUST cover clean offline setup, supported-version upgrade, cancellation/interruption at administrative mutation boundaries, rebuilding the Work index from snapshots, and real repositories with ambiguous local/remote branches on supported platforms.

### Key Entities *(include if feature involves data)*

- **Installed Plugin Package**: Atomic management unit with alias, source identity, origin kind, installed reference/location, declared version, enablement, and owned declarations.
- **Update Candidate**: Fixed proposed revision with version, validation outcome, declaration changes, and dependency impact, separate from the usable installation until committed.
- **Locator Inventory Entry**: Locator declaration, owning package, installation/enablement, policy membership, and availability explanation.
- **Repository Resolution Policy**: User-ordered qualified Locator references, including retained unavailable entries; membership is independent of enablement.
- **Repository Search Roots**: User-managed discovery locations independent of the Work workspace.
- **Administrative Impact**: Determined targets and declaration/policy changes reviewed before confirmation, including dependency-removal authorization.
- **Reference-Package Lifecycle Choice**: Durable user intent distinguishing initial bootstrap or accidental damage from deliberate disablement/removal.
- **Remembered Repository Convention**: Existing repository-scoped choice with availability derived from enabled providers, retained until explicitly replaced.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Across installed/enabled/in-policy/unavailable combinations, 100% of inventories identify correct states/order. Disable → inspect → enable restores participation with zero policy edits and zero disabled-provider executions.
- **SC-002**: In every install, reinstall, enable, and update case, new Locators enter policy zero times without explicit action. Equivalent hub/direct policy and root operations produce identical effective configuration in every acceptance case.
- **SC-003**: Twenty repeated inspections and update checks across fresh, unchanged, newer, invalid, and inaccessible-source cases leave installed content and persisted application state unchanged, with zero temporary retrieval directories after normal termination.
- **SC-004**: Every update uses its reviewed candidate and preserves alias/enablement. Zero recoverably failed or cancelled package changes leave mixed installations or unapproved policy edits; multi-package results account for 100% of targets.
- **SC-005**: Every dependency-aware uninstall either removes exactly the approved package/reference set together or changes nothing. Linked sources and existing Works remain unchanged in 100% of cases; deliberate bundled-package removal survives later commands and a supported upgrade.
- **SC-006**: Users can complete all three hubs with the keyboard in supported terminal sizes/color modes. Every action shows an equivalent direct command; all incomplete non-interactive requests give actionable guidance and zero interactive controls.
- **SC-007**: In every convention-provider disablement/removal case, the stored choice stays inspectable, no replacement is silently selected, and an explicit replacement is reused from another clone without altering existing Works.
- **SC-008**: The F7 demonstration—toggle enablement, add a Locator explicitly, reorder policy through both surfaces, check/apply an update, refuse then approve dependency-aware removal—passes without manual state editing. Prior demonstrations and release-candidate acceptance cases pass on the supported platform matrix.

## Assumptions

- "M7" means roadmap F7, not spec directory 007, which already delivers F6. The next sequential feature directory is `008-sustainable-operation`.
- F1–F6 supply installation, retained enablement with legacy defaults, effective catalogs, resolution/root operations, remembered conventions, terminal presentation, and Work integrity. F7 completes administration without new extension roles or execution models.
- Authority: `docs/roadmap.md` §3 F7 and §4; `docs/prd.md` FR-18–FR-20, FR-40–FR-41, FR-45, FR-47–FR-48 and FR-25/FR-50–FR-64; ADD §5–§8/§12; accepted ADR-0002, ADR-0003, ADR-0005, ADR-0011, ADR-0012, ADR-0015, ADR-0019–ADR-0021. The constitution remains an unratified template. Current PRD/ADR-0021 supersede historical inline-terminal wording.
- Updates follow the recorded source's current default revision for remote pinned packages and recorded source directory for local pinned copies. A different opaque version signals a release without implying an upgrade/downgrade. Additional source channels are outside this milestone.
- Linked installations remain developer-managed: checks report that status and direct users to explicit reinstall to refresh declarations. Update application does not rewrite external sources. The embedded package is not applicable to source updates and follows the offline seed lifecycle. `--all` reports these exclusions explicitly.
- The dependency-removal option's spelling, exact structured-result additions, and recovery mechanics belong to planning; authorization and observable behavior are fixed here. Explicit removals suffice to empty collections.
- Independent updates use per-package commitment; uninstall of a selected set is one consistent removal. Integrity covers normal cancellation, recoverable failures, and supported interruption/restart recovery, without promising protection against hostile external writers or irrecoverable storage/machine failure.
- Existing convention shared-name/provider semantics remain in force. Broader catalog identity changes, automatic convention detection, and rewriting historical Work context are outside this slice.
- Supported platforms remain Linux, macOS, and Windows under the existing release/test matrix. Local-source fixtures can prove update behavior without live integration accounts.
- Formal signatures, telemetry, background updates/notifications, marketplaces, synchronization, graphical interfaces, progress streaming, core-imposed plugin timeouts, and new official integrations remain outside v1.
