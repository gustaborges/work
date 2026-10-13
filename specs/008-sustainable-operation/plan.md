# Implementation Plan: Sustainable Platform Operation (F7)

**Branch**: `feature/sustainable-operation` | **Date**: 2026-10-12 | **Spec**: [spec.md](spec.md)

**Input**: `specs/008-sustainable-operation/spec.md`; roadmap F7 and §4; PRD FR-18–FR-20, FR-25, FR-40–FR-41, FR-45, FR-47–FR-48 and FR-50–FR-64; accepted installation/update/bootstrap, convention, Locator policy, and terminal ADRs.

## Summary

Complete plugin lifecycle management and administrative hubs while preserving explicit repository resolution. Add enable/disable, pure update checks, reviewed fixed-candidate updates, dependency-aware uninstall, durable bundled-package removal, and complete plugin/repository hubs. Preserve unavailable policy entries and remembered conventions until users explicitly change them.

Research identified prerequisites beyond registering commands: current reads bootstrap/create state; package swaps outlive their rollback backup before registry publication; bootstrap lacks durable removal intent and an owned Package record; mutable alias paths can race component execution. Resolve these with a small administrative state layer and package service, reusing existing validators, locks, atomic JSON writes, staging primitives and presentation. No Work snapshot, index, plugin protocol, or dependency migration is required.

## Technical Context

**Language/Version**: Go 1.26.0; existing module `github.com/gustaborges/work`.

**Primary Dependencies**: Cobra 1.10.2; Bubble Tea 2.0.9/Lip Gloss 2.0.6 through `internal/present`; standard library; existing `golang.org/x/sys` atomic replacement/locking support; existing Git wrapper. `modernc.org/sqlite` 1.58.0 remains for existing Work projection only. No additions/upgrades.

**Storage**: Existing config/work.json, state/registry.json, state/branch_conventions.json and plugins/<alias> layout. Add optional package version/source/content_digest; schema-1 state/reference-package.json; schema-1 state/administration.json epoch/control; one finite transaction journal plus owned before-images/backups under state/administration/ and same-filesystem plugin staging/backup directories. Work snapshot schema and database user_version stay 3. No administrative state in Work snapshots or work.db.

**Testing**: Existing Go unit/testscript/native fixture/PTY suites. Add failure checkpoints for prepare, pending publication, every move/JSON publication, commit marker, stable publication and cleanup; deterministic read/write/invocation races. No application builds or tests executed by this planning command.

**Target Platform**: CI Ubuntu/macOS/Windows; release Linux amd64/arm64, macOS amd64/arm64 and Windows amd64. Unix PTY tests supplement portable direct-command/filesystem tests. Keep Windows symlink capability handling from existing linked-install tests.

**Project Type**: Local single-binary CLI/TUI with external plugin subprocesses and embedded offline seed.

**Performance Goals**: Pure administrative reads inspect only administrative JSON and bounded installed-package metadata, not Work trees or repository roots. At most three coherent-read retries. Update retrieval occurs only explicitly and once per candidate; source retrieval and user input never hold the write lock. Existing 30-second lock acquisition limit remains; no plugin runtime timeout or speculative latency SLA.

**Constraints**: No implicit policy insertion/order changes, background updates, automatic convention replacement, component execution during preview/validation, read-path bootstrap/repair/lock creation, or external linked-source deletion. Preserve old grammar/result lines/error codes except documented additive state, maintenance commands and intentional F7 validation/hub changes. All administrative writers must participate in the shared protocol; no admin-lock acquisition under a Work lock.

**Scale/Scope**: One local user's package catalog, global ordered Locator policy/root collection, remembered conventions, three hubs, explicit package batches and one in-flight committed administrative transaction per Work home. Updates commit per package; multi-target uninstall commits as a set. Signing, marketplaces, official integrations, source channels and telemetry remain excluded.

## Constitution Check

*Gate evaluated before research and re-evaluated after design.*

The constitution is an unratified template; placeholder examples are not adopted rules. The established PRD, accepted ADRs and roadmap §4 supply the applicable gates.

