# Data Model: Marketplace Plugin Distribution

**Authority:** ADR-0022–ADR-0030 and ADD-0001 §§4–6. ADR-0030 governs the clean pre-release cutover. These are the current target shapes; no prior development format is supported or converted.

## 1. Provider identity and package address

Every installed provider has `(marketplace, plugin)` identity. Components and conventions add a logical contribution name and retain actual owner identity. Render plugin@marketplace or plugin@marketplace/contribution; public selectors are not filesystem paths. Use one leaf identity/address type rather than a union of current and previous providers. Each package resolves under plugins/<validated-marketplace>/<validated-plugin>/source. Name and reference rules are in contracts/provider-identity.md.

## 2. Registered source configuration — schema 1

Config contains schema:1, workspace, repository_roots, repository_resolution, and marketplaces. Each user-managed source is `{registration_id,name,kind,source,branch?}`; kind is git/local. Git requires a recorded branch and local forbids it. A source registration generates a random 32-lowercase-hex registration_id; idempotent add/refresh retains it and remove/re-add creates another. The builtin provider uses stable code-owned token builtin and is not replaceable configuration.

This source instance binds current remembered choices/private values across namespace reuse; it is continuity evidence, not a trust token. Source comparison also checks actual kind/normalized source/branch. Transport data is private; output uses a safe projection. Policy stores full qualified Locator references. Config-file existence never means policy has been handled.

## 3. Discovery state — schema 1

state/marketplace-catalogs.json is `{schema:1,catalogs:[...]}`. Each catalog stores registration_id, namespace, optional display_name/description, exact source descriptor, revision, catalog_digest/digest_algorithm, nullable capture_digest, and complete validated entries. Entries store name/path/description, manifest version, package digest/algorithm, and complete static contributions. Derived metadata is never a separate author-editable authority.

Git revision is the exact commit; local revision is a work-capture-v1 digest; builtin projects current distribution resources. A mismatched source/registration makes stored discovery unavailable rather than usable from another source. Ready/missing catalog state is separate from the latest action result; a failed refresh preserves a valid stored offer.

Captures live at state/marketplace-snapshots/<64-lowercase-hex>. Public digests retain sha256:, while storage uses its validated hex payload without a colon. Complete capture objects are referenced by catalog state and active process-held plan leases. No source/cache directory is an execution root.

## 4. Registry and installed metadata — schema 1

Registry is `{schema:1,packages:[],components:[],conventions:[]}`. Package stores composed provider, derived public ID/name/marketplace, install_mode copy, enabled, opaque version, content digest/algorithm, and actual provenance. There are no alias/origin-mode variants.

Provenance is `{registration_id,kind,source,branch?,marketplace_name,plugin_name,path,revision,catalog_digest}`. Kind is git/local/builtin, not installation mode. Path may move in a candidate while source/entry identity remains; source instance, descriptor and names prevent redirection.

Components retain provider/name/role and static role-specific fields, with paths under the provider's managed source root. Conventions retain provider/name/prefixes; equal names are distinct records with independent ownership. Registry revision baselines include provider, provenance, and declarations.

.install-meta.json schema 1 lives in the owned package wrapper and records actual provider/provenance/digest facts. Package digest covers source content, not wrapper metadata. Owned-root digests protect journal moves and reviewed removals. Missing content is an inspection error/state, never permission to fabricate or repair it on read.

## 5. Convention memory — schema 1

Store `{schema:1,entries:[{identity,reference}]}`. Repository identity rules remain unchanged. Reference is `{registration_id,marketplace,plugin,name}`; public presentation is plugin@marketplace/name with availability/reason. Source removal preserves memory as unavailable. A replacement source with equal names and another registration_id cannot satisfy it; an explicit convention set changes the chosen reference after baseline validation.

## 6. Current administrative control and journal — schema 1

Use one control `{schema:1,epoch,status,transaction_id}` and one current journal schema. Missing state is the fresh default snapshot; malformed or unsupported present state returns an ordinary invalid-state/recovery diagnostic, without special earlier-format recognition, fallback, conversion, or deletion.

The journal stores id/operation/phase, finite file images, typed marketplace package moves, and owned artifact references. Images preserve exact existence/bytes/digest. The finite allowlist is config/registry/memory/lifecycle/catalogs; no arbitrary writes. Moves store provider address, removal bit, validated transaction-owned staging/backup references, and before/after owned-root digests. Unknown images, addresses, and unsupported schemas are rejected by the same current validator.

Lifecycle is `{schema:1,initialized,policy_handled,removed}` outside removable package storage. Fresh defaults are all false. Required first setup seeds policy only when policy_handled is false, then sets it true atomically with default installation. Every explicit policy choice, including empty replacement, marks handled in its mutation; this is a meaningful state change even if the Locator array was already empty. Source registration does not change the flag. Explicit reference add/reinstall marks handled without inserting Locators. Removed records deliberate default removal and prevents resurrection; a new binary offer does not replace installed copies.

## 7. Frozen plans and lifetimes

Source plans freeze registration/source/namespace/prior catalog facts and the prepared offer. Install/reinstall plans also freeze entry/manifest/digest and affected registry/dependency facts. Removal plans freeze complete targets, owned-root digests, dependencies, and source/catalog facts. Update groups own one candidate source snapshot per source/branch and separate per-target outcomes.

Every mutating preparation closes idempotently. Separate per-plan process-held lease records protect active capture/stage objects; cleanup tries their locks nonblockingly under administration. Close releases the lease before taking administration for record cleanup. Current committed references always retain captures. Checks use OS temporary storage and never persist leases.

| Transition | Effect |
| --- | --- |
| Fresh → registered | Config and first catalog commit together; no plugin or policy effect |
| Registered → same add | Unchanged bytes/epoch; registration instance retained |
| Registered → refreshed | Whole validated discovery offer changes; installed copies unaffected |
| Entry → installed | Selected managed copy, metadata and declarations publish together |
| Installed → reinstall | Exact content unchanged; changed content requires explicit replacement review |
| Installed → update | Different opaque version; apply exact reviewed candidate |
| Entry withdrawn | Offer changes; installed copy survives |
| Source → removed | Config/catalog/all its copies and approved policy references commit together |
| Prepared failure → recovery | Restore exact before state for this current-format transaction |
| Committed failure → recovery | Complete approved after state and report committed cleanup honestly |

## 8. Work state and scope

New Work provenance and remembered contribution values are fully qualified. Private stored keys bind the source registration instance inside the existing meta/link maps; public semantic keys and subprocess envelopes remain provider-independent. Use the existing Work snapshot/SQLite structure where it fits; no older-development-data reader, version-preservation constraint, or data migration is required. Existing current-contract Works are preserved by package/source removal as an ordinary product guarantee.
