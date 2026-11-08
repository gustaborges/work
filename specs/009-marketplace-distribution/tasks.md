# Tasks: Marketplace Plugin Distribution

**Input**: [plan.md](plan.md), [spec.md](spec.md), research.md, data-model.md, contracts/, quickstart.md and ADR-0030.
**Created/revised**: 2026-11-07
**Status**: Milestones M1 (T001–T021), M2 (T022–T041) and M3 (T042–T057) implemented and validated; M4 remains pending.
**Scope correction**: The CLI is unreleased. Existing development plugins/state may break. Implement one purposeful current marketplace model and delete superseded code/tests; no compatibility readers/providers/selectors/adapters, migration/adoption flows, old-journal conversion, old-executable fences or compatibility-only suites.

## Format and execution rules

- Tasks use `- [ ] Tnnn [P?] [USn?] Description with exact repository-relative paths`.
- [P] means disjoint work within its phase after prerequisites. Tasks sharing files execute sequentially; milestone/phase dependencies still apply.
- Story work uses [US1]–[US5]; setup/foundation and cross-cutting delivery have no story label. Requirement IDs refer to this feature.
- Test the intended current behavior in fresh disposable homes, then implement it. Remove or rewrite superseded test expectations directly rather than preserving old behavior through shims.
- Current transaction recovery, leases, exact authorization, source provenance and one-time policy behavior remain required. They are product safety/semantics, not backward-compatibility mechanisms.
- Milestones are controlled demonstrations; public delivery requires the complete contract. No task authorizes a real-home reset/deletion, commit, push or remote publication during this planning pass.

## Milestone M1 — Sources and fixed discovery

**Scope/dependencies:** Shared setup/foundations + US1; product/architecture are formalized.

**Exit gate:** One strict current state/recovery model, complete fixed captures/leases, offline inspection, namespace validation and accurate totals.

### Phase 1 — Setup and fixtures

**Goal:** Establish reproducible author/source and old-format test evidence without changing application dependencies.

**Independent validation:** Fixture setup must be self-contained, disposable, and usable without external accounts.

- [X] T001 Record the intended marketplace baseline, native-platform evidence slots and complete release gate in specs/009-marketplace-distribution/checklists/implementation.md; verify PRD FR-79 and ADR-0030 require one current model and deletion of compatibility scaffolding. (FR-001, FR-024)
- [X] T002 [P] Create valid single/multiple/homonymous Git/local catalogs and invalid path/schema/name/symlink fixtures in tests/fixtures/marketplaces/; keep fixture package runtimes native or explicitly available. (FR-001, FR-004, FR-025)

### Phase 2 — Shared identity and administrative foundations

**Goal:** Provide one composed identity and current administrative model with safe preparation and interruption recovery.

**Independent validation:** Current schema/address/recovery/lease cases prove strict validation, pure inspection, exact rollback/commit completion and zero live-stage collection.