| Gate | Design decision | Verification evidence planned |
| --- | --- | --- |
| Determinism | Alias targets; opaque version inequality; fixed reviewed candidate; explicit policy order; no hidden cascade/fallback. | Candidate and dependency tables; Q2–Q8; SC-001–SC-005. |
| Integrity | Shared lock, latest-state revalidation, retained before-images, explicit journal commit marker, stable/pending epoch and bounded pure reads. | Failure/checkpoint matrix, concurrent writers/readers and restart recovery; Q8–Q10. |
| Process contract | No plugin execution during management; current provider revision checked at invocation; unchanged IPC/projection and warning semantics. | Native fixture invocation logs and activation/update race; Q3, Q5, Q10, Q13. |
| Portability | Reuse platform atomic-file helpers and locks; same-filesystem package moves; never traverse external linked source during uninstall. | Three-OS fault/rename/symlink tests and Unicode/spaced paths; Q7–Q10, Q13. |
| Auditability | Explicit package/reference/operation outcomes; safe source projections; retained recovery evidence; unchanged Work history. | Output/credential tests and byte-tree comparisons; Q1, Q5, Q7–Q9, Q12. |
| UX | Shared service for hub/direct actions; full-screen wizard; determined impact; separate dependency authorization and --yes. | Hub/direct parity, noninteractive/PTY checks and help inventory; Q6, Q11–Q12. |
| Regression | F1–F6 journeys preserved; deliberate F7 updates to old help-only hubs, bootstrap-on-read and duplicate-collapse expectations documented. | Complete existing suites, supported upgrade, index rebuild and ambiguous branches; Q13. |

**Pre-research result: PASS.** Required transaction/recovery and read-purity prerequisites are in scope; no unjustified rule violation.

### Post-design re-check

**Result: PASS.** Research R1–R12 resolves stack, legacy version baseline, source update policy, transaction/recovery, read coherence, seed lifecycle, batch semantics, dependencies, convention availability and invocation lock ordering. Five contracts and Q1–Q13 provide observable evidence. Additional embedded install syntax is necessary to meet FR-024 without an external source or reserved-alias bypass. F7 duplicate rejection and pinned symlink containment are intentional requirement-specific validation changes, not an accidental regression. No unresolved clarification remains.

## Project Structure

### Documentation (this feature)

```text
specs/008-sustainable-operation/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli-work-plugin.md
│   ├── cli-work-repository.md
│   ├── cli-work-convention.md
│   ├── administrative-state.md
│   └── administrative-hubs.md
└── checklists/requirements.md
```

`tasks.md` is generated next by `/speckit-tasks`; planning creates no implementation tasks or source changes.

### Source Code (repository root; proposed)

```text
internal/
├── adminstate/                 # NEW low-level finite admin snapshot/transaction layer
│   ├── snapshot.go             # epoch-guarded pure Read + legacy projections
│   ├── transaction.go          # WithWrite, staged before/after images and moves
│   ├── recovery.go             # prepared rollback; committed forward/cleanup
│   ├── lifecycle.go            # durable reference-package history
│   ├── invocation.go           # existing lock around revalidation + invocation callback
│   └── *_test.go
├── pluginadmin/                # NEW package management domain service
│   ├── inventory.go            # raw views/version fallback/safe output projection
│   ├── enable.go               # atomic deduplicated set transitions
│   ├── update.go               # prepare/check/review/apply exact candidate
│   ├── uninstall.go            # whole-set removal and policy impacts
│   └── *_test.go
├── plugininstall/              # extract pure registration/validation and prepared install
│   ├── install.go              # compatibility wrapper; no early backup deletion
│   ├── prepare.go              # NEW frozen source/digest/runtime validation
│   └── swap.go                 # transaction-owned undo/cleanup primitives
├── bootstrap/bootstrap.go      # legacy adoption, retained lifecycle, locked seed preparation
├── registry/registry.go        # additive metadata and owned embedded package
├── workhome/workhome.go        # admin paths
├── repoconfig/{policy,roots}.go # raw installed lookup, stale retention, cloned root removal
├── repoconv/repoconv.go         # pure availability projection; serialized upserts via callers
├── cli/
│   ├── admin_shared.go         # NEW service composition + impact/result adapters
│   ├── plugin*.go              # new enable/disable/update/uninstall and full hub
│   ├── repository*.go          # full hub; pure read/mutation separation
│   ├── convention*.go          # availability and pure show; shared set service
│   ├── start.go                # safe setup/memory upserts; unavailable-choice handling
│   ├── context_shared.go       # guarded live catalog revalidation
│   └── diagnostics_border.go   # additive admin errors + secret-safe public causes
├── workspace/workspace.go      # latest-state administrative config update
├── starter/, locator/          # invocation callbacks with current revision guard
├── extension/                  # guarded invocation; live automatic Reload; no lock inversion
└── diag/diag.go                # additive categories 50–56

tests/
├── fixtures/plugins/admin-suite/ # NEW native Locator + convention + optional other roles
├── contract/                   # additive JSON/output/errors, presentation boundary
└── integration/
    ├── plugin_management.txtar
    ├── plugin_update_test.go    # local file:// Git sources and fixed-candidate races
    ├── plugin_uninstall_test.go
    ├── admin_recovery_test.go   # process interruption/checkpoint recovery
    ├── admin_purity_test.go     # tree/bytes/existence; 20 reads/checks
    ├── admin_concurrency_test.go
    ├── admin_hubs_test.go       # Unix PTY direct/hub parity and cancellation
    └── release_candidate_test.go
```

