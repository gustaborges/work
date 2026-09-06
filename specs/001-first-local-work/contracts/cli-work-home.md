# Contract: `work` TUI home (F1)

Authority: spec FR-029, RF-50; PRD §10; ADR-0017; research R6.

## Synopsis

```
work            # no arguments, interactive terminal → TUI home
work            # no arguments, non-interactive → exit 2 with usage
```

## Behavior

- **Interactive (stdin AND stdout TTY):** open a Bubble Tea home screen.
  - F1 lists exactly one reachable journey: **"Start a Work"**, which enters the same flow as
    `work start` with no `SOURCE` (prompts for the path first).
  - Keyboard-navigable: ↑/↓ or j/k to move, Enter to select, q / Ctrl-C to quit.
  - The home MUST NOT show actions reserved for later slices (`resume`, `archive`, `status`,
    `import`, `link`, `plugin`, `repository`, `convention`) — F1 exposes none of them
    (FR-029: "need not expose actions reserved for later slices").
  - Quitting the home without choosing anything → exit 0, no state change.
- **Non-interactive:** print a one-line summary of available commands to stderr and exit 2.
  Never render a TUI (RF-51).

## Reachability invariant (FR-029, RF-50)

Every public F1 journey is reachable from `work` with no arguments:

| Journey | Reachable via |
|---|---|
| Create the first local Work | home → "Start a Work" |

`work shell-init` is setup plumbing, not a "journey", and is intentionally not listed in the
home (it is documented in `--help` and in the FR-023 notice).

## Exit codes

| Code | Meaning |
|---|---|
| 0 | home opened and exited normally, or a selected sub-journey completed with exit 0 |
| non-0 | propagated from the selected sub-journey (see `cli-work-start.md`) |
| 2 | invoked non-interactively |
