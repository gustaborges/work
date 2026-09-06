<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/001-first-local-work/plan.md`

Active feature: **F1 — First Local Work** (`specs/001-first-local-work/`).
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
`contracts/`. Stack: Go 1.26 single binary (Cobra + Bubble Tea/`huh`),
system `git` as a subprocess, `modernc.org/sqlite` projection, embedded
self-contained seed package (`seed/`). Module `github.com/gustaborges/work`.
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**