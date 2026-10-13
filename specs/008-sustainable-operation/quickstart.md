# Quickstart Validation: Sustainable Platform Operation

This guide describes validation **after F7 implementation**. New commands, packages, fixtures and named test groups below are planned outputs, not claims that they exist or pass today. Run from the repository root using a disposable Work home; never use personal plugin state for fault tests. All scenarios can run without external accounts or public network access.

## Prerequisites and disposable fixtures

Use the repository's Go 1.26 toolchain, Git, Make and a POSIX shell (Git Bash on Windows). Python 3 below only edits fixture JSON/constructs a file URL; portable automated coverage uses native Go helpers. Build host seed before tests to avoid skips.

```sh
make seed
F7_TMP=$(mktemp -d)
export WORK_HOME="$F7_TMP/home"
F7_WORK="$F7_TMP/work$(go env GOEXE)"
go build -o "$F7_WORK" ./cmd/work
mkdir -p "$F7_TMP/source" "$F7_TMP/repos" "$F7_TMP/linked-source"
cp tests/fixtures/plugins/admin-suite/plugin.json "$F7_TMP/source/plugin.json"
go build -o "$F7_TMP/source/locator$(go env GOEXE)" ./tests/fixtures/plugins/admin-suite
git init "$F7_TMP/repos/demo"
git -C "$F7_TMP/repos/demo" -c user.name=Fixture -c user.email=fixture@example.test commit --allow-empty -m initial
export WORK_ADMIN_FIXTURE_REPOSITORY="$F7_TMP/repos/demo"
```

Planned `admin-suite` fixture: package admin-suite, version release-blue, Locator clone accepting name and returning the path from WORK_ADMIN_FIXTURE_REPOSITORY, convention admin-team with prefix admin/{slug}. Its native executable also supplies test-only blocked/failure/invocation-log modes via the Go harness; it requires no runtime beyond the executable. It declares no fallback Starter, avoiding conflict with the seed. These fixture controls are not production CLI mutation controls.

Fixture source commits are local setup only. For remote-pinned checks use file:// transport:

```sh
git init "$F7_TMP/source"
git -C "$F7_TMP/source" add .
git -C "$F7_TMP/source" -c user.name=Fixture -c user.email=fixture@example.test commit -m release-blue
F7_REMOTE=$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve().as_uri())' "$F7_TMP/source")
```

Automated tests must allocate their own homes/trees and restore environment. They should hash content and record file/directory existence without traversing external linked sources or opening a writable Work database.

## Q1 — Pure reads, fresh home and legacy views

Before any install:

```sh
"$F7_WORK" plugin list --json
"$F7_WORK" repository locator list --json
"$F7_WORK" repository policy list --json
"$F7_WORK" repository root list --json
"$F7_WORK" plugin update --check --json
```

Expect empty inventory/collections and updates=[], exit 0; WORK_HOME still does not exist. From the demo repository, convention show --json gives no remembered choice without creating home state. Repeat each read/check 20 times before and after installation. Verify complete home-tree bytes/existence, installed content, convention memory, Works and temporary retrieval cleanup remain unchanged. Legacy cases cover unknown version, linked live-manifest drift, ownerless seed projection, disabled packages, missing files and malformed state. Tests: `go test ./internal/adminstate ./internal/pluginadmin ./tests/integration -run 'TestAdmin(Read|Purity|Legacy)'`.

## Q2 — Enablement without policy rewrites

```sh
"$F7_WORK" plugin install --embedded
"$F7_WORK" plugin install "$F7_TMP/source" --as local-admin
"$F7_WORK" repository policy add local-admin/clone
"$F7_WORK" repository policy list --json
"$F7_WORK" plugin disable local-admin
"$F7_WORK" plugin list --json
"$F7_WORK" repository locator list --json
"$F7_WORK" repository policy list --json
"$F7_WORK" plugin enable local-admin
```

Explicit embedded installation adds no policy entry even on fresh state. Disabling keeps the local-admin/clone reference and position but marks package-disabled; enabling restores availability. Repeat enable/disable with already-satisfied state and duplicate aliases. Add one unknown alias to a batch and expect usage/2 with no selected package changed. Fixture invocation logs prove disabled Starters/Locators/Linkers/Importers never run; shared conventions remain available with another enabled owner. Tests: `go test ./internal/pluginadmin ./internal/registry ./tests/integration -run 'TestAdminEnable|TestAdminDisable|TestAdminShared'`.

## Q3 — Explicit policy membership and deterministic resolution

```sh
"$F7_WORK" plugin install "$F7_REMOTE" --as remote-admin
"$F7_WORK" repository policy list --json
"$F7_WORK" repository policy add remote-admin/clone --before local-admin/clone
"$F7_WORK" repository policy move local-admin/clone --before remote-admin/clone
"$F7_WORK" plugin disable remote-admin
"$F7_WORK" repository policy replace remote-admin/clone local-admin/clone
"$F7_WORK" repository policy list --json
```

