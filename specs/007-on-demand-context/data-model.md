# Data Model: On-Demand Context (F6)

Authority: [spec.md](spec.md) FR-001–FR-031; [research.md](research.md) R1–R12; ADD §3, §4, §9–§12.

## 1. Canonical Work state — unchanged schema 3

Continue using `work.State` and `WorkSection` from `internal/work/state.go`. Read accepts schemas 1–3; status reads and validates without rewriting. A successful link update uses the existing writer and may upgrade an older accepted snapshot to schema 3. Import never rewrites the snapshot.

| Section | F6 ownership and behavior |
| --- | --- |
| `work` | Core only; identity, lifecycle, branch and timestamps are unchanged by link/import/status. |
| `meta` | Existing typed data; readable as declared importer inputs, unchanged by F6 commands. |
| `links` | Map of semantic key to one non-empty text value; manual association replaces only the selected key. |

A present metadata null is a present input with null value. An absent optional entry is omitted. Link values use the existing Semantic Conventions v1 rules: zero-length text is invalid; accepted text is neither trimmed nor normalized. Private link keys must belong to the selected component's manifest plugin name, not its installation alias.

## 2. Resolved Work context — transient

Proposed `workcontext.Context`:

| Field | Meaning / validation |
| --- | --- |
| `State *work.State` | Canonical snapshot decoded and fully validated. |
| `WorkDir string` | Absolute real directory containing that snapshot; independent of index location claims. |
| `SnapshotPath string` | `<WorkDir>/work-state.json`; no user-provided path interpolation from an identity. |
| `WorktreePath string` | `<WorkDir>/worktree` for active Works; empty for archived Works. Existence is not required for status. |

`Current(cwd)` considers ancestor directories that are immediate children of an `in-progress` or `archived` lifecycle area, chooses the nearest such Work snapshot and stops on invalid/unreadable data. Ordinary tracked or imported files named `work-state.json` elsewhere do not establish a nested Work context. Normalize absolute paths and resolve directory symlinks before ancestry/containment checks. Do not follow a snapshot-file symlink into a different directory. Current resolution does not require configured workspace membership. `ByID` scans only immediate Work directories in configured lifecycle roots, then may consult an optional untrusted index path hint; every returned ID must match exactly. Multiple canonical directories with the same ID are an integrity error. Missing or inaccessible unrelated snapshots are handled as described in research R2.

A manual command retains expected ID and real WorkDir from collection through commitment. It rereads the same canonical location while holding the per-ID lock and verifies identity/status. It never creates the old Work directory if archive has moved it.

## 3. Status view — transient output contract

`StatusView` contains required `id`, `status`, `branch`, `location`, and `links`. `location` is the absolute checkout path for active Works and WorkDir for archived Works. Empty links use an empty object; no field contains provenance, metadata or plugin discovery results. This view has no independent storage or version migration.

See [status contract](contracts/cli-work-status.md) for exact JSON and human lines.

## 4. Package enablement — additive registry field

Extend `registry.Package` with `Enabled *bool` tagged `json:"enabled,omitempty"`.

| Stored state | Effective enabled state |
| --- | --- |
| Field absent or null | true (legacy compatibility) |
| `true` | true |
| `false` | false |
| Component has no parent package record | true (legacy compatibility) |

This is core-maintained retained operational state alongside package identity/origin/reference. It is not generated from the manifest and must survive same-source reinstall and seed repair. Manifests cannot declare it. F6 adds no public enablement mutation and no copy in config/install metadata. F7 will manage transitions using this same state.

Raw queries preserve installed records for install, bootstrap, inventory and dependency inspection. Effective queries exclude disabled packages for all executable roles and evaluate convention availability by existing package name ownership. A disabled reference package remains installed; its state must not be mistaken for missing seed data. No policy reference is removed by filtering.

Convention names and prefix records retain their existing shape; an ownerless legacy convention remains available, and a name with owners is available when at least one owner is enabled. F7 must validate later administrative transitions affecting shared names.

## 5. Manual evaluation and provider identity — transient

`ManualDecision` carries component identity, manual label/description, optional projected importer inputs, and sorted missing required declarations. Qualified identity is always `<alias>/<component-name>`. Semantic Linker key is independent of provider identity; multiple providers may share it.

