# Phase 0 Research: Terminal UX Revamp (F2.5)

**Feature**: `specs/003-terminal-ux-revamp/` · **Plan**: [plan.md](./plan.md) · **Date**: 2026-09-07

This document resolves every unknown in the plan's Technical Context. Each entry is
**Decision / Rationale / Alternatives considered**. The spec carries **no
`NEEDS CLARIFICATION` markers** (checklist iteration 1 passed on 2026-09-07). The
open decisions listed in `temp/tui-revamp.md` §15 are resolved here (R7, R11, R13,
R16–R18).

Governing sources: `docs/prd.md` v3 RF-50–RF-64 / RNF-10–RNF-11;
`docs/adr/adr-0019-superficie-cli-com-home-estatica.md`;
`docs/adr/adr-0020-fronteira-generica-de-apresentacao-inline.md`;
`docs/add/add-0001-work-system-architecture.md` §12; `docs/roadmap.md` §4 + F2.5;
`temp/tui-revamp.md`; the F1/F2 design sets; the current Go implementation.

---

## R1 — Keep the Charm stack; add a Work-owned boundary (ADR-0020, FR-031)

**Decision.** Retain Cobra, Bubble Tea v2, and Lip Gloss v2. Introduce
`internal/present` as the single generic inline-interaction boundary. `huh` stays
*behind* `present.Input` / `present.Confirm` as an implementation detail during
migration; whether it survives is decided in R19 and binds no contract.

**Rationale.** `temp/tui-revamp.md` §4 shows the stack already supports inline
rendering, `WindowSizeMsg`, a graceful final render, arbitrary models, custom themes,
and inline validation. The observed defects are structural (validation outside the
field, pickers quitting while expanded, a local double-render bug, scattered theme
literals, domain DTOs crossing the boundary), not library limits. ADR-0020 accepts
exactly this: keep the stack, add the boundary.

**Alternatives considered.**
- *Replace the Charm stack (Survey, promptui, `gum`)* — rejected (ADR-0020, §11.D):
  trades a capable tested renderer for migration risk, loses model testability, and
  `gum` adds a subprocess + packaging boundary while making closure-based validation
  harder.
- *One big `huh` form for `work start`* — rejected (§11.B): domain-dependent steps
  and external validation complicate form construction, the receipt lifecycle needs
  view hooks, and sequencing migrates into the form; it does nothing for
  resume/archive.
- *Full-screen alternate buffer* — **adopted in phase 7** (ADR-0021, R21). Rejected
  in the original slice because it seemed to erase the useful history of accepted
  decisions; the phase-7 design reprints the receipt trail to the primary buffer on
  exit, so that objection no longer holds, and a full clear+repaint every frame is
  the only structural fix for the inline renderer's resize / back-nav ghosting.
- *Patch only the archive bug + colours* — rejected (§11.F): leaves validation
  scars, missing receipts, retained picker frames, domain-aware APIs, and the
  unhelpful home; insufficient as the F2.5 slice.

---

## R2 — `internal/present` package shape and generic API (ADD §12.2)

**Decision.** `internal/present` exposes four primitives plus an explicit I/O struct:

```go
type IO struct {
    In io.Reader // normally cmd.InOrStdin()
    UI io.Writer // normally cmd.ErrOrStderr()
}

type InputSpec struct {
    Title, Description, Initial string
    Validate func(context.Context, string) error // recoverable → shown in-frame; wrap present.Fatal(err) to abort
    Receipt  func(accepted string) string        // nil ⇒ default "<value>"
    Secret   bool                                // receipt shows "••••" / caller Receipt still wins
}

type Option[T any] struct {
    Value            T
    Primary, Secondary, Group string
}

type SelectSpec[T any] struct {
    Title, Description string
    Options   []Option[T]
    Grouped   bool // render a group/tab bar; groups come from Option.Group
    Filterable bool
    Receipt   func(Option[T]) string
}

type MultiSelectSpec[T any] struct {
    Title, Description string
    Options   []Option[T]
    Filterable bool
    Confirm   *ConfirmSpec // optional impact preview after Enter, before returning
    Receipt   func(picked []Option[T]) string
}

type ConfirmSpec struct {
    Title   string
    Impact  string // multi-line preview shown until acceptance
    Accept, Reject string
    Receipt func(accepted bool) string
}

func Input(ctx context.Context, io IO, spec InputSpec) (string, error)
func Select[T any](ctx context.Context, io IO, spec SelectSpec[T]) (T, error)
func MultiSelect[T comparable](ctx context.Context, io IO, spec MultiSelectSpec[T]) ([]T, error)
func Confirm(ctx context.Context, io IO, spec ConfirmSpec) (bool, error)
```

