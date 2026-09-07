# Contract: the semantic theme and colour capability (F2.5)

**Authority**: `docs/prd.md` RF-60, RF-61; ADD §12.3; spec FR-023–FR-025; SC-003,
SC-008; research R13/R16.

## Token set

One set, defined once in `internal/present/theme`, injected into every renderer. No
renderer defines its own colour; the F1/F2 literals (`huh.ThemeCharm`, `#F780E2`,
ANSI `241`) are removed.

| Token | Dark | Light | Used for | With colour OFF |
|---|---|---|---|---|
| `Primary` | `#11A8CD` | `#087C99` | titles, focused option, active group/tab, brand gradient start | `bold` only |
| `Secondary` | `#8B7CF6` | `#5D4CC9` | key hints, secondary emphasis, brand gradient end | plain |
| `Success` | `#2DBE8C` | `#167A5B` | the `✔` receipt mark | glyph only |
| `Warning` | `#D9A521` | `#8A6D1A` | non-fatal notices | text only |
| `Danger` | `#FF5C7A` | `#B4233F` | the `✘` mark, validation errors, destructive emphasis | glyph / text only |
| `Muted` | adaptive gray | adaptive gray | descriptions, secondary metadata, scroll position line | plain |
| `Text` | terminal foreground | terminal foreground | body text | plain (never recoloured) |

Light variants are contrast-checked against the supported terminals during
implementation; the values above are the starting point (research R13).

## Capability detection

Evaluated **once per process**, from the UI writer (`cmd.ErrOrStderr()`):

| Signal | Source | Rule |
|---|---|---|
| TTY | `golang.org/x/term.IsTerminal` on the UI writer + stdin | not a TTY ⇒ colour OFF, no interactive control opened |
| `NO_COLOR` | env | **non-empty** value ⇒ colour OFF; an **empty** value does **not** disable (spec edge case) |
| `TERM` | env | `dumb` ⇒ colour OFF |
| profile | `github.com/charmbracelet/colorprofile` | `NoColor` / `ANSI16` / `ANSI256` / `TrueColor`; 24-bit tokens and the brand gradient require `ANSI256`+ |
| background | `tea.RequestBackgroundColor` | light ⇒ light variant; unknown ⇒ dark (default) |

## Invariants

- **No colour-only meaning.** Focus is `bold` + a fixed-width textual marker;
  success/failure are `✔`/`✘` glyphs; multi-select state is `[ ]`/`[x]`. All survive
  colour being off (FR-024, SC-003).
- **Body text follows the terminal foreground** — it is never painted an accent
  colour (FR-023).
- **Colour OFF ⇒ zero control sequences.** Non-TTY, `NO_COLOR`, and `TERM=dumb`
  output is plain bytes, understandable, and parseable (FR-025, SC-008).
- `✔` / `✘` fall back to fixed-width ASCII `ok` / `x` only where implementation-phase
  PTY testing shows unstable glyph width on a supported platform; the fallback is
  process-global and padded to equal display width (research R16).
- Behaviour is identical across the supported OS / terminal matrix (FR-032).

## Out of scope

A user-selectable theme, a theme file, and a new `--no-color` flag. Only the
env-based opt-outs above are honoured.
