# Validation: M1–M5

Validation date: 2026-10-12. Original scope: M1–M3, T001–T037; extended in this session to M4–M5, T038–T061. M6–M8 remain pending in `tasks.md`; this record does not certify the full F7 release candidate.

## Delivered behavior

- Native offline fixtures cover local pinned, linked and `file://` remote pinned installation, spaces/Unicode paths, opaque versions, all four executable roles, and pipe-based subprocess synchronization.
- Administrative inspection reads coherent stable epochs without creating layouts, locks, databases, bootstrap state or recovery writes. Legacy seed ownership and installed metadata are projected without backfill; damaged content and unknown metadata remain distinguishable.
- Install, seed bootstrap, policy/root edits, convention-memory changes and workspace configuration participate in one administrative lock and finite before/after journal. Recovery restores prepared transactions, finishes committed transactions, and treats stable leftover evidence as cleanup-only.
- Managed component selection carries its registered package revision. Invocation holds administration through subprocess completion, releases it before Work publication, and rechecks providers with pure reads under Work locks. Manual unavailability retains code 39; automatic extension failures retain their existing warning/committed-Work behavior.
- `plugin list` retains its top-level JSON array and existing fields, with additive enablement, nullable version/digest and inspection status. Exact-alias `plugin enable|disable` validates the entire deduplicated set and commits atomically. Satisfied requests preserve bytes and epoch. Policy traversal order, convention memory, installed content and Work history survive toggles.
- Locator inventory and policy distinguish missing packages, missing components, changed roles and disabled packages without dropping configured references. Shared conventions remain effective while any owning package is enabled.

## Automated evidence

| Gate | Result |
| --- | --- |
| `GOCACHE=/tmp/work-go-cache make seed` | PASS; host assets built before host tests. |
| `GOCACHE=/tmp/work-go-cache make seed-all` | PASS; Linux amd64/arm64, macOS amd64/arm64 and Windows amd64 seed assets cross-built. |
| `GOCACHE=/tmp/work-go-cache go test -race ./internal/adminstate ./internal/pluginadmin` | PASS; transaction, recovery, invocation and concurrent enablement service tests. |
| `GOCACHE=/tmp/work-go-cache XDG_CACHE_HOME=/tmp/work-xdg-cache make lint` | PASS; gofmt, vet and staticcheck. |
| `GOCACHE=/tmp/work-go-cache GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test ./...` | PASS; full unit, contract, native integration, testscript and existing terminal/regression suites. |
| `GOCACHE=/tmp/work-go-cache make build` | PASS; host CLI built and `bin/work plugin --help` exposes install/list/enable/disable. |

Go tests use `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` so disposable Git fixture commits do not inherit the developer's commit-signing requirements. Build/staticcheck caches use writable temporary directories. Production Git signing remains enabled.

Q1–Q3 **MVP portions** are covered by `admin_purity_test.go`, `admin_enable_test.go`, `plugin_management.txtar`, registry/pluginadmin tests and the retained Locator/policy regression suites: fresh/legacy/damaged/linked-drift inventory, 20 repeated pure reads, explicit policy inclusion versus outside-policy installation, preserved membership/order, idempotent/duplicate aliases, invalid-batch atomicity, disabled execution across all roles and shared convention ownership. Offline remote installation pins a local Git commit and never inserts policy.

Foundation checks additionally cover coherent-read retry bounds and intermediate load errors; real child-process interruption at journal preparation, pending publication, every package move, all four administrative JSON images, commit, stable publication and cleanup; failed compensation and unsafe/missing evidence; no stale-journal overwrite; legacy seed adoption, disablement, repair and edited/empty policy; invocation contention/cancellation, revision drift and post-run publication checks. Contract tests audit administrative writer participation and presentation boundaries.

## Scope and platform limits

Runtime verification was performed on Linux amd64 with Go 1.26.4 (module minimum 1.26.0). Seed cross-compilation passed for all five release targets; macOS/Windows runtime and the remote CI matrix have not run in this session. Windows symlink coverage explicitly checks capability before proceeding.

This section records the M3 delivery boundary; M4–M6 evidence follows below. Full Q1–Q13 release acceptance, complete administrative hubs and unavailable-convention presentation remain scheduled for M7–M8. Explicit embedded reinstall is implemented in M6.

## M4 — Explicit repository resolution

