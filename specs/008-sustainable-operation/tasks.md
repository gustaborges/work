# Tasks: Sustainable Platform Operation (F7)

**Input**: Design documents in `specs/008-sustainable-operation/`.
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `quickstart.md`, and all five documents in `contracts/`.
**Tests**: Included because the specification explicitly requires automated acceptance, repeated-read, failure/interruption, concurrency, terminal and supported-platform evidence (SC-001–SC-008). Add behavioral tests before the corresponding implementation and establish the expected failure; retain existing regression coverage.
**Organization**: Milestones contain setup, shared foundations, one phase per user story in priority order, and final cross-cutting validation. Milestone numbers are local to this feature; M7 below is not roadmap F7.

## Milestone Index

| Milestone | Phase / outcome | Task range | Exit gate |
| --- | --- | --- | --- |
| **M1** | Phase 1: Setup and offline fixtures | T001–T003 | Native fixtures, isolated homes, local Git sources and deterministic checkpoints are ready. |
| **M2** | Phase 2: Shared foundations | T004–T028 | Pure coherent reads, retained undo/recovery, all existing writers and revision-protected invocation pass focused checks. |
| **M3** | Phase 3: US1 — Inspect and control plugins / MVP | T029–T037 | Atomic toggles and complete inventories preserve policy and prevent disabled execution. |
| **M4** | Phase 4: US2 — Explicit repository resolution | T038–T047 | Policy/root direct operations preserve unavailable entries and validate whole requests. |
| **M5** | Phase 5: US3 — Deliberate updates | T048–T061 | Pure checks and fixed reviewed updates report every target and commit independently. |
| **M6** | Phase 6: US4 — Dependency-aware removal | T062–T072 | Whole-set uninstall preserves linked sources and durable bundled removal; offline reinstall works. |
| **M7** | Phase 7: US5 — Hubs and convention availability | T073–T085 | All three hubs share direct services; unavailable choices remain explicit and terminal contracts pass. |
| **M8** | Phase 8: Polish and release candidate | T086–T091 | Cumulative F7/F1–F6, upgrade, recovery, portability and release gates have recorded evidence. |

**Default delivery**: M1 → M2 → M3 → M4 → M5 → M6 → M7 → M8. **Suggested MVP**: M1–M3, ending at T037. A direct-control increment must include the complete M2 invocation and writer safeguards.

### Milestone-scoped implementation

- “Implement milestone 5” means M5 / T048–T061; “implement through milestone 5” means M1–M5 / T001–T061. Apply the same mapping to every milestone.
- Inspect completed tasks and evidence first; reuse completed prerequisites and finish any missing prerequisites needed by the requested scope.
- Complete a milestone only when every task in its range and its exit checks pass. Mark verified tasks complete and report task IDs, evidence and limitations.
- Stop at a requested milestone boundary. Continue to later milestones when the user requests them or requests the entire feature.

## Format and boundaries

Every task follows `- [ ] Tnnn [P?] [USn?] Description with exact file paths`. Paths are relative to the repository root. `[P]` permits only dependency-ready work on different files; the parallel batches below define the permitted examples. It never overrides a prerequisite or allows simultaneous edits to a shared file.

Use Go 1.26.0 and existing dependencies, locking, atomic replacement and presentation. Work snapshot schema and database version remain 3; no plugin protocol change, new role, background update, runtime timeout, source channel or new index command. The constitution is an unratified template; requirements and design remain governed by the existing PRD, accepted ADRs and feature artifacts. These tasks implement that design without adding requirements or changing accepted decisions.

All management targets are exact aliases, deduplicated and sorted. All administrative writes participate in the finite journal/epoch protocol. Source retrieval and review occur outside the write lock. No administration acquisition occurs under a Work/branch lock. Pure reads and bare hub entry never bootstrap, recover, create locks/layout/indexes or backfill state. Only a valid explicit mutating invocation with required automation confirmation runs recovery preflight, before semantic lookup/retrieval; final commit rechecks recovery and current state. Recovery of a prior operation is reported separately.

## Milestone 1 (M1): Phase 1 — Setup and Offline Fixtures

**Purpose**: Extend the existing project and test harness; no module initialization or dependency upgrade is needed.

- [X] T001 Create `tests/fixtures/plugins/admin-suite/plugin.json` and `tests/fixtures/plugins/admin-suite/main.go` with version release-blue, Locator clone accepting name, convention admin-team/prefix admin/{slug}, and no fallback Starter; provide harness-selected Starter/Linker/Importer variants plus invocation logging, failure and pipe-blocked modes for all executable roles without production mutation flags.
- [X] T002 Extend `tests/fixtures/plugins/plugins.go` and `tests/integration/integration_test.go` to build native fixture entrypoints with Windows suffix handling, produce local-pinned/local-linked/file:// remote-pinned sources and version/declaration revisions, allocate isolated homes and expose process synchronization without sleeps; depends on T001.
- [X] T003 Add reusable test helpers in `tests/integration/admin_helpers_test.go` for read-only tree existence/byte comparisons, linked-source and Work-tree baselines, owned temporary inventories, legacy registry/seed/lifecycle fixtures, and child-process interruption checkpoints; never open a writable verifier or traverse a linked source as owned state; depends on T002.

**Checkpoint**: Fixture preparation works offline under spaces/Unicode paths; subprocess blocking and restart fixtures are deterministic. Run `make seed` before host application tests.

## Milestone 2 (M2): Phase 2 — Foundations (Blocks Every Story)

**Purpose**: Make existing installation, bootstrap, configuration and activation coherent before exposing new maintenance mutations. FR-007, FR-019, FR-024, FR-028, FR-031–FR-032; research R1–R6/R9/R11–R12; state contract; Q1/Q8–Q10.

