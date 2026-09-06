# Contract: Index rebuild & reconciliation (F2)

The capability that makes `~/.work/state/work.db` genuinely disposable. Authority: spec
FR-022..FR-025, SC-006; ADR-0013; research R11, R12. There is **no public CLI command** for
this in F2 (ADR-0017); it is a library (`internal/reconcile`) plus automatic self-healing on
the `work resume` / `work archive` startup path.

## Authority rule (non-negotiable)

The canonical `work-state.json` snapshots are the **only** source of truth. A rebuild or
reconcile reads them and writes `work.db`; it **never writes a snapshot**. Any disagreement
between a row and a snapshot resolves in favour of the snapshot. `ReconcileReport.snapshots_written`
is always `0` — a test asserts this (SC-006).

## Discovery

Scan, non-recursively at the Work-directory level:

```
<workspace>/in-progress/*/work-state.json      → expect status = in-progress
<workspace>/archived/*/work-state.json          → expect status = archived
```

`<workspace>` is `config.work.json → workspace`. For each snapshot, derive the projection
row's location fields from where the file sits:

| Column | Active Work | Archived Work |
|---|---|---|
| `dir_path` | `<workspace>/in-progress/<name>` | `<workspace>/archived/<yyyymmdd>-<name>[-<n>]` |
| `snapshot_path` | `<dir_path>/work-state.json` | `<dir_path>/work-state.json` |
| `worktree_path` | `<dir_path>/worktree` | `""` |
| `repo_name` | the `<name>` prefix before the last `_` (matches F1's derivation) | same |

All `work.*` columns (`id`, `slug`, `status`, `branch`, `base_branch`, `branch_convention`,
`start_mode`, `starter`, `created_at`, `last_accessed_at`, `archived_at`) come straight from
the snapshot.

## `Rebuild(workspaceRoot, dbPath) (ReconcileReport, error)`

1. Open `dbPath` (create if absent); `DROP`/recreate the `works` table; set
   `PRAGMA user_version = 2`.
2. For each discovered snapshot:
   - unreadable / unparseable / schema not in {1,2} / fails `State.Validate` → **skip**,
     append `{path, reason}` to `report.skipped`, continue (FR-024);
   - otherwise `Upsert` the derived row; `report.indexed++`.
3. Return the report. A non-empty `skipped` is **not** an error — the rest are indexed.

After a rebuild, `work resume` ordering and `work archive` eligibility are identical to
before the projection was lost (FR-022, SC-006).

## `Reconcile(db, workspaceRoot) (ReconcileReport, error)`

For an existing, openable DB at `user_version = 2`:

1. Build the set of ids present on disk (from the scan) and the set present in `works`.
2. For each on-disk snapshot: `Upsert` the derived row (idempotent; fixes stale
   `last_accessed_at`, wrong `status`, missing rows) — snapshot values win (FR-023).
3. For each `works` row whose id is **not** on disk: `Delete` it; `report.dropped++`
   (a Work whose directory was removed outside the tool).
4. Unreadable snapshots → `report.skipped`, left as-is in the index if a row already exists,
   never invented (FR-024).

`Reconcile` never rewrites a snapshot to match a row (FR-023).

## Automatic invocation (research R11)

`work resume` and `work archive` obtain the projection through one shared helper:

| DB state at startup | Action |
|---|---|
| file absent, or `sql.Open`/`PRAGMA` fails, or `user_version < 2` | `Rebuild` |
| openable at `user_version = 2` | lightweight `Reconcile` |

At the F1/F2 scale (tens of Works) the scan is sub-second. `work start` is **not** on this
path (F2 does not change F1).

## Diagnostics

A skipped snapshot is surfaced with the `snapshot-unreadable` token (exit code 25 *were*
this ever a top-level command). In F2 it appears as a `note:` line on stderr from the
command that triggered the rebuild, naming the snapshot path and the reason, and the command
proceeds normally with the Works that indexed.

## Test invariants (SC-006, US3)

- Delete `work.db`, run any F2 command → rebuilt index: identical active/archived
  classification, identical `last_accessed_at DESC, id DESC` order, **0** snapshots modified
  (compare mtime + bytes).
- Hand-corrupt the DB (stale time / extra row / missing row) → after a reconcile the index
  agrees with the snapshots; no snapshot rewritten.
- One corrupt snapshot among many → reported skipped; the others index.
