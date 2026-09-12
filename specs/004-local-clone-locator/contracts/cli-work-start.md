# Contract: `work start` — F3 amendment

**Amends** `specs/001-first-local-work/contracts/cli-work-start.md`. Two things
change: the `SOURCE` clause (a name/reference is now accepted), and the
interactive wizard gains up-front first-run setup. Everything else in that
document (preconditions, the transactional materialisation, the success stdout
lines, and exit codes 0/2/10–17/20) stays authoritative and is protected by the
regression gate (SC-011). The wizard on an install where the workspace root and
at least one search root are already configured is **unchanged**.

Authority: ADR-0014, ADR-0015, ADR-0016, ADR-0019; ADD §7.1; spec FR-001–FR-004,
FR-020, FR-022a, FR-033, FR-035, FR-036, FR-037.

## The changed clause

F1 said:

> `SOURCE` — **F1: a direct path to a local Git repository only.** No
> name/URL/root lookup fallback is attempted (FR-002).

F3 replaces it with:

> `SOURCE` — a path **or** a name/reference. The matched Starter turns it into a
> Repository Reference (`repository-reference.md`). A reference carrying `path` is
> validated directly (unchanged F1 behaviour). A reference without `path` is
> resolved through the Repository Resolution Policy (`repository-locator.md`)
> before any Work is materialised. Resolution must yield **exactly one** valid
> local Git repository for the journey to continue.

No new flag. `--workspace/--base/--slug/--prefix/--yes/--json` are unchanged.

## Argument classification (seed `local-path-starter`)

The seed Starter emits `repository.path` when the argument **looks like a
filesystem path**:

