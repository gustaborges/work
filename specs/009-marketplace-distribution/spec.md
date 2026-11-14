# Feature Specification: Marketplace Plugin Distribution

**Feature Branch**: `feature/marketplace`
**Created**: 2026-11-07
**Status**: Specified; planning only, implementation not started
**Input**: The marketplace-only proposal in the enclosing Work's `proposal.md`; formalize PRD/ADR/ADD and run Spec Kit through tasks, grouped by milestones. All new artifacts are in English.

## Context and authority

Register marketplace → browse a fixed offer → select one plugin → install an independent copy → maintain it explicitly. Marketplace is the publication unit; plugin remains the installation/maintenance unit. This deliberately replaces standalone installation, new development links, global plugin aliases, and earlier installation command names.

[PRD](../../docs/prd.md) FR-16–FR-21, FR-35, FR-40–FR-48, FR-50–FR-83 and NFR-1–NFR-11 govern requirements. ADR-0022–ADR-0030 govern decisions; [ADD-0001](../../docs/add/add-0001-work-system-architecture.md) describes realization. Earlier specs remain delivery history; feature 009 replaces conflicting distribution, identity, and plugin-manager contracts only. Work lifecycle, roles, explicit policy, opaque versions, and current administrative recovery guarantees remain requirements. The CLI is unreleased: earlier development plugins/state may break, and no backward-compatibility branches are in scope.

## Clarifications

### Session 2026-11-07

The compatibility answer below is an explicit user correction. The other recommended defaults were resolved under gusta-speckit-go; all resulting implementation remains future work.

- Q: Must earlier development plugins, state, or command behavior remain supported? → A: No. The user explicitly requires a clean pre-release cutover. Use one marketplace contract and delete compatibility selectors, readers, adapters, adoption/migration flows, and compatibility-only tests (ADR-0030).
- Q: Does same-version changed content qualify for update? → A: No. Validate it and report unchanged; explicit reinstall uses a refreshed fixed offer and its own replacement authorization.
- Q: Can removing and re-registering a marketplace rebind retained choices/private values to its replacement? → A: No. Current-domain references bind the actual source registration instance. This prevents namespace reuse from transferring ownership; it does not require support for earlier development formats.
- Q: Can the official online offer be implicitly registered or identified by catalog name? → A: No. Registration is explicit; official attribution requires the owner-confirmed publication origin. That origin is a delivery dependency, not an invented planning value.
- Q: Do milestone checkpoints authorize partial public releases? → A: No. Demonstrations and focused tests may run per milestone; public delivery requires the complete distribution cut and CLI/TUI parity.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Register and Browse Sources (Priority: P1)

A developer registers a local or remote marketplace, sees its published identity and fixed offer, and browses offline. The included offer is separate from the explicitly registered official online offer.

**Why this priority**: Every new installation must originate in an explicitly selected published entry.
**Independent Test**: Register a local non-Git marketplace and a Git fixture, disconnect sources, inspect offers and totals, and verify zero plugin installation/execution or policy changes.

**Acceptance Scenarios**:

1. **Given** an offer containing A/B/C, **When** registered, **Then** all entries are discoverable and none is installed or executed.
2. **Given** no state, **When** sources are inspected, **Then** the included offer appears without creating state or registering the official online offer.
3. **Given** a stored offer, **When** its source disappears or refresh fails, **Then** offline browsing retains the earlier valid offer.
4. **Given** registration, **When** the same source/name/branch is registered again, **Then** it is unchanged; another source publishing that namespace fails without overwrite/rename.
5. **Given** an invalid/missing catalog, unsafe package path, portable name conflict, or attempted included-provider replacement, **When** registered, **Then** no partial state is published.
6. **Given** equal display names or withdrawn entries, **When** sources are listed, **Then** namespaces distinguish sources and complete local installed/enabled totals are independent of filters and catalog availability.
7. **Given** the explicitly registered official online source, **When** its independent publication changes, **Then** ordinary refresh exposes it without a Work release.

