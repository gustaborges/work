# ADR-0024: Marketplace Namespaces and Composite Provider Identity

**Status:** Superseded in part by ADR-0030 (earlier-provider/reference compatibility clauses only); remaining target decision accepted, implementation pending
**Superseded by:** [ADR-0030](adr-0030-clean-prerelease-marketplace-cutover.md). Original decision body retained as history.
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-21, FR-35, FR-40–FR-48, FR-68–FR-69, FR-72, FR-81
**Supersession scope:** ADR-0002 global alias uniqueness; ADR-0012 package/component qualification; ADR-0011 shared convention-provider identity; ADR-0013 private provider namespace only. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

Homonymous plugin names must coexist without redirecting execution, policy, memory, or maintenance to a different provider.

## Decision

Use the canonical published marketplace namespace with no local rename. Public plugin identity is plugin@marketplace; component and convention references use plugin@marketplace/contribution. Persist structured provider identity and actual source provenance instead of reconstructing either from display strings. Bind retained convention references and core-only private stored keys to an opaque registration instance, stable across refresh/idempotent add and renewed on remove/re-add (builtin uses one stable code-owned instance). Reused public names therefore cannot reclaim historical choices/private values. This instance is continuity evidence, not a trust token. Names are unique within the appropriate scope. Optional display_name is passive presentation metadata. Bind contribution selection, policy, convention memory, subscription resolution, private data scope, dispatch, and revision guards to that composed provider. Bare contribution names are convenience selectors only when unambiguous; plugin selectors always specify marketplace. Legacy provider references retain their earlier meaning and never bind to a new provider by name coincidence.

## Private scope and subscription binding

Author-relative private declarations and IPC keys remain relative to the plugin. Core binds persisted keys to the composed owner; explicit qualified inputs select that owner exactly. Percent-encoding dots in owner names prevents dotted name ambiguity and allows dotted plugin names to own private keys. Qualified private publication is rejected; public semantic keys remain unchanged. New self-package Starter filters bind to their provider; external bare filters require one unambiguous eligible provider, otherwise no execution and an ambiguity diagnostic. Legacy filters retain their prior semantics. The precise grammar is in feature 009 provider-identity.md.

## Alternatives considered

- Preserve globally unique local plugin aliases. Rejected in favor of the single explicit and recoverable distribution contract.
- Allow user-renamed marketplace namespaces. Rejected in favor of the single explicit and recoverable distribution contract.
- Infer a marketplace from the only current match. Rejected in favor of the single explicit and recoverable distribution contract.
- Merge equal convention names across unrelated providers. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

All provider-bearing records and resolvers must change together. Exclusive contribution rules still apply, so two fallback Starters can conflict despite valid distinct identities. Historical Works and public Semantic Convention keys remain unchanged; private provider scope and new provenance become unambiguous. Existing convention repository identity and unavailable-memory behavior remain valid.