T038–T047 completed on 2026-10-12. Raw disabled Locators can be added or replaced; replacement retains explicitly named unavailable references and rejects canonical duplicates/installed-retained bare ambiguity before assignment. Root operations clone collections, reject empty/NUL operands and preserve the entire collection after late validation failures. Shared typed edits reload under administration, compare reviewed affected collections, and merge unrelated settings. Policy/Locator JSON adds installed/enabled/available facts while preserving envelopes, positions and reason precedence. No edit invokes providers.

Validation: focused repoconfig, Locator, CLI and contract suites passed; `TestAdminPolicyAndRoots` and repository policy/root/locator testscript cases passed; `make lint` passed. Q3–Q4 direct acceptance covers disabled inclusion, explicit reorder/replacement/removal, duplicate/anchor rejection, install/reinstall without membership insertion, root deduplication/empty removal, and twenty repeated byte-preserving reads. Existing Locator tests plus the new guarded traversal test cover skip/continue/stop outcomes. Reviewed collection conflicts and unrelated workspace preservation pass. Hub parity remains scheduled for M7; macOS/Windows runtime remains unexecuted.

## M5 — Deliberate updates

T048–T061 completed on 2026-10-12. `work plugin update --check [PLUGIN...] [--json]` is pure for application state; targeted/`--all` application prepares each candidate once, reviews fixed bytes, and commits independently through the existing journal. Linked and embedded origins report source-managed/not-applicable before baseline checks. Unknown pinned baselines require explicit reinstall. Versions compare by opaque inequality; invalid same-version manifests, runtime failures, namespace changes and unsafe content are rejected.

The determined review includes versions, references, digests, added/withdrawn/modified declarations and exact ordered policy withdrawals. Separate dependency approval never expands to newly discovered references. Target revision/enablement, declarations, provider effects and dependencies are revalidated under the write lock; unrelated current settings merge. Independent failures retain exhaustive sorted outcomes, while cancellation and recovery/cleanup errors stop remaining targets with not-applied outcomes. JSON remains valid on aggregate exit 50; the command boundary emits one terminal diagnostic. Candidate ownership closes temporary copies exactly once. Existing safe source/error projections are reused and covered with URL credential/query/fragment and control-text assertions.

| Acceptance / evidence | Result |
| --- | --- |
| Q5 fixed local copy and file:// remote HEAD races | PASS; twenty pure checks per origin, then source/HEAD changes after preparation; installed version/reference/digest match the reviewed candidate and retained disablement. |
| Q5 opaque versions, legacy baseline, all origins and invalid candidates | PASS; service tests cover smaller differing versions, same-version changed code, invalid same-version/name/runtime candidates, shared owners, and closed candidate cleanup. Existing prepared-install path/symlink tests remain applicable. |
| Q6 grammar, authorization and batch outcomes | PASS; CLI/contract/native tests cover flag exclusions, unknown aliases before retrieval, empty check, no confirmation/no dependency authorization before changes, independent valid/invalid candidates and one final error. |
| Q6 interactive selection/review | PASS on Unix PTY; explicit aliases skip selection, untargeted selection precedes review, source drift after review applies frozen bytes, and refusal preserves the home. |
| Q8 update rollback/restart | PASS; injected failures and real subprocess exit at prepared, pending, backup/publish moves, config/registry saves, before-commit, committed, stable and cleanup. Precommit recovery restores exact registry/config/content; committed recovery retains the update; memory remains byte-identical and owned stages/backups are collected. |
| Q10 reviewed drift and batch integrity | PASS; target enablement, check-baseline and dependency drift reject stale reviews; unrelated roots merge; shared declaration updates account for the batch's own changes without admitting unrelated provider drift. Cancellation preserves earlier commits; unresolved recovery and committed cleanup stop remaining targets. |
| Q12 source/output safety and read purity | PASS; credentials are absent from list/check/apply/install-failure output; twenty fresh/unchanged/newer/invalid/inaccessible checks preserve application trees and OS-temp inventories, with no component invocation. A call-site boundary test prevents update retrieval from ordinary Work/inspection commands. |

Final verification:

- `GOCACHE=/tmp/work-go-cache GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test ./...`: PASS, including full integration/testscript/PTY regression (208.270 seconds for the final full integration run).
- After final convention-impact review refinements, focused `TestAdminUpdate|TestAdminBatch|TestAdminCredentials` suites: PASS across pluginadmin, CLI and native/PTY integration (32.722 seconds for integration). Combined convention withdrawal previews and shared-owner application pass.
- `GOCACHE=/tmp/work-go-cache XDG_CACHE_HOME=/tmp/work-xdg-cache make lint`: PASS after final changes (formatting, vet, staticcheck).
- Race checks: PASS for adminstate/pluginadmin/repoconfig; final pluginadmin race suite also PASS after review refinements.
- `GOCACHE=/tmp/work-go-cache make build`: PASS, including host seed generation. Final `go build -trimpath -o bin/work ./cmd/work` and `bin/work plugin update --help`: PASS with all four update flags exposed.

 Runtime evidence remains Linux amd64 only; no remote CI or macOS/Windows runtime is claimed. M6–M8 are intentionally left unchecked. No before/after implementation extension hooks are registered.


