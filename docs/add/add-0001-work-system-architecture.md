# ADD-0001: Work System Architecture

**Status:** Draft — target architecture for feature 009; implementation pending

**Date:** 2026-11-07

**Marketplace decisions:** ADR-0022–ADR-0030 govern the target distribution model and supersede the older decisions only in their declared scopes.

**Decisions governing unchanged behavior:** ADR-0000 (plugin execution), ADR-0002 (installation and registration), ADR-0003 (bootstrap), ADR-0004 (Starter collision), ADR-0005 (updates), ADR-0006 (runtime), ADR-0008 (signing), ADR-0009 (stack), ADR-0011 (convention resolution), ADR-0012 (component model and manifest), ADR-0013 (namespaces and persisted state), ADR-0014 (source vs. location separation), ADR-0015 (resolution policy), ADR-0016 (Repository Reference and Git endpoints), ADR-0019 (CLI surface), ADR-0020 (presentation boundary), and ADR-0021 (full-screen lifecycle).

**Requirements satisfied:** `docs/prd.md` — FR-1 through FR-83, NFR-1 through NFR-11. This document implements the requirements and decisions listed here; incompatibilities must be resolved in the PRD or in an ADR, never by silent design drift.

***

## 1. Component overview

The system has three process types:

* **Work core binary** — interprets commands, coordinates static and interactive presentation, maintains local state, and controls the lifecycle.
* **`git`** — tool invoked by the core for worktrees and branch resolution.
* **Plugin processes** — short-lived subprocesses, one per executable operation; the core never loads their code into its own process.

State lives in `~/.work/` (plugins, configuration, and the SQLite index) and in the workspace directory chosen by the user for each Work. The core creates only the worktree and `work-state.json`; any other file or directory may be incorporated by Importers, with no special meaning to the core.

***

## 2. Concrete stack

As defined by ADR-0009:

* **Language:** Go, as a single portable binary.
* **CLI:** Cobra.
* **Interactive presentation:** Bubble Tea and Lip Gloss behind the generic boundary defined in ADR-0020; Huh may be used internally for simple fields without defining the journey boundary.
* **Persistence:** `work-state.json` is the canonical snapshot for each Work; embedded SQLite with a pure-Go driver, under `~/.work/state/work.db`, is its global query projection.

***

## 3. Directory layout and core state

The workspace directory is chosen by the user on the first use that requires it to exist and is stored in configuration. The core may suggest a default path, but the location is not fixed and is not part of a Work's identity. Later configuration changes affect new Works and do not automatically move existing ones. The structure of each Work is:

```text
<workspace>/
  in-progress/<repo-name>_<branch-name>/
    worktree/
    work-state.json
  archived/<yyyymmdd>-<repo-name>_<branch-name>/
    work-state.json
    # the worktree was destroyed; Importer artifacts, if any, are preserved
```

`work-state.json` is the canonical, self-contained snapshot of each Work. It contains the semantic sections `work`, `meta`, and `links` in a single physical file:

```jsonc
{
  "schema": 3,
  "work": {
    "slug": "my-work",
    "status": "in-progress",
    "start_mode": "fork",
    "starter": "github-pull-request-starter",
    "branch": "feature/my-work",
    "base_branch": "main",
    "branch_convention": "gitflow",
    "created_at": "2026-08-29T00:00:00Z",
    "last_accessed_at": "2026-08-29T00:00:00Z"
  },
  "meta": {
    "github.pull_request.number": 212
  },
  "links": {
    "github.pull_request": "https://github.com/example/project/pull/212"
  }
}
```

`work` contains only state governed and versioned by the core; plugins do not create or write its properties. `meta` contains extensible data published by components, and `links` contains first-class external relations. `branch_convention` is omitted in contribution mode. The resolved repository path from the Starter and the current checkout do not use an ambiguous persisted name; when a component needs to access the checkout, the core may expose the unambiguous `work:worktree_path` key as input.

Each link key has a single current value, and every publication is an upsert: the latest source wins. Operational provenance (`source_component`, `source_operation`, `recorded_at`) is kept by the core in the index, separate from the semantic key and value. The file is updated via temporary write and atomic rename. `work.db` is a projection/index for global queries such as listing and ordering; it can be rebuilt or reconciled from snapshots and is not the sole source of truth for a Work.

***

## 4. Plugin package and manifest

`plugin.json`, at the package root, statically describes executable components and declarative conventions:

