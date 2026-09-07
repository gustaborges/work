# ADR-0013: Data Namespaces and Work Persistent State

**Status:** Accepted

**Date:** 2026-08-29

**Product Context:** `docs/prd.md` — RF-7, RF-26, RF-27, RF-32 to RF-35, and RNF-5

**Governs:** `docs/add/add-0001-work-system-architecture.md`, Sections 3, 4, 7, 9, and 11

## Context

The component model must carry both Work domain state and extensible data published by plugins and external relations. If these data share a single semantic space, plugins can overwrite core state, external relations become indistinguishable from metadata, and the same information tends to be named by the tool that produced it, preventing interoperability between providers.

It is also necessary to keep each Work's state auditable and preservable after archiving, without making the physical persistence structure a plugin API. A global database is useful for listing and searching, but should not be the only source needed to understand or recover an individual Work.

## Decision

Work recognizes three input namespaces: `work:<key>`, `meta:<key>`, and `link:<key>`. `work` contains only domain state defined and governed by the core; plugins cannot create or write keys in this namespace. `meta` contains extensible information produced by components. `link` contains first-class external relations, with at most one current value per key and last-write-wins upsert semantics.

Public keys in `meta` and `link` follow Work Semantic Conventions, which define namespace, key, meaning, and value representation without forming a closed enumeration in the binary. Data that is not an interoperability contract uses private namespace `plugin.<plugin-name>.<key>`. The key identifies the meaning of the data, not its provider: distinct components can produce the same key, and the concrete provenance is recorded separately by the core.

Each Work has `work-state.json` as a canonical, self-contained snapshot versioned by `schema`, with sections for `work`, `meta`, and `links`. The core updates the file via temporary write followed by atomic rename. Plugins do not read or write this file; they interact only through `inputs` and `outputs` contracts.

`~/.work/state/work.db` is a global projection/index for CLI queries. It may contain normalized fields and operational provenance, but must be reconstructible or reconcilable from the snapshots. There is no requirement for structural identity between IPC payloads and the state file: the core translates operation responses to the resulting state.

This ADR replaces the parts of ADR-0012 that limited `inputs[]` to `link` and `meta` and handled link/metadata persistence without the explicit canonical state model defined here.

## Alternatives Considered

* **A single metadata map for all data.** Rejected: allows plugin data to mix with domain state and eliminates the generic handling of external relations needed for eligibility, Linkers, and link operations.
* **Include the provider in the semantic key.** Rejected: `github.pull_request.via_api` and `github.pull_request.via_gh_cli` describe the same relation and would fragment producers and consumers. Provider and provenance belong in the operational record, not in data identity.
* **Use `work.db` as the single source of truth.** Rejected: prevents an archived Work's directory from being self-contained and makes recovery/audit dependent on a separate global database.
* **Expose `work-state.json` directly to plugins.** Rejected: turns physical layout into a public API, couples plugins to schema migrations, and allows writes outside the core-controlled lifecycle.
* **One file per domain (`work.json`, `meta.json`, `links.json`).** Rejected: a transition can affect all three domains; a single snapshot allows validating and publishing the update as one atomic unit.

## Consequences

**Positive:** ownership and eligibility become explicit; plugins interoperate by meaning, not implementation; each Work's state remains auditable and recoverable; the global index can evolve without being a Work integrity dependency.

**Trade-offs:** the core must validate namespaces, keys, and schema, keep the projection reconcilable, and carefully define which `work:*` properties are exposed. Persisting a complete provenance history in the Work directory remains out of scope; the index can record only the operational provenance needed.