### User Story 2 - Install and Run Independent Qualified Plugins (Priority: P1)

A developer installs exactly one offered plugin as a managed copy. Homonymous plugins coexist, with every contribution resolving to its actual provider.

**Why this priority**: Independent copies and qualified identities are the central execution contract.
**Independent Test**: Install homonymous plugins with identical contribution names from two sources, execute each, change/delete source trees, and toggle/remove one without changing the other.

**Acceptance Scenarios**:

1. **Given** A/B/C in a fixed offer, **When** B is chosen, **Then** only B is copied from that offer and policy stays unchanged.
2. **Given** homonymous packages, **When** both are installed, **Then** dispatch, convention choice, policy, memory, Starter selection, subscriptions, private namespaces, and guards distinguish actual providers.
3. **Given** a managed copy, **When** source/capture bytes change or disappear, **Then** installed bytes and independent execution remain unchanged.
4. **Given** a bare name, standalone path, retired command, or forbidden mode, **When** installation is attempted, **Then** it fails before source retrieval/mutation; domain callbacks reject missing marketplace provenance too.
5. **Given** an unavailable exact Git revision, **When** installing its recorded offer, **Then** installation fails without selecting a newer revision.
6. **Given** the included offer, **When** its reference entry is explicitly added/reinstalled, **Then** the common pipeline works offline without inserting policy references.
7. **Given** incompatible exclusive fallback contributions, **When** reviewed, **Then** the functional conflict names both complete identities rather than rejecting mere homonymy.

### User Story 3 - Maintain Plugins and Remove Sources Deliberately (Priority: P1)

A developer refreshes catalogs separately from installations, checks fixed candidates without persistence, reinstalls explicitly, and removes selected plugins or a whole source after exact impact review.

**Why this priority**: Distribution must preserve reproducibility, dependency authorization, and recovery.
**Independent Test**: Publish changed/withdrawn entries, refresh, check/apply a fixed candidate, and remove a source with enabled, disabled, withdrawn, and policy-referenced packages; inject pre/post-commit failures.

**Acceptance Scenarios**:

1. **Given** changed offers, **When** refreshed, **Then** installations/enablement/policy stay unchanged; identical refresh preserves bytes and administrative revision.
2. **Given** recorded provenance, **When** checked, **Then** candidates come from that source independently of discovery freshness, without persistent writes.
3. **Given** valid changed bytes but equal opaque versions, **When** checked, **Then** the result is unchanged; explicit reinstall from the refreshed offer may replace bytes, preserving enablement with exact impact authorization.
4. **Given** a reviewed candidate, **When** applied, **Then** precisely that candidate is published without refetch; invalid/unavailable candidates preserve the old copy.
5. **Given** withdrawn entries or moved paths, **When** maintained, **Then** withdrawal preserves copies; path moves retain identity only for the same entry and provenance.
6. **Given** a source containing disabled/withdrawn plugins and policy dependencies, **When** removal is reviewed, **Then** the entire set is enumerated offline; operation confirmation alone does not authorize policy removal.
7. **Given** cancellation, missing dependency authorization, or changed reviewed facts before commit, **When** removing, **Then** the complete previous set remains.
8. **Given** a committed removal with cleanup failure, **When** recovered, **Then** the exact removal is completed and reported committed, preserving Works and convention memory.
9. **Given** included and online-official sources, **When** removed, **Then** the included source is rejected by the domain; the online source follows ordinary whole-source removal.
10. **Given** batches, **When** refreshing/updating, **Then** results are per target, candidates share one source revision per group, and independent successes remain; cancellation/recovery stops remaining work as contracted.

### User Story 4 - Purposeful First Use and Default Lifecycle (Priority: P1)

A developer starts from a fresh home, registers sources before or after first use, and controls the included reference plugin without implicit reinstalls or policy changes.