- [X] T003 [P] Add one composed provider/contribution/address grammar, safe portable names, hex-only capture storage and invalid-address tests in internal/pluginid/identity_test.go and internal/workhome/marketplace_test.go. (FR-004, FR-006, FR-024, FR-025)
- [X] T004 [P] Add strict current-schema rejection, finite image/address validation and prepared/committed transaction checkpoints in internal/adminstate/marketplace_recovery_test.go; include no prior-format or conversion cases. (FR-013, FR-014, FR-024)
- [X] T005 [P] Add deterministic live/dead capture/stage lease, orphan cleanup, cancellation, and unchanged-operation tests in internal/adminstate/lease_test.go. (FR-013, FR-014, FR-024)
- [X] T006 Implement the single composed provider/contribution/address model and portable storage resolution in internal/pluginid/identity.go and internal/workhome/workhome.go; remove alias address variants rather than keeping wrappers. (FR-004, FR-006, FR-025)
- [X] T007 Replace config, registry and convention memory with one strict current schema-1 shape in internal/config/config.go, internal/registry/registry.go and internal/repoconv/repoconv.go; remove earlier-shape decoders and alternate provider records. (FR-002, FR-006, FR-019, FR-024)
- [X] T008 Replace finite administrative load/serialization/no-op comparison and package moves with current catalog images and composed addresses in internal/adminstate/snapshot.go and internal/adminstate/transaction.go; include no schema conversion or old-binary fencing. (FR-002, FR-013, FR-014, FR-024)
- [X] T009 Implement one current journal validator, prepared rollback, committed replay, nested owned moves and cleanup in internal/adminstate/recovery.go; delete superseded alias move/recovery helpers and fallback decoders. (FR-013, FR-014, FR-024)
- [X] T010 Implement separate process-held per-plan object leases and nonblocking orphan collection in internal/adminstate/lease.go; acquire/validate under administration, release locks before lease-record cleanup, and protect live stages outside journals. (FR-013, FR-014)
- [X] T011 Verify fresh-home pure inspection/checks, strict rejection of malformed/unsupported current state and byte/epoch-preserving no-ops in tests/integration/marketplace_state_test.go and tests/integration/admin_purity_test.go; no earlier executable is required. (FR-014, FR-024)

### Phase 3 — US1: Sources and fixed discovery

**Goal:** Register Git/local sources, project builtin, and inspect fixed offers with display names and accurate local totals.

**Independent validation:** Register a non-Git source and local file:// Git source; disconnect origins and browse unchanged stored offers with zero installation or policy change.

- [X] T012 [P] [US1] Add strict schema/duplicate-key/full-manifest/portable-name/path-overlap/containment tests and canonical digest-vector checks in internal/marketplace/catalog_test.go and internal/marketplace/digest_test.go. (FR-001, FR-002, FR-025)
- [X] T013 [P] [US1] Add local pre/post-content capture, source mutation, exact Git revision/branch, unavailable commit, and snapshot cleanup scenarios in internal/marketplace/source_test.go. (FR-003, FR-007, FR-014, FR-025)
- [X] T014 [US1] Implement strict marketplace.json parsing, complete static entry validation, portable collision detection, and self-contained path validation in internal/marketplace/catalog.go. (FR-001, FR-002, FR-025)
- [X] T015 [US1] Implement the current package/catalog/capture digest algorithms and vector checks in internal/marketplace/digest.go using specs/009-marketplace-distribution/contracts/digest-vectors.json; use a separate owned-root journal digest where its scope requires it. (FR-003, FR-005, FR-025)
- [X] T016 [US1] Extend recorded-default/explicit branch discovery, one revision resolution, and exact committed-tree materialization in internal/gitx/gitx.go; add context cancellation and safe error projection without HEAD fallback. (FR-002, FR-003, FR-007)
- [X] T017 [US1] Implement coherent local capture, fixed Git view, and code-owned builtin provider resolution/resources in internal/marketplace/source.go, internal/marketplace/capture.go, seed/marketplace.json, and seed/embed.go; prepare outside administration and publish only complete leased captures. (FR-003, FR-005, FR-014, FR-025)
- [X] T018 [US1] Implement source registration/refresh, source-instance continuity, exact-source idempotence, builtin reservation, atomic config/catalog publication and independent upgrade results in internal/marketplaceadmin/sources.go; registration leaves default policy unhandled. (FR-002, FR-003, FR-013, FR-015)
- [X] T019 [US1] Implement pure safe source/catalog projection with virtual builtin, missing-catalog diagnostics, optional display names, and complete installed/enabled totals in internal/marketplaceadmin/inventory.go. (FR-002, FR-003, FR-015, FR-020, FR-026)
- [X] T020 [US1] Implement source add/list/upgrade and intended errors 57–61 in internal/cli/plugin_marketplace.go and internal/diag/diag.go; use current-purpose names and safe streams without compatibility tokens or command aliases. (FR-002, FR-020, FR-023, FR-026)
- [X] T021 [US1] Verify zero-install registration, source collision, official-name spoofing, offline/missing catalogs, no-op refresh, source totals, display-name changes, and independent batch outcomes in tests/integration/marketplace_sources_test.go and tests/contract/marketplace_cli_test.go. (FR-001, FR-002, FR-003, FR-015, FR-017, FR-020, FR-023)

