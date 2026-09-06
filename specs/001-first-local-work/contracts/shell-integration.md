# Contract: Shell integration & terminal repositioning (F1)

Authority: spec FR-009?/FR-022, FR-023, RF-9; roadmap §5.1; research R2.

## `work shell-init <shell>`

```
work shell-init bash
work shell-init zsh
work shell-init fish
work shell-init powershell
```

- **Read-only.** Prints a shell snippet to **stdout** and exits 0. Changes no files, no state.
- Unknown/missing `<shell>` → exit 2 with the list of supported shells.
- Intended use: `eval "$(work shell-init bash)"` in `~/.bashrc`, the zsh/fish equivalent, or
  `Invoke-Expression (& work shell-init powershell | Out-String)` in the PowerShell profile.
- Not shown in the `work` home (it is setup plumbing, not a journey).

### Snippet behavior (all shells)

The snippet defines a `work` shell function/command that wraps the real binary:

1. Create a private temp file `T`.
2. Export `WORK_CD_FILE=T` and `WORK_SHELL_INTEGRATION=1` for the child only.
3. Exec the real binary (`command work` / absolute path) with all original args, inheriting
   stdin/stdout/stderr and the exit code.
4. After it returns: if `T` exists and is non-empty, `cd` to the path it contains; then remove `T`.
5. Preserve the child's exit status as the function's return status.

The snippet MUST:
- resolve the real binary without recursing into the function (`command`, `builtin`, absolute
  path, or `$WORK_REAL_BIN`);
- not leak `WORK_CD_FILE` / `WORK_SHELL_INTEGRATION` into the interactive shell's environment
  beyond the child;
- be idempotent to source more than once.

## Core side — writing the target path

On a **successful** `work start` (exit 0), after the `works` row is committed:

- If `WORK_CD_FILE` is set and non-empty → write the **absolute worktree path** (`<dir>/worktree`),
  no trailing newline required, to that file. This is the only thing the core writes there.
- If `WORK_CD_FILE` is unset/empty → take the **FR-023 path** below.

The core never writes `WORK_CD_FILE` on failure, on cancel, or for any command other than
`start` (and, later, `resume`).

## FR-023 path — integration not active

When `WORK_CD_FILE` is unset/empty on a successful create, the core prints to **stderr**
(exit stays 0):

```
note: this shell session was not moved into the new worktree.
note: worktree path:
<absolute-worktree-path>
note: to move automatically next time, add this to your shell startup file:
      eval "$(work shell-init <detected-shell>)"
```

- `<detected-shell>` is inferred from `$SHELL` / ` psmodulepath`/`$PSVersionTable` presence;
  if it cannot be determined, print the generic `bash` form and mention the other supported
  shells.
- The core MUST NOT claim a directory change happened, and MUST print the real worktree path
  unambiguously on its own line (FR-023).
- The stdout success summary (`cli-work-start.md`) is unchanged and still printed.

## Non-goals (F1)

- No automatic editing of rc files (ADR-0017: no `work init`; R2).
- No support for `cmd.exe`, `nushell`, `xonsh` positioning — they get the FR-023 notice.
- No subshell spawning.

## Contract tests (tests/integration/, fake-shell harness)

| Scenario | Expect |
|---|---|
| integration active, successful create | temp file contains the absolute worktree path; harness `cd`s there; exit 0; no FR-023 notice |
| integration active, failed create | temp file empty/absent; no `cd`; non-zero exit |
| integration active, user cancels | temp file empty/absent; exit 20 |
| no integration, successful create | FR-023 notice on stderr with the real path; exit 0; stdout summary present |
| `work shell-init bash` | valid bash, defines `work`, sources twice cleanly |
| `work shell-init powershell` | valid PowerShell function |
| `work shell-init frobnicate` | exit 2, lists supported shells |
