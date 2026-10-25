# Contract: Repository Administration

## Surface and purity

Retain existing direct grammar:

```text
work repository
work repository locator list [--json]
work repository policy list [--json]
work repository policy add <LOCATOR> [--before <LOCATOR> | --after <LOCATOR>]
work repository policy remove <LOCATOR...>
work repository policy move <LOCATOR> (--before <LOCATOR> | --after <LOCATOR>)
work repository policy replace <LOCATOR...>
work repository root list [--json]
work repository root add <PATH...>
work repository root remove <PATH...>
work repository root replace <PATH...>
```

Bare repository opens the hub only with interactive streams; otherwise usage/2 explains direct commands. Read commands use guarded pure snapshots and never bootstrap defaults or create home directories/locks/index/cache. An empty fresh home produces empty list collections, not a synthetic installed default. Known legacy seed evidence may be projected without writes. Mutations reject --json and retain existing stable result lines and no-confirmation direct behavior; hub review is an additional interactive presentation of the same requested change.

## State views

Existing JSON envelopes/keys remain. Installed Locator inventory is raw, including disabled providers, sorted by qualified reference. Add fields installed, enabled, available and unavailable_reason. Membership does not affect availability; a runnable installed Locator outside policy is available but not selected by the strategy.

```json
{"locators":[{"ref":"acme/clone","display_name":"Team clone","description":"Local clone map","accepts":["name"],"in_policy":true,"installed":true,"enabled":false,"available":false,"unavailable_reason":"package-disabled"}]}
```

Policy retains stored order and 1-based positions. Add installed/enabled/unavailable_reason to existing ref/position/available keys. Do not discard missing/wrong-role entries.

```json
{"policy":[{"ref":"acme/clone","position":1,"available":false,"installed":true,"enabled":false,"unavailable_reason":"package-disabled"}]}
```

Reason is null when available; otherwise package-missing, component-missing, role-changed, or package-disabled, in the precedence defined in [data-model.md](../data-model.md). Registered legacy ownerless declarations retain established enabled behavior; known seed ownership is projected. Root view remains `{"roots":["/absolute/path"]}`. Arrays are empty, not null. Human views show the same states/order/reason and remain legible without color. No source credentials are printed.

## Policy editing

Persist qualified `<alias>/<component>` references. Bare new names resolve only if unique among raw installed Locators, including disabled packages; qualification resolves collisions. Existing remove/move/anchor resolution uses policy entries only, including unavailable entries. Never execute a provider to inspect or edit policy.

| Operation | Validation/effect |
| --- | --- |
| add | Installed Locator required; append by default; optional exactly one relative anchor; existing membership is a no-op |
| remove | Resolve all operands against policy; absent ref is no-op; ambiguous bare ref is usage error; package stays installed/enabled |
| move | Target and anchor must exist; exactly one before/after; self-anchor invalid; unavailable entries remain movable |
| replace | Resolve new entries against raw installed Locators; explicitly retain existing unavailable policy references; reject duplicate canonical refs; replace whole order only after complete validation |

Replacement resolution first accepts exact existing qualified refs; otherwise resolves a new installed ref or a unique existing bare unavailable name. If a bare token could name different installed/retained identities, fail ambiguity rather than choose one. Previously collapsing duplicates is intentionally superseded by F7 FR-008; grammar and stable output shape stay unchanged. No new clearing verb or zero-argument replacement: remove all entries explicitly to empty the policy.

No install/reinstall/enable/update inserts or reorders policy entries. One-time clean bootstrap is the only default insertion exception; previously configured or deliberately empty policy is not a reason to reseed. Removing approved dependencies during update/uninstall is governed by the plugin contract.

## Roots and mutation consistency

Reuse existing expansion, absolute-path/readable-directory validation, canonical deduplication and rejection of workspace overlap. Add/replace validate every operand before assignment. Remove clones the previous collection before filtering so a late invalid operand leaves the original intact. Repeated roots deduplicate under existing canonical rules. Removing an absent path is a no-op. Root changes never move Works or alter workspace location.

Direct/hub changes acquire the shared write lock, reload latest config/registry, apply one narrowly scoped operation and commit through administrative state. Rejected input changes nothing; reviewed hub dependencies must still match after lock acquisition. Do not save a captured stale whole config.

Stable stdout remains:

```text
work: policy now <comma-separated-qualified-references-or-(none)>
work: roots now <comma-separated-paths-or-(none)>
```

Reads exit 0 including empty lists. Validation/ambiguity uses usage/2, cancellation 20, malformed existing config bootstrap-failed/16; new busy/conflict/recovery/cleanup errors use codes 53–56. Existing Locator execution outcomes 26–30 remain unchanged. The strategy still skips unavailable/ineligible entries, continues on no result, stops on first valid nonempty result/ambiguity, and stops on operational failure.

## Verification

US2; FR-006–FR-010, FR-027–FR-033; Q1/Q3–Q4/Q10/Q12. Cover disabled new refs, wrong-role/missing retained refs, bare collisions, canonical duplicate rejection, invalid anchors, empty collections, late root failure, state/reason JSON parity, hub/direct equality and no implicit policy insertion.
