# Implementation Evidence: Marketplace Plugin Distribution

**Purpose**: Record the intended baseline, the native-platform evidence and the release gate for feature 009. Entries are filled from commands actually run; an empty slot means "not yet run", never "passed".

## Baseline (T001)

- [x] PRD FR-79 (`docs/prd.md`) requires one purposeful marketplace-only contract and removes superseded installation, identity and state-format paths, with no adapters, dual readers, migration commands or adoption flows.
- [x] ADR-0030 (`docs/adr/adr-0030-clean-prerelease-marketplace-cutover.md`) states the same clean pre-release cutover and replaces the earlier continuity clauses.
- [x] The CLI is unreleased. Development plugins and state created before this feature may break; no real-home reset or deletion is authorized by any task.
- [x] Current-product safety remains required and is not backward compatibility: transaction recovery, process-held leases, exact authorization, source provenance and one-time policy behavior.

## Milestone evidence

### M1 — Sources and fixed discovery

| Area | Evidence |
| --- | --- |
| Composed identity and portable storage | `internal/pluginid`, `internal/workhome/marketplace_test.go` |
| One strict schema-1 shape (config, registry, memory, lifecycle, catalogs) | `internal/config`, `internal/registry`, `internal/repoconv`, `internal/adminstate/lifecycle.go`, `internal/marketplace/state.go` |
| Finite journal, typed moves, prepared rollback / committed replay | `internal/adminstate/marketplace_recovery_test.go` |
| Process-held leases and orphan collection | `internal/adminstate/lease_test.go` |
| Canonical digests (contract vectors) | `internal/marketplace/digest_test.go` |
| Strict catalog, containment, collisions | `internal/marketplace/catalog_test.go` |
| Coherent local capture, exact Git revisions | `internal/marketplace/source_test.go`, `internal/gitx/fixed_test.go` |
| Source registration / refresh / inventory | `internal/marketplaceadmin/*_test.go` |
| CLI contract and error table 57–61 | `tests/contract/marketplace_cli_test.go`, `internal/diag/diag_test.go` |
| Zero-install registration, collisions, offline browsing | `tests/integration/marketplace_sources_test.go` |
| Pure inspection and strict rejection | `tests/integration/marketplace_state_test.go`, `tests/integration/admin_purity_test.go` |

M1 also performs the identity cutover the later milestones build on: the alias-keyed registry, direct-source installers, link/embedded publishing and the update machinery are removed, and the administrative journal, lifecycle and convention memory take their one current shape. The reference package is installed by first-use setup as `work-reference@builtin`. New publication (resolved-entry install, qualified add/remove, update) arrives in M2/M3; until then tests that need an installed provider use `tests/fixtures/installfx`, which writes through the production journal and registration code.

### M2 — Independent copies and composed identity

| Area | Evidence |
| --- | --- |
| Private-key grammar (relative, qualified, core-only storage; dotted names) and ownership | `internal/semconv/provider_test.go`, `internal/plugin/manifest_test.go` |
| Starter filters: self-package, exact external, unique and ambiguous global | `internal/extension/provider_test.go` |
| Private inputs and publications bound to the providing source instance | `internal/extension/provider_test.go`, `internal/starter/bind_test.go`, `tests/contract/manual_import_protocol_test.go` |
| Resolved-entry preparation, selected-only copy, no-op / reinstall boundary, drift, exact Git commit, builtin | `internal/pluginadmin/marketplace_install_test.go` |
| No standalone publisher in the installer package | `TestInstallerExposesNoStandalonePublisher` |
| Provider-owned conventions, policy and retained memory | `internal/registry/provider_test.go`, `internal/repoconfig/provider_test.go` |
| Per-provider and per-source-instance invocation guard | `internal/adminstate/invocation_test.go` |
| `plugin add|list|remove|enable|disable` grammar, retired forms fail as usage before any state | `internal/cli/plugin_remove_test.go`, `tests/integration/marketplace_install_test.go` |
| JSON/receipt contracts for add, list `--available`, remove | `tests/contract/provider_identity_test.go` |

Behavior worth knowing when reading the evidence:

