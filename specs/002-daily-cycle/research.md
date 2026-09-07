# Phase 0 Research: Daily Cycle — Resume and Archive (F2)

**Feature**: `specs/002-daily-cycle/` · **Plan**: [plan.md](./plan.md) · **Date**: 2026-09-06

This document resolves every unknown in the plan's Technical Context. Each entry follows:
**Decision / Rationale / Alternatives considered**. F2 has **no `NEEDS CLARIFICATION`
markers** — the three open questions the spec carried were resolved in `/speckit-clarify`
on 2026-09-06 (id-only targeting, branch left intact, dirty-worktree acknowledgement) and
folded into the spec.

Governing sources: `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md` §3/§5/§9,
`docs/roadmap.md` §3 (F2) / §4, ADR-0013, ADR-0017, ADR-0018, and the F1 design set
(`specs/001-first-local-work/`).

---

## R1 — Command surface and ADR-0017 fit (FR-001, FR-008, FR-030)

**Decision.** F2 adds exactly two public commands and two home-menu rows:

```
work resume [WORK]        # WORK = the opaque id from work-state.json
work archive [WORK...]    # WORK... = one or more opaque ids
```

Both are named daily verbs in ADR-0017's human surface. F2 adds **no** administrative
grammar, **no** new top-level verb, and **no** `--json` (both are mutations; ADR-0017 keeps
`--json` on reads only). The `work` TUI home lists three journeys after F2: "Start a Work"
(F1), "Resume a Work", "Archive Works". Rebuild/reconcile of `work.db` is **not** a public
command (see R11): it is a library plus automatic self-healing, matching F1's decision to
keep `internal/work/verify` behind the CLI.

**Rationale.** ADR-0017 already reserves `resume` and `archive` at the first level and says
"sem alvo e com terminal interativo, `start`, `resume`, `archive` … coletam ou oferecem as
escolhas na TUI; alvos explícitos pulam a seleção correspondente". FR-030 requires both
journeys reachable from the home by keyboard.

**Alternatives considered.**
- *A `work index rebuild` / `work doctor` verb now* — rejected: ADR-0017 forbids expanding
  the surface without need; no F2 user journey requires a human-typed rebuild command, and
  automatic healing (R11) covers the real recovery case. A maintenance verb stays a later
  slice, as F1 decided for `verify`.
- *Fold resume into `work start`* — rejected: different inputs (existing Work vs. new
  source), different guarantees (no plugins, access bump), and ADR-0017 names them separately.

---

## R2 — Snapshot schema evolution: schema 1 → schema 2 (FR-021, ADR-0013)

**Decision.** `work-state.json` moves to **schema 2**, an additive change:

- `work.status` enum widens: `in-progress` → `in-progress | archived`.
- New optional `work.archived_at` (RFC 3339 UTC). Present **iff** `status == "archived"`.
- `work.last_accessed_at` becomes **mutable** (F1 wrote it once at creation; F2 bumps it on
  resume and on archive).
- Everything else (`id`, `slug`, `start_mode`, `starter`, `branch`, `base_branch`,
  `branch_convention`, `created_at`) is unchanged; the `work` object stays closed
  (`additionalProperties: false`); `meta` / `links` unchanged.

Reader rule: `work.Read` accepts `schema ∈ {1, 2}`. A schema-1 document must have
`status == "in-progress"` and no `archived_at` (else it is corrupt). Writer rule:
`work.Write` **always emits schema 2**. Consequence — a resume bump on a never-archived
schema-1 Work rewrites it as schema 2 (a lazy, in-place, superset migration). This is
intentional and documented; there is no separate migration pass and no flag day.

**Rationale.** F1's `data-model.md` already says "bump on any shape change" and marks
`archived` / access bumps as "F2". The change is a strict superset, so old files stay valid
input and new files stay readable by any schema-2 reader. Lazy upgrade avoids a migration
step that would itself need to be transactional across every Work.

**Alternatives considered.**
- *Keep schema 1, just widen the enum in code* — rejected: `additionalProperties: false`
  plus a new field (`archived_at`) is a shape change; silently accepting it under the same
  version number breaks the contract that `schema` identifies the shape.
- *Eager migration of all snapshots on first F2 run* — rejected: turns a read path into a
  bulk mutation, needs its own rollback story, and touches Works the user never resumes.
