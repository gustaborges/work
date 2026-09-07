// Package present is the generic inline-interaction boundary between the CLI and
// the terminal (ADR-0020, ADD §12.2). It owns interaction state, key handling,
// viewport geometry, final receipts, theme application, and explicit I/O; it
// exposes Input, Select, MultiSelect, and Confirm primitives that each run as
// one bounded Bubble Tea program in the current screen buffer with explicit
// editing/completed/cancelled states.
//
// It does not own journey sequencing, domain decisions, option meaning, or
// mutations. The CLI supplies titles, descriptions, generic options,
// side-effect-free validation closures, receipt formatters, and confirmation
// content. present imports no Work domain package; its only internal dependency
// is internal/diag, the shared domain-free error vocabulary. An import-boundary
// test (tests/contract/present_boundary_test.go) enforces this mechanically.
package present
