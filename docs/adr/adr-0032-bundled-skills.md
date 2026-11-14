# ADR-0032: Release-bundled vendor-neutral agent skills

**Status**: Accepted, 2026-11-14; approved by the project owner during implementation.
**Related requirements**: FR-87, FR-91 in [PRD](../prd.md).
**Realized by**: [ADD-0001](../add/add-0001-work-system-architecture.md), skill package and contract generation.

## Context

Authors need guidance matching the Work binary before they can create runtime marketplaces. Runtime packages have four component roles and no agent-skill role.

## Alternatives considered

Runtime marketplace delivery creates a bootstrap dependency; vendor marketplaces require parallel registration/manifests; network fetch permits release skew; separate vendor copies drift; documentation alone is not discovered by agents.

## Decision

Embed one canonical vendor-neutral skill tree in each Work binary. Install identical bytes for both supported local agents; common frontmatter has only name/description. Commit generated contract references so normal builds do not need generation/network. Keep an overlay seam conceptual; ship no overlay or optional vendor metadata now.

## Rationale and consequences

Release parity and offline distribution replace independent skill releases. Binary size increases by small text assets. Refresh remains explicit. Contract descriptors and focused fixtures make represented drift detectable but do not prove prose correctness. Keep maintenance/review obligations for behavior outside descriptors.
