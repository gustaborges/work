# Contract: Plugin Administration

## Grammar and confirmation

```text
work plugin
work plugin list [--json]
work plugin install <SOURCE> [--link] [--as <ALIAS>]
work plugin install --embedded
work plugin enable <PLUGIN...>
work plugin disable <PLUGIN...>
work plugin update
work plugin update --check [PLUGIN...] [--json]
work plugin update <PLUGIN...> [--yes] [--remove-policy-references]
work plugin update --all [--yes] [--remove-policy-references]
work plugin uninstall <PLUGIN...> [--yes] [--remove-policy-references]
```

PLUGIN means exact installed alias, never a manifest name guess, prefix match or component name. Deduplicate/sort target aliases deterministically. Unknown aliases fail the whole request with usage/2 before retrieval or applying any target. Explicit aliases skip selection only. Untargeted update selects interactively; noninteractively it fails usage even with --yes. `--all` resolves the complete installed set once for review, including disabled packages and reported unsupported origins.

Enable/disable require aliases and no confirmation; repeat satisfied states successfully. Apply-update and uninstall require impact confirmation interactively or --yes noninteractively. Missing required confirmation fails usage before new changes. --yes never chooses targets or authorizes dependency removal. When computed policy dependencies exist, noninteractive apply/uninstall also requires --remove-policy-references; otherwise plugin-policy-dependency/52 with no changes. Interactive confirmation must name all exact affected references and expressly approve their removal. A flag does not authorize additional dependencies discovered after review: drift requires review again.

--check cannot combine with --all, --yes or --remove-policy-references. --all cannot combine with explicit targets. --json is accepted only by list and update --check; reject it on mutations and bare hubs. --embedded accepts no SOURCE, --link or --as and installs only the bundled package under its reserved alias offline. Explicit --embedded installation never seeds policy, including on a fresh home, and records that initialization has been handled. Other install grammar, alias collision behavior, linked versus pinned semantics, and existing output lines remain compatible.

## Inventory

Preserve the existing top-level JSON array and package keys alias/origin/reference/components/conventions. Add enabled (boolean), version (string|null), content_digest (string|null), inspection_status (ready|metadata-unknown|content-unavailable). Components retain name/role; arrays are empty rather than null. Packages are alias-sorted; components remain manifest order. Include disabled packages and known legacy seed ownership. Reads never migrate/backfill, bootstrap, create layout/locks or touch Work state.

```json
[
  {
    "alias": "acme", "origin": "remote-pinned",
    "reference": "https://example.test/acme.git@0123456789abcdef",
    "components": [{"name": "clone", "role": "repository-locator"}],
    "conventions": ["team"], "enabled": false, "version": "release-blue",
    "content_digest": "sha256:example", "inspection_status": "ready"
  }
]
```

The example digest/reference values illustrate field shape, not valid fixture identities. Retain legacy leading human alias/origin/reference columns and add labeled version/enabled state. Missing metadata displays unknown; never substitute live linked-source version for the registered baseline.

## Check results and origin matrix

Only explicit check/apply operations retrieve candidate content. Check is read-only for installed and persisted application state; OS-temp retrieval is allowed and must be removed on normal termination. Do not invoke components, fetch within active checkouts, persist caches or update timestamps.

| Origin | Check result | Explicit application |
| --- | --- | --- |
| remote-pinned | Clone recorded source's default revision; validate frozen manifest and runtime | Apply exact reviewed SHA/content if version differs |
| local-pinned | Copy recorded source to temp; validate frozen content | Apply exact reviewed digest/content if version differs |
| local-linked | source-managed | Report source-managed; explicit install SOURCE --link --as ALIAS refreshes registration |
| embedded-seed | not-applicable | Report not-applicable; use offline bundled lifecycle |

A valid different opaque version gives update-available. Same version gives unchanged even if code/reference changed. Pinned candidates must contain only regular content and contained relative symlink chains; absolute/escaping/cyclic links, special files and escaping entrypoints fail validation before manifest/entrypoint reads. Apply the same rule to pinned install, while --link retains developer-managed external-source semantics. No version ordering, release tags or commit-count heuristics. An invalid candidate reports failed, even with the same version. Unknown installed baseline for a managed pinned origin reports failed with explicit reinstall guidance; linked/embedded origins remain source-managed/not-applicable; do not guess. Known plugin manifest-name changes are invalid because they change private namespace ownership.

`update --check --json` returns:

```json
{
  "updates": [
    {
      "alias": "acme", "origin": "remote-pinned", "status": "update-available",
      "current_version": "release-blue", "candidate_version": "release-green",
      "current_reference": "https://example.test/acme.git@old",
      "candidate_reference": "https://example.test/acme.git@new",
      "current_digest": "sha256:old", "candidate_digest": "sha256:new",
      "policy_references_removed": ["acme/clone"],
      "error": null
    }
  ]
}
```