Remote install does not insert policy; explicit add/move defines order. Replacement can include a disabled installed provider. Seed unavailable/wrong-role references in disposable Go fixtures and prove move/remove/explicit retention work without provider execution. Duplicate canonical refs and ambiguous bare names fail without rewriting config. Resolution still stops on first valid/ambiguous result, continues only on no result/ineligibility, and stops on failure. Test: `go test ./internal/repoconfig ./internal/locator ./tests/integration -run 'TestAdminPolicy|TestChain'`.

## Q4 — Root parity and complete validation

```sh
mkdir -p "$F7_TMP/repos-second"
"$F7_WORK" repository root add "$F7_TMP/repos" "$F7_TMP/repos-second"
"$F7_WORK" repository root replace "$F7_TMP/repos-second" "$F7_TMP/repos"
"$F7_WORK" repository root list --json
"$F7_WORK" repository root remove "$F7_TMP/repos-second" "$F7_TMP/repos"
```

Compare direct and hub state. Root set can become empty; workspace root/Works do not move. Add/replace with a final missing/non-directory/workspace-overlapping path fail wholly. Verify repeated canonical paths deduplicate and failed multi-operand removal does not mutate the prior slice. Test: `go test ./internal/repoconfig ./tests/integration -run 'TestAdminRoot'`.

## Q5 — Check versions and apply exact candidates

Publish a different opaque version locally:

```sh
python3 - "$F7_TMP/source/plugin.json" <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[1])
m = json.loads(p.read_text())
m['version'] = 'release-green'
p.write_text(json.dumps(m, indent=2) + '\n')
PY
git -C "$F7_TMP/source" add plugin.json
git -C "$F7_TMP/source" -c user.name=Fixture -c user.email=fixture@example.test commit -m release-green
"$F7_WORK" plugin update --check local-admin remote-admin --json
"$F7_WORK" plugin update local-admin remote-admin --yes
"$F7_WORK" plugin list --json
```

Both managed origins detect the differing version and retain exact installed references/digests after application. Check-only leaves installed content/config/state byte-identical. Same-version changed code is unchanged; a lexically smaller different version is still available. Invalid manifest/runtime/name ownership is failed without replacement. Unknown pinned baseline requires reinstall; unknown linked/embedded baseline remains source-managed/not-applicable.

The harness pauses after candidate review, mutates source/HEAD and resumes application: installed content must be the original reviewed candidate. Test contained symlinks, escaping/absolute/cyclic links, escaping entrypoint and a symlinked manifest; pinned prepare rejects unsafe paths before reading them. Test: `go test ./internal/plugininstall ./internal/pluginadmin ./tests/integration -run 'TestAdminUpdate|TestAdminCandidate'`.

## Q6 — Batch, confirmation and dependency semantics

```sh
"$F7_WORK" plugin update --check --all
"$F7_WORK" plugin update --all local-admin --yes
"$F7_WORK" plugin update --yes
```

Each is usage/2 with no changes. A fully specified noninteractive apply without --yes also fails before changes. Use fixture candidates withdrawing a policy Locator: --yes alone is dependency/52, whereas explicit --remove-policy-references plus --yes permits only the computed references. Interactive approval names the same impact. For independent valid/invalid candidates, valid ones apply, every alias has an outcome, overall exit is 50 and only one terminal diagnostic appears. Cancellation retains already committed independent updates and marks remaining targets not-applied. Tests: `go test ./tests/contract ./tests/integration -run 'TestAdminBatch|TestAdminConfirmation|TestAdminDependency'`.

## Q7 — Whole-set removal and linked ownership

```sh
"$F7_WORK" plugin uninstall local-admin --yes
"$F7_WORK" plugin uninstall local-admin --remove-policy-references --yes
cp "$F7_TMP/source/plugin.json" "$F7_TMP/linked-source/plugin.json"
cp "$F7_TMP/source/locator$(go env GOEXE)" "$F7_TMP/linked-source/locator$(go env GOEXE)"
"$F7_WORK" plugin install "$F7_TMP/linked-source" --link --as linked-admin
"$F7_WORK" plugin uninstall linked-admin --yes
```

The first request refuses the existing dependency without changes; the second removes package and its refs together. Linked removal preserves all external source bytes. Entrypoint existence follows the shared prepared-install validators. For multiple selected targets, unknown alias or a recoverable late failure preserves the whole set/policy. Existing snapshots/links/artifacts/branches/checkouts and convention memory stay unchanged. Tests: `go test ./internal/pluginadmin ./tests/integration -run 'TestAdminUninstall|TestAdminLinked'`.

## Q8 — Transaction failures, interruption and reachable recovery

```sh
go test ./internal/adminstate ./internal/pluginadmin ./tests/integration -run 'TestAdminRecovery|TestAdminInterrupt|TestAdminRollback|TestAdminCleanup'
```