```jsonc
{
  "name": "github-plugin",
  "version": "1.2.0",
  "components": [
    {
      "name": "github-pull-request-starter",
      "role": "starter",
      "pattern": "^https://github\\.com/[^/]+/[^/]+/pull/\\d+$",
      "entrypoint": "starter.py",
      "runtime": "python3"
    },
    {
      "name": "filesystem-repository-locator",
      "role": "repository-locator",
      "display_name": "Filesystem repositories",
      "description": "Finds local Git repositories in configured search roots",
      "accepts": ["git_fetch_urls", "name", "query"],
      "entrypoint": "locator.py",
      "runtime": "python3"
    },
    {
      "name": "github-pull-request-importer",
      "role": "importer",
      "on": [{
        "event": "start:finalized",
        "starters": ["github-pull-request-starter"]
      }],
      "manual": {
        "display_name": "Pull Request Context",
        "description": "Imports the artifacts associated with the pull request"
      },
      "inputs": ["link:github.pull_request", "work:start_mode:optional"],
      "entrypoint": "importer.py",
      "runtime": "python3"
    },
    {
      "name": "github-pull-request-linker",
      "role": "linker",
      "key": "github.pull_request",
      "discover": {
        "automatic": true,
        "on": [{
          "event": "start:finalized",
          "starters": ["github-pull-request-starter"]
        }],
        "inputs": ["work:worktree_path"]
      },
      "manual": {
        "display_name": "GitHub Pull Request",
        "description": "Links the Work to a GitHub pull request"
      },
      "entrypoint": "linker.py",
      "runtime": "python3"
    }
  ],
  "conventions": [
    {
      "name": "gitflow",
      "prefixes": ["feature/{slug}", "release/{slug}", "hotfix/{slug}"]
    }
  ]
}
```

The package remains the atomic unit for installation, update, enablement, and removal (ADR-0012). Under ADR-0024, package identity is the structured pair `(marketplace, plugin)` rendered as `plugin@marketplace`; component and convention identity adds the contribution name, rendered `plugin@marketplace/contribution`. Contributions with equal names remain distinct records. Optional marketplace display names never participate in identity. Source registration instances distinguish retained memory/private scope across namespace removal and reuse; public selectors stay unchanged. Bare contribution selection is allowed only when unambiguous; plugin selection always specifies its marketplace.

### 4.1 Validation by role

`role` is required and is the component's semantic discriminator.

| Role | Required | Allowed | Invalid |
| --- | --- | --- | --- |
| `starter` | `name`, `role`, `entrypoint` | `pattern`, `runtime` | `on`, `manual`, `inputs`, `key`, `discover` |
| `importer` | `name`, `role`, `entrypoint`, at least one of `on` or `manual` | `on`, `manual`, `inputs`, `runtime` | `pattern`, `key`, `discover` |
| `linker` | `name`, `role`, `key`, `entrypoint`, at least one of `discover` or `manual` | `discover`, `manual`, `runtime` | `pattern` |
| `repository-locator` | `name`, `role`, `entrypoint`, non-empty `accepts` | `runtime`, `display_name`, `description` | `pattern`, `on`, `manual`, `inputs`, `key`, `discover` |

`repository-locator.accepts` contains only `git_fetch_urls`, `name`, and `query`. `path` is not accepted because its presence makes the core validate the path directly, without executing Locators. `conventions[]` remains outside `components[]`; its entry contains only `name` and `prefixes[]`. Incompatible fields make installation or update invalid. There is no `invocation`, `type`, `capabilities`, `hooks`, `priority`, `score`, or recommended position field.

### 4.2 Data namespaces and Semantic Conventions

`inputs[]` uses the grammar `<source>:<key>[:optional]`, where `source` is `work`, `meta`, or `link`. `work:<key>` accesses only the state surface exposed by the core; `meta:<key>` accesses extensible metadata; and `link:<key>` accesses a first-class external relation. The namespace is used for resolution and eligibility, but is not repeated in the projected payload.

Public `meta` and `link` keys follow Work Semantic Conventions: they define namespace, key, meaning, and value representation without forming a closed binary whitelist. Data without shared semantics uses a private namespace bound to the composed provider by ADR-0024. Manifest and subprocess self-relative private keys are expanded to the canonical provider scope for persistence and lookup, then projected using the declared input keys. The exact escaped grammar and ownership rules are in feature 009 contracts; neither homonymous providers nor distinct registration instances share private ownership. Public keys remain provider-independent. The key identifies the meaning of the data, not its producer: multiple Linkers may declare the same `key` without creating a namespace collision.