All scalar current/candidate metadata can be null when unknown/not applicable. `policy_references_removed` is an ordered impact preview only, never applied by check. Status is unchanged|update-available|source-managed|not-applicable|failed. A failed row has `error: {"token":"plugin-invalid","message":"candidate manifest is invalid"}` or the appropriate safe public category/message; no raw Git stderr. Other rows have error=null. Include one row per resolved target, alias-sorted. Empty inventory yields updates=[] and exit 0. Human view conveys the same metadata, status and dependency preview; render opaque values safely on one line.

Successful checks exit 0. One or more failed rows still produce the complete result and one final plugin-update-failed/50 diagnostic, exit 50. Structured stdout is valid even when exit is unsuccessful; stderr contains the one terminal diagnostic. Unknown alias/invalid grammar returns usage/2 without partial check results.

## Apply, enablement and removal results

After valid mutating grammar and required automation confirmation, a short locked recovery preflight makes interrupted state reachable before target lookup; it recovers only the prior authorized operation. Apply then prepares candidates once outside the lock, shows old/new versions/references and added/withdrawn declarations, and keeps bytes alive through review. At commitment, reload latest state and validate reviewed target revision/digest/version/enablement and exact policy impact. Merge unrelated current changes. Apply one package transaction at a time in alias order. Failures do not undo earlier independent commits or silently omit subsequent candidates. A changed target produces a conflict outcome requiring another review, never an implicit substitution.

Multi-target uninstall has one reviewed target/reference set and one transaction; restore the complete set on a recoverable precommit failure. Remove all policy references owned by selected aliases, including stale declarations, only when authorized. Update withdrawal removes only dropped/changed-role Locator references. Neither operation changes retained entries' relative order, repository convention memory or any existing Work content. Linked sources are never followed/deleted during removal.

Stable mutation stdout lines:

```text
work: enabled <ALIAS>
work: disabled <ALIAS>
work: update <ALIAS> <STATUS>
work: uninstalled <ALIAS>
```

Enable/disable repeat requests print the same satisfied result; no-op must not rewrite unrelated state. Apply statuses are updated|unchanged|source-managed|not-applicable|failed|not-applied. Per-target outcomes identify safe error tokens in an additional `work: update-error <ALIAS> <TOKEN>` line; the process boundary renders only one terminal failure. Print uninstall lines only after the complete set commits. Existing install receipt remains `work: installed <ALIAS> (<ORIGIN>)` and existing component receipt.

Cancelled batches preserve completed independent updates, mark unstarted targets not-applied, and exit cancelled/20. Failed candidates receive outcomes while remaining independent candidates proceed. Missing dependency authorization/confirmation fails before any new application. Pending/unrecoverable administrative state stops further application and reports remaining targets not-applied. Postcommit cleanup failure identifies that changes were committed, retains recovery evidence, and returns cleanup-failed rather than claiming rollback.

## Errors and output safety

Existing codes/tokens 0–49 remain unchanged. New categories:

| Code | Token | Meaning |
| --- | --- | --- |
| 50 | plugin-update-failed | At least one check/apply target failed; exhaustive results retained |
| 51 | plugin-uninstall-failed | Set removal failed before commitment and was compensated |
| 52 | plugin-policy-dependency | Dependency removal lacks explicit authorization |
| 53 | administration-conflict | Reviewed assumptions changed, or pure snapshot did not stabilize |
| 54 | administration-recovery-required | Pending/corrupt journal or compensation needs mutating recovery |
| 55 | administration-cleanup-failed | Committed state is coherent; owned cleanup failed |
| 56 | administration-busy | Existing administration lock acquisition timed out |

Usage/2 covers invalid flags/unknown aliases/missing confirmation; cancelled/20 covers precommit cancellation; plugin-invalid/31, alias-conflict/32, fallback-conflict/33 and install-failed/34 retain install semantics. Global recovery/cleanup errors take precedence over aggregate update failure; cancellation otherwise takes precedence over aggregate failure. Preserve normal interactive versus stable noninteractive rendering at the sole CLI boundary.

All text/JSON/receipts/review/errors use safe source projections. Remove URL passwords/userinfo credentials and query/fragment secrets; preserve useful host/path and source identity privately. Never interpolate raw Git args/stderr into public messages. Equivalent commands use placeholders when reproducing a sensitive source would disclose credentials. Control characters in author-provided values are escaped. WORK_DEBUG may expose retained technical causes only through the existing explicit diagnostic boundary; normal output remains safe.

## Verification

US1/US3/US4; FR-001–FR-005, FR-011–FR-024, FR-028–FR-032; Q1–Q2/Q5–Q10/Q12. Assert all origins, legacy unknown baselines, exact opaque versions, changed namespace, same-version changed content, unsupported exclusions, stable streams/codes, credential redaction, source drift, multi-target failures, dependency authorization, rollback/restart and durable seed removal.
