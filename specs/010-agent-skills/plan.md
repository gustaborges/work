# Implementation Plan: Agent Skills for Work Plugin Authors

**Branch**: `feature/agentic-skills` | **Date**: 2026-11-14 | **Spec**: [spec.md](spec.md)
**Input**: `../proposal.md`, feature 010 spec, [governance drafts](governance/README.md), local marketplace dependency 009.
**Status**: Implementation in progress; S0 completed against develop with owner approval. Authorities: PRD FR-65–69, ADR-0031–33 and ADD-0001.

## Summary

Bundle one canonical vendor-neutral `work-plugin-builder` text tree with the Work release and explicitly install identical copies for local Claude Code/Codex. Add `work skills install/status` in Setup, with pure status and no Work-home/bootstrap involvement. Use a portable tree digest, ownership marker, canonical target planning, persistent sibling locks and retained-backup recovery per destination. Generate references from actual contract descriptors; verify committed generated content and a reviewed pin without CI mutation.

The proposal mixes requirements/decisions/design. Draft authority amendments are ready for review; S0 promotes approved PRD → ADR → ADD before implementation. Plural naming conflicts with ADR-0019 until its proposed narrow exception is accepted. Feature 009 must first merge into development baseline (`develop`), with contract/ID/diagnostic reconciliation. No merge or acceptance is performed by planning.

## Technical Context

**Language/Version**: Go 1.26.0; module `github.com/gustaborges/work`.
**Primary Dependencies**: Existing Cobra 1.10.2, standard library embed/fs/crypto/encoding, existing internal/lockfile and internal/atomicfile. No new dependency or version upgrade; no presentation framework changes.
**Storage**: Embedded text assets; selected vendor skill trees; root `.work-skill.json`; sibling persistent locks, finite recovery records and owned stages/backups. No Work-home registry/admin state/index/snapshot migration.
**Testing**: Go unit/contract/integration/native helper-process tests; parser/full-tree/every-entry fixtures; deterministic checkpoint/race tests; existing three-OS CI; explicit real-agent dogfood. FR-021 requests these tests. Planning only validates documents.
**Target Platform**: Existing Linux/macOS/Windows CI and release targets. Text assets require no cross-compiled seed; scripts use declared runtimes.
**Project Type**: Local single-binary CLI plus embedded authoring documentation/scaffolds.
**Performance Goals**: No network/repository scanning in skill management; inspect only requested destinations and local coordination. Status at most three coherent-read attempts. Reuse 30-second lock acquisition limit/cancellation; no new arbitrary latency SLA.
**Constraints**: Explicit scope; identical payload bytes; force cannot override unsafe/unreadable/changed-target/recovery conditions. Never lock inside renamed trees. No Work/admin locks, bootstrap, mutationPreflight or repository discovery. No wildcard scratch deletion. No claim of atomic directory visibility or arbitrary power-loss safety.
**Scale/Scope**: One local user, initially one skill, two vendors and explicit target batches. No removal, overlays, online distribution, marketplace validate or new runtime component role.

## Constitution Check

*Evaluated before research and rechecked after design.*

Constitution is an unratified placeholder template, not adopted example principles. Existing PRD, accepted ADRs and roadmap §4 supply current gates. Draft amendments cannot override them.

| Gate | Before research | Post-design evidence / implementation prerequisite |
| --- | --- | --- |
| Authority order | PASS for draft planning | Governance package separates layers; S0 blocks implementation until promotion/approval. |
| Singular command grammar | No accepted change | Proposed narrow exception only; conditional CLI contract must be approved at S0. No conflicting code/current ADD edited. |
| Determinism/integrity | PASS | Canonical ordered targets, framed portable digest, fixed observed state, finite recovery and exact per-target commit outcomes. |
| Read purity | PASS | Status uses no mutating helper/lock; Work state excluded; Q3/Q6 byte/existence checks. |
| Portability | PASS | Existing lock/atomic helpers plus explicit directory moves; three-OS fault/Unicode/mode/case fixtures. |
| Contract correctness | PASS | Real parser differences retained; descriptor generation, pin, fixture routing and explicit unrepresented-behavior review. |
| CLI discovery/presentation | PASS conditionally | Live setup help, stable stream/exit receipts, no new hub or static-home inventory. |
| Delivery regression | PASS | 009 merge recheck, seed/lint/full suites and actual per-agent dogfood retained as delivery gates. |