Cancellation (`q` / `Esc` / `Ctrl-C`) returns `diag.New(diag.Cancelled, "cancelled")`
so the existing exit-20 contract is preserved unchanged. This is the one dependency
`present` has on `internal/diag` — a leaf package with no domain imports (R3).

**Rationale.** Matches ADD §12.2 and `temp/tui-revamp.md` §8.3. Generics keep the
opaque `Value` out of the renderer; the CLI maps a selected `Value` straight back to
its domain object. Closures are the missing seam that keeps domain rules in the CLI
while letting the active control own their transient display.

Phase 7 adds `present.Wizard(ctx, IO, WizardSpec{Title, Initial, Steps})` over
ordered `Step`s (`InputStep` / `SelectStep` / `MultiSelectStep` / `ConfirmStep`,
each with a `build(Answers) (spec, error)` closure so a later step reads earlier
answers); `Input` / `Select` / `MultiSelect` / `Confirm` keep their signatures as
one-step wizards. `StepResolved(answer)` lets a build skip a step a flag already
answered.

**Alternatives considered.**
- *One program for the whole interview* — **adopted in phase 7** (R21, ADR-0021).
  Deferred in the original slice as only easing back-nav (still out of scope); the
  real driver turned out to be eliminating the inline renderer's ghosting and
  keeping accepted receipts visible above the active step. Sequencing stays in the
  CLI via the per-step `build(Answers)` closures — no async protocol.
- *Return a typed `Cancelled` sentinel instead of `diag.Error`* — rejected: every
  caller already switches on `diag` categories; reusing `diag.Cancelled` keeps the
  border code and the exit-code mapping untouched.

---

## R3 — The domain-free boundary is a mechanical test, not a convention (FR-004, ADR-0020)

**Decision.** `internal/present` (and subpackages) may import only: the Go stdlib,
`charm.land/{bubbletea,lipgloss,huh}/v2`, `charm.land/bubbles/v2`,
`github.com/charmbracelet/colorprofile`, `golang.org/x/term`, the runewidth/uniseg
helpers, and `internal/diag`. A test in `tests/contract` runs
`go list -deps ./internal/present/...` and fails if any `internal/*` package other
than `diag` appears. `internal/diag` itself keeps its current zero `internal/`
imports.

**Rationale.** ADR-0020: "não importa conceitos nem tipos de domínio do Work." A
review convention rots; a build-time list check does not. `diag` is allowed through
because it is the shared error vocabulary and is already domain-free.

**Alternatives considered.**
- *Allow `internal/worklist` "just for `WorkRow`"* — rejected: that is exactly the
  DTO leak `temp/tui-revamp.md` §3.5 calls out. The CLI builds `Option[T]` values
  from `WorkRow`; `present` sees only `Primary`/`Secondary`/`Group` strings and an
  opaque `Value`.

---

## R4 — Step lifecycle and the final render (FR-003, FR-005, FR-007, ADD §12.2)

**Decision.** Each primitive is a `stepModel` with explicit states:

```text
editing ──invalid──▶ editing (error replaced in-frame, one error max)
   │
   ├──accepted──▶ status.done (compact receipt)  ──▶ wizard appends receipt, advances
   └──cancel────▶ status.cancelled               ──▶ wizard quits
```

For `Confirm` / `MultiSelect.Confirm`: `choosing → confirming → done`; `Esc`
from `confirming` returns to `choosing` (no mutation — `present` cannot mutate);
`Ctrl-C` goes to `cancelled`. Phase 7: a step never calls `tea.Quit` itself — it
reports a terminal `status()` and the enclosing `present.Wizard` drives the
transition. The receipt is: `<title>` line, then `  ✔ <value>` (or the caller's
`Receipt`), then one blank separator line. While the wizard runs the receipt trail
stays visible above the active step; **the durable render is the wizard's reprint
of that trail to the primary buffer after `?1049l` tears down the alt screen**, and
the cancellation notice (`✘ Operation cancelled`, no title echo) is the diagnostic
border's, printed to the restored primary buffer.

