# The `work` marketplace

The catalog the Work project publishes. This directory is a snapshot of that
publication, kept so the catalog is reviewable and testable next to the code
that consumes it. **The published repository is the source of truth.**

| | |
| --- | --- |
| Origin | `https://github.com/gustaborges/work-cli-marketplace.git` |
| Branch | `master` |
| Namespace | `work` |
| Snapshot of | commit `0518d1fd11358c52b21f813fa4049c58a976666d` |

## Attribution rules

- A source is the official one only if its normalized Git descriptor equals the
  origin above **and** its catalog declares the namespace `work`. The name
  alone proves nothing: anyone can publish a catalog called `work`.
- Registration is explicit (`work plugin marketplace add <origin>`). Nothing
  registers it automatically, and removing it never reinstalls it.
- `builtin` is separate and code-owned; this catalog neither provides nor
  replaces it.

## Contents

| Plugin | Version | Components |
| --- | --- | --- |
| `github` | 1.1.0 | PR and issue starters; PR and issue linkers; PR description, comments and diff importers; issue importer |
| `wsp` | 1.0.1 | repository locator backed by `wsp find` |

Publishing a change: edit the packages in the origin repository through a
branch and pull request, bump the plugin `version` when behavior changes
(installed copies only see an update when the version differs), then refresh
this snapshot.