***

## 5. Marketplace distribution, installation, and registration

**Governing decisions:** ADR-0022 (catalog), ADR-0023 (snapshots/copies), ADR-0024 (identity), ADR-0025 (surface), ADR-0026 (removal), ADR-0027 (administration). Satisfies PRD FR-16–FR-21, FR-35, FR-40–FR-48, FR-65–FR-77, FR-81–FR-82.

```text
~/.work/
  plugins/<encoded-marketplace>/<encoded-plugin>/
    source/plugin.json
    source/<package-files>
    .install-meta.json
  config/work.json                    # source registrations and ordered policy
  state/registry.json                 # structured providers and owned declarations
  state/branch_conventions.json       # actual provider/registration-bound choices
  state/marketplace-catalogs.json      # fixed discovery metadata and capture references
  state/marketplace-snapshots/<hex>/   # local distribution bytes; never execution roots
  state/reference-package.json        # durable initialization/removal history
  state/administration.json           # current schema, stable/pending epoch
  state/administration/<id>/journal.json
  state/work.db                       # Work projection, not administrative storage
```

The distribution catalog has a published namespace, optional passive display name, and explicit entry names/relative package directories. Each package is self-contained and includes its static plugin manifest. Package paths cannot escape or overlap. Entire offers are validated without executing plugins; runtimes are checked when a selected package is installed. Catalog schemas, safe names, canonical digests, vectors, and source rules are defined by [feature 009 contracts](../../specs/009-marketplace-distribution/contracts/marketplace-catalog.md).

`marketplace` resolves typed Git/local/distribution providers. Git fixes the selected commit and recorded branch; local sources capture distributable content with pre/post content validation; the included provider reads binary resources. `marketplaceadmin` prepares registration, refresh, inspection, and whole-source removal plans. `pluginadmin` consumes resolved entries and owns installation, replacement, toggle, update, and removal policy. `plugininstall` validates/copies already resolved package content; no public service may publish arbitrary standalone sources or link new packages. CLI and manager callbacks share these services; presentation receives safe data and opaque identities, never Git/storage access.

Registering a source and its first fixed catalog is one publication. Repeating identical registration is a no-op, not refresh. Names come from validated catalogs; conflicting provenance and replacement of the included provider fail. Refresh changes discovery only. Local installation uses captured bytes; Git installation obtains the exact browsed commit if needed and fails without falling forward. Installed copies do not depend on source/capture availability for execution.

A package registration contains structured identity, actual source provenance, opaque version, package content digest, enablement, and provider-owned declarations. Components, conventions, private scope, policy, memory, subscription matching, dispatch, and revision baselines carry that identity throughout. Rendering the ID is not an identity parser or filesystem path. Genuine exclusive-contribution conflicts remain domain errors.

### 5.1 Administrative storage and services

Under ADR-0027 and ADR-0030, use one current strict administrative model with catalog state, source configuration, composed registry/memory and typed package addresses. The CLI is unreleased: earlier development formats, plugins and command behavior may break. Replace existing shapes directly; there are no dual readers, conversions, adoption paths, old-journal handlers or old-executable fences.

One administration lock protects finite publication. Journal order remains prepared → pending epoch → owned moves and JSON images → committed marker → stable epoch → cleanup. The finite image allowlist includes catalogs; move destinations use separately validated marketplace/plugin segments only. Current prepared recovery restores before-images and committed recovery completes after-images. Ordinary malformed/unsupported current state fails validation without conversion or deletion.

Publish complete local captures before referencing them. Capture/staging leases protect every in-flight plan from orphan collection; cleanup removes only unreferenced, unleased owned artifacts under administration. Slow capture, Git, runtime preparation, and user interaction happen outside the publication lock. Plans close idempotently after success, error, Back, or cancellation. Explicit update checks use temporary storage outside Work home and do not persist catalogs, leases, timestamps.

Pure reads accept matching stable epochs with at most three attempts; they never acquire locks, recover, bootstrap, open the Work database, create state, or perform network access. A valid explicit mutation performs recovery preflight, prepares outside the lock, then revalidates affected facts against current state. Independent changes merge; drift in reviewed source/targets/candidate/dependencies conflicts without expanding authorization.