- *Store `archived` state only in the projection* — rejected by ADR-0013: an archived Work's
  directory must be self-contained and understandable without the global index.

---

## R3 — Recent-access update (FR-003, FR-021, SC-002)

**Decision.** A successful resume performs, in this order, under an advisory lock on the
Work `id` (R17):

1. Read the Work's canonical snapshot; refuse if `status != "in-progress"` (R4).
2. Set `work.last_accessed_at = now` (RFC 3339 UTC, `now` from a single clock read),
   `schema = 2`, and write the snapshot **atomically** (temp + rename, F1 R4). **This is the
   canonical commit point.**
3. `projection.SetAccessed(id, now)` — a single `UPDATE works SET last_accessed_at = ?`.

If step 3 fails after step 2 committed, the run reports success for the reposition but notes
the index is stale and self-heals on the next command (R11); it never rolls the snapshot
back, because the snapshot is authoritative and already correct.

Tie-break for ordering: `ORDER BY last_accessed_at DESC, id DESC` — `id` is a ULID, so the
secondary key is creation order, giving a total, deterministic order even when two Works
carry the same millisecond timestamp.

**Rationale.** Snapshot-first / projection-second is the same ordering F1's `create` uses
("db row last"): the authority is updated first and the derived index trails it, so a crash
between the two leaves the projection reconcilable, never the snapshot torn. `now` read once
keeps the snapshot and the projection identical (acceptance scenario 2).

**Alternatives considered.**
- *Projection-first* — rejected: a crash would leave the index ahead of the authority, the
  one direction reconcile must never trust.
- *Bump on listing / on any touch of the Work* — rejected: FR-003 ties the bump to an actual
  resume selection; `work status` (later) is explicitly read-only (ADR-0017).

---

## R4 — Resume target resolution (FR-005, FR-020, FR-026, edge cases)

**Decision.** An explicit `work resume <target>` accepts **only** the opaque `id`
(`work.id`, a 26-char ULID). Resolution:

1. `projection.Get(id)` after the startup reconcile (R11) has run, so the projection is
   trustworthy.
2. No row → `diag` `target-not-found` (exit 21), no state change.
3. Row with `status == "archived"` → `diag` `target-archived` (exit 22); the command does
   **not** attempt to reposition into a destroyed worktree (un-archiving is out of scope).
4. Row with `status == "in-progress"` → proceed to R3.

No slug / branch / directory targeting is accepted (FR-026, Scope Boundaries). Same-slug
Works are disambiguated **only** by the interactive picker (R13), never on the command line,
so an explicit target is never ambiguous and a mutation never has to guess.

**Rationale.** The spec Assumptions make the `id` "what `work resume` / `work archive` echo
on success and what scripts capture". Resolving through the (already reconciled) projection
keeps resolution O(1); the snapshot scan only happens inside reconcile.

**Alternatives considered.**
- *Accept slug and prompt on ambiguity* — rejected: FR-026 forbids it; a scripted target
  must be unambiguous by construction.
- *Scan snapshots for the id on every resume* — rejected: unnecessary once startup reconcile
  guarantees the projection matches disk.

---

## R5 — Terminal repositioning for resume (FR-004) and "reposition out" on archive

**Decision.** `work resume` reuses the F1 `WORK_CD_FILE` contract verbatim
(`contracts/shell-integration.md`, ADR-0018): on success, after the projection update, if
`WORK_CD_FILE` is set the core writes the absolute worktree path there; if it is unset the
core takes the F1 FR-023 path (exit 0, print the real path, explain how to enable the hook).
No redefinition.

`work archive` gains a **third** `WORK_CD_FILE` write case: when it destroys the worktree
that is the caller's current working directory, it writes the **workspace root** (a safe,
existing, reported location) to `WORK_CD_FILE` so the shell does not end up stranded in a
deleted directory (spec edge case). When the hook is absent it prints, to stderr, that the
session is still inside a now-deleted directory and names the workspace root to `cd` to.

**Rationale.** ADR-0018 §3 already says the core writes `WORK_CD_FILE` for `start` "e,
adiante, `resume`" — resume is the anticipated second writer. The archive "reposition out"
is genuinely new but uses the identical mechanism and only fires for the current-dir Work.

