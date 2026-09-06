# Implementation Plan: First Local Work (F1)

**Branch**: spec/plan/doc work on `master`; F1 implementation uses git-flow — a `develop` integration branch and one `feature/001-first-local-work-p<n>-*` branch per phase (see **Branching Strategy**) | **Date**: 2026-09-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-first-local-work/spec.md`

## Summary

F1 delivers the walking skeleton of Work: `work start <local-git-path>` (and the equivalent journey from the `work` TUI home) takes an already-cloned repository, collects a workspace root, base branch, slug, and the `freeform` prefix, then materializes an isolated Git worktree on its own branch with a canonical `work-state.json` snapshot and a coherent SQLite lookup record. The journey runs on a clean install with no network: a self-contained official reference package (local-path fallback Starter + filesystem Repository Locator + `freeform` convention) is seeded through the normal install pipeline on first use. Creation is transactional up to full publication; any failure or cancellation before that point leaves zero orphan branches, worktrees, directories, snapshots, or records.

Technical approach: a single portable Go binary (Cobra CLI + Bubble Tea/`huh` TUI), shelling out to the system `git` for all ref/worktree operations, persisting each Work as an atomically-written JSON snapshot with a rebuildable `modernc.org/sqlite` projection. The seed package's two executables are Go programs cross-compiled per platform and embedded in the release binary via `//go:embed`, extracted at bootstrap with no interpreter dependency. Terminal repositioning uses an opt-in shell wrapper (`eval "$(work shell-init <shell>)"`) that reads a path the core writes to `$WORK_CD_FILE`; when the integration is absent the core reports that fact, prints the real worktree path, and explains how to enable it.

## Technical Context

**Language/Version**: Go 1.26 (toolchain present: go1.26.4). Single module `github.com/gustaborges/work`.

**Primary Dependencies**:
- `github.com/spf13/cobra` — command tree (`work`, `work start`, `work shell-init`).
- `github.com/charmbracelet/bubbletea` v2 + `github.com/charmbracelet/huh` + `github.com/charmbracelet/lipgloss` — TUI home and selection/confirmation prompts.
- `modernc.org/sqlite` — CGO-free SQLite driver via `database/sql` for the `work.db` projection.
- `golang.org/x/term` — interactive-terminal detection (stdin+stdout) to gate all TUI.
- System `git` (>= 2.5, needs `worktree add -b`, `check-ref-format --branch`) invoked as a subprocess — not a Go library.
- Standard library only for: atomic file writes (`os.CreateTemp` + `f.Sync` + `os.Rename` in the target dir), embedding (`embed`), process exec (`os/exec`), file locking via a tiny build-tagged wrapper over `flock`/`LockFileEx`.

**Storage**:
- Canonical: `<workspace>/in-progress/<repo>_<branch>/work-state.json` per Work (schema-versioned JSON, sections `work`/`meta`/`links`).
- Projection: `~/.work/state/work.db` (SQLite, `PRAGMA user_version`), one `works` row per Work, reconstructible from snapshots.
- Config (human-editable): `~/.work/config/work.json` (`workspace`, `repository_roots`, `repository_resolution.locators`).
- Generated state: `~/.work/state/registry.json` (component registry), `~/.work/plugins/<alias>/` (seed install), `~/.work/state/locks/`.
- `WORK_HOME` env var overrides `~/.work` (tests, isolation).

**Testing**:
- `go test` table-driven unit tests per `internal/*` package.
- Contract tests: golden stdin/stdout JSON + exit-code assertions against the built seed Starter and Locator binaries, with self-contained fixtures.
- Integration: `github.com/rogpeppe/go-internal/testscript` `.txtar` scripts driving the built `work` binary against throwaway git repos; fault injection (`WORK_FAIL_AT=<step>`) to assert transactional rollback; a `internal/work/verify` helper compares snapshot ↔ worktree ↔ git branch ↔ db row (FR-028).
- CI matrix: GitHub Actions `ubuntu-latest`, `macos-latest`, `windows-latest`.

**Target Platform**: Linux (amd64, arm64), macOS (arm64, amd64), Windows (amd64). Terminal-positioning contract (FR-022) officially supported on bash, zsh, fish (Linux/macOS) and PowerShell 7+ (Windows); every other environment (including `cmd.exe`) takes the FR-023 reporting path.

**Project Type**: Single-project CLI tool with a bundled subprocess plugin seed. Structure below.

