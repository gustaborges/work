# Agent authoring skills

Work bundles `work-cli-plugin-builder` as offline text guidance, reference tables and a complete marketplace scaffold. It helps local agents author marketplaces, manifests, conventions and all four component roles. Skill management never contacts an AI service or opens Work application state.

Choose exactly one vendor and scope. Every installation includes **all skills bundled with this Work release**, initially `work-cli-plugin-builder`. Individual skill operands, repeated vendors and `--vendor=all` are not supported.

```sh
work skills install --vendor=codex --scope=local
work skills install --vendor=claude --scope=user
work skills install --vendor=codex --scope=user --yes
work skills status --vendor=codex --scope=local --json
```

`local` installs in the current directory; `user` installs in the agent's personal directory. Both scopes preview all included skills and their destinations and ask for confirmation. `--yes` skips confirmation and is required without an interactive terminal. `--force` separately permits replacing ordinary modified/foreign content; it still prompts unless combined with `--yes`. Neither flag bypasses path/recovery safety checks. Work does not detect or require installed agent executables. No marketplace catalog is required.

Unknown vendors, missing/invalid scope, skill operands and the removed `--global`/`--dir` flags fail before writes. Install rejects --json; status accepts it and never prompts. Bare `work skills` displays help.

| Agent | User destination | Local destination (relative to current directory) |
| --- | --- | --- |
| Claude Code | `${CLAUDE_CONFIG_DIR:-HOME/.claude}/skills/work-cli-plugin-builder` | `.claude/skills/work-cli-plugin-builder` |
| Codex | `HOME/.agents/skills/work-cli-plugin-builder` | `.agents/skills/work-cli-plugin-builder` |

A relative CLAUDE_CONFIG_DIR resolves against the initial working directory. Empty means default. Explicit anchors are canonicalized; linked descendants and unsafe payload nodes are refused. Copies contain identical payload bytes. Invoke `/work-cli-plugin-builder` in Claude Code or `$work-cli-plugin-builder` in Codex. Claude watches existing directories; use `/reload-skills` if the top-level skills directory was created during the session, including after further edits there. Codex detects skills automatically; restart if a copy is missing. Actual override/reload behavior remains a release validation requirement.

The root `.work-skill.json` records the installed payload digest. Current copies are unchanged, including mtimes. Outdated owned copies update explicitly. Modified or foreign ordinary copies require --force, which replaces their local content; inspect or preserve edits before using it. Force never bypasses unsafe nodes, unreadable content, changed observations or unresolved recovery. Binary upgrades do not refresh copies silently.

Status reports absent, current, outdated, modified, foreign, conflict, recovery_required or unreadable. Ordinary states exit zero; operational states fail while retaining all inspected result rows. JSON contains ordered rows and nullable digest evidence; notes and hints go to stderr. Status creates no directories, locks or recovery artifacts and never repairs pending operations.

Install preflights the whole selection, takes persistent sibling locks in canonical order and retains exact old trees as backups until each destination commits. Earlier commits survive later failures. Receipts distinguish committed content, cleanup-pending, failed and not-attempted destinations. A brief directory rename gap is possible for external readers. Locks coordinate cooperating installers; this is not a guarantee against malicious external writers or arbitrary power loss.

Coordination siblings use `.work-skill-NAME.lock`, `.work-skill-NAME.transaction.json`, and operation-specific stage/backup names. Keep these local; only the skill payload directory is intended for Git publication. Work does not edit ignore files. Locks persist after release. Unknown lookalike artifacts are never wildcard-deleted. Re-run an explicit install to recover a valid interrupted operation or finish committed cleanup. If evidence is inconsistent, preserve it and inspect before moving it while no installer is active. Do not delete the only old copy. Automated removal is deferred.

Maintainers: validator and protocol descriptors generate committed references via `go generate ./ai-skills/...`. Review changes to guidance, scaffolds and fixtures, then run `make skill-lock` to update CONTRACT. Tests compare references and pin without writing. This detects represented contract drift; it does not prove prose or runtime semantics. Review parser differences, qualified private ownership, isolated every-entry validation and actual role behavior whenever contracts change. Keep changes in one PR.
