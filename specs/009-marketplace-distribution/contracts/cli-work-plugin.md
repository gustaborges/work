# Contract: Plugin and Marketplace CLI

**Governing decisions:** ADR-0025; ADR-0023/0026 for effects. **Stories:** US1–US5. **Requirements:** FR-001–FR-005, FR-008–FR-012, FR-015, FR-018, FR-020–FR-023.

## Grammar

```text
work plugin
work plugin add <PLUGIN[@MARKETPLACE]> [-m|--marketplace MARKETPLACE] [--yes] [--remove-policy-references] [--json]
work plugin list [-m|--marketplace MARKETPLACE] [--available --json] [--json]
work plugin remove <PLUGIN[@MARKETPLACE]...> [-m|--marketplace MARKETPLACE] [--yes] [--remove-policy-references] [--json]
work plugin enable <PLUGIN[@MARKETPLACE]...> [-m|--marketplace MARKETPLACE]
work plugin disable <PLUGIN[@MARKETPLACE]...> [-m|--marketplace MARKETPLACE]
work plugin update
work plugin update --check [PLUGIN[@MARKETPLACE]...] [-m|--marketplace MARKETPLACE] [--json]
work plugin update <PLUGIN[@MARKETPLACE]...> [-m|--marketplace MARKETPLACE] [--yes] [--remove-policy-references]
work plugin update --all [--yes] [--remove-policy-references]
work plugin marketplace
work plugin marketplace add SOURCE [--branch BRANCH] [--json]
work plugin marketplace list [--json]
work plugin marketplace upgrade [MARKETPLACE] [--json]
work plugin marketplace remove MARKETPLACE [--yes] [--remove-policy-references] [--json]
```

Bare plugin names always require -m even when unique. A qualified selector contains exactly one @ and cannot be combined with -m even if equal. Batches are all qualified or all unqualified under a shared -m; deduplicate exact targets preserving deterministic order. Validate the entire syntax/target set before source retrieval or state creation. --all rejects targets/-m; check rejects --all, --yes, and dependency-removal flags. Untargeted check inspects all installed targets; untargeted update opens selection and fails outside interactive streams. An untargeted check with -m checks all installed targets in that marketplace; it remains a non-persisting check.

Reject plugin install/uninstall, root `work marketplace`, --as/--link/--embedded, standalone path/URL/plugin.json passed to add, mixed forms, and unknown targets as usage/2 before retrieval/mutation. No compatibility translation is provided. Bare hubs require the existing interactive streams and emit no frames outside a terminal.

Only the grammar above is public. Superseded commands/flags are absent from the command tree, with no aliases, translation shims, or special transition selectors. Unknown syntax follows ordinary usage handling.

## Effects and confirmation

Registration captures/validates a whole offer and installs nothing. Identical registration returns unchanged without refresh. Upgrade named source or every known source (including local and builtin) refreshes discovery only, without --yes. Publish per source, keep independent successes, aggregate failures, and stop remaining work on cancellation/recovery. Identical canonical offer is unchanged without advancing bytes/epoch.

Add copies the exact chosen entry. Same identity/content is unchanged. Changed content is explicit reinstall, including equal opaque version: preserve enabled and review replacement/dependency effects. A new CLI installation determined by add needs no extra confirmation; reinstall requires interactive acceptance or --yes, plus --remove-policy-references if applicable. No add/toggle/refresh inserts policy.

Update/check obtains temporary fixed candidates from recorded provenance and validates them even when versions are equal. Different opaque version offers update, with no SemVer order. Apply only reviewed candidates without refetch. Update publishes per package; independent successful targets survive sibling failures. Check writes no Work-home files, locks, timestamps, or caches.

Plugin remove commits the exact set together. Marketplace remove commits registration, discovery references, all installed targets and authorized policy dependencies together, without network. --yes confirms impact, never grants policy removal or chooses targets. If reviewed source/set/dependencies change, return conflict/53 without widening the plan. Included-provider remove is usage/2 with stable reason `distribution-provided-marketplace`. Individual builtin plugins are removable. Online work removal is ordinary source removal.

## Inventory JSON

Plugin list uses one envelope:

```json
{"installed":[{"id":"github@team","name":"github","marketplace":"team","enabled":true,"version":"1.2.0","content_digest":"sha256:abc","inspection_status":"ready","source_kind":"git","source":"https://example.org/team/plugins.git","revision":"0123456789abcdef","components":[],"conventions":[]}],"available":[],"catalog_errors":[]}
```

All three arrays are always present and non-null. Available is populated only for --available, which requires --json. Installed rows use installed metadata, not current catalog versions. Nullable unavailable version/digest/revision/source fields are null, not fabricated. Every installed row has a real marketplace identity and actual provenance; there is no alternate row shape for development installations predating this contract. Components/conventions include full provider references. Sort marketplace rows by namespace/name; contributions preserve manifest order.

Available rows include id/name/marketplace/version/revision, description, components/conventions, and installed/enabled state (enabled null when not installed). They represent stored offers, not current network truth. Optional -m filters both inventories; an existing installed namespace remains inspectable if manually inconsistent source config is missing. Unknown filters are usage/2. Missing/inconsistent catalog adds a safe per-source catalog_errors entry and exit 60 while preserving valid rows; do not fetch/repair.

Marketplace list is an array with name, nullable display_name, source_kind (`git|local|builtin`), safe source, nullable branch/revision/plugin_count, catalog_status (`ready|missing`), installed_count, enabled_count. Totals are nonnegative local counts from the same administrative snapshot, include withdrawn entries, satisfy enabled<=installed, and do not depend on search filters. Missing catalog leaves totals known, plugin_count/revision null. Builtin is projected without home creation; work appears only after explicit registration. List does not claim network freshness.

## Mutation results and errors

Add JSON: id/name/marketplace/revision/content_digest/result (`installed|reinstalled|unchanged`). Plugin remove: result and removed IDs. Source add: name/result (`registered|unchanged`)/revision. Source remove: name/result/removed IDs. Upgrade: results array by source, each with name/status (`upgraded|unchanged|failed`), nullable revision/error. Results also carry committed boolean and cleanup_pending when relevant; committed results remain identifiable if process exit reports cleanup/recovery failure. JSON outputs one structure with safe per-target errors, never human receipts, frames, or raw Git stdout/stderr. --json does not imply --yes.

Human receipts include exact qualified IDs, e.g. `work: installed github@team` and `work: removed github@team`. Use purpose-named outcomes, including plugin-remove-failed/51 for removal. There is no requirement to preserve an unreleased command name, token, JSON shape, or result line. Unrelated current-domain errors remain consistent with their operations.

| Code | Token | Meaning |
| --- | --- | --- |
| 57 | marketplace-invalid | Catalog/schema/name/path/manifest invalid |
| 58 | marketplace-name-conflict | Different source or reserved included namespace |
| 59 | marketplace-source-unavailable | Source/branch/fixed revision/capture unavailable |
| 60 | marketplace-entry-unavailable | Required entry or discovery snapshot missing/inconsistent |
| 61 | marketplace-upgrade-failed | Aggregate refresh failures, retaining per-source results |

Codes 57–61 extend the intended error table; no migration-required category is included. Preserve usage/2, cancellation/20, current plugin failures 31–56, dependency/52, conflict/53, recovery/54, committed cleanup/55, busy/56. Cancellation and recovery/cleanup outrank batch aggregates. Discovery schema/paths fail 57; unavailable fixed content fails 59; unknown syntactic/registered targets are usage/2. All public text uses safe source projections and escaped passive metadata.
