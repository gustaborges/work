# Contract: Repository Reference (F3)

Authority: ADR-0016, ADR-0014, ADD §4.1 & §7.1; spec FR-001, FR-002, FR-005,
FR-006, FR-007, FR-008.

The transient object a Starter produces and a Repository Locator consumes. It is
**never persisted** and never promoted into `work`, `meta`, or `links`
(FR-005).

## Shape

Wire type `ipc.RepositoryReference` (already defined); core-side subset
`starter.Reference` (F3 adds `GitFetchURLs`, `Name`, `Query`).

```jsonc
{
  "repository": {
    "path":           "/abs/path",                       // optional
    "git_fetch_urls": ["https://…/p.git", "git@…:p.git"], // optional
    "name":           "p",                                // optional
    "query":          "pay"                               // optional
  }
}
```

All four fields are **independent and optional** (ADR-0016). A valid reference has
at least one non-empty field.

## Field semantics

| Field | Meaning | Core handling |
|---|---|---|
| `path` | an already-resolved local path | validated **directly** by `reporef.ValidatePath`; **no Locator runs**; the other fields are **not** a fallback for an invalid path (FR-002, ADR-0014) |
| `git_fetch_urls` | known fetch endpoints | **not** canonical identity — no SSH/HTTPS equivalence, no `.git`-suffix or user/host normalisation, no push URLs (ADR-0016); a Locator that consumes them compares across **all** local remotes, not just `origin` |
| `name` | a known logical name, possibly ambiguous | matched by Locators that `accept` `name` |
| `query` | opaque search text | matched by Locators that `accept` `query`; **never** auto-promoted to `name` and vice versa |

## Who sets the fields

- The **seed `local-path-starter`** emits `path` when the argument looks like a
  filesystem path, otherwise `name` (classification in
  `cli-work-start.md` §Argument classification). It never emits `git_fetch_urls`
  or `query` in F3.
- Richer Starters (F4+) may emit any combination.

## Eligibility (which Locators may run)

A policy Locator participates in resolution iff **both**:

1. it is **available** — a registered component with that `<alias>/<name>` and
   `role == "repository-locator"` (`resolution-policy.md` §Availability); and
2. `accepts ∩ { non-empty reference fields } ≠ ∅`, where
   `accepts ⊆ { git_fetch_urls, name, query }`.

Eligibility is computed from **static** data only — no subprocess is started to
determine it (FR-006, FR-044).

## Projection to an eligible Locator

`ipc.LocatorInput` MUST contain **only**:

- `repository`: the accepted-and-present fields — an accepted field that is empty
  on the reference is omitted; a present field the Locator does not `accept` is
  omitted;
- `repository_roots`: the configured search roots, verbatim.

It MUST NOT contain the raw `work start` argument, `base_branch`, `start_modes`,
`meta`, `links`, or any other core state (FR-007, FR-008, SC-004). A contract test
pins the serialised payload for every reference shape.

## Tests (`tests/contract/`, `internal/locator`)

| Case | Expect |
|---|---|
| `{path}` only | `reporef.ValidatePath` called; zero Locator invocations |
| `{path, name}`, path invalid | fails with `invalid-path`/`unusable-repo`; `name` not tried |
| `{name}`, Locator accepts `name` | eligible; `LocatorInput.repository == {"name": …}` only |
| `{git_fetch_urls}`, Locator accepts `name` only | ineligible; not executed |
| `{}` (no field) | `no-eligible-locator` (27) |
| `{query}`, no policy Locator accepts `query` | `no-eligible-locator` (27) |
| any reference | `LocatorInput` never carries arg/base/modes/meta/links |
