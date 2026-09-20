# Quickstart / Validation Guide: Automatic Context on Start (F5)

**Feature**: `specs/006-automatic-start-context/` · **Plan**: [plan.md](./plan.md)

Runnable scenarios that prove F5 end to end. Shapes, tokens and rules live in
[`contracts/`](./contracts/), [`data-model.md`](./data-model.md) and
[`research.md`](./research.md) — not repeated here.

**Compatibility baseline (SC-008):** the F1 suite (`specs/001-first-local-work/quickstart.md`
S1–S12), F2 (`specs/002-daily-cycle/quickstart.md` S1–S13), F2.5
(`specs/003-terminal-ux-revamp/quickstart.md` Q1–Q12), F3
(`specs/004-local-clone-locator/quickstart.md` S1–S13) and F4
(`specs/005-plugin-origins/quickstart.md` S1–S14) MUST pass **unchanged** alongside these.
F5's changes are additive and fire only when an installed Linker/Importer is eligible or a
Starter publishes `meta`/`links` — none of which is true with only the reference package
installed, which is exactly the F1–F4 fixture shape.

## Prerequisites

- Go 1.26+, system `git` >= 2.5, F4 merged (`internal/plugininstall`, `starter.Match`).
- **No network.** Every scenario uses local repositories and locally built fixture plugins.
- The runtime scenarios (S3) need `sh` on `PATH` and are skipped on Windows.
- Automation equivalents live under `tests/integration/` (named per scenario below); this
  guide is the manual/scripted form of the same journeys.

## Build & isolation

```bash
make build
export WORK_HOME="$(mktemp -d)/dotwork"
export PATH="$PWD/bin:$PATH"
export WS="$(mktemp -d)/workspaces"
export R1="$(mktemp -d)/src"
mkrepo() { mkdir -p "$1" && git init -q "$1" && git -C "$1" commit -q --allow-empty -m init && git -C "$1" branch -M main; }
mkrepo "$R1/demo"
work repository root add "$R1"

FIX="$PWD/tests/fixtures/plugins"
# Build a fixture package into an installable directory (plugin.json + one binary per entrypoint).
fixture() { local d; d="$(mktemp -d)"; cp "$FIX/$1/plugin.json" "$d/";
  for e in $(sed -n 's/.*"entrypoint": *"\([^"]*\)".*/\1/p' "$d/plugin.json" | sort -u); do
    go build -o "$d/$e" "$FIX/$1"; done; echo "$d"; }
start() { work start "$1" --workspace "$WS" --base main --slug "${2:-s1}" --prefix '{slug}' --yes; }
export WORK_FIXTURE_LOG="$(mktemp -d)/launches.log"   # every fixture launch appends {role, stdin} here
```

`context-suite` ships entrypoints `starter`, `linker`, `linker2`, `importer`, `importer2`; the
binary picks its role from its own name. `WORK_FIXTURE_MODE` is a comma-separated list of
`<entrypoint>-<mode>` tokens (e.g. `linker-value,importer2-collide`); each entrypoint reads only
the tokens carrying its own name and defaults to its plain success behaviour when it finds none.
`restricted-suite` is the companion package for S7 (a Linker restricted to another Starter, a
Linker with `automatic:false`, an Importer needing an absent link, a manual-only Importer).

---

## S1 — Install a package that declares Linkers and Importers (US1; FR-001, FR-004, FR-005)

```bash
work plugin install "$(fixture context-suite)"
grep -c '"role"' "$WORK_HOME/state/registry.json"        # components registered
```

**Expect**: exit 0. `registry.json` holds, for `importer`/`importer2` the `on`, `manual` and
`inputs`; for `linker`/`linker2` the `key`, `discover` (with `automatic`, `on`, `inputs`) and
`manual`; and `packages[]` carries `plugin_name`. Both Linkers declare the same key and both
installed. `work plugin list` output is unchanged from F4. Reinstalling the same directory
after removing the `importer2` component from its `plugin.json` leaves `importer2` absent from
the registry (FR-005).

## S2 — Invalid declarations register nothing (US1; FR-001..FR-006, SC-009)

Install each of the checked-in invalid manifests under `tests/fixtures/plugins/invalid-*`:

