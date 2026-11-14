# Installation and Recovery Contract

**Governing decision**: [filesystem-local installation](../governance/adr-skill-installation.md), conditional until S0. **Requirements**: FR-007–FR-017.

## Safety and complete preflight

1. Validate grammar/inventory/vendors/scopes before writes. Resolve selected anchors once; derive absolute target paths and stable existing-prefix filesystem identities without creating anything. For local scope reject linked descendants below D. User scope resolves its explicitly selected anchor then applies the same rule below it. Recheck all ancestors before every mutation. Existing linked target/descendant/marker, dangling link, special node and non-directory target are structural conflict. Force never permits traversal.
2. Plan all destinations and inspect local record/target state, distinguishing ENOENT from I/O errors. Canonically deduplicate identical paths and reject ancestor/descendant overlaps/case aliases conservatively. Alias selection must not duplicate work or lock itself. The CLI selects one vendor and every bundled skill. Overlapping processes may select different vendors with aliased anchors; coordination is still based on canonical destination identity, and conflicting ownership/inventory intent fails. Reject uncertain alias identity rather than guessing.
3. An initial conflict/unreadable condition anywhere leaves every requested skill tree unchanged, including no recovery mutations. Pending owned recovery alone is eligible for locked recovery, not an initial permission to overwrite evidence. Coordination parents/files may only be created after initial safety/conflict checks pass; stage/payload mutation waits for full locked preflight.
4. Acquire persistent sibling locks in canonical target order with existing AcquireContext and its 30-second timeout/cancellation. Parent creation is containment-checked. Locks are never inside a renamed tree and never unlinked on release. Hold all selected locks through recovery, stage/publication/cleanup; source retrieval/user input is absent.
5. Under locks, recheck every parent/target/record, safely recover owned operations, then replan all targets before staging anything new. If incompatible recovery evidence appears, preserve it and stop. After initial preflight success, explicit recovery can restore prior operations; these are reported independently of a new batch's later revalidation refusal. This is not a promise to undo a previous committed transaction.
6. Revalidate each complete observed tree/marker immediately before replacement, including force-authorized content. A concurrent edit fails; force does not authorize newly observed edits. Locks coordinate Work installers, not hostile writers; narrow races cannot be described as a sandbox guarantee.

## Local naming and ownership

For skill S under canonical parent P: persistent `.work-skill-S.lock`; finite `.work-skill-S.transaction.json`; unique `.work-skill-S.OP.stage` and `.work-skill-S.OP.backup` siblings. OP is a random validated operation token. No metadata is stored in Work home/admin state. Coordination artifacts are local-only, documented separately from committable payloads.

Records are strict schema-1 finite data; validate selected target basename, operation-bound sibling basenames, node kinds and fingerprints. Derive P from resolved destination, never replay an arbitrary record path. Never enumerate/delete by wildcard; unrecorded matching-name artifacts are untouched. Staging interrupted before record publication may leave an orphan: do not infer ownership or auto-delete it on a later run. The process that still holds explicit in-memory ownership may clean its own unrecorded stage.

## Per-target publication

- Current target: unchanged, no payload mtime changes. Other permitted states: create an entire owned sibling stage, write files 0644/directories 0755 where supported, write valid marker and verify staged payload digest against embedded.
- Capture full old observation, including exact marker bytes and empty-directory membership. Retained backup preserves the exact old tree; digest excludes permissions, but rollback never reconstructs files or changes their mtimes.
- Atomically publish/sync prepared record before moving target. Use existing file/directory sync helpers where supported; no universal power-loss promise.
- Existing target: rename target to owned backup. Initial install: old absence is recorded, skip backup.
- Rename stage to target, verify complete target/marker and atomically publish committed record. The commit marker is the success boundary.
- Remove only matching owned backup/stage, then record last. Cleanup failure preserves committed record and verified target, returns committed-with-cleanup-pending. Release locks after handling all outcomes.

A brief target rename gap exists for unrelated agent discovery. No portable nonempty-directory atomic overwrite is assumed. Batches commit each target in output order, not as one collection-wide transaction. Stop after an apply error and preserve earlier committed successes.

## Recovery evidence matrix

Read record phase AND actual target/stage/backup content. Every listed match includes marker and full old observation, not just payload hash. Unlisted/unsafe/mismatched combinations preserve evidence and fail recovery-required.

| Record / observed boundary | Explicit install recovery |
| --- | --- |
| No record, ordinary target, unrecorded sibling | Classify target normally; never remove the unrelated sibling. |
| Prepared before old move: exact old target, no backup, matching new stage | Retain old target; remove only recorded stage; remove record after rollback completes. |
| Prepared after old move: target absent, exact old backup, matching new stage | Rename backup back; remove owned stage; remove record last. |
| Prepared during rollback: target absent, exact old backup, stage already removed | Restore old backup; remove record only after restoration. |
| Prepared after new move: matching new target, exact old backup, stage absent | Remove only proven new target, restore exact old backup; remove record last. |
| Prepared initial install: old absent, matching stage/target, no backup | Remove proven new stage/target to restore absence; remove record last. |
| Prepared rollback already restored: exact old target, stage/backup already removed | Finish removing record; remain exact old state. |
| Committed: verified new target with matching marker, exact old backup or backup already removed | Keep new target, finish cleanup; record removed last. |
| Committed initial install: verified new target, no backup | Keep target; finish owned stage/record cleanup. |
| Missing only old copy, unexpected target/backup edit, invalid paths/schema or committed target mismatch | Preserve all evidence; explicit recovery-required; never claim success/unchanged. |

Rollback can itself be interrupted: every compensation step is idempotent under the same evidence rules. Removing a proven new tree must not delete the only old copy; old backup remains until restoration. Record remains until rollback/cleanup finishes. Inject interruption during rollback/forward cleanup, not just forward apply.

## Pure inspection coherence

Status never calls locks, mkdir, recovery or write-capable helpers. Observe ancestor/target/record identity and fingerprints before hashing; hash only safe ordinary trees, read marker and reobserve complete identity/record evidence afterward. An observed prepared/committed record immediately yields recovery_required; record disappearance or target churn triggers retry. Use at most three attempts. Continuous churn or unresolvable evidence yields recovery_required with safe token. Inspect every selected target without mixing revisions; do not claim cross-target snapshot atomicity. Missing safe ancestors produce absent; unsafe ancestry conflicts; I/O becomes unreadable.

## Required checkpoints and race proofs

Both initial install and update, every write/move/sync boundary: stage creation/file/marker verification, prepared record temp/publish, old-to-backup move, stage-to-target move, final verification, commit record temp/publish, every backup/stage removal, record removal, rollback restoration/removals and forward-cleanup retry. Assert exact old-or-new state, record retention, per-target commit outcome, unrelated sibling preservation and rerun convergence. Test I/O refusal in inspection and each apply phase.

Deterministic races: two installers same canonical path/case alias, overlapping selection sets with ordered locks, lock timeout/cancellation, target/marker edit between initial/locked preflight and immediately before publish, parent substitution, status during each move/record transition, later-target failure after earlier commit. Keep external linked sources and explicitly configured Work home byte-identical. Run portable cases on all three OSes; document symlink capability skips without skipping other safety coverage.

## Accepted authority reconciliation

S0 approved by the owner on 2026-11-14 against develop revision 161cf233ef1eee8836fcde02cbc5f6433f65d5f4. PRD FR-65–69, ADR-0031–33 and ADD-0001 are authoritative. Plural skills grammar, receipt fields and diagnostic codes 62–66 are frozen. Earlier conditional wording records planning history and no longer represents a pending approval.
