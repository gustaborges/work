# Contract: Importer and Linker declarations in `plugin.json` (F5)

Authority: ADD §4, §4.1, §4.2, §5; ADR-0012; spec FR-001..FR-007; `research.md` R1–R3, R5.
**Extends** `specs/001-first-local-work/contracts/plugin-manifest.md`: every rule there still
holds (top-level fields, role field table, `conventions[]`, alias grammar of F4). This contract
defines the *shapes* of the fields that document only listed by name, and **replaces** the
F1 placeholder shapes (`on: [string]`, `manual: true`, `discover: [string]`), which were never
documented and never executed.

## Shapes

```jsonc
// role: importer            (required: name, role, entrypoint, and at least one of on / manual)
{
  "name": "github-pull-request-importer",
  "role": "importer",
  "entrypoint": "importer.py",
  "runtime": "python3",                                   // optional
  "on": [                                                 // optional; non-empty when present
    { "event": "start:finalized",
      "starters": ["github-pull-request-starter"] }       // optional; non-empty when present
  ],
  "manual": {                                             // optional
    "display_name": "Pull Request Context",               // required, non-empty
    "description": "Imports the artifacts associated with the pull request"   // optional
  },
  "inputs": ["link:github.pull_request", "work:start_mode:optional"]          // optional
}

// role: linker              (required: name, role, key, entrypoint, and at least one of discover / manual)
{
  "name": "github-pull-request-linker",
  "role": "linker",
  "key": "github.pull_request",
  "entrypoint": "linker.py",
  "runtime": "python3",
  "discover": {                                           // optional
    "automatic": true,                                    // bool, default false
    "on": [ { "event": "start:finalized", "starters": ["github-pull-request-starter"] } ],
    "inputs": ["work:worktree_path"]
  },
  "manual": { "display_name": "GitHub Pull Request", "description": "Links the Work to a GitHub pull request" }
}
```

A linker's `inputs` are only valid inside `discover`; a top-level `inputs` on a linker
remains forbidden (ADD §4.1). `starter`, `repository-locator` and `conventions[]` are unchanged.
Unknown fields inside `on[]`, `manual` or `discover` are invalid.

## Rules (every violation → `plugin-invalid`, exit 31, nothing registered)

| # | Rule |
|---|---|
| M1 | `event` must be a core event. **v1 core events: `start:finalized`.** Any other value is invalid, naming it. |
| M2 | Each `starters` entry is `<name>` (matches any Starter with that logical name) or `<alias>/<name>` (exactly that component). Entries must be non-empty; a Starter that is not installed is **not** an error. |
| M3 | Every input is `<work\|meta\|link>:<key>[:optional]`. `work` keys are limited to the exposed facts: `worktree_path`, `start_mode`, `branch`, `base_branch`, `slug`. `meta`/`link` keys must satisfy the key grammar (`semantic-conventions.md`). |
| M4 | A component may not name the same key twice among its inputs, in any namespace (the delivered document is keyed by the bare key). |
| M5 | A linker's `key` must satisfy the key grammar. A private key (`plugin.<name>.…`) must use the manifest's own `name`, and that `name` must contain no `.`. |
| M6 | `manual.display_name` is non-empty. |
| M7 | `automatic: true` without any `on` is **accepted** and never runs automatically (a later slice may give it meaning). |

Several linkers — in one package or in different packages — may declare the same `key`.

## Runtime (FR-007)

`runtime`, when present, is checked with `exec.LookPath` during `work plugin install`,
**after** the manifest validates and **before** anything is staged or registered. A miss fails
`plugin-install-failed` (34):

```text
error: plugin-install-failed: the runtime "python3" declared by component "…" was not found on PATH
```

hint: `Install "python3" or fix PATH, then install again.` The check is repeated when the
component is started; there, a miss is an `extension-start-failed` warning (extensions) or the
existing unusable-repository failure (Starters/Locators). Without `runtime`, the entrypoint
is executed directly; with it, Work runs `<runtime> <entrypoint>` and never relies on a
shebang or an executable bit (ADR-0006). On Windows `.exe` is appended only when no runtime is
declared.

## Registry entry (generated, additive, unversioned)

For the two components above, `registry.json` `components[]` holds, beyond the F4 fields:

```jsonc
{ "alias": "github-plugin", "name": "github-pull-request-importer", "role": "importer",
  "entrypoint": "importer.py", "runtime": "python3",
  "on": [ { "event": "start:finalized", "starters": ["github-pull-request-starter"] } ],
  "manual": { "display_name": "Pull Request Context", "description": "…" },
  "inputs": ["link:github.pull_request", "work:start_mode:optional"] }

{ "alias": "github-plugin", "name": "github-pull-request-linker", "role": "linker",
  "entrypoint": "linker.py", "runtime": "python3", "key": "github.pull_request",
  "discover": { "automatic": true,
                "on": [ { "event": "start:finalized", "starters": ["github-pull-request-starter"] } ],
                "inputs": ["work:worktree_path"] },
  "manual": { "display_name": "GitHub Pull Request", "description": "…" },
  "inputs": ["work:worktree_path"] }
```

and the package record gains `"plugin_name": "github-plugin"`. Reinstalling the same origin
under the same alias replaces every one of these (FR-005). A component registered before F5
has none of them and is never eligible until reinstalled (D10).

## Not part of this contract

`work plugin list` output is unchanged (origin and pinned reference only). Manual availability
(`manual`) is validated and recorded here but not executed until F6.
