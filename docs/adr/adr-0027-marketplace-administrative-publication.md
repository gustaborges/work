# ADR-0027: Extend the Finite Administrative Protocol for Marketplaces

**Status:** Superseded in part by ADR-0030 (previous-format decoding, schema conversion/fencing, and old-executable support only); remaining target decision accepted, implementation pending
**Superseded by:** [ADR-0030](adr-0030-clean-prerelease-marketplace-cutover.md). Original decision body retained as history.
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-52, FR-75–FR-77, FR-79, FR-82
**Supersession scope:** ADR-0002 administrative config/state realization is extended. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

Config, catalog references, composed registrations, copies, policy, and lifecycle must form coherent recoverable state under concurrent administration.

## Decision

Extend the existing finite snapshot, administration lock, epoch, journal, allowlists, package moves, and prepared/committed recovery to include marketplace configuration and catalog state. Publish fixed complete captures before any committed reference and protect referenced/in-flight captures from collection. Add explicit format versions and recover existing journals before new-format writes. Reads stay lock-free, side-effect-free, and bounded; checks prepare temporary candidates only. Source retrieval and interaction stay outside the write lock. Invocation retains revision protection without acquiring administration under a Work lock.

## Alternatives considered

- Introduce a separate catalog database. Rejected in favor of the single explicit and recoverable distribution contract.
- Write config and cache outside the transaction. Rejected in favor of the single explicit and recoverable distribution contract.
- Add independent marketplace locks and recovery. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

Every serialization, no-op, rollback, committed replay, cleanup, and invocation path needs coverage. Future formats fail closed. An administrative control-version fence is required because earlier config/registry decoders ignore unknown fields; new metadata alone cannot make old binaries safe. Downgrade or concurrent older writers after conversion is unsupported. This reuses the spec-008 protocol, not a new arbitrary-path transaction engine.
