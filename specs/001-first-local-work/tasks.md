---
description: "Task list for First Local Work (F1) implementation"
---

# Tasks: First Local Work (F1)

**Input**: Design documents from `specs/001-first-local-work/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Contract, integration, and fault-injection tests **are required** — `docs/roadmap.md` §4 makes them a cross-cutting exit gate for every slice, and the spec's Success Criteria (SC-002..SC-007) are stated as test outcomes. Unit tests are folded into each implementation task ("with table-driven tests"); contract/integration/rollback suites are their own tasks.

**Organization**: Tasks grouped by user story. Module: `github.com/gustaborges/work`. Go 1.26, single binary, system `git` as a subprocess.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: `[US1]`/`[US2]`/`[US3]` for user-story phases only

## Path Conventions

Single Go project at repo root: `cmd/work/`, `internal/<pkg>/`, `seed/`, `tests/`. Paths below are literal.

## Branching (git-flow)

`master` = released code only; **`develop`** = integration line; each phase = a short-lived
**`feature/`** branch off `develop`, merged back to `develop` at the phase checkpoint (see
`plan.md` → **Branching Strategy**). **Before the first task of a phase**, the implementing agent
runs `git switch develop && git pull && git switch -c <feature-branch>` and does all of that
phase's implementation commits there; it merges `--no-ff` to `develop` once the **Checkpoint**
passes and `make lint` + `go test ./...` are green. Never commit F1 implementation code directly
to `develop` or `master` (spec/plan/doc edits are exempt).

**One-time, before Phase 1**: `git switch -c develop master && git push -u origin develop`.

| Phase | Feature branch | Cut from |
|---|---|---|
| 1 | `feature/001-first-local-work-p1-setup` | `develop` |
| 2 | `feature/001-first-local-work-p2-foundational` | `develop` (after P1 merges) |
| 3 | `feature/001-first-local-work-p3-us1-first-work` | `develop` (after P2 merges) |
| 4 | `feature/001-first-local-work-p4-us2-guided-tui` | `develop` (after P3 merges) |
| 5 | `feature/001-first-local-work-p5-us3-recovery` | `develop` (after P3 merges) |
| 6 | `feature/001-first-local-work-p6-polish` | `develop` (after P5 merges) |

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project skeleton, toolchain, CI.

**Branch**: one-time `git switch -c develop master && git push -u origin develop`, then `git switch -c feature/001-first-local-work-p1-setup develop` before T001.

- [X] T001 Initialize Go module `github.com/gustaborges/work` (`go mod init`, `go 1.26`) and create the directory skeleton from plan.md: `cmd/work/`, `internal/`, `seed/starter/`, `seed/locator/`, `seed/manifest/`, `tests/contract/`, `tests/integration/`, `tests/fixtures/`
- [X] T002 Add and pin dependencies in `go.mod`: `github.com/spf13/cobra`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/huh`, `github.com/charmbracelet/lipgloss`, `modernc.org/sqlite`, `golang.org/x/term`, `golang.org/x/sys`, `github.com/rogpeppe/go-internal` (testscript); run `go mod tidy`
- [X] T003 [P] Create `Makefile` with targets: `seed` (cross-compile `seed/starter` + `seed/locator` for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 into `seed/dist/<goos>_<goarch>/`), `build` (depends on `seed`; `go build -o bin/work ./cmd/work`), `test` (`go test ./...`), `lint` (`gofmt -l`, `go vet`, `staticcheck`)
- [X] T004 [P] Create `.github/workflows/ci.yml`: matrix `ubuntu-latest`/`macos-latest`/`windows-latest`, steps `make seed && make build && make lint && go test ./...`
- [X] T005 [P] Append `bin/` and `seed/dist/` to `.gitignore`; add `cmd/work/main.go` stub that calls `internal/cli.Execute()`

**Checkpoint**: `make build` produces an (empty-behavior) `bin/work`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared infrastructure every user story depends on. No user-facing behavior yet.

**Branch**: `git switch develop && git pull && git switch -c feature/001-first-local-work-p2-foundational` (after Phase 1 merged to `develop`) before T006.

**⚠️ CRITICAL**: No user-story work starts until this phase is done.

### Cross-platform primitives