**Follow-up.** A one-line ADR-0018 amendment: add `archive` to the list of commands that may
write `WORK_CD_FILE`, scoped to "the destroyed worktree is the caller's cwd; the value
written is the workspace root". Tracked in the plan's Constitution Check.

**Alternatives considered.**
- *Leave the shell in the deleted directory* — rejected: the spec edge case explicitly
  forbids stranding the session.
- *`cd` to the archived directory* — rejected: it has no `worktree/` and is not a working
  checkout; the workspace root is the least surprising safe landing spot.

---

## R6 — Archive transaction model and crash consistency (FR-012, FR-017, FR-018, SC-005)

**Decision.** `internal/archive` processes a batch of selected Works **one at a time**, each
Work fully independent. Per Work, under an advisory lock on its `id` (R17), an ordered
pipeline with a LIFO compensation stack (F1 R10 pattern):

| # | Step | Compensation (before commit) |
|---|---|---|
| 1 | acquire lock `state/locks/<sha256(id)>.lock` | release |
| 2 | dirty check (`git -C worktree status --porcelain`), unless acknowledged (R7) — if dirty & un-acked: **stop this Work, leave it fully active**, continue the batch | — (no effect yet) |
| 3 | atomic snapshot rewrite **in place** (still under `in-progress/`): `status = archived`, `archived_at = now`, `schema = 2` — **CANONICAL COMMIT POINT** | atomic rewrite back to `status = in-progress`, drop `archived_at` |
| 4 | `git worktree remove --force <in-progress>/<name>/worktree`; a missing worktree dir → `git worktree prune`, treated as success (R9) | `git worktree add <same path> <branch>` (branch still exists) |
| 5 | `os.Rename` `<workspace>/in-progress/<name>` → `<workspace>/archived/<stamp>-<name>` (dedup suffix, R8) | rename back |
| 6 | projection: `MarkArchived(id, row)` — `status`, `archived_at`, `dir_path`, `snapshot_path`, `worktree_path=""` | restore prior row |
| 7 | release lock; if this Work's worktree was the caller's cwd, write `WORK_CD_FILE` (R5) | — |

The **canonical state is decided at step 3**: once the snapshot says `archived`, the Work is
archived, wherever its directory currently sits. Steps 4–6 are the physical follow-through.

**Crash consistency.**
- Crash at 1–2: nothing changed, Work fully active.
- Crash after 3, before 6: the snapshot (atomic) authoritatively says `archived`. A later
  `work` invocation's startup reconcile (R11) makes the projection agree and best-effort
  completes the physical move (worktree prune, dir rename). No torn snapshot; the index is
  reconcilable, never contradicting every snapshot (FR-017 edge case, SC-005).
- Failure at 4 or 5 (genuine error, e.g. permission) **before** we accept it: compensate
  back to fully active; the rest of the batch is unaffected (FR-017).
- Failure at 6 after 3–5 succeeded: the Work is canonically + physically archived; the run
  attempts one targeted `reconcile` for this id, and if that also fails exits with
  `archive-failed` naming the id and telling the user the index will self-heal — it does
  **not** roll a physically-moved, canonically-archived Work back (that would be riskier and
  pointless).

Fault injection: `WORK_FAIL_AT=snapshot|worktree|move|projection` forces the named step to
error, for the transactionality suite.

**Rationale.** Making the atomic snapshot flip the commit point means the one
non-compensatable class of failure (git + filesystem partially done) always lands on a
readable, internally-consistent snapshot whose `status` is the truth, which is exactly what
reconcile is built to trust. Per-Work locks + independent batch members satisfy "a failure
on one Work MUST NOT roll back Works already archived" (FR-017).

**Alternatives considered.**
- *Flip the snapshot last* — rejected: a crash after the worktree is destroyed but before
  the flip leaves an `in-progress` snapshot with no worktree — the state resume can't handle
  and reconcile can't unambiguously classify.
- *One transaction spanning the whole batch* — rejected: FR-017 explicitly wants partial
  success; already-archived Works must not roll back when a later one fails.
- *A write-ahead journal of intended moves* — rejected: over-engineered for a handful of
  steps; the closure stack is local and auditable (same call F1 made in R10).

---

## R7 — Dirty-worktree detection and acknowledgement (FR-013, SC-009)

