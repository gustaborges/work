# Quickstart / Validation Guide: Terminal UX Revamp (F2.5)

**Feature**: `specs/003-terminal-ux-revamp/` · **Plan**: [plan.md](./plan.md)

Runnable scenarios that prove F2.5 end to end. Shapes, tokens, and rules live in
[`contracts/`](./contracts/), [`data-model.md`](./data-model.md), and
[`research.md`](./research.md) — not repeated here.

**Compatibility baseline (SC-007):** the full F1 suite
(`specs/001-first-local-work/quickstart.md` S1–S12) and F2 suite
(`specs/002-daily-cycle/quickstart.md` S1–S13) MUST pass **unchanged** alongside
these. Any diff in their stdout, error tokens, mutations, or exit codes is a
regression — except the single contracted change: interactive bare `work` now exits
0 after the brand instead of opening the selectable home.

## Prerequisites

- Go 1.26+, system `git` >= 2.5.
- A Unix host with a PTY for the interactive scenarios (`//go:build unix`,
  `github.com/creack/pty`); Windows keeps the F1/F2 non-interactive coverage plus a
  manual smoke.
- No network for any scenario.

## Build & isolation

```bash
make build
export WORK_HOME="$(mktemp -d)/dotwork"
export PATH="$PWD/bin:$PATH"
export WS="$(mktemp -d)/workspaces"
src="$(mktemp -d)/demo"; git init -q "$src"
( cd "$src" && git commit -q --allow-empty -m init && git branch -M main )
```

---

## Q1 — `work start`: rejected attempts leave no scrollback debris (US1; FR-005, FR-006, SC-001)

**PTY, 80×24.** Run `work start` with no arguments. At the path prompt enter four
invalid paths (`/no/such/a`, `/no/such/b`, `/tmp` (not a repo), `/no/such/c`), then
the valid `$src`. At the slug prompt enter an invalid slug (`bad slug`), a colliding
slug, then `my-work`.

**Expect:**
- The journey runs full-screen (alternate buffer); each accepted receipt stays
  visible above the active step, and each `✘ …` error **replaces** the previous
  one — never stacks. Resize the terminal mid-selector → **no** duplicated headers.
- On exit the primary buffer is restored and the receipt trail is reprinted there —
  **one** receipt per accepted step and **zero** rejected values or obsolete errors:
  ```
  Local repository path
    ✔ <src>

  Slug
    ✔ my-work

  Base branch
    ✔ main

  ✔ Create Work confirmed
  ```
- The stable stdout lines (`work: created …`, `work: branch …`, `work: path …`),
  printed after the reprint, are unchanged from F1.

## Q2 — Selectors collapse to a compact receipt (US1, US3; FR-003, FR-007, SC-002)

**PTY, 80×24 and 160×50.** In the `work start` journey, open the base-branch
selector; confirm it renders a generous scrolling list while active with **no**
per-row SHA. Press `Enter`.

**Expect:** the list is immediately replaced by `Base branch` / `  ✔ <short-name>`
and one blank line — **no** list rows, **no** blank padding before the next step,
**no** SHA. Same at 160×50 (no wasted rows survive). In the reprinted history after
exit the receipt reads the same.

## Q3 — Confirmation shows impact, then collapses (US1; FR-030, SC-002)

**PTY.** Reach the `work start` confirmation. It shows the full impact block
(repository / base / branch / workspace / directory). Accept.

**Expect:** the block collapses to `✔ Create Work confirmed` **before** the stable
`work: created …` lines. Decline instead → `✘ …` cancellation line, exit 20, nothing
created (F1 S8 still green).

## Q4 — Secret step redaction (US1; FR-004)

**Model test** (no secret step ships in F1/F2, so this is a `present.Input` unit
test): `InputSpec{Secret: true}` → the receipt shows `••••` (or the caller's
`Receipt`), never the typed value; `View()` in `completed` state contains no
substring of the input.

## Q5 — Bounded frame at small and large sizes (US3; FR-002, FR-006, SC-004)

**PTY at 40×10, 80×24, 160×50.** Open the resume picker with a fixture of 12 Works,
some with long branch names and wide-Unicode slugs (`日本語-ブランチ`, combining
marks).

**Expect at every size:** the picker fills the alternate screen but never emits a
frame taller than the viewport; on exit the primary buffer comes back with no stray
lines; primary identity stays visible; secondary metadata truncates with `…` first;
column starts identical across all visible rows.

## Q6 — Stable selector geometry (US3; FR-008, FR-009, SC-003)

**Model + PTY.** In the archive multi-select, move focus across every row, toggle
several checkboxes, filter, and clear the filter.

**Expect:** unchanged rows show **zero** change in line count and **zero** shift in
the starting columns of the focus marker, `[ ]`/`[x]`, primary text, secondary text.
The focused row is **bold** (verified with colour forced off). This is the explicit
regression test for the F2 `archive_picker.go` double-render defect.

## Q7 — Colour disabled: `NO_COLOR` and `TERM=dumb` (US5; FR-025, SC-008)

