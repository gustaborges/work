# ADR-0020: Generic Interactive Presentation Boundary (Inline)

**Status:** Accepted — decision on rendering structure (inline session per step and rejection of alternate buffer) **partially superseded by ADR-0021**. The generic CLI ↔ presentation boundary, explicit step states, stable geometry, stream ownership, and diagnostic boundary remain in effect.

**Date:** 2026-09-07

**Product context:** `docs/prd.md` — RF-51 to RF-62, RF-64 and RNF-11; `specs/003-terminal-ux-revamp/spec.md`

**Related to:** ADR-0009 (central stack), ADR-0019 (CLI surface), ADR-0021 (full-screen presentation)

**Governs:** `docs/add/add-0001-work-system-architecture.md` §12

**Realized by:** `docs/add/add-0001-work-system-architecture.md` §12.2–12.5

## Context

Current behavior divides each interview between terminal controls and domain validations executed after the control terminates. Recoverable errors are then printed as new lines, selectors end still expanded, and Work DTOs cross the presentation boundary. These symptoms do not stem from a limitation of the stack accepted in ADR-0009; they stem from the absence of a boundary that separates journey, domain, diagnostics, and visual state.

A local fix to heights and colors would not prevent the same problems from reappearing in new commands. It is also undesirable to move sequencing and mutations to terminal models, as this would make the presentation layer aware of repositories, branches, Works, and plugins.

## Decision

Keep Bubble Tea and Lip Gloss, per the direction of ADR-0009, behind a Work-specific interactive presentation layer with these responsibilities:

* The CLI/application layer controls the sequence of the journey, assembles generic options, provides deterministic and repeatable validations, classifies errors, decides confirmations, and executes mutations.
* The presentation receives only titles, descriptions, generic options, confirmation content, safe receipt formatting, and non-mutation validation callbacks. It does not import Work domain concepts or types.
* Each interactive step has explicit states for edit/select, completion, and cancellation. Recoverable failure remains in the active state and replaces the previous error; completion and cancellation define a compact final view before normal termination.
* The first adopted structure is an inline session delimited by step. The CLI continues composing steps in order; a single program containing the entire interview will only be considered if a future need for retroactive navigation justifies the complexity.
* Selectors maintain stable geometry during the active state and derive their limit from terminal dimensions. The completed view may and should be smaller than the active view.
* Focus uses style and a reserved text column of constant width. Checkboxes, primary text, and metadata maintain predictable columns and heights; measurements consider displayed width, not byte size.
* Input and UI writer are provided explicitly to the presentation. Interactive UI and human diagnostics use stderr/UI; stdout remains reserved for stable command results.
* The diagnostic boundary transforms causes into public and actionable messages once. The cause chain remains inspectable by explicit diagnostic mechanism, not by normal human output.
* Huh can remain as an internal detail for simple fields during migration, but does not define the presentation contract. Its future maintenance or removal does not alter the CLI/application or product requirements.

## Alternatives considered

* **Fix each prompt and selector in isolation.** Rejected because it maintains domain dependencies, implicit stream ownership, and inconsistent error cycles.
* **Consolidate the entire interview in a single form.** Deferred because it would simplify retroactive navigation and global screen budgeting, but would shift sequencing to presentation or require an asynchronous protocol without current requirement.
* **Replace the entire terminal stack.** Rejected because the current stack already supports inline operation, final state, resize, and styles; the switch does not solve the architectural boundary.
* **Enter the alternate buffer/full-screen.** Rejected because it erases the useful history of accepted choices and conflicts with the expected continuity of a daily CLI.

## Consequences

**Positive:** domain validations remain outside rendering without leaking errors into history; controls are reusable and testable with generic data; stdout and stderr become deterministic; visual dependencies can evolve behind the boundary; new selectors inherit coherent theme, geometry, and lifecycle.

**Negative / trade-offs:** Work now has an additional layer of adaptation and visual models; validations provided by the CLI must be idempotent and safe for repetition; terminal behavior requires tests of viewport, Unicode, color, and PTY across supported platforms.
