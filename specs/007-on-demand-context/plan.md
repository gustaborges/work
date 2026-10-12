# Implementation Plan: On-Demand Context (F6)

**Branch**: `feature/on-demand-context` | **Date**: 2026-10-11 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/007-on-demand-context/spec.md`; roadmap F6; PRD FR-30, FR-31, FR-38 with FR-27–FR-28, FR-32–FR-35 and FR-50–FR-64; ADD §3, §4, §9–§12; accepted ADR-0000, ADR-0006, ADR-0012, ADR-0013, ADR-0019–ADR-0021.

## Summary

F6 exposes saved context and manual enrichment through three commands. `work status [WORK] [--json]` reads canonical state without extension execution or persisted changes. `work link [LINKER] [VALUE]` associates exact supplied text without a subprocess. `work import [IMPORTER]` executes exactly one eligible manual provider and incorporates all its artifacts or none.

Reuse the existing manifest/registry identities, input namespaces, IPC, atomic snapshot writer, per-Work lock and staging. Add a pure `internal/workcontext` reader; refactor `internal/extension` into reusable operation helpers with separate automatic and manual policy; extend staging to retain its undo list until cleanup and commitment are complete. Existing automatic start failures remain warnings and do not undo a Work.

Research found three prerequisites that the documents assumed already existed:

1. Current-Work resolution is absent, and existing global lookup helpers create/repair the index. Status needs a separate pure reader, full snapshot validation and a canonical directory scan.
2. Installed package enablement is not represented. Add one optional retained `enabled` field, default true for old data, and shared effective catalog filtering. F7 still owns administration commands.
3. Import incorporation currently holds no Work lock, cannot cancel copying, and ignores removal errors. Shared staging/import execution must be hardened to satisfy manual archive races, cancellation and accurate cleanup results.

These are directly required by FR-005, FR-008–FR-012, FR-016–FR-017 and FR-020–FR-025. No new plugin API, dependency, public lifecycle event or second canonical state is introduced. Snapshot schema and database version remain 3.

## Technical Context

**Language/Version**: Go 1.26.0, single module `github.com/gustaborges/work` (current `go.mod`).

**Primary Dependencies**: Existing Cobra 1.10.2 for command registration; Bubble Tea v2/Lip Gloss v2 through `internal/present`; `modernc.org/sqlite` 1.58.0 for optional path hints and operational provenance; standard library context, JSON, filesystem and subprocess facilities; system Git only for existing creation/archive and fixture setup. No dependency additions or upgrades.

**Storage**: Canonical `work-state.json` remains schema 3; readers continue accepting schemas 1–3. `work.db` stays `user_version=3`, with new provenance operation text `link` and no migration. `registry.json` adds optional retained `packages[].enabled`; no config/install-metadata field. One temporary import stage per execution, removed before artifact commitment.

**Testing**: Plan meaningful unit tables for context resolution, full canonical validation, manual eligibility/name ambiguity, input projection, publication races, cancellation and compensation. Use existing Go testscript integration harness and local native fixture packages; PTY/VT tests cover interactive journeys on Unix. Existing contract boundary tests protect presentation isolation and stable streams. No tests/builds are executed during this planning command.

**Target Platform**: Existing CI matrix Ubuntu/macOS/Windows; release platforms Linux amd64/arm64, macOS amd64/arm64 and Windows amd64. PTY-specific cases remain Unix-only; direct CLI, filesystem, state and IPC cases run on all three CI operating systems.

**Project Type**: Local single-binary CLI/TUI with external plugin subprocesses and embedded offline seed package.

**Performance Goals**: Current-directory lookup is bounded by ancestor depth. Identity lookup scans only immediate lifecycle directories, not checkouts or repository roots; O(number of Works), deterministic directory order. Optional index hint query is bounded to 250 ms and may be ignored. No extension execution timeout, extra network call or streaming status protocol. This slice introduces no speculative latency SLA.

**Constraints**: Status must never invoke bootstrap, migration/reconcile, Git, a plugin, lock creation or shell integration. Manual association preserves exact non-empty text and changes only one semantic link/current provenance. Import commitment serializes with archive, copies cancellably and never overwrites. Ordinary reads may affect OS access metadata, but no application state mutation occurs. Preserve existing command grammar, outputs, diagnostic tokens and automatic-warning semantics. No process sandbox, process-tree isolation or crash-recovery journal is added.

**Scale/Scope**: Single user/machine, local workspace directories and installed package catalog. Three commands; one new reader package; focused changes to extension/staging/registry/projection/diagnostics and effective-catalog call sites. No F7 enable/disable/update/uninstall CLI, explicit Work targets for mutations, link deletion, bulk enrichment or new official integration.

## Constitution Check

*GATE: evaluated before research and re-evaluated after design.*

`.specify/memory/constitution.md` is still the unratified template. Its placeholder examples are not adopted as project rules. As in earlier slices, the roadmap §4 gates, PRD and accepted ADRs govern this feature.

| Gate | Design decision | Planned evidence |
| --- | --- | --- |
| Determinism | Exact opaque Work IDs; explicit identity wins; canonical values; sorted qualified providers; ambiguous bare names fail; manual inputs share F5 resolver. | Resolver and eligibility tables; Q1–Q7, Q10; SC-001–SC-003. |
| Integrity | Full snapshot validation; latest locked link upsert; importer runs outside lock but plans/applies/cleans/commits inside it; retained undo ownership and error-aware rollback. | Snapshot fault injection, concurrent-link/archive tests, cancellation during copying, refusal matrix; Q8–Q12; SC-002, SC-004. |
| Process contract | Reuse F5 IPC/runtime/response cap; invoke one manual Importer or no process for link/status; manual errors are typed, automatic warnings preserved. | Native fixture input logs, failure-class table, streams/exit assertions; Q6–Q9, Q14; SC-002–SC-005. |
| Portability | Native path operations and escaped SQLite URI; no filesystem sensitivity assumptions; native preflight plus exclusive create; shared lock; regular output only. | Three-OS CI, paths with spaces/Unicode, reserved/case/symlink tests; Q1, Q8, Q11. |
| Auditability | Current provenance operation `link`; component/Work/operation diagnostics; no value echoes in mutation receipts; canonical status displays requested persisted values safely. | Provenance assertions, redacted-receipt and debug-output checks; Q4–Q5, Q14. |
| UX | Existing full-screen wizard and semantic theme per current PRD/ADR-0021; only omitted values collected; no empty selector; stable stdout and local status `--json`; grouped help. | PTY/VT sizes and cancellation, non-interactive usage, help inventory; Q10, Q13–Q14; SC-005–SC-006. |
| Regression | F1–F5 demos preserved; shared operation refactors retain F5 phase ordering/eligibility/warnings and reference-only behavior. | Existing suites plus F6 complete demonstration; Q15; SC-007. |

**Authority checks**:

- PRD FR-30/31/38 map to the three command contracts; FR-32–FR-35 preserve canonical state and semantic keys.
- ADR-0000/0006 remain unchanged: external process, explicit runtime, no plugin execution to determine eligibility.
- ADR-0012/0013 remain unchanged: role-based declarations, static input projection, last publication wins, index-only operational provenance and core-owned snapshots.
- ADR-0019/0020/0021 govern actual command inventory and presentation separation. The roadmap's historical inline wording is superseded by current accepted full-screen presentation.
- F5 protocol/version contracts remain applicable. F6 adds manual activation and typed error outcomes; package enablement is additive retained operational state, not a manifest shape change.

**Pre-research result: PASS.** No unjustified violation. The identified prerequisites are necessary to meet the approved specification and are included in the design rather than deferred.

### Post-design re-check

**Result: PASS.** Research R1–R12, data model, four contracts and Q1–Q15 resolve the initial unknowns. Pure status avoids all writing helpers; registry enablement has one source and compatibility default; manual inputs and operations reuse F5 contracts; cancellation/cleanup are part of shared commitment; no snapshot/database migration or new dependency is needed. Remaining implementation risks have explicit validation cases below. Tests have been planned, not run.

## Project Structure

### Documentation (this feature)

```text
specs/007-on-demand-context/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli-work-status.md
│   ├── cli-work-link.md
│   ├── cli-work-import.md
│   └── manual-context.md
└── checklists/
    └── requirements.md
