---
description: "Task list for Terminal UX Revamp (F2.5) implementation"
---

# Tasks: Terminal UX Revamp (F2.5)

**Input**: Design documents from `specs/003-terminal-ux-revamp/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Model-level, golden, PTY-integration, and non-interactive contract tests **are required** — `docs/roadmap.md` §4 makes them a cross-cutting exit gate for every slice, and the spec's Success Criteria (SC-001..SC-010) are stated as test outcomes. Unit/model tests are folded into each implementation task ("with model tests"); golden / PTY / `.txtar` suites are their own tasks.

**Organization**: Tasks grouped by user story. Module `github.com/gustaborges/work`, Go 1.26, single binary, system `git` as a subprocess. F2.5 adds **no new dependencies** and **no persisted storage change**.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: `[US1]`..`[US5]` for user-story phases only

## Path Conventions

Single Go project at repo root: `cmd/work/`, `internal/<pkg>/`, `tests/`. Paths below are literal. New F2.5 tree: `internal/present/` (+ `theme/`, `brand/`, `diagrender/`); changed: `internal/cli/{root,start,resume,archive}.go`, new `internal/cli/{help,diagnostics_border}.go`, `internal/diag/diag.go`; shrinking-then-removed: `internal/tui/`.

## Phase order vs. spec priority

The spec priorities are P1 (US1) > P2 (US2) > P3 (US3) > P4 (US4) > P5 (US5). The **phase** order below is US1 → US3+US4 → US2 → US5 because the interaction lifecycle and theme (US1/US3) are prerequisites for a coherent brand (US2), the selector and diagnostic work (US3/US4) share the same picker migration, and the non-interactive sweep (US5) is a verification pass that runs last. Every story is still **independently testable** at its checkpoint, and US1 alone is the MVP.

## Branching (stacked PRs, one branch per phase)

**The PRs are stacked.** Phase 1 branches off the **current branch**
(`feature/003-terminal-ux-revamp-p0-specs`) and its PR targets that branch; every later phase
branches off the previous phase's branch and its PR targets that previous branch. Nothing
targets `develop` directly — the whole stack lands on `feature/003-terminal-ux-revamp-p0-specs`,
which is then merged onward (its own PR; the user's call on cadence). Release `0.3` is cut from
`develop` after the stack lands.

**The implementing agent creates and switches to the phase branch BEFORE its first task.**
First action of every phase:

```
git switch <base> && git pull && git switch -c <feature-branch>
```

> **Confirm before you cut.** `/speckit-implement` MUST confirm the base branch and PR target
> with the user at the start of each phase — do not assume the table below. If an earlier
> phase's PR has already merged into `feature/003-terminal-ux-revamp-p0-specs`, a later phase
> may be rebased to branch straight off it.

| Phase | Feature branch | Branch off | PR targets | Planned PR title |
|---|---|---|---|---|
| 1 — Setup & PTY proof | `feature/003-terminal-ux-revamp-p1-setup` | `feature/003-terminal-ux-revamp-p0-specs` (current) | `feature/003-terminal-ux-revamp-p0-specs` | `chore: F2.5 Phase 1 (Setup) — present skeleton, capability probe, PTY proof (spec-003)` |
| 2 — Foundational | `feature/003-terminal-ux-revamp-p2-foundational` | `feature/003-terminal-ux-revamp-p1-setup` | `feature/003-terminal-ux-revamp-p1-setup` | `feat: F2.5 Phase 2 — Foundational: present primitives, theme, geometry, receipts, import-boundary test (spec-003)` |
| 3 — US1 Inline lifecycle for `work start` 🎯 MVP | `feature/003-terminal-ux-revamp-p3-us1-start` | `feature/003-terminal-ux-revamp-p2-foundational` | `feature/003-terminal-ux-revamp-p2-foundational` | `feat: US1 — inline step lifecycle with compact receipts for work start (F2.5 Phase 3)` |
| 4 — US3 stable selectors + US4 diagnostics | `feature/003-terminal-ux-revamp-p4-us3-us4` | `feature/003-terminal-ux-revamp-p3-us1-start` | `feature/003-terminal-ux-revamp-p3-us1-start` | `feat: US3 + US4 — stable selectors and a single diagnostic border (F2.5 Phase 4)` |
| 5 — US2 brand home & grouped help | `feature/003-terminal-ux-revamp-p5-us2-brand` | `feature/003-terminal-ux-revamp-p4-us3-us4` | `feature/003-terminal-ux-revamp-p4-us3-us4` | `feat: US2 — branded static home and grouped help (F2.5 Phase 5)` |
| 6 — US5 non-interactive sweep & polish | `feature/003-terminal-ux-revamp-p6-polish` | `feature/003-terminal-ux-revamp-p5-us2-brand` | `feature/003-terminal-ux-revamp-p5-us2-brand` | `feat: F2.5 Phase 6 — non-interactive compatibility sweep, remove internal/tui, huh decision (spec-003)` |

**Merge gate** (every phase): the phase **Checkpoint** is met and `make lint` + `go test ./...`
are green on `ubuntu-latest` / `macos-latest` / `windows-latest`, with **F1 quickstart S1–S12
and F2 quickstart S1–S13 still green unchanged** (SC-007), plus every earlier F2.5 phase's
suite. When a phase PR merges into `feature/003-terminal-ux-revamp-p0-specs`, rebase the rest of
the stack onto the new tip.

Spec/plan/contract/doc edits stay on `feature/003-terminal-ux-revamp-p0-specs`; the per-phase
rule covers implementation code only. **A picker file in `internal/tui/` is deleted in the same
phase that moves its `internal/cli` caller to `internal/present`**, so `go test ./...` never
compiles two implementations of one control.

---

## Phase 1: Setup & PTY proof (Shared Infrastructure)

**Purpose**: The `internal/present` package skeleton, the colour/TTY capability probe, and a
throwaway PTY proof that the final-frame receipt behaviour actually works on real terminals
**before** any broad refactor (research R18 step 1). No production behaviour change.

**Branch**: `git switch feature/003-terminal-ux-revamp-p0-specs && git pull && git switch -c feature/003-terminal-ux-revamp-p1-setup` before T001. PR targets `feature/003-terminal-ux-revamp-p0-specs`.

- [X] T001 Create the `internal/present` tree with a `doc.go` in each package carrying the one-paragraph purpose from `plan.md` → Project Structure and `contracts/presentation-boundary.md`: `internal/present/doc.go`, `internal/present/theme/doc.go`, `internal/present/brand/doc.go`, `internal/present/diagrender/doc.go`. Empty packages that compile; `go build ./...` stays green.
- [X] T002 [P] Implement `internal/present/theme/capability.go` per `contracts/theme.md` → *Capability detection* and research R13: a `Capability` struct + `Detect(ui io.Writer, in io.Reader) Capability` computing `ColorEnabled` (`isTTY(ui) && os.Getenv("NO_COLOR")=="" && os.Getenv("TERM")!="dumb"` — **empty `NO_COLOR` does not disable**), `Profile` (`colorprofile.Detect`), `Interactive` (`isTTY(ui) && isTTY(in)`), and an `AsciiMarks` flag (default false). Move `internal/tui/tty.go`'s `IsInteractive`/`MustInteractive` here (or into a tiny `internal/present` helper) and re-point `internal/tui` + `internal/cli` imports. Table tests with fake TTY/non-TTY writers and each env combination, including `NO_COLOR=` empty and `NO_COLOR=0`.
- [X] T003 [P] Add a PTY proof test `tests/integration/present_proof_test.go` (`//go:build unix`, reuse the `console` harness + `ansiRe` from `interactive_test.go`): a tiny throwaway `main`-style model (in the test package) that (a) shows an input with an in-frame `✘` that is replaced on the next keystroke, and (b) on Enter final-renders `Title` / `  ✔ <value>` and quits. Assert the post-quit scrollback contains the receipt and **not** the rejected value, at 80×24. This test is deleted in Phase 2 once the real `present.Input` exists — it only de-risks the approach.
- [X] T004 [P] Confirm in the PR description that `go.mod`, `Makefile`, and `.github/workflows/ci.yml` need **no** change for F2.5 (no new deps; `colorprofile`, `runewidth`, `uniseg`, `creack/pty` are already present) and that the CI matrix is unchanged.

