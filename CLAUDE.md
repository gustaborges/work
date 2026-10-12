<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/007-on-demand-context/plan.md`

Active feature: **F6 — On-Demand Context** (`specs/007-on-demand-context/`).
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
`contracts/`. Implementation is complete; milestone and validation evidence is in `tasks.md`. Builds on F1–F5, including
`specs/006-automatic-start-context/` for input projection, semantic keys,
external component execution, atomic link publication and safe import staging.
Stack remains Go 1.26, Cobra, Bubble Tea/Lip Gloss through `internal/present`,
system Git, `modernc.org/sqlite`, and the embedded offline seed package.

F6 adds `work status [WORK] [--json]`, `work link [LINKER] [VALUE]` and
`work import [IMPORTER]`. Status reads canonical snapshots without bootstrap,
index repair, extensions, access updates or shell repositioning. Link associates
exact non-empty text without a plugin subprocess; import invokes only the
selected eligible manual provider and shares F5 staging/IPC contracts.
The design adds pure `internal/workcontext` resolution, retained optional
package enablement (legacy default true), shared manual eligibility/publication/
import helpers, and cancellable artifact application with rollback ownership
retained through cleanup and commitment. Package management commands remain F7.
Snapshot schema and database version remain 3; manual provenance uses operation
`link`. Manual failures return the documented command errors; automatic start
failures retain existing warnings and success behavior. All prior command and
presentation contracts remain binding.

Authorities: PRD FR-30/31/38 and related integrity/UX requirements; ADD §3, §4,
§9–§12; accepted ADR-0000, ADR-0006, ADR-0012, ADR-0013, ADR-0019–ADR-0021.
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**