| Fixture | Rule |
|---|---|
| `invalid-event` | subscription to an event Work does not define (M1) |
| `invalid-input` | malformed input string; `work:` fact outside the set (M3) |
| `invalid-dup-input` | same key in two namespaces (M4) |
| `invalid-key-owner` | Linker `key` private to another plugin (M5) |
| `invalid-manual` | `manual` without `display_name`, or `manual: true` (M6, R1) |

**Expect** each: exit 31, `error: plugin-invalid: …` naming the rule, `registry.json` and
plugin storage unchanged.

## S3 — Runtime is honored and preflighted (US1; FR-007, D8)

```bash
work plugin install "$(fixture runtime-sh)"              # starter.sh, "runtime": "sh"
start demo-rt-1 rt1                                      # runs `sh starter.sh` — no shebang/exec bit needed
work plugin install "$FIX/invalid-runtime"               # "runtime": "no-such-interpreter"
```

**Expect**: the first `start` creates a Work (the interpreted Starter ran). The `invalid-runtime`
install exits **34**, `error: plugin-install-failed: … "no-such-interpreter" … not found on PATH`,
nothing registered.

## S4 — Starter-published context is in the first snapshot (US2; FR-012..FR-015, SC-006)

```bash
export WORK_FIXTURE_MODE=starter-context               # Starter returns meta + links
start demo-ctx-1 c1
cat "$WS"/in-progress/demo_c1/work-state.json
```

**Expect**: `schema` is 3; `meta` = `{"github.pull_request.number": 212}`, `links` =
`{"github.pull_request": "https://example.test/pr/212"}`; `work` section unchanged in shape.
`sqlite3 "$WORK_HOME/state/work.db" 'select * from work_provenance'` shows both entries with
`source_component=<alias>/starter`, `source_operation=start`. With `WORK_FIXTURE_MODE` unset the
sections are `{}` and no provenance rows exist. Reading the file with the F4 tooling
(`work resume` from an F4 build) succeeds (FR-015).

## S5 — An invalid Starter key fails the start with nothing left (US2; FR-012, SC-006)

```bash
WORK_FIXTURE_MODE=starter-bad-key start demo-ctx-1 c2   # links: {"GitHub.PR": "..."}
WORK_FIXTURE_MODE=starter-foreign-private start demo-ctx-1 c3   # meta key plugin.someone-else.x
```

**Expect** each: exit **37**, `error: starter-response-invalid: … key …`; no branch, worktree,
Work directory, snapshot or index row (`git branch`, `ls "$WS"`, `work.db`). (A non-string link
value is a decode failure and exits 11 as it always has — `contracts/starter-protocol.md`.)

## S6 — An eligible Linker discovers a link (US3; FR-016..FR-026)

```bash
: > "$WORK_FIXTURE_LOG"
WORK_FIXTURE_MODE=linker-value start demo-ctx-1 l1 2>err.txt; echo "exit=$?"
```

**Expect**: exit 0; stdout is exactly the three `work:` lines; `err.txt` contains `work: running
<alias>/linker (discover)` then `work: <alias>/linker: linked github.pull_request`;
`links["github.pull_request"]` holds the Linker's value, replacing the Starter's (last source
wins); provenance row is `source_operation=discover`. `WORK_FIXTURE_MODE=linker-none` changes
nothing and prints nothing for that Linker. `linker-empty-value` yields an
`extension-response-invalid` warning and no change.

## S7 — Ineligible components never start; inputs are exactly the declared ones (US3; FR-018, FR-021, SC-002, SC-003)

Use a second package whose Linker restricts to a different Starter, and Importers requiring an
absent link. Run `start` and inspect `$WORK_FIXTURE_LOG`.

**Expect**: the log has **no** entry for the restricted/unsubscribed/`automatic:false`/
missing-input components. Every entry that exists has an `inputs` object whose keys are exactly
the declared inputs that resolved (bare keys; an absent optional input omitted, never null); no
`starters`, no snapshot path.

## S8 — Same-key providers resolve deterministically (US3; FR-020, FR-026, SC-007)

`linker` and `linker2` both declare `github.pull_request` and both return values. Run the start
20 times (fresh repo each) and compare.

**Expect**: identical order every time (`…/linker` before `…/linker2`), the stored value is
`linker2`'s in all 20, and provenance names `<alias>/linker2`.

## S9 — An Importer consumes a discovered link and its files land beside the worktree (US4; FR-019, FR-027..FR-033)