**Pre-research and post-design result**: PASS for draft planning; S0 remains a required external authority/dependency gate, not a waived violation. If S0 is unapproved, implementation must stop there.

## Project Structure

### Documentation (this feature)

```text
specs/010-agent-skills/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── governance/                 # proposed PRD / three ADRs / conditional ADD
├── contracts/
│   ├── cli-work-skills.md
│   ├── skill-installation.md
│   └── skill-content.md
├── checklists/requirements.md
└── tasks.md                    # generated by speckit-tasks after planning
```

### Source Code (repository root; future implementation)

```text
ai-skills/                      # package skills; canonical tree plus embed/generation tests
├── embed.go
├── digest.go
├── generate.go
├── internal/generate/main.go
├── internal/skilllock/main.go
├── skills_test.go
└── work-plugin-builder/
    ├── SKILL.md
    ├── CONTRACT
    ├── references/{marketplace-catalog,plugin-manifest,component-roles,validate-and-publish}.md
    ├── references/generated/{catalog-schema,manifest-schema,role-matrix}.md
    └── templates/{marketplace/marketplace.json,plugin/plugin.json,plugin/bin/}
internal/
├── skillinstall/
│   ├── types.go
│   ├── targets.go
│   ├── inspect.go
│   ├── plan.go
│   ├── apply.go
│   ├── recovery.go
│   └── *_test.go
├── plugin/contract.go           # shared validator-derived descriptor
├── marketplace/contract.go     # dependency 009: extract inline key sets without semantic changes
├── pluginid/contract.go
├── semconv/contract.go
├── ipc/contract.go             # derive actual request/response types and constants
├── cli/skills.go
└── diag/diag.go
 tests/contract/skills_test.go
 tests/integration/{skills_install,skills_status,skills_recovery,skills_concurrency}_test.go
 tests/fixtures/skills/          # catalog/manifest/IPC/runtime fixtures
 docs/agent-skills.md
 docs/marketplaces.md           # exists after 009 merge
 .github/pull_request_template.md
```

**Structure Decision**: `ai-skills` owns assets, digest and generation output; it imports actual domain descriptors. Domain packages never import assets. `skillinstall` consumes skill inventory/trees/digests and owns filesystem consistency, independent of Work administration. CLI renders typed results with existing diagnostic conventions. Share one digest implementation for embedded/staged/installed trees. No cycles or duplicated contract tables.

## Phase 0 — Research (complete)

[research.md](research.md) resolves R1–R10 with current/dependency commit evidence and official vendor sources. Important corrections: selected validation lives in plugininstall/prepare.go after 009; catalogs and manifests differ in strictness; runtime checks occur only during selected install; the package digest is unsuitable for embedded skill modes; reload behavior differs by vendor.

## Phase 1 — Design (complete)

- [Data model](data-model.md): exact digest framing, ownership, target/observation/result and journal records.
- [CLI contract](contracts/cli-work-skills.md): conditional grammar, scopes, sorted JSON/human outputs, exit aggregation and diagnostic reconciliation.
- [Installation contract](contracts/skill-installation.md): canonical containment, complete preflight, ordered locks, per-target state machine, coherent inspection and failure matrix.
- [Content contract](contracts/skill-content.md): workflow, descriptor coverage, generated references/pin, fixture routing and complete isolated validation.
- [Quickstart](quickstart.md): runnable future local checks, fault/portability matrix, real-agent dogfood and release checks.
- [Governance drafts](governance/README.md): reviewable amendments; promote only at S0.
- Refresh configured AGENTS.md via extension script, plus CLAUDE.md managed plan reference, after plan is written. Preserve unrelated instructions.

