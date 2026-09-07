# ADR-0012: Component Model and Plugin Manifest

**Status:** Accepted

**Date:** 2026-08-29

**Product Context:** `docs/prd.md` — RF-1, RF-2, RF-8, RF-22, RF-26 to RF-32, RNF-1, RNF-2, RNF-3 and RNF-7

**Governs:** `docs/add/add-0001-work-system-architecture.md`, Sections 4, 5, 7 and 9 to 11

## Context

The previous model mixed component role, open capabilities and hooks defined by Starter. This leaves extension eligibility dependent on poorly explicit conventions and allows the plugin to describe control flow that belongs to Work. It also does not precisely separate an extension's data needs from the form by which it is activated.

The product requires that a package be able to declare Starters, Repository Locators, Importers, Linkers and branch conventions without the core knowing the integration. A package also needs to be a coherent unit of installation and maintenance: an integration can bring together components that make sense only together. At the same time, the core must decide, without executing code, whether an operation can run, what data it receives and when its effects can be incorporated.

## Decision

A plugin is a versioned package and the atomic unit of installation, update, enablement and removal. Its `plugin.json`, read by Work in the installation pipeline, declares zero or more executable components in `components[]` and zero or more declarative conventions in `conventions[]`. Each component has a mandatory `role`: `starter`, `repository-locator`, `importer` or `linker`. `role` is the semantic discriminator: it determines the contract, valid manifest fields and operations Work can request. There is no `invocation` field or equivalent; protocol and activation form are not component identity.

Component identity is its logical `name`, never the file implementing it. The package's `name` is a local alias proposal: the installed registry ensures uniqueness for the user and, when component names collide between enabled packages, Work qualifies them as `<alias>/<name>`.

`conventions[]` remains separate from `components[]`: a convention is a static contribution, without `role`, `entrypoint` or `runtime`. Without dynamic detection in v1, executing a process to obtain a catalog already known in the manifest only adds runtime, preflight and conditional paths to the core.

The manifest statically describes what the component is, what operations it offers, activation points and required data. Work controls lifecycle, Starter selection, event publishing, eligibility, input projection, subprocesses, metadata/link persistence and file incorporation. Plugins implement domain behavior, not Work's control flow.

Repository Locators declare non-empty `accepts` with Repository Reference fields they know how to consume. The core considers them only when they are in the Repository Resolution Policy, the plugin is enabled and some accepted field is present; the policy and search roots are controlled by the user as per ADR-0015. Importers declare events in `on`, manual availability in `manual` and inputs in `inputs`. Linkers declare a `key`, optional discovery in `discover` and optional manual association in `manual`. Events are defined by the core; `starters` is an activation filter, never input. Before any subprocess, the core evaluates eligibility statically and does not execute a component just to discover whether it should run.

Links and metadata are distinct spaces. Inputs use `link:<key>` or `meta:<key>`, with `:optional` when applicable. Links have one value per key and are updated by upsert: the last source wins. Importers write to an exclusive temporary directory and their artifacts are incorporated only after complete collision validation.

The v1 `start:finalized` event is processed in phases: persist Starter data, execute/persist eligible Linkers and then execute eligible Importers. There is no guaranteed order between components of the same phase.

## Alternatives Considered

* **Keep `capabilities` and hooks commanded by Starter.** Rejected: open capability does not precisely describe operation, activation and inputs, and hooks in Starter unnecessarily couple extensions to the initiator that fired them.
* **Execute components to decide eligibility.** Rejected: turns common data absence into unnecessary subprocess, makes lifecycle less predictable and introduces effects before the core decides the operation is valid.
* **Pass the real Work directory to Importer.** Rejected: prevents comprehensive collision validation and allows partial alteration before the core controls incorporation.
* **Model conventions as a fourth role.** Rejected: conventions are not executable and need no runtime, input or process; keeping the two arrays separate makes this difference visible in the schema, instead of spreading conditionals across manifest, registry and execution.
* **One package per component.** Rejected: forces naturally coupled contributions, like recognizing and importing data from a pull request, to be versioned and installed separately.

## Consequences

**Positive:** the core decides operations in a predictable and auditable way; contracts are minimal per role; integration data is preserved without expanding the core model; Importers do not silently overwrite Work files.

**Trade-offs:** the manifest becomes more expressive and requires role-discriminated validation; repository resolution gains an explicit policy; automatic extensions can fail after Work creation, so Work concludes with a warning instead of undoing an already-materialized environment.

## Relationship with Previous Decisions

This ADR consolidates package/manifest and declarative conventions decisions previously recorded separately. It replaces the model that used `type`, `capabilities` or `hooks`, as well as the previous hook contract. ADRs 0014 to 0016 extend this model with the `repository-locator` role and its contract. Decisions on external subprocesses, explicit runtime and Starter collision remain in their own ADRs.
