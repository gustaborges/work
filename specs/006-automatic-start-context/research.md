# Research: Automatic Context on Start (F5)

Phase 0 output for `specs/006-automatic-start-context/plan.md`. Each entry records a
decision, why, and what was rejected. Entries marked **[code finding]** come from reading
the F1–F4 code rather than the documents; they are the reason the plan carries work the
roadmap does not list. No `NEEDS CLARIFICATION` remains: the spec's D1–D10 defaults are
carried through unchanged.

## R1 — The Importer/Linker manifest model follows ADD §4, and replaces the F1 placeholder shapes

**[code finding]** `internal/plugin.Component` types `on` as `[]string`, `manual` as `bool`
and `discover` as `[]string`. A manifest written exactly as ADD §4 shows it
(`on: [{event, starters}]`, `manual: {display_name, description}`, `discover: {automatic,
on, inputs}`) fails `plugin.Parse` today with `cannot unmarshal object into ... manual of
type bool` (verified with a throw-away test). F1's contract (`specs/001.../plugin-manifest.md`)
pinned only which fields are *present* per role; its tests use placeholders (`"on":["clone"]`,
`"manual":true`, `"discover":["x"]`) that were never a documented shape and were never
executed.

**Decision**: replace those three field types with the ADD §4 objects.

| Field | New type | Rules |
|---|---|---|
| `on` (importer), `discover.on` (linker) | non-empty `[]Subscription{event, starters?}` | `event` must be a core event (R6); `starters` is a non-empty list of names (`name` or `alias/name`); unknown fields rejected |
| `manual` | `{display_name, description?}` | `display_name` non-empty; unknown fields rejected |
| `discover` | `{automatic bool, on []Subscription?, inputs []string?}` | unknown fields rejected; `automatic:true` without `on` is accepted and inert (D6) |
| `inputs` (importer top level; `discover.inputs` for a linker) | `[]string` | grammar and rules in R5; a duplicate key across namespaces is rejected |

The role table (required/allowed/forbidden) is unchanged; the linker's `inputs` stays
forbidden at the top level because ADD §4 places them under `discover`.

**Rationale**: ADD §4 is the only public statement of the shape (ADR-0012 says "`on`",
"`manual`", "`inputs`", "`discover`" without shapes). Keeping a second, looser accepted
form would let a manifest install and then never be eligible.

**Alternatives rejected**: accepting both shapes (two code paths, one of which is
undocumented); leaving the manifest alone and only extending the registry (an ADD-conformant
plugin would still be uninstallable — SC-001 unreachable).

## R2 — The registry records activation data; both writers share one entry builder

**[code finding]** `registry.Component` has no field for `on`, `manual`, `inputs`, `key` or
`discover`, although F4's spec assumed the registry recorded them. Two places build the
entry from a manifest component — `plugininstall.Install` and `bootstrap.registerComponents`
— by copying the same eight fields.

**Decision**: add the five fields to `registry.Component` (persisted forms `Subscription`,
`Manual`, `Discover`, defined in `registry`, independent of the manifest parse types) and
add one exported `plugininstall.ComponentEntry(alias, plugin.Component) registry.Component`
used by both writers, so a future field cannot be added to one and forgotten in the other.
`registry.Package` gains `PluginName` (the manifest `name`, needed for private-key ownership
because the alias can differ under `--as`); readers fall back to the alias when it is empty.

`registry.json` stays additive and unversioned. A component registered before this slice has
none of the new fields: an Importer without `on` and a Linker without `discover` are never
eligible (D10). Nothing is migrated; reinstalling the same origin refreshes the entry
(F4's FR-006a replace-on-reinstall already guarantees the alias's records equal the new
manifest).

**Rejected**: re-deriving activation data from `plugins/<alias>/plugin.json` at start time
(violates FR-004 — "no manifest re-read"; and a `--link` package's manifest can change under
the registry).

## R3 — Every component is invoked through its declared runtime; the runtime is preflighted at install