**Checkpoint**: `go build ./...` + `go vet ./...` green with the new empty packages; the capability probe has green tests on all 3 OSes; the PTY proof passes on Linux and macOS (documented in the PR).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The generic `present` primitives, the semantic theme, the viewport-geometry
helper, the shared receipt/notice renderer, the `diag` field additions, and the
import-boundary test. No `internal/cli` command is migrated yet.

**Branch**: `git switch feature/003-terminal-ux-revamp-p1-setup && git pull && git switch -c feature/003-terminal-ux-revamp-p2-foundational` before T005. PR targets `feature/003-terminal-ux-revamp-p1-setup`.

**⚠️ CRITICAL**: No user-story work starts until this phase is done.

- [X] T005 [P] Implement `internal/present/theme/theme.go` per `contracts/theme.md` → *Token set*: a `Theme` with `Primary/Secondary/Success/Warning/Danger/Muted/Text` as `lipgloss` styles, dark and light variants with the exact hex values, built from a `Capability` (T002) + a background signal; `ColorEnabled==false` degrades every style to plain but keeps `Bold`. Constructor `New(cap Capability, dark bool) Theme`. Golden-string tests: each token in dark true-colour, light true-colour, ANSI16, and colour-off (asserting **zero** `\x1b` bytes when off).
- [X] T006 [P] Implement `internal/present/geometry.go` per `contracts/interaction.md` §4 and research R5: `Budget{Width, Height, Reserved}` → `VisibleRows() int` (never < 1); `TruncTail(s string, max int) string` and `TruncMiddle` using **display width** (`runewidth`/`uniseg`), never byte length; a `Row` helper that lays out `[marker][checkbox][primary]` with fixed-width slots and truncates `secondary` before `primary`. Table tests with CJK, combining marks, and ANSI-styled input; assert equal display width for `[ ] `/`[x] ` and for `❯ `/`  `.
- [X] T007 [P] Implement `internal/present/receipt.go` per `data-model.md` §3 + `contracts/interaction.md` §2: `Receipt(theme, title, mark, displayValue) string` → `"<title>\n  ✔ <displayValue>\n"`; `CancelNotice(theme) string` → `"✘ Operation cancelled"`; `ConfirmReceipt(theme, title) string` → `"✔ <title> confirmed"`; ASCII fallback (`ok`/`x`) when `cap.AsciiMarks`. Golden tests for colour-on and colour-off.
- [X] T008 Implement `internal/present/present.go`: the `IO{In, UI}` type; `Fatal(err) error` + `IsFatal(err)` marker; a shared `run(ctx, io, model) (tea.Model, error)` that creates the `tea.Program` with `tea.WithInput(io.In)`, `tea.WithOutput(io.UI)`, `tea.WithContext(ctx)`, maps `context.Canceled` and the model's `cancelled` state to `diag.New(diag.Cancelled, "cancelled")`, and requires the model to expose its final `View()` string. This is the **only** `internal/diag` dependency in `present`. Unit tests with a stub model for the cancel mapping and the fatal-unwrap.
- [X] T009 [US-none] Implement `internal/present/input.go` — `Input(ctx, io, InputSpec) (string, error)` per `contracts/presentation-boundary.md` + `contracts/interaction.md` §2–3 + research R4/R8/R9: a Bubble Tea model with `editing`/`completed`/`cancelled` states; runs `Validate` in a `tea.Cmd`, shows `⋯ checking…` after ~120 ms, shows `✘ <msg>` in-frame for a returned error (one error max, replaced on edit), aborts on `Fatal`; on accept sets the receipt (`Secret` ⇒ `••••` unless `Receipt` given) **before** `tea.Quit`; `q`/`Esc`/`Ctrl-C` → cancelled. `huh` MAY back the text editing internally (research R19). Model tests: one-error invariant, error replaced on retry, fatal aborts, secret redaction, accepted final `View()` is the exact receipt, cancelled final `View()` is the exact notice, `WindowSizeMsg` keeps the frame ≤ height.
- [X] T010 [P] Implement `internal/present/select.go` — `Select[T any](ctx, io, SelectSpec[T]) (T, error)` per the same contracts + research R6: single-select model over `[]Option[T]`; optional group/tab bar from `Option.Group` (empty groups never rendered); `/` filter when `Filterable`; fixed 2-cell focus slot (`❯ `/`  `), bold focused row (both lines), `Secondary` line truncated before `Primary` (via T006); viewport from `WindowSizeMsg` (via T006); collapses to `Receipt(Option)` on Enter, `CancelNotice` on `q`/`Esc`/`Ctrl-C`. Model tests for SC-003 invariants (line count + column starts unchanged across move/filter/tab), bounded frame, and the exact final views.
- [X] T011 [P] Implement `internal/present/multiselect.go` — `MultiSelect[T comparable](ctx, io, MultiSelectSpec[T]) ([]T, error)`: like `Select` plus a fixed 4-cell `[ ] `/`[x] ` slot **independent of focus**; `Space` toggles; `Enter` with nothing checked is a no-op; when `Confirm != nil`, `Enter` → a `confirming` sub-state showing `ConfirmSpec.Impact`, `Esc` back to `choosing`, accept → `ConfirmReceipt` then quit; `Ctrl-C` anywhere → cancelled. Model tests: checkbox/focus independence, empty-Enter no-op, `choosing→confirming→completed`, `Esc` from confirming does not mutate, SC-003 invariants, this is the **regression test for the F2 `archive_picker.go` double-render defect** (assert every unselected row has identical rendered height and column starts before/after a focus move).
- [X] T012 [P] Implement `internal/present/confirm.go` — `Confirm(ctx, io, ConfirmSpec) (bool, error)`: `choosing` shows `Impact` + `Accept`/`Reject`; accept → `ConfirmReceipt(Title)` before quit (research R7); reject → returns `(false, nil)` with a compact `✘ <Reject>` line; `Ctrl-C` → cancelled error. Model tests for all three exits and the final views.
- [X] T013 Extend `internal/diag/diag.go` per `contracts/diagnostics.md` → *`internal/diag` changes* and research R10: add optional `Summary string` and `Hint string` fields to `Error`, a `Cause() error` method (alias of `Unwrap`), and constructor variants (`WithHint`, or option funcs) — **no new `Category`, token, or exit code**. `Format`, `ExitCode`, `Token`, and the `All` table are untouched. Extend `diag_test.go` only to cover the new fields' zero-value behaviour (a `Summary`/`Hint`-less `Error` formats exactly as today); `TestCategoryTable` is unchanged.
- [X] T014 [P] Add the import-boundary test `tests/contract/present_boundary_test.go` per `contracts/presentation-boundary.md` → *Enforcement test*: run `go list -deps ./internal/present/...`, fail if any `github.com/gustaborges/work/internal/*` other than `internal/diag` appears. Also assert `internal/diag` itself still has zero `internal/*` imports.
- [X] T015 [P] Fix the F2 `internal/tui/archive_picker.go` double-render defect in place (the unselected branch appends the first row line twice — research R6 / `temp/tui-revamp.md` §3.4) and add an `archive_picker_test.go` assertion that unselected rows keep identical rendered height and column starts across a focus move. This lands the regression guard on the **live** picker before Phase 4 rewrites it, so the bug cannot silently return.

