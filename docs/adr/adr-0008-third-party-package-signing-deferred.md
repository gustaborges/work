# ADR-0008: Third-Party Package Signing/Verification — Deferred, with Rationale Recorded

**Status:** Accepted (decision to defer, with rationale recorded)
**Date:** 2026-08-23
**Product context:** `docs/prd.md` — Section 12 (named risk), RNF-7
**Governs:** `docs/add/add-0001-work-system-architecture.md`, Section 12

## Context

The PRD names "a malicious or poorly written external plugin compromises user data" as a risk, and RNF-7 treats minimizing trust as a core value. The question was whether a package signing/verification policy needs to be decided already in v1.

## Decision

Explicitly out of scope for v1 — for a specific reason, not merely "defer it": under the v1 installation model (ADR-0002), a plugin's source is always a URL or local path explicitly chosen by the user, and the content reference is pinned at installation time. This already provides sufficient integrity and reproducibility for that specifically user-chosen source — there is currently no mechanism in the product that installs a plugin without the user having directly provided the source. Without such a mechanism, there is no additional relevant gap that a signature would close: the user is already the one deciding which source to trust.

This decision should be revisited if, in the future, Work introduces any mechanism that installs or suggests a plugin without the user having directly provided the source — at that point, the user's trust decision would no longer be "I chose this source" and would instead depend on how much they trust that intermediary mechanism, opening a class of risk that merely pinning the content reference does not address.

## Alternatives considered

* **Decide and implement signing already in v1.** Rejected: it would absorb complexity without closing any real risk gap today, since the current installation model already depends entirely on a source explicitly chosen by the user.
* **Ignore the topic indefinitely without recording the rationale.** Rejected: it would lose the reasoning already established, forcing a fresh reassessment in the future without context for why the decision was made.

## Consequences

**Positive:** v1 does not absorb complexity that closes no real risk gap in the current installation model; the rationale is recorded so it does not need to be derived again from scratch if the assumptions change.

**Negative / trade-offs:** none for v1 — the risk that would motivate signing (a plugin source not directly chosen by the user) simply does not exist in the product today.
