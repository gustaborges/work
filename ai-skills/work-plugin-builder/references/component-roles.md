# Component roles and protocols

Read the [generated role/IPC contract](generated/role-matrix.md) and [manifest role constraints](generated/manifest-schema.md). Commands execute directly with JSON stdin, bounded stdout (1 MiB) and a stderr tail (4 KiB). Runtime must be present for selected installation. Installation/discovery never execute components.

- Starter: receives arg, returns repository reference (path, git_fetch_urls, name or query), optional base_branch/start_modes/meta/links. Patterned starters match arguments; a starter without pattern is fallback. Return real repository information.
- Repository Locator: declares accepts from name/git_fetch_urls/query, receives repository and repository_roots, returns matches containing repo_path. Return actual Git roots, not fabricated paths.
- Importer: declares on or manual, optional inputs. Receives inputs and output_dir. Write only under that provided directory. Empty stdout or one JSON object is accepted; returned object fields are ignored.
- Linker: declares owned key plus discover or manual. Receives resolved inputs. Empty stdout, {}, absent value or null value mean nothing found. A present value must be a nonempty string. Automatic discovery requires its event declarations to run.

Events currently include start:finalized. Starter restrictions use bare contribution names or plugin@marketplace/component. Inputs use work:key, meta:key or link:key with optional :optional. Work facts are limited to the generated list. Public semantic keys are dotted lowercase segments. Relative private publication/declaration keys use plugin.<encoded-plugin>.<local>; cross-provider input keys use plugin.<encoded-plugin>@<encoded-marketplace>.<local>. Encode dots in owner names as %2E. The ~registration suffix is core storage only. A plugin may publish only its own private keys. Never expose core-only state through undeclared inputs.

The Python scaffold demonstrates all four protocols without running anything during management validation. Adapt behavior and test actual invocation separately; parser acceptance alone cannot prove runtime behavior.
