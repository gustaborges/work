# Contract: Provider Identity, References, and Private Data

**Governing decisions:** ADR-0024 for composed identity and ADR-0030 for the clean pre-release cutover. **Stories:** US2, US4. **Requirements:** FR-004, FR-006–FR-007, FR-018–FR-019, FR-024–FR-026.

## Identity and names

A qualified plugin is the typed pair `(marketplace, plugin)`, rendered `plugin@marketplace`. A component/convention adds its logical name, rendered `plugin@marketplace/name`. Persist those fields separately; never obtain a filesystem path by splitting unchecked display text. No global plugin alias is available.

New marketplace and entry names are ASCII single segments matching `[A-Za-z0-9][A-Za-z0-9._-]*`, at most 64 bytes, not ending in `.` or `.old`; reject ASCII case-insensitive Windows device basenames CON/PRN/AUX/NUL/COM1–COM9/LPT1–LPT9 before the first dot. Reject `@`, colon, slash/backslash, percent, whitespace, controls, dot/dot-dot, and names beginning with a dot. Comparisons detect ASCII case-fold equivalence without correcting author spelling. `builtin` is reserved case-insensitively. Each filesystem segment is validated separately; journal backups use encoded typed addresses.

A bare component/convention selector is usable only if exactly one enabled eligible provider matches; ambiguity is explicit. New policy selections store the full qualified contribution, even when initially unique. Memory and installed provenance also bind the source registration_id. User-managed source instances receive 32 lowercase hex characters; builtin uses stable token builtin. Identical add/refresh retains it, remove/re-add creates another. Equal public names with another registration_id cannot satisfy retained memory or private lookup.

## Convention ownership and references

Conventions are keyed by `(provider, convention-name)` and retain independent prefixes; equal names are not shared mutable catalog records. Enablement, withdrawal, review impact, and memory availability use actual ownership. Repository identity rules remain unchanged. Removing its owner leaves unavailable memory; another registration with equal public names cannot satisfy it.

New Work Starter, branch-convention, and operational source-component values are fully qualified strings. Use one current Work/provenance shape; no earlier development string format or snapshot version imposes compatibility obligations. Package/source removal preserves Works created under the current contract.

## Starter subscriptions

For new marketplace packages, resolve each declared filter as follows:

1. `plugin@marketplace/component` matches exactly that typed provider/component.
2. A bare name declared by a Starter in the subscribing package binds to that package's exact provider.
3. Other bare names match only one enabled eligible Starter globally. No match is inactive; multiple matches produce an ambiguity diagnostic and no execution of that subscriber.
4. Explicit external qualification uses plugin@marketplace/component; alias/name is not part of the manifest reference grammar.

Resolution needs registry context; string equality against the selected Starter alone cannot determine ambiguity. No-filter subscriptions retain their any-Starter behavior. Role/event/activation/input semantics do not otherwise change.

## Private key grammar and translation

`enc(name)` preserves valid name bytes except `.` becomes uppercase `%2E`. No other escape is canonical because `%` and `@` are invalid source-name bytes. Reject lowercase escapes, unnecessary escapes, double encoding, and decoded invalid names. Local key suffixes retain `[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*`.

| Context | Form | Example |
| --- | --- | --- |
| Author-relative declaration or publication | `plugin.<enc(plugin)>.<local>` | `plugin.acme%2Etools.build_id` |
| Logical qualified input | `plugin.<enc(plugin)>@<enc(marketplace)>.<local>` | `plugin.acme%2Etools@team%2Edev.build_id` |
| Core-only persisted key | `plugin.<enc(plugin)>@<enc(marketplace)>~<registration_id>.<local>` | `plugin.github@team~0123456789abcdef0123456789abcdef.build_id` |

The first dot after `plugin.` ends the encoded owner token; exactly one raw `@` separates the qualified pair. In core storage only, a single ~ terminates the marketplace token and precedes the registration_id; author declarations, publications, and logical inputs reject this storage-only suffix. Persisted-key inspection uses its own validator, not the author grammar. Decode before typed identity comparison. This explicitly extends private-key capability to dotted plugin names; existing dot-free relative declarations stay byte-for-byte valid.

A new marketplace provider publishes only public semantic keys or its own relative private keys. Validate decoded owner against its actual manifest name, then bind through the actual installed provider. Reject qualified private publications, including an attempt to write another namespace. Public keys and their last-writer semantic behavior are unchanged.

An unqualified private input in a new marketplace package must decode to that declaring plugin name; meta:plugin.other.local is invalid in github and requires explicit qualified syntax.

For `meta:plugin.github.build_id`, core lookup uses the executing provider's recorded registration-instance stored key and the subprocess receives the declared `plugin.github.build_id` key. For `meta:plugin.github@personal.build_id`, lookup binds to the currently registered instance of exactly that provider and projects the declared key unchanged. Never search another registration instance or unbound value. Source remove/re-add does not make private values from the earlier instance visible to the replacement. Link inputs behave equivalently. Missing required values make the component ineligible; absent optional values are omitted. No new IPC envelope or undeclared inputs are introduced.

Tests cover canonical encoding and dotted names, homonymous publishers, ownership rejection, relative/qualified inputs, optionality, stable payload keys, public-key behavior, foreign-owner relative-input rejection, and source-instance non-rebinding. There is one current author/input/storage grammar and no compatibility form.

## Execution baseline

The invocation guard compares the selected provider record, provenance, declaration, and revision, not a bare alias or name. It retains administration through the subprocess, releases it before Work locks, and revalidates subsequent publication as before. Shared semantic Linker keys do not merge provider identity. Exclusive fallback conflicts still name both complete providers.
