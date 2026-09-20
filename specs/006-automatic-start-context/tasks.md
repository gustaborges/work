---
description: "Task list for F5 — Automatic Context on Start"
---

# Tasks: Automatic Context on Start (F5)

**Input**: Design documents from `specs/006-automatic-start-context/`
**Prerequisites**: `plan.md`, `spec.md` (US1–US7, FR-001–FR-046, SC-001–SC-011), `research.md`
(R1–R17), `data-model.md`, `contracts/` (`plugin-manifest-extensions.md`,
`extension-protocol.md`, `semantic-conventions.md`, `starter-protocol.md`,
`cli-work-start.md`, `index-provenance.md`), `quickstart.md` (S1–S15)

**Tests**: Included — the plan mandates unit, contract, `testscript` and PTY coverage for every
new package and behavior. Test tasks are first-class and precede the code they cover within a
story.

**Organization**: Phases follow the plan's Branching Strategy (one
`feature/006-automatic-start-context-p<n>-*` branch per phase group, each cut from the previous
tip). Story order is spec priority (US1–US3 P1, US4–US6 P2, US7 P3), which is also dependency
order: Starter context (US2) and Linker discovery (US3) exist before Importers (US4) consume
them, collision hardening (US5) builds on the Importer happy path, and failure/visibility
(US6, US7) are properties over everything before.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: US1–US7, only in user-story phases
- File paths are relative to the repository root

## Path & module conventions

- Single Go module `github.com/gustaborges/work`, Go 1.26. No new dependency, binary, exit
  code, flag, stdout line or `config/work.json` key.
- **New packages**: `internal/semconv/` (leaf, imports nothing internal), `internal/extension/`,
  `internal/staging/`. `extension` and `staging` never import `cli` or `present`.
- **Edits**: `internal/plugin/manifest.go`, `internal/registry/registry.go`,
  `internal/plugininstall/install.go`, `internal/bootstrap/bootstrap.go`, `internal/ipc/ipc.go`,
  `internal/starter/starter.go`, `internal/locator/chain.go`, `internal/create/create.go`,
  `internal/projection/projection.go`, `internal/reconcile/reconcile.go`,
  `internal/workhome/workhome.go`, `internal/archive/`, `internal/resume/` (lock-key
  extraction only), `internal/diag/diag.go`, `internal/present/diagrender/`,
  `internal/cli/start.go`, `internal/cli/diagnostics_border.go`.
- **Persisted schemas**: `work-state.json` stays schema 3; `work.db` → `user_version` 3
  (`work_provenance`); `registry.json` gains additive fields only.
- **Comment rule** (project memory + `CLAUDE.md`): plain godoc explaining *why*; **no phase
  numbers, task IDs or spec IDs in source comments**.
- **Test rule**: run Go tests as `GIT_CONFIG_GLOBAL=/dev/null go test ./...` (sandbox git signing
  otherwise breaks `gittest` repository creation).
- **Existing tests that are edited, not weakened**: F1 placeholder manifest-shape cases
  (`internal/plugin`, `internal/registry`, `internal/plugininstall`), `ipc` signature callers,
  and the `user_version` 2 → 3 assertions (`internal/projection/projection_test.go`,
  `internal/reconcile/reconcile_test.go`, `tests/integration/integration_test.go`). Every other
  F1–F4 test passes unmodified (SC-008).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Branch and the fixture packages every later phase's tests build on.
Branch: `feature/006-automatic-start-context-p1-foundational` (shared with Phase 2), cut from
`feature/f5-automatic-context` tip.

- [X] T001 Confirm base/target branch with the user, then `git switch feature/f5-automatic-context && git pull && git switch -c feature/006-automatic-start-context-p1-foundational`. Also confirm F4 has merged or that the branch is cut from F4's tip (plan prerequisite).
- [X] T002 [P] Create the `context-suite` fixture package in `tests/fixtures/plugins/context-suite/`: one Go `main.go` that picks its role from its own binary name (`starter`, `linker`, `linker2`, `importer`, `importer2`), reads `WORK_FIXTURE_MODE` as a comma-separated list of `<entrypoint>-<mode>` tokens (each entrypoint reads only tokens carrying its own name, defaulting to plain success), and appends `{role, stdin}` to the file named by `WORK_FIXTURE_LOG` on every launch. Ship a `plugin.json` declaring one Starter, two Linkers sharing key `github.pull_request` (automatic discovery at `start:finalized`, restricted to that Starter, one `work:worktree_path` input), and two Importers (`on` `start:finalized` restricted to that Starter, required `link:github.pull_request`, optional `work:start_mode`, `manual` display text). Modes to implement: Starter `starter-context` (returns `meta {"github.pull_request.number": 212}` + `links {"github.pull_request": "https://example.test/pr/212"}`), `starter-bad-key` (`links {"GitHub.PR": "x"}`), `starter-foreign-private` (`meta` key `plugin.someone-else.x`); Linker `linker-value`, `linker-none`, `linker-empty-value`, `linker-exit1`, `linker-garbage`, `linker-hang`, `linker2-value`; Importer `importer-ok` (writes `notes/context.md`), `importer-empty`, `importer-mixed`, `importer-symlink`, `importer-into-worktree`, `importer-over-state`, `importer-exit1`, `importer-garbage`; `importer2-collide` (writes the same relative path as `importer-ok`).
- [X] T003 [P] Create the `restricted-suite` fixture package in `tests/fixtures/plugins/restricted-suite/`: a Linker restricted to a different Starter, a Linker with `discover.automatic: false`, a Linker subscribed to no matching event, an Importer requiring an absent link, and a manual-only Importer — each logging every launch to `WORK_FIXTURE_LOG` (they must never start).
- [X] T004 [P] Create manifest-only invalid fixtures in `tests/fixtures/plugins/`: `invalid-event/` (subscription to an undefined event), `invalid-input/` (malformed input string and a `work:` fact outside the exposed set, as two manifests or two components), `invalid-dup-input/` (`meta:x.y` and `link:x.y`), `invalid-key-owner/` (Linker `key` private to another plugin), `invalid-manual/` (`manual` without `display_name`, and `manual: true`), `invalid-runtime/` (`"runtime": "no-such-interpreter"`).
- [X] T005 [P] Create the Unix-only `runtime-sh` fixture in `tests/fixtures/plugins/runtime-sh/`: a Starter `starter.sh` (no executable bit) declared with `"runtime": "sh"`, returning a valid Repository Reference.
- [X] T006 Extend `tests/fixtures/plugins/plugins.go` with helpers to build a fixture package into an installable directory (copy `plugin.json`, `go build` one binary per distinct entrypoint) so `context-suite` and `restricted-suite` are reusable from `tests/contract/` and `tests/integration/`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The leaf and shared types every story depends on: key grammar, the ADD §4 manifest
shapes, registry activation data, runtime-aware invocation, index provenance, the non-fatal
warning type. No CLI wiring in this phase.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### `internal/semconv` (leaf)