- [X] T006 [P] Implement `internal/workhome/workhome.go`: resolve root from `WORK_HOME` env or `~/.work`; `EnsureLayout()` creates `config/`, `plugins/`, `state/`, `state/locks/`; path accessors (`ConfigFile()`, `RegistryFile()`, `DBFile()`, `PluginsDir()`, `LockPath(name)`). With table-driven tests using a temp `WORK_HOME`.
- [X] T007 [P] Implement `internal/diag/diag.go`: `Category` enum + fixed exit codes + stable tokens + message templates exactly per `contracts/cli-work-start.md` / research R19 (`ok`,`usage`,`invalid-path`,`unusable-repo`,`no-base-branch`,`invalid-branch-name`,`branch-collision`,`destination-unavailable`,`bootstrap-failed`,`materialization-failed`,`cancelled`); `Error` type carrying category + user message; `func ExitCode(err error) int`. Test asserts the full table (codes + tokens) and that messages never interpolate secrets.
- [X] T008 [P] Implement `internal/atomicfile/` (`atomicfile.go` + `atomicfile_unix.go` + `atomicfile_windows.go`): `WriteFile(path, data)` = CreateTemp in `filepath.Dir(path)` → write → `Sync` → `Rename`; unix variant also fsyncs the parent dir; windows variant uses `MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH)` via `golang.org/x/sys/windows`. Tests: replace existing, crash-before-rename leaves target untouched, concurrent writers never yield a partial file.
- [X] T009 [P] Implement `internal/lockfile/` (`lockfile.go` + `lockfile_unix.go` (flock) + `lockfile_windows.go` (LockFileEx)): `Acquire(path) (release func(), err error)`, blocking with a timeout; advisory, process-scoped. Tests: second acquire blocks until release; stale lockfile is reusable.

### Git & config

- [X] T010 [P] Implement `internal/gitx/gitx.go`: `os/exec` wrapper, one entry per command in research R7 (`Version`, `IsWorkTree`, `IsBare`, `HasCommit`, `ForEachRef(patterns...)`, `CheckRefFormat(name)`, `ShowRefVerify(ref)`, `WorktreeList`, `WorktreeAdd(dir,branch,base)`, `WorktreeRemove(dir)`, `BranchDelete(name)`, `RevParse(rev)`, `MergeBase`); `Preflight()` errors if `git` absent or `< 2.5`. Tests create real temp repos and assert behavior on each.
- [X] T011 [P] Implement `internal/config/config.go`: `WorkConfig` struct (`workspace`, `repository_roots`, `repository_resolution.locators`); `Load()` / `Save()` (atomic via `internal/atomicfile`); malformed JSON → `diag` `bootstrap-failed` naming the file; `Save` never leaves a partial file. Round-trip + malformed tests.

### Plugin model, registry, IPC

- [X] T012 [P] Implement `internal/plugin/manifest.go`: `plugin.json` types + role-discriminated validation for all four roles exactly per `contracts/plugin-manifest.md` (required/allowed/forbidden field matrix; `accepts ⊆ {git_fetch_urls,name,query}` non-empty for locator; forbidden `invocation`/`type`/`capabilities`/`hooks`/`priority`/`score`; `conventions[]` = `{name, prefixes[]}` only). Table tests for every valid/invalid row in the contract.
- [X] T013 [P] Implement `internal/registry/registry.go`: `registry.json` read/write (atomic); `Component` + `Convention` entry types per `contracts/plugin-manifest.md`; `Upsert` keyed by `(alias,name)` (components) / `name` (conventions) — idempotent, never duplicates; `ByRole(role)`, `Conventions()`, `StarterFallback()` queries. Tests: upsert twice = one entry; query by role.
- [X] T014 [P] Implement `internal/ipc/ipc.go`: typed payloads `StarterInput`/`StarterResponse`/`LocatorInput`/`LocatorResponse` (per `contracts/ipc-*.md`); `Run(entrypointPath string, stdinJSON []byte) (stdoutJSON []byte, exitCode int, stderr string, err error)` — executes the entrypoint directly (no shell, no shebang reliance), one JSON in / at most one JSON out; treats non-zero exit and unparseable stdout as errors. Tests with a tiny echo helper binary.

### Seed reference package

- [X] T015 [P] Implement `seed/starter/main.go` — `local-path-starter` per `contracts/ipc-starter.md`: read `{"arg":...}` from stdin, `filepath.Abs` it, emit `{"repository":{"path":"<abs>"}}`; empty/missing/whitespace arg → exit non-zero + stderr, no stdout; ignore extra stdin fields; garbage stdin → exit non-zero.
- [X] T016 [P] Implement `seed/locator/main.go` — `filesystem-repository-locator` per `contracts/ipc-repository-locator.md`: read `{"repository":{name?,git_fetch_urls?,query?},"repository_roots":[...]}`, walk each root to a bounded depth, match by `name`/`query` (dir name) and `git_fetch_urls` (compare fetch URLs across all local remotes via native git, no normalization), emit `{"matches":[{"repo_path":"<abs>"},...]}` (all matches, no ranking keys); empty roots → `{"matches":[]}`; garbage stdin / unreadable root → exit non-zero.
- [X] T017 Create `seed/manifest/plugin.json` exactly as the reference block in `contracts/plugin-manifest.md` (alias `work-reference`, `local-path-starter` + `filesystem-repository-locator` (no `runtime`), `freeform` convention with `prefixes:["{slug}"]`)
- [X] T018 Implement `seed/embed.go`: `//go:embed dist/**`; `AssetsFor(goos, goarch string) (starter, locator []byte, err error)` and `ManifestJSON()` (embed `../manifest/plugin.json`); `ContentDigest()` = sha256 over the host-platform pair + manifest. (Requires `make seed` to have populated `seed/dist/`.)

