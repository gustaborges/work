<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/005-plugin-origins/plan.md`

Active feature: **F4 — New Origins via Plugin** (`specs/005-plugin-origins/`).
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
`contracts/`. Builds on shipped **F1** (`specs/001-first-local-work/`, release 0.1),
**F2** (`specs/002-daily-cycle/`, release 0.2), **F2.5** (`specs/003-terminal-ux-revamp/`,
Terminal UX Revamp), and **F3** (`specs/004-local-clone-locator/`, release 0.4,
Find the Local Clone). Stack unchanged: Go 1.26 single binary (Cobra + Bubble
Tea/Lip Gloss via `internal/present`), system `git` as a subprocess,
`modernc.org/sqlite` projection, embedded self-contained seed package (`seed/`).
Module `github.com/gustaborges/work`.

F4 makes a plugin installed outside the core binary a first-class origin for
`work start`, including the `contribution` and `fork` modes only a plugin can
produce. `work.db` needs **no migration** (`user_version` stays 2 —
`branch_convention` is already a nullable, unconstrained column); `registry.json`
gains only the additive, unversioned `packages[]` array; `config/work.json` gains
no key. `work-state.json` moves to **schema 3** (additive over schema 2):
`start_mode` widens to `{"new","contribution","fork"}`, and `slug` +
`branch_convention` both become required-unless-`start_mode == "contribution"`
(absent exactly then — the slug/convention/prefix steps never run in
contribution mode). It adds `internal/plugininstall` (the shared install
pipeline `work plugin install <source> [--link] [--as <alias>]` and
`bootstrap.install` both use — local pinned/linked via copy-or-symlink, remote
pinned via `git clone` + pinned SHA), `internal/repoidentity` (the ADR-0011
three-layer repository identity key), and `internal/repoconv` (the generated,
per-repository convention-choice memory in
`~/.work/state/branch_conventions.json`). It extends `internal/starter` with
`Match` (pattern evaluation across every registered Starter, fallback, and an
unmemoized collision choice — ADR-0004) and widens the consumed Starter response
to `base_branch`/`start_modes` (`meta`/`links` stay deliberately unconsumed —
F5/F6 scope). It teaches `internal/create` a second materialization path
(`WorktreeAddExisting` for contribution mode, whose rollback never deletes the
pre-existing branch) and wires all of this into `internal/cli/start.go`'s SOURCE,
mode, and convention steps. It adds `work plugin install|list` (a help-only
parent, mirroring F3's `work repository` deferral) and `work convention
show|set` plus this codebase's first genuinely interactive hub (`work
convention` with no subcommand — ADR-0011/FR-030 do not defer it the way
`work plugin`'s hub is deferred to F7). `internal/diag` gains `plugin-invalid`
(31), `plugin-alias-conflict` (32), `plugin-fallback-conflict` (33),
`plugin-install-failed` (34), `starter-not-matched` (35), `starter-ambiguous`
(36), `starter-response-invalid` (37), `convention-unknown` (38). Every
F1/F2/F2.5/F3 command grammar, transaction guarantee, stable stdout line, error
token, and exit code is preserved; `work start <path>` and the reference
Starter's fallback-only journey are unchanged.
Authorities: ADR-0000, ADR-0002, ADR-0003, ADR-0004, ADR-0006, ADR-0011,
ADR-0012, ADD §4, §5, §7, §8.
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**