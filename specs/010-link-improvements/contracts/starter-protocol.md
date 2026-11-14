# Contract: Mode-specific Starter Context

Authority: PRD FR-84, ADR-0031, ADD §7. Additive to [feature 005 protocol](../../005-plugin-origins/contracts/starter-protocol.md) and current context publication rules.

## Invocation and response

Input remains exactly `{"arg":"<source>"}`. One invocation per resolution attempt; selection does not invoke again.

```json
{
  "repository": {"path": "/clone/project"},
  "base_branch": "feature/source",
  "start_modes": ["contribution", "fork"],
  "links": {"github.source_pull_request": "https://github.com/o/r/pull/1"},
  "meta": {"example.title": "shared"},
  "modes": {
    "contribution": {"links": {"github.pull_request": "https://github.com/o/r/pull/1"}},
    "fork": {"links": {"github.forked_pull_request": "https://github.com/o/r/pull/1"}}
  }
}
```

Keys in examples must satisfy existing Semantic Conventions publication rules. `modes` does not create a new registry of public semantic keys.

- modes absent/null/{}: no override.
- Known names: new/contribution/fork. Unknown names fail code 37, even if not selected. `start_modes` still offers only contribution/fork; absent means new. A known but unoffered entry is validated but never selected.
- Present mode entries must be objects; scalar/array/null entries fail 37. Omitted/null links/meta maps mean empty overrides. Wrong-shaped maps and non-string/empty/null link values fail 37. Metadata may contain any JSON value including null.
- Only links/meta are consumed from each entry; unrelated fields cannot override repository, branch, mode offers, or core Work state.
- Validate base plus all overrides before repository resolution/materialization. Existing malformed-JSON/process failures outside the new modes field keep existing diagnostics.
- Merge selected maps over base, shallowly, with selected values winning equal keys. No field deletion semantics. Absent selected mode leaves copied base maps intact. Do not mutate response maps.
- Bind merged maps once with the selected Starter's provider/current source instance. Persist through existing creation path; unselected fields and modes map never persist.
- Errors use starter-response-invalid (37), identify offending field/mode/key without values, and create no Work/branch/worktree. The mode is named in the user-visible message itself (not only in a wrapped cause) for key and value failures inside a mode entry. Starter-supplied names and keys are quoted, and every such error carries the same summary and hint as the other structural Starter failures.

## Contract evidence to add

Contribution/fork/new, absent modes, missing selected entry, empty maps, null metadata replacement, duplicate-key override, malformed modes shapes, unknown names, invalid unselected keys/values, private binding, and one invocation per successful start. Assert no creation effects on failure and stable diagnostics.
