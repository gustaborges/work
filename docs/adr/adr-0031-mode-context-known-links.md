# ADR-0031: Select Starter Context by Mode and Skip Known-link Discovery

**Status:** Accepted
**Date:** 2026-11-14
**Product Context:** PRD FR-84–FR-86, FR-26/FR-32/FR-35; issue #61
**Realized by:** ADD §§7/9/11; specs/010-link-improvements/

## Context

A Starter executes before the creation mode is chosen, yet a source PR has different meaning for contribution and fork. Automatic Linkers can then replace explicitly supplied context with guesses. We need mode-specific publication and preservation of known links without broadening subprocess inputs or changing manual association.

## Decision

Add optional mode-indexed contextual output to the single Starter response. Validate all entries before materialization, overlay only the selected mode's links/meta, then use existing binding and atomic creation. Keep the Starter input unchanged. Add a static known-output-key exclusion to automatic start Linker eligibility using the current provider/source-instance storage identity, before subscription diagnostics. Retain the fixed Linker phase and later Importer evaluation.

## Alternatives Considered

- Invoke the Starter again after choosing the mode: adds side effects, latency, and inconsistent resolutions; rejected.
- Make the Starter choose the mode: moves a user/core decision into an external component; rejected.
- Skip overwrites at publication time: still runs unnecessary scripts and emits traces; rejected.
- Re-evaluate after each Linker: introduces order-dependent eligibility and inputs; rejected.
- Require plugin-only checks: cannot uniformly protect context already published by the Starter; rejected.

## Rationale

The change stays within the typed external-component model and reuses existing key ownership, persistence, phase orchestration, and mode selection. It implements FR-84–FR-86 without a new mode-aware invocation protocol or persisted schema.

## Consequences

Starter authors may distinguish source and contribution relationships using one response. Invalid unused entries are rejected early. Automatic discovery no longer refreshes an already-known link at start; manual association remains an explicit replacement path. Multiple Linkers eligible for an initially missing key keep existing upsert behavior. Missing-key guesses remain a plugin responsibility, tracked separately in work-cli-marketplace#7.

This refines automatic eligibility without superseding ADR-0000/0006/0012/0013/0024: process trust, typed output, semantic namespaces, and provider ownership remain intact.
