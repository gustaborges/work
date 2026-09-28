<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/006-automatic-start-context/plan.md`

Active feature: **F5 — Automatic Context on Start** (`specs/006-automatic-start-context/`).
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
`contracts/`. Builds on **F1** (`specs/001-first-local-work/`, release 0.1),
**F2** (`specs/002-daily-cycle/`, release 0.2), **F2.5** (`specs/003-terminal-ux-revamp/`,
Terminal UX Revamp), **F3** (`specs/004-local-clone-locator/`, release 0.4, Find the
Local Clone) and **F4** (`specs/005-plugin-origins/`, New Origins via Plugin). Stack
unchanged: Go 1.26 single binary (Cobra + Bubble Tea/Lip Gloss via `internal/present`),
system `git` as a subprocess, `modernc.org/sqlite` projection, embedded self-contained
seed package (`seed/`). Module `github.com/gustaborges/work`.

F5 makes installed Linkers and Importers run when `work start` creates a Work. After
`create.Run` commits, `start:finalized` runs eligible Linker discovery, persists the links,
then runs eligible Importers through exclusive staging; an extension failure is a warning
and `work start` still exits 0 with a usable Work. A Starter's `meta`/`links` are now
consumed into the Work's first snapshot. It adds `internal/semconv` (key grammar, ownership,
the five exposed `work` facts), `internal/extension` (pure `Eligible`, the phases, failure →
warning) and `internal/staging` (read-only plan, all-or-nothing incorporation). It also
closes three gaps found in the F1–F4 code: the Importer/Linker manifest shapes now follow
ADD §4 (the F1 placeholder shapes are replaced), the registry records activation data
through one shared `plugininstall.ComponentEntry`, and a component's declared `runtime` is
finally used to invoke it and preflighted at install (`ipc.Target`). `work-state.json` stays
at schema 3 (no new field); `work.db` moves to `user_version` 3 (`work_provenance`, index-only
and not restored by a rebuild); `registry.json` gains only additive fields; `config/work.json`
gains no key. Semantic Conventions v1 is published as
`specs/006-automatic-start-context/contracts/semantic-conventions.md`. **No new exit code**:
manifest problems reuse `plugin-invalid` (31), a missing runtime `plugin-install-failed`
(34), an invalid Starter key `starter-response-invalid` (37); extension failures are
warnings with six stable `extension-*` tokens on the UI channel. Every
F1/F2/F2.5/F3/F4 command grammar, transaction guarantee, stable stdout line, error token
and exit code is preserved; `work start <path>` and the reference Starter's fallback-only
journey are unchanged, start no extension, and print nothing extra.
Authorities: ADR-0000, ADR-0002, ADR-0003, ADR-0006, ADR-0012, ADR-0013, ADD §3, §4,
§9, §10, §11.
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**