## M6 — Dependency-aware removal and offline bundled reinstall

T062–T072 completed on 2026-10-12. `work plugin uninstall <PLUGIN...>` resolves exact aliases, deduplicates/sorts the complete set and reviews every alias-owned policy reference, including stale declarations. `--yes` confirms removal independently of `--remove-policy-references`; interactive dependency refusal and final refusal leave the home unchanged. Sorted receipts appear only after whole-set commitment, including an explicit committed-cleanup outcome.

Removal uses the shared journal with explicit removal moves and retained owned backups. Reviewed package registrations/enablement, owned-root digests, exact policy impact, convention-provider facts and seed lifecycle are rechecked against latest locked state. Unrelated current config merges, retained policy entries preserve relative order, shared convention ownership survives, and repository convention memory is never deleted. Wrapper source symlinks and legacy alias-root symlinks are unlinked without traversing their external sources. Missing owned roots can still be unregistered safely.

Bundled removal publishes lifecycle `removed=true` in the same transaction as content/registry/policy removal. Ordinary bootstrap/start respects it. `work plugin install --embedded` restores the bundled package offline through the already-locked seed path, rejects SOURCE/--link/--as, retains existing disablement on reinstall, marks initialization handled, and never inserts policy even on a fresh home. Existing fallback ownership is validated before staging. Missing Starter/Locator/convention diagnostics give explicit install/enable/configure guidance.

| Acceptance / evidence | Result |
| --- | --- |
| Q7 complete-set validation, dependencies and preservation | PASS; `TestAdminUninstallWholeSetAndSharedOwners`, contract diagnostics and native `TestAdminUninstallLinkedAndWorkRetention` cover unknown aliases before changes, exact stale/current dependencies, duplicate sorting, disabled removal, unchanged linked sources, existing Work trees and convention-memory bytes. |
| Q7 linked ownership shapes | PASS; native wrapper-directory/source-link and legacy alias-root-symlink removal preserve external source trees. Windows cases check symlink capability explicitly. |
| Q8 recoverable failures and cancellation | PASS; service checkpoints at prepared/pending, each of two package backup/removal moves, config/registry saves and before-commit restore the entire set. Cancellation preserves absent roots and all registrations; no partial receipts appear. |
| Q8 process interruption and committed cleanup | PASS; real child-process exits at every removal boundary recover before-state before commitment and after-state after the commit marker. Committed/stable/cleanup errors return 55 with committed results; recovery cleans owned stages/backups and retains memory bytes. Unsafe removal journals fail closed. |
| Q8 reachable direct recovery | PASS; `TestAdminUninstallNativeRecovery` proves pure reads and invalid grammar do not recover pending state; valid uninstall reaches preflight before alias lookup, reports prior recovery separately, and does not repeat receipts for an already committed set. |
| Q9 bundled history and offline reinstall | PASS; fresh explicit install without default policy, default bootstrap history, ownerless/install-meta legacy adoption, deliberately empty policy, disabled reinstall/repair, historical-release digest refresh, durable removed history and explicit reinstall. Seed lifecycle/config/registry publication failures compensate together. A committed seed removal remains absent after recovery and bootstrap. |
| Q10 reviewed facts and activation | PASS; dependency/provider/content/enablement drift requires a fresh review; unrelated roots merge; stale component selections cannot execute after uninstall. |
| Q12 interactive review/automation | PASS on Unix PTY; dependency refusal, final refusal and acceptance use shared services and print accepted receipts once. CLI grammar requires aliases/automation confirmation, rejects mutation JSON and invalid flags with usage/2, and exposes both uninstall flags in help. |

Final verification:

