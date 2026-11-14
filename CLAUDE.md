<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/009-marketplace-distribution/plan.md`

Active feature: **Marketplace-only plugin distribution** (`specs/009-marketplace-distribution/`).
Builds on F1–F7 (`specs/008-sustainable-operation/` still defines the
administrative lock, journal, epoch and recovery rules). Design artifacts:
`plan.md`, `research.md`, `data-model.md`, `quickstart.md`, `contracts/`;
progress is in `tasks.md` and `checklists/implementation.md`.

Plugins are installed only as `plugin@marketplace` copies from registered
marketplaces (Git or local) or the code-owned `builtin` provider; there are no
aliases, link mode or embedded install. Registration and `marketplace upgrade`
change discovery only; `plugin update` applies the exact reviewed candidate from
each plugin's recorded source. Whole-source removal is one reviewed transaction.
`--remove-policy-references` is separate from `--yes`. `work plugin` and
`work plugin marketplace` open one persistent manager (Installed / Marketplaces).
Stack is unchanged: Go 1.26, Cobra, Bubble Tea/Lip Gloss through
`internal/present`, system Git, portable locks/atomic files, `modernc.org/sqlite`
for the Work index, and the embedded offline seed. Work snapshot schema and
database version remain 3; no dependency upgrade. Administrative error codes are
50–56 and 57–61 (marketplace).

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