```bash
id="$(WORK_FIXTURE_MODE=linker-value,importer-ok start demo-ctx-1 i1 2>/dev/null | sed -n 's/^work: created //p')"
ls "$WS/in-progress/demo_i1"          # worktree/  work-state.json  notes/context.md  (importer output)
work archive "$id" --yes              # F2 archive
ls "$WS"/archived/*-demo_i1/          # work-state.json  notes/context.md  — artifacts preserved
```

**Expect**: the Importer's logged input carries the link the Linker just discovered (phase 2
re-evaluates after phase 1); nothing was written inside `worktree/`; no `work/import-*` stage
remains in `$TMPDIR`; `importer-empty` succeeds silently; the artifacts survive archiving.

## S10 — Collisions are refused entirely (US5; FR-029, FR-033, SC-004)

```bash
WORK_FIXTURE_MODE=importer-ok,importer2-collide start demo-ctx-1 x1
```

**Expect**: `importer` (sorted first) incorporated; `importer2` contributed **nothing** and one
`extension-output-refused` warning named `<alias>/importer2`, the operation `import` and the
relative path. Repeat with `importer-mixed` (one clean + one colliding file): neither is
added. Repeat with `importer-symlink`, `importer-into-worktree`, `importer-over-state`: each
refused; the Work directory listing and bytes are identical before/after that Importer
(hash the tree to check).

## S11 — Incorporation failure is rolled back (US5; FR-030, SC-004)

```bash
WORK_FAIL_AT=incorporate:1 WORK_FIXTURE_MODE=importer-ok start demo-ctx-1 f1
```

**Expect**: the fault fires after the first placed item; the tree is byte-identical to a run
with no Importer; `extension-persist-failed` warning; exit 0; no stage directory remains.

## S12 — Extension failure never damages the Work (US6; FR-035..FR-039, SC-005, SC-010)

Run one start per mode: `linker-exit1`, `linker-garbage`, `importer-exit1`, `importer-garbage`,
plus a Linker whose entrypoint was deleted after install (`--link` package).

**Expect** each: exit **0**; stdout still the three lines; worktree present; `work-state.json`
valid; exactly one `warning: extension-…` line naming the Work id, `<alias>/<name>` and the
operation; the healthy sibling extension still ran; a dependent Importer whose input the failed
Linker would have supplied did not run and added no second warning. `WORK_DEBUG=1` additionally
prints the exit status and stderr tail; **without it** no warning contains a link/meta value,
artifact content, or the extension's stderr.

## S13 — Interrupt during the automatic phases (US6; FR-038, D9)

```bash
WORK_FIXTURE_MODE=linker-hang start demo-ctx-1 h1 &   # then send SIGINT to the work process
```

**Expect**: the hung Linker is killed; one `extension-interrupted` warning; remaining components
skipped; the Work exists and is valid; exit 0; the cd-target file is still written.

## S14 — Visibility, plain output and determinism (US7; FR-040..FR-042, SC-007)

```bash
NO_COLOR=1 TERM=dumb start demo-ctx-1 v1 2>plain.txt
grep -c $'\x1b' plain.txt        # 0
```

**Expect**: same progress/warning text as S6/S12 with no control sequences; interactive (PTY)
shows the same lines, themed. With **only the reference package** installed the run prints
nothing extension-related and `$WORK_FIXTURE_LOG` stays empty (FR-041).

## S15 — The roadmap's five-step F5 demonstration, as one journey (SC-011)

1. Starter publishes metadata + a link → S4. 2. Linker discovers another value; upsert → S6.
3. Importer consumes the new link, stages, incorporates without collision → S9. 4. A second
Importer collides; none of its output incorporated → S10. 5. An automatic extension fails;
`work start` ends with a warning, Work usable → S12.
Automated as `tests/integration/f5_demo_test.go`; runs green alongside F1–F4 demos.

---

## Regression sweep

```bash
GIT_CONFIG_GLOBAL=/dev/null go test ./... && make lint
```

`GIT_CONFIG_GLOBAL=/dev/null` keeps `gittest` repositories independent of host signing config.
Then re-run the F1–F4 quickstarts against the same build; any stdout/token/exit-code diff is a
regression. Confirm on a fresh `WORK_HOME` with **only the reference package**: `work start
<path>` output, exit code and created files are byte-identical to the F4 build, and
`work.db`'s `user_version` is 3 after the first F5 command on an F4-era home (one lossless
rebuild — `contracts/index-provenance.md`).
