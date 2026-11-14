# Contract: Known-link Suppression at Start

Authority: PRD FR-85/FR-86, ADR-0031, ADD §§9/11.

For RoleLinker on start:finalized, resolve its output key using the existing storedKey provider/source-instance mapping. If that key is present in the phase's initial Subject.State.Links, exclude the component before subscription diagnostics and ResolveInputs. Exclusion produces no subprocess, warning, progress event, provenance, or run receipt. `Applicable` observes the same eligibility rule.

No suppression applies to Importers, manual work link, or unrelated events. Otherwise retain existing automatic activation, enablement, subscription, Starter filter, required-input checks, and deterministic ordering. Failed stored-key resolution cannot match unrelated private state.

Compute all Linker decisions and projected inputs once before the first execution. Do not let a Linker's output satisfy another Linker's inputs within that phase, or suppress another initially eligible Linker for the same key. Evaluate Importers after the Linker phase; Starter-published and discovered links remain valid Importer inputs. Existing publication upsert/provenance semantics remain unchanged for components that actually run.

Validation matrix: known/missing public key; known empty stored value; current/wrong provider and current/former registration instance private keys; known key with ambiguous Starter filter; missing required input; two same-key initially missing Linkers; fixed inputs; eligible Importer on known/discovered links; Applicable false when only known-key Linkers exist; manual replacement with no script invocation.
