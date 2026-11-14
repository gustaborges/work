# Included plugins

`seed/` holds the plugins distributed inside the `work` binary. They are offered
by the code-owned `builtin` marketplace (`seed/marketplace.json`) and installed
through exactly the same validate, copy and publish pipeline as any other
offered plugin: there is no embedded install mode and no alternate layout.

- `manifest/plugin.json` declares the `work-reference` plugin; `starter/` and
  `locator/` are its Go sources, compiled per release platform into `dist/`.
- A fresh home receives `work-reference` once, as a default. Disabling or
  removing it is remembered: later setups and upgrades never restore it.
- The `builtin` source itself cannot be removed or replaced. Users may remove
  its plugins individually and add them again with
  `work plugin add work-reference@builtin`.

## Adding another included plugin

1. Add the package under `seed/` and an entry to `seed/marketplace.json` whose
   `path` stays inside the package (no absolute paths, `..` or symlinks that
   escape).
2. Extend the seed build so every release platform gets its binaries.
3. Run `make seed-all` and `go test ./seed/...`.
