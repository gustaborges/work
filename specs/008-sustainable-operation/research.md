# Research: Sustainable Platform Operation (F7)

**Date**: 2026-10-12. **Status**: Complete. Research is based on the current repository; no implementation or application tests ran. Two research agents examined lifecycle/storage and administrative interfaces. All initial unknowns below are resolved.

## R1 — Preserve the existing stack and authority

**Decision**: Retain Go 1.26.0, Cobra 1.10.2, Bubble Tea 2.0.9/Lip Gloss 2.0.6 through `internal/present`, existing Git subprocesses and JSON storage. No new dependency, plugin role, IPC version, Work schema, or SQLite migration.

**Evidence/rationale**: `go.mod`, `Makefile`, `.github/workflows/ci.yml`, and F6 `plan.md` establish the stack/matrix. `.specify/memory/constitution.md` is unratified template text, so PRD/accepted ADRs/roadmap §4 govern. `setup-plan.sh --json` resolved feature paths correctly but returned an empty BRANCH; direct `git branch --show-current` confirms `feature/sustainable-operation`. Do not create/switch branches to compensate.

**Alternatives considered**: A new package manager, database-backed administrative state, or dependency upgrades add unrelated migration risk.

## R2 — Separate pure reads, mutation preparation, and commitment

**Decision**: Add `adminstate.Read` for coherent, non-writing config/registry/convention-memory/lifecycle snapshots. Missing files yield existing defaults. Read paths never call layout creation, bootstrap, recovery, writable projection, or lock creation. Use pure projections for inventories/hub initial inspection; mutations invoke preparation only after complete input validation.

**Evidence/rationale**: `loadPluginContext` creates layout; `loadRepositoryContext` and `loadConventionContext` bootstrap even for lists/show. In contrast, `config.Load`, `registry.Load`, `repoconv.Load`, and F6 `loadContextGlobals` are already pure. F7 must fix these call sites rather than weaken FR-028. Ordinary OS access metadata is outside application-state purity.

**Alternatives considered**: Running bootstrap before reads, or promising purity only for selected files, fails fresh-home and disabled/removed-default behavior.

## R3 — One administrative write lock and a bounded journal

**Decision**: Add low-level `internal/adminstate` with a shared `administration` lock and a versioned journal for package moves plus the finite administrative JSON files. Keep source retrieval/user input outside the lock. Under lock: recover, reload latest state, check reviewed assumptions, prepare before/after images and package backups, publish pending state, apply, mark committed, publish stable state, then clean backups. All config/registry/convention-memory/lifecycle writers participate, including install/bootstrap, start first-run setup and convention memoization, `workspace` setup, and direct/hub mutations.

**Evidence/rationale**: `plugininstall.Install` calls `StageThenSwap` before CLI `registry.Save`; the swap deletes the backup too early. Bootstrap writes content, registry and policy separately. `config.Save`/`repoconv.Save` make one-file replacement safe but cannot prevent stale read-modify-write. Existing `lockfile.AcquireContext` provides portable bounded acquisition; reuse its 30-second default. Split bootstrap into an external locking wrapper and an already-locked preparation helper; never reacquire the same lock.

**Alternatives considered**: Independent registry/config locks permit split commitment; a general transaction framework or changing all installed paths to immutable generations is unnecessary. Reuse low-level staging/copy/rename primitives but retain undo ownership until commitment, rather than use the existing early-cleanup `StageThenSwap` unchanged.

## R4 — Pure reads can detect concurrent or interrupted commitment

**Decision**: One atomically replaced `state/administration.json` carries monotonically increasing epoch, `stable|pending`, and transaction ID. Absence means stable epoch zero. Read it before and after loading all administrative data and any installed manifests used for legacy views. Accept only identical stable records; retry at most three times on a changing epoch, then fail `administration-conflict`. Even if an intermediate load fails, reread the control and retry drift before classifying a stable file error. Pending or malformed recovery data returns `administration-recovery-required`, never repairs. A writer publishes pending at n+1 before authoritative changes and stable at n+2 after coherent commit or rollback; epochs never reset after rollback.

**Rationale**: Checking journal presence alone misses a whole transaction that starts and finishes during a read. Two stable epoch observations detect that race without creating read locks. Prepared transaction directories are not authoritative until referenced by the pending record.

**Alternatives considered**: Creating shared read locks violates fresh-home purity; unbounded retries can starve; returning partial state while pending misreports usable installations.

## R5 — Recovery and commit boundary

**Decision**: A prepared journal has explicit existence bits, before/after JSON bytes/digests, owned package move paths, and phase `prepared|committed`. No external source paths are deletion targets. On restart under the write lock, prepared rolls back; committed rolls forward and cleans. Retain the journal until stable epoch publication succeeds. A failure before the committed marker restores the whole transaction, without using a cancelled context for compensation. After that marker, cancellation cannot undo success; cleanup failures report that commitment happened and preserve recovery evidence. Invalid/unsafe journals fail closed.

