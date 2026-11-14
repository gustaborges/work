<!-- SPECKIT START -->
For technologies, project structure, commands and active feature guidance, read
`specs/010-agent-skills/plan.md`.

Active feature: **010 — Agent Skills for Work Plugin Authors**. Implementation
is in progress against develop revision 161cf23. The owner approved S0;
PRD FR-87–91, ADR-0032–34 and ADD-0001 govern the feature. Remaining task and
validation evidence is recorded under specs/010-agent-skills/.
Stack remains Go 1.26/Cobra and existing portable filesystem/locking helpers;
no dependency upgrades or Work state/schema changes.

<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**
@AGENTS.md
