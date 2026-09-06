# Quickstart / Validation Guide: First Local Work (F1)

**Feature**: `specs/001-first-local-work/` · **Plan**: [plan.md](./plan.md)

Runnable scenarios that prove F1 end to end. Each maps to spec acceptance scenarios and
Success Criteria. Details of shapes and codes live in [`contracts/`](./contracts/),
[`data-model.md`](./data-model.md), and [`research.md`](./research.md) — not repeated here.

## Prerequisites

- Go 1.26+ and system `git` >= 2.5 on `PATH`.
- A POSIX shell or PowerShell 7+ for the shell-integration scenarios.
- No network is required for any scenario (that is the point of S3).

## Build

```bash
make build     # runs `make seed` (host platform) then go build -o bin/work ./cmd/work
make install   # copy bin/work to $(DESTDIR)$(PREFIX)/bin  (default ~/.local/bin)
make test      # unit + contract + integration (testscript)
```

`make build` embeds only the host platform's seed components. `make seed-all` stages
every release platform (used by `go test ./seed` and `make build-all`); `make release`
cross-compiles `bin/release/<goos>_<goarch>/work` for every target.

## Test isolation

All scenarios below run with a scratch home so they never touch a real `~/.work`:

```bash
export WORK_HOME="$(mktemp -d)/dotwork"
export PATH="$PWD/bin:$PATH"
```

---

## S1 — Create the first Work from a clean install (happy path)

**Covers:** US1 scenarios 1, 3, 4; SC-001; SC-002; FR-004, FR-006, FR-009, FR-010, FR-011,
FR-014, FR-015, FR-016, FR-019, FR-022.

```bash
# a throwaway "already-cloned" repo
src="$(mktemp -d)/demo"; git init -q "$src"
( cd "$src" && git commit -q --allow-empty -m init && git branch -M main )

# non-interactive form so the scenario is scriptable (interactive form: just `work start`)
work start "$src" \
  --workspace "$(mktemp -d)/workspaces" \
  --base main \
  --slug add-retry-logic \
  --prefix '{slug}' \
  --yes
```

**Expect:**
- exit 0; stdout has exactly:
  ```
  work: created <id>
  work: branch add-retry-logic  (from main @ <sha>)
  work: path <abs>/workspaces/in-progress/demo_add-retry-logic/worktree
  ```
- `<workspace>/in-progress/demo_add-retry-logic/` contains `worktree/` and `work-state.json`
  and nothing else.
- `work-state.json` validates against `contracts/work-state.schema.json`; `work.status ==
  "in-progress"`, `work.start_mode == "new"`, `work.branch == "add-retry-logic"`,
  `work.base_branch == "main"`, `work.branch_convention == "freeform"`,
  `work.starter == "local-path-starter"`.
- `git -C <worktree> rev-parse --abbrev-ref HEAD` → `add-retry-logic`; its first commit is
  `main`'s tip.
- The source repo at `$src` is untouched: still on `main`, no new branches, clean status.
- `~/.work/state/work.db` has one `works` row matching the snapshot.
- First run performed the offline bootstrap: `~/.work/plugins/work-reference/` exists with
  `plugin.json`, `source/starter`, `source/locator`; `~/.work/state/registry.json` has the
  starter + locator + `freeform`; `config/work.json` lists the locator in
  `repository_resolution.locators`.
- `internal/work/verify.Check(<id>)` passes.

**Interactive variant:** run `work` with no args → home → "Start a Work"; or `work start`
with no source. Provide the path when prompted, type the slug (`freeform`'s single `{slug}`
prefix is applied without asking), pick `main` from the base-branch picker's `Local` tab,
accept the suggested `~/work` (or edit it), confirm. Each answered step collapses to a
`✓ <value>` line as you go. Same end state, reachable with the keyboard only (FR-029, RF-50).

**Enclosing git repo:** the workspace root (and therefore every Work under it) may sit
inside an unrelated Git repository — `work start` does not reject that. Only a root that
overlaps a configured `repository_roots` entry is refused.

---

## S2 — Base branch selection distinguishes homonyms / divergent refs

**Covers:** US1 edge cases (remote vs local same name); FR-009.

```bash
# repo with a local 'main' and a remote-tracking 'origin/main' at a different commit
# (fixture builds this with a bare "remote" and a divergent local commit)
work start "$src2"   # interactive
```

