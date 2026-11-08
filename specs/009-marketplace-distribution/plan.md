# Implementation Plan: Marketplace Plugin Distribution

**Branch**: `feature/marketplace` | **Date**: 2026-11-07 | **Spec**: [spec.md](spec.md)
**Input**: Proposal with the owner's clean-cutover correction, PRD FR-16–FR-21/FR-35/FR-40–FR-48/FR-50–FR-83, ADR-0022–ADR-0030, and ADD-0001.
**Status**: Planned through tasks; implementation not started.

## Summary

Make marketplaces the only publication source and composed providers the only plugin identity. Register Git/local sources or use builtin, browse fixed offers, and copy one selected entry into independent execution storage. Carry real provider ownership through conventions, policy, memory, subscriptions, private data and revision guards. Separate catalog refresh, opaque-version update and explicit reinstall. Extend current finite administration for catalogs and whole-source removal, then expose every operation in the existing terminal manager.

The CLI is unreleased. **ADR-0030 removes backward-compatibility obligations:** delete superseded paths, old-format readers, alias variants, migration/adoption flows, conversion/fencing and compatibility-only tests. Update code, fixtures, examples and active docs directly to the intended model. Keep current transaction safety and purposeful product behavior.

## Technical Context

**Language/Version**: Go 1.26.0; module github.com/gustaborges/work.
**Primary Dependencies**: Existing Cobra 1.10.2, Bubble Tea 2.0.9, Lip Gloss 2.0.6, Git wrapper, lockfile/atomicfile and x/sys; SQLite 1.58.0 remains the Work projection. No dependency addition/upgrade planned.
**Storage**: One strict current schema-1 shape for config/registry/memory/catalog/control/journal/lifecycle. Typed managed roots plugins/<marketplace>/<plugin>/source; hex-only capture directories and process-held leases. Source-instance IDs protect retained choices/private values. Lifecycle uses initialized/policy_handled/removed. No older-format decoder or conversion layer.
**Testing**: Current-product unit/contract/testscript/integration tests, canonical vectors, grouped fixed candidates, exact impact, checkpoint/lease/concurrency races, pure reads, CLI/TUI parity and three-OS CI. Fresh disposable state only; no application checks are claimed run during planning.
**Target Platform**: Linux/macOS/Windows native execution; release Linux amd64/arm64, macOS amd64/arm64, Windows amd64. Unix PTY stays platform-tagged. Cross-compilation is not execution evidence.
**Project Type**: Local single-binary CLI/TUI and external plugin processes, not a hosted marketplace service.
**Performance Goals**: At most three coherent-read attempts; existing lock acquisition budget; one revision per source/branch candidate group. Listing has no network/Work-database access. Slow preparation/review stays outside publication lock. No new SLO or plugin timeout.
**Constraints**: One catalog/copy/provider/storage contract, no direct publisher bypass, no read writes/recovery/bootstrap, no implicit policy insertion or binary-driven replacement, and no administration-under-Work lock. Current Works survive package/source removal. No prior development format/behavior preservation requirement or real-home reset is introduced.
**Scale/Scope**: One Work home's source/package inventory. No aggregation, discovery service, bulk-catalog install, dependency manager, SemVer/channels, external artifacts, signing, billing, reputation or same-user tamper protection. Actual official origin/publication is owner-supplied delivery evidence.

## Constitution Check

The constitution remains an unratified placeholder; its commented examples are not adopted rules. PRD and ADRs supply the current gates. The explicit user correction supersedes the inferred continuity work through ADR-0030.

| Gate | Design and evidence |
| --- | --- |
| Product authority | PRD FR-79 requires clean pre-release cutover; ADR-0030 replaces the relevant compatibility clauses; active artifacts agree. |
| Simplicity | One provider type, one copy mode, one current decoder/journal, one manager journey. Delete superseded code/tests instead of adapting them. |
| Determinism | Explicit entries/namespaces, fixed offers/candidates, actual source provenance, opaque versions and exact authorized sets. |
| Integrity/recovery | Finite images/typed moves, one lock/epoch, current prepared/committed recovery and leased preparation. |
| Purity/trust | No read/check persistence; code-owned builtin; safe text/source output; discovery executes no plugin. |
| UX/parity | S01–S12 and P01–P12 verified through current callbacks, filters, selections, one session and semantic theme. |
| Portability | Safe names/hex paths, recorded executable semantics, native three-OS tests, Unix PTY and relevant races. |

**Pre-research and post-design:** PASS. R1–R12 resolve current design questions. The official publication origin is a release prerequisite, not an invented source or unresolved implementation choice. Compatibility-only work is excluded by explicit user direction.

## Project Structure

### Documentation

```text
specs/009-marketplace-distribution/
  spec.md
  checklists/requirements.md
  plan.md
  research.md
  data-model.md
  quickstart.md
  contracts/
    marketplace-catalog.md
    digest-vectors.json
    provider-identity.md
    cli-work-plugin.md
    administrative-state.md
    administrative-hubs.md
    bootstrap-and-publication.md
  tasks.md
```

