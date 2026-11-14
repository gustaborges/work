# Skill Content and Contract Maintenance

**Governing decision**: [bundled skills](../governance/adr-bundled-skills.md). **Requirements**: FR-001/004–006/017–021.

## Canonical payload

`ai-skills/work-plugin-builder/SKILL.md` uses only name and description frontmatter. Trigger covers creating/editing/reviewing/validating Work marketplaces, manifests, all four roles and branch conventions. Body is vendor-neutral: no tool names, vendor invocation syntax or `$ARGUMENTS`. References use actual relative Markdown links; comments are not includes. No agents/openai.yaml or overlays in v1.

Workflow: classify request and read existing catalog → choose role before JSON → scaffold cohesive self-contained packages → implement declared-runtime entrypoints → recapture/validate every entry → explain Git publication/qualified consumer install. One plugin is the atomic package maintenance unit. Bump opaque version on behavioral changes. Respect reserved builtin identity and exact normalized official descriptor; a catalog name alone never grants official status. Never invent manifest fields or execute components merely to validate discovery/installability.

References: marketplace-catalog.md, plugin-manifest.md, component-roles.md, validate-and-publish.md; generated catalog-schema.md, manifest-schema.md, role-matrix.md; complete marketplace/plugin templates and text runtime entrypoints. All four roles need manifest/invocation examples, including Importer output_dir and Linker owned keys. Inventory includes intentional dot/underscore files and rejects nested modules, binary assets, symlinks/special nodes.

## Descriptors / generation / pin

- Share actual validator definitions: manifest roleRules (including anyOf), keys, valid roles, inputs/on/discover grammar, events and conventions; catalog keys/required fields/path/name rules; identity/reserved names; semantic-convention facts and qualified ownership; IPC actual request/response field names/types and shared constants.
- Extract inline catalog lists into values consumed by both validation and Contract(), with no new validation behavior. Return deep immutable copies and canonical sorted output. IPC descriptor traverses actual Go structs/tags and models role request/response variants; behavioral fixtures remain separate.
- `go generate ./ai-skills/...` writes committed generated references via `ai-skills/internal/generate/main.go`. Normal build embeds committed output without generation/network.
- CONTRACT hashes canonical versioned descriptor serialization (including protocol descriptor). `make skill-lock` invokes `ai-skills/internal/skilllock/main.go` to explicitly update only the pin after human review of guidance/examples. TestSkillContractPinned and byte comparison against in-memory rendered generation never write files.
- Failure message: “the Work contract changed; update ai-skills/work-plugin-builder (references/ and templates/), then run make skill-lock”. Independent role/catalog/IPC mutations must prove gates fail. Blind re-pinning is possible; never claim proof of meaningful prose revision.

## Fixture routing

Templates' complete JSON files and fenced reference JSON explicitly identify catalog/manifest parser. Marker forms: `<!-- fixture:valid parser="catalog|manifest" -->` and `<!-- fixture:invalid parser="catalog|manifest" reason="stable-reason" -->`; values use one parser, not the literal union. Invalid reasons match stable parser outcomes. Partial/IPC examples carry explicit fixture kinds and their own validators; unmarked full JSON blocks fail inventory checking instead of being silently skipped. Scripts are never fed to JSON manifest parsers.

Catalog parser strictness and manifest behavior are documented separately. Complete marketplace fixtures run BuildOffer/name/path/containment validation, then selected-install each entry in an isolated home with declared test runtimes. Missing entrypoint/runtime tests belong to selected validation, not claimed catalog rejection. Native IPC fixtures cover invocation behavior separately; management fixture paths execute zero components.

## Complete isolated validation recipe

There is no validate subcommand. Repeated add of the same registered source is unchanged and does not recapture edits. Each attempt starts new scratch homes after each source edit. Verify expected catalog name and exact offered set from this source, ignoring unrelated builtin offers. Safely enumerate every entry from parsed catalog/offer; never use shell eval or whitespace splitting for qualified names/paths.

For each entry, a fresh subshell/scoped environment: create empty temporary WORK_HOME; register source; inspect available JSON; assert expected source/entry; selected-install exactly `plugin@marketplace`; preserve command failure if cleanup also fails; finally remove only owned scratch home. Separate entries allow valid alternative fallback providers. POSIX and separately tested PowerShell recipes restore any previous WORK_HOME and handle success/failure/cancellation. Never export temporary state into the continuing caller session. Include this one-entry POSIX building block, with safe concrete substitutions by the agent:

```sh
(
  set -eu
  validation_home=$(mktemp -d)
  trap 'rm -rf -- "$validation_home"' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  export WORK_HOME="$validation_home"
  work plugin marketplace add .
  work plugin list --available --json
  work plugin add 'github@team'
)
```

Repeat the complete block for every actual entry; the example identity is illustrative. Required recipe tests: later-entry failure, alternatives, missing runtime/entrypoint, corrected-source recapture, offline local source, no component invocation, real WORK_HOME/source unchanged, pre-existing environment restoration and scratch cleanup including cancellation. Runtime smoke checks are explicit separate tests; installation alone proves no runtime behavior.

## Maintenance / documentation

After S0, add the proposal's same-PR maintenance rule outside managed Spec Kit AGENTS markers; CLAUDE imports AGENTS. Cover catalogs, roles/keys/grammar/events/semconv/IPC/packaging/validation/reserved and official identity/install commands. No hand-editing generated references; use go generate, review prose/templates, then local skill-lock. PR template asks about contract/protocol changes and references/templates/pin/docs updates. docs/agent-skills.md lists inventory/targets/update/status/local-only coordination and safe recovery/removal limitations; docs/marketplaces.md links authoring guidance; README lists real commands. Binary updates do not refresh installed copies automatically. CI only verifies committed artifacts, never re-pins or auto-generates in place.

## Accepted authority reconciliation

S0 approved by the owner on 2026-11-14 against develop revision 161cf233ef1eee8836fcde02cbc5f6433f65d5f4. PRD FR-65–69, ADR-0031–33 and ADD-0001 are authoritative. Plural skills grammar, receipt fields and diagnostic codes 62–66 are frozen. Earlier conditional wording records planning history and no longer represents a pending approval.
