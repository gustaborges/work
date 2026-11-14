# Feature Specification: Agent Skills for Work Plugin Authors

**Feature Branch**: `feature/agentic-skills`
**Created**: 2026-11-14
**Status**: Draft planning baseline; S0 authority/dependency gate required before implementation
**Input**: `../proposal.md`; run Spec Kit through task planning only.

## Context and authority

Help a marketplace author use local Claude Code or Codex to create, review and validate Work marketplaces using guidance matching the installed Work release. Installation is explicit, offline and independent of Work application state.

Starting phase: NEW. Number 009 exists on `feature/marketplace`; 010 is available. The dependency must merge into the development baseline (`develop`) before implementation; recheck its contracts, diagnostics and authority IDs at S0. Planning does not merge that dependency.

The proposal is design input, not accepted authority. [Governance drafts](governance/README.md) separate proposed PRD requirements, decisions and conditional design. Promote approved content in PRD → ADR → ADD order before implementation. Existing accepted ADRs remain authoritative. Plural `work skills` is conditional on a narrow exception to ADR-0019, never a silent override.

## Clarifications

### Session 2026-11-14

Recommended defaults accepted under gusta-speckit-go; no user questions were needed. All functional, entity, interaction, failure and completion categories are clear. Architecture-ID/naming approval is explicitly deferred to S0, not guessed.

- Q: Which vendors are selected when omitted? → A: Exactly one explicit vendor per command; no default, repeated vendor or all-vendors option (revised by owner, FR-003).
- Q: Which scope is used when omitted? → A: None; exactly one explicit local/user scope is required (FR-002).
- Q: Include validate/removal/overlays in the first delivery? → A: Defer them; provide complete isolated per-entry validation guidance (FR-006, Assumptions).
- Q: Can planning proceed before dependency merge and authority acceptance? → A: Draft planning can; implementation/finalization cannot bypass S0 (FR-022).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Equip an Agent to Author a Marketplace (Priority: P1)

A developer explicitly installs release-matched authoring guidance for exactly one of Claude Code or Codex, in the current directory or at user scope. Their agent chooses roles, scaffolds a plugin, validates every offered package and explains publication.

**Why this priority**: Accurate, discoverable guidance is the primary product value.
**Independent Test**: Install in isolated homes/folders and complete a marketplace-authoring exercise with each agent using only the delivered guidance.

**Acceptance Scenarios**:

1. **Given** Work and no network, **When** local or user installation runs, **Then** the selected agent receives identical complete release guidance without accessing Work application state.
2. **Given** a new/existing marketplace, **When** an agent follows the skill, **Then** it reads any existing catalog, chooses supported roles, scaffolds valid packages and uses qualified publication/install commands.
3. **Given** multiple entries including alternative fallback providers, **When** validation runs, **Then** it discovery-checks and independently installs every entry in fresh scratch environments without component execution, source edits or environment leaks.
4. **Given** source edits after a failure, **When** validation retries, **Then** it captures corrected content anew and cleans scratch homes on success/failure/cancellation.
5. **Given** an existing folder without a catalog, **When** guidance is installed, **Then** installation succeeds after confirmation and supports later scaffolding.

### User Story 2 - Refresh Guidance Without Losing Local Work (Priority: P1)

A developer reruns installation after a Work upgrade. Current copies remain untouched, older managed copies refresh, and local content survives unless replacement is explicitly authorized. Interrupted operations resume safely.

**Why this priority**: Agent directories are user-owned; trustworthy installation is essential.
**Independent Test**: Exercise all ownership states, target races and interruption boundaries with exact before/after content comparisons.

**Acceptance Scenarios**:

1. **Given** current content, **When** installation repeats, **Then** it reports unchanged and preserves mtimes; older unmodified managed copies instead refresh completely.
2. **Given** modified/foreign ordinary content, **When** installation runs, **Then** it refuses unchanged unless force is explicit; unreadable/unsafe content remains protected even with force.
3. **Given** an initial conflict in one selected destination, **When** preflight runs, **Then** no requested skill tree changes.
4. **Given** interruption before commit, **When** explicit installation resumes, **Then** it restores the exact old tree/absence; after commit it retains verified new content and finishes cleanup.
5. **Given** recovery evidence inconsistent with actual edits, **When** recovery runs, **Then** it preserves evidence/content and reports an explicit recovery action.
6. **Given** a later target apply failure, **When** results are reported, **Then** earlier commits remain, unprocessed targets are untouched and overall failure accurately distinguishes committed cleanup-pending outcomes.

### User Story 3 - Inspect Guidance Without Side Effects (Priority: P2)

