# Implementation Plan: Daily Cycle — Resume and Archive (F2)

**Branch**: spec/plan/doc work on the F2 spec branch (`feature/002-daily-cycle-p0-specs`);
F2 implementation uses one `feature/002-daily-cycle-p<n>-*` branch per phase (see
**Branching Strategy**) | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/002-daily-cycle/spec.md`

## Summary

F2 makes a Work outlive the day it was created. It adds two daily commands on top of the
F1 walking skeleton:

- **`work resume [id]`** — lists existing in-progress Works ordered by last access, lets the
  user pick one (or names one by its opaque `id`), bumps that Work's `last_accessed_at` in
  the canonical snapshot and the projection, and repositions the terminal into its worktree
  using the same shell-integration contract F1 defined for `work start`.
- **`work archive [id...]`** — multi-selects active Works, shows a confirmation screen,
  then for each Work destroys its Git worktree (leaving the branch ref intact), relocates
  its `work-state.json` (and any imported artifacts) into `<workspace>/archived/`, and flips
  the snapshot's `status` to `archived`. Each per-Work move is transactional; a dirty
  worktree is never destroyed without an extra acknowledgement.

It also proves the SQLite projection is genuinely disposable: `work.db` can be **rebuilt**
from scratch and **reconciled** against the canonical snapshots (active and archived), with
the snapshots always authoritative and never rewritten to match the index.

Technical approach: extend the existing single Go binary. `work-state.json` moves to
**schema 2** (additive: `status` gains `archived`, a new optional `archived_at`, and
`last_accessed_at` becomes mutable); readers still accept schema-1 files and upgrade them
lazily on the next write. Two new transactional orchestrators (`internal/resume`,
`internal/archive`) follow F1's `internal/create` compensation-stack pattern, keyed by an
advisory lock per Work `id`. A new `internal/reconcile` package walks
`<workspace>/{in-progress,archived}/*/work-state.json` to rebuild or reconcile the
projection; it runs automatically when the projection is missing or its `user_version` is
behind, and is exercised directly by tests. The `work` TUI home gains "Resume a Work" and
"Archive Works" entries. The interactive resume/archive pickers are small hand-rolled Bubble
Tea models styled from `huh`'s theme (the same approach as F1's base-branch picker), sharing
a two-line row renderer: an unambiguous name (repository + slug) on top, a relative access
time and the branch beneath.

## Technical Context

**Language/Version**: Go 1.26 (toolchain go1.26.4). Single module `github.com/gustaborges/work`.
No new language or major-dependency additions.

**Primary Dependencies** (all already vendored by F1):
- `github.com/spf13/cobra` — adds `work resume` and `work archive` to the command tree.
- `charm.land/bubbletea/v2` + `charm.land/huh/v2` + `charm.land/lipgloss/v2` — the resume
  picker (single-select), the archive picker (multi-select + confirmation view), and the
  expanded home menu.
- `modernc.org/sqlite` — projection gains `user_version = 2` (adds `archived_at`, a
  `status` index) and rebuild/reconcile queries.
- `golang.org/x/term` — unchanged interactive gating.
- System `git` (>= 2.5) as a subprocess — F2 adds `git status --porcelain` (dirty check),
  `git worktree remove --force`, and `git worktree prune`. **No `git branch -D`** on the
  archive path: the branch a Work created is deliberately left intact (FR-014).
- Standard library for atomic writes, directory rename within the workspace, and the
  advisory lockfile — all reused from F1.

**Storage**:
- Canonical: `work-state.json` **schema 2** per Work. Active Works stay at
  `<workspace>/in-progress/<repo>_<branch>/work-state.json`; archived Works move to
  `<workspace>/archived/<yyyymmdd>-<repo>_<branch>[-<n>]/work-state.json` (no `worktree/`).
- Projection: `~/.work/state/work.db`, `user_version = 2`. `works.status` now takes
  `in-progress | archived`; new nullable `archived_at`; `works_status` index; `dir_path` /
  `snapshot_path` / `worktree_path` follow the Work to the archived area.
- Config (`~/.work/config/work.json`) and generated state (`registry.json`, plugin install,
  locks): **unchanged** by F2. Archiving touches no configuration or bootstrap state (FR-016).
- `state/locks/<sha256(work-id)>.lock` — a new advisory-lock key scope, per Work rather than
  per (workspace, repo, branch); reuses `internal/lockfile`.

**Testing**:
- `go test` table-driven unit tests per new/changed `internal/*` package.
- Integration: `github.com/rogpeppe/go-internal/testscript` `.txtar` scripts driving the
  built `work` binary against throwaway git repos and the F1 fake-shell harness. New
  fault-injection points `WORK_FAIL_AT=snapshot|worktree|move|projection` on the archive
  path assert per-Work transactionality and batch independence (FR-017, SC-005).
- A `verify`-style coherence helper is extended to cover archived Works (no worktree; dir
  under `archived/`).
- Concurrency: a test runs two `work resume` / `resume`+`archive` invocations against the
  same Work and asserts exactly one mutation wins, the other fails cleanly.
- Rebuild/reconcile: delete `work.db`, rebuild, assert active/archived classification and
  recency order are byte-for-byte identical and 0 snapshots were modified (SC-006).
- CI matrix unchanged: GitHub Actions `ubuntu-latest`, `macos-latest`, `windows-latest`.

**Target Platform**: Same as F1 — Linux (amd64, arm64), macOS (arm64, amd64), Windows
(amd64). Terminal repositioning for `work resume` (and the "reposition out" on archiving the
current directory's Work) uses the F1 `WORK_CD_FILE` contract on bash/zsh/fish + PowerShell
7+; every other environment takes the F1 FR-023 reporting path unchanged.

**Project Type**: Single-project CLI tool (unchanged). New `internal/` packages listed below.

**Performance Goals**: Not latency-critical. `work resume` must land in a checkout in
< 30 s including human choices (SC-001), so the machine portion (list + snapshot bump +
projection upsert + path hand-off) stays well under ~1 s. The list is one indexed
`SELECT ... ORDER BY last_accessed_at DESC`. Startup reconcile scans
`<workspace>/{in-progress,archived}/*` once — O(number of Works), acceptable at the F1/F2
scale of tens of Works; it is skipped entirely when the projection opens cleanly at the
current `user_version`.

**Constraints**:
- Deterministic, offline, no AI on any path (FR-029). No network for worktree removal — a
  missing worktree or unreachable source repo degrades to "preserve the snapshot, record
  archived" rather than a hard failure (FR-015, Assumptions).
- Snapshot writes atomic (temp + rename, R4 from F1); the `status` flip and the
  `last_accessed_at` bump are each a single atomic write. A partial write is never
  observable as canonical state (FR-021, SC-005).
- Per-Work archive is transactional: fully active or fully archived, never half-moved; a
  failure on one Work does not roll back Works already archived in the same batch (FR-017).
- The canonical snapshot is the sole authority for `status` and `last_accessed_at`; the
  projection is always rebuildable and is never treated as a competing source (FR-021,
  FR-023, ADR-0013).
- `work resume` and `work archive` run **no** plugin or extension code (FR-006).
- The Git branch a Work created is never deleted by F2 (FR-014).
- No new canonical store; the projection may gain columns but stays derived (FR-025,
  Assumptions).
- Diagnostics never leak repository contents or secrets (carried over from F1 `internal/diag`).

**Scale/Scope**: Single user, single machine, tens of Works (some archived). F2 code
surface: `work resume` + `work archive` + expanded `work` home; ~6 new `internal/` packages
and changes to `internal/{work,projection,gitx,tui,cli,diag}`; no new seed components; no
`work plugin|repository|convention` surface.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unratified template (placeholder principles,
no ratified version), so there are no project-specific constitutional gates. As in F1, this
plan is held to the **cross-cutting gates in `docs/roadmap.md` §4** and the spec's
**Success Criteria**, treated as binding:

| Gate (roadmap §4) | How this plan satisfies it | Verified by |
|---|---|---|
| Determinism | Resume order is a total order: `last_accessed_at DESC`, ties broken by the ULID `id` (creation order) — no scores, no hidden recency heuristic. Explicit targets are the opaque `id` only, never a mutable attribute, so a mutation is never ambiguous. Archived-area collisions resolve by a deterministic `-<n>` suffix. | Unit tests on ordering + id resolution; SC-002; FR-026 |
| Integrity | `internal/resume` and `internal/archive` reuse F1's LIFO compensation stack. Archive canonical commit = the snapshot `status` flip; the projection update trails it and is reconcilable. `WORK_FAIL_AT` fault injection + a per-Work advisory lock. Batch members are independent (FR-017). | `testscript` fault-injection suite; SC-003, SC-005 |
| Process contract | F2 runs no components; the only subprocess is `git`. Exit codes extend the F1 `diag` table with 21–25 and a stable "nothing to do" (exit 0). `--json` stays off both mutations. | `diag` table test; `testscript` non-interactive scenarios; FR-027, FR-028 |
| Portability | Directory relocation is a rename **within one workspace filesystem** (`os.Rename`), reusing F1's atomic-rename posture. `git worktree remove --force` / `prune` are portable. Same 3-OS CI matrix. | CI matrix; SC-006 |
| Auditability | `diag` gains typed categories that name: nothing to resume/archive, id not found, id archived, dirty worktree not acknowledged, worktree-removal failure, archive-move failure, snapshot-write failure, unreadable snapshot during rebuild, incompatible shell. Success output identifies the affected Work(s) and their locations. | `diag` tests; FR-027, FR-028 |
| UX | Both journeys reachable from `work` home by keyboard (FR-030). Explicit `id` skips only the selection, never the confirmation or the recent-access update. Non-interactive: `resume` needs a resolvable target, `archive` needs ids + an approval flag; neither ever opens a TUI. | `testscript` interactive + non-interactive; FR-007, FR-011, FR-030 |
| Regression | The full F1 quickstart (S1–S12) stays green; `work start` is unchanged. F2's own automated demo becomes an additional baseline. | CI; SC-007 |

**Architectural-authority gates (PRD → ADR → ADD):**

- `work-state.json` remains the single canonical authority; `work.db` stays a projection,
  now proven disposable by an explicit rebuild + reconcile (ADR-0013, FR-021–FR-025). ✅
- Schema 2 is **additive and governed by the core only** — plugins still never read or write
  the file (ADR-0013). The bump follows the schema-versioning rule already stated in F1's
  `data-model.md`. ✅
- CLI additions stay inside ADR-0017: `resume` and `archive` are named daily verbs in that
  ADR's human surface; F2 adds no administrative grammar and no new top-level verb. Rebuild/
  reconcile is a library + automatic behavior, not a public command (a `work status
  --verify` / `work doctor` remains a later concern, exactly as F1 decided for `verify`). ✅
- The archived-area layout (`archived/<yyyymmdd>-<repo>_<branch>/`, worktree destroyed,
  snapshot + Importer artifacts preserved) is taken verbatim from ADD §3. ✅
- Archiving leaves the Git branch intact (FR-014) — consistent with the ADRs; no ADR
  mandates branch deletion. ✅
- **⚠️ tracked, not a violation:** ADR-0018 (`work shell-init` / `WORK_CD_FILE`) names
  `start` and "later, `resume`" as the writers of `WORK_CD_FILE`. F2 makes `resume` a
  writer (already anticipated) **and** adds a third case: `work archive` writes
  `WORK_CD_FILE` when it destroys the worktree that is the caller's current directory, to
  move the session to a safe reported location (spec edge case). Phase 0 records this as a
  one-line ADR-0018 amendment to ratify before v1 — the mechanism and protocol are
  unchanged.

**Gate result: PASS.** No violations requiring Complexity Tracking.

### Post-design re-check (after Phase 1)

Re-evaluated after `research.md`, `data-model.md`, and `contracts/` were written. Still PASS:

- No new dependency or subsystem; every new package is pure Go over the stdlib, `git`, and
  the existing SQLite driver.
- `contracts/work-state.schema.json` (schema 2) keeps `additionalProperties: false` and the
  `work` object closed; `archived_at` is the only new field and is required *only* when
  `status == "archived"`. The projection stays a strict function of the snapshots
  (`contracts/index-rebuild.md`).
- The only surface additions remain `work resume` / `work archive` (both ADR-0017 verbs) and
  two home-menu rows. The ADR-0018 amendment for `archive` is the single tracked item.
- All seven roadmap §4 gates have a concrete contract or test harness behind them (table
  above → `contracts/` and `research.md` R18).

## Project Structure

### Documentation (this feature)

```text
specs/002-daily-cycle/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output — decisions R1–R19
├── data-model.md        # Phase 1 output — entities, schema 2, projection v2
├── quickstart.md        # Phase 1 output — runnable validation scenarios S1–S13
├── contracts/           # Phase 1 output
│   ├── cli-work-resume.md       # `work resume` args/flags/exit codes/stdout/stderr
│   ├── cli-work-archive.md      # `work archive` selection, confirmation, dirty-ack, batch
│   ├── cli-work-home.md         # F2 `work` home reachability (start + resume + archive)
│   ├── work-state.schema.json   # canonical snapshot JSON Schema (schema = 2)
│   └── index-rebuild.md         # rebuild + reconcile contract (authority rule, skip rule)
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

Changes and additions to the F1 tree:

```text
internal/
├── cli/
│   ├── root.go                 # + register resume, archive; home dispatch to both
│   ├── resume.go               # NEW — `work resume [id]` flow (interactive + non-interactive)
│   └── archive.go              # NEW — `work archive [id...]` flow, confirmation, --yes, --force-dirty
├── tui/
│   ├── home.go                 # + "Resume a Work", "Archive Works" menu rows (FR-030)
│   ├── resume_picker.go        # NEW — single-select recency list, shared two-line rows
│   └── archive_picker.go       # NEW — multi-select list + confirmation view (space/enter)
├── worklist/                   # NEW — active/all Work listing + id resolution over the
│                               #   projection; the row model the pickers render (repo+slug
│                               #   name, disambiguation, relative time, branch)
├── reltime/                    # NEW — "<n> seconds/minutes/hours/days ago" formatting
├── resume/                     # NEW — resume orchestrator: resolve target, bump
│                               #   last_accessed_at (atomic snapshot + projection),
│                               #   reposition via shellintegration; lock per work id
├── archive/                    # NEW — archive orchestrator: per-Work transactional move
│                               #   (dirty check → status flip → worktree remove → dir
│                               #   rename → projection), batch driver, archived-area
│                               #   pathing + collision suffix; lock per work id
├── reconcile/                  # NEW — Rebuild(workspace, dbPath) and Reconcile(db,
│                               #   workspace): scan in-progress + archived snapshots,
│                               #   upsert/fix rows, drop orphan rows, skip+diagnose
│                               #   unreadable snapshots; never writes a snapshot
├── work/
│   ├── state.go                # schema 2: StatusArchived, ArchivedAt, tolerant Read
│                               #   (accepts schema 1|2), Write emits schema 2, Touch/Archive helpers
│   └── verify/verify.go        # + archived-Work coherence (no worktree; dir under archived/)
├── projection/
│   └── projection.go           # Migrate → user_version 2 (archived_at, works_status index);
│                               #   ListActive(), SetAccessed(id, ts), MarkArchived(id, row)
├── gitx/
│   └── gitx.go                 # + StatusPorcelain()/IsDirty(), WorktreePrune()
└── diag/
    └── diag.go                 # + TargetNotFound(21), TargetArchived(22), DirtyWorktree(23),
                                #   ArchiveFailed(24), SnapshotUnreadable(25)

tests/
├── integration/               # + resume_ordering, resume_by_id, resume_archived_error,
│                              #   archive_multiselect, archive_explicit_confirm,
│                              #   archive_non_interactive, archive_dirty, archive_partial_fail,
│                              #   archive_current_dir, rebuild_after_db_delete,
│                              #   reconcile_stale_rows, rebuild_skips_unreadable
└── fixtures/                  # + repos with several Works, dirty worktrees, pre-archived dirs
```

**Structure Decision**: Single Go project (unchanged from F1). New logic sits in small
role-focused `internal/` packages that mirror the F2 journeys: `worklist` + `reltime` +
the two `tui` pickers own presentation and selection; `resume` and `archive` are the
transactional orchestrators (parallel to F1's `create`); `reconcile` owns the
projection-from-snapshots capability. Changes to `work`, `projection`, `gitx`, `diag`, and
`cli` are additive extensions of existing F1 packages. `seed/` is untouched — F2 introduces
no components.

## Branching Strategy

F2 follows the same git-flow shape as F1: one short-lived `feature/002-daily-cycle-p<n>-*`
branch per `tasks.md` phase, each cut from the previous phase's tip, merged forward at the
phase **Checkpoint** once `make lint` + `go test ./...` are green on the CI matrix.

**Base branch**: the F1 implementation currently lives on **`develop`** (release `0.1`
merged there; `master` carries docs only). F2 phase branches are cut from `develop` (or from
the prior F2 phase branch when it has not merged).

> **Merge cadence is the user's call.** Per the standing project note, the phase branches
> have in practice been **stacked** (phase N+1's PR targets phase N's branch, not
> `develop`). Before starting a phase, `/speckit-implement` MUST confirm with the user which
> branch to base the new phase branch on and which branch its PR targets — do not assume
> `develop`.

Spec/plan/contract/doc edits stay on the current spec branch
(`feature/002-daily-cycle-p0-specs`); the per-phase feature-branch rule covers
implementation code only.

| Phase (tasks.md) | Feature branch | Cut from |
|---|---|---|
| 1 — Setup | `feature/002-daily-cycle-p1-setup` | `develop` (or as the user directs) |
| 2 — Foundational (schema 2, projection v2, reconcile, gitx, diag, shared row/reltime) | `feature/002-daily-cycle-p2-foundational` | phase 1 tip |
| 3 — US1 Resume by recency 🎯 MVP | `feature/002-daily-cycle-p3-us1-resume` | phase 2 tip |
| 4 — US2 Archive with preserved context | `feature/002-daily-cycle-p4-us2-archive` | phase 3 tip |
| 5 — US3 Rebuild the lookup index | `feature/002-daily-cycle-p5-us3-rebuild` | phase 2 tip (rebase onto phase 4 if it landed first) |
| 6 — Polish & cross-cutting | `feature/002-daily-cycle-p6-polish` | phase 5 tip |

Rules (identical spirit to F1):

- **Naming**: `feature/002-daily-cycle-p<n>-<short>` — the phase identifier stays in one
  hyphen-separated segment under `feature/` (no nested path that would D/F-conflict).
- **First action of each phase**: confirm the base/target with the user, then
  `git switch <base> && git pull && git switch -c <feature-branch>`.
- **Merge gate**: a phase merges forward only after its **Checkpoint** in `tasks.md` is met
  and `make lint` + `go test ./...` are green on all three OSes, with every prior slice's
  automated demo (F1 S1–S12, and earlier F2 phases) still green.
- **Phase 4 / Phase 5 contention**: both extend `internal/cli/root.go` and `internal/tui/home.go`.
  A solo run does them in order (4 then 5); if both are in flight, whichever merges second rebases.
- **Release**: when Phase 6 merges, F2 ships via `release/0.2` cut from `develop`, merged to
  `master` and tagged (standard git-flow release). The release step is the hand-off, not a task.

## Complexity Tracking

No Constitution Check violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
