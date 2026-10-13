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
