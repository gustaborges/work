# Shared Contract: Manual Context Operations (F6)

Authority: spec FR-007–FR-031; research R4–R12. Public CLI contracts are [status](cli-work-status.md), [link](cli-work-link.md) and [import](cli-work-import.md).

## 1. Effective installed catalog

Add optional package `enabled` to `registry.json`; omitted/null means true. Installation manifests do not accept this field. Reinstall and seed repair preserve it. This is retained core-maintained package state, with no duplicate in global config or installation metadata. Legacy ownerless components remain enabled.

Expose raw installed lookups separately from effective catalogs. Starter resolution/fallback, Locator execution, automatic/manual extension activation and convention availability consume effective state. Administration/inventory, install collision checks and bootstrap integrity consume raw records as appropriate. Disabled Locators retain their policy references and appear unavailable. Do not seed policy, reinstall a disabled package, or silently alter precedence because an effective query found nothing. F7 owns the eventual enable/disable API and dependency checks.

Manual choices use `manual.display_name` and optional description, with qualified provider identity always visible. Sort choices bytewise by `alias/name`; install order has no selection significance.

## 2. Shared application interfaces

These are internal Go design contracts, not a new plugin API. Implementations remain presentation-free except the CLI composition layer.

| Interface | Responsibility |
| --- | --- |
| `workcontext.Current(cwd)` | Pure nearest-ancestor resolution with canonical validation. |
| `workcontext.ByID(ctx, workspace, id, hints)` | Pure exact identity lookup, canonical scan and optional verified path hint. |
| `projection.LookupPathHint(ctx, path, id)` | Optional noncreating immutable read; no migration/pragma/repair/lock or authoritative state result. |
| `registry.PackageEnabled(alias)` | One backward-compatible enablement predicate. |
| `registry.Effective()` | Read-only filtered catalog for activation; leaves the raw installed registry intact. |
| `extension.ResolveInputs(declarations, subject)` | Typed namespace resolution, present optional values, sorted missing required declarations and malformed-declaration error. |
| `extension.ManualCandidates(reg, role, subject)` | Eligible manual choices without subprocesses. |
| `extension.ResolveManual(reg, role, reference)` | Exact qualified or unique bare manual provider resolution before importer input eligibility. |
| `extension.PublishLink(ctx, params)` | Active/identity-checked, locked atomic publication with operation-specific current provenance. |
| `extension.ImportOne(ctx, params)` | One runtime invocation and shared locked, cancellable, staged artifact commitment; returns structured result/failure. |
| `staging.Plan.ApplyContext(ctx)` | Apply with exclusive creation and undo ownership retained until explicit terminal action. |
| `staging.Applied.Commit / Rollback` | Seal successful changes or remove this execution's creations; preserve rollback errors. |

Automatic `extension.Run` continues returning its report with warning policy. It adapts shared publication/import failures into existing F5 warnings; manual CLI translates them into command errors once. Shared code never prints to streams or decides whether an automatic start should fail.

Revalidate selected component role/manual/key/enablement from the latest atomically loaded registry before manual invocation or publication and before importer incorporation. Required importer inputs resolve immediately before invocation; no input recomputation is sent after the subprocess completes. Per-Work locks serialize commitment with resume/archive, without locking an entire unbounded plugin execution.

## 3. Manual failure table

Codes 39–49 extend `diag.All`; existing codes/tokens remain unchanged. Category variable names are implementation details; these tokens and codes are stable public contracts.

| Code | Token | Meaning |
| --- | --- | --- |
| 0 | `ok` | Canonical association or complete import committed; empty successful import also succeeds |
| 2 | `usage` | Missing non-interactive arguments, empty supplied link value, invalid grammar or mutation `--json` |
| 16 | `bootstrap-failed` | Required global configuration or registry cannot be read, without attempting bootstrap |
| 20 | `cancelled` | User cancellation before commitment, with cleanup/rollback complete |
| 21 | `target-not-found` | No current Work, including a disappeared destination when no archived state is available |
| 22 | `target-archived` | Current/expected Work is canonically archived before commitment |
| 25 | `snapshot-unreadable` | Invalid/unreadable canonical snapshot, changed identity or duplicate identity |
| 39 | `manual-component-unavailable` | Named component unknown, disabled, wrong role, no longer matching selected identity/key, or lacks manual availability |
| 40 | `manual-component-ambiguous` | Bare component name identifies multiple enabled manual providers |
| 41 | `no-eligible-component` | No selectable manual provider for requested role/current inputs |
| 42 | `required-input-missing` | Selected Importer lacks one or more required declared inputs |
| 43 | `link-failed` | Work lock or atomic canonical publication failed |
| 44 | `extension-start-failed` | Importer runtime/entrypoint could not launch |
| 45 | `extension-failed` | Importer exits unsuccessfully |
| 46 | `extension-response-invalid` | Importer stdout exceeds limit or violates the existing final response contract |
| 47 | `extension-output-refused` | Whole staged execution refused for unsafe destination/content or collision |
| 48 | `extension-persist-failed` | Stage creation/inspection, commitment lock, or incorporation failed with complete rollback |
| 49 | `extension-cleanup-failed` | Stage removal or destination rollback failed; recovery needed |

