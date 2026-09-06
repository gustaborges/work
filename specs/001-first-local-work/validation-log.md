# F1 Manual Validation Log

Manual end-to-end run of `quickstart.md` S1–S12 (task T066).

| Field | Value |
|---|---|
| Date | 2026-09-06 |
| Commit | Phase 6 (`feature/001-first-local-work-p6-polish`) |
| Linux | Ubuntu (Linux 7.0.0-31-generic), Go 1.26.4, git 2.43.0 — **run manually** |
| Windows | **not run manually** — covered by the CI `windows-latest` matrix job (`make seed-all && make lint && go test ./... && make build`), green on the Phase 5/6 PRs |

## Results (Linux)

| Scenario | Result | Notes |
|---|---|---|
| S1 — happy path (flag form) | ✅ pass | 3 stdout summary lines exact; `worktree/` + `work-state.json` only; snapshot fields correct; source repo clean, no new branch; one `works` row; bootstrap artifacts present |
| S2 — base-branch homonyms | ✅ pass | `--base origin/main` → `work.base_branch == "origin/main"`, new branch tip == `origin/main`'s object, distinct from local `main` |
| S3 — offline (`unshare -rn`) | ✅ pass | identical success with no network namespace |
| S4 — invalid path | ✅ pass | missing → exit 10; plain dir → exit 11; bare repo → exit 11; each names only the supplied path |
| S5 — invalid slug | ✅ pass | `has spaces` → exit 13; source `git branch` unchanged; no Work dir |
| S6 — branch collision | ✅ pass | re-run of a succeeded create → exit 14 ("checked out in another worktree"); no partial state |
| S7 — rollback (`WORK_FAIL_AT`) | ✅ pass | `worktree` / `snapshot` / `projection` each → exit 17; no `rollme` branch, no worktree entry, no Work dir, no row; config + seed intact |
| S8 — cancel before confirm | ✅ pass (with doc fix) | see Deviations |
| S9 — non-interactive missing value | ✅ pass | piped stdin + missing `--slug` → exit 2 naming `--slug`; no config write, no Work state |
| S10 — shell integration / FR-023 | ✅ pass | covered by `tests/integration/shell_integration.txtar` + fakeshell harness; FR-023 notice verified manually (real path on its own line, `shell-init` hint, no false `cd` claim) |
| S11 — bootstrap idempotency | ✅ pass | `internal/bootstrap` stress test: 100× `EnsureSeed` with 20 injected interruptions + 12-goroutine concurrent run + orphan-staging sweep — exactly one registry entry each, one plugin dir, no partial dir |
| S12 — seed subprocess contracts | ✅ pass | `tests/contract/` starter + locator tables, plus new garbled-input cases (truncated / non-UTF-8 / NUL / ~2 MiB blob): non-zero exit, no stdout, one stderr line, no hang |

## Deviations

### D1 — S8 quickstart example could not decline via a pipe (doc fixed)

`quickstart.md` S8 showed `printf 'n\n' | work start …` expecting exit 20. A piped
stdin makes the run **non-interactive**, so no confirm prompt is shown; without
`--yes` it fails fast with exit **2** (`usage`) and mutates nothing. The exit-20
"declined" path requires an interactive terminal (stdin **and** stdout TTYs), which
is how `tests/integration/interactive_test.go` (`TestInteractiveCancelAtConfirm`)
exercises it via a pty.

This is a **documentation inconsistency, not a code defect** — the contract
(`contracts/cli-work-start.md`) already specifies exit 2 for a non-interactive run
missing `--yes`, and exit 20 for an interactive decline / SIGINT-before-commit, both
of which behave correctly.

**Resolution:** `quickstart.md` S8 updated in this phase to describe the interactive
decline / SIGINT paths and to note that the piped form is a distinct (also
artifact-free) exit-2 outcome. No separate issue filed.

## Notes

- Windows was not exercised by hand; the CI matrix is the standing guarantee
  (T062). If a Windows-only regression surfaces later, file it against the
  portability sweep.
