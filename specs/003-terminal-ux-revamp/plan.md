# Implementation Plan: Terminal UX Revamp (F2.5)

**Branch**: spec/plan/doc work on `feature/003-terminal-ux-revamp-p0-specs`; F2.5
implementation uses one `feature/003-terminal-ux-revamp-p<n>-*` branch per phase
(see **Branching Strategy**) | **Date**: 2026-09-07 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/003-terminal-ux-revamp/spec.md` and the
staged research note `temp/tui-revamp.md`, with the explicit requirement that the
`WORK` wordmark be terminal art using a gradient of the settled primary and
secondary colors.

## Summary

F2.5 is a vertical presentation-layer slice that runs before F3. It does not add a
domain journey. It replaces the *how* of every interactive path shipped by F1 and F2
so the terminal experience becomes coherent, inline, bounded, accessible, and
scriptable — without changing any command grammar, transaction guarantee, stable
stdout line, error token, or exit code except the one explicitly contracted change
(interactive bare `work` now exits 0 after a static brand instead of opening a
selectable home).

Four things change:

- **An inline interaction lifecycle.** Every step (`Input`, `Select`, `MultiSelect`,
  `Confirm`) is one bounded Bubble Tea program in the current screen buffer with
  explicit `editing → completed | cancelled` states. On acceptance it collapses to a
  one-line **receipt** (title + success mark + accepted value, redacted for secrets);
  on cancel it collapses to one concise notice. Recoverable validation runs inside
  the active frame via a CLI-supplied closure and replaces the previous error in
  place — rejected attempts never reach terminal history.

- **A generic presentation boundary (`internal/present`).** The CLI supplies titles,
  descriptions, generic options, deterministic side-effect-free validation closures,
  receipt formatters, and confirmation content. `internal/present` owns only
  interaction state, geometry, theme, and rendering. It imports **no** Work domain
  package. An import-boundary test enforces this (ADR-0020).

- **A branded static home and a grouped help.** Interactive `work` with no arguments
  prints a terminal-art `WORK` wordmark (primary→secondary gradient in true colour),
  a tagline, and a `work --help` direction, then exits 0. `work --help` renders the
  brand plus every registered command grouped by context (`Daily`, `Inside a Work`,
  `Administration`, `Setup`) from the Cobra command tree — no empty groups, no
  unavailable commands. The selectable `homeModel` is deleted.

- **A split diagnostic border and a semantic theme.** One human diagnostic is
  rendered exactly once at the CLI/process boundary; lower layers return structured
  errors and never print. Interactive cancellation renders `✘ Operation cancelled`
  once and keeps exit 20. Colour is one semantic theme (brand accents distinct from
  success/warning/failure) and is disabled for non-TTY output, non-empty `NO_COLOR`,
  or `TERM=dumb`, emitting zero styling bytes in those cases.

Technical approach: keep Cobra, Bubble Tea v2, and Lip Gloss v2 (ADR-0009 / ADR-0020);
keep the typed `diag` category/token/code concept. Add `internal/present` with the
generic primitives, the semantic `theme`, the brand renderer, and the interactive
diagnostic renderer. Migrate `internal/cli/{start,resume,archive,root}` onto it.
Retire the domain-aware pickers in `internal/tui` and the `homeModel`. `huh` may stay
behind `present.Input` / `present.Confirm` during migration; the decision to keep or
drop it is deferred to a prototype and does not affect any contract. No `internal/`
package outside `cli` / `present` changes behaviour; `work-state.json`, `work.db`,
`seed/`, config, and the shell-integration contract are untouched.

## Technical Context

**Language/Version**: Go 1.26 (toolchain go1.26.4). Single module
`github.com/gustaborges/work`. No new language features.

**Primary Dependencies** (all already vendored by F1/F2 — F2.5 adds none):
- `github.com/spf13/cobra` — command tree stays the source of truth for `work --help`;
  F2.5 adds `cobra.Group` / `GroupID` and a custom help renderer sourced from it.
- `charm.land/bubbletea/v2` — inline renderer (no alternate screen), `WindowSizeMsg`,
  graceful final render, explicit `WithInput` / `WithOutput`. This is the engine for
  every `present` primitive.
- `charm.land/lipgloss/v2` — semantic theme styles, width-aware truncation, colour
  blending for the wordmark gradient.
- `charm.land/huh/v2` — retained behind `present.Input` / `present.Confirm` as an
  implementation detail during migration; not on any contract.
- `github.com/charmbracelet/colorprofile` (already indirect) + `golang.org/x/term` —
  capability detection (TTY, colour profile, `NO_COLOR`, `TERM=dumb`).
- `github.com/mattn/go-runewidth` / `github.com/rivo/uniseg` (already indirect) —
  display-width measurement for bounded frames and stable columns.
- `github.com/creack/pty` (already a test dep) — PTY integration tests.
- System `git`, `modernc.org/sqlite` — unchanged; F2.5 issues no new git subcommand
  and no new projection query.

**Storage**: **No change.** `work-state.json` stays schema 2, `work.db` stays
`user_version = 2`, config and generated state untouched. F2.5 persists nothing new;
receipts and brand output are transient terminal bytes. The `WORK_CD_FILE`
shell-integration contract (ADR-0018) is reused verbatim.

**Testing**:
- `go test` model-level unit tests per `present` primitive: invalid input stays in
  `editing` with exactly one error; retry replaces the error; accepted/cancelled
  final `View()` is the exact compact string before `tea.Quit`; selected vs.
  unselected rows have identical display width and line count; cursor movement
  changes style only; `[ ]`/`[x]` are equal display width; `WindowSizeMsg` recomputes
  the viewport without exceeding height; no-colour views remain understandable.
- Golden view tests for the wordmark (wide true-colour, wide no-colour, narrow) and
  for `work --help` group rendering.
- An import-boundary test (`go list -deps ./internal/present` must not contain
  `internal/work`, `internal/worklist`, `internal/basebranch`, `internal/archive`,
  `internal/resume`, `internal/projection`, `internal/gitx`, …).
- PTY integration (`tests/integration`, `//go:build unix`, existing `console`
  harness): four invalid repo paths then a valid one leave only the valid receipt;
  invalid-slug/collision retries likewise; the base selector is large while active
  and compact after Enter; the confirmation begins right after the branch receipt
  with no padded frame; moving across every archive row causes zero vertical / column
  movement (the F2 archive double-render defect gets a regression test); `q` / `Esc`
  / `Ctrl-C` on resume leave one concise notice, exit 20, and do not bump
  `last_accessed_at`; TTY UI bytes go to the UI writer, stable results to stdout.
