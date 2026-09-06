# Contract: Seed Repository Locator subprocess — `filesystem-repository-locator` (F1)

Authority: ADR-0000, ADR-0006, ADR-0012 (role `repository-locator`), ADR-0014, ADR-0015,
ADR-0016, ADD §7.1 & §7.2 & §11; research R3, R17.

> **F1 scope note.** On the F1 happy path this Locator is **never executed**: the seed Starter
> always returns `repository.path`, so the core validates the path directly and skips all
> Locators (ADD §7.1). It is still built, embedded, extracted, registered, and added to the
> Repository Resolution Policy at bootstrap (ADD §6), and its contract is tested in isolation
> now so F3 activates it with no new bootstrap work. Nothing in the F1 user journey depends on
> its runtime behavior.

## Process model

Identical to the Starter (`ipc-starter.md`): subprocess, no `runtime` (executed directly),
one JSON in / at most one JSON out / exit code, stderr for diagnostics, no streaming.

## Manifest entry (in seed `plugin.json`)

```jsonc
{
  "name": "filesystem-repository-locator",
  "role": "repository-locator",
  "display_name": "Filesystem repositories",
  "description": "Finds local Git repositories in configured search roots",
  "accepts": ["name", "git_fetch_urls", "query"],
  "entrypoint": "locator"
}
```

- `accepts` MUST be non-empty and MUST contain only `name`, `git_fetch_urls`, `query` — never
  `path` (ADD §4.1, ADR-0016). `path` present in a reference makes the core validate directly
  and not run any Locator.
- Fields `pattern`, `on`, `manual`, `inputs`, `key`, `discover` are invalid for this role and
  MUST be absent.
- Registered in `config/work.json` → `repository_resolution.locators` as
  `"work-reference/filesystem-repository-locator"` at bootstrap.

## Input (stdin)

```jsonc
{
  "repository": {
    // only the fields the core has AND this Locator `accepts`, when present:
    "git_fetch_urls": ["https://github.com/example/project.git"],
    "name": "project",
    "query": "proj"
  },
  "repository_roots": ["/home/user/src", "/home/user/projects"]
}
```

- The core projects **only** accepted-and-present reference fields plus the configured search
  roots (ADD §7.2). No other core state.
- `repository_roots` may be empty (F1 default) — then the Locator returns `{"matches":[]}`.

## Output (stdout, on success — exit 0)

```json
{ "matches": [ { "repo_path": "/home/user/src/example/project" }, { "repo_path": "..." } ] }
```

Rules (ADD §7.1):
- `matches` is an array (possibly empty) of `{ "repo_path": "<absolute path>" }` objects only.
- **No** `confidence`, `score`, `priority`, `winner`, or ranking fields.
- The Locator returns **all** filesystem matches it finds; it does not pick a winner — the core
  validates, deduplicates, and (if >1) asks the user (ADR-0015).
- Search is bounded by the Locator's own depth limit; it MUST NOT treat `workspace/in-progress`
  or `workspace/archived` as a repo catalog (it only sees `repository_roots`, ADD §7.2).
- When it consumes `git_fetch_urls`, it compares fetch URLs across **all** local remotes (not
  just `origin`) using native git, with no protocol/user/`.git`-suffix normalization
  (ADR-0016).

## Outcomes as seen by the core (ADR-0015, F3 — informational here)

| Locator result | Core behavior |
|---|---|
| `{"matches":[]}` | continue to next eligible Locator in the policy |
| exactly one valid, deduped match | resolution done |
| multiple valid matches | present to user, chain stops |
| all matches invalid | resolution fails with diagnostic |
| non-zero exit / invalid stdout | resolution fails (operational failure ≠ no result) |

## Failure (exit non-zero)

- Unreadable root, internal error → exit non-zero, stderr message, no stdout.
- Garbage stdin → exit non-zero, no stdout.

## Contract tests (tests/contract/)

| Case | stdin | expect |
|---|---|---|
| name match, one repo | roots with one `project/.git` + `{"repository":{"name":"project"},"repository_roots":[R]}` | exit 0, one `repo_path` (absolute) |
| name match, two repos | two `project` clones under R | exit 0, two `repo_path` entries, no ranking keys |
| no match | `{"repository":{"name":"nope"},"repository_roots":[R]}` | exit 0, `{"matches":[]}` |
| empty roots | `{"repository":{"name":"project"},"repository_roots":[]}` | exit 0, `{"matches":[]}` |
| fetch-url match on non-origin remote | clone with `upstream` = target URL | exit 0, match found |
| garbage stdin | `xxx` | exit ≠ 0, no stdout |
| unreadable root | `{"repository_roots":["/root/denied"], ...}` | exit ≠ 0 |
