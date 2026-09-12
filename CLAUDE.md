<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/004-local-clone-locator/plan.md`

Active feature: **F3 — Find the Local Clone** (`specs/004-local-clone-locator/`).
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
`contracts/`. Builds on shipped **F1** (`specs/001-first-local-work/`, release 0.1),
**F2** (`specs/002-daily-cycle/`, release 0.2), and **F2.5** (`specs/003-terminal-ux-revamp/`,
Terminal UX Revamp). Stack unchanged: Go 1.26 single binary (Cobra + Bubble
Tea/Lip Gloss via `internal/present`), system `git` as a subprocess,
`modernc.org/sqlite` projection, embedded self-contained seed package (`seed/`).
Module `github.com/gustaborges/work`.

F3 makes `work start <name-or-reference>` resolve a local clone through the
Repository Resolution Policy instead of requiring an absolute path, and ships the
`work repository` surface. **No persisted-schema change** (`work-state.json`
schema 2, `work.db` `user_version = 2`, `config/work.json` keys unchanged — F1
already declared `repository_roots` and `repository_resolution.locators`). It adds
`internal/locator` (a chain-of-responsibility resolution engine — eligibility by
`accepts`, projection of accepted+present fields plus roots, `ipc.InvokeLocator`,
core-side candidate validation via `reporef.ValidatePath`, dedup) and
`internal/repoconfig` (policy & search-root operations). It wires resolution into
`internal/cli/start.go` (path → `reporef` unchanged; name/url/query →
`locator.Resolve`; ambiguity → a `present.Select` step or `repository-ambiguous`),
extends `starter.Reference`, teaches the seed Starter to classify path vs name,
and adds `work repository locator list` / `policy list|add|remove|move|replace` /
`root list|add|remove|replace`. `work repository` with no subcommand prints grouped
help and exits 0 — the interactive `repository` hub of ADR-0019 is **deferred**
past F3 (a later slice ships a full TUI). On an interactive `work start`, first run
only, the wizard prompts up front for the workspace root and at least one
repository search root, each with a purpose explanation; a search root may not
overlap the workspace and vice versa. `internal/diag` gains
`no-repository-found` (26), `no-eligible-locator` (27),
`repository-candidate-invalid` (28), `locator-failed` (29), `repository-ambiguous`
(30). Every F1/F2/F2.5 command grammar, transaction guarantee, stable stdout line,
error token, and exit code is preserved; `work start <path>` is unchanged.
Authorities: ADR-0014, ADR-0015, ADR-0016, ADR-0019, ADD §7.1–§7.2.
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**