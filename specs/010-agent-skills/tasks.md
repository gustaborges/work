# Tasks: Agent Skills for Work Plugin Authors

**Input**: [plan.md](plan.md), [spec.md](spec.md), research/data model/contracts/quickstart and governance drafts.
**Status**: S0 approved and reconciled; implementation in progress.
**Tests**: Explicitly required by FR-021 and proposal acceptance matrix. Add meaningful failing test coverage before the corresponding implementation; do not add mirror-only tests.
**Format**: `- [ ] Tnnn [P?] [USn?] Action with exact path`. [P] means independent files after the stated phase prerequisites, not unrestricted concurrent edits.

## Phase 1: Setup — S0 Authorities and Dependency

Establish approved authorities before any application code. If merge/approval is absent, stop at this gate; planning has not authorized a merge or ADR acceptance.

- [X] T001 Verify feature 009 is merged into the repository’s development baseline (`develop`); inspect merged command/parser/identity/diagnostic contracts and record revisions/differences in specs/010-agent-skills/research.md; stop if unavailable (FR-022).
- [X] T002 Promote reviewed AS-R1–AS-R5 from specs/010-agent-skills/governance/prd-amendment.md into docs/prd.md using available requirement IDs, then update feature traces (FR-001–FR-006, FR-020, FR-022).
- [X] T003 Promote the three proposed decisions from specs/010-agent-skills/governance/ into docs/adr/ with available IDs after approval; explicitly resolve/record the narrow ADR-0019 naming exception without silently rewriting its accepted decision (FR-022).
- [X] T004 Promote the approved conditional design into docs/add/add-0001-work-system-architecture.md, citing accepted ADR/PRD IDs and preserving separate filesystem-local skill state (FR-016, FR-022).
- [X] T005 Reconcile specs/010-agent-skills/spec.md, plan.md, research.md, data-model.md, contracts/, quickstart.md and tasks.md against merged 009/approved authorities; freeze naming, diagnostic allocations and safe output formats and record S0 evidence in specs/010-agent-skills/validation.md (FR-015, FR-022).

**Checkpoint**: S0 approved and reconciled; only then continue.

## Phase 2: Foundational — Shared Safe Content and Target Model

S0 blocks this entire phase. Establish the reusable digest, descriptors and safe observations before any writable CLI surface.

- [X] T006 [P] Add portable digest golden/framing/mode/hidden/empty-directory/unsafe-node tests in ai-skills/digest_test.go, using one contract for embedded/staged/installed content (FR-017).
- [ ] T007 [P] Add anchor/relative-override/canonical-alias/case/overlap/linked-parent tests in internal/skillinstall/targets_test.go with test-owned homes and platform capability handling (FR-002–FR-003, FR-009).
- [ ] T008 [P] Add exact marker/foreign/unreadable/record/observation classification tests in internal/skillinstall/inspect_test.go, including marker bytes/full old-tree identity and I/O-versus-absence distinctions (FR-008, FR-011, FR-013).
- [X] T009 Implement shared work-skill-tree-v1 framing in ai-skills/digest.go and explicit safe asset inventory in ai-skills/embed.go, including hidden assets and rejecting unsafe/nested-module assets (FR-001, FR-017).
- [ ] T010 Define immutable Destination/Observation/Marker/Plan/Outcome/Record types and validated schema fields in internal/skillinstall/types.go per data-model.md; preserve complete old content and marker evidence (FR-008–FR-014).
- [ ] T011 Implement canonical anchor/target discovery and deterministic selection/deduplication/overlap protection in internal/skillinstall/targets.go, with no filesystem creation during planning (FR-002–FR-003, FR-009–FR-010).
- [ ] T012 Implement pure marker/tree/record inspection and reusable classification in internal/skillinstall/inspect.go; expose injected read/filesystem seams without Work-home dependencies (FR-007–FR-009, FR-013, FR-016).
- [ ] T013 Extract shared validator-derived immutable sorted descriptors in internal/plugin/contract.go, internal/marketplace/contract.go, internal/pluginid/contract.go and internal/semconv/contract.go; refactor inline key lists without changing parser behavior and include role anyOf/qualified ownership (FR-005, FR-018).
- [ ] T014 Derive actual request/response/constants descriptors in internal/ipc/contract.go and add defensive-copy/determinism/field-discovery tests in internal/ipc/contract_test.go and domain contract_test.go files (FR-018).
- [X] T015 Implement deterministic Markdown rendering in ai-skills/internal/generate/main.go and go:generate wiring in ai-skills/generate.go from shared descriptors, ready for US1 initial references (FR-018).

