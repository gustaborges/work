# Contract: Starter response — F5 amendment (`meta` and `links` consumed)

Authority: ADD §7, §9; ADR-0013; spec FR-012..FR-015; `research.md` R4, R10.
**Extends** `specs/005-plugin-origins/contracts/starter-protocol.md` (which extends
`specs/001-first-local-work/contracts/ipc-starter.md`). Everything there holds: the Starter
receives only `{ "arg": <argument> }`; `repository`, `base_branch` and `start_modes` are
consumed as F4 defined; a non-zero exit or structurally invalid response fails `work start`.
The one change: **F4's "`meta`/`links` are read by the wire format but deliberately discarded"
no longer holds.**

## Response

```jsonc
{
  "repository": { "git_fetch_urls": ["https://github.com/example/project.git"], "name": "project" },
  "base_branch": "feature/source-branch",
  "start_modes": ["contribution", "fork"],
  "meta":  { "github.pull_request.number": 212 },
  "links": { "github.pull_request": "https://github.com/example/project/pull/212" }
}
```

Both new fields are optional and independent; absent = empty. Any other key in the response
is still ignored.

## Validation (before any materialization)

Each `meta` key and each `links` key must satisfy `semantic-conventions.md` §1–§2 with the
**Starter's own plugin name** as owner (its package's manifest `name`); each `links` value
must be a non-empty string. `meta` values may be any JSON value. A key or value that violates
these rules is a structurally invalid response:

```text
error: starter-response-invalid: the Starter returned a link or metadata key that is not valid: "GitHub.PR"
```

exit 37, exactly as F4 defined for that class, raised by `starter.ValidateResponse` before
`create.Run`, so no branch, worktree, directory, snapshot or index entry exists to undo. The
message names the offending key; it does not print values.

A `links` value that is not a JSON string never reaches validation: the response fails JSON
decoding, which keeps its existing F1/F4 classification (`unusable-repo`, exit 11) — the same
as any other undecodable Starter output. F5 changes no existing token or exit code
(spec FR-045); the outcome the spec requires (`work start` fails, nothing created) holds either way.

## Persistence

The validated maps become the `meta` and `links` sections of the Work's **first** snapshot,
written by the same step that creates the Work (no window in which the Work exists without
them), for every start mode. Provenance for each entry is recorded in the same index
transaction as the Work's row: component = the Starter's `alias/name`, operation `start`,
time = creation time. The Repository Reference stays transient: nothing in `repository` is
promoted into `meta` or `links` by the core.

## Trust boundary (unchanged)

The Starter still receives nothing derived from a Work. What it publishes lands only in the
`meta` and `links` sections; it cannot write the `work` section (`work.state`, `starter`,
`branch`, …), and the core alone writes the snapshot.
