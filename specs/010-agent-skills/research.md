# Research: Agent Skills

> Historical planning record. Current CLI scope, vendor, collection and confirmation behavior is governed by [ADR-0034](../../docs/adr/adr-0034-complete-skill-collection.md).

**Date**: 2026-11-14. Read-only code research and vendor-documentation research were delegated under speckit-plan Phase 0. No application code or tests ran.

## R1 — Dependency and authorities

**Decision**: Draft feature 010 against local 009, gate implementation on merged development baseline and authority approval. **Evidence**: current HEAD `41c0e2c68ca30caca087f5c98752a86d18845f6c`; `feature/marketplace` `e8cb3193d9ccea5c72a48424bd7e5129ff7aa0d8`; primary `master` `3fa2790fb529096c4dd5defc53db870bf1751c14`. Feature 009 and ADR-0022–0030 exist only in the dependency branch inspected. Diagnostics 57–61 are used there. **Rationale**: current worktree cannot supply the intended command surface. **Alternatives**: merge during planning, reuse 009, or freeze 62/ADR-0031 unconditionally; rejected. [Governance drafts](governance/README.md) prepare S0 without accepting decisions.

## R2 — Vendor discovery and reload

**Decision**: Local Claude Code and Codex only; common name/description frontmatter; no optional overlay metadata. Codex locations `$HOME/.agents/skills` and folder `.agents/skills`; Claude default `~/.claude/skills` and folder `.claude/skills`. **Sources checked 2026-11-14**: [OpenAI Build skills](https://learn.chatgpt.com/docs/build-skills.md), [Claude skills](https://code.claude.com/docs/en/skills), [Claude environment variables](https://code.claude.com/docs/en/env-vars). Codex auto-detects edits; restart if missing. Claude watches existing skill directories; when the top-level skills directory was created mid-session, use `/reload-skills` and repeat for subsequent changes there. Claude personal skills take precedence over same-name project skills; Codex may expose both, with no merge promise.

`${CLAUDE_CONFIG_DIR}/skills` is an inference from Claude's configuration-root override, not an explicitly documented skills subpath in the environment page. Retain an actual override discovery exercise at S4. Relative override resolution against initial CWD is Work policy, not a claimed vendor guarantee. Empty override uses default; do not shell-expand values. **Alternatives**: legacy `.codex/skills`, blanket restart instructions and cloud-agent promises; rejected.

## R3 — Actual parsers are different

**Decision**: Separate catalog/manifest descriptors and fixture routes. **Evidence on 009**: `internal/marketplace/catalog.go` ParseCatalog rejects duplicate keys at all depths, nulls, unknown fields, future schema, case-folded identity collisions and unsafe/overlapping paths. `plugin.Parse` in `internal/plugin/manifest.go` uses JSON maps: duplicates overwrite and null component/convention collections decode as empty. It rejects unknown and role-forbidden fields, but does not share every catalog strictness rule. **Rationale**: guidance must describe real contracts, not silently harden validators. **Alternative**: one generic strict-JSON reference; rejected.

## R4 — Discovery is not installability

**Decision**: Fresh scratch home for each selected catalog entry on every validation attempt. **Evidence on 009**: marketplaceadmin.PrepareAdd (`internal/marketplaceadmin/sources.go`) returns unchanged for already registered matching source/kind/branch. BuildOffer/ValidatePackage passively validate all packages but tolerate missing entrypoints/runtimes. plugininstall.Validate (`internal/plugininstall/prepare.go`) and PreflightRuntimes (`register.go`) enforce selected installation. Orchestration is pluginadmin.PrepareInstall/InstallPlan.ApplyWithOptions (`internal/pluginadmin/install.go`); the old plugininstall/install.go does not exist there. **Rationale**: recapture edits and avoid cross-package fallback-provider conflicts. **Alternatives**: register once/retry, or install all entries together; rejected. Native fixtures test runtime behavior separately; no management step executes code.

## R5 — Contract descriptors

**Decision**: Extract immutable sorted descriptors from shared validator definitions, not parallel lists/source hashes. **Evidence**: manifest roleRules/allComponentKeys/topLevelKeys/conventionKeys/validRoles/acceptsVocab/inputsGrammar/CoreEvents; Importer requires on or manual, Linker discover or manual. Catalog key lists need extracting from inline validation into shared sets. Include `internal/pluginid/identity.go` identity rules and `internal/semconv/semconv.go` Facts/ValidKey/ValidatePublished/ValidateOwnInput. Qualified private key forms on 009 differ from current tree: publishing owner, cross-provider owner@marketplace, storage-only registration suffix, and encoded dots must be represented accurately. Derive IPC fields from actual `internal/ipc/ipc.go` types/constants. **Alternative**: hashing source files; rejected as refactor-sensitive. Descriptors cannot represent all semantics; fixtures/review remain mandatory.

## R6 — IPC behavior fixtures

**Decision**: Role-specific invocation guidance and validators. **Evidence**: direct exec.CommandContext, not shell; stdout limit 1 MiB, stderr tail 4 KiB. Linker permits empty stdout/empty object/absent or null value, rejects present empty/nonstring value; Importer permits empty stdout or one object and writes via output_dir. Starter/Locator decoding differs. **Rationale**: reflected fields cannot express behavior. **Alternative**: treating all roles as one response parser; rejected.

## R7 — Independent portable digest

**Decision**: work-skill-tree-v1 with explicit framing, modes/mtimes excluded, root marker excluded, all other hidden assets included. **Evidence**: marketplace PackageDigest includes executable bits/symlink content; embed.FS regular files are 0444 and Windows cannot preserve Unix bits. **Alternative**: reuse package digest; rejected. Assets are regular text files/directories only; explicit embed inventory covers dot/underscore assets; declared runtimes avoid executable-bit dependence.

## R8 — Coordination and recovery

**Decision**: Persistent sibling locks in canonical order, full preflight, observed-content revalidation, owned stage/backup and prepared/committed record, per-target commit. **Evidence**: lockfile.AcquireContext creates parents/lock and has DefaultTimeout 30 seconds; it cannot be used in status. atomicfile replaces files, not nonempty directories; Windows replacement uses MoveFileEx/write-through/retry and directory sync is no-op. **Alternative**: one directory swap or Work administrative journal; rejected. Reuse atomic-file support where available, test process interruption and retain evidence when recovery cannot prove ownership. See [failure contract](contracts/skill-installation.md).

## R9 — CLI and outputs

**Decision**: Conditional plural skills setup command; explicit scope, all vendors default, status JSON only. **Evidence**: root command GroupID groupSetup and live help; no global bootstrap hook in inspected CLI. Static brand is separate. Dependency marketplace mutations explicitly support JSON despite an outdated root comment, so rejection is specific to skills install. Proposed diagnostics start after merged 009 allocation; freeze at S0. **Alternative**: silently override singular ADR-0019 or assume all mutations reject JSON; rejected.

## R10 — Drift enforcement and scope

**Decision**: Committed deterministic references, non-mutating verification, explicit local skill-lock, real parser/full-tree/isolated-install fixtures, same-PR review rule. **Rationale**: detect represented drift while honestly retaining prose/behavior review. No marketplace validate, removal, overlay, network distribution or agent service invocation added to Work. Real agent dogfood is a future manual delivery check; unavailable sessions cannot be replaced by invented evidence.

## Resolution

No unresolved technical clarification remains for draft planning. S0 authority acceptance and merged dependency reconciliation are explicit execution prerequisites, not assumed answers. Vendor discovery under override requires S4 evidence. No tests or dogfood results are claimed by this research.

## S0 reconciliation (2026-11-14)

Owner approved develop baseline and governance package. HEAD and develop are 161cf233ef1eee8836fcde02cbc5f6433f65d5f4 (merged PR #58, feature 009). Actual parsers, plugin identity and semantic ownership match R3–R6. Diagnostics 57–61 remain occupied; reserve 62–66 for skills. Promote requirements as PRD FR-65–69; decisions ADR-0031–33. ADR-0032 explicitly supersedes only ADR-0019’s singular-resource rule for skills. Output examples and recovery contracts remain unchanged.
