# ADR-0011: Branch Convention Resolution per Repository — Interview with Memoization

**Status:** Accepted
**Date:** 2026-08-23
**Product Context:** `docs/prd.md` — RF-5, RF-23, RF-24, RF-25, RNF-1, RNF-3, Roadmap (automatic convention discovery)
**Governs:** `docs/add/add-0001-work-system-architecture.md`, Sections 6 and 8

## Context

Since branch convention is declarative data (ADR-0012), Work needs a model to decide, in `work start`, which convention — among those declared by all enabled plugins — applies to the destination repository. The PRD already excludes support for multiple simultaneous conventions within the same project (Roadmap/Out of scope v1); what remains is to decide how a single convention is chosen and resolved in subsequent runs without repeating the question.

## Decision

**Resolution model.** There is no *match* or plugin candidacy against repository content — there is currently no component that examines the repository to infer its convention (ADR-0012). On the first `work start` of a repository with no memoized convention, Work lists all conventions declared by enabled packages and the user chooses one, reusing the same TUI selection paradigm as the rest of the product (PRD, Section 10). The choice is memoized and automatically reused in subsequent runs in that same repository, without asking again.

**Name collision between plugins** (two plugins declaring a convention named `gitflow`) is resolved by the manifest alias mechanism (`<package-alias>/<name>`, ADR-0012) — not by the starter collision mechanism (ADR-0004), which solves a different problem: pattern matching disputes between recognizers over an argument, not choice between statically declared names.

**Repository identity key**, in layers:

1. **Remote URL** (e.g., `origin`) — stable when moving or recloning the local repository; primary layer.
2. **Hash of root commit(s)** (`git rev-list --max-parents=0 HEAD`) — used when no remote is configured. Stable when moving/renaming/recloning the directory, and does not introduce new infrastructure cost (git is already invoked in-process, ADR-0000). When the history has more than one root (unrelated histories merged, e.g., `--allow-unrelated-histories` or grafts), the hashes are sorted and concatenated into a single composite key — never is one of them chosen arbitrarily, preserving key determinism (RNF-3).
3. **Absolute local path** — last resort, used only when layer 2 is not reliable: repository in shallow clone (`git clone --depth=1`) *and* with no remote configured. Shallow clone breaks the invariance of the root commit (the "root" observed is the boundary of the shallow fetch, not the actual commit at the start of history); the intersection "shallow + no remote" is rare in practice, since a shallow clone normally already comes with an origin remote configured.

A repository with no commits (`HEAD` unborn) does not need specific handling here: `work start` already fails before this point, due to lack of base branch (RF-6 assumes a list of branches to offer, which doesn't exist in this state) — it is not a gap in this model, it is a general precondition of the flow.

**PR fork and repository identity.** A "fork" in the Work vocabulary (PRD, journey 7.3) is a checkout of a new branch from the branch of an existing pull request — never the creation of a distinct hosted repository. The local repository remote remains the same in any of the `work start` modes (7.1, 7.2, 7.3); there does not exist, in Work's data model, a scenario where two different remotes share the same history root and dispute the same memoized convention. Layer 2 of the key (root commit) only comes into play in the absence of a configured remote, something orthogonal to the `work start` mode chosen.

**Inspect or change the memoized convention** of a repository uses the top-level hub `work convention` (RF-25), executable from any clone of the repository.

**`work convention` command.** The convention memoized per repository is one of the choices that Work keeps on behalf of the user — starter collision (ADR-0004) stopped being memoized. A single choice does not justify grouping it under plugins. `work convention` opens the TUI showing the current choice and allowing it to be changed; `work convention show` is the direct query and `work convention set <convention>` does the explicit substitution. Runs from any clone of the repository — the identity key (above) is derived from the git of the current directory, not from a worktree initialized by Work; outside a git repository, it fails with a clear message. Syntax and flow in `add-0001` §8; grammar governed by ADR-0017.

## Alternatives Considered

* **Executable detection component**, which examines the repository and returns the convention name directly, eliminating the interview. Not rejected as infeasible — the complexity of such a script is decided by the plugin author, not imposed by the core (same reasoning as ADR-0000/ADR-0006), and could even return a fixed value when real detection is not implemented. Deferred to a future version (recorded in PRD Roadmap): v1 resolves entirely by interview and memoization; a dynamic detector, when it exists, is a new executable role in `components[]` (ADR-0012) that can pre-fill or skip the interview without requiring a change to the identity/memoization model decided here.
* **Pattern matching against repository content**, along the lines of what ADR-0004 does for starters against a textual argument. Rejected for v1: conventions are static data (name + prefixes), with no signal to compare against the repository state — this comparison is only possible with an executable detection component (see above), which is exactly the future extension left open, not something to build now.
* **Reuse the collision mechanism from ADR-0004** for convention choice. Rejected: these are different problems — ADR-0004 resolves pattern matching ambiguity between starters over an argument, always on-the-fly and without persistence; here there is a first-use choice that needs to be memoized to avoid repeating the question. Nothing to share between the two.
* **Group convention change under `work plugin`.** Rejected: it mixes plugin lifecycle management (install/enable/update) with a user choice about a repository — two different mental models. A dedicated top-level hub (`work convention`) is more direct, since it is a repository configuration choice with its own commands.
* **Flag on `work start`** instead of a dedicated command. Rejected: changing the convention is a one-off and rare action, not tied to the start of a specific work item; embedding it as a flag of `work start` would force the user to start a work item just to change the convention.

## Consequences

**Positive:** deterministic (RNF-3) without depending on heuristic detection against the repository — which would inherently be subject to false positives/negatives; identity key reuses git already invoked in-process, without new infrastructure; clear separation between name collision (alias), pattern collision (ADR-0004), and convention choice (here), each resolved by the right mechanism; path open for future dynamic detection without reopening this decision.

**Negative / Trade-offs:** first use of each repository costs an extra interaction compared to hypothetical automatic detection — mitigated by being a one-time cost per repository (memoized thereafter) and by RF-24 guaranteeing that there always exists at least one convention available, even without any third-party plugins installed.
