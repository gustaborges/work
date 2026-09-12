# Contract: `work repository` (F3)

CLI-surface contract for the new command family. Operation semantics, `--json`
shapes, and config rules live in [`resolution-policy.md`](./resolution-policy.md);
this document governs args/flags, streams, non-interactive behaviour, and exit
codes.

Authority: ADR-0019 (grammar & discovery), ADD §7.2 & §12.5; spec FR-022,
FR-024, FR-029, FR-034; F2.5 `contracts/cli-help.md` + `contracts/cli-work-home.md`
(parity for `--json`, colour opt-out).

## Scope note — the interactive hub is deferred

ADR-0019 lists an interactive `repository` hub among the preserved human surface.
**F3 does not ship it.** `work repository` with no subcommand behaves like an
ordinary command that prints its help. ADR-0019's text is left intact; a later
slice delivers the full-screen hub. Everything ADR-0019 fixes about the *direct*
grammar is implemented here in full.

## Synopsis

```
work repository                                    # prints grouped help, exit 0
work repository locator list                        [--json]
work repository policy  list                         [--json]
work repository policy  add     <LOCATOR> [--before <LOCATOR> | --after <LOCATOR>]
work repository policy  remove  <LOCATOR...>
work repository policy  move    <LOCATOR> (--before <LOCATOR> | --after <LOCATOR>)
work repository policy  replace <LOCATOR...>
work repository root    list                         [--json]
work repository root    add     <PATH...>
work repository root    remove  <PATH...>
work repository root    replace <PATH...>
```

- `work repository`, `work repository policy`, `work repository root` are parent
  commands with `GroupID = "admin"` — they appear once, under **Administration**,
  in `work --help` (SC — help lists every available command exactly once).
- There are **no** command or resource aliases (ADR-0019).

## Discovery (FR-022)

`work` → brand → `work --help` → **Administration** → `work repository`. Every F3
capability is reachable this way and directly invocable by the grammar above.
`work repository` with no subcommand prints the grouped help for `locator`,
`policy`, and `root` (the same content as `work repository --help`).

## `work repository` with no subcommand

| Stream configuration | Behaviour |
|---|---|
| interactive (stdin & stdout TTY) | prints grouped help to stdout, **exit 0** |
| non-interactive / redirected | identical text, no colour, no ANSI, **exit 0** |

- It opens **no** menu, wizard, or selector, and mutates nothing.
- It is the only command in this family whose no-arg form exits 0 rather than 2 —
  it is informational (a help listing), not a missing mandatory value. `work
  repository --json` is still rejected (**exit 2**), as on every mutation.

## Subcommand streams & interactivity

| Invocation | Interactive | Non-interactive |
|---|---|---|
| a `list` subcommand | plain table to stdout (colour only on a colour TTY) | identical, no colour, no ANSI |
| a mutation subcommand | runs directly; one stable stdout line | identical |

- No subcommand opens a journey/wizard. These are flat, synchronous operations.
- `--json` on a `list`: machine-readable object, **pure**, exit 0 even when empty.
- `--json` on a mutation: **exit 2** (rejected, as on `work start`/`work resume`).
- Colour is disabled for non-TTY output, non-empty `NO_COLOR`, or `TERM=dumb`;
  those streams carry zero control sequences (F2.5 `theme.md` parity).

## Streams

- **stdout**: the grouped help (no-subcommand), `list` tables, and the one-line
  mutation result.
- **stderr**: `note:` lines for empty collections, and any diagnostic.
- On failure: one `error: <token>: <message>` line.

## Exit codes

| Code | Token | When |
|---|---|---|
| 0 | `ok` | success (including `work repository` help, no-op add/remove, empty `list`) |
| 2 | `usage` | unknown `<LOCATOR>`; `move` without exactly one of `--before`/`--after`; `root add`/`replace` on a non-directory / unreadable path, or a path overlapping the workspace root in either direction; `--json` on a mutation |
| 16 | `bootstrap-failed` | `~/.work` unreadable / config file malformed (via `config.Load`) |

F3 adds **no** new exit code for this surface; the resolution codes 26–30 belong
to `work start` (`cli-work-start.md`).

## Invariants

- Every mutation is a single atomic `config.Save`; a rejected mutation writes
  nothing.
- `list` commands and the no-subcommand help never change `work.json`, recent
  access, checkout, or provenance.
- The persisted `work.json` after any sequence of these commands is valid input
  to `config.Load` and to a subsequent `work start` (round-trip — FR-034).
- No configured search root is equal to or nested within the workspace root, and
  the workspace root is not equal to or nested within any search root — enforced
  on every `root add`/`replace` (FR-020).

## Tests

`tests/integration/repository_policy.txtar`, `repository_root.txtar`
(incl. workspace-overlap rejection both directions), `repository_locator_list.txtar`
(grammar, `--json`, exit codes, purity); `repository_help.txtar` (`work
repository` prints grouped help, exit 0, no ANSI on a non-TTY, `--json` still
rejected); `tests/integration/help_inventory.*` (appears once under
Administration). See also `resolution-policy.md` §Tests.
