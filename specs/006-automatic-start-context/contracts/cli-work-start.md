# Contract: `work start` — F5 amendment (post-creation extensions)

Authority: PRD FR-8, FR-9, FR-26..FR-29; ADD §9..§11; spec FR-016, FR-034..FR-046;
`research.md` R9, R12–R14. **Extends** `specs/005-plugin-origins/contracts/cli-work-start.md`
and, through it, F1–F4. Grammar, flags, prompts, transaction guarantees, stable stdout lines,
error tokens and exit codes are **unchanged**. This amendment adds only what happens after
`create.Run` commits.

## Ordering

```text
… Starter → resolution → mode/base/slug/convention/prefix → create.Run (commit)
   1. stdout: the three stable lines (unchanged)
   2. extension pipeline (UI channel only)              ← new; skipped entirely when nothing is eligible
   3. terminal repositioned into the worktree (shell integration, unchanged)
```

A start that fails or is cancelled before `create.Run` commits never reaches step 2. Step 2
never changes stdout, the exit code, or step 3.

## Streams

| Stream | Content |
|---|---|
| stdout | exactly the F4 lines: `work: created <id>`, `work: branch …`, `work: path …`. **Nothing about extensions.** |
| UI channel (stderr) | progress lines and warnings below |
| exit code | unchanged; a warning never changes it (D3) — an automatic-extension failure still exits 0 |

## Progress lines (UI channel; same text interactive and not; themed muted when interactive)

```text
work: running <alias>/<name> (discover)
work: <alias>/<name>: linked <key>                  ; a Linker returned a value (the key, never the value)
work: running <alias>/<name> (import)
work: <alias>/<name>: imported <n> item(s)          ; n > 0 files/dirs incorporated
```

Printed in execution order (`alias/name` ascending within a phase). A component that is not
eligible, a Linker with no value, and an Importer with no output print no result line. No
control sequences when `NO_COLOR` is non-empty, `TERM=dumb`, or the UI writer is not a
terminal. When nothing is eligible, **nothing** is printed and no extension process starts.

## Warnings (UI channel; one per failure; never on stdout; never an exit-code change)

Non-interactive (frozen, mirrors `error: <token>: <message>`):

```text
warning: <token>: <alias>/<name> (<operation>) for Work <id>: <summary>
```

Interactive: `⚠ <summary>` then, when there is one, `  → <hint>` (border layer, theme
`Warning`). `<token>` ∈ `extension-start-failed`, `extension-failed`,
`extension-response-invalid`, `extension-output-refused`, `extension-persist-failed`,
`extension-interrupted` (`extension-protocol.md` §7). Examples:

```text
warning: extension-output-refused: acme/notes-importer (import) for Work 01J9…: it would overwrite "notes/pr.md"; none of its files were added
warning: extension-failed: acme/pr-linker (discover) for Work 01J9…: it exited unsuccessfully; no link was recorded
```

A warning **never** contains a link value, a metadata value, artifact content, or the
extension's own error text. `WORK_DEBUG` non-empty appends, after the human line, the
component's exit status and the last 4 KiB of its stderr (F2.5's explicit-diagnosis
affordance). A refused output names the colliding **relative path**; that is information
about the refusal, not artifact content.

## Interrupt (Ctrl-C during step 2)

Stops the running extension, discards its output, skips the rest, prints one
`extension-interrupted` warning, and continues to step 3. Exit 0. The Work is kept.

## Non-interactive behavior

No prompts are added: eligibility is static and there is nothing to choose. The pipeline runs
the same in a non-TTY, `--yes`, or automation context, producing the same progress lines and
warnings without control sequences.

## Regression contract (SC-008)

On a machine with only the reference package the run is byte-identical to F4 on stdout, exit
code and created files, and starts no process beyond the Starter and Locator. Every F1–F4
`testscript`, PTY and contract test passes unmodified, except the `plugin`/`ipc`/`registry`
unit tests that pin the F1 placeholder manifest shapes and the `ipc` signatures (updated, not
weakened, in Phase 1 of `plan.md`).
