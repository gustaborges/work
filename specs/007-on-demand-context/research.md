# Research: On-Demand Context (F6)

**Date**: 2026-10-11 · **Status**: Complete

Research used the current repository, F5 contracts, the PRD/ADD and accepted ADRs. Parallel research covered read-only resolution and manual execution. Findings below distinguish existing primitives from required changes. No application tests were run during planning.

## R1 — Resolve canonical context without mutation

**Decision**: Add `internal/workcontext`, a presentation-free reader returning the validated state, actual Work directory, snapshot path and applicable checkout path. `Current(cwd)` walks ancestors to the nearest canonical Work directory (an immediate child of an `in-progress` or `archived` area) containing `work-state.json`; `ByID(ctx, workspace, id, hints)` matches the entire opaque identity. Always call both `work.Read` and `State.Validate`.

**Evidence/rationale**: `internal/work/state.go` decodes JSON/schema in `Read`/`Decode`, but full required-field/lifecycle validation is separate. `internal/worklist/worklist.go` establishes exact identity matching. The only existing containment helper is private to archive and does not resolve snapshots. ADD §3 allows changing the configured workspace without moving existing Works, so current-directory resolution must work independently of the configured root.

**Alternatives considered**: Reuse `worklist.List` and reconcile (writes); infer a Work from Git/branch/recency (ambiguous and contrary to FR-002); constrain all current resolution to the configured root (loses older workspace contexts).

## R2 — Read-only explicit identity lookup

**Decision**: First scan the configured workspace's immediate `in-progress` and `archived` Work directories, in sorted order, without entering checkouts. A valid snapshot with matching ID is authoritative. Use an optional `projection.LookupPathHint(ctx, dbPath, id)` only when the scan finds no match: open an existing file with URI `mode=ro&immutable=1`, select only its recorded snapshot location, close immediately, and read/validate that snapshot independently. No migration, WAL pragma, reconciliation or file lock. URI construction must use proper escaping, including Windows drive handling. Bound this optional query to 250 ms, not extension execution.