Invocation validates composed provider revision/declarations/eligibility under administration and retains protection through subprocess completion. Administration is released before acquiring Work/branch locks; publication after execution revalidates current facts without lock inversion. Package copies, references, and catalog refresh do not alter role/IPC semantics.

### 5.2 Maintenance and removal

ADR-0023 preserves opaque-version inequality and immutable reviewed candidates. Update resolves one current revision per source/branch group from recorded actual provenance; all targets in that group use the same revision. Discovery freshness or absence does not redirect maintenance. Equal valid versions are unchanged; invalid content still fails. Explicit add from a fixed offer can reinstall changed bytes at the same version, preserving enablement with exact dependency review. Identical content is a no-op.

ADR-0026 governs removal. Plugin removal publishes the selected installation set and authorized dependencies without removing sources/siblings. Marketplace removal reviews its registration/catalog and every installed package from local inventory, including disabled and withdrawn entries, without contacting the source. Configuration, catalogs, copies, registry, and authorized policy withdrawal commit together. Works and convention memory are preserved. Relevant set/provenance drift conflicts; the namespace is reusable only after commitment, without historical reassociation.

### 5.3 Clean pre-release cutover

ADR-0030 realizes PRD FR-79: support one marketplace-only identity, source and storage contract. Delete standalone/link/embedded-mode publishing APIs, alias variants, compatibility selectors/readers/adapters, adoption/migration flows and compatibility-only fixtures. Update active callers/examples/tests directly to the intended design. Developer validation starts from fresh disposable homes; no reset tool or real-home deletion is authorized by this feature.

Preserving marketplace-created Works and source-bound remembered/private state across ordinary removal remains current product behavior. Source registration IDs distinguish replacement origins even when public names repeat; they are provenance, not a previous-format bridge. No prior development Work schema or identifier representation imposes a storage-preservation requirement.

***

## 6. Included offer, bootstrap, and official online publications

**Governing decisions:** ADR-0028 (included provider/default lifecycle), ADR-0029 (online offer), ADR-0022–ADR-0024 (common entry/copy/identity). Satisfies PRD FR-10, FR-24, FR-41, FR-78–FR-80.

The binary provides reserved marketplace `builtin`, initially offering `work-reference` with a fallback Starter, filesystem Locator, and freeform convention. The provider obtains its catalog/packages from embedded distribution resources; an extracted editable catalog or author-supplied field cannot redefine its authority. `builtin` is visible without initializing home and cannot be removed or replaced; individual installed plugins remain removable and toggleable.

Current lifecycle records initialized, policy_handled and removed. Source registration does not handle policy. Explicit policy choice, including empty, sets policy_handled in the same transaction; config-file existence is not intent. Required first setup seeds once only while policy remains unhandled.

The first command requiring genuinely fresh default setup resolves `work-reference@builtin` through the common entry/copy/registry transaction and initializes the qualified default policy once. Durable initialization, policy handling, and deliberate-removal history remain outside removable storage. Explicit add marks setup/policy handled without inserting Locators. Repair, reinstall, reads, catalog refresh, and new binary offers never reseed edited/empty policy or silently replace installed copies. Current disablement/removal is preserved; earlier development installations are outside the supported contract.

`work` is a separate online Git catalog with publication independent of Work releases. It is explicitly registered and follows normal branch/snapshot/refresh/update and whole-source removal. Its owner-confirmed publication origin is a release prerequisite, not an inferred URL or a reserved-name trust signal. No automatic online registration or resurrection occurs. The included provider's lifecycle does not confer protection against same-user file modification; installed copies share the existing trust boundary.

***

## 7. Starter resolution and Work creation

In `work start [source]`, the absence of `source` in an interactive terminal opens collection in the TUI; in non-interactive mode, it fails with actionable usage guidance. Once the value is obtained, specific Starters (`pattern` present and non-empty) are evaluated locally against the argument. A single match is used; multiple matches are chosen through the TUI; no match falls back to the single enabled fallback. Collision is prompted each time it occurs. The absence of a fallback produces an actionable error that instructs the user to enable or install a Starter.

The Starter receives:

```json
{ "arg": "https://github.com/example/project/pull/212" }
```

On success, it returns the data it managed to resolve. Core fields are explicit; integration data goes into `meta` and external relations into `links`:

```jsonc
{
  "repository": {
    "git_fetch_urls": [
      "https://github.com/example/project.git",
      "git@github.com:example/project.git"
    ],
    "name": "project"
  },
  "base_branch": "feature/source-branch",
  "start_modes": ["contribution", "fork"],
  "meta": { "github.pull_request.number": 212 },
  "links": {
    "github.pull_request": "https://github.com/example/project/pull/212"
  }
}
```