**[code finding]** `runtime` is copied into the registry and then ignored: `ipc.Run` does
`exec.Command(entrypoint)`, `starter.Invoke` and the locator chain pass
`Component.EntrypointPath`, and `EntrypointPath` appends `.exe` on Windows unconditionally.
No `LookPath` exists for a runtime (the only one is for `git`). F1's contract said "checked
in preflight" and F4's FR-008 required it; neither was built. A Starter declared
`"runtime": "python3"` cannot run today.

**Decision** (spec FR-007, D8):

- `ipc` gains a `Target{Runtime, Path}`; `Run`/`InvokeStarter`/`InvokeLocator` take it (and
  a `context.Context`, see R14). With a runtime: `exec.Command(runtime, path)`; without:
  `exec.Command(path)` — behaviour byte-identical for the seed and every F4 fixture.
- `registry.Component.EntrypointPath` appends `.exe` only when `Runtime == ""` (a script is
  not an executable), and a new `Component.Target(pluginsDir)` builds the `ipc.Target`.
- `plugininstall.Install` calls `exec.LookPath(runtime)` for every component that declares
  one, **before** staging; a miss fails `plugin-install-failed` (34) with the runtime named
  and the hint "install it or fix PATH". The same check runs again when the component is
  started (a runtime can disappear after install): a miss there is `extension-start-failed`
  (R12) for an extension, and the existing unusable-repo failure for a Starter/Locator.

**Rationale**: it is the same invocation path every extension uses; fixing it once now is
cheaper than shipping F5's first interpreted Linker/Importer on a path known not to work.

**Rejected**: leaving it as a separate F4 bug fix (F5's fixtures could not include an
interpreted component, so SC-001's "written outside the core" would be tested only for
compiled plugins); shell-out through `sh -c` (violates ADR-0006 — no shebang/shell reliance).

## R4 — Key grammar, ownership, and value rules live in a leaf package; no closed key list

**Decision**: new leaf package `internal/semconv` (imports nothing internal) holding:

- the **key grammar**: a *public* key is `segment(.segment)+` with `segment = [a-z][a-z0-9_]*`;
  the first segment is the namespace; the first segment may not be `plugin` or `work`
  (reserved). A *private* key is `plugin.<plugin-name>.<local>` where `<local>` is
  `segment(.segment)*` and `<plugin-name>` is the publishing plugin's manifest `name`.