### Bootstrap, projection, work state

- [X] T019 Implement `internal/bootstrap/bootstrap.go`: `EnsureSeed()` per research R11 — acquire `bootstrap.lock`; if registry already has `local-path-starter` + `filesystem-repository-locator` + `freeform` **and** `.install-meta.json` digest matches embed → no-op; else extract host-platform binaries to a temp dir, atomic-rename into `plugins/work-reference/`, write `plugin.json` + `.install-meta.json` (`origin:"embedded-seed"`, digest, `installed_at`), validate via `internal/plugin`, upsert `internal/registry` entries, add `work-reference/filesystem-repository-locator` to `config` `repository_resolution.locators` if absent. Depends on T011–T014, T018. Unit tests for the happy path + stale-digest re-seed + partial-dir repair.
- [X] T020 [P] Implement `internal/projection/projection.go`: open `modernc.org/sqlite` DB at `workhome.DBFile()`, `PRAGMA journal_mode=WAL`, `busy_timeout=5000`; `Migrate()` sets `user_version=1` and creates the `works` table + `UNIQUE(dir_path)` + `works_last_accessed` index per `data-model.md` §3; `Upsert(Work)`, `Get(id)`, `Delete(id)`, `List()`. Tests: migrate is idempotent; upsert/get/delete round-trip.
- [X] T021 [P] Implement `internal/work/state.go`: `State` / `WorkSection` types for `work-state.json` schema 1 per `contracts/work-state.schema.json`; `Read(path)`, `Write(path, State)` via `internal/atomicfile`; `Validate()` enforces the schema (required fields, enums `status=in-progress`, `start_mode=new`). Tests validate the schema's example and reject each missing-field / bad-enum case.
- [X] T022 Implement `internal/work/verify/verify.go`: `Check(workID) (Report, error)` per research R12 / `data-model.md` §3.2 — snapshot parses & schema-valid; `<dir>/worktree` HEAD == `work.branch`; branch started from `work.base_branch` (merge-base == base tip); `works` row matches snapshot on id/slug/status/branch/base/convention/paths. Depends on T010, T020, T021.

### CLI & TTY scaffold

- [X] T023 Implement `internal/cli/root.go` + wire `cmd/work/main.go`: Cobra root command `work`; `Execute()` maps returned `diag.Error` → `os.Exit(diag.ExitCode(err))`; register (empty for now) `start` and `shell-init` subcommands; `--json` persistent flag defined but only honored by read commands.
- [X] T024 [P] Implement `internal/tui/tty.go`: `IsInteractive() bool` = `term.IsTerminal(stdin)` **and** `term.IsTerminal(stdout)`; `MustInteractive()` helper returning `diag` `usage` when false. Test with pipe vs. pty.

**Checkpoint**: `bin/work` runs, resolves `~/.work`, can bootstrap the seed offline, has an empty `work start`. All foundational packages have green unit tests on all 3 OSes.

---

## Phase 3: User Story 1 - Create the first Work from a local clone (Priority: P1) 🎯 MVP

**Branch**: `git switch develop && git pull && git switch -c feature/001-first-local-work-p3-us1-first-work` (after Phase 2 merged to `develop`) before T025.

**Goal**: `work start <local-git-path>` (with the required choices supplied as flags, or collected interactively) materializes an isolated worktree on its own branch with a canonical `work-state.json` and a coherent `works` row, and — with shell integration active — leaves the session in the new checkout.

**Independent Test**: On a clean install with no network, run `work start <valid clone> --workspace <tmp> --base main --slug demo --prefix '{slug}' --yes`; confirm the worktree/branch/snapshot/db row are mutually coherent (`verify.Check` passes) and the source checkout is untouched. (quickstart S1–S3, S10.)

### Tests for User Story 1

- [X] T025 [P] [US1] Contract test `tests/contract/starter_test.go`: drive `seed/dist/<host>/starter` with every row of the `contracts/ipc-starter.md` test table (happy, relative arg, empty arg, missing arg, garbage stdin, extra fields); assert stdout JSON + exit code.
- [X] T026 [P] [US1] Contract test `tests/contract/locator_test.go`: drive `seed/dist/<host>/locator` with every row of the `contracts/ipc-repository-locator.md` test table (name match ×1/×2, no match, empty roots, non-origin fetch-url match, garbage stdin, unreadable root); assert no ranking keys ever appear.
- [X] T027 [P] [US1] Integration test `tests/integration/create_happy.txtar` (testscript): quickstart S1 flag form — assert stdout summary lines, on-disk layout (`worktree/` + `work-state.json` only), snapshot field values, `git rev-parse` in the worktree, source repo untouched, one `works` row, bootstrap artifacts present; run `verify.Check` after.
- [X] T028 [P] [US1] Integration test `tests/integration/create_base_branch.txtar`: quickstart S2 — fixture with divergent local `main` vs `origin/main`; assert the picker distinguishes them by short SHA and the branch starts from the exact chosen revision.
- [X] T029 [P] [US1] Integration test `tests/integration/create_offline.txtar`: quickstart S3 — run S1 under a no-network sandbox; identical success.
- [X] T030 [P] [US1] Integration test `tests/integration/shell_integration.txtar` + `tests/fixtures/fakeshell`: quickstart S10 — with integration, temp file receives the absolute worktree path; without, exit 0 + FR-023 notice on stderr with the real path, no false `cd` claim.