- [X] T004 [P] Add additive metadata, default enablement, shared convention ownership and path-containment cases in `internal/registry/registry_test.go` and `internal/workhome/workhome_test.go`, including missing version/source/digest, ownerless legacy declarations and reserved alias/transaction path traversal.
- [X] T005 Add optional version/source/content_digest fields and embedded origin support in `internal/registry/registry.go`, preserving old reference fields and enabled defaults; add lifecycle/control/journal/owned-stage path helpers in `internal/workhome/workhome.go`, with validated aliases/IDs and owner-only new recovery directories; depends on T004.
- [X] T006 [P] Add categories 50–56 and exact tokens from `contracts/cli-work-plugin.md` to `internal/diag/diag.go` and `internal/diag/diag_test.go`, preserving 0–49 and mapping cancellation, default 30-second contention, conflict, unresolved recovery and committed cleanup distinctly.
- [X] T007 [P] Add `internal/adminstate/snapshot_test.go` for absent-home defaults, legacy installed-manifest projection, no linked-live version backfill, stable malformed state, pending/unsafe control, cancellation, three-attempt bounds, intermediate-load-error drift and a whole transaction completing between control reads.
- [X] T008 Implement `adminstate.Read` and typed finite snapshots in `internal/adminstate/snapshot.go`: read control A, config/raw registry/memory/lifecycle and bounded managed metadata, then control B even after load errors; accept equal stable epochs only, retry drift at most three times, buffer results and never mutate or open the Work database; depends on T005–T007.
- [X] T009 [P] Add `internal/adminstate/transaction_test.go` for latest-state narrow edits, whole-set publication, recorded absent files, prepared-before-pending ordering, every move/JSON boundary, epoch monotonicity, no-op behavior, lock timeout/cancellation and before/after committed cancellation using injected filesystem/checkpoint seams.
- [X] T010 [P] Add `internal/adminstate/recovery_test.go` for prepared rollback, committed forward completion, stable leftover cleanup without overwriting a newer transaction, unsafe/malformed/missing evidence, failed compensation, linked-root ownership and postcommit cleanup failure; use process-helper restart coverage where needed.
- [X] T011 Implement `WithWrite`, locked sessions and finite image/move journals in `internal/adminstate/transaction.go` using existing lockfile/atomicfile primitives: recover then load latest state, retain existence/bytes/digests, publish prepared → pending → after-images → committed → stable → cleanup, with cancellation checks before commitment; depends on T008–T010.
- [X] T012 Implement locked recovery and mutation preflight in `internal/adminstate/recovery.go`: compensate with a fresh bounded context, authorize live recovery only through the control transaction ID, retain unsafe evidence, keep stable leftovers cleanup-only, and return typed prior-recovery/committed-cleanup results with concrete interrupted-command guidance; depends on T011.
- [X] T013 [P] Add `internal/plugininstall/prepare_test.go` for frozen local/remote content, deterministic digest/file-kind/executable/link-text normalization, contained symlink chains and symlinked manifests, rejection of absolute/escaping/cyclic links/special files/escaping entrypoints before reads, runtime/name/fallback validation, and exactly-once owned cleanup.
- [X] T014 Extract owned `Prepare` and pure registration helpers into `internal/plugininstall/prepare.go` from `internal/plugininstall/install.go`, retaining a single parsed manifest/source identity/reference/digest through use; share pinned install/update validators, preserve external --link semantics and revalidate catalog-sensitive constraints against latest state without executing components; depends on T005, T013.
- [X] T015 Refactor `internal/plugininstall/swap.go` and `internal/plugininstall/install.go` to expose same-filesystem transaction-owned staging/moves/undo; retain backups until registry/control publication and make production installs use the journal instead of early-cleanup StageThenSwap; preserve existing install receipts/alias-conflict behavior; depends on T011–T014.
- [X] T016 [P] Add legacy adoption/bootstrap cases in `internal/bootstrap/bootstrap_test.go` and `internal/bootstrap/stress_test.go` for genuinely fresh seeding, intact ownerless seed, install-meta policy_seeded, edited/empty policy, explicit disablement, damage versus removed history, ambiguous evidence and nested-lock prevention.
- [X] T017 Implement schema-1 reference lifecycle and proven virtual legacy seed ownership in `internal/adminstate/lifecycle.go` and `internal/adminstate/snapshot.go`: initialized/policy_seeded/removed remain outside the package, enablement remains in the registry, reads only project and authorized writes adopt; reject ambiguous history instead of reseeding; depends on T008, T012, T016.
- [X] T018 Refactor `internal/bootstrap/bootstrap.go` into a common-lock public entry point plus an already-locked seed preparation path using prepared content and journal publication; register owned version/digest/conventions, seed policy only on genuinely new setup, honor removed/disabled state and preserve policy edits on repair/upgrade; depends on T014–T017.
- [X] T019 Route existing source install/reinstall in `internal/cli/plugin_install.go` through prepared retrieval outside the lock and current-state journal publication inside it; preserve alias/source collision rules and retained disablement, publish content/registry together and never insert Locators into policy; depends on T015, T018.
- [X] T020 Add shared pure-read/service composition, mutation grammar/preflight sequencing and typed result adapters in `internal/cli/admin_shared.go`; update `internal/cli/plugin_shared.go`, `internal/cli/repository_shared.go` and `internal/cli/convention_shared.go` so list/show and initial hub inspection bypass layout/bootstrap/recovery, and update `internal/cli/diagnostics_border.go` for one safe boundary-rendered failure; depends on T008, T012, T019.
- [X] T021 Route existing policy/root writers in `internal/cli/repository_policy.go`, `internal/cli/repository_root.go` and `internal/cli/repository_shared.go` through narrow latest-state operations composed in `internal/cli/admin_shared.go`; preserve current direct grammar/receipts and stop saving captured whole configs; depends on T020.
- [X] T022 Route existing convention direct/hub upserts in `internal/cli/convention_set.go`, `internal/cli/convention.go` and `internal/cli/convention_shared.go` through a shared locked latest-memory operation, retaining unrelated repository choices and validating current effective providers; depends on T020.
- [X] T023 Route start setup/implicit first-choice/accepted replacement and workspace config writes in `internal/cli/start.go` and `internal/workspace/workspace.go` through latest-state journal operations; keep all input outside locks and prevent stale whole-config or memory saves; depends on T020–T022.
- [X] T024 [P] Add revision/eligibility/invocation lock tests in `internal/adminstate/invocation_test.go` and deterministic subprocess/publication races in `tests/integration/admin_concurrency_test.go`, including identical declarations with changed bytes, disabled/removed providers, blocked maintenance, post-run drift/pending state and administration/Work lock inversion.
- [X] T025 Implement callback-based `WithInvocation` in `internal/adminstate/invocation.go`, protecting the alias path through subprocess completion after current revision/declaration/eligibility revalidation; release before Work/branch locks and retain existing acquisition timeout without adding execution timeouts; depends on T012, T024.
- [X] T026 Wire guarded Starter/Locator execution and revision-aware selections in `internal/starter/starter.go`, `internal/locator/locator.go`, `internal/locator/chain.go` and `internal/cli/start.go`; return conflict/53 before materialization if selected facts changed and keep existing resolution outcomes; depends on T023, T025.
- [X] T027 Wire managed Importer/Linker execution and live revision-aware reloads in `internal/extension/import_one.go`, `internal/extension/importer.go`, `internal/extension/linker.go`, `internal/extension/run.go`, `internal/extension/publish.go`, `internal/cli/context_shared.go`, `internal/cli/link.go` and `internal/cli/import.go`; use pure post-run checks under Work locks, administration → Work for no-process manual association, manual-unavailable/39 versus existing automatic warnings, and preserve committed Works on automatic failure; depends on T025–T026.
- [X] T028 Add writer participation/import-boundary checks in `tests/contract/administrative_state_test.go` and foundation read/restart cases in `tests/integration/admin_purity_test.go` and `tests/integration/admin_recovery_test.go`; audit every production config.Save/registry.Save/repoconv.Save caller, prove current-directory status still works during admin recovery, and run focused adminstate/install/bootstrap/workspace/activation suites before enabling story mutations; depends on T021–T027.