**Checkpoint**: `internal/present/...` has green model + golden tests on all 3 OSes; the import-boundary test passes; `go test ./...` green (F1 + F2 suites untouched); the live archive picker no longer jumps.

---

## Phase 3: User Story 1 — Inline steps without scrollback debris (Priority: P1) 🎯 MVP

**Branch**: `git switch feature/003-terminal-ux-revamp-p2-foundational && git pull && git switch -c feature/003-terminal-ux-revamp-p3-us1-start` before T016. PR targets `feature/003-terminal-ux-revamp-p2-foundational`.

**Goal**: Every `work start` step (path, prefix, slug, base branch, workspace root, confirmation) runs as a bounded inline `present` primitive: recoverable domain failures appear **inside** the active field and are replaced on retry, and each accepted step collapses to a one-line receipt. Rejected attempts and stale errors leave no terminal history.

**Independent Test**: quickstart Q1–Q4 — an interactive `work start` with four bad paths then a valid one, a bad slug then a good one, ending with only the accepted receipts in scrollback; the base selector large while active and compact after Enter; the confirmation collapsing to `✔ Create Work confirmed` before the stable `work: created …` lines. F1 S1–S12 stay green (only presentation changes).

### Tests for User Story 1

- [X] T016 [P] [US1] PTY integration test `tests/integration/start_receipts_test.go` (`//go:build unix`, `console` harness, 80×24): `work start` with no args; feed `/no/such/a`,`/no/such/b`,`/tmp`,`/no/such/c` then `$src`; feed `bad slug`, a colliding slug, then `my-work`; accept prefix + base + workspace + confirm. Assert: each `✘` replaces the previous (never two visible); post-run scrollback has one `✔` receipt per step and **none** of the rejected inputs/errors; stdout still has the exact F1 `work: created` / `work: branch` / `work: path` lines. (US1 #1, #2; SC-001, SC-002.)
- [X] T017 [P] [US1] PTY test `tests/integration/start_selector_collapse_test.go` (80×24 and 160×50): the base-branch selector renders a multi-row scrolling list while active; after `Enter` the very next bytes are `Base branch` / `  ✔ <ref>` + one blank line — no list rows, no padding; at 160×50 no extra blank rows survive. The confirmation block appears immediately after the branch receipt (no picker padding between). (US1 #2; SC-002, SC-004.)
- [X] T018 [P] [US1] Model test `internal/present/input_secret_test.go`: `InputSpec{Secret:true}` completed `View()` contains `••••` and no substring of the typed value. (US1 #3; FR-004.)

### Implementation for User Story 1

- [X] T019 [US1] Rewrite the path step in `internal/cli/start.go`: replace the `for { … tui.InputPath() … fmt.Fprintln(errOut, diag.Format(ierr)) … }` loop with a single `present.Input` call whose `Validate` closure runs `starter.Invoke` (read-only) + `reporef.ValidatePath`, returns the `*diag.Error` for `InvalidPath`/`UnusableRepo` (shown in-frame) and `present.Fatal` for anything else; `Receipt` returns the normalized path. Build `present.IO` from `cmd.InOrStdin()` / `cmd.ErrOrStderr()`. Non-interactive path unchanged (still the `diag.Usage` "no SOURCE" error). (FR-005, FR-014; research R8/R14.)
- [X] T020 [US1] Rewrite the slug step in `internal/cli/start.go`: one `present.Input` whose `Validate` runs `tui.ValidateSlug` (move it to `internal/branchname` or keep as a plain func) then `catalog.DeriveName` + `branchname.Validate` + `branchname.DetectCollision`, returning the `*diag.Error` for `InvalidBranchName`/`BranchCollision` in-frame; `Receipt` shows the slug. Remove the slug retry loop. (FR-005, S5, S6.)
- [X] T021 [US1] Rewrite the prefix step: `present.Select[string]` over `prefixes` (skip entirely when `len==1`, unchanged); `Receipt` shows the chosen prefix. Delete `tui.SelectPrefix`.
- [X] T022 [US1] Rewrite the base-branch step: build `[]present.Option[basebranch.Choice]` in `selectBase` with `Primary` = `c.Short`, `Secondary` = `c.ObjectShort` (+ scope), `Group` = `"Remote"`/`"Local"`; call `present.Select` with `Grouped:true, Filterable:true`; `Receipt` = `base.Format()`. Delete `internal/tui/basebranch_picker.go` + `basebranch_picker_test.go` and `tui.BaseBranchItem`/`SelectBaseBranch`. Port the tab-switch and two-tab-hidden behaviour into `select.go` if not already there (Phase 2 T010). (FR-007, FR-008.)
- [X] T023 [US1] Rewrite the workspace-root step in `resolveWorkspace`: `present.Input` pre-filled with `workspace.SuggestDefault()`, `Validate` runs `workspace.Validate`, `Receipt` shows the absolute path. Delete `tui.EditWorkspaceRoot`.
- [X] T024 [US1] Rewrite the confirmation step: `present.Confirm{Title:"Create Work", Impact: <the current summary block>, Accept:"Create", Reject:"Cancel"}`; on accept the frame collapses to `✔ Create Work confirmed` (via `present`), then `create.Run` proceeds and the stable stdout lines print unchanged. `--yes` still skips it. Delete `tui.ConfirmCreate`. (FR-030; research R7.)
- [~] T025 [US1] `prompts_test.go` deleted; `tui.InputPath`/`SelectPrefix`/`InputSlug`/`EditWorkspaceRoot`/`ConfirmCreate` removed from `prompts.go`; `ValidateSlug` moved to `internal/branchname` (`slug.go`, now returning `diag.InvalidBranchName`). `prompts.go` is retained only for `ConfirmArchive`/`AckDirtyWork`, which `internal/cli/archive.go` still imports — it is deleted in Phase 4 (T031) when archive migrates. `go build ./...` green.
- [X] T026 [US1] Update `internal/cli/start_test.go` and any F1 `.txtar` under `tests/integration` that asserts the **old** prompt/error text (e.g. a rejected-path line on stderr) to the new receipt/in-frame behaviour — **without** changing any stdout, token, or exit-code assertion. Confirm `invalid_path.txtar` and `invalid_slug.txtar` still assert the same exit codes and the same `work: created` success lines on the eventual success path.

**Checkpoint**: `work start` runs entirely through `internal/present`; `internal/tui/{prompts,basebranch_picker}.go` are gone; quickstart Q1–Q4 pass on Linux + macOS; F1 S1–S12 and F2 S1–S13 green.

---

## Phase 4: User Story 3 — Stable selectors + User Story 4 — Concise cancellation & errors (Priority: P3 / P4)

**Branch**: `git switch feature/003-terminal-ux-revamp-p3-us1-start && git pull && git switch -c feature/003-terminal-ux-revamp-p4-us3-us4` before T027. PR targets `feature/003-terminal-ux-revamp-p3-us1-start`.

**Goal (US3)**: The `work resume` and `work archive` selectors adopt the Phase-2 primitives — stable geometry, bold+marker focus, fixed checkbox columns, viewport bounds, compact final views.
**Goal (US4)**: One human diagnostic is rendered exactly once at the CLI/process border; interactive cancellation shows `✘ Operation cancelled` once and keeps exit 20; lower layers never print.

**Independent Test**: quickstart Q5, Q6 (geometry, at 40×10/80×24/160×50, colour forced off), Q11, Q12 — cancel resume/archive from list and confirmation → one line, exit 20, zero mutation, `verifycoherent` still passes; a non-recoverable failure → one `✘` line + optional hint, no wrapped chain, and the non-interactive form still `error: <token>: <message>`.

### Tests for User Story 3 / 4

- [ ] T027 [P] [US3] PTY test `tests/integration/selector_geometry_test.go` (40×10, 80×24, 160×50; a 12-Work fixture with long branches and wide-Unicode slugs): in the resume picker and the archive multi-select, move focus across every row, toggle checkboxes, filter and clear — assert **zero** change in rendered line count and in the starting columns of marker / checkbox / primary / secondary for unchanged rows; assert no frame exceeds the viewport and secondary truncates before primary. Run once with `NO_COLOR=1` and assert the focused row is still identifiable (bold + `❯`). (US3 #1–#3; SC-003, SC-004.)
- [ ] T028 [P] [US4] PTY test `tests/integration/cancellation_test.go`: `work resume` then `q` (repeat `Esc`, `Ctrl-C`); `work archive` → check rows → confirmation → `Ctrl-C`. Each: terminal shows exactly `✘ Operation cancelled`, exit **20**, every snapshot's `last_accessed_at` unchanged, `verifycoherent` passes, no worktree/dir/branch/config change. (US4 #1; FR-012, FR-013; SC-010.)
- [ ] T029 [P] [US4] `.txtar` + PTY test `tests/integration/diag_border_test.go`: a non-recoverable failure interactively → one `✘ <summary>` (+ `  → <hint>` when set), no `error:`-prefixed line, no cause chain; `WORK_DEBUG=1` → same line plus the chain; the same failure non-interactively → `error: <token>: <message>` byte-identical to F2 and the same exit code. (US4 #2, #3; FR-014–FR-016; SC-007.)

### Implementation for User Story 3 / 4

- [ ] T030 [US3] Rewrite `internal/cli/resume.go` to build `[]present.Option[worklist.WorkRow]` (`Primary` = `row.DisplayName`, `Secondary` = `row.RelativeTime + " • " + row.Branch`) and call `present.Select[worklist.WorkRow]{Filterable:true}`; map the chosen `Value.ID` back to the resume orchestrator. Delete `internal/tui/resume_picker.go` + `resume_picker_test.go` and `tui.SelectResume`. Behaviour (ordering, id resolution, access bump, non-interactive gating, exit codes 21/22) is **unchanged** — only the rendering path changes.
- [ ] T031 [US3] Rewrite `internal/cli/archive.go` to build `[]present.Option[worklist.WorkRow]` and call `present.MultiSelect[worklist.WorkRow]` with `Confirm: &present.ConfirmSpec{Title:"Archive Works", Impact:<the current consequences block>, Accept:"Archive", Reject:"Cancel"}`; the dirty-worktree acknowledgement becomes a per-Work `present.Confirm` (`Title` = the Work, `Impact` = the uncommitted-changes warning, `Accept:"Archive anyway"`, `Reject:"Keep active"`). Delete `internal/tui/archive_picker.go` + `archive_picker_test.go` and `tui.SelectArchive`/`ConfirmArchive`/`AckDirtyWork`/`archivedDir`. Batch transactionality, `--yes`, `--force-dirty`, exit codes 23/24, and the `WORK_CD_FILE` write when archiving the cwd's Work are **unchanged**.
- [ ] T032 [US4] Implement `internal/present/diagrender/diagrender.go` per `contracts/diagnostics.md` → *Renderer selection*: `Human(theme, err) string` → `✘ <Summary|Msg>` + optional `  → <Hint>`; `Cancel(theme) string` → `✘ Operation cancelled`; `Unexpected(theme) string` for a non-`diag` error. `✘` uses the `Danger` token; ASCII fallback honoured. Golden tests colour-on/off.
- [ ] T033 [US4] Implement `internal/cli/diagnostics_border.go` and wire it into `Execute` (`internal/cli/root.go`): at the process border, pick the renderer by `tui.IsInteractive()`/capability — non-interactive ⇒ `diag.Format` (**unchanged**); interactive + `diag.Cancelled` ⇒ `diagrender.Cancel`; interactive + other `*diag.Error` ⇒ `diagrender.Human`; interactive + non-`diag` ⇒ `diagrender.Unexpected`; append the cause chain when `WORK_DEBUG` is non-empty. Exit code still `diag.ExitCode(err)`. Remove the bare `fmt.Fprintln(os.Stderr, diag.Format(err))` in `Execute` in favour of this.
- [ ] T034 [US4] Grep the tree for any remaining lower-layer printing of diagnostics (`fmt.Fprintln(.*diag\.Format`, `os.Stderr` writes in `internal/{create,resume,archive,reconcile,bootstrap}`) and convert each to a returned structured error carrying `Summary`/`Hint` where a next action is known (e.g. dirty-worktree → hint `run with --force-dirty to archive it anyway`; target-archived → hint `run \`work status <id>\` to inspect it`). Model/unit tests assert the border renders them once.
- [ ] T035 [US3] Update any F2 `.txtar` that asserted the old picker help text or the `error: cancelled: cancelled` double-wrap (`resume_errors.txtar`, `archive_*.txtar`) to the new one-line interactive form — keeping every stdout / token / exit-code assertion. Non-interactive `.txtar` assertions are unchanged.

**Checkpoint**: `internal/tui/` contains only `home.go`/`home_test.go`; resume + archive run through `internal/present`; quickstart Q5, Q6, Q11, Q12 pass; F1 + F2 suites green; the diagnostic border is the only place a failure prints.

---

## Phase 5: User Story 2 — Branded landing and complete help (Priority: P2)

**Branch**: `git switch feature/003-terminal-ux-revamp-p4-us3-us4 && git pull && git switch -c feature/003-terminal-ux-revamp-p5-us2-brand` before T036. PR targets `feature/003-terminal-ux-revamp-p4-us3-us4`.

**Goal**: Interactive `work` with no arguments prints the `WORK` terminal-art wordmark (primary→secondary gradient in true colour), the tagline, and a `work --help` direction, then exits 0 — no selector. `work --help` shows the compact brand plus every registered command grouped by context, sourced from the Cobra tree, with no empty groups and no unavailable commands.

**Independent Test**: quickstart Q8 (wide gradient / narrow compact / piped usage+exit 2), Q9 (grouped inventory), Q10 (piped help plain + exit 0). SC-005, SC-009.

### Tests for User Story 2

- [ ] T036 [P] [US2] Golden tests `internal/present/brand/brand_test.go` per `contracts/brand.md`: `Render(width, profile, colorEnabled)` for wide+TrueColor (gradient present, per-column interpolation endpoints `#11A8CD`→`#8B7CF6`), wide+colour-off (art, **zero** `\x1b`), narrow (compact `WORK`, fits width, no wrap breakage). Assert `artWidth` is stable.
- [ ] T037 [P] [US2] PTY + `.txtar` test `tests/integration/brand_test.go`: interactive `work` at `COLUMNS=120` → multi-line `WORK` + tagline + `Run 'work --help'` on **stdout**, exit **0**, no selector program; at `COLUMNS=30` → compact form; `work | cat` → the F1/F2 non-interactive usage line on stderr, exit **2**, no ANSI. (US2 #1, #2; FR-017–FR-019, FR-028; SC-009.)
- [ ] T038 [P] [US2] Test `internal/cli/help_test.go` (the SC-005 correspondence check per `contracts/cli-help.md`): walk `root.Commands()`, assert every non-hidden command has a `GroupID` in the four declared groups, every rendered group has ≥1 member, the rendered help's command-line count equals the non-hidden command count, and no empty group renders. Run `work --help` through a pipe and assert exit 0 + no `\x1b`.

### Implementation for User Story 2

- [ ] T039 [P] [US2] Implement `internal/present/brand/wordmark.go`: the static embedded multi-line block `WORK` art as a `const` (draft from `temp/tui-revamp.md` §10.1), plus `artWidth`/`artHeight`. No external generator.
- [ ] T040 [US2] Implement `internal/present/brand/brand.go` per `contracts/brand.md`: `Render(width int, profile colorprofile.Profile, colorEnabled bool) string` choosing full-gradient / full-plain / compact; the gradient colours each terminal **column** of the art by linear `#11A8CD`→`#8B7CF6` interpolation across `artWidth`; tagline `Isolated work, ready when you are.` + `Run 'work --help' to get started.`. Depends on T039, T005.
- [ ] T041 [US2] Rewrite `internal/cli/root.go` `RunE` per `contracts/cli-work-home.md` + research R11: interactive (`tui.IsInteractive()`) ⇒ write `brand.Render(width, profile, colorEnabled)` to **stdout**, return nil (exit 0); non-interactive ⇒ the existing `diag.New(diag.Usage, …)` (exit 2, unchanged). Delete the `runHome` dispatch switch.
- [ ] T042 [US2] Delete `internal/tui/home.go` + `home_test.go` and remove `tui.RunHome`/`homeModel`/`HomeChoice` references from `internal/cli`. If `internal/tui` is now empty except `tty.go` (already moved in T002), delete the package directory. `go build ./...` green.
- [ ] T043 [US2] Implement `internal/cli/help.go` per `contracts/cli-help.md`: register `cobra.Group{ID:"daily",Title:"Daily Commands"}`, `{"in-work","Inside a Work"}`, `{"admin","Administration"}`, `{"setup","Setup"}` on the root; set `GroupID` on `start`/`resume`/`archive` = `daily`, `shell-init` = `setup`; assign Cobra's auto `completion` (and `help`) to `setup` or hide `help`; install a custom help/usage renderer that prints the **compact** brand, `Usage`, `Flags`, then one block per non-empty group in the fixed order. Depends on T040.
- [ ] T044 [US2] Update `internal/cli/root_test.go`: `TestBareWorkNonInteractiveIsUsage` stays green as-is; add/adjust a case for interactive bare `work` → exit 0 + brand on stdout (using a fake TTY or the PTY test). `TestRootHasSubcommands` unchanged.

**Checkpoint**: `internal/tui/` is gone; interactive `work` shows the brand and exits 0; `work --help` is grouped and inventory-driven; quickstart Q8–Q10 pass; F1 + F2 suites green.

---

## Phase 6: User Story 5 — Preserve scripts & output contracts + Polish (Priority: P5)

**Branch**: `git switch feature/003-terminal-ux-revamp-p5-us2-brand && git pull && git switch -c feature/003-terminal-ux-revamp-p6-polish` before T045. PR targets `feature/003-terminal-ux-revamp-p5-us2-brand`.

**Goal**: Prove the revamp broke no automation contract across every affected command, in redirected / no-colour / minimal-capability environments; finish the migration (remove dead code, decide `huh`).

**Independent Test**: quickstart Q7, Q10, Q13, Q14, Q15 — piped/redirected runs emit no ANSI and open no TUI; `NO_COLOR` / `TERM=dumb` plain; stdout vs UI-channel separation byte-exact; the full F1 + F2 suites green on all 3 OSes.

- [ ] T045 [P] [US5] `.txtar` test `tests/integration/no_ansi_when_piped.txtar`: run `work start … --yes` (all flags), `work resume <id>`, `work archive <id> --yes`, `work --help`, and bare `work` with stdout+stderr redirected; assert **no** byte `\x1b` in either stream and no prompt text, and that each opens no interactive control. (US5 #1, #2; FR-025, FR-027; SC-007, SC-008.)
- [ ] T046 [P] [US5] `.txtar` test `tests/integration/no_color_env.txtar`: repeat the key commands with `NO_COLOR=1` and with `TERM=dumb` (interactive-forced where the harness allows) → plain, legible, no control sequences; and one assertion that `NO_COLOR=` (empty) does **not** disable colour. (FR-025; SC-008; edge case.)
- [ ] T047 [P] [US5] PTY test `tests/integration/stream_separation_test.go`: capture stdout and the UI channel separately through a full interactive `work start`; assert every frame/receipt/help/diagnostic byte is on the UI channel and **only** `work: created`/`work: branch`/`work: path` are on stdout, byte-identical to F1. (US5 #2; FR-026; SC-007.)
- [ ] T048 [US5] Run the entire F1 quickstart (`specs/001-first-local-work/quickstart.md` S1–S12) and F2 quickstart (`specs/002-daily-cycle/quickstart.md` S1–S13) unchanged; fix any presentation-only assertion drift in the corresponding `.txtar` / `_test.go` **without** touching stdout / token / exit-code expectations; record the diff in the PR. (SC-007.)
- [ ] T049 [US5] Terminal-size sweep: parametrize the key PTY tests at 40×10, 80×24, 160×50 (extend the `console` harness to take a `Winsize`); assert no active control ever exceeds the viewport and long / wide-Unicode content produces zero stray scrollback lines. (SC-004.)
- [ ] T050 [P] Decide `huh` retention (research R19): once every entry point uses `present`, evaluate replacing `huh`-backed `Input`/`Confirm` with `bubbles/textinput` + a small confirm model. Either remove `charm.land/huh/v2` from `go.mod` (and `go mod tidy`) or leave a one-paragraph note in `internal/present/doc.go` explaining why it stays. No contract or `internal/cli` change either way.
- [ ] T051 [P] Remove every now-dead symbol: any leftover `internal/tui` reference, `retryable()` in `start.go` if the closures made it unused, `tui.MustInteractive` callers, dead help text. `go vet` + `staticcheck` clean.
- [ ] T052 [P] Update `docs/` for the shipped behaviour: confirm `docs/add/add-0001` §12.1–12.5, ADR-0019, ADR-0020, and `docs/prd.md` RF-50..RF-64 match what shipped (they were promoted in Phase 0); add a short "Presentation" note to the top-level `README`/`CLAUDE.md` if the wordmark/theme changed anything a contributor needs to know. Append the final chosen wordmark glyph design + a screenshot reference to `contracts/brand.md`.
- [ ] T053 [P] Add the SC-006 moderated-discovery check as a manual test script `specs/003-terminal-ux-revamp/checklists/discovery.md`: the steps a facilitator runs (give a first-time user only `work` and `work --help`, time finding the command for each public journey, target ≥90% within 30 s). Not automated; referenced from `quickstart.md`.
- [ ] T054 Run `make lint` + `go test ./...` on `ubuntu-latest`, `macos-latest`, `windows-latest`; confirm the full F1 + F2 + F2.5 suites green and the `diag.All` table test unchanged. This is the release-0.3 gate.

**Checkpoint**: `internal/tui/` deleted; no ANSI escapes anywhere output is non-interactive; every F1/F2 automation contract intact; quickstart Q1–Q15 green on all 3 OSes.

---

## Dependencies & Execution Order

### Phase dependencies

- **Phase 1 (Setup)**: branches off `feature/003-terminal-ux-revamp-p0-specs`. No code dependency.
- **Phase 2 (Foundational)**: needs Phase 1's capability probe + `present` skeleton. **Blocks all user stories.**
- **Phase 3 (US1)**: needs Phase 2 (`Input`, `Select`, `Confirm`, `receipt`, `geometry`, `theme`).
- **Phase 4 (US3+US4)**: needs Phase 2 (`Select`, `MultiSelect`, `diag` fields) and Phase 3's border-free `start.go` migration pattern; stacks on Phase 3 for merge order.
- **Phase 5 (US2)**: needs Phase 2's `theme` and Phase 4's diagnostic border (the brand shares the capability probe); stacks on Phase 4.
- **Phase 6 (US5)**: verification + cleanup; needs every prior phase.

### Within a user story

- Tests (model / golden / PTY / `.txtar`) are written alongside implementation and MUST pass at the checkpoint (roadmap §4 — not strict TDD, but no phase merges red).
- Primitives before their `cli` callers; a `tui` picker is deleted in the same phase its caller moves.
- `diag` field additions (T013) before the border (T032–T034).

### Parallel opportunities

- Phase 1: T002, T003, T004 in parallel after T001.
- Phase 2: T005, T006, T007 in parallel; then T009/T010/T011/T012 in parallel (different files) after T008; T013, T014, T015 in parallel.
- Phase 3: T016, T017, T018 (tests) in parallel; T019–T024 touch the same `start.go` so are **sequential**; T025/T026 after.
- Phase 4: T027, T028, T029 in parallel; T030 and T031 in parallel (different files); T032 before T033/T034.
- Phase 5: T036, T037, T038 in parallel; T039 before T040; T041/T042 sequential (both touch `root.go`); T043 after T040.
- Phase 6: T045–T047, T050–T053 largely in parallel; T054 last.

---

## Implementation Strategy

### MVP (User Story 1 only)

1. Phase 1 (Setup & PTY proof) → 2. Phase 2 (Foundational) → 3. Phase 3 (US1).
4. **STOP and VALIDATE**: quickstart Q1–Q4 on Linux + macOS; F1 S1–S12 + F2 S1–S13 green.
5. `work start` now leaves a clean, auditable inline history. Demo-able.

### Incremental delivery

- + Phase 4 → stable selectors + a single diagnostic border (US3, US4). Q5, Q6, Q11, Q12.
- + Phase 5 → the branded home + grouped help (US2). Q8–Q10.
- + Phase 6 → the non-interactive compatibility proof + cleanup (US5). Q7, Q13–Q15.
- Each phase merges forward only with the whole F1 + F2 + prior-F2.5 suite green (SC-007).

### After the stack lands

`feature/003-terminal-ux-revamp-p0-specs` merges onward (user's cadence call); `release/0.3` is
cut from `develop`, merged to `master`, tagged.

---

## Notes

- `[P]` = different files, no dependency on an incomplete task.
- `[Story]` label maps a task to a spec user story for traceability; Setup / Foundational / Polish tasks carry none.
- **No stdout line, error token, or exit code changes** for any existing command except interactive bare `work` → exit 0. Every task that edits a flow must keep the F1/F2 `.txtar` stdout/token/exit assertions intact.
- `internal/present` must never import a Work domain package (enforced by T014).
- Commit after each task or logical group; the phase branch is created **before** the phase's first task.
