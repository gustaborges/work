# Contract: `work status` (F6)

Authority: spec FR-001–FR-006, FR-028–FR-031; PRD FR-38 and FR-52; ADD §12; ADR-0019.

## Grammar and resolution

```text
work status [WORK] [--json]
```

WORK is the complete opaque canonical Work ID, matching the daily-cycle identity contract. Explicit targets override current context. No target means the nearest enclosing canonical Work directory (an immediate child of an `in-progress` or `archived` lifecycle area), including checkout/artifact descendants or an archived Work directory. An ordinary tracked/imported file named `work-state.json` does not create a nested context. No target selection is offered. More than one argument or an unsupported flag is usage failure.

Resolve canonical snapshots without bootstrap, reconciliation, migration, locks or index writes. Explicit IDs scan configured lifecycle roots; an existing index may supply an optional read-only path hint, verified against the canonical ID. Unknown IDs never become slug, branch or path searches. Reads of old-workspace Works from inside their directory are supported; an outside lookup cannot discover an unconfigured old root when its index hint is unavailable (research R2).

Call full snapshot validation. Archived Works are valid. A missing checkout does not invalidate canonical inspection; reported location is recorded/derived location, not a checkout health claim.

## stdout — exit 0

Human form, with bytewise-sorted link keys:

```text
work: id 01K00000000000000000000000
work: status in-progress
work: branch feature/context
work: path /absolute/workspace/in-progress/project_feature-context/worktree
work: link github.pull_request https://example.test/pr/212
```

For an archived Work, status is `archived` and path is its archived directory. When there are no links, replace link lines with:

```text
work: links none
```

Human values containing newlines, tabs or terminal controls must be rendered as quoted escaped text so each record remains one line and cannot inject terminal controls. Ordinary printable values retain the form above. Do not truncate saved keys or values. This escaping is presentation only; it does not change saved data.

`--json` emits exactly one object plus a trailing newline, without a prefix:

```json
{
  "id": "01K00000000000000000000000",
  "status": "in-progress",
  "branch": "feature/context",
  "location": "/absolute/workspace/in-progress/project_feature-context/worktree",
  "links": {
    "github.pull_request": "https://example.test/pr/212"
  }
}
```

All five fields are required. `links` is `{}` when empty; values preserve exact saved text through JSON escaping. Object member order and formatting whitespace are not machine contracts. No metadata, provenance or live extension information is included.

## stderr and failures

Successful status has no progress, warnings, prompts or receipts. Errors use the existing single diagnostic border; JSON does not change the error envelope. Failure produces no partial status result on stdout.

| Code | Token | Condition |
| --- | --- | --- |
| 0 | `ok` | Complete canonical view emitted |
| 2 | `usage` | Invalid arguments/flags |
| 16 | `bootstrap-failed` | Required global location/configuration cannot be read; no repair attempted |
| 20 | `cancelled` | Interrupted before result |
| 21 | `target-not-found` | No current Work or exact identity could be found |
| 25 | `snapshot-unreadable` | Target snapshot invalid/unreadable, duplicate identity, or configured canonical search area inaccessible |

Missing context explains entering a Work or passing an explicit identity. A malformed enclosing snapshot must not cause fallback to another ancestor. An explicit target with invalid data fails without substituting current context.

## Purity invariants

No Work, registry, configuration, index, sidecar, provenance, access timestamp, lock file or shell handoff file is created, rewritten or repaired. No plugin, Git subprocess or TUI runs. Legacy snapshot schemas are never upgraded by status. The immutable index reader is an optional untrusted path hint, never a source for displayed state. Filesystem access-time effects of ordinary reads are outside application mutation semantics.
