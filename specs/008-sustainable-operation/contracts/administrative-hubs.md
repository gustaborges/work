# Contract: Administrative Hubs and Confirmation

## Shared behavior

`work plugin`, `work repository` and `work convention` are interactive hubs. They use the same application services as direct commands; no independent validation/persistence paths. Initial inspection is pure and does not install the seed. Read-only hub navigation, Back/Leave unchanged and cancellation do not mutate state. If setup is needed, show an explicit install/configuration action rather than initialize on entry.

Repository/convention use existing `present.IO{In, UI}` and full-screen `present.Wizard`. Plugin management uses a dedicated domain-free full-screen manager with the same IO, semantic theme and viewport rules. Input/UI use the configured interface channel, stable results stdout. Restore the primary buffer on exit. Repository/convention retain accepted, safe compact receipts and one final result. Plugin management keeps all action feedback in the interface and prints only “Changes saved successfully.” on explicit normal exit. Sensitive source inputs and credential-bearing repository identities are redacted/omitted. Error-attributable active fields show one current recoverable error, with no rejected attempts in history. Domain services return typed values/errors and never import `internal/present`.

Repository/convention keyboard navigation/filtering/multiselection/confirmation follow existing primitives. The plugin manager exposes its inventory/form/review keymap in its footer. Empty inventories yield an explanatory state/action, not an empty selector. Redirected/noninteractive hubs fail usage/2 with direct-command guidance and no controls. Bare hubs reject --json; concrete read actions show their --json equivalent.

## Plugin journey

Authoritative behavior: spec FR-035–FR-039 and US5.7–US5.12. This correction replaces the original action-interview plugin hub; Start, repository and convention remain unchanged.

- Open directly on Installed (count). The top navigation contains Installed (count), Official and Install plugin only. Installed rows show alias/name and Enabled/Disabled; versions, source origins and local/linked distinctions belong in details. No enablement filter tabs, hypothetical marketplace tabs or Add marketplace appear.
- Left/right switch top-level destinations, including when arriving at Install plugin; that tab initially retains navigation focus. Enter/Tab enters the form (typing or pasting also starts editing); the i shortcut opens the form ready to type. While editing, left/right move the cursor or select mode, and Esc returns focus to the tab bar without discarding the draft. Esc from the inactive form returns to Installed. Up/down and page keys move/scroll inventories, Enter opens details, `/` edits a case-insensitive name search, Space toggles the selected installed package, and Esc clears search/returns to the parent or closes from top level. Form fields accept ordinary text including `q`; navigation must not accidentally execute actions.
- Official lists only known official offerings, initially work-reference (bundled). It shows installed/available state and exposes explicit offline install in details. Browsing remains pure; no network marketplace catalog is invented.
- Install plugin provides a visible source field (local path or git URL), optional alias and copy/link-local choice, then review of the prepared package before install. Invalid fields remain editable; Back preserves useful form state. Ordinary paths and URLs remain visible while editing; the source field is not a password field. Existing supported source grammar is preserved. Credential redaction in review/details/errors and stable outputs remains unchanged.
- Details show metadata, components/conventions and enable/disable, check/update and uninstall actions. Explicit checks retain the frozen update candidate through review; review includes old/new versions/references/digests, declaration changes and convention effects. Applying revalidates latest state through the existing service.
- Uninstall review names the selected package, owned installation path, affected policy references and convention effects. Linked sources, remembered conventions and existing Works remain. Policy-reference removal is authorized independently before final confirmation for update and uninstall. Refusal/Back discards the plan without ending the manager.
- Operations run through typed application services, outside the UI loop; no direct CLI handlers or printed receipts drive the interface. Busy state remains visible. Success, cancellation and recoverable error return to an actionable screen inside the same terminal lifecycle. Refresh uses a pure snapshot; errors remain actionable inside the manager.
- One current status/result replaces previous feedback. No receipt trail, repeated headings or per-action CLI equivalents. Normal close restores the terminal and emits exactly one line: `Changes saved successfully.` No action log or cancellation diagnostic follows it.
- Visual hierarchy uses a bold title/current destination, a highlighted full focus row with a textual marker, normal primary content, muted metadata/footer and semantic success/warning/error feedback. Plain-text mode retains markers and explicit states. Lists and content scroll to keep focused actions reachable; viewport changes never create a new terminal session.

Direct equivalents remain documented in CLI help and cli-work-plugin.md, with unchanged grammar, outputs and transactional semantics. The manager provides equivalent effective state through the shared services.

## Repository journey

Show ordered policy entries with reasons and installed Locator inventory including disabled providers. Offer add/remove/move/replace policy entries and list/add/remove/replace search roots. Only installed Locators can be newly included, but retained unavailable entries can be moved/removed/kept explicitly. Position selection translates directly to --before/--after. Whole collection replacement previews the resulting order/root set before acceptance. Removing the last entry is possible through explicit selection. Every screen shows its direct command and distinguishes roots from workspace location.

## Convention journey

Show the repository identity safely, remembered convention and availability. Offer inspection, explicit available replacement, or Leave unchanged. Do not automatically substitute an available name for an unavailable remembered one. Successful replacement changes only that repository's memory and shows the equivalent set command.

## Confirmation and cancellation boundaries

Review/confirmation always precede commitment and run outside the write lock. Repository/convention and direct commands retain their existing cancelled/20 exit semantics. In the plugin manager, Back/refusal cancels only the pending action and returns to its parent screen; explicit normal exit succeeds. Before the journal's committed marker, operation cancellation rolls back the operation; after commitment it cannot undo success. A cancelled update batch may retain earlier independently committed updates and explicitly marks unstarted targets not-applied. Uninstall does not commit a partial target set.

Rendering belongs to the sole CLI boundary: individual batch outcomes are records, followed by at most one terminal failure diagnostic. Long-running plugin execution can cause maintenance lock contention; return administration-busy/56 with retry guidance, without new plugin timeouts.

## Verification

US5; Q6/Q11–Q12: direct/hub state equivalence, all three keyboard journeys, no empty selectors, dimensions/colors/NO_COLOR/TERM=dumb, cancelled review, invalid value recovery, safe receipts, redirected streams, exact help inventory and one final error. Keep the existing presentation boundary test prohibiting domain imports of presentation.
