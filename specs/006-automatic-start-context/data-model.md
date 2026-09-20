# Data Model: Automatic Context on Start (F5)

Entities, fields, validation and state transitions. Names are the Go identifiers the plan
introduces or extends; persisted shapes are in the linked contracts. "Δ" marks a change to
an existing type; "NEW" a new one. Authority: spec FR-001..FR-046; `research.md` R1–R17.

## 1. Manifest (`internal/plugin`) — Δ

```text
Component (Δ)
  Name, Role, Entrypoint, Runtime, Pattern, Accepts, DisplayName, Description   (unchanged)
  Key        string
  On         []Subscription        (was []string)     importer only
  Manual     *Manual               (was bool)         importer, linker
  Inputs     []string                                  importer only (linker: Discover.Inputs)
  Discover   *Discover             (was []string)     linker only

Subscription (NEW)  { Event string; Starters []string }
Manual       (NEW)  { DisplayName string; Description string }
Discover     (NEW)  { Automatic bool; On []Subscription; Inputs []string }
Input        (NEW)  { Source string; Key string; Optional bool }     // ParseInput result

CoreEvents (NEW)    = { "start:finalized" }        // plugin.EventStartFinalized
```

**Validation** (all failures → `plugin-invalid`, 31, nothing registered):

| Rule | Source |
|---|---|
| role table unchanged (required / allowed / forbidden / anyOf) | FR-001, ADD §4.1 |
| `on`, `discover.on`: non-empty array of Subscription | FR-001 |
| `Subscription.Event ∈ CoreEvents` | FR-002 |
| `Subscription.Starters[i]`: non-empty; `name` or `alias/name` (both parts non-empty) | FR-002 |
| `Manual.DisplayName` non-empty | FR-001 |
| every input parses via `ParseInput` | FR-003 |
| `Input.Source == "work"` ⇒ `Key ∈ semconv.Facts` | FR-003, FR-017 |
| `Input.Source ∈ {meta,link}` ⇒ `semconv.ValidKey(Key)` (syntax only) | FR-003 |
| no key repeated within one component's inputs (any namespace) | FR-003 |
| `Key` (Linker) satisfies `semconv.ValidKey`; a private key must be `plugin.<manifest.Name>.<local>` and `manifest.Name` must contain no `.` | FR-006, FR-008 |
| every unknown field inside `on[]`, `manual`, `discover` rejected | FR-001 |

A `runtime` that is not on `PATH` is **not** a manifest error: it is checked by
`plugininstall.Install` after parsing and fails `plugin-install-failed` (34) (FR-007).

## 2. Registry (`internal/registry`) — Δ

```text
Component (Δ)   existing fields, plus
  On        []Subscription `json:"on,omitempty"`
  Manual    *Manual        `json:"manual,omitempty"`
  Inputs    []string       `json:"inputs,omitempty"`     // importer inputs, or linker discover.inputs
  Key       string         `json:"key,omitempty"`
  Discover  *Discover      `json:"discover,omitempty"`   // { automatic, on, inputs }

Package (Δ)     Alias, Origin, Reference, Conventions (unchanged), plus
  PluginName string `json:"plugin_name,omitempty"`       // manifest name; alias when empty

Methods (Δ/NEW)
  Component.Target(pluginsDir) ipc.Target                // NEW: {Runtime, Path}
  Component.EntrypointPath                               // Δ: ".exe" only when Runtime == ""
  Component.QualifiedName() string                       // NEW: Alias + "/" + Name
  Registry.PluginNameOf(alias) string                    // NEW
  Registry.Extensions(role) []Component                  // NEW: Importers/Linkers only
```

`Discover.Inputs` and top-level `Inputs` are stored in one `Inputs` slot per component: a
Linker's value is copied from `discover.inputs`, so the runtime reads one place.
Registered-before-F5 Importers/Linkers carry none of the new fields and are inert (D10).
`plugininstall.ComponentEntry(alias, plugin.Component)` is the single builder used by
`plugininstall.Install` and `bootstrap.registerComponents`.

## 3. Keys and facts (`internal/semconv`) — NEW leaf package

```text
ValidKey(key string) error                        // grammar only
ValidatePublished(owner string, key string) error // grammar + ownership (private keys)
ValidLinkValue(v string) error                    // non-empty string
Facts = [worktree_path, start_mode, branch, base_branch, slug]
```

Grammar and ownership: `research.md` R4, `contracts/semantic-conventions.md`.

## 4. Starter response (`internal/starter`, `internal/ipc`) — Δ

```text
starter.Reference (Δ)   Path, GitFetchURLs, Name, Query, BaseBranch, StartModes (unchanged)
                        + Meta  map[string]any
                        + Links map[string]string
```

`ipc.StarterResponse` already decodes both. `starter.ValidateResponse(ref, owner)` (Δ) adds:
every `Meta` and `Links` key passes `semconv.ValidatePublished(owner, key)`; every link
value passes `ValidLinkValue`. `owner` is `Registry.PluginNameOf(chosen.Alias)`. A violation
is `starter-response-invalid` (37), raised before `create.Run` — so no state exists to undo.

## 5. Snapshot (`internal/work`) — semantics only, no schema change

