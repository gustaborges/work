// Package resume is the transactional orchestrator for `work resume`. It
// resolves the target Work, bumps its last_accessed_at in the canonical
// snapshot (the commit point) and then in the projection (a reconcilable
// post-commit trailer), and returns the worktree path the caller repositions
// into.
//
// The operation runs under an advisory lock keyed by the Work id
// (state/locks/<sha256(id)>.lock); a second concurrent operation on the same
// Work fails cleanly on the lock timeout with no state change. Operations on
// different Works never contend.
package resume
