# ADR-0029: Official Online Marketplace as an Ordinary Explicit Git Source

**Status:** Accepted for the target contract of feature 009; implementation pending
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-67, FR-71, FR-75, FR-80
**Supersession scope:** No independent accepted decision; separates the former combined official offer. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

Project plugins need publication independent of Work releases while preserving explicit source choice and normal removal.

## Decision

Publish the official online catalog under work in an owner-confirmed Git repository, separately from builtin. Users register its actual origin explicitly. It follows ordinary Git snapshot, branch, refresh, install, update, and whole-source removal rules without special administrative restrictions. Never infer official authorship from the catalog name; attribution is based on the confirmed normalized publication origin. No automatic registration, installation, or resurrection occurs.

## Alternatives considered

- Combine online publications and included content under one namespace. Rejected in favor of the single explicit and recoverable distribution contract.
- Ship every official publication only in Work releases. Rejected in favor of the single explicit and recoverable distribution contract.
- Give the official Git source privileged lifecycle restrictions. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

The owner-confirmed publication origin and a valid first publication are required release evidence. They are not fabricated in planning and no remote publication is authorized by this documentation task. The plan includes an explicit delivery prerequisite to establish and verify that origin. Local fixture repositories prove ordinary-source semantics; no central aggregator is introduced.
