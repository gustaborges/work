# CLI Contract: work skills

**Authority**: Conditional on S0 approval of [plural noun decision](../governance/adr-skills-command.md). Freeze command names and numeric diagnostics only after merged 009 reconciliation. This draft is internally precise, not accepted authority.

## Grammar and validation

```text
work skills install [SKILL...] (--global | --dir PATH) [--vendor claude|codex]... [--force]
work skills status  [SKILL...] (--global | --dir PATH) [--vendor claude|codex]... [--json]
```

Register in `groupSetup`. Bare `work skills` displays group help; no selection UI/implicit install. Root help uses live command registration; static brand home has no inventory change. No aliases and no implicit scope, including in a TTY.

- Omitted SKILL means all embedded skills (initially work-plugin-builder). Omitted vendor means both; repeat values deduplicate, sort vendor then skill. Reject explicit empty values, unknown identities and path-like skill operands. Cobra missing-value/unknown-flag failures and neither/both scope are usage/2.
- `--dir` must already exist and be a directory. Missing catalog emits one escaped-path informational stderr note, never validates the catalog or fails installation/status.
- `install --json` fails usage/2 before any destination/lock/recovery creation; root-inherited flag handling must explicitly reject it. `status --force` is invalid.
- Validate every selection/scope before writes; inspect safety/overlap before creating coordination parents. No bootstrap/mutationPreflight/repository discovery, Work home/index/registry access or Work/admin lock.

## Destinations

| Vendor | Global | Folder D |
| --- | --- | --- |
| claude | `${CLAUDE_CONFIG_DIR:-HOME/.claude}/skills/SKILL` | `D/.claude/skills/SKILL` |
| codex | `HOME/.agents/skills/SKILL` | `D/.agents/skills/SKILL` |

HOME uses os.UserHomeDir, not hard-coded Unix home. Empty/unset CLAUDE_CONFIG_DIR defaults; relative values become absolute against initial CWD, never shell-expanded. Select the anchor once and canonicalize existing prefix safely: D for folder, configured Claude config dir for global Claude, user `.agents/skills` root for global Codex. Permit explicitly selected linked anchors only by resolving and recording their canonical location; reject linked descendants and target/marker links. Missing suffix creation belongs only to install and requires revalidation. See [installation safety](skill-installation.md).

## Human installation output

Exactly one deterministic stdout receipt per selected target in vendor/skill order. Use fixed named fields and Go/JSON-style quoted escaped paths; control characters never reach terminal literally. Suggested stable form (freeze at S0):

```text
installed vendor=claude skill=work-plugin-builder path="/tmp/repo/.claude/skills/work-plugin-builder" commit=committed
unchanged vendor=codex skill=work-plugin-builder path="/tmp/repo/.agents/skills/work-plugin-builder" commit=none
```

Actions: installed/updated/unchanged/failed/not-attempted. A force replacement of existing ordinary foreign/modified content is updated. Successful commit with cleanup error retains installed/updated and commit=cleanup-pending with error=skill-cleanup-pending. Failed rollback/inconsistent evidence uses failed, commit=recovery-required. No false rollback/unchanged receipt after commit.

Initial whole-batch preflight errors report failing target(s) and all other targets not-attempted; no payload changes. During apply, stop after the first failed/cleanup-pending target in target order, retain earlier success receipts, mark later targets not-attempted. Any outcome error fails overall. Notes/errors/agent hints use stderr, preserving one stdout result line per target. Pre-validation usage failure produces no target receipts.

On successful selected-agent installs (including unchanged), hints identify `/work-plugin-builder` for Claude Code and `$work-plugin-builder` for Codex. Claude: watched existing directories reload automatically; use `/reload-skills` when a top-level skills directory was newly created in-session, repeating after subsequent edits there. Codex: automatic detection; restart if missing. Do not promise same-name scope merging/precedence beyond verified vendor behavior. Warn of a failed target through its actual remedy, not a success hint. Folder payloads may be committed to the marketplace; coordination files are local-only; no git or ignore-file mutation.

## Status output and exits

Human status is one state line per target. JSON stdout is one non-null array, vendor then skill, no receipts/hints/notes; each row has exactly required fields plus optional error:

```json
[
  {
    "vendor": "codex",
    "skill": "work-plugin-builder",
    "path": "/tmp/repo/.agents/skills/work-plugin-builder",
    "state": "absent",
    "embedded_digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
    "recorded_digest": null,
    "actual_digest": null
  }
]
```

The illustrative digest is not a released constant. States are absent/current/outdated/modified/foreign/conflict/recovery_required/unreadable. See [data model](../data-model.md) for safe nullable evidence. Failed rows carry a stable error token, never a raw cause. Retain successfully inspected rows on operational failures. Ordinary absence/current/outdated/modified/foreign exit 0. Structural/recovery/I/O conditions exit nonzero. CLI usage failure occurs before results. No directory/lock creation, bootstrap, repair or cleanup. Retry coherent inspection at most three times; continuing instability reports recovery_required and a safe recovery/busy diagnostic, never current.

## Diagnostics and aggregation

Current 009 uses 57–61; the following provisional additive table starts at 62. S0 must reallocate if occupied and update code/fixtures/docs atomically. Existing usage/cancellation conventions remain.

| Provisional code | Token | Meaning/remedy |
| --- | --- | --- |
| 62 | skill-target-conflict | Modified/foreign without force, unsafe path/type, overlap or observed target changed. Force is a remedy only for initial ordinary modified/foreign content; never changed/unsafe paths. |
| 63 | skill-install-busy | Persistent local lock timeout. Retry after other installer; cancellation uses existing category. |
| 64 | skill-recovery-required | Pending/inconsistent record or unresolved coherent-read evidence. Explicit install retries owned recovery; inconsistent evidence requires preserving/moving it only after inspection and no active installer. |
| 65 | skill-cleanup-pending | New target committed; cleanup incomplete. Re-run explicit install to finish owned cleanup. |
| 66 | skill-io-failed | Read/write/sync/move failure. Check access/storage; never infer absence or suggest force. |

For status, aggregate by recovery-required, then conflict, then I/O precedence, retaining every row. For install, the first apply failure in target order controls exit, except unresolved recovery takes precedence over underlying I/O; an already committed cleanup failure always uses cleanup-pending. Initial preflight failure aggregate uses the same recovery/conflict/I/O precedence. Freeze against the merged diagnostic table and preserve raw technical causes for explicit debug only.

## Accepted authority reconciliation

S0 approved by the owner on 2026-11-14 against develop revision 161cf233ef1eee8836fcde02cbc5f6433f65d5f4. PRD FR-65–69, ADR-0031–33 and ADD-0001 are authoritative. Plural skills grammar, receipt fields and diagnostic codes 62–66 are frozen. Earlier conditional wording records planning history and no longer represents a pending approval.