- contains a path separator (`/` or, on Windows, `\`), or
- starts with `.`, `..`, `~`, or a drive letter, or
- names an existing filesystem entry.

Otherwise it emits `repository.name = <argument>`. It never emits
`git_fetch_urls` or `query` in F3. (A future Starter with a `pattern` may classify
differently; the fallback Starter's job is only to get a bare token into the
chain — FR-032.)

## First-run setup (interactive only, new — precedes every step)

Before the Source step, on an **interactive** `work start`, the wizard ensures the
location configuration exists:

1. **Workspace root** — asked only when `config.Workspace` is unset. This is the
   F1 workspace-root step (`FR-006`: suggest a value, allow change, validate
   fitness, persist) **moved to the front** of the wizard. The prompt states its
   purpose: where Work stores and organises worktrees; in-progress and archived
   Works live here, kept separate from source clones.
2. **Repository search root** — asked only when `config.RepositoryRoots` is empty.
   Accepts one directory path (`~` expanded, absolutised). The prompt states its
   purpose: a directory that holds your Git clones, so `work start <name>` can
   find them without a full path; more can be added later with
   `work repository root add`. Validated as an existing readable directory that
   does **not** overlap the workspace root in either direction (`FR-020`);
   persisted via `config.Save` before resolution.

- Both are **first-run only**: once `Workspace` is set and `RepositoryRoots` is
  non-empty, neither prompt appears and the wizard is byte-identical to F1/F2.5.
- The two values are written in a **single** `config.Save` once both prompts are
  answered (nothing is persisted incrementally). Cancelling either prompt
  (`FR-037` / F2.5 cancellation parity) persists nothing and creates no Work;
  exit 20. When only the workspace prompt was needed (a root already exists), it
  is saved on its own exactly as in F1.
- **Non-interactive** `work start` runs **no** setup:
  - `work start <path>` proceeds with no search root configured (unchanged F1).
  - `work start <name>`/reference with `RepositoryRoots` empty →
    `no-repository-found` (**exit 26**) with a hint to run `work repository root
    add <dir>` (or start interactively). It does not prompt or hang.

## Interactive flow (delta from F1 steps 1–2)

The single **Source** step now:

1. runs the Starter → Repository Reference;
2. **if `path`** → `reporef.ValidatePath` exactly as F1 (a rejected path is shown
   in-frame and re-promptable for `invalid-path`/`unusable-repo`);
3. **else** → `locator.Resolve`:
   - one repo → accept the step, `repoPath` set;
   - ≥2 repos → accept the step, stash candidates, mark ambiguity pending;
   - `no-repository-found` (26) → shown in-frame, re-promptable (a different
     reference may resolve);
   - `no-eligible-locator` (27) / `repository-candidate-invalid` (28) /
     `locator-failed` (29) → terminal: the wizard aborts to the diagnostic border.

A new conditional **Repository** step (`present.Select`) appears **only** when
ambiguity is pending: it lists the deduplicated candidates — primary line the
absolute path, secondary line the first remote fetch URL or the parent directory
(research R15) — bounded and filterable per F2.5, collapsing to a
`name (parent)` receipt on accept. The chosen path is `repoPath`; the chain is
**not** resumed (FR-011).

The remaining steps (prefix, slug, branch derivation + collision, base branch,
confirm, materialise, report) are **unchanged**. The F1 workspace-root step no
longer appears here on a fresh install — it runs in first-run setup above — and
on an already-configured install it was already skipped in F1.

## Non-interactive flow (delta from F1)

- An explicit `SOURCE` that resolves to **one** repo proceeds exactly as F1.
- `SOURCE` resolving to **≥2** repos → **exit 30** `repository-ambiguous`, with a
  hint to pass a more specific reference or adjust `repository_roots`. **No
  selector is opened** (FR-033).
- `RepositoryRoots` empty and the reference needs resolution → exit 26
  `no-repository-found`, hint to configure a root (no setup prompt).
- `no-repository-found` → exit 26 (no re-prompt). `no-eligible-locator` → 27.
  `repository-candidate-invalid` → 28. `locator-failed` → 29.
- On every one of these exits: **zero** new branch, worktree, Work directory,
  snapshot, or `works` row; config unchanged; workspace root not persisted
  (FR-037, SC-010).

## Exit codes — F3 additions

| Code | Token | Meaning |
|---|---|---|
| 26 | `no-repository-found` | policy traversed, no Locator matched the reference |
| 27 | `no-eligible-locator` | policy empty, or no Locator accepts any field the reference carries |
| 28 | `repository-candidate-invalid` | matches were found but none is a usable Git repository |
| 29 | `locator-failed` | a Locator errored; resolution halted with no fallback |
| 30 | `repository-ambiguous` | ≥2 repos matched and the run could not prompt |

Codes 0/2/10–17/20 keep their F1 meanings. A located clone produces the **same**
three success stdout lines as a typed path (`work: created / branch / path`), and
`internal/work/verify.Check` still passes (SC-008).

## Tests

| Layer | Coverage |
|---|---|
| `tests/integration/start_by_name_test.go` (pty) | name → one match completes; name → picker → chosen repo materialised; picker geometry at 40×10 / 80×24 / 160×50 |
| `tests/integration/start_first_run_test.go` (pty) | fresh install: first two prompts are workspace root + search root, each with a purpose line; both persisted before resolution; cancelling either → exit 20, nothing written; already-configured install shows neither |
| `tests/integration/start_by_name_non_interactive.txtar` | one match proceeds; `<name>` with no roots → exit 26 + hint (no prompt); `<path>` with no roots still proceeds; `--json` still rejected; missing SOURCE still exit 2 |
| `tests/integration/resolution_outcomes.txtar` | exits 26 / 28 / 29 distinct, no Work |
| `tests/integration/*` ambiguity | exit 30 non-interactively, no selector, no Work |
| F1 `create_happy.txtar`, `invalid_path.txtar`, `rollback.txtar`, … non-interactive | **unchanged, green** — `work start <path>` regression |
| F1 interactive fresh-install pty scenarios | pty input scripts gain the two up-front setup answers; stdout, exit codes, and resulting snapshot unchanged |
| head-to-head test | `work start <path>` vs `work start <name>` (same repo) → identical snapshot + repositioning (SC-008) |