- [X] T007 [P] Write table tests in `internal/semconv/semconv_test.go`: `ValidKey` (valid public keys; invalid — uppercase, hyphen, empty segment, single segment, reserved `plugin`/`work` namespaces, `plugin.`/`work.` prefix that is not a valid private key); `ValidatePublished(owner, key)` (private key owned by `owner`, foreign private key rejected, a dotted plugin name cannot own private keys but may publish public ones); `ValidLinkValue` (non-empty string only); `Facts` equals `worktree_path, start_mode, branch, base_branch, slug`.
- [X] T008 Implement `ValidKey`, `ValidatePublished`, `ValidLinkValue` and `Facts` in `internal/semconv/semconv.go` per `contracts/semantic-conventions.md` §1–§3 and `research.md` R4, with godoc on every exported name.

### Manifest model (`internal/plugin`)

- [X] T009 [P] Update and extend `internal/plugin/manifest_test.go`: replace the F1 placeholder cases (`"on":["clone"]`, `"manual":true`, `"discover":["x"]`, `"work:slug"`) with ADD-shaped cases; add cases for rules M1–M7 of `contracts/plugin-manifest-extensions.md` (undefined event, empty/malformed `starters`, empty `manual.display_name`, malformed input, `work:` fact outside the set, duplicate key across namespaces, Linker key ownership incl. dotted manifest name, `automatic: true` without `on` accepted, unknown fields inside `on[]`/`manual`/`discover` rejected, top-level `inputs` on a Linker still forbidden); assert the ADD §4 example manifest parses and the old placeholder shapes are rejected.
- [X] T010 Implement in `internal/plugin/manifest.go`: types `Subscription{Event, Starters}`, `Manual{DisplayName, Description}`, `Discover{Automatic, On, Inputs}`, `Input{Source, Key, Optional}`; change `Component.On`/`Manual`/`Discover`; add `Component.Key`/`Inputs` handling; `ParseInput`; `CoreEvents` with `EventStartFinalized`; validation rules M1–M7 per `data-model.md` §1, using `semconv`; all failures are `plugin-invalid` (31).

### Registry and runtime-aware invocation

- [X] T011 [P] Extend `internal/registry/registry_test.go`: new fields (`On`, `Manual`, `Inputs`, `Key`, `Discover`, `Package.PluginName`) round-trip through `Save`/`Load`; `Component.Target` and `EntrypointPath` (`.exe` appended only when `Runtime == ""`, never for a script); `QualifiedName`; `PluginNameOf` falls back to the alias when `PluginName` is empty; `Extensions(role)` returns only Importers/Linkers; a legacy entry without the new fields loads and is inert.
- [X] T012 Implement in `internal/registry/registry.go`: additive `Component` fields and `Package.PluginName` per `data-model.md` §2, `Component.Target(pluginsDir) ipc.Target`, fix `EntrypointPath` to append `.exe` only without a runtime, `Component.QualifiedName()`, `Registry.PluginNameOf(alias)`, `Registry.Extensions(role)`.
- [X] T013 [P] Extend `internal/ipc/ipc_test.go`: `Target` with a runtime (`sh` on Unix) and without; cancellation of the context kills the child; the stderr tail is capped at 4 KiB; existing tests keep passing through the adapted signatures.
- [X] T014 Implement in `internal/ipc/ipc.go`: `Target{Runtime, Path}`; `RunContext(ctx, Target, stdin)` using `exec.CommandContext` (runs `<runtime> <path>` when a runtime is declared, the path directly otherwise, never relying on shebang/exec bit); capped stderr tail; `InvokeLinker`/`InvokeImporter` with their input/response types per `contracts/extension-protocol.md` §4; change `InvokeStarter`/`InvokeLocator` to take a `Target`.
- [X] T015 [P] Update `internal/starter/starter.go` (`Invoke`) and `internal/locator/chain.go` to pass `comp.Target(pluginsDir)` — mechanical, no behavior change; fix any callers/tests broken by the signature change.

### Index provenance (`work.db` schema 3)

- [X] T016 [P] Write tests in `internal/projection/projection_test.go` and `internal/reconcile/reconcile_test.go`: migration 2 → 3 creates `work_provenance`; `Reset` drops it before `works`; `Delete` cascades; `Upsert` does **not** cascade; `UpsertWithProvenance` is one transaction (a failing provenance insert leaves no `works` row); `RecordProvenance` is last-source-wins on `(work_id, section, key)`; `Provenance(workID)` orders by `(section, key)`; a rebuild from snapshots restores Works but not provenance; update the `user_version` assertions 2 → 3 (also in `tests/integration/integration_test.go`).
- [X] T017 Implement in `internal/projection/projection.go`: `SchemaVersion = 3`, migration 2 → 3 per `contracts/index-provenance.md`, `Reset` dropping `work_provenance` first, `Provenance` type, `UpsertWithProvenance`, `RecordProvenance`, `Provenance(workID)`. In `internal/reconcile/reconcile.go` update docs only (rebuild restores Works, not provenance).