**Checkpoint**: Existing install/bootstrap commit coherently; fresh administrative reads create nothing; legacy adoption never reseeds policy; journal boundary/restart checks pass; every writer and executable role participates. M3–M7 cannot bypass this gate.

## Milestone 3 (M3): Phase 3 — US1: Inspect and Control Installed Plugins (P1) — MVP

**Goal**: Show complete package/Locator state and atomically toggle whole packages while retaining configuration and history. FR-001–FR-007/FR-025/FR-028/FR-031; SC-001–SC-002; Q1–Q3/Q9–Q10.

**Independent test**: Install two fixtures, explicitly order their Locators, disable one, inspect human/JSON inventory and policy, then reenable. Policy bytes/order and remembered choices/Works stay unchanged; all executable roles reject disabled providers and restored providers resume at the retained position. Repeat outside-policy, idempotent, duplicate-alias and unknown-in-batch cases.

### Tests

- [X] T029 [P] [US1] Add `tests/contract/plugin_administration_test.go` for the retained plugin-list array/keys, nonnull arrays, sorted aliases/manifest component order, enabled/version/digest/inspection_status fields and stable enable/disable receipts with usage/2 on unknown aliases.
- [X] T030 [P] [US1] Add `internal/pluginadmin/enable_test.go` for complete alias validation, deterministic deduplication, atomic whole-set toggles, no-op byte preservation, concurrent latest-state edits, raw disabled inventory and retained shared convention ownership.
- [X] T031 [P] [US1] Add `tests/integration/plugin_management.txtar` for install → explicit policy inclusion → disable/inspect/enable and an outside-policy provider, asserting policy/memory/Work preservation and disabled/legacy/embedded inventory states.
- [X] T032 [P] [US1] Add native invocation-log and repeated-read cases in `tests/integration/admin_enable_test.go` for disabled Starters/Locators/Linkers/Importers, another enabled owner of a shared convention, retained Locator positions and 20 pure reads of fresh/legacy/damaged/linked-drift state.

### Implementation

- [X] T033 [US1] Implement raw package inventory and secret-safe public source/reference/identity projection in `internal/pluginadmin/inventory.go`, including nullable legacy baseline metadata, unknown versus unavailable inspection status, deterministic ordering and escaped author control text; render compatible human/JSON results through `internal/cli/plugin_list.go`; depends on T029, T032.
- [X] T034 [US1] Implement `SetEnabled` in `internal/pluginadmin/enable.go` as one latest-state validated transaction for the deduplicated alias set, preserving all policy/config/memory/content and avoiding writes for satisfied requests; depends on T030.
- [X] T035 [US1] Add exact-alias `work plugin enable|disable <PLUGIN...>` in `internal/cli/plugin_enable.go`, `internal/cli/plugin_disable.go` and `internal/cli/plugin.go`, composing the shared preflight/services with no confirmation, rejecting mutation --json and printing one stable satisfied line per alias; depends on T033–T034.
- [X] T036 [US1] Add reusable Locator state/reason projection in `internal/repoconfig/policy.go` and expose it through `internal/cli/repository.go` and `internal/cli/repository_policy.go`, distinguishing package-missing/component-missing/role-changed/package-disabled in precedence order while preserving raw inventory, policy traversal order and legacy ownerless behavior; depends on T033–T035.
- [X] T037 [US1] Run Q1–Q3 enablement/inventory checks and focused registry/pluginadmin/activation tests, extend assertions in `tests/integration/admin_enable_test.go` for zero policy writes and disabled executions, and record the MVP evidence in `specs/008-sustainable-operation/validation.md`; depends on T029–T036.

