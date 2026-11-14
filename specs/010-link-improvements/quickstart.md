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

## Implementation evidence (2026-11-14)

Implemented the single-response mode overlay and known-link exclusion. Reviewed FR-84–FR-86, ADR-0031, ADD §§7/9/11, and the historical protocol addendum: no contract drift found. Existing manual replacement regressions in `link.txtar` already cover replacing known values without launching a provider.

Validation uses `GOCACHE=/tmp/work-go-cache` because the host cache is read-only in the workspace sandbox. Disposable test commits use `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=commit.gpgsign GIT_CONFIG_VALUE_0=false`; the delivery commit retains configured SSH signing. Built `make seed-all` before testing. Cross-compilation is preparation only, not native OS execution.

Passed: focused IPC/Starter/extension suites; Starter subprocess contract suite (including one arg-only invocation); selected fork/contribution snapshots; new-mode overlay, invalid unselected output and known-link preservation testscript scenarios; existing manual link tests; `go vet ./...`. Full `go test ./...` passed on Linux (integration package: 345.415s); `go vet ./...` passed. Native CI is pending PR publication.

Native macOS and Windows execution is delegated to the repository's existing CI matrix. Those outcomes are not claimed as local validation.