## Phase 2 — Sequence for task generation

1. **S0 Authorities/dependency**: merge 009 into development baseline (`develop`) through the authorized project workflow, inspect merged facts, assign available IDs/codes, promote PRD then accepted ADRs then ADD, reconcile every feature artifact. This planning request does not authorize doing the merge. If approvals/merge are absent, stop future implementation at this gate.
2. **Foundations**: safe canonical target resolution, exact digest/inventory, typed observation/result/marker/record models and filesystem seams; shared validator/protocol descriptors and initial deterministic reference renderer. Shared protection must precede any writable public surface.
3. **US1 / S1**: canonical vendor-neutral content, role/scaffold/validation references; embed assets and basic fixture validation. Fresh initial installation will use the full US2 engine. This story can be content-tested independently, but no unsafe partial installer is released.
4. **US2 / S2a**: ownership classification, full batch preflight, ordered persistent locks, recoverable stage/backup transaction, revalidation and partial outcomes; wire install only when fault/race evidence passes.
5. **US3 / S2b**: pure coherent status and deterministic output; test without requiring writable installer activation.
6. **US4 / S3**: strengthen foundational descriptors/generation with pin and full fixture gates; maintainer instructions, PR rule and non-mutating CI.
7. **S4 Delivery**: docs/help/README, clean real-agent exercises, cross-platform fault/purity matrix and full existing regression suite. MVP demonstration is US1 content plus safe fresh install after US2 foundations; public delivery requires every slice.

## Validation Strategy

| Requirements / outcomes | Planned proof | Quickstart |
| --- | --- | --- |
| FR-001–FR-006; SC-001/004/006 | Offline payload parity, vendor discovery, every-entry isolated validation, source recapture/environment cleanup and agent-only authoring. | Q1/Q4/Q7 |
| FR-007–FR-012/017; SC-002/007 | Marker/state/force matrix, no-op mtimes, safe paths/case aliases, ordered races and every journal interruption. | Q2/Q5 |
| FR-013–FR-016; SC-003 | Status states/output/aggregate error/coherence; byte/existence checks including explicit WORK_HOME and no lock creation. | Q3/Q6 |
| FR-018–FR-019; SC-005 | Independently mutated descriptor contracts, real-parser routes, full discovery/install fixtures and non-mutating pin/output tests. | Q4 |
| FR-020–FR-022; SC-006/007 | S0 evidence, help/docs/review rules, three-OS full suites and actual agent dogfood. | Q7/Q8 |

Use injected filesystem/checkpoint/helper-process seams, not production environment fault switches. Synchronize races with channels/pipes. Windows symlink capability skips must be explicit, while non-link path/type tests still run. `make seed` precedes host tests; preserve CI `make seed-all`, `make lint`, `go test ./...`, host build. CI never invokes mutating `make skill-lock` or auto-fixes generated files. No implementation build/test has run during planning.

## Risks and Boundaries

- S0 was approved during implementation; see validation.md for develop revision and authority IDs.
- Descriptor coverage detects represented drift only; maintain semantic fixtures and same-PR prose review.
- Canonical path/case semantics vary by filesystem. Resolve existing identity, reject unresolved overlaps conservatively and test aliases on native CI volumes.
- A local lock does not prevent hostile external edits. Recheck before moves and preserve recovery evidence rather than overclaiming atomic visibility.
- Recovery must include marker bytes in old-content observation even though payload digest excludes the root marker; otherwise foreign metadata edits could be lost.
- Actual Claude/Codex sessions and runtime prerequisites are needed for dogfood. Missing access is an incomplete delivery task, not successful simulated evidence.

## Complexity Tracking

No adopted constitution exception. The proposed ADR-0019 exception is gated, not implemented. A finite per-destination record/backup protocol is required because file replacement helpers cannot replace nonempty directories and early backup deletion violates FR-011. Separate coordination is necessary for FR-016. This is not a new Work state store or generic transaction framework.
