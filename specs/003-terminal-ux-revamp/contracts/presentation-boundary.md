# Contract: the generic presentation boundary (F2.5)

**Authority**: ADR-0020; ADD §12.2; spec FR-004 (boundary), FR-031; `temp/tui-revamp.md`
§8.3; research R2/R3.

This is an **architectural** contract: the interface `internal/cli` depends on and
the import rule a test enforces.

## Package

`internal/present` (with subpackages `theme`, `brand`, `diagrender`). It owns:
interaction state, key handling, viewport geometry, final receipts, theme
application, brand assembly, the interactive diagnostic renderer, explicit I/O.

It does **not** own: journey sequencing, domain decisions, option meaning, mutations,
Work / repository / branch / archive / projection / git / plugin knowledge.

## Generic API (illustrative shape — not frozen signatures)

```go
type IO struct {
    In io.Reader // cmd.InOrStdin()
    UI io.Writer // cmd.ErrOrStderr()
}

type InputSpec struct {
    Title, Description, Initial string
    Validate func(context.Context, string) error // nil=accept; err=in-frame; present.Fatal(err)=abort
    Receipt  func(accepted string) string
    Secret   bool
}

type Option[T any] struct { Value T; Primary, Secondary, Group string }

type SelectSpec[T any] struct {
    Title, Description string
    Options    []Option[T]
    Grouped, Filterable bool
    Receipt    func(Option[T]) string
}

type MultiSelectSpec[T comparable] struct {
    Title, Description string
    Options    []Option[T]
    Filterable bool
    Confirm    *ConfirmSpec
    Receipt    func([]Option[T]) string
}

type ConfirmSpec struct {
    Title, Impact, Accept, Reject string
    Receipt func(accepted bool) string
}

func Input(ctx context.Context, io IO, spec InputSpec) (string, error)
func Select[T any](ctx context.Context, io IO, spec SelectSpec[T]) (T, error)
func MultiSelect[T comparable](ctx context.Context, io IO, spec MultiSelectSpec[T]) ([]T, error)
func Confirm(ctx context.Context, io IO, spec ConfirmSpec) (bool, error)

func Fatal(err error) error // marks a Validate error as non-recoverable
```

- Cancellation returns `diag.New(diag.Cancelled, "cancelled")` (preserves exit 20).
- Callers gate on an interactive terminal before calling; a non-interactive caller
  never reaches these (`cli-work-home.md`, FR-027).

## Allowed imports for `internal/present/...`

Go stdlib · `charm.land/{bubbletea,lipgloss,huh,bubbles}/v2` ·
`github.com/charmbracelet/colorprofile` · `golang.org/x/term` ·
runewidth / uniseg · **`internal/diag`** (the shared error vocabulary, itself
domain-free).

## Forbidden imports (enforced by test)

Any other `internal/*` package — specifically `internal/work`, `internal/worklist`,
`internal/basebranch`, `internal/archive`, `internal/resume`, `internal/create`,
`internal/projection`, `internal/reconcile`, `internal/gitx`, `internal/config`,
`internal/registry`, `internal/starter`, `internal/convention`,
`internal/shellintegration`, `internal/plugin`, …

## Enforcement test

`tests/contract` runs `go list -deps ./internal/present/...` and fails if the
dependency set contains any `github.com/gustaborges/work/internal/*` other than
`internal/diag`. This test is part of the phase-2 checkpoint and every later CI run
(FR-031, SC covered indirectly via the Integrity gate).

## Migration note

`huh` is permitted **behind** `Input` / `Confirm` during migration. Whether it stays
is decided in implementation phase 6 (research R19) and changes neither this contract
nor any `internal/cli` code.