A developer or automation inspects installation state without creating directories, locks or recovery artifacts.

**Why this priority**: Inspection must be safe on absent and interrupted installations.
**Independent Test**: Inspect the entire state matrix in human/machine output and compare filesystem and Work-state bytes/existence before/after.

**Acceptance Scenarios**:

1. **Given** missing paths, **When** status runs, **Then** it reports absent and creates nothing.
2. **Given** outdated/modified/foreign content, **When** machine status runs, **Then** one ordered result per destination reports state/digests with no human stdout text.
3. **Given** recovery/structural/I/O failure, **When** status runs, **Then** it retains other inspected rows, fails and performs no repair or locking.
4. **Given** concurrent replacement, **When** status observes a change, **Then** bounded retries either establish coherent inspection or report recovery/error, never fabricated current state.

### User Story 4 - Keep Guidance Aligned With Work (Priority: P2)

A maintainer changes a marketplace, manifest or invocation contract and sees verification fail until represented references and the reviewed pin are updated. Complete scaffolds and examples are validated by the correct real validators.

**Why this priority**: Stale guidance defeats successful installation.
**Independent Test**: Independently perturb a catalog rule, role rule and invocation field; each intended gate fails, then reviewed regeneration/re-pinning restores verification without CI rewriting files.

**Acceptance Scenarios**:

1. **Given** a represented change, **When** verification runs, **Then** stale generated tables/pins fail with an update instruction.
2. **Given** valid/invalid/partial/invocation examples, **When** fixtures run, **Then** each reaches its appropriate validator and yields the declared outcome.
3. **Given** a scaffold with name mismatch or missing entrypoint, **When** complete validation runs, **Then** discovery/selected installation fails even if standalone JSON parses.
4. **Given** behavior absent from descriptors, **When** reviewed, **Then** focused fixtures and explicit maintenance obligations cover it; a pin is never claimed to prove prose correctness.

### Edge Cases

- Missing/invalid scope; unknown/empty/repeated vendors; all-vendors values; any skill operands; obsolete scope flags; unknown flags; install with machine output: reject before target writes.
- Relative user configuration anchor; canonical duplicate/case aliases or overlapping destinations; linked parent/target/descendant, dangling link, special node and unreadable content.
- Missing/malformed/wrong-skill/future marker; extra hidden/deleted/edited file; platform permission differences and Unicode/spaced paths.
- Concurrent overlapping installers; lock cancellation/timeout; edit before locked preflight or immediately before replacement, including force mode.
- Interruption at every record/move/commit/cleanup boundary; unexpected recovery edits; unrecorded lookalike scratch directories; committed cleanup failure and status during rename gap.
- Real WORK_HOME/environment already configured; later catalog entry missing runtime/entrypoint; corrected-source retry; alternative fallback providers needing independent install homes.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Deliver `work-cli-plugin-builder` offline with the installed Work release, independently of runtime plugin distribution.
- **FR-002**: Require explicit local/user scope. Local uses the current directory; no catalog is required or inspected. Both install scopes require confirmation before writes; --yes skips it and is required non-interactively. Force is separate replacement permission and never skips confirmation.
- **FR-003**: Support Claude Code/Codex with exactly one explicit vendor per command. Always select the complete release inventory; no individual skill selection or all-vendors option. Reject unknown/empty/repeated vendors and all positional operands before writes. Do not detect or require installed vendor executables.
- **FR-004**: Deliver identical vendor-neutral content at documented discovery locations with accurate invocation/reload guidance.
- **FR-005**: Teach existing/new marketplaces/plugins, all four roles, conventions, inputs/events, safe packaging, opaque version changes, qualified publication and reserved/official identities.
- **FR-006**: Distinguish discovery/installability/runtime behavior. Recapture edited source, selected-install every entry independently, preserve real environment/source/Work state and clean scratch homes on success/failure/cancellation; management validation never executes components.
- **FR-007**: Preserve current copies/mtimes and replace complete older unmodified managed copies.
- **FR-008**: Protect modified/foreign ordinary trees unless force is explicit. Missing/malformed/unsupported/wrong-skill ownership data is foreign; I/O failure never implies absence or replacement permission.
- **FR-009**: Reject unsafe linked paths, special nodes, non-directory targets and overlaps even with force; resolve anchors consistently, recheck descendants and preserve external targets.
- **FR-010**: Preflight all destinations before payload changes; serialize cooperating overlaps, recheck all destinations under coordination and each observed target immediately before replacement. Refuse changed targets even with force.
- **FR-011**: Recover exact old content/absence before commit and retain verified new content after commit. Preserve mismatched evidence with explicit recovery instructions; never sweep unrelated temporary files.
- **FR-012**: Commit destinations individually; retain earlier successes on later failures, identify failed/not-attempted targets, fail overall and distinguish committed cleanup-pending outcomes.
- **FR-013**: Report absent/current/outdated/modified/foreign/conflict/recovery_required/unreadable. Status is strictly read-only, including locks/recovery/missing paths, and never combines different observed revisions.
- **FR-014**: Status supports deterministic machine rows with vendor/skill/absolute path/state and embedded/recorded/actual digests, null unavailable values and safe error tokens. Notes/diagnostics remain on stderr. Ordinary inspected states exit zero; structural/recovery/I/O failures fail while retaining other rows.
- **FR-015**: Install rejects machine output before writes and emits deterministic per-target outcomes, escaped paths, commit/cleanup state and selected-agent next steps. Usage errors exit 2; operational failures use reconciled stable diagnostics and appropriate remedies.
- **FR-016**: Both commands avoid Work-home, registry, index, bootstrap, repository discovery and Work/admin locks; coordination remains local to selected filesystem destinations.
- **FR-017**: Content identity agrees across delivered/staged/installed trees regardless of platform modes/mtimes, detects added/deleted/edited/hidden files and rejects unsafe asset nodes.
- **FR-018**: Real represented catalog/manifest/role/invocation definitions drive deterministic generated references and a reviewed fingerprint. Verification detects stale output/pins without rewriting; focused fixtures/review cover unrepresented behavior.
- **FR-019**: Route every scaffold/marked example to its actual validator; complete marketplaces also pass discovery and every-entry isolated installation. Invalid examples fail for declared reasons.
- **FR-020**: Maintainer/user/help/PR guidance covers install/update/status, local-only coordination and contract maintenance. Binary upgrades never refresh copies silently.
- **FR-021**: Delivery includes fault/race/portability/purity/per-entry matrices and independent Claude/Codex authoring exercises with zero manual contract corrections.
- **FR-022**: Before implementation, S0 reconciles merged 009 and promotes approved requirements/decisions/design in authority order. Naming cannot override ADR-0019 silently; final authority IDs/diagnostics cannot rely on stale branch assumptions.