**Checkpoint**: Validate this phase’s independent goal and relevant quickstart evidence before marking its tasks complete.

## Phase 3: User Story 1 — Equip an Agent (P1, S1)

Provide a complete independently inspectable/fixture-testable authoring payload. Fresh CLI installation integrates after US2; no partial unsafe installer is released.

- [X] T016 [P] [US1] Add frontmatter/vendor-neutrality/link/inventory tests in ai-skills/skills_test.go, including four-role workflow and explicit generated-reference links (FR-001, FR-004–FR-005).
- [ ] T017 [P] [US1] Add every-entry validation recipe tests in tests/integration/skills_validation_test.go and owned multi-entry fixtures in tests/fixtures/skills/, covering fallback alternatives, later runtime/entrypoint failure, fresh retry, cleanup/cancellation/offline/environment purity and zero invocation (FR-006, FR-021).
- [X] T018 [US1] Write canonical workflow/trigger/guardrails in ai-skills/work-cli-plugin-builder/SKILL.md using only name/description frontmatter and vendor-neutral body (FR-004–FR-006).
- [X] T019 [US1] Write marketplace-catalog.md, plugin-manifest.md and component-roles.md under ai-skills/work-cli-plugin-builder/references/, accurately separating parser strictness and covering conventions, private qualified keys, all role protocols and runtime prerequisites (FR-005).
- [X] T020 [US1] Create complete catalog/plugin/four-role/convention scaffolds under ai-skills/work-cli-plugin-builder/templates/ with portable text entrypoints and declared runtimes; mark reference examples with actual fixture kinds (FR-005, FR-017, FR-019).
- [ ] T021 [US1] Write complete safe enumeration/per-entry POSIX and PowerShell scoped-environment recipes in ai-skills/work-cli-plugin-builder/references/validate-and-publish.md; preserve failure on cleanup errors and distinguish discovery/install/runtime behavior (FR-006).
- [X] T022 [US1] Render and commit initial catalog-schema.md, manifest-schema.md and role-matrix.md under ai-skills/work-cli-plugin-builder/references/generated/ using go generate ./ai-skills/...; never hand-copy contract tables (FR-005, FR-018).
- [X] T023 [US1] Finish Names/Tree/Digest embedding API in ai-skills/embed.go and verify full payload parity, normal-build availability and rejected unknown skill names through ai-skills/skills_test.go (FR-001, FR-003–FR-004, FR-017).
- [X] T024 [US1] Implement baseline full scaffold discovery/every-entry selected-install checks in ai-skills/fixtures_test.go using BuildOffer and merged pluginadmin/plugininstall APIs with isolated homes and zero component execution (FR-006, FR-019).

**Checkpoint**: Validate this phase’s independent goal and relevant quickstart evidence before marking its tasks complete.

## Phase 4: User Story 2 — Refresh Safely (P1, S2a)

Expose install only after containment, complete preflight and recoverable replacement work. Prove exact old/new content and accurate partial outcomes.

