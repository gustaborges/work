# ADR-0022: Mandatory Marketplace Catalogs and Self-Contained Packages

**Status:** Accepted for the target contract of feature 009; implementation pending
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-16, FR-65–FR-67
**Supersession scope:** ADR-0002 direct-source installation. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

Every new installation or content replacement must be selected from an explicitly published offer, including sources containing one plugin.

## Decision

Require a root marketplace.json with a published namespace and explicit entries referring to self-contained package subdirectories with plugin.json. Git distributes catalog and packages in one commit; local sources are captured as one coherent revision. Validate the whole offer without executing components. No manifest scan, implicit catalog synthesis, root-package fallback, or external package URL is accepted.

## Alternatives considered

- Keep direct installation as an optional shortcut. Rejected in favor of the single explicit and recoverable distribution contract.
- Discover packages by scanning manifests. Rejected in favor of the single explicit and recoverable distribution contract.
- Allow catalogs of external artifact URLs. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

One publication contract prevents CLI, bootstrap, and callback bypasses. Authors must supply a minimal catalog and prepare complete packages, even for one plugin. Existing role, runtime, static manifest, containment, and external-process rules are preserved.
