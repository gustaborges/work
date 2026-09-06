# Phase 1 Data Model: First Local Work (F1)

**Feature**: `specs/001-first-local-work/` · **Plan**: [plan.md](./plan.md) · **Research**: [research.md](./research.md) · **Date**: 2026-09-05

Entities are grouped by lifetime: **transient** (exist only during one `work start` invocation),
**persisted canonical** (`work-state.json` — the authority, ADR-0013), **persisted projection**
(`work.db` — derived, rebuildable), and **persisted configuration / generated state** (`~/.work/`).
Authority rule: the snapshot is the single source of truth for a Work; the projection and any
in-memory view are derived from it and must never be treated as competing sources (FR-016, FR-019, ADR-0013).

---

## 1. Transient entities (one invocation)

### 1.1 LocalSource
The path the user supplies to `work start`.

| Field | Type | Notes |
|---|---|---|
| `raw` | string | exactly as given (arg or TUI-collected) |
| `resolved` | string | abs + symlink-resolved (R14) |

Rules: `resolved` must exist, be a directory, be readable, be a usable non-bare git repo with ≥1 commit and ≥1 selectable base branch (R14). Not persisted; drives validation only. In F1 it is produced by the seed **Starter** returning `{repository:{path}}` and consumed directly by the core (no Locator — ADD §7.1).

### 1.2 RepositoryReference
Transient object returned by a Starter (ADR-0016). F1 seed Starter populates only `path`.

| Field | Type | F1 use |
|---|---|---|
| `path` | string? | present → core validates directly, Locators skipped |
| `git_fetch_urls` | []string? | unused in F1 (seed Starter never sets it) |
| `name` | string? | unused in F1 |
| `query` | string? | unused in F1 |

Rules: must contain `path` **or** ≥1 field some policy Locator `accepts`. Never promoted to `work`/`meta`/`links` (ADR-0016). Discarded after `repo_path` is validated.

### 1.3 StarterResponse
Parsed stdout of the Starter subprocess (ADD §7).

| Field | Type | F1 handling |
|---|---|---|
| `repository` | RepositoryReference | required; must yield exactly one valid local repo |
| `base_branch` | string? | seed Starter omits → core prompts (FR-006/FR-009) |
| `start_modes` | []string? | seed Starter omits → `work.start_mode = "new"` (ADD §7) |
| `meta` | object? | seed Starter omits |
| `links` | object? | seed Starter omits |

Structurally-invalid response or non-zero exit → `work start` fails (`diag` `unusable-repo`/`materialization-failed`); there is no `matched:false` sentinel (ADD §7).

### 1.4 BaseBranchChoice
One row of the base-branch picker (R15).

| Field | Type | Notes |
|---|---|---|
| `refname` | string | full ref, e.g. `refs/heads/main` or `refs/remotes/origin/main` |
| `short` | string | `main`, `origin/main` |
| `scope` | enum | `local` \| `remote-tracking` |
| `object_short` | string | short SHA, disambiguates homonyms/divergence |

The chosen row's `refname` is used verbatim as the base for `git worktree add -b`; `short` is stored in `work.base_branch`.

### 1.5 BranchConventionSelection
| Field | Type | F1 value |
|---|---|---|
| `convention` | string | `freeform` (only one seeded; no memorization in F1 — spec Assumptions) |
| `prefix` | string | `{slug}` (the one `freeform` prefix) |

### 1.6 DerivedBranchName
| Field | Type | Notes |
|---|---|---|
| `slug` | string | non-empty; prompt-validated, git-authoritative |
| `value` | string | `interpolate(prefix, slug)` → for `freeform`, `== slug` |

Rules: `value` passes `git check-ref-format refs/heads/<value>` and collides with no local branch, no remote-tracking branch, and no branch bound to an existing worktree (R16). Shown to the user before materialization (FR-011).

### 1.7 CreationAttempt
The bounded operation (spec "Creation attempt" entity). Not persisted; owns the compensation stack (R10).

| Field | Type | Notes |
|---|---|---|
| `lock` | handle | `state/locks/<sha256(workspace+repo+branch)>.lock` |
| `steps` | stack of compensators | LIFO unwind on failure/cancel |
| `committed` | bool | true only after the `work.db` upsert (step 5) |
| `outcome` | enum | `published` \| `rolled-back` \| `cancelled` |