**Checkpoint**: US1 is independently usable through direct commands. Complete M1–M3 is the MVP; repository hubs arrive in M7.

## Milestone 4 (M4): Phase 4 — US2: Manage Repository Resolution Explicitly (P1)

**Goal**: Preserve ordered unavailable references and all-or-none roots while making every policy membership change explicit. FR-006–FR-010/FR-028/FR-031–FR-033; SC-002; Q1/Q3–Q4/Q10.

**Independent test**: Install another Locator and verify no policy insertion; explicitly add/reorder disabled and enabled providers, retain/move/remove unavailable refs, then exercise every root operation. Invalid late operands, canonical policy duplicates and invalid anchors change nothing. Direct-to-hub parity is checked after M7 implements the shared-service UI.

### Tests

- [X] T038 [P] [US2] Add `tests/contract/repository_administration_test.go` for retained JSON envelopes, state/reason fields, nonnull empty collections, 1-based policy order, stable result lines and unchanged direct positioning grammar.
- [X] T039 [P] [US2] Extend `internal/repoconfig/repoconfig_test.go` for disabled new Locators, retained missing/wrong-role refs, bare installed/retained ambiguity, duplicate canonical replacement rejection, self/missing anchors, empty collections, root dedup/workspace overlap and a late invalid removal operand leaving the original slice untouched.
- [X] T040 [P] [US2] Extend `internal/locator/locator_test.go` for ordered available/eligible traversal, continue-on-no-result/ineligibility, stop-on-valid-result/ambiguity/operational-error and invalid candidates; prove policy editing never executes components.
- [X] T041 [P] [US2] Add direct-command purity/rejection/concurrency cases in `tests/integration/admin_repository_test.go`, including install/reinstall with no membership change, disabled additions and whole-request preservation on validation failure.

### Implementation

- [X] T042 [US2] Resolve newly included Locator references against raw installed declarations in `internal/repoconfig/policy.go`, including disabled owners and qualified/bare collision handling; keep remove/move/anchor lookup policy-only and preserve unavailable entries; depends on T039.
- [X] T043 [US2] Implement full validation before policy assignment in `internal/repoconfig/policy.go`, explicitly retain existing unavailable qualified refs during replacement, reject duplicate canonical refs and ambiguous installed/retained bare names, and retain existing no-op add/remove behavior; depends on T042.
- [X] T044 [US2] Make root filtering/validation in `internal/repoconfig/roots.go` operate on cloned collections and validate all operands before assignment, retaining expansion/readability/dedup/workspace-overlap rules and empty-set removal; depends on T039.
- [X] T045 [US2] Extend typed repository operations in `internal/cli/admin_shared.go` and `internal/cli/repository_shared.go` with precise requested edits and reviewed preconditions for future hubs, merging unrelated latest config changes and never persisting stale slices or whole configs; depends on T043–T044.
- [X] T046 [US2] Integrate raw Locator/state projections and shared policy/root operations in `internal/cli/repository.go`, `internal/cli/repository_policy.go` and `internal/cli/repository_root.go`, preserving human/JSON parity, explicit membership, no provider execution, mutation --json rejection and stable results; depends on T036, T038, T045.
- [X] T047 [US2] Run Q3–Q4 and focused repoconfig/Locator/direct repository suites, update intentional duplicate-collapse/bootstrap-on-read expectations in `tests/integration/repository_policy.txtar` and `tests/integration/repository_locator_list.txtar`, and record evidence in `specs/008-sustainable-operation/validation.md`; depends on T038–T046.

**Checkpoint**: Repository direct commands independently satisfy policy/root rules; resolution retains its established outcome distinctions. M7 must reuse these operations and prove hub parity.

## Milestone 5 (M5): Phase 5 — US3: Check and Apply Updates Deliberately (P1)

**Goal**: Retrieve only explicitly, review fixed candidates, and apply per-package transactions with exhaustive outcomes. FR-011–FR-019/FR-022/FR-028–FR-032; SC-003–SC-004; Q1/Q5–Q6/Q8/Q10/Q12.

**Independent test**: Publish differing local-pinned and file:// remote-pinned opaque versions, check 20 times with no persistent changes/temp leaks, then pause after review and change the source before applying. The reviewed bytes/reference must install with retained alias/disablement/policy. Invalid/same-version/name-changing candidates, withdrawal approval/refusal and mixed-success batches must match the contract.

### Tests

- [X] T048 [P] [US3] Add `tests/contract/plugin_update_test.go` for exact grammar/flag exclusions, check JSON fields/nulls/statuses, sorted exhaustive rows, empty-set success, apply result/error lines, one terminal diagnostic and exit precedence for aggregate/cleanup/recovery/cancellation.
- [X] T049 [P] [US3] Add `internal/pluginadmin/update_test.go` for all origins, unknown aliases before retrieval, installed-versus-live legacy baselines, opaque inequality including smaller versions, invalid same-version candidates, runtime/namespace ownership, registration deltas/shared owners and exactly-once candidate cleanup.
- [X] T050 [P] [US3] Add fixed local-copy/remote-HEAD review races in `tests/integration/plugin_update_test.go`, including source/link-target drift, changed enablement/revision/dependencies requiring renewed review, unrelated config merges and check-baseline/epoch drift before result output.
- [X] T051 [P] [US3] Add `tests/integration/admin_update_batch_test.go` for missing confirmation/dependency authorization before any application, independent valid/failed targets, cancellation preserving earlier commits, every swap/publication interruption and unresolved recovery stopping remaining targets with not-applied outcomes.