**Rationale.** `temp/tui-revamp.md` §3.2/§8.4 and §6.1. `huh`'s standalone form
returns an empty view on completion (values vanish); the hand-rolled pickers never
switch `View()` to a compact frame (padded blank region survives). Bubble Tea
explicitly guarantees a final render on graceful shutdown — the fix is to make the
final `View()` be the compact frame.

**Alternatives considered.**
- *Second UX option — replace each step with the next* — rejected as default
  (spec §6.2 / §11.G): the user cannot verify prior inputs before a destructive
  confirmation, and the concise audit trail that makes an inline CLI feel native is
  lost. Kept as a possible future "minimal" mode only.

---

## R5 — Bounded frame and viewport budget (FR-002, FR-006, SC-004)

**Decision.** Every model tracks `width,height` from `tea.WindowSizeMsg`. The list
region height is `height − reserved`, where `reserved` = title (1–2) + optional group
bar (2) + scroll line (1) + filter line (1) + error line (1) + help line (1) +
optional confirmation block. If the computed list height is < 1 the model still
renders at least the title + one row + help and lets the terminal scroll (documented
degenerate case in `contracts/interaction.md`). Content wider than `width` is
truncated with `…` using **display width** (`runewidth` / `uniseg`), never byte
length; **secondary metadata is truncated before primary identity**, and primary is
truncated before being dropped. No frame is ever emitted taller than `height` (the
one thing no renderer can undo once it scrolls into scrollback).

**Rationale.** `temp/tui-revamp.md` §3.3/§8.6 and ADD §12.3. The current pickers use
fixed row counts (8 / 6 / 6) with blank padding and never consult the window; on a
short terminal they overflow, on a tall one they waste rows and — worse — keep the
padded frame after completion.

**Alternatives considered.**
- *Keep fixed heights, just collapse on completion* — rejected: fixes the
  scrollback-debris half but not SC-004 (40×10 overflow). Both are required.
- *Always size the list to the full window* — rejected for the base-branch selector,
  which SHOULD keep a generous but not maximal viewport (FR-007); the budget formula
  gives a `VisibleRows` hint the caller can cap.

---

## R6 — Selector geometry: fixed columns, bold focus (FR-008, FR-009, SC-003)

**Decision.** Rules baked into `select.go` / `multiselect.go` / `geometry.go`:

1. Every row reserves the **same** leading indicator column — a fixed 2-cell slot
   holding `❯ ` on the focused row and `  ` otherwise (equal display width; the
   marker is textual, not colour).
2. The focused row (both lines of a two-line row) is **bold** and MAY take the
   `Primary` accent; bold alone identifies it with colour off.
3. Checkbox glyphs are a fixed 4-cell slot: `[ ] ` / `[x] `. Checked state and focus
   state are independent.
4. A two-line option moves and styles as one unit; the second line's indent equals
   the first line's content column exactly.
5. Filtering, toggling, group/tab changes, and cursor movement never add or remove a
   printable-width prefix and never change an unchanged row's line count.
6. All widths measured with display-width helpers.

The F2 archive double-render defect (`archive_picker.go` appends the first line twice
for unselected rows) is deleted in the rewrite and gets an explicit
equal-height/equal-column regression test.

**Rationale.** `temp/tui-revamp.md` §3.4/§6.1/§8.6 and ADD §12.3. `SC-003` demands
zero change in rendered line count and zero change in the starting columns of the
focus marker, checkbox, primary text, and secondary text across every interaction.

**Alternatives considered.**
- *Keep the `"> "` vs `"  "` prefix (equal nominal width)* — rejected: it is equal
  width only when every view is disciplined; combined with theme padding and the
  archive bug the columns visibly jitter today. A single reserved fixed slot removes
  the whole class of bug.

---

## R7 — Confirmation history: collapse to a receipt (FR-030, RF-64; open decision §15.1)