A cleanup/rollback failure takes precedence over cancellation or the original execution failure and preserves all causes internally. F5 automatic execution uses its existing warning tokens; a new cleanup cause maps to existing `extension-persist-failed`, with no new automatic exit behavior. A provenance-only failure after canonical link commitment does not become `link-failed`.

For a moved destination, check the captured snapshot path and, when absent, the configured archive area for the expected ID: known archived state gives 22, genuinely unavailable context 21, unexpected unreadability/changed identity 25. Never recreate the old directory or redirect an import into the archive.

## 4. Diagnostics and channels

All final errors reach `cli.renderDiagnostic` exactly once. Non-interactive form is `error: <token>: <message>` on stderr; human interactive form uses summary and actionable hint. Execution diagnostics include Work ID, qualified component and operation. Missing inputs report sorted declarations only. Ambiguity reports qualified alternatives without selection.

Plugin stdout, stderr, link/meta values and artifact content are not normal diagnostic content. Retain bounded stderr and original causes for `WORK_DEBUG`; extend the existing border to show retained execution details for manual errors in interactive and non-interactive modes. Normal stdout contains only the command's contracted result. Progress/receipts use the configured UI writer. Status is the requested saved-value presentation surface and escapes controls in human output.

## 5. Presentation and cancellation

Use existing interactive detection and `present.IO`, wizard/selector/input primitives, semantic theme, viewport and key bindings. Link is one wizard across omitted steps. Explicit arguments skip their collection; they do not authorize a mutation whose validation failed. Mutation happens after successful collection, outside presentation callbacks. Input validation is repeatable and mutation-free.

No unconditional overwrite confirmation is added to upsert: the supplied value or accepted input is the user's association action, consistent with PRD FR-31. A link receipt names its semantic relation and uses `value supplied`; it never prints the value. An import selector receipt names the qualified component. Accepted-choice receipts are not a claim that the later domain operation succeeded.

Before commitment cancellation unwinds and exits 20 when cleanup succeeds. Once a snapshot rename or applied-output commit succeeds, the result is committed success even if a signal arrives immediately afterwards. No shell handoff file is written by any F6 command.

## 6. Test fixture contract (to implement)

Add native offline `tests/fixtures/plugins/manual-suite`, using existing fixture `Prepare` and per-entrypoint build conventions:

| Component | Declaration / behavior |
| --- | --- |
| `linker` | Manual GitHub PR relation `github.pull_request`; discovery requires deliberately absent metadata, proving association ignores those inputs. Any unexpected invocation is logged and fails. |
| `importer` | Manual context importer; requires `link:github.pull_request`; optional `work:start_mode` and private nullable metadata. Event filter names an unrelated Starter, proving manual activation independence. |
| `needs-input` | Manual importer requiring absent `link:github.issue`. |
| `automatic-only` | Importer without manual availability; must never run through manual commands. |

Install the same local fixture under two aliases to produce real name ambiguity without source conflicts. Use `WORK_FIXTURE_LOG` to record every process input, and `WORK_FIXTURE_MODE` modes consistent with F5: `importer-empty`, `importer-exit1`, `importer-garbage`, `importer-overflow`, `importer-mixed`, `importer-worktree`, `importer-snapshot`, `importer-symlink`, `importer-hang`, plus `importer-case-alias` and `importer-null` cases. Default output is `notes/context.md` and `refs/source.txt` (four new items including directories).

`WORK_FAIL_AT=incorporate:<n>` remains the shared rollback injection. Cancellation during copy, cleanup/rollback failure and registry changes between selection/commit use deterministic Go test hooks/dependencies rather than new public CLI flags. Disabled package cases persist `enabled:false` through the registry writer in the fixture harness; no unimplemented F7 command is used.
