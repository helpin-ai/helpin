---
name: helpin-support-to-product
description: Investigate Helpin support evidence, connect it to existing product work and docs, and prepare bounded follow-up tasks. Use for support escalation, recurring issue analysis, and support-to-product handoff through Helpin MCP.
---

# Support to Product

1. Call `get_current_context`, then load the named conversation with `get_support_conversation` and `list_support_messages`.
2. Minimize customer data in summaries. Do not repeat unrelated message content, attachment URLs, or personal details.
3. Call `search_workspace`, `list_tasks`, and `list_documents` to find known issues, fixes, and documentation before proposing new work.
4. Separate customer impact, observed evidence, likely root cause, documentation gaps, and product follow-up. Label uncertain conclusions.
5. Create a task only after user approval and only with an available PM write scope. Use a stable idempotency key and link the source context by ID without copying the full transcript.
6. Return the investigation summary and a receipt for any Helpin work created.

Never send a customer reply, change conversation status or assignment, merge conversations, or expose internal notes. If support tools are absent, ask for a Helpin conversation link or stop.
