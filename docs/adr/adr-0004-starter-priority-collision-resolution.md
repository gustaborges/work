# ADR-0004: Starter Plugin Priority and Collision Resolution

**Status:** Accepted
**Date:** 2026-08-23
**Product context:** `docs/prd.md` — RF-1, RF-3, RNF-3, Section 11 (community plugins metric)
**Governs:** `docs/add/add-0001-work-system-architecture.md`, Section 7 (collision resolution)

## Context

Defining starter priority purely by the order of a configuration array, without signaling to the user that other plugins would also match the same argument, does not scale with dynamic third-party plugin installation (ADR-0002): "at what position does a new plugin enter?" gets worse with each new installed plugin — and the product's own success metric is the number of plugins created by the community (PRD Section 11).

## Decision

Two-layer resolution, without manual ordering list:

* **`fallback` layer** — plugins that match any argument. Only one can be enabled at a time; enabling a second is an error detected at enable time, not at runtime.
* **`specific` layer** — plugins with their own recognition pattern, no order between them. In `work start <arg>`, every enabled `specific` pattern is evaluated locally against the argument, without subprocess (RNF-4):
  * exactly one matches → invokes directly;
  * none match → falls back to the single enabled fallback plugin;
  * more than one matches → collision: Work informs the user that multiple candidate plugins exist for that argument, lists them, and reuses the same TUI selection component already planned for contribution-vs-fork disambiguation (RF-3) to let the user choose which to use (RF-1).
* **The choice is not persisted.** The same collision is resolved with the same explicit question on each `work start` — there is no memorization, priority command, or "forget" command. No attempt is made to compare "specificity" between recognizers to automatically decide — such comparison is not well-grounded for free-form patterns (see Alternatives).

This ADR specifically resolves *pattern matching* collision between starters over the same argument, always at the moment and without state. The branch convention choice per repository (ADR-0011) is triggered by a different event — first use of a repository — and that one is memorized, by its own key and command; separate decision, unrelated to this.

## Alternatives considered

* **A — explicit manual ordering, new plugin always at end.** Simple, but doesn't scale with ecosystem growth; two third-party plugins with colliding recognizers depend on the user remembering to manually reorder — rejected.
* **B — eliminate ordering completely via "more specific recognizer wins".** Removes manual ordering concept, but "more specific" is ambiguous for a free-form recognizer — reintroduces non-determinism, violating RNF-3 — rejected.
* **C — memorize the choice by exact set of colliding plugins, with escape hatch commands (`work memory priority` / `work memory forget-choice`).** Was the previous decision of this ADR. Rejected for cost/benefit: a real collision requires two `specific` plugins with overlapping patterns enabled at the same time — rare situation — and the mechanism required a dedicated persistence key, two new subcommands, and a grouping (`work memory`) to host them. Asking again on each occurrence is small, localized friction for those who actually have the collision, and keeps the core smaller (RNF-1).

## Consequences

**Positive:** new plugin never requires reordering by default; determinism preserved (same collision → same explicit question, no hidden state influencing resolution); collision only appears when it actually occurs, not as anticipated modeling; smaller core — no priority table, no override subcommand.

**Negative / trade-offs:** a real collision asks again on each `work start` with the same ambiguous argument. Acceptable: requires two `specific` plugins with overlapping patterns enabled simultaneously (rare), and the user resolves it on the spot without needing to anticipate priority configuration.
