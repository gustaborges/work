// Package reconcile rebuilds and reconciles the SQLite projection from the
// canonical Work snapshots. The snapshots are authoritative and are never
// written by this package.
//
// Rebuild drops and recreates the works table at user_version 2 and indexes one
// row per readable, schema-valid snapshot found directly under
// <workspace>/in-progress/*/work-state.json and
// <workspace>/archived/*/work-state.json. Reconcile upserts every on-disk
// snapshot's row (snapshot values win, fixing a stale last_accessed_at or
// status) and drops any row whose Work is no longer on disk. An unreadable,
// unparseable or schema-invalid snapshot is skipped and collected in the Report
// rather than aborting the operation.
package reconcile