**Structure Decision**: `adminstate` owns only the finite state protocol and typed snapshots, using config/registry/repoconv/workhome/lock/atomic primitives; those storage packages do not import it. `pluginadmin` owns package policy and imports adminstate, plugininstall and bootstrap's seed preparation. Bootstrap imports adminstate/plugininstall but never pluginadmin, avoiding cycles. CLI alone imports presentation and renders typed results. Invocation guard accepts callbacks so IPC remains independent of administrative storage.

## Phase 0 — Research (complete)

[research.md](research.md) records twelve decisions, code evidence, rationale and rejected alternatives. Official Go/Git documentation supports the portability boundary and remote default-revision fixtures. No external account or service is required to prove behavior.

## Phase 1 — Design (complete)

- [data-model.md](data-model.md): metadata compatibility, read projections, lifecycle, candidates, review preconditions and exact journal transitions.
- [Plugin contract](contracts/cli-work-plugin.md): grammar, origin matrix, version gate, confirmations, dependency flag, outputs and error codes.
- [Repository contract](contracts/cli-work-repository.md): additive state/reasons, explicit policy operations, roots and validation changes.
- [Convention contract](contracts/cli-work-convention.md): pure remembered-choice inspection and explicit unavailable-choice recovery.
- [State contract](contracts/administrative-state.md): internal service boundaries, journal/epoch recovery, writer inventory and invocation ordering.
- [Hub contract](contracts/administrative-hubs.md): keyboard journeys, impact review, direct equivalents and terminal behavior.
- [quickstart.md](quickstart.md): runnable future validation, fixture setup, targeted tests and full release gate.
- Agent context: run the configured agent-context script with `specs/008-sustainable-operation/plan.md`; update the managed CLAUDE.md block to F7 while preserving unrelated instructions.

## Phase 2 — Implementation sequence for task generation

1. **Foundations**: additive metadata, owned seed projection/lifecycle, pure admin snapshot/epoch; low-level journal with complete failure/restart tests. Extract prepared install/registration and transaction-owned swap. Route every existing administrative writer through the shared lock/protocol and land invocation protection/live post-run revalidation before enabling new lifecycle mutations. Exit: existing install/bootstrap now commit coherently; fresh reads write nothing; legacy adoption never reseeds user policy.
2. **US1 + US2 direct surface**: inventory fields, atomic enable/disable sets, Locator reason matrix, raw installed policy add/replace, unavailable retention and root mutation safety. Exit: deterministic state transitions, policy byte preservation on toggles, no disabled activation and unchanged resolution chain semantics.
3. **US3 updates**: immutable candidates/origin handling, pure checks, current-baseline validation, exact reviewed apply, per-package results, dependency-aware withdrawal, source redaction and interruption recovery. Exit: candidates cannot drift between preview/apply; invalid/failing updates preserve old installation.
4. **US4 removal**: whole-set transaction, ownership-safe linked removal, dependency authorization, bundled removal/reinstall and upgrade behavior. Exit: no implicit resurrection, no partial set removal, no Work/convention-memory deletion.
5. **US5 hubs/conventions + activation regression**: complete hubs through shared services; pure availability-aware show; explicit replacement; exercise the foundation invocation guard across all roles and live revalidation before publication. Steps 2–4 depend on those guards; they must not ship independently without them. Exit: hub/direct parity, no lock inversion, stable results and restored terminal.
6. **Release candidate**: portability/upgrade/interruption, clean setup, index rebuild, real ambiguous branch fixtures and all previous demos; complete contract/help/credential matrix. Exit: existing lint/full suite and supported-OS CI green.