### Implementation

- [X] T052 [US3] Define owned prepared candidates and deterministic alias/origin resolution in `internal/pluginadmin/update.go`, using installed proven baselines and `plugininstall.Prepare` once per target; classify linked as source-managed and embedded as not-applicable before baseline checks, require reinstall for unknown pinned baselines and retain frozen manifests/bytes until Close or transaction transfer; depends on T049.
- [X] T053 [US3] Implement pure exhaustive `Check` in `internal/pluginadmin/update.go`, validate even same-version candidates, compare opaque versions only, return every safe result and verify current target baseline/control before output with bounded retry/conflict; keep retrieval in OS temp, no installed fetch/cache/timestamp and clean on all normal exits; depends on T048, T050, T052.
- [X] T054 [US3] Compute exact reviewed declaration changes, ordered withdrawn/role-changed policy refs and provider effects in `internal/pluginadmin/update.go`; require distinct dependency-removal authorization, retain candidate identities and invalidate review when target/source/version/digest/enablement/name or impact changes; depends on T043, T052.
- [X] T055 [US3] Implement fixed-candidate `Apply` in `internal/pluginadmin/update.go` with latest-state revalidation and one journal transaction per package, publishing frozen content/registrations/approved references together; retain alias/source identity/disablement/shared owners/unrelated settings and never insert new policy entries; depends on T051, T054.
- [X] T056 [US3] Implement deterministic batch outcome/cleanup orchestration in `internal/pluginadmin/update.go`, proceeding after independent target failures, preserving prior commits on cancellation, marking unstarted targets not-applied, and prioritizing recovery/cleanup then cancellation over aggregate failure; reject missing batch-wide authorization before any new apply; depends on T053, T055.
- [X] T057 [US3] Add `work plugin update` grammar and read/direct automation paths in `internal/cli/plugin_update.go` and `internal/cli/plugin.go`, accepting --json only for --check, rejecting illegal --check/--all/target/--yes/dependency combinations before preflight, requiring noninteractive confirmation and printing exhaustive rows plus one final failure; depends on T048, T056.
- [X] T058 [US3] Implement interactive untargeted selection and determined-impact review in `internal/cli/plugin_update.go` using existing present.Wizard controls outside the lock; explicit targets skip selection, --yes never chooses targets, dependency approval names exact refs, and retained candidates close on refusal/cancellation; depends on T054, T057.
- [X] T059 [US3] Apply safe source projections to check/review/results/install conflicts and normal Git failures in `internal/pluginadmin/inventory.go`, `internal/plugininstall/install.go`, `internal/cli/plugin_update.go` and `internal/cli/diagnostics_border.go`; add credential/control-text assertions in `tests/integration/admin_credentials_test.go` for userinfo/password/query/fragment secrets and placeholder direct equivalents without raw Git stderr.
- [X] T060 [US3] Add no-source-retrieval regression checks in `tests/integration/admin_purity_test.go` for start/resume/archive/status/link/import, list/show and hub inspection; run 20 fresh/unchanged/newer/invalid/inaccessible explicit checks with tree/content/temp inventories and no component execution; depends on T053, T057, T059.
- [X] T061 [US3] Run Q5–Q6/Q8/Q10 update/candidate/batch suites, confirm valid JSON stdout on exit 50 and fixed reviewed bytes under source races, and record evidence in `specs/008-sustainable-operation/validation.md`; depends on T048–T060.

**Checkpoint**: Direct/selected updates are complete without the plugin hub. Every resolved target receives an outcome; invalid/refused candidates preserve old installations and no update silently expands dependency consent.

## Milestone 6 (M6): Phase 6 — US4: Uninstall with Explicit Dependency Handling (P1)

**Goal**: Remove one approved package set atomically, retain external/Work data and preserve bundled removal across setup/upgrade. FR-020–FR-025/FR-029–FR-032; SC-005; Q7–Q10.

**Independent test**: Refuse then approve removal of multiple packages with several current/stale policy refs. Unknown aliases and late recoverable failures preserve the whole set; success removes exactly the approved refs. Linked sources and existing Works/memory remain byte-identical. Remove the bundled package, run ordinary setup and upgrade, then explicitly reinstall offline without policy insertion.

### Tests

- [X] T062 [P] [US4] Add `tests/contract/plugin_uninstall_test.go` for exact alias/confirmation/dependency grammar, --yes versus dependency authorization, stale-ref impacts, whole-set stable receipts and codes 51–56 with commitment-aware diagnostics.
- [X] T063 [P] [US4] Add `internal/pluginadmin/uninstall_test.go` for deduplicated complete-set review, unknown aliases before target changes, late move/save failures restoring all targets, concurrent impact drift and preservation of unaffected policy relative order/shared convention owners.
- [X] T064 [P] [US4] Add `tests/integration/plugin_uninstall_test.go` for linked symlink/directory ownership, external source byte preservation, unchanged Works/snapshots/artifacts/links/branches/checkouts/memory, disabled target removal and no partial receipts on failed sets.
- [X] T065 [P] [US4] Add durable seed-removal/reinstall and upgrade cases in `tests/integration/admin_seed_test.go`, including removed history across a newer embedded release, disabled repair, ownerless/install-meta legacy adoption, edited/empty policy, no capability resurrection and mutually exclusive --embedded grammar.

