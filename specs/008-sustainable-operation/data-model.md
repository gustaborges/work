# Data Model: Sustainable Platform Operation

## 1. Storage boundaries and compatibility

Canonical Work snapshots and work.db remain schema/version 3. F7 changes only package administrative state. Existing human configuration stays `config/work.json`; repository convention memory stays `state/branch_conventions.json`. Neither is copied into Work snapshots. Installed package directories retain their existing layout, including the seed's different root-manifest layout.

Additive state:

| Location | Contents | Authority |
| --- | --- | --- |
| state/registry.json | Existing declarations/packages plus version/source/content_digest | Registered installed baseline, enablement and ownership |
| state/reference-package.json | Schema-1 initialization/policy-seeding/removal history | Durable user lifecycle intent, outside removable package |
| state/administration.json | Schema-1 epoch/status/transaction identity | Coherent visibility and recovery guard |
| state/administration/<id>/journal.json | Finite before/after images, owned moves and prepared/committed phase | Recovery only; not a parallel live catalog |
| plugins/.admin-<id>-<alias>-stage or -backup | Owned candidate/undo directories on package filesystem | Temporary, transaction-owned |

All generated paths come from workhome helpers; alias/id validation prevents traversal. Transaction paths cannot point outside Work home or through linked external sources. Local and remote source credentials may remain privately necessary for retrieval, but never appear in public views or journals' diagnostic summaries. Existing file permissions are retained unless a new transaction directory is created, which should use owner-only access for before-images/source metadata.

## 2. Installed package

Existing fields remain: alias, origin, reference, conventions, plugin_name, optional enabled. Add optional `version`, `source`, `content_digest`.

- Alias is a validated unique local identity and never changes during update.
- Origin is local-pinned, local-linked, remote-pinned, or embedded-seed (newly explicit existing seed origin).
- Source is the original absolute local directory or exact recorded remote source. Legacy fallback extracts remote source from the last pinned `@`, otherwise the existing path reference.
- Reference remains compatible: remote source plus pinned SHA, local source path, or embedded content digest. Public serialization uses a safe display projection of source-bearing values.
- Version is the last registered nonempty opaque manifest string. Equality compares exact values; no semantic ordering. Null/omitted legacy values mean unknown, never version zero.
- Content digest identifies frozen installed bytes for pinned local content and selection conflict checks. Digest includes normalized relative paths, file kinds, executable bits and bytes in deterministic order, excluding VCS administration; platform suffix/path-separator normalization must be shared by prepare and install tests. Remote HEAD remains an additional recorded identity. Do not rehash arbitrary Work trees.
- Enabled null/omitted means true; false remains false across update/reinstall.
- Plugin name owns private semantic keys; a known name change makes an update invalid.
- Declared convention names retain shared ownership under existing registry semantics. Removing an owner never removes another package's contribution.

Legacy projections are read-only: pinned version may be read from the installed managed manifest; unreadable/missing manifest yields unknown with a reason. A linked directory's live version is not a reliable registered baseline, so never use it to backfill during a read. Unknown managed-pinned update baselines require explicit reinstall; linked/embedded origins still report source-managed/not-applicable; mutation adoption may persist only information that can be proven from the installed copy. Registration stores metadata and declarations from one validated manifest/candidate.

Pinned preparation permits regular files/directories and relative symlinks whose complete resolved chain stays inside the frozen package; reject absolute/escaping links, cycles, special files and escaping entrypoints. Include raw relative link text in the digest. Validate the copied tree before reading plugin.json or any entrypoint. This intentionally tightens pinned install/update validation to make reviewed package bytes fixed; developer-managed --link remains external by design. Runtime binaries and arbitrary plugin-requested external resources are outside the package snapshot guarantee.

## 3. Snapshot and projections

`AdminSnapshot`: control epoch plus config, raw registry, convention memory, lifecycle, derived virtual legacy seed ownership and inspection issues. Pure Read obtains identical stable control records before/after these loads. Three attempts maximum; pending/invalid control refuses immediately without writes. No Work database is opened.