`repository` fields are independent and optional: `path` is a local path already resolved; `git_fetch_urls` are known Git endpoints for fetch; `name` is a known logical name, potentially ambiguous; and `query` is opaque text intended for local mechanisms. The response must contain `path` or information that makes at least one configured Locator eligible. The reference is transient: it is not automatically promoted to `work`, `meta`, or `links`.

The absence of `start_modes` means a new Work. When present, the core offers exactly the modes returned and persists the chosen mode in `work.start_mode`; when absent, it persists `new`. Branch convention is neither an input nor an output of the Starter. Failure of the chosen Starter subprocess or structurally invalid output fails `work start`; there is no `matched: false` result or equivalent.

### 7.1 Repository Resolution

The pipeline is:

```text
argument
  → Starter
  → Repository Reference
  → direct path validation or chain of Repository Locators
  → validated repo_path
  → normal Work creation
```

When `repository.path` exists, the core directly validates that the path exists, is accessible, and identifies a usable Git repository. An invalid path ends `work start`; Locators and the other fields are not fallback. When there is no `path`, the core walks the global Repository Resolution Policy in declared order. A Locator participates only if it still exists, its plugin is enabled, and at least one field from `accepts` is present in the reference.

The core projects to the subprocess only the accepted and present fields, plus the configured search roots:

```jsonc
{
  "repository": {
    "git_fetch_urls": ["https://github.com/example/project.git"],
    "name": "project"
  },
  "repository_roots": ["/home/user/src", "/home/user/projects"]
}
```

The Locator returns only candidates:

```json
{ "matches": [{ "repo_path": "/home/user/src/example/project" }] }
```

There is no `confidence`, `score`, `priority`, `winner`, or aggregation across Locators. `matches: []` continues to the next eligible Locator; process or protocol failure interrupts resolution; returned candidates are all validated and deduplicated by the core. One valid candidate completes resolution; multiple valid candidates are shown to the user and stop the chain. If all candidates from a Locator are invalid, resolution fails with diagnostics.

`git_fetch_urls` is not a universal canonical identity. Locators that support them compare fetch endpoints across all local remotes, not only `origin`, and prefer native Git operations to query URLs and their rewrites. The core does not assume equivalence between SSH, HTTPS, usernames, protocols, paths, or `.git` suffixes.

### 7.2 Policy, Locators, and search roots

The global configuration keeps the declarative policy and the search roots separate from the workspace root:

```jsonc
{
  "workspace": "/home/user/.workspaces",
  "repository_roots": ["/home/user/src", "/home/user/projects"],
  "repository_resolution": {
    "locators": ["work-reference@builtin/filesystem-repository-locator"]
  }
}
```

`work repository` provides a TUI for the same operations as the direct commands. `work repository policy list` shows the effective sequence, including unavailable references; `policy add`, `remove`, `move`, and `replace` modify it. `add` accepts optional positioning with `--before` or `--after`; `move` requires exactly one of them; `replace` replaces the full sequence. `work repository locator list` shows all installed Locators and their state. `work repository root list`, `add`, `remove`, and `replace` manage search roots. Removing from the policy leaves the component installed and enabled, only outside the strategy; disabling is a plugin state and preserves the policy reference for possible reactivation.

The official filesystem Locator searches only the configured roots, with depth limited by its own configuration. It does not use `workspace/in-progress` or `workspace/archived` as a repository catalog. It may use `name`, `query`, and `git_fetch_urls`, returns all matches, and does not depend on an external service.

***

## 8. Branch convention

Each enabled package contributes its `conventions[]` to the global catalog. Outside contribution mode, Work computes the repository identity, reuses the available convention stored in `branch_conventions.json`, or asks the user to choose one and persists it. It then presents its prefixes. `work convention` opens the TUI with the current choice and the switch action; `work convention show` only displays it and `work convention set <convention>` replaces it directly. Identity uses, in this order, the `origin` URL, root commits, or the absolute path in a shallow clone without a remote.

Under ADR-0011, remembered choice and effective provider availability are separate
projections. Provider-qualified choices retain availability only through their actual enabled owner; homonymous conventions from another provider do not substitute. Unavailable memory stays
stored and visible with its reason until explicitly replaced; noninteractive start
returns convention-unknown, and interactive start asks for a replacement even when
only one remains. The shared setter revalidates the reviewed memory and provider
facts while merging unrelated identities. Existing Work conventions are unchanged.

