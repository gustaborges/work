# Phase 1 Data Model: Terminal UX Revamp (F2.5)

**Feature**: `specs/003-terminal-ux-revamp/` · **Plan**: [plan.md](./plan.md) ·
**Research**: [research.md](./research.md) · **Date**: 2026-09-07

F2.5 changes **no persisted storage**. `work-state.json` stays schema 2, `work.db`
stays `user_version = 2`, config and generated state are untouched, and nothing new
is written to disk. Every entity below is **transient** — it lives for one
interaction step or one process — or is a **declared constant** (theme tokens, help
taxonomy). The entities map one-to-one to the spec's *Key Entities* and are realised
in `internal/present` (domain-free) and `internal/cli` (owns sequencing and meaning).

Authority note: `internal/present` types carry only presentation strings and an
opaque `Value`. The meaning of a `Value`, a `Group`, an id, or a consequence stays
in `internal/cli` (ADR-0020; research R2/R3).

---

## 1. Interaction Step (transient — one input / selection / confirmation)

The generic unit every journey step becomes. One Bubble Tea program, explicit state.

| Field | Type | Notes |
|---|---|---|
| `title` | string | shown while active and echoed in the receipt |
| `description` | string | optional helper text under the title |
| `kind` | enum | `input` \| `select` \| `multiselect` \| `confirm` |
| `state` | enum | `editing` \| `choosing` \| `confirming` \| `completed` \| `cancelled` |
| `width`, `height` | int | last `tea.WindowSizeMsg`; drives the viewport budget (research R5) |
| `currentError` | string | at most one; replaced on the next attempt; cleared on accept |
| `accepted` | value | the returned value once `state == completed` |
| `receipt` | string | compact durable record, set **before** `tea.Quit` (research R4) |

### State transitions

```text
editing/choosing ──invalid──▶ same state, currentError replaced in-frame (one max)
editing/choosing ──accept───▶ completed  → receipt set → tea.Quit
choosing ─────────Enter─────▶ confirming (only when the spec has a ConfirmSpec)
confirming ───────Esc───────▶ choosing (no mutation — present cannot mutate)
confirming ───────accept────▶ completed → receipt set → tea.Quit
any ──────────────Ctrl-C────▶ cancelled → "✘ Operation cancelled" → tea.Quit
editing ──────────q/Esc─────▶ cancelled (input/select; MultiSelect list too)
```

### Validation contract (research R8)

`InputSpec.Validate func(context.Context, string) error`:

| Return | Effect |
|---|---|
| `nil` | accept; render receipt |
| plain `error` / field-recoverable `*diag.Error` | show `✘ <msg>` in-frame, stay editing, replace prior error |
| `present.Fatal(err)` | stop program, return `err` to the CLI diagnostic border |

Closures MUST be deterministic and side-effect-free with respect to Work state
(read-only Starter / git inspection is allowed; any branch/worktree/snapshot/config
write is not). They must be safe to call repeatedly. If a closure can exceed ~120 ms
the frame shows a neutral `⋯ checking…` line (research R9).

---

## 2. Presentation Option (transient — one selectable value)

Built by `internal/cli` from a domain object; the renderer never sees the domain
object.

| Field | Type | Notes |
|---|---|---|
| `Value` | `T` (generic) | opaque to `present`; the CLI maps it back to its domain object |
| `Primary` | string | primary identity line (e.g. `repo  slug`, or a branch name) |
| `Secondary` | string | optional metadata line (e.g. `3 hours ago • feature/x`); truncated **before** `Primary` when width is tight (research R5) |
| `Group` | string | optional; when `SelectSpec.Grouped`, becomes a group/tab; empty groups are never rendered |
| — focus state | (model-internal) | which option is focused; **independent** of selected state; bold + fixed-width marker (research R6) |
| — selected state | (model-internal) | for `MultiSelect`: `[ ]` / `[x]`, fixed 4-cell column, independent of focus |

Ordering is the caller's slice order (the CLI sorts before building options, e.g.
`last_accessed_at DESC, id DESC` for resume — unchanged from F2).

---

## 3. Receipt (transient — the durable record of an accepted step)

| Field | Type | Notes |
|---|---|---|
| `title` | string | the step title |
| `mark` | glyph | `✔` (Success token) or ASCII `ok` fallback (research R16) |
| `displayValue` | string | caller-formatted safe text; for `Secret` steps a redaction (`••••`) or omitted entirely (FR-004) |

Rendered as:

```text
<title>
  ✔ <displayValue>
```

followed by one blank separator line. A selector's receipt replaces its entire
expanded list — no rows, no padding survive (FR-007, SC-002). A confirmation's
receipt is `✔ <title> confirmed` and is written before the command's stable stdout
(FR-030, research R7).

**Cancellation notice** (not a receipt): a single line `✘ Operation cancelled`, no
title echo. Leaves the programmatic `diag.Cancelled` / exit 20 intact (FR-012).

---

## 4. Diagnostic (transient — a field message or a terminal command result)

`internal/diag.Error` gains two optional fields; **categories, tokens, codes, and the
`All` table are unchanged** (research R10).

