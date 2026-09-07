# Feature Specification: Terminal UX Revamp

**Feature Branch**: N/A — no branch was created by the specification flow

**Created**: 2026-09-07

**Status**: Draft

**Input**: User description: `temp/tui-revamp.md`, with the explicit requirement that the `WORK` wordmark be terminal art using a gradient of the settled primary and secondary colors.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Complete Interactive Steps Without Scrollback Debris (Priority: P1)

As a developer completing an interactive Work journey, I see only the current attempt while editing and a compact record of each accepted answer afterward, so I can verify what I chose without rejected values, old errors, or expanded pickers cluttering terminal history.

**Why this priority**: The current retry and completion behavior makes ordinary creation flows noisy and difficult to audit. Fixing the interaction lifecycle delivers the central value of the revamp across every journey.

**Independent Test**: Run an interactive `work start`, enter several invalid repository paths and branch-derived values before valid ones, finish each selector, and verify that terminal history contains one compact receipt per accepted step and none of the rejected attempts or obsolete errors.

**Acceptance Scenarios**:

1. **Given** an active input step, **When** the user submits an invalid value four times and then a valid value, **Then** each error replaces the preceding error in the active frame and the completed history contains only the valid receipt.
2. **Given** an expanded selector with unused viewport rows, **When** the user accepts an option, **Then** the selector is replaced by a compact receipt and neither its list nor blank padding remains before the next step.
3. **Given** a value marked sensitive by the journey, **When** the step completes, **Then** its receipt identifies the completed step without revealing the sensitive value.

---

### User Story 2 - Discover Work Through a Branded Landing and Complete Help (Priority: P2)

As a user who does not remember Work's commands, I can run `work`, recognize the product from its terminal-art `WORK` wordmark, and follow the prompt to a complete, grouped help view that exposes every journey available in the installed binary.

**Why this priority**: Replacing the selectable home must preserve the original discoverability goal while producing a smaller, faster, and more accurate entry point.

**Independent Test**: Run `work` and `work --help` in wide, narrow, colored, monochrome, interactive, and redirected environments; verify the responsive brand, exit behavior, contextual command groups, and exact correspondence between help entries and available commands.

**Acceptance Scenarios**:

1. **Given** a wide interactive true-color terminal, **When** the user runs `work`, **Then** a terminal-art wordmark spelling `WORK` is shown with a primary-to-secondary gradient, followed by the tagline and a `work --help` direction, without opening a selector.
2. **Given** a narrow or color-disabled terminal, **When** the user runs `work`, **Then** the compact plain `WORK` brand and help direction remain legible without broken wrapping.
3. **Given** commands available in the current binary, **When** the user opens `work --help`, **Then** every available public command appears exactly once under an appropriate context group and no unavailable command is advertised.

---

### User Story 3 - Navigate Stable, Accessible Selectors (Priority: P3)

As a keyboard user moving through branch, Work, plugin, repository, convention, Importer, or Linker choices, I can see focus clearly and use consistent keys without rows jumping, checkboxes shifting, or the control exceeding the terminal viewport.

**Why this priority**: Geometry changes undermine trust and make multi-selection error-prone; consistent focus and key behavior are required for a coherent terminal product.

**Independent Test**: Exercise every implemented single-select and multi-select control across cursor movement, selection toggles, filtering, group changes, resize, long labels, and Unicode content; compare row heights and column starts before and after each action.

**Acceptance Scenarios**:

1. **Given** a two-line multi-select list, **When** focus moves between selected and unselected rows, **Then** unchanged rows retain identical line counts and text/checkbox starting columns.
2. **Given** color is unavailable, **When** focus moves, **Then** bold styling and the fixed-width textual focus marker still identify the focused option.
3. **Given** a short or narrow viewport with long content, **When** the control renders or resizes, **Then** primary identity remains visible, secondary content yields first, and no active frame exceeds the available rows or columns.

---

### User Story 4 - Receive Concise Cancellation and Actionable Errors (Priority: P4)

As a developer who cancels a journey or encounters a terminal failure, I receive one concise result in my vocabulary, with a next action when known, while automation retains the established error category and exit code.

