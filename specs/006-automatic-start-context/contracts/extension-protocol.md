# Contract: Linker and Importer execution protocol (F5)

Authority: ADD §9, §10, §11; ADR-0000, ADR-0006, ADR-0012, ADR-0013; spec FR-016..FR-039;
`research.md` R6–R8, R11, R12, R14. Applies to components the core starts automatically at
`start:finalized`. Manual availability (`manual`) is F6.

## 1. The event and its phases

`start:finalized` is published once per `work start`, only after the Work, its worktree, its
snapshot (including Starter-published `meta`/`links`) and its index row are committed. A start
that failed or was cancelled earlier never reaches it.

```text
phase 1  Linker discovery      eligible Linkers, sequential, sorted by "alias/name"
         persist               each returned value: locked read-modify-write of the snapshot + provenance
phase 2  Importers             eligibility re-evaluated after phase 1; eligible Importers sequential,
                               sorted by "alias/name": stage → run → plan → incorporate → remove stage
```

Order inside a phase is fixed and reproducible for a given installed set and **not** a
contract authors may rely on. There is no priority, position, or dependency field.

## 2. Eligibility (static — nothing is started to decide)

A component runs only if **all** hold; otherwise it is skipped with no output and no warning:

1. *Linker*: `discover.automatic == true`. *Importer*: `on` present.
2. Some subscription (`discover.on` / `on`) names `start:finalized`.
3. That subscription's `starters` (if present) matches the Starter that produced this Work:
   `name` matches any Starter component with that name; `alias/name` matches exactly.
4. Every input without `:optional` resolves (below).

`starters` is never sent to the component. A component skipped for a missing input because an
earlier component failed produces no extra warning.

## 3. Input resolution and projection

| Source | Resolved from | Absent when |
|---|---|---|
| `work:<fact>` | the just-created Work | `slug` in contribution mode; never otherwise |
| `link:<key>` | current `links` (phase 2 sees phase 1's results) | no such key |
| `meta:<key>` | `meta` | no such key |

Facts: `worktree_path` (absolute), `start_mode` (`new`/`fork`/`contribution`), `branch`,
`base_branch`, `slug`. The delivered `inputs` object contains **only** the declared inputs
that resolved, keyed by the **bare key** (the namespace is not repeated); values keep their
JSON type. An absent optional input is omitted, never sent as `null` or empty. Nothing else
about the Work, its links, its metadata, the snapshot path, or the registry is sent.

## 4. Wire format

Exactly one JSON document on stdin; at most one JSON document on stdout; exit status;
free-form stderr. The entrypoint is run per the manifest's `runtime` (see
`plugin-manifest-extensions.md`). The working directory is inherited and not part of the
contract. Work imposes no timeout.

### Linker discovery

```jsonc
// stdin
{ "inputs": { "worktree_path": "/home/u/.workspaces/in-progress/project_feature-x/worktree" } }
// stdout — any of:
{ "value": "https://github.com/example/project/pull/212" }     // a value
{}                                                              // no value (success)
                                                                // (empty stdout is also "no value")
```

`value` absent or `null` = no value: nothing changes, nothing is reported. `value` that is
`""` or not a string = **invalid response**. Extra fields are ignored. The link written is
always `links[<the Linker's declared key>]`; nothing the response names can redirect it.

### Importer

```jsonc
// stdin
{ "inputs": { "github.pull_request": "https://…", "start_mode": "contribution" },
  "output_dir": "/tmp/work/import-8f3a…" }
// stdout: empty, or one JSON object whose content is ignored
```

The Importer writes only under `output_dir` (a new, empty, mode-0700 directory unique to this
execution, outside the Work). It is given no other location to write to.

## 5. Persisting a Linker value

Under the per-Work lock (same key as `work resume` / `work archive`): read the snapshot, set
`links[key] = value` (replacing any earlier value — Starter's or another Linker's — last
source wins), write atomically, then record provenance `(work, links, key, "alias/name",
"discover", now)`. If two Linkers share a key and both return a value, the later in the
phase order wins and provenance names it. A lock/read/write failure is
`extension-persist-failed` for that Linker; earlier Linkers' results stay.

## 6. Importer staging, planning, incorporation

After exit 0 and a valid stdout, and **before any change to the Work**, Work builds a plan
from the staged tree:

| Refused when… (whole execution; nothing incorporated) | Reason |
|---|---|
| a staged **file**'s destination already exists (any type) | `Exists` |
| a staged **directory**'s destination exists and is not a directory | `Exists` |
| a destination is `worktree` or under it, or is `work-state.json` (compared case-insensitively) | `ReservedPath` |
| a staged entry is not a regular file or a directory (symlinks included) | `NotRegular` |
| a destination's deepest existing ancestor resolves outside the Work directory | `EscapesWork` |

Destination = `<Work directory>/<staged relative path>`; the Work directory is
`<workspace>/in-progress/<repo>_<branch>/` (beside `worktree/`, never inside it). An existing
directory is merged into provided no file collides. If the plan is clean, incorporation
creates directories then files with exclusive create (never overwrites; on a case-insensitive
volume a second name differing only in case fails here), recording what **it** created. Any
failure removes exactly what was recorded, files then directories, in reverse. Empty output
succeeds and changes nothing. The stage directory is removed on every outcome, including
refusal, failure and interruption. Importers run one at a time, so a later plan sees an
earlier Importer's incorporated files.

## 7. Failure classes → warnings (never rollback of the Work)

Each yields exactly one warning, all effects of that component absent, and the run continues
with the next component (FR-035, FR-036).

| Token | When |
|---|---|
| `extension-start-failed` | entrypoint missing/unreadable, runtime missing, exec error |
| `extension-failed` | non-zero exit |
| `extension-response-invalid` | stdout not the allowed single JSON, wrong shape/type, `value` invalid |
| `extension-output-refused` | plan refused (§6) |
| `extension-persist-failed` | snapshot write/lock failed, or incorporation failed and was rolled back |
| `extension-interrupted` | user interrupt during the automatic phases (§8) |

## 8. Interrupt

The first interrupt kills the running extension, discards whatever it produced (a returned
value is not persisted; the stage is removed and nothing is incorporated), skips every remaining
component, reports one `extension-interrupted` warning, and lets `work start` finish normally:
the Work is kept, the exit is success, the terminal is repositioned into it.

## 9. What extensions never get

The snapshot, its path, `work.db`, other components' data, registry contents, the subscription
or Starter restriction, credentials, or a way to write outside `output_dir`. Work never reads
a plugin's files to learn its behavior. Staging is a data boundary, not a sandbox: a plugin
is still a process with the user's permissions (PRD non-goal).