**Why this priority**: Offline first use and durable user choices are current product behavior, independent of backward compatibility.
**Independent Test**: Use fresh disposable homes to test registration-before-first-start, explicitly empty policy, first-use setup, explicit add, disablement/removal, and a newer included offer; verify common installation and one-time policy handling.

**Acceptance Scenarios**:

1. **Given** a fresh home requiring default capabilities, **When** setup runs, **Then** the included reference entry is installed through the common marketplace pipeline and default policy is initialized once.
2. **Given** source registration before first required setup, **When** a Work is started, **Then** registration alone has not marked default policy handled and the same fresh-setup behavior applies.
3. **Given** an explicit policy choice, including an empty policy, **When** setup or later use runs, **Then** that choice is preserved; explicit empty selection is meaningful even when the array was already empty.
4. **Given** explicit add/reinstall of the included reference entry, **When** applied, **Then** it uses ordinary copy/review behavior and marks policy handled without inserting Locators.
5. **Given** a disabled or deliberately removed reference plugin, **When** later commands, catalog refresh, reads, or a new binary offer run, **Then** neither content nor enablement nor policy is restored automatically.
6. **Given** a newer included offer, **When** a copy is already installed, **Then** only explicit update/reinstall can replace it.
7. **Given** unsupported or malformed administrative state, **When** it is loaded, **Then** the single current validator rejects it without conversion, migration, or automatic deletion. Tests use disposable homes and do not require execution of earlier binaries/plugins.

### User Story 5 - Complete Administration in the Terminal Manager (Priority: P2)

A developer uses Installed and Marketplaces in one persistent manager to complete all supported administration, including batches, without switching to CLI.

**Why this priority**: Completion requires CLI/TUI capability and authorization parity.
**Independent Test**: Exercise every parity-matrix row, compare resulting state, and repeat Back/cancel/error, no-color, keyboard, narrow viewport, and resize cases.

**Acceptance Scenarios**:

1. **Given** the manager, **When** opened, **Then** only Installed and Marketplaces are top-level destinations; special Official/direct-install, alias and link controls, and per-source tabs are absent.
2. **Given** Git/local registration, **When** entered, **Then** source fields are visible/editable, branch is offered only for Git, and the validated published namespace is shown.
3. **Given** any CLI administrative capability, **When** its manager action runs, **Then** targets, data, effects, and exact dependency authorization match, including check-only and same-version reinstall.
4. **Given** empty/missing catalogs, failed refresh, missing runtime, conflicts, or recovery/cleanup, **When** encountered, **Then** current actionable feedback preserves valid offers and manager navigation.
5. **Given** details/review/progress/results, **When** Back or pre-commit cancellation occurs, **Then** candidates are cleaned and valid filters/selection survive in the same session.
6. **Given** normal, colorless, or reduced terminals, **When** navigating, **Then** hierarchy, focus, selection, status, destructive actions, and labeled local totals remain distinguishable using the existing theme.
7. **Given** normal exit, **When** closing, **Then** the terminal restores once and prints only the existing acknowledgment, without accumulated transcript, secrets, or interactive bytes in stable stdout.

### Edge Cases