## Milestone M2 — Independent copies and composed identity

**Scope/dependencies:** US2; depends on M1.

**Exit gate:** One selected entry copied; homonyms resolve through every contribution/private/guard path; direct-source/link/alias publishing branches are deleted.

### Phase 4 — US2: Independent copies and composed execution identity

**Goal:** Install one fixed entry, run homonymous providers independently, and remove direct-source publication in every production path.

**Independent validation:** Install two homonyms with equal components/conventions/private declarations; execute/select each, remove author sources, and prove independent copy behavior.

- [X] T022 [P] [US2] Add resolved-entry-only publication, exact snapshot/digest, selected-only copy, runtime, no-op/reinstall boundary, and standalone-service rejection cases in internal/pluginadmin/marketplace_install_test.go. (FR-001, FR-004, FR-005, FR-007, FR-025)
- [X] T023 [P] [US2] Add provider-owned convention/policy/memory and removed/re-registered-source retention tests in internal/registry/provider_test.go and internal/repoconfig/provider_test.go; same-named replacement providers cannot claim current retained choices. (FR-004, FR-006)
- [X] T024 [P] [US2] Add canonical private-owner encoding, dotted names, homonymous ownership, cross-provider input, self-relative foreign-owner rejection, source-instance non-rebinding, required/optional projection, and qualified-publication rejection cases in internal/semconv/provider_test.go. (FR-006, FR-025, FR-026)
- [X] T025 [P] [US2] Add self-package, exact-external, unique-global and ambiguous-global Starter subscription cases in internal/extension/provider_test.go using only the current reference grammar. (FR-006, FR-019)
- [X] T026 [US2] Introduce immutable resolved-entry preparation and selected package materialization in internal/marketplace/entry.go and internal/plugininstall/prepare.go, carrying actual source/entry/revision/digest facts through validation. (FR-001, FR-005, FR-007, FR-025)
- [X] T027 [US2] Route all new plugin publication through resolved-entry validation and one copy/registry transaction in internal/pluginadmin/install.go; remove arbitrary standalone/link/embedded publishing routes from internal/plugininstall/install.go, internal/plugininstall/link.go, and internal/plugininstall/swap.go. (FR-001, FR-005, FR-013)
- [X] T028 [US2] Implement actual provider and registration-instance ownership in registry upsert/effective/lookup/removal, convention records, and composed target paths in internal/registry/registry.go; equal bare names must not overwrite or share a mutable convention. (FR-004, FR-006, FR-007)
- [X] T029 [US2] Use full qualified references for policy availability/mutation and Locator resolution in internal/repoconfig/policy.go and internal/locator/locator.go; delete alias lookup/fallback rather than supporting multiple identifier schemes. (FR-006, FR-019)
- [X] T030 [US2] Use provider-qualified convention catalog/memory availability and explicit selection in internal/convention/convention.go, internal/repoconv/repoconv.go, and internal/cli/convention_shared.go; never substitute a same-named provider. (FR-006, FR-019)
- [X] T031 [US2] Implement canonical encoded relative/bound private keys, publication ownership, and explicit qualified input validation in internal/semconv/semconv.go and internal/plugin/manifest.go. (FR-006, FR-025, FR-026)
- [X] T032 [US2] Bind Starter private publications to the actual provider instance and persist full current Starter/source-component values in internal/starter/starter.go, internal/cli/start.go and internal/create/create.go; update current Work/projection contracts directly as needed. (FR-006, FR-019, FR-026)
- [X] T033 [US2] Bind private input lookup/output persistence to the recorded source instance while preserving declared subprocess payload keys and public semantic keys in internal/extension/eligible.go, internal/extension/publish.go, internal/extension/manual.go, and internal/extension/linker.go. (FR-006, FR-026)
- [X] T034 [US2] Implement registry-aware self-package/exact-qualified/unique-global subscription binding in internal/plugin/manifest.go and internal/extension/eligible.go; remove superseded alias grammar and emit ambiguity without subscriber execution. (FR-006, FR-025)
- [X] T035 [US2] Extend composed package/declaration/provenance baselines and live post-run reloads in internal/adminstate/invocation.go and internal/extension/run.go; preserve administration-before-invocation and release-before-Work-lock ordering. (FR-006, FR-013, FR-014)
- [X] T036 [US2] Consume the M1 builtin catalog/provider and resolve genuinely fresh work-reference@builtin setup through common copy/registration in internal/bootstrap/bootstrap.go; initialize qualified default policy only once. (FR-001, FR-005, FR-015, FR-016)
- [X] T037 [US2] Adapt atomic toggle and selected-plugin removal to composed addresses and exact dependencies in internal/pluginadmin/enable.go and internal/pluginadmin/uninstall.go, preserving source/siblings, Works, and memory. (FR-004, FR-011, FR-013)
- [X] T038 [US2] Implement the intended qualified plugin add/list/remove commands and one inventory envelope in internal/cli/plugin_add.go, internal/cli/plugin_list.go and internal/cli/plugin_remove.go; every row has marketplace provenance. (FR-004, FR-020, FR-023)
- [X] T039 [US2] Remove retired install/uninstall registration, old flags, alias inference, and direct-source UI callback entry points in internal/cli/plugin.go, internal/cli/plugin_install.go, internal/cli/plugin_uninstall.go, and internal/cli/plugin_hub.go; fail old syntax before retrieval/mutation. (FR-001, FR-004, FR-023)
- [X] T040 [US2] Adapt existing enable/disable/update selectors and manual component/convention selection to composed references in internal/cli/plugin_enable.go, internal/cli/plugin_disable.go, internal/cli/plugin_shared.go, and internal/cli/context_shared.go. (FR-004, FR-006, FR-023)
- [X] T041 [US2] Prove selected-only Git/local/builtin copies, homonym execution/conventions/private scope, independent source disappearance, exclusive fallback and invalid path behavior, and CLI/domain bypass rejection in tests/integration/marketplace_install_test.go and tests/contract/provider_identity_test.go. (FR-001, FR-004, FR-005, FR-006, FR-007, FR-015, FR-025, FR-026)

