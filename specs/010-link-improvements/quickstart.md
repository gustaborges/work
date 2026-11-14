# Validation Guide: Link Improvements

These are implementation acceptance checks, not outcomes claimed during planning. Use temporary WORK_HOME/workspaces and local Git repositories only; do not modify the real Work home or require live GitHub access.

## Prerequisites

Go 1.26.0, Git, and existing Go fixture/test helpers. Implement tasks.md first. Preserve native Windows/macOS/Linux fixture invocation; no new shell or Python runtime requirement.

## Focused checks

```sh
go test ./internal/ipc ./internal/starter ./internal/extension ./internal/cli
go test ./tests/contract ./tests/integration
```

Extend existing subprocess fixtures and mode integration helpers to exercise:

1. A one-call Starter with shared source link and contribution/fork/new overrides. Inspect work-state.json for selected links/meta, selected mode, provider-bound private keys, and no unselected data. A marker records one invocation per resolution attempt.
2. Invalid unselected publication, unknown mode, wrong-shaped modes/entry/maps and empty/null/non-string link values. Expect exit 37 and unchanged Git refs, no new worktree, Work directory, or Work snapshot. Check that diagnostic text contains no link/meta values.
3. No modes, null/empty modes, missing selected entry, shallow replacement and null metadata. Compare legacy snapshots and prove input maps remain unchanged in unit tests.
4. Marker-writing automatic Linkers for known/missing public and private keys; known-key components emit nothing, missing-key controls run, and private provider/source-instance isolation holds. Include ambiguous-filter silence and Applicable false.
5. One Linker produces another's required input; the second stays ineligible for that phase. Two initially eligible same-key Linkers both retain their decisions. Importers consume known and newly discovered links afterward.
6. Existing manual work link replaces a known value without invoking discovery; rerun tests/integration/link.txtar via the existing integration suite.

## Complete validation

```sh
go test ./...
go vet ./...
```

Run native CI on supported operating systems; cross-compilation alone is not execution evidence. Review changed contracts and test receipts, then deliver both stories together. Marketplace-side removal of PR guessing and branch upstream behavior remain separate work.