- [X] T025 [P] [US2] Add whole-batch ownership/force/no-op/observed-change tests in internal/skillinstall/plan_test.go, including initial conflict leaving all payloads untouched and no-op mtimes (FR-007–FR-010).
- [ ] T026 [P] [US2] Add forward/rollback/cleanup checkpoint and malformed/mismatched/path-injecting record tests in internal/skillinstall/recovery_test.go for initial install/update and retained exact old content (FR-011–FR-012, FR-021).
- [ ] T027 [P] [US2] Add deterministic overlapping-installer/ordered-lock/parent-marker-edit/status-interleave/later-target-failure tests in tests/integration/skills_concurrency_test.go with helper-process barriers (FR-009–FR-012, FR-021).
- [ ] T028 [P] [US2] Add CLI install grammar/output/diagnostic/help/purity contracts in tests/contract/skills_test.go, including inherited --json rejection before writes and stderr-only notes/hints (FR-002–FR-004, FR-015–FR-016).
- [X] T029 [US2] Implement complete initial and locked preflight/force planning in internal/skillinstall/plan.go using immutable observations; refuse newly changed targets even with force (FR-007–FR-010).
- [ ] T030 [US2] Implement containment-checked persistent sibling lock acquisition in canonical order in internal/skillinstall/apply.go using lockfile.AcquireContext; preserve lock files and cancellation/timeout behavior (FR-009–FR-010, FR-016).
- [ ] T031 [US2] Implement verified sibling staging/marker writes/prepared record/retained backup/new-tree move/commit publication and injected fault seams in internal/skillinstall/apply.go using supported atomic-file/sync helpers (FR-011, FR-017).
- [X] T032 [US2] Implement evidence-checked prepared rollback/committed forward cleanup and interrupted compensation in internal/skillinstall/recovery.go; validate owned sibling paths and preserve mismatches/unrecorded artifacts (FR-011).
- [X] T033 [US2] Implement per-target commit/cleanup-pending/failed/not-attempted aggregation and immediate observation revalidation in internal/skillinstall/apply.go; stop later processing after failure and retain earlier commits (FR-010–FR-012).
- [ ] T034 [US2] Allocate S0-reconciled skill conflict/busy/recovery/cleanup/I/O categories in internal/diag/diag.go and safe CLI border/hints in internal/cli/diagnostics_border.go with focused tests (FR-015).
- [X] T035 [US2] Wire conditional skills install command and bare-group help in internal/cli/skills.go and internal/cli/root.go without Work preflight/bootstrap/discovery; render escaped receipts, selected-agent invocation/reload hints and partial outcomes (FR-001–FR-004, FR-015–FR-016).
- [ ] T036 [US2] Prove full installation interruption/restart matrix in tests/integration/skills_recovery_test.go and ownership/offline/vendor parity/no-op/Work-home byte-existence matrix in tests/integration/skills_install_test.go on native platforms (FR-007–FR-012, FR-016–FR-017, FR-021).

**Checkpoint**: Validate this phase’s independent goal and relevant quickstart evidence before marking its tasks complete.

## Phase 5: User Story 3 — Pure Status (P2, S2b)

Inspect every state coherently without creating locks, directories, recovery artifacts or application state. Service tests can run independently of install by materializing fixture trees.

- [ ] T037 [P] [US3] Add pure coherent-inspection/state/null-digest/error-token tests in internal/skillinstall/status_test.go, including three-attempt limit and record/target changes (FR-013–FR-014).
- [ ] T038 [P] [US3] Add status JSON/human/exit-aggregation/stderr/absent-parent/read-only matrix in tests/integration/skills_status_test.go, with Work-home and coordination byte/existence comparisons (FR-013–FR-016, FR-021).
- [X] T039 [US3] Implement bounded before/after identity/record coherent inspection in internal/skillinstall/inspect.go, returning recovery-required on pending/churning evidence and unreadable on real I/O failures (FR-013–FR-014).
- [X] T040 [US3] Wire skills status in internal/cli/skills.go with ordered non-null JSON rows, nullable digest evidence, safe error tokens, stderr notes and deterministic aggregate operational exits (FR-003, FR-013–FR-016).
- [ ] T041 [US3] Extend tests/contract/skills_test.go and tests/integration/skills_status_test.go for all malformed/future/foreign/unsafe/recovery cases, real install checkpoint interleavings and zero bootstrap/lock/repair writes (FR-008–FR-009, FR-013–FR-016, FR-021).

**Checkpoint**: Validate this phase’s independent goal and relevant quickstart evidence before marking its tasks complete.