- First-use setup and `plugin add work-reference@builtin` share one preparation and registration path (`plugininstall.Prepare`/`Publish`).
- An explicit add of the reference settles first-use setup and policy (`initialized` and `policy_handled`) without inserting a Locator, so a later setup neither reinstalls it nor seeds the default policy. The broader lifecycle matrix is M3 evidence below.
- Reinstall (changed fixed content, including an equal opaque version) keeps enablement and needs `--yes` or an interactive acceptance; dropping policy Locators needs `--remove-policy-references` separately.
- Private values are stored under `plugin.<plugin>@<marketplace>~<registration_id>.<local>`; `work status` currently prints stored keys as written.
- `plugin remove` is the former uninstall (journal label `work plugin remove`); `plugin update` and whole-source removal arrive in M3 (below).

### M3 — Maintenance and default lifecycle

| Area | Evidence |
| --- | --- |
| Shared source/branch candidates, frozen reviewed apply, equal/changed opaque versions, invalid equal-version, missing registration/discovery, path move, withdrawal, per-target drift and error precedence | `internal/pluginadmin/marketplace_update_test.go`, `internal/marketplace/candidate.go` |
| Whole-source set (enabled, disabled, withdrawn), offline removal, separate policy authorization, domain refusal of the included provider, instance-bound membership, drift conflicts, retained convention memory | `internal/marketplaceadmin/remove_test.go` |
| Journal checkpoints for source removal and update (prepared rollback, committed replay), pre-commit injection, committed cleanup failure, live capture lease, read purity during recovery | `tests/integration/marketplace_recovery_test.go` |
| Pure refresh/check, one revision per batch, exact reviewed apply, same-version reinstall, withdrawal, atomic offline source removal, dependency refusal, batch precedence, update grammar | `tests/integration/marketplace_maintenance_test.go` |
| Concurrent refresh/install/remove, force-pushed source, lease lifetime, running-package removal, unaffected-change merging (run with `-race`) | `tests/integration/marketplace_concurrency_test.go` |
| Lifecycle matrix: registration before first use, explicit policy, explicit reference add, disabled/removed defaults, damaged content, invalid lifecycle | `tests/integration/marketplace_bootstrap_test.go`, `internal/bootstrap/bootstrap_test.go`, `internal/cli/policy_lifecycle_test.go` |

Behavior worth knowing when reading the evidence:

- A check judges the source recorded in each package's provenance and opens each source/branch once; it never consults or writes discovery, and exposes its rows only if the administrative epoch is unchanged.
- Update applies the frozen staged candidate per package (a separate transaction each); drifted targets fail alone with conflict/53, which outranks the generic update failure. Cancellation, recovery and cleanup failures stop the remainder.
- Source removal membership is by registration instance, not namespace text. Stored captures of a removed source are collected by the next mutation unless a live lease names them.
- Every explicit `repository policy` mutation marks `policy_handled` in the same transaction; rejected edits do not. There was no pre-existing default-adoption suite to delete (T057): the bootstrap tests were already written against the current lifecycle.

### Commands run (fill in with real outcomes)

| Command | Platform | Outcome |
| --- | --- | --- |
| `go test ./internal/pluginid ./internal/marketplace ./internal/marketplaceadmin` | linux/amd64 | passed |
| `go test ./internal/adminstate ./internal/pluginadmin ./internal/bootstrap` | linux/amd64 | passed |
| `go test -race ./internal/adminstate ./internal/marketplaceadmin` | linux/amd64 | passed |
| `go test ./...` | linux/amd64 | see M1 completion record below |
| `go test -count=1 ./...` (after M2) | linux/amd64 | passed |
| `go vet ./...`, `gofmt -l .`, `go tool staticcheck ./...` (after M2) | linux/amd64 | clean |
| `go test -race ./internal/adminstate ./internal/marketplaceadmin ./internal/pluginadmin ./internal/extension ./internal/semconv ./internal/starter` | linux/amd64 | passed |
| `go test -count=1 ./...` (after M3) | linux/amd64 | passed |
| `go vet ./...`, `gofmt -l .`, `go tool staticcheck ./...` (after M3) | linux/amd64 | clean |
| `go test ./internal/adminstate ./internal/marketplace ./internal/marketplaceadmin ./internal/pluginadmin` and the M3 `tests/integration` concurrency/recovery cases with `-race` (after M3) | linux/amd64 | passed |

