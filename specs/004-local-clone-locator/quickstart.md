# Quickstart / Validation Guide: Find the Local Clone (F3)

**Feature**: `specs/004-local-clone-locator/` · **Plan**: [plan.md](./plan.md)

Runnable scenarios that prove F3 end to end. Shapes, tokens, and rules live in
[`contracts/`](./contracts/), [`data-model.md`](./data-model.md), and
[`research.md`](./research.md) — not repeated here.

**Compatibility baseline (SC-011):** the F2 suite
(`specs/002-daily-cycle/quickstart.md` S1–S13), the F2.5 suite
(`specs/003-terminal-ux-revamp/quickstart.md` Q1–Q12), and all non-interactive F1
scenarios MUST pass **unchanged** alongside these. Any diff in their stdout, error
tokens, mutations, or exit codes is a regression. F3's contracted changes:
`work start` SOURCE may be a name/reference, and the interactive `work start`
wizard gains up-front first-run setup (`contracts/cli-work-start.md`). Interactive
F1 fresh-install scenarios (`specs/001-first-local-work/quickstart.md` S1–S12)
therefore answer two extra up-front prompts (workspace root — which F1 already
asked, later — and one search root); their pty scripts are updated with those
answers and nothing else — no stdout/exit/snapshot change.

## Prerequisites

- Go 1.26+, system `git` >= 2.5.
- A Unix host with a PTY for the interactive scenarios (`//go:build unix`,
  `github.com/creack/pty`); Windows keeps non-interactive coverage plus a manual
  smoke of S1 and S8.
- **No network** for any scenario.
- F2.5 merged (`internal/present` with `Wizard`/`SelectStep` present).

## Build & isolation

```bash
make build
export WORK_HOME="$(mktemp -d)/dotwork"
export PATH="$PWD/bin:$PATH"
export WS="$(mktemp -d)/workspaces"

# two search roots, separate from the workspace
export R1="$(mktemp -d)/src"; export R2="$(mktemp -d)/mirror"
mkrepo() { mkdir -p "$1" && git init -q "$1" && git -C "$1" commit -q --allow-empty -m init && git -C "$1" branch -M main; }
mkrepo "$R1/payments"
mkrepo "$R1/ledger"
```

---

## S1 — First run puts the seed Locator in the policy (US5; FR-030, SC-005)

Fresh `WORK_HOME`. Run `work repository policy list`.

**Expect**: exit 0, one row
`1  work-reference/filesystem-repository-locator  (available)`.
`work repository locator list --json` shows that Locator with
`"accepts":["name","git_fetch_urls","query"]` and `"in_policy":true`. No network.

---

## S2 — Configure search roots (US3; FR-020, FR-022)

```bash
work repository root add "$R1" "$R2"
work repository root list --json      # {"roots":["…/src","…/mirror"]}
```

**Expect**: roots persisted in `work.json` **beside** `workspace`, absolute, in
order. `work repository root list` (no `--json`) prints one path per line. Adding
`$R1` again is a no-op, exit 0. `work.json` remains valid `config.Load` input.

Then check the isolation rule (FR-020):

```bash
work repository root add "$WS"            # workspace root itself
work repository root add "$WS/in-progress"
mkdir -p "$R2/nested"; export WS2="$R2/nested"   # a workspace inside a root
```

