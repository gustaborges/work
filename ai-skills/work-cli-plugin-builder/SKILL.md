---
name: work-cli-plugin-builder
description: Create, edit, review and validate Work marketplaces, plugin manifests, all four component roles and branch conventions; prepare Git publication and consumer installation.
---

# Build a Work marketplace

Read the existing marketplace.json and plugin.json files before editing. Clarify the author's intended behavior and select a component role before producing JSON. Keep each plugin a cohesive, self-contained maintenance unit. Guidance matches the Work binary that delivered this copy; refresh explicitly after binary upgrades.

1. Inspect the catalog and choose portable, unique marketplace/plugin/contribution names. Read [marketplace catalogs](references/marketplace-catalog.md).
2. Choose Starter, Repository Locator, Importer or Linker using [component roles](references/component-roles.md). Do not add a new runtime role for agent skills.
3. Copy the [complete marketplace scaffold](templates/marketplace/marketplace.json), preserving plugin package containment. Adapt the [manifest](templates/marketplace/plugins/example/plugin.json) and declared-runtime entrypoints together. Read [plugin manifests](references/plugin-manifest.md).
4. Implement role-specific stdin/stdout behavior. Never invent manifest fields or run a component just to validate catalog discovery or installation.
5. After each source edit, recapture in fresh isolated homes and validate **every** catalog entry, separately. Follow [validation and publication](references/validate-and-publish.md). Registration alone is insufficient; later entries can fail selected installation.
6. Bump the opaque plugin version when behavior changes. Review diffs, publish the marketplace Git repository and give consumers the explicit `plugin@marketplace` install identity. Do not publish temporary validation homes or local coordination files.

Use the generated [catalog schema](references/generated/catalog-schema.md), [manifest schema](references/generated/manifest-schema.md) and [role matrix/protocol](references/generated/role-matrix.md) for the represented contract. These do not replace semantic examples and validation. Never silently override local user edits or claim validation succeeded without running it. Explain unavailable prerequisites and failures precisely.