### Key Entities *(include if feature involves data)*

- **Delivered skill**: Release-owned named guidance, references, scaffolds and represented-contract pin.
- **Destination**: Vendor, selected scope anchor and canonical skill location controlling deduplication/coordination.
- **Ownership evidence**: Skill identity and last installed content identity.
- **Inspection result**: Coherent destination state and available digest/error evidence.
- **Install operation**: Observed old content, staged new content, owned backup and recovery/commit evidence.
- **Contract descriptor**: Deterministic representation derived from actual validators/protocol types for references and drift detection.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every local/user × Claude/Codex combination works offline; vendor payloads match and repeat installs change zero mtimes.
- **SC-002**: Every ownership/path/interruption case preserves exact pre-commit content/absence or verified committed content; zero unowned/external files are deleted.
- **SC-003**: Every status case and both commands' Work-state checks cause zero unexpected writes; concurrent inspection never invents current content.
- **SC-004**: Validation covers 100% of offered entries, catches later failures and corrected-source retries, executes zero components and leaves zero scratch/environment leaks.
- **SC-005**: Independent catalog/role/invocation mutations each fail intended stale-guidance gates; verification leaves generated/pin artifacts unchanged.
- **SC-006**: One Claude Code and one Codex exercise each create a marketplace whose catalog registers and every entry installs using delivered guidance alone with zero manual contract corrections.
- **SC-007**: Complete automated matrix passes on Linux/macOS/Windows including Unicode/spaces and declared runtime prerequisites.

## Assumptions

- One local user and initially one delivered skill; no new plugin role, external service or telemetry.
- Cooperating installers are coordinated; arbitrary malicious writers and power/media loss are outside the guarantee. Tested process-interruption recovery is required.
- Read-only marketplace validate, skill removal, vendor overlays and interactive scope selection are deferred. Future removal must honor recovery/coordination.
- S0 completed during implementation with owner approval against develop; PRD FR-65–69 and ADR-0031–33 govern this feature. See validation.md.
- Fault/integration/contract tests are explicitly required by FR-021. Real-agent dogfood is a delivery check, never inferred from parser success.

## Owner-approved experience revision (2026-11-14)

PRD agent skills FR-66 and ADR-0034 replace default-both, selectable-skill, global/folder grammar with explicit single-vendor local/user complete-collection installation. Both scopes confirm before writes unless --yes is supplied. The initial skill is named work-cli-plugin-builder. Previous S0 records describe the historical approval; this revision governs current behavior.
