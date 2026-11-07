# ADR-0028: Distribution-Provided Builtin Marketplace and Default Lifecycle

**Status:** Superseded in part by ADR-0030 (earlier-installation retention and adoption/migration clauses only); remaining target decision accepted, implementation pending
**Superseded by:** [ADR-0030](adr-0030-clean-prerelease-marketplace-cutover.md). Original decision body retained as history.
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-10, FR-24, FR-41, FR-78–FR-79
**Supersession scope:** ADR-0003 seed source representation; ADR-0002 embedded direct installation. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

First use must remain offline and removable without making default domain behavior core code or confusing it with independent online publications.

## Decision

The binary provides a reserved builtin marketplace and initial work-reference entry, accessed through an internal distribution provider. No external editable catalog or author field grants provider authority. Resolve work-reference@builtin through the same entry validation, copy, and publication pipeline for bootstrap and explicit add. Only genuinely fresh required setup initializes default policy once. Record whether policy is untouched, explicitly chosen (including empty), or unknown legacy state; source registration on absent config preserves fresh-policy intent, while config existence alone never grants or suppresses seeding. Unknown legacy intent never authorizes reseeding. Ambiguous legacy package ownership remains inspectable and cannot control builtin lifecycle. Explicit add, repair, reinstall, reads, catalog refresh, and binary upgrades do not reseed policy. Durable disablement/removal history is outside removable plugin storage. Users may remove/toggle individual plugins but cannot remove or replace the provider. Legacy reference installations remain legacy until explicit remove/add; no automatic ownership adoption is introduced.

## Alternatives considered

- Keep a privileged direct embedded installation mode. Rejected in favor of the single explicit and recoverable distribution contract.
- Make an extracted editable local source authoritative. Rejected in favor of the single explicit and recoverable distribution contract.
- Merge included and online offers under one identity. Rejected in favor of the single explicit and recoverable distribution contract.
- Remove offline bootstrap. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

Distribution resources and build packaging gain a catalog but default components remain external subprocesses. New releases can offer new content without silently replacing installed copies. Existing first-use, removal-intent, and policy preservation principles in ADR-0003 survive; the old seed-origin installation interface is superseded. Included origin is a lifecycle fact, not tamper resistance for installed files.
