# Contract: `work archive` (F2)

CLI contract for the archive journey. Authority: spec FR-008..FR-021, FR-026..FR-030;
ADR-0017 (daily verb, `archive` = "encerra um Work preservando seu estado"); ADR-0018
(`WORK_CD_FILE` — amendment for the reposition-out case); research R1, R6–R10, R15–R17.

## Synopsis

```
work archive [WORK...] [--yes] [--force-dirty]
```

- `WORK...` — one or more **opaque `id`s** (FR-026). Omitted in an interactive terminal →
  the multi-select picker. Omitted non-interactively → exit 2.
- `--yes` — clears the ordinary confirmation for a non-interactive run (FR-011). Never
  selects Works. Interactively, the confirmation view is still shown even with `--yes`
  unless `--yes` is combined with explicit ids… (see *Confirmation*).
- `--force-dirty` — clears the dirty-worktree guard (FR-013). **Separate** from `--yes`;
  neither implies the other.
- `--json` is **not** accepted (mutation; ADR-0017) → exit 2.

## Preconditions

1. `git` >= 2.5 on `PATH` — else exit 16.
2. Projection opened through the rebuild-or-reconcile helper (research R11), transparently.

## Interactive flow (stdin AND stdout are TTYs)

1. **List** — `projection.ListActive()` (status = in-progress), ordered `last_accessed_at
   DESC, id DESC`. Empty → `note: no active Works to archive` on stderr, exit 0.
2. **Multi-select** — picker (research R13), same two-line rows as resume, with a checkbox:
   ```
   [ ] <repo-name>  <slug>
       <relative-time> • <branch>
   [ ] ...
   ```
   `Space` toggle · `↑/↓` move · `/` filter · `Enter` → confirmation view · `Ctrl-C` cancel.
   **Nothing is preselected** (FR-009). `Enter` with nothing checked is a no-op (stays on
   the list).
3. **Confirmation view** — lists the checked Works and states: *"N worktree(s) will be
   destroyed. Snapshots move to `<workspace>/archived/`. Branches are kept."* `Enter` =
   confirm · `Esc` = back to the list. Nothing changes until confirm (FR-010).
4. **Per-Work archive** — for each confirmed Work, in order, under an advisory lock on its
   `id` (research R6):
   a. **dirty check** — `git -C <worktree> status --porcelain`. If non-empty and not
      `--force-dirty`: show an extra per-Work prompt (default **no**). Declining leaves that
      Work **fully active**; the batch continues (FR-013, SC-009).
   b. atomic snapshot rewrite in place: `status = archived`, `archived_at = now`,
      `last_accessed_at = now`, `schema = 2` — **canonical commit** (FR-012).
   c. `git worktree remove --force <src>/worktree`; missing worktree → `git worktree prune`,
      treated as success (FR-015, research R9).
   d. `os.Rename` `<workspace>/in-progress/<name>` → `<workspace>/archived/<yyyymmdd>-<name>[-<n>]`
      (research R8).
   e. `projection.MarkArchived(id, row)` — `status`, `archived_at`, new paths,
      `worktree_path = ""` (FR-018).
   Any failure at (b)–(d) before it is accepted → compensate LIFO back to fully active; the
   Work's `outcome = left-active` or `failed`; **other Works are unaffected** (FR-017).
5. **Reposition + report** — if a just-archived Work's worktree was the caller's current
   directory, write the workspace root to `WORK_CD_FILE` (or print the stderr notice); then
   print the summary.

`work archive` runs **no** plugin or extension code, and touches **no** non-selected Work,
configuration, or bootstrap state (FR-006, FR-016).

## Explicit-target flow (`work archive <id...>`)

- The multi-select list is skipped (FR-010).
- Unknown ids and already-archived ids are **reported and dropped from the batch** — they do
  not fail the run (FR-019). See *Exit codes*.
