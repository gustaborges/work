# Feature Specification: Automatic Context on Start

**Feature Branch**: `feature/f5-automatic-context`

**Created**: 2026-09-19

**Status**: Draft

**Input**: User description: "`docs/roadmap.md` — we are advancing to F5 — Automatic context on start. A Work created by an origin can start with links and useful artifacts, without manual steps and without allowing an extension to corrupt the already created Work. Sources: `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md`. Identify any open question, then proceed through planning." Authorities: `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md` §3, §4, §9, §10, §11, ADR-0000, ADR-0006, ADR-0012, ADR-0013.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install a Plugin That Declares Linkers and Importers (Priority: P1)

A plugin author ships a package whose manifest declares, next to a Starter, a **Linker** (owns one link key and can discover its value when a Work starts) and an **Importer** (runs when a Work starts and contributes files, consuming a link the Linker or the Starter produced). The developer installs it with the same `work plugin install` command delivered by F4. Work accepts those declarations exactly as the architecture documents them, records when each component may run and what it needs, and executes nothing while doing so. A manifest that declares an activation, an input, or a key incorrectly is rejected with nothing registered.

**Why this priority**: Nothing else in this slice is reachable until such a package can be installed. It is also independently testable and closes a real gap: the manifest model the architecture documents for Importers and Linkers is not what installation accepts today, and the registry does not keep the data eligibility needs.

**Independent Test**: Install a fixture package declaring one Starter, one Linker (automatic discovery at start, restricted to that Starter, one input) and one Importer (activation at start, restricted to that Starter, a required link input and an optional `work` input, plus manual availability text); confirm `work plugin list` shows it and the registry holds each declaration in full. Then install fixtures that each break one declaration rule and confirm each is rejected with nothing registered and nothing written to plugin storage.

**Acceptance Scenarios**:

1. **Given** a manifest declaring an Importer with an event subscription restricted to named Starters, manual availability with display text, and inputs, and a Linker with a key, automatic discovery with its own subscription and inputs, and manual availability, **When** the user runs `work plugin install <path>`, **Then** installation succeeds and the registry holds every one of those declarations for the respective component.
2. **Given** a subscription to an event Work does not define, **When** installation runs, **Then** it fails as an invalid plugin naming the event, registering nothing.
3. **Given** an input that names a Work fact outside the set Work exposes, a malformed input string, or a `meta`/`link` key that is syntactically invalid, **When** installation runs, **Then** it fails as an invalid plugin, registering nothing.
4. **Given** a component that declares the same key in two different input namespaces (for example `meta:x.y` and `link:x.y`), **When** installation runs, **Then** it fails as an invalid plugin, because the delivered input would be ambiguous.
5. **Given** a Linker whose key is a private key belonging to a different plugin's namespace, **When** installation runs, **Then** it fails as an invalid plugin.
6. **Given** two Linkers, in the same or in different installed packages, declaring the same link key, **When** they are installed, **Then** both install; sharing a key is not a collision.
7. **Given** a component that declares a runtime that is not available on the machine, **When** installation runs, **Then** it fails with an actionable message naming the runtime and registers nothing.
8. **Given** an Importer restricted to a Starter that is not installed, **When** installation runs, **Then** it succeeds; the restriction is matched only when a Work starts.
9. **Given** the same origin reinstalled under the same alias after its manifest changed an Importer's or Linker's declarations, **When** installation completes, **Then** the registry holds exactly the new declarations for that alias and nothing the previous version declared and the new one dropped.

---

### User Story 2 - Starter-Published Metadata and Links Appear in the First Snapshot (Priority: P1)

The developer runs `work start <PR-link>` against a plugin whose Starter, besides the repository, base branch and start modes, publishes metadata (for example the pull request number) and a link (the pull request address). Work validates what the Starter published and writes it into the Work's first snapshot together with the core state, so the Work exists from its first moment already carrying the context its origin knew.

**Why this priority**: This is the seed of the whole slice — Linkers and Importers consume what is already persisted — and it is the first step of the roadmap demonstration. It is also valuable and testable on its own.

**Independent Test**: With a fixture Starter that returns valid `meta` and `links`, run `work start <argument>` and read the resulting snapshot: both sections hold exactly what the Starter published. With a fixture that returns an invalid key, run it again and confirm `work start` fails and leaves no Work, branch, directory, snapshot or index entry.

**Acceptance Scenarios**:

1. **Given** a Starter response with valid `meta` and `links`, **When** `work start` completes, **Then** the snapshot's metadata section and links section hold those entries, and they were written by the same atomic step that created the Work, never afterwards.
2. **Given** a Starter response with neither, **When** `work start` completes, **Then** both sections are empty and the observable result is identical to F4.
3. **Given** a Starter response with a key that violates the key rules (bad syntax, or a private key outside the Starter's own plugin namespace) or a link whose value is not non-empty text, **When** `work start` runs, **Then** it fails as an invalid Starter response before anything is materialized.
4. **Given** contribution mode, fork mode, and the default new mode, **When** each completes, **Then** each persists the published entries the same way.
5. **Given** the published entries, **When** the Work's provenance is inspected in the index, **Then** each entry is attributed to the Starter that published it, with the operation and the time.
6. **Given** a snapshot written by this slice, **When** a build that predates it reads the Work, **Then** the snapshot is valid and readable; this slice adds no required field.

---

### User Story 3 - An Eligible Linker Discovers a Link When the Work Starts (Priority: P1)

After the Work is created, Work announces that the start has finished. A Linker that subscribed to that moment — and whose Starter restriction, if any, names the Starter that produced this Work — and whose required inputs are all present, is run. It receives only the inputs it declared, may return a value, and Work stores that value as the link for the Linker's key, remembering which component produced it.

**Why this priority**: It is the first extension execution and establishes the eligibility, input projection, and persistence guarantees that Importers reuse. It is roadmap demonstration step 2.

**Independent Test**: Install a fixture Linker that echoes its received input into its own log and returns a fixed value; run `work start` with a matching Starter; confirm the link is persisted with provenance, the log contains exactly the declared inputs, and a second Linker with a non-matching restriction left no trace of running.

**Acceptance Scenarios**:

1. **Given** an eligible Linker that returns a value, **When** `work start` completes, **Then** the Work's links hold that value under the Linker's key, and provenance records the component, the operation, and the time.
2. **Given** an eligible Linker whose discovery returns no value, **When** `work start` completes, **Then** no link changed, nothing is reported as a problem, and the Work is as if the Linker had not existed.
3. **Given** a key already published by the Starter, **When** an eligible Linker returns a value for the same key, **Then** the discovered value replaces it; the last source wins and provenance names the Linker.
4. **Given** a Linker restricted to a different Starter, or missing a required input, or without automatic discovery enabled, or not subscribed to this event, **When** `work start` runs, **Then** it is not executed at all.
5. **Given** a Linker that declares three inputs, one of them optional and absent, **When** it is executed, **Then** it receives the two present inputs and nothing else; the absent optional input is omitted rather than sent empty.
6. **Given** two eligible Linkers with the same key that both return a value, **When** `work start` completes, **Then** exactly one value is stored — the one from the component that runs later in Work's fixed order — and provenance names that component.

---

### User Story 4 - An Importer Brings Artifacts Into the Work Through Exclusive Staging (Priority: P2)

Once discovered links are persisted, Work looks at the Importers. One that needs the link a Linker just discovered is now eligible and runs. It writes what it produces into a private staging location Work gave it; Work checks where everything would land, and only then moves the artifacts into the Work's directory next to the worktree, and cleans the staging up.

**Why this priority**: It completes the automatic pipeline (roadmap demonstration step 3) and delivers the "useful artifacts" half of the feature; it depends on the eligibility and persistence machinery of User Stories 2 and 3, so it follows them.

**Independent Test**: With a fixture Linker that discovers a link and a fixture Importer that requires that link and writes two files, run `work start`; confirm the Importer received the discovered value, both files exist in the Work directory (not inside the worktree), and no staging location remains.

**Acceptance Scenarios**:

1. **Given** an Importer that requires a link a Linker discovers in the same start, **When** `work start` completes, **Then** the Importer ran after the Linker phase, received the discovered value, and its artifacts are in the Work directory.
2. **Given** produced content in subfolders, **When** it is incorporated, **Then** each item keeps its relative path beneath the Work directory, and nothing is placed inside the worktree.
3. **Given** an Importer that produces nothing, **When** it finishes, **Then** the run succeeds and nothing changes.
4. **Given** an Importer whose required input is absent (for example the Linker found nothing), **When** `work start` runs, **Then** it is not executed and nothing is reported.
5. **Given** an Importer restricted to another Starter, or activated only manually, **When** `work start` runs, **Then** it is not executed.
6. **Given** any outcome of an Importer run, **When** `work start` returns, **Then** the Importer's staging location no longer exists.
7. **Given** a Work with imported artifacts, **When** it is archived (F2), **Then** the artifacts are preserved in the archived directory with the snapshot, exactly as any other remaining file.

---

### User Story 5 - Colliding Importer Output Is Refused Entirely (Priority: P2)

Two Importers may want the same file, or an Importer may produce something that already exists. Work never overwrites and never half-incorporates: if any single item of one execution cannot be placed safely, none of that execution's output is incorporated, and the Work directory is exactly as it was before that Importer ran.

**Why this priority**: It is the safety property that lets users trust automatic importing (roadmap demonstration step 4 and exit criterion "no Importer can produce partial incorporation"). It builds on User Story 4's happy path.

**Independent Test**: Install two Importers that both produce the same relative path, plus an Importer that produces one clean file and one colliding file; run `work start`; confirm the first Importer's output is present, the second's is entirely absent, the mixed one contributed nothing at all, and warnings name the component and the colliding relative path.

**Acceptance Scenarios**:

1. **Given** two Importers whose outputs share a relative path, **When** both run in one start, **Then** the one that runs first is incorporated, the other contributes nothing, and the first Importer's files are byte-identical to what it produced.
2. **Given** an Importer whose output collides with a file that already exists in the Work directory, **When** it runs, **Then** none of its output is incorporated.
3. **Given** an Importer that produces one clean file and one colliding file, **When** it runs, **Then** neither is incorporated.
4. **Given** an Importer whose output would land at or under the worktree location or on the snapshot file, **When** it runs, **Then** the whole execution is refused as a collision.
5. **Given** an Importer that produces a symbolic link or any content that is not a regular file or a directory, or content whose destination would fall outside the Work directory, **When** it runs, **Then** the whole execution is refused.
6. **Given** a failure while incorporating (for example the disk becomes full after some items were placed), **When** the failure occurs, **Then** everything that execution had already placed is removed and the Work directory is as it was before.
7. **Given** a destination filesystem that treats names differing only in letter case as the same name, **When** an Importer produces such a pair or matches an existing name that way, **Then** it is treated as a collision.

---

### User Story 6 - An Extension Failure Never Damages the Work (Priority: P2)

Extensions are outside code; they will crash, print garbage, hang, or collide. Because the Work already exists and is usable, an automatic Linker or Importer that fails cannot undo it. `work start` finishes with a warning that says which component failed doing what, keeps everything that is valid, and still delivers the user into the new worktree.

**Why this priority**: It is the non-destructive failure guarantee of the roadmap exit criterion and PRD FR-29 (demonstration step 5). It is P2 because it hardens the pipeline the earlier stories introduce.

**Independent Test**: Install one fixture extension for each failure class (cannot start, non-zero exit, invalid response, refused output) plus one healthy Linker and one healthy Importer; run `work start`; confirm the Work, snapshot, worktree and terminal repositioning are intact, the healthy extensions still ran, and one warning per failure names Work, component, and operation.

**Acceptance Scenarios**:

1. **Given** an automatic Linker that exits non-zero, **When** `work start` runs, **Then** it completes with a warning; the Work, its snapshot and worktree exist and are valid; the exit code is success; the stable result lines are unchanged; the terminal is repositioned into the worktree.
2. **Given** an automatic Linker whose response is not a single structured document, or whose value is not non-empty text, **When** it runs, **Then** the outcome is the same as scenario 1 and nothing from that response is persisted.
3. **Given** an automatic Importer that fails, **When** it runs, **Then** no artifact of that run is incorporated and its staging location is removed.
4. **Given** one failing extension and other eligible ones, **When** `work start` runs, **Then** the others still run.
5. **Given** a Linker that failed and an Importer that required its link, **When** the Importer is evaluated, **Then** it is ineligible and is not executed; that is not a second warning.
6. **Given** an extension that is still running, **When** the user interrupts, **Then** the running extension is stopped, its output is discarded, the remaining automatic steps are skipped, the Work is kept and usable, and the interruption is reported as a warning.
7. **Given** a failure, **When** the user reads the normal output, **Then** they see a plain-language summary and a next step when one is known; the extension's own error text and exit status appear only in explicit diagnostic mode.

---

### User Story 7 - See What Runs, and Get the Same Result Every Time (Priority: P3)

While extensions run the user can see which one is running and what it did; without any eligible extension nothing changes at all. Given the same installed plugins and the same Work inputs, the same extensions run in the same order and produce the same result.

**Why this priority**: Visibility (PRD FR-8, UX §10) and determinism (the roadmap's cross-cutting gate) are properties over the behavior above rather than new capabilities, and they are lowest priority because they need the earlier stories in place to be observable.

**Independent Test**: Install three eligible fixture extensions; run `work start` interactively and non-interactively and compare what is shown; repeat the run repeatedly on a fresh clone each time and compare execution order and resulting snapshots; run once more on a machine with only the reference package and compare with an F4 baseline.

**Acceptance Scenarios**:

1. **Given** an interactive terminal, **When** extensions run, **Then** the user sees, for each, the component and operation currently running and its outcome, on the interface channel and never on the stable result stream.
2. **Given** `NO_COLOR` set, `TERM=dumb`, or redirected streams, **When** extensions run, **Then** the same information appears as plain sequential lines with no control sequences.
3. **Given** the same installed set and the same inputs, **When** `work start` is repeated, **Then** the same extensions run in the same order and the resulting links and artifacts are identical.
4. **Given** a machine with only the reference package, **When** `work start` runs, **Then** no extension is started, nothing extension-related is displayed, and stdout, exit code and created files are identical to F4.
5. **Given** a manifest that tries to influence order (a priority or position field), **When** it is installed, **Then** it is rejected as invalid, as it already is.

### Edge Cases

- **Nothing eligible**: a machine may have Linkers and Importers installed and none eligible for this Starter or these inputs; the phases run over an empty set with no output and no subprocess.
- **A Linker discovers a value that an Importer optionally consumes**: an optional input never gates eligibility, but it is delivered when present at evaluation time — so a link discovered in the Linker phase is delivered to an Importer that declared it optional.
- **A component declares only manual availability**: it is registered (so a later slice can offer it) and is never run automatically.
- **A Linker declares discovery but not `automatic`, or no matching subscription**: it is never run automatically.
- **A `work:` fact absent in this mode**: `work:slug` does not exist in contribution mode; a component requiring it is ineligible there, one declaring it optional simply does not receive it.
- **Entrypoint or runtime disappears after install** (for example a linked development plugin whose file was renamed): that extension fails when it is started, as an ordinary automatic failure with a warning; it does not invalidate the installation.
- **An extension writes noise on its error stream or extra fields in its response**: only the single structured response is read; extra fields are ignored; the error stream is not shown outside diagnostic mode.
- **A Linker response carries a different key than the one it declared**: the key is always the one the manifest declared; nothing the extension names can redirect a write.
- **An Importer produces an empty directory**: it is content; it is created unless a file already occupies that name. A directory that already exists is merged into, provided no file inside it collides.
- **A previously installed package (F4) already declared an Importer or Linker**: its registry entries carry no activation data, so it is inert until the same origin is reinstalled; nothing it declared could have run before this slice.
- **The process is killed mid-pipeline**: the snapshot is a complete valid state (before or after the last step); a leftover staging location is never treated as Work content and never affects a later run.
- **Resuming or archiving an existing Work**: never runs the automatic pipeline; `start:finalized` occurs only when a Work is created.
- **A very long-running extension**: Work imposes no time limit of its own; the user can interrupt (User Story 6, scenario 6).
- **Index update fails after the snapshot was written**: the snapshot is authoritative; the Work stays valid and the index is reconciled later, as for any index failure.
- **`work start` fails or is cancelled before materialization**: no extension is ever run and no published context survives anywhere.
- **Extension output that would put a secret in a diagnostic**: warnings never contain link or metadata values or artifact content.

## Requirements *(mandatory)*

### Functional Requirements

**Extension declarations and registration**

- **FR-001**: A plugin manifest MUST be able to declare Importers and Linkers in the model the architecture documents. An Importer declares its activation — event subscriptions, each with an optional restriction to named Starters, and/or manual availability with display text — and its inputs. A Linker declares its link key and its discovery — whether it is automatic, its event subscriptions with optional Starter restrictions, its inputs — and/or manual availability with display text. Installation MUST accept manifests in that model and MUST reject malformed variants of it, registering nothing.
- **FR-002**: Every event named in a subscription MUST be one Work defines. In this slice the only such event is `start:finalized`; a subscription to any other event MUST fail installation as an invalid plugin. A Starter restriction that names a Starter not currently installed MUST NOT fail installation.
- **FR-003**: Every input MUST have the form `<work|meta|link>:<key>` with an optional `:optional` suffix. A `work` key MUST be one of the facts Work exposes (FR-017); a `meta` or `link` key MUST be a valid key (FR-008). A single component declaring the same key in two input namespaces MUST fail installation.
- **FR-004**: Installation MUST record, for each Importer and Linker, everything needed to decide eligibility and to present the component later — subscriptions with their Starter restrictions, manual availability with its display text, inputs, the Linker's key, and its discovery settings — so that no component ever has to be executed, and no manifest re-read, to learn what it declares.
- **FR-005**: Reinstalling the same origin under the same alias MUST leave that alias's Importer and Linker records equal to what the new manifest declares (extending F4's FR-006a to these records).
- **FR-006**: Several Linkers MAY declare the same key; that is not a collision. A Linker's key MUST be valid under FR-008, and a private key MUST belong to the declaring plugin's own namespace.
- **FR-007**: Components of every role MUST be executed through the runtime their manifest declares, invoked explicitly over the entrypoint and never relying on a shebang or an executable bit, and directly when no runtime is declared. A declared runtime that cannot be found on the machine MUST fail installation with an actionable message naming it, registering nothing.

**Keys, values, and Semantic Conventions**

- **FR-008**: A public key MUST follow the published Semantic Conventions' key grammar; a private key MUST have the form `plugin.<plugin-name>.<key>` and MUST be published only by the plugin it names. Work MUST enforce the grammar and the ownership rule everywhere it accepts a key from a plugin (installation, Starter response, Linker key). Work MUST NOT reject a well-formed public key merely because the published conventions do not list it, and MUST NOT interpret what a key means.
- **FR-009**: A link's value MUST be non-empty text. A metadata value MAY be any structured data value. Work MUST store values exactly as received, without normalization.
- **FR-010**: This slice MUST publish version 1 of the Semantic Conventions as a versioned document: the key grammar, namespace and ownership rules, value-representation rules, the policy for adding keys, and the initial public keys used by the shipped fixtures. Every public key used by any shipped fixture or example MUST be defined in it.
- **FR-011**: The `work` namespace is owned by the core: no extension MAY publish or modify anything in it, and nothing an extension returns MAY reach the core-governed section of the snapshot.

**Starter-published context**

- **FR-012**: Work MUST read the Starter's `meta` and `links` and validate them under FR-008 and FR-009 before any Work is materialized. Any violation MUST be treated as a structurally invalid Starter response, failing `work start` exactly as F4 already fails that class and leaving no branch, worktree, directory, snapshot or index entry.
- **FR-013**: The validated metadata and links MUST be written into the Work's first snapshot by the same atomic step that creates the Work, in their own sections; there MUST be no moment at which a Work exists without the context its Starter published.
- **FR-014**: The Repository Reference MUST remain transient: nothing in it is promoted into metadata or links by the core; only what the Starter explicitly published in `meta` and `links` is persisted.
- **FR-015**: The snapshot's version MUST NOT change and no field MUST become required; a snapshot written by this slice MUST remain valid for the previous slice's readers and validators.

**Event, eligibility, and phases**

- **FR-016**: Once the Work, its core state and its Starter-published context are persisted, Work MUST publish `start:finalized` and run, in order: eligible automatic Linker discovery; persistence of the links discovered; evaluation of Importer eligibility; eligible Importer execution with staging, validation and incorporation. A `work start` that failed or was cancelled before materialization MUST run none of it.
- **FR-017**: The `work` facts Work exposes as inputs are exactly: `worktree_path`, `start_mode`, `branch`, `base_branch`, and `slug`. `slug` is absent in contribution mode. No other core state is exposed.
- **FR-018**: Before starting any subprocess, Work MUST evaluate, statically and without executing the component: that the component's declared activation covers this event — for a Linker, automatic discovery enabled and a matching subscription; for an Importer, a matching subscription — that its Starter restriction, if any, names the Starter that produced this Work, and that every required input is resolvable. A component that fails any check MUST NOT be executed and MUST NOT produce a warning. Optional inputs MUST NOT participate in eligibility.
- **FR-019**: Importer eligibility MUST be evaluated after the Linker phase's results are persisted, so an Importer can consume a link discovered in the same start.
- **FR-020**: Eligible components MUST run one at a time. The order within a phase MUST be fixed and reproducible for a given installed set — ordered by the component's qualified name — and MUST NOT be influenceable by the manifest, by installation order, or by any priority. Plugin authors MUST NOT be given any guarantee about order within a phase.

**Input projection**

- **FR-021**: An extension MUST receive exactly one input document containing only the declared inputs that are present, keyed by their key with the namespace not repeated, plus — for an Importer — its staging location. The subscription, the Starter restriction, and eligibility data MUST NOT be passed, and no other Work state, links, or metadata MUST reach the extension.
- **FR-022**: Extensions MUST NOT be given the snapshot or its location as part of the contract, and MUST NOT be able to change it through it; the core alone writes the snapshot.

**Linkers**

- **FR-023**: An automatic discovery MAY return one value or nothing. A value MUST be stored as the link for the Linker's declared key, replacing any earlier value (last source wins), regardless of whether the earlier value came from the Starter or another Linker. No value MUST leave everything unchanged with no error and no warning. A value that is not non-empty text is an automatic-extension failure (FR-035).
- **FR-024**: Discovered links MUST be persisted before any Importer is evaluated; every persisted update MUST be atomic; failure to persist is an automatic-extension failure of that Linker.
- **FR-025**: For every entry published by a Starter or discovered by a Linker, Work MUST record provenance — producing component, operation, time — separately from the key and value, so that it never becomes part of a key's identity.
- **FR-026**: When two Linkers sharing a key both return a value in one start, the value stored MUST be that of the one that runs later in the fixed order (FR-020), and provenance MUST name it.

**Importers and staging**

- **FR-027**: Each Importer execution MUST receive its own new, empty, exclusive staging location outside the Work, and it MUST be the only location the Importer is given to write into.
- **FR-028**: After a successful execution and a valid response, Work MUST enumerate everything produced, compute each item's destination — its relative path beneath the Work directory — and validate every destination before changing the Work at all.
- **FR-029**: Work MUST refuse the entire execution, incorporating nothing, when any of these holds: a produced file's destination already exists; a destination is at or under the worktree location or is the snapshot file; a produced item is not a regular file or a directory; a destination would fall outside the Work directory; or two names collide under the destination filesystem's own naming rules, including differences of letter case where that filesystem ignores them.
- **FR-030**: When validation passes, incorporation MUST be all-or-nothing: if it fails part-way, everything that execution had already placed MUST be removed. Existing content MUST NOT be overwritten, altered, or deleted under any outcome.
- **FR-031**: The staging location MUST be removed after every outcome — success, failure, refusal, interruption. A leftover from a killed process MUST NOT be mistaken for Work content and MUST NOT affect a later run.
- **FR-032**: An execution that produced nothing MUST succeed and change nothing.
- **FR-033**: Because Importers run one at a time, the validation of a later Importer MUST see what an earlier Importer incorporated.

**Failure semantics**

- **FR-034**: No time limit is imposed by Work on an extension; the user can interrupt.
- **FR-035**: For an automatic extension, each of these MUST be an automatic-extension failure: cannot be started (missing entrypoint, missing runtime, not executable); exits unsuccessfully; returns anything other than the single structured response its contract allows or a response with a wrong shape or type; produces output refused under FR-029; fails to persist or incorporate; is interrupted. In every case the extension's effects MUST be entirely absent — no link changed, no artifact incorporated — and Work MUST continue with the remaining components.
- **FR-036**: An automatic-extension failure MUST NOT roll back, remove or invalidate the Work. `work start` MUST still complete: the Work, its snapshot and worktree remain valid, the stable result lines are unchanged, the exit is a success, and the terminal is repositioned into the new worktree.
- **FR-037**: Each failure MUST produce exactly one warning on the interface channel — never on the stable result stream — identifying the Work, the component (qualified name), and the operation, with a plain-language summary and a next step when known. A warning MUST NOT contain link values, metadata values, or artifact content. The extension's own error text and exit status MUST be available only in explicit diagnostic mode.
- **FR-038**: A user interruption during the automatic phases MUST stop the running extension, discard its output, skip the remaining automatic steps, keep the Work usable, and be reported as a warning; the terminal MUST still be repositioned into the Work.
- **FR-039**: A component that becomes ineligible only because an earlier component failed to supply an input MUST simply not run, with no additional warning.

**Visibility**

- **FR-040**: While each extension runs, Work MUST show which component and operation is running and its outcome, on the interface channel. In an interactive terminal this may be a live indicator; in non-interactive use, with `NO_COLOR` non-empty, with `TERM=dumb`, or with redirected streams, it MUST be plain sequential lines with no control sequences.
- **FR-041**: When no component is eligible, Work MUST start no extension process and display nothing about extensions.
- **FR-042**: The terminal MUST be repositioned into the new worktree only after the automatic phases have ended, whatever their outcome.

**Integrity and contract preservation**

- **FR-043**: Every snapshot update in the pipeline MUST be atomic; at any instant, including after the process is killed, the snapshot MUST be a complete valid earlier or later state.
- **FR-044**: A failure to update the global index MUST NOT invalidate the snapshot; the index is reconciled from snapshots later. Rebuilding the index from snapshots MUST restore every Work's core data, metadata and links; provenance is operational data held in the index and MAY be lost by a rebuild.
- **FR-045**: All F1, F2, F2.5, F3 and F4 command grammar, transaction guarantees, stable stdout lines, error tokens and exit codes MUST be preserved. On a machine with only the reference package, `work start` MUST behave identically: same stdout, same exit code, same files, and no extension activity.
- **FR-046**: No new stable stdout line, flag, or exit code MUST be introduced for extension outcomes; they are reported only through the interface channel (FR-037, FR-040). New diagnostic categories for the failure classes of FR-035 and for the key and manifest rejections are defined during planning, consistent with the existing error taxonomy.

### Key Entities

- **Extension (Linker or Importer)**: A component that runs after a Work is created. Declares when it may run (subscriptions, optional Starter restriction, manual availability) and what it needs (inputs). Never chooses when or whether it is executed; Work decides, statically.
- **Event**: A lifecycle moment defined by Work, in the form `<command>:<event>`. In this slice only `start:finalized`, published after the Work and its core state are materialized.
- **Eligibility Decision**: Work's static, no-execution determination of whether an extension may run for this Work: activation, subscription, Starter restriction, and required inputs.
- **Input / Work Fact**: A declared piece of data an extension asks to receive, from the core-owned `work` namespace, from metadata, or from links. Delivered by key, alone, without its namespace.
- **Link**: A first-class external relation with one current value per key; every publication is an upsert and the latest source wins.
- **Metadata**: Extensible data published by components, distinct from Work state and from links.
- **Semantic Convention**: A public, versioned contract naming a key, its meaning and its value representation, forming an open set rather than a list built into the core; private keys carry the plugin's own namespace.
- **Provenance Record**: Operational attribution of a published or discovered entry — component, operation, time — held by the core apart from the entry's key and value.
- **Staging Area**: The exclusive, temporary location one Importer execution writes into; a data boundary, not a security sandbox.
- **Artifact**: A file or directory an Importer produced that Work incorporated into the Work directory, without special meaning to the core.
- **Extension Warning**: The single non-fatal report of an automatic-extension failure, naming Work, component and operation.

### Scope and Dependencies

- Builds on delivered F1 (`specs/001-first-local-work/`), F2 (`specs/002-daily-cycle/`), F2.5 (`specs/003-terminal-ux-revamp/`), F3 (`specs/004-local-clone-locator/`) and F4 (`specs/005-plugin-origins/`): installation and the registry, Starter matching and its response, the resolution pipeline, snapshot writing, the index, archiving, and the presentation contract are consumed unchanged unless a requirement above says otherwise.
- Product authority is `docs/prd.md` FR-8, FR-26 to FR-29, FR-32 and FR-35 (F5 as listed in `docs/roadmap.md`), with FR-27 for input projection and FR-33/FR-34 for snapshot and index integrity. ADR-0000 governs the external-process execution model; ADR-0006 governs the runtime and invocation model; ADR-0012 governs the component model, manifest and phases; ADR-0013 governs namespaces and persisted state. `docs/add/add-0001-work-system-architecture.md` §3, §4, §9, §10 and §11 describe their realization.
- **Roadmap demonstration coverage**: step 1 (Starter publishes metadata and a link) is User Story 2; step 2 (Linker discovers and the core upserts) is User Story 3; step 3 (Importer consumes a discovered link through staging) is User Story 4; step 4 (second Importer collides, nothing incorporated) is User Story 5; step 5 (automatic extension fails, Work stays usable) is User Story 6.
- **Prerequisites from the roadmap**: a first published version of the Semantic Conventions (`docs/roadmap.md` §5) did not exist in the repository; this slice delivers it (FR-010).
- **Gaps found in earlier slices that this slice must close**, because it is the first to depend on them: the manifest model for Importers and Linkers that installation accepts differs from the documented one (FR-001); the registry does not keep activation data although F4's specification assumed it did (FR-004); a component's declared runtime is neither used to invoke the component nor checked at install (FR-007, which F4's FR-008 already required).
- Every earlier slice's automated end-to-end demonstration must remain green.

### Out of Scope

- `work link`, `work import` and `work status` — manual association, manual import and inspection are F6. Manual availability is only *declared and recorded* here so F6 needs no second installation pass.
- `work plugin enable|disable|update|uninstall` and the `work plugin` hub — F7. Every plugin installed under F4/F5 is available once installed.
- New events beyond `start:finalized`, parallel execution of extensions, retries, dependency ordering between components of one phase, per-extension timeouts, and streaming progress from extensions — deferred by the architecture for v1.
- Formal plugin signing (ADR-0008) and treating staging as a security sandbox: plugins remain processes run with the user's permissions.
- Any official integration (GitHub, GitLab, an issue tracker): fixtures may simulate them, but no integration becomes a dependency of the core contracts.
- Consuming or acting on Starter-published data beyond storing it (for example rendering links in `work status`, or navigating to them) — F6.
- Changing `work start <path>`, `work start <name-or-reference>`, or any established command grammar, flag, stdout line, error token, or exit code.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A package written and installed entirely outside the core — declaring a Starter, a Linker and an Importer — drives `work start` to a Work whose snapshot holds the Starter-published and the Linker-discovered links and whose Work directory holds the Importer's artifacts, with zero core code changes required.
- **SC-002**: In 100% of test runs, an extension that is ineligible — by restriction, subscription, discovery setting, or a missing required input — is never started (verified by fixtures that record every launch).
- **SC-003**: In 100% of test runs, an extension receives exactly its declared and present inputs and nothing else (verified by a fixture that echoes what it received).
- **SC-004**: Across every collision, refusal, failure and interruption scenario, 0 runs leave a partially incorporated Importer output, and after any refused or failed Importer the Work directory is byte-identical to its state before that Importer ran.
- **SC-005**: In 100% of automatic-extension failure scenarios, `work start` ends with a usable Work — valid snapshot, present worktree, terminal repositioned — a success exit code, and exactly one warning per failure that names the Work, the component and the operation.
- **SC-006**: A Starter response with an invalid key or value leaves zero branches, worktrees, Work directories, snapshots or index entries in 100% of test fixtures.
- **SC-007**: Repeating `work start` 20 times against the same fixtures and installed set produces the same execution order and equivalent resulting links and artifacts in 100% of repetitions.
- **SC-008**: All existing F1, F2, F2.5, F3 and F4 contract and integration tests pass without modification, and a machine with only the reference package shows no extension process, no extension output and unchanged stdout and exit codes.
- **SC-009**: Every public key used by a shipped fixture or example is defined in the published Semantic Conventions v1 (100%), and every manifest declaration rule of FR-001 to FR-007 has at least one fixture that violates it and is rejected with nothing registered.
- **SC-010**: No warning or diagnostic in normal output contains a link value, a metadata value, artifact content, or an extension's raw error text (0 occurrences across all failure fixtures); the raw text appears only in explicit diagnostic mode.
- **SC-011**: The roadmap's five-step demonstration for F5 runs as one automated end-to-end journey and passes, alongside the previous slices' demonstrations.

## Assumptions

- F1–F4 are delivered. F4's Starter selection, response validation, resolution pipeline, materialization (including the contribution path) and the installation pipeline are reused unchanged except where a requirement above extends them.
- **Defaults chosen for questions the source documents leave open.** None blocks specification; each is a recorded default and is listed here so it can be overturned during clarification:
  - **D1 — Semantic Conventions v1 is a deliverable of this slice.** The roadmap makes it a prerequisite of F5 and none exists. The core enforces only key grammar and private-namespace ownership (FR-008); it holds no closed list of public keys (ADR-0013), so a well-formed key absent from the document is accepted.
  - **D2 — Exposed `work` facts are `worktree_path`, `start_mode`, `branch`, `base_branch` and `slug`.** The architecture names only the first two as examples and asks for a carefully defined set; these are the minimum a discovery or import realistically needs. Identifiers, timestamps, status and the convention are not exposed. An unknown `work` key fails installation so a plugin that needs a newer Work is refused early rather than silently ineligible.
  - **D3 — An automatic-extension failure exits with success.** PRD FR-29 says `work start` completes with a warning; a distinct non-zero exit would signal a broken start for a Work that exists and is usable. Automation that needs to know reads the warning on the interface channel; no new stable stdout line or exit code is added (FR-046).
  - **D4 — Provenance lives in the index only, as the architecture states.** It is not part of the snapshot, so an index rebuild restores everything canonical but may lose provenance (FR-044). Putting provenance in the snapshot would need a new snapshot version and contradicts ADR-0013's decision to keep it operational.
  - **D5 — Imported artifacts go beside the worktree, never inside it.** The Work directory holds the worktree and the snapshot; the worktree and the snapshot are reserved (FR-029), which keeps generated context out of the tracked tree as the PRD's motivation requires. Only regular files and directories are accepted (no symbolic links) because links can escape the Work directory and behave differently across supported operating systems.
  - **D6 — Automatic discovery needs both `automatic` and a matching subscription.** The architecture describes `discover.automatic` ("requested by Work") and `discover.on[]` ("event-driven") without saying whether they are alternatives; in this slice the only trigger is an event, so both must hold. Whether `automatic` alone gains a meaning later is a later slice's call.
  - **D7 — Order inside a phase is by qualified component name, and the later Linker wins a shared key.** The architecture guarantees no order but the roadmap requires determinism and forbids hidden priority; a fixed order that authors can neither influence nor rely on satisfies both.
  - **D8 — Honoring the declared runtime is in scope.** It is the same invocation path every extension uses, F4 required it and did not deliver it, and this slice is the first where interpreted Linkers and Importers are the expected case.
  - **D9 — A user interruption during the automatic phases keeps the Work.** The Work already exists and is usable; the interruption is treated like a failed extension (warning, success exit, terminal repositioned).
  - **D10 — Extensions installed under F4 with Importer/Linker declarations stay inert until reinstalled.** They could never have run and carry no activation data; reinstalling the same origin refreshes them.
- The looser Importer/Linker declaration shapes the F4 validator happened to accept were never part of a documented contract and were never executed; the documented model of FR-001 replaces them.
- Every installed plugin is enabled; enable/disable arrives with F7. Eligibility therefore never has to consider a disabled plugin in this slice, though it is written so it can.
- Extensions run sequentially and are not sandboxed; Work imposes no timeout of its own (architecture §11).
- Extension diagnostics follow F2.5: a plain-language summary and hint in normal output; technical causes only in explicit diagnostic mode.
- Metadata and link values are stored as received; keeping secrets out of what a plugin publishes is the plugin author's responsibility, and Work's own diagnostics never repeat them.
- Component execution follows ADR-0000 and ADR-0006: one structured input document on standard input, at most one structured response on standard output, an exit status, and free-form standard error; no new transport is introduced.
- No critical product, security, or privacy decision is left unresolved for specification; planning-stage prototyping may refine exact message wording, diagnostic category names and the registry's field layout without weakening these outcomes.
