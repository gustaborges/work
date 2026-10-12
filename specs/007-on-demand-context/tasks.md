# Tasks: On-Demand Context (F6)

**Input**: Design documents from `specs/007-on-demand-context/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, and all four documents in `contracts/`.
**Tests**: Included to verify the specification's acceptance scenarios and automated success criteria SC-001–SC-007. Add the relevant behavioral tests before implementation; establish the expected failure without replacing an existing contract with an implementation-specific assertion.
**Organization**: Shared prerequisites precede independently verifiable story increments. Story milestones follow specification priority: US1 (P1), then US2, US3, US4 (P2).

## Milestone Index

Use **Milestone 1–7** or **M1–M7** when directing the implementation agent. These names map to the task ranges below; user-story labels remain attached to individual tasks for specification traceability.

| Milestone | Name | Task range | Complete when |
| --- | --- | --- | --- |
| **M1** | Test infrastructure | T001–T002 | Offline manual-provider fixtures and the isolated test harness are ready. |
| **M2** | Foundations | T003–T014 | Pure canonical Work lookup, retained package enablement, diagnostics and shared command support pass their focused checks. |
| **M3** | Status — US1 / MVP | T015–T020 | Human/JSON status works for active and archived Works, with repeated-read proof of zero application writes or plugin/Git execution. |
| **M4** | Manual link — US2 | T021–T033 | Interactive/direct association preserves exact text, publishes atomically, records current provenance when available and executes no plugin. |
| **M5** | Manual import — US3 | T034–T048 | Exactly one selected importer runs; safe output commits completely, recoverable failures/cancellation roll back, and successful artifacts survive archive. |
| **M6** | Errors and interaction — US4 | T049–T057 | Context/provider/input/automation failures are actionable and rendered once; terminal geometry, receipts and streams meet the contracts. |
| **M7** | Final validation | T058–T062 | F6 demonstration, documentation, regression checks and supported-platform validation are complete, with unavailable environments recorded explicitly. |

**Default delivery order**: M1 → M2 → M3 → M4 → M5 → M6 → M7. **First usable delivery (MVP)**: M1–M3, ending at T020. Detailed technical dependencies and permitted parallel batches are listed below.

### Instructions for milestone-scoped implementation

- “Implement milestone 4” means M4 / T021–T033; “implement through milestone 4” means M1–M4 / T001–T033. The same mapping applies to every milestone.
- Before starting, inspect existing task completion and verification evidence. Reuse completed prerequisites; complete any unfinished prerequisites required by the requested scope.
- A milestone is complete only when every task in its range is implemented and its checkpoint/independent tests pass. Mark verified tasks complete in this file and report completed task IDs, validation results and any remaining limitation.
- For a milestone-scoped request, finish at the requested milestone boundary. Continue into later milestones when the user requests them or requests the whole feature.

## Format and implementation boundaries

Every task uses `- [ ] Tnnn [P?] [USn?] Description with exact file paths`. `[P]` permits parallel work only within the dependency-ready batches listed below; it does not override prerequisites. Paths are relative to the repository root; new files are named explicitly.

Keep Go 1.26.0 and existing dependencies. Preserve snapshot schema 3, reads of schemas 1–3, database version 3, F5 IPC, automatic warning policy, presentation isolation, and existing command grammar. Do not add F7 administration, execution timeouts, lifecycle events, shell handoffs, or a second canonical state. The constitution remains an unratified template; the approved spec, plan, PRD and accepted ADRs govern these tasks.

## Milestone 1 (M1): Test Infrastructure (Setup)

**Purpose**: Extend the existing offline fixture/harness; no module initialization or dependency changes are needed.

- [X] T001 Create the native manual fixture in `tests/fixtures/plugins/manual-suite/plugin.json` and `tests/fixtures/plugins/manual-suite/main.go` with the four components, exact declarations, invocation/input logging, default two-file/four-item output, and empty/exit1/garbage/overflow/mixed/worktree/snapshot/symlink/hang/case-alias/null modes from `contracts/manual-context.md` §6; log and fail any Linker invocation and support deterministic process synchronization without sleeps.
- [X] T002 Integrate manual fixture preparation and isolated registry-state setup in `tests/fixtures/plugins/plugins.go` and `tests/integration/integration_test.go`, reusing per-entrypoint native builds and the registry writer for persisted `enabled:false`; support installing the same source under two aliases, portable paths, launch-log inspection, and stage inventory without an F7 command.

**Checkpoint**: The fixture can be installed locally under two aliases and failures can be synchronized without network access or public test-only CLI flags.

## Milestone 2 (M2): Foundations (Blocking Prerequisites)

**Purpose**: Supply pure canonical context, one retained enablement model, and shared CLI composition before exposing the stories.

- [X] T003 [P] Add canonical reader tables in `internal/workcontext/context_test.go` and `internal/workcontext/resolve_test.go` for nearest lifecycle-child ancestry, checkout/artifact/archive descendants, symlink-normalized directories, rejected snapshot symlinks, nested ordinary snapshot files, schemas 1–3/full validation, missing checkout, exact explicit IDs, duplicate IDs, unrelated malformed entries, matching invalid targets, inaccessible roots, explicit precedence, and old-workspace resolution.
- [X] T004 [P] Add read-only hint tests in `internal/projection/lookup_test.go` for missing/corrupt/stale/old-version/WAL indexes, pending uncheckpointed rows, malformed or mismatched paths, URI escaping on spaces/Unicode/Windows paths, bounded cancellation, and zero database/sidecar/lock creation or byte changes.
- [X] T005 [P] Extend retained-state tests in `internal/registry/registry_test.go` for absent/null/true/false package enablement, legacy ownerless components/conventions, multiple convention owners, deterministic effective filtering, and unchanged raw installed inventory.
- [X] T006 [P] Add codes 39–49 and their exact tokens from `contracts/manual-context.md` to `internal/diag/diag.go` and extend `internal/diag/diag_test.go`, preserving existing codes/tokens and distinguishing typed manual execution, refusal, persistence and recovery failures.
- [X] T007 Implement the transient validated context and pure `Current`/`ByID` readers in `internal/workcontext/context.go` and `internal/workcontext/resolve.go`: scan immediate configured lifecycle children deterministically before optional hints, fully validate canonical snapshots, stop on malformed enclosing data, reject duplicate identities, and never call Git, bootstrap, mutable projection, locks or presentation; depends on T003 and T006.
- [X] T008 Implement optional `LookupPathHint` in `internal/projection/lookup.go` using an existing-file, escaped SQLite URI with `mode=ro&immutable=1`, a 250 ms bound, immediate close and no pragmas/migrations/repair; ignore hint failures and validate absolute snapshot paths/IDs through the reader in `internal/workcontext/resolve.go`; depends on T004 and T007.
- [X] T009 Implement optional retained `Package.Enabled *bool`, `PackageEnabled` and raw/effective catalog separation in `internal/registry/registry.go`, with default-enabled legacy behavior and existing convention-name ownership semantics; depends on T005.
- [X] T010 Add reinstall/seed-retention tests in `internal/plugininstall/plugininstall_test.go` and `internal/bootstrap/bootstrap_test.go`, proving explicit disablement survives same-source reinstall/repair, disabled seed presence does not trigger reinstall, install collisions still use raw records, and manifests cannot supply retained enablement; extend `internal/plugin/manifest_test.go` for the last assertion.
- [X] T011 Preserve retained enablement in `internal/plugininstall/install.go` and `internal/bootstrap/bootstrap.go` before replacement/repair; use raw installed presence for integrity and collision checks and keep retained state out of config/install metadata; depends on T009 and T010.
- [X] T012 Add shared enablement regression cases in `internal/starter/starter_test.go`, `internal/locator/locator_test.go`, `internal/convention/convention_test.go`, `internal/repoconfig/repoconfig_test.go`, and `internal/extension/eligible_test.go`, covering disabled Starter/fallback/Locator/extensions/conventions, retained Locator policy references, ownerless defaults and installed inventory retention.
- [X] T013 Route activation and availability through effective state in `internal/starter/starter.go`, `internal/locator/locator.go`, `internal/convention/convention.go`, `internal/repoconfig/policy.go`, `internal/extension/eligible.go`, `internal/cli/start.go`, `internal/cli/convention_shared.go`, and `internal/cli/repository_shared.go`; retain raw inventory/integrity queries and never seed policy or change precedence merely because the effective catalog is empty; depends on T009, T011 and T012.
- [X] T014 Add shared nonbootstrapping composition in `internal/cli/context_shared.go` for current canonical context, required global location/config/registry reads, active-Work checks, expected real WorkDir/ID capture, selected-provider reload, `present.IO` and configured stream access; current-directory status must work with nonexistent WORK_HOME, and configuration/registry read failures must return typed errors without repair; depends on T007–T009 and T013.

**Checkpoint**: Canonical lookup works without writable index access; enabled defaults and raw/effective use are protected; no new public command is exposed yet. Run focused reader, projection, registry, bootstrap, install, activation and diagnostic tests before proceeding.

## Milestone 3 (M3): User Story 1 — Inspect Current or Named Work Without Changing It (Priority: P1) — MVP

**Goal**: Expose exact canonical human/JSON status for active and archived Works without application writes or extension execution.

**Independent Test**: Inspect one active and one archived Work from roots/descendants and by explicit ID in both formats, including empty links, missing checkout and legacy snapshots. Repeat each read 20 times with missing/corrupt/stale/WAL indexes; snapshots, operational rows, configuration, registry, directory inventories and shell handoff remain unchanged and no plugin/Git runs (Q1–Q3, Q11, Q14; SC-001).

### Tests

- [X] T015 [P] [US1] Add status contract cases to `tests/contract/manual_context_test.go` for five required JSON fields, exact link round-trip, `{}` empty links, stable sorted human lines, archived/active location, successful empty stderr, and no partial stdout on failure.
- [X] T016 [P] [US1] Add `tests/integration/status.txtar` for root/checkout/artifact descendants, explicit target overriding another current Work, archive descendants, contribution mode, spaces/Unicode paths, missing checkout, and accepted legacy snapshots without rewrite.
- [X] T017 [P] [US1] Add `tests/integration/status_purity_test.go` to compare bytes/mtime/inventories and read-only operational observations across 20 repeated human/JSON reads per target form, missing/corrupt/stale/WAL/old indexes, nonexistent WORK_HOME, and old-workspace hint success/failure; detect any Git/plugin execution, bootstrap, locks, sidecars or handoff writes without writable verification helpers.
- [X] T018 [P] [US1] Add output and failure unit cases in `internal/cli/status_test.go` for exact fields, sorted links, untruncated values, safe one-line quoting of newline/tab/control text, JSON preservation, invalid enclosing/explicit data, duplicate IDs and cancellation before output.

### Implementation

- [X] T019 [US1] Implement `work status [WORK] [--json]` in `internal/cli/status.go`, using pure resolution, a transient five-field view, complete serialization before output, bytewise link sorting and escaped human control text; emit one canonical result with empty links represented explicitly, without probes/progress/prompts/receipts or mutable helpers; depends on T015–T018.
- [X] T020 [US1] Register status in `internal/cli/root.go` under `groupInWork` with a visible local `--json` flag, preserve the hidden inherited root flag, and update `internal/cli/help_test.go` and `tests/integration/help_inventory.txtar` for status grammar/discoverability without changing bare-work branding or unrelated help.

**Checkpoint**: US1 is independently useful and is the suggested MVP. Its acceptance tests pass before starting mutations.

## Milestone 4 (M4): User Story 2 — Associate a Link With the Current Work (Priority: P2)

**Goal**: Associate exact non-empty text through an enabled manual Linker, directly or through one wizard, with no subprocess and atomic current-source publication.

**Independent Test**: Install a manual Linker whose discovery metadata is absent; associate interactively, replace directly with spaces/shell characters, and repeat through another provider of the same key. Verify exact canonical values, `link` provenance, unchanged unrelated/newer state and zero subprocesses. Cancellation/write failure leaves state/provenance unchanged (Q4–Q5, Q12; SC-002).

### Tests

- [ ] T021 [P] [US2] Extend `internal/extension/eligible_test.go` for shared typed input resolution, all five public Work facts, absent optional versus present-null metadata, sorted missing required declarations and malformed declarations while preserving automatic subscription/Starter policy.
- [ ] T022 [P] [US2] Add `internal/extension/manual_test.go` for sorted manual choices, declared labels/descriptions and visible qualified identities, manual Linkers ignoring discovery inputs/runtime/event filters, alternative providers of one key, private-key manifest ownership, qualified/bare matching, ambiguity before importer input filtering, and unavailable/disabled/wrong-role/nonmanual components.
- [ ] T023 [P] [US2] Add `internal/extension/publish_test.go` for exact upsert, full latest-state validation, expected identity/active path checks, lock/write failure, cancellation around commitment, best-effort provenance failure, and current source/time refresh even when publishing the same value.
- [ ] T024 [P] [US2] Extend `tests/contract/manual_context_test.go` and add `tests/integration/link.txtar` for stable value-free success stdout, exact whitespace/metacharacters/leading-dash arguments, replacement of Starter/discovery values, semantic-key alternatives, zero Linker launches even without an executable, unchanged access/metadata and provenance operation `link`.
- [ ] T025 [P] [US2] Add association journeys in `tests/integration/manual_context_pty_test.go` for zero/one/two explicit arguments, omitted steps only, recoverable empty input, redacted accepted receipts, selection/input cancellation, and no mutation or duplicate diagnostics on rejected/cancelled collection.
- [ ] T026 [P] [US2] Add deterministic publication races in `internal/extension/publish_race_test.go` for unrelated concurrent link updates, archive or destination disappearance before lock/commit, selected provider disablement/removal/key change, cancellation during lock waits and immediately before/after atomic commitment; verify no stale directory recreation or provider substitution.

### Implementation

- [ ] T027 [US2] Export/refactor `ResolveInputs` in `internal/extension/eligible.go` to return typed present inputs, sorted missing required declarations and parse errors, adapting automatic `Eligible` without changing F5 eligibility/order; depends on T021.
- [ ] T028 [US2] Implement `ManualCandidates` and `ResolveManual` in `internal/extension/manual.go`, carrying manual display text and qualified identity, using effective catalogs and manifest-owned Linker keys, separating direct name resolution from importer input checks, and keeping manual policy independent of subscriptions/Starter/discovery inputs; depends on T022 and T027.
- [ ] T029 [US2] Implement shared `PublishLink` in `internal/extension/publish.go` with latest-provider validation, cancellable per-ID lock acquisition and existing 3-second contention policy, latest full canonical reread, expected active ID/real-directory checks, exact one-key upsert and final cancellation check before atomic snapshot commitment; classify moved archives/missing/invalid destinations without recreating or redirecting them, and keep postcommit cancellation successful; depends on T023, T026 and T028.
- [ ] T030 [US2] Extract/reuse pure selected-snapshot row conversion in `internal/reconcile/reconcile.go`, then wire selected-row-only best-effort projection/provenance recording through `internal/cli/context_shared.go` and `internal/extension/publish.go`, using operation `link` and UTC RFC 3339 time after commitment under the Work lock; preserve unrelated provenance, avoid global reconcile/reset, and retain index failures for debug without failing a committed association; extend `internal/reconcile/reconcile_test.go` and `internal/projection/projection_test.go` for row/current-source behavior.
- [ ] T031 [US2] Adapt automatic discovery in `internal/extension/linker.go` and operation/result definitions in `internal/extension/extension.go` to shared publication with operation `discover`; retain warning-only failures, best-effort provenance, event order and reference-only behavior, updating behavioral coverage in `internal/extension/run_linker_test.go`.
- [ ] T032 [US2] Implement `work link [LINKER] [VALUE]` in `internal/cli/link.go` with one existing wizard, `StepResolved` for explicit arguments, zero-length-only validation, exact preserved text and `value supplied` receipts; collect before domain mutation, revalidate the selected provider, reject incomplete noninteractive forms and explicitly supplied JSON (including false), and emit only `work: linked <id> <key> <alias/name>` after commitment; depends on T024–T031.
- [ ] T033 [US2] Register link under `groupInWork` in `internal/cli/root.go` and extend `internal/cli/help_test.go` and `tests/integration/help_inventory.txtar` with its exact grammar and existing mutation flag visibility.

**Checkpoint**: US2 is testable with an installed manual Linker and active Work even before US3. Status immediately displays the saved value; discovery remains automatic-only and warning-compatible.

## Milestone 5 (M5): User Story 3 — Import Artifacts Into the Current Work on Demand (Priority: P2)

**Goal**: Invoke exactly one eligible manual Importer using current declared inputs, then incorporate all safe artifacts or none with coordinated cancellation, cleanup and archive behavior.

**Independent Test**: Import the fixture's two nested files using a saved link (count 4), then test empty output, repeated/mixed collision, unsafe content, runtime/process/response failure, cancellation during process/copy/lock wait and recoverable incorporation failure. Success preserves artifacts through archive; every recoverable failure contributes zero artifacts and leaves no stage or snapshot/access changes (Q6–Q9, Q11–Q12; SC-003–SC-004).

### Tests

- [ ] T034 [P] [US3] Extend native destination/preflight tables in `internal/staging/staging_test.go` and `internal/staging/refusal_unix_test.go` for complete mixed collisions, snapshot/checkout reservations, traversal/escaping ancestors, source/destination symlinks, unsupported types, safe directory merges, existing-directory aliases, case-sensitive valid distinct names, native file/directory aliases and exclusive-create races.
- [ ] T035 [P] [US3] Add `internal/staging/incorporate_test.go` for cancellable bounded copying, retained undo, rollback of only newly created entries, preexisting directory preservation, `WORK_FAIL_AT=incorporate:<n>`, idempotent commit/rollback, rollback-after-commit misuse, removal errors retaining unresolved paths and retry, and compatibility-wrapper counts/behavior.
- [ ] T036 [P] [US3] Add `internal/extension/import_one_test.go` for unique stages, launch/exit/response/refusal/persistence/cleanup categories, empty output, only-selected invocation, stage removal before commit, joined cleanup/rollback causes, recovery-error precedence over cancellation, provider revalidation and no snapshot/index/access mutations.
- [ ] T037 [P] [US3] Extend `tests/contract/manual_context_test.go` for exact imported result/count, failure/cancel exits 20 and 44–49, no success stdout on failure, Work/component/operation diagnostics, no values/content/stage paths in normal output, and unchanged automatic warning severity/tokens.
- [ ] T038 [P] [US3] Add `tests/contract/manual_import_protocol_test.go` using native launch logs to verify F5 runtime/response/stdout-cap/inherited-pipe behavior, only declared present typed inputs, optional null versus omission, current saved link consumption, no automatic Starter/event restrictions or discovery, no imposed execution timeout and exactly one selected importer process.
- [ ] T039 [P] [US3] Add `tests/integration/import.txtar` for four-item nested success, empty count 0, existing-directory merges, repeat/mixed collisions, reserved/unsafe output and fixture exit/garbage/overflow/incorporation failures; compare preexisting artifacts/state and verify every normally terminated attempt removes its stage.
- [ ] T040 [P] [US3] Add deterministic commitment races in `internal/extension/import_race_test.go` for archive winning during process execution, archive waiting during apply/rollback, changed/disabled selected provider after execution, native destination changes, cancellation during copy/lock wait and after apply/before commit, late cancellation after success, and independent cleanup/rollback failure injection; run Windows holder processes outside the checkout.
- [ ] T041 [P] [US3] Add `tests/integration/manual_import_pty_test.go` for omitted versus explicit selection, manual labels/qualified identity, eligible-only choices, accepted component receipt and configured-channel progress, plus synchronized interruption of a hanging importer with partial staged output and complete cleanup.
- [ ] T042 [P] [US3] Add `tests/integration/import_archive.txtar` proving a successful import survives normal archive alongside its snapshot and previously imported artifacts remain unchanged after a later failed import.

### Implementation

- [ ] T043 [US3] Harden complete read-only native preflight in `internal/staging/plan.go`, preserving F5 safety/refusal reasons and case-sensitive valid names, and record/check destination directory identities so aliases or changed ancestors cannot silently merge distinct staged paths; retain exclusive-creation backstops for remaining aliases/races; depends on T034.
- [ ] T044 [US3] Implement `Plan.ApplyContext`, retained `Applied` undo ownership and idempotent terminal `Commit`/`Rollback` in `internal/staging/incorporate.go`; check context before mutations and during bounded copies, preserve all removal errors/unresolved paths for retry, roll back independently of cancelled contexts, and keep `Incorporate` as the compatible apply/commit wrapper; depends on T035 and T043.
- [ ] T045 [US3] Implement shared `ImportOne` in `internal/extension/import_one.go`: reread active canonical state/provider and resolve inputs immediately before one F5 IPC invocation outside the Work lock, then acquire the cancellable shared lock, reread expected active identity/path/provider, build/apply, remove stage, check cancellation and commit before releasing; classify moved archives/missing/invalid destinations, preserve captured inputs and original causes, clean every outcome independently of cancellation and return typed recovery failures without falsely claiming rollback; depends on T036–T040 and T044.
- [ ] T046 [US3] Adapt automatic imports in `internal/extension/importer.go` and shared result definitions in `internal/extension/extension.go` to `ImportOne`, mapping new cleanup failures to existing `extension-persist-failed` warnings and preserving Linker-before-Importer order, counts, continuation and reference-only behavior; update `internal/extension/run_importer_test.go` for these observable guarantees.
- [ ] T047 [US3] Implement `work import [IMPORTER]` in `internal/cli/import.go` with eligible-only omitted selection, direct identity resolution before input checks, latest provider/input validation, configured-channel progress, exactly one `ImportOne` call and stable `work: imported <id> <alias/name> <count>` success; reject missing noninteractive arguments/explicit JSON, report manual failures through the sole diagnostic border and preserve committed success on a late interrupt; depends on T041–T042 and T045–T046.
- [ ] T048 [US3] Register import under `groupInWork` in `internal/cli/root.go` and extend `internal/cli/help_test.go` and `tests/integration/help_inventory.txtar` with its exact grammar and mutation flag behavior.

**Checkpoint**: US3 is independently testable with a preexisting saved link, without running link. Shared staging/import changes preserve the F5 automatic pipeline and archive integrity.

## Milestone 6 (M6): User Story 4 — Resolve Missing Context and Unavailable Operations Clearly (Priority: P2)

**Goal**: Finish actionable boundary guidance and uniform interaction/diagnostic behavior across the three usable commands. Core safety checks already belong to the earlier stories; this milestone verifies their complete public failure surface and polishes collection/output.

**Independent Test**: Run all commands outside a Work; request unknown/disabled/wrong-role/nonmanual/ambiguous providers and missing importer inputs; omit automation arguments and try archived enrichment. Each fails once with the documented token, actionable guidance, zero processes/mutations and no empty selector. Verify keyboard journeys at supported sizes, monochrome output, redacted receipts and debug detail (Q7, Q10–Q14; SC-003, SC-005–SC-006).

### Tests

- [ ] T049 [P] [US4] Add `tests/integration/manual_context_errors.txtar` covering complete-argument missing-context failures, unknown explicit Work, invalid target data, archived mutations, unknown/disabled/wrong-role/nonmanual providers, two-alias ambiguity even when only one importer has inputs, qualified success, sorted missing declarations without values, no eligible providers, missing/empty/extra automation arguments, unsupported flags and mutation `--json`/`--json=false`; assert exact codes, one diagnostic, empty failure stdout and zero execution/mutation.
- [ ] T050 [P] [US4] Add `internal/cli/context_shared_test.go` for registry/provider/input changes between collection and invocation/commit, no silent provider substitution, actionable distinctions between no installed/enabled manual provider and missing required data, and no candidate collection before grammar, required noninteractive arguments and current context are validated.
- [ ] T051 [P] [US4] Extend `tests/integration/manual_context_pty_test.go` with shared selector/input geometry at 80×24 and existing narrow/minimum sizes, resize/navigation/filter behavior, colliding display names with visible identities, empty-selector avoidance, compact accepted receipts and terminal-history exclusion of rejected values/obsolete errors.
- [ ] T052 [P] [US4] Add `tests/integration/manual_context_streams_test.go` for redirected streams, nonempty NO_COLOR, TERM=dumb and normal terminal modes across human/JSON status, direct mutations and failures; assert result-only stdout, configured UI diagnostics/progress/receipts, no styling/control leakage or sensitive values/content in mutation output, safe human status controls and exact JSON round-trip.
- [ ] T053 [P] [US4] Extend `internal/cli/diagnostics_border_test.go` and `internal/cli/diagnostics_test.go` for manual Work/component/operation context, stable code/token inventory, one final render, bounded retained stderr/original causes under WORK_DEBUG in interactive and noninteractive modes, cleanup recovery precedence and unchanged automatic warning rendering.

### Implementation

- [ ] T054 [US4] Complete context/target guidance in `internal/cli/context_shared.go` and `internal/cli/status.go`, distinguishing enter-a-Work/explicit-status recovery, archived enrichment, unreadable target/global data and unconfigured-old-root lookup limitations without fallback or repair; depends on T049–T050.
- [ ] T055 [US4] Complete provider/input/automation guidance in `internal/cli/context_shared.go`, `internal/cli/link.go`, and `internal/cli/import.go`: qualified ambiguity alternatives, sorted missing input declarations, install/enable versus supply-input next steps, no empty selector, no confirmation shortcut and no prompt for supplied empty/invalid direct arguments; depends on T049–T050 and T054.
- [ ] T056 [US4] Extend the sole final diagnostic renderer in `internal/cli/diagnostics_border.go` to expose retained manual execution stderr/causes only under WORK_DEBUG, include Work/component/operation normally, preserve joined cleanup/rollback recovery detail and render exactly once on stderr/UI with existing automatic warnings unchanged; depends on T053.
- [ ] T057 [US4] Complete manual option rendering, redacted receipts, viewport behavior and configured-channel/style handling in `internal/cli/context_shared.go`, `internal/cli/link.go`, and `internal/cli/import.go` using existing presentation primitives; prioritize visible qualified identities over truncatable secondary text, keep mutation outside callbacks, and satisfy geometry/stream cases without passing domain DTOs into `internal/present`; depends on T051–T052 and T055.

**Checkpoint**: All four stories satisfy their independent criteria, including failure cases and supported interactive modes. No additional navigation hub or presentation model is introduced.

## Milestone 7 (M7): Final Validation (Polish & Cross-Cutting Concerns)

**Purpose**: Demonstrate F6, verify previous slices, and document the delivered commands.

- [ ] T058 [P] Add `tests/integration/f6_demo_test.go` for a fresh offline status → link → import → status journey, all three complete-argument missing-context failures and explicit outside status, with no manual snapshot editing in the product journey; assert correct values, exactly one import launch and artifact preservation.
- [ ] T059 [P] Update `README.md` with the three public grammars, direct/interactive usage, status-only JSON, exact-value association, manual eligibility and failure recovery; reconcile runnable examples and implementation acceptance evidence in `specs/007-on-demand-context/quickstart.md` with the delivered behavior.
- [ ] T060 Run host validation using `Makefile` targets `make seed`, `make lint`, `GIT_CONFIG_GLOBAL=/dev/null go test ./...` and `make build`; confirm no seed-dependent skips, retain the boundary gate in `tests/contract/present_boundary_test.go`, and resolve failures within the touched implementation/test files before recording actual results in `specs/007-on-demand-context/quickstart.md`.
- [ ] T061 Verify the existing Ubuntu/macOS/Windows matrix in `.github/workflows/ci.yml` with `make seed-all`, `make lint`, `go test ./...` and `make build`, including portable direct CLI/filesystem/IPC cases and Unix-only PTY cases; run the existing F1–F5 automated demonstrations and contracts, especially `tests/integration/f1_regression_test.go`, `tests/integration/f5_demo_test.go`, `tests/integration/extensions_reference_only.txtar` and `tests/contract/extension_protocol_test.go`, and record actual results or unavailable-platform limitations in `specs/007-on-demand-context/quickstart.md`.
- [ ] T062 Audit FR-001–FR-031, SC-001–SC-007 and Q1–Q15 coverage against the implemented tests/contracts, confirm no snapshot/database migration, dependency addition, F7 API or shell-handoff side effects, and update completed acceptance evidence and any real remaining limitation in `specs/007-on-demand-context/tasks.md` and `specs/007-on-demand-context/quickstart.md`.

## Dependencies & Execution Order

### Milestone and story graph

```text
M1: fixture (T001) → harness (T002)
    ↓