**Decision.** "Dirty" = `git -C <worktree> status --porcelain` produces any output
(uncommitted tracked changes **or** untracked files). Before step 4 of R6, for each selected
Work:

- **Interactive**: if dirty, show an extra per-Work prompt naming the Work and asking to
  archive it anyway (default **no**). Declining leaves that Work fully active; the rest of
  the batch proceeds.
- **Non-interactive**: a dirty Work is archived only if `--force-dirty` was passed. Without
  it, that Work is skipped (left active) and reported via `diag` `dirty-worktree` context in
  the batch summary; it does **not** fail the whole invocation or the other Works.

`--force-dirty` is a **separate** flag from `--yes` (Assumptions): `--yes` clears the
ordinary confirmation, `--force-dirty` clears the dirty guard. Neither implies the other.

**Rationale.** `git status --porcelain` is the portable, locale-stable dirty check and is
the same signal `git worktree remove` (without `--force`) would refuse on. The spec wants an
*extra* deliberate step for data loss, distinct from the normal confirm.

**Alternatives considered.**
- *Let `git worktree remove` fail on dirty and surface that* — rejected: `--force` is needed
  anyway for a clean removal, and the spec wants the acknowledgement *before* the destructive
  call, per Work, not a blanket flag.
- *Treat untracked-only as clean* — rejected: FR-013 and SC-009 both say "uncommitted or
  untracked".

---

## R8 — Archived-area pathing and collision (FR-012, edge cases, ADD §3)

**Decision.** An archived Work's directory is
`<workspace>/archived/<yyyymmdd>-<repo>_<branch-sanitized>/`, where `<yyyymmdd>` is the
archival date in local time and `<branch-sanitized>` applies F1's `/`→`-` rule. If that path
already exists (same repo, same branch, archived twice on the same day), append the smallest
`-<n>` (`-2`, `-3`, …) that does not collide. The directory is produced by **renaming** the
existing `in-progress/<name>` directory into place (`os.Rename`) — a move within one
workspace filesystem, never a copy — so Importer artifacts ride along and the `worktree/`
subdirectory is already gone (removed in step 4).

**Rationale.** ADD §3 fixes the `archived/<yyyymmdd>-…` shape. A deterministic numeric
suffix resolves the documented same-day collision "without overwriting a previously archived
Work" (edge case) and without a timestamp-to-the-second that would make the path unstable to
predict. Rename (not copy) keeps the operation atomic and cheap and matches F1's
atomic-rename portability posture.

**Alternatives considered.**
- *Append `HHMMSS` instead of `-<n>`* — rejected: still collides on a fast double-archive in
  the same second and is noisier; `-<n>` is deterministic and readable.
- *Copy then delete* — rejected: not atomic, doubles disk use, and can half-copy artifacts.

---

## R9 — Degradation: missing worktree / unreachable source repo (FR-015, Assumptions)

**Decision.** Archive does not treat a missing worktree or a missing source repository as a
hard failure:

- Worktree directory already gone (manually deleted): step 4 runs `git worktree prune` and
  proceeds; the snapshot is still relocated and marked archived.
- Source repository unreachable (`git worktree remove` cannot run at all): the core skips
  the git call, still performs steps 3, 5, 6, and records the archived state; the run's
  summary notes that the worktree could not be removed via Git and names the leftover path,
  if any.

The snapshot in the archived area must be readable later **without** the source repository
or any plugin (FR-015) — which it already is, being self-contained JSON.

**Rationale.** The spec Assumptions: "a missing worktree or missing source repository
degrades to preserving the snapshot and recording the archived state rather than failing
hard." Archiving is fundamentally about preserving the snapshot; the worktree is disposable.

**Alternatives considered.**
- *Fail the Work when the worktree can't be removed cleanly* — rejected by the Assumptions;
  it would trap Works whose source clone was moved or deleted.

---

## R10 — Archiving the current directory's Work (edge case, R5)

**Decision.** If a selected Work's worktree is (or contains) the process's current working
directory, the archive still proceeds (destroys the worktree) and then repositions the
session out via `WORK_CD_FILE` → workspace root (R5), or, with no shell hook, prints an
explicit stderr notice that the shell is now in a deleted directory and names the workspace
root to `cd` to. Detection: compare the resolved cwd against each selected Work's
`worktree_path` prefix.