### Implementation

- [X] T066 [US4] Implement uninstall planning in `internal/pluginadmin/uninstall.go`, determining the complete sorted alias set, owned roots, convention effects and all policy refs owned by those aliases including stale declarations; retain exact reviewed facts and require dependency authorization separately from confirmation; depends on T062–T063.
- [X] T067 [US4] Implement whole-set `Uninstall` in `internal/pluginadmin/uninstall.go` as one latest-state revalidated journal transaction, removing only owned package roots/registrations/approved refs and preserving shared ownership/unrelated config; never follow external linked roots and compensate the entire set on precommit failure; depends on T064, T066.
- [X] T068 [US4] Include reference-package removed=true lifecycle after-images in the same uninstall transaction in `internal/pluginadmin/uninstall.go` and `internal/adminstate/lifecycle.go`, retaining initialization/policy history and registry-only disablement; ordinary bootstrap and upgraded repair must honor that durable intent; depends on T065, T067.
- [X] T069 [US4] Add explicit offline `work plugin install --embedded` in `internal/cli/plugin_install.go` via an already-locked seed path in `internal/bootstrap/bootstrap.go`, rejecting SOURCE/--link/--as, retaining the reserved alias, setting initialization handled/removed=false and never inserting policy even on fresh state; depends on T018, T065, T068.
- [X] T070 [US4] Add missing-capability install/enable/configure guidance to `internal/cli/start.go`, `internal/starter/starter.go` and `internal/locator/chain.go` when retained bundled removal/disablement leaves no usable provider, without reinstalling, clearing memory or restoring policy; depends on T068–T069.
- [X] T071 [US4] Add `work plugin uninstall <PLUGIN...>` in `internal/cli/plugin_uninstall.go` and `internal/cli/plugin.go`, sharing preflight/typed plan/review, explicit interactive dependency approval and noninteractive --yes/--remove-policy-references rules; print receipts only after the complete set commits and render committed cleanup distinctly; depends on T062, T067–T070.
- [X] T072 [US4] Run Q7–Q9 removal/linked/seed/upgrade and injected rollback suites, compare external/Work/memory baselines and record evidence in `specs/008-sustainable-operation/validation.md`; depends on T062–T071.

**Checkpoint**: No recoverable removal leaves a partial selected set, and deliberate seed removal survives ordinary commands and supported upgrade. Reinstall remains offline and does not restore membership.

## Milestone 7 (M7): Phase 7 — US5: Complete Administration Through Discoverable Hubs (P2)

**Goal**: Provide three complete keyboard hubs using direct services, explicit unavailable-convention handling and safe terminal output. FR-025–FR-031/FR-033; SC-006–SC-007; Q6/Q11–Q12.

**Independent test**: Perform plugin install/toggle/check/apply/remove, policy reorder/root replacement and convention replacement through hubs and direct equivalents against identical fixtures. Across two clones, disable/remove the remembered convention's only provider: show retains its value, interactive start explicitly asks for replacement even with one alternative, and automation fails with guidance. Cancellation, Leave unchanged, invalid input and redirected streams preserve state/terminal contracts.

### Tests

- [X] T073 [P] [US5] Add availability/remembered-choice cases to `internal/repoconv/repoconv_test.go` and `internal/cli/convention_test.go`, covering no choice, unavailable/shared enabled owners, pure human/JSON views, clone identity and narrow concurrent upserts without changing existing Works.
- [X] T074 [P] [US5] Add `tests/contract/administrative_hubs_test.go` for full command inventory/direct equivalents, automation values/flags, stable result/UI separation, noninteractive usage with zero controls and no presentation imports from adminstate/pluginadmin.
- [X] T075 [P] [US5] Add Unix PTY journeys in `tests/integration/admin_hubs_test.go` for all three hubs, direct/hub equality, dependency review/refusal, empty inventories, Back/Leave unchanged, active-field correction, cancellation/resize, supported viewports/colors/NO_COLOR/TERM=dumb, terminal restoration, accepted safe receipts and one final error.
- [X] T076 [P] [US5] Add `tests/integration/admin_convention_test.go` for remembered unavailable choices across clones, explicit sole-alternative replacement during start, noninteractive convention-unknown/38, fresh sole-choice defaults, contribution bypass and provider/memory drift after review.

### Implementation