**Evidence/rationale**: Existing `atomicfile` flushes JSON and provides a Windows replacement helper; existing swap helpers support backup/restore but not multi-file recovery. [Go 1.26 os.Rename documentation](https://pkg.go.dev/os@go1.26.0#Rename) does not promise atomic rename on every platform. Use the existing atomic file replacement for journal/control records and test directory moves/recovery on the supported OS matrix. This design protects supported process interruption, not arbitrary power-loss/storage corruption.

**Alternatives considered**: Delete backups before registry publication; infer a transaction's phase solely from directory existence; claim whole-directory rename is universally atomic.

## R6 — Capture installed versions and source identity without read migration

**Decision**: Extend package records additively with `version`, `source`, and `content_digest`. Preserve the existing `reference` format: remote `<source>@<sha>`; local path for local copies/links. Store opaque manifest version and digest at install/update. Legacy reads may derive version from the installed managed manifest, never a newer pinned source; missing/unreadable content yields unknown version and an inspection reason. Linked versions remain the last registered baseline, or unknown for legacy records. First subsequent mutation can backfill safe legacy metadata under the write lock.

**Evidence/rationale**: Manifest `Version` is required, but `registry.Package` does not retain it. `existingIdentity` can recover remote source from the last `@`, and local references already identify origin paths. Local pinned references currently identify the source, not exact copied bytes, hence an additive digest is needed for reviewed-candidate/conflict checks. Reject an update that changes a known manifest plugin name: `PluginNameOf` governs private semantic namespace ownership.

**Alternatives considered**: Compare current source manifest as installed version; infer versions from commit chronology; rewrite reference strings or convention identity.

## R7 — Explicit update discovery and fixed candidate ownership

**Decision**: High-level `internal/pluginadmin` prepares update candidates using extracted install validators. Remote pinned sources clone their recorded source's default revision into process-owned temporary storage and record HEAD; local pinned sources copy into temporary storage before parse/validation/digest. Compare version strings for inequality only. Validate candidates even when versions match, but only a differing valid version is update-available. Keep prepared bytes until apply/cancel; never fetch again to apply the reviewed candidate. Check-only candidates always clean up and never persist a cache or fetch in an installed checkout.

**Evidence/rationale**: ADR-0005 uses opaque versions and explicit checks. `gitx.Clone` already returns HEAD. [Git clone documentation](https://git-scm.com/docs/git-clone#_description) describes the initial default-branch checkout; its [Git URL documentation](https://git-scm.com/docs/git-clone#_git_urls) also supports `file://` for offline remote-origin fixtures. Linked packages are source-managed and the embedded package is not-applicable, as specified; `--all` reports those outcomes. Unknown explicit targets fail globally before retrieval/apply. Check/apply errors are per-target outcomes plus one overall failure.

**Snapshot boundary**: Existing copyTree preserves symlinks. Pinned preparation therefore validates the frozen tree before manifest reads, allows only contained relative symlink chains, hashes link text, and rejects absolute/escaping/cyclic links and escaping entrypoints. Reuse this tightened rule for pinned install/update; --link remains developer-managed. Otherwise an external link could change the reviewed package after copying. Test a linked manifest/entrypoint and link-target mutation between review and apply.

**Alternatives considered**: Pull the installed checkout, automatic notifications, release-tag requirements, semantic version ordering, or refreshing developer-owned linked registrations implicitly.

## R8 — Explicit dependencies and batch semantics

**Decision**: Use `--remove-policy-references` for update/uninstall automation. Interactive impact approval authorizes exactly the displayed references. `--yes` confirms already determined work but never replaces dependency authorization or target selection. Update commits each package independently in deterministic alias order; uninstall commits the complete selected set together. Revalidate target revision/enablement and dependency impact under lock after review; changed reviewed facts return conflict. Unrelated changes are merged from the latest state.

**Evidence/rationale**: ADR-0015/PRD FR-48 and spec FR-017–FR-023 require explicit removal. Stale references owned by an uninstalled alias count even if the declaration no longer exists. Update impacts include a withdrawn name or role change. Only remove affected entries; preserve relative order of the remainder. No plugin executes for impact calculation.

**Alternatives considered**: `--yes` as blanket cascade authorization; full-batch update rollback; per-package uninstall leaving half the selected set removed.

## R9 — Durable seed lifecycle and legacy ownership

**Decision**: Retain lifecycle separately in `state/reference-package.json` with schema 1, initialized/policy-seeded facts and deliberate removal. Disabled state remains solely `Package.Enabled`. Bootstrap registers a proper embedded Package record with version/digest/convention ownership. Legacy ownerless seed declarations are projected as a virtual embedded package for reads and adopted under lock on mutation; legacy `install-meta.json.policy_seeded` or a complete existing seed registration proves prior initialization. Ambiguous historical state is reported rather than treated as a new setup. Explicit uninstall retains lifecycle outside the deleted package; later bootstrap never resurrects it. `work plugin install --embedded` explicitly reinstalls bundled content offline; alias remains reserved against unrelated sources, and explicit embedded install never inserts policy, including on a fresh home, and records initialization as handled.

**Evidence/rationale**: `bootstrap.registerComponents` does not create a Package; `PolicySeeded` lives in removable install-meta; current `EnsureSeed` otherwise repairs missing content unconditionally. F7 needs retained intent, not a mirror of whether live files happen to exist. `--embedded` is an additive selector under existing install, mutually exclusive with SOURCE/`--link`/`--as`.

**Alternatives considered**: Keep a tombstone inside a removed package; reserve bundled-package removal as unsupported; reuse disablement to mean uninstall; silently infer deliberate removal from accidental deletion.

**Reachable recovery**: After valid mutating grammar/required automation confirmation, run a short locked recovery preflight before semantic alias lookup or pure preparation. Then release for retrieval/review and reacquire for latest-state apply. This breaks the pending-snapshot/recovery circularity. Pure reads and bare hubs never recover; guidance names the interrupted direct mutation with required flags.

## R10 — Availability, policy validation, and convention memory

**Decision**: Use raw ownership for inventory and policy editing, effective catalogs for activation. Resolve new policy refs against installed Locators, including disabled packages. Replacement can retain an existing unavailable qualified ref, but rejects duplicate canonical refs. Remove/move remain policy-only. Availability reasons distinguish missing package/component, changed role and disablement. Show convention memory with availability; unavailable remembered values require explicit replacement even if only one available name remains.

**Evidence/rationale**: `repoconfig.ReplacePolicy` currently deduplicates and uses effective-only lookup; F7 FR-008 intentionally tightens this validation and expands unavailable-reference retention. Existing grammar/output keys remain; tests for previous duplicate collapsing must change deliberately. `RemoveRoots` should clone its input before filtering. `registry.Effective`/`RemoveAlias` already preserve shared convention ownership; keep their name/prefix semantics. `start.resolveConvention` currently accepts unavailable memory blindly; use latest effective catalog and serialize only the eventual memory upsert.

**Alternatives considered**: Remove unavailable references silently; add individual Locator enablement; automatically replace a convention with the remaining name; change shared-name identity in F7.

## R11 — Protect invocation against alias-path replacement

**Decision**: At each Starter/Locator/Linker/Importer invocation acquire the same administration lock, reload and revalidate selected package revision plus component/eligibility, execute, and release before Work-lock publication/incorporation. No input collection or source retrieval holds that lock. Manual association, which has no subprocess, uses a short administration-before-Work lock scope for final provider validation/publication. Under a Work lock, subsequent extension revalidation uses the pure epoch reader only and fails safely on pending/drift; it never acquires administration. Automatic reload closures must read current state, not return captured catalogs.

**Evidence/rationale**: `Component.Target` resolves mutable `<alias>/source` paths. `ImportOne` invokes before acquiring its Work lock, and Linker discovery precedes `PublishLink`; Starter/Locator invocation precedes Work creation. Component equality alone misses changed code with identical declarations. Existing admin acquisition timeout bounds contenders; no new plugin timeout is added. A long-running plugin can temporarily block maintenance, which reports contention rather than replacing its files on Windows.

**Alternatives considered**: Compare revision then unlock before launch (race); acquire admin under Work lock (inversion); hold admin across whole wizard (unnecessary contention); redesign installation into immutable revision directories (larger migration).

## R12 — Presentation, diagnostics, tests, and bounded delivery

**Decision**: Compose existing `present.Wizard` selectors/input/confirm for hubs; domain services return typed results and import no presentation. Keep stdout stable and UI on stderr. New errors start at code 50; preserve 0–49. Add safe source projections for text/JSON/receipts/errors, including credentials and query secrets. Use native offline fixture packages plus local Git remote sources, deterministic fault/checkpoint synchronization, testscript, existing PTY harness and contract boundary gates. No source tests/builds run in planning.

**Evidence/rationale**: Existing plugin/repository hubs are help-only stubs; convention hub and full-screen primitives exist. Existing `diag.All` exhaustively asserts categories. `plugin_list` currently prints raw references and install conflict diagnostics can print raw sources. `Makefile` and CI define seed-before-test and three-OS gates.

**Alternatives considered**: Duplicate hub/domain implementations; emit per-target errors and repeat them at process exit; assert only registry bytes for purity; time-only sleeps as race evidence.