```bash
NO_COLOR=1 work --help | cat        # Q7a
TERM=dumb work --help | cat          # Q7b
printf '' | NO_COLOR=x work start 2>&1 | cat   # Q7c (non-interactive)
```

**Expect:** output contains **no** ESC (`\x1b`) bytes; text is fully legible; a
`NO_COLOR=` **empty** value does **not** disable colour (spec edge case — covered by
a separate assertion). `TERM=dumb` interactive `work` still emits the brand but with
zero control sequences.

## Q8 — Brand: wide gradient vs compact plain (US2; FR-018, FR-019, SC-009)

**PTY.** `work` (no args) at `COLUMNS=120` true-colour → the multi-line `WORK` art
with a visible left-to-right `#11A8CD`→`#8B7CF6` gradient, tagline, `work --help`
direction, **exit 0**. Repeat at `COLUMNS=30` → the compact `WORK` word + tagline +
direction, no broken wrapping. Repeat piped (`work | cat`) → non-interactive usage
line, **exit 2**, no ANSI.

**Golden tests** cover: wide-true-colour, wide-no-colour, narrow.

## Q9 — `work --help` lists exactly the registered commands, grouped (US2; FR-020, FR-021, SC-005)

```bash
work --help
```

**Expect:** a `Daily Commands` block with `start`, `resume`, `archive`; a `Setup`
block with `shell-init`, `completion`; **no** `Inside a Work` or `Administration`
block (empty ⇒ hidden); every command appears exactly once; nothing ungrouped. The
correspondence test (`cli-help.md`) asserts counts against `root.Commands()`.

## Q10 — `work --help` is plain and exit 0 when piped (US5; FR-020, SC-007)

```bash
work --help | cat ; echo "exit=$?"
```

**Expect:** identical text, no ANSI, `exit=0`.

## Q11 — Cancellation is one concise line, code preserved, no mutation (US4; FR-012, FR-013, SC-010)

**PTY.** `work resume`, then press `q` (repeat with `Esc`, then `Ctrl-C`). `work
archive`, select rows, reach the confirmation, press `Ctrl-C`.

**Expect each time:** terminal shows exactly `✘ Operation cancelled`; exit code
**20**; `last_accessed_at` in every snapshot is unchanged (`verifycoherent` from the
F2 harness still passes); no worktree/branch/dir/config change.

## Q12 — Actionable errors, single border, cause hidden (US4; FR-014–FR-016)

**PTY + non-interactive.**
- Interactive: trigger a non-recoverable failure (e.g. `git` made unusable) → **one**
  `✘ <summary>` line, optionally `  → <hint>`; no wrapped chain; no lower-layer print.
- `WORK_DEBUG=1 work start …` → the human line **plus** the cause chain appended.
- Non-interactive: same failure → `error: <token>: <message>` exactly as F1/F2, same
  exit code.

## Q13 — UI bytes vs stdout separation (US5; FR-026, SC-007)

**PTY with stdout and stderr captured separately.** Run an interactive `work start`
to completion.

**Expect:** every frame, receipt, help line, and diagnostic is on the **UI channel**
(stderr); **only** `work: created …` / `work: branch …` / `work: path …` are on
stdout — byte-identical to F1.

## Q14 — Non-interactive missing value still fails without a TUI (US5; FR-027, FR-028, SC-007)

```bash
printf '' | work start ; echo "exit=$?"      # missing SOURCE → exit 2, actionable usage, no TUI
printf '' | work ; echo "exit=$?"            # bare work non-interactive → exit 2 (unchanged)
```

Covered by the existing `tests/integration/non_interactive_missing.txtar` plus a new
assertion that no ANSI / no prompt was emitted.

## Q15 — Full F1 + F2 regression (SC-007)

```bash
go test ./...            # includes tests/integration F1 S1–S12 + F2 S1–S13
make lint
```

**Expect:** green on `ubuntu-latest`, `macos-latest`, `windows-latest`. Zero change
to any F1/F2 `.txtar` assertion except the interactive-bare-`work` exit code.

---

## Traceability

| Scenario | User story | Key FRs | Success criteria |
|---|---|---|---|
| Q1 | US1 | FR-005, FR-006 | SC-001 |
| Q2 | US1, US3 | FR-003, FR-007 | SC-002 |
| Q3 | US1 | FR-030 | SC-002, SC-010 |
| Q4 | US1 | FR-004 | SC-001 |
| Q5 | US3 | FR-002, FR-006 | SC-004 |
| Q6 | US3 | FR-008, FR-009 | SC-003 |
| Q7 | US5 | FR-025 | SC-008 |
| Q8 | US2 | FR-018, FR-019 | SC-009 |
| Q9 | US2 | FR-020, FR-021 | SC-005 |
| Q10 | US5 | FR-020 | SC-007 |
| Q11 | US4 | FR-012, FR-013 | SC-010 |
| Q12 | US4 | FR-014–FR-016 | SC-010 |
| Q13 | US5 | FR-026 | SC-007 |
| Q14 | US5 | FR-027, FR-028 | SC-007 |
| Q15 | all | FR-031, FR-032 | SC-007 |
| Q6 (discovery moderation) | US2 | FR-022 | SC-006 (moderated check, not automated) |
