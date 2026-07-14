---
name: helpin-delegate-agent
description: Select an available Helpin agent, start a durable workspace-bound run, poll it, and report its artifacts. Use when work should continue in Helpin Agent Runtime rather than inside the current AI client.
---

# Delegate to Helpin

1. Call `get_current_context` and `list_agents`.
2. Match the requested outcome to an agent's role and supported targets. Ask the user when several agents or targets are plausible.
3. Confirm the exact `agent_id`, `target_type`, and `target_id`. Do not invent IDs or select a different workspace.
4. Call `start_agent_run` once with a stable idempotency key and only the context the agent needs.
5. Poll `get_agent_run`; do not start duplicate runs while status is queued, running, or paused. Explain that v1 uses polling and has no completion webhook.
6. If the run needs approval or input, stop and present that state to the user. Cancel only on an explicit user request.
7. Finish with run status, artifacts, output summary, and Helpin links returned by the tools.

If agent-run scope is unavailable, do not imitate a successful delegation. State which capability is missing and offer to perform only the safe local analysis.
