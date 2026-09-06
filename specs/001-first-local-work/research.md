# Phase 0 Research: First Local Work (F1)

**Feature**: `specs/001-first-local-work/` · **Plan**: [plan.md](./plan.md) · **Date**: 2026-09-05

This document resolves every unknown in the plan's Technical Context and the three
contracts the roadmap (`docs/roadmap.md` §5) requires F1's plan to close. Each entry
follows: **Decision / Rationale / Alternatives considered**.

Governing sources: `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md`,
ADR-0000, ADR-0002, ADR-0003, ADR-0004, ADR-0006, ADR-0009, ADR-0011, ADR-0012,
ADR-0013, ADR-0014, ADR-0015, ADR-0016, ADR-0017.

---

## R1 — Supported OS + shell matrix (roadmap §5.2, RNF-6)

**Decision.**
- **Operating systems (CI-enforced):** Linux `amd64` + `arm64`, macOS `arm64` + `amd64`, Windows `amd64`.
- **CI:** GitHub Actions matrix `ubuntu-latest`, `macos-latest`, `windows-latest`. `arm64` Linux/macOS covered by cross-compiled build + a best-effort runner where available; not a merge blocker in F1.
- **Shells covered by the terminal-positioning contract (FR-022):** `bash`, `zsh`, `fish` on Linux/macOS; `PowerShell` 7+ on Windows.
- **Everything else** (`cmd.exe`, `nu`, `xonsh`, non-interactive pipelines, no integration installed): the FR-023 reporting path — Work is still created, the command prints that the session did not move, prints the absolute worktree path, and prints the enable instruction.
- **Git requirement:** system `git` >= 2.5 on `PATH` (needs `git worktree add -b`; `check-ref-format --branch` exists since 1.7). Checked in preflight before any mutation with a specific, actionable message.

**Rationale.** These three OSes are what GitHub Actions provides first-class and cover the PRD's audience (local agentic dev). Fixing the shell list makes the shell-integration contract testable rather than open-ended. Delegating to system `git` (see R7) means the git floor is low and universally satisfied.

**Alternatives considered.**
- *Support `cmd.exe` positioning* — rejected: `cmd.exe` has no robust "run then cd from output" idiom without a batch wrapper users won't install; PowerShell is the modern Windows default and enough for F1.
- *Vendor a git implementation / go-git* — rejected, see R7.
- *No Windows in F1* — rejected: RNF-6 and SC-002 name "every officially supported OS/shell"; deferring Windows would hide the portability cost the roadmap wants surfaced now.

---

## R2 — Terminal-positioning contract (roadmap §5.1, RF-9, FR-022, FR-023)

**Decision.** An **opt-in shell wrapper** installed by the user, plus a **file drop** channel from the core.

