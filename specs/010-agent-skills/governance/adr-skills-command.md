# Proposed ADR: Explicit plural skills setup command

> Historical S0 draft. Current accepted interface: [ADR-0034](../../../docs/adr/adr-0034-complete-skill-collection.md).

**Status**: Proposed; final numeric ID assigned at S0.
**Related requirements**: AS-R2, AS-R4 in [PRD amendment](prd-amendment.md).
**Realized by**: [ADD amendment](add-amendment.md), command adapter.

## Context

Accepted ADR-0019 requires singular resources. The reviewed proposal prefers the conventional agent ecosystem term `skills`. That is an explicit decision conflict, not permission to reinterpret the existing rule.

## Alternatives considered

`work skill` follows the existing convention; `work agent-skills` is longer; implicit current-directory/global selection risks writing to the wrong scope; a prompt adds a second interface.

## Decision

Propose a narrow plural exception for `work skills install/status`, in Setup, requiring exactly one of global/folder scope and defaulting to both vendors. No interactive hub or aliases. Status supports JSON; install explicitly rejects it. All unrelated ADR-0019 discovery, stream and grammar rules continue.

## Rationale and consequences

The noun matches the installed collection and permits multiple skill operands. It is a deliberate exception with a consistency cost. S0 must explicitly approve/promote this decision and record partial supersession/exception of ADR-0019 in repository convention. Until then all examples/contracts are conditional drafts; do not implement the conflicting surface. If reviewers choose singular, update all feature contracts/examples/tasks together before implementation.
