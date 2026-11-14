# Contract: Default Lifecycle and Official Publication

**Governing decisions:** ADR-0028–ADR-0030. **Stories:** US1, US2, US4. **Requirements:** FR-015–FR-019, FR-023–FR-024.

## Fresh setup and durable current choices

The builtin provider is code-owned and offers work-reference through the same validation/copy/publication pipeline as any selected entry. There is no embedded installation mode, alternate source layout, or alias-based ownership. Users can toggle/remove its installed plugins but cannot remove or replace the source.

Lifecycle schema 1 records initialized, policy_handled, and removed outside removable package storage. Fresh source registration affects discovery only and does not handle default policy. Explicit policy choice, including empty, marks handled atomically with that choice. Current configuration existence does not imply intent.

| Current state | Required behavior |
| --- | --- |
| Fresh home, only list/help/catalog browse | Project builtin without creating home or installing plugins |
| Fresh required setup | Common entry pipeline installs work-reference@builtin; seed default Locator once only if policy is unhandled |
| Marketplace registration before first start | Registration leaves policy unhandled; required setup follows the same fresh behavior |
| Explicit policy, including empty | Preserve it; setup and later use never insert defaults |
| Explicit reference add/reinstall | Install/review normally; mark policy handled without inserting Locators |
| Installed reference disabled | Preserve content and disabled state until explicit action |
| Reference deliberately removed | Preserve removal flag; later commands, refresh, reads, or a changed binary offer do not resurrect it |
| New binary offers changed reference bytes | Preserve current installed copy; update/reinstall remains explicit |
| Missing/corrupt current content | Surface the current invalid/unavailable state; recover only an authorized current journal or use explicit reinstall, without policy reseeding |

## Scope of the pre-release cutover

The CLI is unreleased. ADR-0030 permits breaking development plugins, state, and prior CLI behavior. Support one marketplace-only model. Remove standalone/link/embedded-mode publishers, global aliases, compatibility selectors, previous-format readers, adoption/migration/reset flows, and compatibility-only tests. Update ordinary examples/callers/tests directly to the intended model.

A strict current validator rejects invalid/unsupported present state without conversion or deletion. Validation uses fresh disposable homes. This plan does not authorize deleting a real development home or adding a reset tool. Current marketplace-created Works and remembered choices remain preserved on plugin/source removal as product behavior. Source-registration instance binding prevents a replacement origin from claiming retained choices/private values; this is current-domain provenance, not a backward-compatibility layer.

## Official online publication dependency

The project offers work through an owner-confirmed Git repository, separately from builtin and binary releases. This documentation work creates no remote or publication. Delivery tasks obtain the actual origin, record its normalized descriptor in docs/marketplaces.md and attribution source, and verify a valid first catalog/publication. Local fixtures prove ordinary source behavior but do not satisfy the real publication requirement.

Official attribution requires confirmed origin and expected namespace; catalog name/display text alone grants neither authorship nor provider authority. Without confirmation show ordinary source metadata and keep the official-offer release gate incomplete. Registration remains explicit and removal does not cause re-registration or reinstallation.

Acceptance requires an independent source publication visible through ordinary catalog upgrade and whole-source removal preserving builtin. Remote publishing/access changes require their own authorization when those implementation tasks are executed.

## Active documentation

Help, examples, author guides and ordinary fixtures describe the one current add/remove, qualified-identity, catalog-refresh and plugin-update contract. No aliases or transition guides retain superseded runtime behavior. Earlier ADR bodies are document history only; ADR-0030 removes their compatibility obligations from the target product.
