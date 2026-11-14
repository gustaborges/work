# Validation evidence

## S0

2026-11-14: Owner confirmed develop baseline and explicitly approved the drafted governance package. Feature 009 merged via PR #58 at 161cf233ef1eee8836fcde02cbc5f6433f65d5f4, identical to HEAD and develop. PRD FR-65–69, ADR-0031–33 and ADD-0001 promoted in authority order. CLI skills grammar and receipt format accepted; diagnostic slots 62–66 verified available. No application tests yet.

## Implemented and validated locally

- Linux Go 1.26: digest framing golden, modes/hidden assets/unsafe nodes/empty directories; detached validator and IPC descriptors; generated-reference and pin byte checks.
- Full scaffold discovery and every offered entry selected installation through marketplaceadmin/pluginadmin, with declared Python runtime and no component invocation in management paths.
- Installer ownership/current no-op/foreign force/batch refusal/observed edit tests; status state/purity matrix; same-process concurrency and cancellation.
- Native helper-process abrupt exits at prepared/old-moved/new-moved/committed boundaries, both initial and existing trees. Every discovered staged-file/publication checkpoint tested, plus interrupted compensation and preservation of mismatched backups/injected records.
- `go test -race ./internal/skillinstall ./ai-skills/...` passed. `make seed-all` passed all five release platforms; this is cross-compilation evidence, not native macOS/Windows execution. `make lint` passed with GOCACHE and STATICCHECK_CACHE redirected to /tmp.
- Built CLI smoke: two installed receipts, repeated unchanged receipts, ordered current JSON rows, byte-identical Claude/Codex payloads and no configured WORK_HOME creation. Sandbox: /tmp/work-skills-smoke-mvnnoerq.
- Vendor documentation rechecked: https://code.claude.com/docs/en/skills and https://learn.chatgpt.com/docs/build-skills. Real reload/override exercise is separate.

## Outstanding delivery evidence

Native macOS/Windows portability/fault runs, full POSIX/PowerShell per-entry recipe cleanup/cancellation matrix, complete semantic/IPC fixture routing, and real agent authoring/discovery/reload exercises remain unchecked. Claude Code 2.1.325 and codex-cli 0.159.0 are installed; clean isolated configuration authentication checks returned loggedIn=false / Not logged in. No successful real-agent authoring evidence is claimed. Broader path/case/alias/I/O and status churn matrices, represented-descriptor perturbation gate assertions and detailed retained debug causes still need coverage/completion as recorded in unchecked tasks.

The feature is not release-complete. Any PR must remain draft until remaining tasks and acceptance evidence are resolved. No before_implement or after_implement hooks are configured in .specify/extensions.yml.

## Final local checks

Full host `go test ./...` passed (integration package 254.494s), with temporary Git fixture signing disabled through environment configuration; repository commit signing remains enabled. Final focused suite including CLI/domain/assets/installer passed. Final make lint passed. Generated references and pin were regenerated/reviewed after annotating the existing LinkerResponse wire field; non-mutating verification passed.

Authenticated existing Claude/Codex profiles are available. Clean profile checks above remain true, but are not evidence that existing profiles cannot run. The actual Claude authoring attempt was rejected by automatic approval review because exporting the installed skill and temporary workspace to the external AI service requires explicit user approval. No model request ran, and no alternative agent request bypassed the rejection. Approval for both exercises has been requested.

## Owner-approved CLI experience revision

2026-11-14: Revised agent skills PRD FR-66 and accepted ADR-0034 superseding ADR-0032. Both commands require one vendor and local/user scope, take no skill operands and select the complete release inventory. Both install scopes confirm unless --yes; force remains independent. Renamed the bundled skill to work-cli-plugin-builder; existing copies under the prior name are not automatically migrated or deleted.

- CLI tests cover both vendors × both scopes, full inventory/current status, unselected-vendor absence, Work-home purity, invalid/missing/repeated/all vendor selection, obsolete scope flags, forbidden skill operands and install JSON/status mutation flags.
- Confirmation previews cover multiple skills/destinations and force disclosure; --yes preserves local edits without force and --force --yes replaces them. Non-interactive unconfirmed installs create no target or coordination files.
- Relative Claude user configuration and local current-directory anchors are tested without writes.
- Reviewed renamed references and complete scaffold templates. `go generate ./ai-skills/...` and explicit `make skill-lock` completed; all three generated reference files and the represented contract pin remain byte-identical to the prior tree. Only the skill name/frontmatter changed in the delivered content.
- `make lint` passed with Go/staticcheck caches under /tmp. Fixture Git signing was disabled through process-only environment configuration for the test run; repository signing configuration was not changed.
- Full host `go test ./...` passed, including interactive accept/decline checks for both local and user scope (integration package 195.943s). Declining leaves target roots and coordination artifacts absent. Final focused installer suite also passed after adding the Claude override anchor case. These results validate this CLI revision; the pre-existing feature-wide delivery gaps above remain separate.