- `testscript` non-interactive contract tests: redirected/piped runs emit zero ANSI
  and construct no TUI; `NO_COLOR=1` and `TERM=dumb` produce plain output;
  non-interactive bare `work` still exits 2; `work --help` exits 0 in every mode and
  lists exactly the registered commands.
- The full F1 quickstart (S1–S12) and F2 quickstart (S1–S13) stay green unchanged
  (SC-007). Terminal sizes exercised: 40×10, 80×24, 160×50 (SC-004).
- CI matrix unchanged: `ubuntu-latest`, `macos-latest`, `windows-latest`. PTY tests
  stay `unix`-tagged; Windows keeps the F1/F2 non-interactive + manual-smoke posture.

**Target Platform**: Same as F1/F2 — Linux (amd64, arm64), macOS (arm64, amd64),
Windows (amd64). No change to the supported terminal / shell matrix.

**Project Type**: Single-project CLI tool (unchanged). One new `internal/` package
tree (`present`), plus changes to `internal/cli`; `internal/tui` shrinks and is
eventually removed.

**Performance Goals**: Not latency-critical. A step renders and responds to a
keystroke in well under one frame; the brand and help are static string assembly.
The one new time budget: if a validation closure can exceed ~100 ms (a read-only
Starter or git inspection), the active field shows a neutral "checking…" state
without moving validation into `present` (spec Assumptions; research R15).

