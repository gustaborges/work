# Work Semantic Conventions — version 1

Authority: PRD FR-35; ADD §4.2; ADR-0013; spec FR-008..FR-011 and decision D1;
`docs/roadmap.md` §5 ("before F5 there must be a first published version of the Semantic
Conventions"). This file **is** that publication. Plugin authors code against it; the core
enforces only §1–§3 and holds no list of the keys in §5 (ADR-0013: not a closed enumeration).

**Version**: 1 · **Status**: published with F5 · **Changes**: only by a new version (§6).

## 1. Key grammar

A key names the *meaning* of a datum, never its producer: two plugins that both know a pull
request's address publish the same key, and the producer is recorded separately as provenance.

```text
public key   = namespace "." segment *("." segment)
segment      = lower *( lower / digit / "_" )          ; lower = a-z, digit = 0-9
namespace    = segment, and not "plugin" and not "work"   ; both reserved
private key  = "plugin." plugin-name "." local
local        = segment *("." segment)
```

Examples: `github.pull_request`, `github.pull_request.number` (public);
`plugin.acme-tools.build_id` (private to the plugin named `acme-tools`).
Uppercase, hyphens, empty segments, a single segment, and any key starting `plugin.`/`work.`
that is not a valid private key are invalid.

## 2. Ownership of private keys

A private key may be published only by the plugin whose manifest `name` it carries, and
only names without a `.` can own private keys (a name such as `foo.bar` would make
`plugin.foo.bar.x` indistinguishable from plugin `foo`'s key `bar.x`; such a plugin may still
publish public keys). Reading a private key is not restricted. Use a private key for data
that is not an interoperability contract; use a public key when another provider could
supply the same meaning.

## 3. Values

| Section | Value | Enforced by the core |
|---|---|---|
| `links` | a **non-empty string**; SHOULD be an absolute URI identifying an external resource | non-empty string |
| `meta` | any JSON value; scalar, array or object | valid JSON |

Values are stored exactly as received: no trimming, normalization, or rewriting. One current
value per key; every publication replaces the previous one (last source wins). Do not
publish secrets; Work never repeats values in its diagnostics, but it does store them in the
Work's snapshot.

## 4. What the core does and does not check

The core checks §1 (grammar), §2 (ownership) and §3 (value shape) wherever it accepts a key
from a plugin: a Starter's `meta`/`links` (a violation fails `work start` before anything is
created: `starter-response-invalid`), a Linker's declared `key`, and the keys in `inputs`
(grammar only). It does **not** check that a public key appears in §5, and it does not
interpret what any key means. Conformance to a key's meaning and representation is the plugin
author's responsibility.

## 5. Public keys defined in version 1

| Key | Section | Meaning | Representation |
|---|---|---|---|
| `github.pull_request` | link | the pull request the Work relates to | absolute URL of the pull request |
| `github.issue` | link | the issue the Work relates to | absolute URL of the issue |
| `github.pull_request.number` | meta | the pull request's number in its repository | JSON integer ≥ 1 |

These are the keys used by the shipped fixtures and examples; every public key any shipped
fixture uses is in this table (SC-009). Fixture-only data uses private keys.

## 6. Evolving the conventions

- **Adding** a key is a new *minor* revision of this document (1.1, 1.2, …); older revisions
  stay valid.
- A published key's meaning and representation **never change**. A change of meaning is a new
  key; the old key is marked deprecated in the document and stays defined.
- A new namespace (a forge, a tracker) is added by the same process. The core needs no change
  for any of it.
- A **major** version is reserved for changing §1–§3 themselves and requires an ADR.
