# Phase 1 Data Model: Find the Local Clone (F3)

**Feature**: `specs/004-local-clone-locator/` · **Plan**: [plan.md](./plan.md) ·
**Research**: [research.md](./research.md) · **Date**: 2026-09-07

F3 changes **no persisted schema**. `work-state.json` stays schema 2, `work.db`
stays `user_version = 2`, and `config/work.json` gains **no new key** — F1 already
declared `repository_roots` and `repository_resolution.locators`. What changes is
that the `work repository` commands and the `work start` resolution path now
*read and write* those two fields past bootstrap.

Every entity below is either **transient** (lives for one resolution, one process)
or an existing **config field** F3 starts exercising. Entities map to the spec's
*Key Entities*.

Authority note: `internal/locator` types carry only what the resolution pipeline
needs. The meaning of a Work, the snapshot, and provenance stay untouched
(research R16).

---

## 1. Repository Reference (transient — one `work start` invocation)

The object a Starter produces; the input to resolution. Independent optional
fields (ADR-0016). Never persisted (FR-005).

| Field | Type | Set by | Notes |
|---|---|---|---|
| `path` | string | Starter | already-resolved local path; when present, the core validates it directly and **runs no Locator** (ADR-0014) |
| `git_fetch_urls` | []string | Starter | known fetch endpoints; **not** canonical identity; compared across all local remotes with no normalisation |
| `name` | string | Starter | a known, possibly ambiguous logical name |
| `query` | string | Starter | opaque text for local mechanisms; never auto-promoted to `name` |

Realised as `starter.Reference` (core-side subset) built from
`ipc.RepositoryReference` (wire type — already has all four fields).

**Validity for resolution**: at least one of `path`, `git_fetch_urls`, `name`,
`query` must be non-empty. `path` only ⇒ direct validation. No `path` and no field
any policy Locator `accepts` ⇒ `no-eligible-locator` (27).

**Eligibility rule** (per Locator): `accepts ∩ {non-empty reference fields} ≠ ∅`,
where `accepts ⊆ {git_fetch_urls, name, query}` (`path` is never in `accepts`).

**Projection to a Locator** (`ipc.LocatorInput`): only the accepted-and-present
reference fields, plus `repository_roots`. Nothing else (FR-007, FR-008, SC-004).

---

## 2. Locator Candidate (transient — one resolution)

One local repository path a Locator returned, before/after core validation.

| Field | Type | Notes |
|---|---|---|
| `repo_path` (raw) | string | exactly what the Locator emitted in `matches[].repo_path` |
| `resolved` | string | `reporef.ValidatePath` result: absolute, `EvalSymlinks`-resolved |
| `valid` | bool | false ⇒ dropped; if every candidate from the ending Locator is invalid ⇒ `repository-candidate-invalid` (28) |
| `remote` | string | first remote fetch URL, for the picker's secondary line (research R15); computed only in the ambiguous case |

**Dedup key**: `resolved`. Two candidates with the same `resolved` collapse to one
(SC-009).

---

## 3. Resolution Outcome (transient — the result of `locator.Resolve`)

```go
type Outcome struct {
    Resolved   string      // one repo: the validated absolute path
    Candidates []Candidate // >= 2 validated, deduped candidates (ambiguous)
}
```

| Shape | `work start` behaviour |
|---|---|
| `Resolved != ""` | continue the identical F1 creation journey with this path |
| `len(Candidates) >= 2` | interactive: insert the `repository` `present.Select` step; non-interactive / argv: `repository-ambiguous` (30) |
| error `no-repository-found` (26) | interactive source step: recoverable in-frame; else fail |
| error `no-eligible-locator` (27) | fail (configuration) |
| error `repository-candidate-invalid` (28) | fail |
| error `locator-failed` (29) | fail (operational; halts the chain, no fallback) |

All error shapes are `*diag.Error` with `Summary` + `Hint` for the F2.5
interactive renderer.

---

## 4. Repository Resolution Policy (config — `repository_resolution.locators`)

Ordered `[]string` of qualified Locator references `"<alias>/<component>"`. Already
in `config.Config`; F1 bootstrap seeds it with
`"work-reference/filesystem-repository-locator"`.

### Policy Entry (derived view for `policy list`)

| Field | Source | Notes |
|---|---|---|
| `ref` | the string | `"<alias>/<component>"` |
| `position` | index + 1 | 1-based, the traversal order |
| `available` | registry lookup | `true` iff a registered component with that alias+name has `role == "repository-locator"` (research R12) |

### Operations (`internal/repoconfig/policy.go`)

