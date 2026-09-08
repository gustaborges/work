// Package present is the generic interaction boundary between the CLI and the
// terminal (ADR-0020, ADR-0021, ADD §12.2). It owns interaction state, key
// handling, viewport geometry, final receipts, theme application, and explicit
// I/O; it exposes Input, Select, MultiSelect, and Confirm primitives and the
// Wizard that composes ordered Steps into one full-screen alternate-buffer
// Bubble Tea program per flow.
//
// A flow renders as a Primary top rule, the flow title, the trail of
// accepted-step receipts, and the current step's body, with a full clear +
// repaint every frame (which makes the inline renderer's resize / back-nav
// ghosting impossible). Each step has explicit editing/completed/cancelled
// states and never quits the program itself — it reports a terminal status and
// the Wizard drives the transition. On exit the primary buffer is restored and
// the compact receipts are reprinted to the UI channel, ahead of the command's
// stable stdout.
//
// It does not own journey sequencing, domain decisions, option meaning, or
// mutations. The CLI supplies titles, descriptions, generic options,
// side-effect-free validation closures, receipt formatters, confirmation
// content, and — through each Step's build(Answers) closure — the order in which
// later steps read earlier answers. present imports no Work domain package; its
// only internal dependency is internal/diag, the shared domain-free error
// vocabulary. An import-boundary test (tests/contract/present_boundary_test.go)
// enforces this mechanically.
//
// Text editing, selection, and confirmation are implemented directly on
// charm.land/bubbletea/v2 and charm.land/lipgloss/v2. The design (research R19)
// allowed charm.land/huh/v2 to back Input/Confirm during migration; it was never
// needed — the per-step lifecycle here (a stepModel the Wizard composes, no
// standalone form model, caller-supplied validation) does not match huh's
// standalone form model — so huh was dropped from the module in the final F2.5
// slice.
package present