## Milestone M3 — Maintenance and default lifecycle

**Scope/dependencies:** US3 + US4; depends on M1–M2.

**Exit gate:** Grouped frozen candidates, exact reinstalls/removal authorization, current interruption recovery and purposeful policy-handled/default removal behavior.

### Phase 5 — US3: Maintenance and whole-source removal

**Goal:** Keep discovery separate from candidates and publish exact reviewed source/package removal with coherent recovery.

**Independent validation:** Refresh/check/reinstall/update changed and withdrawn entries; remove one source with enabled/disabled/withdrawn dependencies while injecting failure and concurrent drift.

- [X] T042 [P] [US3] Add grouped source/branch acquisition, frozen candidate, equal/changed opaque version, invalid equal-version, missing registration/discovery, path-move and withdrawal cases in internal/pluginadmin/marketplace_update_test.go. (FR-007, FR-008, FR-009, FR-010, FR-014)
- [X] T043 [P] [US3] Add exact whole-source target/dependency set, offline remove, refused authorization, namespace reuse, and concurrent target drift cases in internal/marketplaceadmin/remove_test.go. (FR-011, FR-012, FR-013)
- [X] T044 [P] [US3] Add every current catalog/config/package image/move/commit checkpoint, active invocation/lease, rollback, committed recovery and cleanup failure scenario in tests/integration/marketplace_recovery_test.go. (FR-012, FR-013, FR-014, FR-024)
- [X] T045 [US3] Resolve one complete current source snapshot per recorded source/branch group for check/update in internal/pluginadmin/update.go and internal/marketplace/candidate.go, without publishing discovery or using stale name-only provenance. (FR-007, FR-008, FR-010, FR-014)
- [X] T046 [US3] Freeze validated candidate content, version, declaration/dependency effects and current baseline through exact apply in internal/pluginadmin/update.go; preserve per-target results and aggregate error precedence without refetch. (FR-008, FR-010, FR-013)
- [X] T047 [US3] Implement same-version explicit reinstall/no-op with retained enablement and exact replacement authorization in internal/pluginadmin/install.go and internal/cli/plugin_add.go. (FR-009, FR-010, FR-013)
- [X] T048 [US3] Prepare whole-source removal from complete local inventory and exact source/catalog/dependency facts in internal/marketplaceadmin/remove.go, including disabled and withdrawn targets and domain-level builtin rejection. (FR-011, FR-012, FR-013)
- [X] T049 [US3] Apply source/config/catalog/owned-copy/registry and explicitly authorized policy removal as one reviewed transaction in internal/marketplaceadmin/remove.go and internal/pluginadmin/uninstall.go; preserve Works/memory and namespace continuity rules. (FR-011, FR-012, FR-013, FR-019)
- [X] T050 [US3] Add source remove CLI review/result and independent dependency authorization in internal/cli/plugin_marketplace.go; emit exact removed IDs, committed/cleanup state, and safe errors. (FR-012, FR-013, FR-023, FR-026)
- [X] T051 [US3] Implement the intended qualified interactive/explicit/all check-update target rules and safe per-target/source outcomes in internal/cli/plugin_update.go and internal/cli/admin_shared.go; remove source-managed and previous-origin mode branches. (FR-007, FR-008, FR-010, FR-023)
- [X] T052 [US3] Verify refresh/check purity, one source revision per batch, exact reviewed apply, same-version reinstall, withdrawal, offline atomic source removal, dependency refusal and batch error precedence in tests/integration/marketplace_maintenance_test.go. (FR-003, FR-008, FR-009, FR-010, FR-012, FR-013, FR-014)
- [X] T053 [US3] Exercise concurrent refresh/install/remove, source force-push, capture/stage lease lifetime, running package removal, and unaffected change merging in tests/integration/marketplace_concurrency_test.go; use deterministic synchronization and relevant race detection. (FR-013, FR-014, FR-024)

