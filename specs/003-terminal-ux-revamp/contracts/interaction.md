# Contract: interactive step lifecycle, geometry, and keys (F2.5)

**Authority**: `docs/prd.md` RF-53–RF-57, RF-64; ADR-0020; ADD §12.2/§12.3; spec
FR-001–FR-013, FR-030, FR-031; SC-001–SC-004; research R4–R9.

Applies to **every** currently implemented public interactive choice: the `work
start` path / slug / prefix / base-branch / workspace / confirmation steps, the
`work resume` recency picker, and the `work archive` multi-select + confirmation +
dirty-worktree acknowledgement.

## 1. Inline operation (FR-001)

- All collection and selection happens in the **current** terminal screen buffer.
- No alternate / full-screen buffer is ever entered.
- Each step is one bounded Bubble Tea program created with explicit
  `WithInput` / `WithOutput` bound to the CLI's injected streams (`diagnostics.md`,
  research R14).

## 2. Step states and final render (FR-003, FR-005, FR-007, SC-002)

```text
editing/choosing ──invalid──▶ same state; the one current error is replaced in-frame
editing/choosing ──accept───▶ completed → compact receipt set → tea.Quit
choosing ─────────Enter─────▶ confirming        (only when a confirmation is defined)
confirming ───────Esc───────▶ choosing          (no mutation)
confirming ───────accept────▶ completed → receipt → tea.Quit
any ──────────────Ctrl-C────▶ cancelled → "✘ Operation cancelled" → tea.Quit
input/select ─────q or Esc──▶ cancelled
```

- The `completed` / `cancelled` view is set **before** `tea.Quit`, so Bubble Tea's
  graceful final render commits only the compact frame.
- **Receipt** (accepted step):

  ```text
  <title>
    ✔ <displayValue>
  ```

  then one blank separator line. A selector's receipt **replaces its whole expanded
  list** — no rows, no blank padding remain in history (FR-007, SC-002).
- A journey may supply a redacted `displayValue` or omit it for a value it marks
  sensitive (FR-004).
- **Cancellation notice**: the single line `✘ Operation cancelled`. No title echo, no
  picker frame retained (FR-012).
- A confirmation's accepted receipt is `✔ <title> confirmed`, written **before** the
  command's stable stdout result (FR-030, research R7).

## 3. Recoverable errors (FR-005, FR-006, SC-001)

- A recoverable failure attributable to the active field appears **inside that
  field's live frame** as `✘ <message>`.
- **At most one** current error is shown; the next edit / retry replaces it.
- Rejected values and obsolete error messages **never** appear in terminal history
  after the step is accepted.
- The message comes from the CLI-supplied `Validate` closure (a plain error or a
  field-recoverable `*diag.Error`). `present.Fatal(err)` ends the step and hands
  `err` to the diagnostic border instead (research R8).
- No `fmt.Fprintln(errOut, diag.Format(...))` is emitted from inside a flow for a
  recoverable field error (removes the F1 behaviour in `start.go`).
- If the closure may take > ~120 ms, the frame shows a neutral `⋯ checking…` line
  (not an error) until it returns (FR-011, research R9).

## 4. Bounded frame (FR-002, FR-006, SC-004)

- Every active model tracks `width,height` from `tea.WindowSizeMsg`.
- The visible list height = `height − reserved`, where `reserved` counts title,
  optional group bar, scroll line, filter line, error line, help line, and any
  confirmation block.
- **No frame is ever emitted taller than `height`** (content that scrolls into
  scrollback cannot be reclaimed).
- Content wider than `width` is truncated with `…` using **display width**, never
  byte length. **Secondary metadata is truncated before primary identity**; primary
  is truncated before being dropped (FR-006).
- Verified at 40×10, 80×24, 160×50 with long and wide-Unicode content (SC-004).
- The base-branch selector MAY keep a generous scrolling viewport while active
  (FR-007) but still obeys the height bound and collapses to its receipt on accept.

## 5. Selector geometry (FR-008, FR-009, SC-003)

Across cursor movement, selection toggles, filtering, and group/tab changes,
**unchanged rows show zero change** in rendered line count and zero change in the
starting columns of: the focus marker, the checkbox, the primary text, the secondary
text.

- Fixed 2-cell leading slot: `❯ ` focused, `  ` otherwise (equal display width;
  textual, not colour).
- Focused row: **bold** (both lines of a two-line row), MAY also take `Primary`.
  Bold identifies focus with colour off.
- Fixed 4-cell checkbox slot: `[ ] ` / `[x] `. Checked state and focus state are
  independent.
- A two-line option moves and styles as one unit; the second line's indent equals
  the first line's content column.
- No interaction adds or removes a printable-width prefix.

## 6. Keys (FR-010)

| Key | Action | Where |
|---|---|---|
| `↑` / `k`, `↓` / `j` | move focus | all selectors |
| `Enter` | accept selection / continue / confirm | all |
| `Space` | toggle checkbox | multi-select |
| `/` | start filtering | selectors that declare `Filterable` |
| `Esc` | clear filter, or step back from a confirmation | all |
| `q` / `Esc` | cancel the step | input & single/multi-select list |
| `Ctrl-C` | cancel from anywhere, including during a confirmation or validation | all (FR-011) |
| `←` / `→` / `Tab` | switch group/tab | grouped selectors |

The active cancellation key is always shown in the on-screen help line.

## 7. Ctrl-C and transactions (FR-011, FR-013)

- `Ctrl-C` stays responsive throughout collection and before/during a mutation,
  per the existing transaction contract (F1 S8, F2 archive transactionality —
  unchanged).
- Cancellation **before** a mutation or confirmation changes no Work state, access
  time, files, branches, worktrees, configuration, or plugin state (FR-013, SC-010).

## 8. Uniformity (FR-031)

All the controls listed at the top of this file share this lifecycle, these keys,
this focus semantics, these terminal bounds, this cancellation behaviour, and the
one semantic theme (`theme.md`).
