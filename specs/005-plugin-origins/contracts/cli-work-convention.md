# Contract: `work convention` (F4)

Authority: ADR-0011, ADR-0012, ADR-0019, ADD §8; research R13–R16.

## Scope note

Unlike `work plugin`/`work repository`, `work convention`'s interactive hub is
**not deferred** in this slice — FR-030 and ADR-0011 require it, and the
spec's Out of Scope section names only `work plugin`'s hub as pushed to F7.
This is the first genuinely interactive hub this codebase ships.

## Repository identity resolution (shared by every subcommand and the hub)

Every form below first discovers the git repository rooted at (or above) the
current working directory (`gitx.DiscoverRepoRoot`, `git rev-parse
--show-toplevel`) and computes its identity key (ADR-0011, three layers —
`data-model.md` §9). Outside a git repository, every form fails identically:

```text
error: usage: not inside a git repository; run from any clone
```

(exit 2, `usage` — this is a client-context error, not a new category).

## `work convention show [--json]`

Read-only (FR-028): does not create, persist, or otherwise touch a
memoized choice, recent access, or any other state — including the case
where none exists yet.

### Output (stdout, text)

```text
work: convention <name>
```

or, when nothing is memoized for this repository's identity yet:

```text
work: convention not set
```

### Output (`--json`)

```jsonc
{ "identity": "<repository identity key>", "convention": "gitflow" }
```

or `{ "identity": "<key>", "convention": null }` when unset.

### Exit codes

| Code | Token | Condition |
|---|---|---|
| 0 | — | Always (a repository with no memoized choice is not an error) |
| 2 | `usage` | Not inside a git repository |

## `work convention set <CONVENTION>`

Mutation. Replaces the memoized choice for this repository's identity,
independent of which clone the command runs from (FR-027).

### Behaviour

1. Resolve repository identity (as above).
2. Look up `CONVENTION` in the enabled catalog (`registry.Conventions`,
   loaded across every installed package). If not found: fail, persisting
   nothing (`convention-unknown`, exit 38).
3. Persist `{identity, CONVENTION}` into `~/.work/state/
   branch_conventions.json` (`internal/repoconv.Set`), overwriting any
   existing entry for this identity.

### Output (stdout, success)

```text
work: convention set to <name>
```

### Exit codes

| Code | Token | Condition |
|---|---|---|
| 0 | — | Set |
| 2 | `usage` | Not inside a git repository; missing `CONVENTION` argument |
| 38 | `convention-unknown` | `CONVENTION` is not currently enabled |

## `work convention` (no subcommand)

### Interactive terminal

Opens a `present.Wizard` hub:

1. A read-only line showing the current choice (or "not set").
2. A `present.SelectStep` offering every enabled convention (current choice,
   if any, pre-marked) plus the option to leave it unchanged.
3. On acceptance of a different choice, persists it exactly as `set` does,
   then prints, as the receipt, the equivalent direct command:

```text
work: convention set to gitflow
work: (equivalent: `work convention set gitflow`)
```

4. Leaving it unchanged (or cancelling) persists nothing and reports a single
   concise cancellation result exactly like every other F2.5 journey
   (`cancelled`, exit 20) if explicitly cancelled, or exit 0 with no receipt
   if the user re-confirms the existing choice unchanged.

### Non-interactive

Fails exactly like `work start` with a missing required argument in
non-interactive mode — an actionable usage error, never a silently-opened
hub:

```text
error: usage: run `work convention show` or `work convention set <convention>`; see `work --help`
```

(exit 2, `usage`).

## Contract tests (tests/integration/convention_show_set.txtar, PTY convention_hub_test.go)

| Case | Command | Expect |
|---|---|---|
| show, unset | `work convention show [--json]` in a fresh repo | exit 0; "not set" / `convention: null` |
| show, set | after a fork-mode `work start` memoized a choice | exit 0; the memoized name, from a second clone too |
| set, unknown | `work convention set nope` | exit 38, nothing persisted |
| set, ok | `work convention set gitflow` (enabled) | exit 0; `show` from any clone now reports it |
| outside a repo | any subcommand run from a non-git directory | exit 2, no hub, no crash |
| hub, interactive | `work convention` (PTY, inside a repo with 2+ conventions) | shows current choice; change persists; equivalent command receipt shown |
| hub, non-interactive | `work convention` (no TTY) | exit 2, no selector opened |