### Phase 6 — US4: Purposeful first use and default lifecycle

**Goal:** Install fresh defaults through the common pipeline and preserve current explicit policy, disablement and removal choices.

**Independent validation:** Fresh disposable homes cover registration-before-start, explicit empty policy, default add/disable/remove, newer offers and current journal recovery.

- [X] T054 [P] [US4] Add the current lifecycle matrix from contracts/bootstrap-and-publication.md in tests/integration/marketplace_bootstrap_test.go, covering registration-first, explicit empty policy, explicit add, disabled/removed defaults, changed offers and invalid current state. (FR-015, FR-016, FR-018, FR-024)
- [X] T055 [US4] Replace alias/embedded-digest inference with current initialized/policy_handled/removed lifecycle in internal/bootstrap/bootstrap.go and internal/adminstate/lifecycle.go; remove adoption, earlier-installation retention and implicit content replacement. (FR-015, FR-016, FR-018, FR-024)
- [X] T056 [US4] Mark every explicit policy choice, including empty, handled in the same transaction in internal/repoconfig/policy.go; source registration leaves it false, fresh required setup seeds only while unhandled, and explicit reference add handles without insertion. (FR-013, FR-016, FR-018)
- [X] T057 [US4] Verify common-pipeline fresh setup, one-time policy, no resurrection/content replacement and current source-instance ownership using tests/integration/marketplace_bootstrap_test.go and tests/integration/admin_seed_test.go; delete superseded default-adoption tests. (FR-015, FR-016, FR-018, FR-019, FR-024)

## Milestone M4 — Complete interface and delivery evidence

**Scope/dependencies:** US5 + current documentation/publication/platform evidence; depends on M1–M3.

