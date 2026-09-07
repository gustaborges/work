<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/002-daily-cycle/plan.md`

Active feature: **F2 — Daily Cycle: Resume and Archive** (`specs/002-daily-cycle/`).
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
`contracts/`. Builds on shipped **F1** (`specs/001-first-local-work/`, release 0.1).
Stack: Go 1.26 single binary (Cobra + Bubble Tea/`huh`), system `git` as a
subprocess, `modernc.org/sqlite` projection, embedded self-contained seed
package (`seed/`). Module `github.com/gustaborges/work`.

F2 adds `work resume [id]` and `work archive [id...]`, evolves `work-state.json`
to schema 2 (`status` gains `archived`, new `archived_at`, mutable
`last_accessed_at`) and `work.db` to `user_version = 2`, and adds
`internal/reconcile` (rebuild + reconcile the projection from canonical
snapshots). The F1 `work start` journey is unchanged.
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**