# Conditional ADD-0001 amendment: Agent authoring skills

**Status**: Proposed design; apply only after related PRD/ADRs are accepted at S0.
**Destination**: `docs/add/add-0001-work-system-architecture.md`, new agent skills section.

## Governing decisions and satisfaction

The [bundling decision](adr-bundled-skills.md) satisfies AS-R1/AS-R5; the [command decision](adr-skills-command.md) satisfies AS-R2/AS-R4; the [installation decision](adr-skill-installation.md) satisfies AS-R3/AS-R4. These are proposed citations and must become accepted numeric ADR references before this becomes normative ADD content. No current accepted architecture is overridden.

## Components and dependencies

`ai-skills` contains canonical text assets and an embedded inventory; it exposes Names/Tree/Digest. Deterministic catalog/manifest/semantic-convention/invocation descriptors originate in the domain validators/types and drive committed references and a pin. Domain packages never import the skill package, avoiding dependency cycles.

`internal/skillinstall` owns destination resolution, pure inspection, portable digest/marker classification, plan/apply and filesystem-local recovery. It uses portable lock/atomic-file helpers and owns no Work administrative state. `internal/cli/skills.go` validates flags, composes the service and renders results through existing CLI conventions; it never invokes bootstrap, mutation preflight or repository discovery.

## Data and flows

See [data model](../data-model.md) for ownership/operation records and exact digest framing. [Installer contract](../contracts/skill-installation.md) defines preflight, ordered locking, stage/backup publication and recovery. [CLI contract](../contracts/cli-work-skills.md) defines scope, output and errors. Pure status observes coordination and tree identity around hashing with at most three attempts and never opens a writable helper.

## Delivery verification

[Guidance contract](../contracts/skill-content.md) separates parser, full discovery, selected installability and runtime behavior checks. Fault/restart/concurrency fixtures, generated-output/pin checks, three-OS matrix and actual agent dogfood realize AS-R3–AS-R5. This section adds no new product requirement.
