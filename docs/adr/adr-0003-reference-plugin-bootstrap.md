# ADR-0003: Bootstrap of Reference Plugin Package on First Use

**Status:** Accepted
**Date:** 2026-08-23
**Product context:** `docs/prd.md` — RF-10, RF-24, Section 2 (motivation), RNF-1, RNF-7
**Governs:** `docs/add/add-0001-work-system-architecture.md`, Section 6

## Context

A 100% empty core makes `work start` "out of the box" do nothing, contradicting the product's very value proposition (eliminate setup friction — PRD Section 2). Embedding default resolution logic as core code would directly violate RNF-1/RNF-7 and the external-process execution principle (ADR-0000) — domain behavior would exist within the minimal trust surface, unable to be uninstalled or replaced.

## Decision

An official reference package contains, at minimum, a fallback Starter for local repository references, a filesystem-based Repository Locator, and a `freeform` branch convention (ADRs 0012, 0014, and 0015). It is automatically installed on first use, using **exactly the same installation mechanism** as any third-party plugin (ADR-0002) — never as core code. To avoid network dependency, the package is embedded in binary/release assets as a *seed*, but is still "installed" through the normal pipeline (writes manifest, registry, and entrypoint — see `add-0001` §6); the default Locator initially enters the global policy. The user can uninstall this package like any other, and it truly disappears without leaving phantom behavior in the core.

## Alternatives considered

* **A — 100% empty core.** Maximum purity with respect to RNF-1/RNF-7, but poor onboarding — rejected.
* **B — default logic embedded in core.** Zero friction, but directly violates ADR-0000/RNF-1/RNF-7: domain behavior living in the surface that users must trust without review, not auditable, not uninstallable — rejected.

## Consequences

**Positive:** onboarding works immediately without violating architecture; also serves as *dogfooding* of the manifest/registry contract itself.

**Negative / trade-offs:** increases binary/release size and requires keeping the seed compatible with the plugin contract — cost accepted to ensure functional first use, offline and without domain logic embedded in core.
