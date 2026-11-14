# ADR-0026: Whole-Marketplace Removal with Exact Impact Authorization

**Status:** Accepted for the target contract of feature 009; implementation pending
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-20, FR-48, FR-72, FR-74–FR-77
**Supersession scope:** ADR-0002 package-only removal scope is extended. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

Removing a registered source must remove its installed plugins without unreviewed dependency changes or inconsistent orphan state.

## Decision

Review the registered source, discovery reference, every installed plugin from it including disabled and withdrawn entries, and exact affected dependencies without contacting the origin. Publish removal of that complete set and separately authorized policy references as one administrative transaction. Confirmation does not authorize dependency deletion by itself. Revalidate the reviewed source, full installation set, and dependencies at commitment; any relevant drift conflicts rather than widening the set. Preserve Works and convention memory. Release the namespace only after commitment; registering it again does not reassociate history. Removing one plugin leaves its source and siblings.

## Alternatives considered

- Remove only the registration and leave installed orphans. Rejected in favor of the single explicit and recoverable distribution contract.
- Block source removal while any plugin is installed. Rejected in favor of the single explicit and recoverable distribution contract.
- Require the user to remove plugins individually first. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

Whole-source removal is a destructive capability at execution time and therefore needs exact review and dependency authorization. It follows existing pre-commit preservation and post-commit cleanup semantics. It applies equally to the online official Git source; the included provider cannot be removed.
