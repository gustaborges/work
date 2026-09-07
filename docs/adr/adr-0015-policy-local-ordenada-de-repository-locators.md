# ADR-0015: Ordered Local Policy of Repository Locators

**Status:** Accepted

**Date:** 2026-08-31

**Product Context:** `docs/prd.md` — RF-40 to RF-48 and RNF-9

**Governs:** `docs/add/add-0001-work-system-architecture.md`, Section 7.2

## Context

A machine may have multiple locator mechanisms, such as aliases, corporate index, and filesystem. Preference between these mechanisms is a property of the user's environment; prioritization in the manifest, order of installation, or aggregation of results would introduce implicit precedence and arbitration without common authority.

## Decision

Maintain a global, declarative, and ordered Repository Resolution Policy formed by qualified references `<alias>/<component>`. Resolution traverses the policy by Chain of Responsibility: ineligible Locator is ignored; zero matches advances; one valid match concludes; multiple valid matches are chosen by the user and end the chain; operational failure interrupts resolution. v1 does not aggregate results, has no scores, and no `continue_on_locator_error`.

Installing or enabling a plugin does not modify the policy. Removing a Locator from the policy simply stops using it; it does not create an individual enablement state. Disabling the plugin makes its Locators unavailable, but preserves their positions in the policy. Uninstalling a plugin removes, upon explicit confirmation, the affected references in the same consistent change.

The TUI and direct commands are grouped under `work repository`. The direct API uses `policy list|add|remove|move|replace` for the sequence, `locator list` for installed Locators, and `root list|add|remove|replace` for search roots. The grammar is governed by ADR-0017.

## Consequences

Behavior is deterministic, auditable, and does not change silently when plugins are installed. In exchange, the user must explicitly include and order Locators, and a failure of a configured Locator interrupts resolution in v1.

## Rejected Alternatives

* Numeric priority in manifest or order of installation: would be implicit and unstable.
* Official plugin always in fixed position: preference belongs to the user.
* Run all and aggregate results: requires arbitration, increases cost, and makes the result sensitive to installed plugins.
* Policies per Starter: recreate coupling between origin and location in v1.
