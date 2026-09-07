# ADR-0009: Core Stack — Go + Cobra + Bubble Tea + Embedded SQLite

**Status:** Accepted
**Date:** 2026-08-23
**Product Context:** `docs/prd.md` — RNF-5, RNF-6, Section 10

## Context

The choice of core stack (Go, Cobra, Bubble Tea, SQLite) is an architectural decision, not a product requirement — that is why it lives here, with alternatives and justification recorded, not as a direct statement in the PRD. The core needs to: (a) be distributed as a single, portable binary, without requiring an external runtime to be installed on the user's machine (RNF-6); (b) offer a keyboard-navigable TUI across the entire selection surface of the product (Section 10 of the PRD); (c) maintain local auditable state that can be queried in an ordered manner, without relying on filesystem scanning (RNF-5, RF-11: list work items by `last_accessed_at`).

## Decision

* **Go** as the core language — compiles to a single, cross-platform binary without requiring an external runtime to be installed.
* **Cobra** as the CLI framework — industry standard for CLIs in Go, used by `kubectl`, `gh`, `hugo`, among others; reduces maintenance risk by being widely adopted and documented.
* **Bubble Tea** as the TUI framework — it is the idiomatic Go option for the home and interactive selection screens that the product requires in virtually every flow (concurrent plugins, branch prefixes, base branch, `work resume`, `work archive`, and administrative hubs).
* **Embedded SQLite**, via pure Go driver without CGO dependency, for local state persistence — stored within the layout described in `docs/add/add-0001-work-system-architecture.md`, Section 3.

## Alternatives Considered

* **Node.js or Python for the core.** Rejected: both require an external runtime to be installed on the user's machine, directly contradicting the goal of a single binary and RNF-6 portability — the very problem this product seeks to avoid for plugins (ADR-0006) would become a problem for the core itself.
* **Rust.** Also produces a single binary without external runtime, but was rejected for having a less mature and consolidated CLI/TUI ecosystem than Go for this specific use case, without clear gains that would justify the learning curve cost and younger library ecosystem equivalent to Cobra/Bubble Tea.
* **Loose JSON files instead of embedded SQLite.** Rejected: listing work items ordered by last access (RF-11) without varying cost with the number of items would require, with loose files, scanning the filesystem and reordering in memory on each call — exactly the problem that an embedded database with indexes solves by design.
* **CGO enabled for the SQLite driver.** Rejected: would require a C toolchain available on the user's machine/in the cross-platform build process, reintroducing an external dependency that the pure Go driver avoids.

## Consequences

**Positive:** distribution as a single binary, without external runtime dependency, on any supported OS; a single framework paradigm (Cobra for commands, Bubble Tea for interactive selection) throughout the product; ordered state queries without scanning cost.

**Negative / Trade-offs:** couples the core to a single language ecosystem (Go) — acceptable because this choice affects only the core, not plugins, which remain free of any language restriction (ADR-0006); the embedded SQLite driver is an additional dependency to keep updated, mitigated by being pure Go and not requiring an external toolchain.