## Phase 6: User Story 4 — Contract Alignment (P2, S3)

Complete fixture routing/drift enforcement and maintainer obligations without claiming descriptors prove prose correctness.

- [X] T042 [P] [US4] Add TestSkillContractPinned and in-memory generated-byte comparison in ai-skills/contract_test.go, asserting verification never writes generated files or CONTRACT (FR-018).
- [ ] T043 [P] [US4] Add independent role/catalog/IPC descriptor perturbation tests and deep-copy/sorted-output checks in ai-skills/drift_test.go proving each intended gate fails (FR-018, FR-021).
- [X] T044 [US4] Implement canonical versioned descriptor serialization/pin checking in ai-skills/contract.go and local pin writer in ai-skills/internal/skilllock/main.go with actionable stale-contract errors (FR-018).
- [ ] T045 [US4] Complete explicit valid/invalid/catalog/manifest/partial/IPC fixture routing in ai-skills/fixtures_test.go, failing unclassified full JSON examples and checking stable failure reasons (FR-019).
- [ ] T046 [US4] Extend complete-tree fixtures in tests/fixtures/skills/ and ai-skills/fixtures_test.go for names/containment, missing runtimes/entrypoints, semantic keys and role-specific actual IPC behavior; keep management validation passive (FR-005–FR-006, FR-019).
- [X] T047 [US4] Add local make skill-lock and non-mutating verification wiring to Makefile and .github/workflows/ci.yml; retain existing OS/build gates, never auto-repin/regenerate in CI (FR-018, FR-020).
- [X] T048 [US4] Add same-PR marketplace/plugin/protocol maintenance instructions outside managed markers in AGENTS.md, have CLAUDE.md import AGENTS, and add the review question to .github/pull_request_template.md (FR-018, FR-020).
- [X] T049 [US4] Review guidance/templates against all actual represented and unrepresented contracts, regenerate references then explicitly create the reviewed ai-skills/work-cli-plugin-builder/CONTRACT pin; record review boundaries in docs/agent-skills.md (FR-005, FR-018–FR-020).

**Checkpoint**: Validate this phase’s independent goal and relevant quickstart evidence before marking its tasks complete.

## Phase 7: Polish and Cross-Cutting Delivery — S4

All stories and S0 must be complete for public delivery. Record actual evidence; unavailable agents/OS prerequisites remain incomplete, never simulated.

- [X] T050 [P] Write docs/agent-skills.md catalog/targets/ownership/update/status/recovery/local-only-coordination documentation, explaining explicit refresh after binary upgrade and safe deferred removal (FR-020).
- [X] T051 [P] Add marketplace-authoring link in docs/marketplaces.md and real setup command table entries in README.md; verify reload/invocation guidance against current vendor docs (FR-004, FR-020).
- [ ] T052 Verify setup/root/bare-group help and static-home behavior through internal/cli/help_test.go and tests/contract/skills_test.go, checking no aliases or human text contaminating machine stdout (FR-015, FR-020).
- [ ] T053 Run Q1–Q6 portability/fault/race/purity/recipe matrix from specs/010-agent-skills/quickstart.md on Linux/macOS/Windows and record native evidence/capability skips in specs/010-agent-skills/validation.md (FR-021; SC-001–SC-005, SC-007).
- [ ] T054 Perform isolated real Claude Code and Codex Q7 authoring/discovery/reload exercises, including Claude configuration override; record agent versions and every-entry registration/install with zero manual contract fixes in specs/010-agent-skills/validation.md (FR-004, FR-021; SC-006).
- [ ] T055 Run make seed-all, make lint, go test ./... and make build plus existing release-platform embed checks; confirm clean non-mutating pin/generated verification and record Q8 results in specs/010-agent-skills/validation.md (FR-001, FR-018, FR-021; SC-007).
- [ ] T056 Review all S0 authority/diagnostic traces and acceptance outcomes, mark only evidenced tasks complete in specs/010-agent-skills/tasks.md and document any unavailable-agent/delivery gaps in specs/010-agent-skills/validation.md (FR-020–FR-022).