### Implementation for User Story 1

- [X] T031 [P] [US1] Implement `internal/reporef/reporef.go`: `RepositoryReference` type; `ValidatePath(raw) (absPath string, err error)` per research R14 — `~`-expand, `filepath.Abs`, `EvalSymlinks`, stat (dir + readable), `gitx.IsWorkTree`, reject `gitx.IsBare`, reject `!gitx.HasCommit`; distinct `diag` categories `invalid-path`(10) / `unusable-repo`(11); diagnostics quote only the supplied path. Table tests incl. spaces / non-ASCII / `..` / symlink / bare / empty repo.
- [X] T032 [P] [US1] Implement `internal/basebranch/basebranch.go`: `List(repo) ([]Choice, error)` via `gitx.ForEachRef("refs/heads","refs/remotes")` (drop `refs/remotes/*/HEAD`); `Choice{Refname, Short, Scope, ObjectShort}`; `Resolve(choices, flagValue)` matches `--base` (short or `remote/short`, ambiguous → `diag` `usage`); empty list → `diag` `no-base-branch`(12). Tests with local-only, remote-only, and homonym fixtures.
- [X] T033 [P] [US1] Implement `internal/convention/convention.go`: load catalog from `internal/registry`; `Interpolate(prefix, slug) string` (`{slug}` → slug); `DeriveName(conventionName, prefix, slug) (string, error)`; F1 resolves to `freeform` with prefix `{slug}` (no memorization — spec Assumptions). Tests.
- [X] T034 [P] [US1] Implement `internal/branchname/branchname.go`: `Validate(name)` via `gitx.CheckRefFormat("refs/heads/"+name)` → `diag` `invalid-branch-name`(13); `DetectCollision(repo, name)` checks local (`ShowRefVerify`), remote-tracking (`ForEachRef refs/remotes/*/<name>`), and worktree-bound (`WorktreeList`) → `diag` `branch-collision`(14) naming which kind. Tests for each kind + valid names.
- [X] T035 [P] [US1] Implement `internal/workspace/workspace.go`: `SuggestDefault()` = `~/work` (`%USERPROFILE%\work` on Windows); `Validate(path)` per research R8 (writable dir or creatable; not inside any `repository_roots`; an enclosing unrelated git repo is allowed); `Persist(path)` writes `config.workspace` + creates `<root>/in-progress/` + `<root>/archived/`. Tests.
- [X] T036 [US1] Implement `internal/starter/starter.go`: `Select(registry, arg)` → the single enabled `fallback` starter in F1 (no `pattern` eval needed; collision path is F4); `Invoke(component, arg)` via `internal/ipc` → parse `ipc.StarterResponse`; structurally-invalid response or non-zero exit → `diag` `unusable-repo`. Depends on T013, T014, T031. Tests against the built seed starter. **FR-018 (trust boundary):** `Invoke` surfaces only the typed `ipc.StarterResponse` fields (`repository` / `base_branch` / `start_modes` / `meta` / `links`); add a test feeding a Starter response with extra top-level keys and `work.*`-shaped fields (`id`, `status`, `branch`, `starter`) and assert none are read or forwarded.
- [X] T037 [US1] Implement `internal/shellintegration/shellintegration.go` + `internal/shellintegration/snippets/{bash.sh,zsh.sh,fish.fish,powershell.ps1}`: `WriteTargetPath(absWorktree)` writes to `$WORK_CD_FILE` when set; `ReportNoIntegration(absWorktree, detectedShell)` prints the exact FR-023 notice block from `contracts/shell-integration.md` to stderr; `Snippet(shell)` returns the embedded per-shell wrapper; `DetectShell()`. Tests: file-drop path, notice text, unknown shell.
- [X] T038 [US1] Implement `internal/cli/shellinit.go`: `work shell-init <bash|zsh|fish|powershell>` — print `shellintegration.Snippet(shell)` to stdout, exit 0; unknown/missing shell → `diag` `usage`(2) listing supported shells; read-only. Test each shell + the error case.
- [X] T039 [US1] Implement `internal/create/create.go`: the transactional orchestrator per research R10 / `data-model.md` §1.7. Steps with a LIFO compensation stack: (1) acquire `lockfile` on `sha256(workspace+repo+branch)`; (2) `gitx.WorktreeAdd(dir/worktree, branch, baseRefname)` ↩ `WorktreeRemove`+`BranchDelete`; (3) own `<dir>` ↩ `RemoveAll`; (4) `work.Write` snapshot (atomic); (5) **commit**: `projection.Upsert` ↩ `Delete`. Pre-step check: target `<dir>` not occupied → `diag` `destination-unavailable`(15). `WORK_FAIL_AT=lock|worktree|dir|snapshot|projection` injects a failure at the named step. On any error/cancel before commit → unwind + return `diag` `materialization-failed`(17) (or `cancelled`(20)). Depends on T006–T010, T020, T021. Unit tests drive each `WORK_FAIL_AT` and assert zero residue.
- [X] T040 [US1] Implement `internal/tui` prompts used when a value is omitted in an interactive terminal: `huh` wrappers in `prompts.go` — `SelectPrefix([]string)`, `InputSlug()` (inline validation: non-empty, no whitespace, no `..`, no leading `-`), `EditWorkspaceRoot(suggested)`, `ConfirmCreate(summary)`; plus the staged base-branch picker `SelectBaseBranch(ctx, []BaseBranchItem)` as a small Bubble Tea model in `basebranch_picker.go` (Remote/Local tabs, `/` filter, styled from huh's Charm theme — R15). Unit-test the validation funcs and the picker model; prompt wiring covered by T027/T030.
- [X] T041 [US1] Implement `internal/cli/start.go` (flag-driven happy path): `work start [SOURCE]` with `--workspace/--base/--slug/--prefix/--yes` per `contracts/cli-work-start.md`. Pipeline: `bootstrap.EnsureSeed` → `gitx.Preflight` (error → `diag` `bootstrap-failed`(16), message names the missing or too-old git) → `starter.Select`+`Invoke` (or direct path from SOURCE) → `reporef.ValidatePath` → prefix (freeform) → `convention.DeriveName` → `branchname.Validate`+`DetectCollision` → `basebranch.List`+`Resolve` → resolve/persist workspace root (T035; F1 rule: persist only when none configured, else `--workspace` present ⇒ `diag` usage) → `ConfirmCreate` (skipped by `--yes`) → `create.Run` → print the 3 stdout summary lines → `shellintegration.WriteTargetPath` or `ReportNoIntegration`. Slug/name validation runs before the base branch so the cheapest correction comes first; workspace-root resolution is deferred to just before confirm so a run rejected earlier never persists a root (SC-004). When all required values come from flags, no prompt is constructed. Depends on T031–T040.
- [X] T042 [US1] Wire `internal/work` ↔ `internal/projection` mapping in `internal/create`: build the `work.State` (id = ULID, `starter="local-path-starter"`, `branch_convention="freeform"`, timestamps) and the `projection.Work` row (adds `repo_name`, `dir_path`, `worktree_path`, `snapshot_path`) from one source so they cannot drift. Unit test asserts every shared field is equal. The test also asserts every governed `work.*` field is produced by the core alone (ULID `id`, `status=in-progress`, `start_mode=new`, `starter`, `created_at`/`last_accessed_at`) and that no Starter/IPC output can override them — FR-018.

**Checkpoint**: quickstart S1, S2, S3, S10 pass on all 3 OSes. MVP is demoable: clean install → `work start <path> --flags` → ready worktree. `verify.Check` green.

---

## Phase 4: User Story 2 - Start the same flow through the guided interface (Priority: P2)

**Branch**: `git switch develop && git pull && git switch -c feature/001-first-local-work-p4-us2-guided-tui` (after Phase 3 merged to `develop`) before T043.

**Goal**: `work` with no arguments opens a keyboard-navigable home that reaches the "Start a Work" journey; `work start` with no source prompts for the path; an already-configured workspace root is reused silently; non-interactive input with a missing required value fails with actionable guidance and never opens a TUI or mutates state.

**Independent Test**: In an interactive terminal, `work` → choose "Start a Work" → provide a local path → complete the same journey as US1 using only the keyboard. Separately, `work start --base main --slug x --prefix '{slug}' --yes </dev/null` exits 2 naming the missing `--slug`... (quickstart S9), and re-running with a configured root does not re-ask for it. (US2 scenarios 1–5.)

### Tests for User Story 2

- [X] T043 [P] [US2] Integration test `tests/integration/home_reachability.txtar` (pty via `creack/pty` helper in `tests/fixtures/`): `work` no-args opens the home; only "Start a Work" is listed; selecting it enters the path prompt; `q` exits 0 with no state change. (US2 #1, `contracts/cli-work-home.md`.)
- [X] T044 [P] [US2] Integration test `tests/integration/start_no_source.txtar` (pty): `work start` with no source prompts for a path; providing it converges to the same validations/guarantees as `work start <path>`. (US2 #2.)
- [X] T045 [P] [US2] Integration test `tests/integration/workspace_root_reuse.txtar`: first create configures the root; second create reuses it without asking; changing `config.workspace` then creating a third Work uses the new root and does **not** move the first two. (US2 #3, #4; FR-007.)
- [X] T046 [P] [US2] Integration test `tests/integration/non_interactive_missing.txtar`: quickstart S9 — stdin not a TTY + missing `--slug` → exit 2, stderr names the flag, no TUI, no `config` write, no Work state. Also assert missing `SOURCE` non-interactively → exit 2.

### Implementation for User Story 2

- [X] T047 [US2] Implement `internal/tui/home.go`: Bubble Tea model for the `work` home per `contracts/cli-work-home.md` — single entry "Start a Work"; ↑/↓/j/k + Enter + q/Ctrl-C; MUST NOT list `resume`/`archive`/`status`/`import`/`link`/`plugin`/`repository`/`convention`; quitting without a choice → exit 0. Unit-test the model's update/view for key handling.
- [X] T048 [US2] Update `internal/cli/root.go`: `work` with no subcommand → if `tui.IsInteractive()` launch `tui.home` (and dispatch the chosen journey), else print a one-line command summary to stderr and exit 2.
- [X] T049 [US2] Update `internal/cli/start.go` for interactive collection: when `tui.IsInteractive()`, any omitted required value is collected via its `internal/tui/prompts.go` wrapper (adding `InputPath()` for a missing `SOURCE`); an explicit flag skips **only** its own prompt; validations still run. When **not** interactive, any still-missing required value (`SOURCE`, `--base`, `--slug`, `--prefix`, `--yes`, and `--workspace` when no root configured) → `diag` `usage`(2) naming it, before any mutation or `config` write.
- [X] T050 [US2] Implement silent workspace-root reuse in `internal/cli/start.go` / `internal/workspace`: when `config.workspace` is set, use it without prompting or accepting `--workspace` (F1: `--workspace` with a configured root → `diag` usage directing the user to edit `config/work.json`); document that a later slice owns root changes. Covered by T045.

**Checkpoint**: US1 + US2 both pass independently. Every F1 journey is reachable from `work` with no arguments; scripts get deterministic non-interactive failures.

---

## Phase 5: User Story 3 - Recover from errors without leaving partial state (Priority: P3)

**Branch**: `git switch develop && git pull && git switch -c feature/001-first-local-work-p5-us3-recovery` (after Phase 3 merged to `develop`; rebase onto Phase 4 if US2 landed first — both touch `internal/cli/start.go`) before T051.

**Goal**: Invalid path, invalid/again-invalid branch name, incompatible branch collision, materialization failure at any boundary, and cancellation before confirmation each produce a clear, category-specific diagnostic and leave zero orphan branch / worktree / Work directory / snapshot / index entry. In interactive mode the user can fix the offending choice in the same flow.

**Independent Test**: Exercise invalid paths, rejected names, all three collision kinds, cancellations, and an injected failure at each materialization boundary; verify no orphan artifacts remain and (interactive) the flow allows correction without restarting the command. (quickstart S4–S8.)

### Tests for User Story 3

- [X] T051 [P] [US3] Integration test `tests/integration/invalid_path.txtar`: quickstart S4 — nonexistent path → exit 10; a plain directory → exit 11; a bare repo → exit 11; each with a category token on stderr, the supplied path only (no repo contents), and zero artifacts. Plus an empty repo (`no commit`) case → exit 11 and a repo with no branches → exit 12.
- [X] T052 [P] [US3] Integration test `tests/integration/invalid_slug.txtar`: quickstart S5 — `--slug 'has spaces'` and `--slug '..'` → exit 13 naming the offending slug; `git branch` in the source unchanged; no Work dir.
- [X] T053 [P] [US3] Integration test `tests/integration/branch_collision.txtar`: quickstart S6 — pre-existing local branch, pre-existing remote-tracking branch, and a branch already bound to another worktree each → exit 14 stating which kind; re-running the S1 command after a success → exit 14; no partial state in any case.
- [X] T054 [P] [US3] Integration test `tests/integration/rollback.txtar`: quickstart S7 — `WORK_FAIL_AT=worktree|snapshot|projection` each → exit 17; assert no `rollme` branch, no worktree entry, no Work dir, no `works` row; `config/work.json` and the seed install intact.
- [X] T055 [P] [US3] Integration test `tests/integration/cancel.txtar`: quickstart S8 — declining the confirm prompt → exit 20, zero artifacts; and SIGINT delivered before the commit step → rollback + exit 20.
- [X] T056 [P] [US3] Integration test `tests/integration/interactive_recovery.txtar` (pty): invalid path → re-prompt for another path (no exit); invalid slug → return to the slug prompt; collision → return to the slug prompt; then a valid choice completes — all without restarting `work start`. (US3 #1–#3, FR-013.)

### Implementation for User Story 3

- [X] T057 [US3] Harden `internal/create/create.go` rollback: guarantee the compensation stack unwinds LIFO on every early-return path and on `context` cancellation (SIGINT handler in `internal/cli`); the pre-commit occupied-`<dir>` check returns `destination-unavailable`(15) before step 2; add a test that randomly injects failure at each step 20× and asserts `verify`-style "no residue" invariants.
- [X] T058 [US3] Implement the interactive correction loop in `internal/cli/start.go`: on `diag` `invalid-path`/`unusable-repo` in interactive mode, re-run `InputPath()` instead of exiting; on `invalid-branch-name`/`branch-collision`, jump back to `InputSlug()` and re-run derivation + validation; each retry re-runs **all** affected validations. Non-interactive mode still exits with the code. Covered by T056.
- [X] T059 [US3] Finalize category-specific diagnostics across `internal/gitx` (preflight), `internal/reporef`, `internal/basebranch`, `internal/branchname`, `internal/bootstrap`, `internal/create`: every failure maps to exactly one `diag.Category`; messages name the offending user choice and category only; add a single test enumerating each category → exit code → token (the FR-025/FR-027 contract) and asserting no path/URL/env content leaks beyond the user's own input. `bootstrap-failed`(16) is exercised from both the `gitx.Preflight` path (git absent / < 2.5) and the seed extraction/registration path.

**Checkpoint**: All three stories pass independently. quickstart S4–S8 green; SC-003/SC-004 demonstrated by fault injection.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Branch**: `git switch develop && git pull && git switch -c feature/001-first-local-work-p6-polish` (after Phase 5 merged to `develop`) before T060.

- [X] T060 [P] Stress test `internal/bootstrap/stress_test.go`: quickstart S11 — loop `bootstrap.EnsureSeed()` 100× with 20 injected interruptions at rotating install checkpoints, a 12-goroutine concurrent run, and an orphan-staging sweep; assert exactly one registry entry for each of `local-path-starter` / `filesystem-repository-locator` / `freeform`, one `plugins/work-reference/` dir, no partial dir. (SC-007, FR-005.) *In-package (needs the unexported `installCheckpoint` hook) rather than `tests/integration/`; `bootstrap.go` gained the hook + a stale-staging sweep.*
- [X] T061 [P] Garbled-IPC contract cases in `tests/contract/garbled_test.go`: truncated JSON, unbalanced braces, non-UTF-8, NUL bytes, ~2 MiB garbage blob, array-not-object, trailing junk — both seed binaries exit non-zero, write no stdout, emit one stderr line, and finish inside a 10 s deadline. (roadmap §4 "Contrato de processo".)
- [X] T062 [P] Portability sweep: CI runs `make seed-all && make lint && go test ./... && make build` on `ubuntu-latest`/`macos-latest`/`windows-latest`; new tests use `filepath`, `os.DevNull`, `binaryNames()`, and `git fast-import` (all OS-portable). Validated green on the Phase 5 matrix; Phase 6 additions ride the same matrix. (RNF-6, SC-002, SC-006.)
- [X] T063 [P] Performance sanity test `tests/integration/performance_test.go`: builds a 1000-commit repo via one `git fast-import`, warms bootstrap, times only `create.Run` (worktree + snapshot + db) and asserts < 5 s (observed ~26 ms on Linux). (SC-001.)
- [X] T064 [P] `README.md`: requirements, build (`make seed && make build`), install + `~/.work` layout, `eval "$(work shell-init <shell>)"` setup, non-interactive + interactive `work start` walkthrough, flag table, transactional guarantee, and the exit-code table.
- [X] T065 [P] `docs/adr/adr-0018-work-shell-init.md` (Status: Proposta): ratifies the `work shell-init` name as the one exception to ADR-0017's grammar, the per-shell snippet contract, and the `WORK_CD_FILE` protocol for FR-022/FR-023.
- [X] T066 Ran `quickstart.md` S1–S12 on Linux; results in `specs/001-first-local-work/validation-log.md`. Windows left to the CI matrix. One deviation (D1): S8's piped-stdin example can't reach the confirm prompt — `quickstart.md` S8 corrected; no code defect, no separate issue.
- [X] T067 `make lint` clean (`gofmt -l` empty, `go vet`, `staticcheck`); removed dead `tests/fixtures/.gitkeep`; `--json` rejected on `work start` (exit 2) and now hidden from all help output until F1's first read command needs it.
- [X] T068 Build/install tooling: `Makefile` gains `install`/`uninstall` (honouring `PREFIX` default `~/.local` and `DESTDIR`), splits `seed` (host-only, the default `make build` path) from `seed-all` (full matrix) with a shared `seed-one`, and adds `build-all` plus `release` (per-platform `bin/release/<goos>_<goarch>/work`, each embedding only its own seed — ~20 MB vs the former 47 MB). `seed/embed_test.go` `TestAssetsForEveryReleasePlatform` now requires only the host pair and skips the rest unless `make seed-all` staged them. CI switched to `make seed-all && make lint && go test ./... && make build`. README/quickstart/plan updated; `go install` limitation documented. Supersedes the T004 CI command; real distribution (Homebrew, `--version`, release automation) remains a future feature (own spec + ADR).
- [X] T069 Guided-interview polish in `internal/cli/start.go`: reorder the interactive steps to slug → base branch → workspace root (slug/name validation, the cheapest correction, now precedes the base-branch scan; workspace resolution stays deferred to just before confirm per SC-004). `contracts/cli-work-start.md` interactive-flow steps renumbered to match; `plan.md`/`research.md` pipeline notes and the `README.md`/`quickstart.md` walkthroughs reordered. Tests: reordered pty assertions in `tests/integration/interactive_test.go`. (FR-008, FR-009, FR-013, SC-004, SC-008.)
- [X] T070 Correct bootstrap interruption recovery, base-branch picker edge cases, and release-seed matrix validation in `internal/bootstrap/`, `internal/tui/`, `seed/embed_test.go`, and `tests/contract/garbled_test.go`; add regressions for checkpoint recovery, empty filter behavior, empty picker construction, and partial matrix staging. (FR-005, SC-007, RNF-6.)

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: needs Setup. `make seed` (T003) must run before T018/T019 and before any contract test. **Blocks all user stories.**
- **US1 (Phase 3)**: needs Foundational. This is the MVP.
- **US2 (Phase 4)**: needs Foundational; builds on `internal/cli/start.go` from US1 (T041) — do US1 first, or coordinate on that file.
- **US3 (Phase 5)**: needs Foundational + `internal/create` (T039) and `internal/cli/start.go` (T041) from US1.
- **Polish (Phase 6)**: needs all target stories complete.

### Within Foundational

- T006–T014, T015–T016, T020, T021, T024 are mutually independent → parallel.
- T017 → T018 → T019 (seed manifest → embed → bootstrap); T019 also needs T011–T014.
- T022 needs T010, T020, T021. T023 needs T007.

### Within US1

- Tests T025–T030 are independent → parallel (T025/T026 need `make seed`).
- T031–T035 independent → parallel. T036 needs T031. T037 independent. T038 needs T037.
- T039 needs Foundational (T006–T010, T020, T021). T040 independent.
- T041 needs T031–T040. T042 needs T039 + T020 + T021.

### Within US2

- T043–T046 parallel. T047 independent. T048 needs T047. T049 needs T041 + T024. T050 needs T035 + T049.

### Within US3

- T051–T056 parallel. T057 needs T039. T058 needs T041 + T049. T059 touches multiple packages — do after T057/T058.

### Parallel opportunities

- Phase 1: T003, T004, T005 together.
- Phase 2: the whole primitives + git/config + plugin/registry/ipc + seed-binaries set (T006–T016) and T020/T021/T024 — up to ~14 tasks in parallel.
- Each user story's test tasks run in parallel; its `[P]` implementation tasks (independent packages) run in parallel.
- With multiple people: after Phase 2, one takes US1, and once T041 lands, US2 and US3 can proceed in parallel.

---

## Parallel Example: User Story 1

```bash
# Contract + integration tests for US1 (after `make seed`):
Task: T025 Contract test seed starter in tests/contract/starter_test.go
Task: T026 Contract test seed locator in tests/contract/locator_test.go
Task: T027 Integration test create happy path in tests/integration/create_happy.txtar
Task: T028 Integration test base-branch selection in tests/integration/create_base_branch.txtar
Task: T029 Integration test offline create in tests/integration/create_offline.txtar
Task: T030 Integration test shell integration in tests/integration/shell_integration.txtar

# Independent implementation packages for US1:
Task: T031 internal/reporef/reporef.go
Task: T032 internal/basebranch/basebranch.go
Task: T033 internal/convention/convention.go
Task: T034 internal/branchname/branchname.go
Task: T035 internal/workspace/workspace.go
Task: T037 internal/shellintegration/shellintegration.go
Task: T040 internal/tui/prompts.go
```

---

## Implementation Strategy

### MVP first (User Story 1 only)

1. Phase 1 Setup → Phase 2 Foundational (all of it — it is the walking-skeleton plumbing).
2. Phase 3 US1.
3. **STOP and VALIDATE**: quickstart S1–S3, S10 on all 3 OSes; `verify.Check` green; source repo untouched.
4. Demo: clean install → `work start <path> --flags` → ready isolated worktree, offline.

### Incremental delivery

1. Setup + Foundational → skeleton that bootstraps offline.
2. + US1 → **MVP**: create a Work from a local path (flag or interactive).
3. + US2 → guided TUI home + `work start` with no source + deterministic non-interactive failures.
4. + US3 → transactional recovery: every error path is category-specific and orphan-free.
5. + Polish → idempotency stress, portability sweep, docs, shell-init ADR.

### Notes

- `[P]` = different files, no dependency on an incomplete task.
- Contract/integration/rollback suites are first-class tasks (roadmap §4); unit tests ship inside each implementation task.
- The compensation-stack commit point is the `projection.Upsert`; nothing before it may be observable after a failure (SC-003).
- Commit after each task or logical group; keep every prior story's automated demo green (roadmap §4 Regression).
- git-flow, one `feature/` branch per phase (see **Branching (git-flow)** above / `plan.md`): cut it from `develop` before the phase's first task, commit the phase there, merge `--no-ff` to `develop` at the green checkpoint. Never commit F1 implementation code straight to `develop` or `master`.
