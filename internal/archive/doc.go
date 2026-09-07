// Package archive is the transactional orchestrator for `work archive`. For
// each selected Work it runs an ordered pipeline — dirty-worktree check, flip
// the snapshot status to "archived" (the canonical commit point), destroy the
// Git worktree (the branch ref is left intact, FR-014), rename the Work
// directory under <workspace>/archived/<yyyymmdd>-<repo>_<branch>[-<n>], and
// update the projection — with a LIFO compensation stack that unwinds any step
// failing before the commit point. Each Work is processed under its own
// advisory lock (state/locks/<sha256(id)>.lock) and its own compensation stack:
// a failure on one batch member never rolls back a Work already archived
// (FR-017).
//
// # Fault injection
//
// For the transactionality test suite, the environment variable WORK_FAIL_AT
// forces one named step of the per-Work pipeline to return an error:
//
//	WORK_FAIL_AT=snapshot     fail the snapshot status flip (before the commit point)
//	WORK_FAIL_AT=worktree     fail the git worktree removal
//	WORK_FAIL_AT=move         fail the directory rename into <workspace>/archived/
//	WORK_FAIL_AT=projection   fail the post-commit projection update
//
// A failure at or before "move" must leave the Work fully active; a failure at
// "projection" leaves the Work canonically archived on disk, with the row
// brought into agreement by the next command's reconcile. The variable is
// honoured only to drive tests and has no effect on a normal run when unset.
package archive