Injected checkpoints cover prepared journal, pending control, each root move and JSON save, committed marker, stable publication and cleanup. Precommit recoverable failures restore exact before-state; committed interruptions recover exact after-state. Failed compensation remains pending/recovery-required with concrete guidance. A pure list/check while pending changes nothing and returns 54; rerunning the interrupted direct mutation with required flags enters recovery preflight before baseline lookup, reports prior recovery separately, then prepares normally. Malformed/unsafe journals fail closed. Stable leftover committed journals are cleanup-only and cannot roll back later transactions. Postcommit cleanup errors explicitly report commitment and code 55.

## Q9 — Bundled lifecycle and upgrade

```sh
"$F7_WORK" plugin disable work-reference
"$F7_WORK" plugin list --json
"$F7_WORK" plugin enable work-reference
"$F7_WORK" plugin uninstall work-reference --yes
"$F7_WORK" plugin list --json
"$F7_WORK" plugin install --embedded
"$F7_WORK" repository policy list --json
```

No seed policy dependency was created by explicit install, so this uninstall needs no dependency flag; if a test explicitly includes the seed Locator first, require it. Deliberate removal survives later start/setup and a supported upgrade; missing Starter yields guidance without resurrection. Reinstall is offline and leaves policy unchanged. Legacy upgrade fixtures include install-meta policy_seeded, ownerless declarations, edited/empty policy, disabled seed, damaged installation and deliberate removal. Tests: `go test ./internal/bootstrap ./internal/adminstate ./tests/integration -run 'TestAdminSeed|TestAdminUpgrade'`.

## Q10 — Concurrent reads, writers and component use

```sh
go test ./internal/adminstate ./tests/integration -run 'TestAdminConcurrent|TestAdminInvocation|TestAdminEpoch'
```

Use synchronized checkpoints, never timing-only sleeps. Interleave reads with a transaction finishing entirely inside a read and with intermediate JSON load failure; detect epoch drift before returning any output. Race unrelated config edits, memory upserts, enablement/bootstrap/install and whole-set uninstall; preserve unrelated latest state. Hold a fixture subprocess on a pipe and attempt replacement/removal; alias content cannot change until invocation completes. Release administration before Work-lock publication, then race provider drift/pending state and verify safe rejection, existing automatic warnings and no deadlock. Contention times out with 56 using an injectable test timeout rather than a 30-second test sleep.

## Q11 — Unavailable convention memory across clones

```sh
"$F7_WORK" plugin enable remote-admin
git -C "$F7_TMP/repos/demo" remote add origin "$F7_TMP/repos/demo"
git clone "$F7_TMP/repos/demo" "$F7_TMP/clone-demo"
(cd "$F7_TMP/repos/demo" && "$F7_WORK" convention set admin-team)
(cd "$F7_TMP/clone-demo" && "$F7_WORK" convention show --json)
"$F7_WORK" plugin disable remote-admin
(cd "$F7_TMP/repos/demo" && "$F7_WORK" convention show --json)
```

Both clones now have the same origin identity and remembered choice. Disable/remove its only provider. Show retains the string with available=false and changes nothing. Interactive start requests an explicit available replacement even if only one exists; noninteractive start fails convention-unknown/38. Explicit set from either clone is seen by the other; prior Works remain byte-identical. Reenable an owner and verify availability returns without a memory rewrite. Tests: `go test ./internal/cli ./tests/integration -run 'TestAdminConvention'`.

## Q12 — Hubs, help, streams and credentials

```sh
"$F7_WORK" --help
"$F7_WORK" plugin --help
"$F7_WORK" repository --help
"$F7_WORK" plugin
"$F7_WORK" repository
"$F7_WORK" convention
```

The last three run in a real terminal for hub acceptance. Navigate install/toggle/check/apply/remove, policy/root operations and convention replacement by keyboard; compare effective state with direct equivalents. Review exact references then cancel, reject values and resize; terminal history retains only accepted safe receipts and one result. Repeat supported viewport/color modes, NO_COLOR and TERM=dumb. Redirect input/output to verify bare hubs fail usage/2 with zero controls. Fixture credentials must appear nowhere in normal text/JSON/review/errors/equivalent commands, including repository identity URLs. Tests: `go test ./tests/contract ./tests/integration -run 'TestAdminHub|TestAdminOutput|TestAdminCredentials|TestPresentBoundary'` (PTY cases run on Unix).

## Q13 — Release-candidate acceptance and full regression

```sh
make seed-all
make lint
go test ./...
make build
```

Run the existing three-OS CI matrix. Add the complete F7 demonstration and retain every F1–F6 demonstration. Native release fixtures cover clean offline first bootstrap (default policy seeded once), explicit embedded install (no insertion), supported upgrade, interruption/restart, real repositories with ambiguous local/remote branches, and index loss/rebuild from canonical snapshots using the existing reconcile test helper. No public reindex command is invented here. Validate host build and existing release platform asset coverage.

Success means SC-001–SC-008 and the acceptance matrix pass with no manual state editing. Save test results in the implementation-phase validation record; this planning guide does not itself certify the release candidate.
