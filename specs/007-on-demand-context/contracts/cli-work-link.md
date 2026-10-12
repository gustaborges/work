# Contract: `work link` (F6)

Authority: spec FR-007–FR-017, FR-026–FR-031; PRD FR-31–FR-35; ADD §9, §11–§12.

## Grammar

```text
work link [LINKER] [VALUE]
```

Zero arguments collects component and value interactively. One argument resolves that Linker and collects only the value. Two arguments resolve and associate directly. More than two arguments fail usage. VALUE is one text argument; users quote shell spaces/metacharacters normally. A value beginning with `-` can follow the argument separator: `work link LINKER -- VALUE`.

`--json` is rejected even when explicitly set false. No `--yes` or explicit Work target is added. Both current active Work and a manually available Linker are required.

## Selection and value collection

Only enabled Linkers declaring manual availability appear. Sort choices by qualified identity, show manual display name/description plus alias/name, and keep alternative providers of one semantic key. Explicit bare names must be unique among enabled manual Linkers; qualified identity selects exactly one. Discovery inputs, subscriptions, Starter filters and automatic discovery flags do not gate association. No executable or runtime preflight is needed for association.

For omitted arguments in an interactive terminal, use one journey with selector then value field; skip explicit choices. Empty text fails field validation and permits correction. A supplied empty direct value fails usage without falling back to a prompt. Values are preserved without trimming or URL normalization. Accepted value receipts are redacted as `value supplied`; normal diagnostics/stdout contain no link value.

With non-interactive input, both arguments are mandatory even if one candidate exists. Missing arguments fail before mutation, with guidance showing the direct form. Cancellation before commitment exits 20 with no link/provenance mutation.

## Commitment

Reload the selected provider; check enablement, role, manual availability, declared key and namespace ownership. Under the existing per-Work lock, read/validate the latest snapshot, verify the expected identity and active location, and replace only `links[key]`. Commit through the existing atomic writer. Preserve unrelated concurrent updates and recent access.

Record current source with qualified component, operation `link` and UTC recording time in the existing provenance index. Snapshot commitment defines success; provenance-only failures retain F5 diagnostic semantics and cannot undo the value. Repeating the same value is a valid publication and refreshes its current source/time when provenance is available.

Exactly zero plugin processes run. No discovery, lifecycle event or importer is triggered. Neither the current directory nor shell handoff changes.

## stdout — success

```text
work: linked <id> <semantic-key> <alias/name>
```

One stable line; no value. It is printed only after canonical commitment. Omitted interactive choices leave accepted receipts on the UI channel before this result. On failure stdout is empty.

## Failures

See [shared manual contract](manual-context.md) for the complete table. Applicable categories are usage (2), unreadable global context (16), cancellation (20), missing current Work (21), archived Work (22), invalid snapshot (25), unavailable provider (39), ambiguous provider (40), no eligible provider (41), and failed canonical link publication (43). A provenance-only failure after commitment preserves exit 0 and is available under `WORK_DEBUG`.

Unknown/disabled/wrong-role/non-manual components fail without fallback. Empty candidate sets give guidance to install/enable a package with a manual Linker. Link write/lock failure identifies the Work and selected provider without echoing its value.
