---
description: "Task list for Daily Cycle — Resume and Archive (F2) implementation"
---

# Tasks: Daily Cycle — Resume and Archive (F2)

**Input**: Design documents from `specs/002-daily-cycle/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Contract, integration, and fault-injection tests **are required** — `docs/roadmap.md` §4 makes them a cross-cutting exit gate for every slice, and the spec's Success Criteria (SC-002..SC-009) are stated as test outcomes. Unit tests are folded into each implementation task ("with table-driven tests"); integration / fault-injection / concurrency suites are their own tasks.

**Organization**: Tasks grouped by user story. Module: `github.com/gustaborges/work`. Go 1.26, single binary, system `git` as a subprocess. F2 adds **no new dependencies**.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: `[US1]`/`[US2]`/`[US3]` for user-story phases only

## Path Conventions

Single Go project at repo root: `cmd/work/`, `internal/<pkg>/`, `seed/`, `tests/`. Paths below are literal. New F2 packages: `internal/worklist/`, `internal/reltime/`, `internal/resume/`, `internal/archive/`, `internal/reconcile/`, `internal/tui/{resume_picker,archive_picker}.go`, `internal/cli/{resume,archive}.go`.

## Branching (stacked PRs, one branch per phase)

**The PRs are stacked.** Phase 1 branches off the **current branch**
(`feature/002-daily-cycle-p0-specs`) and its PR targets that branch; every later phase branches
off the previous phase's branch and its PR targets that previous branch. Nothing targets
`develop` directly — the whole stack lands on `feature/002-daily-cycle-p0-specs`, which is then
merged onward (its own PR, the user's call on cadence).

**The implementing agent creates and switches to the phase branch BEFORE its first task.**
First action of every phase:

```
git switch <base> && git pull && git switch -c <feature-branch>
```

> **Confirm before you cut.** `/speckit-implement` MUST confirm the base branch and PR target
> with the user at the start of each phase — do not assume the table below. If an earlier
> phase's PR has already merged into `feature/002-daily-cycle-p0-specs`, a later phase may be
> rebased to branch straight off it instead of the (now-merged) intermediate branch.

| Phase | Feature branch | Branch off | PR targets | Planned PR title |
|---|---|---|---|---|
| 1 — Setup | `feature/002-daily-cycle-p1-setup` | `feature/002-daily-cycle-p0-specs` (current) | `feature/002-daily-cycle-p0-specs` | `chore: F2 Phase 1 (Setup) — package skeletons and multi-Work fixtures (spec-002)` |
| 2 — Foundational | `feature/002-daily-cycle-p2-foundational` | `feature/002-daily-cycle-p1-setup` | `feature/002-daily-cycle-p1-setup` | `feat: F2 Phase 2 — Foundational: schema 2, projection v2, reconcile, shared row model (spec-002)` |
| 3 — US1 Resume 🎯 MVP | `feature/002-daily-cycle-p3-us1-resume` | `feature/002-daily-cycle-p2-foundational` | `feature/002-daily-cycle-p2-foundational` | `feat: US1 — resume a Work by recency (F2 Phase 3)` |
| 4 — US2 Archive | `feature/002-daily-cycle-p4-us2-archive` | `feature/002-daily-cycle-p3-us1-resume` | `feature/002-daily-cycle-p3-us1-resume` | `feat: US2 — archive Works while preserving their context (F2 Phase 4)` |
| 5 — US3 Rebuild index | `feature/002-daily-cycle-p5-us3-rebuild` | `feature/002-daily-cycle-p4-us2-archive` | `feature/002-daily-cycle-p4-us2-archive` | `feat: US3 — rebuild the lookup index from canonical snapshots (F2 Phase 5)` |
| 6 — Polish | `feature/002-daily-cycle-p6-polish` | `feature/002-daily-cycle-p5-us3-rebuild` | `feature/002-daily-cycle-p5-us3-rebuild` | `feat: F2 Phase 6 — Polish & cross-cutting concerns (spec-002)` |

US3 (Phase 5) has **no code dependency** on US2 — it needs only `internal/reconcile` from
Phase 2 — but it stacks on Phase 4 for merge order and reuses the archived-Work fixtures added
there.

**Merge gate** (every phase): the phase **Checkpoint** is met and `make lint` + `go test ./...`
are green on `ubuntu-latest` / `macos-latest` / `windows-latest`, with F1 quickstart S1–S12 and
every earlier F2 phase's suite still green. When a phase PR merges into
`feature/002-daily-cycle-p0-specs`, rebase the rest of the stack onto the new tip.

Spec/plan/contract/doc edits stay on `feature/002-daily-cycle-p0-specs`; the per-phase rule
covers implementation code only.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package skeletons and test fixtures every later phase leans on. No behavior.

**Branch**: `git switch feature/002-daily-cycle-p0-specs && git pull && git switch -c feature/002-daily-cycle-p1-setup` before T001. PR targets `feature/002-daily-cycle-p0-specs`.

- [X] T001 Create the F2 package directories with a `doc.go` in each carrying the one-paragraph package purpose from `plan.md` → Project Structure: `internal/worklist/doc.go`, `internal/reltime/doc.go`, `internal/resume/doc.go`, `internal/archive/doc.go`, `internal/reconcile/doc.go`. Empty packages that compile; `go build ./...` stays green.
- [X] T002 [P] Add `tests/fixtures/works.go` (or extend the existing testscript helper set): a helper `seedWorks(src string, slugs ...string)` that runs `work start "$src" --workspace "$WS" --base main --slug <s> --prefix '{slug}' --yes` once per slug with a ≥1 s gap so each `last_accessed_at` is distinct; plus helpers to (a) make a Work's worktree dirty (write an untracked file) and (b) pre-create an `<WS>/archived/<yyyymmdd>-demo_<slug>/` directory for the collision-suffix test. Used by Phases 3–5.
- [X] T003 [P] Document the archive fault-injection contract in `internal/archive/doc.go`: `WORK_FAIL_AT=snapshot|worktree|move|projection` forces the named step of the per-Work pipeline (research R6) to error, for the transactionality suite. Confirm in the PR description that `go.mod`, `Makefile`, and `.github/workflows/ci.yml` need **no** changes for F2 (no new deps, same 3-OS matrix).

**Checkpoint**: `go build ./...` and `go vet ./...` green with the empty new packages; the fixture helpers compile.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Schema 2, projection `user_version = 2`, the reconcile capability, the `gitx`/`diag`
extensions, and the shared row model + relative-time formatter. No user-facing command yet.

**Branch**: `git switch feature/002-daily-cycle-p1-setup && git pull && git switch -c feature/002-daily-cycle-p2-foundational` before T004. PR targets `feature/002-daily-cycle-p1-setup`.

**⚠️ CRITICAL**: No user-story work starts until this phase is done.

- [ ] T004 [P] Extend `internal/diag/diag.go`: add categories + fixed exit codes + stable tokens `target-not-found`(21), `target-archived`(22), `dirty-worktree`(23), `archive-failed`(24), `snapshot-unreadable`(25) exactly per research R15; keep the reused F1 `ok`/`usage`(2)/`bootstrap-failed`(16)/`cancelled`(20). Extend the table test to assert the full F1+F2 set (code ↔ token) and that no message interpolates a path, URL, or env value beyond the user's own `id` argument.
- [ ] T005 [P] Implement `internal/reltime/reltime.go`: `Format(d time.Duration) string` with the exact coarse buckets in research R14 (`just now` / `a minute ago` / `<n> minutes ago` / `an hour ago` / `<n> hours ago` / `a day ago` / `<n> days ago`); negative durations (clock skew) → `just now`. Deterministic, offline. Table-driven tests at every bucket boundary.
- [ ] T006 [P] Extend `internal/gitx/gitx.go`: `StatusPorcelain(worktree) (string, error)` and `IsDirty(worktree) (bool, error)` (`git -C <wt> status --porcelain`; **any** output ⇒ dirty, tracked-modified or untracked-only, per research R7); `WorktreeRemoveForce(dir)` (`git worktree remove --force`); `WorktreePrune(repo)` (`git worktree prune`). Real-temp-repo tests: clean / tracked-modified / untracked-only; remove-force on clean and dirty; prune after a manual worktree-dir delete. **No `BranchDelete` is added to or called on any archive path** (FR-014).
- [ ] T007 Implement snapshot **schema 2** in `internal/work/state.go` per `contracts/work-state.schema.json` + research R2: widen `status` to `in-progress | archived`; add `ArchivedAt` (RFC 3339 UTC), required **iff** `status == "archived"`; `LastAccessedAt` becomes mutable. `Read` accepts `schema ∈ {1, 2}` (a schema-1 doc must be `in-progress` with no `archived_at`, else corrupt); `Write` **always emits `schema: 2`** (lazy in-place superset upgrade). Add helpers `Touch(now)` (bump `last_accessed_at`) and `Archive(now)` (`status=archived`, `archived_at=now`, `last_accessed_at=now`). `Validate()` enforces the schema-2 rules and keeps `work` closed (`additionalProperties: false`, `archived_at` the only new key). Tests: schema-1 read → schema-2 write round-trip; reject schema-1-with-`archived_at`; reject archived-without-`archived_at`; reject `schema: 3`; `additionalProperties` still rejected.
- [ ] T008 Extend `internal/projection/projection.go` to `user_version = 2` per `data-model.md` §3 + research R12: version-stepped `Migrate()` step `1 → 2` = `ALTER TABLE works ADD COLUMN archived_at TEXT` + `CREATE INDEX IF NOT EXISTS works_status ON works(status)` + `PRAGMA user_version = 2`. New ops: `ListActive()` (`WHERE status='in-progress' ORDER BY last_accessed_at DESC, id DESC`), `SetAccessed(id, ts)` (`UPDATE works SET last_accessed_at=? WHERE id=?`), `MarkArchived(id, row)` (`status='archived'`, `archived_at`, new `dir_path`/`snapshot_path`, `worktree_path=''`). Tests: an in-place v1→v2 migration is idempotent and preserves existing rows; a fresh DB opens at v2; the three queries round-trip; ordering is total (equal timestamps break by `id DESC`).
- [ ] T009 [P] Implement `internal/worklist/worklist.go` per `data-model.md` §1.1–1.2 + research R4: `WorkRow` type; `List(db, includeArchived bool) ([]WorkRow, error)` in `last_accessed_at DESC, id DESC` order; `Resolve(rows, rawID) (WorkRow, Outcome)` — matches **only** `work.id` (FR-026), `Outcome ∈ {resolved, not-found, archived}`; `display_name` = `"<repo_name>  <slug>"`, plus `"  (" + id[:6] + ")"` **iff** `(repo_name, slug, branch)` collides with another row; `relative_time` via `reltime.Format`. Depends on T005, T008. Table tests: recency ordering + ULID tie-break, id-only resolution, `archived` outcome, same-slug disambiguation (branch on line 2, then `id[:6]`).
- [ ] T010 Implement `internal/reconcile/reconcile.go` per `contracts/index-rebuild.md` + research R11: `Rebuild(workspaceRoot, dbPath) (Report, error)` — open/create the DB, `DROP`+recreate `works`, `PRAGMA user_version = 2`, scan **non-recursively** `<ws>/in-progress/*/work-state.json` and `<ws>/archived/*/work-state.json`, derive `dir_path` / `snapshot_path` / `worktree_path` / `repo_name` from where the file sits (table in the contract), `Upsert` one row per readable schema-valid snapshot, **skip + collect** `{path, reason}` for any unreadable / unparseable / schema-not-in-{1,2} / `Validate`-failing snapshot (not an error). `Reconcile(db, workspaceRoot) (Report, error)` — `Upsert` every on-disk snapshot's row (snapshot values win — fixes stale `last_accessed_at`, wrong `status`, missing rows), `Delete` every `works` row whose id is not on disk (`report.dropped++`), **never write a snapshot**. `Report{indexed, dropped, skipped []SkippedSnapshot, snapshotsWritten int}` — `snapshotsWritten` is always `0`. Depends on T007, T008. Tests over a synthetic on-disk tree (active + archived + one corrupt snapshot): active/archived classification and `last_accessed_at DESC, id DESC` order match a reference; ghost row dropped; stale time corrected from the snapshot; assert `snapshotsWritten == 0` by comparing every snapshot's mtime **and** bytes before/after.
- [ ] T011 Implement the shared projection-open helper `internal/projection.OpenReconciled(workspaceRoot) (*DB, reconcile.Report, error)` (or in `internal/reconcile`) per `contracts/index-rebuild.md` → *Automatic invocation*: run `reconcile.Rebuild` when the DB file is absent, `sql.Open`/`PRAGMA` fails, or `user_version < 2`; otherwise run a lightweight `reconcile.Reconcile`; return the `Report` so callers can print skipped-snapshot notes. Depends on T010. Tests: missing DB → rebuilt at v2; healthy v2 DB → reconcile only (no drop); v1 DB → rebuilt.
- [ ] T012 [P] Extend `internal/work/verify/verify.go` for archived Works per `data-model.md` §3.3: for `status='archived'` assert `worktree_path == ""`, `dir_path` under `<workspace>/archived/`, the snapshot at `snapshot_path` has `status=='archived'` with a well-formed `archived_at`, and run **no** worktree/branch check; keep every F1 check for `status='in-progress'`; assert `works.status` / `last_accessed_at` / `archived_at` equal the snapshot (snapshot wins). Tests with an archived-Work fixture.

**Checkpoint**: `internal/{diag,reltime,gitx,work,projection,worklist,reconcile}` all have green unit tests on all 3 OSes. Deleting a hand-built `work.db` and calling `OpenReconciled` reproduces the index from snapshots with 0 snapshot writes.

---

## Phase 3: User Story 1 - Resume a Work by recency (Priority: P1) 🎯 MVP

**Branch**: `git switch feature/002-daily-cycle-p2-foundational && git pull && git switch -c feature/002-daily-cycle-p3-us1-resume` before T013. PR targets `feature/002-daily-cycle-p2-foundational`.

**Goal**: `work resume` lists in-progress Works most-recently-accessed first, lets the user pick one (or names one by its opaque `id` and skips the list), bumps that Work's `last_accessed_at` in the snapshot **and** the projection, and repositions the terminal into its worktree via the F1 `WORK_CD_FILE` contract (same fallback when unavailable).

**Independent Test**: With two or more Works created via F1, `work resume` shows them in last-access order; selecting the least-recent one repositions the session into its worktree and moves it to the top of a repeated `work resume`; the snapshot and `work.db` agree on the new time. Repeat with an explicit `id` and confirm no list is shown. (quickstart S1–S4.)

### Tests for User Story 1

- [ ] T013 [P] [US1] Integration test `tests/integration/resume_ordering.txtar` (pty via the F1 harness): three Works (`seedWorks`), `work resume` lists `charlie, bravo, alpha` top-to-bottom with the two-line row (`demo  <slug>` / `  <relative-time> • <branch>`); select `alpha` → exit 0, stdout exactly `work: resumed <id>` + `work: path <WS>/in-progress/demo_alpha/worktree`; `demo_alpha/work-state.json` now `schema: 2` with `last_accessed_at` newer than bravo's/charlie's; `work.db` `last_accessed_at` for alpha equals the snapshot; a second `work resume` lists `alpha, charlie, bravo`; with the fake shell active the session cwd is the alpha worktree. (US1 #1, #2; SC-001, SC-002, SC-008; FR-002, FR-003, FR-004.)
- [ ] T014 [P] [US1] Integration test `tests/integration/resume_by_id.txtar`: `work resume "$id_alpha"` → **no** TUI rendered, the same two stdout lines, the same bump as the interactive path. (US1 #3; FR-005.)
- [ ] T015 [P] [US1] Integration test `tests/integration/resume_errors.txtar`: unknown id → exit 21; `work resume < /dev/null` (non-interactive, no target) → exit 2, no TUI; an archived id → exit 22; each prints one `error: <token>: <message>` on stderr and performs **zero** snapshot + zero projection writes. From inside `demo_alpha/worktree`, `work resume` with no arg still lists `alpha` and selecting it is a valid no-op reposition that still bumps `last_accessed_at`. (US1 #4, #5; FR-005, FR-007, FR-020; edge cases.)

### Implementation for User Story 1

- [ ] T016 [US1] Implement `internal/tui/resume_picker.go`: a small single-select Bubble Tea model styled from `huh.ThemeCharm` (the `internal/tui/basebranch_picker.go` precedent), rendering the **shared two-line row** (line 1 `"<repo-name>  <slug>"` bold, line 2 `"  <relative-time> • <branch>"` dim; `(id[:6])` appended to line 1 on a `(repo,slug,branch)` collision — from `worklist.WorkRow.display_name`). Keys: `↑/↓` + `j/k` move, `/` filter, `Enter` select, `q`/`Esc`/`Ctrl-C` cancel → `diag` `cancelled`(20). Input: `[]worklist.WorkRow`; output: the chosen `id`. Unit-test update/view for key handling, filtering, and cancel.
- [ ] T017 [US1] Implement `internal/resume/resume.go` — the resume orchestrator per research R3: under an advisory lock `state/locks/<sha256(id)>.lock` (reuse `internal/lockfile`, short `AcquireContext` timeout → clean "another work operation is using this Work" failure, no state change): (a) read the snapshot; `status != "in-progress"` → `diag` `target-archived`(22); (b) `state.Touch(now)` (one clock read) + atomic `work.Write` — **canonical commit**; (c) `projection.SetAccessed(id, now)` as a post-commit trailer — a failure here reports success for the reposition, notes the index is stale (self-heals next command), and **never** rolls the snapshot back. Returns the absolute worktree path. Depends on T007, T008. Unit tests: happy bump (snapshot == projection, single `now`), archived refusal, `SetAccessed` failure still succeeds, lock contention fails cleanly.
- [ ] T018 [US1] Implement `internal/cli/resume.go` per `contracts/cli-work-resume.md`: `work resume [WORK]`; `--json` → exit 2; `gitx.Preflight()` failure → `bootstrap-failed`(16); open the projection via `projection.OpenReconciled` (T011) and print any `note: snapshot-unreadable: …` lines to stderr. **Interactive + no target**: `worklist.List(db, false)` (empty → `note: no Works to resume` on stderr, exit 0) → `resume_picker` → `resume.Run`. **Explicit target**: `worklist.Resolve` (id-only) → `not-found`(21) / `archived`(22) / proceed; no list shown. **Non-interactive + no resolvable target**: `usage`(2), no TUI, no state change. On success: `shellintegration.WriteTargetPath(worktree)` when `WORK_CD_FILE` is set, else `shellintegration.ReportNoIntegration(...)` (F1 FR-023 path, verbatim), then print `work: resumed <id>` + `work: path <abs>` to stdout. Runs no plugin/extension code; touches no other Work. Depends on T009, T016, T017.
- [ ] T019 [US1] Register `resume` in `internal/cli/root.go` and add a **"Resume a Work"** row to `internal/tui/home.go` (home now lists "Start a Work", "Resume a Work"), dispatching to `cli` resume with no arguments; keyboard model unchanged; still hides `status`/`import`/`link`/`plugin`/`repository`/`convention`. Update `root_test.go` / `home_test.go`. *(Both files are also edited by Phase 4 T031 — see Dependencies & Execution Order.)*

**Checkpoint**: quickstart S1–S4 pass on all 3 OSes. MVP demoable: F1 create ×N → `work resume` → land in a chosen checkout, recency order updated. Snapshot ↔ projection agree (SC-002).

---

## Phase 4: User Story 2 - Archive Works while preserving their context (Priority: P2)

**Branch**: `git switch feature/002-daily-cycle-p3-us1-resume && git pull && git switch -c feature/002-daily-cycle-p4-us2-archive` before T020. PR targets `feature/002-daily-cycle-p3-us1-resume`.

**Goal**: `work archive` multi-selects active Works, shows a confirmation, then for each: detects a dirty worktree (extra ack required), atomically flips the snapshot to `archived` (canonical commit), destroys the Git worktree (branch left intact), renames the directory under `<workspace>/archived/<yyyymmdd>-…`, and updates the projection. Each per-Work move is transactional; a batch member's failure never rolls back an earlier member. Explicit `id`s skip only the selection, never the confirmation.

**Independent Test**: With several Works, `work archive` → multi-select a subset → confirm → exactly those worktrees are gone, their snapshots are readable under `archived/` with `status: archived`, still-active Works are untouched, the projection no longer lists the archived ones, and the source-repo branches still exist. Repeat with explicit `id`s and confirm the confirmation still appears. (quickstart S5–S10.)

### Tests for User Story 2

- [ ] T020 [P] [US2] Integration test `tests/integration/archive_multiselect.txtar` (pty): 3 active Works, **none preselected**; `Space` on alpha + charlie, `Enter` → confirmation view naming both and stating "2 worktree(s) will be destroyed. Snapshots move to `<workspace>/archived/`. Branches are kept."; decline (`Esc` then `Ctrl-C`) → **nothing changed**; re-run, confirm → `in-progress/demo_alpha` and `demo_charlie` gone; `archived/<yyyymmdd>-demo_alpha/work-state.json` + `…-demo_charlie/…` readable, `schema: 2`, `status: "archived"`, `archived_at` set, no `worktree/`; `demo_bravo` untouched; `work.db` alpha/charlie `status=archived` + `worktree_path=""`, bravo unchanged; `work resume` no longer lists alpha/charlie; `git -C "$src" branch --list alpha charlie` still shows both; stdout the two `work: archived <id>  (<dir>)` lines + `work: archived 2 of 2`. (US2 #1, #2; SC-003, SC-004; FR-009, FR-010, FR-012, FR-014, FR-015, FR-016.)
- [ ] T021 [P] [US2] Integration test `tests/integration/archive_explicit_confirm.txtar` (pty): `work archive "$id_bravo"` still shows the confirmation view; declining → exit 20, bravo still active, nothing moved. (US2 #3; FR-010.)
- [ ] T022 [P] [US2] Integration test `tests/integration/archive_non_interactive.txtar`: `work archive "$id_bravo"` (no `--yes`, stdin not a TTY) → exit 2, nothing archived; `work archive "$id_bravo" --yes` → exit 0, archived with no prompt; `work archive "$id_bravo" --json` → exit 2. (US2 #4; FR-011.)
- [ ] T023 [P] [US2] Integration test `tests/integration/archive_dirty.txtar`: a Work with an untracked file in its worktree; `work archive "$id_dirty" "$id_clean" --yes` → `clean` archived, `dirty` **left active** (worktree + snapshot intact) with `note: <id_dirty>: worktree has uncommitted or untracked changes — left active (use --force-dirty)` on stderr, exit 0; re-run with `--force-dirty` → `dirty` archived too; interactive form: an extra per-Work prompt (default no) for `dirty`, declining leaves it active and archives the rest. (US2 #5; SC-009; FR-013.)
- [ ] T024 [P] [US2] Integration test `tests/integration/archive_partial_fail.txtar`: `WORK_FAIL_AT=worktree work archive "$id_x" "$id_y" --yes` → `x` fully archived (dir under `archived/`, snapshot + row `archived`), `y` **fully active** (worktree + snapshot + row `in-progress`) — never half-moved; the projection matches on-disk reality after the run (FR-018). Repeat `WORK_FAIL_AT=move` (→ `y` fully active) and `WORK_FAIL_AT=projection` (→ `y` canonically archived on disk; the run self-heals the row or exits 24 naming `y`, and the next `work` command's reconcile brings the row into agreement). (US2 #6; SC-005; FR-017, FR-018.)
- [ ] T025 [P] [US2] Integration test `tests/integration/archive_already_archived.txtar`: `work archive "$id_alpha_archived" "$id_bravo_active" 01000000000000000000000000 --yes` → `bravo` archived; `note: <id>: already archived` and `note: <id>: not found` on stderr; neither aborts the run; exit 0; `work: archived 1 of 1`. (US2 #7; FR-019.)
- [ ] T026 [P] [US2] Integration test `tests/integration/archive_current_dir.txtar` (fake shell): `eval "$(work shell-init bash)"`, `cd "$WS"/in-progress/demo_bravo/worktree`, `work archive "$id_bravo" --yes` → harness asserts cwd is now `"$WS"` (workspace root), not a deleted directory. Without the hook: exit 0, stderr notes the shell is in a now-deleted directory and names `$WS` to `cd` to; no false `cd` claim. (edge case; ADR-0018 amendment; research R5, R10.)

### Implementation for User Story 2

- [ ] T027 [US2] Implement `internal/tui/archive_picker.go` per research R13: a multi-select Bubble Tea model reusing the shared two-line row with a `[ ]`/`[x]` checkbox. `Space` toggle (**nothing preselected** — FR-009), `↑/↓` + `j/k` move, `/` filter, `Enter` → **confirmation view** listing the checked Works and stating "N worktree(s) will be destroyed. Snapshots move to `<workspace>/archived/`. Branches are kept." with `Enter` = confirm / `Esc` = back to the list; `Ctrl-C` cancels the whole operation (→ `cancelled`(20)); `Enter` with nothing checked is a no-op. Unit-test toggle, empty-`Enter` no-op, confirm/back transitions, cancel.
- [ ] T028 [US2] Implement `internal/archive/pathing.go` per research R8: `ArchiveDir(workspaceRoot, repoName, branchSanitized string, date time.Time) string` = `<ws>/archived/<yyyymmdd>-<repo>_<branch-sanitized>`, appending the smallest free `-<n>` (`-2`, `-3`, …) when that path already exists; reuse F1's `/`→`-` branch sanitization. Table tests: no collision; `-2` then `-3` on same-day, same-repo, same-branch repeats; a pre-existing unrelated dir is not overwritten.
- [ ] T029 [US2] Implement `internal/archive/archive.go` — the per-Work transactional pipeline + batch driver per research R6, with a LIFO compensation stack, under `state/locks/<sha256(id)>.lock`: (1) acquire lock ↩ release; (2) dirty check via `gitx.IsDirty` unless acknowledged (interactive per-Work ack) or `--force-dirty` — dirty & un-acked ⇒ leave the Work **fully active**, record `outcome=left-active`, continue the batch; (3) atomic snapshot rewrite in place `state.Archive(now)` — **CANONICAL COMMIT POINT** ↩ atomic rewrite back to `in-progress`, drop `archived_at`; (4) `gitx.WorktreeRemoveForce(<src>/worktree)`; a missing worktree dir → `gitx.WorktreePrune`, treated as success (research R9) ↩ `gitx.WorktreeAdd(<same path>, branch, branch)`; (5) `os.Rename` `<ws>/in-progress/<name>` → `ArchiveDir(...)` ↩ rename back; (6) `projection.MarkArchived(id, row)` ↩ restore the prior row. Batch: process Works one at a time, **independent** locks + stacks — a failure on one never rolls back an earlier one (FR-017). Post-commit failure at step 6 → one targeted `reconcile` for that id; if that also fails → `diag` `archive-failed`(24) naming the step and the `id` (do **not** roll a physically-moved, canonically-archived Work back). Degradation (research R9): source repo unreachable → skip the git call, still do steps 3, 5, 6, note the leftover worktree path. `WORK_FAIL_AT=snapshot|worktree|move|projection` forces the named step to error. Depends on T006, T007, T008, T028. Unit tests: each `WORK_FAIL_AT` value; dirty-skip; missing-worktree degradation; batch independence (Work 2 fails, Work 1 stays archived) — every case asserts the Work is **fully** in one state and leaves zero residue.
- [ ] T030 [US2] Implement `internal/cli/archive.go` per `contracts/cli-work-archive.md`: `work archive [WORK...] [--yes] [--force-dirty]`; `--json` → exit 2; `gitx.Preflight`; open the projection via `projection.OpenReconciled` (print skipped-snapshot notes). **Interactive + no targets**: `worklist.List(db, false)` (empty → `note: no active Works to archive`, exit 0) → `archive_picker` → confirmation → batch. **Explicit targets**: resolve id-only; drop **and report** unknown / already-archived ids as `note:` lines (do not fail the batch — FR-019); the applicable confirmation is still required (interactive → the confirmation view; non-interactive → `--yes`). **Non-interactive**: requires ids **and** `--yes` (missing → exit 2, nothing archived); a dirty Work without `--force-dirty` is skipped + reported, exit stays 0 as long as the eligible Works archived. Single-explicit-target fatal edge codes: `target-archived`(22) / `dirty-worktree`(23) only when that one target is the entire job. Reposition-out: if a destroyed worktree was (or contained) the caller's cwd, write the **workspace root** to `WORK_CD_FILE`, else print the stderr notice (research R5/R10); never claim a `cd` happened. stdout: one `work: archived <id>  (<absolute-archived-dir>)` per archived Work + a trailing `work: archived <k> of <n>`; Works left active / reported → stderr only. Runs no plugin/extension code; touches no non-selected Work, config, or bootstrap state. Depends on T009, T027, T029.
- [ ] T031 [US2] Register `archive` in `internal/cli/root.go` and add the **"Archive Works"** row to `internal/tui/home.go` (home now lists all three journeys, in the order "Start a Work", "Resume a Work", "Archive Works", per `contracts/cli-work-home.md`); non-interactive `work` with no args → a one-line summary naming `work start`, `work resume`, `work archive` (and `work --help`) on stderr, exit 2. Update `root_test.go` / `home_test.go`. *(Both files were also edited by Phase 3 T019 — whichever phase merges second rebases; see Dependencies.)*

**Checkpoint**: quickstart S5–S10 pass on all 3 OSes. US1 + US2 both pass independently. Fault injection (`WORK_FAIL_AT`) demonstrates SC-005; branches survive archiving (FR-014).

---

## Phase 5: User Story 3 - Rebuild the lookup index from canonical snapshots (Priority: P3)

**Branch**: `git switch feature/002-daily-cycle-p4-us2-archive && git pull && git switch -c feature/002-daily-cycle-p5-us3-rebuild` before T032. PR targets `feature/002-daily-cycle-p4-us2-archive`. (US3 has no code dependency on US2 — only `internal/reconcile` from Phase 2 — but it stacks on Phase 4 and reuses its archived-Work fixtures.)

**Goal**: A deleted, truncated, or inconsistent `work.db` is fully rebuilt from the Works' canonical snapshots (active **and** archived), the snapshots are treated as authoritative and never rewritten, an unreadable snapshot is skipped with a diagnostic rather than aborting the rebuild, and afterwards `work resume` / `work archive` behave exactly as before.

**Independent Test**: Create and access several Works (some archived), delete or corrupt `work.db`, trigger a rebuild (run any F2 command), and verify the index lists every Work with correct status and recency order and that **no** canonical snapshot was modified. (quickstart S11–S13.)

> The `reconcile` package and the auto-heal helper are built in Phase 2 (T010, T011). Phase 5
> owns the exhaustive behavioural coverage, the unreadable-snapshot diagnostic surfacing, and
> the hardening the Success Criteria demand.

### Tests for User Story 3

- [ ] T032 [P] [US3] Integration test `tests/integration/rebuild_after_db_delete.txtar`: populate active + archived Works; `find "$WS" -name work-state.json -exec sha256sum {} + | sort > before` and `sqlite3 work.db 'SELECT id,status,last_accessed_at FROM works ORDER BY 3 DESC,1 DESC' > order-before`; `rm work.db`; `work resume < /dev/null` (triggers the rebuild, then exits 2 for no target — fine); re-capture → both diffs empty (0 snapshots modified; identical classification + order); `PRAGMA user_version` == 2. (US3 #1, #3; SC-006; FR-022, FR-025.)
- [ ] T033 [P] [US3] Integration test `tests/integration/reconcile_stale_rows.txtar`: without touching any snapshot, `UPDATE works SET last_accessed_at='2000-01-01T00:00:00Z'` for one id and `INSERT` a ghost row whose snapshot does not exist on disk, and delete one real row; run a command → the stale `last_accessed_at` is corrected to the snapshot's value, the ghost row is dropped, the missing row is re-added, and no snapshot file is modified (re-check the S11 fingerprint). (US3 #2; FR-023.)
- [ ] T034 [P] [US3] Integration test `tests/integration/rebuild_skips_unreadable.txtar`: `echo 'not json' > demo_bravo/work-state.json`; `rm work.db`; run a command → stderr has `note: snapshot-unreadable: <path>: <reason>`; `SELECT count(*) FROM works` shows every **other** Work indexed; the rebuild did not abort. (US3 #4; FR-024.)

### Implementation for User Story 3

- [ ] T035 [US3] Harden `internal/reconcile`: every skip reason (`unreadable`, `unparseable`, `schema ∉ {1,2}`, `Validate` failure) is collected into `Report.skipped` and is non-fatal; `Reconcile` leaves an already-present row untouched when its snapshot has become unreadable (never invents a row, never deletes on a read error); `repo_name` is derived as the `<name>` prefix before the **last** `_` (matches F1's derivation) for both active and archived paths. Extend the unit tests for each skip reason and the "unreadable but a row already exists" case.
- [ ] T036 [US3] Surface skipped-snapshot notes through the CLI: `projection.OpenReconciled` returns `Report.skipped`; `internal/cli/resume.go` and `internal/cli/archive.go` print each as `note: snapshot-unreadable: <path>: <reason>` on stderr before proceeding with the Works that indexed. Depends on T011, T018, T030. Covered by T034.
- [ ] T037 [US3] Verify-parity test (`internal/reconcile` or `tests/integration`): after a `Rebuild`, `work/verify` (T012) passes for every active and archived Work, and `worklist.List` order is byte-identical to a capture taken before the DB was deleted. (SC-006.)

**Checkpoint**: quickstart S11–S13 pass on all 3 OSes. `work.db` is demonstrably disposable — delete it, run any F2 command, get an identical index with 0 snapshot writes.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Branch**: `git switch feature/002-daily-cycle-p5-us3-rebuild && git pull && git switch -c feature/002-daily-cycle-p6-polish` before T038. PR targets `feature/002-daily-cycle-p5-us3-rebuild`.

- [ ] T038 [P] Concurrency test `internal/resume/concurrency_test.go` (or `tests/integration`): two `work` invocations (real processes, or two goroutines over the `resume` / `archive` orchestrators with a real `lockfile`) racing on **one** Work — `resume`+`resume` and `resume`+`archive`: exactly one mutation wins, the other fails cleanly on the advisory-lock timeout with "another work operation is using this Work" and no state change; the snapshot is never torn and the index never contradicts every snapshot. Different Works never contend. (research R17; spec edge case.)
- [ ] T039 [P] Integration test `tests/integration/home_reachability.txtar` (pty) per `contracts/cli-work-home.md`: `work` no-args lists exactly "Start a Work", "Resume a Work", "Archive Works"; arrowing to "Resume a Work" + `Enter` opens the resume picker, "Archive Works" + `Enter` the archive picker; `q` exits 0 with no state change; non-interactive `work` → exit 2 with the one-line summary. (FR-030.)
- [ ] T040 [P] F1 regression: wire `specs/001-first-local-work/quickstart.md` S1–S12 to run unchanged in CI alongside the F2 suites; confirm `work start` behaviour and output are byte-identical to release 0.1. (SC-007.)
- [ ] T041 [P] `docs/adr/adr-0018-work-shell-init.md` — one-paragraph amendment: add `work archive` to the commands that may write `WORK_CD_FILE`, scoped to "the destroyed worktree is the caller's current directory; the value written is the workspace root". Mechanism and protocol unchanged. (plan Constitution Check tracked item; research R5.)
- [ ] T042 [P] `README.md`: add `work resume` and `work archive` walkthroughs (interactive + non-interactive), the `--yes` and `--force-dirty` flags, the `<workspace>/archived/<yyyymmdd>-…` layout, the schema-2 note (lazy in-place upgrade; readers still accept schema 1), that `work.db` is disposable (auto-rebuilt from snapshots), and the extended exit-code table (adds 21–25).
- [ ] T043 Run `quickstart.md` S1–S13 on Linux; record results and any deviations in `specs/002-daily-cycle/validation-log.md`. Windows/macOS left to the CI matrix.
- [ ] T044 `make lint` clean (`gofmt -l` empty, `go vet`, `staticcheck`); `--json` rejected (exit 2) and hidden from help on `work resume` / `work archive`; remove any dead Phase 1 scaffolding; add the `Supersedes specs/001-first-local-work/contracts/cli-work-home.md` note in `internal/tui/home.go` pointing at `specs/002-daily-cycle/contracts/cli-work-home.md`.

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: needs Setup. **Blocks all user stories.**
- **US1 (Phase 3)**: needs Foundational. This is the MVP.
- **US2 (Phase 4)**: needs Foundational; extends `internal/cli/root.go` + `internal/tui/home.go` that US1 also edited (T019 → T031), which is fine since Phase 4 stacks on Phase 3.
- **US3 (Phase 5)**: needs Foundational only (`internal/reconcile` from T010/T011). No code overlap with US2; stacked on Phase 4 for merge order and reuses its archived-Work fixtures.
- **Polish (Phase 6)**: stacked on Phase 5; exercises all three stories together.

### Within Foundational (Phase 2)

- T004, T005, T006 are mutually independent → parallel.
- T007 → T008 (state helpers before the projection ops that mirror them, for the shared round-trip test).
- T009 needs T005 + T008. T010 needs T007 + T008. T011 needs T010. T012 needs T008 (+ existing F1 verify).

### Within US1 (Phase 3)

- Tests T013–T015 are independent → parallel.
- T016 needs T009. T017 needs T007 + T008. T018 needs T009 + T016 + T017. T019 needs T018.

### Within US2 (Phase 4)

- Tests T020–T026 are independent → parallel.
- T027 needs T009. T028 independent. T029 needs T006 + T007 + T008 + T028. T030 needs T009 + T027 + T029. T031 needs T030.

### Within US3 (Phase 5)

- Tests T032–T034 are independent → parallel.
- T035 hardens T010. T036 needs T011 + T018 + T030. T037 needs T012.

### Parallel opportunities

- Phase 1: T002, T003 together (after T001).
- Phase 2: T004 + T005 + T006 together; then T009 + T012 in parallel once T008 lands; T010/T011 sequential.
- Each user story's test tasks (T013–T015, T020–T026, T032–T034) run in parallel.
- Phase 6: T038–T042 are all independent → parallel.

---

## Parallel Example: User Story 2

```bash
# Integration tests for US2 (all independent .txtar files):
Task: T020 tests/integration/archive_multiselect.txtar
Task: T021 tests/integration/archive_explicit_confirm.txtar
Task: T022 tests/integration/archive_non_interactive.txtar
Task: T023 tests/integration/archive_dirty.txtar
Task: T024 tests/integration/archive_partial_fail.txtar
Task: T025 tests/integration/archive_already_archived.txtar
Task: T026 tests/integration/archive_current_dir.txtar

