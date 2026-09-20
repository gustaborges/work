# Phase 1 Data Model: New Origins via Plugin (F4)

Authorities: `spec.md` Key Entities; `research.md` R5, R7, R10, R13, R14;
ADR-0002, ADR-0004, ADR-0011, ADR-0012. Every entity here is either new in F4
or an existing entity gaining fields — none is persisted-schema-incompatible
with what F1–F3 already wrote to disk (`work-state.json` schema 3 is an
additive superset of schema 2; `registry.json`'s `packages` array is additive
and unversioned; `work.db` is unchanged).

## 1. Plugin Package (manifest-level, transient at install time)

Unchanged from ADR-0012/`specs/001-first-local-work/contracts/
plugin-manifest.md`: `plugin.json` with `name`, `version`, `components[]`
(role-discriminated), `conventions[]`. `internal/plugin.Manifest`/`Component`/
`Convention` are reused verbatim — F4 adds no field, no role, no validation
rule. What changes is *who calls* `plugin.Parse`: previously only
`bootstrap.install` against the embedded seed; now also `internal/
plugininstall` against an arbitrary local or remote source.

## 2. Registry Package (new persisted entity — `registry.json`)

The generated, per-installed-package record `work plugin list` reads (FR-007).
Lives alongside the existing `Components[]`/`Conventions[]` in
`registry.Registry`.

```go
type Package struct {
    Alias     string // local unique identity; default = manifest name
    Origin    string // "local-linked" | "local-pinned" | "remote-pinned"
    Reference string // absolute source path (local kinds) or "<source>@<sha>" (remote)
    Conventions []string // convention names this package declared, so a reinstall can retract them
}
```

- Identity: `Alias`, unique within the registry (FR-006), and matching the
  alias grammar of FR-004b (it names a directory under `plugins/`). The
  reference package's alias is reserved though it has no `Package` record
  (bootstrap registers its components only).
- `Origin` is exactly one of the three ADR-0002 install kinds; it is what
  `work plugin list` prints as "linked"/"pinned" (US1 AC1/AC2).
- `Reference` is what changed at install time and what a future update slice
  (F7) would compare against a newer resolution — for F4 it is write-once per
  install/reinstall, never mutated by any other command.
- Relationship: every `registry.Component`/`registry.Convention` registered
  from this package carries the same `Alias` — the existing `(Alias, Name)`
  component identity and `Name` convention identity are unchanged; `Package`
  is a new *parent* record, not a new identity scheme.
- Lifecycle: created by `work plugin install`; read by `work plugin list`;
  replaced wholesale by a reinstall of the same origin under the same alias —
  the alias's components and its previously declared conventions are removed
  before the new manifest's are registered (FR-006a; a convention another
  package also declares stays). Never deleted in F4
  (enable/disable/update/uninstall are F7).

## 3. Starter (Component) — matching outcome (transient, per `work start` invocation)

Not persisted. The result of `internal/starter.Match`:

```go
type Outcome struct {
    Matched    registry.Component   // set iff exactly one match (or the fallback)
    Ambiguous  []registry.Component // set iff >= 2 pattern matches; Matched is zero
}
```

- A component is "specific" iff `Pattern != ""`; "fallback" iff `Pattern ==
  ""` (existing `Component.IsFallbackStarter`/`StarterLayer` fields from F3,
  unchanged).
- Exactly one of `Matched`/`Ambiguous` is populated on success; `Match`
  returns an error (`starter-not-matched`, 35) when neither applies.
- The choice among `Ambiguous` candidates is never persisted anywhere —
  no field, no file, no table records it (FR-012, SC-006). This is a
  deliberate absence: there is no entity to describe here beyond "asked and
  discarded".

## 4. Repository Reference (unchanged from ADR-0016/F3)

`starter.Reference` (soon to be renamed conceptually to "Starter response" in
code comments, not in wire shape) gains two fields read from the same
`ipc.StarterResponse` that already carries them:

```go
type Response struct {
    Path, GitFetchURLs, Name, Query string // unchanged Repository Reference fields (ADR-0016)
    BaseBranch string                       // NEW — consumed for FR-023/FR-024
    StartModes []string                     // NEW — consumed for FR-019/FR-021/FR-022
    // Meta, Links: present on the wire, deliberately not exposed here (R8) — F5/F6 scope.
}
```

- Still transient, still never persisted verbatim (only its *effects* —
  `work.branch`, `work.base_branch`, `work.start_mode` — are).
- Resolution of the repository-shaped fields (`Path`/`GitFetchURLs`/`Name`/
  `Query`) into a validated local path is completely unchanged: it still goes
  through `reporef.ValidatePath` (path present) or `internal/locator.Resolve`
  (otherwise), with no branch in that pipeline keyed on which Starter produced
  the reference (FR-017, FR-018).

## 5. Start Mode (new enum, persisted in `work.start_mode`)

```text
"new"          — start_modes absent from the Starter response (unchanged F1/F3 meaning)
"fork"         — start_modes present and the user selected "fork"
"contribution" — start_modes present and the user selected "contribution"
```

- Exactly the Starter's returned value (for `fork`) or the fixed default
  (`new`) — never inferred, never overridden by the core (FR-022).
- Determines the `internal/create.Run` materialization path (R12): `new`/
  `fork` create a new branch (`WorktreeAdd` + branch-owning compensator);
  `contribution` checks out an existing one (`WorktreeAddExisting` + a
  compensator that removes only the worktree).
- Determines whether the slug/convention/prefix steps run at all in
  `start.go` (skipped entirely for `contribution`, FR-020).

## 6. Work (existing entity, `internal/work.WorkSection` — schema 3)

Field-level changes only; identity (`ID`) and every other field are unchanged
from schema 2.

