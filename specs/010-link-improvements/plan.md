# Implementation Plan: Per-mode Starter Links and Known-link Eligibility

**Branch**: `feature/link-improvements` | **Date**: 2026-11-14 | **Spec**: [spec.md](spec.md)
**Input**: Issue #61; PRD FR-84–FR-86; ADR-0031; ADD §§7, 9, 11.
**Status**: Planned through tasks; implementation not started.

## Summary

Extend the existing single Starter response with optional per-mode links/meta. Validate the entire response before any materialization, retain its unbound context through mode selection, shallow-merge the chosen mode, then bind once and pass the existing maps to `create.Run`. During start eligibility, silently suppress Linkers for already-present stored keys before subscription diagnostics. Preserve fixed Linker decisions, later Importer evaluation, manual upserts, and current provider revision guards.

## Technical Context

**Language/Version**: Go 1.26.0, module `github.com/gustaborges/work`.
**Primary Dependencies**: Existing standard JSON/maps/slices packages, Cobra 1.10.2, Bubble Tea 2.0.9; no new dependency.
**Storage**: Existing canonical work-state.json links/meta and work.db projection; no schema migration or new persisted modes field.
**Testing**: Go unit, subprocess contract, and disposable-repository integration tests explicitly required by FR-009. Use native fixture executables and existing test helpers.
**Target Platform**: Linux, macOS, Windows; retain platform-neutral IPC, map logic, and CLI behavior.
**Project Type**: Single-binary CLI/TUI orchestrating external components.
**Performance Goals**: One Starter invocation per accepted resolution attempt; one fixed Linker eligibility pass per phase; zero subprocesses/events for known-key Linkers. No timing SLO added.
**Constraints**: Invalid mode entries, even unselected, fail code 37 before creation; private values use real provider/source instance; no second mode-aware invocation; no plugin guessing fix or branch upstream change.
**Scale/Scope**: One response with three possible mode names, two maps per entry; existing output limits and Work/component inventory apply.

## Constitution Check

The constitution is an unratified placeholder and supplies no adopted MUST principles. PRD and accepted ADRs govern product/architecture.

| Gate | Pre-research / post-design evidence |
| --- | --- |
| Requirement authority | FR-84–FR-86 formalize issue #61; FR-32 clarifies upsert behavior after eligibility. |
| Minimal core/trust boundary | ADR-0000/0006/0012 retained: one external invocation, unchanged input, only typed contextual output consumed. |
| Ownership | ADR-0013/0024 and existing Bind/storedKey retain private provider and source-instance isolation. |
| Persistence | Merge before create.Run; existing atomic snapshot path and revision revalidation retained. |
| Phase semantics | ADR-0031 adds a static skip; Linker batch then Importers and manual replacement remain intact. |
| Scope | Separate feature 010; no implementation, migrations, external marketplace edits, or branch tracking changes. |

Pre-research: PASS. Post-design: PASS. All research questions resolved in research.md.

## Project Structure

### Documentation (this feature)

```text
specs/010-link-improvements/
  spec.md
  checklists/requirements.md
  plan.md
  research.md
  data-model.md
  quickstart.md
  contracts/starter-protocol.md
  contracts/linker-eligibility.md
  tasks.md
```

### Source Code (repository root)

```text
internal/ipc/ipc.go                 # retain/decode optional modes wire payload
internal/starter/starter.go         # mode type, full validation, pure merge, Bind
internal/cli/start.go               # retain response, select mode, merge/bind/create
internal/extension/eligible.go      # start Linker presence guard through storedKey
internal/starter/*_test.go
internal/extension/*_test.go
internal/ipc/*_test.go
tests/contract/starter_context_test.go
tests/integration/start_modes_test.go
tests/integration/start_by_plugin_starter_test.go
tests/integration/link.txtar
tests/fixtures/plugins/             # deterministic author fixtures
specs/005-plugin-origins/contracts/starter-protocol.md
docs/prd.md
docs/adr/adr-0031-mode-context-known-links.md
docs/add/add-0001-work-system-architecture.md
```

**Structure Decision**: Reuse IPC → starter → CLI → create flow and extension eligibility. Do not put mode selection into IPC, duplicate persistence, or add guards to manual PublishLink. ADR-0031 records the architectural choice; contracts here define its wire/phase details.

## Phase 0 — Research

Complete: [research.md](research.md), R1–R5. Important failure-path detail: adding a typed modes map directly to IPC would turn wrong-shaped values into UnusableRepo, violating code 37. Preserve the raw modes field for Starter-level validation.

## Phase 1 — Design

Complete: [data-model.md](data-model.md), [Starter contract](contracts/starter-protocol.md), [eligibility contract](contracts/linker-eligibility.md), and [quickstart.md](quickstart.md). Context hook updates AGENTS.md to this plan. No application tests are claimed run in planning.

## Phase 2 — Task planning

Tasks grouped by US1 and US2 with shared setup/foundations and final combined regression validation. US1 is an independently testable first increment; both stories must ship together for issue #61. User requested stopping after tasks and analysis; implementation and convergence are deferred.