- Empty catalogs are valid; absent/inconsistent stored catalogs are separate inspection states.
- Portable-equivalent names conflict without silent normalization; mixed selector forms fail before retrieval.
- Local source changes during capture reject the whole capture rather than publishing mixed revisions.
- Reusing a removed namespace does not reassociate historical memory, private values, or Work provenance; retained facts bind the actual source registration instance.
- Source registration does not handle default policy. Explicit policy choice, including empty, sets the durable policy-handled flag; config-file existence is not initialization evidence.
- Public digests are never used verbatim as filesystem segments; capture paths use validated hex without the algorithm prefix.
- Invalid same-version candidates still fail validation.
- Executing packages and in-flight plans protect their owned copies/captures from concurrent removal/cleanup.
- Missing source registration prevents maintenance rather than creating an unregistered-source mode.
- Extracted editable catalog files cannot redefine the included provider.
- Catalog text is passive and escaped; public results omit transport secrets and raw Git errors.
- One current schema validates administrative state and journals; unsupported shapes fail without fallback or conversion.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Require a valid explicit marketplace entry for every new installation/content replacement; never synthesize a catalog. PRD FR-16, FR-65–FR-66.
- **FR-002**: Support registration, pure inspection, explicit refresh, and removal of Git/local sources with unique published namespaces. PRD FR-67.
- **FR-003**: Separate fixed discovery from installed content; browsing is offline and refresh never changes installations. PRD FR-70–FR-71.
- **FR-004**: Qualify plugin selection by marketplace, allow cross-source homonyms, and prohibit aliases/implicit lookup. PRD FR-21, FR-68–FR-69.
- **FR-005**: Copy only the selected package into independently managed execution storage. PRD FR-65, FR-70.
- **FR-006**: Carry composed provider identity through components/conventions, policy, memory, Starters/subscriptions, dispatch, private namespaces, and guards. PRD FR-35, FR-40–FR-48, FR-68.
- **FR-007**: Retain real fixed provenance and maintain only through the recorded source and matching entry, independently of discovery state and without name-based redirection. PRD FR-72.
- **FR-008**: Check/apply updates explicitly using opaque version inequality and frozen reviewed candidates; equal versions remain unchanged despite valid changed bytes. PRD FR-19, FR-73.
- **FR-009**: Permit explicit reinstall from the fixed offer, preserving enablement with exact replacement/dependency authorization; identical content is unchanged. PRD FR-16, FR-48, FR-73.
- **FR-010**: Preserve copies on withdrawal, inaccessible origins, invalid candidates, and pre-commit failure. PRD FR-74, FR-77.
- **FR-011**: Remove selected installations as a set without removing sources/siblings, preserving Works and convention memory. PRD FR-20, FR-48.
- **FR-012**: Remove a user-managed source, discovery state, and all its installed plugins in one recoverable reviewed publication, including disabled/withdrawn entries and separately authorized policy references. PRD FR-75.
- **FR-013**: Revalidate exact reviewed facts, reject conflicts without expanding authorization, and distinguish pre-commit preservation from committed cleanup/recovery. PRD FR-52, FR-64, FR-77.
- **FR-014**: Keep reads pure and checks temporary; prepare sources/review outside the publication lock and preserve coherent recovery/invocation ordering. PRD FR-52, FR-76–FR-77; NFR-3.
- **FR-015**: Provide the included offline offer through the same entry/copy lifecycle; prohibit user replacement/removal of that provider in the domain. PRD FR-78.
- **FR-016**: Preserve one-time initial policy setup, disablement, and removal history; explicit add/repair/reinstall and inspection never reseed policy. PRD FR-10, FR-41, FR-78.
- **FR-017**: Provide a distinct explicitly registered official online source, with release-independent publication, ordinary Git lifecycle, and provenance-based attribution. PRD FR-80.
- **FR-018**: Define one clean marketplace-only runtime and remove superseded installation/identity/state code, compatibility readers/selectors/adapters, migration/adoption flows, and compatibility-only fixtures. Earlier development plugins/state may break. PRD FR-79.
- **FR-019**: Preserve Works and remembered choices produced by the current contract across package/source removal; a new registration reusing public names must not claim retained choices/private values. PRD FR-20, FR-25, FR-35, FR-72.
- **FR-020**: Present optional passive display names separately from canonical identity and complete local installed/enabled totals even with missing catalogs. PRD FR-81–FR-82.
- **FR-021**: Expose every CLI administrative capability in the manager, including all-source catalogs, source filters, branches, multiple targets, checks, update all, refresh all, reinstall, and whole-source removal. PRD FR-76.
- **FR-022**: Replace destinations with Installed/Marketplaces, preserving one session, keyboard, cleanup, current feedback, exit acknowledgment, existing visual hierarchy/theme, no-color and resize behavior. PRD FR-53–FR-64, FR-83.
- **FR-023**: Define the intended command surface, structured inventory/results, partial failures, and safe errors; obsolete command/flag aliases and translation shims are absent. PRD FR-50–FR-52, FR-58, FR-62; NFR-10.
- **FR-024**: Use one strict current persistence and journal format with coherent current-transaction recovery and ordinary invalid/unsupported-state errors; include no previous-format decoding or conversion. PRD FR-77, FR-79; NFR-5–NFR-6.
- **FR-025**: Validate passive catalogs, portable identities, contained packages, complete static manifests, and installation runtime/entrypoints without discovery execution. PRD FR-66, FR-70; NFR-3, NFR-7.
- **FR-026**: Preserve existing plugin trust: escape metadata, redact credentials, and introduce no signing, continuous integrity, sandbox, or same-user tamper-resistance promise. PRD NFR-7, FR-83.