| Field | Schema 2 rule | Schema 3 rule |
|---|---|---|
| `start_mode` | `enum: ["new"]` | `enum: ["new","contribution","fork"]` |
| `slug` | always required, non-empty | required + non-empty **unless** `start_mode == "contribution"`; MUST be absent exactly then (the slug step never runs, FR-020) |
| `branch_convention` | always required, non-empty | required + non-empty **unless** `start_mode == "contribution"`; MUST be absent exactly then |
| `base_branch` | always required, non-empty | unchanged: always required. In contribution mode it equals `branch` (the Starter-resolved branch checked out directly — there is no separate base). |
| `branch` | the newly created branch name | in contribution mode, the Starter-resolved existing branch (not newly created) |

`work.db`'s `works` row (`projection.Work`) needs no struct or column change:
`BranchConvention string` already round-trips an empty Go string to SQL
`NULL` via the existing `branch_convention TEXT` (no `NOT NULL`) column and
the existing `Upsert`/`scanWork` code (R11) — contribution-mode rows simply
carry an empty `BranchConvention` the same way an archived Work's row already
carries a non-empty `ArchivedAt`. No migration, no new index.

## 7. Branch Convention (existing catalog entity, `registry.Convention` — unchanged)

Unchanged shape (`Name`, `Prefixes[]`). What changes is population: after F4,
`registry.Conventions` may hold entries from more than one installed package
(FR-025). Two packages declaring the same `Name` is explicitly not a
collision (spec edge cases) — `internal/convention.Catalog` already looks up
by bare `Name` with no alias-qualification path; this plan does not add one,
since the spec treats same-named conventions as visible-but-independent
catalog data, never disambiguated the way component names are.

## 8. Convention Choice (new persisted entity — `~/.work/state/
branch_conventions.json`)

The per-repository memoized choice ADR-0011 requires, generated and
never hand-edited (mirrors `registry.json`'s own governance).

```go
type Entry struct {
    Identity   string // the repository identity key (§9)
    Convention string // a registry.Convention.Name at the time it was chosen
}
```

- Identity: `Identity`, unique within the file.
- Not validated against the *current* catalog on read — a convention a plugin
  later uninstalls (F7) or a name collision does not retroactively invalidate
  an already-memoized choice; that reconciliation is explicitly F7 scope
  (Out of Scope: "Uninstalling... a plugin: out of scope for this slice's
  commands").
- Lifecycle: created on a repository's first fork/new-mode use when the
  catalog has more than one entry and the user chooses (or silently, when it
  has exactly one — R15); read by every later `work start` against the same
  identity and by `work convention show`; overwritten by `work convention
  set <name>` or an interactive hub change (FR-026, FR-027).
- Never created or consulted for `contribution` mode (branch convention is
  neither an input nor an output of contribution mode — ADD §7, FR-020).

## 9. Repository Identity (transient, computed — ADR-0011)

Not persisted itself; it is the *key* under which entity 8 is stored. Computed
fresh on every `work start`/`work convention *` invocation from the current
repository (never cached):

```text
1. origin remote fetch URL, if configured           -> identity = that URL
2. else, root commit hash(es):                       -> identity = sorted, "+"-joined hashes
   git rev-list --max-parents=0 HEAD
3. else (shallow clone, no remote):                  -> identity = absolute, symlink-resolved repo path
```

- Layers are tried in this fixed order; a repository never uses a lower-
  priority layer merely because a higher one is "less convenient" — only
  because it is absent (ADR-0011).
- Layer 2 sorts+joins on more than one root commit (merged unrelated
  histories) so the key is deterministic regardless of history-merge order
  (RNF-3).
- Never computed for a Work's *worktree* path — always for the resolved
  source repository (`work start`) or the current working directory's
  discovered repository root (`work convention *`), per ADR-0011's "runs from
  any clone" requirement.

## 10. Entity relationships

```text
Plugin Package  --installs into-->  Registry Package (1) --carries--> Registry Component (0..n)
                                                        \--carries--> Registry Convention (0..n)

work start <arg>
  --Starter Match--> Starter Outcome (transient)
  --Invoke-->        Response (transient: repository fields + BaseBranch + StartModes)
  --resolve-->        validated local repo path (unchanged F3 pipeline)
  --mode-->           Start Mode ∈ {new, fork, contribution}
     ├─ new/fork:     Repository Identity --lookup/persist--> Convention Choice --supplies--> chosen Convention's Prefixes
     └─ contribution: (no identity lookup, no convention, no prefix)
  --materialize-->    Work (schema 3), projection.Work row
```

## 11. Validation summary (new rules only; existing F1–F3 rules unchanged)

| Rule | Enforced by | Failure |
|---|---|---|
| Manifest role-field validation | `internal/plugin.Parse` (unchanged) | `plugin-invalid` (31) |
| Alias resolves to a different existing origin | `internal/plugininstall` (R5) | `plugin-alias-conflict` (32) |
| A second enabled fallback Starter would result | `internal/plugininstall` (R6) | `plugin-fallback-conflict` (33) |
| `--link` combined with a remote source | `internal/cli/plugin_install.go` | `usage` (2, unchanged category) |
| No pattern match and no registered fallback | `internal/starter.Match` | `starter-not-matched` (35) |
| Non-interactive Starter pattern collision | `internal/cli/start.go` | `starter-ambiguous` (36) |
| Unrecognized `start_modes` value, or contribution with no base branch | `internal/cli/start.go` (R9) | `starter-response-invalid` (37) |
| `work convention set <name>` names a non-enabled convention | `internal/cli/convention_set.go` | `convention-unknown` (38) |
| `work-state.json` `branch_convention` present iff `start_mode != "contribution"` | `internal/work.State.Validate` | (programming error if violated — the CLI never constructs an invalid `Params`) |