Outside contribution mode, after interpolating the convention prefix with the chosen slug, the core validates the resulting branch name using Git's own rules. It also checks for collisions with existing local and remote branches before materializing the new branch/worktree. An invalid name or collision blocks creation and returns the flow to the choice that produced the name.

The core must prefer native Git operations for these checks, including `git check-ref-format --branch` for syntactic validity and ref queries to detect already existing names, rather than maintaining a parallel implementation of branch naming rules. In contribution mode this rule does not require the branch to be new, because the expected behavior is precisely to check out the branch resolved by the Starter.

***

## 9. Events, eligibility, and Linkers

Events are lifecycle identifiers defined by the core in the format `<command>:<event>`. Plugins subscribe to them, but do not create events. In v1, `start:finalized` occurs after the Work and its core state have been materialized.

`on[]` for Importers and `discover.on[]` for Linkers accept `event` and an optional `starters` filter; without a filter, the subscription applies to any Starter. ADR-0024 binds self-package names to their provider and explicit qualified references exactly; an unrelated ambiguous bare filter never matches arbitrary homonyms. Filters determine eligibility and are not passed to subprocesses.

Before starting a subprocess, the core checks: plugin enabled; operation available for the current activation; subscription and filter when activation is event-driven; and existence of all required inputs. Inputs use `work:<key>`, `link:<key>`, or `meta:<key>`, with `:optional` for acceptable absence. Optional inputs do not participate in eligibility and are omitted from input when absent. The core resolves the namespace, but projects only the requested key and its value into the payload.

A Linker declares a namespaced `key`. Multiple Linkers may produce the same key without this being a collision: they are alternative providers of the same semantic relation. `discover.automatic` allows discovery requested by Work; `discover.on[]` allows event-driven discovery. Discovery receives only its inputs and optionally returns `{ "value": "..." }`; success without a value does not alter state and is not an error. With a value, the core upserts into `links[key]` and records its provenance in the index. `manual` makes the Linker available in `work link`: after selection, the core collects a non-empty value and performs the same upsert, without starting a subprocess.

***

## 10. Importers and staging

An Importer has the single domain operation `import`, activatable by `on`, `manual`, or both. `manual` makes it available in `work import` for the current Work. Eligibility and inputs are identical between manual and event-driven activation.

The core creates a unique temporary directory for each execution and sends only the resolved inputs and `output_dir`:

```jsonc
{
  "inputs": {
    "github.pull_request": "https://github.com/example/project/pull/212",
    "start_mode": "contribution"
  },
  "output_dir": "/tmp/work-import-01J..."
}
```

After success, the core enumerates all produced content, computes destinations in the Work directory, and checks all collisions before modifying it. Without collisions, it incorporates the artifacts and removes the temporary directory. With collision or failure, it incorporates no artifact from that execution and removes the temporary directory. Staging is a data boundary, not a sandbox.

***

## 11. `start:finalized` order and execution contract

When publishing `start:finalized`, the core performs the phases:

```text
Starter resolved
  → `work`, links, and metadata from the Starter persisted atomically
  → subscribed and eligible Linkers
  → new links persisted
  → subscribed and eligible Importers
  → staging, collision validation, and incorporation
```

There is no guaranteed order among components of the same role within a phase. Importer eligibility is evaluated after Linkers, allowing consumption of newly discovered links.

Every executable component receives exactly one JSON on stdin, writes at most one structured JSON on stdout when its operation requires it, and uses exit code for success or failure. With `runtime`, the command is `<runtime> <entrypoint>`; without it, the entrypoint is a self-contained executable. The runtime is checked at installation time. The core does not depend on implicit state between invocations and never executes a component during discovery or installation for self-description.

In v1, the IPC protocol has no intermediate progress, heartbeat, or streaming status messages. During execution, the core may display a visual indicator containing the component and current operation, but the subprocess produces only the final result expected by the contract.

The core does not impose its own timeout on Starter, Importer, or Linker executions in v1. Explicit user cancellation or process termination can still interrupt the operation. Retry, latency, or external service unavailability policies belong to the component.