**Expect:** the base-branch picker opens on the **Remote** tab with `origin/main` (showing
its short SHA); switching to the **Local** tab shows `main` with its own, different short
SHA. Choosing `origin/main` starts the new branch from exactly that revision
(`work.base_branch == "origin/main"`, branch tip == `origin/main`'s object), distinct from
local `main`.

---

## S3 — First use works fully offline

**Covers:** US1 scenario 2; SC-005; FR-004, FR-026.

```bash
# run S1 under a network sandbox that blocks all sockets, e.g.:
unshare -rn bash -c 'work start "$src" --workspace "$WS" --base main --slug offline-ok --prefix "{slug}" --yes'
```

**Expect:** identical success to S1. No DNS, no sockets. Bootstrap used only embedded assets.

---

## S4 — Invalid path is caught before materialization and is correctable

**Covers:** US3 scenario 1; SC-004; FR-003, FR-027; exit 10/11.

```bash
work start /does/not/exist --workspace "$WS" --base main --slug x --prefix '{slug}' --yes ; echo $?   # -> 10
work start "$(mktemp -d)"  --workspace "$WS" --base main --slug x --prefix '{slug}' --yes ; echo $?    # dir, not a repo -> 11
# bare repo:
git init -q --bare "$bare"
work start "$bare" --workspace "$WS" --base main --slug x --prefix '{slug}' --yes ; echo $?            # -> 11
```

**Expect:** each exits with the code above; stderr `error: <token>: <message>`; the message
names only the supplied path and the category (no repo contents); **no** branch, worktree,
Work dir, snapshot, or `works` row created. Interactive form re-prompts for another path
instead of exiting for codes 10/11.

---

## S5 — Invalid slug / branch-name rejected before any Git mutation

**Covers:** US3 scenario 2; SC-004; FR-012, FR-013; exit 13.

```bash
work start "$src" --workspace "$WS" --base main --slug 'has spaces' --prefix '{slug}' --yes ; echo $?   # -> 13
work start "$src" --workspace "$WS" --base main --slug '..'         --prefix '{slug}' --yes ; echo $?   # -> 13
```

**Expect:** exit 13; message identifies the offending slug; `git branch --list` in `$src`
unchanged; no Work dir. Interactive form returns to the slug prompt.

---

## S6 — Branch collision detected (local / remote-tracking / worktree)

**Covers:** US3 scenario 3; edge cases; FR-012; exit 14.

```bash
( cd "$src" && git branch taken )
work start "$src" --workspace "$WS" --base main --slug taken --prefix '{slug}' --yes ; echo $?          # local -> 14
# remote-tracking collision and existing-worktree collision covered by fixtures
```

**Expect:** exit 14; message states which kind of collision; no partial state. Re-running S1's
exact command a second time now also yields exit 14 (the branch it created exists).

---

## S7 — Failure during materialization rolls back completely

**Covers:** US3 scenario 4; SC-003; FR-020; exit 17.

```bash
for step in worktree snapshot projection ; do
  WORK_FAIL_AT=$step work start "$src" --workspace "$WS" --base main --slug rollme --prefix '{slug}' --yes ; echo "$step -> $?"
done
```

**Expect:** each → exit 17; after each: `git -C "$src" branch --list rollme` empty,
`git -C "$src" worktree list` has no `rollme`, no `<workspace>/in-progress/demo_rollme/`,
no `works` row for it. `config/work.json` and the seed install remain intact.

---

## S8 — Cancel before confirmation leaves nothing

**Covers:** US3 scenario 5; FR-021; exit 20.

```bash
# Declining the confirm prompt requires an interactive terminal — the prompt is
# only shown when stdin AND stdout are TTYs. Under a pty harness:
work start "$src" --workspace "$WS" --base main --slug cancelme --prefix '{slug}'   # answer "n" at the confirm
echo $?    # -> 20

# SIGINT before the commit step: same outcome.
work start "$src" --workspace "$WS" --base main --slug intr --prefix '{slug}'       # Ctrl-C at the confirm
echo $?    # -> 20
```

A piped-stdin run is non-interactive, so it never reaches a confirm prompt: without
`--yes` it fails fast with exit 2 (`usage`), mutating nothing — which is also a valid
"leaves nothing" outcome, just a different code.

**Expect:** exit 20 for the interactive decline / SIGINT; zero artifacts for
`cancelme` or `intr` in every case.

---

## S9 — Non-interactive missing value fails without a TUI or mutation

**Covers:** US2 scenario 5; FR-024, RF-51; exit 2.

```bash
work start "$src" --workspace "$WS" --base main --prefix '{slug}' --yes < /dev/null ; echo $?   # no --slug -> 2
```

**Expect:** exit 2; stderr names the missing `--slug`; no TUI; no config write; no Work state.

---

## S10 — Shell integration repositions; its absence is reported honestly

**Covers:** US1 scenario 1 (session ends in the checkout); FR-022, FR-023; SC-006.

```bash
# with integration (fake-shell harness):
eval "$(work shell-init bash)"          # via harness
work start "$src" --workspace "$WS" --base main --slug shell-ok --prefix '{slug}' --yes
# harness asserts: cwd is now <workspace>/in-progress/demo_shell-ok/worktree

# without integration:
env -u WORK_CD_FILE work start "$src" --workspace "$WS" --base main --slug shell-note --prefix '{slug}' --yes
```

**Expect:**
- With integration: temp file received the absolute worktree path; harness `cd`ed there; exit
  0; no FR-023 notice.
- Without: exit 0; stdout success summary present; stderr has the FR-023 notice with the real
  worktree path on its own line and the `eval "$(work shell-init …)"` hint. No claim of a `cd`.

---

## S11 — Bootstrap is idempotent under repetition and interruption

**Covers:** US1 edge case (bootstrap already completed / interrupted); SC-007; FR-005.

Run by the test suite: loop `bootstrap.EnsureSeed()` 100× and interrupt extraction at injected
points 20×. **Expect:** exactly one registry entry for each of `local-path-starter`,
`filesystem-repository-locator`, `freeform`; one `plugins/work-reference/` dir; no partial dir.

---

## S12 — Seed subprocess contracts (isolated)

**Covers:** roadmap §4 "Contrato de processo"; `contracts/ipc-starter.md`,
`contracts/ipc-repository-locator.md`.

Run by `tests/contract/`: feed golden stdin JSON to `seed/dist/<host>/starter` and `.../locator`,
assert stdout JSON shape and exit code for every row in those contracts' test tables,
including garbled stdin → non-zero exit, no stdout.

---

## Traceability

| Scenario | Spec acceptance / SC |
|---|---|
| S1 | US1 #1,#3,#4 · SC-001, SC-002 |
| S2 | US1 edge cases · FR-009 |
| S3 | US1 #2 · SC-005 |
| S4 | US3 #1 · SC-004 |
| S5 | US3 #2 · SC-004 |
| S6 | US3 #3 |
| S7 | US3 #4 · SC-003 |
| S8 | US3 #5 |
| S9 | US2 #5 |
| S10 | US1 #1 · SC-006 |
| S11 | SC-007 |
| S12 | roadmap §4 |