### Planned source changes

```text
internal/
  pluginid/                 # NEW single composed provider/contribution/address model
  marketplace/              # NEW strict catalog, fixed Git/local/builtin sources and digests
  marketplaceadmin/         # NEW source plans and safe projections
  config/, registry/, repoconv/ # replace models with one current shape
  repoconfig/, locator/, convention/ # qualified references and real owner availability
  adminstate/               # finite catalog images, typed moves, current recovery and leases
  workhome/                 # validated composed roots/captures/leases
  plugininstall/            # resolved-entry copy validation; remove standalone/link publishers
  pluginadmin/              # common install/toggle/update/remove and frozen impact
  bootstrap/                # common builtin entry plus policy-handled/removal lifecycle
  gitx/                     # recorded branches and exact revisions
  plugin/, semconv/         # current qualified filters/private owner grammar
  starter/, extension/      # provider-aware input/output, eligibility and guards
  cli/                      # intended add/remove/marketplace commands and manager callbacks
  present/                  # generic nested collections, selection sets, review/progress
  create/, projection/      # current fully qualified provenance values
  diag/                     # purposeful error names and new categories 57–61
seed/                       # builtin catalog and current native reference assets
examples/marketplaces/      # minimum single/multiple author catalogs
tests/
  fixtures/marketplaces/    # current local/Git/homonym/invalid source fixtures
  contract/, integration/   # current grammar/state/safety/parity/platform evidence
.github/workflows/ci.yml
README.md, docs/            # active one-contract documentation; ADR bodies retain document history
```

**Structure Decision**: pluginid is a leaf. Marketplace resolves immutable entries; marketplaceadmin owns source policy; pluginadmin consumes entries; plugininstall cannot publish arbitrary sources. Adminstate has a finite owned model, not arbitrary path writes; storage packages do not import it. Share entry validation/copy with bootstrap without cycling through pluginadmin. CLI composes domain operations and safe callbacks; presentation remains domain-free. Delete old structs/helpers/test expectations that have no current caller instead of leaving deprecated wrappers.

## Phase 0 — Research (complete)

[research.md](research.md) records R1–R12 and concrete code evidence. Core work remains real convention ownership, scoped private data, exact Git materialization, safe preparation cleanup, common builtin installation and multi-selection/nested UI. The user correction eliminates old-provider unions, dual readers, schema conversion, old journal/executable tests, adoption and migration journeys.

## Phase 1 — Design (complete)

- [data-model.md](data-model.md): single current models, ownership, source instance, lifecycle and transitions.
- [Catalog](contracts/marketplace-catalog.md): strict offer, fixed source semantics and digest vectors.
- [Provider identity](contracts/provider-identity.md): contribution/private ownership, current filters/inputs and guards.
- [CLI](contracts/cli-work-plugin.md): intended grammar, results, confirmation and safe error names.
- [Administration](contracts/administrative-state.md): current finite publication, leases/recovery and exact revalidation.
- [Manager](contracts/administrative-hubs.md): S01–S12, P01–P12 and session/theme/viewport verification.
- [Bootstrap/publication](contracts/bootstrap-and-publication.md): one-time current defaults, no resurrection and official source evidence.
- [Quickstart](quickstart.md): future validation using current disposable homes; no application outcome claimed now.
- Configured agent-context hook points to this plan.

## Phase 2 — Tasks grouped by milestone

User-requested milestone grouping contains setup/foundations and story phases, with exact paths and story labels. Include focused tests only for the intended current behavior; remove compatibility fixtures/expectations. All implementation/validation tasks remain unchecked.

| Milestone | Demo/stories | Dependencies | Exit gate |
| --- | --- | --- | --- |
| M1 — Sources and fixed discovery | Shared foundations + US1; Git/local registration, builtin projection, offline offer browsing | Formalized product/architecture | Strict whole-offer validation, current state/recovery, complete captures/leases, pure inspection/totals |
| M2 — Independent copies and identity | US2; one selected copy, two homonyms and all provider-bearing paths | M1 | Common entry pipeline; standalone/link/alias branches removed in domain and CLI |
| M3 — Maintenance and default lifecycle | US3 + US4; grouped candidates, exact whole-source removal, purposeful first use/removal policy | M1–M2 | Current recovery/authorization, explicit replacement, no resurrection or accidental policy seeding |
| M4 — Complete interface and delivery | US5 plus active docs/examples/real publication/platform evidence | M1–M3 | Every parity/surface check, current-only code/tests/docs, official source evidence, native platform validation |

M1 is a discovery demonstration; M1+M2 demonstrates controlled install/execution. Public delivery requires all milestones and official publication evidence. Earlier development state/plugins may break; no release, commit, remote publication or real-home deletion occurs in this planning pass.

## Risks and limits

Current source reuse, same-user file changes, interrupted transactions and concurrent operations remain real domain concerns; their guarantees are not compatibility scaffolding. Official origin is external owner input required before delivery. Keep changes purposeful and local to the intended distribution feature; remove obsolete paths without unrelated refactoring.