- Focused M6 service, CLI, contract, native and Unix PTY suites: PASS, including the final invalid-flag and direct-recovery checks.
- `GOCACHE=/tmp/work-go-cache GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test ./...`: PASS on the final tree; full integration/testscript/PTY regression completed in 237.025 seconds. Existing custom-fallback bootstrap journeys remain compatible; fallback-owner rejection applies to explicit embedded installation.
- `GOCACHE=/tmp/work-go-cache go test -race ./internal/adminstate ./internal/pluginadmin ./internal/bootstrap`: PASS.
- `GOCACHE=/tmp/work-go-cache XDG_CACHE_HOME=/tmp/work-xdg-cache make lint`: PASS after final production changes.
- `GOCACHE=/tmp/work-go-cache make build`: PASS; final host `go build -trimpath -o bin/work ./cmd/work` also PASS after the compatibility correction.
- Q9 host CLI journey in a disposable home: PASS for offline install, disabled reinstall, dependency refusal/removal, durable lifecycle and explicit reinstall without policy.

A historical embedded release is simulated with old registered version/install metadata to exercise the supported repair path; no separate newer release binary or remote CI was executed. Runtime evidence is Linux amd64 with Go 1.26.4 only. M7–M8 remain unchecked. No before/after implementation extension hooks are registered.

## M7 — Administrative hubs and convention availability

The three bare administration commands now open full-screen keyboard hubs with pure initial inspection, direct equivalents, explicit setup actions and refreshed state after accepted actions. Plugin install (source or bundled), enable/disable, check/apply and uninstall reuse the direct command handlers and their typed transaction services. Updates retain frozen candidates through review; dependency removal remains a separate explicit authorization. Reviews include exact safe automation equivalents and owned uninstall paths.

Repository navigation exposes installed/disabled Locators, ordered unavailable policy entries, all membership/order operations and all search-root operations. Replacements are assembled through ordered selections and validated directory fields, then previewed before the shared reviewed edit commits. Clearing the last entry has a concrete direct remove equivalent. Empty inventories provide guidance and Back, and redirected bare hubs return usage/2 without terminal controls. Explicit --help retains the complete direct command inventory.

Convention views preserve remembered values independently of availability, add available/unavailable_reason, and redact identity credentials without changing private matching keys. Shared enabled providers keep a choice available. The common setter compares the reviewed identity's memory and selected convention/provider revisions under the write lock, while merging unrelated memory. Start requires explicit replacement of unavailable memory even with one alternative, fails noninteractively with convention-unknown/38, retains fresh sole-choice defaults and contribution bypass, and rechecks convention facts before Work materialization. Existing Work state is unchanged.

| Acceptance / evidence | Result |
| --- | --- |
| Q6 plugin hub/direct behavior | PASS; Unix PTY journeys install from source and embedded content, correct an invalid alias, enable/disable, perform pure checks, apply reviewed updates, refuse dependencies and uninstall with exact approval. Direct idempotent equivalents preserve the resulting state. Existing fixed-candidate and direct dependency suites remain applicable. |
| Q11 availability and identity | PASS; unit projection checks cover no choice, unavailable memory and shared enabled owners. Native tests inspect the same remembered unavailable choice across two repositories with the same credential-bearing origin, twenty reads per clone, without changing the home. |
| Q11 explicit replacement and drift | PASS; Unix PTY start asks for the sole available replacement, cancellation preserves memory/home, acceptance creates a Work with that choice, and provider drift after review fails with conflict/53. Setter tests reject memory/provider revision drift and retain unrelated upserts. Existing contribution/fresh-default journeys remain regression coverage. |
| Q12 repository and convention hubs | PASS; PTY journeys add disabled policy entries, move/reorder/remove the complete set, correct invalid directory input, add/replace/clear roots and replace remembered conventions. Direct equivalents produce the same effective collections; root operands with spaces, quotes and URL-like punctuation retain their exact text. |
| Q12 terminal/output safety | PASS; Back/Leave unchanged and empty navigation preserve fresh-home absence. Cancellation and resize cover 80×24 through 120×50 viewports with NO_COLOR and TERM=dumb, restore the primary buffer and render one final error. Corrected invalid fields do not remain in terminal history. Credential-bearing identity/source projections and equivalent commands omit credentials/query/fragment. |
| Q12 discovery and boundaries | PASS; direct command/flag inventory, updated help-only expectations, redirected-stream checks, pure convention show, and application-service/presentation import boundaries pass. |

Local runtime evidence is Linux amd64 with Go 1.26.4. No macOS/Windows runtime or remote CI result is claimed. M8 remains outside this delivery boundary. No before/after implementation extension hooks are registered.

Final verification:

