# Contract: Administrative State, Recovery and Activation

## Internal boundaries

Proposed application interfaces are contracts for implementation, not a new public plugin API:

| Interface | Inputs/result | Side effects/lifetime |
| --- | --- | --- |
| adminstate.Read(ctx, home) | Coherent raw snapshot with epoch, config, registry, convention memory, lifecycle and legacy projections | No writes, locks, bootstrap, repair or database; maximum three attempts |
| adminstate.WithWrite(ctx, home, operation) | Already-locked session, latest mutable snapshot, typed result | Recover prior pending operation; serialize narrow mutation; release exactly once |
| adminstate.WithInvocation(ctx, home, selected, invoke) | Expected package revision/declarations and operation-specific eligibility callback | Existing admin lock through subprocess completion; release before Work/branch locks |
| plugininstall.Prepare(ctx, source, options) | Frozen content, parsed validated manifest, source identity/reference/digest and owned cleanup | Process-owned staging only; no live installation/registry mutation |
| pluginadmin.Check(ctx, snapshot, targets) | Exhaustive check rows/candidates | Temporary retrieval; closes candidates in check-only mode; no persistent cache |
| pluginadmin.Apply(ctx, reviewed, options) | Per-target outcomes and aggregate error | Exact retained candidates; one transaction/package |
| pluginadmin.Uninstall(ctx, reviewed, options) | Whole-set committed result | One transaction for all aliases and approved dependencies |
| pluginadmin.SetEnabled(ctx, aliases, bool) | Satisfied state rows | One validated transaction; no policy change |
| Repository/convention service operations | Narrow requested edit plus optional reviewed preconditions | Latest-state transaction; shared by direct commands and hubs |

A prepared candidate has one owner and a Close cleanup path. Never pass a live source path as if it were a fixed local update snapshot. Registration uses the same parsed manifest that was reviewed. No service imports presentation or renders terminal errors. Public results contain safe source projections, never raw internal source identity/credentials.

## Complete writer inventory

The shared lock/protocol must cover:

- CLI plugin install, enable/disable, update/uninstall and embedded reinstall.
- bootstrap.EnsureSeed, legacy adoption, seed lifecycle and initial policy seeding.
- repository policy/root mutations and their hub equivalents.
- convention set/hub changes and start's implicit first-choice or accepted replacement memory upserts.
- start first-run config setup and internal/workspace config initialization/change helpers.

Low-level config.Save/registry.Save/repoconv.Save remain storage primitives; application call sites may use them only inside a locked transaction publication or fixture setup. Review all production callers by search. No captured pre-prompt object may be saved wholesale over current state. Fresh read helpers bypass all writer setup.

Bootstrap's public entry point acquires the common administration lock and calls its already-locked preparation/transaction path. Pluginadmin embedded install calls the already-locked helper, not the public entry point. Remove the independent bootstrap-writer race; never acquire administration recursively. Readers do not take even a shared lock that creates state.

## Epoch read protocol

Read stable control A (absent=epoch0), load finite administrative state/installed legacy metadata, read control B even if intermediate loads failed. Classify a load error only after matching stable records; drift retries instead of reporting false corruption. Accept only equal stable records. Retry at most three times on changed stable records, checking request cancellation. Pending, malformed control or unsafe referenced journal is recovery-required/54, without repair. Repeated concurrent changes are conflict/53. Buffer output until a complete snapshot succeeds; no partially emitted inventory rows.

Update checks acquire a pure snapshot and freeze candidates. Before returning results, verify installed target baseline/control still matches; bounded retry or conflict if it changed. Holding a read snapshot does not authorize applying it later. Administrative reads may succeed while a component holds the invocation lock because control remains stable.

## Transaction protocol

Use the schema/ordering in [data-model.md](../data-model.md) §6. The journal owns before/after JSON with existence bits, same-filesystem staged roots/backups and exact reviewed dependencies. Write prepared journal before pending publication; write pending before any authoritative rename/save; mark committed only after all after-images are present; publish stable before deleting undo evidence. Do not call early-cleanup StageThenSwap as the live transaction implementation.