**Decision.** An accepted pre-mutation confirmation **collapses to a one-line
receipt** (`✔ <title> confirmed`, or the caller's `ConfirmSpec.Receipt`) *before* the
command writes its stable success lines. The full impact preview stays visible until
the user accepts. The durable operation summary is the command's existing stable
stdout (e.g. `work: created <id>` / `work: branch …` / `work: path …`), so repeating
the preview afterward is redundant.

**Rationale.** RF-64 is explicit: "quando aceitas, reduzir-se a um recibo compacto
antes do resultado estável." `temp/tui-revamp.md` §6.1 and §15.1 flag both options as
defensible and ask for a PTY prototype; RF-64 settles it. The preview is the mutation
consent surface, not the audit record.

**Alternatives considered.**
- *Keep the detailed preview in history* — rejected: duplicates the stable stdout
  summary and reintroduces a tall frame between steps.
- *Follow the confirmation with a compact operation summary from `present`* —
  rejected: the command already owns the stable summary on stdout; `present` must not
  format operation results.

---

## R8 — Recoverable vs fatal validation (FR-005, FR-014; `temp/tui-revamp.md` §7)

**Decision.** `InputSpec.Validate` (and the equivalent hook wherever a selection
needs post-choice checking) communicates by its return value:

- `nil` → accept, render the receipt.
- a plain `error` (or a `*diag.Error` in a field-recoverable category:
  `InvalidPath`, `UnusableRepo`, `InvalidBranchName`, `BranchCollision`) → show
  `✘ <message>` inside the active frame, stay in `editing`, replace any prior error.
- `present.Fatal(err)` (a thin wrapper) → stop the program and return `err` to the
  CLI, which sends it to the single diagnostic border.

The CLI's closures call the existing domain validators (`reporef.ValidatePath`,
`branchname.Validate`, `branchname.DetectCollision`, a read-only
`starter.Invoke` + `reporef.ValidatePath`) and return their `*diag.Error` directly;
`present` shows the carried `Msg`. No `fmt.Fprintln(errOut, diag.Format(...))` inside
the flow anymore.

