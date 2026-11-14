# Agent authoring skills

Work bundles `work-plugin-builder` as offline text guidance, reference tables and a complete marketplace scaffold. It helps local agents author marketplaces, manifests, conventions and all four component roles. Skill management never contacts an AI service or opens Work application state.

Choose exactly one scope:

```sh
work skills install --global
work skills install --dir /path/to/existing/marketplace
work skills status --dir /path/to/existing/marketplace --json
```

Both agents are selected by default. Repeat `--vendor claude` or `--vendor codex` to select agents explicitly. Omitted skill operands select all bundled skills. Unknown names, vendors and missing scope fail before writes. A catalog is optional for skill management. Install does not accept --json; status does. Bare `work skills` displays help.

| Agent | Personal destination | Folder destination |
| --- | --- | --- |
| Claude Code | `${CLAUDE_CONFIG_DIR:-HOME/.claude}/skills/work-plugin-builder` | `.claude/skills/work-plugin-builder` |
| Codex | `HOME/.agents/skills/work-plugin-builder` | `.agents/skills/work-plugin-builder` |

A relative CLAUDE_CONFIG_DIR resolves against the initial working directory. Empty means default. Explicit anchors are canonicalized; linked descendants and unsafe payload nodes are refused. Copies contain identical payload bytes. Invoke `/work-plugin-builder` in Claude Code or `$work-plugin-builder` in Codex. Claude watches existing directories; use `/reload-skills` if the top-level skills directory was created during the session, including after further edits there. Codex detects skills automatically; restart if a copy is missing. Actual override/reload behavior remains a release validation requirement.

The root `.work-skill.json` records the installed payload digest. Current copies are unchanged, including mtimes. Outdated owned copies update explicitly. Modified or foreign ordinary copies require --force, which replaces their local content; inspect or preserve edits before using it. Force never bypasses unsafe nodes, unreadable content, changed observations or unresolved recovery. Binary upgrades do not refresh copies silently.

Status reports absent, current, outdated, modified, foreign, conflict, recovery_required or unreadable. Ordinary states exit zero; operational states fail while retaining all inspected result rows. JSON contains ordered rows and nullable digest evidence; notes and hints go to stderr. Status creates no directories, locks or recovery artifacts and never repairs pending operations.

Install preflights the whole selection, takes persistent sibling locks in canonical order and retains exact old trees as backups until each destination commits. Earlier commits survive later failures. Receipts distinguish committed content, cleanup-pending, failed and not-attempted destinations. A brief directory rename gap is possible for external readers. Locks coordinate cooperating installers; this is not a guarantee against malicious external writers or arbitrary power loss.

Coordination siblings use `.work-skill-NAME.lock`, `.work-skill-NAME.transaction.json`, and operation-specific stage/backup names. Keep these local; only the skill payload directory is intended for Git publication. Work does not edit ignore files. Locks persist after release. Unknown lookalike artifacts are never wildcard-deleted. Re-run an explicit install to recover a valid interrupted operation or finish committed cleanup. If evidence is inconsistent, preserve it and inspect before moving it while no installer is active. Do not delete the only old copy. Automated removal is deferred.

Maintainers: validator and protocol descriptors generate committed references via `go generate ./ai-skills/...`. Review changes to guidance, scaffolds and fixtures, then run `make skill-lock` to update CONTRACT. Tests compare references and pin without writing. This detects represented contract drift; it does not prove prose or runtime semantics. Review parser differences, qualified private ownership, isolated every-entry validation and actual role behavior whenever contracts change. Keep changes in one PR.
