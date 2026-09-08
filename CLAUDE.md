<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/003-terminal-ux-revamp/plan.md`

Active feature: **F2.5 — Terminal UX Revamp** (`specs/003-terminal-ux-revamp/`).
Design artifacts: `plan.md`, `research.md`, `data-model.md`, `quickstart.md`,
`contracts/`. Builds on shipped **F1** (`specs/001-first-local-work/`, release 0.1)
and **F2** (`specs/002-daily-cycle/`, release 0.2). Stack unchanged: Go 1.26 single
binary (Cobra + Bubble Tea/Lip Gloss, `huh` optional), system `git` as a subprocess,
`modernc.org/sqlite` projection, embedded self-contained seed package (`seed/`).
Module `github.com/gustaborges/work`.

F2.5 is a presentation-layer slice: no domain journey, no schema change. It adds
`internal/present` — a domain-free generic interaction boundary (ADR-0020, ADR-0021) —
with `Input`/`Select`/`MultiSelect`/`Confirm` primitives composed into one
full-screen `Wizard` per flow, one semantic theme, a `WORK` terminal-art wordmark
with a primary→secondary gradient, and a split diagnostic border. It migrates
`internal/cli/{start,resume,archive,root}` onto it,
deletes the selectable `internal/tui` home and pickers, replaces bare interactive
`work` with a static brand (exit 0), and makes `work --help` a grouped view sourced
from the Cobra command tree. Every F1/F2 command grammar, transaction guarantee,
stable stdout line, error token, and exit code is preserved (the one contracted
change: interactive bare `work` exits 0 instead of opening the home).
<!-- SPECKIT END -->


## Comments

Write comments only when they add context the code cannot clearly express—especially *why*,
constraints, trade-offs, invariants, and workarounds. Do not narrate obvious code; improve
names or structure instead.

Document every public API in SDKs and libraries: purpose, inputs, outputs, errors, side
effects, lifecycle/thread-safety requirements, and noteworthy constraints. Keep comments
concise and update them with code changes.

Motto: **Code shows what; comments preserve why.**