**Rationale.** `temp/tui-revamp.md` §3.1/§7.3: the current split is correct about
*where* domain rules live but wrong that "return an error" is the only channel to the
field. The closure is the seam. Classification stays in the CLI (it knows which
categories are re-promptable — the existing `retryable()` helper's category list).

**Alternatives considered.**
- *`present` inspects `diag` categories itself* — rejected: that is domain-policy
  knowledge; `present` should treat any non-`Fatal` error as "show it, stay editing"
  and let the CLI decide what is fatal by wrapping.

---

## R9 — Async validation feedback (spec Assumptions; open decision §15.3)

**Decision.** `Input` runs `Validate` in a `tea.Cmd`. If it has not returned within
~120 ms the frame shows a neutral `⋯ checking…` line (no colour semantics, replaces
the help line, not an error). The result then either accepts or shows the error. The
closure remains side-effect-free and safe to repeat; `present` never learns why it
was slow.

**Rationale.** Starter / git inspection is usually fast but can spike;
`temp/tui-revamp.md` §15.3. Keeping the call in a `tea.Cmd` also keeps `Ctrl-C`
responsive during validation (FR-011).

**Alternatives considered.**
- *Block the model on `Validate`* — rejected: a slow read would freeze the frame and
  swallow `Ctrl-C`.

---

## R10 — Diagnostic border split (FR-014–FR-016, ADD §12.4)

**Decision.** `internal/diag` gains two optional fields on `Error` —
`Summary string` (public, user-vocabulary) and `Hint string` (next action) — and a
`Cause() error` accessor (alias for `Unwrap`). **No new category, token, or code.**
The `All` table test is untouched. Rendering splits at the CLI/process border
(`internal/cli/diagnostics_border.go`):

| Context | Renderer | Output |
|---|---|---|
| non-interactive UI channel | `diag.Format` (unchanged) | `error: <token>: <message>` — one line, byte-for-byte as today |
| interactive UI channel, `Cancelled` | `diagrender` special case | `✘ Operation cancelled` (one line), exit 20 |
| interactive UI channel, other | `diagrender` | `✘ <Summary or Msg>` then, if `Hint != ""`, `  → <Hint>` |

Only the border prints. Every lower layer (`present`, orchestrators, domain) returns
structured errors. The wrapped cause chain is retained and is printed **only** when
`WORK_DEBUG` is set to a non-empty value (an env affordance, not a new flag — the
"no new `--no-color` flag" scope note is about colour, not diagnostics; documented in
`contracts/diagnostics.md`).

**Rationale.** `temp/tui-revamp.md` §7.3/§8.2. Keeps the non-interactive contract
frozen (scripts still parse `error: token: message`), gives humans a clean single
notice, and keeps causes debuggable without polluting normal output (FR-016).

**Alternatives considered.**
- *A new `--debug` flag* — rejected: an env var is enough and adds no public surface
  (ADR-0019 forbids surface growth without need).
- *Structured multi-line human diagnostics for every category* — deferred: `Summary`
  + optional `Hint` covers the spec; a richer `detail` block can come later without a
  contract change.

---

## R11 — Bare `work` and the static brand (FR-017–FR-019, FR-028, ADR-0019)

**Decision.** `internal/cli/root.go` `RunE`:

- **stdin & stdout both TTY**: render the brand (R12) to **stdout**, exit 0. No
  selector. (`work --help` direction is part of the brand block.)
- **otherwise**: the current behaviour is preserved exactly — `diag.New(diag.Usage,
  "run \`work start …\`; see \`work --help\`")`, printed by the border to stderr,
  exit 2.

`tui.RunHome`, `homeModel`, `HomeChoice`, and `home_test.go` are deleted. The bare-
command dispatch table in `runHome` is removed; `work start` / `work resume` /
`work archive` are reached only by name (they always were, for scripts).

**Rationale.** ADR-0019 decision bullets 1 and 5; RF-50, RF-63; `root_test.go`
`TestBareWorkNonInteractiveIsUsage` already asserts the non-interactive path and
stays green.

**Alternatives considered.**
- *Print the full help on bare interactive `work`* — rejected (ADR-0019
  alternatives): a short branded entry keeps identity without dumping information
  when the user is just probing the command.
- *Keep a minimal home with only "start / resume / archive"* — rejected: ADR-0019
  removes the selectable home entirely; discovery is the brand + `work --help`.

---

## R12 — Wordmark: terminal art + primary→secondary gradient (FR-018, FR-019, SC-009)

**Decision.** Ship a static, embedded multi-line block-style `WORK` wordmark as a Go
string constant in `internal/present/brand/wordmark.go` (no `figlet` / no runtime
generator — spec Assumptions). The renderer:

- **wide (`width ≥ wordmark width`) + true-colour (or 256-colour) + not disabled**:
  print the art; colour each column by interpolating from `#11A8CD` to `#8B7CF6`
  across the wordmark's total width (`lipgloss` colour blend / manual RGB lerp per
  cell), so the gradient runs left→right through all four glyphs. Then the tagline
  and `Run 'work --help' to get started.`, centred to `width` (capped).
- **narrow, or ≤16-colour, or monochrome**: print the compact plain form — the
  single word `WORK` (optionally bold) + tagline + help direction, no escapes beyond
  an optional bold.
- **colour disabled** (non-TTY / `NO_COLOR` / `TERM=dumb`): plain `WORK` art if it
  fits, else the compact word; **zero** styling bytes.

Tagline: `Isolated work, ready when you are.` (draft — confirmed in visual review,
SC-009). The exact glyph design is chosen and screenshotted during phase 5 and
recorded in `contracts/brand.md`; the draft in `temp/tui-revamp.md` §10.1 is the
starting point.

**Rationale.** FR-018 requires terminal art spelling `WORK` with a visible
primary→secondary gradient in a suitable true-colour terminal; FR-019 requires a
compact plain fallback without broken wrapping. SC-009 wants ≥90% of reviewers to
read it as `WORK` and rate gradient + fallback readable — a visual gate, hence the
phase-5 screenshot record.

**Alternatives considered.**
- *Runtime `figlet`/`go-figure` dependency* — rejected (spec Assumptions,
  `temp/tui-revamp.md` §10.1): adds a product dependency for a static asset.
- *Per-glyph solid colours instead of a gradient* — rejected: FR-018 says "degradê …
  ao longo do wordmark."
- *ANSI-shaded blocks in the fallback* — rejected: FR-019 / FR-025 require no unsafe
  styling in the compact form.

---

## R13 — Semantic theme + capability detection (FR-023–FR-025, ADD §12.3)

**Decision.** `internal/present/theme` defines one token set, injected everywhere:

| Token | Dark | Light | Role |
|---|---|---|---|
| `Primary` | `#11A8CD` | `#087C99` | titles, focused option, active group/tab, brand start |
| `Secondary` | `#8B7CF6` | `#5D4CC9` | key hints, secondary emphasis, brand end |
| `Success` | `#2DBE8C` | `#167A5B` | `✔` receipt mark |
| `Warning` | `#D9A521` | `#8A6D1A` | non-fatal notices |
| `Danger` | `#FF5C7A` | `#B4233F` | `✘` validation / destructive emphasis |
| `Muted` | adaptive gray | adaptive gray | descriptions, secondary metadata, scroll line |
| `Text` | terminal foreground | terminal foreground | body text |

Capability (`theme/capability.go`), evaluated once per process from the UI writer:

- `colorEnabled = isTTY(UI) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"`
  — matches `NO_COLOR` semantics (only a **non-empty** value disables; spec edge
  case) and `colorprofile`.
- `profile ∈ {NoColor, ANSI16, ANSI256, TrueColor}` from `colorprofile.Detect`.
- Light vs dark from Bubble Tea's `RequestBackgroundColor` (already used by the
  pickers) with a dark default.
- `colorEnabled == false` ⇒ every style degrades to plain; **bold and the textual
  focus marker / `✔` / `✘` marks remain** (never colour-only meaning, SC-003/SC-008).

`✔` / `✘` fall back to fixed-width ASCII `ok` / `x` when the profile / charset can't
render them predictably (decided per platform in phase 1 PTY testing; default keeps
the Unicode marks).

**Rationale.** ADD §12.3 and `temp/tui-revamp.md` §9. Removes the scattered
`ThemeCharm` reconstruction and the hard-coded `#F780E2` / `241` literals in the
three pickers.

**Alternatives considered.**
- *A user-configurable theme / `--no-color` flag* — explicitly out of scope (spec).
  Only the env-based opt-outs are honoured.

---

## R14 — Explicit I/O to every program (FR-026, ADD §12.4)

**Decision.** `present.IO{In, UI}` is built by each `cli` command from
`cmd.InOrStdin()` and `cmd.ErrOrStderr()`. Every `tea.NewProgram` is created with
`tea.WithInput(io.In)` and `tea.WithOutput(io.UI)` (today they use process
defaults). The brand on bare `work` is the one deliberate exception — it goes to
**stdout** because it is the command's successful result, not interactive UI (it is
static text, no program). Stable command results (`work: created …`, `work: resumed
…`, JSON from future read commands) stay on stdout untouched.

**Rationale.** `temp/tui-revamp.md` §3.6. Contracts already say interactive UI →
stderr, stable results → stdout; the programs just weren't wired to the injected
streams, which also makes tests non-deterministic.

**Alternatives considered.**
- *Brand to stderr* — rejected: `work` with no args in an interactive terminal is a
  successful invocation whose entire output is the brand; a user piping `work | less`
  is non-interactive and gets the exit-2 usage path instead.

---

## R15 — `work --help`: grouped, inventory-driven (FR-020, FR-021, SC-005, ADR-0019)

**Decision.** Register `cobra.Group`s on the root command and set `GroupID` on each
subcommand:

| GroupID | Title | Members (as they ship) |
|---|---|---|
| `daily` | `Daily Commands` | `start`, `resume`, `archive` (+ later `status`) |
| `in-work` | `Inside a Work` | (later: `status`, `import`, `link`) |
| `admin` | `Administration` | (later: `plugin`, `repository`, `convention`) |
| `setup` | `Setup` | `shell-init`, `completion` |

A custom help function (`internal/cli/help.go`, set via `root.SetHelpFunc` /
`SetUsageTemplate`) renders: the **compact** brand line, `Usage:`, `Flags:`, then one
block per **non-empty** group in the fixed order above, each listing its registered
commands with `Short`. Cobra's auto-generated `help` and `completion` commands are
assigned to `setup` (or `completion` is grouped and `help` hidden) so no command is
ungrouped. An unregistered command cannot appear (the tree is the only source). A
test walks `root.Commands()` and asserts every non-hidden command shows exactly once
under a group and no empty group renders (SC-005).

