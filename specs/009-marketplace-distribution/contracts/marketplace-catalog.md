# Contract: Catalogs, Sources, Fixed Content, and Digests

**Governing decisions:** ADR-0022–ADR-0023, ADR-0027–ADR-0030. **Stories:** US1–US3. **Requirements:** FR-001–FR-005, FR-007–FR-010, FR-014–FR-017, FR-020, FR-025–FR-026.

## Published schema 1

```json
{
  "schema": 1,
  "name": "team",
  "display_name": "Team Plugins",
  "description": "Team integrations",
  "plugins": [
    {"name": "github", "path": "plugins/github", "description": "Pull request context"}
  ]
}
```

Root `marketplace.json` requires schema integer 1, name, and plugins array (empty is valid). Entry requires name/path; description is optional string. display_name is optional string, trimmed for presentation and nonempty after trim. Unknown fields, duplicate JSON keys at any depth, nulls where fields are typed, duplicate entries, future schemas, malformed complete manifests, and manifest-name mismatch reject the entire offer. Optional descriptions default to empty strings for canonical digest purposes. Ordering of object keys/entries does not affect semantic offer identity.

Names follow provider-identity.md. Paths are relative POSIX subdirectories, not empty, root `.`, absolute, URL, drive/UNC, backslash, NUL/control, empty segment, dot/dot-dot, or `.git`/Work-owned metadata paths. Segments must be portable; detect case-equivalent and prefix-overlapping package roots. Catalog root/entry ancestors may not be symlinks. A package can contain relative links only when fully resolvable inside itself without cycles, escapes, special files, or external entrypoints. Validate every static manifest; missing runtime does not prevent discovery but fails selected installation. No hooks, builds, execution, submodules, external artifacts, or dependency fetch are performed.

Only explicitly listed subdirectories are distributable entries. Root plugin.json is never a fallback. A single-plugin author uses this same minimum catalog and subdirectory layout.

## Source contract

| Kind | Registration/refresh | Install selected offer | Explicit maintenance |
| --- | --- | --- | --- |
| git | Resolve optional/default branch once, record its exact name, validate a fixed commit | Materialize that exact commit/entry, retrieving if needed; fail without tip fallback | Resolve one current commit per stored source/branch group; validate complete catalog and exact namespace/entry |
| local | Normalize absolute source path; capture catalog and packages coherently without requiring Git | Copy only from published captured bytes, offline | Capture a new temporary source view without publishing discovery |
| builtin | Binary-owned reserved provider; project current embedded offer, no user registration | Read embedded distribution assets and use common validation/copy | Compare current binary offer to installed entry |

A local checkout path means live-directory capture, including uncommitted content; `file://` explicitly selects committed Git transport. --branch is Git-only. Discover the default branch only on initial registration. Later default-branch changes do not switch the stored branch. No shorthand hosting syntax, arbitrary tag/commit channels, sparse checkout, or external artifacts are introduced.

Git/source equivalence uses the stored normalized transport descriptor and branch; do not infer HTTPS/SSH or differently spelled URLs equivalent. Local paths become absolute/cleaned without treating a checkout as Git. Re-registering identical kind/source/branch/name preserves registration_id and is unchanged without refresh. First registration generates a new instance token; removal and re-registration generates another, preserving the rule against historical reassociation. Any changed namespace during refresh fails; explicit remove/register is required, with whole-source effects.

`builtin` is reserved by code-selected provider authority, not an author catalog field. `work` is ordinary Git; its name alone proves no official ownership. Only owner-confirmed origin metadata enables official attribution. No official source is auto-registered.

## Local capture and selection

Read/validate/copy the catalog and every listed package into private staging, then re-enumerate and compare content/entry inventories against the source. Any observed difference fails rather than publishing mixed content. Exclude `.git`, `.install-meta.json`, Work administration/staging artifacts, and unlisted siblings from distributable package content. Reject these reserved artifacts if a catalog attempts to name them as package roots. Publish a complete capture at its fixed digest before referencing it. A changed source after publication cannot change the selected offer. Capture directory names contain only the validated 64 lowercase digest hex characters; sha256: remains the public/persisted representation. Lease/object references carry that typed digest rather than arbitrary path text. No snapshot directory is an execution root.

Selected install fixes source descriptor, catalog revision/digest, entry name/path, complete manifest, and package digest. Any relevant administrative drift conflicts at apply. Source missing or content unavailable never selects a replacement by name/tip. Local source disappearance after capture does not prevent install from that capture. Git catalogs promise offline browsing, not offline installation.

## Canonical digest algorithms

Use SHA-256 and lowercase hex with `sha256:` prefix. Persist `digest_algorithm` with each target-format record. This defines the current algorithm; no earlier-development digest interpretation is retained.

`F(bytes)` is ASCII decimal byte length, `:`, then those exact bytes; decimal has no leading zeros except `0`. All strings are UTF-8; paths are normalized POSIX. Length framing avoids escaping ambiguities.

- **work-package-v1:** header `work-package-v1\n`, followed by sorted relative entries excluding root itself. Each entry is `F(path) F(kind) F(exec) F(content)`, concatenated without intervening spaces. Kinds are dir/file/link. dir content is empty and exec is `0`; file content is raw bytes and exec `1` iff source declares executable; link content is its exact validated relative link text, exec `0`. Normalize permissions to this semantic bit, not OS-specific mode values. Git supplies its tree executable bit, included assets supply recorded modes, and local capture records the source file's executable bit (Windows local files use `0`, relying on native extension/runtime rules). The same recorded bit follows preparation on hosts that cannot represent it. Exclude only reserved distribution metadata, never arbitrary author dotfiles. Include empty directories for local packages; Git naturally has none.
- **work-catalog-v1:** header `work-catalog-v1\n`, then F(schema `1`), F(name), F(trimmed display_name or empty), F(description or empty), F(entry count), then each entry sorted by name byte order as F(name), F(path), F(description or empty), F(package digest). Full manifest bytes are already covered by package digest; offered version/contributions are derived from them.
- **work-capture-v1:** header `work-capture-v1\n`, then F(catalog digest), F(entry count), then sorted entries F(name), F(path), F(package digest). This fixes the catalog and every distributable package without including source/config/transport secrets.

Each plugin digest excludes siblings, marketplace metadata, provenance, and config. Catalog/capture digests change for meaningful display metadata or package changes. Git revision remains the exact commit; catalog digest distinguishes semantic no-op from a new commit with the same offer. A refresh whose canonical offer is identical keeps the old published revision/bytes/epoch and returns unchanged.

[Digest vectors](digest-vectors.json) fix the byte-framing and domain separation. Implementation adds filesystem, executable-bit, Unicode, link, manifest-formatting, cross-platform, and sibling-isolation tests; no digest is a signature or per-invocation rehash promise.