---

## 2. Persisted canonical — `work-state.json` (schema = 1)

One file per Work at `<workspace>/in-progress/<repo>_<branch>/work-state.json`.
Atomically written (R4). Sections `work` / `meta` / `links` in one physical file (ADR-0013, ADD §3).
JSON Schema: [`contracts/work-state.schema.json`](./contracts/work-state.schema.json).

### 2.1 top level
| Field | Type | Required | Notes |
|---|---|---|---|
| `schema` | integer | yes | `1` in F1; bump on any shape change |
| `work` | object | yes | core-governed domain state (§2.2) |
| `meta` | object | yes | `{}` in F1 (no plugins publish) |
| `links` | object | yes | `{}` in F1 |

Plugins never read or write this file; the core translates IPC outputs into it (ADR-0013).

### 2.2 `work` object — F1 fields
| Field | Type | Required | F1 value / rule |
|---|---|---|---|
| `id` | string | yes | opaque, sortable, generated at creation (e.g. ULID); stable identity of the Work |
| `slug` | string | yes | the chosen slug |
| `status` | enum | yes | `in-progress` (F1 only ever writes this; `archived` is F2) |
| `start_mode` | enum | yes | `new` (F1 only; `contribution`/`fork` are F4) |
| `starter` | string | yes | logical name of the Starter that resolved the source → `local-path-starter` |
| `branch` | string | yes | `DerivedBranchName.value` |
| `base_branch` | string | yes | `BaseBranchChoice.short` |
| `branch_convention` | string | yes (F1) | `freeform` (omitted only in `contribution` mode, not reachable in F1) |
| `created_at` | string (RFC 3339 UTC) | yes | set once |
| `last_accessed_at` | string (RFC 3339 UTC) | yes | `== created_at` at creation |

Not stored: the repository path / current checkout path (ambiguous — ADD §3). When a
component later needs it, the core exposes `work:worktree_path` derived from the Work's
directory; F1 does not need to persist it.

### 2.3 State transitions (F1 scope)
```
(none) --create--> in-progress
```
`in-progress → archived` and `last_accessed_at` bumps on resume are **F2**. F1 writes the
snapshot exactly once, at creation (no in-place updates in the F1 journey).

---

## 3. Persisted projection — `~/.work/state/work.db` (SQLite, user_version = 1)

Derived from snapshots, rebuildable (ADR-0013, ADR-0009). F1 needs one table.

### 3.1 `works`
| Column | Type | Notes |
|---|---|---|
| `id` | TEXT PRIMARY KEY | `work.id` |
| `slug` | TEXT NOT NULL | |
| `status` | TEXT NOT NULL | `in-progress` |
| `start_mode` | TEXT NOT NULL | `new` |
| `starter` | TEXT NOT NULL | |
| `branch` | TEXT NOT NULL | |
| `base_branch` | TEXT NOT NULL | |
| `branch_convention` | TEXT | |
| `repo_name` | TEXT NOT NULL | derived from the source repo dir name (used in the Work dir name) |
| `dir_path` | TEXT NOT NULL UNIQUE | absolute `<workspace>/in-progress/<repo>_<branch>` |
| `worktree_path` | TEXT NOT NULL | absolute `<dir_path>/worktree` |
| `created_at` | TEXT NOT NULL | mirrors snapshot |
| `last_accessed_at` | TEXT NOT NULL | mirrors snapshot |
| `snapshot_path` | TEXT NOT NULL | absolute `<dir_path>/work-state.json` |

Indexes: `PRIMARY KEY(id)`, `UNIQUE(dir_path)`. `CREATE INDEX works_last_accessed ON works(last_accessed_at DESC)` is added now (cheap) though ordered listing is an F2 journey.

Provenance columns (`source_component`, `source_operation`, `recorded_at`) for links are
**not** in F1 (no links). They arrive with F5.

### 3.2 Invariants (checked by `internal/work/verify`, R12)
- exactly one `works` row per on-disk snapshot under `<workspace>/in-progress/`;
- `works.branch` == `git -C worktree rev-parse --abbrev-ref HEAD`;
- `works.*` equals the snapshot's `work.*` for id/slug/status/branch/base/convention;
- no `works` row without a readable snapshot (rollback removes the row before the dir, R10 unwinds top-down so this holds at rest).

