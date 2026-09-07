# Phase 1 Data Model: Daily Cycle — Resume and Archive (F2)

**Feature**: `specs/002-daily-cycle/` · **Plan**: [plan.md](./plan.md) ·
**Research**: [research.md](./research.md) · **Date**: 2026-09-06

F2 changes no storage *system* — it evolves the F1 canonical snapshot to **schema 2** and
the projection to **`user_version = 2`**, both additively, and adds transient entities for
the resume and archive journeys. Authority rule is unchanged and now actively exercised:
`work-state.json` is the sole source of truth for a Work's `status` and `last_accessed_at`;
`work.db` is derived and fully rebuildable from the snapshots (ADR-0013, FR-021–FR-025).

Entities are grouped by lifetime: **transient** (one invocation), **persisted canonical**
(`work-state.json`), **persisted projection** (`work.db`), and **on-disk layout**.

---

## 1. Transient entities (one invocation)

### 1.1 WorkRow
The list-row model the resume and archive pickers render. Built by `internal/worklist` from
projection rows; not persisted.

| Field | Type | Notes |
|---|---|---|
| `id` | string | `work.id` (ULID); the value echoed on success and captured by scripts |
| `repo_name` | string | from `works.repo_name`; part of the unambiguous display name |
| `slug` | string | from `works.slug` |
| `branch` | string | from `works.branch` (real branch, may contain `/`) |
| `status` | enum | `in-progress` \| `archived` |
| `last_accessed_at` | time | parsed from the snapshot-authoritative column |
| `display_name` | string | `"<repo_name>  <slug>"`; `+ "  (" + id[:6] + ")"` iff `(repo_name, slug, branch)` collides with another row |
| `relative_time` | string | `reltime.Format(now - last_accessed_at)` (R14) |

Ordering (both pickers, and `projection.ListActive`): `last_accessed_at DESC, id DESC` — a
total, deterministic order (R3).

### 1.2 ResumeTarget
Resolution of `work resume [target]`.

