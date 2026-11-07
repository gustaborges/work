# Contract: Plugin Manager Surfaces and CLI/TUI Parity

**Governing decisions:** ADR-0025, ADR-0020/0021 preserved boundaries. **Story:** US5. **Requirements:** FR-020–FR-023, FR-026; PRD FR-53–FR-64, FR-76, FR-81–FR-83.

## Session and presentation

`work plugin` starts on Installed; `work plugin marketplace` starts on Marketplaces in the same manager. These are the only top-level destinations. Sources are collection rows, not tabs. Remove Official, Install plugin, standalone source/alias/embedded/link controls. Preserve semantic theme, expressive focus/selection, hierarchy, primary identity versus secondary metadata, stable spacing, visible keyboard help, and recognizable destructive actions. Presentation receives safe display models, opaque identities, and callbacks; it imports no domain/storage/Git packages.

Installed rows show qualified ID and Enabled/Disabled, with search, source filter, selection sets, individual and all-target actions. Source rows use display_name with canonical namespace visible, falling back to namespace alone without repetition. Labels are `Included with Work`, `Git repository`, and `Local directory`; these are source kinds, not trust levels. Show explicit installed/enabled totals from complete inventory, independent of search. Do not show fictional sources or placeholders as real data.

Current errors/results replace earlier feedback. Back, abandoned review, and pre-commit cancellation retain the session and clean candidates. Keep valid filters, selection, and navigation stack across details/results; prune only disappeared targets. Escape/Back from a child returns to its parent; normal manager exit restores the terminal once and CLI prints exactly `Changes saved successfully.` No per-action receipt replay or cancellation transcript. Interview and repository/convention journeys retain their existing behavior.

## Logical surfaces

| ID | Surface | Required data/actions |
| --- | --- | --- |
| S01 | Installed | Qualified inventory, enabled state, search/source filter, multi-selection, enable/disable/check/update/remove, check all/update all |
| S02 | Installed details | ID, enablement, version, safe provenance, revision/digest, contributions; individual actions and available-entry navigation |
| S03 | Marketplaces | Display name, namespace, source kind, catalog state, complete local totals; add, details, upgrade all catalogs |
| S04 | Source registration | Visible editable Git/local source fields; optional branch for Git only; acquire/validate, show published namespace, register/result |
| S05 | Source details | Namespace/display/description, safe origin/branch/revision/catalog state, available/local totals; entries, filtered installed, upgrade/remove as provider permits |
| S06 | Available collection | One-source and all-source offers, source/search filters, exact identities/revisions, offered version and installed/enabled state; no bulk install |
| S07 | Available details | Identity, offered description/version/revision/contributions; add/reinstall entry |
| S08 | Install/reinstall review | Exact chosen offer, old/new content/contribution effects, dependency authorization, apply or Back; same-version reinstall supported |
| S09 | Check/update | One/selected/all pure checks, per-target versions/outcomes/failures, explicit candidate selection and separate fixed-candidate apply/review |
| S10 | Plugin removal review | One/selected identities, exact contributions/policy references, distinct authorization and acceptance; preserves source |
| S11 | Source removal review | Source plus complete installed set including disabled/withdrawn, dependencies and exact authorization; no included-provider action |
| S12 | Progress/results | Acquire/capture/validate/publish phases, per-source/target outcomes, permitted cancellation, unchanged/failure/committed cleanup; return with refreshed state |

These may share details/review/result primitives; no mandatory screen/widget per action is imposed. Runtime missing, catalog empty versus missing, source unavailable, collision, dependency refusal, concurrency conflict, recovery-required, and cleanup-pending are distinct actionable states. Refresh failure does not invalidate an older usable catalog. Check never implies apply or catalog upgrade.

## Parity matrix

| Row | CLI capability | Manager journey | Verification |
| --- | --- | --- | --- |
| P01 | marketplace list | S03 → S05 | Same metadata, source state, and full local totals |
| P02 | marketplace add SOURCE [--branch] | S03 → S04 → S12 | Git/local and optional recorded Git branch, same validation/identity |
| P03 | marketplace upgrade NAME | S05 → Upgrade → S12 | Discovery-only fixed offer and no-op behavior |
| P04 | marketplace upgrade, all | S03 → Upgrade all catalogs → S12 | Every known provider, independent results, no installation |
| P05 | marketplace remove NAME | S05 → S11 → S12 | Exact complete target/dependency review; builtin prohibited |
| P06 | plugin list [-m] | S01 search/source filter → S02 | Installed identities/state and safe provenance inspection |
| P07 | plugin list --available --json [-m] | S03/S05 → S06 → S07 | One/all-source views, source filter, missing-catalog outcomes |
| P08 | plugin add ID | S07 → S08 → S12 | One selected entry; fixed offer; explicit same-version reinstall |
| P09 | plugin enable/disable, one/multiple | S01 selection or S02 → action | Same provider set, atomic toggle, no policy insertion |
| P10 | plugin update --check, named/all | S01/S02 → Check → S09 | Temporary only; per-target failures; no automatic apply |
| P11 | plugin update, named/--all/interactive | S01/S02 → S09 review → S12 | One/selected/all targets, exact candidates and dependencies |
| P12 | plugin remove, one/multiple | S01/S02 → S10 → S12 | Same exact set and separately authorized references |

JSON is CLI serialization; the same data is accessible in manager rows/details/results. --yes corresponds to acceptance, not dependency permission. --remove-policy-references corresponds to a distinct explicit authorization of enumerated references, never preselected. Choosing a row already determines identity; do not require typing its selector. Selection sets apply only to installed maintenance/removal, not whole-catalog installation. No migration, reset, or compatibility flow is part of the manager.

## Verification dimensions

Each P01–P12 journey must be exercised with normal success and applicable no-op, error, cancellation/Back, and dependency cases. Cover S01–S12 at 100x30 and reduced 60x18 logical viewport, dynamic resize down/up, long Unicode display text, long inventories, search/selection persistence, visible source edits, and no-color modes. Small terminals may scroll/truncate secondary metadata but must retain reachable current identity/actions/help and reserved focus columns. NO_COLOR/TERM=dumb honor existing capability policy; no control sequences enter noninteractive stdout. Verify candidate Close on every exit and stale async completion, and exactly one normal terminal restoration/acknowledgment.