- T073–T085 completed and verified; the requested next milestone stops at M7.
- `GOCACHE=/tmp/work-go-cache GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test ./...`: PASS; full integration/testscript/PTY regression completed in 246.407 seconds.
- After the final command-quoting refinement, focused convention, hub, equivalent-command and contract suites: PASS (12.489 seconds for integration).
- `GOCACHE=/tmp/work-go-cache GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test -race ./internal/repoconv ./internal/adminstate ./internal/pluginadmin ./internal/cli`: PASS. CLI race suite completed in 62.219 seconds; the final quoting refinement is covered by the focused suite above.
- `GOCACHE=/tmp/work-go-cache XDG_CACHE_HOME=/tmp/work-xdg-cache make lint`: PASS after final source changes (formatting, vet and staticcheck).
- `GOCACHE=/tmp/work-go-cache make build`: PASS, including host seed generation and the host CLI binary.
- The initial full run identified intentional help-only expectations plus a testscript matcher syntax issue; updated bare-hub/help/stream expectations and their focused rerun pass. No unresolved failures remain.

### M7 follow-up — Persistent hub navigation and read results

The reported list action reproduced a presentation defect: the direct read printed its result and immediately reopened the menu, covering it. Each successive hub prompt also started and stopped a separate full-screen program. Administration hubs now retain one terminal session across their prompts and service calls. Plugin lists/update checks, repository lists, and remembered-convention inspection remain in a scrollable result view until Back. Accepted output is visible within the session and published to its original streams after terminal restoration. Receipt history yields space to the active menu, and long result paths wrap without losing content.

PTY regression assertions now inspect the reconstructed screen as well as emitted text; the test terminal supports the renderer's scrolling regions and line insertion/deletion. Successful hub journeys assert exactly one alternate-screen entry and one restoration, covering source/alias/mode navigation, Back, mutations, and read views. Read navigation and installation Back preserve administrative state. Unit checks cover scrolling, resize, long paths, prompt transitions without an empty frame, receipt publication once, and cancellation between prompts.

Verification:

- Focused presentation, hub and terminal-emulator suites: PASS; integration completed in 9.481 seconds.
- `GOCACHE=/tmp/work-go-cache GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test ./...`: PASS; full integration/testscript/PTY regression completed in 226.937 seconds.
- Presentation and CLI race suites: PASS (1.853 and 64.607 seconds respectively).
- Actual hub process race coverage with `GOFLAGS=-race`: PASS; read views and source-install Back completed in 21.236 seconds.
- After the final long-path wrapping refinement, focused result/viewer tests: PASS (2.021 seconds for integration), presentation race suite: PASS, and final `make lint`: PASS.
- `GOCACHE=/tmp/work-go-cache make build`: PASS, including host seed generation and CLI compilation.

Evidence remains local Linux amd64. M8 remains unchecked; no implementation extension hooks are registered.

## M7 correction — Installed plugin manager (2026-10-13)

The user correction in spec FR-035–FR-039 / SC-009 supersedes the original plugin action-interview and receipt-history evidence above. The manager opens on Installed (count), with only Official and Install plugin alongside it. Installed rows show name and Enabled/Disabled, with version/source metadata in details. Search, scrolling and Back preserve navigation; Space changes package enablement in place. The Official category contains the known bundled work-reference offering without a marketplace registration/discovery service.

A dedicated `present.Catalog` model handles inventory, structured details, installation fields and reviews in one Bubble Tea lifecycle. Typed CLI callbacks use adminstate/pluginadmin/plugininstall/bootstrap services directly. Installation publication is shared with direct commands through `pluginadmin.InstallPrepared` / `InstallEmbedded`; direct outputs are unchanged. Pure review preparation never creates a fresh Work home; accepted application performs recovery preflight and shared transactional validation. Frozen updates survive a source change after review, and dependency removal requires its own checkbox authorization before final acceptance.

Success replaces the current status; failed/declined/cancelled actions keep the manager open. Esc returns/clears search/closes; q and Ctrl+C at the inventory also succeed normally. Primary-buffer output after normal restoration contains only `Changes saved successfully.` once. No receipt dump, equivalent-command trail or cancellation diagnostic is produced. Start, repository and convention still use their existing paths and feedback.

