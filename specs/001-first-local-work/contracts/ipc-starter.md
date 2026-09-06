# Contract: Seed Starter subprocess — `local-path-starter` (F1)

Authority: ADR-0000, ADR-0006, ADR-0012 (role `starter`), ADR-0014, ADR-0016, ADD §7 & §11;
research R3, R17. This is the F1 seed's `fallback`-layer Starter.

## Process model

- Invoked as a **subprocess** by the core, even though it ships embedded in the release binary
  (ADR-0003). The core never loads its code in-process (ADR-0000).
- Manifest declares **no `runtime`** → the core executes the entrypoint directly (`source/starter`,
  `source/starter.exe` on Windows). No shebang / exec-bit reliance (ADR-0006).
- One JSON object on **stdin**, at most one JSON object on **stdout**, diagnostics on **stderr**,
  exit code 0 = success / non-zero = failure (ADD §11).
- No progress/heartbeat/streaming messages (ADD §11, v1).

## Manifest entry (in seed `plugin.json`)

```jsonc
{
  "name": "local-path-starter",
  "role": "starter",
  "entrypoint": "starter"
}
```

No `pattern` → this is the single enabled **fallback** Starter (ADR-0004). Fields `on`,
`manual`, `inputs`, `key`, `discover` are invalid for role `starter` and MUST be absent
(ADD §4.1).

## Input (stdin)

```json
{ "arg": "<the SOURCE string the user gave to `work start`>" }
```

- `arg` is always present (the core collects the path first in the interactive flow; in the
  non-interactive flow it is the positional `SOURCE`).
- The Starter receives only `{ "arg": ... }` — no environment context, no roots, no core state
  (ADD §7).

## Output (stdout, on success — exit 0)

```json
{ "repository": { "path": "<absolute, cleaned local path>" } }
```

Rules:
- The Starter resolves `arg` to an absolute path (expanding `~`, making it absolute against its
  own cwd is **not** done — the core passes what the user typed; the Starter may `filepath.Abs`
  it, but symlink resolution and the authoritative repo validation are the **core's** job, R14).
- It sets **only** `repository.path`. It does not set `git_fetch_urls`, `name`, `query`,
  `base_branch`, `start_modes`, `meta`, or `links` (a bare local path carries none of that).
- Absence of `start_modes` → the core creates a **new** Work (`work.start_mode = "new"`, ADD §7).

The Starter does **not** verify that the path is a git repository — returning a path that
fails the core's validation results in `work start` exiting 10/11 (ADD §7.1: "core continua a
autoridade final para validar todo caminho").

## Failure (exit non-zero)

- `arg` missing, empty, or only whitespace → exit non-zero, one human line on stderr
  (`"local-path-starter: no path given"`). No stdout.
- Any other internal error → exit non-zero, stderr message, no stdout.

A non-zero exit or a structurally invalid stdout object makes `work start` fail
(`materialization-failed` / `unusable-repo`); there is no `matched:false` result (ADD §7).

## Contract tests (tests/contract/)

| Case | stdin | expect |
|---|---|---|
| happy | `{"arg":"/tmp/x/repo"}` | exit 0, stdout `{"repository":{"path":"/tmp/x/repo"}}` (abs) |
| relative arg | `{"arg":"./repo"}` | exit 0, `repository.path` is absolute |
| empty arg | `{"arg":""}` | exit ≠ 0, no stdout, stderr non-empty |
| missing arg | `{}` | exit ≠ 0, no stdout |
| garbage stdin | `not json` | exit ≠ 0, no stdout |
| extra stdin fields | `{"arg":"/r","x":1}` | exit 0 (ignored) |
