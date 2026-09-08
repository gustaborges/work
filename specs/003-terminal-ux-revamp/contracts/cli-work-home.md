# Contract: bare `work` (F2.5)

**Supersedes the presentation behaviour of**
`specs/001-first-local-work/contracts/cli-work-home.md` and
`specs/002-daily-cycle/contracts/cli-work-home.md`. Those documents remain historical
delivery records; the selectable TUI home they describe is removed.

**Authority**: `docs/prd.md` RF-50, RF-63; ADR-0019; ADD §12.1; spec FR-017–FR-019,
FR-022, FR-028; research R11/R12/R14.

## Synopsis

```
work            # no arguments
```

## Behaviour

| Environment | Output channel | Content | Exit |
|---|---|---|---|
| stdin **and** stdout are TTYs | **stdout** | the static brand block: the `WORK` wordmark (terminal art, primary→secondary gradient when the terminal supports it — see `brand.md`), the tagline, and `Run 'work --help' to get started.` | **0** |
| stdin or stdout not a TTY (piped, redirected, CI) | stderr | the existing one-line usage summary naming `work start`, `work resume`, `work archive`, and `work --help` | **2** |

- The interactive form **MUST NOT** open a selector, menu, or any Bubble Tea
  program. It is static text; the process renders it and exits.
- The interactive form **MUST** exit 0 (successful invocation).
- The non-interactive form is **unchanged from F1/F2**: same message intent, exit 2,
  empty stdout. `internal/cli/root_test.go:TestBareWorkNonInteractiveIsUsage` stays
  green.
- No journey is started from bare `work`. `work start` / `work resume` /
  `work archive` are invoked by name; they remain directly invocable and are
  discoverable via the brand's `work --help` direction (`cli-help.md`).
- When colour is disabled (`NO_COLOR` non-empty, `TERM=dumb`, or non-TTY) the brand
  contains **zero** styling control sequences (`theme.md`, FR-025).

## Reachability invariant (FR-022, SC-006)

Every public journey is reachable by a documented command and discoverable from bare
`work` → `work --help`:

| Journey | Discover via | Invoke |
|---|---|---|
| Create a local Work | `work --help` → `Daily Commands` | `work start [SOURCE]` |
| Resume a Work | `work --help` → `Daily Commands` | `work resume [WORK]` |
| Archive Works | `work --help` → `Daily Commands` | `work archive [WORK...]` |

`work shell-init` and `work completion` are setup plumbing under `Setup` in help.

## Removed

- `internal/tui/home.go` (`homeModel`, `HomeChoice`, `RunHome`) and `home_test.go`.
- The `runHome` dispatch switch in `internal/cli/root.go`.
