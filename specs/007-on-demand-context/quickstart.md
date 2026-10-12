# Quickstart and Validation Guide: On-Demand Context (F6)

**Feature**: `specs/007-on-demand-context/` · **Plan**: [plan.md](plan.md)

These scenarios describe the delivered F6 commands and offline fixture. Automated acceptance evidence is recorded below. Exact fields, tokens, IPC and commitment behavior are defined in [contracts/](contracts/manual-context.md) and [data-model.md](data-model.md).

## Prerequisites and isolated setup

Use Go 1.26, system Git >= 2.5, F5 code, and the implemented F6 native `manual-suite` fixture. No network or external integration account is needed. Run from the repository root. The POSIX commands below work on Linux/macOS or Windows Git Bash; automated portable equivalents belong in testscript, and PTY scenarios run only on Unix.

```bash
make build
repo_root="$PWD"
work_bin="$repo_root/bin/work"
demo_dir=$(mktemp -d)
export WORK_HOME="$demo_dir/work-home"
export WORK_FIXTURE_LOG="$demo_dir/launches.jsonl"
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=F6-Demo
export GIT_AUTHOR_EMAIL=f6@example.test
export GIT_COMMITTER_NAME=F6-Demo
export GIT_COMMITTER_EMAIL=f6@example.test

git init -b main "$demo_dir/demo"
git -C "$demo_dir/demo" -c commit.gpgsign=false commit --allow-empty -m initial
"$work_bin" start "$demo_dir/demo" --workspace "$demo_dir/workspace" \
  --base main --slug f6 --prefix '{slug}' --yes
work_dir="$demo_dir/workspace/in-progress/demo_f6"
cd "$work_dir/worktree"
"$work_bin" status --json > "$demo_dir/status.json"
work_id=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["id"])' "$demo_dir/status.json")
```

Python 3 is used only for manual artifact inspection in this guide, not by Work or fixtures. On Windows use the platform null device for Git config variables if `/dev/null` is not translated by the environment.

Build the fixture as the test suite’s existing `plugins.Prepare` helper does:

```bash
manual_pkg="$demo_dir/manual-suite"
mkdir -p "$manual_pkg"
cp "$repo_root/tests/fixtures/plugins/manual-suite/plugin.json" "$manual_pkg/plugin.json"
fixture_ext=
if [ "$(go env GOOS)" = windows ]; then fixture_ext=.exe; fi
for component in linker importer needs-input automatic-only; do
  (cd "$repo_root" && go build -o "$manual_pkg/$component$fixture_ext" \
    ./tests/fixtures/plugins/manual-suite)
done
"$work_bin" plugin install "$manual_pkg" --as manual
```

