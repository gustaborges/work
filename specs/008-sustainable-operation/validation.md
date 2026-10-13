# Validation: M1–M3 MVP

Validation date: 2026-10-12. Requested scope: M1–M3, T001–T037. Later milestones remain pending in `tasks.md`; this record does not certify the full F7 release candidate.

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

Full Q1–Q13 acceptance remains work for later milestones. In particular, disabled-policy add/replace validation and complete root semantics (M4), update/check (M5), uninstall and explicit embedded reinstall (M6), complete administrative hubs and unavailable-convention presentation (M7), and release-candidate acceptance (M8) are outside this delivery. Existing install/bootstrap provide the embedded inventory fixture for the MVP; `plugin install --embedded` is not implemented here.


## M4 — Explicit repository resolution

T038–T047 completed on 2026-10-12. Raw disabled Locators can be added or replaced; replacement retains explicitly named unavailable references and rejects canonical duplicates/installed-retained bare ambiguity before assignment. Root operations clone collections, reject empty/NUL operands and preserve the entire collection after late validation failures. Shared typed edits reload under administration, compare reviewed affected collections, and merge unrelated settings. Policy/Locator JSON adds installed/enabled/available facts while preserving envelopes, positions and reason precedence. No edit invokes providers.

Validation: focused repoconfig, Locator, CLI and contract suites passed; `TestAdminPolicyAndRoots` and repository policy/root/locator testscript cases passed; `make lint` passed. Q3–Q4 direct acceptance covers disabled inclusion, explicit reorder/replacement/removal, duplicate/anchor rejection, install/reinstall without membership insertion, root deduplication/empty removal, and twenty repeated byte-preserving reads. Existing Locator tests plus the new guarded traversal test cover skip/continue/stop outcomes. Reviewed collection conflicts and unrelated workspace preservation pass. Hub parity remains scheduled for M7; macOS/Windows runtime remains unexecuted.