Inventory rows retain existing fields and add enabled, nullable version, nullable content_digest and inspection status. Raw inventory includes disabled packages; activation uses Effective. A virtual legacy seed Package can appear when root installed manifest/declarations prove its ownership, without persisting migration.

Locator state:

| Condition in precedence order | installed | enabled | available | reason |
| --- | --- | --- | --- | --- |
| Neither owning package nor known legacy owner | false | false | false | package-missing |
| Package exists but component missing | false | package state | false | component-missing |
| Same qualified component has another role | false | package state | false | role-changed |
| Locator declared, package disabled | true | false | false | package-disabled |
| Locator declared, package enabled | true | true | true | null |

Installed Locator inventory contains raw declared Locators only. Missing/wrong-role references remain in policy views, whose rows also carry position and availability reason. Known legacy ownerless declarations retain established enabled behavior; absence of a Package record alone is not evidence that a known pre-F7 declaration was uninstalled.

Convention view: identity, nullable remembered convention, available and nullable reason. A null choice has available=false/reason=not-set. A stored name with no effective catalog entry has reason=provider-unavailable. Shared enabled owners preserve availability. Do not choose or save a replacement during projection.

## 4. Update candidate and reviewed impact

`PreparedCandidate`: target alias, origin/source identity, current baseline (version/reference/digest/enablement/plugin_name), frozen owned content path, candidate version/reference/digest, parsed manifest, registration delta, affected policy refs and validation result. Its owner closes/cleans it once, on check, refusal, cancellation or transfer to a transaction. No process-local candidate becomes a persistent update cache.

Origin policy:

| Origin | Candidate discovery | Apply behavior |
| --- | --- | --- |
| remote-pinned | Recorded source default revision, cloned outside active installation | Fixed cloned content/SHA, only differing valid version |
| local-pinned | Copy recorded source to temporary snapshot before inspection | Exact frozen snapshot, only differing valid version |
| local-linked | No managed candidate; source-managed | No overwrite; explicit reinstall refreshes declarations |
| embedded-seed | No source candidate; not-applicable | Explicit offline install/repair through seed lifecycle |

Check statuses: unchanged, update-available, source-managed, not-applicable, failed. Apply statuses: updated, unchanged, source-managed, not-applicable, failed, not-applied (refused authorization/cancellation before a batch starts). Unknown aliases are request failures, not silently skipped statuses. Unreadable baseline is failed with reinstall guidance; newer invalid content is failed even if the version string matches. Unknown managed-pinned baselines never become an implicit update. Linked/embedded origin classification precedes baseline checks.

`ReviewedImpact`: sorted deduplicated target aliases, candidate identities, displayed declaration changes, exact ordered affected policy refs, prior target enablement/revision and relevant convention/provider effects. Unrelated latest config edits can merge. Changed candidate/target/dependency facts invalidate review and return conflict rather than widening consent. A noninteractive dependency flag authorizes only the computed impact for explicitly resolved targets, not future undisplayed removals.

## 5. Reference-package lifecycle

Schema 1 fields: initialized (bool), policy_seeded (bool), removed (bool). Enabled is deliberately absent: registry.Package.Enabled is the sole source for disablement.

| Current lifecycle | Operation | Result |
| --- | --- | --- |
| No history and no prior installation/configuration evidence | First required setup | Install offline, seed default policy once, persist initialized=true/policy_seeded=true/removed=false |
| Legacy intact seed or install-meta policy_seeded evidence | Authorized mutation adoption | Persist ownership/version/history without policy insertion or enablement reset |
| Existing explicit repository configuration without seed history | Required seed installation | Adopt configuration as user-managed; do not insert default policy entry |
| initialized, removed=false; damaged content | Repair | Restore embedded content/registration, preserve disabled state and policy history |
| Installed enabled or disabled | Confirmed uninstall | Remove package and approved refs, keep initialized=true/policy_seeded history, removed=true |
| removed=true | Ordinary required setup | Leave absent; missing capability gives actionable guidance |
| Any lifecycle, including fresh | Explicit install --embedded | Install offline, initialized=true/policy_seeded=true/removed=false; never insert policy |