**Performance Goals**: Not latency-critical. Budget from SC-001: clean install → ready worktree in < 3 min including human choices, so the machine portion (bootstrap + git worktree + snapshot + db) should stay well under ~5 s on a typical repo. Base-branch listing and collision checks are single `git for-each-ref`/`show-ref` calls — no per-branch subprocess.

**Constraints**:
- Offline: no network on the F1 journey (FR-026, SC-005). Bootstrap uses only embedded assets.
- No AI / no LLM on any path (FR-026, RNF-4).
- Deterministic: no hidden priority, fallback, or implicit policy mutation (roadmap §4).
- Core never loads plugin code in-process (ADR-0000); seed components run as subprocesses via the documented IPC contract even though they ship with the binary (ADR-0003).
- Transactional to full publication; config/bootstrap survive a failed creation (FR-020, spec Assumptions).
- Snapshot writes atomic; a partial write is never observable as canonical state (FR-017).
- No secret/repository-content leakage in diagnostics (FR-027).

**Scale/Scope**: Single user, single machine, tens of Works. F1 code surface: `work start` + `work` home + `work shell-init`; ~15 `internal/` packages; 2 seed binaries; no `work plugin|repository|convention` public surface (later slices).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unratified template (placeholder principles, no ratified version). There are therefore no project-specific constitutional gates to evaluate. In their place this plan is held to the **cross-cutting gates in `docs/roadmap.md` §4** and the spec's **Success Criteria**, treated as binding:

| Gate (roadmap §4) | How this plan satisfies it | Verified by |
|---|---|---|
| Determinism | No priority/score fields; single seed Locator in policy; branch name derived purely from convention+prefix+slug; collision → explicit prompt, never auto-resolve | Unit tests on `convention`, `starter`, `locator`; SC-004 |
| Integrity | Compensation stack unwinds branch/worktree/dir/snapshot/db LIFO; final publish = snapshot rename + db upsert both succeed; lockfile serializes same-branch attempts | `testscript` fault-injection suite; SC-003, SC-007 |
| Process contract | seed components: one JSON in / at most one JSON out / exit code; `runtime` absent → executed directly (ADR-0006); no progress/heartbeat messages | `ipc` contract tests with fixtures |
| Portability | `os.Rename` atomic-replace helper per-OS; explicit git invocation (no shebang reliance); OS+shell matrix fixed; CI on 3 OSes | CI matrix; SC-002, SC-006 |
| Auditability | `diag` typed categories name path/repo/name/collision/destination/bootstrap/materialization/shell; snapshot is self-contained; no repo content in errors | `diag` tests; FR-025, FR-027 |
| UX | Every F1 journey reachable from `work` home; explicit values skip only their own prompt; non-interactive missing value → actionable failure, never a TUI; `--json` on the read path | `testscript` interactive + non-interactive; FR-024, FR-029, RF-51 |
| Regression | F1 is the first slice; its automated demo becomes the baseline others must keep green | CI |

**Architectural-authority gates (PRD → ADR → ADD):**

- The core does **not** embed domain logic: local-path interpretation and filesystem location live in the seed **package**, installed through the normal pipeline, uninstallable like any plugin (ADR-0000, ADR-0003, RNF-1, RNF-7). ✅
- `work-state.json` is the single canonical authority; `work.db` is a projection only (ADR-0013, FR-016, FR-019). ✅
- Repository Reference resolution keeps origin interpretation (Starter) separate from local location (Locator); `path` present → core validates directly, no Locator run (ADR-0014, ADR-0016). ✅
- CLI surface stays within ADR-0017: F1 adds only `work start` and its options plus a read-only `work shell-init`. `work start` per-choice flags (`--workspace/--base/--slug/--prefix/--yes`) are additive options on an already-public command, required by RF-51/FR-008 and not governed away by ADR-0017 (which fixes the *administrative* grammar). `work shell-init` is read-only and prints to stdout; **Phase 0 records it as needing a follow-up ADR to ratify** — it is not `work init` (ADR-0017 forbids that name for a bootstrap command; this is a shell-snippet emitter). ⚠️ tracked, not a violation.

**Gate result: PASS.** No violations requiring Complexity Tracking.

### Post-design re-check (after Phase 1)

Re-evaluated after `research.md`, `data-model.md`, and `contracts/` were written. Still PASS:

