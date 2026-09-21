# Implementation Plan: Automatic Context on Start (F5)

**Branch**: spec/plan/doc work on `feature/f5-automatic-context`; F5 implementation uses one
`feature/006-automatic-start-context-p<n>-*` branch per phase (see **Branching Strategy**) |
**Date**: 2026-09-19 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/006-automatic-start-context/spec.md`; authorities
`docs/prd.md` (FR-8, FR-26–FR-29, FR-32, FR-35; FR-27, FR-33, FR-34),
`docs/add/add-0001-work-system-architecture.md` §3, §4, §9, §10, §11, ADR-0000, ADR-0006,
ADR-0012, ADR-0013.

## Summary

F5 is where the extension model stops being declared and starts being *executed*. After a
Work is created, Work publishes `start:finalized`; eligible Linkers discover links, the core
persists them, eligible Importers write into an exclusive staging area, and the core
incorporates their artifacts only if every destination is safe — all without any extension
ever being able to corrupt, roll back, or block the already-created Work. This is the
roadmap's "v1 feature preview".

Reading the F1–F4 code before planning showed that the groundwork is *less* complete than the
documents suggest, which is why F5 carries more foundational work than its roadmap entry lists.
Four things were already in place and cost nothing:

- `work.State` already has `Meta`/`Links` (required objects in `Validate`), and
  `ipc.StarterResponse` already decodes `meta`/`links` — the snapshot **needs no new
  schema version**; F4 only kept them off `starter.Reference` as a deliberate type constraint.
- `create.Run`'s single commit point (`db.Upsert`) is exactly where Starter-published
  provenance can join the same transaction.
- `projection.Upsert` is `ON CONFLICT DO UPDATE` and the connection has `foreign_keys=ON`, so
  a provenance table cascades on delete without being wiped by reconcile's upserts.
- `archive` renames the whole Work directory after removing the worktree, so Importer
  artifacts placed beside `worktree/` survive archiving with no F2 change.

Three gaps were **found in the code, not the documents**, and are closed here because F5 is the
first slice that depends on them (spec "Scope and Dependencies"):

1. **The manifest shapes.** `plugin.Component` types `on`/`manual`/`discover` as
   `[]string`/`bool`/`[]string`; a manifest exactly as ADD §4 shows it is *rejected* by
   `plugin.Parse` (verified). F1's tests use placeholders that were never a documented shape.
2. **The registry.** `registry.Component` records none of `on`/`manual`/`inputs`/`key`/`discover`
   although F4's spec assumed it did; `plugininstall` and `bootstrap` duplicate the
   manifest→registry copy.
3. **The runtime.** `runtime` is recorded and never used: `ipc.Run` is `exec.Command(entrypoint)`,
   `EntrypointPath` appends `.exe` unconditionally, and no `LookPath` exists (F1's contract and
   F4's FR-008 both said it was checked). A Starter declared `"runtime": "python3"` cannot run.

So F5 is genuinely new logic in five places, each independently testable:

1. **Declaration model and runtime** (`internal/plugin`, `internal/registry`,
   `internal/plugininstall`, `internal/ipc`, new leaf `internal/semconv`) — ADD §4 shapes,
   activation data in the registry via one shared entry builder, `Target{Runtime, Path}`
   invocation with an install-time `LookPath`, key grammar / ownership / the five exposed
   `work` facts (research R1–R5).
2. **Starter context in the first snapshot** (`internal/starter`, `internal/create`,
   `internal/projection`) — `Reference` gains `Meta`/`Links`, validated before
   materialization; the maps and their provenance rows are written by the existing commit;
   `work.db` moves to `user_version` 3 (R10).
3. **The extension pipeline** (new `internal/extension`) — a pure `Eligible` function, input
   projection, the Linker phase with locked read-modify-write persistence, the Importer phase,
   fixed `alias/name` ordering, failure→warning mapping, interrupt handling (R6–R8, R10, R12,
   R14).
4. **Staging and incorporation** (new `internal/staging`) — exclusive stage, a read-only
   plan that refuses whole executions, and an all-or-nothing incorporation with an undo log
   (R11, R12).
5. **The border** (`internal/diag`, `internal/cli/start.go`, `diagnostics_border.go`,
   `diagrender`) — a non-fatal `diag.Warning`, `renderWarning`, progress lines, and the wiring
   after `create.Run` (R9, R13).

No dependency, binary or IPC transport is new (ADR-0000/ADR-0006 are consumed as documented).
`config/work.json` gains no key; `work-state.json` stays at schema 3; `registry.json` gains
only additive per-component and per-package fields (unversioned, regenerated); `work.db` moves
to `user_version` 3 (one new table, `work_provenance`). **No new exit code**: manifest problems
reuse 31, a missing runtime reuses 34, an invalid Starter key reuses 37; extension failures
are *warnings* with six new stable tokens and no exit-code effect.

## Technical Context

**Language/Version**: Go 1.26 (toolchain go1.26.x). Single module
`github.com/gustaborges/work`. No new language features.

**Primary Dependencies** (all already vendored — F5 adds none):
- `github.com/spf13/cobra` — no new command; `work start` gains behavior only.
- `charm.land/*` via `internal/present` — **no new Bubble Tea model**: progress and warnings
  are plain lines on the existing UI writer, themed by the existing `Warning`/`Muted` tokens.
- Standard library: `os/exec` (`CommandContext`, `LookPath`), `io/fs`, `path/filepath`
  (`EvalSymlinks`, `WalkDir`), `os` (`MkdirTemp`, `OpenFile` with `O_EXCL`), `encoding/json`.
- System `git` — unchanged; no new git subcommand.
- `modernc.org/sqlite` — one new table and one migration, same API surface.

**Storage**:
- `work-state.json`: **schema 3, unchanged.** `meta`/`links` are filled by the core; no
  field becomes required; an F4 reader accepts an F5 snapshot (FR-015).
- `work.db`: **`user_version` 3.** New `work_provenance` table (`contracts/index-provenance.md`).
  `Reset` drops it before `works`. `reconcile.Open`'s existing "rebuild when below
  `SchemaVersion`" means every F4 database is rebuilt once — lossless (no provenance existed).
  Provenance is lost by a later rebuild by design (D4, FR-044).
- `registry.json`: additive, unversioned. `components[]` gains `on`, `manual`, `inputs`, `key`,
  `discover`; `packages[]` gains `plugin_name` (`contracts/plugin-manifest-extensions.md`).
- `config/work.json`: **no new keys.**
- Import stages: `<os temp dir>/work/import-*`, mode 0700, one per Importer execution, removed
  on every outcome; never inside the Work or the workspace.

**Testing**:
- `internal/semconv` unit tests: key grammar table (valid/invalid, reserved `plugin`/`work`,
  single segment, uppercase, hyphen); ownership incl. a dotted plugin name that cannot own
  private keys; link/meta value rules; the five facts.
- `internal/plugin` unit tests: **updated** — the F1 placeholder cases (`"on":["clone"]`,
  `"manual":true`, `"discover":["x"]`, `"work:slug"`) are replaced by ADD-shaped cases; new
  cases for M1–M7 (`contracts/plugin-manifest-extensions.md`); the ADD §4 example manifest
  parses; the old placeholder shapes are rejected.
- `internal/registry` unit tests: new fields round-trip; `Component.Target`/`EntrypointPath`
  (`.exe` only without a runtime); `PluginNameOf` fallback; a legacy entry without the new
  fields loads and is inert.
- `internal/plugininstall` unit tests: `ComponentEntry` used by both writers (bootstrap test
  asserts equality with install for the same component); runtime `LookPath` miss → 34 with
  nothing staged or registered; reinstall replaces the new fields (FR-005); the ADD §4
  manifest installs end to end.
- `internal/ipc` unit tests: `Target` with/without runtime (`sh` on Unix), context
  cancellation kills the child, stderr tail capped at 4 KiB; existing tests keep passing
  through the adapted signatures.
- `internal/starter` unit tests: `Meta`/`Links` consumed; `ValidateResponse` rejects bad
  keys, foreign private keys, empty link values; unrelated response keys still ignored.
- `internal/create` unit tests: `Params.Meta/Links` reach the snapshot; provenance rows are
  written in the commit transaction (a failing provenance insert leaves no `works` row and
  unwinds exactly as today); F4 paths byte-identical when both are empty.
- `internal/projection` / `internal/reconcile` unit tests: migration 2→3; `Reset` drops
  provenance first; `Delete` cascades; `Upsert` does not cascade; rebuild restores Works but
  not provenance; `user_version` assertions updated 2→3
  (`projection_test.go`, `reconcile_test.go`, `tests/integration/integration_test.go`).
- `internal/extension` unit tests (fake `Observer`, fake `Indexer`, `t.TempDir` Home, stub
  entrypoints): `Eligible` truth table (activation × subscription × Starter restriction ×
  inputs × optional); Starter restriction by bare name and `alias/name`; ordering is
  bytewise by qualified name and independent of registry order; same-key Linkers → later wins;
  Linker phase inputs are pre-phase state while Importer phase sees results; failure class →
  token table; a failed Linker makes a dependent Importer ineligible with no second warning;
  interrupt mid-run → one `extension-interrupted`, rest skipped, stage removed; `Run` never
  returns an error.
- `internal/staging` unit tests: plan refuses `Exists`, `ReservedPath` (incl. `Worktree/`,
  `WORK-STATE.JSON`), `NotRegular` (symlink, fifo where the OS allows), `EscapesWork`
  (existing ancestor is a symlink out of the Work); clean plan merges into an existing
  directory; refusal leaves the tree byte-identical (hash before/after); `Incorporate`
  fault-injected at every item index leaves the tree byte-identical (table over N);
  intra-output case-only duplicates fail at exclusive create and roll back; stage removed on
  every path.
- `internal/diag` unit tests: `Warning` formatting (frozen `warning: <token>: …` line), token
  set, no exit-code effect; existing category table test **unchanged** (no new category).
- `internal/cli` unit tests: `renderWarning` interactive/non-interactive, `WORK_DEBUG`
  append; progress line format; nothing printed when nothing eligible.
- `tests/contract/`: a component-protocol contract (`extension_protocol_test.go`) running the
  fixture binaries under the exact stdin/stdout rules of `contracts/extension-protocol.md`
  (Linker `{}`/empty/`value`/`null`/`""`/non-string; Importer empty/object/garbage stdout).
- `tests/integration/` `testscript` (`extensions_*.txtar`): S1–S3 (install shapes, invalid
  declarations, runtime), S4–S5 (Starter context, bad key), S6–S8 (Linker, ineligible/inputs
  log, deterministic winner ×20), S9–S12 (Importer, collisions, fault-injected rollback,
  failure classes), S14 (`NO_COLOR`/`TERM=dumb`/piped: no control sequences; only-reference
  machine prints nothing and logs nothing).
- `tests/integration/` PTY (`//go:build unix`): S13 interrupt with a hung Linker; interactive
  progress/warning rendering; the start wizard unchanged when extensions are installed.
- `tests/integration/f5_demo_test.go`: the roadmap five-step demonstration as one journey
  (S15, SC-011).
- Regression: F1 (S1–S12), F2 (S1–S13), F2.5 (Q1–Q12), F3 (S1–S13), F4 (S1–S14) unchanged —
  the reference-package-only run is byte-identical on stdout, exit code and files (SC-008).
- CI matrix unchanged: `ubuntu-latest`, `macos-latest`, `windows-latest`. `runtime-sh` is
  Unix-only; symlink refusal and `EqualFold` reserved names are exercised on all three.

**Target Platform**: Same as F1–F4 — Linux (amd64, arm64), macOS (arm64, amd64), Windows
(amd64). No change to the supported terminal/shell matrix.

**Project Type**: Single-project CLI tool (unchanged). Three new `internal/` packages
(`semconv`, `extension`, `staging`); `plugin`, `registry`, `plugininstall`, `ipc`, `starter`,
`create`, `projection`, `reconcile`, `diag`, `workhome`, `bootstrap`, `locator` and `cli`
gain fields/functions but no behavior change on any path that does not opt in.

**Performance Goals**: Not latency-critical (ADR-0000: subprocess spawns happen at discrete
points, never in a hot loop). The only measurable target is the null case: with nothing
eligible, no extension process starts and no output is produced (FR-041, SC-008). Eligibility
is in-memory over the registry; no I/O to decide.

**Constraints**:
- Extensions receive only declared, present inputs, keyed by bare key; nothing else about the
  Work, no snapshot path, no subscription or restriction data (FR-021, FR-022, SC-003).
- Eligibility is static — no component is started to decide anything; an ineligible component
  is never started and never reported (FR-018, FR-039, SC-002).
- Order inside a phase is bytewise by `alias/name`, uninfluenceable and uncontracted
  (FR-020, D7). The Linker phase evaluates against pre-phase state; the Importer phase against
  post-Linker state (FR-019).
- No extension outcome can invalidate a created Work: no compensator exists after
  `create.Run`'s commit; every failure is a warning and `work start` exits 0 (FR-036, D3).
- Importer output is all-or-nothing: the plan is built read-only *before* any change; a
  refused or failed execution leaves the Work directory byte-identical (FR-028–FR-030, SC-004).
  Nothing is ever written inside `worktree/` or over `work-state.json`; only regular files and
  directories are accepted (D5).
- Snapshot updates are atomic (`work.Write`) under the same per-Work lock `resume`/`archive`
  use; provenance is index-only and non-fatal in the pipeline (FR-043, FR-044).
- Warnings and progress are written only at the CLI border; lower layers return values
  (`renderDiagnostic`'s invariant, extended). Warnings never contain link/meta values,
  artifact content or an extension's raw error text outside `WORK_DEBUG` (FR-037, SC-010).
- No new exit code, no new stdout line, no new flag (FR-046). Every F1–F4 grammar,
  transaction guarantee, stable stdout line, token and exit code is preserved (FR-045).
- No timeout is imposed on an extension; the first interrupt kills it and ends the automatic
  phases (FR-034, FR-038, D9).
- A `runtime` is honored for every role and preflighted at install; a script never gets a
  `.exe` suffix (FR-007, D8).

**Scale/Scope**: Single user, single machine. Code surface: `internal/extension` (≈4 files),
`internal/staging` (≈3 files), `internal/semconv` (≈2 files), and edits to `plugin/manifest.go`,
`registry/registry.go`, `plugininstall/install.go`, `bootstrap/bootstrap.go`, `ipc/ipc.go`,
`starter/starter.go`, `locator/chain.go`, `create/create.go`, `projection/projection.go`,
`reconcile/reconcile.go`, `diag/diag.go`, `workhome/workhome.go`, `archive`/`resume`
(lock-key extraction only), `cli/start.go`, `cli/diagnostics_border.go`,
`present/diagrender`. Two new fixture trees (`context-suite`, `restricted-suite`) plus a few
manifest-only invalid fixtures under `tests/fixtures/plugins/`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unratified template, exactly as for F1–F4. As
before, this plan is held to the **cross-cutting gates in `docs/roadmap.md` §4** and the
spec's **Success Criteria**, treated as binding.

| Gate (roadmap §4) | How this plan satisfies it | Verified by |
|---|---|---|
| Determinism | Eligibility is a pure function of registry + state + chosen Starter (R6). Order inside a phase is bytewise by `alias/name` with no other influence (R7). The one place two providers can conflict — a shared Linker key — resolves by that order and is recorded. No hidden priority, no install-order dependence, no implicit fallback. | `internal/extension` table tests; S8 (×20 runs); SC-007 |
| Integrity | Starter context and its provenance join the existing commit transaction; Linker results are locked read-modify-write with atomic rename; Importer output is planned read-only then incorporated with exclusive create and an undo log, so a refusal or failure leaves the Work byte-identical; the stage is removed on every path; no compensator exists after the commit, so nothing can un-create a Work. | staging fault-injection table; S10–S12; SC-004, SC-005, SC-006 |
| Process contract | One JSON in, at most one JSON out, exit status, free-form stderr (`contracts/extension-protocol.md`); invalid responses classified into six stable warning tokens; Starter key violations reuse 37; manifest violations 31; missing runtime 34. No new exit code and no new stdout line. | contract tests with self-contained fixtures; `diag` tests; SC-010 |
| Portability | `runtime + path` invocation never relies on shebang/exec bit (ADR-0006); `.exe` only for runtime-less entrypoints; symlinks refused in staged output; `EqualFold` reserved names and `O_EXCL` cover case-insensitive volumes; stage under `os.TempDir()`; copy (not rename) across volumes; the existing lock implementation is reused. | 3-OS CI; staging tests; S3 (Unix) |
| Auditability | Every warning names Work, qualified component and operation; provenance records component, operation and time per entry; raw extension output appears only under `WORK_DEBUG`; values and artifact content are never in diagnostics. | `internal/cli` renderer tests; S12; SC-005, SC-010 |
| UX | Discoverable and unchanged: no new command or prompt. Progress shows what runs; warnings say what failed with a next step; non-interactive output has no control sequences; nothing extension-related appears when nothing is eligible. No new Bubble Tea model. | PTY + `testscript`; S14; SC-005 |
| Regression | Reference-package-only `work start` is byte-identical to F4 (stdout, exit code, files, no extra process). F1–F4 suites pass unchanged; the only edited existing tests are those that pin the F1 placeholder manifest shapes, the `ipc` signatures, and `user_version` 2. | CI; S15 + regression sweep; SC-008 |

**Architectural-authority gates (PRD → ADR → ADD):**

- **PRD**: FR-8, FR-26–FR-29, FR-32, FR-35 (with FR-27, FR-33, FR-34) are the product
  authority the spec cites; F5 delivers them within its stated scope (manual `link`/`import`
  and `status` remain F6; plugin lifecycle remains F7, per the roadmap). ✅
- **ADR-0000** (accepted): every extension is an external subprocess; the core never loads
  plugin code. Staging and incorporation are core file operations on data the extension
  produced, not plugin execution. ✅
- **ADR-0006** (accepted): the declared `runtime` is invoked explicitly over the entrypoint,
  never via shebang/exec bit, and preflighted at install — the decision F1–F4 documented and
  did not build (R3). ✅
- **ADR-0012** (accepted): role is the sole discriminator; `on`/`manual`/`inputs`/`discover`
  take the shapes ADD §4 shows; events are core-defined and `starters` is a filter, never
  input; phases are Linker → persist → Importer; eligibility is static; Importers write to an
  exclusive directory and incorporate only after full collision validation. No `priority`
  anywhere. ✅
- **ADR-0013** (accepted): `work`/`meta`/`link` namespaces; the `work` section is core-only
  and only five facts are exposed; public keys follow Semantic Conventions with no compiled-in
  list; provenance is operational and index-only; the snapshot stays the single canonical
  document, plugins never touch it. ✅ (D1, D2, D4)
- **ADR-0002 / ADR-0003** (accepted): the seed and third-party plugins still install through
  one pipeline; the shared `ComponentEntry` builder makes that literal for the new fields
  too. ✅
- **ADR-0004** (accepted): unaffected — Starter matching and collision are consumed as F4
  delivered them; only the *result* of the chosen Starter (its `meta`/`links`) is newly used. ✅
- **Historical contracts**: `specs/001-first-local-work/contracts/plugin-manifest.md` stays
  authoritative for everything it says; `contracts/plugin-manifest-extensions.md` defines the
  shapes it left open and replaces its placeholder tests' shapes.
  `specs/005-plugin-origins/contracts/starter-protocol.md` is extended by
  `contracts/starter-protocol.md` (its "`meta`/`links` discarded" clause is superseded).
  `specs/005-plugin-origins/contracts/work-state.schema.json` (schema 3) is unchanged. ✅

**Gate result: PASS.** No violations requiring Complexity Tracking.

### Post-design re-check (after Phase 1)

Re-evaluated after `research.md`, `data-model.md`, `contracts/` and `quickstart.md`. Still
**PASS**:

- No new dependency, no new IPC transport, no new shipped binary (fixtures under
  `tests/fixtures/plugins/` are test-only).
- The snapshot needs no schema bump — verified against `internal/work/state.go` (`Meta`/`Links`
  already required objects, `Links` already `map[string]string`).
- The one schema change (`work.db` → 3) is additive, and its rebuild interplay was checked
  against `reconcile.Open`/`Reset`/`Upsert` (R10): rebuilds are lossless for F4 homes and
  `Upsert` cannot cascade.
- Every warning token and every reused exit code has exactly one contract row; no existing
  token or code changes (a non-string Starter link value keeps its F1 exit 11 —
  `contracts/starter-protocol.md`).
- The three code-found gaps each map to a requirement (FR-001, FR-004, FR-007), a research
  decision (R1, R2, R3) and a scenario (S1–S3), so none is left as an unstated assumption.
- All seven roadmap §4 gates have a concrete contract or test harness (table above).

## Project Structure

### Documentation (this feature)

```text
specs/006-automatic-start-context/
├── plan.md                              # This file
├── spec.md                              # Feature specification
├── research.md                          # Phase 0 output — decisions R1–R17
├── data-model.md                        # Phase 1 output — manifest/registry/keys/pipeline/staging/provenance
├── quickstart.md                        # Phase 1 output — scenarios S1–S15 + F1–F4 regression
├── contracts/                           # Phase 1 output
│   ├── plugin-manifest-extensions.md    # Importer/Linker shapes (M1–M7), runtime preflight, registry entries
│   ├── extension-protocol.md            # phases, eligibility, projection, wire format, staging, failure classes
│   ├── semantic-conventions.md          # Work Semantic Conventions v1 (the roadmap §5 prerequisite)
│   ├── starter-protocol.md              # F5 amendment: meta/links consumed and validated
│   ├── cli-work-start.md                # F5 amendment: ordering, streams, progress, warnings, interrupt
│   └── index-provenance.md              # work.db schema 3: work_provenance, rebuild behavior
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

Additions and edits to the F1–F4 tree:

```text
internal/
├── semconv/                      # NEW — leaf package (imports nothing internal)
│   ├── semconv.go                #   ValidKey, ValidatePublished(owner,key), ValidLinkValue, Facts
│   └── semconv_test.go
├── extension/                    # NEW — the start:finalized pipeline (presentation-free)
│   ├── eligible.go               #   Eligible(reg,state,starter,event) []Decision; input resolution; ordering
│   ├── run.go                    #   Run(ctx, Context) Report — phases, persistence, failure→Warning, interrupt
│   ├── linker.go / importer.go   #   one-component runners over ipc.Target
│   └── *_test.go
├── staging/                      # NEW — exclusive stage, read-only plan, all-or-nothing incorporation
│   ├── stage.go                  #   NewStage / Remove
│   ├── plan.go                   #   Build(stageDir, workDir) (Plan, error) → *Refusal
│   ├── incorporate.go            #   (Plan).Incorporate — O_EXCL, undo log, WORK_FAIL_AT=incorporate:<n>
│   └── *_test.go
├── plugin/
│   └── manifest.go               # Δ — Subscription/Manual/Discover types; ParseInput; CoreEvents; M1–M7
├── registry/
│   └── registry.go               # Δ — Component{On,Manual,Inputs,Key,Discover}; Package.PluginName;
│                                  #     Target(), QualifiedName(), PluginNameOf(), Extensions(); EntrypointPath .exe fix
├── plugininstall/
│   └── install.go                # Δ — exported ComponentEntry(alias, comp); runtime LookPath before staging (→ 34)
├── bootstrap/
│   └── bootstrap.go              # Δ — registerComponents uses plugininstall.ComponentEntry
├── ipc/
│   └── ipc.go                    # Δ — Target{Runtime,Path}; RunContext (CommandContext); InvokeLinker/InvokeImporter;
│                                  #     stderr tail capped at 4 KiB; existing InvokeStarter/InvokeLocator take Target
├── starter/
│   └── starter.go                # Δ — Reference{Meta,Links}; ValidateResponse(ref, owner) checks keys/values
├── locator/
│   └── chain.go                  # Δ — passes comp.Target(...) (mechanical)
├── create/
│   └── create.go                 # Δ — Params{Meta,Links,StarterComponent}; build() fills state and derives the
│                                  #     provenance rows (it owns the Work id); UpsertWithProvenance at commit
├── projection/
│   └── projection.go             # Δ — SchemaVersion 3; migration 2→3 (work_provenance); Reset drops it first;
│                                  #     UpsertWithProvenance, RecordProvenance, Provenance
├── reconcile/
│   └── reconcile.go              # Δ — comment/doc only: rebuild restores Works, not provenance
├── workhome/
│   └── workhome.go               # Δ — Home.WorkLockPath(id) (one derivation; archive/resume switch to it, same key)
├── diag/
│   └── diag.go                   # Δ — Warning type, NewWarning, FormatWarning, six extension-* tokens (no exit codes)
├── present/diagrender/
│   └── diagrender.go             # Δ — Warn(th, summary, hint)
└── cli/
    ├── start.go                  # Δ — pass Starter Meta/Links into create; after create: stable lines → extension.Run →
    │                              #     shell integration; Observer renders progress + warnings on the UI writer
    └── diagnostics_border.go     # Δ — renderWarning next to renderDiagnostic (border-only writes)

tests/
├── contract/
│   └── extension_protocol_test.go        # NEW — fixture binaries under the wire rules
├── integration/
│   ├── extensions_install.txtar          # NEW — S1–S3
│   ├── extensions_start.txtar            # NEW — S4–S8, S12, S14
│   ├── extensions_import.txtar           # NEW — S9–S11
│   ├── extensions_interrupt_test.go      # NEW (pty) — S13
│   └── f5_demo_test.go                   # NEW — S15
└── fixtures/
    └── plugins/
        ├── context-suite/                # NEW — starter, linker, linker2, importer, importer2 (mode by env)
        ├── restricted-suite/             # NEW — components that must never start
        ├── runtime-sh/                   # NEW — Unix-only interpreted Starter (runtime: "sh")
        └── invalid-{event,input,dup-input,key-owner,manual,runtime}/   # NEW — manifest-only
```

**Structure Decision**: Single Go project (unchanged). `internal/extension` and
`internal/staging` are presentation-free and unit-testable in isolation, in the same shape as
`internal/locator` (F3) and `internal/plugininstall` (F4): they import `registry`, `work`,
`ipc`, `semconv`, `diag`, `projection` (an interface for provenance), `lockfile`,
`atomicfile` — never `cli` or `present`. `internal/cli` keeps every journey decision: it
decides *when* the pipeline runs (after the stable stdout lines, before shell integration),
what to render for each `Observer` event, and that the run's result is success. `semconv`
is a leaf so that `plugin` (install-time) and `starter` (run-time) can share one key grammar
without a cycle. `internal/create` keeps being the only owner of the Work's commit point.
No other package changes.

## Branching Strategy

F5 follows the same git-flow shape as F1–F4: one short-lived
`feature/006-automatic-start-context-p<n>-*` branch per `tasks.md` phase, each cut from the
previous phase's tip, merged forward at the phase **Checkpoint** once `make lint` +
`GIT_CONFIG_GLOBAL=/dev/null go test ./...` are green on the CI matrix.

> **Prerequisite**: F4 must have merged (or the phase-0 branch must be cut from F4's tip):
> every extension is registered through F4's installation pipeline and the chosen Starter
> comes from F4's matching. The current spec/plan branch, `feature/f5-automatic-context`, was
> cut from the F4 branch and still tracks it as upstream — push it explicitly
> (`git push -u origin feature/f5-automatic-context`) so it does not target the F4 branch.

> **Merge cadence is the user's call.** Prior phase branches were stacked (phase N+1's PR
> targets phase N's branch, not `develop`). Before starting a phase, `/speckit-implement`
> MUST confirm with the user which branch to base the new phase branch on and which branch its
> PR targets — do not assume `develop`.

| Phase (tasks.md) | Feature branch | Cut from |
|---|---|---|
| 0 — Specs & doc (this spec set, including Semantic Conventions v1 as `contracts/semantic-conventions.md`; ADR-0000/6/12/13 need no edit) | `feature/f5-automatic-context` | F4 tip |
| 1 — Foundational, no CLI wiring: `internal/semconv`; manifest types + `ParseInput` + M1–M7 (updating F1 placeholder tests); registry fields + `ComponentEntry` shared by install/bootstrap; `ipc.Target` + `RunContext` + `EntrypointPath` fix + install-time `LookPath`; `projection` v3 + `Reset`/provenance API (updating `user_version` assertions); `workhome.WorkLockPath`; `diag.Warning` + tokens; `diagrender.Warn` | `feature/006-automatic-start-context-p1-foundational` | phase 0 tip |
| 2 — US1 Install a package that declares Linkers and Importers 🎯 MVP: wire the above into `work plugin install` and bootstrap; fixtures `context-suite` (declarations only), `invalid-*`, `runtime-sh`; S1–S3 | `feature/006-automatic-start-context-p2-us1-install` | phase 1 tip |
| 3 — US2 Starter-published context: `starter.Reference{Meta,Links}` + `ValidateResponse`; `create.Params` + `UpsertWithProvenance`; `start.go` passes them; S4–S5 | `feature/006-automatic-start-context-p3-us2-context` | phase 2 tip |
| 4 — US3 Linker discovery: `internal/extension` (`Eligible`, Linker phase, locked persistence, ordering, failure→Warning, Observer); wire after `create.Run`; `renderWarning` + progress lines; S6–S8 | `feature/006-automatic-start-context-p4-us3-linkers` | phase 3 tip |
| 5 — US4 Importer + staging: `internal/staging` (stage, plan, incorporate), Importer phase, archive-preservation check; S9 | `feature/006-automatic-start-context-p5-us4-importers` | phase 4 tip |
| 6 — US5 Collision hardening: refusal matrix (symlink, reserved, escape, case), `WORK_FAIL_AT=incorporate:<n>` fault injection, byte-identical assertions; S10–S11 | `feature/006-automatic-start-context-p6-us5-collisions` | phase 5 tip |
| 7 — US6/US7 Failure semantics, interrupt, visibility, determinism, polish: failure-class matrix, `CommandContext` interrupt, `WORK_DEBUG` tail, `NO_COLOR`/`TERM=dumb` parity, ×20 determinism, full F1–F4 regression sweep, roadmap demo; S12–S15 | `feature/006-automatic-start-context-p7-polish` | phase 6 tip |

Rules (identical spirit to F1–F4):

- **Naming**: `feature/006-automatic-start-context-p<n>-<short>` — the phase identifier stays
  in one hyphen-separated segment under `feature/` (no nested path).
- **First action of each phase**: confirm base/target with the user, then
  `git switch <base> && git pull && git switch -c <feature-branch>`.
- **Merge gate**: a phase merges forward only after its **Checkpoint** in `tasks.md` is met
  and `make lint` + `go test ./...` are green on all three OSes, with every prior slice's
  automated demo (F1 S1–S12, F2 S1–S13, F2.5 Q1–Q12, F3 S1–S13, F4 S1–S14, and earlier F5
  phases) still green.
- **Release**: when phase 7 merges, F5 ships via `release/0.6` cut from `develop`, merged to
  `master` and tagged (standard git-flow release; `master` requires signed commits — check the
  ruleset via `gh api`, not local `git log`). The release step is the hand-off, not a task.

Spec/plan/contract/doc edits stay on the phase-0 branch; the per-phase feature-branch rule
covers implementation code only.

## Complexity Tracking

No Constitution Check violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