**Expect**: `work repository root add "$WS"` and `"$WS/in-progress"` both fail
`error: usage:` naming the root and the workspace and the reason; nothing is
persisted. (The mirror case — pointing `workspace` inside a configured root — is
rejected by `work`'s workspace validation with the same message.)

---

## S3 — Start a Work by name, single match (US1 🎯; FR-001, FR-004, SC-001)

**PTY, 80×24.** `work start payments`. No `SOURCE` path is typed.

**Expect**: no path prompt, no repository picker — resolution finds `$R1/payments`
silently. The wizard continues at prefix/slug/base/workspace/confirm exactly as
F1. On confirm: the three F1 stdout lines (`work: created … / branch … / path …`)
and the terminal repositions into the new worktree. `internal/work/verify.Check`
passes.

---

## S4 — `work start <path>` still works identically (regression; FR-036, SC-008)

**PTY.** `work start "$R1/payments"` (explicit path) in a second run.

**Expect**: byte-identical journey and snapshot fields to S3 (same `work.starter`,
base, branch shape) apart from the slug/branch the user picks. A head-to-head test
diffs the two `work-state.json` files (normalising slug/branch/timestamps) → no
other difference.

---

## S5 — Choose between two matching clones (US2; FR-013, SC-002)

```bash
mkrepo "$R2/payments"        # a second clone of the same name
```

**PTY, 80×24.** `work start payments`.

**Expect**: a **Repository** select step appears with two options — primary line
the absolute path, secondary line the remote URL or parent dir — bounded to the
viewport, filterable, `❯` focus marker. Pick `$R2/payments`. The wizard continues;
the created Work's source is `$R2/payments`, **not** `$R1/payments`. The step
collapses to a `payments (…/mirror)` receipt. The chain does not resume.

---

## S6 — Ambiguous match, non-interactive → exit 30 (US2; FR-033)

```bash
work start payments --workspace "$WS" --base main --prefix '{slug}' --slug x --yes </dev/null
```

**Expect**: **exit 30**, `error: repository-ambiguous: …` on stderr with a hint to
pass a more specific reference or adjust roots. **No selector.** No Work, no
branch, no config write.

---

## S7 — No match → exit 26, distinct from a broken folder (US4; FR-019)

Non-interactive `work start nonesuch --workspace "$WS" --base main --prefix '{slug}' --slug x --yes </dev/null`.

**Expect**: **exit 26** `no-repository-found`, guidance to check the reference or
`repository_roots`. No Work.

**PTY variant**: at the source prompt type `nonesuch`; the error shows **in-frame**
and the field is re-promptable; then type `payments` and the journey continues
(S3 path).

---

## S8 — Invalid candidate vs Locator failure are distinct (US4; FR-015, FR-018)

Using fake Locators from `tests/fixtures/locators/`:

- policy = `[invalid]` (returns a match that is not a git repo) → non-interactive
  `work start whatever …` → **exit 28** `repository-candidate-invalid`, naming the
  rejected path.
- policy = `[boom]` (exits 1) → **exit 29** `locator-failed`, naming the Locator;
  a second policy entry after `boom` is **never consulted** (no fallback).

**Expect**: 26 ≠ 27 ≠ 28 ≠ 29 in all cases; no Work in any.

---

## S9 — Reorder and prune the policy (US5; FR-023, FR-024)

```bash
work repository policy add acme/corp-index --after work-reference/filesystem-repository-locator   # fixture Locator, pre-installed
work repository policy move acme/corp-index --before work-reference/filesystem-repository-locator
work repository policy remove acme/corp-index
work repository policy replace work-reference/filesystem-repository-locator
```

**Expect**: each command prints one stable `work: policy now …` line; the
persisted order matches after each; `move` with neither/both of
`--before`/`--after` → exit 2; an unknown `<LOCATOR>` → exit 2. Removing
`acme/corp-index` leaves it **installed** (`work repository locator list` still
shows it, `in_policy:false`).

---

## S10 — Installing a Locator does not touch the policy (US5; FR-027, SC-005)

Capture `work.json`. Install a second fixture Locator plugin
(`work plugin install <fixture> --link` when F4 lands; for F3 use the test that
registers a component directly). Re-read `work.json`.

**Expect**: `repository_resolution.locators` is **byte-identical**. The new
Locator appears in `work repository locator list` with `in_policy:false`.

---

## S11 — Unavailable policy entry is shown, not hidden (US5; FR-025)

Hand-edit `work.json` to add `ghost/missing-locator` to `repository_resolution.locators`.
`work repository policy list`.

**Expect**: the entry appears in its position marked `(unavailable)`;
`--json` shows `"available:false`. A `work start <name>` run **skips** it and
resolves via the seed Locator; no `locator-failed`.

---

## S12 — `work repository` discovery + help-only parent (US5; FR-022, FR-029)

- `work` → brand; `work --help` → **Administration** block lists `repository`
  exactly once.
- `work repository </dev/null` (non-interactive) → grouped help for `locator` /
  `policy` / `root` on stdout, **exit 0**, no ANSI.
- **PTY** `work repository` → the **same** grouped help, exit 0, **no menu**.
- `work repository --json` → `error: usage:` **exit 2** (mutation-style rejection).
- (The interactive `repository` hub of ADR-0019 is deferred past F3 —
  `contracts/cli-work-repository.md` §Scope note.)

---

## S13 — First-run setup at `work start` (US3; FR-022a, FR-020, SC-013)

Fresh `WORK_HOME` **and** unset `workspace` (do not pre-run S2).

- **PTY** `work start payments`:
  - the **first** prompt asks for the workspace root, with a line explaining what
    it is for; accept the suggestion or type `$WS`.
  - the **second** prompt asks for a repository search root, with its own purpose
    line; type `$R1`.
  - resolution then finds `$R1/payments` and the wizard continues at
    prefix/slug/base as in S3. `work.json` has `workspace` and `repository_roots`
    persisted **before** the first journey step.
- **PTY**, run `work start ledger` again → **neither** prompt appears; straight to
  resolution.
- **PTY**, at the search-root prompt type `$WS/in-progress` → rejected in-frame
  with the overlap message, re-promptable; Ctrl-C at either prompt → exit 20, no
  Work, `work.json` unchanged.
- **Non-interactive** `work start payments </dev/null` on a fresh home with no
  roots → `error: no-repository-found` **exit 26** with a hint to
  `work repository root add`; **no** prompt, **no** hang. `work start "$R1/payments"`
  (a path) on the same fresh home → proceeds normally (no root needed).

---

## Regression checklist (must stay green)

- F1 `quickstart.md` S1–S12 — non-interactive unchanged; interactive fresh-install
  scenarios add the two up-front setup answers to their pty scripts, no other diff.
- F2 `quickstart.md` S1–S13 — unchanged.
- F2.5 `quickstart.md` Q1–Q12 — unchanged.
- `tests/contract/locator_test.go` (seed binary IPC) — unchanged.
- `diag` category table test — extended with 26–30, codes 0–25 unchanged.
- Performance: a 500-repo tree, `work start <unique-name>` non-interactive
  completes < 2 s on the CI reference runner (SC-012).