- No new dependency or subsystem beyond what ADR-0009 already fixes; `huh`, `modernc.org/sqlite`,
  `testscript`, and `golang.org/x/{term,sys}` are all pure-Go and CGO-free (portability gate).
- The seed remains a subprocess package delivered offline via `//go:embed` — no domain logic
  entered the core (ADR-0000/0003 gate holds; see `research.md` R3, `contracts/plugin-manifest.md`).
- `contracts/work-state.schema.json` keeps `work.db` strictly a projection of the snapshot
  (ADR-0013 gate).
- The only surface additions remain `work start` options and read-only `work shell-init`
  (`contracts/cli-work-start.md`, `contracts/shell-integration.md`); the `shell-init`
  follow-up ADR is still the single tracked item, not a violation.
- All seven roadmap §4 cross-cutting gates now have a concrete contract or test harness behind
  them (see the table above → `contracts/` and `research.md` R18).

## Project Structure

### Documentation (this feature)

```text
specs/001-first-local-work/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output — decisions R1–R19
├── data-model.md        # Phase 1 output — entities & persisted state
├── quickstart.md        # Phase 1 output — runnable validation scenarios
├── contracts/           # Phase 1 output
│   ├── cli-work-start.md        # `work start` args/flags/exit codes/stdout/stderr
│   ├── cli-work-home.md         # `work` TUI home reachability contract
│   ├── ipc-starter.md           # seed Starter subprocess contract (stdin/stdout JSON)
│   ├── ipc-repository-locator.md# seed Locator subprocess contract
│   ├── shell-integration.md     # `work shell-init` + WORK_CD_FILE protocol (FR-022/FR-023)
│   ├── work-state.schema.json   # canonical snapshot JSON Schema (schema=1)
│   └── plugin-manifest.md       # plugin.json role validation rules used by bootstrap
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
go.mod                         # module github.com/gustaborges/work
Makefile                       # `make build` (host-only seed) / `make install` /
                               #   `make seed-all` / `make release`; `make test`
cmd/
└── work/
    └── main.go                # wires cobra root, delegates to internal/cli

internal/
├── cli/                       # cobra commands: root (TUI home), start, shellinit
├── tui/                       # bubbletea home model + huh prompt wrappers, isatty gating
├── diag/                      # typed diagnostic categories → stable exit codes + messages
├── workhome/                  # ~/.work layout resolution (WORK_HOME), dir creation
├── config/                    # work.json read/write (workspace, roots, resolution.locators)
├── workspace/                 # workspace-root suggest (per-OS) / validate / persist  (FR-006)
├── lockfile/                  # flock / LockFileEx wrapper (build-tagged _unix.go / _windows.go)
├── atomicfile/                # CreateTemp-in-dir + Sync + Rename replace helper (per-OS)
├── gitx/                      # git subprocess wrapper: refFormat, forEachRef, showRef,
│                              #   revListRoots, worktreeAdd, worktreeList, worktreeRemove,
│                              #   branchDelete, revParse, isBare, headBorn
├── plugin/                    # plugin.json schema + role-discriminated validation (ADD §4.1)
├── registry/                  # generated component registry: build from manifest, query, upsert
├── bootstrap/                 # seed extraction (embed → ~/.work/plugins) + install pipeline,
│                              #   idempotency + repair, adds seed Locator to policy
├── ipc/                       # subprocess contract types + exec (one JSON in / one JSON out / code)
├── reporef/                   # RepositoryReference type; direct path validation (ADD §7.1, FR-003)
├── starter/                   # specific-vs-fallback selection, collision prompt, invoke, parse resp
├── locator/                   # resolution policy chain: eligibility, invoke, dedup+validate,
│                              #   single/multi/none/error outcomes (ADR-0015)
├── convention/               # branch-convention catalog, prefix interpolation, name derivation
├── branchname/                # validate (via gitx.refFormat) + collision detection (FR-012)
├── basebranch/                # list local + remote-tracking refs with short SHA; staged Remote/Local tab select (FR-009)
├── work/                      # Work domain model, work-state.json (read/write via atomicfile),
│   └── verify/                #   coherence check: snapshot ↔ worktree ↔ git branch ↔ db (FR-028)
├── projection/                # sqlite work.db: open, migrate (user_version), upsert, query
├── create/                    # the transactional `work start` orchestrator + compensation stack
└── shellintegration/          # shell-init snippets (bash/zsh/fish/pwsh), WORK_CD_FILE emit,
                               #   FR-023 "session did not move" reporting

seed/
├── starter/                   # main: local-path fallback Starter (own tiny binary)
│   └── main.go
├── locator/                   # main: filesystem Repository Locator (own tiny binary)
│   └── main.go
├── manifest/plugin.json       # seed plugin.json (starter + repository-locator + freeform)
├── dist/                      # populated by `make seed` (host) / `make seed-all`: <goos>_<goarch>/{starter,locator}[.exe]
└── embed.go                   # //go:embed dist ; exposes per-GOOS/GOARCH asset lookup

tests/
├── integration/               # *.txtar testscript files (create, cancel, rollback, offline,
│                              #   invalid-path, invalid-name, collision, non-interactive, shell)
├── contract/                  # ipc golden-file tests for seed starter/locator
└── fixtures/                  # self-contained git bundles, fake-shell harness
```