**Constraints**:
- Inline only — never enter the alternate / full-screen buffer (FR-001).
- Every active frame stays within the terminal's current rows and columns after
  reserving title / error / help / filter / confirmation lines; long and Unicode
  content wraps or truncates (secondary before primary) and never scrolls the frame
  into history (FR-002, FR-006–FR-008, SC-004).
- Focus is bold + a fixed-width textual marker; never a printable-width prefix that
  shifts columns; colour may reinforce but is never required (FR-009, SC-003).
- One current validation error per field; rejected attempts and stale errors leave
  no scrollback (FR-005, FR-006, SC-001).
- One human diagnostic at the process border; lower layers and active controls never
  also print (FR-014, FR-015). Cause chains stay inspectable only via explicit
  diagnostics, never in normal output (FR-016).
- Interactive cancellation → one concise notice, unchanged programmatic category and
  exit code (20 where a command defines it); no mutation, no access-time bump
  (FR-012, FR-013, SC-010).
- Colour disabled for non-TTY, non-empty `NO_COLOR`, `TERM=dumb`; those outputs
  contain no control sequences (FR-025, SC-008).
- Interactive UI + human diagnostics on the configured UI channel (stderr); stable
  results stay on stdout per existing command contracts (FR-026).
- Non-interactive input/UI channel ⇒ no interactive control is opened; existing args,
  flags, stdout, tokens, exit codes stay compatible (FR-027–FR-029, SC-007).
- `internal/present` imports no Work domain package (FR-004 boundary, ADR-0020).
- Behaviour identical across the supported OS / terminal matrix (FR-032).

**Scale/Scope**: Single user, single machine. Code surface: new `internal/present`
(primitives, theme, brand, diagnostic renderer, capability detection); rewrites of
`internal/cli/{root,start,resume,archive}` and deletion of `internal/tui/home.go`
plus the domain-aware pickers; `internal/diag` gains structured `Summary` / `Hint`
fields and a `Cancelled`-aware interactive renderer (categories and codes unchanged).
No `seed/` change, no new command, no new flag.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unratified template (placeholder
principles, no ratified version), so there are no project-specific constitutional
gates. As in F1/F2, this plan is held to the **cross-cutting gates in
`docs/roadmap.md` §4** and the spec's **Success Criteria**, treated as binding.

| Gate (roadmap §4) | How this plan satisfies it | Verified by |
|---|---|---|
| Determinism | No hidden heuristics in presentation: viewport size derives from `WindowSizeMsg`, focus geometry is fixed-width, group order in help is a declared constant, wordmark form is a pure function of (width, colour profile). Colour opt-out is the documented env rule only. | Golden view tests; capability-matrix tests; SC-003, SC-008 |
| Integrity | F2.5 performs **no** mutation. Validation closures are contractually side-effect-free and repeatable; `present` callbacks never mutate (import-boundary test makes a domain write impossible from `present`). Cancellation before confirmation changes nothing (carried F1/F2 tests stay green). | Import-boundary test; PTY cancellation tests; SC-010; F1 S8 / F2 regression |
| Process contract | stdout unchanged for every affected command; `error: <token>: <message>` unchanged for non-interactive failures; exit codes unchanged (`diag.All` table test is untouched — no new category). Only contracted change: interactive bare `work` → exit 0 (FR-017, FR-028); non-interactive bare `work` still exit 2. | `diag` table test; `testscript` non-interactive suite; `root_test.go`; SC-007 |
| Portability | Inline rendering, `WindowSizeMsg`, and width measurement are cross-platform in Bubble Tea v2 / lipgloss. No new subprocess. PTY tests `unix`-tagged as today; Windows keeps non-interactive coverage + manual smoke. Same 3-OS CI. | CI matrix; SC-008 |
| Auditability | The diagnostic border names what failed in user vocabulary with a next action; the cause chain is retained (`errors.Unwrap`) and surfaced only through an explicit diagnostic affordance. Receipts are a durable, secret-safe audit trail of accepted choices. Brand / help contain no secrets. | `diag` renderer tests; FR-015, FR-016; SC-001 |
| UX | Every journey stays directly invocable and is discoverable from the brand direction + grouped help (FR-020–FR-022). Interactive paths are inline, viewport-bounded, keyboard-navigable with consistent keys (FR-001, FR-002, FR-010). Automation paths never open a TUI and keep `--json` on reads (FR-027). | Help correspondence test; PTY geometry tests; `testscript`; SC-004–SC-006 |
| Regression | F1 S1–S12 and F2 S1–S13 automated demos stay green unchanged; they become the compatibility baseline for the migration. | CI; SC-007 |

