# Contract: `work import` (F6)

Authority: spec FR-007–FR-012, FR-018–FR-031; PRD FR-27–FR-30; ADD §10–§12.

## Grammar

```text
work import [IMPORTER]
```

No argument collects one eligible Importer interactively. One argument resolves it directly; more arguments fail usage. Current active Work is always the target. No explicit Work target, `--yes`, bulk mode or new discovery trigger is introduced. `--json` is rejected, including explicit false.

Non-interactive use requires an Importer argument and never opens a selector. Explicit identity skips only selection, not availability, lifecycle or required-input validation. Names resolve as specified in [manual-context.md](manual-context.md), before input eligibility, so an ambiguous bare name cannot implicitly choose the sole currently eligible provider.

## Manual eligibility and inputs

The component must be enabled, have role importer, declare manual availability and have all required inputs present. Event subscriptions and Starter filters are ignored for this activation. The interactive list contains only eligible components sorted by qualified identity, with manual display name/description and qualified identity. No eligible component is an actionable failure; do not open an empty selector.

Resolve declared inputs immediately before invocation from current canonical state, using the same five public Work facts and namespace projection as F5. Preserve values/types; omit absent optional inputs; retain present null metadata. Missing required declarations are named without values and prevent subprocess execution. No Linker is run to fill missing data.

## Execution and incorporation

Reuse the F5 [execution protocol](../../006-automatic-start-context/contracts/extension-protocol.md) §§3–4 and safety matrix in §6. F6 changes activation, commitment coordination and error severity, not the IPC format:

```json
{
  "inputs": {
    "github.pull_request": "https://example.test/pr/212",
    "start_mode": "new"
  },
  "output_dir": "/absolute/temp/work-import-unique"
}
```

Exactly one selected Importer is invoked. Runtime invocation, the 1 MiB stdout cap, response shape and bounded inherited-pipe wait remain F5 rules. No execution timeout is introduced. Staging remains a data boundary, not a sandbox or process-tree isolation promise.

Run the subprocess outside the Work lock. After a successful valid response, acquire the existing per-Work lock, reread and fully validate active identity/path, and revalidate the selected provider. Build the complete destination plan under the lock. Validate the complete output plan read-only for existing collisions, reserved snapshot/checkout paths, escaping or symbolic-link destinations and nonregular output before tentative application. Existing directories may merge when safe; files never overwrite. Native exclusive file creation and directory identity checks supply the final destination-name-equivalence and race checks during application. Case-distinct names remain valid on case-sensitive targets; native alias refusal rolls back this execution before commitment.

Apply output with cancellation checks during copying. Keep its undo list until staging is removed and the final cancellation check passes; then commit and release the lock. If archive already moved or canonically archived the Work, incorporate nothing. On failure or interruption, roll back only this execution's paths and clean the stage without using the cancelled context. Return explicit recovery diagnostics if removal itself fails; never claim successful rollback when it failed.

## stdout, progress and effects

Success emits exactly:

```text
work: imported <id> <alias/name> <count>
```

Count is newly created files plus directories, matching F5 staging counts; merged directories are excluded. Empty successful output has count 0. Progress names the selected component and `import` on the configured UI channel. No artifact content, inputs, link values or absolute stage path appears in normal progress/results.

Failure has no success stdout and returns a nonzero category through the sole diagnostic border. A late interrupt after commitment does not report a discarded import. Work state/metadata/links/recent access and shell position are unchanged in all outcomes. Earlier successfully imported files remain untouched; normal archive later preserves successful artifacts.

## Failures

Common preconditions and codes are in [manual-context.md](manual-context.md). Import-specific errors distinguish launch (44), process exit (45), response (46), refused output (47), staging/planning/lock/incorporation (48) and cleanup/rollback recovery (49). User cancellation is 20 when cleanup and rollback succeeded. An archived target or invalid snapshot discovered before commitment uses its existing category (22 or 25).

Automatic `work start` continues using its warning tokens and success policy for these execution failures. Only manual commands turn them into command errors.
