# Quickstart: Future Validation of Agent Skills

**Status**: Implementation validation in progress; S0 approved. Commands are implemented. Record actual validation in validation.md; native OS and agent exercises are separate delivery gates.

## Prerequisites

- S0 verifies 009 merged into development baseline (`develop`), approves/promotes authorities and reconciles names/diagnostics. Plural examples below remain conditional until then.
- Go 1.26, system Git, existing supported build prerequisites; declared runtimes for text template examples. `make seed` before host test/build. Use explicit built binary path if `work` on PATH is another release.
- Use scratch HOME/USERPROFILE, CLAUDE_CONFIG_DIR, WORK_HOME and source directories in a test harness that restores the caller's environment. Never install into real agent roots for automated tests. Windows harness controls os.UserHomeDir inputs appropriately.

## Q1 — Offline scoped installation and payload parity

```sh
make build
sandbox=$(mktemp -d)
mkdir -p "$sandbox/marketplace"
bin/work skills install --dir "$sandbox/marketplace"
bin/work skills status --dir "$sandbox/marketplace" --json
```

Expect installed/current for both agents, one missing-catalog stderr note, identical complete vendor trees and no Work home creation. Compare all payload files plus marker; rerun install and assert unchanged with every payload mtime stable. Repeat global tests only under test-owned homes; test vendor selection and empty/relative/absolute Claude override. Delete only this owned sandbox after checking no operation/recovery remains.

## Q2 — Ownership and scope/path matrix

Use `go test ./internal/skillinstall -run 'Test(Targets|Ownership|Digest|Plan)'` after these tests are implemented. Cover absent/current/outdated/modified/foreign, malformed/future/wrong-skill marker, added/deleted/hidden assets, platform mode independence, linked parent/target/descendant, special nodes, unreadable marker/tree, Unicode/spaces/case aliases/overlaps. Ordinary modified/foreign is replaced only with force; unsafe/unreadable/changed-under-observation never is. Assert no initial-conflict payload changes across vendors.

CLI contract tests: `go test ./tests/contract -run TestSkills`. Neither/both scopes, unknown/empty operands/vendors, path-like skills, missing/file --dir, unknown flags and install --json fail before target writes. Root/setup/group help and static home follow [CLI contract](contracts/cli-work-skills.md).

## Q3 — Pure coherent status

```sh
bin/work skills status --dir "$sandbox/marketplace" --json
```

Run the implemented `go test ./tests/integration -run TestSkillsStatus` matrix. Snapshot directory existence, bytes and local coordination before/after, including absent paths, foreign/unreadable content and pending records. Expect one ordered non-null JSON array with nullable digests/safe error fields and stderr-only notes. Ordinary inspected states exit zero; structural/recovery/I/O rows fail but retain other rows. Synchronize status with publication checkpoints; max three attempts, no invented current and no lock files/repair.

## Q4 — Content, fixtures and contract drift

```sh
go test ./ai-skills/... ./internal/plugin ./internal/marketplace ./internal/pluginid ./internal/semconv ./internal/ipc
```

Verify two-field frontmatter, relative links and complete inventory. Route valid/invalid catalog/manifest examples separately, partial/IPC examples to their actual validators, complete scaffolds through BuildOffer and every-entry isolated selected install. No component execution in management fixtures.

In a disposable development checkout, independently perturb represented role rule, catalog key and IPC request/response field; each intended verification must fail. Restore edits, review guidance, run `go generate ./ai-skills/...` and `make skill-lock` only as deliberate local updates. CI equivalent runs tests and fails stale references/pin without modifying them. Compare clean generated/pin bytes before and after verification; a successful re-pin is not evidence of prose review.

## Q5 — Fault/restart/concurrency evidence

```sh
go test ./internal/skillinstall -run 'Test(Recovery|Apply|Lock)'
go test ./tests/integration -run 'TestSkills(Recovery|Concurrency|Install)'
```

Follow every forward/rollback/cleanup checkpoint in [installation contract](contracts/skill-installation.md), for absent and existing targets. Kill helper process/restart explicit install; assert exact old state before commit, verified new state after commit, accurate cleanup-pending and no unowned deletion. Cover malformed/path-injecting records, edited backups/new targets, unrecorded lookalikes, lock cancellation/timeout, overlapping target batches, parent/marker edits, status rename gap and later-target failure retaining earlier successes. Run native cases on Linux/macOS/Windows; explicitly record unavailable symlink capability while retaining ordinary path/type coverage.

## Q6 — Validation recipe and application-state independence

Use full multi-entry fixtures and separately tested POSIX/PowerShell recipes from delivered validate-and-publish.md. Provide alternative fallback Starters, valid early entry and later missing runtime/entrypoint. Verify exact expected source offer, each entry isolated, every attempt new homes, corrected-source retry recaptured, offline local operation and zero component invocations. Inject failure/cancellation, confirm all owned scratch homes removed and pre-existing WORK_HOME/environment/source bytes unchanged. For both skill commands compare explicitly configured Work home's existence/bytes/index before/after without opening a writable verifier.

## Q7 — Actual local-agent dogfood

After deterministic tests, in clean isolated Claude Code and Codex sessions, install into a temporary marketplace with --dir. Record actual discovery, invocation and reload results, then ask each agent to create a new catalog/plugin using only delivered skill/references/templates. Register and selected-install every output entry with no manual contract fixes. Record agent/version, Work revision, runtime prerequisites, outputs and failures in `specs/010-agent-skills/validation.md` during implementation.

Check Claude global discovery with CLAUDE_CONFIG_DIR override separately because the skills subpath is inferred from its configuration-root documentation. Claude uses `/work-plugin-builder`; `/reload-skills` is needed for a newly created top-level skill directory in-session. Codex uses `$work-plugin-builder`, auto-detects changes and can restart if absent. Verify duplicate-scope behavior without promising merges. Missing agent access leaves this task incomplete; parser fixtures cannot substitute for it.

## Q8 — Release gate

```sh
make seed-all
make lint
go test ./...
make build
```

Preserve existing CI OS matrix and prior journeys. Confirm host build needs no reference generation/network and skills text assets ship in release targets without seed cross-compilation additions. Review docs/agent-skills.md, marketplace authoring link, README command table, AGENTS/CLAUDE/PR maintenance rules and diagnostic/help snapshots. Ensure generated/pin verification is non-mutating, all S0 decisions/IDs are accepted/reconciled and Q1–Q7 have actual evidence. No commit, merge, PR or publish is implied by running these checks.
