# Research: Link Improvements

All findings are from this checkout, 2026-11-14. No dependency research or upgrade is necessary.

## R1 — Wire decoding and error identity

**Decision**: Add optional raw `modes` payload to `internal/ipc/ipc.go` and carry it into `internal/starter/starter.go`, where it is decoded into validated mode context. Missing or top-level null means no overrides; malformed non-null shapes are code 37. A present mode entry must be an object. Sort names for stable diagnostics.
**Rationale**: InvokeStarter currently json.Unmarshal-decodes stdout; starter.Invoke wraps errors as UnusableRepo. A typed map alone misclassifies malformed modes. Retaining the new raw field localizes the new code-37 behavior without changing legacy IPC failures.
**Alternatives considered**: Decode all Starter output anew or globally change IPC error mapping; both broaden unrelated behavior. A typed map plus tagged custom decoder is possible but unnecessary for this isolated addition.

## R2 — Validate, select, merge, bind

**Decision**: ValidateResponse validates all mode entries via the existing validatePublished rules, including entries not offered/selected. A pure Starter helper copies base maps and overlays the selected mode at top-level key granularity. Keep the validated unbound reference until CLI finalizes startMode, then merge and Bind before create.Run.
**Rationale**: internal/cli/start.go currently binds immediately around line 293, while mode settles around line 824 and create.Run occurs around line 875. internal/create/create.go already clones and persists supplied maps atomically. Bind must apply to the effective maps once, after merge.
**Alternatives considered**: Bind every mode early, merge at persistence, or invoke Starter twice; these duplicate work, obscure ownership, or violate the one-response requirement.

## R3 — Presence and provider ownership

**Decision**: EligibleDiagnosed uses storedKey(reg, c.Provider, c.Key), then map membership on Subject.State.Links, for RoleLinker on start:finalized, before subscribed.
**Rationale**: storedKey already resolves public and relative/qualified private declarations to actual provider/source-instance storage. A bare declared-key lookup is wrong for private values. Checking before subscribed prevents ambiguity warnings for a component that should leave no trace.
**Alternatives considered**: Skip in runLinker or PublishLink; too late for eligibility events, diagnostics, Applicable, and manual semantics.

## R4 — Preserve phase snapshots

**Decision**: Keep internal/extension/run.go's existing single EligibleDiagnosed call before the Linker loop. Importers remain evaluated in internal/extension/importer.go after links are published.
**Rationale**: Decisions and input maps are already fixed before execution. Do not re-evaluate eligibility between Linkers. Two providers of an initially missing key remain eligible; no ordering-dependent winner rule is introduced.
**Alternatives considered**: Recompute after each Linker; creates new ordering semantics and contradicts issue #61.

## R5 — Delivery and validation boundary

**Decision**: Use Go-native unit fixtures, Starter subprocess contract cases, and disposable Git integration tests. Extend the historical feature-005 protocol with an explicit current addendum instead of rewriting its earlier F4 context. Formalize requirements in PRD, decision in ADR-0031, flows in ADD.
**Rationale**: Existing tests/contract/starter_context_test.go, tests/integration/start_modes_test.go, tests/integration/start_by_plugin_starter_test.go, and internal/extension tests cover the actual paths. Feature 009 provider semantics are already present in the code despite unfinished delivery tasks.
**Alternatives considered**: Implement marketplace PR guessing or --no-track in this feature; both are explicitly excluded. Run real-home GitHub workflows during planning; unnecessary and non-deterministic.