**Structure Decision**: Single Go project (Option 1). The core binary is `cmd/work`; all logic sits under `internal/` in small role-focused packages that mirror the F1 pipeline stages (source → Starter → Repository Reference → path validation / Locator chain → prefix → slug → branch-name validation → base branch → workspace root → worktree → snapshot → projection → shell repositioning), each independently testable. `seed/` is a physically separate concern: two standalone `main` packages built ahead of the core and embedded as opaque platform binaries, so the core depends on them only through `internal/ipc` — never by import. `tests/` holds the cross-package contract and integration suites that the roadmap's cross-cutting gates require.

## Branching Strategy

F1 follows **git-flow**. `master` holds released code only; a long-lived **`develop`** branch is
the integration line, and each `tasks.md` phase is built on its own short-lived **`feature/`**
branch cut from `develop` and merged back to `develop` at the phase checkpoint.

The implementing agent (`/speckit-implement`) **MUST** create and switch to the phase's feature
branch *before* writing any code for that phase, and **MUST NOT** commit F1 implementation work
directly to `develop` or `master`. Spec/plan/contract/doc edits (e.g. `/speckit-analyze`
remediations) stay on `master` — the feature-branch rule covers implementation code only.

**One-time setup** (before Phase 1): `git switch -c develop master && git push -u origin develop`.

| Phase (tasks.md) | Feature branch | Cut from | Merges to |
|---|---|---|---|
| 1 — Setup | `feature/001-first-local-work-p1-setup` | `develop` | `develop` |
| 2 — Foundational | `feature/001-first-local-work-p2-foundational` | `develop` (after P1 merges) | `develop` |
| 3 — US1 First local Work | `feature/001-first-local-work-p3-us1-first-work` | `develop` (after P2 merges) | `develop` |
| 4 — US2 Guided interface | `feature/001-first-local-work-p4-us2-guided-tui` | `develop` (after P3 merges) | `develop` |
| 5 — US3 Error recovery | `feature/001-first-local-work-p5-us3-recovery` | `develop` (after P3 merges; rebase onto P4 if it landed first) | `develop` |
| 6 — Polish | `feature/001-first-local-work-p6-polish` | `develop` (after P5 merges) | `develop` |

Rules:

- **Naming**: `feature/001-first-local-work-p<n>-<short>` — the phase identifier stays in one
  hyphen-separated segment under `feature/` (no nested `feature/001-first-local-work/...`, which
  would D/F-conflict with a bare `feature/001-first-local-work`).
- **First action of each phase**: `git switch develop && git pull && git switch -c <feature-branch>`.
  If the previous phase is not yet merged, branch from its tip instead.
- **Merge gate**: a feature branch merges to `develop` (`--no-ff`) only after that phase's
  **Checkpoint** in `tasks.md` is met and `make lint` + `go test ./...` are green on the CI
  matrix (roadmap §4 Regression — every prior phase's automated demo stays green on `develop`).
  Each checkpoint is a coherent state: P1/P2 leave green tests with no behaviour change; P3 is the
  demoable MVP.
- **T041 contention**: Phases 4 and 5 both extend `internal/cli/start.go` (T041). A solo run
  does them in order (4 then 5). If both are in flight, whichever merges second rebases onto the
  first.
- **Release**: when Phase 6 is merged, F1 ships via `release/0.1.0` cut from `develop`, merged to
  `master` and tagged, then merged back to `develop` (standard git-flow release). The release
  step itself is out of F1's implementation scope — it is the hand-off, not a task.

## Complexity Tracking

No Constitution Check violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
