# Contract: diagnostic rendering border (F2.5)

**Authority**: `docs/prd.md` RF-57, RF-58, RF-62; ADR-0020; ADD §12.4; spec
FR-012–FR-016, FR-026; SC-007, SC-010; research R10.

## `internal/diag` changes

- **New optional fields** on `diag.Error`: `Summary string` (user-vocabulary
  statement of what failed) and `Hint string` (the known next action).
- **New accessor** `Cause() error` (alias of `Unwrap`).
- **Unchanged**: `Category`, `Token`, `Code`, the `Cancelled` category (`code 20`),
  the full `All` table, `Format`, `ExitCode`, `Token`. `diag_test.go:TestCategoryTable`
  is not modified. **No new category, token, or exit code is introduced.**

## Single border (FR-014)

Exactly one place prints a terminal failure: `internal/cli/diagnostics_border.go`,
invoked from `Execute` at the CLI/process boundary. `internal/present`, the
orchestrators (`create`, `resume`, `archive`), and the domain/infra layers **return**
structured errors and **never print**. The removed `fmt.Fprintln(errOut,
diag.Format(ierr))` calls inside `runStart` are not reintroduced anywhere.

## Renderer selection

| UI channel | Error | Output | Exit |
|---|---|---|---|
| **non-interactive** (UI writer not a TTY) | any | `error: <token>: <Msg>` — one line, **byte-for-byte as F1/F2** | category `Code` |
| **interactive** | `Cancelled` | `✘ Operation cancelled` — one line | **20** (unchanged) |
| **interactive** | any other `*diag.Error` | `✘ <Summary if set, else Msg>`; then, if `Hint != ""`, a second line `  → <Hint>` | category `Code` |
| **interactive** | non-`diag` error | `✘ something went wrong` + `  → run with WORK_DEBUG=1 for details` | 1 |

- The interactive renderer uses the `Danger` token for `✘` (glyph carries the
  meaning with colour off — `theme.md`).
- Internal wrapping chains are **never** shown in normal output (FR-015).

## Cause inspection (FR-016)

- The wrapped cause chain stays reachable via `errors.Unwrap` / `errors.As` — the
  API is unchanged.
- Setting `WORK_DEBUG` to a non-empty value makes the border additionally print the
  full chain (`%+v`-style) to the UI channel, after the human line. This is an env
  affordance, **not** a new flag, and does not change the normal human or
  programmatic output (the scope note forbidding a new flag is about `--no-color`).

## Stream contract (FR-026, SC-007)

- Interactive presentation and human diagnostics → the configured UI writer
  (`cmd.ErrOrStderr()`, normally stderr).
- Stable command results → **stdout**, exactly as the F1/F2 command contracts
  define. The revamp adds nothing to stdout for any existing command (the bare
  interactive `work` brand is the sole, contracted stdout addition — `cli-work-home.md`).
- Every existing argument, flag, stable stdout line, error token, and exit code is
  unchanged (SC-007). The only contracted behavioural change is interactive bare
  `work` → exit 0.

## Cancellation semantics (FR-012, FR-013, SC-010)

- Interactive cancellation leaves **one** concise human line and performs **zero**
  mutations (no state, no `last_accessed_at` bump, no files/branches/worktrees/config).
- The programmatic outcome is unchanged: commands that contract exit 20 for
  cancellation still exit 20; `diag.Cancelled`'s token stays `cancelled`.
