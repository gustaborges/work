# Contract: Starter subprocess protocol — F4 extension

Authority: ADR-0000, ADR-0004, ADR-0006, ADR-0012, ADR-0016, ADD §7 & §11;
research R7–R9. **Extends** `specs/001-first-local-work/contracts/
ipc-starter.md`, which remains authoritative for the fallback Starter's
behaviour (path-only response, `start_modes` always absent) — every rule
there still holds. This contract documents what changes when more than one
Starter is registered and what a *specific* Starter may additionally return.

## Manifest: `pattern` distinguishes specific from fallback

Unchanged field, now load-bearing for the first time outside F1's seed
(which always omits it):

```jsonc
{ "name": "github-pull-request-starter", "role": "starter",
  "pattern": "^https://github\\.com/[^/]+/[^/]+/pull/\\d+$",
  "entrypoint": "starter.py", "runtime": "python3" }
```

- Non-empty `pattern` → **specific** layer: evaluated locally (no subprocess)
  against `work start`'s argument, as a Go-syntax regular expression
  (`regexp.Compile`), before any Starter is invoked. A `pattern` that does
  not compile is rejected at install time (`plugin-invalid`, FR-004a), so
  every registered `pattern` compiles; `Match` keeps a defensive skip only
  for a hand-edited registry.
- Empty/absent `pattern` → **fallback** layer: never pattern-matched; invoked
  only when no specific Starter matches. At most one may be registered
  (`plugin-fallback-conflict` at install time — `cli-work-plugin.md`).

## Selection (before any subprocess runs)

```text
0 specific matches  -> invoke the registered fallback (fail starter-not-matched, 35, if none)
1 specific match    -> invoke it directly, no selection step
>=2 specific matches -> Ambiguous outcome: present.Select interactively,
                        or fail starter-ambiguous (36) non-interactively — never invoke any of them first
```

No score, priority, specificity comparison, or installation-order tiebreak
(ADR-0004). The collision question is never memoized — the identical
argument asked again on a later invocation gets asked again (FR-012, SC-006).

## Input (stdin) — unchanged

```json
{ "arg": "<the SOURCE string the user gave to `work start`>" }
```

Still exactly `{"arg": ...}` — no start mode, no base branch, no metadata,
no links, no indication of which other Starters exist or collided (FR-014,
unchanged trust boundary).

## Output (stdout, on success) — extended fields

```jsonc
{
  "repository": { "git_fetch_urls": ["https://github.com/example/project.git"], "name": "project" },
  "base_branch": "feature/source-branch",
  "start_modes": ["contribution", "fork"]
}
```

- `repository.*`: unchanged (ADR-0016) — `path` short-circuits to direct
  validation; otherwise the reference resolves through the identical F3
  Locator chain (no Starter-specific resolution path, FR-017/FR-018).
- `base_branch` (string, optional): when present, the core uses it directly
  and does not prompt (FR-024). In contribution mode this is **the branch to
  check out**, not a base to branch from (see below). When absent, the core
  prompts among local/remote branches exactly as it does with no plugin
  involved (FR-023) — this applies in fork/new modes only; contribution mode
  with no `base_branch` is a structurally invalid response (below), never an
  implicit prompt.
- `start_modes` (array of strings, optional): when present, MUST contain only
  values from `{"contribution", "fork"}`. Absence means new-Work creation
  (`work.start_mode = "new"`). Any other value present is a structurally
  invalid response.
- `meta`, `links`: accepted on the wire (unchanged shape from ADD §7) but
  **not consumed** by F4 — no code path reads them yet. Persisting them is
  F5/F6 (`start:finalized`) scope; a Starter author may include them now for
  forward-compatibility, but nothing in F4 acts on them.

## Failure — extended trigger

In addition to the F1 rules (non-zero exit, unparseable stdout ⇒
`materialization-failed`/`unusable-repo`), a **structurally valid** JSON
response is still rejected, before any Work materialization, when:

- `start_modes` contains a value outside `{"contribution", "fork"}`, or
- `start_modes` contains `"contribution"` and `base_branch` is absent (there
  is no implicit prompt for the branch to check out in contribution mode —
  it must come from the Starter).

Both are `starter-response-invalid` (exit 37) — the same failure shape as a
malformed response, per the spec's own edge cases ("treated as a structurally
invalid response, failing the same way as a malformed Starter output").

## Persisted effect (unchanged mapping, now exercised for all three values)

| Starter response | `work.start_mode` | Journey |
|---|---|---|
| `start_modes` absent | `"new"` | full new-Work journey (slug, convention, prefix, new branch) |
| `start_modes` present, user selects `"fork"` | `"fork"` | identical new-Work journey, using the Starter's `repository`/`base_branch` |
| `start_modes` present, user selects `"contribution"` | `"contribution"` | checkout of `base_branch` directly; no slug/convention/prefix; branch need not be new |

## Contract tests (tests/contract/starter_match_test.go, tests/fixtures/plugins/)

| Case | Fixture(s) | Expect |
|---|---|---|
| specific match | `specific-starter` alone, matching arg | invoked directly, no selection |
| fallback used | `specific-starter` installed, non-matching arg, reference fallback present | reference fallback invoked |
| no match, no fallback | only `specific-starter` (non-matching arg), no fallback registered | `starter-not-matched` (35) |
| collision | `specific-starter` + `colliding-starter`, both matching arg | `Ambiguous` outcome naming both; neither invoked yet |
| collision not memoized | same collision, run twice, different choice each time | both runs ask again |
| unrecognized start_mode | fixture emits `"start_modes":["rebase"]` | `starter-response-invalid` (37) |
| contribution w/o base_branch | fixture emits `"start_modes":["contribution"]`, no `base_branch` | `starter-response-invalid` (37) |
| meta/links ignored | fixture emits `meta`/`links` | Work created; `work-state.json` `meta`/`links` remain `{}` |