Explicit embedded installation marks default-policy initialization as already handled even on a fresh home, preventing later bootstrap insertion.

A new seed release may refresh a still-installed seed through normal offline repair, retaining enabled state and all policy edits. It never clears removed=true. Ambiguous/corrupt historical evidence is a recovery error, not a reason to insert defaults. Lifecycle adoption belongs to a mutation, never a list/show/check.

## 6. Journal and epoch state machine

Control: `{schema:1, epoch:N, status:"stable", transaction_id:null}` or pending with a validated transaction ID. Absence is stable epoch 0. Every completed rollback or commit increments epoch; never return to a prior epoch value.

Journal: schema, id, operation, target aliases, phase prepared|committed, before/after existence+bytes+digest for the finite affected JSON files, package move descriptors, reviewed candidate identities and exact approved dependency refs. Store full images in owned files, not public diagnostics. Unchanged files need not be journaled.

0. Parse complete grammar and automation confirmation requirements. For an explicitly mutating invocation, run a short locked recovery preflight before snapshot/alias lookup, candidate preparation or review, then release. This recovers only the prior authorized transaction and reports it separately. Retry guidance names the interrupted direct mutating command with its required flags. Reads and bare hub entry never run this phase.
1. Outside lock: resolve aliases from a pure stable snapshot, fetch/freeze candidates, validate static fields and collect review. Check-only uses OS temporary storage only.
2. Acquire administration lock cancellably. Recover any earlier operation before loading authoritative mutable state; recovery is reported separately from the new request. Revalidate targets and impact against latest state.
3. Stage owned content on the package filesystem and journal before/after images. Flush a prepared journal before touching live files.
4. Publish control pending at n+1. Only now may live package paths or JSON change.
5. Move old owned installation roots to transaction backups, publish candidate roots/remove selected roots, and replace affected JSON using atomicfile. Check cancellation at each precommit boundary.
6. Publish journal phase committed. This is the logical point of no return after all recorded after-images are present.
7. Publish stable control at n+2. Only now can pure reads accept the new coherent state. Retain journal until this publication succeeds.
8. Delete owned backups/staging/journal. Postcommit cleanup failure returns cleanup-failed with committed=true; it never claims the previous state was restored.

Recovery under the same write lock:

- No referenced pending journal: unreferenced prepared staging is abandoned and cleaned; unreferenced committed journals are cleanup-only. Never roll back an older committed journal over newer live state.
- Pending prepared: restore recorded before-images and original existence/package roots; publish a newer stable epoch after successful restoration; then clean.
- Pending committed: verify/reapply recorded after-images/package placement, publish newer stable epoch, then clean.
- Unparseable, unsafe-path or unresolvable evidence: retain pending status and return recovery-required; do not guess which package is usable.

Compensation/cleanup use a fresh bounded operational context rather than an already-cancelled request. Backups are renamed owned installation roots; removing a linked source entry removes the link, never follows it. No promise is made for arbitrary power loss or hostile external writers.

## 7. Mutation and invocation ordering

All administrative writes reload current state inside the same lock. Whole enable/disable sets and whole uninstall sets each use one transaction. Each update candidate uses its own transaction so independent completed updates remain applied. Root/policy/convention/start-setup edits use latest-state narrowly scoped operations rather than saving stale captured whole objects.

Invocation guard: selected qualified component + package reference/digest/version + declaration/activation requirements are rechecked under administration lock; the subprocess runs while that lock protects mutable alias paths. Release before any branch/Work lock. Manual no-process link uses administration → Work ordering for final validation/publication. Under Work locks, post-run provider checks use pure guarded reads only. If maintenance raced with post-run publication, reject changed/pending providers and preserve existing F5 warning versus F6 error semantics.

Pure `work status` remains independent of administrative state and continues to read canonical Work snapshots even if administration needs recovery.
