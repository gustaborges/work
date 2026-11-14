# Plugin manifests

Read [generated manifest rules](generated/manifest-schema.md) and the [complete scaffold](../templates/marketplace/plugins/example/plugin.json). Name must match the listing; version is an opaque nonempty string. Components and conventions belong to one atomic plugin package. Runtime is optional for directly executable entrypoints; portable text scaffolds declare python3 and require it installed on the consumer's PATH. Entry points are invoked directly, without a shell; do not include a shell command line in entrypoint.

Manifest JSON decoding differs from catalog strictness: duplicate object keys overwrite, and null component/convention collections decode empty. Unknown keys and fields forbidden for a role fail. There is no invocation, capabilities, hooks, priority or score field. Do not invent schema fields.

Conventions provide named prefix templates using {slug}. Starter selection uses pattern or fallback; competing fallback providers require separate selected-install validation homes. Bump version on behavior changes. Validate complete discovery and selected installation separately.