**Why this priority**: Cancellation is normal control flow and errors should help recovery; duplicated wrappers and retained pickers obscure both.

**Independent Test**: Cancel resume and archive from selection and confirmation, trigger recoverable and terminal failures, and verify one human result, no mutations, stable programmatic outcomes, and optional access to the underlying cause only through explicit diagnostics.

**Acceptance Scenarios**:

1. **Given** an interactive resume selection, **When** the user cancels, **Then** the terminal retains one concise cancellation result, exits with the established cancellation code, and does not update recent access.
2. **Given** a recoverable field error, **When** the user edits or retries, **Then** the old message is replaced in place and no terminal-level diagnostic is printed.
3. **Given** a non-recoverable failure with a known recovery action, **When** the command ends, **Then** the user sees one summary and that action, while internal cause text is absent from normal output.

---

### User Story 5 - Preserve Scripts and Output Contracts (Priority: P5)

As a user invoking Work from scripts or redirected streams, I continue to receive stable plain output and exit behavior without interactive controls or styling bytes, so the visual revamp does not break automation.

**Why this priority**: The terminal experience may change substantially, but existing direct commands are a public automation contract.

**Independent Test**: Run all affected commands with explicit arguments over redirected input/output, with color disabled and with a minimal terminal capability, then compare stdout, error tokens, mutation behavior, and exit codes to the existing command contracts.

**Acceptance Scenarios**:

1. **Given** non-interactive input and a required value is omitted, **When** the command runs, **Then** it fails with actionable usage without creating an interactive control.
2. **Given** a successful direct command in non-interactive mode, **When** it completes, **Then** stable results remain on stdout and presentation or human diagnostics do not contaminate them.
3. **Given** `work` without arguments is invoked non-interactively, **When** it runs, **Then** it preserves the established usage failure; `work --help` succeeds in the same environment.

### Edge Cases

