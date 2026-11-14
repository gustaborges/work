# Tasks: Per-mode Starter Links and Known-link Eligibility

**Input**: [plan.md](plan.md), [spec.md](spec.md), research.md, data-model.md, contracts/
**Tests**: Explicitly required by issue #61 and FR-009. Implementation progress and validation evidence are recorded below.
**Organization**: Shared setup/foundations, then independently testable US1 and US2, then combined validation. `[P]` means work in separate files after prerequisites, not permission to run the same fixture edits concurrently.

## Phase 1: Setup

- [X] T001 Inspect the existing fixture builders and disposable Git setup in tests/contract/starter_context_test.go, tests/integration/start_modes_test.go, and tests/integration/start_by_plugin_starter_test.go; identify reusable native fixtures and preserve no-live-GitHub/no-real-home execution (FR-009).

## Phase 2: Foundational

- [X] T002 Add an optional raw modes payload to StarterResponse in internal/ipc/ipc.go and carry it through internal/starter/starter.go without converting malformed modes into UnusableRepo; keep existing malformed legacy stdout and subprocess failure classification (FR-001/FR-003).
- [X] T003 Add IPC round-trip/decode regression cases in internal/ipc/ipc_test.go for absent, null, object, and wrong-shaped raw modes and unchanged legacy fields/input; run the targeted IPC tests (FR-001/FR-003/FR-004).

## Phase 3: US1 — Mode-specific Starter context (Priority: P1)

**Goal**: One response publishes context appropriate to the chosen creation mode.
**Independent test**: Deterministic Starter responses produce selected contribution/fork/new snapshots; invalid unselected output fails 37 with no creation.

- [X] T004 [US1] Add table-driven validation cases in internal/starter/starter_test.go for all three recognized override names, unknown names, invalid unselected keys/ownership/link values, wrong-shaped modes/entries/maps, absent/null/empty maps, and deterministic value-free diagnostics (FR-003/FR-004).
- [X] T005 [US1] Decode mode context and extend ValidateResponse/validatePublished in internal/starter/starter.go to validate base and every entry before any materialization; sort names and preserve existing start_modes/base_branch rules; return starter-response-invalid (37) for all invalid new modes shapes (FR-003/FR-004).
- [X] T006 [US1] Add pure merge tests in internal/starter/starter_test.go for contribution/fork/new, missing selected entry, base preservation, selected-key precedence, shallow nested metadata replacement, explicit metadata null, and response-map immutability (FR-002/FR-004).
- [X] T007 [US1] Implement a copying selected-mode merge helper in internal/starter/starter.go that overlays only links/meta, excludes unselected data, and leaves repository/base/start_modes untouched (FR-001/FR-002).
- [X] T008 [US1] Extend internal/starter/bind_test.go to merge then Bind public/private selected fields and assert current provider/source-instance ownership and no response mutation (FR-005).
- [X] T009 [US1] Retain the validated unbound response across internal/cli/start.go mode collection, replace the early Bind with merge-then-Bind after final mode resolution and before create.Run, preserve provider revision checks and reset response context on source retries; pass effective links/meta to existing atomic creation (FR-001/FR-002/FR-005).
- [X] T010 [US1] Extend subprocess fixtures under tests/fixtures/plugins/ and tests/contract/starter_context_test.go for one-invocation mode output, unchanged arg-only stdin, no-override publication, invalid unselected fields and code-37 diagnostics (FR-001/FR-003/FR-004/FR-009).
- [X] T011 [US1] Extend tests/integration/start_modes_test.go and tests/integration/start_by_plugin_starter_test.go for contribution/fork/new selected snapshots, legacy publication, private binding, and malformed/unknown/unselected entry failures; assert unchanged refs and absence of new worktree/Work/snapshot on code 37 (FR-002/FR-003/FR-004/FR-005/FR-009).

**Checkpoint**: US1 can be demonstrated independently; both stories remain required for issue delivery.

## Phase 4: US2 — Silent known-link suppression (Priority: P1)

**Goal**: Automatic start discovery preserves known links without affecting manual association or Importers.
**Independent test**: Marker-writing known-key Linkers never run or warn; missing-key controls do, with ownership and phase semantics preserved.