**Exit gate:** Every P01–P12/S01–S12 case verified, current-only code/tests/docs, actual official publication and native platform outcomes recorded.

### Phase 7 — US5: Complete terminal manager

**Goal:** Expose every direct administrative capability through the current premium persistent terminal experience.

**Independent validation:** Execute P01–P12 with equivalent effective state/authorization and cover S01–S12 at normal/reduced sizes, no-color, resize, Back/cancel, and stale async results.

- [X] T058 [P] [US5] Add manager collection/filter/selection/back-stack and S01–S12 geometry/theme/error cases in internal/present/marketplace_catalog_test.go, preserving text focus and destructive semantics without color. (FR-020, FR-021, FR-022)
- [X] T059 [P] [US5] Add typed callback boundaries, immutable review cleanup, distinct dependency authorization, pure checks, stale async completion, and safe-source tests in tests/contract/marketplace_hubs_test.go. (FR-013, FR-021, FR-022, FR-026)
- [X] T060 [US5] Extend domain-free collection/navigation/selection-set models in internal/present/catalog.go to Installed/Marketplaces, nested one/all-source entries, installed source filter, and selected/all actions without adding one tab per source. (FR-021, FR-022)
- [X] T061 [US5] Render display-name/visible-namespace source rows, complete labeled totals, source-kind labels, installed/available details, and missing/empty/error states with existing frame/theme primitives in internal/present/catalog.go and internal/present/frame.go. (FR-020, FR-022, FR-026)
- [X] T062 [US5] Provide Git/local visible-editable registration and optional Git branch, source details/catalog navigation, and explicit upgrade/all-upgrade callbacks in internal/cli/plugin_hub.go and internal/cli/plugin_marketplace.go. (FR-021, FR-022)
- [X] T063 [US5] Compose exact add/reinstall and selected/all check-update review callbacks in internal/cli/plugin_hub.go using shared frozen plans; keep check distinct from apply and preserve dependency authorization. (FR-009, FR-013, FR-021, FR-022)
- [X] T064 [US5] Compose selected plugin and whole-source removal reviews in internal/cli/plugin_hub.go, including disabled/withdrawn targets, exact authorization and builtin restrictions; remove obsolete installation/migration callbacks. (FR-011, FR-012, FR-013, FR-021, FR-022)
- [X] T065 [US5] Implement per-target progress/results, permitted pre-commit cancellation, candidate Close on all exit paths, valid filter/selection retention, and current-feedback replacement in internal/present/catalog.go and internal/cli/plugin_hub.go. (FR-013, FR-021, FR-022)
- [X] T066 [US5] Verify persistent session lifecycle, exactly one terminal restore and Changes saved successfully. acknowledgment, no per-action transcript, and unchanged other journeys in tests/integration/marketplace_hubs_pty_test.go. (FR-021, FR-022, FR-023)
- [X] T067 [US5] Exercise P01–P12 against direct counterparts, including source branch, one/all-source catalogs, multi-target enable/check/update/remove, update all, upgrade all, and same-version reinstall in tests/integration/marketplace_parity_pty_test.go. (FR-020, FR-021, FR-022)
- [X] T068 [US5] Verify S01–S12 at 100x30/60x18 and resize down/up with long Unicode names, no-color and dumb terminals, source failure, missing catalog/runtime, dependencies, recovery/cleanup, and Back in tests/integration/marketplace_geometry_pty_test.go. (FR-020, FR-022, FR-026)

### Phase 8 — Documentation, publication dependency, and release evidence

**Goal:** Complete active documentation/fixtures and collect real platform and official-publication evidence before public delivery.

**Independent validation:** The full distribution contract is documented and verified across native CI; no unknown official URL or partial milestone is represented as a released capability.