**Rationale.** Spec edge case: "the worktree is still destroyed and the session is
repositioned out of it (to a safe, reported location), rather than leaving the shell
stranded."

---

## R11 — Rebuild and reconcile the projection from snapshots (FR-022–FR-025, SC-006)

**Decision.** New package `internal/reconcile` with two entry points and one automatic hook:

- `Rebuild(workspaceRoot, dbPath) (Report, error)` — recreate the `works` table from empty,
  walk `<workspace>/in-progress/*/work-state.json` and `<workspace>/archived/*/work-state.json`,
  upsert one row per **readable, schema-valid** snapshot (deriving `repo_name`, `dir_path`,
  `worktree_path`, `snapshot_path` from the on-disk location), and **skip + collect** any
  snapshot that cannot be read or parsed.
- `Reconcile(db, workspaceRoot) (Report, error)` — for every snapshot on disk, upsert/fix
  its row; delete rows whose snapshot no longer exists on disk; **never write a snapshot**.
  Disagreements resolve in favour of the snapshot (stale `last_accessed_at`, missing row,
  row for a vanished Work).
- **Automatic**: `work resume` / `work archive` open the projection through a shared helper
  that runs `Rebuild` when the DB file is absent or unopenable or its `PRAGMA user_version`
  is behind, and otherwise runs a lightweight `Reconcile` pass. At the F1/F2 scale (tens of
  Works) this is a sub-second directory walk; it is the mechanism that makes SC-006 true for
  a real user, not just a test.

A snapshot skipped during rebuild produces a `diag` `snapshot-unreadable` (exit code 25 if
it were ever the top-level operation) **diagnostic** in the report, and rebuild continues
with the rest (FR-024).