Every explicit mutating invocation has a short recovery preflight after grammar/required automation confirmation and before pure snapshot, semantic alias lookup, retrieval or review. Acquire the administration lock, resolve any prior pending transaction, report recovery separately, and release before normal preparation. At final commit lock acquisition, check recovery again before loading current authoritative state. This is recovery of the prior authorized operation, reported separately, not permission to apply the new request. Invalid grammar/missing automation confirmation fail before mutation setup; unknown aliases for a new operation fail before that operation changes targets. Pure commands never recover.

Prepared recovery rolls back; committed recovery completes recorded after-images and cleans. Use safe owned paths and recorded existence, not guesses from whether a destination happens to exist. If stable is already published, leftover committed journals are cleanup-only and can never restore old data over a newer transaction. Only the control record's transaction ID can authorize live rollback/forward recovery.

Use existing atomicfile replacement for control/journal/JSON files; platform directory moves need errors checked and backups retained. Store candidates/backups on the destination package filesystem. Cleanup must not follow a local-linked source. Abort precommit cancellation with compensation independent of cancelled request context; after the committed marker, return committed success or committed-cleanup failure. Recovery-required identifies affected transaction/aliases and recommends the concrete interrupted direct command with required flags, such as `work plugin update acme --yes --remove-policy-references`; never advise manual deletion of unknown backups.

Default acquisition timeout remains lockfile.DefaultTimeout (30 seconds). Acquisition cancellation maps to cancelled/20; timeout maps to administration-busy/56. Failed compensation or invalid recovery data maps to recovery-required/54. Postcommit owned cleanup failure maps to cleanup-failed/55 with an explicit committed result. Record unresolved owned paths only in appropriate diagnostic detail, never candidate file content or credentials.

## Review and conflict rules

Compare selected package source identity/reference/digest/version/enablement/name and reviewed declaration/policy impact to current locked state. Changes to those facts require another review. Merge independent latest config/memory updates. Two explicitly chosen updates are independent transactions; a failure in one does not roll back the other. An uninstall set is indivisible.

A metadata-only enable/disable/root/policy/convention mutation still advances the epoch around its publication. A fully satisfied no-op need not publish an epoch or journal. An unknown or rejected operand must not leak partial in-memory slice changes into the snapshot; clone collections before mutation.

## Component activation ordering

Before each managed component invocation, protect the mutable alias path with administration lock, reload latest package and declaration, compare expected revision as well as component structure, check effective eligibility, run the existing IPC callback, then release. No prompt, remote update retrieval or Work mutation happens while this invocation lock is held. No execution timeout or process-tree isolation is added.

Starter/Locator calls occur before Work materialization locks. Linker discovery and Importer execution occur before publication/incorporation Work locks. Their later validation under a Work lock must use nonlocking pure snapshots; if a provider changed or administrative state is pending, reject the result safely. Replace automatic closures returning captured catalogs with current snapshots and expected package revisions. Never acquire administration while holding a Work or branch lock. Manual association can acquire administration then Work for its short no-subprocess publication; it cannot invert that order.

If revalidation fails, preserve existing operation semantics: a stale manual provider is manual-component-unavailable/39; automatic extension results produce existing warning behavior and do not undo a Work; a stale Starter/Locator selection before Work creation returns administration-conflict/53 with retry guidance. Pure status remains independent and works from canonical snapshots despite administrative recovery problems.

## Recovery/compatibility validation

Inject failure/interruption before and after pending, each package move and JSON publication, committed marker, stable publication and cleanup. For every prepared failure verify exact before-state and no avoidable temporary output; for every committed interruption verify exact after-state after recovery. Test missing-before files and linked roots separately. Pending/unsafe journals never yield usable mixed catalogs.

Race pure reads with whole transactions that both overlap and completely finish inside the read interval. Race config/enablement, convention upserts, bootstrap/install and uninstall batches. Block a fixture subprocess on a pipe, attempt maintenance, and verify no path replacement until it releases; then prove no admin/Work deadlock during post-run publication. Supported upgrade fixtures include legacy Package records, ownerless seed, existing policy edits, disabled packages and deliberate seed removal. Older binaries concurrently writing without this protocol are outside the supported upgrade model.
