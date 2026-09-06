# Contract: `work start` (F1)

CLI contract for the primary F1 journey. Governs args, flags, interactive vs. non-interactive
behavior, stdout/stderr, and exit codes. Authority: spec FR-001..FR-027, RF-51, RF-52; ADR-0017
(daily command names); research R13, R19.

## Synopsis

```
work start [SOURCE] [--workspace PATH] [--base REF] [--slug SLUG] [--prefix PREFIX] [--yes]
```

- `SOURCE` — **F1: a direct path to a local Git repository only.** No name/URL/root lookup
  fallback is attempted (FR-002). If omitted in an interactive terminal, it is collected via
  a TUI prompt; if omitted non-interactively → exit 2.
- `--workspace PATH` — workspace root for this run. If no root is configured yet, the value is
  validated and persisted (FR-006). If a root is already configured, F1 rejects `--workspace`
  with exit 2 and directs the user to edit `~/.work/config/work.json` (changing a configured
  root is a later slice).
- `--base REF` — base branch; must match one listed local or remote-tracking ref (short name
  or `remote/short`). Ambiguous match (local vs remote homonym) without a disambiguating
  prefix → exit 2.
- `--slug SLUG` — the slug. Prompt-level rules: non-empty, no whitespace, no `..`, no leading
  `-`, no control chars. Authoritative validation is git's (see exit 13).
- `--prefix PREFIX` — convention prefix. F1 `freeform` offers exactly `{slug}`; any other value
  → exit 2.
- `--yes` — confirm the final, already-determined creation without the interactive confirm
  step. Never selects `SOURCE`, `--base`, `--slug`, or `--prefix` (RF-52).
- `--json` is **not** accepted on `work start` (it is a mutation; RF-52 restricts `--json` to
  reads). Passing it → exit 2.

## Preconditions

1. `git` >= 2.5 on `PATH` — else exit 16 (`bootstrap`/preflight), message names the missing/old tool.
2. Seed package present or seedable offline — `bootstrap.EnsureSeed()` runs first (idempotent, R11).
   Failure → exit 16.

## Interactive flow (stdin AND stdout are TTYs)

Ordered; each explicit flag skips **only** its own step, never validation. As each step
resolves, its prompt is replaced in place by a one-line completed summary (the step title,
then `✓ <chosen value>`), separated from the next prompt by exactly one blank line; a value
supplied by flag renders no summary.

1. **Source** — if `SOURCE` omitted: prompt for a path.
2. **Resolve + validate repository** (R14): abs+symlink resolve; must be a readable dir, a
   usable non-bare git repo, ≥1 commit, ≥1 base branch. Failure → the interactive flow
   re-prompts for another path (does not exit) for `invalid-path`/`unusable-repo`; `no-base-branch`
   exits 12 (not correctable by a different path to the same repo).
3. **Prefix** — if `--prefix` absent: show the branch convention's prefixes and select. A
   convention that offers a single prefix (`freeform` → `{slug}`) is not a choice: no prompt
   and no completed summary are shown.
4. **Slug** — if `--slug` absent: text prompt with inline validation. Asked before the base
   branch: a rejected slug is the cheapest failure to recover from (no repo scan), so it
   comes first.
5. **Derive + validate branch name** (R16): `interpolate(prefix, slug)`; `git check-ref-format`;
   collision check against local + remote-tracking + worktree branches. Any failure → return to
   step 4 (slug) with a specific message; no mutation has occurred (SC-004).
6. **Base branch** — if `--base` absent: a picker with a `Remote` / `Local` tab bar over
   the branch list (`←/→`/`Tab` to switch, `↑/↓` to move, `/` to filter the active tab,
   `Enter` to select); each row `<short>  <short-sha>`; select one (FR-009). A tab with no
   refs is hidden, and the tab bar is omitted when only one kind exists. `Other work` is a
   reserved source for a later slice and is not shown. On selection the list collapses to the
   completed summary `✓ <short>  <short-sha> [remote|local]`.
7. **Workspace root** — if none configured and `--workspace` absent: suggest `~/work`
   (`%USERPROFILE%\work` on Windows), allow editing, validate (R8), persist. If already
   configured: reuse silently (no prompt, no summary). Deferred to here so a run rejected at
   an earlier step never persists a root or creates its directories (SC-004).
8. **Confirm** — show repo, base branch (+ short SHA), derived branch name, workspace root,
   and target directory. Proceed on confirm (or `--yes`). Decline → exit 20.
9. **Materialize** (transactional, R10): lock → `git worktree add -b <branch> <dir>/worktree
   <base-refname>` → write `work-state.json` (atomic) → upsert `works` row. Any failure →
   full rollback, exit 17.
10. **Reposition + report** — see `shell-integration.md`. Always print the success summary to
    stdout.

Cancelling (Ctrl-C) at any point before step 9's commit → rollback of anything already done,
exit 20.

## Non-interactive flow (stdin OR stdout not a TTY)

- No TUI is ever constructed (FR-024, RF-51).
- Every required value must come from a flag/arg: `SOURCE`, `--base`, `--slug`, and
  `--prefix`; plus `--workspace` when no root is configured. Any missing → exit 2 with a
  message naming the missing flag. **No** repository mutation, **no** config write on that failure.
- `--yes` is required to pass the confirm step; without it → exit 2.
- All validations from the interactive flow still run (steps 2, 5). A validation failure exits
  with its code (10/11/12/13/14/15) — there is no re-prompt.
- On success, behavior matches interactive step 9–10 minus the confirm prompt.

## stdout (success, exit 0)

Stable, greppable. Exactly these lines, in order (FR-025):

```
work: created <id>
work: branch <branch>  (from <base-branch> @ <base-short-sha>)
work: path <absolute-worktree-path>
```

Nothing else goes to stdout on success. Post-creation shell notices (FR-023) go to **stderr**.

## stderr

- All prompts and progress indicators (interactive only).
- On the FR-023 path (shell integration not active): a notice block — see `shell-integration.md`.
- On failure: one `error: <token>: <message>` line plus optional actionable follow-up lines.

## Exit codes (R19)

| Code | Token | Meaning |
|---|---|---|
| 0 | `ok` | Work created (a shell notice on stderr is still exit 0) |
| 2 | `usage` | bad/missing flags; non-interactive missing required value |
| 10 | `invalid-path` | SOURCE missing / not a dir / unreadable |
| 11 | `unusable-repo` | not a git repo / bare / no commits |
| 12 | `no-base-branch` | repo has no selectable base branch |
| 13 | `invalid-branch-name` | derived name fails `git check-ref-format` |
| 14 | `branch-collision` | derived name collides with local / remote-tracking / worktree branch |
| 15 | `destination-unavailable` | target Work directory exists / occupied / not writable |
| 16 | `bootstrap-failed` | git preflight or seed bootstrap failed |
| 17 | `materialization-failed` | a create step failed; rollback completed, no orphans |
| 20 | `cancelled` | user declined confirmation / interrupted before commit |

## Invariants (verified by tests — SC-002, SC-003, SC-004, FR-028)

- On exit ≠ 0 from steps 1–8: zero new branches, worktrees, Work directories, snapshots, or
  `works` rows attributable to this attempt. Config/bootstrap state may persist.
- On exit 0: `internal/work/verify.Check` passes (snapshot ↔ worktree ↔ git branch ↔ db row).
- The source repository's checkout, current branch, and uncommitted changes are unchanged
  (FR-014, acceptance scenario 3).
- Re-running with the same inputs after a success → exit 14 (branch-collision), no partial state.
