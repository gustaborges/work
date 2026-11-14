# Proposed ADR: Recoverable filesystem-local skill installation

**Status**: Proposed; final numeric ID assigned at S0.
**Related requirements**: AS-R3, AS-R4 in [PRD amendment](prd-amendment.md).
**Realized by**: [ADD amendment](add-amendment.md), installation/inspection flows.

## Context

Agent skill folders live outside Work home, can contain user edits and may overlap across installer processes. Portable rename cannot overwrite a nonempty directory. Work administrative state must remain untouched.

## Alternatives considered

In-place copying exposes partial trees; delete-then-copy loses old content; a simple rename assumes unsupported replacement; using Work admin journal violates independence; no ownership record cannot distinguish update from user edits.

## Decision

Propose marker-based content ownership with a portable mode-independent tree digest, filesystem-local persistent locks, complete preflight and per-target retained-backup recovery records. Before commit recovery restores old content/absence; after commit it retains verified new content and completes cleanup. Status only observes and retries boundedly. Force applies only to ordinary modified/foreign trees. Records validate sibling ownership; unknown/mismatched evidence is preserved.

## Rationale and consequences

Individual targets are recoverable, not one cross-vendor transaction. Earlier commits survive a later failure; cleanup-pending is explicit. A short rename gap exists for external readers. Locks coordinate cooperating Work processes, not malicious writers. Process-interruption recovery is tested; arbitrary power/media loss is not guaranteed. Coordination artifacts are local-only and not part of payloads to commit.