`State.Meta` (`map[string]any`) and `State.Links` (`map[string]string`) already exist and
are required objects. F5 changes who fills them: `create.Params{Meta, Links, StarterComponent}`
→ initial snapshot (`StarterComponent` is the Starter's qualified `alias/name`; `create` owns
the Work id and the timestamp, so it derives the `start` provenance rows itself); the pipeline
later upserts `Links[key]`. `work.Write` still emits `schema: 3`; `Validate` unchanged. Link
values are non-empty by construction (R4).

## 6. Provenance (`internal/projection`) — NEW

```text
Provenance { WorkID, Section, Key, SourceComponent, SourceOperation, RecordedAt string }
  Section         ∈ {"meta", "links"}
  SourceComponent = Component.QualifiedName()          // e.g. "github-plugin/github-pull-request-linker"
  SourceOperation ∈ {"start", "discover"}              // Starter publication | Linker discovery
  RecordedAt      = RFC 3339 UTC

DB.UpsertWithProvenance(row Work, entries []Provenance) error   // one SQL tx; create's commit point
DB.RecordProvenance(entries ...Provenance) error                // pipeline, after a Linker persists
DB.Provenance(workID string) ([]Provenance, error)              // read side (used by tests; F6 status)
```

Table, migration 2→3 and rebuild behaviour: `contracts/index-provenance.md`. Keyed
`(work_id, section, key)`: an upsert replaces the previous source (last source wins,
FR-023/FR-025) — the *value* lives only in the snapshot.

## 7. Extension pipeline (`internal/extension`) — NEW

```text
Context      { Home workhome.Home; Registry *registry.Registry; State *work.State;
               Starter registry.Component; WorktreePath, WorkDir, SnapshotPath string;
               Index Indexer; Observer Observer; Now func() time.Time }

Indexer      interface { RecordProvenance(...projection.Provenance) error }   // *projection.DB
Observer     interface { OnEvent(Event) }                                     // renders; never in extension

Decision     { Component registry.Component; Eligible bool; Inputs map[string]any }   // Eligible(...) result
Event        { Kind EventKind; Component string; Operation string; Detail string; Warning *diag.Warning }
EventKind    ∈ { Running, Linked, Imported, Warned }
Operation    ∈ { "discover", "import" }

Report       { Ran int; Warnings []diag.Warning }

func Run(ctx context.Context, c Context) Report        // phases 1–4, sequential, never returns an error
func Eligible(reg, state, starter, event) []Decision   // pure; sorted by QualifiedName (R7)
```

`Run` never returns an error: every failure becomes a `Warning` (FR-035/FR-036). It returns
after the last phase, or immediately after an interrupt (R14).

### Pipeline state machine (per `work start`, after `create.Run` commits)

```text
COMMITTED ──▶ LINKER_PHASE ──▶ IMPORTER_PHASE ──▶ DONE
    │             │                  │
    │ (no eligible│ per Linker:      │ per Importer:
    │  component  │  Running → run → │  Running → stage → run → plan → incorporate
    │  ⇒ skip,    │  value?  ─ yes ─▶│  ok ⇒ Imported(n) │ refusal/failure ⇒ Warned, effects undone
    │  print      │  persist+prov    │  stage removed on every path
    │  nothing)   │  no value ⇒ silent
    │             │  failure ⇒ Warned, nothing persisted
    ▼             ▼                  ▼
 interrupt at any point ⇒ kill child, discard its output, Warned(extension-interrupted),
                          skip the rest ⇒ DONE   (Work kept, exit success)
```

Transitions never revisit a phase; the Importer phase re-evaluates eligibility from the state
*after* every Linker result is persisted (FR-019).

## 8. Staging (`internal/staging`) — NEW

```text
Stage       { Dir string }                                   // NewStage() creates <tmp>/work/import-*; Remove()
Item        { Rel string; Dest string; IsDir bool }
Plan        { Items []Item }                                 // built read-only; sorted (dirs before their files)
Refusal     { Rel string; Reason RefusalReason }             // implements error; Reason ∈ below
RefusalReason ∈ { Exists, ReservedPath, NotRegular, EscapesWork }

Build(stageDir, workDir string) (Plan, error)                // *Refusal if any item is refused; touches nothing
(Plan).Incorporate() (created int, err error)                // O_EXCL, undo log, all-or-nothing (R11)
```

`Build` is pure over the filesystem (reads only). `Incorporate` returns an error only after
it has already undone everything it placed.

## 9. Warning (`internal/diag`) — NEW

```text
Warning { Token string; Summary string; Hint string; Cause error; Work string;
          Component string; Operation string }
Tokens: extension-start-failed | extension-failed | extension-response-invalid |
        extension-output-refused | extension-persist-failed | extension-interrupted
```

No exit code. `diag.FormatWarning(w)` yields the frozen non-interactive line
`warning: <token>: <message>` where `<message>` carries `Work`, `Component` and `Operation`
(FR-037). Never contains a link/meta value or artifact content.

## 10. Relationships

```text
plugin.Manifest ──install──▶ registry.{Package, Component}     (activation data, R2)
registry.Component ──Eligible──▶ Decision ──Run──▶ ipc.Target subprocess
Starter response ──ValidateResponse──▶ create.Params{Meta,Links} ──▶ work.State + Provenance(start)
Linker value ──persist (locked RMW)──▶ work.State.Links + Provenance(discover)
Importer stage ──Build/Incorporate──▶ <workspace>/in-progress/<repo>_<branch>/…   (beside worktree/)
```
