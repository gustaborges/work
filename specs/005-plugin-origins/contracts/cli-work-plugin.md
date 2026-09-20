# Contract: `work plugin install|list` (F4)

Authority: ADR-0000, ADR-0002, ADR-0003, ADR-0006, ADR-0012, ADD §4/§5;
`specs/001-first-local-work/contracts/plugin-manifest.md` (manifest validation,
unchanged and reused verbatim — this contract governs only the new install-time
caller around it); research R1–R6, R18.

## Scope note

`work plugin` with no subcommand prints grouped help for `install`/`list` and
exits 0 in every stream configuration, mirroring `specs/004-local-clone-
locator/contracts/cli-work-repository.md`'s scoped deferral exactly — the
interactive `work plugin` hub and `enable|disable|update|uninstall` are F7
(spec Out of Scope). `--json` on the bare command is rejected as a usage error
(it prints help, not data), identical to `work repository`.

## `work plugin install <SOURCE> [--link] [--as <ALIAS>]`

### Grammar

- `SOURCE` (required, positional): a local filesystem path or a remote source
  (e.g., a git URL). Classified with the same heuristic
  `seed/starter/main.go`'s `looksLikePath` already uses for `work start`'s
  argument (research R4): a path separator, a `.`/`..`/`~`/drive-letter
  prefix, or an existing filesystem entry means "local"; anything else means
  "remote".
- `--link` (bool, default false): install the local `SOURCE` by reference
  (directory symlink at `plugins/<alias>/source`, research R2) instead of
  copying. **Invalid with a remote `SOURCE`** — rejected before any I/O
  (`usage`, exit 2).
- `--as <ALIAS>` (string, optional): override the default alias (the
  manifest's `name`).

### Behaviour

1. Classify `SOURCE`; reject `--link` + remote immediately.
2. Obtain the content:
   - **Local, no `--link`**: copy `SOURCE`'s tree into staging (a *pinned*
     copy — later edits at `SOURCE` do not affect the installed plugin).
   - **Local, `--link`**: no copy; the final `plugins/<alias>/source` is a
     directory symlink (junction on Windows) to `SOURCE`.
   - **Remote**: `git clone --depth 1 SOURCE <tmp>`; the pinned reference is
     `git rev-parse HEAD` of the clone; the clone's tree (minus `.git`) is
     copied into staging.
3. Parse and fully validate `plugin.json` at the content root
   (`internal/plugin.Parse`). Any violation fails the whole install —
   nothing is registered, nothing is written to `plugins/`
   (`plugin-invalid`, exit 31). Beyond the role-based field table this
   includes: every Starter `pattern` must compile as a Go regular expression
   (FR-004a), and the manifest `name` must be a valid alias (FR-004b).
