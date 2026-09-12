# Contract: Repository Resolution Policy & Search Roots (F3)

Authority: ADR-0015, ADR-0016; ADD §7.2 & §12.5; spec FR-020–FR-028, NFR-9.

## Config file (`~/.work/config/work.json`)

**No new key.** F1 already declared both fields; F3 starts writing them past
bootstrap.

```jsonc
{
  "workspace": "/home/user/.workspaces",          // unchanged, F1
  "repository_roots": ["/home/user/src", "/home/user/projects"],
  "repository_resolution": {
    "locators": ["work-reference/filesystem-repository-locator"]
  }
}
```

- Both lists default to `[]` (`config.Default`). A missing file ⇒ defaults, no
  write. Writes stay atomic (`config.Save` → `atomicfile`).
- `repository_roots` entries are **plain absolute paths** (leading `~` expanded,
  not symlink-resolved — matches `workspace` storage).
- `repository_resolution.locators` entries are **qualified references**
  `"<alias>/<component>"`, in **traversal order**.

## Ordering & precedence (ADR-0015, NFR-9)

- The list order is the **only** precedence. No numeric priority, no
  installation order, no manifest-declared position, no core heuristic.
- Resolution is a chain of responsibility over this list (`repository-locator.md`).

## Availability

A policy entry is **available** iff the registry holds a component with that
`<alias>/<component>` and `role == "repository-locator"`.

- Unavailable entries are **kept in place** and shown marked (FR-025) — never
  dropped.
- Resolution **skips** unavailable entries and continues.
- F3 only produces the unavailable state via a hand-edited `work.json` that names
  a never-installed Locator. Plugin **disable** (Locator temporarily unavailable,
  reference kept) and **uninstall** (remove references on confirmation) are
  **F7** and out of scope here.

## No silent insertion (FR-027, ADR-0015)

Only two writers ever append to `repository_resolution.locators`:

1. `bootstrap` — the seed `filesystem-repository-locator`, once, at first run;
2. `work repository policy add` — explicit user action.

Installing or enabling **any** plugin (F4+) MUST NOT touch the list. A test
installs a second fixture Locator and asserts the policy is byte-identical
(SC-005).

## `work repository` grammar (ADR-0019 direct API)

```
work repository                                    # prints grouped help, exit 0 (hub deferred)
work repository locator list                        [--json]
work repository policy list                         [--json]
work repository policy add <LOCATOR> [--before <LOCATOR> | --after <LOCATOR>]
work repository policy remove <LOCATOR...>
work repository policy move <LOCATOR> (--before <LOCATOR> | --after <LOCATOR>)
work repository policy replace <LOCATOR...>
work repository root list                           [--json]
work repository root add <PATH...>
work repository root remove <PATH...>
work repository root replace <PATH...>
```

- Parent commands carry `GroupID = "admin"` → shown under **Administration** in
  `work --help`.
- `<LOCATOR>` is `"<alias>/<component>"`; a bare `<component>` is accepted when
  unambiguous across installed Locators and echoed back qualified.
- `add` optional positioning; `move` requires **exactly one** of
  `--before`/`--after` (zero or both → `usage`, exit 2).
- Reads (`locator list`, `policy list`, `root list`) honour `--json` and are
  **pure**. Mutations **reject** `--json` (exit 2), like `work start`/`resume`.
- `--yes` is not used by this surface (no destructive confirmation in F3).

### Operation semantics

| Command | Behaviour | Exit |
|---|---|---|
| `policy add <ref>` | `ref` must resolve to a registered repository-locator; append or position; adding a present `ref` is a no-op | 0; `usage` 2 on unknown `ref` or bad positioning |
| `policy remove <ref...>` | drop each; component stays installed & enabled; absent `ref` is a no-op | 0 |
| `policy move <ref> --before\|--after <x>` | `ref` and `x` must be present; reorder | 0; `usage` 2 otherwise |
| `policy replace <ref...>` | validate every `ref`, then atomic write of the new list | 0; `usage` 2 if any unknown |
| `root add <path...>` | expand `~`, abs; each must be an existing **readable directory** that is **not** equal to or nested within the workspace root (and the workspace root must not be nested within it); canonical dedup | 0; `usage` 2 on a non-dir / unreadable / workspace-overlapping path |
| `root remove <path...>` | canonical match; absent is a no-op | 0 |
| `root replace <path...>` | validate all (incl. workspace-overlap), atomic write | 0; `usage` 2 on any bad path |

### `--json` shapes

```jsonc
// locator list
{ "locators": [ { "ref": "work-reference/filesystem-repository-locator",
                  "display_name": "Filesystem repositories",
                  "description": "Finds local Git repositories in configured search roots",
                  "accepts": ["name","git_fetch_urls","query"],
                  "in_policy": true } ] }

// policy list
{ "policy": [ { "ref": "work-reference/filesystem-repository-locator",
                "position": 1, "available": true } ] }

// root list
{ "roots": ["/home/user/src", "/home/user/projects"] }
```

Human output: one row per line, greppable; empty collection prints a `note:` line
to stderr and exits 0.

### Mutation output

One stable line to stdout, e.g.:

```
work: policy now work-reference/filesystem-repository-locator, acme/corp-index
work: roots now /home/user/src, /home/user/projects
```

## `work repository` with no subcommand (hub deferred)

- Prints the grouped help for `locator` / `policy` / `root` to **stdout** and
  exits **0**, in every stream configuration. No menu, no mutation.
- The interactive `repository` hub of ADR-0019 — and the equivalent-command echo
  it would carry (FR-029) — is **deferred past F3**. A later slice ships it; when
  it lands, each hub action must call the same `internal/repoconfig` operation as
  the direct command (no second code path).
- Full behaviour: [`cli-work-repository.md`](./cli-work-repository.md).

## Tests

| Layer | Coverage |
|---|---|
| `internal/repoconfig` unit | every operation; unknown-locator rejection; availability marker; canonical root dedup |
| `tests/integration/repository_policy.txtar` | list/add/remove/move/replace output + exit codes; unavailable entry shown; second Locator install ⇒ policy unchanged (SC-005) |
| `tests/integration/repository_root.txtar` | list/add/remove/replace; non-dir rejected; workspace-overlap rejected in both directions; `--json` shapes |
| `tests/integration/repository_locator_list.txtar` | `--json` + human shapes; `in_policy` flag |
| `tests/integration/repository_help.txtar` | `work repository` with no subcommand → grouped help, exit 0, no ANSI on non-TTY, `--json` rejected |
| `tests/integration/help_inventory.*` | `work repository` appears once under Administration |