| Operation | Rule |
|---|---|
| `list` | every entry in order, each with `available`; unavailable entries are **shown, marked**, never dropped (FR-025) |
| `add <ref> [--before x \| --after x]` | `ref` must resolve to a registered repository-locator (else `usage` 2); appended, or positioned relative to `x`; adding an already-present `ref` is a no-op success |
| `remove <ref...>` | drop each; removing an absent `ref` is a no-op success; the component stays installed and enabled (ADR-0015) |
| `move <ref> (--before x \| --after x)` | `ref` must be present; exactly one of `--before`/`--after` (else `usage` 2) |
| `replace <ref...>` | validate the whole new list, then atomic write |

**Invariant**: only `bootstrap` (seed, once) and these operations ever write the
list. Plugin install/enable never does (FR-027).

---

## 5. Repository Search Root (config — `repository_roots`)

`[]string` of plain absolute directory paths. Already in `config.Config`; F1
defaults it to `[]`. Distinct from `config.Workspace` (FR-020).

### Operations (`internal/repoconfig/roots.go`)

| Operation | Rule |
|---|---|
| `list` | the stored order; `{"roots":[]}` when none (never an error) |
| `add <path...>` | expand `~`, `filepath.Abs`; must be an existing readable directory (else `usage` 2); must **not** overlap `config.Workspace` in either direction — root == / inside / containing the workspace ⇒ `usage` 2 with both paths named (research R13); canonical dedup |
| `remove <path...>` | canonical match; absent ⇒ no-op success |
| `replace <path...>` | validate all (incl. workspace-overlap), then atomic write |

Also exported for the CLI first-run steps (no `present` import in `repoconfig`):

| Symbol | Purpose |
|---|---|
| `NeedsSetup(cfg) (wantWorkspace, wantRoot bool)` | true while `Workspace` is unset / `RepositoryRoots` is empty — drives the interactive `work start` first-run prompts (research R21) |
| `ValidateRoot(cfg, path) (abs string, err error)` | the `add` rule above, reused by the first-run search-root prompt |

**Projection**: the stored roots are passed to every executed Locator verbatim in
`repository_roots`. The core never stats them (research R11); the Locator does.

### First-run setup (interactive `work start` only)

Not a data-model entity — a wizard behaviour. On an interactive `work start`,
before any journey step: if `Workspace` is unset, prompt for it (the F1 FR-006
step, moved to the front); if `RepositoryRoots` is empty, prompt for one search
root (`ValidateRoot`). Persist via `config.Save` before resolution. First-run
only; never shown once both are set. Non-interactive mode runs no setup
(`contracts/cli-work-start.md`).

---

## 6. Registered Locator (read-only view for `locator list`)

Projection of `registry.Component` where `role == "repository-locator"`.

| Field | Source |
|---|---|
| `ref` | `alias + "/" + name` |
| `display_name`, `description` | manifest, via registry |
| `accepts` | manifest, via registry (`⊆ {name, git_fetch_urls, query}`) |
| `in_policy` | membership in `repository_resolution.locators` |

F3 reads this; it is written only by `bootstrap` / plugin install (F4+).

---

## 7. Diagnostic Categories (constants — `internal/diag`)

Appended to `diag.All` after F2's `snapshot-unreadable` (25):

| Const | Token | Code |
|---|---|---|
| `NoRepositoryFound` | `no-repository-found` | 26 |
| `NoEligibleLocator` | `no-eligible-locator` | 27 |
| `RepositoryCandidateInvalid` | `repository-candidate-invalid` | 28 |
| `LocatorFailed` | `locator-failed` | 29 |
| `RepositoryAmbiguous` | `repository-ambiguous` | 30 |

The `diag.All` table test is extended; codes 0/2/10–25 and every existing token
are unchanged (FR-036).

---

## 8. What is explicitly unchanged

- `work-state.json` — schema 2, same sections, `work.starter = "local-path-starter"`
  for a located clone exactly as for a typed path (SC-008, R16).
- `work.db` — `user_version = 2`, same `works` projection, rebuildable from
  snapshots.
- `config/work.json` — same keys and `workspace` semantics. `workspace.Validate`
  gains one clause: the workspace root may not be equal to, inside, or containing
  a configured repository search root (the mirror of the `root add` rule — R13).
  Its existing "not rejected merely because a Git repo encloses it" behaviour is
  unchanged.
- `registry.json`, `plugin.json` (seed) — the `filesystem-repository-locator`
  entry is already correct; no manifest edit.
- The Repository Reference is never written to any of the above.
