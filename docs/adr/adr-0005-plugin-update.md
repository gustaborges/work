# ADR-0005: Plugin Update Mechanism

**Status:** Accepted
**Date:** 2026-08-23
**Product Context:** `docs/prd.md` — RF-19, RNF-3, RNF-4
**Governs:** `docs/add/add-0001-work-system-architecture.md`, Section 12

## Context

The update trigger is to compare only an opaque version field declared by the plugin author, never raw commit history — a hybrid between pinning an exact reference (reproducibility) and tracking a branch using that field as a gate (minimal friction for the author). It was missing to define (a) whether the update can be automatic, and (b) how to check the remote version without disturbing the currently installed and in-use copy of the plugin.

## Decision

* Updates are always explicit, never automatic/silent (RF-19). `work plugin update --check [plugin...]` is the read-only query; `work plugin update <plugin...>` updates explicit targets; `work plugin update --all` batch updates, showing what will change and requiring confirmation. `work plugin update` without a target opens TUI selection. `work start`/`resume`/`archive` never trigger any of this (RNF-4). Cross-cutting grammar is governed by ADR-0017.
* Checking for updates never touches the working copy that serves the currently installed plugin's entrypoint — only the explicit and confirmed query moves what is actually in use to a new resolved reference. The concrete mechanics are in `add-0001` §12.
* No additional release convention (e.g., tag) is required of the plugin author beyond the version field already declared in the manifest (ADR-0012) — maintains the promise of low friction for authorship; the cost is an incremental check per plugin, acceptable because it only happens in explicit queries, never in `start`/`resume`/`archive`.

## Alternatives Considered

* **Fully manual, individual only.** Does not scale beyond a few plugins — rejected as the only option (maintained as an available path within the model above).
* **Automatic/silent** (browser extension style). **Directly rejected**: a plugin could change behavior between executions without the user knowing, contradicting RNF-3 and the risk of malicious/poorly written plugins already named in the PRD. There is no trade-off that justifies considering this option for this product.
* **Require a release tag corresponding to the declared version**, to allow checking without transferring any objects. Technically cheaper, but rejected for v1 because it adds an authorship obligation beyond what is already promised; noted as a possible future performance optimization, not a requirement change.
* **Passive notification in `start`/`resume`.** Reduces "forgetting to check", but would require a TTL cache to not violate the spirit of RNF-4 if checking were synchronous — noted as a candidate for non-blocking v2.

## Consequences

**Positive:** no new obligation for the author beyond what is already promised; never mutates a plugin's active working copy during a simple check; a single field (`version`) is the signal of "what changed" throughout the system.

**Negative / trade-offs:** every check of "what is outdated" still costs one network query per plugin — acceptable because it is initiated by the user and infrequent, not a blocker for v1.
