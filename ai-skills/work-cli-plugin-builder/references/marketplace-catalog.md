# Marketplace catalogs

Use [generated catalog keys](generated/catalog-schema.md). A complete local source has marketplace.json at its root and self-contained plugin directories. The catalog lists each package's exact manifest name and relative path. Paths must stay inside the source, use portable slash-separated segments and cannot overlap. Names use the shared portable identity grammar, are unique under case folding and cannot be reserved device names or end in .old or a dot. `builtin` is reserved for the binary provider. A catalog merely named like the official marketplace does not gain official status: origin identity uses the exact normalized official descriptor.

Catalog parsing rejects duplicate keys at every depth, nulls, unknown fields and future schemas. An empty plugins array is valid. Full discovery also checks package names/containment/manifests. It does not prove selected entrypoint/runtime availability. See [validation](validate-and-publish.md).
