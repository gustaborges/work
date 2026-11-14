# Component roles and protocols

Read the [generated role/IPC contract](generated/role-matrix.md) and [manifest role constraints](generated/manifest-schema.md). Commands execute directly with JSON stdin, bounded stdout (1 MiB) and a stderr tail (4 KiB). Runtime must be present for selected installation. Installation/discovery never execute components.

- Starter: receives arg, returns repository reference (path, git_fetch_urls, name or query), optional base_branch/start_modes/meta/links/modes. Patterned starters match arguments; a starter without pattern is fallback. Return real repository information.
- Repository Locator: declares accepts from name/git_fetch_urls/query, receives repository and repository_roots, returns matches containing repo_path. Return actual Git roots, not fabricated paths.
- Importer: declares on or manual, optional inputs. Receives inputs and output_dir. Write only under that provided directory. Empty stdout or one JSON object is accepted; returned object fields are ignored.
- Linker: declares owned key plus discover or manual. Receives resolved inputs. Empty stdout, {}, absent value or null value mean nothing found. A present value must be a nonempty string. Automatic discovery requires its event declarations to run.

Events currently include start:finalized. Starter restrictions use bare contribution names or plugin@marketplace/component. Inputs use work:key, meta:key or link:key with optional :optional. Work facts are limited to the generated list. Public semantic keys are dotted lowercase segments. Relative private publication/declaration keys use plugin.<encoded-plugin>.<local>; cross-provider input keys use plugin.<encoded-plugin>@<encoded-marketplace>.<local>. Encode dots in owner names as %2E. The ~registration suffix is core storage only. A plugin may publish only its own private keys. Never expose core-only state through undeclared inputs.

## Starter mode context

Input remains exactly `{"arg":"<source>"}`; choosing a mode does not invoke the Starter again. Optional `modes` maps `new`, `contribution`, and `fork` to links/meta overrides. It does not offer modes: `start_modes` still offers only contribution/fork, and contribution requires a nonempty `base_branch`. With no offers, creation uses new.

```json
{
  "repository": {"name": "project"},
  "base_branch": "feature/source",
  "start_modes": ["contribution", "fork"],
  "meta": {"plugin.example.title": "shared"},
  "modes": {
    "contribution": {"meta": {"plugin.example.title": "contribution"}},
    "fork": {"meta": {"plugin.example.title": "fork"}}
  }
}
```

Absent/null/empty modes mean no overrides. Every present entry must be an object with a known name; omitted/null links/meta maps are empty. Links require nonempty string values; metadata accepts any JSON value, including null. All base and mode publication keys must satisfy the same semantic-key and plugin ownership rules, including unselected entries. Invalid mode shapes, names, maps, keys or values fail with starter-response-invalid (37) before materialization. Unknown entry fields cannot change repository, branch, mode offers or core state.

The core shallowly overlays the selected maps on shared links/meta, with selected values winning; null metadata replaces a value, without deleting its key. It then binds effective private keys to the Starter provider/current source instance. Unselected context and the modes map do not persist.

## Automatic Linker phase

At start:finalized, an automatic Linker is silently skipped when its output key already exists in the initial Work links. Private keys are compared using the actual provider/current source-instance storage identity. This happens before subscription diagnostics and input resolution, without invoking the component or producing a run receipt. Manual work link can still replace a known value.

All Linker eligibility and projected inputs are fixed before that phase starts. One Linker's output cannot enable another during the phase or suppress another initially eligible Linker for the same missing key. Importers evaluate afterward and may consume Starter-published or discovered context. Existing activation, subscription, filter and required-input checks still apply to missing-key Linkers.

The Python scaffold demonstrates all four protocols without running anything during management validation. Adapt behavior and test actual invocation separately; parser acceptance alone cannot prove runtime behavior.
