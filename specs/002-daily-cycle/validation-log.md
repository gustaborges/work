# F2 Manual Validation Log

End-to-end run of `quickstart.md` S1–S13 (task T043).

| Field | Value |
|---|---|
| Date | 2026-09-06 |
| Commit | Phase 6 (`feature/002-daily-cycle-p6-polish`), tip `ea7ffb4` |
| Linux | Ubuntu (Linux 7.0.0-31-generic), Go 1.26.4, git 2.43.0 |
| Windows / macOS | **not run manually** — covered by the CI matrix (`macos-latest`, `windows-latest`) running `make seed-all && make lint && go test ./...`, which executes every scenario below as an automated suite |

Each F2 quickstart scenario has a dedicated automated counterpart that drives the
built `work` binary (a `.txtar` scenario, or a pty test where the scenario is
interactive — `testscript` is not a pty). The run below is those suites executed
on Linux plus manual spot-checks of the human-facing output.

## Results (Linux)

| Scenario | Automated counterpart | Result | Notes |
|---|---|---|---|
| S1 — resume by recency, interactive | `resume_test.go::TestResumeOrderingAndReposition` | ✅ pass | picker lists most-recent-first with the two-line row; selecting the least-recent repositions the session, bumps `last_accessed_at` in snapshot **and** `work.db`, and a second `resume` shows the new order |
| S2 — resume by explicit id, no list | `resume_by_id.txtar` | ✅ pass | `work resume <id>` renders no TUI, same two stdout lines, same bump |
| S3 — resume error paths | `resume_errors.txtar` | ✅ pass | unknown id → 21; non-interactive no-target → 2 (no TUI); `--json` → 2; archived id → 22; zero snapshot / zero projection writes on every path |
| S4 — home TUI reaches both journeys | `interactive_test.go::TestHomeReachability`, `TestHomeReachesPopulatedPickers` | ✅ pass | home lists exactly Start / Resume / Archive; arrowing to each + Enter opens its picker; `q` → exit 0, no state change; non-interactive `work` → exit 2 with the one-line summary |
| S5 — archive multi-select, confirm, preserve snapshot | `archive_test.go::TestArchiveMultiSelectAndConfirm` | ✅ pass | nothing preselected; confirmation names the checked Works; decline changes nothing; confirm removes exactly those worktrees, snapshots readable under `archived/` with `status: archived` + `archived_at`, other Work untouched, branches still present, projection no longer lists them |
| S6 — archive explicit targets still confirm | `archive_test.go::TestArchiveExplicitTargetStillConfirms`, `archive_non_interactive.txtar` | ✅ pass | explicit id shows the confirmation view (no multi-select list); declining → exit 20, nothing moved |
| S7 — archive dirty worktree is protected | `archive_dirty.txtar` | ✅ pass | dirty Work left active with the `note:` on stderr, clean Work archived, exit 0; `--force-dirty` re-run archives it; interactive per-Work ack path covered by `archive_test.go` |
| S8 — archive partial batch failure stays consistent | `archive_partial_fail.txtar` | ✅ pass | `WORK_FAIL_AT=worktree/move` → earlier Work fully archived, failing Work fully active; `WORK_FAIL_AT=projection` → Work canonically archived on disk, row healed or exit 24 naming it, next command's reconcile agrees |
| S9 — already-archived / unknown targets don't break the batch | `archive_already_archived.txtar` | ✅ pass | `note: already archived` / `note: not found` on stderr, batch continues, exit 0, `archived 1 of 1` |
| S10 — archiving the current directory's Work repositions out | `archive_current_dir.txtar` | ✅ pass | with the shell hook: `WORK_CD_FILE` receives the workspace root; without: exit 0, stderr names the workspace root, no false `cd` claim |
| S11 — rebuild the index from snapshots | `rebuild_after_db_delete.txtar`, `rebuild_verify_parity.txtar` | ✅ pass | `rm work.db`; next command rebuilds from active + archived snapshots; snapshot fingerprints and full recency order byte-identical before/after; `user_version` == 2; `verify` passes for every Work |
| S12 — reconcile a stale / inconsistent index | `reconcile_stale_rows.txtar` | ✅ pass | stale `last_accessed_at` corrected from the snapshot, ghost row dropped, missing row re-added, zero snapshot writes |
| S13 — rebuild skips an unreadable snapshot and diagnoses it | `rebuild_skips_unreadable.txtar` | ✅ pass | `note: snapshot-unreadable: <path>: <reason>` on stderr; every other Work indexed; rebuild does not abort |

Concurrency (research R17 / spec edge case), not a numbered scenario:
`internal/resume/concurrency_test.go` — `resume`+`resume` and `resume`+`archive`
racing on one Work: the loser fails cleanly on the advisory-lock timeout with no
state change; operations on different Works never contend. ✅ pass.

## Deviations

### D1 — `repo_name` derivation on rebuild

`contracts/index-rebuild.md` describes the reconstructed `repo_name` as "the
`<name>` prefix before the last `_`". The implementation instead strips the exact
sanitized-branch suffix recovered from the snapshot (the precise inverse of F1's
`<repo>_<branch>` directory naming), falling back to "prefix before the last `_`"
only when the name does not fit that shape. The two agree for every ordinary name;
the stricter rule additionally survives a repository or branch name that itself
contains `_`. No contract or quickstart assertion changes.

## Notes

- Windows and macOS were not exercised by hand; the CI matrix is the standing
  guarantee. A platform-specific regression later should be filed against the
  portability sweep.
- F1 quickstart S1–S12 were re-run as part of `go test ./...` and remain green
  (`f1_regression_test.go` pins the `work start` stdout contract — SC-007).