**Rationale.** ADR-0019 §12.1 / RF-50; `temp/tui-revamp.md` §10.2. Cobra already
owns the live inventory — a custom template cannot drift from registration.

**Alternatives considered.**
- *A hand-maintained command list in the help text* — rejected (ADR-0019
  alternatives): duplicates the inventory and can diverge.
- *Hide `completion` entirely* — acceptable but kept visible under `Setup` for
  discoverability; decided in phase 5.

---

## R16 — ASCII fallback marks (spec Assumptions; open decision §15.4)

**Decision.** Default to `✔` (success) and `✘` (failure/cancel), each rendered in a
**fixed 1-cell slot with a trailing space** so ASCII and Unicode variants occupy the
same column. Fallback to `ok` / `x` (padded to equal width) only when phase-1 PTY
testing shows a supported platform / console renders the glyphs at unstable width
(Windows legacy console is the prime suspect). The choice is process-global, from the
same capability probe as colour.

**Rationale.** `temp/tui-revamp.md` §9/§15.4: the marks are desired but Windows /
unusual fonts need testing; define the fallback only if evidence requires it, and
never let width jitter.

---

## R17 — Non-interactive bare `work` exit code (FR-028; open decision §15.5)

**Decision.** Unchanged: non-interactive bare `work` keeps the concise usage failure
and **exit 2**; `work --help` exits **0** in every mode; interactive bare `work`
exits **0** after the brand. This is the compatibility posture `root_test.go` already
encodes and ADR-0019 ratifies.

**Rationale.** Keeps accidental script invocation of a bare `work` detectable
(non-zero) while giving humans the landing surface. RF-63.

---

## R18 — Migration order (roadmap F2.5; `temp/tui-revamp.md` §13)

**Decision.** Phases as in `plan.md` Branching Strategy:

0. Specs + doc supersession (this set).
1. Capability detection + a throwaway `Input`+`Select` PTY proof: in-frame
   invalidation, compact final receipt, verified scrollback on Linux + macOS.
2. `present` primitives + `theme` + `geometry` + `receipt` + the import-boundary
   test. Fix the archive double-render defect here with height/column invariants even
   though `archive_picker.go` is still the live picker, so the regression test exists
   before the rewrite.
3. Migrate `work start` (US1): path/slug/prefix/base/workspace/confirm onto
   `present`; receipts; in-frame validation closures; delete `prompts.go` +
   `basebranch_picker.go`.
4. Migrate resume/archive selectors (US3) + the diagnostic border and cancellation
   notice (US4); delete `resume_picker.go` + `archive_picker.go`.
5. Brand home + grouped help (US2); delete `home.go` / `homeModel`.
6. Non-interactive compatibility sweep (US5) + F1/F2 regression + remove
   `internal/tui` + the `huh` keep-or-drop decision (R19).
7. **Follow-up (R21, ADR-0021):** move the three flows onto a single full-screen
   `present.Wizard` per flow; hash-free base selector; VT emulator alt-screen
   support. Every F1/F2 non-interactive contract stays byte-for-byte.

**Rationale.** Prototype the risky final-frame behaviour before broad refactoring;
keep each phase independently shippable and green; never compile two implementations
of one control at once.

---

## R19 — `huh` retention (deferred; `temp/tui-revamp.md` §15.7)

**Decision.** Keep `huh` behind `present.Input` / `present.Confirm` through phases
2–5. In phase 6, once every entry point uses `present`, evaluate whether `huh` still
reduces code versus a `bubbles/textinput` + small confirm model. Removing or keeping
it changes no contract and no CLI code (ADR-0020 final bullet). Record the outcome in
`plan.md` post-design notes or a follow-up ADR if the team deems the boundary
significant.

**Rationale.** The decision should be evidence-based and localized; nothing
user-facing depends on it.

**Alternatives considered.**
- *Drop `huh` up front* — rejected: adds text-editing / cursor / paste / Unicode
  work to the critical path with no requirement forcing it now.
- *Commit to `huh` permanently* — rejected: its standalone-form lifecycle is not the
  receipt lifecycle; keeping the option open costs nothing.

---

## R20 — PTY / golden test strategy (SC-001–SC-004, SC-008, roadmap §4)

**Decision.** Three test layers:

