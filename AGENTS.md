<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
at specs/010-agent-skills/plan.md
<!-- SPECKIT END -->

When marketplace, plugin, semantic-convention or IPC contracts change, review and update ai-skills/work-plugin-builder references and templates in the same PR. Run go generate ./ai-skills/... and review the generated differences, then explicitly run make skill-lock. Verification must never regenerate or re-pin in CI. Descriptors do not prove prose correctness; review unrepresented behavior and fixtures.