- **ownership**: a private key may be published only by the plugin it names.
  `<plugin-name>` is matched by the exact prefix `plugin.<name>.`; a plugin whose name
  contains `.` **cannot publish private keys** (a name such as `foo.bar` would make
  `plugin.foo.bar.x` ambiguous with plugin `foo`'s key `bar.x`); it can still publish public
  keys. Documented in `contracts/semantic-conventions.md`.
- **values**: link value = non-empty string; meta value = any JSON value; stored verbatim.
- `Facts`: the five exposed `work` facts (R5).

Enforced at: install (Linker `key`, `meta:`/`link:` inputs — syntax only), Starter response
(`meta`/`links` keys — syntax + ownership + values), Linker discovery (the key is the
declared one, so only the value rule applies). **No list of public keys is compiled into the
binary** (ADR-0013): a well-formed key not in the published document is accepted.

**Semantic Conventions v1 (D1)** is `contracts/semantic-conventions.md`: grammar, ownership,
representation rules, the "adding a key" policy, and the initial keys the fixtures use
(`github.pull_request` link; `github.pull_request.number` meta; `example.*` for
fixture-only keys). Contracts under `specs/` are authoritative for plugin authors here, the
same precedent as F1's `plugin-manifest.md`; `docs/roadmap.md` §5's prerequisite is
satisfied by that file.

**Rejected**: a compiled-in allow-list (ADR-0013 forbids a closed enumeration); no
validation at all (violates FR-008 and lets a plugin write another's private namespace).

## R5 — The `work` fact surface is a fixed, small set, checked at install

**Decision** (D2): `worktree_path`, `start_mode`, `branch`, `base_branch`, `slug`. Values
come from the just-created `work.State` (`worktree_path` from `create.Result`). `slug` is
absent in contribution mode. Input grammar: `^(work|meta|link):[^:]+(:optional)?$` (the F1
regexp, kept) plus: a `work` key must be a fact; a `meta`/`link` key must be a valid key
(R4); the same key twice in one component (in any namespace) is rejected, because the
delivered document is keyed by bare key. An unknown `work` key fails installation
(`plugin-invalid`, 31): a plugin needing a newer Work is refused early, not made silently
ineligible.

**Rejected**: exposing `id`/timestamps/status/convention (no consumer, widens a contract
that is hard to shrink); leaving unknown `work` keys to fail at eligibility (a typo would be
undetectable).

## R6 — Eligibility is a pure function of (registry, state, chosen Starter, event)

**Decision**: `internal/extension.Eligible` returns, per component, `Decision{Eligible bool,
Reason, Inputs map[string]any}` with **no I/O beyond reading the snapshot values already in
memory**. Order of checks and semantics:

1. role is Linker with `Discover != nil && Discover.Automatic`, or Importer with `On != nil`;
2. some subscription's `event` equals the published event (`start:finalized` — the only core
   event, defined as `plugin.EventStartFinalized`, the sole member of `plugin.CoreEvents`);
3. that subscription's `starters` (if any) matches the **chosen Starter component**: an entry
   without `/` matches any Starter with that name; `alias/name` matches exactly. Matching
   the component (not the recorded `work.starter` string) removes the surprise where
   `work.starter` becomes `alias/name` only when two Starters share a name;
4. every non-optional input resolves; optional inputs never gate and are omitted when absent.

A component that is not eligible is neither run nor reported (FR-018, FR-039).

**Input resolution sources**: `work:` from the in-memory facts; `link:` from the *current*
`State.Links` (so Importers see Linker results); `meta:` from `State.Meta`. The Linker phase
evaluates every Linker against the state **as it was before the phase started**, so no
Linker can depend on another's result in the same phase (consistent with "no guaranteed
order within a phase"); the Importer phase re-reads after the persisted Linker results
(FR-019).

## R7 — Fixed order inside a phase: bytewise by `alias/name`

**Decision** (D7): eligible components run sequentially, sorted ascending by the qualified
name `alias + "/" + name`. Nothing else influences order: not install order, not manifest
order, not any field. The manifest already rejects `priority`/`score`; nothing new is added.
For two Linkers sharing a key, the later in this order wins (FR-026); provenance records the
winner. The document tells authors order is *deterministic but not a contract* (FR-020).

**Rejected**: concurrent execution (non-deterministic winner, interleaved progress; out of
scope per the architecture); install order (NFR-9 forbids install-order precedence);
"first non-empty wins" (contradicts ADD §9's last-source-wins).

## R8 — Wire contracts: one JSON in, at most one JSON out, `inputs` keyed by bare key

**Decision** (full text in `contracts/extension-protocol.md`):

- Linker discovery: stdin `{"inputs": {...}}`; stdout empty, `{}` or `{"value": <string>}`.
  `value` absent or `null` = no value; an empty string or any non-string = invalid response.
  Extra fields ignored. The key is never taken from the response.
- Importer: stdin `{"inputs": {...}, "output_dir": "<absolute staging path>"}`; stdout empty
  or one JSON object whose content is ignored. Anything else on stdout = invalid response.
- Exit 0 = success; any other exit or failure to start = failure. stderr is captured (last
  4 KiB) for `WORK_DEBUG` only.
- `inputs` values keep their JSON type (strings for `work`/`link`; any JSON for `meta`).
- No timeout imposed (FR-034); the working directory is inherited, not part of the contract.

`starters`, the subscription, the eligibility data, the snapshot path and any other Work
state are never sent (FR-021, FR-022).

## R9 — Where the pipeline runs: after the stable lines, before the terminal is repositioned

**Decision**: in `runStart`, after `create.Run` returns (the Work exists and its Starter
context is persisted): (1) print the three existing `work:` lines to stdout, unchanged;
(2) run the pipeline, reporting on the UI writer; (3) then the existing shell-integration
step. Stdout gains nothing (FR-046); exit stays 0 on warnings (D3); a pipeline never turns a
success into an error.

`create.Run` remains the commit point for the *Work*. Because F5 adds no compensator after
that point, no extension outcome can invalidate `SC-009`-style guarantees (a failed
*creation* leaves nothing; a completed creation is never rolled back).

**Rejected**: running the pipeline before printing the stable lines (delays the durable
result behind arbitrary third-party code); running it after the terminal is repositioned
(the shell wrapper reads the target path when the process exits, so ordering inside the
process is otherwise indistinguishable — but FR-042 states the order, and writing the path
last also keeps a hung extension from leaving a half-updated cd target).

## R10 — Persistence: Starter context in the commit; Linker results by locked read-modify-write; provenance in the index

**Starter context** — `create.Params` gains `Meta map[string]any` and `Links
map[string]string`; `build` puts them in the initial `State` (today it hard-codes empty
maps). The snapshot, the `works` row and the provenance rows for these entries are written
by the same step that is already the commit point (`db.Upsert`): `Upsert` grows a sibling
`UpsertWithProvenance(row, entries)` that runs both in one SQL transaction, so there is no
state with the Work indexed and its provenance missing.

**Linker results** — per Linker, under the per-Work lock resume and archive already take
(same `sha256(id)` key; extracted once as `workhome.Home.WorkLockPath(id)` and used by
create, resume, archive and the pipeline): read the snapshot, set `Links[key]`, `work.Write`
(atomic rename), then record provenance. Per-Linker persistence lets a failed write be
attributed to that Linker (FR-024). A failure to read/lock/write is `extension-persist-failed`.

**Provenance** (D4) lives in a new `work_provenance` table (`contracts/index-provenance.md`),
`ON DELETE CASCADE` from `works` (the connection already sets `foreign_keys=ON`). `Upsert`
uses `ON CONFLICT DO UPDATE`, not replace, so reconcile's upserts do not cascade-delete it.
`SchemaVersion` becomes 3; `Reset` drops `work_provenance` before `works`.

**Rebuild interplay [code finding]**: `reconcile.Open` does a *full rebuild* whenever the
on-disk `user_version < SchemaVersion`. So every F4 database is rebuilt once, on its first
F5 command. That is lossless (no provenance existed) and is the same mechanism F2 used for
1→2. Thereafter a rebuild restores every Work's core data, metadata and links from
snapshots but **not** provenance (FR-044, D4); the migration and `Reset` comments say so.

**Snapshot version**: unchanged at 3. `Meta`/`Links` already exist in `work.State`, are
required objects in `Validate`, and are decoded by `ipc.StarterResponse` (`Links` is
`map[string]string`, so a non-string link value already fails to decode). No field becomes
required, so an F4 reader accepts an F5 snapshot (FR-015).

**Rejected**: provenance inside `work-state.json` (needs schema 4 and contradicts ADR-0013's
"operational provenance stays out of the canonical state"); one write for the whole Linker
phase (cannot attribute a failure; a late failure would discard earlier Linkers' valid
results); writing the snapshot from a long-lived in-memory copy (a concurrent `work resume`
in another terminal would lose its `last_accessed_at` update).

## R11 — Staging and incorporation: copy with exclusive create, undo log, no rename across volumes

**Decision** (`internal/staging`, presentation-free):

1. **Stage**: `os.MkdirTemp(<os temp>/work, "import-")` (mode 0700), unique per execution,
   outside the Work. Removed with `os.RemoveAll` in a `defer` on every path. A killed
   process can leave one behind; it is never read by any later run (each run makes a fresh
   name) and lives in the OS temp area.
2. **Plan** (read-only): walk the stage; every entry must be a regular file or directory
   (`Lstat`); compute `dest = workDir/<rel>`; refuse per R12; `Lstat` each destination
   (exists ⇒ collision if the staged entry is a file; if the staged entry is a directory the
   destination may be an existing directory — merge — but not a file); resolve the deepest
   existing ancestor of each destination with `EvalSymlinks` and require it to remain inside
   the resolved Work directory. No change to the Work happens in this step, so a refusal
   leaves it byte-identical (SC-004).
3. **Incorporate**: create directories then files in sorted order; each file is copied to its
   final path with `O_CREATE|O_EXCL|O_WRONLY` (never overwrites, and on a case-insensitive
   filesystem the second of two names differing only in case fails here too), recording every
   directory and file *this run created* in an undo log. On any error, remove the recorded
   files then the recorded (still empty) directories in reverse; content this run did not
   create is never touched (FR-030).
4. Files keep their permission bits masked to `0o644`/`0o755` classes; nothing else is
   preserved (timestamps, ownership, xattrs).

**Why copy, not rename**: the stage is in the OS temp area, usually a different volume from
the workspace; `rename` is not portable across volumes (`EXDEV`) and a partial move is exactly
what FR-030 forbids.

**Rejected**: staging inside the Work directory (a crash would leave junk in the Work and the
collision walk would have to ignore it); `atomicfile` per file (atomic per file, not per
execution — the undo log is what gives all-or-nothing); probing filesystem case-sensitivity
up front (fragile; O_EXCL + undo gives the same outcome with no probe).

## R12 — Refusal rules and the failure-class → warning-token map

**Refusals** (whole execution, FR-029): destination exists as a file; destination at or under
`worktree` or equal to `work-state.json` (compared with `strings.EqualFold`, deliberately
conservative — refusing `Worktree/` on a case-sensitive filesystem is harmless, allowing it
on a case-insensitive one would write into the checkout); non-regular, non-directory entry
(symlinks refused outright: they can escape and behave differently on Windows); destination
whose existing ancestor resolves outside the Work directory.

**Warning tokens** (new, stable, on the UI channel; no exit codes — `contracts/cli-work-start.md`):

| Token | Class (FR-035) |
|---|---|
| `extension-start-failed` | entrypoint missing, runtime missing, not executable |
| `extension-failed` | exited unsuccessfully |
| `extension-response-invalid` | stdout not the allowed single JSON, or wrong shape/type (incl. `value` empty/non-string) |
| `extension-output-refused` | Importer output refused (R11/R12) |
| `extension-persist-failed` | snapshot/lock write failed, or incorporation failed and was rolled back |
| `extension-interrupted` | user interrupt during the automatic phases |

**Errors (with exit codes) reuse existing categories**: manifest declaration problems →
`plugin-invalid` (31); missing runtime at install → `plugin-install-failed` (34); invalid
Starter `meta`/`links` → `starter-response-invalid` (37). **No new exit code is added.**

## R13 — Warnings and progress render at the CLI border; lower layers return values

**[code finding]** `renderDiagnostic`'s contract says it is the only place a terminal
failure is printed and lower layers "never write to a diagnostic stream". `diag` has no
non-fatal concept; the theme has a `Warning` token already.

**Decision**: `internal/diag` gains `Warning{Token, Summary, Hint, Cause}` and
`NewWarning`; `internal/extension` *returns* warnings through an `Observer` callback and
never writes. `internal/cli/diagnostics_border.go` gains `renderWarning(ui, in,
interactive, w)` next to `renderDiagnostic`: non-interactive `warning: <token>: <message>`
(byte-stable, no control sequences, mirroring `error: <token>: <message>`); interactive
`⚠ <Summary>` + `  → <Hint>` through a new `diagrender.Warn(th, summary, hint)`.
`WORK_DEBUG` appends the extension's stderr tail and exit status (FR-037). Progress lines
(`work: running <component> (<operation>)`, `work: <component>: linked <key>`, `work:
<component>: imported <n> item(s)`) are plain text on the UI writer in every mode, themed
muted when interactive. **No new Bubble Tea model**: an animated indicator is optional in
FR-040 and would add a live region the wizard's geometry rules (ADR-0021) do not need.

A warning names Work id, qualified component and operation; it never contains a link/meta
value or artifact content (the colliding *relative path* is metadata about the refusal, not
content).

## R14 — Interruption: the root context, `CommandContext`, and a finalizer that ignores it

`root.go` already runs commands under `signal.NotifyContext(os.Interrupt)`. **Decision**
(D9): `ipc` runs extensions with `exec.CommandContext`, so cancellation kills the child; the
pipeline treats a cancelled context as `extension-interrupted` for the running component,
skips everything after it, and returns. Cleanup that must still run after cancellation
(`os.RemoveAll` of the stage, the undo log) uses no context. `runStart` then proceeds to the
shell-integration step with success — the Work exists (FR-038). A value the Linker returned
after the context was cancelled is discarded, not persisted.

## R15 — Unchanged surfaces (regression contract)

`work start <path>` and the reference fallback Starter: the seed Starter returns no
`meta`/`links`, no Linker/Importer is installed, so `Eligible` returns nothing, `create`
persists empty maps exactly as today, no extension process starts and nothing is printed
(FR-041, FR-045). `ipc` signature changes (R3) are mechanical; seed components declare no
runtime, so their `exec.Command` argument list is unchanged. `work plugin list` output is
unchanged (it shows origin and reference, not declarations); declarations are asserted
through `registry.json` in tests. Snapshot schema stays 3 (R10); `config/work.json` gains no
key.

## R16 — Test strategy: one multi-role fixture package with behaviour selected by environment

Fixture plugins are compiled Go binaries built by `tests/fixtures/plugins.Prepare`, which
builds **one binary per distinct entrypoint name from the same `main` package**. So a single
fixture package `context-suite` ships entrypoints `starter`, `linker`, `linker2`,
`importer`, `importer2` and the binary picks its role from `filepath.Base(os.Args[0])`. The
mode is chosen with `WORK_FIXTURE_MODE` and every launch appends `{role, stdin}` to the file
named by `WORK_FIXTURE_LOG` — that log is how "never started" (SC-002) and "received exactly
its declared inputs" (SC-003) are asserted. `WORK_FIXTURE_MODE` is a comma-separated list of
`<entrypoint>-<mode>` tokens; each entrypoint reads only its own tokens, so one environment
variable configures a whole suite (`linker-value,importer2-collide`). Modes: Starter
`context`, `bad-key`, `foreign-private`; Linker `value`, `none`, `empty-value`, `garbage`,
`exit1`, `hang` (blocks until killed, for R14); Importer `ok`, `empty`, `garbage`, `exit1`,
`collide`, `symlink`, `into-worktree`, `over-state`, `mixed` (one clean + one colliding file).
A companion package `restricted-suite` carries the components that must **never** start
(Starter-restricted, `automatic:false`, missing required input, manual-only).
A second fixture, `runtime-sh` (Unix-only, `runtime: "sh"`, `starter.sh`), exercises R3; a
fixture declaring `runtime: "no-such-interpreter"` exercises the install-time miss. Incorporation
fault injection follows the existing `WORK_FAIL_AT` convention (`create.failPoint`):
`WORK_FAIL_AT=incorporate:<n>` fails after the n-th placed item.

## R17 — Windows and portability notes

`Target` builds `runtime + path` without `.exe` for scripts (R3); symlinks in staged output
are refused (R12) so Windows' privilege-gated symlinks never matter for incorporation;
`EqualFold` reserved-name checks and `O_EXCL` cover case-insensitive volumes; the stage
uses `os.TempDir()`; per-Work lock reuse keeps the existing Windows lock implementation.
Same 3-OS CI matrix; the `runtime-sh` fixture is skipped on Windows, and an equivalent
`runtime: "cmd"`-free assertion is not attempted (no portable interpreter in the runner).