M2: pure context + hints + retained enablement + diagnostics/composition
    ↓
M3 / US1: status (MVP)
    ↓ recommended priority/delivery order
M4 / US2: manual evaluation + shared publication + link
    ↓ shared manual evaluation needed by import
M5 / US3: deferred staging commitment + shared importer + import
    ↓
M6 / US4: complete boundary guidance + diagnostic/terminal consistency
    ↓
M7: F6 demonstration + documentation + full regression/matrix
```

- **M1 / Setup** has no prerequisite. T002 consumes T001; do not parallelize them.
- **M2 / Foundations** block all story commands. After setup, T003–T006 are disjoint. T007 → T008; T009 → T010 → T011 → T012 → T013; T014 joins the reader/catalog/activation work. The reader/hint and retained-state tracks can progress concurrently once their own prerequisites are met.
- **M3 / US1** needs only foundations. Finish T015–T018 before T019, then T020. Stop here for MVP validation.
- **M4 / US2** needs foundations; it can be tested without US1 using canonical state/provenance. The recommended sequence delivers status first. Finish T021–T026, then T027 → T028 → T029 → T030 → T031 → T032 → T033. T024–T026 use the setup fixture/harness.
- **M5 / US3** consumes T027–T028 manual/input helpers and shared expected-context/lock conventions from US2; it does not require the link command for validation because its inputs may already be saved. Finish T034–T042 before T043 → T044 → T045 → T046 → T047 → T048.
- **M6 / US4** consumes all three command implementations. T049–T053 can be authored concurrently; T054 → T055 → T057 share CLI files and remain sequential, while T056 can run alongside that implementation chain after T053.
- **M7 / Final validation** follows all stories. T058 and T059 are disjoint and can run together. Complete both before T060 → T061 → T062; seed generation/build gates share output directories and must not run concurrently.

### Within a story

Write the listed behavioral tests first and observe expected missing/new-behavior failures. Implement pure data/evaluation helpers before mutation services, then CLI integration. Run the focused package/contract/integration tests for the milestone before its checkpoint. Use deterministic barriers/injected dependencies for races and cleanup faults, not timing-only sleeps. Failed rollback is a recovery outcome, never evidence of unchanged artifacts.

## Parallel Execution Examples

These describe optional implementation scheduling, not a requirement to spawn agents. Every batch assumes setup/foundations and the prerequisites above are complete. `[P]` tasks in one batch have disjoint edited files; new tests may initially fail until the later implementation tasks exist.

### User Story 1

```text
Together: T015 status contract tests; T016 status testscript;
          T017 repeated-read purity tests; T018 status output unit tests.