# Then the independent implementation pieces:
Task: T027 internal/tui/archive_picker.go
Task: T028 internal/archive/pathing.go
```

---

## Implementation Strategy

### MVP first (User Story 1 only)

1. Phase 1: Setup → 2. Phase 2: Foundational (CRITICAL — blocks everything) →
3. Phase 3: US1 Resume → **STOP and validate** against quickstart S1–S4 → demo.

`work resume` on top of F1's `work start` is the smallest slice that turns "create a Work"
into "work across days".

### Incremental delivery

1. Setup + Foundational → foundation ready (`work.db` v2, schema 2, reconcile).
2. + US1 Resume → test (S1–S4) → demo (MVP).
3. + US2 Archive → test (S5–S10) → demo.
4. + US3 Rebuild → test (S11–S13) → demo (`work.db` proven disposable).
5. Polish → concurrency, home reachability, F1 regression, docs, ADR-0018 amendment.
6. The whole stack lands on `feature/002-daily-cycle-p0-specs`, which merges onward to `develop`; then `release/0.2` is cut from `develop`, merged to `master`, tagged.

### Story independence

- **US1** delivers resume with a two-row home; testable with only F1-created Works.
- **US2** delivers archive with the full three-row home; depends on nothing from US1 beyond the
  shared `root.go`/`home.go` file it also edits (and Phase 4 stacks cleanly on Phase 3).
- **US3** is exercised by a recovery path (delete `work.db`, run a command); independent of the
  daily journeys beyond the shared `reconcile` package.

---

## Notes

- `[P]` = different files, no dependency on an incomplete task.
- `[Story]` label maps a task to US1/US2/US3 for traceability; Setup/Foundational/Polish carry none.
- F2 adds **no new dependencies** and no new canonical store; the projection stays derived.
- The Git branch a Work created is **never** deleted by F2 (FR-014) — no `git branch -D` on any archive path.
- The canonical snapshot is the sole authority for `status` and `last_accessed_at`; the projection always trails it and is rebuildable (FR-021, ADR-0013).
- Commit after each task or logical group (Conventional Commit + spec-002 footer via the repo's commit skill).
- Stop at any checkpoint to validate a story independently.
