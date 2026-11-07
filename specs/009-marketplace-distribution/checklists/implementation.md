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
- An explicit add of the reference settles first-use setup and policy (`initialized` and `policy_handled`) without inserting a Locator, so a later setup neither reinstalls it nor seeds the default policy. The broader lifecycle matrix remains M3 (T054–T057).
- Reinstall (changed fixed content, including an equal opaque version) keeps enablement and needs `--yes` or an interactive acceptance; dropping policy Locators needs `--remove-policy-references` separately.
- Private values are stored under `plugin.<plugin>@<marketplace>~<registration_id>.<local>`; `work status` currently prints stored keys as written.
- `plugin remove` is the former uninstall (journal label `work plugin remove`); `plugin update` arrives in M3.

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

## Native platform evidence slots (T076)

| Platform | Acceptance / path / schema / recovery | Unix PTY | Race |
| --- | --- | --- | --- |
| Ubuntu | | | |
| macOS | | | |
| Windows | | not applicable | |

Cross-compilation (`make release`) is not execution evidence.

## Release gate

Public delivery requires all of: M1–M4 complete; every P01–P12 / S01–S12 case verified; current-only code, tests and docs; native three-OS outcomes recorded above; and the owner-authorized real initial `work` publication evidenced (T071, T072). Local fixtures do not satisfy the publication gate.

- [x] M2 complete
- [ ] M3 complete
- [ ] M4 complete
- [ ] Owner-confirmed official origin recorded (T071)
- [ ] Real initial publication and later refresh/removal evidenced (T072)