**Architectural-authority gates (PRD → ADR → ADD):**

- **PRD**: RF-50 through RF-64 and RNF-10/RNF-11 were added in `docs/prd.md` v3 (staged
  in this branch's working tree) and are the product authority this spec cites. ✅
- **ADR-0019** (accepted, supersedes ADR-0017): governs the command surface — static
  brand home, grouped help from the live inventory, preserved grammar, interactive
  bare `work` exits 0 / non-interactive exits 2. This plan implements exactly that
  surface and adds no verb, alias, or flag. ✅
- **ADR-0020** (accepted): governs the presentation boundary — retain Bubble Tea /
  Lip Gloss behind a Work-owned generic inline adapter, per-step session, explicit
  states, stable geometry, explicit I/O, single diagnostic transformation, `huh`
  optional. `internal/present` is that adapter. ✅
- **ADD §12** (§12.1–12.5, staged): realises both ADRs; this plan's package layout
  and lifecycle match §12.2–12.4 verbatim. ✅
- **Historical F1/F2 contracts**: `specs/001-first-local-work/contracts/cli-work-home.md`
  and `specs/002-daily-cycle/contracts/cli-work-home.md` describe the *superseded*
  selectable home. `specs/003-terminal-ux-revamp/contracts/cli-work-home.md`
  supersedes their presentation behaviour; the F1/F2 documents stay as delivery
  records. Every other F1/F2 contract (grammar, transactions, stdout, exit codes)
  stays authoritative and is protected by the regression gate. ✅
- **⚠️ tracked, not a violation:** the F2 plan already tracks a one-line ADR-0018
  amendment (`work archive` also writes `WORK_CD_FILE`). F2.5 does not touch shell
  integration and does not change that item.

**Gate result: PASS.** No violations requiring Complexity Tracking.

### Post-design re-check (after Phase 1)

Re-evaluated after `research.md`, `data-model.md`, `contracts/`, and `quickstart.md`
were written. Still PASS:

- No new dependency, no new binary, no new persisted state, no schema touch.
- The only surface change is the contracted interactive bare-`work` exit code; the
  `diag` category table is unchanged (`Summary` / `Hint` are new *fields*, not new
  categories or codes — `contracts/diagnostics.md`).
- `contracts/presentation-boundary.md` makes the domain-free rule a mechanical test,
  not a review convention.
- All seven roadmap §4 gates have a concrete contract or test harness (table above →
  `contracts/` and `research.md`).
- `quickstart.md` re-runs the entire F1 + F2 validation set as the compatibility
  proof (SC-007) and adds Q1–Q12 for the new behaviour.

## Project Structure

### Documentation (this feature)

```text
specs/003-terminal-ux-revamp/
├── plan.md                       # This file
├── spec.md                       # Feature specification
├── research.md                   # Phase 0 output — decisions R1–R20
├── data-model.md                 # Phase 1 output — generic presentation entities, theme tokens, help taxonomy
├── quickstart.md                 # Phase 1 output — validation scenarios Q1–Q12 + F1/F2 regression
├── contracts/                    # Phase 1 output
│   ├── cli-work-home.md          # interactive static brand (exit 0) / non-interactive usage (exit 2)
│   ├── cli-help.md               # `work --help` brand + grouped command inventory rules
│   ├── brand.md                  # `WORK` wordmark: terminal art, primary→secondary gradient, responsive/plain fallback
│   ├── theme.md                  # semantic token set + colour capability / opt-out rules
│   ├── interaction.md            # step lifecycle, receipts, ephemeral errors, bounded frame, selector geometry, keys, cancellation
│   ├── diagnostics.md            # interactive vs non-interactive diagnostic rendering, single-border rule, cause inspection
│   └── presentation-boundary.md  # generic `present` API shape + the domain-free import ban (testable)
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

Changes and additions to the F1/F2 tree:

```text
internal/
├── present/                      # NEW — the generic inline interaction boundary (ADR-0020, ADD §12.2)
│   ├── present.go                #   IO{In,UI}, shared session runner, editing/completed/cancelled state model
│   ├── input.go                  #   Input(ctx, io, InputSpec) — text field, in-frame Validate closure, receipt, Secret
│   ├── select.go                 #   Select[T](ctx, io, SelectSpec[T]) — single-select, optional groups/tabs, filter
│   ├── multiselect.go            #   MultiSelect[T](ctx, io, MultiSelectSpec[T]) — fixed-width checkbox, independent focus
│   ├── confirm.go                #   Confirm(ctx, io, ConfirmSpec) — impact preview → compact confirmation receipt
│   ├── receipt.go                #   compact receipt / cancellation-notice rendering shared by every primitive
│   ├── geometry.go               #   viewport budget from WindowSizeMsg; display-width truncation (secondary before primary)
│   ├── theme/
│   │   ├── theme.go              #   semantic tokens (Primary/Secondary/Success/Warning/Danger/Muted/Text) + light variants
│   │   └── capability.go         #   TTY / colour-profile / NO_COLOR / TERM=dumb detection → active variant
│   ├── brand/
│   │   ├── brand.go              #   wordmark assembly: full art + gradient, compact plain fallback, responsive choice
│   │   └── wordmark.go           #   embedded static `WORK` terminal-art glyphs (no figlet dependency)
│   └── diagrender/
│       └── diagrender.go         #   interactive human renderer (✘ summary + hint) + cancellation special case
├── cli/
│   ├── root.go                   # bare `work`: interactive → brand (exit 0); non-interactive → usage (exit 2); delete home dispatch
│   ├── help.go                   # NEW — cobra.Group registration + custom help template/renderer from the command tree
│   ├── start.go                  # migrate path/slug retry loops to present.Input Validate closures; add receipts; explicit IO
│   ├── resume.go                 # migrate the recency picker to present.Select; concise cancellation via the border
│   ├── archive.go                # migrate to present.MultiSelect + present.Confirm; dirty-ack via present.Confirm
│   └── diagnostics_border.go     # NEW — the single CLI/process diagnostic border (interactive vs stable renderer choice)
├── tui/                          # SHRINKS then is removed
│   ├── home.go                   # DELETE — the selectable home (superseded by the static brand)
│   ├── home_test.go              # DELETE
│   ├── basebranch_picker.go      # DELETE after start.go moves to present.Select (grouped)
│   ├── resume_picker.go          # DELETE after resume.go moves to present.Select
│   ├── archive_picker.go         # DELETE after archive.go moves to present.MultiSelect
│   ├── prompts.go                # DELETE after start.go moves to present.Input/Confirm
│   └── tty.go                    # MOVE — IsInteractive/MustInteractive fold into present (capability) or a tiny ttyx package
└── diag/
    └── diag.go                   # + Error.Summary, Error.Hint (optional) fields; + a recoverable-field sentinel;
                                  #   categories, tokens, codes, and the All table are UNCHANGED

tests/
├── integration/                  # + interactive_receipts_test, interactive_geometry_test,
│                                 #   brand_render_test, help_groups_test, diag_border_test (all pty / golden);
│                                 #   + non-interactive: no_ansi_when_piped.txtar, no_color_env.txtar,
│                                 #     help_inventory.txtar, bare_work_exit.txtar
├── contract/                     # + present import-boundary test
└── fixtures/                     # + narrow / no-colour terminal fixtures for the brand and selectors
```

**Structure Decision**: Single Go project (unchanged). The new logic is one
role-focused tree, `internal/present`, that mirrors ADD §12.2–12.4: primitives own
interaction state and geometry; `theme` owns the semantic token set and capability
detection; `brand` owns the wordmark; `diagrender` owns the interactive human
diagnostic. `internal/cli` keeps all journey sequencing, option construction,
validation closures, and the single diagnostic border. `internal/tui` is retired
package-by-package as each `cli` command moves over, so the tree never has two active
implementations of the same control. `internal/diag` keeps its stable table and only
gains presentation fields. No other package changes.

## Branching Strategy

F2.5 follows the same git-flow shape as F1/F2: one short-lived
`feature/003-terminal-ux-revamp-p<n>-*` branch per `tasks.md` phase, each cut from
the previous phase's tip, merged forward at the phase **Checkpoint** once
`make lint` + `go test ./...` are green on the CI matrix.

**Base branch**: F1 + F2 live on **`develop`** (`master` carries docs only). The PRD
v3 / ADR-0019 / ADR-0020 / ADD §12 doc edits and this spec are the p0 payload. F2.5
phase branches are cut from `develop` (or from the prior F2.5 phase branch when it
has not merged).

> **Merge cadence is the user's call.** Per the standing project note the F1/F2 phase
> branches were in practice **stacked** (phase N+1's PR targets phase N's branch, not
> `develop`). Before starting a phase, `/speckit-implement` MUST confirm with the
> user which branch to base the new phase branch on and which branch its PR targets —
> do not assume `develop`.

| Phase (tasks.md) | Feature branch | Cut from |
|---|---|---|
| 0 — Specs & doc supersession (PRD v3, ADR-0019, ADR-0020, ADD §12, this spec set) | `feature/003-terminal-ux-revamp-p0-specs` | `develop` |
| 1 — Setup & PTY proof (capability detection, a throwaway `Input`+`Select` that final-renders a receipt; verify scrollback on Linux + macOS) | `feature/003-terminal-ux-revamp-p1-setup` | phase 0 tip |
| 2 — Foundational (`present` primitives, `theme`, `geometry`, `receipt`, import-boundary test) | `feature/003-terminal-ux-revamp-p2-foundational` | phase 1 tip |
| 3 — US1 Inline lifecycle for `work start` 🎯 MVP (migrate path/slug/prefix/base/workspace/confirm; receipts; in-frame validation) | `feature/003-terminal-ux-revamp-p3-us1-start` | phase 2 tip |
| 4 — US3 Stable selectors + US4 diagnostics (migrate resume/archive pickers; fix the archive double-render defect; the diagnostic border + cancellation notice) | `feature/003-terminal-ux-revamp-p4-us3-us4` | phase 3 tip |
| 5 — US2 Brand home & grouped help (wordmark + gradient, responsive/plain fallback; `cobra.Group` + custom help; delete `homeModel`) | `feature/003-terminal-ux-revamp-p5-us2-brand` | phase 4 tip |
| 6 — US5 Non-interactive compatibility sweep + polish (no-ANSI / `NO_COLOR` / `TERM=dumb` tests; F1/F2 regression; remove `internal/tui`; `huh` keep-or-drop decision) | `feature/003-terminal-ux-revamp-p6-polish` | phase 5 tip |

Rules (identical spirit to F1/F2):

- **Naming**: `feature/003-terminal-ux-revamp-p<n>-<short>` — the phase identifier
  stays in one hyphen-separated segment under `feature/` (no nested path).
- **First action of each phase**: confirm base/target with the user, then
  `git switch <base> && git pull && git switch -c <feature-branch>`.
- **Merge gate**: a phase merges forward only after its **Checkpoint** in `tasks.md`
  is met and `make lint` + `go test ./...` are green on all three OSes, with every
  prior slice's automated demo (F1 S1–S12, F2 S1–S13, and earlier F2.5 phases) still
  green.
- **`internal/tui` retirement**: a picker file is deleted in the same phase that
  moves its `cli` caller to `present`, so `go test ./...` never compiles two
  implementations. `internal/tui` disappears entirely in phase 6.
- **Release**: when phase 6 merges, F2.5 ships via `release/0.3` cut from `develop`,
  merged to `master` and tagged (standard git-flow release). The release step is the
  hand-off, not a task.

Spec/plan/contract/doc edits stay on the p0 branch; the per-phase feature-branch rule
covers implementation code only.

## Complexity Tracking

No Constitution Check violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