---

## 4. Persisted configuration & generated state — `~/.work/`

Layout: R9. Root overridable via `WORK_HOME`.

### 4.1 `config/work.json` (human-editable, ADR-0002)
| Key | Type | F1 use |
|---|---|---|
| `workspace` | string | absolute workspace root; set on first creation (FR-006), validated (R8) |
| `repository_roots` | []string | `[]` in F1 (roots are an F3 journey; key present for shape stability) |
| `repository_resolution.locators` | []string | `["work-reference/filesystem-repository-locator"]` — seeded by bootstrap, unused on the F1 happy path |

Validation: `workspace` must resolve to a writable directory, not inside a git work tree,
not inside any `repository_roots` entry (R8). Malformed JSON → `diag` `bootstrap-failed`
with the file path, no partial write.

### 4.2 `state/registry.json` (generated, ADR-0002)
Component registry built from `plugin.json` at bootstrap. One entry per component:

| Field | Type | Notes |
|---|---|---|
| `alias` | string | package alias (`work-reference`) |
| `name` | string | logical component name |
| `role` | enum | `starter` \| `repository-locator` (F1 seed has these two) |
| `entrypoint` | string | relative path under `plugins/<alias>/source/` |
| `runtime` | string? | **absent** for seed (self-contained, ADR-0006) |
| `pattern` | string? | absent for the `fallback` starter |
| `starter_layer` | enum | `fallback` for `local-path-starter` (ADR-0004) |
| `accepts` | []string? | `["name","git_fetch_urls","query"]` for the Locator |
| `on` / `inputs` / `key` / `discover` / `manual` | — | not present for F1 roles |

Also records the `freeform` convention separately: `{ name: "freeform", prefixes: ["{slug}"] }`.

Identity for idempotency = `(alias, name)` (R11). Upsert on re-bootstrap; never duplicated.

### 4.3 `plugins/<alias>/` (generated by bootstrap)
| File | Content |
|---|---|
| `plugin.json` | the seed manifest (starter + repository-locator + `freeform`) |
| `source/starter`, `source/locator` | extracted platform binaries (`.exe` on Windows) |
| `.install-meta.json` | `{ origin: "embedded-seed", content_digest: "<sha256>", installed_at: "<rfc3339>" }` |

### 4.4 `state/locks/`
Advisory lockfiles (R10, R11): `bootstrap.lock`, `<sha256(workspace+repo+branch)>.lock`.
Created on demand; safe to delete when no `work` process runs.

---

## 5. Directory layout of a materialized Work (ADD §3)

```text
<workspace>/
  in-progress/
    <repo-name>_<branch-name>/      # dir_path;  <branch-name> with '/' → '-' for filesystem safety
      worktree/                     # git worktree, HEAD = <branch-name>, started from base
      work-state.json               # canonical snapshot (schema 1)
```

- `<repo-name>` = base name of the source repo's top-level directory.
- `<branch-name>` in the dir is filesystem-sanitized (`/` → `-`); the real branch keeps its
  slashes. Collision of the sanitized dir name with an existing directory → `diag`
  `destination-unavailable` (exit 15) before any mutation.
- The core creates **only** `worktree/` (via `git worktree add`) and `work-state.json`.
  Any other file/dir would come from an Importer (F5) and has no meaning to the core (FR-015).

---

## 6. Entity → requirement traceability

| Entity | Key requirements |
|---|---|
| LocalSource / RepositoryReference | FR-002, FR-003, ADR-0014, ADR-0016 |
| StarterResponse | FR-002, FR-003, FR-004 (seed), ADD §7 |
| BaseBranchChoice | FR-009 |
| BranchConventionSelection | FR-010, FR-011, ADR-0011 |
| DerivedBranchName | FR-011, FR-012, FR-013 |
| CreationAttempt | FR-020, FR-021, SC-003 |
| `work-state.json` / `work` object | FR-015, FR-016, FR-017, FR-018, ADR-0013 |
| `work.db` / `works` | FR-019, ADR-0009, ADR-0013 |
| `config/work.json` | FR-006, FR-007 |
| `registry.json` / `plugins/<alias>` | FR-004, FR-005, ADR-0002, ADR-0003 |
