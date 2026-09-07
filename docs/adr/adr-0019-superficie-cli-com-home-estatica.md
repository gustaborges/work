# ADR-0019: Progressive CLI Surface with Static Home and Grouped Help

**Status:** Accepted

**Date:** 2026-09-07

**Supersedes:** ADR-0017 — Progressive and Consistent CLI Surface

**Product context:** `docs/prd.md` — journeys 7.1 to 7.7, RF-11 to RF-20, RF-25, RF-30, RF-31, RF-38, RF-45, RF-50 to RF-64, and RNF-10 to RNF-11

**Governs:** `docs/add/add-0001-work-system-architecture.md`, Sections 5, 6, 7.2, 8, and 12

**Realized by:** `docs/add/add-0001-work-system-architecture.md` §12

## Context

ADR-0017 correctly established a shallow human surface, interactive hubs for administration,
and a regular direct API. Its selectable home, however, mixes discovery with execution: it
retains a large interactive frame in the history, duplicates the live command inventory, and
requires a selection flow even for those who merely need to recall the syntax.

The product objective remains the same — recover without memorizing the command tree — but
the home TUI is no longer the appropriate mechanism. Work needs to present its own identity,
direct users to complete help, and keep each journey directly invocable, without undoing the
grammar and automation guarantees already accepted.

## Decision

Maintain the grammar, everyday commands, administrative hubs, and cross-cutting semantics
of ADR-0017, replacing only the home decision:

* In an interactive terminal, `work` without arguments renders static output with the `WORK`
  terminal art wordmark, tagline, and direction to `work --help`; does not open a selector
  and exits 0.
* On terminals with true color and adequate contrast, the wordmark uses a gradient from the
  primary accent `#11A8CD` to the secondary `#8B7CF6`. Narrow, monochromatic, or non-interactive
  outputs use `WORK` in compact, legible form.
* `work --help` is the source of discovery: includes mark, usage, options, and exactly the
  commands registered in the binary, grouped by context. Empty groups and unimplemented commands
  do not appear.
* The minimal taxonomy distinguishes everyday/global commands, commands dependent on a materialized
  Work, administration, and setup/plumbing. The classification does not alter the grammar of commands.
* In non-interactive mode, `work` without arguments preserves the usage failure and exit code 2;
  `work --help` exits 0. Neither path emits interactive controls when relevant streams are not TTYs.
* Remain valid the everyday commands `start`, `resume`, `archive`, `status`, `import`, and `link`;
  the interactive hubs `plugin`, `repository`, and `convention`; the absence of aliases; the purity
  of reads; `--json`; `--yes`; and the stable contracts of stdout, error, and exit code.
* Every public journey remains directly invocable by documented command. Home and help facilitate
  recognition; they do not become an alternate execution path.

### Preserved human surface

```text
work
work --help
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

Without a target and with an interactive terminal, `start`, `resume`, `archive`, `import`, and
`link` collect or offer the necessary choices. Explicit targets skip only the corresponding selection.
`status` without a target uses the Work associated with the current directory. In non-interactive use,
missing mandatory value fails with actionable usage, without opening interactive controls.

### Preserved direct API

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

Terms before the verb continue to be resources in the singular; reads use `list` for collections
and `show` for a single value; `replace` replaces an entire collection. `plugin update --check`
is read-only; `plugin update` without a target opens selection; explicit names or `--all` determine
the set to update, with applicable confirmation. `plugin install --link` requires a local path.
`work init` and aliases continue outside the public API.

## Alternatives considered

* **Keep the home TUI and fix only its final frame.** Rejected because it would continue duplicating
  discovery and execution, and require interaction to consult the product.
* **Make the static home list all commands.** Rejected because it would duplicate the inventory
  that already belongs to help and could diverge from it.
* **Print only the complete help on `work`.** Rejected because a brief entry preserves identity
  and orientation without dumping information when the user merely tests the command.
* **Alter the administrative grammar along with it.** Rejected because the observed problem does
  not invalidate the shallow depth of recall or the verbs and resources already accepted.

## Consequences

**Positive:** discovery and execution are separated; help does not announce non-existent commands;
the empty command leaves short history; visual identity appears without introducing a second
navigation tree; scripts preserve the detectable behavior of the empty command.

**Negative / trade-offs:** starting a journey from the empty command requires consulting help and
executing the indicated command; brand rendering needs responsive forms without color; F1/F2
documentation describing the old home remains historical and must be explicitly superseded by
the contract of this feature.