4. Resolve the alias (`--as`, else the manifest's `name`). An `--as` value
   must satisfy the same alias grammar as `name` (FR-004b): a single path
   segment matching `^[A-Za-z0-9][A-Za-z0-9._-]*$` that does not end in
   `.old`; otherwise `plugin-invalid`, exit 31, before `plugins/` is touched.
   If the registry already has a `Package` at that alias whose origin
   identity (absolute local path, or remote source URL ignoring the pinned
   SHA) differs, or the alias already owns registered components without a
   `Package` (the reference package's alias, `work-reference`, which
   bootstrap registers without a `Package`), fail — nothing registered
   (`plugin-alias-conflict`, exit 32). The same origin under the same alias
   is a reinstall: idempotent, and it *replaces* what that alias registered
   (step 7). The user-facing message
   depends on where the alias came from and never shows the existing
   package's path or origin (it stays in the `WORK_DEBUG` message only):

   | Alias came from | Summary | Hint |
   |---|---|---|
   | manifest `name` (no `--as`) | `Plugin "<name>" was not installed: its name collides with the name of a plugin already installed.` | `Install it under another name: work plugin install "<source>" --as <alias>, or remove the existing plugin first: work plugin uninstall <alias>` |
   | `--as <alias>` | `Plugin "<name>" was not installed: the alias "<alias>" you proposed with --as conflicts with a plugin already installed under that alias.` | `Choose a different alias: work plugin install "<source>" --as <alias>` |
5. If the manifest declares a fallback Starter (`role: starter`, empty
   `pattern`) and a *different* alias already has one registered, fail —
   nothing registered (`plugin-fallback-conflict`, exit 33).
6. Stage-then-atomically-swap the content into `plugins/<alias>/` (the same
   primitives `bootstrap.install` uses for the embedded seed — research R1).
7. Register: first remove everything the alias registered before (its
   components, and the conventions its previous `Package` declared unless
   another `Package` also declares them — FR-006a), then one
   `registry.Package{Alias, Origin, Reference}`, one `registry.Component` per
   manifest component (unchanged shape from F1/F3), one `registry.Convention`
   per manifest convention (upserted by `Name`). A reinstall therefore leaves
   the registry equal to the new manifest for that alias.
8. On any I/O failure not covered above (clone failure, staging failure,
   permission error): fail with `plugin-install-failed` (exit 34); nothing
   registered.

### Output (stdout, success)

```text
work: installed <alias> (<origin>)
work: components: <name> (<role>)[, <name> (<role>) ...]
```

One line naming the alias and origin kind, one line listing every registered
component by name and role (omitted if the manifest declares none). Stable
across invocations of the same content; script-usable (FR-034).

### Exit codes

| Code | Token | Condition |
|---|---|---|
| 0 | — | Installed (or idempotently reinstalled) |
| 2 | `usage` | `--link` with a remote `SOURCE`; missing `SOURCE` |
| 31 | `plugin-invalid` | Manifest fails role-based field validation, declares a Starter `pattern` that does not compile, or the manifest `name` / `--as` is not a valid alias |
| 32 | `plugin-alias-conflict` | Alias resolves to a different existing origin, or to the reference package's alias |
| 33 | `plugin-fallback-conflict` | A second enabled fallback Starter would result |
| 34 | `plugin-install-failed` | I/O, clone, or staging failure |

## `work plugin list [--json]`

Read-only; mutates nothing (FR-034). Lists every `registry.Package`, each
with its alias, origin, pinned reference, and the components/conventions it
registered.

### Output (stdout, text)

```text
<alias>  <origin>  <reference>
  <name> (<role>)
  ...
```

One block per package, sorted by alias. A package with no components prints
just its header line.

### Output (`--json`)

```jsonc
[
  {
    "alias": "github-plugin",
    "origin": "remote-pinned",
    "reference": "https://example.com/github-plugin.git@a1b2c3d",
    "components": [
      { "name": "github-pull-request-starter", "role": "starter" }
    ],
    "conventions": []
  }
]
```

Field names and shape are stable; new fields may be appended in a later
slice, never removed or repurposed (F2.5 `--json` contract carried forward
unchanged).

### Exit codes

| Code | Token | Condition |
|---|---|---|
| 0 | — | Always, including zero installed packages (empty array/no blocks) |

## Contract tests (tests/integration/plugin_install.txtar, plugin_list.txtar)

| Case | Command | Expect |
|---|---|---|
| local pinned | `work plugin install <fixture-dir>` | exit 0; `plugins/<alias>/source` is a real copy |
| local linked | `work plugin install <fixture-dir> --link` | exit 0; `plugins/<alias>/source` is a symlink to `<fixture-dir>` |
| link + remote | `work plugin install https://example.com/x.git --link` | exit 2, nothing installed |
| remote pinned | `work plugin install <remote-fixture>` | exit 0; reference is `<source>@<sha>` |
| alias conflict | install fixture A under alias `x`, then fixture B under `--as x` | exit 32, B not registered, A unchanged |
| idempotent reinstall | install the same fixture twice under the same alias | both exit 0, one registry entry |
| fallback conflict | install a fixture fallback Starter, then a second fixture fallback Starter under a different alias | second install exit 33, nothing from it registered |
| invalid manifest | install `tests/fixtures/plugins/invalid-manifest` | exit 31, nothing registered |
| invalid pattern | install a fixture whose Starter `pattern` is `(unclosed` | exit 31, nothing registered |
| invalid alias | `--as ..`, `--as a/b`, `--as x.old`; a fixture named `..` | exit 31, `plugins/` untouched, nothing registered |
| reserved alias | `--as work-reference` (or a fixture named so) | exit 32, the reference package unchanged, nothing registered |
| reinstall drops entries | install a fixture declaring Starters A and B and a convention, then the same origin declaring only A | exit 0; `work plugin list` shows only A; the dropped convention is gone unless another package declares it |
| list empty | `work plugin list` before any install | exit 0, empty output/`[]` |
| list after install | `work plugin list [--json]` | every installed package's alias/origin/reference/components present |
