---
name: helpin-release-readiness
description: Assess release readiness from Helpin tasks, documents, repositories, and agent context, then prepare bounded follow-up work. Use for release reviews, launch checklists, risk summaries, and release-note preparation through Helpin MCP.
---

# Release Readiness

1. Call `get_current_context`, then discover connected repositories with `list_repositories`.
2. Search the release name, tag, milestone, or date with `search_workspace`. Load relevant tasks and documents; do not infer completion from titles alone.
3. Classify shipped work, open blockers, migration or rollout risk, missing documentation, and ownership gaps. Separate evidence from inference.
4. Use `list_agents` when a Helpin review or release agent could perform durable work. Start one only after the user confirms the agent and target.
5. Create or update Helpin follow-up only when the user requests it and the needed write tool is available. Use stable idempotency keys.
6. Return a concise readiness verdict, blocker list, and receipt of any records or runs created.

Do not publish release notes, deploy code, change integrations, or mark work complete without evidence. If repository or write tools are absent, complete the evidence review and state the limitation.
