# Contract: `work --help` (F2.5)

**Authority**: `docs/prd.md` RF-50, RF-63; ADR-0019 §12.1; ADD §12.1; spec
FR-020, FR-021; SC-005; research R15.

## Synopsis

```
work --help
work -h
work help [COMMAND]
```

Exits **0 in every environment** (interactive, piped, `NO_COLOR`, `TERM=dumb`).

## Rendered structure

```
<compact WORK brand line>
<tagline>

Usage:
  work [command]

Daily Commands:
  start       Create a new Work from a local git repository
  resume      Resume an existing Work
  archive     Archive one or more Works

Setup:
  shell-init  Print the shell integration snippet
  completion  Generate the autocompletion script for the given shell

Flags:
  -h, --help   help for work

Use "work [command] --help" for more information about a command.
```

- The header is the **compact** brand form (`brand.md`) regardless of terminal
  width — never the full art in help.
- Command blocks are rendered **one per non-empty group**, in this fixed order:
  `Daily Commands`, `Inside a Work`, `Administration`, `Setup` (`data-model.md` §6).
- A group with no registered command is **not printed** (SC-005: zero empty groups).
- Each command appears **exactly once**, under its `GroupID`'s block, with its
  `Short` string.
- The command list is derived from `root.Commands()` — there is **no** hand-written
  inventory. A command that is not registered in the running binary cannot appear
  (SC-005: zero unavailable commands).
- Cobra's auto-generated `help` and `completion` commands are given a `GroupID`
  (`setup`) or hidden, so no command is ungrouped.
- When colour is enabled, group titles use the `Primary` token and flag names use
  `Secondary`; when disabled, the output contains no control sequences and stays
  legible (FR-025).

## Correspondence test (SC-005)

A test walks `root.Commands()` and asserts:

1. every non-hidden command has a non-empty `GroupID` that matches one of the four
   declared groups;
2. every declared group that renders has ≥ 1 member;
3. the count of command lines in the rendered help equals the count of non-hidden
   registered commands;
4. no group title renders with zero lines beneath it.

## Non-interactive

Piped / redirected `work --help` emits the same text with **no ANSI escapes** and
exit 0 (`quickstart.md` Q10).