Manual Linker eligibility is `enabled && role=linker && manual-present && valid-owned-key`. Discovery, event subscriptions and discovery inputs play no part. Manual Importer eligibility is `enabled && role=importer && manual-present && no-missing-required-inputs`. Optional input absence never blocks it. Input projection uses the same namespace resolver and five public Work facts as F5.

Direct-name resolution considers manually available providers before importer input filtering. Ambiguity is an error; interactive lists show only fully eligible providers. Missing-input errors carry declarations/keys, never values.

## 6. Publication and provenance

Manual association transition under the shared Work lock:

```text
resolved active context + selected provider + accepted text
  → reload/revalidate provider
  → lock by Work ID; reread and validate canonical identity/active state
  → set links[provider.key] = exact text
  → final cancellation check
  → atomic snapshot publication [commit]
  → best-effort operational provenance write
```

Provenance retains schema 3 and primary identity `(work_id, section, key)`:

| Field | Manual association |
| --- | --- |
| `work_id` | Canonical Work ID |
| `section` | `links` |
| `key` | Selected Linker's semantic key |
| `source_component` | Qualified provider identity |
| `source_operation` | `link` (existing sources remain `start` and `discover`) |
| `recorded_at` | Commitment time, UTC RFC 3339 |

No history is introduced. Provenance remains index-only, best effort after canonical publication, and unrecoverable from snapshots after a full index rebuild. Manual import does not open or maintain the index. Association may initialize/migrate the existing projection after canonical commitment and upsert only the selected Work row before recording provenance; it does not perform a global reconcile/reset or delete unrelated provenance. Extract the existing pure snapshot-to-row conversion from reconcile for this selected-row use. If the existing index cannot be opened or maintained safely, retain diagnostic detail under `WORK_DEBUG` and keep canonical publication successful. Cancellation during collection has no index effects.

## 7. Import execution and deferred artifact commitment

One execution has component identity, captured declared input values, unique stage path, validated plan, and terminal result. No new persisted execution record or importer provenance table is introduced.

```text
validated active Work + selected eligible Importer
  → stage created → subprocess → response validated
  → acquire Work lock → reread active identity/path and provider
  → build full destination plan → apply with cancellable copying
  → staged output removed → final cancellation check
  → commit applied output → release lock → stable success result
```

Before commitment, any failure rolls back only paths created by this execution, then cleans staging independently of the cancelled context. A concurrent archive may win before the lock: reject the stale destination and remove the stage. After this import commits, an archive may move the completed artifacts normally.

`staging.Applied` is transient and owns its undo list. `Commit` and `Rollback` are idempotent terminal operations. Repeating rollback removes nothing after the first successful rollback; rollback failure retains unresolved paths for diagnostic/recovery purposes and permits a retry rather than discarding the undo information. Rollback after commit is disallowed and returns a clear internal misuse error. Application code defers rollback until commitment has succeeded.

Plans preserve F5 refusal reasons and validate all existing native destinations before tentative application. Native exclusive creation and directory identity checks detect remaining target-filesystem aliases/races during application; any failure rolls back before commitment. Case-distinct names remain valid on case-sensitive targets. Existing directories may merge when safe; existing files never do. Only directories and regular files are valid. Reserved snapshot/checkout locations and escaping destinations are refused. Exclusive create is retained for races and filesystem equivalence not covered by preflight.

## 8. Cancellation and error precedence

Before snapshot/artifact commitment, cancellation returns existing `cancelled` (20) after rollback and stage cleanup. After commitment, a late interrupt cannot convert a completed change into a reported uncommitted cancellation. A cleanup/rollback failure supersedes normal cancellation with a recovery error; preserve cancellation and original failure as internal causes. Neither status nor manual commands reposition the shell or update recent access.

## 9. Persistence summary

| Surface | F6 change |
| --- | --- |
| `work-state.json` | No schema change; association updates one existing link entry. |
| `work.db` | No migration; adds documented provenance operation `link`. |
| `registry.json` | Optional retained package `enabled` field; absent means true. |
| `config/work.json` | No new field. |
| Install metadata | No new field. |
| Files under Work directory | All-or-nothing manual importer artifacts. |
| System temporary directory | One exclusive stage per importer execution, removed before commitment. |
