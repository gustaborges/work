# Quickstart and Future Validation: Marketplace Plugin Distribution

**Status:** Future implementation validation guide. Commands below describe target behavior, not results executed during planning. Run in the worktree repository root after implementation with a disposable Work home; never point failure fixtures at a real home.

## Prerequisites and build

Use the checkout's Go 1.26 toolchain, Git, supported host platform, and configured Git authentication for private sources. Tests construct disposable homes and local file:// Git remotes. Build the included native assets before executing the binary:

```sh
make build
```

Use the produced `bin/work`; choose a disposable HOME via the test harness or an isolated shell environment so existing state is untouched. Existing tests already provide isolated HOME, Git config, PTY, and failure hooks. Do not invent a new public WORK_HOME flag.

## Q1 — Minimal author catalog and offline browsing

Copy examples/marketplaces/single and examples/marketplaces/multiple into temporary local directories. The former includes root marketplace.json and plugins/sample/plugin.json, rather than a root-only plugin. Register the multiple example whose canonical namespace is demo:

```sh
bin/work plugin marketplace add ./examples/marketplaces/multiple --json
bin/work plugin marketplace list --json
bin/work plugin list --available --marketplace demo --json
```

Expect registered/ready, zero installations, exact offered identities, optional display name with canonical namespace, and zero local totals. Disconnect/remove a disposable source copy and repeat list; stored browsing remains available without writing persistent bytes. A catalogless root fails 57; add of its path fails usage/2 before acquisition.

## Q2 — Select one copy and qualify homonyms

```sh
bin/work plugin add sample@demo --json
bin/work plugin list --marketplace demo --json
bin/work plugin disable sample --marketplace demo
bin/work plugin enable sample@demo
```

Only sample is installed under its composed address; siblings and policy are unchanged. Register a personal fixture with another sample, install sample@personal, and exercise both provider-owned components/conventions/private publications. Delete author source/capture in the disposable test and verify managed-copy execution. Bare sample, qualified-plus--marketplace, old flags/commands, and direct service publication without resolved entry all fail. Exercise incompatible fallback and invalid managed-address conflicts separately.

## Q3 — Refresh, check, reinstall, and update

```sh
bin/work plugin marketplace upgrade demo --json
bin/work plugin update --check sample@demo --json
bin/work plugin add sample@demo --yes --json
bin/work plugin update sample@demo --yes
bin/work plugin update --all --yes
```

Use fixture publications to distinguish equal-version valid changed bytes (check unchanged, explicit add reinstalled), invalid equal-version bytes (failed), and different opaque versions (update available). Assert installed enablement/policy stay unchanged by refresh/check. Freeze reviewed candidates, move the source tip, then apply and prove the original candidate was used. Verify one revision acquisition per recorded source/branch group, exact browsed commit installation, unavailable-commit failure, entry path moves, withdrawn entries, and no-op byte/epoch preservation.

## Q4 — Whole-source removal and recovery

Use a fixture source with enabled, disabled, and withdrawn installed packages plus policy dependencies. Review remove, refuse it, and compare exact source/package/policy bytes. --yes without --remove-policy-references still fails dependency authorization. Then explicitly authorize:

```sh
bin/work plugin marketplace remove demo --yes --remove-policy-references --json
```

Expect one committed source/package set and removed IDs, no source access, preserved Works/convention memory, and reusable namespace without reassociation. Test every journal/control/move/image/commit/cleanup failure checkpoint: pre-commit restores before, post-commit completes after and reports cleanup pending honestly. Repeat while a package executes and a capture/prepare lease is live.

## Q5 — Purposeful builtin/default lifecycle

```sh
bin/work plugin marketplace list --json
bin/work plugin add work-reference@builtin --json
bin/work plugin remove work-reference@builtin --yes
bin/work plugin marketplace remove builtin --yes
```

Expect offline common-entry installation, deliberate individual removal and usage/2 for source removal. No list/refresh or changed binary offer resurrects content or changes installed copies. Use fresh disposable homes for fresh required setup, source-registration-before-first-start, explicit empty policy, explicit add, disabled and removed defaults. Source registration leaves policy unhandled; required setup seeds once only while policy_handled is false. Explicit policy choice (including empty) and explicit reference add handle policy without automatic insertion.

Only the current marketplace model is validated. Ordinary callers/fixtures are updated directly; no previous-executable, previous-format, linked-provider, adoption or migration suite is retained. Malformed/unsupported present state fails the strict current validator without conversion/deletion. Recover interrupted prepared/committed current journals. Remove/re-register a source with equal public names and verify retained choices/private values remain bound to their original registration instance.

## Q6 — Manager parity and presentation

```sh
bin/work plugin
bin/work plugin marketplace
```

Run every P01–P12 row against its direct-command equivalent in disposable homes, including source branch, all-source available view, source filtering, multiple installed selections, all-target checks/update, refresh all, same-version reinstall, dependencies, and complete-source removal. Cover S01–S12 at 100x30, 60x18, down/up resize, no color, long Unicode names, missing/empty catalogs, valid stored snapshots after refresh error, Back/cancel and stale async completion. Confirm candidate cleanup, preserved valid navigation/filter/selection, one terminal restoration, and exactly `Changes saved successfully.` at normal exit.

## Q7 — Official online delivery evidence

The owner supplies the actual official Git publication origin; record it in docs/marketplaces.md and the supported attribution source during implementation. A local work fixture proves ordinary source behavior but does not satisfy real publication. Verify an independently published valid work catalog using normal explicit registration/upgrade and whole-source remove, preserving builtin. Do not infer origin from the module URL/name or publish remotely as part of planning. Public delivery remains blocked until owner-confirmed origin and initial publication are evidenced.

## Q8 — Focused and release validation

After relevant implementation tasks, run focused tests, then once the feature is complete run the full gates:

```sh
go test ./internal/pluginid ./internal/marketplace ./internal/marketplaceadmin
go test ./internal/adminstate ./internal/pluginadmin ./internal/bootstrap
go test ./internal/registry ./internal/repoconfig ./internal/repoconv ./internal/convention ./internal/semconv ./internal/starter ./internal/extension
go test ./internal/present ./internal/cli ./tests/contract ./tests/integration
go test -race ./internal/adminstate ./internal/marketplaceadmin ./internal/pluginadmin ./tests/integration
make seed-all
make lint
go test ./...
make build
```

Record command outcomes and platform in implementation evidence. Existing CI must actually execute portable acceptance/current-format/path/recovery cases on Ubuntu/macOS/Windows, with Unix PTY on Ubuntu/macOS and appropriate race-tool support. `make release` cross-compiles supported release architectures but is not execution evidence. No future command outcome may be marked passed by this documentation-only workflow.