- [X] T077 [US5] Implement pure remembered-choice availability projection in `internal/repoconv/repoconv.go`, `internal/cli/convention_shared.go` and `internal/cli/convention_show.go`, retaining identity/convention keys and adding available/unavailable_reason with not-set/provider-unavailable; sanitize displayed identity credentials while preserving private matching identity; depends on T073.
- [X] T078 [US5] Extend the shared convention setter in `internal/cli/convention_shared.go` and `internal/cli/convention_set.go` to validate the reviewed repository choice/provider against the latest locked snapshot, upsert only that identity and return conflict on changed reviewed facts while merging unrelated memory; depends on T073, T077.
- [X] T079 [US5] Update unavailable remembered-convention selection/memoization in `internal/cli/start.go` to request explicit available replacement even with one alternative, retain unavailable memory on cancellation, fail noninteractively with guidance and reject provider/memory drift; preserve no-choice sole-default and contribution behavior without rewriting historical Works; depends on T076, T078.
- [X] T080 [US5] Compose shared full-screen hub navigation/review/result adapters in `internal/cli/admin_shared.go` using present.IO/Wizard, pure initial reads, explicit setup actions, no empty selectors, safe equivalent commands and refresh after accepted actions; keep services free of presentation and no locks across input/retrieval; depends on T074–T075, T079.
- [X] T081 [US5] Replace the plugin help-only entry point with the complete hub in `internal/cli/plugin.go` and `internal/cli/plugin_hub.go`, exposing list/source-or-embedded-install/enable/disable/check/apply/uninstall through the existing typed services, fixed review and explicit dependency authorization; reject bare --json/redirected input and keep only accepted safe receipts; depends on T058, T071, T080.
- [X] T082 [US5] Replace the repository help-only entry point with the complete hub in `internal/cli/repository.go` and `internal/cli/repository_hub.go`, exposing Locator/policy/root views and every add/remove/move/replace operation, retained unavailable entries, empty-set removal and exact order/root previews using the shared latest-state services; depends on T046, T080.
- [X] T083 [US5] Complete the convention hub in `internal/cli/convention.go` with safe identity/current availability, show/set equivalents, explicit available replacement and Leave unchanged, refusing empty selection without writing; use the same setter as direct commands and keep entry/navigation pure; depends on T078, T080.
- [X] T084 [US5] Update grouped discovery/help in `internal/cli/help.go`, `internal/cli/help_test.go` and `tests/integration/help_inventory.txtar` for all administrative actions and exact flags; replace intentional help-only hub expectations in `tests/integration/repository_help.txtar` and extend `tests/integration/admin_credentials_test.go` for safe source/identity reviews, receipts and equivalent commands; depends on T081–T083.
- [X] T085 [US5] Run Q6/Q11–Q12, PTY/redirected-stream/direct-hub parity and presentation-boundary suites, update intentional bootstrap-on-show expectations in `tests/integration/convention_show_set.txtar` and record evidence in `specs/008-sustainable-operation/validation.md`; depends on T073–T084.

**Checkpoint**: Every direct operation is discoverable in its hub and yields equivalent effective state. Unavailable convention memory remains explicit; cancellation and invalid/noninteractive requests never start unconfirmed changes or leave terminal controls behind.

## Milestone 8 (M8): Phase 8 — Polish and Cross-Cutting Release Validation

**Purpose**: Prove cumulative SC-001–SC-008 and FR-034 after all stories. Q1–Q13; roadmap release-candidate gates.

- [ ] T086 [P] Extend `tests/integration/admin_concurrency_test.go` and `tests/integration/admin_recovery_test.go` with cumulative config/memory/enable/bootstrap/install/update/uninstall races and helper-process interruption at prepare/pending/every move/every JSON/committed/stable/cleanup; verify exact before/after existence and bytes, reachable grammar-valid recovery, fail-closed unsafe journals and no invocation/publication deadlock.
- [ ] T087 [P] Add `tests/integration/release_candidate_test.go` for the complete F7 demonstration, clean offline bootstrap versus explicit embedded install, supported legacy/removed/disabled upgrade, real ambiguous local/remote branch fixtures, and index loss/rebuild through existing reconcile helpers; retain every F1–F6 demonstration and avoid public reindex commands.
- [ ] T088 [P] Complete output/compatibility checks in `tests/contract/administrative_state_test.go`, `tests/contract/present_boundary_test.go` and `tests/integration/admin_output_test.go` for unchanged 0–49 codes/old receipts/grammar/Work schemas, all new categories, secret-safe human/JSON/errors, exhaustive batch accounting and committed-cleanup reporting; document intentional duplicate/purity/pinned-symlink/hub expectation changes.
- [ ] T089 [P] Update `README.md` and `docs/add/add-0001-work-system-architecture.md` for implemented administrative workflows, storage/service boundaries and recovery guidance, tracing the existing PRD/accepted installation-update-bootstrap-policy-terminal ADRs; keep new implementation instructions in README and feature docs without inventing requirements or revising accepted decisions.
- [ ] T090 Run `make seed-all`, `make lint`, `go test ./...` and `make build` using `Makefile` and `.github/workflows/ci.yml`; verify Ubuntu/macOS/Windows coverage, Windows symlink capability handling and all five release asset targets, recording actual results or unavailable environments in `specs/008-sustainable-operation/validation.md`; depends on T086–T089.
- [ ] T091 Execute Q1–Q13 from `specs/008-sustainable-operation/quickstart.md` in disposable homes, complete the FR-001–FR-034/SC-001–SC-008 evidence matrix in `specs/008-sustainable-operation/validation.md`, and reconcile checked tasks in `specs/008-sustainable-operation/tasks.md` with passing evidence; distinguish local verification from unexecuted remote CI and record remaining limitations; depends on T090.

**Checkpoint**: Release-candidate evidence covers the entire current behavior and supported upgrade/recovery model. Do not mark unavailable platform/CI checks as passed.

## Dependencies and Execution Order

### Milestone and story graph

```text
M1 Setup → M2 Foundations → M3 US1 → M4 US2 ─┬→ M5 US3 ─┐
                                          └→ M6 US4 ─┴→ M7 US5 → M8 Release
```

The graph shows technical readiness, not permission to ship without milestone gates. Default implementation is the sequential order in the index.

| Phase/story | Hard prerequisites | Integration and independent validation |
| --- | --- | --- |
| M1 Setup | None | Uses the existing repository/toolchain. |
| M2 Foundations | M1 | Blocks all new maintenance; T028 requires every writer and invocation route, not only the state package. |
| M3 / US1 | M2 | Complete direct MVP and state projection; no later story needed. |
| M4 / US2 | M2; T036 for final CLI projection integration | Policy/root unit work can start after M2; final integration reuses US1 state fields. No update/removal needed to test stale refs: use fixtures. |
| M5 / US3 | M3–M4 | Reuses inventory/safe projections, dependency rules and guarded transactions; test independently with published local sources. |
| M6 / US4 | M3–M4 | Can technically proceed alongside M5 using separate domain files; default is after M5 to avoid shared CLI/bootstrap edits. Removal tests require no update. |
| M7 / US5 | M3–M6 | Full hubs require every direct service; convention unit work can be prepared earlier but completion requires complete parity. |
| M8 Release | M1–M7 | Cumulative verification, documentation and supported-platform evidence. |