- [X] T069 Update active user/author documentation in README.md, docs/marketplaces.md and docs/roadmap.md for one marketplace-only contract, qualified add/remove, separate refresh/update and purposeful default lifecycle; remove compatibility/transition guides. (FR-001, FR-003, FR-004, FR-018, FR-019, FR-023)
- [X] T070 [P] Create runnable one/multiple-plugin author examples in examples/marketplaces/single/marketplace.json and examples/marketplaces/multiple/marketplace.json with contained sample packages; document new builtin packaging in seed/README.md. (FR-001, FR-005, FR-015, FR-025)
- [X] T071 Obtain the official publication origin from the project owner before public delivery; record the confirmed normalized source and attribution rules in docs/marketplaces.md. The catalog is published and maintained only in that origin repository; this repository keeps no copy. Do not infer a URL or create/publish a remote without separate authorization. (FR-017, FR-023, FR-026)
- [X] T072 Verify an owner-authorized real initial work publication and a later independent catalog refresh/removal while preserving builtin; record exact origin/revision evidence in specs/009-marketplace-distribution/checklists/implementation.md. Keep the public release gate incomplete if origin/publication is unavailable. (FR-002, FR-003, FR-012, FR-017)
- [X] T073 Replace superseded direct-install/alias/link expectations with current marketplace fixtures in tests/integration/plugin_install.txtar, tests/integration/plugin_list.txtar, tests/integration/plugin_management.txtar and internal/cli/help_test.go; delete compatibility-only fixtures/tests rather than retaining old-behavior suites. (FR-001, FR-004, FR-023)
- [X] T074 Update full CLI/help/JSON/receipt/error-table and presentation-boundary contracts in tests/contract/plugin_administration_test.go, tests/contract/present_boundary_test.go, and internal/diag/diag_test.go; ensure stable stdout contains no interactive/Git output or secrets. (FR-021, FR-022, FR-023, FR-026)
- [X] T075 Wire current acceptance/path/schema/recovery cases, Unix PTY and relevant race jobs into .github/workflows/ci.yml; execute on Ubuntu/macOS/Windows without previous-executable, format-conversion or migration jobs. (FR-013, FR-014, FR-024, FR-025)
- [X] T076 Run the focused Q1–Q8 validations, relevant race detection, make seed-all, make lint, go test ./..., and host build; record actual outcomes/platforms and any unsupported runner checks in specs/009-marketplace-distribution/checklists/implementation.md. (FR-001, FR-006, FR-013, FR-014, FR-015, FR-018, FR-021, FR-022, FR-024, FR-025, FR-026)
- [X] T077 Reconcile actual evidence against all current outcomes in specs/009-marketplace-distribution/checklists/implementation.md; verify superseded publishers/provider variants/readers/adapters/compatibility-only tests are removed and require M1–M4 plus real official publication before release. (FR-017, FR-021, FR-022, FR-023, FR-024)

## Dependencies and implementation strategy

M1 setup → M1 foundations → US1 → M2 US2 → M3 US3/US4 → M4 US5 → final delivery gates. US3/US4 test authoring can overlap after M2; shared adminstate/bootstrap/pluginadmin mutations remain sequential. The manager composes domain operations, never replaces their validation.

US1 is the first discovery demonstration; M1+M2 adds controlled install/execution. Each story has its own validation above, while public release requires all four milestones. Existing development state is not a deliverable or test target.

### Parallel examples

- **US1:** T012, T013 are disjoint tests to author after the milestone prerequisites; ordered implementation follows.
- **US2:** T022, T023, T024, T025 are disjoint tests to author after the milestone prerequisites; ordered implementation follows.
- **US3:** T042, T043, T044 are disjoint tests to author after the milestone prerequisites; ordered implementation follows.
- **US4:** T054 are disjoint tests to author after the milestone prerequisites; ordered implementation follows.
- **US5:** T058, T059 are disjoint tests to author after the milestone prerequisites; ordered implementation follows.

## Task summary

| Group | Tasks |
| --- | ---: |
| M1 | 21 |
| M2 | 20 |
| M3 | 16 |
| M4 | 20 |
| Total | 77 |