- A terminal becomes narrower or shorter while a selector or input is active.
- A label, path, branch, repository name, or description contains wide Unicode characters, combining marks, or styling-insensitive long text.
- The terminal is smaller than the preferred selector viewport and cannot display every optional element at once.
- `NO_COLOR` is present with an empty value versus a non-empty value; only the non-empty value disables color under this contract.
- `TERM=dumb`, redirected output, or a monochrome terminal cannot safely render the gradient or semantic colors.
- A font or console cannot display the preferred success/failure glyphs at stable width.
- A destructive confirmation is declined, cancelled, or returns to selection before acceptance.
- An option disappears or the available list becomes empty before a selection can complete.
- A validation operation takes perceptible time or is cancelled before returning.
- The help view contains an empty command group or a command unavailable in the current build; neither may be displayed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Each interactive command flow MUST run as one full-screen alternate-buffer program; on exit the primary buffer MUST be restored and the compact accepted-step receipts MUST be reprinted to the UI channel so they enter terminal history (ADR-0021).
- **FR-002**: Every active control MUST remain bounded by the terminal's current rows and columns after reserving space for its title, current error, help, filter, and confirmation content.
- **FR-003**: When an interactive step is accepted, its live control MUST be replaced by a compact receipt containing the step title, a success mark, and the accepted display value.
- **FR-004**: A journey MUST be able to provide a redacted receipt or omit the receipt value for sensitive input.
- **FR-005**: A recoverable validation failure attributable to the active field MUST appear within that field's live frame; editing or retrying MUST replace the previous failure.
- **FR-006**: At most one current validation error MUST be visible for the active field, and rejected values or obsolete validation messages MUST NOT remain in terminal history after acceptance.
- **FR-007**: A selector MAY retain a generous, stable scrolling viewport while active, but MUST collapse to its compact receipt on acceptance; that receipt appears in the primary-buffer history reprinted when the flow exits, with no blank padding.
- **FR-008**: Moving focus, toggling a choice, filtering, or changing a group MUST NOT change the starting columns or rendered line counts of unchanged rows.
- **FR-009**: Focused options MUST be bold and MUST use a textual focus position whose display width is reserved for every row; color MAY reinforce focus but MUST NOT be required to perceive it.
- **FR-010**: Single-select and multi-select controls MUST consistently support arrow keys and `j`/`k` for movement, Enter for selection or continuation, `/` for filtering where filtering exists, and a clearly displayed cancellation key.
- **FR-011**: Ctrl-C MUST remain responsive during collection and before or during mutation according to the existing transaction contract.
- **FR-012**: Interactive cancellation MUST leave one concise human result and MUST preserve the established programmatic cancellation category and exit code for commands that define one.
- **FR-013**: Cancellation before mutation or confirmation MUST NOT change Work state, access time, files, branches, worktrees, configuration, or plugin state.
- **FR-014**: A non-recoverable failure MUST be rendered exactly once at the command/process boundary; lower layers and active controls MUST NOT also print it.
- **FR-015**: Expected failures MUST identify what failed in user vocabulary and MUST include a known next action when one exists; internal wrapping chains MUST NOT appear in normal human output.
- **FR-016**: Underlying causes MUST remain available to explicit diagnostic inspection without changing the normal human or programmatic output contract.
- **FR-017**: In an interactive terminal, `work` without arguments MUST render a static brand, tagline, and direction to `work --help`, MUST NOT open a selectable interface, and MUST exit successfully.
- **FR-018**: The full brand MUST spell `WORK` as terminal art and, in a suitable true-color terminal, MUST apply a visible gradient from primary `#11A8CD` to secondary `#8B7CF6` across the wordmark.
- **FR-019**: Narrow, limited-color, monochrome, and non-interactive presentation MUST use a compact plain `WORK` representation without broken wrapping or unsafe styling.
- **FR-020**: `work --help` MUST include the brand, usage, options, and every command implemented in the current binary, grouped by usage context; it MUST NOT show empty groups or unavailable commands.
- **FR-021**: Help grouping MUST distinguish, when populated, daily/global commands, commands that require a materialized Work, administration, and setup/internal plumbing.
- **FR-022**: Every public journey MUST remain directly invocable by a documented command and discoverable from the bare-command direction plus help, without requiring a selectable home.
- **FR-023**: The presentation MUST use one semantic theme in which brand accents remain distinct from success, warning, and failure meanings, while body text follows the terminal foreground.
- **FR-024**: Light-background, limited-color, and monochrome environments MUST receive contrast-safe output, and no state or focus meaning MAY be conveyed by color alone.
- **FR-025**: Color MUST be disabled when output is non-interactive, `NO_COLOR` is non-empty, or `TERM=dumb`; these outputs MUST contain no styling control sequences and MUST remain understandable.
- **FR-026**: Interactive presentation and human diagnostics MUST use the configured UI/error channel; stable command results MUST remain on stdout according to existing command contracts.
- **FR-027**: When the relevant input or UI channel is non-interactive, commands MUST NOT open interactive controls. Existing arguments, flags, stable stdout, error tokens, and exit codes MUST remain compatible unless a later contract explicitly changes them.
- **FR-028**: In non-interactive mode, `work` without arguments MUST preserve the current usage failure and exit code 2, while `work --help` MUST exit successfully.
- **FR-029**: Explicit command values MUST continue to skip only the corresponding selection; they MUST NOT skip validation, confirmation, mutation safeguards, or stable result reporting.
- **FR-030**: A confirmation preceding a mutation MUST present its impact before acceptance; after acceptance it MUST collapse to a compact confirmation receipt before stable operation results are written.
- **FR-031**: All currently implemented public interactive choices MUST adopt the same lifecycle, key conventions, focus semantics, terminal bounds, cancellation behavior, and semantic theme.
- **FR-032**: Presentation behavior MUST remain consistent across all operating systems and terminal environments already supported by Work.

### Key Entities

- **Interaction Step**: One bounded unit of input, single selection, multiple selection, or confirmation, with a title, active state, accepted result, and cancellation result.
- **Presentation Option**: A generic selectable value with primary identity, optional secondary description, optional group, focus state, and independent selected state.
- **Receipt**: The compact durable record of an accepted step; contains safe display text and never exposes a value designated sensitive.
- **Diagnostic**: A recoverable field message or terminal command result with public summary, optional next action, stable programmatic classification, and a separately inspectable cause.
- **Theme**: Semantic presentation tokens for brand, focus, success, warning, failure, muted content, and body text, with capability-appropriate variants.
- **Command Group**: A non-empty help category whose members are exactly the public commands available in the current binary.

