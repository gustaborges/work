# Contract: Convention Administration

```text
work convention
work convention show [--json]
work convention set <CONVENTION>
```

Retain existing repository identity resolution from any clone: origin URL, sorted root commits, then path for the established shallow/no-remote case. Outside a usable Git repository return existing actionable error. Inspection may invoke existing read-only Git identity commands but never bootstrap, change Git, create Work-home state, update recent access or save memory.

Show reads the remembered string and effective availability independently. Existing JSON keys identity and convention remain; add available and unavailable_reason:

```json
{"identity":"repository-identity","convention":"team","available":false,"unavailable_reason":"provider-unavailable"}
```

No choice gives convention=null, available=false, reason=not-set. An available choice gives available=true/reason=null. Human output retains `work: convention <NAME>` for available choices and `work: convention not set` for absence; an unavailable stored choice adds an explicit unavailable label and install/enable/set guidance. Shared convention names remain available while any owning package is enabled under existing prefix/catalog semantics.

Set accepts an exact available convention name under existing catalog semantics, rejects --json, and retains `work: convention set to <NAME>`. Under the administration lock reload current catalog and memory, validate availability, upsert only the current repository identity and commit. Never overwrite another repository's concurrent choice or modify existing Work snapshots/branches.

The hub initially inspects without writing, shows current value/availability, offers available names and Leave unchanged, and displays equivalent `work convention show` or `work convention set <CONVENTION>`. No available choice gives guidance without opening an empty selector. Bare noninteractive invocation returns usage/2 as before.

During non-contribution `work start`, a remembered unavailable name stays persisted until an explicit replacement is accepted. Interactive creation asks for an available replacement even if only one exists; noninteractive creation fails convention-unknown/38 with enable/install/set guidance. A new repository with no remembered choice retains the existing sole-available-convention default behavior. Contribution mode still bypasses convention selection. Final memoization reloads memory under lock and rejects changed reviewed choice/provider state instead of silently overriding it.

Inspection is pure; cancellation/Leave unchanged saves nothing. Normal views/diagnostics must redact credentials embedded in a repository identity's origin URL while retaining the raw identity privately for matching. No replacement is inferred from removal, update or disablement.

Codes remain usage/2, bootstrap-failed/16 for malformed stored non-journal state, cancelled/20, convention-unknown/38; concurrency/recovery use additive 53–56. US5/FR-025–FR-031; Q1/Q11–Q12 verify unavailable choice across clones, shared owners, explicit replacement and unchanged historical Works.