**Rationale.** FR-022–FR-025 phrase this as a capability ("MUST be able to rebuild /
reconcile"), and the Independent Test just says "trigger a rebuild" — a library exercised by
tests plus automatic self-healing satisfies both without a new CLI verb (ADR-0017). Treating
the snapshots as the only authority and never rewriting them is FR-023 verbatim.

**Alternatives considered.**
- *A `work index rebuild` command* — rejected for now (R1): no journey needs a human to type
  it; auto-heal covers the corruption case. A maintenance verb can arrive with a later
  slice.
- *Reconcile on every `work` command including `start`* — rejected: F1 `start` is unchanged
  by F2 (Scope Boundaries); F2 only guarantees the behaviour for its own commands.
- *Full scan + rebuild on every resume regardless of DB health* — rejected: wasteful once
  the DB is known-good at the right version; a cheap health check gates it.

---

## R12 — Projection schema v2 migration (FR-025, Assumptions)

**Decision.** `projection.Migrate` becomes forward-only, version-stepped. `user_version 1 → 2`:

```sql
ALTER TABLE works ADD COLUMN archived_at TEXT;            -- nullable, "" / NULL for active
CREATE INDEX IF NOT EXISTS works_status ON works(status); -- active-list / archive-list filter
PRAGMA user_version = 2;
```

`works.status` is already `TEXT` so `archived` needs no DDL. `works.worktree_path` stays
`NOT NULL` and holds `""` for archived Works. `dir_path` / `snapshot_path` follow the Work
to `archived/…`. New queries: `ListActive()` (`WHERE status = 'in-progress' ORDER BY
last_accessed_at DESC, id DESC`), `SetAccessed(id, ts)`, `MarkArchived(id, row)`.

**Rationale.** The spec Assumptions explicitly allow the projection to "gain columns or a
table … but it remains derived and rebuildable". An additive `ALTER TABLE` + index is the
minimal change; a version-stepped `Migrate` means a v1 DB from an F1 install upgrades in
place, and a missing/older DB is simply rebuilt (R11).

**Alternatives considered.**
- *Drop and rebuild on every version change* — rejected: works, but needlessly loses a
  healthy index; the `ALTER` is trivial and rebuild remains the fallback.
- *A separate `archived_works` table* — rejected: doubles every query and the reconcile
  logic for no benefit; one table with a `status` column and an index is enough at this scale.

---

## R13 — Interactive resume / archive pickers (FR-002, FR-009, FR-026, user input)

**Decision.** Both pickers are small hand-rolled Bubble Tea models styled from
`huh.ThemeCharm`, following F1's base-branch picker precedent (`internal/tui/basebranch_picker.go`).
They share a **two-line row renderer**:

```
<name>                       <- line 1: unambiguous — "<repo-name>  <slug>", bold
  <relative-time> • <branch>  <- line 2: dim; "•" is a small bullet
```

- **`work resume`** — single-select. `↑/↓`/`j/k` move, `/` filters, `Enter` selects
  immediately, `q`/`Esc`/`Ctrl-C` cancels (→ `diag` cancelled). Rows are the in-progress
  Works in `last_accessed_at DESC, id DESC` order. Selecting a row returns its `id` to the
  orchestrator (R3).
- **`work archive`** — multi-select. `Space` toggles a row's checkbox, **nothing is
  preselected** (FR-009), `↑/↓` move, `/` filters, `Enter` moves to a **confirmation view**
  listing the checked Works and stating "N worktree(s) will be destroyed; snapshots move to
  archived/; branches are kept", with `Enter` = confirm / `Esc` = back. `Ctrl-C` cancels the
  whole operation. Rows are the active Works in the same recency order.

Disambiguation: line 1 is `repo-name + slug`; line 2's branch usually separates same-slug
Works. If `(repo, slug, branch)` still collide (two source repos with the same directory
name and branch), append `  (<id[:6]>)` to line 1. Explicit CLI targeting is `id`-only
regardless (R4), so ambiguity never reaches a mutation.

**Rationale.** The user's stated design is exactly a checkbox list with a two-line row and
`space`/`enter` semantics. `huh`'s `MultiSelect` renders single-line options only; a custom
model is already the established pattern here (R15 of F1) and gives the two-line row, the
confirmation view, and the recency ordering in one place.

**Alternatives considered.**
- *`huh.NewMultiSelect` with a one-line "`repo slug — 3h ago — branch`" label* — rejected:
  loses the visual hierarchy the user asked for and can't host the confirmation view.
- *Preselect all in archive* — rejected by FR-009 ("MUST NOT preselect any Work").

---

## R14 — Relative-time formatting (user input, SC-008)

**Decision.** New `internal/reltime.Format(d time.Duration) string` with coarse buckets, the
largest unit that fits:

| Range | Rendered |
|---|---|
| < 45 s | `just now` |
| 45 s – 90 s | `a minute ago` |
| 90 s – 45 min | `<n> minutes ago` |
| 45 min – 90 min | `an hour ago` |
| 90 min – 22 h | `<n> hours ago` |
| 22 h – 36 h | `a day ago` |
| ≥ 36 h | `<n> days ago` |

Computed from `now - last_accessed_at`. Days is the largest unit F2 needs (a Work older than
that is still just "<n> days ago" — weeks/months can arrive later if ever wanted). Negative
durations (clock skew) render as `just now`.

**Rationale.** The user's spec says "seconds/minutes/hours/days ago". Coarse, human buckets
(the `moment.js`-style thresholds) read better in a list than exact `2h 14m` and keep rows
narrow. Deterministic and offline.

**Alternatives considered.**
- *Absolute timestamps* — rejected: the user asked for relative; harder to scan for "which
  did I touch last".
- *Exact composite durations (`3h 12m ago`)* — rejected: noisy in a dense list.

---

## R15 — Diagnostics and exit codes (FR-027, FR-028, roadmap §4 Auditability)

**Decision.** Extend the F1 `internal/diag` table. New categories:

| Code | Token | Category |
|---|---|---|
| 21 | `target-not-found` | explicit `id` matches no Work |
| 22 | `target-archived` | resume target is archived (or archive target already archived, when surfaced as fatal) |
| 23 | `dirty-worktree` | a selected Work's worktree is dirty and was not acknowledged (non-interactive, no `--force-dirty`) |
| 24 | `archive-failed` | during archive: worktree removal, archived-area move, or snapshot write failed — the message names which step and which Work |
| 25 | `snapshot-unreadable` | a snapshot could not be read/parsed during rebuild or reconcile (per-Work diagnostic; rebuild continues) |

Reused from F1: `usage` (2), `bootstrap-failed` (16), `cancelled` (20).

**"Nothing to resume / archive" is exit 0**, not an error: the command prints a one-line
note to stderr ("no Works to resume" / "no active Works to archive") and returns nil
(spec edge case: "exits without error state").

FR-019 targets that are **already archived or nonexistent** during `work archive` are
reported **without failing the batch**: they appear in the run summary with the
`target-not-found` / `target-archived` wording but the overall exit code reflects the Works
that *were* processed (0 if the rest succeeded). Exit 24's message text distinguishes the
three archive sub-failures (worktree-remove / archive-move / snapshot-write) per FR-027; a
reviewer who wants three distinct codes can split later — the shared code keeps the table
lean and scripts branch on the token plus the summary.

Success output (FR-028): `work resume` prints `work: resumed <id>` + `work: path <abs>`;
`work archive` prints one `work: archived <id>  (<archived-dir>)` line per Work plus a
trailing `work: archived <k> of <n>` count. Machine-relevant lines → stdout; prompts,
progress, and shell notices → stderr.

**Rationale.** SC / FR-027 / FR-028 want stable codes and messages plus no content leakage;
a central enum (already the F1 mechanism) keeps them consistent and testable in one table
assertion.

**Alternatives considered.**
- *Reuse F1's `materialization-failed` (17) for archive failures* — rejected: different
  operation, different recovery advice; a distinct token is cheap and clearer for scripts.
- *Make "nothing to do" exit 2* — rejected by the spec edge case (no error state).

---

## R16 — Non-interactive contracts (FR-007, FR-011, FR-029)

**Decision.**
- **`work resume`** non-interactive (stdin or stdout not a TTY): requires a resolvable
  `<id>` argument. No target → `diag` `usage` (exit 2), no TUI, no state change. A valid id
  → the normal bump + reposition/notice.
- **`work archive`** non-interactive: requires **both** one or more `<id>` arguments **and**
  `--yes`. Missing ids or missing `--yes` → `diag` `usage` (exit 2), no TUI. `--force-dirty`
  additionally required per dirty Work (R7). With everything present it archives without any
  prompt and returns a stable exit code.
- Neither command ever constructs a TUI when non-interactive (same `tui.IsInteractive`
  gate on stdin **and** stdout as F1).

**Rationale.** Verbatim FR-007 / FR-011 and the roadmap §4 UX gate ("comandos destinados à
automação … nunca tentam abrir TUI").

---

## R17 — Concurrency (FR-021 edge case, Assumptions)

**Decision.** Reuse `internal/lockfile` with a new key scope: `state/locks/<sha256(work-id)>.lock`,
acquired for the duration of a resume's steps 1–3 (R3) and an archive's per-Work steps 1–7
(R6). A second `work` process that wants the same Work blocks briefly (`AcquireContext` with
a short timeout) and, on timeout, fails cleanly with a "another `work` operation is using
this Work" message and no state change. Different Works never contend (distinct keys).

**Rationale.** The spec Assumptions: "Concurrency between `work` invocations is guarded by
the same advisory-lock mechanism F1 introduced." Per-`id` scope is the natural granularity
for F2 (F1 locked per computed branch+dir because the Work didn't exist yet; here it does
and has a stable id).

**Alternatives considered.**
- *A single global `~/.work` lock* — rejected: needlessly serialises archiving Work A while
  resuming Work B.
- *Rely on SQLite's `busy_timeout`* — rejected: it guards the DB write, not the
  snapshot/worktree/dir sequence that must be exclusive.

---

## R18 — Testing strategy (roadmap §4, spec Independent Tests & SCs)

**Decision.**
- **Unit** (`go test ./internal/...`): `reltime` buckets; `worklist` ordering + id
  resolution + disambiguation; `work` schema-2 round-trip and tolerant read of schema 1;
  `projection` v1→v2 migration and the new queries; `reconcile` rebuild/reconcile against a
  synthetic on-disk tree including an unreadable snapshot; `archive` archived-area suffixing;
  `diag` full table.
- **Integration** (`tests/integration/*.txtar` via `testscript`, built `work`, throwaway git
  repos, F1 fake-shell harness):
  - `resume_ordering` — three Works, resume the least-recent, assert it moves to the top and
    a repeated `work resume` reflects it; assert snapshot ↔ projection agree on the new time
    (US1 #1, #2; SC-002).
  - `resume_by_id` — explicit id, no list shown, same bump (US1 #3).
  - `resume_errors` — unknown id → 21; archived id → 22; no target non-interactive → 2;
    inside a worktree, resume "self" is a no-op reposition that still bumps (US1 #4, #5;
    edge cases).
  - `archive_multiselect` — three active Works, multi-select two, confirm, assert exactly
    those two worktrees gone, snapshots readable under `archived/`, third untouched,
    projection no longer lists them active (US2 #1, #2; SC-003).
  - `archive_explicit_confirm` — explicit ids still prompt; declining changes nothing
    (US2 #3).
  - `archive_non_interactive` — ids + `--yes` archives without prompt, stable exit;
    without `--yes` → exit 2, nothing archived (US2 #4).
  - `archive_dirty` — dirty worktree not destroyed without the extra ack / `--force-dirty`;
    the un-acked Work stays active, the rest of the batch proceeds (US2 #5; SC-009).
  - `archive_partial_fail` — `WORK_FAIL_AT=worktree|move|projection` on the second Work of a
    batch: the first stays consistently archived, the second is fully active or fully
    archived, the projection matches disk after the run (US2 #6; SC-005).
  - `archive_already_archived` — a target id that is already archived / nonexistent is
    reported without touching the rest (US2 #7).
  - `archive_current_dir` — archiving the cwd Work repositions the session out to the
    workspace root (edge case).
  - `rebuild_after_db_delete` — populate active + archived Works, `rm work.db`, run a
    command, assert the rebuilt index has identical status classification and recency order
    and that no snapshot's mtime/content changed (US3 #1, #3; SC-006).
  - `reconcile_stale_rows` — hand-edit the DB (stale time, extra row, missing row), run a
    command, assert the index matches the snapshots and no snapshot was rewritten (US3 #2).
  - `rebuild_skips_unreadable` — one corrupt snapshot: it is reported skipped, the rest are
    indexed (US3 #4).
- **Concurrency**: a Go test spawns two `work` processes (or two goroutines over the
  orchestrator with a real lockfile) against one Work; asserts exactly one mutation wins.
- **Regression**: the F1 quickstart S1–S12 run unchanged in CI (SC-007).

**Rationale.** Mirrors F1's `testscript` + fault-injection approach, which the roadmap §4
gates require; every Independent Test and Success Criterion maps to a named scenario.

---

## R19 — Home TUI expansion (FR-030)

**Decision.** `internal/tui/home.go` grows from one row to three: "Start a Work",
"Resume a Work", "Archive Works", each dispatching to the corresponding `cli` entry with no
arguments (so the sub-journey collects its own selection). Keyboard model unchanged
(`↑/↓`/`j/k`, `Enter`, `q`/`Esc`/`Ctrl-C`). The home still shows **no** actions reserved for
later slices (`status`, `import`, `link`, `plugin`, `repository`, `convention`).

**Rationale.** FR-030 ("reach the resume and archive journeys by keyboard, in addition to
the F1 journeys") and the roadmap §4 UX gate ("toda jornada é alcançável pela home TUI").

**Alternatives considered.**
- *A separate "manage Works" submenu* — rejected: three flat rows are still trivially
  scannable; a submenu adds a keystroke for no clarity gain at this size.

---

## Resolved unknowns checklist

| Plan Technical-Context point | Resolved by |
|---|---|
| New command surface & ADR-0017 fit | R1 |
| Snapshot schema 1 → 2 shape & reader/writer rules | R2 |
| Recent-access update mechanism & ordering | R3 |
| Resume target resolution (id-only) | R4 |
| Terminal repositioning for resume / archive-out | R5 + ADR-0018 amendment |
| Archive per-Work transaction & crash consistency | R6 |
| Dirty-worktree detection & acknowledgement | R7 |
| Archived-area path & collision suffix | R8 |
| Missing worktree / unreachable repo degradation | R9 |
| Archiving the current directory's Work | R10 |
| Rebuild & reconcile from snapshots (+ auto-heal) | R11 |
| Projection v1 → v2 migration | R12 |
| Interactive resume / archive pickers | R13 |
| Relative-time formatting | R14 |
| Diagnostics & exit-code table additions | R15 |
| Non-interactive contracts | R16 |
| Concurrency / advisory lock scope | R17 |
| Testing strategy | R18 |
| Home TUI expansion | R19 |

No `NEEDS CLARIFICATION` markers remain.
