// Package diagrender renders a human-facing diagnostic for an interactive
// terminal (contracts/diagnostics.md, ADD §12.4): a single `✘ <summary>` line
// with an optional `  → <hint>` follow-up, and a dedicated one-line notice for
// cancellation. The `✘` mark uses the Danger token and carries the meaning with
// colour off. The non-interactive `error: <token>: <message>` form stays in
// internal/diag and is unchanged; this package is only the interactive path.
package diagrender
