# Marketplaces and plugins

Plugins are distributed only through **marketplaces**. A marketplace is a Git
repository or a local directory with a `marketplace.json` catalog at its root.
Registering one only records where it is; installing copies one offered plugin
into independent storage under `~/.work/plugins/`.

## Identity

Every plugin is `plugin@marketplace`. The marketplace name is the namespace the
catalog publishes in its `name` field; it cannot be chosen or changed locally.
`builtin` is reserved for the code-owned provider that ships with the binary
and offers `work-reference`. The same plugin name in two marketplaces is two
different plugins.

## Using marketplaces

```sh
work plugin marketplace add https://example.org/team/plugins.git [--branch main]
work plugin marketplace add ./my-marketplace          # local directory
work plugin marketplace list                          # sources and local totals
work plugin marketplace upgrade [name]                # refresh catalogs only
work plugin marketplace remove name [--yes] [--remove-policy-references]

work plugin list --available --json                   # stored offers, offline
work plugin add github@team                           # copy one offered plugin
work plugin update --check [plugin@marketplace...]    # pure check
work plugin update plugin@marketplace --yes           # apply the reviewed candidate
work plugin remove plugin@marketplace --yes
```

- **Add vs. refresh vs. update.** Registering reads a catalog and installs
  nothing. `marketplace upgrade` refreshes discovery only. `plugin update`
  compares each installed plugin with its recorded source and applies the exact
  candidate that was reviewed. Versions are opaque and compared only for
  equality.
- **Reinstall.** `plugin add` on an installed plugin replaces it when the
  offered content differs, even at the same version. Enablement is kept.
- **Removal.** Removing a marketplace removes its registration, stored offer
  and every plugin installed from it, including disabled ones, in one reviewed
  transaction that works offline. The included marketplace cannot be removed.
- **Policy references.** Dropping repository-policy entries needs
  `--remove-policy-references`; `--yes` never grants it.

`work plugin` opens the interactive manager on Installed and
`work plugin marketplace` opens it on Marketplaces; both reach the same
screens and the same reviewed operations as the commands above.

## Default lifecycle

A fresh home installs `work-reference` once, through the normal pipeline.
Registering a marketplace first, choosing an explicit (even empty) repository
policy, disabling or removing the default are all remembered and never
overridden by later runs or upgrades.

## Writing a marketplace

See `examples/marketplaces/single` and `examples/marketplaces/multiple`:

```json
{
  "schema": 1,
  "name": "team",
  "display_name": "Team Plugins",
  "plugins": [
    {"name": "github", "path": "plugins/github", "description": "GitHub origin"}
  ]
}
```

Each `path` is relative, stays inside the marketplace and points to a package
with a `plugin.json`. The package name must match the entry name. Packages are
copied whole, so keep them self-contained.

## Official `work` marketplace

The Work project publishes one catalog, `work`:

```sh
work plugin marketplace add https://github.com/gustaborges/work-cli-marketplace.git
work plugin add github@work       # pull request / issue starters, linkers, importers
work plugin add wsp@work          # repository locator backed by `wsp find`
```

- **Origin:** `https://github.com/gustaborges/work-cli-marketplace.git`, branch
  `master`, namespace `work`. This is the only source that may be called
  official: its normalized Git descriptor must equal the origin and its catalog
  must declare `work`. A catalog that merely names itself `work` is an ordinary
  source, and the CLI still presents every registered source the same way.
- **Explicit only.** Nothing registers it for you, and removing it never
  reinstalls it. `builtin` is unaffected either way.
- **Contents:** `github` (pull request and issue starters, linkers and importers) and `wsp`
  (repository locator backed by `wsp find`). The catalog lives only in the origin
  repository; this repository keeps no copy.
- **Publishing a change:** edit the packages in the origin repository through a
  branch and pull request, and bump the plugin `version` when behavior changes
  (installed copies only see an update when the version differs).
