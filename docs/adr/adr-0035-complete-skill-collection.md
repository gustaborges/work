# ADR-0035: Complete Work skill collection for one explicit vendor

**Status**: Accepted, 2026-11-14; approved by the project owner in the CLI experience review.
**Supersedes**: [ADR-0033](adr-0033-skills-command.md).
**Related requirements**: Agent authoring skills FR-88 and FR-90 in [PRD](../prd.md).
**Realized by**: [ADD-0001](../add/add-0001-work-system-architecture.md), skill command adapter and inventory.

## Context

Work will bundle multiple authoring skills. Defaulting to both agents installs unrequested copies, and selecting individual skills makes the command depend on catalog contents. Users want local installation anchored to their current directory and a review opportunity in either scope.

## Alternatives considered

Retain default-both selection and arbitrary folder paths; offer individual skill operands; or require one vendor and install the complete bundled collection. Automatic vendor detection would couple offline file provisioning to executable discovery and would not establish which agent the user intends to configure.

## Decision

Retain the narrow ADR-0019 plural exception for `work skills install/status` in Setup. Require exactly one `--vendor=claude|codex` and `--scope=local|user`. Reject repeated vendors, `all`, skill operands, `--global` and `--dir`. Local scope uses the invocation's current directory; user scope retains documented per-agent user destinations. Resolve every bundled skill from the release inventory; there is no skill selection or all-vendors option. Do not detect or require an installed agent executable.

Both installation scopes show the full skill/destination preview through the existing confirmation control. `--yes` skips this confirmation and is required non-interactively. `--force` separately authorizes replacement of ordinary modified/foreign skill content and does not skip confirmation. Status is read-only, accepts JSON and never prompts. Install rejects JSON. No interactive selection hub or aliases.

Name the first skill `work-cli-plugin-builder` to identify the Work CLI plugin ecosystem. Subsequent skills extend the inventory without changing command grammar. No legacy-name installation, migration or deletion is implicit.

## Rationale and consequences

Explicit vendor selection avoids unrequested destinations. A release collection stays cohesive as it grows; users cannot selectively install its members. Confirmation makes destinations reviewable, while automation supplies explicit consent. ADR-0034 continues to govern per-target recovery and partial failures: complete-collection selection does not introduce a collection-wide atomic filesystem transaction. The existing confirmation component supplies the shared CLI keymap and presentation.