Automatic Linker or Importer failures are logged and shown as warnings, but do not roll back the Work already created. In manual commands, the failure is returned to the user without changing links or incorporating artifacts. Plugins never read or write `work-state.json` directly: they receive inputs and return outputs through IPC contracts; the core converts those contracts into the persisted snapshot.

***

## 12. Update, presentation, and integrity

`work status [work]` is a read-only operation. The core reads the canonical snapshot and presents identity, state, branch, applicable location — active worktree or archived directory — and persisted links; it does not execute extensions, does not trigger discovery, and does not update recent access, provenance, or link timestamps. Without a target, it resolves the Work associated with the current directory; if there is none, it fails with an actionable message.

The presentation uses the semantic key and the persisted value. Additional representation-specific behaviors — for example, offering navigation when a Semantic Convention defines a navigable value — may be added without changing the command's basic semantics.

`work plugin update --check [plugin...]` checks for updates without changing the checkout; without names, it checks all installed plugins. `work plugin update <plugin...>` and `work plugin update --all` change versions only through explicit action; `--check` and `--all` are mutually exclusive. `work plugin update` without a target opens the TUI selection. The update reads and validates the new manifest before changing the recorded version.

### 12.1 Discovery surface

As defined by ADR-0019, `work` without arguments does not open an interactive model. In an interactive terminal, the static renderer produces the `WORK` wordmark in terminal art, a tagline, and guidance for `work --help`, then exits 0. The wordmark distributes the theme accents in a gradient from the primary `#11A8CD` to the secondary `#8B7CF6` when capability and contrast allow; the renderer chooses a compact, escape-free `WORK` for narrow, colorless, or non-interactive terminals.

`work --help` uses the registered command tree as the single inventory and groups only the commands that are present by context: everyday/global, within a Work, administration, and setup/plumbing. Branding, usage, options, and groups are rendered by the same capability policy. The home does not maintain a second command list.

`work plugin`, `work repository`, and `work convention` remain interactive hubs. Repository/convention operations show the equivalent direct command. The plugin inventory manager shows current feedback within its session and one acknowledgment after normal exit, as specified by spec-008 FR-035–FR-039. `work resume`, `work archive`, `work import`, and `work link` use interactive presentation only for omitted values; explicit targets skip the corresponding selection, but not validations or confirmations. In non-interactive stdin, missing required values fail without opening interactive controls.

### 12.2 Interactive presentation boundary

As defined by ADR-0020, the CLI/application composes the journey and translates domain concepts into generic presentation specifications. It provides titles, descriptions, options, groups, repeatable and mutation-free validators, receipt formatters, and confirmation content. It also retains all operation over Work, Git, snapshot, projection, plugins, and configuration.

The presentation layer keeps only visual and input state: editing, cursor, filter, viewport, selection, confirmation, and step lifecycle. It does not receive Work DTOs or query domain or infrastructure. Options carry opaque values and presentation text; group meaning, identities, and consequences remain in the CLI.

As defined by ADR-0021, each journey runs as **one full-screen program** (alternate buffer) — a `present.Wizard` over ordered steps. The primitives implement a domain-free `stepModel`; the wizard composes the ruler, journey title, trail of accepted receipts, and current step body, and performs a full repaint on each frame. No step terminates the program: it reports a terminal state and the wizard drives the transition. Its state flow is:

```text
active --recoverable error--> active with the current error replaced
active --accepted-----------> status "completed"; wizard appends the receipt and advances
active --cancelled----------> status "cancelled"; wizard exits
```

In controls with confirmation, the sequence is `selection → confirmation → completed`; going back from confirmation returns to selection without mutating domain state, and cancellation ends the step. On exit, the primary buffer is restored automatically and the compact receipt trail is reprinted to the UI channel, ahead of the stable stdout lines — this reprint is what enters terminal history.

For looping administration hubs, the presentation session hosts successive wizards inside the same full-screen program, following ADR-0021's one-program-per-journey lifecycle. The CLI performs service work between prompts; the session retains terminal dimensions, theme, and accepted receipts across screens. Read results use a scrollable view that waits for Back before returning to the menu. Operation output is shown within the session and published to its configured streams after terminal restoration; retained history yields space to the active control.

