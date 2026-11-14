# Contract: Marketplace Administration, Recovery, and Lifetimes

**Governing decisions:** ADR-0027 for current transaction safety; ADR-0030 removes previous-format compatibility. **Stories:** all. **Requirements:** FR-005–FR-016, FR-018–FR-019, FR-024.

## One current publication model

Use one strict current schema for administrative control/journal and one current shape for config, registry, memory, lifecycle, and catalogs, as specified in data-model.md. Missing state represents a fresh home; malformed/unsupported present state is invalid and remains untouched. There are no dual decoders, old journals, conversion images, old-executable fences, adoption paths, or prior-format fixtures.

One administration lock protects finite publication: prepared journal → pending epoch → owned package moves and JSON images → committed marker → stable epoch → cleanup. The finite allowlist is config/registry/memory/lifecycle/catalogs. Typed moves address separately validated marketplace/plugin segments, never arbitrary paths. Before-images and package backups remain owned until stable publication.

Recovery of a prepared current transaction restores before-images/moves; recovery of a committed transaction completes after-images/moves. A committed cleanup failure reports applied effects rather than rollback. Unknown schema/image/address evidence is rejected and preserved for diagnosis. No-op operations create no journal, timestamps, or epoch change. Current interruption/recovery guarantees do not imply support for an earlier development format.

## Preparation, locking, and cleanup

1. Parse intended syntax before state access. A valid explicit mutation runs current recovery preflight; reads/checks never recover.
2. Read a coherent baseline. Under a short administration lock, verify any persistent capture/staging reference and acquire its process-held lease.
3. Release administration for Git, coherent capture/copy, runtime validation, and review. Never acquire administration under a Work/branch lock.
4. Reacquire administration, recover if needed, and reload. Compare exact affected registration/provenance/entry/revision/content/target/dependency facts. Merge unrelated changes; reject relevant drift as conflict/53 without expanding authorization.
5. Publish through the current finite journal and report pre-commit versus committed outcome. Close candidates idempotently on success, error, Back, cancellation, and stale async completion.

Use separate per-plan lease records with OS-held locks to support concurrent preparation on all platforms. Each record contains validated object identity. Cleanup under administration tries its lock nonblockingly; live locks retain objects, while a crashed process releases its locks and abandoned records can be collected. Close releases the lease lock before acquiring administration to remove its record. Never infer liveness from age/PID alone. Cleanup removes only unreferenced and unleased owned objects, not arbitrary paths. It is an internal mutation, never a list/check side effect.

Complete captures publish at validated hex-only digest paths before committed references. A crash may leave an unreferenced complete capture, never a reference to partial bytes. Lease ownership also protects stages prepared outside the journal from orphan sweeping. Committed cleanup failure returns 55 with exact applied targets and retry evidence.

## Exact operation facts

- Source add/refresh compares registration_id/source/name/branch and prior canonical catalog; commits are not ordered by ancestry. Registration alone does not mark default policy handled.
- Install/reinstall compares actual resolved-entry provenance, selected snapshot/entry, digest, declarations and authorized dependencies, including service callbacks. There is one copy mode and no standalone publishing path.
- Check/update validates recorded source against current registration or code-owned builtin. Missing discovery need not prevent an explicit check; absent/mismatched registration prevents maintenance. Resolve once per source/branch group and apply reviewed candidates without refetch; publish per plugin.
- Plugin removal freezes its exact selected set. Source removal adds source/catalog and every installed target, including disabled/withdrawn entries. Both preserve current Works/memory and require separate dependency authorization, without origin access.
- Explicit policy mutation marks policy_handled true in the same transaction; choosing empty may change lifecycle even if the array is empty. Fresh default installation seeds only an unhandled policy; explicit reference add/reinstall handles it without seeding.
- Invocation guards actual provider/provenance/declarations through subprocess completion. Replacement/removal waits or returns busy, never deletes active bytes. Release administration before Work locks and revalidate current facts after execution.

## Purity and outcomes

Reads/checks observe matching stable controls before/after loading, retry at most three times, and report pending/invalid state without repair. No home creation, lock, recovery, bootstrap, network list, backfill, config/cache/timestamp write, or Work-database open occurs. Checks may retrieve OS-temporary candidates but persist nothing in Work home.

Pre-commit cancellation preserves the new operation's before state. Recovery preflight may finish an already authorized interrupted current transaction and reports that separately. After commitment, report applied effects even if cleanup/cancellation follows. Cancellation/20, conflict/53, recovery/54, cleanup/55, and busy/56 keep clear precedence over aggregates.

## Required evidence

Inject failures around every current journal/control image, package move, commit marker, stable write, lease release, and cleanup. Cover prepared/committed recovery, nested-address attacks, orphan/live stages, captures in use, simultaneous refresh/install/remove, reviewed-set/dependency drift, execution versus removal, policy-handled publication, and no-op preservation. Run portable cases on Linux/macOS/Windows with deterministic synchronization and relevant race checks. Do not add tests for old executables, previous formats, linked providers, or migration.