- [X] T012 [P] [US2] Extend internal/extension/eligible_test.go with known/missing public keys, empty-but-present values, nil maps, known-key plus ambiguous Starter filter, Applicable false when only known-key Linkers exist, and unchanged Importer/unrelated-event behavior (FR-006/FR-007).
- [X] T013 [P] [US2] Extend internal/extension/provider_test.go with current provider/source-instance known private links and other-provider/former-instance controls using existing storedKey semantics (FR-005/FR-006).
- [X] T014 [US2] In internal/extension/eligible.go, suppress RoleLinker on start:finalized when storedKey resolves to a present Subject.State.Links entry, before subscribed or input resolution; do not alter manual publication or fixed decision ordering (FR-006/FR-007/FR-008).
- [X] T015 [US2] Extend internal/extension/run_linker_test.go with marker fixtures proving zero execution/events/diagnostics/provenance for known keys, missing-key execution, fixed inputs, two initially eligible same-key Linkers, and no eligibility chaining; preserve internal/extension/run.go's single pre-loop decision pass (FR-006/FR-007/FR-008).
- [X] T016 [US2] Extend tests/integration/start_by_plugin_starter_test.go to prove Starter-known links survive automatic discovery and feed eligible Importers while missing links are discovered and imported after the Linker phase; assert silent skipped receipts (FR-006/FR-008/FR-009).
- [X] T017 [US2] Extend tests/integration/link.txtar and internal/cli/link_test.go as needed to verify manual replacement of a known link without invoking discovery; run the existing manual-link regressions (FR-007/FR-009).

**Checkpoint**: Known links are protected, missing-key behavior and importer phase stay operational.

## Phase 5: Polish and combined acceptance

- [X] T018 Check the implemented wire/phase behavior against specs/005-plugin-origins/contracts/starter-protocol.md, specs/010-link-improvements/contracts/, docs/prd.md, docs/adr/adr-0031-mode-context-known-links.md, and docs/add/add-0001-work-system-architecture.md; correct documentation drift without adding out-of-scope plugin/upstream changes (FR-009).
- [X] T019 Run the focused suites and full go test ./... plus go vet ./... from specs/010-link-improvements/quickstart.md, inspect native supported-OS CI results, and record actual acceptance evidence and any limitations in that guide; do not mark cross-compilation as native execution (FR-009, SC-001–SC-004).

## Dependencies and execution order

```text
T001 → T002 → T003 → US1: T004 → T005 → T006 → T007 → T008 → T009 → T010 → T011
                   ↘ US2: (T012 || T013) → T014 → T015 → T016 → T017
US1 + US2 → T018 → T019
```

The US2 eligibility unit work is independent of US1 after setup; its foundation arrow is an organizational gate, not a data dependency. T016 touches the same integration file as T011, so complete T011 before T016 if both stories are developed concurrently. Within-story tests precede implementation; fixtures may be added with their owning test task. T002 lays transport foundations, not a separate deliverable.

## Parallel examples

- US1: T008 binding tests can be prepared alongside T009 CLI integration after T007; they edit separate files, but run both before T010.
- US2: T012 eligibility tests and T013 provider-isolation tests are independent files; both precede T014.
- Across stories: US1 Starter/CLI implementation and US2 eligibility unit changes can proceed independently; serialize their shared integration fixture/file edits.

## Implementation strategy

First demonstrate US1 with one accepted response per resolution and atomic selected context. Then demonstrate US2 using existing known context even without modes. Deliver both in one change as requested by issue #61; the first increment alone is not issue completion. Run the combined validation before review. No implementation/convergence occurred during this planning pass.

## Requirement coverage

| Requirement | Tasks |
| --- | --- |
| FR-001 | T002, T003, T007, T009, T010 |
| FR-002 | T006, T007, T009, T011 |
| FR-003 | T002–T005, T010, T011 |
| FR-004 | T003–T006, T010, T011 |
| FR-005 | T008, T009, T011, T013 |
| FR-006 | T012–T016 |
| FR-007 | T012, T014, T015, T017 |
| FR-008 | T014–T016 |
| FR-009 | T001, T010, T011, T016–T019 |
| SC-001–SC-004 | T010, T011, T015–T017, T019 |

Validation: full `go test ./...` and `go vet ./...` passed on Linux. The full integration package passed in 345.415s. Native macOS/Windows CI results are pending PR publication; see quickstart.md for evidence and limitations.
