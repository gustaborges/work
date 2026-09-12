# Contract: Repository Locator resolution (F3)

**Promotes** `specs/001-first-local-work/contracts/ipc-repository-locator.md` from
"built and contract-tested but never executed on the happy path" to the **live**
resolution contract. The F1 document stays as the seed-binary IPC record; its
"Outcomes as seen by the core" table is now normative here.

Authority: ADR-0014, ADR-0015, ADR-0016; ADD §7.1; spec FR-003, FR-004,
FR-006–FR-019, FR-037.

## Pipeline

```text
Repository Reference (no path)
  → for each entry in repository_resolution.locators, in order:
        available?  ─no→ skip, continue
        eligible?   ─no→ skip, continue          (accepts ∩ present fields)
        InvokeLocator(projected input):
            transport / non-zero exit / unparseable stdout ─→ HALT: locator-failed (29)
            {"matches": []}                                 ─→ continue
            {"matches": [>=1]}                              ─→ END TRAVERSAL, evaluate this Locator only
  → validate + dedup this Locator's candidates (reporef.ValidatePath, resolved-path key)
        1 valid   → Outcome.Resolved
        >=2 valid → Outcome.Candidates   (ambiguous)
        0 valid   → repository-candidate-invalid (28)
  → traversal ended with no Locator returning candidates:
        some Locator was eligible → no-repository-found (26)
        none was ever eligible    → no-eligible-locator (27)
```

## Rules

- **Order is the only precedence.** No score, rank, confidence, `priority`,
  `winner`, installation-order, or manifest-declared position — anywhere
  (FR-014, NFR-9, ADR-0015).
- **First results win the chain.** Once a Locator returns a non-empty `matches`,
  later Locators are **never consulted** (FR-011). `matches: []` is the only
  "continue" signal.
- **No aggregation.** Candidates are evaluated per-Locator; results from different
  Locators are never merged (ADR-0015).
- **Operational failure ≠ no result.** A Locator error **halts** resolution with a
  diagnostic naming that Locator; there is **no silent fallback** to the next one
  (FR-015, ADR-0015 — no `continue_on_locator_error` in v1).
- **The core is the final validity authority** (`reporef.ValidatePath`:
  existing + readable dir, non-bare git, ≥1 commit) (FR-016, ADR-0014).
- **Dedup** by `filepath.EvalSymlinks`-resolved absolute path — path variants and
  symlinked ancestors collapse to one candidate (FR-017, SC-009).
- **Roots only.** Locators receive `repository_roots` and nothing pointing at
  `workspace/in-progress` or `workspace/archived` (FR-021, ADD §7.2).
- **Transient.** The reference and the candidates are discarded once `repoPath` is
  known; nothing about them is written to the snapshot or index (FR-005, R16).

## Outcome → exit code (in `work start`)

| Outcome | Token | Code | Interactive source step |
|---|---|---|---|
| one valid repo | — | 0 | proceeds |
| ≥2 valid repos | (picker) / `repository-ambiguous` | 0 / 30 | interactive: `present.Select`; non-interactive/argv: exit 30 |
| every eligible Locator returned `[]` | `no-repository-found` | 26 | recoverable in-frame (re-enter reference) |
| empty policy, or no Locator accepts any reference field | `no-eligible-locator` | 27 | terminal |
| ending Locator's candidates all invalid | `repository-candidate-invalid` | 28 | terminal |
| a Locator errored | `locator-failed` | 29 | terminal (chain halted) |

## Seed `filesystem-repository-locator` (unchanged from F1)

Manifest entry, IPC shape, depth bound, `git_fetch_urls` comparison across all
remotes, and "unreadable root → exit ≠ 0" are exactly as
`specs/001-first-local-work/contracts/ipc-repository-locator.md` and
`tests/contract/locator_test.go` — **no change**. In F3 the unreadable-root exit
is classified by the core as `locator-failed` (29) for that Locator (research
R11).

## Tests

| Layer | Coverage |
|---|---|
| `tests/contract/locator_test.go` | seed binary IPC — **already green, untouched** |
| `tests/contract/locator_resolution_test.go` (new) | projected `LocatorInput` for every reference shape; the six outcome rows against fake Locators |
| `internal/locator` unit | eligibility, traversal order, halt-no-fallback, dedup, the five error categories |
| `tests/integration/resolution_outcomes.txtar` (new) | `no-repository-found` / `repository-candidate-invalid` / `locator-failed` are distinct exits, no Work created |