**Evidence/rationale**: `projection.Open` creates databases and sets WAL; `reconcile.Open` scans and writes rows; neither is pure. SQLite documents that read-only WAL access may still need sidecars, while immutable access skips locking/change detection. Immutable mode on a concurrently changing file can give incorrect results or corruption errors, so this is explicitly an untrusted location hint, never an authoritative view. See [SQLite URI parameters](https://www.sqlite.org/uri.html#uriimmutable) and [read-only WAL databases](https://www.sqlite.org/wal.html#read_only_databases). Every hinted path must be absolute, point to `work-state.json`, and resolve to the exact requested canonical ID. Ignore hint errors, absent rows, missing paths and mismatched identities.

A damaged enclosing snapshot or a matching target snapshot that fails full validation is `snapshot-unreadable`. Do not fall through to another enclosing Work. Unrelated malformed snapshots encountered during an identity scan are skipped unless a safely decoded ID identifies the requested target. Duplicate valid identities at distinct canonical directories fail as `snapshot-unreadable`, never choose the first. Absent workspace roots are empty; inaccessible configured roots report a read failure rather than pretending there is no target.

**Limit**: With an unavailable index and an old workspace no longer named by configuration, an outside caller cannot locate its ID from no path information. Return `target-not-found` with guidance to enter that Work and run status there. Current-directory reads of that Work still succeed. This is a discovery limitation, not loss of its snapshot.

**Alternatives considered**: Mutable helper (violates purity); `mode=ro` alone (insufficient guarantee of no sidecar activity); a new persistent directory catalog (second state source); copy the index to a temporary database (unnecessary writes on a read command). Main-file hints may miss uncheckpointed rows; canonical scanning and current resolution remain independent.

## R3 — Status output is a view, not snapshot export

**Decision**: Status emits `id`, `status`, `branch`, `location`, and `links`. Human output uses stable labeled lines with links sorted bytewise by semantic key. JSON uses one object and `{}` for empty links. No metadata, provenance, repository probing, extension discovery, implicit refresh, shell repositioning or snapshot upgrade.

**Evidence/rationale**: PRD FR-38 and ADD §12 request precisely these fields and read purity. The snapshot lacks a persisted checkout path, so derive active location as `<actual WorkDir>/worktree`; archived location is WorkDir. A missing checkout does not prevent canonical inspection.

**Alternatives considered**: Dump the complete snapshot (unrequested data and couples presentation to physical schema); include live Git status or external link freshness (would redefine status and introduce extra dependencies).

## R4 — Close the absent enablement prerequisite once

**Decision**: Add optional `registry.Package.Enabled *bool` with absent/null interpreted as true. It is retained operational package state, not a manifest field. `PackageEnabled(alias)` is the single predicate. Legacy components without package parent records remain enabled. Preserve explicit values during same-source reinstall and seed repair; new installs default enabled. Do not add enable/disable commands in F6.

Separate raw installed queries from effective activation queries. Runtime Starter matching/fallback, Locator execution, automatic/manual extensions and convention availability use effective catalogs; bootstrap/install integrity and plugin inventory use raw records. A convention name remains available when any enabled owning package declares it; legacy ownerless conventions remain available. Existing bare convention identities and prefix records stay unchanged. F7 remains responsible for user-facing enable/disable transitions and their collision/dependency validation.

**Evidence/rationale**: `registry.Package`, `config.Config` and plugin CLI have no enablement state; F7 is explicitly deferred. Pretending installed always means enabled would leave F6's disabled-component cases unimplementable. The registry already holds package identity/origin/reference alongside generated declarations. Keeping one optional retained field avoids a second manual-only model. `RemoveAlias` during reinstall currently destroys the old package record, so preservation must occur before removal. Bootstrap checks must use raw presence to avoid reinstalling a disabled seed.

**Alternatives considered**: A second enabled flag in config or install metadata (competing sources); consume an in-memory fake flag only in tests (does not implement persisted disabled cases); ship F7 administration now (outside scope). Shared convention-name collision behavior is historical and unchanged; F7 must validate any later enable/disable transition that would expose a conflicting contribution.

## R5 — One input resolver, two activation policies

**Decision**: Export/refactor `extension.ResolveInputs(declared, subject)` to return projected values, sorted missing required declarations, and validation errors. Automatic eligibility keeps the current event/Starter checks. Add manual evaluation requiring enabled package + correct role + `Manual != nil`; Importers additionally resolve their inputs. Manual Linkers do not resolve discovery inputs. Sort offered providers by qualified identity.

**Evidence/rationale**: `extension/eligible.go` already preserves metadata types, distinguishes presence from null, omits absent optional inputs, and exposes the five F5 Work facts. Its current boolean failure result cannot explain missing keys. `registry.Component.Inputs` means importer inputs or Linker discovery inputs; using it blindly for association would reject valid manual Linkers.

**Alternatives considered**: Pass a fake lifecycle event to `Eligible` (applies incorrect restrictions); duplicate namespace resolution (creates divergent contracts); execute components to discover eligibility (violates static evaluation).

## R6 — Explicit resolution precedes importer input checks

**Decision**: Resolve direct names among enabled manually available components of the requested role, before filtering Importers by inputs. Bare names must match exactly one provider; qualified names match exactly one alias/name. After identity resolution, validate required inputs and report their declarations. Interactive candidate lists contain only eligible providers. Reload the registry and revalidate before link commitment or importer invocation; revalidate identity/role/manual/key after the subprocess before incorporation.

**Rationale**: An explicitly requested Importer missing inputs deserves a missing-input diagnostic, not an unknown-name error. Ambiguous bare names remain ambiguous even if only one provider happens to have inputs today. Two providers owning one semantic link remain valid choices.

**Alternatives considered**: Choose first match or sole currently eligible provider (hidden selection); open another chooser for an ambiguous explicit argument (breaks automation semantics).

## R7 — Share and harden canonical link publication

**Decision**: Extract a shared `extension.PublishLink(ctx, params)` used by automatic discovery and manual association. Acquire `Home.WorkLockPath(id)` through `AcquireContext` with the existing daily-cycle 3-second contention policy. Check context before and after acquisition; read and fully validate the latest snapshot; verify expected ID, active state and actual directory; upsert only `Links[key]`; check context immediately before the atomic write. That write is the commitment point. Do not turn a cancellation arriving after a successful write into a reported uncommitted cancellation.

Use provenance operation `link` for manual association and `discover` for discovery. Record provenance after canonical commitment under the same lock. Preserve F5's best-effort operational provenance behavior: a provenance failure is diagnostic detail and does not undo or misreport a saved link. Manual code validates the selected component and private-key ownership using its manifest plugin name before publication.

**Evidence/rationale**: `extension/linker.go:persistLink` already rereads under the shared lock and writes atomically, but uses uncancellable acquisition, has no active/ID recheck, and hardcodes `discover`. `workhome.WorkLockPath` is shared by resume/archive. `projection.RecordProvenance` accepts operation text; no database migration is required.

**Alternatives considered**: Modify the initially selected state (loses unrelated concurrent changes); put provenance into the snapshot (violates ADR-0013); roll back a canonical link after an index-only failure (contradicts F5 snapshot authority).

## R8 — Run import outside the lock; serialize commitment

**Decision**: Extract one shared Importer executor used by both automatic and manual orchestration. It creates a stage, invokes `ipc.InvokeImporter`, validates the response, then acquires the same per-Work lock before rereading/validating active identity/path and building/incorporating output. A manual command returns a typed failure; the automatic pipeline translates the same failure into its existing warning and continues. Do not call the whole automatic `Run` from a manual command.

Manual input resolution occurs immediately before invocation from current canonical state; output is associated with those resolved inputs, not recomputed after the process completes. Post-run checks confirm that the Work and selected provider still exist in the expected active context. If archive won the race, refuse commitment and clean the stage. If import holds the lock first, archive can only move a fully completed or fully rolled-back artifact set. No Work lock is held while the external process runs.

**Evidence/rationale**: Current `extension/importer.go` calls `staging.Build`/`Incorporate` without a Work lock. Existing archive performs state change and directory movement under that lock. `ipc.InvokeImporter` already supplies the correct wire protocol, runtime invocation, output cap, cancellation and bounded inherited-pipe wait.

**Alternatives considered**: Hold a Work lock across an unbounded external process (blocks lifecycle unnecessarily); run the automatic pipeline (executes unwanted components and suppresses failures); duplicate staging for manual use (two safety models).

## R9 — Cancellation and cleanup are part of artifact commitment

**Decision**: Extend shared staging with `Plan.ApplyContext(ctx) (*Applied, error)`. `Applied` owns the created-path undo list until `Commit()` or `Rollback()`; both terminal methods are idempotent. Keep `Plan.Incorporate()` as a compatibility wrapper over apply + commit. Check cancellation before each mutation, during bounded copy chunks and before final commitment. On failure, rollback is independent of the cancelled context and retains removal errors.

The shared importer sequence under the Work lock is: build → apply → remove stage → final context check → commit. If stage cleanup fails before commitment, roll back destinations and return a cleanup failure. This prevents a reported cleanup failure from leaving otherwise committed artifacts. On early process/planning failure, remove stage without using the cancelled context. Join cleanup and rollback causes without hiding them, and never claim unchanged content if rollback itself failed. F5 maps new cleanup failure to its existing `extension-persist-failed` warning.

**Evidence/rationale**: `staging.Incorporate` has no context, uses `io.Copy`, and ignores rollback errors; importer defers `stage.Remove` without checking errors. Merely calling these unchanged cannot satisfy F6 cancellation/cleanup acceptance cases. Existing `WORK_FAIL_AT=incorporate:<n>` remains available.

**Limit**: The all-or-nothing guarantee assumes the filesystem permits compensating removals, as scoped in the spec. If the OS refuses rollback or cleanup, return an explicit recovery diagnostic with safe relative paths and retain causes. Abrupt machine failure and hostile processes remain outside this transaction model.

**Alternatives considered**: Check context only after subprocess (cancellation during copy is missed); commit output before checking cleanup (command can fail after leaving committed artifacts); silently ignore removal errors (incorrect success/no-change claims).

## R10 — Preserve native filesystem semantics with full preflight and a final creation check

**Decision**: Retain F5's read-only validation of every staged entry, native existing destination and ancestor before tentative application. Preserve its reserved-path, type, symlink and escape checks. Do not reject arbitrary case-distinct names on case-sensitive targets. During application, native exclusive file creation is the final alias/race check; merging an already-existing directory requires verifying it is still an actual directory. Track staged directories by native destination identity once created so two distinct staged paths cannot silently merge through a target-filesystem alias. A second staged directory alias is refused, while an exactly spelled preexisting directory can still merge. Any refusal/failure rolls back this execution before commitment.

**Evidence/rationale**: `staging/plan.go` validates existing native destinations; `copyExclusive` already detects native file aliases using `O_EXCL`. A portable case-fold comparison is not a faithful model of every volume's per-directory case/Unicode rules. A blanket rejection would change valid F5 behavior. The approved product guarantee is complete successful incorporation or zero contribution on failure, not visibility isolation of every tentative file creation from independent readers. FR-020 is narrowly clarified to make the existing preflight plus native backstop explicit, without changing FR-021/FR-022 or SC-004.

**Alternatives considered**: Blanket Unicode case folding (rejects valid F5 imports); filesystem probes in the Work (unnecessary mutation); infer semantics from OS (volumes and directories differ); add a cross-platform filesystem rules subsystem (unbounded work). Native exclusive creation and rollback handle remaining aliases/races without claiming complete native alias prediction before the first tentative write.

## R11 — Return manual failures through the existing border

**Decision**: Reuse existing categories for grammar (2), cancellation (20), missing context/target (21), archived target (22), invalid snapshot (25) and unreadable global configuration/registry (16). Add the F6 categories documented in `contracts/manual-context.md`, codes 39–49. Existing automatic tokens and exit success remain unchanged; corresponding execution-class tokens may appear as manual errors with new codes. Link/import reject `--json`, even `--json=false` when explicitly supplied, as existing mutations do.

**Evidence/rationale**: `diag.All` currently ends at convention-unknown (38). The CLI's sole `renderDiagnostic` border already supports category, summary, hint and retained cause. Keep captured plugin stderr/raw cause under `WORK_DEBUG`, including non-interactive manual execution, without duplicating the terminal diagnostic. Receipts, progress and mutation stdout name components/keys/counts, never link values. Status is the explicit value presentation surface.

**Alternatives considered**: Return generic exit 1 for everything (no stable branching); promote automatic warnings wholesale into the CLI (manual command would incorrectly succeed); reuse Starter/Locator categories for different domain operations (misleading tokens).

## R12 — Preserve the existing presentation and delivery workflow

**Decision**: Register `status`, `link` and `import` in existing `groupInWork`. Status has a visible local `--json` flag; leave the root's historical hidden inherited flag unchanged. Build association as one `present.Wizard` with a component selector and input step, skipping explicit arguments using `StepResolved`. Use a redacted receipt (`value supplied`) for accepted link values. Import uses the existing single-selection wizard and existing progress observer concepts without warning rendering in the manual path. Domain mutation happens after collection in CLI/application orchestration; `internal/present` receives no Work DTOs.

**Evidence/rationale**: `cli/help.go` already defines the empty Inside a Work group; `present/wizard.go` and `present/input.go` support skipped steps and custom receipts. PRD and ADR-0021 supersede the roadmap's historical inline wording. Existing fixtures and testscript/PTY harnesses provide the required validation infrastructure; no dependency is needed.

**Alternatives considered**: A new manual hub or another presentation model (unnecessary); showing raw link values in receipt/stdout (unnecessary disclosure); creating implementation branches during planning (planning only).

## Resolved technical unknowns

| Question | Resolution |
| --- | --- |
| How does status avoid reconciliation and work without a database? | R1–R3: canonical readers and workspace scan; optional index path hint only. |
| How are disabled packages represented before F7? | R4: one additive retained package flag, default enabled, shared effective queries. |
| Which inputs gate manual Linkers? | R5: none; discovery inputs belong to discovery. |
| How do mutations avoid archive races and stale data? | R7–R9: latest canonical read under shared lock, lifecycle/identity checks, deferred artifact commitment. |
| What changes to persistence/protocol are necessary? | No snapshot/database migration or new IPC; additive package state and provenance operation only. |
| How are manual errors and interactive journeys presented? | R11–R12 and the command contracts. |

All technical unknowns are resolved. Implementation and validation remain for subsequent phases.