### Shared lock, warning type, rendering

- [X] T018 [P] Add `Home.WorkLockPath(id)` to `internal/workhome/workhome.go` (one derivation of the per-Work lock key) and switch `internal/archive/` and `internal/resume/` to it with the **same key** as today; add a test proving the derived path equals what both previously computed.
- [X] T019 [P] Write tests in `internal/diag/diag_test.go`: `Warning` formatting yields the frozen `warning: <token>: <alias>/<name> (<operation>) for Work <id>: <summary>` line; the six `extension-*` tokens are exactly `extension-start-failed`, `extension-failed`, `extension-response-invalid`, `extension-output-refused`, `extension-persist-failed`, `extension-interrupted`; a warning carries no exit code; the existing category table test is **unchanged**.
- [X] T020 Implement in `internal/diag/diag.go`: `Warning{Token, Summary, Hint, Cause, Work, Component, Operation}`, `NewWarning`, `FormatWarning`, the six tokens. No new `Category`, no exit code.
- [X] T021 [P] Add `Warn(th, summary, hint)` to `internal/present/diagrender/diagrender.go` (`⚠ <summary>` then `  → <hint>` when present, `Warning` theme token, ASCII fallback consistent with the existing marks) with a test alongside the existing diagrender tests.

**Checkpoint**: `make lint && GIT_CONFIG_GLOBAL=/dev/null go test ./...` green; every F1–F4 test still passes (only the placeholder-shape, `ipc`-signature and `user_version` tests listed above were edited). Foundation ready.

---

## Phase 3: User Story 1 — Install a Plugin That Declares Linkers and Importers (Priority: P1) 🎯 MVP

**Goal**: `work plugin install` accepts Importer/Linker declarations exactly as ADD §4 documents
them, records every activation datum in the registry through one shared builder, preflights a
declared runtime, and rejects each malformed variant with nothing registered.

**Independent Test**: Install `context-suite` and confirm `registry.json` holds every
declaration; install each `invalid-*` fixture and confirm exit 31 (34 for the runtime) with
nothing registered or staged (quickstart S1–S3).

Branch: `feature/006-automatic-start-context-p2-us1-install`, cut from the phase 1 tip.

### Tests for US1

- [ ] T022 [P] [US1] Extend `internal/plugininstall/install_test.go`: `ComponentEntry(alias, comp)` records `on`, `manual`, `inputs`, `key`, `discover` and `PluginName`; a Linker's `discover.inputs` is copied into the single `Inputs` slot; the ADD §4 example manifest installs end to end; a missing `runtime` on `PATH` returns `plugin-install-failed` (34) with **nothing staged or registered**; a restriction to a Starter that is not installed still installs; reinstalling under the same alias leaves exactly the new declarations (a dropped Importer is gone) — FR-005.
- [ ] T023 [P] [US1] Extend `internal/bootstrap/bootstrap_test.go`: the reference package registered by bootstrap and the same component registered by `plugininstall.Install` produce equal `registry.Component` entries via `ComponentEntry`.
- [ ] T024 [P] [US1] Write `tests/integration/extensions_install.txtar` (S1–S3): install `context-suite` and assert the registry contents and unchanged `work plugin list`; install each `invalid-*` fixture and assert exit 31, `error: plugin-invalid: …` naming the rule, registry and plugin storage unchanged; `invalid-runtime` exits 34 with `error: plugin-install-failed: … "no-such-interpreter" … not found on PATH`; on Unix, install `runtime-sh` and `work start` runs the interpreted Starter without an exec bit (guard the Unix-only part).

### Implementation for US1

- [ ] T025 [US1] Implement the exported `ComponentEntry(alias string, comp plugin.Component) registry.Component` in `internal/plugininstall/install.go` and use it in `Install`; set `Package.PluginName` from the manifest `name`; reinstall replaces every new field (FR-005).
- [ ] T026 [US1] In `internal/plugininstall/install.go`, add the runtime preflight: after the manifest validates and **before** anything is staged or registered, `exec.LookPath` every declared `runtime`; a miss returns `plugin-install-failed` (34) with the message and hint from `contracts/plugin-manifest-extensions.md` ("Runtime").
- [ ] T027 [US1] Make `internal/bootstrap/bootstrap.go` `registerComponents` use `plugininstall.ComponentEntry` (no duplicate manifest → registry copy).
- [ ] T028 [US1] Confirm `work plugin list` output is unchanged (origin and pinned reference only) and that pre-F5 registry entries load inert; adjust only if a test shows drift.

**Checkpoint**: S1–S3 pass; `make lint && GIT_CONFIG_GLOBAL=/dev/null go test ./...` green on all three OSes with F1–F4 demos still green. **MVP** — a plugin declaring a Starter, Linker and Importer installs; nothing executes yet.

---

## Phase 4: User Story 2 — Starter-Published Metadata and Links Appear in the First Snapshot (Priority: P1)

**Goal**: A Starter's `meta`/`links` are validated before materialization and written into the
Work's first snapshot by the same atomic step that creates it, with `start` provenance in the
same index transaction.

**Independent Test**: With `context-suite` `starter-context`, `work start` yields a snapshot with
exactly the published `meta`/`links` and two provenance rows; `starter-bad-key` /
`starter-foreign-private` exit 37 leaving no branch, worktree, directory, snapshot or index row
(quickstart S4–S5).

Branch: `feature/006-automatic-start-context-p3-us2-context`, cut from the phase 2 tip.

### Tests for US2

