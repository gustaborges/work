# ADR-0014: Separate Origin Resolution from Local Repository Location

**Status:** Accepted

**Date:** 2026-08-31

**Product Context:** `docs/prd.md` — RF-2, RF-10, RF-39, RF-44, and RNF-8

**Governs:** `docs/add/add-0001-work-system-architecture.md`, Section 7

## Context

The previous contract required each Starter for `work start` to produce a local path. Integrations with forges, issue trackers, and other origins thus became aware of the particular organization of clones on the user's machine. This organization varies by environment and may use filesystem scanning, aliases, indices, or local tools.

## Decision

Separate the interpretation of the origin from the physical location of the clone. A Starter resolves `argument → Repository Reference`, along with branch, startup modes, metadata, and links it knows. An executable component with the `repository-locator` role resolves `Repository Reference → candidates for local Git repository`.

If the reference includes `path`, the core validates the path directly and does not run Locators. The core remains the final authority to validate every path used in the Git lifecycle. Locators do not interpret the argument, do not define mode/base branch, do not publish metadata or links, do not create worktrees, and do not select candidates.

## Consequences

Starters become portable across local organizations and Locators reusable across origins, allowing N + M composition instead of N × M. The startup pipeline gains an explicit phase and the manifest/protocol gains the `repository-locator` role.

## Alternatives Rejected

* Have each Starter locate its clone: duplicates logic and couples the integration to the environment.
* Implement all strategies in the core: violates the simplicity and extensibility of the kernel.
* Require a single workspace tool: reduces portability and introduces external dependency.