Then:     T019 status implementation → T020 registration/help.
```

### User Story 2

```text
Together: T021 typed input tests; T022 manual eligibility/name tests;
          T023 publication tests; T024 CLI contract/testscript;
          T025 association PTY tests; T026 publication race tests.
Then:     T027–T033 in dependency order (shared extension/CLI files).
```

### User Story 3

```text
Together: T034 preflight tests; T035 retained-undo/copy tests;
          T036 single-import tests; T037 result/error contract tests;
          T038 IPC parity tests; T039 import testscript;
          T040 commitment races; T041 import PTY; T042 archive preservation.
Then:     T043–T048 in dependency order.
```

### User Story 4

```text
Together: T049 negative CLI matrix; T050 composition/revalidation tests;
          T051 terminal geometry; T052 stream/color tests; T053 diagnostic tests.
Then:     T054 → T055 → T057, alongside T056 after T053.
```

Do not parallelize story milestones that edit `tests/contract/manual_context_test.go`, `internal/cli/root.go`, help inventory, `internal/extension/eligible.go`, shared CLI composition or wizard files. Cross-story parallelism requires isolating those file owners and satisfying the explicit helper dependencies first.

## Requirement and Validation Traceability

| Requirement / outcome | Primary implementation tasks | Acceptance evidence |
| --- | --- | --- |
| FR-001–FR-006; SC-001 | T007–T008, T014, T019–T020, T054 | T003–T004, T015–T018; Q1–Q3, Q11 |
| FR-007–FR-012; SC-003 | T009–T014, T027–T029, T045, T055 | T005, T010, T012, T021–T022, T026, T038, T040, T049–T050; Q7, Q10–Q12 |
| FR-013–FR-017; SC-002 | T029–T033 | T023–T026, T030, T038; Q4–Q6, Q12 |
| FR-018–FR-025; SC-004 | T027–T028, T043–T048 | T034–T042; Q6–Q9, Q11–Q12 |
| FR-026–FR-030; SC-005–SC-006 | T019–T020, T032–T033, T047–T048, T054–T057 | T018, T025, T037, T041, T049–T053; Q5, Q10, Q13–Q14 |
| FR-031; SC-007 | T011, T013, T031, T046, T058–T062 | Existing F1–F5 suites, T010, T012, T058, T060–T061; Q15 |

The four interface contracts map to story-local tests: status → T015; link → T023–T025; import → T034–T042; shared manual context → T005–T006, T010, T012, T021–T022, T026, T040, T049–T053. Resolved context and retained package state are foundational; manual decisions/typed inputs and current provenance first belong to US2; deferred import/undo state belongs to US3. No new persisted entity requires a migration.

## Implementation Strategy

### MVP first

Complete M1–M3 (T001–T020), then demonstrate canonical human/JSON inspection and prove repeated-read purity independently. This is the smallest public feature increment. Keep US4's applicable status precondition checks in the reader/status implementation from the outset; M6 completes shared guidance and terminal coverage.

### Incremental delivery

Complete M4 (US2) and demonstrate exact association/replacement with zero plugin launches. Complete M5 (US3) over the same manual evaluation and shared safety primitives, then demonstrate saved-link consumption and all-or-nothing artifact commitment. Complete M6 (US4) for the diagnostic/automation/terminal matrix, then M7 for the F6 demonstration, documentation and supported-platform regression gate.

Keep implementation on the current feature branch unless the user selects another delivery cadence. This task-generation artifact does not request commits, PRs, releases or deployments. During implementation, mark a task complete only after its described behavior and relevant verification pass; record unsupported validation environments honestly instead of claiming a matrix result.

## Task Inventory

| Milestone | Tasks | Count |
| --- | --- | ---: |
| M1 — Test infrastructure | T001–T002 | 2 |
| M2 — Foundations | T003–T014 | 12 |
| M3 — Status / US1 (P1 / MVP) | T015–T020 | 6 |
| M4 — Manual link / US2 (P2) | T021–T033 | 13 |
| M5 — Manual import / US3 (P2) | T034–T048 | 15 |
| M6 — Errors and interaction / US4 (P2) | T049–T057 | 9 |
| M7 — Final validation | T058–T062 | 5 |
| **Total** | **T001–T062** | **62** |

Parallel-ready tasks: 30, grouped into foundation, story test and documentation/demo batches above. Milestone checkpoints are validation criteria, not additional unnumbered checklist tasks.

### M1–M2 host verification (2026-10-11)

M1: fixture unit tests and `TestScripts/manual_fixture` passed. M2: focused workcontext, projection, registry, diagnostics, install, bootstrap, plugin, starter, locator, convention, repoconfig, extension and CLI tests passed with `GOCACHE=/tmp/work-go-cache GIT_CONFIG_GLOBAL=/dev/null`. Bootstrap already retains package records during repair; the new tests verify this without changing its install pipeline. Other platforms remain for the existing CI matrix.

### M3 host verification (2026-10-11)

T015–T020 passed: five-field JSON and exact link round-trip, sorted safe human records, empty links, active/archive locations, explicit precedence, invalid/duplicate targets, cancellation, grouped help and the daily-cycle testscript. `TestStatusPurity` compares bytes, mtimes and directory inventories across 20 reads of each human/JSON current-root, artifact-descendant and explicit target form for active and archived Works. It covers missing/corrupt/stale/old/WAL indexes, nonexistent home and old-workspace hint success/failure, with native launch tripwires and an unchanged shell handoff. Legacy schemas and missing checkouts are accepted without rewrites.

Host gates passed: `make seed`, `GIT_CONFIG_GLOBAL=/dev/null go test ./...`, `make lint`, and `make build` (sandbox caches under `/tmp`). After extending archived purity and failure cases, their focused tests and lint passed again. No M4–M7 implementation was attempted; macOS/Windows execution remains for CI. No before/after implementation hooks are registered in `.specify/extensions.yml`.