### Key Entities

- **Marketplace**: Namespace, display name, source descriptor, fixed offer, and administrative ownership.
- **Catalog entry**: Independent plugin name and contained location, with static offered metadata.
- **Discovery snapshot**: Fixed catalog/content revision; distribution capture is separate from execution copies.
- **Installed plugin**: Composite identity, managed content, enablement, opaque version, actual provenance, and contributions.
- **Provider reference**: Qualified contribution identity for policy, memory, selection, dispatch, and execution protection.
- **Administrative plan**: Fixed targets/candidates/impact, separate dependency authorization, reviewed preconditions, and recoverable outcome.
- **Default lifecycle history**: Durable evidence of initialization, policy handling, and deliberate removal.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All three source classes expose valid offers; a three-plugin fixture installs one chosen plugin and zero siblings.
- **SC-002**: Two homonymous fixtures with equal contribution names execute, toggle, update, and remove independently with zero cross-provider substitution.
- **SC-003**: Every read/check scenario preserves persistent bytes; offline browsing succeeds for every valid previously obtained offer.
- **SC-004**: Every pre-commit failure/cancellation preserves the earlier complete set; every post-commit recovery completes the approved set and reports committed cleanup honestly.
- **SC-005**: Every CLI/TUI parity row has a verified keyboard journey and equivalent state/authorization for individual, selected, and all targets.
- **SC-006**: Fresh, registration-first, explicit-empty-policy, disabled, removed, and changed-included-offer scenarios preserve the current product choices; the runtime exposes one marketplace-only model with zero compatibility branches.
- **SC-007**: Every new logical surface remains navigable at normal/reduced terminal sizes with/without color; normal exit restores the terminal once with one acknowledgment.
- **SC-008**: Acceptance, current-format validation, recovery, and portable-path scenarios pass on Linux, macOS, and Windows runners; Unix interactive paths additionally pass terminal tests. Cross-compilation is not acceptance evidence.

## Assumptions

- One coordinated public release covers the distribution cut. Milestones are development checkpoints, not releases of contradictory installation contracts.
- Existing administrative transactions, guards, Git transport, and presentation primitives are extended.
- Current Works and source provenance are part of this design. Persistence can change to suit the design; no prior development format or plugin behavior must remain readable/executable.
- The owner supplies the real official publication repository before public delivery. Planning specifies its contract and verification without inventing a URL or publishing remotely.
- The user explicitly rejects backward compatibility because the CLI is unreleased. Development tests and examples are updated to the intended contract; no migration/reset subsystem or real-home deletion is authorized.
- Implementation validation is explicitly required: focused unit/contract/integration tests, deterministic failures/concurrency, relevant race detection, terminal journeys, lint, and supported-OS execution. None is claimed run during planning.
- Central aggregation, automatic discovery, whole-catalog installation, dependency installation, SemVer/channels, external package artifacts, signing, reputation, billing, and same-user tamper protection are excluded.
