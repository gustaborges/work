# ADR-0025: Marketplace Commands Under Plugin and Qualified Selectors

**Status:** Accepted for the target contract of feature 009; implementation pending
**Date:** 2026-11-07
**Related requirements:** [PRD](../prd.md) — FR-50–FR-64, FR-69, FR-71, FR-76, FR-83
**Supersession scope:** ADR-0019 plugin administration grammar only. Unaffected earlier decisions remain in force.
**Realized by:** [ADD-0001](../add/add-0001-work-system-architecture.md) §§4–6, 7–9, 12; [feature 009](../../specs/009-marketplace-distribution/plan.md).

## Context

The public command tree must distinguish source discovery from installed-package maintenance while exposing equivalent persistent-manager journeys.

## Decision

Expose plugin add|list|remove and plugin marketplace add|list|upgrade|remove. Retain plugin enable|disable|update. Accept PLUGIN@MARKETPLACE or an unqualified PLUGIN with an explicit marketplace flag, never both or mixed batch forms. Remove install/uninstall, direct SOURCE installation, alias/link/embedded flags, and a root marketplace family without compatibility aliases. Catalog upgrade and plugin update are different actions. The manager has Installed and Marketplaces, with no special Official destination or per-source tabs; every direct capability has an equivalent manager path and authorization. Retain the existing semantic theme, persistent-session feedback, and normal-exit acknowledgment.

## Alternatives considered

- Add a root marketplace command family. Rejected in favor of the single explicit and recoverable distribution contract.
- Overload install SOURCE to mean both packages and catalogs. Rejected in favor of the single explicit and recoverable distribution contract.
- Resolve bare names automatically. Rejected in favor of the single explicit and recoverable distribution contract.
- Preserve the old direct-install and Official screens. Rejected in favor of the single explicit and recoverable distribution contract.

## Rationale and consequences

This is an intentional script/help/JSON breaking change, requiring migration guidance and fixtures. ADR-0019 static home/grouped help, unchanged commands, automation streams, and absence of aliases remain in force. ADR-0020 presentation isolation and ADR-0021 single full-screen lifecycle remain; persistent manager Back/current feedback follow the established spec-008 manager contract rather than interview receipt replay.
