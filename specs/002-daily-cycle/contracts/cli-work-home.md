# Contract: `work` TUI home (F2)

Supersedes `specs/001-first-local-work/contracts/cli-work-home.md` for the F2 slice.
Authority: spec FR-030; ADR-0017; PRD §10; research R19.

## Synopsis

```
work            # no arguments, interactive terminal → TUI home
work            # no arguments, non-interactive → exit 2 with a one-line command summary
```

## Behavior

- **Interactive (stdin AND stdout TTY):** a Bubble Tea home screen listing **three**
  journeys, in this order:

  | Row | Enters |
  |---|---|
  | Start a Work | `work start` with no `SOURCE` (F1, unchanged) |
  | Resume a Work | `work resume` with no target → the recency picker (`cli-work-resume.md`) |
  | Archive Works | `work archive` with no targets → the multi-select picker (`cli-work-archive.md`) |

  - Keyboard-navigable: `↑/↓` or `j/k` move, `Enter` select, `q` / `Esc` / `Ctrl-C` quit.
  - Quitting without choosing → exit 0, no state change.
  - The home MUST NOT show actions reserved for later slices: `status`, `import`, `link`,
    `plugin`, `repository`, `convention`.
- **Non-interactive:** print a one-line summary naming `work start`, `work resume`,
  `work archive` (and `work --help`) to stderr, exit 2. Never render a TUI (FR-029).

## Reachability invariant (FR-030)

Every public F2 journey is reachable from `work` with no arguments and by keyboard alone:

| Journey | Reachable via |
|---|---|
| Create a local Work | home → "Start a Work" |
| Resume a Work by recency | home → "Resume a Work" |
| Archive one or more Works | home → "Archive Works" |

`work shell-init` remains setup plumbing and is intentionally not listed.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | home opened and exited normally, or a selected sub-journey completed with exit 0 |
| non-0 | propagated from the selected sub-journey (`cli-work-start.md`, `cli-work-resume.md`, `cli-work-archive.md`) |
| 2 | invoked non-interactively |