## Native platform evidence slots (T076)

| Platform | Acceptance / path / schema / recovery | Unix PTY | Race |
| --- | --- | --- | --- |
| Ubuntu (local linux/amd64, 2026-11-08) | `go test ./...` passed | passed (manager lifecycle, parity, geometry) | passed (administrative, present, concurrency/recovery journeys) |
| macOS | pending CI run | pending CI run | not scheduled |
| Windows | pending CI run | not applicable | not scheduled |

Local gates on 2026-11-08: `gofmt`, `go vet ./...`, `go tool staticcheck ./...`, `go test ./...` and `make build` passed. Native macOS/Windows outcomes are not claimed until the CI matrix runs.

Cross-compilation (`make release`) is not execution evidence.

## Official publication evidence (T071, T072)

Origin supplied by the project owner on 2026-11-08: `https://github.com/gustaborges/work-cli-marketplace.git`, branch `master`, namespace `work`. All changes reached `master` through pull requests except where noted. Every command ran with the freshly built `bin/work` (linux/amd64) in disposable homes against the real remote, never a fixture.

| Revision on `master` | Change | Plugin versions |
| --- | --- | --- |
| `c8fc143` | PR #1: first catalog (`marketplace.json`, `plugins/github`, `plugins/wsp`) | github 1.0.0, wsp 1.0.0 |
| `60893e4` | linker REST fallback. **Pushed to `master` directly by mistake**, not through a PR; the content was then carried into PR #2 | github 1.0.0 (content changed) |
| `46806a6` | PR #2: four importers | github 1.1.0 |
| `0518d1f` | PR #4: wsp locator normalizes clone URLs | wsp 1.0.1 |

| Scenario | Observed |
| --- | --- |
| Initial registration and install from the real origin | `marketplace add` registered at `c8fc143`; `plugin add github@work` and `wsp@work` installed 1.0.0 |
| Real start through installed 1.0.0 | `work start <PR URL>` resolved the clone through the wsp locator, offered contribution/fork, created the Work with the `github.pull_request` link and number |
| Equal version, changed bytes (`60893e4`) | `marketplace upgrade` moved the stored revision; `update --check` reported `unchanged` with differing digests; `plugin add github@work` reinstalled (`result: reinstalled`), after which check was `unchanged` with equal digests |
| New version adds components (`46806a6`) | `update --check` reported `update-available 1.0.0 -> 1.1.0` listing the four importers as added and nothing withdrawn or changed; `update github@work --yes` applied; enablement kept; importers then ran against the Work started under 1.0.0 |
| Patch update with a policy reference (`0518d1f`) | `update --check` per plugin: builtin and github `unchanged`, wsp `update-available 1.0.0 -> 1.0.1`; `update --all --yes` updated only wsp; the resolution policy still listed `wsp@work/wsp-repository-locator` |
| Automatic importers on a fresh start | home on 1.1.0: `work start <issue URL>` ran the issue linker and the issue importer automatically and wrote `context/github/issue/issue.md`; no PR importer ran |
| Manual import contracts | missing link: `required-input-missing`; repeated import: `extension-output-refused`; PR description/comments/diff and issue imported |
| Whole-source removal | `marketplace remove work --yes` refused with `plugin-policy-dependency`; state unchanged; with `--remove-policy-references` it removed `github@work` and `wsp@work`, emptied the policy, kept `builtin` and the Work directory |

Defects found by this run and fixed before release: the PR linker failed without a logged-in `gh` (fixed in `60893e4`); the wsp locator never matched URL-only references (fixed in 1.0.1). The component classes involved were not covered by any local fixture.

## Release gate

Public delivery requires all of: M1–M4 complete; every P01–P12 / S01–S12 case verified; current-only code, tests and docs; native three-OS outcomes recorded above; and the owner-authorized real initial `work` publication evidenced (T071, T072). Local fixtures do not satisfy the publication gate.

- [x] M2 complete
- [X] M3 complete
- [ ] M4 complete
- [x] Owner-confirmed official origin recorded (T071)
- [x] Real initial publication and later refresh/removal evidenced (T072)
