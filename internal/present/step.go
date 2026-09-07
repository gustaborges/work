package present

import (
	tea "charm.land/bubbletea/v2"
)

// stepStatus is the terminal signal a step exposes after every Update. The
// wizard polls it to decide whether to advance, abort, or cancel; a step never
// quits the program itself.
type stepStatus struct {
	// done is set once the step has an accepted answer.
	done bool
	// cancelled is set when the user backed out of the step (Esc / q / Ctrl-C).
	cancelled bool
	// fatal, when non-nil, aborts the whole wizard and is handed to the CLI
	// diagnostic border (same effect as present.Fatal).
	fatal error
	// receipt is the compact durable record appended to the wizard's receipt
	// trail once the step is done. Empty for a cancelled or aborted step.
	receipt string
	// answer is the accepted value, stored under the step key in Answers.
	answer any
}

// stepModel is one interview step: a self-contained interaction the wizard
// drives. Unlike a standalone tea.Model it never returns tea.Quit and never
// paints the wizard chrome (the top rule, the wizard title, the receipt trail) —
// body renders only this step's own content and the wizard composes the frame.
type stepModel interface {
	Init() tea.Cmd
	Update(tea.Msg) (stepModel, tea.Cmd)
	// body renders this step's content for an interior of width×height cells,
	// without the wizard's rule/title/receipts and without the viewport clamp
	// (the wizard clamps the whole composed screen).
	body(width, height int) string
	// status reports whether the step has reached a terminal state.
	status() stepStatus
	// cursor reports where the hardware text cursor should sit within body's
	// output (row 0 = body's first line); ok is false when the step wants no
	// visible cursor.
	cursorPos() (pos tea.Position, ok bool)
}