- [ ] T029 [P] [US2] Extend `internal/starter/starter_test.go`: `Reference` carries `Meta`/`Links`; `ValidateResponse(ref, owner)` rejects a bad key, a foreign private key, and an empty link value; unrelated response keys are still ignored; a non-string link value keeps failing at decode (exit 11) per `contracts/starter-protocol.md`.
- [ ] T030 [P] [US2] Extend `internal/create/create_test.go`: `Params.Meta`/`Links`/`StarterComponent` reach the first snapshot for new, fork and contribution modes; provenance rows (`links`/`meta`, component = the Starter's `alias/name`, operation `start`, creation time) are written in the commit transaction; a failing provenance insert leaves no `works` row and unwinds exactly as today; with both maps empty the result is byte-identical to F4 and no provenance rows exist.
- [ ] T031 [P] [US2] Add contract tests in `tests/contract/starter_test.go` for the extended Starter response (valid `meta`+`links`; neither; invalid key → `starter-response-invalid`).
- [ ] T032 [P] [US2] Write `tests/integration/extensions_start.txtar` section for S4–S5 (create the file if absent): `starter-context` snapshot has `schema 3`, exact `meta`/`links`, provenance rows; unset mode leaves `{}` and no provenance; `starter-bad-key` and `starter-foreign-private` exit 37 with `error: starter-response-invalid: … key …` and nothing left (`git branch`, workspace listing, `work.db`).

### Implementation for US2

- [ ] T033 [US2] In `internal/starter/starter.go` add `Meta map[string]any` and `Links map[string]string` to `Reference`, populate them in `Invoke` from the Starter response, and extend `ValidateResponse(ref, owner)` to check every key with `semconv.ValidatePublished(owner, key)` and every link value with `semconv.ValidLinkValue`; violations are `starter-response-invalid` (37) naming the key, never the value.
- [ ] T034 [US2] In `internal/create/create.go` add `Params{Meta, Links, StarterComponent}`; have `build` fill `State.Meta`/`Links` and derive the `start` provenance entries (it owns the Work id and timestamp); commit through `UpsertWithProvenance` so no Work exists without its published context (FR-013). The Repository Reference stays transient (FR-014).
- [ ] T035 [US2] In `internal/cli/start.go` pass the validated `Reference.Meta`/`Links` and the chosen Starter's qualified name into `create.Params`, using `Registry.PluginNameOf(chosen.Alias)` as the `owner` for `ValidateResponse`; validation runs before `create.Run`.

**Checkpoint**: S4–S5 pass; a `work start` on a reference-package-only machine is byte-identical to F4 (S1 of F4 baseline unchanged).

---

## Phase 5: User Story 3 — An Eligible Linker Discovers a Link When the Work Starts (Priority: P1)

**Goal**: After `create.Run` commits, `start:finalized` runs eligible automatic Linkers in fixed
order; each value is stored as the link for the Linker's declared key with provenance, and
progress/warnings are rendered at the CLI border.

**Independent Test**: A `linker-value` run stores the link with `discover` provenance and a log
holding only the declared inputs; restricted/unsubscribed/`automatic:false`/missing-input
components leave no launch trace; two same-key Linkers resolve deterministically 20/20
(quickstart S6–S8).

Branch: `feature/006-automatic-start-context-p4-us3-linkers`, cut from the phase 3 tip.

### Tests for US3

- [ ] T036 [P] [US3] Write `internal/extension/eligible_test.go`: truth table over activation × subscription × Starter restriction × required inputs × optional inputs; Starter restriction by bare `name` and by `alias/name`; a `work:slug` requirement is ineligible in contribution mode and an optional `work:slug` is simply not delivered; ordering is bytewise by qualified name and independent of registry order; optional inputs never gate eligibility; a Linker phase evaluates against pre-phase state.
- [ ] T037 [P] [US3] Write `internal/extension/run_linker_test.go` (fake `Observer`, fake `Indexer`, `t.TempDir` Home, stub entrypoints): a returned value upserts `links[key]` and records provenance (component, `discover`, time); `{}`, empty stdout and `null` value are silent no-ops; `""` or non-string value → `extension-response-invalid` with nothing persisted; the response cannot redirect the key; same-key Linkers → the later in order wins and provenance names it; the delivered input contains exactly the declared present inputs (absent optional omitted, never `null`); a snapshot lock/read/write failure → `extension-persist-failed` and earlier Linkers' results stay; a provenance failure does not fail the Linker; `Run` never returns an error.
- [ ] T038 [P] [US3] Add contract tests in `tests/contract/extension_protocol_test.go` running the `context-suite` fixture binaries under the exact stdin/stdout rules of `contracts/extension-protocol.md` §4 for Linkers (`{}`, empty, `value`, `null`, `""`, non-string).
- [ ] T039 [P] [US3] Write `internal/cli` tests in `internal/cli/diagnostics_border_test.go` and `internal/cli/start_test.go`: `renderWarning` non-interactive prints the frozen `warning: <token>: …` line and interactive prints `⚠ <summary>` + `  → <hint>`; `WORK_DEBUG` appends exit status and the stderr tail; progress lines format exactly `work: running <alias>/<name> (discover)` and `work: <alias>/<name>: linked <key>` (key, never value); nothing is printed and no process starts when nothing is eligible.
- [ ] T040 [P] [US3] Extend `tests/integration/extensions_start.txtar` with S6–S8: `linker-value` (stdout is exactly the three `work:` lines; stderr has the two progress lines; link replaces the Starter's value; provenance `discover`), `linker-none` prints nothing, `linker-empty-value` → `extension-response-invalid`; S7 launch-log assertions with `restricted-suite` (no entry for ineligible components; inputs keys are exactly the declared resolved bare keys; no `starters`, no snapshot path); S8 runs the start 20 times on fresh repos asserting identical order (`…/linker` before `…/linker2`), `linker2`'s value stored, provenance naming `<alias>/linker2`.

### Implementation for US3

- [ ] T041 [US3] Implement `Eligible(reg, state, starter, event) []Decision` in `internal/extension/eligible.go`: pure and static (no I/O, nothing started), sorted bytewise by `QualifiedName`, input resolution per `contracts/extension-protocol.md` §3 (keyed by bare key, typed values, absent optional omitted); define `Decision`, `Context`, `Indexer`, `Observer`, `Event`, `EventKind`, `Report` per `data-model.md` §7.
- [ ] T042 [US3] Implement the Linker runner in `internal/extension/linker.go` over `ipc.Target` via `ipc.RunContext`: build the input document, parse the single response (`value` absent/`null` = none, `""`/non-string = invalid, extra fields ignored), classify failures into the six tokens (`extension-start-failed`, `extension-failed`, `extension-response-invalid`), and never let response content choose the key.
- [ ] T043 [US3] Implement locked persistence in `internal/extension/run.go`: under the per-Work lock from `Home.WorkLockPath`, read the snapshot, set `links[key]`, write atomically with `work.Write`, then `Indexer.RecordProvenance`; a lock/read/write failure is `extension-persist-failed` for that Linker; provenance failure is non-fatal (surfaced only under `WORK_DEBUG`). Implement `Run(ctx, Context) Report` with the Linker phase (sequential, in order, one `Running`/`Linked`/`Warned` event each) and a Linker-failed → dependent-Importer-ineligible-without-a-second-warning contract (FR-039) ready for the Importer phase.
- [ ] T044 [US3] Add `renderWarning(ui, in, interactive, w)` next to `renderDiagnostic` in `internal/cli/diagnostics_border.go` (non-interactive frozen line; interactive via `diagrender.Warn`; `WORK_DEBUG` appends exit status and last 4 KiB of stderr; never a link/meta value or artifact content) and an `Observer` implementation rendering progress lines on the UI writer (plain text in every mode, muted-themed when interactive, no control sequences when `NO_COLOR` non-empty / `TERM=dumb` / non-TTY).
- [ ] T045 [US3] Wire `internal/cli/start.go`: after `create.Run` succeeds and the three stable stdout lines are printed, call `extension.Run` with the Starter component, snapshot state and paths; skip entirely (no output, no process) when `Eligible` yields nothing; then continue to shell integration. The run's result is always success — a `Report` never changes the exit code (FR-036). `work start <path>` remains unchanged.

**Checkpoint**: S6–S8 pass and the interactive demo from the approved "A" experience renders exactly the contract's progress lines; a reference-package-only start prints nothing extension-related.

---

## Phase 6: User Story 4 — An Importer Brings Artifacts Into the Work Through Exclusive Staging (Priority: P2)

**Goal**: After Linker results persist, eligible Importers run one at a time, each writing into an
exclusive stage; Work plans, validates and incorporates the output beside `worktree/` and removes
the stage on every outcome.

**Independent Test**: `linker-value,importer-ok` yields `notes/context.md` in the Work directory
(not in the worktree), the Importer received the discovered link, no `work/import-*` stage
remains, and archiving preserves the artifact (quickstart S9).

Branch: `feature/006-automatic-start-context-p5-us4-importers`, cut from the phase 4 tip.

### Tests for US4

- [ ] T046 [P] [US4] Write `internal/staging/stage_test.go` and `internal/staging/plan_test.go`: `NewStage` creates a new empty mode-0700 directory under `os.TempDir()/work/import-*` unique per call; `Remove` deletes it; `Build` on a clean stage returns items sorted with directories before their files; a destination directory that already exists is merged into when nothing inside collides; an empty stage yields an empty plan; `Build` never modifies the filesystem.
- [ ] T047 [P] [US4] Write `internal/staging/incorporate_test.go` (happy path): `Incorporate` creates directories then files with exclusive create, preserves relative paths, returns the created count, and copies (not renames) so it works across volumes.
- [ ] T048 [P] [US4] Write `internal/extension/run_importer_test.go`: an Importer becomes eligible after a Linker persists its required link and receives the discovered value; an Importer whose required input is absent does not run and produces no warning; restricted/manual-only Importers never run; the stage is removed on success, empty output and failure; empty output succeeds silently; progress events `Running`/`Imported(n)`; a later Importer's plan sees an earlier one's incorporated files (FR-033).
- [ ] T049 [P] [US4] Extend `tests/contract/extension_protocol_test.go` for Importers (empty stdout, object stdout ignored, garbage stdout) and `tests/integration/extensions_import.txtar` with S9: `importer-ok` output lands in the Work directory beside `worktree/`, nothing inside `worktree/`, the launch log shows the discovered link as input, no `work/import-*` remains, `importer-empty` succeeds silently, and after `work archive <id> --yes` the archived directory still holds `work-state.json` and `notes/context.md`.

### Implementation for US4

- [ ] T050 [P] [US4] Implement `internal/staging/stage.go`: `NewStage` (`os.MkdirTemp` under `<os temp dir>/work/`, prefix `import-`, mode 0700, never inside the Work or workspace) and `Stage.Remove`.
- [ ] T051 [P] [US4] Implement `internal/staging/plan.go`: `Build(stageDir, workDir) (Plan, error)` — walk the stage read-only, compute `Item{Rel, Dest, IsDir}` destinations beneath the Work directory (`<workspace>/in-progress/<repo>_<branch>/`), sort directories before their files; define `Refusal{Rel, Reason}` (implements `error`) and `RefusalReason` ∈ `Exists`, `ReservedPath`, `NotRegular`, `EscapesWork`. Build only the clean-path logic here; the refusal matrix is proven in US5.
- [ ] T052 [US4] Implement `internal/staging/incorporate.go`: `(Plan).Incorporate() (created int, err error)` — create directories then files with `O_EXCL`, record exactly what it created in an undo log; on any error remove the recorded items (files then directories, reverse) before returning; honor `WORK_FAIL_AT=incorporate:<n>` for fault injection.
- [ ] T053 [US4] Implement the Importer runner in `internal/extension/importer.go` and extend `Run` in `internal/extension/run.go` with the Importer phase: re-run `Eligible` against the post-Linker state (FR-019), then per Importer in order: emit `Running`, `NewStage`, invoke with `{inputs, output_dir}` via `ipc.RunContext`, accept empty stdout or one JSON object (content ignored), `staging.Build`, `Incorporate`, emit `Imported(n)` when `n > 0`, and always `Stage.Remove` (defer with no context dependency).
- [ ] T054 [US4] Extend the `Observer` rendering in `internal/cli/diagnostics_border.go` with `work: running <alias>/<name> (import)` and `work: <alias>/<name>: imported <n> item(s)` (only when `n > 0`; no line for an Importer with no output).
- [ ] T055 [US4] Confirm the archive path preserves Importer artifacts unchanged (F2 `archive` renames the whole Work directory after removing the worktree); add the assertion to the S9 script and change `internal/archive/` only if the test proves otherwise.

**Checkpoint**: S9 passes; the full happy pipeline (Starter → Linker → Importer) works end to end with F1–F4 demos still green.

---

## Phase 7: User Story 5 — Colliding Importer Output Is Refused Entirely (Priority: P2)

**Goal**: An Importer execution is all-or-nothing: any unsafe item refuses the whole execution,
and any part-way failure is fully undone, leaving the Work directory byte-identical.

**Independent Test**: Two Importers sharing a path, a mixed clean+colliding output, symlink,
worktree/state-file destinations and a fault-injected incorporation each leave the tree
byte-identical after the affected Importer, with one warning naming component and relative path
(quickstart S10–S11).

Branch: `feature/006-automatic-start-context-p6-us5-collisions`, cut from the phase 5 tip.

### Tests for US5

- [ ] T056 [P] [US5] Extend `internal/staging/plan_test.go` with the refusal matrix from `contracts/extension-protocol.md` §6: `Exists` (file destination exists of any type; directory destination exists and is not a directory), `ReservedPath` (`worktree` and anything under it, `work-state.json`, and case variants `Worktree/`, `WORK-STATE.JSON`), `NotRegular` (symlink; FIFO where the OS allows), `EscapesWork` (deepest existing ancestor is a symlink resolving outside the Work directory via `EvalSymlinks`); a refusal returns `*Refusal` naming the relative path and leaves the tree byte-identical (hash before/after).
- [ ] T057 [P] [US5] Extend `internal/staging/incorporate_test.go` with fault injection at every item index (`WORK_FAIL_AT=incorporate:<n>`, table over N): the Work tree is byte-identical to before; a case-only duplicate within one output fails at exclusive create on a case-insensitive volume and rolls back; existing content is never overwritten, altered or deleted under any outcome.
- [ ] T058 [P] [US5] Extend `tests/integration/extensions_import.txtar` with S10–S11: `importer-ok,importer2-collide` (first incorporated, second contributes nothing, one `extension-output-refused` warning naming `<alias>/importer2`, `import` and the relative path); `importer-mixed`, `importer-symlink`, `importer-into-worktree`, `importer-over-state` each refused with the Work directory hash unchanged by that Importer; `WORK_FAIL_AT=incorporate:1` with `importer-ok` → byte-identical tree, `extension-persist-failed`, exit 0, no stage directory left.

### Implementation for US5

- [ ] T059 [US5] Complete `internal/staging/plan.go`: implement every refusal in the matrix, comparing reserved names case-insensitively (`EqualFold`), resolving the deepest existing ancestor with `filepath.EvalSymlinks` to detect `EscapesWork`, and rejecting any entry that is not a regular file or directory (`NotRegular`) — all before any change to the Work.
- [ ] T060 [US5] Harden `internal/staging/incorporate.go`: verify the undo log removes exactly what this execution created (never a pre-existing merged directory), keep exclusive create as the case-insensitive-volume backstop, and make the injected-failure path leave nothing behind.
- [ ] T061 [US5] Map staging outcomes to warnings in `internal/extension/importer.go`: a `*Refusal` → `extension-output-refused` (names the relative path, never content); an `Incorporate` error (already rolled back) → `extension-persist-failed`; both leave the run continuing with the next Importer.

**Checkpoint**: S10–S11 pass; SC-004 holds across every refusal and failure scenario.

---

## Phase 8: User Story 6 — An Extension Failure Never Damages the Work (Priority: P2)

**Goal**: Every automatic-extension failure class yields exactly one warning, leaves no effect of
that component, keeps the Work valid, exits 0 and still repositions the terminal; interruption
stops the running extension and skips the rest.

**Independent Test**: One start per failure mode (`linker-exit1`, `linker-garbage`,
`importer-exit1`, `importer-garbage`, missing entrypoint) plus healthy siblings: Work, snapshot,
worktree and repositioning intact, healthy siblings ran, one warning per failure (quickstart
S12–S13).

Branch: `feature/006-automatic-start-context-p7-polish` (shared with Phase 9), cut from the phase
6 tip.

### Tests for US6

- [ ] T062 [P] [US6] Extend `internal/extension/run_test.go` with the failure-class → token table (cannot start / missing runtime → `extension-start-failed`; non-zero exit → `extension-failed`; not a single valid structured response or wrong shape/type → `extension-response-invalid`; plus refused/persist/interrupt from earlier phases); exactly one warning per failure carrying Work id, qualified component and operation; a failed Linker makes a dependent Importer ineligible with **no** second warning; other components still run; no link changed and no artifact incorporated for a failed component.
- [ ] T063 [P] [US6] Add interrupt tests in `internal/extension/run_test.go`: cancelling the context mid-run kills the child, discards a value returned after cancellation, removes the stage, skips every later component, yields one `extension-interrupted` warning, and `Run` still returns normally; cleanup (`os.RemoveAll` of the stage, undo log) does not use the cancelled context.
- [ ] T064 [P] [US6] Extend `tests/integration/extensions_start.txtar` with S12: each failure mode → exit 0, stdout still the three `work:` lines, worktree present, `work-state.json` valid, exactly one `warning: extension-…` naming Work id, `<alias>/<name>` and operation; a linked (`--link`) package whose entrypoint was deleted after install → `extension-start-failed` and the installation stays valid; `WORK_DEBUG=1` adds exit status and stderr tail, and **without** it no warning contains a link/meta value, artifact content or the extension's stderr (assert with a marker string).
- [ ] T065 [P] [US6] Write `tests/integration/extensions_interrupt_test.go` (`//go:build unix`, PTY): `linker-hang` with SIGINT sent to the `work` process → the Linker is killed, one `extension-interrupted` warning, remaining components skipped, the Work exists and is valid, exit 0, and the shell-integration target file is still written (S13).

### Implementation for US6

- [ ] T066 [US6] Complete the failure→warning mapping in `internal/extension/`: classify start errors (missing entrypoint/runtime/exec error), non-zero exit, and invalid stdout into the tokens above; build each `diag.Warning` with Work id, qualified component, operation, a plain-language summary and a next step when known; ensure a warning never contains a link/meta value, artifact content or the extension's raw text (raw text only in `Cause` for `WORK_DEBUG`).
- [ ] T067 [US6] Implement interruption in `internal/extension/run.go`: use the command context from the root `signal.NotifyContext`, treat a cancelled context for the running component as `extension-interrupted`, skip everything after it and return; a value returned after cancellation is discarded; the stage and undo log are cleaned without the context.
- [ ] T068 [US6] In `internal/cli/start.go` guarantee the ordering after any `Report`: shell integration (`shellintegration.WriteTargetPath`) runs after the automatic phases end, whatever the outcome, and `runStart` returns success (FR-036, FR-038, FR-042).
- [ ] T069 [US6] Add `WORK_DEBUG` handling in `renderWarning`: append the component's exit status and the last 4 KiB of stderr after the human line only when `WORK_DEBUG` is non-empty.

**Checkpoint**: S12–S13 pass; SC-005 and SC-010 hold across every failure fixture.

---

## Phase 9: User Story 7 — See What Runs, and Get the Same Result Every Time (Priority: P3)

**Goal**: Progress and warnings look right interactively and are plain sequential lines
otherwise; nothing extension-related appears when nothing is eligible; runs are deterministic.

**Independent Test**: Interactive (PTY) and non-interactive runs of three eligible fixtures show
the same lines (themed vs plain); a reference-package-only machine shows nothing and logs
nothing; 20 repeats produce identical order and results (quickstart S14).

### Tests for US7

- [ ] T070 [P] [US7] Write `tests/integration/extensions_visibility_test.go` (`//go:build unix`, PTY): an interactive start with `context-suite` shows, after the confirmation receipt and the three stable stdout lines, exactly the contract's progress lines in execution order (`running … (discover)`, `linked <key>`, `running … (import)`, `imported <n> item(s)`) on the UI channel, and a failing component adds one `⚠` warning with a `→` hint; the start wizard (steps, receipts, geometry) is unchanged when extensions are installed.
- [ ] T071 [P] [US7] Extend `tests/integration/extensions_start.txtar` with S14: with `NO_COLOR=1 TERM=dumb` and with piped/redirected streams the same lines appear with `grep -c $'\x1b'` equal to 0; extension output never appears on stdout (`2>/dev/null` yields exactly the F4 stdout); with only the reference package installed nothing extension-related is printed and `$WORK_FIXTURE_LOG` stays empty (FR-041).
- [ ] T072 [P] [US7] Extend `tests/integration/stream_separation_test.go` (or a new `extensions_streams_test.go`) asserting progress and warnings are written only to the UI channel, never stdout, and that the exit code is unchanged by any warning.

### Implementation for US7

- [ ] T073 [US7] Review the `Observer` rendering in `internal/cli/diagnostics_border.go` against `contracts/cli-work-start.md`: identical text interactive and not, muted theme token only when interactive, `Warning` token for `⚠`, ASCII fallback for marks, no control sequences when `NO_COLOR` is non-empty, `TERM=dumb`, or a non-TTY; fix any drift found by T070–T072.
- [ ] T074 [US7] Confirm no new Bubble Tea model, live region, flag, prompt or stdout line was added (`research.md` R13, FR-046); if a check finds one, remove it rather than documenting it.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: End-to-end demonstration, regression sweep, docs. Same branch as Phase 8–9.

- [ ] T075 Write `tests/integration/f5_demo_test.go` (S15, SC-011): the roadmap five-step journey as one automated test — (1) Starter publishes metadata + link, (2) a Linker discovers another value and the core upserts it, (3) an Importer consumes the new link through staging and incorporates without collision, (4) a second Importer collides and none of its output is incorporated, (5) an automatic extension fails and `work start` ends with a warning and a usable Work. Runs green alongside the F1–F4 demos.
- [ ] T076 [P] Add the SC-001 external-package journey: install `context-suite` from a directory built with no core code changes and assert the snapshot holds both Starter-published and Linker-discovered links and the Work directory holds the Importer's artifacts.
- [ ] T077 [P] Add the SC-009 coverage check: every public key used by any shipped fixture or example (`github.pull_request`, `github.pull_request.number`, `example.*`) is defined in `specs/006-automatic-start-context/contracts/semantic-conventions.md` §5, and every manifest rule FR-001–FR-007 has at least one violating fixture (script or table test under `tests/contract/`).
- [ ] T078 [P] Add the migration regression test: an F4-era `WORK_HOME` (`work.db` at `user_version` 2) is rebuilt once on the first F5 command to `user_version` 3, losslessly, and a later rebuild restores Works, `meta` and `links` but not provenance (`contracts/index-provenance.md`).
- [ ] T079 [P] Update `README`/help text only where a user-visible fact changed (plugin authors: Importer/Linker manifest shapes, Semantic Conventions v1, the six warning tokens); do **not** change `work start --help` grammar. Verify `internal/cli/help_test.go` and `tests/integration/help_inventory.txtar` still pass unmodified.
- [ ] T080 Run the full regression sweep: `GIT_CONFIG_GLOBAL=/dev/null go test ./... && make lint`, then re-run the F1 (S1–S12), F2 (S1–S13), F2.5 (Q1–Q12), F3 (S1–S13) and F4 (S1–S14) quickstarts against the same build; any stdout/token/exit-code diff is a regression. On a fresh `WORK_HOME` with **only the reference package**, confirm `work start <path>` output, exit code and created files are byte-identical to the F4 build and no extension process starts (SC-008).
- [ ] T081 Run all quickstart scenarios S1–S15 against a built binary using the setup in `quickstart.md`, on Linux locally and via the CI matrix (`ubuntu-latest`, `macos-latest`, `windows-latest`; `runtime-sh` and PTY tests are Unix-only).
- [ ] T082 Final review pass: grep the diff for phase numbers, task IDs or spec IDs in source comments and remove them (plain godoc only); confirm every exported name in `internal/semconv`, `internal/extension` and `internal/staging` has purpose/inputs/outputs/errors godoc.

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: none — start immediately.
- **Foundational (Phase 2)**: depends on Phase 1 fixtures only where tests need them; **blocks all user stories**.
- **US1 (Phase 3)** → **US2 (Phase 4)** → **US3 (Phase 5)** → **US4 (Phase 6)** → **US5 (Phase 7)** → **US6 (Phase 8)** → **US7 (Phase 9)** → **Polish (Phase 10)**. Each phase branch is cut from the previous tip (plan Branching Strategy); merge cadence and PR targets are confirmed with the user at the start of each phase, never assumed.

### Story dependencies

- **US1**: Foundational only (registry/manifest/ipc/semconv). Independently testable.
- **US2**: Foundational (`semconv`, `projection` v3); independent of US1's install wiring at unit level, but its integration test installs `context-suite`, so it needs US1.
- **US3**: needs US1 (registered activation data) and US2 (Starter `meta`/`links` exist to be replaced/consumed).
- **US4**: needs US3 (`Run`, `Eligible`, observer, persistence) and the Foundational `ipc`.
- **US5**: needs US4's `staging` happy path.
- **US6**: needs the phases from US3–US5 to have failure surfaces to classify.
- **US7**: needs all of the above to be observable.

### Within a phase

- Tests are written first and must fail before the implementation (T036–T040 before T041–T045, and so on).
- Types/leaf packages before consumers; `internal/extension` before the `cli` wiring; `staging` before the Importer phase.

### Parallel opportunities

- **Phase 1**: T002–T005 are independent fixture trees ([P]).
- **Phase 2**: `semconv` (T007–T008), manifest (T009–T010), registry (T011–T012), `ipc` (T013–T014), projection (T016–T017), lock/diag/diagrender (T018–T021) touch different packages; the T015 mechanical caller update follows T014, and T012 depends on T014's `ipc.Target` type.
- **Within a story**: all tasks marked [P] in its Tests block; `staging/stage.go` and `staging/plan.go` (T050, T051) are independent.

### Parallel example: Foundational

```text
Task: T007 semconv table tests            Task: T009 manifest tests
Task: T011 registry tests                  Task: T013 ipc tests
Task: T016 projection/reconcile tests      Task: T019 diag Warning tests
```

### Parallel example: User Story 3 tests

```text
Task: T036 internal/extension/eligible_test.go
Task: T037 internal/extension/run_linker_test.go
Task: T038 tests/contract/extension_protocol_test.go
Task: T039 internal/cli renderWarning/progress tests
```

---

## Implementation Strategy

### MVP first (User Story 1)

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 (US1): a package declaring Starter, Linker and Importer installs with full activation data; nothing executes.
3. **Stop and validate** with quickstart S1–S3 before starting US2.

Note that US1 alone delivers no visible extension behavior; the first end-to-end user value is
the P1 trio (US1 + US2 + US3), after which `work start` produces a Work carrying discovered
links. Treat that trio as the practical release cut if a partial delivery is ever needed.

### Incremental delivery

1. Foundation ready → US1 (install) → US2 (Starter context) → US3 (Linkers: first execution, and the progress/warning rendering "A" experience).
2. US4 (Importers) → US5 (collision safety) → US6 (failure and interrupt guarantees) → US7 (visibility and determinism).
3. Each phase adds value without breaking earlier stories; the reference-package-only run stays byte-identical to F4 at every checkpoint (SC-008).

### Release

When Phase 10 merges, F5 ships via `release/0.6` cut from `develop`, merged to `master` and
tagged (`master` requires signed commits — check the ruleset via `gh api`, not local `git log`).
The release step is the hand-off, not a task.

---

## Notes

- Every task names its file(s); an LLM should be able to execute it with only this file plus the referenced contract sections.
- [P] tasks touch different files with no dependency on an incomplete task.
- Commit after each task or logical group; stop at each phase Checkpoint to validate independently.
- Failure to update the index after a snapshot write never invalidates the Work; the snapshot is authoritative (FR-044).
- Options **not** built in F5: an animated indicator or any new Bubble Tea model (a possible later polish outside this task list), `work link`/`work import`/`work status` (F6), the `work plugin` hub and lifecycle commands (F7).