| Story | Tasks |
| --- | ---: |
| US1 | 10 |
| US2 | 20 |
| US3 | 12 |
| US4 | 4 |
| US5 | 11 |
| Shared | 20 |

## Requirement coverage

| Requirement | Task IDs |
| --- | --- |
| FR-001 | T001, T002, T012, T014, T021, T022, T026, T027, T036, T039, T041, T069, T070, T073, T076 |
| FR-002 | T007, T008, T012, T014, T016, T018, T019, T020, T021, T072 |
| FR-003 | T013, T015, T016, T017, T018, T019, T021, T052, T069, T072 |
| FR-004 | T002, T003, T006, T022, T023, T028, T037, T038, T039, T040, T041, T069, T073 |
| FR-005 | T015, T017, T022, T026, T027, T036, T041, T070 |
| FR-006 | T003, T006, T007, T023, T024, T025, T028, T029, T030, T031, T032, T033, T034, T035, T040, T041, T076 |
| FR-007 | T013, T016, T022, T026, T028, T041, T042, T045, T051 |
| FR-008 | T042, T045, T046, T051, T052 |
| FR-009 | T042, T047, T052, T063 |
| FR-010 | T042, T045, T046, T047, T051, T052 |
| FR-011 | T037, T043, T048, T049, T064 |
| FR-012 | T043, T044, T048, T049, T050, T052, T064, T072 |
| FR-013 | T004, T005, T008, T009, T010, T018, T027, T035, T037, T043, T044, T046, T047, T048, T049, T050, T052, T053, T056, T059, T063, T064, T065, T075, T076 |
| FR-014 | T004, T005, T008, T009, T010, T011, T013, T017, T035, T042, T044, T045, T052, T053, T075, T076 |
| FR-015 | T018, T019, T021, T036, T041, T054, T055, T057, T070, T076 |
| FR-016 | T036, T054, T055, T056, T057 |
| FR-017 | T021, T071, T072, T077 |
| FR-018 | T054, T055, T056, T057, T069, T076 |
| FR-019 | T007, T025, T029, T030, T032, T049, T057, T069 |
| FR-020 | T019, T020, T021, T038, T058, T061, T067, T068 |
| FR-021 | T058, T059, T060, T062, T063, T064, T065, T066, T067, T074, T076, T077 |
| FR-022 | T058, T059, T060, T061, T062, T063, T064, T065, T066, T067, T068, T074, T076, T077 |
| FR-023 | T020, T021, T038, T039, T040, T050, T051, T066, T069, T071, T073, T074, T077 |
| FR-024 | T001, T003, T004, T005, T007, T008, T009, T011, T044, T053, T054, T055, T057, T075, T076, T077 |
| FR-025 | T002, T003, T006, T012, T013, T014, T015, T017, T022, T024, T026, T031, T034, T041, T070, T075, T076 |
| FR-026 | T019, T020, T024, T031, T032, T033, T041, T050, T059, T061, T068, T071, T074, T076 |

## Success-outcome verification

| Outcome | Current-product evidence |
| --- | --- |
| SC-001 | US1 strict sources and US2 selected copies; Q1/Q2 |
| SC-002 | US2 provider/convention/private/guard integration; Q2 |
| SC-003 | Current pure reads/checks, offline offers and no-op bytes; Q1/Q3 |
| SC-004 | Current finite recovery/checkpoints/leases and exact removal; Q4 |
| SC-005 | US5 P01–P12 state/authorization parity; Q6 |
| SC-006 | US4 fresh/registration-first/explicit-empty/disabled/removed/changed-offer lifecycle; Q5 |
| SC-007 | US5 S01–S12 viewport/no-color/session restoration; Q6 |
| SC-008 | Native three-OS current-format/recovery/race/PTY evidence; Q8 |

## Delivery dependency

The owner supplies the actual official publication origin and authorizes its real publication before public delivery. Keep this gate incomplete until evidenced; local fixtures do not substitute for it. This does not block completing the present specification/design/tasks request. No remote action is performed now.
