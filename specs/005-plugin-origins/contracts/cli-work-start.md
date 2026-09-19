# Contract: `work start [SOURCE]` — F4 amendment

Authority: ADR-0004, ADR-0011, ADR-0012, ADD §7/§8; research R7, R9, R12,
R15. **Amends** `specs/004-local-clone-locator/contracts/cli-work-start.md`,
which stays authoritative for everything not named here: preconditions,
first-run setup, the direct-path-vs-Locator-chain resolution pipeline itself,
the repository-ambiguity picker, and every F1/F2/F2.5 stdout line/exit code
this contract does not mention.

## Amended clause: Starter selection

F3's contract says the reference Starter is invoked directly (F1's only
Starter is always the fallback). **Superseded**: the SOURCE step now runs
`internal/starter.Match` first (`starter-protocol.md`), which may invoke a
specific Starter, the fallback, or present a collision — before the
existing "classify the reference, then resolve" logic runs unchanged on
whatever that Starter returns.

```text
SOURCE
  -> Match (pattern eval, local, no subprocess)          [NEW]
  -> Invoke the selected/chosen Starter                   (unchanged mechanism)
  -> classify repository.{path,git_fetch_urls,name,query} (unchanged, F3)
  -> direct validation or Locator chain                   (unchanged, F3)
  -> validated repo_path                                  (unchanged, F3)
```

### Interactive flow

- Zero/one pattern match: unchanged from the user's perspective — the SOURCE
  step behaves exactly as before, just against a different (possibly
  plugin-provided) Starter.
- Two or more pattern matches: a `present.Select` step ("Starter") appears
  **before** the SOURCE step's own validation runs, listing the colliding
  components (display name/alias, no ranking). The chosen Starter is then
  invoked exactly as a single match would be. The choice is not remembered
  for the next invocation. Within one invocation it is kept only for the
  argument it was made for: the Starter selector never opens inside a running
  `present.Wizard`, so re-submitting the SOURCE field unchanged reuses that
  choice, while an edited value that collides again fails in-field ("N
  Starters match ...; run `work start <arg>` to choose one").
- A Starter-provided reference that cannot be resolved (a Starter failure, or
  `no-repository-found` from the Locators) is never a silent fallback. The
  SOURCE field opens with that failure shown as an in-field error, and
  re-submitting the field shows the resulting error in-field again.

### Non-interactive / explicit-argv flow

A collision with no TTY to prompt on fails immediately —
`starter-ambiguous` (36), naming the colliding Starters, no selector opened,
no Work created (mirrors the existing `repository-ambiguous` (30) pattern
exactly).

## New clause: start modes

Once a Starter response is obtained (`starter-protocol.md`):

- **Absent `start_modes`**: unchanged F1/F3 journey — slug, convention,
  prefix, confirmation, new branch. `work.start_mode = "new"`.
- **Present `start_modes`**: after the repository is resolved (and, if
  needed, disambiguated), an additional `present.Select` step ("Mode") offers
  exactly the returned values and no others. Selecting **fork** continues
  into the identical slug/convention/prefix/confirmation sequence as the
  absent case, using the Starter's `base_branch` (if given, skipping the base
  prompt per FR-024) or prompting for one (FR-023). Selecting **contribution**
  skips directly to confirmation: no slug step, no convention step, no prefix
  step; the confirmation's impact preview shows the Starter-resolved branch
  being checked out, not a new branch being created from a base.
- Non-interactively, `start_modes` presence still requires the mode to be
  resolvable without a prompt: F4 introduces no new flag to preselect a mode
  non-interactively in this slice (Out of Scope: no additional `work start`
  flag surface beyond what F1/F3 already defined) — a non-interactive
  invocation against a Starter that returns `start_modes` therefore always
  needs `--yes` plus every other already-required flag, and the mode
  selection itself has no non-interactive equivalent in F4 (tracked as a
  known gap, not silently worked around — a future slice may add a `--mode`
  flag).

## New clause: convention resolution generalizes

F1/F3's convention handling was `convention.Freeform`, always. **Superseded**
outside contribution mode: the convention step now (research R15):

1. Loads the full catalog (`convention.Load(reg)`, already general).
2. Resolves the current repository's identity (ADR-0011) and looks up a
   memoized choice.
3. If memoized: uses it, no step shown.
4. If unmemoized and the catalog has exactly one entry: silently adopts and
   memoizes it, no step shown (mirrors the existing single-prefix collapse).
5. If unmemoized and the catalog has more than one entry: a
   `present.SelectStep` ("Convention") appears before the existing prefix
   step; the accepted choice is memoized for this repository's identity
   before the wizard advances.

Contribution mode never reaches this step at all (no convention is
input/output of contribution mode — unchanged from ADD §7/§8).

## Amended: persisted result

`work.start_mode` and `work.branch_convention` reflect exactly the resolved
mode and (when applicable) convention — never inferred, never overridden
(FR-022). `work-state.json` is written as schema 3
(`work-state.schema.json`).

## Amended: rollback guarantee

Every existing rollback guarantee (SC-009/FR-033) is preserved, with one new
rule: a failure or cancellation during a **contribution**-mode creation
leaves the checked-out worktree removed but the pre-existing branch intact —
it was never created by this Work, so there is nothing of the branch itself
to roll back (research R12). Fork/new-mode rollback is byte-identical to
F1/F3 (worktree removed, branch deleted).

## Contract tests (tests/integration/, pty and non-interactive)

| Case | Expect |
|---|---|
| S1 — single-pattern match, no start_modes | full new-Work journey via the specific Starter; identical stdout/snapshot shape to a direct-path F1 creation |
| S2 — collision, interactive | Starter selector appears before SOURCE resolves; only the chosen one invoked |
| S2b — collision, SOURCE from argv, reference not located | Starter selector once; SOURCE field opens with the resolution error; Enter re-validates in-field with no second selector and no hang |
| S3 — collision, non-interactive | exit 36, no Work, no selector |
| S4 — start_modes present, fork selected | full slug/convention/prefix sequence; `start_mode: "fork"` persisted |
| S5 — start_modes present, contribution selected | no slug/convention/prefix steps; existing branch checked out; `start_mode: "contribution"`, no `branch_convention` persisted |
| S6 — contribution cancelled mid-confirmation | worktree/dir/snapshot absent; pre-existing branch still present in the source repo |
| S7 — base_branch supplied by Starter (fork) | no base-branch prompt |
| S8 — base_branch absent (fork) | base-branch prompt, unchanged from F1 |
| S9 — unrecognized start_modes value | exit 37, no Work |
| S10 — contribution with no base_branch | exit 37, no Work |
| S11 — convention: single enabled | no convention step; memoized silently |
| S12 — convention: multiple enabled, first use | convention step appears once; memoized; not shown again from a second clone |
| S13 — regression | `work start <path>` (no plugin involved) byte-identical to F3 |
