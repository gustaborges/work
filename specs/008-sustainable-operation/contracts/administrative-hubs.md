# Contract: Administrative Hubs and Confirmation

## Shared behavior

`work plugin`, `work repository` and `work convention` are interactive hubs. They use the same application services as direct commands; no independent validation/persistence paths. Initial inspection is pure and does not install the seed. Read-only hub navigation, Back/Leave unchanged and cancellation do not mutate state. If setup is needed, show an explicit install/configuration action rather than initialize on entry.

Use existing `present.IO{In, UI}`, full-screen `present.Wizard`, semantic theme and viewport rules. Input/UI use the configured interface channel, stable results stdout. Restore the primary buffer on exit; retain only accepted, safe compact receipts and one final result. Sensitive source inputs and credential-bearing repository identities are redacted/omitted. Error-attributable active fields show one current recoverable error, with no rejected attempts in history. Domain services return typed values/errors and never import `internal/present`.

Keyboard navigation/filtering/multiselection/confirmation follow existing primitives and visible help; do not introduce a second keymap. Empty inventories yield an explanatory state/action, not an empty selector. Redirected/noninteractive hubs fail usage/2 with direct-command guidance and no controls. Bare hubs reject --json; concrete read actions show their --json equivalent.

## Plugin journey

1. Show all installed packages with alias, registered version, enablement and origin.
2. Offer list, install from source or bundled seed, enable, disable, check updates, apply updates, uninstall, or exit.
3. Select targets explicitly; collect only omitted source/alias values. Invalid names remain recoverable in the active control when correction is possible.
4. Check/retrieve candidates outside the administrative lock; show each outcome. Apply selection is distinct from check-only.
5. Apply review names fixed old/new versions/references, added/withdrawn declarations and policy references removed. Uninstall review names the complete target set, owned managed content and affected policy refs. The dependency-removal choice is explicit before final confirmation.
6. Confirm then apply that reviewed impact. If state drift invalidates it, show actionable conflict; do not silently widen approval. Print stable direct-equivalent mutation results after commitment.

Direct equivalents include exact safe aliases/flags. For dependency-aware automation show `work plugin uninstall ALIAS --remove-policy-references --yes` or the corresponding update form. --yes confirms an already computed impact; it never chooses a plugin or means blanket cascade.

Enable/disable have no extra direct confirmation requirement; the hub's accepted target/action performs the same atomic set transition. Install retains existing direct semantics. After applying a hub action, refresh through the pure snapshot before showing further actions; no stale captured state may overwrite a later change.

## Repository journey

Show ordered policy entries with reasons and installed Locator inventory including disabled providers. Offer add/remove/move/replace policy entries and list/add/remove/replace search roots. Only installed Locators can be newly included, but retained unavailable entries can be moved/removed/kept explicitly. Position selection translates directly to --before/--after. Whole collection replacement previews the resulting order/root set before acceptance. Removing the last entry is possible through explicit selection. Every screen shows its direct command and distinguishes roots from workspace location.

## Convention journey

Show the repository identity safely, remembered convention and availability. Offer inspection, explicit available replacement, or Leave unchanged. Do not automatically substitute an available name for an unavailable remembered one. Successful replacement changes only that repository's memory and shows the equivalent set command.

## Confirmation and cancellation boundaries

Review/confirmation always precede commitment and run outside the write lock. Before the journal's committed marker, cancellation rolls back the operation and exits cancelled/20; after commitment it cannot undo success. A cancelled update batch may retain earlier independently committed updates and explicitly marks unstarted targets not-applied. Uninstall does not commit a partial target set.

Rendering belongs to the sole CLI boundary: individual batch outcomes are records, followed by at most one terminal failure diagnostic. Long-running plugin execution can cause maintenance lock contention; return administration-busy/56 with retry guidance, without new plugin timeouts.

## Verification

US5; Q6/Q11–Q12: direct/hub state equivalence, all three keyboard journeys, no empty selectors, dimensions/colors/NO_COLOR/TERM=dumb, cancelled review, invalid value recovery, safe receipts, redirected streams, exact help inventory and one final error. Keep the existing presentation boundary test prohibiting domain imports of presentation.
