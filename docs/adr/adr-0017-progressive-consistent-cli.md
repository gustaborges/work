# ADR-0017: Progressive and Consistent CLI Surface

**Status:** Superseded by ADR-0019

**Superseded by:** ADR-0019 — Progressive CLI Surface with Static Home and Grouped Help

**Date:** 2026-09-04

**Product context:** `docs/prd.md` — journeys 7.1 to 7.7, RF-11 to RF-20, RF-25, RF-30, RF-31, RF-38, RF-45, RF-50 to RF-52, and RNF-10

**Governs:** `docs/add/add-0001-work-system-architecture.md`, Sections 5, 6, 7.2, 8, and 12

## Context

Work delegates difficult choices to the TUI so that starting, resuming, and archiving work does not require memorizing a command tree. The proposed surface, however, used different grammars in each administrative area: `policy` without a verb, `locator list`, `roots list`, `plugin outdated`, `plugin update <name>`, and `convention set`. Forcing the entire tree into uniformity would produce either many top-level commands or flags that function as verbs, trading syntactic depth for ambiguity.

It is necessary to separate human language, discovered through recognition, from the direct API used by scripts, keeping both grounded in the same operations.

## Decision

Adopt **low recall depth**, not zero syntactic depth:

* `work` without arguments opens a TUI home from which all journeys are reachable.
* Everyday commands stay at the first level: `start`, `resume`, `archive`, `status`, `import`, and `link`.
* `plugin`, `repository`, and `convention` are interactive hubs. Without a subcommand, they show the domain TUI and make visible the direct command equivalent to the completed operation.
* Administrative subcommands form the automation API and follow `work <resource path> <verb>`: the verb comes last and each preceding term is a resource in the singular. Reading uses `list` for collections and `show` for a single value; `replace` substitutes an entire list.
* No official aliases exist for commands or resources. Completion reduces typing without duplicating the public vocabulary.
* `work init` is not part of the public API. Bootstrap and initial configuration happen on demand; later configuration is reachable through the TUI hubs.

### Human surface

```text
work
work start [SOURCE]
work resume [WORK]
work archive [WORK...]
work status [WORK]
work import [IMPORTER]
work link [LINKER] [VALUE]
work plugin
work repository
work convention
```

Without a target and with an interactive terminal, `start`, `resume`, `archive`, `import`, and `link` collect or offer necessary choices in the TUI. Explicit targets skip the corresponding selection. `status` without a target uses the Work associated with the current directory; outside a Work it requires a target. On non-interactive stdin, any missing required value fails with actionable usage, without attempting to open the TUI.

### Direct API

```text
work plugin list
work plugin install <SOURCE> [--link] [--as <ALIAS>]
work plugin enable <PLUGIN...>
work plugin disable <PLUGIN...>
work plugin update
work plugin update --check [PLUGIN...]
work plugin update <PLUGIN...>
work plugin update --all
work plugin uninstall <PLUGIN...>

work repository locator list
work repository policy list
work repository policy add <LOCATOR> [--before <LOCATOR> | --after <LOCATOR>]
work repository policy remove <LOCATOR...>
work repository policy move <LOCATOR> (--before <LOCATOR> | --after <LOCATOR>)
work repository policy replace <LOCATOR...>
work repository root list
work repository root add <PATH...>
work repository root remove <PATH...>
work repository root replace <PATH...>

work convention show
work convention set <CONVENTION>
```

`plugin update --check` is read-only; without explicit plugins, it checks all installed ones. `plugin update` without a target opens interactive selection; with explicit names it updates those plugins; `--all` updates all outdated ones after confirmation. `--check` and `--all` are mutually exclusive.

`plugin install <SOURCE>` accepts remote origin or local path. `--link` requires a local path and chooses the development link instead of a pinned installation; the flag describes the effect, not just the origin.

### Transverse semantics

* Flags modify operations; `add`, `remove`, `move`, `replace`, `install`, and `uninstall` remain verbs.
* `--yes` confirms already-determined impacts and never chooses target, mode, or value on the user's behalf.
* Read commands (`status`, `list`, `show`, and `update --check`) do not alter recent access, configuration, checkout, or provenance.
* Every read command accepts `--json`. Mutations have stable output and exit codes for automation.
* `remove` takes an item out of a collection; `uninstall` removes a package; `archive` ends a Work preserving its state; `replace` substitutes an entire collection.

## Alternatives considered

* **Keep the previous surface.** Rejected because each branch required learning different defaults, pluralization, and verbs.
* **Flatten every operation into top-level commands.** Rejected for polluting everyday vocabulary and erasing the relationship between resource and operation.
* **Express administrative mutations through flags.** Rejected because flags like `--add-root` and `--remove-locator` would function as disguised verbs, with worse composition and help.
* **Offer short aliases like `repo`.** Rejected because completion resolves typing cost and a second name expands the recognizable and documentable surface.
* **Keep `work view` only for links.** Rejected because `view` does not convey what will be displayed and limits the natural evolution of a Work summary. `work status` includes core state and links without running extensions.

## Consequences

**Positive:** a person can forget the entire tree and recover with `work`; everyday commands remain shallow; the non-interactive API gains regular grammar and stable outputs; the TUI teaches the CLI through recognition.

**Negative / trade-offs:** administrative operations may reach three levels after `work`; `work view`, `plugin outdated`, `repository roots`, and `work init` cease to exist before v1, requiring documentation and completion to use only canonical forms.
