# Proposed PRD amendment: Agent assistance for marketplace authors

> Historical planning record. Current CLI scope, vendor, collection and confirmation behavior is governed by [ADR-0034](../../../docs/adr/adr-0034-complete-skill-collection.md).

**Status**: Draft requirement amendment; promote after merged 009 reconciliation.
**Destination**: `docs/prd.md`, plugin authoring journey and functional requirements.

## Requirement

- **AS-R1**: A marketplace author can obtain offline, release-matched guidance for supported local Claude Code and Codex agents without installing a runtime plugin or requiring an AI service during Work commands. Covers feature FR-001, FR-004–FR-006.
- **AS-R2**: A user explicitly chooses global or existing-folder scope and supported agents. Both agents are the default. Invalid requests have no destination side effects. Covers FR-002–FR-003, FR-015.
- **AS-R3**: Refresh preserves current/local content, protects unsafe/unreadable paths, coordinates concurrent installers and safely resumes interrupted operations with accurate partial outcomes. Covers FR-007–FR-012, FR-017.
- **AS-R4**: Inspection is read-only and offers deterministic human/machine installation states without accessing Work application state. Covers FR-013–FR-016.
- **AS-R5**: Delivered guidance and complete scaffolds stay aligned with supported authoring/invocation contracts through checked references, fixtures and maintainer obligations. Covers FR-018–FR-021.

## Acceptance

Feature spec SC-001–SC-007 provide measurable acceptance. Binary upgrade alone never refreshes user copies. Removal, marketplace validate, overlays and interactive scope choice are outside this delivery.

## Related decisions

Proposed decisions: [bundled skills](adr-bundled-skills.md), [skills command](adr-skills-command.md), [safe installation](adr-skill-installation.md). Promote with final references after acceptance; no storage fields, implementation technology or fingerprints are prescribed by these product requirements.