**Checkpoint**: Validate this phase’s independent goal and relevant quickstart evidence before marking its tasks complete.

## Dependencies and Execution Order

```text
S0 → Foundations → US1 payload → US2 safe install → US3 status integration → S4 delivery
                 └────────────────────────────→ US4 drift/review gates ────┘
                 └→ US3 service tests may run with fixture trees before US2
```

All application tasks are blocked by S0. US1 payload/scaffold checks are independently testable after foundations. Full install-facing US1 acceptance depends on US2; this is deliberate, avoiding an unsafe preliminary installer. US2 service/fault tests consume US1 payload. US3 service tests materialize owned fixture trees and need only foundations; its real publication interleavings and shared CLI edits integrate after US2. US4 consumes foundational descriptors/generator and US1 examples; its separate test modules can proceed alongside US2/US3, while shared skill-test/Makefile/CLI edits are serialized. S4 requires every story and real agent evidence.

## Parallel Examples

- US1: frontmatter/link tests and every-entry recipe fixtures are separate modules; after those contracts, workflow/reference text and scaffold drafting can be split by file, then rendered/embedded together.
- US2: plan, recovery, CLI contract and concurrency test modules can be authored independently after foundations/US1; apply.go mutations remain sequential.
- US3: status unit and integration fixture modules can proceed together; inspect.go and shared skills.go changes integrate serially after US2 service wiring.
- US4: pin tests and descriptor perturbation tests are separate modules; descriptor/renderer code already exists from foundations. Serialize fixture-test and guidance edits where files overlap.
- S4: catalog docs and README/marketplace links are separate files; evidence/release checks follow completed integration.

## Implementation Strategy

The smallest useful demonstration is the US1 payload plus safe fresh installation after US2, following S0 and foundations. Stories have independent service/content tests, but the public release includes every slice, pure status, drift gates and actual agent dogfood. Milestone demonstrations do not authorize partial public delivery. Do not merge, commit, push, deploy or accept ADRs merely because tasks are generated. Future implementation stops at unfulfilled S0/real-agent gates with explicit evidence.

## Task Summary

- Phase 1: Setup — S0 Authorities and Dependency: T001–T005 (5 tasks).
- Phase 2: Foundational — Shared Safe Content and Target Model: T006–T015 (10 tasks).
- Phase 3: User Story 1 — Equip an Agent (P1, S1): T016–T024 (9 tasks).
- Phase 4: User Story 2 — Refresh Safely (P1, S2a): T025–T036 (12 tasks).
- Phase 5: User Story 3 — Pure Status (P2, S2b): T037–T041 (5 tasks).
- Phase 6: User Story 4 — Contract Alignment (P2, S3): T042–T049 (8 tasks).
- Phase 7: Polish and Cross-Cutting Delivery — S4: T050–T056 (7 tasks).

Total: 56 tasks. Required coverage is explicit in task FR/SC references; numeric ranges are inclusive.

## Implementation tracking note

Tests for the installer are colocated in internal/skillinstall (including native helper-process crash tests) and CLI tests in internal/cli. A checked task records implemented behavior with current evidence; unchecked tasks retain broader matrix, fixture, diagnostic or delivery obligations. See validation.md.

## Owner-approved CLI experience revision (2026-11-14)

- [X] Replace default-both/selectable-skill/global-folder grammar with exactly one vendor, explicit local/user scope and complete embedded inventory selection; reject legacy flags and skill operands.
- [X] Confirm both installation scopes before writes, support --yes for automation, and keep --force as separate content-replacement permission.
- [X] Rename the bundled skill to work-cli-plugin-builder and update assets, references, generation/pin tooling, docs and tests. Record the revised requirement and superseding ADR-0034 before updating ADD/CLI contracts.
- [X] Verify both vendor/scope combinations, invalid grammar purity, full inventory status, local-edit protection, preview contents and relative Claude user anchors; regenerate/review references, explicitly run make skill-lock, and pass lint.
- [X] Pass full host go test ./... and interactive terminal accept/decline checks for both scopes without cancellation side effects.