| Field | Type | Notes |
|---|---|---|
| `raw` | string? | the CLI argument, if any |
| `id` | string | the resolved opaque id (== `raw` when given; chosen row's id interactively) |
| `outcome` | enum | `resolved` \| `not-found` (exit 21) \| `archived` (exit 22) |

Rules: `raw`, when present, is matched **only** against `work.id` (FR-026). No slug / branch
/ path form is accepted. A `raw` that resolves to nothing → `not-found`, no state change
(FR-005). A `raw` resolving to an archived Work → `archived`, no reposition (FR-020).

### 1.3 ArchiveSelection
The set chosen for `work archive`, from the multi-select picker or explicit ids.

| Field | Type | Notes |
|---|---|---|
| `ids` | []string | opaque ids; **none preselected** in the interactive picker (FR-009) |
| `confirmed` | bool | true only after the confirmation view / `--yes` (FR-010) |
| `unknown` | []string | explicit ids that name no Work — reported, do not fail the batch (FR-019) |
| `already_archived` | []string | explicit ids already archived — reported, do not fail the batch (FR-019) |

### 1.4 ArchiveOperation (per Work)
The bounded, transactional move of one Work. Owns a LIFO compensation stack (R6). Not
persisted.

| Field | Type | Notes |
|---|---|---|
| `id` | string | the Work |
| `lock` | handle | `state/locks/<sha256(id)>.lock` (R17) |
| `dirty` | bool | `git -C <worktree> status --porcelain` non-empty (R7) |
| `acknowledged` | bool | interactive per-Work ack, or `--force-dirty` |
| `src_dir` | string | `<workspace>/in-progress/<repo>_<branch-sanitized>` |
| `dst_dir` | string | `<workspace>/archived/<yyyymmdd>-<repo>_<branch-sanitized>[-<n>]` (R8) |
| `steps` | stack of compensators | LIFO unwind on failure before commit |
| `committed` | bool | true once the snapshot `status` flip is durable (R6 step 3 — the canonical commit point) |
| `outcome` | enum | `archived` \| `left-active` (dirty, un-acked, or compensated) \| `failed` (exit 24) |

State per Work is **fully active or fully archived** after the run (FR-017). A batch member's
failure never rolls back an earlier member (independent locks + independent stacks).

### 1.5 ReconcileReport
Result of a rebuild or reconcile pass (`internal/reconcile`).

| Field | Type | Notes |
|---|---|---|
| `indexed` | int | rows written from readable snapshots |
| `dropped` | int | rows removed for Works no longer on disk (reconcile only) |
| `skipped` | []{path, reason} | snapshots that could not be read/parsed — diagnosed, not fatal (FR-024) |
| `snapshots_written` | int | **always 0** — the invariant a test asserts (FR-023, SC-006) |

---

## 2. Persisted canonical — `work-state.json` (schema = 2)

JSON Schema: [`contracts/work-state.schema.json`](./contracts/work-state.schema.json).
One file per Work. Atomic write (temp + rename, F1 R4). Sections `work` / `meta` / `links`
unchanged in shape except within `work`.

### 2.1 top level
| Field | Type | Required | F2 change |
|---|---|---|---|
| `schema` | integer | yes | `2`. Readers accept `1` **or** `2`; writers always emit `2` (R2) |
| `work` | object | yes | see §2.2 |
| `meta` | object | yes | unchanged (`{}` unless a plugin published — not in F2) |
| `links` | object | yes | unchanged |

### 2.2 `work` object — F2 fields (delta from schema 1)
| Field | Type | Required | F2 rule |
|---|---|---|---|
| `status` | enum | yes | **widened** to `in-progress` \| `archived` |
| `archived_at` | string (RFC 3339 UTC) | **iff `status == "archived"`** | set once, at the archive commit; absent for active Works |
| `last_accessed_at` | string (RFC 3339 UTC) | yes | **now mutable** — bumped to "now" on a successful resume, and set to the archival time on archive |
| `id`, `slug`, `start_mode`, `starter`, `branch`, `base_branch`, `branch_convention`, `created_at` | — | yes | **unchanged**; F2 never rewrites these |

`additionalProperties: false` on `work` is retained; `archived_at` is the only new key.

### 2.3 State transitions (F2 scope)
```
                 resume (bump last_accessed_at)
                ┌───────────────┐
                ▼               │
   (F1 create) ─►  in-progress ─┘
                       │
                       │ archive  (status→archived, set archived_at,
                       │           last_accessed_at→archival time)
                       ▼
                    archived        ── terminal in F2 (un-archive is out of scope)
```

- `in-progress → in-progress`: `last_accessed_at` bump only. Single atomic write. Canonical
  commit point of a resume (R3).
- `in-progress → archived`: `status`, `archived_at`, `last_accessed_at` set in one atomic
  write **before** the worktree is destroyed. Canonical commit point of an archive (R6 step 3).
- No `archived → *` transition in F2.
- A resume MUST refuse a Work whose `status == "archived"` (FR-020).

---

## 3. Persisted projection — `~/.work/state/work.db` (SQLite, user_version = 2)

Derived from snapshots, rebuildable (R11, R12). One table, extended.

### 3.1 `works` (delta from F1)
| Column | Type | F2 change |
|---|---|---|
| `status` | TEXT NOT NULL | values now `in-progress` \| `archived` |
| `archived_at` | TEXT | **NEW**, nullable; mirrors `work.archived_at` (NULL/`""` for active) |
| `dir_path` | TEXT NOT NULL UNIQUE | for archived Works, the `archived/<yyyymmdd>-…` path |
| `snapshot_path` | TEXT NOT NULL | follows `dir_path` |
| `worktree_path` | TEXT NOT NULL | `""` for archived Works (no worktree) |
| `last_accessed_at` | TEXT NOT NULL | mirrors the (now mutable) snapshot value |
| all other columns | — | unchanged from F1 `data-model.md` §3.1 |

Indexes: `PRIMARY KEY(id)`, `UNIQUE(dir_path)`, `works_last_accessed` (from F1),
**`works_status` (NEW)** on `status` — supports `ListActive` and the archive list.

Migration (`projection.Migrate`, forward-only, version-stepped — R12):
```sql
-- user_version 1 -> 2
ALTER TABLE works ADD COLUMN archived_at TEXT;
CREATE INDEX IF NOT EXISTS works_status ON works(status);
PRAGMA user_version = 2;
```
A DB that is absent, unopenable, or still at `user_version < 2` when `work resume` /
`work archive` starts is **rebuilt** from snapshots instead (R11).

### 3.2 New operations
| Op | SQL shape | Used by |
|---|---|---|
| `ListActive()` | `SELECT … WHERE status='in-progress' ORDER BY last_accessed_at DESC, id DESC` | resume + archive pickers |
| `Get(id)` | (F1) `SELECT … WHERE id=?` | explicit-target resolution |
| `SetAccessed(id, ts)` | `UPDATE works SET last_accessed_at=? WHERE id=?` | resume commit trailer (R3) |
| `MarkArchived(id, row)` | `UPDATE works SET status='archived', archived_at=?, dir_path=?, snapshot_path=?, worktree_path='' WHERE id=?` | archive commit trailer (R6 step 6) |
| `Upsert(row)` / `Delete(id)` | (F1) | reconcile / rebuild (R11) |

### 3.3 Invariants (checked by `internal/work/verify`, extended)
- exactly one `works` row per on-disk snapshot under `in-progress/` **or** `archived/`;
- for `status='in-progress'`: `worktree_path` exists and `git -C worktree rev-parse
  --abbrev-ref HEAD == branch` (F1 checks);
- for `status='archived'`: `worktree_path == ""`; `dir_path` is under `<workspace>/archived/`;
  the snapshot at `snapshot_path` has `status=='archived'` and a well-formed `archived_at`;
  **no** worktree/branch check (the worktree is gone; the branch is intentionally left and
  is not tracked);
- `works.status`, `works.last_accessed_at`, `works.archived_at` always equal the snapshot's
  — the snapshot wins in any disagreement (FR-023);
- `snapshots_written == 0` for any rebuild/reconcile (FR-023, SC-006).

---

## 4. On-disk layout

### 4.1 Active Work (unchanged from F1)
```text
<workspace>/in-progress/<repo>_<branch-sanitized>/
  worktree/            # git worktree, HEAD = branch
  work-state.json      # schema 1 or 2; status = in-progress
```

### 4.2 Archived Work (ADD §3, R8)
```text
<workspace>/archived/<yyyymmdd>-<repo>_<branch-sanitized>[-<n>]/
  work-state.json      # schema 2; status = archived; archived_at set
  <importer artifacts> # any non-core files preserved verbatim by the move (F5+)
  # NO worktree/ — destroyed via `git worktree remove --force`
```
- `<yyyymmdd>` = archival date (local time). `-<n>` (`-2`, `-3`, …) is appended only to
  resolve a same-day, same-repo, same-branch collision — the smallest free integer (R8).
- The directory is produced by **renaming** `in-progress/<name>` → this path (a move within
  one workspace filesystem), after the `worktree/` subdirectory is removed.
- The Git branch the Work created in the source repository is **left intact** (FR-014); it
  is not represented here and F2 does not track it.

### 4.3 Advisory locks (R17)
`~/.work/state/locks/<sha256(work-id)>.lock` — one per Work, held for the duration of that
Work's resume (steps 1–3) or archive (steps 1–7). Created on demand; safe to delete when no
`work` process runs.

---

## 5. Entity → requirement traceability

| Entity | Key requirements |
|---|---|
| WorkRow / ordering | FR-002, FR-009, SC-002, SC-008 |
| ResumeTarget | FR-005, FR-020, FR-026 |
| ArchiveSelection | FR-009, FR-010, FR-019 |
| ArchiveOperation | FR-012, FR-013, FR-014, FR-015, FR-017, FR-018, SC-003, SC-005, SC-009 |
| ReconcileReport | FR-022, FR-023, FR-024, FR-025, SC-006 |
| `work-state.json` schema 2 (`status`, `archived_at`, `last_accessed_at`) | FR-003, FR-012, FR-015, FR-021, ADR-0013 |
| `work.db` v2 (`status`, `archived_at`, `works_status`, new ops) | FR-018, FR-020, FR-022, FR-025 |
| Archived-area layout | FR-012, FR-015, ADD §3 |
| Advisory locks | FR-021 (edge case), Assumptions |
