# Quickstart / Validation Guide: New Origins via Plugin (F4)

**Feature**: `specs/005-plugin-origins/` · **Plan**: [plan.md](./plan.md)

Runnable scenarios that prove F4 end to end. Shapes, tokens, and rules live in
[`contracts/`](./contracts/), [`data-model.md`](./data-model.md), and
[`research.md`](./research.md) — not repeated here.

**Compatibility baseline (SC-010):** the F1 suite
(`specs/001-first-local-work/quickstart.md` S1–S12), the F2 suite
(`specs/002-daily-cycle/quickstart.md` S1–S13), the F2.5 suite
(`specs/003-terminal-ux-revamp/quickstart.md` Q1–Q12), and the F3 suite
(`specs/004-local-clone-locator/quickstart.md` S1–S13) MUST pass **unchanged**
alongside these. Any diff in their stdout, error tokens, mutations, or exit
codes is a regression. F4's contracted changes are additive: `work start`'s
SOURCE step now runs Starter matching before resolution, may offer
`start_modes`, and the convention step is no longer hardcoded to `freeform`
(`contracts/cli-work-start.md`) — none of that fires when only the reference
package is installed and no Starter response ever includes `start_modes`,
which is exactly F1–F3's fixture shape.

## Prerequisites

- Go 1.26+, system `git` >= 2.5.
- A Unix host with a PTY for the interactive scenarios (`//go:build unix`,
  `github.com/creack/pty`); Windows keeps non-interactive coverage plus a
  manual smoke of S1, S4, and S6 (the directory-symlink `--link` path needs
  Developer Mode there — `research.md` R2).
- **No network** for any scenario — the "remote install" scenarios use a
  local `file://` git remote, never a real network fetch.
- F3 merged (`internal/locator`, `internal/repoconfig`, `work repository`
  present and unchanged).

## Build & isolation

```bash
make build
export WORK_HOME="$(mktemp -d)/dotwork"
export PATH="$PWD/bin:$PATH"
export WS="$(mktemp -d)/workspaces"
export R1="$(mktemp -d)/src"
mkrepo() { mkdir -p "$1" && git init -q "$1" && git -C "$1" commit -q --allow-empty -m init && git -C "$1" branch -M main; }
mkrepo "$R1/demo"
work repository root add "$R1"   # F3 first-run setup already covers this; explicit here for scripting

FIXTURES="$PWD/tests/fixtures/plugins"
```

---

## S1 — Install a plugin locally, pinned and linked (US1; FR-001, FR-002, SC-003)

```bash
work plugin install "$FIXTURES/specific-starter"
work plugin install "$FIXTURES/colliding-starter" --link
```

**Expect**: both exit 0. `work plugin list --json` shows `specific-starter`'s
package with `"origin":"local-pinned"`; editing a file under
`$FIXTURES/specific-starter` after install does **not** change the installed
copy. `colliding-starter`'s package shows `"origin":"local-linked"`; editing a
file under `$FIXTURES/colliding-starter` **is** immediately visible to a
subsequent `work start` (the installed `source/` is a symlink).

---

## S2 — Install a plugin from a remote source, pinned (US1; FR-003, SC-003)

```bash
REMOTE="$(mktemp -d)/remote-plugin.git"
git init -q --bare "$REMOTE"
git clone -q "$REMOTE" /tmp/seed-remote-plugin
cp -r "$FIXTURES/specific-starter/." /tmp/seed-remote-plugin/
git -C /tmp/seed-remote-plugin add -A
git -C /tmp/seed-remote-plugin commit -q -m seed
git -C /tmp/seed-remote-plugin push -q origin HEAD:main

work plugin install "file://$REMOTE" --as remote-demo
```

**Expect**: exit 0; `work plugin list --json` shows `remote-demo` with
`"origin":"remote-pinned"` and a reference of the form
`file://<REMOTE>@<40-hex-sha>`.

---

## S3 — Alias collision fails explicitly; reinstall is idempotent (US1; FR-006, SC-004)

```bash
work plugin install "$FIXTURES/colliding-starter" --as remote-demo   # different origin, same alias as S2
```

**Expect**: exit 32 (`plugin-alias-conflict`), names `remote-demo` and both
origins; `work plugin list` unchanged from S2. Then:

```bash
work plugin install "file://$REMOTE" --as remote-demo   # same origin, same alias
```

**Expect**: exit 0, no new entry (still one `remote-demo` package).

---

## S4 — Invalid manifest registers nothing (US1; FR-004, SC-005)

```bash
work plugin install "$FIXTURES/invalid-manifest"
```

**Expect**: exit 31 (`plugin-invalid`); `work plugin list` shows no new
package and no partial component entries; `~/.work/state/registry.json`
byte-identical to before the attempt.

---

## S5 — `--link` rejected against a remote source (US1; FR-002)

```bash
work plugin install "file://$REMOTE" --link
```

**Expect**: exit 2 (`usage`), nothing installed, before any clone is
attempted.

---

## S6 — Two enabled fallback Starters is rejected (US1 edge case; FR-011)

```bash
work plugin install "$FIXTURES/fallback-starter-a" --as fb-a
work plugin install "$FIXTURES/fallback-starter-b" --as fb-b
```

**Expect**: first exit 0; second exit 33 (`plugin-fallback-conflict`), naming
`fb-a`'s starter as the existing fallback; `fb-b` registers nothing (its
manifest may declare other components — none of them register either, since
the whole install fails atomically).

---

## S7 — Start a Work from a plugin's Starter, full journey (US2; FR-001–FR-010)