| Requirement / acceptance | Evidence |
| --- | --- |
| FR-035; US5.7 | Native PTY assertions inspect the reconstructed inventory for name and state, and absence of version/origin metadata. Installed/Official/Install plugin navigation is exercised on fresh and populated homes. |
| FR-036; US5.8/US5.12 | Model tests cover case-insensitive search, long Unicode names, long details, visible actions, editable long fields, 48×16 / 80×24 / 120×50 rendering and scroll bounds. Native PTY tests cover a 28-plugin inventory, repeated details/Back, search, resize and NO_COLOR/TERM=dumb. |
| FR-037; US5.9 | Native PTY journeys install local pinned and file:// remote-pinned sources, correct an invalid alias, and explicitly install the bundled package through Official. Back and failed preparation preserve fresh-home absence. |
| FR-038; US5.10 | Native PTY update review freezes a release, changes the source, applies the reviewed release and continues to uninstall. Separate reference authorization is refused, then approved. Source errors and abandoned reviews remain inside the usable manager. Model tests verify review-resource cleanup and authorization gating. |
| FR-039; US5.11 | Native PTY normal exits via Esc/q/Ctrl+C assert exactly one alternate-screen entry/restoration, one acknowledgment and no post-restoration transcript/cancellation diagnostic. Manager installation Back and read navigation preserve state. Existing Start/repository/convention coverage remains in the regression suite. |

Focused model/CLI/contract/PTY checks passed; final focused integration runtime was 12.439 seconds. Terminal emulation gained ICH/DCH/ECH assertions so renderer character edits are represented correctly. The static update-retrieval boundary now permits only the explicit `prepareManagerUpdate` callback in addition to the direct update handler; initial inventory/navigation remain pure.

Final verification (requested `GOCACHE=/tmp/work-go-cache` and `XDG_CACHE_HOME=/tmp/work-xdg-cache`; Git configuration isolated for tests):

- `make seed`: PASS; host fixtures present before tests.
- Focused model/contract/native-PTY checks: PASS; final integration run 12.439 seconds.
- `go test ./...`: PASS; final full integration/testscript/PTY run 238.460 seconds.
- `go test -race ./internal/present ./internal/cli ./internal/pluginadmin`: PASS (presentation 2.021, CLI 73.624, pluginadmin 45.711 seconds). Final presentation refinements additionally pass `go test -race ./internal/present` (1.908 seconds).
- Final actual manager processes with `GOFLAGS=-race`, covering all plugin hub/manager and install-Back tests: PASS; 111.657 seconds.
- Final `make lint`: PASS (formatting, vet, staticcheck).
- `make build`: PASS; host seed and `bin/work` rebuilt.
- `git diff --check`: PASS.

The first full run exposed the old file-only update-discovery whitelist; its callback-specific correction and final full rerun pass. ICH/DCH/ECH emulation fixes and focus-aware viewport assertions distinguish actual renderer output from stale historical text. No unresolved test failures remain.

T092–T097 are complete. Evidence is local Linux amd64 with Go 1.26.4; macOS/Windows runtime and remote CI are unexecuted. M8 / T086–T091 remains pending.

### Source input visibility correction — T098

FR-037 and the hub contract now explicitly require visible editing of local paths and git URLs. The manager source field no longer uses secret-input mode; its hint names these input forms. Existing Git source classification/transports and safe projections for reviews/details/errors remain unchanged.

Native rendered-screen tests enter a local path with spaces, a GitHub HTTPS URL, SCP-style SSH and ssh:// form without submitting retrieval. Each stays visible/editable; Back and normal exit leave the home unchanged. Focused visible-source/local-install/file:// remote-install/credential-output checks: PASS (4.765 seconds). `make build`: PASS; `bin/work` rebuilt. `git diff --check`: PASS. T098 complete; M8 remains pending.

### Install tab navigation correction — T099

FR-036 and the hub contract distinguish top-level navigation focus from form editing. Arrow arrival on Install plugin keeps tab navigation active, so left/right can cycle through every destination in both directions. Enter/Tab starts editing; ordinary text/paste also starts editing and i remains a direct form shortcut. Esc restores navigation focus without losing source/alias/mode values; another Esc from the inactive form returns to Installed. In-field arrows retain normal cursor/mode behavior.

Model tests and actual reconstructed PTY screens cover forward/backward wrap through Install, Enter/Tab editing, cursor movement, Esc and retained drafts, plus all plugin-manager operation/source/Back regressions. Focused model/native-PTY suite: PASS (integration 13.940 seconds). Presentation race suite: PASS (1.850 seconds). `make lint`, `make build`, and `git diff --check`: PASS. T099 complete; M8 remains pending.

Terminology correction: the source hint reads `Local path or git URL`; the spec, contract, plan and README use the same name without a GitHub-only implication. Existing source transports are unchanged. Visible-source PTY checks: PASS (1.763 seconds); `make build` and `git diff --check`: PASS.
