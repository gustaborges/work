# Contract: `work resume` (F2)

CLI contract for the resume journey. Authority: spec FR-001..FR-007, FR-020, FR-026..FR-030;
ADR-0017 (daily verb); ADR-0018 (`WORK_CD_FILE`); research R1, R3, R4, R5, R13, R15, R16.

## Synopsis

```
work resume [WORK]
```

- `WORK` — the **opaque `id`** of a Work (the 26-char ULID in its `work-state.json` /
  `work.id`). No slug, branch, or directory form is accepted (FR-026). Omitted in an
  interactive terminal → the recency picker. Omitted non-interactively → exit 2.
- `--json` is **not** accepted (it is a mutation; ADR-0017). Passing it → exit 2.

## Preconditions

1. `git` >= 2.5 on `PATH` — else exit 16.
2. The projection is opened through the shared helper that **rebuilds it from snapshots when
   it is absent, unopenable, or below `user_version = 2`**, and otherwise runs a light
   reconcile (research R11). This is transparent and changes no snapshot.

## Interactive flow (stdin AND stdout are TTYs)

1. **List** — `projection.ListActive()` → Works with `status = in-progress`, ordered
   `last_accessed_at DESC, id DESC`. Empty → print `note: no Works to resume` to stderr,
   exit 0 (FR — edge case, no error state).
2. **Pick** — single-select picker (research R13). Two-line rows:
   ```
   <repo-name>  <slug>
     <relative-time> • <branch>
   ```
   `↑/↓`/`j/k` move · `/` filter · `Enter` select · `q`/`Esc`/`Ctrl-C` cancel (→ exit 20).
   Works sharing a slug stay distinguishable (branch on line 2; `(id[:6])` appended to
   line 1 if `(repo, slug, branch)` still collide). The Work whose worktree is the current
   directory is just one of the rows; selecting it is a valid no-op reposition that still
   bumps access (FR — edge case).
3. **Resume** — under an advisory lock on the Work `id`:
   a. read the snapshot; if `status != in-progress` → exit 22 (should not happen from the
      list; guards a race);
   b. set `work.last_accessed_at = now`, `schema = 2`, write atomically — **canonical
      commit** (FR-003, FR-021);
   c. `projection.SetAccessed(id, now)` (FR-003).
4. **Reposition + report** — see *Shell integration* below. Always print the success
   summary to stdout.

`work resume` runs **no** plugin or extension code and modifies **no** Work other than the
one resumed (FR-006).

## Explicit-target flow (`work resume <id>`)

- No list is shown (FR-005).
- Resolve `<id>` via `projection.Get`:
  - no row → exit 21 (`target-not-found`), no state change (FR-005);
  - `status = archived` → exit 22 (`target-archived`); no reposition into a destroyed
    worktree (FR-020);
  - `status = in-progress` → steps 3–4 above, identical bump (FR-005).

## Non-interactive flow (stdin OR stdout not a TTY)

- No TUI is ever constructed (FR-007, FR-029).
- A resolvable `<id>` argument is **required**. Missing → exit 2, message: pass a Work id;
  no state change. Unknown id → exit 21. Archived id → exit 22.
- On success: steps 3–4 minus any prompt.

## stdout (success, exit 0)

Stable, greppable (FR-028):

```
work: resumed <id>
work: path <absolute-worktree-path>
```

Nothing else on stdout. Shell notices (FR-023 path) go to **stderr**.

## stderr

- Picker, prompts, progress (interactive only).
- `note: no Works to resume` when the active set is empty (exit 0).
- FR-023 notice when no shell integration is active — identical to `work start`
  (`specs/001-first-local-work/contracts/shell-integration.md`), with the worktree path on
  its own line.
- On failure: one `error: <token>: <message>` line.

## Shell integration (`WORK_CD_FILE`)

Reuses the F1 contract verbatim (ADR-0018): on success, after `SetAccessed`, if
`WORK_CD_FILE` is set the core writes the absolute worktree path there; if unset it takes
the FR-023 stderr path and never claims a `cd` happened. The core writes `WORK_CD_FILE` only
on a successful resume, never on failure or cancel.

## Exit codes

| Code | Token | Meaning |
|---|---|---|
| 0 | `ok` | Work resumed (a shell notice on stderr is still exit 0); or nothing to resume |
| 2 | `usage` | `--json` passed; non-interactive with no resolvable target |
| 16 | `bootstrap-failed` | git preflight failed |
| 20 | `cancelled` | user left the picker without choosing |
| 21 | `target-not-found` | `<id>` matches no Work |
| 22 | `target-archived` | `<id>` names an archived Work |

## Invariants (verified by tests — SC-001, SC-002, SC-007)

- On exit ≠ 0 (21, 22, 2): zero snapshot writes, zero projection writes.
- On exit 0 (resumed): the resumed Work's `last_accessed_at` in the snapshot equals the
  value in `work.db`, and both are ≥ every other Work's; a repeated `work resume` lists it
  first (SC-002).
- No Work other than the resumed one changes (FR-006).
- The F1 `work start` journey and quickstart S1–S12 are unaffected (SC-007).
