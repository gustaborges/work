# Moderated Discovery Check (SC-006)

**Purpose**: Verify that a first-time user, given only `work` and `work --help`,
can find the entry command for every public journey quickly and without help.
This is a **manual, moderated** check — it is not automated. It is referenced
from [`quickstart.md`](../quickstart.md) (Q6 discovery-moderation row).

**Target (SC-006)**: ≥ 90% of participants locate the correct command for each
journey within **30 seconds**, unaided.

## Setup (facilitator)

1. Build a clean binary: `make build`, `export PATH="$PWD/bin:$PATH"`.
2. Use a fresh `WORK_HOME`: `export WORK_HOME="$(mktemp -d)/dotwork"`.
3. Give the participant a terminal and exactly two permitted commands:
   `work` and `work --help`. No documentation, no README, no hints.
4. For each task below: read the goal aloud, start a timer when the participant
   starts typing, stop it when they name (or run) the correct command. Record
   the time and whether they needed a nudge.

## Tasks

| # | Goal given to the participant | Correct answer | Time | Unaided? |
|---|---|---|---|---|
| 1 | "Turn a repo you have already cloned into an isolated worktree to start new work." | `work start` | | |
| 2 | "Go back to a Work you were on earlier this week." | `work resume` | | |
| 3 | "Close out one or more Works you are done with, keeping their branches." | `work archive` | | |
| 4 | "Make your shell follow `work` into the new worktree automatically." | `work shell-init` (`eval "$(work shell-init <shell>)"`) | | |
| 5 | "See everything `work` can do." | `work --help` (or `work -h`, `work help`) | | |

## Pass criteria

- Every task: correct command identified within 30 s, no facilitator nudge.
- Across ≥ 10 participants, ≥ 90% success rate per task.
- No participant expects bare `work` to open an interactive menu (it prints the
  brand and a `work --help` pointer, then exits).

## Recording

Log one row per participant per task (id, task #, seconds, unaided y/n) and
attach the summary to the F2.5 release notes. A task that misses the 90% bar is
a help-text or grouping defect — file it against `internal/cli/help.go`.