1. **Model unit tests** (`internal/present/*_test.go`): drive `Update` with synthetic
   `KeyPressMsg` / `WindowSizeMsg`; assert `View()` strings, `editing`/`completed`/
   `cancelled` state, one-error invariant, equal row width/height, fixed columns,
   viewport ≤ height. Reuse the existing `press(k)` / `apStep` helpers' style.
2. **Golden views** (`tests/integration` or package-local `testdata/`): the wordmark
   in wide-true-colour / wide-no-colour / narrow; `work --help` group blocks; a
   completed-step receipt; a cancellation notice.
3. **PTY integration** (`tests/integration`, `//go:build unix`, existing `console`
   harness at 40×10 / 80×24 / 160×50): the four-invalid-then-valid path story; slug
   retry story; base selector large→compact; confirmation with no padded frame;
   archive row traversal with zero movement; resume cancel → one notice + exit 20 +
   no access bump; UI bytes on the UI writer, stable bytes on stdout.

Non-interactive: `testscript` `.txtar` — redirected runs emit no ANSI (assert with a
`! stdout '\x1b\['`-style check), `NO_COLOR=1` / `TERM=dumb` plain, bare `work`
exit 2, `work --help` exit 0 and lists exactly the registered commands. The full F1
(`specs/001-first-local-work/quickstart.md` S1–S12) and F2
(`specs/002-daily-cycle/quickstart.md` S1–S13) suites run unchanged (SC-007).

**Rationale.** `temp/tui-revamp.md` §14 and the roadmap §4 process/portability/
regression gates. The `console` PTY harness and `ansiRe` scrubber already exist.

**Alternatives considered.**
- *Only model tests* — rejected: cannot prove the scrollback / final-frame behaviour,
  which is the whole point (SC-002, SC-004).
- *Only PTY tests* — rejected: slow, `unix`-only, and poor at pinning exact geometry;
  model + golden tests give fast deterministic coverage of SC-003.

---

## R21 — Phase 7: one full-screen wizard per flow (ADR-0021)

**Decision.** Move `work start` / `resume` / `archive` from per-step inline Bubble
Tea programs to **one alternate-screen program per flow** — a `present.Wizard` over
ordered `Step`s. The four primitives become a domain-free `stepModel`
(`body(w,h)` / `status()` / `cursorPos()`); the wizard composes a `Primary` top
rule, the flow title, the accepted-receipt trail, and the current step's body, and
returns `tea.View{AltScreen: true}` (the only alt-screen mechanism in Bubble Tea
v2). On exit the primary buffer is restored and the receipt trail is reprinted to
the UI channel, ahead of the stable stdout. `Input` / `Select` / `MultiSelect` /
`Confirm` keep their signatures as one-step wizards, so `resume.go` / `archive.go`
need no change; only `start.go` is restructured, its F1 validation closures reused
verbatim. The base-branch selector drops the per-row short SHA; its receipt and the
confirm summary show the short name only.

**Rationale.** Bubble Tea's inline (standard) renderer loses frame height on a
terminal resize and on the `archive` confirm→list `Esc`-back, repainting the header
over stale rows (the base title / `[Local] Remote` bar stacks 5–6×; the
`deSoftWrap` test hack is the same class). A full clear+repaint every frame — which
the alternate buffer gives for free — makes that structurally impossible. Per-step
programs also could not keep earlier receipts on screen during the interview. The
original rejection of the alt buffer (R1, ADR-0020 §11.E — "erases useful history")
is answered by the post-exit reprint.

**Alternatives considered.**
- *Fix the inline renderer's height tracking* — rejected: reimplements part of
  Bubble Tea and stays fragile per new frame transition.
- *Keep inline steps, stack receipts manually* — rejected: does nothing for the
  ghost in the active selector or in `archive`, and cross-program buffer splicing
  stays unpredictable.
- *A single non-alt full-screen form* — rejected: the renderer still strands rows
  when a tall frame (long list) shrinks to a short one (receipt) in the current
  buffer.

**VT emulator.** `tests/integration/vt_test.go` gains alt-screen support: the
DECSET 1049/1047/47 toggle (save/blank/restore the primary grid, no scrollback
capture in alt mode) and VPA (`CSI n d`), which is how Bubble Tea's per-line frame
diff repositions the cursor. A focused unit test (`TestVTAlternateScreen`) pins it.