With `specific-starter` installed (S1) matching the literal token `demo-pr-1`:

```bash
work start demo-pr-1 --slug demo-review --base main --prefix '{slug}' --yes
```

**Expect**: exit 0; the specific Starter was invoked (not the reference
fallback); its `repository.name` resolved through the F3 Locator chain to
`$R1/demo`; the resulting worktree/`work-state.json`/index entry are
indistinguishable in shape from an F1 direct-path creation (SC-002).
`work.starter` names the plugin's Starter (qualified `<alias>/<name>` if it
collides with another enabled Starter's bare name).

---

## S8 — Fork mode: full journey, Starter-provided base branch (US2/US3; FR-021, FR-024)

`specific-starter` returns `start_modes: ["contribution","fork"]` and
`base_branch: "main"` for `demo-pr-1`. Interactively:

```bash
work start demo-pr-1   # PTY
```

**Expect**: a Mode step offers exactly `contribution`/`fork`; selecting
**fork** skips the base-branch prompt (already supplied), then runs the
normal slug/convention/prefix sequence; `work.start_mode == "fork"`.

---

## S9 — Contribution mode: no slug/convention/prefix, existing branch (US3; FR-020, SC-007)

Same fixture and argument, selecting **contribution** instead:

**Expect**: no slug, convention, or prefix step is shown; the confirmation
preview names the branch being checked out, not a base+new-branch pair;
`git worktree add` runs without `-b` (`WorktreeAddExisting`); on success,
`work.start_mode == "contribution"`, `work-state.json` has no `slug` and no
`branch_convention` key at all (schema 3, `work-state.schema.json`).

---

## S10 — Contribution mode rollback never deletes the branch (US3; SC-009)

Repeat S9 but cancel at the confirmation step (PTY: Esc).

**Expect**: exit 20 (`cancelled`); no worktree/dir/snapshot/index entry
created; the branch the Starter resolved still exists in the source
repository (`git -C "$R1/demo" branch --list <branch>` is non-empty) — it was
never created by this Work, so there is nothing to delete.

---

## S11 — Starter collision: explicit choice, never memorized (US4; FR-012, SC-006)

With `specific-starter` and `colliding-starter` both matching `demo-pr-1`:

```bash
work start demo-pr-1   # PTY, run twice with different choices
```

**Expect**: both runs show a Starter selector naming both packages before
either is invoked; the two runs may pick different Starters; neither run's
choice affects the other (no memoization anywhere on disk).

Non-interactively:

```bash
work start demo-pr-1 --slug x --base main --prefix '{slug}' --yes
```

**Expect**: exit 36 (`starter-ambiguous`), naming both colliding Starters, no
selector opened, no Work created (US4 AC4).

---

## S12 — No pattern match and no fallback (US1/US2 edge case; FR-013)

Uninstall or never install any fallback Starter (a fresh `WORK_HOME` before
bootstrap seeds one — use a raw registry fixture for this case), then:

```bash
work start not-a-known-shape
```

**Expect**: exit 35 (`starter-not-matched`), actionable message naming
"enable or install a Starter", no Work created.

---

## S13 — Branch convention: single enabled, then multiple, memoized per repository (US5; FR-025, FR-026)

With only the reference package installed (`freeform`), a fork-mode
`work start` against `$R1/demo` shows no convention step (S7 already
demonstrates this). Then install a plugin declaring `gitflow`
(`specific-starter`'s manifest includes it) and repeat:

```bash
work start demo-pr-1   # PTY, select fork
```

**Expect**: a Convention step now appears (2 enabled: `freeform`,
`gitflow`); the accepted choice is memoized. Clone `$R1/demo` to a second
directory and run `work convention show` from inside it:

```bash
git clone -q "$R1/demo" /tmp/demo-clone-2
(cd /tmp/demo-clone-2 && work convention show)
```

**Expect**: reports the same convention chosen in the first clone (identity
follows the repository, not the working directory — ADR-0011).

---

## S14 — `work convention set` and the interactive hub (US5; FR-027–FR-030)

```bash
work convention set nope       # from inside $R1/demo
```

**Expect**: exit 38 (`convention-unknown`), nothing persisted.

```bash
work convention set gitflow
work convention show --json    # {"identity":"...","convention":"gitflow"}
```

**Expect**: both exit 0; a subsequent fork-mode `work start` against
`$R1/demo` (or its clone) uses `gitflow` without prompting.

```bash
work convention   # PTY, interactive terminal, inside $R1/demo
```

**Expect**: the hub shows the current choice (`gitflow`); changing it to
`freeform` persists immediately and prints the equivalent direct command
receipt (`work convention set freeform`). Outside a git repository, every
form (`show`, `set`, the bare hub) fails `usage` (exit 2) with a clear
"not inside a git repository" message. Non-interactively, the bare command
fails `usage` (exit 2) without opening a selector.

---

## Regression sweep

Run the full F1–F3 automated suites unchanged (`make test` covers all of
them). Specifically confirm:

- `work start <path>` (a plain path, no plugin argument shape involved) is
  byte-identical to F3: same stdout, same `work.start_mode: "new"`, same
  `branch_convention: "freeform"` when only the reference package is
  installed.
- `work repository`/`work plugin` bare-command help output is grouped and
  exits 0 in every stream configuration (both help-only parents, same
  underlying pattern — `contracts/cli-work-plugin.md` §Scope note).
- `NO_COLOR=1`/`TERM=dumb`/piped stdout carry no ANSI escapes for every new
  command (`plugin install/list`, `convention show/set`, and the new
  `present.Select` steps inside `work start`).