| Field | Type | Existing? | Notes |
|---|---|---|---|
| `Category` | `Category` | yes | stable `{Token, Code}` — unchanged set |
| `Msg` | string | yes | the current stable human message (non-interactive line) |
| `Summary` | string | **new, optional** | user-vocabulary statement of what failed (interactive) |
| `Hint` | string | **new, optional** | the known next action, shown as `  → <Hint>` |
| `Err` | error | yes | retained internal cause; surfaced only via `WORK_DEBUG` |

Rendering border (`internal/cli/diagnostics_border.go`):

| UI channel | Category | Output |
|---|---|---|
| non-interactive | any | `error: <token>: <Msg>` (one line, unchanged) |
| interactive | `Cancelled` | `✘ Operation cancelled` |
| interactive | other | `✘ <Summary\|Msg>` + optional `  → <Hint>` |

Exactly one layer prints (the border). `present` and orchestrators return, never
print (FR-014).

---

## 5. Theme (declared constant — semantic presentation tokens)

`internal/present/theme`. One set, injected into every renderer. Full values in
[`contracts/theme.md`](./contracts/theme.md); summary:

| Token | Role | Colour-off behaviour |
|---|---|---|
| `Primary` `#11A8CD` / light `#087C99` | titles, focused option, active group, brand start | bold only |
| `Secondary` `#8B7CF6` / light `#5D4CC9` | key hints, brand end | plain |
| `Success` `#2DBE8C` / light `#167A5B` | `✔` mark | glyph only |
| `Warning` `#D9A521` / light `#8A6D1A` | non-fatal notices | text only |
| `Danger` `#FF5C7A` / light `#B4233F` | `✘`, validation, destructive emphasis | glyph/text only |
| `Muted` (adaptive gray) | descriptions, secondary metadata, scroll line | plain |
| `Text` (terminal foreground) | body | plain |

Capability model (`theme/capability.go`, evaluated once from the UI writer):

| Signal | Source | Effect |
|---|---|---|
| `isTTY(UI)` | `golang.org/x/term` | false ⇒ colour off, no interactive controls |
| `NO_COLOR` non-empty | env | colour off (empty value does **not** disable — spec edge case) |
| `TERM == "dumb"` | env | colour off |
| colour `profile` | `colorprofile.Detect` | `NoColor` / `ANSI16` / `ANSI256` / `TrueColor` → gradient & 24-bit tokens gated on `TrueColor`/`ANSI256` |
| background | `tea.RequestBackgroundColor` | selects light vs dark token variant; dark default |

Invariant: no state or focus meaning is ever colour-only — bold, the fixed-width
focus marker, `✔`/`✘`, and `[ ]`/`[x]` all carry meaning without colour (FR-009,
FR-024, SC-003, SC-008).

---

## 6. Command Group (declared constant — a non-empty help category)

`internal/cli/help.go`. Fixed order; a group renders only when it has ≥1 registered
command (FR-021, SC-005).

| `GroupID` | Title | Members as of F2.5 | Future members |
|---|---|---|---|
| `daily` | `Daily Commands` | `start`, `resume`, `archive` | `status` |
| `in-work` | `Inside a Work` | — (hidden: empty) | `status`, `import`, `link` |
| `admin` | `Administration` | — (hidden: empty) | `plugin`, `repository`, `convention` |
| `setup` | `Setup` | `shell-init`, `completion` | — |

Rules:
- Members are exactly the non-hidden commands registered on the root command with
  that `GroupID`. No hand-maintained list (research R15).
- Cobra's built-in `help` / `completion` are assigned a `GroupID` (or hidden) so no
  command is ungrouped.
- An unregistered / not-yet-shipped command cannot appear.
- An empty group is not printed.

---

## 7. Brand (transient — the bare-`work` and help header output)

`internal/present/brand`. A pure function of `(width, colourProfile, colourEnabled)`
over a static embedded wordmark (research R12). Not persisted, not themeable by the
user.

| Form | When | Content |
|---|---|---|
| full gradient | `width ≥ artWidth` and profile ∈ {`TrueColor`,`ANSI256`} and colour enabled | multi-line `WORK` art, per-column `#11A8CD`→`#8B7CF6` gradient, tagline, `work --help` direction |
| full plain | `width ≥ artWidth` and (colour disabled or ≤ANSI16) | same art, no colour (optional bold), tagline, direction |
| compact | `width < artWidth` | bold-or-plain `WORK`, tagline, direction; no wrapping breakage |

`work --help` uses the **compact** form as its header regardless of width. Bare
interactive `work` writes the brand to **stdout** and exits 0 (research R11/R14).

---

## 8. What is explicitly NOT modelled

- No persisted preference, no theme file, no `--no-color` flag (spec Out of Scope).
- No backward-navigation / edit-earlier-step state (spec Out of Scope; research R2).
- No new `diag` category, token, or exit code (research R10).
- No change to `WorkRow`, projection rows, or snapshot fields — the CLI still builds
  those exactly as F2 does and only adapts them into `Option[T]` at the boundary.