Dependencies are deliberate: no maintenance mutation may bypass foundations; source retrieval remains outside transactions. Agent context and design are the only non-spec documents changed during planning. No commit, PR, implementation branch, build or release is created here.

## Validation Strategy

| Spec coverage | Focus | Quickstart |
| --- | --- | --- |
| FR-001–FR-007; SC-001–SC-002 | Legacy inventory/seed adoption; enabled defaults; role filtering; unchanged policy on all install/toggle/update paths. | Q1–Q3, Q9 |
| FR-008–FR-010; SC-002 | Raw disabled additions, stale retention, duplicate rejection, root all-or-none, deterministic chain outcomes. | Q3–Q4 |
| FR-011–FR-019; SC-003–SC-004 | Source origin matrix, opaque versions, fixed snapshots, candidate validation/name ownership, batches and aggregate failure. | Q1, Q5–Q6, Q8, Q10 |
| FR-020–FR-024; SC-005 | Exact approved dependency removal, entire uninstall set, linked sources, durable seed lifecycle and supported upgrade. | Q7–Q9 |
| FR-025–FR-026; SC-007 | Shared owners, unavailable memory, explicit replacement across clones, unchanged Works. | Q11 |
| FR-027–FR-031; FR-033; SC-006 | Hubs, flags, help, UI/results, keyboard, color, cancellation and source credentials. | Q6, Q11–Q12 |
| FR-032; SC-004–SC-005 | All journal boundaries, coherent pure readers, concurrent latest-state writes and provider invocation/commit races. | Q8, Q10 |
| FR-034; SC-008 | Full F7 journey; F1–F6; clean setup/upgrade/rebuild and supported platform matrix. | Q13 |

Run `make seed` before host tests; retain existing `make seed-all`, `make lint`, `go test ./...` CI gates. Fault hooks belong to injected filesystem/service seams or test helper process modes, not production environment-driven mutation controls. Synchronize races with channels/pipes/checkpoints rather than timing-only sleeps. Compare home-tree existence and bytes plus managed content, linked sources and Work trees; never open a writable verifier during purity tests.

## Risks and Boundaries

- **Missing writer participation** breaks the epoch guarantee. Audit all config.Save/registry.Save/repoconv.Save callers, including start/workspace/bootstrap; add a focused call-site/boundary check.
- **Invocation protection** serializes external component execution against maintenance. A long-running component can make administration reach its existing acquisition timeout; do not add an execution timeout or hold the lock over user input.
- **Journal scope** is finite administrative JSON and owned package directories. It is not a new canonical Work model. Invalid records or failed compensation stop activation and require explicit mutating recovery; pure reads never repair them.
- **Legacy state** cannot reveal a missing installed version reliably; show unknown and require explicit reinstall to establish a baseline. Preserve known enablement/policy history while adopting seed ownership.
- **Compatibility** retains output keys/receipts and error tokens; new fields, unavailable labels, true hub behavior and strict policy duplicate and pinned symlink validation are intentional F7 changes requiring updated expectations.
- **Durability** covers tested process interruptions on supported platforms; no promise is added for arbitrary storage loss, hostile external linked-source edits, or concurrent older-version binaries bypassing the new protocol. Supported upgrade is forward migration with fixtures, not mixed-version simultaneous operation.

## Complexity Tracking

No governing gate violation requires an exception. The two new packages separate low-level administrative consistency from package management policy. The finite journal, epoch control and external seed lifecycle record are necessary for explicit FR-023/FR-024/FR-028/FR-032 guarantees that existing one-file writes and early backup cleanup cannot provide.