### Within milestones

- Tests precede their corresponding implementation. `[P]` test authoring may share a batch only when files differ; running them green waits for the named services.
- In M2, T004 → T005; T005–T007 → T008; T009–T010 precede T011 → T012. T013 → T014, then T015; T016 → T017 → T018. T019–T023 route writers before T025–T027 activation integration; T028 gates the whole foundation.
- Prepared install validation (T014) never imports adminstate; low-level storage packages never import it. Bootstrap may import adminstate/plugininstall; pluginadmin may import bootstrap, never the reverse. This preserves the planned acyclic boundaries.
- M3 implementation runs T033 → T034 → T035 → T036 → T037. M4 runs T042 → T043, then T044 → T045 → T046 → T047; all tests are ready first.
- M5 runs T052 → T053/T054 → T055 → T056 → T057 → T058 → T059 → T060 → T061. T053/T054 share `update.go` and are sequential edits, despite their conceptual independence.
- M6 runs T066 → T067 → T068 → T069 → T070 → T071 → T072. Lifecycle and package/policy removal after-images must be part of the same transaction, never shipped as separate writes.
- M7 runs T077 → T078 → T079 → T080, then T081–T083 on distinct files, followed by T084 → T085. Avoid concurrent edits to `admin_shared.go` or shared test harnesses.
- M8 T086–T089 can run together after M7; T090 → T091 follows all code/documentation changes. `[P]` identifies file independence, not an instruction to spawn agents.

## Parallel Execution Examples

These are optional dependency-ready batches for implementers; no delegation is required by this task list.

| Scope | Parallel tasks | Readiness / separation |
| --- | --- | --- |
| Foundation tests | T004, T006, T007, T009, T010, T013, T016, T024 | After M1; distinct test/diagnostic files. Tests refer to planned contracts until implementations exist. |
| US1 | T029, T030, T031, T032 | After M2; contract, domain, testscript and native invocation tests use separate files. |
| US2 | T038, T039, T040, T041 | After M2; state/CLI final integration still waits for T036. |
| US3 | T048, T049, T050, T051 | After M4; separate contract/domain/fixed-candidate/batch files. |
| US4 | T062, T063, T064, T065 | After M4 technically, M5 by default; separate contract/domain/linked/lifecycle files. |
| US5 | T073, T074, T075, T076 | After M6 by default; distinct convention/contract/PTY/start tests. |
| Hub implementation | T081, T082, T083 | After T080 and each service prerequisite; separate hub files/entry points, shared adapters already complete. |
| Release | T086, T087, T088, T089 | After M7; disjoint race/release/output/documentation files. |

## Coverage and Milestone Acceptance

| Requirements / success criteria | Owning milestones | Required evidence |
| --- | --- | --- |
| FR-001–FR-005; SC-001 | M2–M3 | All roles, whole-set/idempotent toggles, raw inventory, default legacy enablement; Q1–Q2. |
| FR-006–FR-010; SC-002 | M2–M4, M7 parity | Reason precedence, explicit membership/order, unavailable retention, complete root validation and chain outcomes; Q3–Q4. |
| FR-011–FR-019; SC-003–SC-004 | M2, M5 | All origins/versions, fixed candidate, repeated purity, exact authorization, exhaustive batch outcomes; Q1/Q5–Q6. |
| FR-020–FR-024; SC-005 | M2, M6 | Whole-set rollback/removal, linked ownership, durable seed intent and supported upgrade; Q7–Q9. |
| FR-025–FR-026; SC-007 | M3/M6 retention, M7 | Shared providers, unavailable remembered choices, explicit replacement across clones, unchanged Works; Q11. |
| FR-027–FR-031/FR-033; SC-006 | M3–M7, M8 | Human/JSON/direct/hub parity, confirmation, keyboard/cancellation, safe streams/help/credentials; Q6/Q11–Q12. |
| FR-032; SC-004–SC-005 | M2, M5–M6, M8 | Every journal boundary/restart, coherent reads, current-state writers and guarded invocation/post-run checks; Q8/Q10. |
| FR-034; SC-008 | M8 | F7 and F1–F6 journeys, clean setup/upgrade, index rebuild, real ambiguous branches and supported OS/assets; Q13. |

## Implementation Strategy

1. Complete M1–M2 and verify the foundation gate before exposing lifecycle mutations. This prerequisite includes invocation integration across all roles and every existing writer.
2. Deliver M3 as the direct-control MVP. Validate disable → inspect → enable without policy, memory or Work edits.
3. Deliver M4, M5 and M6 incrementally, running each independent acceptance test before proceeding. Updates commit per package; uninstall commits one entire set.
4. Deliver M7 through the existing services and finish deferred direct/hub parity. Never implement a separate hub persistence/validation path.
5. Complete M8 and record actual evidence. Milestone completion does not imply a commit, PR, build publication or deployment; these tasks authorize implementation and verification only when invoked.

## Task Summary

- Total: **91 tasks** — setup **3**, foundations **25**, US1 **9**, US2 **10**, US3 **14**, US4 **11**, US5 **13**, polish/release **6**.
- **32 `[P]` tasks** identify separate-file test/diagnostic/documentation opportunities; hub implementation has an additional dependency-ready three-file batch described above.
- All task IDs are sequential; every user-story task has its matching `[USn]` label and every task names concrete files. No implementation is claimed complete by generation of this list.
