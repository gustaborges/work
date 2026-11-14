# Research: Marketplace Plugin Distribution

**Date:** 2026-11-07
**Method:** Read-only checkout inspection, including the plan skill's storage and identity/UI research. No application tests or remote publication occurred. **Scope correction:** The owner explicitly rejects backward compatibility for the unreleased CLI. ADR-0030 replaces the initial compatibility assumptions; findings below justify one current model and removal of superseded branches.

## R1 — Require actual resolved-entry provenance

**Decision:** Marketplace resolution produces immutable source/namespace/entry/snapshot/digest facts; plugin administration can publish only this entry. Installer code materializes/validates content and has no standalone publishing API.
**Evidence:** plugininstall/{install,prepare,swap,link}.go currently accept direct sources; Prepared.ValidateCatalog checks installed collisions, not a marketplace offer. pluginadmin and bootstrap add publishing paths.
**Alternatives considered:** Parser-only rejection or optional marketplace strings on arbitrary prepared packages. Both preserve bypasses. Delete superseded publishers and update their callers/tests directly.

## R2 — One composed identity and real convention ownership

**Decision:** Use one leaf `(marketplace,plugin)` provider/address type, extended by contribution name. Registry, policy, memory, dispatch and guards use it; there is no alternate alias/earlier-provider type.
**Evidence:** registry currently uses Alias and flat paths; Convention has only Name/Prefixes and UpsertConvention overwrites homonyms. repoconv remembers bare strings, while repoconfig/locator/convention resolve them.
**Alternatives considered:** Qualify display only, put @ into Alias, or keep both provider types. They preserve ambiguity or unnecessary branches.

## R3 — One current storage and recovery format

**Decision:** Give current config/registry/memory/catalog/control/journal/lifecycle a strict schema-1 target shape. Reuse the finite transaction mechanism while replacing its flat alias moves and image model. No older-format reader, conversion, executable fence, version-preservation obligation, or upgrade fixture is required.
**Evidence:** Current config/registry/memory decoders are unversioned, while administration has schema 1 and four finite images. New catalog ownership and typed addresses require changing those models. The CLI is unreleased, so recreating development state is acceptable outside this task.
**Alternatives considered:** Dual decoding, conversion journals and old-executable tests. Rejected by user policy/ADR-0030. Ordinary current-schema validation and interruption recovery remain necessary.

## R4 — Protect live preparation from collection

**Decision:** Publish complete captures before references and use process-held per-plan leases for capture/staging ownership. Cleanup checks locks nonblockingly under administration and removes only unreferenced/unleased objects. Check candidates use OS temporary storage.
**Evidence:** recovery currently sweeps .admin-stage-* roots; preparation outside the lock otherwise looks orphaned. Age/PID alone cannot establish safe liveness.
**Alternatives considered:** Hold administration for long copy/network work or perform collection during list. These harm responsiveness or violate purity.

## R5 — Exact Git branch and revision primitives

**Decision:** Persist the chosen/default branch at registration, resolve once per source/branch group, and materialize exact browsed/reviewed commits without falling forward.
**Evidence:** gitx.Clone shallow-clones default HEAD and lacks branch persistence/exact old-object retrieval. CommandError can expose args/stderr and needs a safe projection.
**Alternatives considered:** Reclone current HEAD for install or infer branch again. Both change selected content.

## R6 — Explicit normalized digest domains

**Decision:** Use current named package/catalog/capture algorithms and reference vectors. Package hashes exclude siblings/config; journal owned-root hashes cover wrapper metadata separately. Store capture directories as validated 64-hex payloads, never raw sha256: strings.
**Evidence:** Existing digest helpers distinguish content versus owned roots but lack canonical offer/capture domains; raw colon-bearing paths are invalid on Windows.
**Alternatives considered:** Hash arbitrary JSON formatting/whole source or retain multiple digest interpretations. The first couples formatting/siblings to content; the latter serves only compatibility.

## R7 — Private scope and Starter subscriptions

**Decision:** Bind relative author/IPC private keys to encoded provider-instance storage keys. Explicit qualified inputs select actual providers, and foreign-owner relative input declarations are invalid. Self-package Starter filters bind exactly; unrelated bare filters require a unique eligible provider. Use one current grammar.
**Evidence:** semconv currently scopes only by manifest name; extension eligibility matches a bare Starter name globally. Two homonymous plugins otherwise share private data or activation incorrectly. ParseInput uses colon delimiters, so @/%2E fit the input shape.
**Alternatives considered:** Only qualify component IDs or concatenate dotted names. Neither resolves private ownership unambiguously.

## R8 — Current source continuity and Work provenance

**Decision:** Source registration IDs remain stable on refresh/idempotent add and change on remove/re-add; builtin has a stable code-owned ID. Retained convention choices/private values bind that instance. New Work/source-component values use full provider references. Current Works survive package removal; no prior-development decoder or format preservation is required.
**Evidence:** Equal public names can recur after source removal, but their provenance is different. Work/projection already carry identity strings. Runtime source-reuse protection is necessary independent of unreleased-format compatibility.
**Alternatives considered:** Match retained memory by public name only or carry an older-provider union. The first silently rebinds; the second adds an unneeded alternate runtime contract.

## R9 — Purposeful default lifecycle

**Decision:** Replace special seed publication with the common builtin entry pipeline. Lifecycle uses initialized/policy_handled/removed. Source registration leaves policy unhandled; explicit policy choice, including empty, and explicit reference add mark it handled. Fresh required setup seeds once only if unhandled. New binary offers never replace installed copies automatically.
**Evidence:** bootstrap currently uses alias-specific assets, config existence and embedded digest repair; registering a source before first use can otherwise suppress default policy, while digest repair can replace content without explicit action.
**Alternatives considered:** Preserve seed-origin/alias inference, three-way unknown earlier policy intent, or automatic adoption. Replace these branches directly; no prior development state is supported.

## R10 — Extend generic manager collections

**Decision:** Reuse safe CatalogItem/Field/Review, Close, asynchronous results, theme/frame/viewport and one terminal session. Add nested source/entry navigation, filters, installed selection sets and per-target outcomes through CLI callbacks. Remove direct-install/Official/alias/link controls rather than retaining alternate journeys.
**Evidence:** present/catalog.go hardcodes three tabs and a single selected string; plugin_hub.go composes domain callbacks. Normal exit prints Changes saved successfully.
**Alternatives considered:** Rename tabs only or let presentation call storage/Git. These fail parity or the presentation boundary.

## R11 — Real official origin is a delivery dependency

**Decision:** Define ordinary explicit Git-source behavior and owner-confirmed attribution, plus a real publication evidence gate. Do not infer a URL or publish during planning.
**Evidence:** README/docs/CI contain no official marketplace origin; the module repository is not publication evidence.
**Alternatives considered:** Trust catalog name work or invent a hosting URL. Both fabricate provenance.

## R12 — Current-product validation only

**Decision:** Use fresh disposable homes/current fixtures and focused tests, make seed-all, make lint, go test ./..., relevant race checks, host builds and native three-OS execution, with Unix PTY. Update/delete superseded test expectations instead of retaining compatibility suites.
**Evidence:** Makefile/CI already provide these runners and admin/purity/concurrency/manager infrastructure. Application tests are future implementation work.
**Alternatives considered:** Previous-executable/format-conversion tests or cross-compilation as portability proof. The first is out of scope; the second is not execution evidence.
