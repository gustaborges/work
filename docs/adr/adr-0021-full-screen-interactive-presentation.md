# ADR-0021: Full-Screen Interactive Presentation for Multi-Step Journeys

**Status:** Accepted

**Date:** 2026-09-07

**Product context:** `docs/prd.md` — RF-53, RF-55, RF-57, RF-64; `specs/003-terminal-ux-revamp/spec.md` FR-001, FR-007

**Related to:** ADR-0009 (central stack), ADR-0020 (generic presentation boundary)

**Governs:** `docs/add/add-0001-work-system-architecture.md` §12.2/§12.3

**Realized by:** `internal/present` (`Wizard`, `stepModel`), `internal/cli/start.go`

## Context

ADR-0020 adopted, as a first structure, an **inline session delimited by step**: each step of `work start` / `work resume` / `work archive` was an independent Bubble Tea program rendered in the terminal's current buffer, which collapsed to a one-line receipt when accepted, and explicitly rejected the alternate buffer.

In real use, two problems proved to be structural, not adjustable:

1. **Rendering ghosts.** The Bubble Tea inline renderer (default) loses the frame height on a terminal resize and in the confirm→list transition of `archive` (`Esc` to go back): it repaints the header without clearing old lines, stacking the title bar or the `[Local] Remote` tab bar five or six times on the screen. The `deSoftWrap` hack in tests is a symptom of the same class.
2. **Independent steps don't share the screen.** Each step was its own program; the receipts of already-accepted steps only reappeared in the primary buffer *after* that program terminated, and the next program entered on top. The journey had no visual continuity.

ADR-0020 already foresaw that "a single program containing the entire interview will only be considered if a future need for retroactive navigation justifies the complexity". The need that emerged was not retroactive navigation but rather **eliminating the ghost** and **keeping receipts visible throughout the journey**.

## Decision

Keep Bubble Tea and Lip Gloss and the generic boundary from ADR-0020, changing only the rendering structure:

* Each journey runs as **a single full-screen program** (alternate buffer), a `present.Wizard` over ordered steps. The alternate buffer is activated by `tea.View{AltScreen: true}` — there is no program option for this in Bubble Tea v2.
* A **clear + full repaint on each frame** makes the ghost of resize and confirm-return structurally impossible.
* The four primitives (`Input`, `Select`, `MultiSelect`, `Confirm`) stop being autonomous `tea.Model`s that call `tea.Quit` and start implementing a domain-free `stepModel` (`body`, `status`, `cursorPos`). The wizard composes the frame: a `Primary` ruler at the top, the journey title, the trail of accepted receipts, and the body of the current step. No step terminates the program; it reports a terminal `status()` and the wizard drives the transition.
* The public signatures `Input` / `Select` / `MultiSelect` / `Confirm` are preserved as single-step wizards, so `resume` and `archive` migrate without code change; only `start` is restructured to compose its six conditional steps into a wizard.
* **On termination**, the primary buffer is automatically restored and compact receipts of accepted steps are **reprinted** to the UI channel (stderr, primary buffer), ahead of the stable stdout lines the CLI writes next. The "terminal history" of SC-001/SC-002 metrics now means this post-termination reprint.
* The import boundary of `internal/present` is unchanged (stdlib + Charm stack + `colorprofile` + `x/term` + runewidth/uniseg + `internal/diag`); the test `tests/contract/present_boundary_test.go` remains green. Option values of any type `T` cross via generics and return to the CLI via type assertion, without `present` knowing the type.

## Alternatives considered

* **Fix the inline renderer (track height, clear before repainting).** Rejected: it would be reimplementing part of Bubble Tea's renderer and still fragile with each new frame transition; the alternate buffer with full repaint solves the entire class.
* **Keep inline steps independent and only manually stack receipts.** Rejected: it doesn't solve the ghost in the active selector or in `archive`, and the buffer splicing between programs remains unpredictable.
* **A single form in the current buffer (no alt-screen).** Rejected for the same reason as ADR-0020: the renderer in this mode too loses frame height when shrinking (long list → short receipt) and leaves garbage in scrollback.
* **Reject the alternate buffer (ADR-0020's original position).** Reviewed: the useful history of accepted choices is preserved by reprinting in the primary buffer on exit, so the reason for rejection (erasing history) does not apply to this structure.

## Consequences

**Positive:** the ghost of resize and confirm-return is impossible by construction; accepted receipts remain visible above the active step throughout the `work start` journey; the base selector no longer shows the short SHA per line; the journey is a coherent app; `deSoftWrap` is no longer needed for new screens.

**Negative / trade-offs:** interactive collection now occupies the full screen while running (the primary buffer returns intact on exit); the VT emulator in integration tests had to learn the alternate buffer toggle (DECSET 1049/1047/47) and VPA; `work start` now resolves `--slug` / `--base` from flags *after* the path step when SOURCE is interactive, and `workspace.Persist` moves to after the wizard (a rejected execution no longer writes the workspace root).
