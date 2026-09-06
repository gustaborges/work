# Contract: `plugin.json` validation used by F1 bootstrap

Authority: ADR-0012, ADR-0006, ADD §4 & §4.1 & §5 & §6; research R3, R11. F1 implements the
**minimum** manifest-validation surface needed to seed and register the reference package.
The public `work plugin install` command is F4 — F1 only runs this pipeline internally against
the embedded seed.

## Package shape

```text
<package-root>/
  plugin.json          # required, at the root
  <entrypoint files>   # referenced by components[].entrypoint, relative to the package root
```

## `plugin.json` — top level

| Field | Type | Required | F1 rule |
|---|---|---|---|
| `name` | string | yes | proposed local alias; F1 seed uses `work-reference` |
| `version` | string | yes | opaque version token (ADR-0005); not interpreted in F1 |
| `components` | array | yes (may be empty) | validated per role below |
| `conventions` | array | yes (may be empty) | each: `{ "name": string, "prefixes": [string,...] }` only |

Unknown top-level fields → invalid.

## `components[]` — common rules

- `name` (string, required) — logical component identity (never the file).
- `role` (string, required) — one of `starter`, `repository-locator`, `importer`, `linker`.
  F1 seed only uses the first two; the validator implements all four role rule-sets (ADD §4.1)
  so F4 needs no change.
- `entrypoint` (string, required) — path relative to the package root; must exist after
  extraction.
- `runtime` (string, optional) — declared interpreter. **Absent → executed directly** (ADR-0006).
  If present, its presence on `PATH` is checked in preflight; **the F1 seed omits it**.
- The core never relies on shebang or the OS exec bit to choose the interpreter (ADR-0006).

## Role rule-sets (ADD §4.1)

| role | required | allowed (beyond required) | forbidden |
|---|---|---|---|
| `starter` | `name`, `role`, `entrypoint` | `pattern`, `runtime` | `on`, `manual`, `inputs`, `key`, `discover` |
| `repository-locator` | `name`, `role`, `entrypoint`, non-empty `accepts` | `runtime`, `display_name`, `description` | `pattern`, `on`, `manual`, `inputs`, `key`, `discover` |
| `importer` | `name`, `role`, `entrypoint`, at least one of `on` / `manual` | `on`, `manual`, `inputs`, `runtime` | `pattern`, `key`, `discover` |
| `linker` | `name`, `role`, `key`, `entrypoint`, at least one of `discover` / `manual` | `discover`, `manual`, `runtime` | `pattern` |

Additional constraints:
- `repository-locator.accepts` ⊆ `{ "git_fetch_urls", "name", "query" }`, non-empty, no `path`.
- `starter.pattern`, when present and non-empty, marks a `specific` starter; absent/empty →
  `fallback` layer (ADR-0004). F1 seed Starter is `fallback` (no `pattern`).
- No `invocation`, `type`, `capabilities`, `hooks`, `priority`, `score`, or recommended-position
  field anywhere — their presence is invalid (ADD §4.1).
- `inputs[]` (importer/linker) grammar `"<work|meta|link>:<key>[:optional]"` — validated
  syntactically (not exercised by F1 seed).
- `conventions[]` entries have **only** `name` and `prefixes[]` — no `role`/`entrypoint`/`runtime`.

Any violation → the seed bootstrap fails with `diag` `bootstrap-failed` (exit 16) and no
partial registry/plugin-dir state (atomic plugin-dir rename, R11).

## Generated registry entry (per component) — `~/.work/state/registry.json`

Built from a valid manifest (ADR-0002: registry is generated state, never hand-edited):

```jsonc
{
  "alias": "work-reference",
  "name": "local-path-starter",
  "role": "starter",
  "entrypoint": "starter",
  "starter_layer": "fallback"
  // runtime omitted; pattern omitted
}
```

```jsonc
{
  "alias": "work-reference",
  "name": "filesystem-repository-locator",
  "role": "repository-locator",
  "entrypoint": "locator",
  "accepts": ["name", "git_fetch_urls", "query"],
  "display_name": "Filesystem repositories",
  "description": "Finds local Git repositories in configured search roots"
}
```

Conventions are registered separately: `{ "name": "freeform", "prefixes": ["{slug}"] }`.

Idempotency key = `(alias, name)` for components, `name` for conventions (R11). Re-bootstrap
upserts; never duplicates. Alias collision with a *different* origin → fail (ADR-0002) — not
reachable in F1 (only the seed is installed), but the check exists.

## The F1 seed `plugin.json` (reference)

```jsonc
{
  "name": "work-reference",
  "version": "1.0.0",
  "components": [
    { "name": "local-path-starter", "role": "starter", "entrypoint": "starter" },
    {
      "name": "filesystem-repository-locator",
      "role": "repository-locator",
      "display_name": "Filesystem repositories",
      "description": "Finds local Git repositories in configured search roots",
      "accepts": ["name", "git_fetch_urls", "query"],
      "entrypoint": "locator"
    }
  ],
  "conventions": [
    { "name": "freeform", "prefixes": ["{slug}"] }
  ]
}
```