1. `work shell-init <bash|zsh|fish|powershell>` prints a shell snippet to stdout (read-only, no state change). The user adds `eval "$(work shell-init bash)"` (or the shell's equivalent) to their rc file.
2. The snippet defines a `work` function that:
   - creates a private temp file, exports `WORK_CD_FILE=<that path>` and `WORK_SHELL_INTEGRATION=1`,
   - execs the real binary (`command work "$@"` / absolute path) inheriting stdio,
   - on return, if `WORK_CD_FILE` is non-empty, `cd`s to its contents, then deletes it.
3. The core, only on a **successful** `work start` (and later `work resume`), writes the absolute worktree path to `$WORK_CD_FILE` when that env var is set.
4. If `WORK_CD_FILE` is unset/empty (integration absent) the core takes the **FR-023 path**: exit 0, Work-created success message on stdout, and a stderr notice — "this shell session was not moved into the new worktree", the absolute worktree path on its own line, and the one-liner to enable integration for the detected shell (`$SHELL` / `$PSVersionTable`). It never claims a `cd` happened.

**Rationale.** A child process cannot change its parent shell's cwd; every tool that does this (`direnv`, `zoxide`, `jj`, `pyenv`, `fnm`) ships a shell hook. The **temp-file** channel (vs. writing `cd ...` to a captured fd or stdout) keeps human stdout clean, needs no reserved file descriptor, and works identically in PowerShell. Opt-in (not auto-installed) respects RNF-7 / ADR-0017's "no `work init`" — the user chooses to add the hook.

**Alternatives considered.**
- *Emit `cd <path>` on stdout, user does `eval "$(work start ...)"`* — rejected: destroys normal interactive stdout (progress, the success summary, TUI) and is fragile with the TUI.
- *Reserved file descriptor 3* — rejected: awkward to set up in fish/PowerShell; temp file is universal.
- *Spawn a replacement shell as a child (`os.Exec` a subshell in the worktree)* — rejected: nests shells, breaks job control, loses the parent's history/session.
- *`work init` that edits rc files* — rejected by ADR-0017 (`work init` not public) and it is intrusive; `shell-init` only prints.

**Follow-up.** `work shell-init` is a new public command. It is read-only and outside the administrative grammar ADR-0017 governs, but a short ADR should ratify it (name, per-shell snippets, the `WORK_CD_FILE` protocol) before v1. Tracked in the plan's Constitution Check.

---

## R3 — Self-contained reference-package format (roadmap §5.3, FR-004, RF-10, ADR-0003, ADR-0006)

**Decision.** The seed package's two executable components are **Go programs, cross-compiled per platform, embedded in the release binary** and extracted at bootstrap.

- `seed/starter/` and `seed/locator/` are independent `main` packages.
- `make seed` compiles them for the **host** platform into `seed/dist/<goos>_<goarch>/{starter,locator}` (`.exe` on Windows); `make seed-all` does every target in R1.
- `seed/embed.go` embeds `seed/dist` via `//go:embed`; a lookup returns the pair matching the running `runtime.GOOS`/`GOARCH`. A default `make build` therefore carries only the host pair.
- Release builds are **per-platform** (`make release`, later GoReleaser): each `work` artifact embeds only its own OS/arch seed pair, so size overhead is ~2 small static binaries.
- At bootstrap (R11) the core writes, through the normal install pipeline: `~/.work/plugins/<alias>/source/{starter,locator}`, `~/.work/plugins/<alias>/plugin.json` (**no `runtime` field** → executed directly per ADR-0006), `.install-meta.json` (origin = `embedded-seed`, content digest), and the generated registry entries.
- `plugin.json` also carries the `freeform` convention (`prefixes: ["{slug}"]`) — pure manifest data, no executable.

**Rationale.** RF-10 forbids any external tool or network on first use; ADR-0006 forbids relying on shebang / a declared interpreter that might be absent (notably on Windows). A statically-linked Go binary with no `runtime` declared satisfies both by construction, and the core already builds Go binaries so there is no new toolchain. ADR-0003 requires the seed to go through the *same* install pipeline as any plugin — the embed is just the offline delivery of the bytes.

**Alternatives considered.**
- *Ship seed as `runtime: python3` / `sh` scripts* — rejected: violates RF-10/ADR-0006 (no guaranteed interpreter, especially Windows).
- *Implement local-path + filesystem logic inside the core behind a flag* — rejected: violates ADR-0000/ADR-0003/RNF-1/RNF-7 (domain logic in the trust boundary, not uninstallable).
- *Download the seed on first run* — rejected: violates FR-026/SC-005 (offline).
- *One fat binary embedding all platforms* — rejected: needless size; per-platform release is standard with GoReleaser.

---

## R4 — Atomic snapshot write (FR-017, ADR-0013)

**Decision.** `internal/atomicfile.WriteFile(path, data)`:
1. `os.CreateTemp(filepath.Dir(path), ".work-state-*.tmp")`,
2. write all bytes, `f.Sync()`, `f.Close()`,
3. `os.Rename(tmp, path)` — replaces atomically on POSIX; on Windows use a small `_windows.go` variant calling `MoveFileEx(MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH)` via `golang.org/x/sys/windows` (Go's `os.Rename` already does replace-existing but not write-through),
4. best-effort `fsync` of the parent directory on POSIX (`_unix.go`).
On any error before step 3 the temp file is removed and no canonical file is touched.

**Rationale.** Temp-in-same-dir + rename is the portable atomic-publish pattern and keeps the rename on one filesystem. Directory fsync guarantees the rename survives a crash on ext4/xfs. This is the mechanism ADR-0013 already prescribes ("escrita temporária seguida de rename atômico").

**Alternatives considered.**
- `github.com/google/renameio/v2` — POSIX-only; would still need a Windows path, so a ~30-line in-repo helper is simpler and has no dependency.
- Write-in-place + `O_TRUNC` — rejected: a crash mid-write leaves an invalid canonical file (violates FR-017).
- Lockfile instead of atomic rename — rejected: doesn't help a crash; used *additionally* only to serialize concurrent *creations* (R10), not snapshot writes.

---

## R5 — SQLite projection driver (ADR-0009)

**Decision.** `modernc.org/sqlite` (pure Go, CGO-free) through `database/sql`. DB at `~/.work/state/work.db`. Schema version in `PRAGMA user_version`; F1 = version 1 with one `works` table (the "global lookup record" / projection). `PRAGMA journal_mode=WAL`, `busy_timeout=5000`. The projection is written *after* the snapshot is durable and is always reconstructible from snapshots (full rebuild is F2; F1 only needs coherent create/update).

**Rationale.** ADR-0009 mandates a pure-Go embedded SQLite with no CGO. `modernc.org/sqlite` is the de-facto standard for that, `database/sql`-compatible, actively maintained. WAL + busy_timeout covers the rare concurrent-write case without app-level DB locking.

**Alternatives considered.**
- `zombiezen.com/go/sqlite` (also pure Go) — nicer API but smaller ecosystem and not `database/sql`; no advantage for a one-table projection.
- `mattn/go-sqlite3` — CGO, explicitly rejected by ADR-0009.
- Loose JSON index file — rejected by ADR-0009 (ordered `last_accessed_at` listing without filesystem scan).

---

## R6 — CLI / TUI frameworks (ADR-0009, PRD §10, FR-029, RF-51)

**Decision.**
- **Cobra** for the command tree: `work` (no args → TUI home), `work start [SOURCE]`, `work shell-init <shell>`. Persistent `--json` wired only on read commands.
- **Bubble Tea v2** + **Lipgloss** for the `work` home model (a simple menu; F1 lists only "Start a Work").
- **`huh`** (charmbracelet) for each individual prompt in `work start`: base-branch select, prefix select, slug text input with inline validation, and the final confirm. `huh` is built on Bubble Tea and removes hundreds of lines of hand-rolled model code.
- **Interactive detection:** `golang.org/x/term.IsTerminal` on *both* stdin and stdout. If either is not a TTY the process is non-interactive: no TUI is ever constructed; a missing required value fails with usage guidance and a stable exit code (FR-024, RF-51).

**Rationale.** ADR-0009 fixes Cobra + Bubble Tea. `huh` is the idiomatic 2025+ way to build exactly the kind of discrete selection/confirm prompts F1 needs, all keyboard-navigable (PRD §10), and it shares the Bubble Tea renderer so the home and the prompts look consistent.

**Alternatives considered.**
- Hand-rolled Bubble Tea models per prompt — rejected: more code, same result; `huh` is maintained by the same org.
- `survey` (AlecAivazis) — rejected: not Bubble Tea, inconsistent styling, less active.
- `promptui` — rejected: effectively unmaintained.

---

## R7 — Git invocation strategy (ADR-0000, ADD §8)

**Decision.** Shell out to the system `git` binary via `os/exec` from `internal/gitx`; never a Go git library. All commands run with `-C <repo>` (or `--git-dir`/`--work-tree` for the worktree). Commands F1 uses:

| Purpose | Command |
|---|---|
| Repo usable? | `git -C P rev-parse --is-inside-work-tree` / `--git-dir` |
| Bare? | `git -C P rev-parse --is-bare-repository` |
| Has a commit? | `git -C P rev-parse --verify --quiet HEAD` (unborn → fail) |
| List base branches | `git -C P for-each-ref --format '%(refname) %(objectname:short) %(HEAD)' refs/heads refs/remotes` (drop `refs/remotes/*/HEAD`) |
| Convention identity (future) | `git -C P config --get remote.origin.url`, `git -C P rev-list --max-parents=0 HEAD` |
| Branch-name syntax | `git check-ref-format "refs/heads/<name>"` |
| Local collision | `git -C P show-ref --verify --quiet "refs/heads/<name>"` |
| Remote-tracking collision | `git -C P for-each-ref "refs/remotes/*/<name>"` |
| Worktree collision / list | `git -C P worktree list --porcelain` |
| Create branch + worktree | `git -C P worktree add -b <name> <worktree-path> <base-refname>` |
| Rollback worktree | `git -C P worktree remove --force <worktree-path>` |
| Rollback branch | `git -C P branch -D <name>` |

Preflight: `git --version` parsed once; error if `git` absent or < 2.5.

**Rationale.** ADR-0000 says git is the one tool the core invokes directly; ADD §8 explicitly wants native git (`git check-ref-format --branch`, ref queries) rather than a parallel reimplementation of ref rules. go-git has no `worktree add` and incomplete `check-ref-format` semantics. `git worktree add -b` creates the branch and the worktree in one call, and its own failure (e.g. branch exists) is a clean rollback boundary.

**Alternatives considered.**
- `go-git` (`github.com/go-git/go-git`) — rejected: no linked-worktree support, would force reimplementing ref-format validation, contradicts ADD §8.
- `libgit2`/`git2go` — rejected: CGO, contradicts the no-CGO posture of ADR-0009.
- Re-implement `check-ref-format` in Go — rejected explicitly by ADD §8.

---

## R8 — Workspace-root default suggestion (FR-006, RNF-6, ADD §3)

**Decision.** Suggested default on first use: **`~/work`** on every platform (`%USERPROFILE%\work` on Windows). The user may accept or type another path. Validation before persisting: path (after `~` expansion + `filepath.Abs` + symlink resolve) is creatable/writable, is a directory (or does not exist and its parent is writable → created), is **not** inside any configured `repository_root` and not inside a git work tree, and is not a file. On accept the core creates `<root>/in-progress/` and `<root>/archived/` and writes `workspace` to `~/.work/config/work.json`. Later changes affect only new Works (FR-007) — no migration of existing ones.

**Rationale.** The spec Assumptions allow a per-platform value as long as it is "explicit, writable, persisted, and separate from the source clones". A single memorable rule (`~/work`) beats platform-specific `XDG`/`Library` paths for a directory the user will `cd` into daily; keeping worktrees out of the `~/.work` state dir avoids mixing user-facing checkouts with tool internals (ADD §3 keeps them separate anyway).

**Alternatives considered.**
- `$XDG_DATA_HOME/work/workspaces` (Linux), `~/Library/Application Support/...` (macOS) — rejected: buried, hostile to daily `cd`.
- `~/.work/workspaces` — rejected: mixes user checkouts into the hidden state dir.
- No default, always ask — rejected: FR-006 requires a *suggested* value.

---

## R9 — `~/.work` home layout & config format (ADD §5, ADR-0002)

**Decision.** Root = `WORK_HOME` env var if set (tests/isolation), else `~/.work`.

```text
~/.work/
  config/
    work.json          # human-editable
  plugins/
    <alias>/           # seed install (alias: "work-reference" by default)
      plugin.json
      source/          # extracted seed binaries
      .install-meta.json
  state/
    work.db            # sqlite projection
    registry.json      # generated component registry
    locks/             # lockfiles for concurrent-creation serialization
```

`work.json` (F1 keys only):
```jsonc
{
  "workspace": "/home/user/work",
  "repository_roots": [],
  "repository_resolution": { "locators": ["work-reference/filesystem-repository-locator"] }
}
```

**Rationale.** Matches ADD §5 layout and ADR-0002's config-vs-generated-state split (`work.json` hand-editable; `registry.json` + `.install-meta.json` are generated, never hand-edited). `WORK_HOME` makes every test hermetic.

**Alternatives considered.**
- Registry inside `work.db` — rejected: ADR-0002 keeps the registry in generated state; a JSON file is trivially inspectable and rebuildable, and F1's registry is tiny.
- TOML/YAML config — rejected: JSON matches `work-state.json` and needs no extra dependency.

---

## R10 — Transactional creation & rollback (FR-020, FR-021, SC-003, roadmap §4 Integrity)

**Decision.** `internal/create` runs an ordered pipeline; each side-effecting step pushes a compensating closure onto a LIFO stack:

| # | Step | Compensation |
|---|---|---|
| 1 | acquire lock `state/locks/<sha256(workspace+repo+branch)>.lock` | release lock |
| 2 | `git worktree add -b <branch> <dir>/worktree <base>` | `git worktree remove --force` then `git branch -D` |
| 3 | create `<dir>` scaffolding already implied by step 2 target; ensure `<dir>` owned by this attempt | `os.RemoveAll(<dir>)` |
| 4 | write `work-state.json` (atomic, R4) | covered by step 3's `RemoveAll` |
| 5 | **publish**: upsert `works` row in `work.db` | delete `works` row |

The **commit point** is after step 5 succeeds. Any error or user cancellation before that unwinds the stack top-down and returns a `diag` error (or exit 20 for cancel). Steps that touch `~/.work/config` (workspace root) and bootstrap are **not** on the stack — they persist across a failed creation (spec Assumptions, FR-020). Fault-injection hook: `WORK_FAIL_AT=lock|worktree|dir|snapshot|projection` makes the named step return an error, for the rollback test suite.

**Rationale.** A compensation stack is the minimal transaction model that spans four heterogeneous resources (git, filesystem, JSON, SQLite) with no distributed-transaction machinery. Ordering "db row last" means the projection never advertises a Work whose snapshot/worktree isn't fully there. The lock (R-scoped to the computed branch+dir) is exactly what the "two concurrent attempts compute the same branch" edge case needs: the loser fails cleanly without touching the winner.

**Alternatives considered.**
- Write everything then "verify and cleanup on next run" — rejected: leaves observable orphans between runs (violates SC-003).
- A generic WAL/journal of intended operations — rejected: over-engineered for 4 steps; the closure stack is auditable and local.
- Global `~/.work` lock — rejected: needlessly serializes unrelated Works; per-branch/dir hash lock is enough.

---

## R11 — Bootstrap idempotency & repair (FR-005, SC-007, ADR-0003)

**Decision.** First command that needs the seed (`work start`) calls `bootstrap.EnsureSeed()`:
1. Acquire `state/locks/bootstrap.lock`.
2. Compute the embedded seed's content digest.
3. If `registry.json` already has the seed's logical component names **and** `.install-meta.json` digest matches → return (no-op).
4. Otherwise (absent, partial, or stale): extract binaries to a temp dir, atomically rename it into `~/.work/plugins/<alias>`, (re)write `plugin.json`, `.install-meta.json`, and **upsert** the registry entries keyed by logical component name; add the Locator to `repository_resolution.locators` only if not already present.
5. Release lock.

Identity is the **logical component name** (`local-path-starter`, `filesystem-repository-locator`, convention `freeform`), never a boolean "seeded" flag. Re-running or resuming after a kill converges to exactly one entry each.

**Rationale.** SC-007 demands "exactly one usable record of each official component" after 100 runs + 20 interruptions. Keying on logical name + idempotent upsert + atomic plugin-dir rename + a bootstrap lock gives that by construction. Digest comparison also makes a `work` binary upgrade re-seed the newer components without duplication.

**Alternatives considered.**
- Marker file `~/.work/.bootstrapped` — rejected: goes stale if the plugin dir is deleted; can't detect partial state.
- Re-extract every run — rejected: wasteful and races with a plugin the user may have intentionally uninstalled (F4 concern, but the digest/name check is forward-compatible).

---

## R12 — F1 integration self-check (FR-028, SC-002)

**Decision.** No new public command. `internal/work/verify.Check(workID)` compares, for a given Work:
- `work-state.json` parses, `schema==1`, required `work.*` fields present;
- `<dir>/worktree` exists and `git -C worktree rev-parse --abbrev-ref HEAD` == `work.branch`;
- the branch exists and its merge-base with `work.base_branch` is the base's tip (branch started from base);
- the `works` row in `work.db` matches the snapshot on id, slug, status, branch, base, path.
It returns a structured report. It is invoked by the integration test suite and is available for a future `work status --verify` / `work doctor` without expanding the F1 surface.

**Rationale.** FR-028 asks for a check "without requiring F2's resume features" — a library function exercised by tests satisfies it while keeping the CLI within ADR-0017. `work status` (F-later) already surfaces identity/state/branch/location for humans.

**Alternatives considered.**
- Ship `work doctor` now — rejected: new top-level verb, not needed by any F1 user journey, ADR-0017 pressure.
- Fold into `work status` now — rejected: `work status` is an F6-era command; F1 doesn't build it.

---

## R13 — `work start` options & non-interactive behavior (FR-008, FR-024, RF-51, RF-52)

**Decision.** `work start [SOURCE]` accepts, in addition to the positional local path:
- `--workspace <path>` — set/confirm the workspace root for this run; if none is configured, validate + persist it (FR-006); if one exists, use this value for this run without changing the stored one unless combined with an explicit persist… (F1: simply overrides for the run; changing the stored root is a later `work repository`/config concern — for F1, `--workspace` on a machine with no root configured persists it, otherwise it is an error directing the user to edit config, keeping scope minimal).
- `--base <ref>` — base branch (matched against the listed local/remote refs).
- `--slug <slug>` — the slug.
- `--prefix <prefix>` — the convention prefix (freeform has one: `{slug}`; flag exists for grammar symmetry and forward-compat).
- `--yes` — confirm the already-determined creation (never selects a value).
- `--json` — **not** offered on `start` (it is a mutation, not a read; RF-52/ADR-0017 restrict `--json` to reads).

Interactive terminal: any omitted value is collected via its `huh` prompt; an explicit flag skips **only** its own prompt, never validation. Non-interactive (stdin or stdout not a TTY): any still-missing required value → exit code 2 with a message naming the missing flag; **no** TUI; **no** Work mutation, no config write.

**Rationale.** RF-51/FR-008 explicitly describe "explicit values skip only their selection"; that requires flags. ADR-0017 fixes the *administrative* grammar (`plugin`/`repository`/`convention` verbs) and the daily command *names*; it does not enumerate or forbid options on `work start`. Keeping `--json` off `start` respects the read/mutation split.

**Alternatives considered.**
- A single `--set k=v` bag — rejected: worse help output, non-standard.
- No flags, interactive-only in F1 — rejected: violates FR-024/RF-51 which require a working non-interactive failure mode *and* explicit values; also blocks scripted tests.
- Positional `work start <path> <slug>` — rejected: ambiguous, not extensible.

---

## R14 — Direct `repository.path` validation (ADD §7.1, FR-003, FR-027, edge cases)

**Decision.** `internal/reporef.ValidatePath(raw)`:
1. Expand `~`, `filepath.Abs`, then `filepath.EvalSymlinks` (unambiguous resolution across the edge cases: spaces, non-ASCII, `..`, symlinks).
2. `os.Stat` → must exist and be a directory; not readable → `diag` "invalid path" (exit 10).
3. `git -C P rev-parse --git-dir` fails → "unusable repository" (exit 11).
4. `git -C P rev-parse --is-bare-repository` == `true` → reject "bare repository" (exit 11).
5. `git -C P rev-parse --verify --quiet HEAD` fails → reject "repository has no commits" (exit 11).
6. Base-branch list (R15) empty → "no selectable base branch" (exit 12).
7. A path that resolves into an existing **linked worktree** is accepted — git operations resolve to the common dir and branches are available; F1 does not need to forbid it.

Diagnostics quote only the path the user supplied and the category — never repo contents, remote URLs, or file listings.

**Rationale.** Mirrors ADD §7.1 ("core valida diretamente que o caminho existe, é acessível e identifica um repositório Git utilizável") and the spec's edge-case list. `EvalSymlinks` + `Abs` is the portable canonicalization. Each rejection is a distinct `diag` category so SC-004/FR-027 hold.

**Alternatives considered.**
- Accept bare repos and create the worktree from them — rejected: spec edge cases explicitly call bare "rejected with actionable diagnostics" (Assumptions).
- Use `git rev-parse --show-toplevel` only — rejected: doesn't distinguish bare, and fails oddly inside `.git`.

---

## R15 — Base-branch listing & selection (FR-009)

**Decision.** One `git for-each-ref --format '%(refname) %(objectname:short) %(refname:short) %(upstream:short)' refs/heads refs/remotes`, drop `refs/remotes/*/HEAD`. Present two groups — **Local** and **Remote-tracking** — each row `"<short-name>  <short-sha>  <subject-first-line?>"`. When a local and a remote name coincide but point at different objects, both appear with their differing short SHAs so the choice is unambiguous (spec edge case). The selected entry keeps its full `refname`; the branch is created from that exact ref (`git worktree add -b <new> <dir> <refname>`), and `work.base_branch` stores the short name. No network (`refs/remotes` is whatever was already fetched).

**Rationale.** FR-009 wants homonymous/divergent refs distinguishable and an exact revision selectable — short SHA per row does both. Using the full refname for `worktree add` pins the exact revision (FR-011 / acceptance scenario 3: "new branch starts exactly from the selected revision").

**Alternatives considered.**
- `git branch -a` text parsing — rejected: locale-dependent, needs the `*`/`remotes/` prefixes stripped, no SHA.
- `git ls-remote` — rejected: network; F1 is offline.

---

## R16 — Branch-name derivation, validation & collision (FR-011, FR-012, FR-013, ADD §8)

**Decision.**
- **Derivation:** `name = interpolate(prefix, slug)`. `freeform` prefix is `{slug}` → `name == slug`. Slug input is pre-validated in the prompt (non-empty, no whitespace, no `..`, no leading `-`, no control chars) but the authoritative check is git's.
- **Syntax:** `git check-ref-format "refs/heads/<name>"` (pure syntactic form; avoids the `--branch` shorthand's `@{-N}` resolution). Failure → "invalid branch name" (exit 13) naming the offending slug/prefix; interactive → return to the slug prompt.
- **Collision (before any mutation):**
  - local: `git show-ref --verify --quiet refs/heads/<name>`
  - remote-tracking: `git for-each-ref refs/remotes/*/<name>` non-empty
  - worktree: `<name>` appears as a branch in `git worktree list --porcelain`
  - Any hit → "branch collision" (exit 14), identifying which kind; interactive → return to the slug prompt.
- The derived name is **shown to the user** before materialization (FR-011) as part of the confirm step.

**Rationale.** ADD §8 mandates native git for both checks. Doing all three collision checks before `git worktree add` means the mutation only runs on a known-good name (SC-004: "rejected before the first Git or Work mutation"). Returning to the exact prompt that produced the name gives the in-flow correction FR-013 requires.

**Alternatives considered.**
- Rely on `git worktree add -b` to fail on collision — rejected: that is already a mutation attempt; spec wants rejection *before* the first mutation, and the error wouldn't distinguish remote-tracking vs. worktree cases.
- A Go regex for ref rules — rejected by ADD §8.

---

## R17 — Seed IPC contract (ADR-0000, ADD §7, §7.1, §11, ADR-0016)

**Decision.** Both seed components obey the ADD §11 contract: exactly one JSON object on stdin, at most one JSON object on stdout, exit 0 = success / non-zero = failure, stderr = diagnostics. No `runtime` → executed directly. Detailed payloads in `contracts/ipc-starter.md` and `contracts/ipc-repository-locator.md`. Summary:

- **Starter** (`local-path-starter`, a `fallback` starter — no `pattern`): stdin `{ "arg": "<user path>" }`. It resolves the arg as a filesystem path and returns `{ "repository": { "path": "<abs path>" } }`. No `start_modes` (→ core treats as new Work), no base branch, no meta/links. If the arg is empty/whitespace it exits non-zero with a message (the core normally collects the path first, so this is a guard).
- **Repository Locator** (`filesystem-repository-locator`, `accepts: ["name","git_fetch_urls","query"]`): stdin `{ "repository": {…accepted fields…}, "repository_roots": [".."] }`. Walks each root to a bounded depth, returns `{ "matches": [ { "repo_path": "<abs>" }, … ] }`. In F1 the Locator is effectively **not exercised** on the happy path — the fallback Starter always returns `path`, so the core validates directly and never runs a Locator (ADD §7.1). It is still seeded, registered, and placed in the policy so F3 activates with zero new bootstrap work, and its contract is tested in isolation now.

**Rationale.** ADR-0014/0016: Starter does origin→reference, Locator does reference→local candidates, `path` present short-circuits Locators. The simplest correct F1 seed Starter just wraps the path the user typed. Building + contract-testing the Locator now (even though F1 doesn't call it in the main flow) is cheap and de-risks F3.

**Alternatives considered.**
- Make the seed Starter also do filesystem search (merge Starter+Locator) — rejected: violates ADR-0014's separation; F3 would have to unpick it.
- Skip building the Locator until F3 — rejected: bootstrap (R11) and ADD §6 require the seed to contain it and put it in the policy; leaving a dangling policy entry pointing at nothing violates determinism.

---

## R18 — Testing strategy (roadmap §4, spec Independent Tests & SCs)

**Decision.**
- **Unit** (`go test ./internal/...`): table-driven, per package. Pure logic (convention interpolation, diag mapping, manifest role validation, config round-trip) has no I/O.
- **Contract** (`tests/contract/`): drive the built `seed/dist` binaries with golden input JSON, assert output JSON + exit code, using self-contained fixtures (roadmap §4 "Contrato de processo"). Also assert an invalid/garbled stdin response is a clean non-zero exit.
- **Integration** (`tests/integration/*.txtar` via `testscript`): build `work`, run against git repos created inside the script; a fake-shell harness exercises the `WORK_CD_FILE` protocol and the FR-023 no-integration path. Scenarios: happy path (US1 scenarios 1–4), offline bootstrap, `--workspace` first-run persist, invalid path, invalid slug, all three collision kinds, non-interactive missing value, cancel-before-confirm, and `WORK_FAIL_AT=*` rollback (SC-003), plus a `verify.Check` assertion after every successful create (SC-002/FR-028).
- **Idempotency**: a test loops `bootstrap.EnsureSeed()` 100× and interrupts extraction at injected points, asserting one registry entry each (SC-007).
- **CI**: the three-OS matrix runs unit + contract + integration; `go vet`, `staticcheck`, `gofmt -l`.

**Rationale.** `testscript` is the standard way to test a Go CLI end-to-end with filesystem + subprocess assertions and is itself cross-platform. Fault injection via env var keeps the rollback tests deterministic. Running the whole suite on all three OSes is what SC-002/SC-006 require.

**Alternatives considered.**
- Bats / shell-based integration — rejected: not portable to Windows CI, no Go coverage integration.
- Mock `git` — rejected: the whole point is real git behavior on real repos (roadmap RC criteria).

---

## R19 — Diagnostics & stable exit codes (FR-025, FR-027, roadmap §4 Auditability)

**Decision.** `internal/diag` defines a `Category` enum, each with a fixed exit code, a short stable machine token, and a human message template. `work start` maps every failure to one:

| Code | Token | Category |
|---|---|---|
| 0 | `ok` | success (may still print an FR-023 shell notice to stderr) |
| 2 | `usage` | bad flags / missing required value in non-interactive mode |
| 10 | `invalid-path` | path missing, not a dir, unreadable |
| 11 | `unusable-repo` | not a git repo / bare / no commits |
| 12 | `no-base-branch` | no selectable base branch |
| 13 | `invalid-branch-name` | fails `git check-ref-format` |
| 14 | `branch-collision` | collides with local / remote-tracking / worktree branch |
| 15 | `destination-unavailable` | Work dir exists / occupied / not writable |
| 16 | `bootstrap-failed` | git preflight (git absent / < 2.5) or seed extraction / registration failed |
| 17 | `materialization-failed` | a create step failed; rollback completed |
| 20 | `cancelled` | user aborted before confirmation |

Messages state the offending user choice and the category; they never include repository file contents, remote URLs beyond what the user supplied, or environment secrets. Human output → stderr; on success the machine-relevant "created" line (Work id, branch, absolute path) → stdout in a stable, greppable format (FR-025).

**Rationale.** SC / FR-025 / FR-027 require stable codes and messages for scripting plus no content leakage. A central enum keeps them consistent and testable (one test asserts the full table).

**Alternatives considered.**
- Reuse Cobra's default exit 1 for everything — rejected: FR-025/FR-027 want distinguishable outcomes.
- Sentinel error strings only — rejected: not machine-stable; the code table is the contract.

---

## Resolved unknowns checklist

| Plan Technical-Context unknown | Resolved by |
|---|---|
| Go version / module path | R (context): Go 1.26, `github.com/gustaborges/work` |
| CLI/TUI libraries + versions | R6 |
| SQLite driver | R5 |
| Git access method + version floor | R7 |
| Atomic snapshot mechanism | R4 |
| OS + shell support matrix / CI | R1 |
| Terminal-positioning contract (roadmap §5.1) | R2 |
| Self-contained seed format (roadmap §5.3) | R3 |
| Workspace-root default per platform | R8 |
| `~/.work` layout & config format | R9 |
| Transaction / rollback model | R10 |
| Bootstrap idempotency | R11 |
| FR-028 integration check surface | R12 |
| `work start` flags & non-interactive rule | R13 |
| Direct path validation steps | R14 |
| Base-branch listing | R15 |
| Branch-name validation & collision | R16 |
| Seed IPC payloads | R17 + contracts/ |
| Test tooling | R18 |
| Exit-code / diagnostic table | R19 |

No `NEEDS CLARIFICATION` markers remain.
