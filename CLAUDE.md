<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/008-sustainable-operation/plan.md`

Active feature: **F7 — Sustainable Platform Operation** (`specs/008-sustainable-operation/`).
M1–M3 (T001–T037) are implemented and validated; M4–M8 remain pending.
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
and `contracts/`. Progress is in `tasks.md`; MVP evidence is in `validation.md`.

Builds on F1–F6. Stack remains Go 1.26, Cobra, Bubble Tea/Lip Gloss through
`internal/present`, system Git, existing portable locks/atomic-file helpers,
`modernc.org/sqlite` for the existing Work index, and the embedded offline seed.
Work snapshot schema and database version remain 3; no dependency upgrade.

F7 completes plugin enable/disable/update/uninstall and administrative hubs.
Pure administrative reads never bootstrap, repair, create locks or application
state. Mutations use one administrative lock, a finite before/after journal,
and a monotonic stable/pending epoch; all existing config/registry/convention
memory writers must participate. Review/fetch happens outside the lock.
Update applies the exact reviewed candidate; opaque versions compare only for
equality. Updates commit per package; selected uninstall commits as a set.
`--remove-policy-references` is separate from `--yes` confirmation.
Durable seed removal survives bootstrap/upgrades; `plugin install --embedded`
explicitly restores it offline without policy insertion, even on fresh state.
Pinned snapshots reject escaping/cyclic symlinks and escaping entrypoints;
linked sources remain developer-owned. Unavailable conventions remain stored
until explicitly replaced. New administrative errors occupy codes 50–56.

Protect component invocation through current-revision validation and execution
under the administrative lock; release before Work/branch locks. Never acquire
administration under a Work lock; post-run checks use guarded pure snapshots.
Keep existing F5 automatic warning and F6 manual failure/commit guarantees.

Authorities: roadmap F7/§4; PRD FR-18–FR-20, FR-25, FR-40–FR-41, FR-45,
FR-47–FR-48, FR-50–FR-64; accepted ADR-0002/0003/0005/0011/0012/0015/0019–0021.
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**