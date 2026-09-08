# Contract: the `WORK` brand / wordmark (F2.5)

**Authority**: `docs/prd.md` RF-59; ADR-0019; ADD §12.1/§12.3; spec FR-018, FR-019,
FR-025; SC-009; research R12.

## Inputs

A pure function of `(width int, profile colorprofile.Profile, colorEnabled bool)`.
`width` is the terminal columns (or 80 when unknown / non-TTY). No I/O, no state, no
external generator.

## Forms

| Form | Condition | Content |
|---|---|---|
| **full gradient** | `width ≥ artWidth` **and** `colorEnabled` **and** `profile ∈ {TrueColor, ANSI256}` | the multi-line block `WORK` terminal art; each terminal **column** of the art is coloured by linear interpolation from `#11A8CD` (primary) at the left edge to `#8B7CF6` (secondary) at the right edge, so the gradient sweeps through all four glyphs; then a blank line, the tagline, a blank line, and `Run 'work --help' to get started.` |
| **full plain** | `width ≥ artWidth` **and** (not `colorEnabled` **or** `profile ∈ {NoColor, ANSI16}`) | the same art with **no colour** (an optional single `bold`); tagline; direction |
| **compact** | `width < artWidth` | the single word `WORK` (optional `bold`), then the tagline and the direction on their own lines; **no wrapping breakage**, fits in `width` |

- `artWidth` is the fixed display width of the embedded art (measured with
  display-width helpers). The shipped glyph design is recorded in
  [§ Shipped wordmark](#shipped-wordmark) below; it is the single `const
  wordmarkArt` in `internal/present/brand/wordmark.go`.
- Tagline (confirmed in visual review): `Isolated work, ready when you are.`
  followed by `Run 'work --help' to get started.`
- The gradient is **never** required to read the word: the plain and compact forms
  must be equally legible (SC-009 asks ≥ 90 % of reviewers to read `WORK` and rate
  both gradient and fallback readable).

## Where it is used

| Caller | Form | Channel |
|---|---|---|
| bare interactive `work` | responsive (full or compact by `width`) | **stdout**, exit 0 |
| `work --help` header | always **compact** | stdout, exit 0 |

## Colour-off guarantee (FR-025, SC-008)

When `colorEnabled` is false (non-TTY output, `NO_COLOR` non-empty, or `TERM=dumb`)
the returned string contains **no** ANSI/OSC control sequences at all — not even
bold. Verified by a golden test and a `! match \x1b` assertion in `quickstart.md`
Q7/Q8.

## Non-goals

- No animation, no per-frame redraw (it is static text).
- No `figlet` / `go-figure` / runtime font dependency.
- No user-configurable wordmark or colours.

## Shipped wordmark

A hand-placed block-glyph asset (Unicode box-drawing / block elements), six rows,
`artWidth = 34`, `artHeight = 6`. Every row is right-padded to `artWidth` at init
so the block is rectangular and the per-column gradient lines up across all four
glyphs.

```
██╗    ██╗ ██████╗ ██████╗ ██╗  ██╗
██║    ██║██╔═══██╗██╔══██╗██║ ██╔╝
██║ █╗ ██║██║   ██║██████╔╝█████╔╝
██║███╗██║██║   ██║██╔══██╗██╔═██╗
╚███╔███╔╝╚██████╔╝██║  ██║██║  ██╗
 ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝
```

Full interactive form (gradient `#11A8CD`→`#8B7CF6` swept left-to-right by column,
TrueColor):

```
<WORK art>

Isolated work, ready when you are.

Run 'work --help' to get started.
```

Golden strings for the wide-TrueColor, wide-colour-off, and compact forms are
pinned in `internal/present/brand/brand_test.go`; a terminal screenshot lives with
the F2.5 release notes.
