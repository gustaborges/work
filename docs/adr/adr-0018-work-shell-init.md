# ADR-0018: `work shell-init` and the `WORK_CD_FILE` protocol

**Status:** Proposal

**Date:** 2026-09-06

**Product context:** `docs/prd.md` — RF-9; `specs/001-first-local-work/spec.md` FR-022, FR-023; roadmap §5.1

**Governs:** `docs/add/add-0001-work-system-architecture.md` §5 (CLI surface) and §11 (process contract); `specs/001-first-local-work/contracts/shell-integration.md`

**Relates to:** ADR-0017 (CLI surface), ADR-0006 (direct component execution), `research.md` R2

## Context

At the end of `work start`, the expected journey leaves the terminal **inside the new
worktree** (US1 scenario 1, FR-022). A child process cannot change the working directory
of its parent shell; every tool that does so — `direnv`, `zoxide`, `pyenv`, `fnm`, `jj` —
installs a hook in the user's shell.

F1 therefore needs a new public command that emits this hook. ADR-0017 established that
**`work init` is not public** (bootstrap and configuration happen on-demand) and defined
the administrative grammar `work <resource> <verb>`. A shell snippet emitter is neither
bootstrap nor does it fit that grammar — it does not administer any resource, it merely
prints text. `research.md` R2 recorded the technical decision (channel via temporary file,
opt-in wrapper) and marked the need for a short ADR to ratify **name, snippets per shell,
and protocol** before v1. This ADR provides that ratification.

## Decision

### 1. Command: `work shell-init <shell>`

* Name **`shell-init`**, hyphenated, a single level below `work`. It is the only recognized
  exception to the grammar of ADR-0017: it has no final verb because it is not an operation
  on a resource, but rather a configuration generator for the shell.
* `<shell>` is mandatory and assumes the values `bash`, `zsh`, `fish`, `powershell`.
  Missing or unknown value → exit **2** (`usage`) listing the supported shells.
* **Read-only.** Writes the snippet to **stdout**, exits 0, does not touch any file or state.
  Accept `--json`? No — there is no structured payload; it is text for `eval`.
* **Does not appear in the home TUI** (`work` without arguments). It is installation plumbing,
  not a journey.
* Intended usage, documented in `--help` and in the README:
  * `eval "$(work shell-init bash)"` / `zsh` in the corresponding rc;
  * `work shell-init fish | source`;
  * `Invoke-Expression (& work shell-init powershell | Out-String)`.

### 2. Snippet contract (all shells)

The snippet defines a function/command `work` that wraps the real binary and:

1. creates a private temporary file `T`;
2. exports, **only to the child process**, `WORK_CD_FILE=T` and
   `WORK_SHELL_INTEGRATION=1`;
3. executes the real binary with all original arguments, inheriting
   stdin/stdout/stderr;
4. upon return: if `T` exists and is non-empty, performs `cd` to the path contained in it;
   then removes `T`;
5. preserves the child's exit code as the return value of the function.

The snippet **must**: resolve the real binary without recursion into the function (`command`,
`builtin`, absolute path, or `$WORK_REAL_BIN`); not leak `WORK_CD_FILE` / `WORK_SHELL_INTEGRATION`
into the interactive shell beyond the child; be idempotent when loaded more than once.

### 3. `WORK_CD_FILE` protocol (core side)

* On **successful** `work start` (exit 0), **after** committing the line to `works`,
  if `WORK_CD_FILE` is set and non-empty, the core writes the **absolute path of the worktree**
  (`<dir>/worktree`) to that file. It is the only thing the core writes there.
* The core **never** writes to `WORK_CD_FILE` on failure, cancellation, or for
  any command other than `start`, `resume`, or `archive` (see Amendment F2).
* If `WORK_CD_FILE` is absent/empty on successful creation, the core takes the
  **FR-023 path**: exits 0, keeps the success summary in stdout and prints to
  **stderr** a warning that the session was not moved, the real path of the worktree on
  its own line, and the line `eval "$(work shell-init <detected shell>)"`. The shell
  is inferred from `$SHELL` / presence of `$PSVersionTable`; if indeterminate, uses the
  `bash` form and mentions the others. The core never asserts that a `cd` occurred.

### 4. Non-objectives (F1)

* No automatic editing of rc files (ADR-0017: no `work init`; R2).
* No positioning for `cmd.exe`, `nushell`, `xonsh` — they receive the FR-023 warning.
* No subshell spawning.

## Alternatives considered

* **Emit `cd <path>` to stdout, user does `eval "$(work start …)"`.** Rejected:
  destroys normal interactive stdout (success summary, TUI) and is fragile with the TUI.
* **Reserved file descriptor (fd 3).** Rejected: awkward configuration in
  fish/PowerShell; the temporary file is universal.
* **Spawn a child shell already inside the worktree.** Rejected: nests shells,
  breaks job control, loses parent's history/session.
* **`work init` that edits the rc.** Rejected by ADR-0017 and for being intrusive;
  `shell-init` merely prints and the user decides to install.
* **Fit as `work shell init` (resource `shell`, verb `init`).** Rejected:
  `shell` is not an administrable Work resource and `init` would reintroduce the verb
  that ADR-0017 removed; `shell-init` as a single token makes clear it is a special case.

## Consequences

**Positive:** the US1 journey ends inside the worktree when the hook is installed; the
absence of the hook is reported honestly and with instructions for correction; the core
does not need a reserved fd nor pollutes stdout; the protocol is the same on POSIX and
PowerShell.

**Negative / trade-offs:** `work shell-init` is a name that deviates from the ADR-0017 grammar
and must always be documented as an exception; the user has a manual installation step; shells
outside the supported matrix never reposition.

## Amendment (F2 — Daily Cycle, 2026-09-06)

Successful `work resume` now writes `WORK_CD_FILE` with the absolute path of the resumed
worktree, exactly by the same protocol of §3 (after canonical commit, never on failure/cancellation)
— already anticipated by the "and, further, `resume`" above. `work archive` adds a **third** case,
restricted: when the destroyed worktree **is (or contains) the current working directory of the
caller**, the core writes to `WORK_CD_FILE` the **path of the workspace root** (not a worktree),
to exit a session from a directory that no longer exists; in any other situation `archive` does not
touch `WORK_CD_FILE`. Without the hook installed, `archive` takes the equivalent FR-023 path:
exits 0, warns on stderr that the session is in a removed directory and names the workspace root
for `cd`, never asserting that a `cd` occurred. Mechanism, channel (private temporary file), and
snippets per shell remain unchanged.

## Follow-up

When promoting to **Accepted**, reference this ADR in `add-0001` §5 and remove the
"shell-init follow-up" item from the Constitution Check of
`specs/001-first-local-work/plan.md`.
