# ADR-0030: Clean Pre-Release Marketplace Cutover

**Status:** Accepted for feature 009; implementation pending
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) FR-16, FR-21, FR-65–FR-79; NFR-1–NFR-3.
**Supersedes in part:** ADR-0024's earlier-provider/reference branches, ADR-0027's previous-format decoding/conversion/fencing, and ADR-0028's earlier-installation retention/adoption policy. Their marketplace identity, current transaction safety, and current default-lifecycle decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6 and [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

The CLI has not been released. The owner explicitly prefers a clean, purposeful codebase and accepts breaking existing development plugins and state. The initial feature plan inferred continuity obligations that would add multiple runtime identities, old/new format readers, migration selectors, journal conversion, and tests unrelated to the desired product.

## Decision

Define a single marketplace-only runtime, command surface, identity model, and current persistence schema. Remove superseded standalone/link/embedded-mode publishers, global aliases, previous format readers, compatibility selectors, adoption/migration flows, and compatibility-only fixtures/tests. Update active callers, plugin examples, tests, and documentation to the intended contract rather than preserving superseded behavior through adapters.

Use one strict current decoder and one current journal/recovery model. Ordinary invalid/unsupported input fails validation; there is no special older-state recognition, conversion, old-executable fence, or downgrade support subsystem. Developer validation uses fresh disposable state. This decision does not authorize deleting a real development home during planning or implementation.

Retain safety required by the current product: atomic publication, current-journal recovery, leases, exact candidate and dependency review, source-registration continuity against namespace reuse, independent copies, pure reads, and deliberate default removal/one-time policy handling. Keep source publication identity distinct from executable trust. These are current-domain guarantees, not compatibility machinery.

## Alternatives considered

- Retain copied/linked development installations behind a legacy selector. Rejected: preserves an additional distribution and execution contract with no released-user obligation.
- Read previous formats and convert them on mutation. Rejected: makes storage/recovery carry permanent branches for disposable development state.
- Introduce old-executable tests, dual journal decoders, and schema conversion fencing. Rejected: does not serve the unreleased product's current execution model.
- Drop transaction/recovery safety together with compatibility. Rejected: interruption and concurrency affect every new installation and remain product requirements.

## Consequences

Existing development plugins/state may stop working and may need to be recreated outside this feature. No reset/migration command or automatic state deletion is added. New code, fixtures, examples, and active documents describe only marketplace sources and qualified providers. Persistence schema labels identify the current format; version numbers do not imply support for earlier shapes.

The original decision bodies in superseded ADRs remain documentary history. Their compatibility clauses have no authority over feature 009 after this decision. The PRD, ADD, specification, contracts, plan, and milestone tasks must reflect this clean cutover before implementation starts.