Under ADR-0025 the plugin manager realizes the same domain-free boundary and single terminal
lifecycle through Installed and Marketplaces, with nested source/entry collections, source filters,
installed selection sets, and all-target actions. Display names retain visible namespaces; source details
show complete local installed/enabled totals. The twelve logical surfaces and every CLI/TUI
parity row are specified in feature 009 contracts. CLI callbacks supply inventory/details, prepare frozen candidates
and apply accepted service operations. The model owns cursor/filter/form/review
state and candidate cleanup across Back and asynchronous results. Dependency
authorization is separate from acceptance. Current status replaces prior feedback;
normal exit restores the primary buffer once and emits only its acknowledgment.
Start and repository/convention retain their wizard receipt behavior.

### 12.3 Geometry, theme, and capacity

The theme is semantic and injected into all renderers. `Primary` (`#11A8CD`) and `Secondary` (`#8B7CF6`) express branding and focus; `Success`, `Warning`, and `Danger` remain their own tokens; body text uses the terminal foreground; light variants and constrained palettes preserve contrast. A non-empty `NO_COLOR`, `TERM=dumb`, and non-TTY writers disable color. Bold and textual markers preserve meaning without color.

Every active model computes available rows and columns after reserving title, filter, error, help, and confirmation — and, under the wizard, also the ruler, journey title, and receipt trail. Unfocused and focused options reserve the same indicator column; `[ ]`/`[x]` boxes have equal rendered width; the two lines of an option move as a unit — the primary line receives `Primary` when focused, and the secondary line is always `Muted`. The fixed-width `❯ ` marker identifies focus without color. Resize recalculates viewport and cursor clamping. Secondary metadata is truncated before the primary identity. Measurement ignores styling sequences and accounts for displayed Unicode width.

### 12.4 Streams and diagnostics

Each session receives input and a UI writer explicitly from the CLI. Frames, contextual help, and human diagnostics are written to the configured stderr/UI channel. Stdout receives only the stable results defined in the command contracts.

Recoverable validations return a public message to the active control, without printing. Terminal errors rise structurally to the CLI/process boundary, which chooses exactly one renderer: interactive human or stable non-interactive. Category, token, code, and cause remain separate; the human form uses a summary and actionable hint, while technical chains appear only in explicit diagnostic mode.

### 12.5 Satisfaction and governing decisions

This section satisfies `docs/prd.md` FR-50 through FR-64, FR-76, FR-81–FR-83, and NFR-10 through NFR-11. ADR-0025 governs the marketplace surface and preserved manager experience. ADR-0019 governs the surface and discovery; ADR-0020 governs the presentation boundary and lifecycle; ADR-0021 governs the full-screen rendering structure (wizard per journey), replacing ADR-0020's inline-per-step session decision; ADR-0009 continues to govern the stack. The historical F1/F2 contracts continue to record the behavior delivered in those slices, but conflicting presentation is replaced by `specs/003-terminal-ux-revamp/`.

The direct administrative API is:

```text
work plugin list [-m MARKETPLACE] [--available --json] [--json]
work plugin add PLUGIN@MARKETPLACE [--yes] [--remove-policy-references] [--json]
work plugin marketplace add SOURCE [--branch BRANCH] [--json]
work plugin marketplace list [--json]
work plugin marketplace upgrade [MARKETPLACE] [--json]
work plugin marketplace remove MARKETPLACE [--yes] [--remove-policy-references] [--json]
work plugin enable <PLUGIN...>
work plugin disable <PLUGIN...>
work plugin update
work plugin update --check [PLUGIN...]
work plugin update <PLUGIN...> [--yes] [--remove-policy-references]
work plugin update --all [--yes] [--remove-policy-references]
work plugin remove <PLUGIN...> [--yes] [--remove-policy-references] [--json]

work repository locator list
work repository policy list
work repository policy add <LOCATOR> [--before <LOCATOR> | --after <LOCATOR>]
work repository policy remove <LOCATOR...>
work repository policy move <LOCATOR> (--before <LOCATOR> | --after <LOCATOR>)
work repository policy replace <LOCATOR...>
work repository root list
work repository root add <PATH...>
work repository root remove <PATH...>
work repository root replace <PATH...>

work convention show
work convention set <CONVENTION>
```

Read commands (`status`, `list`, `show`, and `plugin update --check`) accept `--json` and are pure. `--yes` confirms already determined impacts; it never chooses targets or values. There are no official aliases. The source of every plugin is explicitly chosen by the user and the installed reference is pinned; formal signing is deferred according to ADR-0008. ADR-0025 replaces the plugin grammar; ADR-0019 governs unchanged cross-cutting semantics. Plugin selectors are qualified or use an explicit marketplace flag, with no mixed forms. No compatibility selector or translation alias is part of the API.