```

`tasks.md` is generated next by `/speckit-tasks`; this command does not create implementation tasks or code.

### Source Code (repository root; proposed changes)

```text
internal/
├── workcontext/                 # NEW pure canonical Current/ByID resolution
│   ├── context.go
│   ├── resolve.go
│   └── *_test.go
├── cli/
│   ├── status.go                # NEW pure read + human/JSON view
│   ├── link.go                  # NEW wizard/direct association
│   ├── import.go                # NEW wizard/direct one-import operation
│   ├── context_shared.go        # NEW context/candidate composition and safe output helpers
│   ├── root.go                  # register groupInWork commands
│   └── diagnostics_border.go    # manual error/debug detail, one final renderer
├── extension/
│   ├── manual.go                # NEW candidates/direct resolution and manual activation checks
│   ├── publish.go               # NEW shared locked link publication
│   ├── import_one.go            # NEW shared staged single-import operation
│   ├── eligible.go              # shared typed input resolution + missing declarations
│   ├── linker.go                # automatic adapter around shared publication
│   ├── importer.go              # automatic adapter around shared import
│   └── *_test.go
├── staging/
│   ├── plan.go                  # full native destination preflight and directory-identity checks
│   ├── incorporate.go           # cancellable apply + retained undo + compatibility wrapper
│   └── *_test.go
├── registry/registry.go         # retained Enabled and raw/effective catalogs
├── projection/lookup.go         # NEW optional strictly read-only path hint
├── reconcile/reconcile.go       # extract pure selected-snapshot row conversion if needed
├── plugininstall/install.go     # preserve retained enablement during reinstall
├── bootstrap/bootstrap.go       # raw seed integrity, preserve retained enablement during repair
├── diag/diag.go                 # documented F6 manual failure categories
└── starter/, locator/, convention/, repoconfig/, cli/*_shared.go
                                # route activation to effective catalog; installed inventory stays raw

tests/
├── fixtures/plugins/manual-suite/
│   ├── plugin.json              # NEW manual providers + ineligible companions
│   └── main.go                  # NEW native offline failure/input-log modes
├── contract/
│   ├── manual_context_test.go   # NEW result/stream/exit and IPC parity
│   └── present_boundary_test.go # unchanged boundary gate
└── integration/
    ├── status.txtar             # NEW canonical views and pure lookup cases
    ├── link.txtar               # NEW association, replacement, no subprocess
    ├── import.txtar             # NEW selected import, refusal/failure cases
    ├── manual_context_errors.txtar
    ├── manual_context_test.go   # NEW race/input/debug/cancellation cases
    ├── manual_context_pty_test.go
    └── integration_test.go      # minimal fixture helpers/persisted disabled-state setup
```

**Structure Decision**: Keep the current single Go module and existing application/presentation boundary. `workcontext` imports work/path/diagnostics only and receives optional path hints through a narrow interface; no dependency on CLI, registry, Git, mutable projection or presentation. Extension/staging return values and typed failure causes. CLI owns collection, JSON/human presentation and final diagnostic translation. Use existing interfaces/clock hooks; add no generic framework or duplicate importer safety implementation.

## Phase 0 — Research (complete)

[research.md](research.md) contains twelve decisions with source-code evidence, rationale and rejected alternatives. It resolves pure lookup, missing enablement state, activation semantics, concurrent mutation, deferred artifact commitment, error categories and presentation. External SQLite references are limited to official documentation for immutable/read-only caveats. No unresolved technical clarification remains.

## Phase 1 — Design (complete)

- [data-model.md](data-model.md): canonical context/view, retained enablement, typed input evaluation, current provenance and commitment transitions.
- [Status contract](contracts/cli-work-status.md): exact read grammar, fields, human/JSON results, errors and purity.
- [Link contract](contracts/cli-work-link.md): association without subprocess, exact values, receipts, atomic commitment and provenance.
- [Import contract](contracts/cli-work-import.md): eligibility, unchanged IPC, staging, lock/cancellation/cleanup and result/error rules.
- [Shared contract](contracts/manual-context.md): effective catalog, internal interfaces, codes 39–49 and fixture behavior.
- [quickstart.md](quickstart.md): runnable future implementation validation Q1–Q15.
- Agent context: run the configured update script with this plan's project-relative path, then update the managed `CLAUDE.md` block to this feature, retaining unrelated instructions.

## Phase 2 — Implementation sequence for task generation

1. **Foundations**: pure Work context/current and identity resolution; optional read-only path hint; additive retained package state and effective catalog; F6 diagnostics. Protect raw install/bootstrap behavior and existing enabled defaults. Exit: canonical read/eligibility tables pass without exposing new CLI mutations.
2. **US1 status (MVP)**: register status and local visible `--json`; implement exact canonical human/JSON output and safe escaping; integrate grouped help. Exit: all pure-read scenarios including missing/corrupt index and legacy/archived snapshots pass, with zero plugins/Git/sidecars/locks.
3. **US2 association**: shared input/manual candidate evaluation and direct-name resolution; extract active/identity-checked publication; build one wizard/direct link command; provenance operation `link`. Exit: direct/interactive replacement and no-subprocess behavior pass with unchanged unrelated state.
4. **Shared import commitment + US3**: retain undo until final cleanup, cancellation-aware copying, full native preflight plus exclusive-create backstop, shared single-Importer executor and automatic adapters; wire manual import. Exit: successful, empty, failed, refused, interrupted and archive-race cases pass; F5 warnings/order/regression remain correct.
5. **US4/polish and regression**: unavailable/ambiguous/missing-input diagnostics, persisted disabled fixtures, terminal modes/receipts, cleanup recovery, all prior demos and the F6 end-to-end demonstration. Exit: lint plus the full supported-OS suite is green.

Implementation stays on the current feature branch unless the user selects a phase-branch delivery cadence. Do not infer base/PR target from the older F5 branching table. No implementation branch, commit, PR or release is created by planning. Phase-level git-flow delivery can be chosen during implementation without changing this dependency sequence.

## Validation Strategy

Use unit tests where behavior is pure, and integration/contract tests where correctness depends on files, subprocesses, terminal streams or races. Existing fixture helpers build local native binaries so no service account/network is needed. Run `make seed` before host tests to avoid seed-dependent skips; run `make seed-all`, `make lint` and `go test ./...` in the existing CI matrix. Unix PTY tests supplement portable testscript cases.

| Spec coverage | Validation focus | Quickstart |
| --- | --- | --- |
| FR-001–FR-006; SC-001 | Active/archived/current/explicit contexts; validation errors; missing checkout; 20 repeated reads; existing corrupt/missing/stale/WAL index without writes; old workspace discovery boundary. | Q1–Q3, Q11 |
| FR-007–FR-012; SC-003 | Effective enabled state, raw bootstrap/install retention, role/manual/inputs, provider ambiguity and post-selection revalidation. | Q6–Q7, Q10, Q12 |
| FR-013–FR-017; SC-002 | Exact values, atomic update refusal/cancel, concurrent unrelated updates, last source wins, best-effort index provenance, zero subprocess. | Q4–Q5, Q12 |
| FR-018–FR-025; SC-004 | Only selected importer inputs/process; nested/empty output; full refusal/failure matrix; archive race; interrupted copying; cleanup/rollback errors and prior artifacts preserved. | Q6, Q8–Q9, Q11–Q12 |
| FR-026–FR-030; SC-005–SC-006 | Omitted/explicit arguments, no non-interactive controls, full-screen geometry, redacted receipts, color modes, stdout/stderr and debug detail. | Q10, Q13–Q14 |
| FR-031; SC-007 | Original F1–F5 demos/contracts and complete F6 workflow. | Q15 |

Tests should assert user-observable results and failure invariants, not mirror implementation or create redundant broad sweeps. Use deterministic synchronization for cancellation/races; no timing-only sleeps as proof. Hash snapshots/artifact trees and inspect operational rows without opening a writable verification connection between purity measurements.

## Risks and boundaries

- **Index hints** can be stale or unreadable, especially with a changing WAL database. They supply locations only; canonical reads validate every result. Never silently add repair to status.
- **Enablement** is retained operational registry state. Reinstall/seed repair must preserve it; raw/effective confusion could reset state or silently reenable the seed. Shared-name convention semantics retain the historical name/prefix model; F7 must validate user-facing transitions.
- **Shared refactoring** touches F5 automatic behavior. Use compatibility wrappers and adapters, retaining phase ordering, output limits, warnings and reference-only output; regression is a release gate.
- **Compensation can fail** when the OS refuses removal. Report recovery precisely and retain safe unresolved paths/cause details; do not claim byte identity in such cases. The spec bounds integrity to recoverable failures, not machine crashes or hostile independent writers.
- **Status lookup scope** cannot discover an unconfigured old root without any usable hint. Current-directory status still works there and guidance explains the direct recovery path.

## Complexity Tracking

No Constitution Check violations require an exception. The added reader package, retained package field and staging undo lifecycle each address a specific approved requirement and avoid competing state/execution models.