The fixture is manual for these journeys, logs every invocation, and defaults to two nested artifacts: `notes/context.md` and `refs/source.txt`. Its exact provider declarations/modes are in [manual-context.md §6](contracts/manual-context.md#6-test-fixture-contract-to-implement).

## Q1 — Inspect active Work from every supported context (US1; SC-001)

```bash
"$work_bin" status
"$work_bin" status --json
cd "$work_dir"
"$work_bin" status
mkdir -p "$work_dir/artifacts/deep"
cd "$work_dir/artifacts/deep"
"$work_bin" status --json
cd "$demo_dir"
"$work_bin" status "$work_id" --json
```

Expect exit 0 in all cases, the same ID/branch/state/links, and active location `$work_dir/worktree`. Explicit target wins even when called from another Work. Empty links are `{}` in JSON and `work: links none` in human output. No launch-log entries or activity-history updates occur. The automated reader tables also cover paths containing spaces/Unicode and contribution-mode snapshots.

## Q2 — Prove status purity, including unusable index (US1; SC-001)

Capture snapshot/config/registry/index/sidecar bytes and modification times, provenance/access rows, directory inventories and shell handoff contents using read-only inspection. Repeat each human/JSON status form 20 times, then compare. Expect zero application writes and no plugin/Git execution.

In isolated copies, remove the projection, replace it with invalid bytes, give it a stale location row, leave a WAL with pending index changes, and provide an older accepted database version. Run current status and explicit status against the configured workspace each time. Expect correct canonical data and no database creation, migration, sidecar/lock creation or repair. Include an otherwise empty/nonexistent WORK_HOME with current-directory status; it must not bootstrap.

Do not inspect purity by opening the projection through Work's writable helper between measurements. Treat OS access-time effects separately from application writes.

## Q3 — Archived views and missing checkout (US1; SC-001)

Using a separate demo Work, archive it through `work archive <id> --yes`. Inspect with an explicit identity from outside, then from its archived directory and a descendant. Expect `archived` and archived directory location, with saved links intact and no checkout requirement.

Using an isolated filesystem fixture, remove an active checkout while leaving its valid snapshot. Status still reports canonical identity and expected active location without repair or a claim of checkout health. Inspect schema-1/schema-2 snapshots and verify no schema rewrite.

## Q4 — Associate and replace exact values without a process (US2; SC-002)

```bash
cd "$work_dir/worktree"
"$work_bin" link manual/linker 'https://example.test/pr/212'
"$work_bin" status --json
"$work_bin" link manual/linker ' value with spaces and $literal characters '
"$work_bin" status --json
```

Expect one `work: linked <id> github.pull_request manual/linker` success line per association, no echoed value and no fixture process launch. JSON preserves the supplied text exactly, including surrounding spaces. Snapshot metadata, other links, lifecycle fields and access timestamp are unchanged. Current provenance names `manual/linker`, operation `link` and recording time. Repeat after a Starter/discovery value and through a second provider sharing the same semantic key; the last committed value/source wins.

## Q5 — Interactive association and cancellation (US2; SC-002, SC-006)

From the active checkout in a PTY, run `work link`; choose the manual provider, submit an empty value, correct it and accept. Expect one live field error, compact accepted receipts, `value supplied` instead of the raw accepted value, and the stable success line after leaving the wizard. Missing discovery metadata must not hide this manual Linker.

Run `work link manual/linker` and confirm only the value step appears. Cancel separately at selection and input. Expect exit 20, one cancellation rendering, no saved link/provenance/access change and no plugin launch. Inject canonical write failure after collection; expect `link-failed` (43), zero publication and no success stdout.

## Q6 — Import using the manually associated link (US3; SC-002–SC-004)

```bash
"$work_bin" link manual/linker 'https://example.test/pr/212'
"$work_bin" import manual/importer
"$work_bin" status --json
```

Expect `work: imported <id> manual/importer 4`; the two files and two new parent directories exist beside the checkout, with no tracked-tree modifications. Exactly one importer invocation appears in the launch log. Its input contains the current declared link and present optional facts only, plus an exclusive output directory now removed. No Linker/discovery or companion importer runs. A deliberately nonmatching automatic Starter filter must not block manual activation.

After successful import, archive this Work in a separate scenario and verify the files survive with its snapshot.

## Q7 — Missing and optional inputs (US3/US4; SC-003)

```bash
"$work_bin" import manual/needs-input
WORK_FIXTURE_MODE=importer-empty "$work_bin" import manual/importer
```

First command: exit 42, diagnostic names required `link:github.issue` but no values, no process or new stage. Second command: exit 0 with count 0 and no artifact change. In isolated fixture snapshots, include a present optional null metadata value and omit it in another case; the launch log shows null in the first and no key in the second. Interactive choices exclude missing-input and automatic-only providers. Missing link data never triggers discovery to fill it.

## Q8 — Refuse the whole unsafe or colliding import (US3; SC-004)

Repeat a default import after Q6: expect exit 47 and no overwrite. Compare pre/post artifact trees and snapshot bytes. Then use clean isolated Works for each mode:

```bash
WORK_FIXTURE_MODE=importer-mixed "$work_bin" import manual/importer
WORK_FIXTURE_MODE=importer-worktree "$work_bin" import manual/importer
WORK_FIXTURE_MODE=importer-snapshot "$work_bin" import manual/importer
WORK_FIXTURE_MODE=importer-case-alias "$work_bin" import manual/importer
```

The mixed mode includes a reserved snapshot path alongside a clean companion file. The staging tables separately prepare ordinary destination collisions. Expect whole-execution refusal for reserved paths/mixed collisions, no clean companion file incorporated, existing artifacts byte-identical and no stage remaining. Case-alias output is refused and rolled back on a target that aliases those names; on a case-sensitive target both names may succeed, preserving F5 semantics. The staging tables also cover symbolic links (where supported), special files, escaping destination ancestors, safe existing-directory merging, native aliases against existing directories and exclusive-creation backstops. No writes may enter the checkout or snapshot path.

## Q9 — Failures, rollback and user interruption (US3; SC-004)

```bash
WORK_FIXTURE_MODE=importer-exit1 "$work_bin" import manual/importer
WORK_FIXTURE_MODE=importer-garbage "$work_bin" import manual/importer
WORK_FIXTURE_MODE=importer-overflow "$work_bin" import manual/importer
WORK_FAIL_AT=incorporate:1 "$work_bin" import manual/importer
```

Use a clean eligible Work for each invocation. Expect exit 45, 46, 46 and 48 respectively, with one diagnostic, no success stdout, no artifact contribution and no remaining stage. Missing runtime/entrypoint after installation yields exit 44; association through its manual Linker still requires no executable.

Run `WORK_FIXTURE_MODE=importer-hang work import manual/importer` in a PTY and interrupt after fixture synchronization confirms staged partial output. Expect exit 20, process terminated and no artifacts/stage. Deterministic unit tests cancel during copying, before commitment and during lock waits. Inject stage-remove and rollback-remove failures: expect 49 with precise recovery guidance and retained causes, without a false claim that rollback succeeded.

## Q10 — Explicit names and automation boundaries (US4; SC-003, SC-005)

Install the same fixture under `--as alternate`. Bare `linker` and `importer` now fail 40 with qualified alternatives. Qualified names work; a bare name remains ambiguous even if only one Importer has inputs. Unknown, wrong-role, automatic-only and persisted disabled requests fail 39. No eligible interactive provider fails 41 without opening an empty selector.

With non-interactive stdin, zero-argument link/import and one-argument link fail 2 with direct-use guidance. Supplied empty link text fails 2. Explicit arguments never produce a selector. Both mutation commands reject `--json`/`--json=false`. No confirmation flag chooses a component or value.

From `$demo_dir` outside any Work, run `work status`, `work link manual/linker VALUE` and `work import manual/importer`; all fail 21 with guidance and no mutations. Supply complete mutation arguments here to distinguish missing context from missing-argument usage failures. Status with `$work_id` remains supported there. Extra arguments/unsupported flags fail 2.

## Q11 — Canonical validation, lookup boundaries and enablement retention (US1/US4; SC-001, SC-003)

Fixture cases: missing required snapshot field, bad lifecycle/timestamp, unsupported schema, unreadable enclosing snapshot, mismatching hinted identity, duplicate canonical IDs and symlinked snapshot file. Expect safe refusal or ignored untrusted hint according to the contracts; do not substitute another Work. Corrupt unrelated scan entries must not prevent finding an otherwise valid exact target.

Change workspace configuration without moving the existing Work. Status from within the old Work still works; outside identity lookup can use a verified index hint. With no usable hint and no configured old root, fail with guidance to enter that Work; do not create a new persistent location catalog.

Persist `packages[].enabled=false` with the test registry writer. Confirm manual and automatic extensions exclude that package, while installed inventory retains it. Reinstall the same source and repair the seed in dedicated fixtures: explicit disablement survives. Effective Starter/Locator/convention queries respect the same predicate and keep Locator policy references. Missing `enabled` and legacy ownerless entries remain enabled.

## Q12 — Concurrent changes do not target stale state (US2–US4; SC-002–SC-004)

Use synchronization barriers in the Go integration/unit harness, not sleep-based races:

1. Change an unrelated link between value collection and commitment; both the newer unrelated value and accepted target link survive.
2. Archive after manual selection but before link commitment; association fails without modifying the archive.
3. Archive while an Importer is running; when it finishes, no stale Work directory is recreated and staged output is removed.
4. Start archive while import holds its commitment lock; archive sees either the completed import or the fully rolled-back attempt.
5. Remove/disable or alter the selected provider before invocation/commitment; return unavailable-provider failure without fallback.
6. Cancel during copy or after applying output but before final commitment; all newly created destination entries are removed. Cancel immediately after successful commitment; report the committed result consistently.

Verify canonical snapshots, actual artifacts, current provenance and preexisting files in each case. The Windows race fixture must run its holding process outside the checkout to avoid unrelated OS current-directory deletion restrictions.

## Q13 — Interactive geometry, receipts and help (US4; SC-006)

Use existing PTY/VT harness sizes (80×24 and the repository's supported narrow/minimum viewport cases), resize during selection and input, navigate by keyboard, and filter where supported. Accepted receipts remain compact; rejected values and old errors never enter terminal history. Provider identity remains distinguishable when display names collide; secondary text truncates before identity.

Run `work --help` and individual command help. All three commands appear once in Inside a Work; status advertises `--json`; mutations retain current flag visibility and reject JSON. Bare `work` branding and its non-interactive usage behavior remain unchanged.

## Q14 — Streams, monochrome and debug (SC-005–SC-006)

Exercise human/JSON status, direct mutations and failures with redirected streams, `NO_COLOR=1`, `TERM=dumb`, and normal supported terminals. Status contains no controls; mutation stdout has only its contracted result; prompts/progress/receipts/diagnostics stay on UI/stderr. Link values, metadata and file content do not appear in mutation receipts/diagnostics.

Store a link containing control characters through a correctly supplied direct argument; human status escapes it to one safe line and JSON round-trips the exact value. `WORK_DEBUG=1` exposes bounded retained importer stderr/cause details once, while normal failures show a concise actionable diagnostic. F5 automatic warning output remains unchanged.

## Q15 — Complete demonstration and regression gate (SC-007)

Demonstrate status → link → import → status on a fresh Work, then demonstrate all three missing-context failures and explicit outside status. No manual snapshot editing is used in this product journey.

After implementation, run from the repository root:

```bash
make seed
make lint
GIT_CONFIG_GLOBAL=/dev/null go test ./...
make build
```

For focused iterations:

```bash
go test ./internal/workcontext ./internal/extension ./internal/staging ./internal/registry ./internal/projection ./internal/cli
go test ./tests/contract
go test ./tests/integration -run 'TestScripts/(status|link|import|manual_context_errors)$'
```

Run the existing CI sequence (`make seed-all`, `make lint`, `go test ./...`, `make build`) on Linux/macOS/Windows. Ensure no seed-dependent test was skipped. All F1/F2/F2.5/F3/F4/F5 quickstart demonstrations and existing CLI/IPC/presentation contracts remain green, including automatic start warning semantics and reference-only start output. Existing tests that pin exact diagnostic inventories/help command lists may be updated only for the documented additive F6 surface.


## Delivered acceptance evidence (2026-10-12)

All 62 tasks are implemented. The following automated evidence maps the delivered behavior to FR-001–FR-031, SC-001–SC-007 and Q1–Q15; the earlier milestone records in [tasks.md](tasks.md) retain focused verification details.

| Scenarios | Requirements / outcomes | Evidence |
| --- | --- | --- |
| Q1–Q3 | FR-001–FR-006; SC-001 | `internal/workcontext/*_test.go`, `internal/projection/lookup_test.go`, `tests/integration/status_purity_test.go`, `status.txtar`, `TestStatusOutputContract`: canonical/legacy/archive views, exact identity precedence, missing checkout, invalid/duplicate targets and 20 repeated reads per form with unchanged bytes/mtimes/inventories and launch tripwires. |
| Q4–Q5 | FR-013–FR-017; SC-002 | `internal/extension/publish*_test.go`, `internal/cli/link_test.go`, `link.txtar`, `TestManualLinkJourney`: exact values, one-key replacement, provenance, concurrent updates, cancellation and zero subprocesses. |
| Q6–Q9 | FR-018–FR-025; SC-003–SC-004 | `manual_import_protocol_test.go`, `internal/staging/*_test.go`, `internal/extension/import*_test.go`, `import.txtar`, `import_archive.txtar`, `TestManualImportJourney`: declared typed inputs/null/omission, one selected invocation, empty/nested output, IPC failures, whole-output refusal, cancellation, cleanup/rollback recovery and archive preservation. |
| Q10–Q11 | FR-007–FR-012, FR-026–FR-030; SC-003, SC-005 | Manual eligibility/input tables, registry/install/bootstrap/activation tests, `manual_context_errors.txtar`, `TestManualDiagnosticsContract`, `TestManualEmptySelectors`: retained disablement, ambiguity before inputs, grammar and context precedence, invalid target refusal and actionable unavailable/input guidance. |
| Q12 | FR-010, FR-016–FR-017, FR-020–FR-025; SC-002–SC-004 | Publication/import race tests and `TestManualSelectionRevalidated`: lock serialization, archive during execution, provider changes, fresh canonical inputs, bounded-copy cancellation, precommit rollback and late committed success. |
| Q13–Q14 | FR-026–FR-030; SC-005–SC-006 | `TestManualContextGeometry`, link/import PTY journeys, `manual_context_streams_test.go`, diagnostic border tests, help inventory and existing presentation geometry/stream tests: 40×10, 80×24, 160×50, navigation/filter/resize, redacted receipts, normal/NO_COLOR/TERM=dumb streams, exact JSON and debug-only execution/recovery detail. |
| Q15 | FR-031; SC-007 | `TestF6RoadmapJourney` creates and enriches a fresh Work through product commands, inspects outside, checks all three missing-context failures, one plugin launch and unchanged shell handoff, then preserves artifacts through archive. Full-suite F1 regression, F5 roadmap/external package demos, reference-only testscript, extension protocol and presentation-boundary contracts remain release gates. |

Scope audit: `go.mod`/`go.sum` are unchanged from develop; canonical snapshot schema and database version remain 3. The optional retained registry enablement field defaults to enabled for older data. No F7 management command, plugin API, execution timeout, lifecycle event or shell handoff behavior was added. Status avoids writable lookup/repair paths. Manual failures use the single diagnostic border; automatic extension failures retain warning policy.

Platform boundary: `.github/workflows/ci.yml` retains Ubuntu/macOS/Windows with `make seed-all`, lint, the full suite and host build. PTY tests are Unix-only; direct CLI/filesystem/IPC cases are portable. macOS and Windows runtime execution is unavailable on this Linux host and must be verified by that CI matrix. Cross-compilation is separate evidence and is not a runtime pass.


Final Linux verification passed: `make seed-all`, `make lint`, `GIT_CONFIG_GLOBAL=/dev/null go test -json ./...`, and `make build` (which runs `make seed`). Sandbox caches were redirected with `GOCACHE=/tmp/work-go-cache` and `XDG_CACHE_HOME=/tmp/work-tool-cache`. The suite passed 48 packages and 1,076 test/subtest cases, including F1/F5 demonstrations and presentation/IPC contracts. No seed-dependent tests skipped. The existing `TestAddRootsExpandsTilde` skipped because its temporary directory under the real home is outside the sandbox's writable roots.

Windows amd64 and macOS arm64 CLI, contract and integration test binaries cross-compiled successfully with `go test -c`; all five release seed targets built with `make seed-all`. macOS/Windows runtime and filesystem behavior remains for CI, as stated above.
