# ADR-0023: Fixed Discovery Snapshots and Independent Installed Copies

**Status:** Accepted for the target contract of feature 009; implementation pending
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-70–FR-74, FR-78
**Supersession scope:** ADR-0002 linked installation; ADR-0005 candidate acquisition only. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

Browsing an offer must not mutate installed content, and selecting an entry must reproduce what was browsed despite changes to its author tree.

## Decision

Separate a fixed discovery snapshot from each independent managed execution copy. Git snapshots pin an exact commit on the recorded branch; local snapshots capture distributable bytes under a deterministic digest; the included provider obtains bytes from distribution resources. Install only the chosen package. Refresh publishes discovery only. Update obtains a temporary fixed candidate from recorded provenance, without refreshing discovery. Equal opaque versions remain unchanged; explicit add can reinstall changed content at the same version.

## Alternatives considered

- Install from the live remote tip or author tree. Rejected in favor of the single explicit and recoverable distribution contract.
- Execute from shared discovery storage. Rejected in favor of the single explicit and recoverable distribution contract.
- Retain development links for new installations. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

This preserves opaque version and exact-reviewed-candidate guarantees from ADR-0005 while changing source acquisition. Local captures cost storage and require protected cleanup; Git installation may need retrieval of the pinned commit and must fail rather than fall forward. Digests identify revisions, not signatures or ongoing integrity guarantees.