### Scope and Dependencies

- This feature supersedes the selectable bare-command home behavior documented by the F1/F2 home contracts; those documents remain historical delivery records.
- The existing direct command grammar, selection semantics, transaction guarantees, stable outputs, and exit codes remain authoritative except for the explicitly changed interactive bare-command behavior.
- The feature applies to the existing `start`, `resume`, and `archive` journeys and to every other public interactive choice present when the revamp ships.
- The established supported operating-system matrix and shell-repositioning contract remain unchanged.
- Product authority is `docs/prd.md` RF-50 through RF-64 and RNF-10 through RNF-11. ADR-0019 governs the command surface; ADR-0020 governs the presentation boundary; `docs/add/add-0001-work-system-architecture.md` describes their realization.

### Out of Scope

- Backward navigation that edits an accepted earlier step and invalidates later choices.
- New public journeys, aliases, command grammar, domain mutations, or plugin capabilities.
- A graphical interface.
- A user-selectable theme editor or new `--no-color` flag; existing environment-based opt-outs are included.
- Choosing or exposing a presentation library as a public contract.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After any number of rejected attempts followed by acceptance, the terminal history reprinted when the flow exits contains exactly one receipt for the step and zero rejected values or obsolete validation errors.
- **SC-002**: 100% of completed interactive controls leave a compact final result (in the reprinted history) with no selectable rows or viewport padding; 100% of cancelled controls leave one concise cancellation result.
- **SC-003**: Across focus movement, selection toggles, filtering, and group changes, unchanged selector rows exhibit zero change in rendered line count and zero change in the starting columns of their focus marker, checkbox, primary text, and secondary text.
- **SC-004**: At terminal sizes 40×10, 80×24, and 160×50, every active control remains within the available viewport; long and Unicode content causes zero unintended scrollback lines from an oversized frame.
- **SC-005**: Help lists 100% of public commands present in the tested binary exactly once, lists zero unavailable commands, and renders zero empty command groups.
- **SC-006**: In a moderated discovery check, at least 90% of first-time users can identify the command for each available public journey within 30 seconds using only `work` and `work --help`.
- **SC-007**: Across all affected non-interactive contract tests, 100% of existing arguments, flags, stable stdout lines, error tokens, and exit codes remain unchanged except the explicitly contracted interactive bare-command behavior.
- **SC-008**: On every supported operating system, the wide-color, narrow, monochrome, `NO_COLOR`, minimal-capability, and redirected-output scenarios all remain legible and contain styling control sequences only when allowed.
- **SC-009**: In visual review, at least 90% of reviewers recognize the full terminal-art wordmark as `WORK` and rate both its primary-to-secondary gradient and its plain fallback as readable.
- **SC-010**: Every expected terminal failure is printed once, and every failure with a known recovery action includes that action; cancellation and recoverable-error tests perform zero unintended mutations.

## Assumptions

- The settled dark-terminal brand colors are primary `#11A8CD` and secondary `#8B7CF6`; success, warning, and failure retain separate semantic colors.
- “Terminal art” means a static multi-line ASCII/Unicode-style wordmark that visibly spells `WORK`; the exact glyph design is selected during planning and visual verification without adding a product dependency on an external generator.
- Accepted confirmation detail collapses after acceptance because the stable command result remains the durable operation summary; the full impact remains visible until the user confirms.
- Existing public command contracts define which cancellation paths use exit code 20; this feature preserves rather than broadens that category.
- Preferred Unicode success/failure marks may fall back to fixed-width ASCII where a supported console cannot render them predictably.
- Validation may show a neutral in-progress state when it takes perceptible time, but validation does not mutate Work state and remains safe to retry.
- The current supported platforms, terminal foreground/background detection behavior, and direct command surface are reused.
- No critical product, security, or privacy choice remains unresolved for specification; implementation-level prototype findings may refine layout without weakening these outcomes.
