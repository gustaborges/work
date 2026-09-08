# ADR-0006: Component Runtime Technology and Invocation Model

**Status:** Accepted
**Date:** 2026-08-23
**Product Context:** `docs/prd.md` — RNF-2, RNF-6
**Governs:** `docs/add/add-0001-work-system-architecture.md`, Section 11

## Context

The product promises that anyone should be able to write a plugin without depending on a merge in the Work repository (RNF-2); RNF-6 requires portability across supported operating systems. The external process execution model (ADR-0000) is already what makes language neutrality possible — what remains is to decide how the core effectively invokes a process written in an arbitrary language in a portable way.

## Decision

* No language/stack restrictions are imposed on a plugin. Restricting to self-contained binaries was considered and rejected: it buys no security (a malicious binary is as dangerous as a malicious script — restricting stack is portability control, not security) and would exclude the most likely author of a simple plugin, who prefers a scripting language over learning a new language just to publish a small integration.
* The manifest (ADR-0012) optionally declares which interpreter a component needs (e.g., `python3`, `node`, `sh`; absent means self-contained executable).
* Work **never** depends on shebang or the operating system's execute bit to choose the interpreter — it explicitly invokes the declared interpreter over the entrypoint when it exists, or the entrypoint directly when absent. This is a portability requirement, not style: shebang lines are not interpreted by Windows process creation — depending on them would silently break every plugin in an interpreted language (likely majority of the ecosystem) on a supported OS (RNF-6).
* The presence of the declared interpreter is checked in preflight (at installation/before first invocation): if it does not exist in `PATH`, Work fails with a specific and actionable message, rather than letting the subprocess fail cryptically.
* An automatic detection model between platform-precompiled binary and interpreted script via shebang (seen in other CLI extension tools) was evaluated and not adopted as such: it solves a different problem (choosing between multiple *release artifacts* for the same logical extension, per platform), and its interpreted mode still depends on shebang + execute bit — exactly the mechanism avoided here because of Windows.

## Alternatives Considered

* **Fully open, without interpreter declaration in the manifest.** Minimal friction for the author, but produces "invalid executable format" failures without context when the interpreter is missing — rejected.
* **Restrict to self-contained binaries.** Eliminates the "missing interpreter" error class, but contradicts RNF-2/ADR-0000 and does not improve security, as described above — rejected.

## Consequences

**Positive:** plugins in any language work uniformly across supported operating systems, without depending on a non-portable OS mechanism; failures due to missing interpreter are clearly diagnosed at installation/preflight, not at some arbitrary invocation later.

**Negative / trade-offs:** the declared interpreter is self-declared, without independent verification beyond checking for existence in `PATH` — it confirms that the interpreter exists, not that the specific entrypoint actually runs without error. This residual risk is accepted, in line with the trust given to the rest of the manifest (ADR-0012): if the entrypoint fails for another reason, this manifests on first real use as a common process error (non-zero exit code, Section 11 of ADD), never as a silent failure.
