# ADR-0000: Plugin Execution Model — External Subprocesses with Standardized Contract, Not In-Process Library

**Status:** Accepted
**Date:** 2026-08-23
**Product Context:** `docs/prd.md` — RNF-1, RNF-2, RNF-7, RF-22

## Context

The PRD establishes that, except for Git, the core embeds no knowledge of any specific tool (RNF-1, RNF-2, RF-22): "the core knows only contracts and external executables — the only direct and internal invocation is Git itself" is an architectural decision that follows from this principle, not a product requirement in itself — therefore it lives here, not in the PRD. This ADR gives that decision the formal record that the other ADRs in this set (0001 onwards) already assume as a premise.

The underlying problem: how should Work execute a plugin's domain logic (interpret an argument, enrich context, relate external resources) without a bug or malicious behavior in that plugin compromising the stability or security of the core?

## Decision

`git` is the only tool that the Work core invokes directly, in-process. All other domain behavior involving logic — any plugin fulfilling the role of Starter, Importer, or Linker — runs as an external process, separate from the Work process, communicating via a well-defined and publicly documented input-output contract. The core never imports, loads, or executes plugin code within its own process.

A plugin contribution with no logic at all — the branch convention catalog (ADR-0012) — does not fall under this decision because it has no domain behavior to execute: it is static manifest data, not a process. The decision recorded here is about how Work runs what needs to be executed, not a requirement that every plugin contribution be executable.

The concrete details of the contract (payload format, transport, exit code convention) are described in `docs/add/add-0001-work-system-architecture.md`, Section 11 — this ADR records only the decision that execution is always via external process, not the exact mechanics of that communication.

This choice is what makes possible the language neutrality decided in ADR-0006: an in-process execution model would necessarily tie plugins to the core's language/runtime.

## Alternatives Considered

* **In-process plugin, loaded as library/extension within the Work process** (VS Code model). Rejected: a buggy or malicious plugin crashes or compromises the Work process itself, and expands the surface of code that the user must trust without review — the direct opposite of RNF-7. This model would also tie plugins to the core's language/runtime, which a survey of market precedents (git subcommands, credential helpers, LSP, `asdf`, `gh extension`, Terraform) shows is unnecessary: every tool whose model is "protocol + external subprocess" is language-agnostic by design — the only observed exception (VS Code) is precisely the model rejected here.
* **Plugins compiled/linked statically into the Work binary.** Rejected: would require recompiling and republishing Work itself to add a third-party plugin, the opposite of RNF-2 (friction-free extensibility) and RF-22.

## Consequences

**Positive:** a buggy plugin never crashes the Work process; the surface of code the user must audit without review stays limited to the core; no language tie-in is imposed on plugin authors (see ADR-0006).

**Negative / trade-offs:** each plugin invocation pays the cost of spawning a separate process — acceptable because these invocations happen at discrete points in the flow (start of a Work, finalization hooks), never in a hot loop.