- The applicable **confirmation is still required** (FR-010): interactive → the confirmation
  view for the resolved Works; non-interactive → `--yes`.
- Per-Work archive is identical to step 4 above.

## Non-interactive flow (stdin OR stdout not a TTY)

- No TUI ever (FR-011, FR-029).
- Requires **both** `WORK...` ids **and** `--yes`. Missing ids → exit 2. Missing `--yes` →
  exit 2, nothing archived (FR-011).
- A dirty Work without `--force-dirty` is **skipped** (left active) and reported; it does
  not fail the invocation or the other Works (FR-013).
- On success: step 4 minus every prompt; stable exit code.

## stdout (success / partial, exit 0)

One line per archived Work, then a count (FR-028):

```
work: archived <id>  (<absolute-archived-dir>)
work: archived <id>  (<absolute-archived-dir>)
work: archived 2 of 3
```

Works left active (dirty, un-acked) or reported (unknown / already archived) are named on
**stderr**, not stdout.

## stderr

- Picker, confirmation view, per-Work dirty prompts, progress (interactive only).
- `note: no active Works to archive` (exit 0).
- Per non-archived target: `note: <id>: not found` / `note: <id>: already archived` /
  `note: <id>: worktree has uncommitted or untracked changes — left active (use --force-dirty)`.
- Reposition-out notice when the current directory's Work was archived and no shell hook is
  active.
- On a hard failure: `error: <token>: <message>`.

## Exit codes

| Code | Token | Meaning |
|---|---|---|
| 0 | `ok` | all confirmed, eligible Works archived (or nothing to archive); Works left active for dirtiness or reported as unknown/already-archived do **not** change this |
| 2 | `usage` | `--json`; non-interactive missing ids or missing `--yes` |
| 16 | `bootstrap-failed` | git preflight failed |
| 20 | `cancelled` | user declined the confirmation / interrupted before the first commit |
| 22 | `target-archived` | (only when a **single explicit** target is already archived and there is nothing else to do) |
| 23 | `dirty-worktree` | (only when a **single explicit** target is dirty, non-interactive, no `--force-dirty`) |
| 24 | `archive-failed` | worktree removal, archived-area move, or snapshot write failed for a Work after its canonical commit and self-heal did not recover; message names the step and the `id` |

For a **batch**, codes 22/23 do not fail the run (FR-019, FR-013) — they are per-target
notes on stderr and the exit stays 0 as long as the eligible Works archived. 24 is returned
when a Work is left in a state that needs the index to self-heal on the next command.

## Shell integration (`WORK_CD_FILE`) — reposition-out

When `work archive` destroys the worktree that is (or contains) the caller's current working
directory: if `WORK_CD_FILE` is set, the core writes the **workspace root** there (a safe,
existing location); if unset, it prints to stderr that the session is now inside a deleted
directory and names the workspace root to `cd` to. The core never claims a `cd` happened.
This is the ADR-0018 amendment tracked in the plan (research R5/R10).

## Invariants (verified by tests — SC-003, SC-004, SC-005, SC-009)

- **No destructive action before** an interactive confirmation or `--yes` (SC-004): on a
  decline / SIGINT before step 4b of the first Work, zero worktrees removed, zero snapshots
  changed, zero dirs moved, zero rows changed.
- After a full or partial run: exactly the confirmed, eligible Works are archived — their
  worktrees gone, their snapshots readable under `archived/` with `status = archived`, their
  rows `status = archived`; every non-selected Work unchanged (SC-003).
- A Work left active for dirtiness has an intact worktree and an unchanged snapshot (SC-009).
- Fault injection (`WORK_FAIL_AT=snapshot|worktree|move|projection`) on any Work leaves that
  Work fully in one state and every earlier Work in the batch consistently archived
  (SC-005); the projection matches disk after the run (FR-018).
- The Git branch each archived Work created still exists in the source repository (FR-014).
- `work start` and quickstart S1–S12 are unaffected (SC-007).
