# Feature Specification: Per-mode Starter Links and Known-link Eligibility

**Feature Branch**: `feature/link-improvements`
**Created**: 2026-11-14
**Status**: Ready for planning
**Input**: [Issue #61](https://github.com/gustaborges/work/issues/61), separate from feature 009; deliver both capabilities together. Planning only in this pass.

## Clarifications

### Session 2026-11-14

Recommendations auto-accepted under the invoked autopilot skill; no user decision required.

- Q: Which override names are recognized? → A: `new`, `contribution`, and `fork`; overrides do not alter the modes offered for selection.
- Q: How do equal metadata keys merge? → A: Shallow replacement, including null, with no recursive merge or deletion semantics.
- Q: Does an existing empty stored link count as known? → A: Yes, use map presence and existing provider/source-instance binding.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Preserve the meaning of the source PR (Priority: P1)

A user starting from a pull request chooses contribution or fork. The created Work records the source relationship appropriate to that choice, while retaining shared context returned by the Starter.

**Why this priority**: The same PR is the contribution target in one journey and the origin of a new effort in the other; confusing them makes imported context misleading.

**Independent Test**: Use a deterministic Starter offering both modes and distinct link/metadata overrides; create a Work in each mode and inspect its snapshot.

**Acceptance Scenarios**:

1. **Given** shared source-PR context and contribution/fork overrides, **When** contribution is chosen, **Then** shared fields and contribution overrides are persisted, with contribution winning equal keys and no fork-only fields.
2. **Given** the same response, **When** fork is chosen, **Then** shared fields and fork overrides are persisted, with no contribution-only fields.
3. **Given** a Starter without mode-specific output, **When** any existing supported creation journey completes, **Then** links and metadata match existing behavior.
4. **Given** an invalid key, invalid link value, or unknown mode anywhere in the response, including an unselected mode, **When** start processes it, **Then** it fails with `starter-response-invalid` (37) before branch, worktree, or Work creation.

### User Story 2 - Keep already known links during automatic discovery (Priority: P1)

A user starts with a known issue or PR link. Automatic discovery preserves that link and runs only Linkers whose output link is missing, while Importers can still consume known links.

**Why this priority**: Automatic guessing must not replace a relation the Starter already established.

**Independent Test**: Start with a known link and marker-writing Linkers for known/missing keys; verify only the missing-key Linker executes and eligible Importers still run.

**Acceptance Scenarios**:

1. **Given** a persisted link for an automatic Linker's stored key, **When** start eligibility is evaluated, **Then** it is omitted silently, without subprocess, warning, progress event, or run receipt.
2. **Given** no persisted value for the output key, **When** all existing eligibility conditions pass, **Then** the Linker still runs.
3. **Given** one Linker can produce another Linker's required input, **When** eligibility is decided, **Then** the second Linker is not made eligible by the first's output during that phase; Importers are evaluated after discovery and may use the new link.
4. **Given** a link already exists, **When** the user explicitly supplies a replacement through manual `work link`, **Then** the value can still be replaced without running a Linker subprocess.

### Edge Cases

- An absent selected-mode entry or empty mode map adds nothing; base fields survive.
- A mode-specific metadata value replaces the whole value at an equal key, including explicit null; there is no recursive merge or deletion marker.
- An invalid unselected mode cannot hide invalid publication keys or link values.
- `new` is the implicit creation mode when `start_modes` is absent and may have an override; `modes` does not offer additional selectable modes.
- Private link presence is checked using the producing provider's current source-instance key; another provider's or former source-instance's value does not suppress it.
- Presence, rather than truthiness, determines whether a stored link is known; a hand-edited empty value still counts as present.
- A skipped known-key Linker emits no ambiguous-filter warning; Importer eligibility diagnostics are unaffected.
- Multiple Linkers for the same initially missing key remain governed by existing phase behavior; this feature adds no winner selection or ordering promise.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A Starter MUST be able to supply optional mode-specific links and metadata in one response, without a second invocation or adding the chosen mode to its input.
- **FR-002**: After choosing the creation mode and before creating the Work, the system MUST merge the selected entry over base links and metadata by key, preserving base-only entries and excluding unselected entries. Metadata replacements are shallow, including null values.
- **FR-003**: Before any materialization, the system MUST validate base and every mode entry against existing publication ownership/key and non-empty link-value rules, reject unknown mode names, and report invalid mode output as `starter-response-invalid` (37).
- **FR-004**: A response without mode entries MUST retain existing behavior. Missing/empty selected entries MUST act as no override. Supported override names are `new`, `contribution`, and `fork`; selectable modes remain controlled by the existing mode-offer rules.
- **FR-005**: The merged result MUST use existing binding and atomic Work-creation persistence, preserving shared public keys and private provider/source-instance ownership.
- **FR-006**: On `start:finalized`, an automatic Linker whose stored output key is already present in the initial Work links MUST be excluded before subscription diagnostics or input resolution, with no trace or warning.
- **FR-007**: Missing-key Linkers MUST retain existing enablement, subscription, filter, required-input, and ordering behavior. Manual `work link` MUST remain unchanged.
- **FR-008**: Linker eligibility and inputs MUST be decided once for that phase, before any Linker executes. Importers MUST continue to evaluate after Linkers and consume available known/discovered context.
- **FR-009**: The Starter protocol and targeted unit, contract, and integration tests MUST cover both changes together, including rejected responses that create nothing and silent known-link skipping.

### Key Entities

- **Starter response**: repository reference, branch/mode offers, shared links/metadata, optional overrides indexed by creation mode.
- **Mode context**: links/metadata that override shared context for exactly one supported creation mode.
- **Known link**: an entry present in the initial canonical Work links, under its stored semantic or provider-scoped key.
- **Eligibility decision**: the fixed set of components and their projected inputs for a phase.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All contribution/fork/new-mode acceptance cases retain shared context and exactly the selected overrides, with one Starter response per resolution attempt.
- **SC-002**: Every invalid-mode-output case fails before creating any Work, worktree, or branch, including invalid unselected entries.
- **SC-003**: Known-key automatic Linkers execute zero times and produce zero warnings or run events; eligible missing-key controls execute normally.
- **SC-004**: Existing no-override Starter, manual link replacement, and post-discovery Importer scenarios retain their expected outcomes.

## Assumptions

- Current provider identities and binding from feature 009 are available in this checkout; feature 010 is independent of its remaining delivery tasks.
- The marketplace-side removal of PR guessing is tracked separately in `gustaborges/work-cli-marketplace#7`. This CLI change cannot prevent a missing-key Linker from guessing incorrectly.
- Branch upstream changes, resume/link discovery, new public semantic keys, new mode selection UX, and plugin implementation changes are out of scope.
- Existing semantic-key validation applies to PR example keys; this feature does not add a key catalog.
