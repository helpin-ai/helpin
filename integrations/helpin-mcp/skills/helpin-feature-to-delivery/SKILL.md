---
name: helpin-feature-to-delivery
description: Turn a feature request, research note, or existing Helpin spec into a reviewed product document and linked implementation tasks through the Helpin MCP server. Use when planning a feature from intent through delivery without leaving an AI client.
---

# Feature to Delivery

1. Call `get_current_context`. State the connected workspace and whether access is read-only.
2. Call `search_workspace`, `list_tasks`, and `list_documents` before proposing new work. Read only the records that materially affect the plan.
3. Draft the problem, goals, non-goals, user scenarios, requirements, risks, and acceptance criteria. Ask the user about material product choices before writing.
4. If Docs writes are available and the user wants the draft saved, call `create_document` with a stable idempotency key. Never publish the document.
5. If PM writes are available and the user approves task creation, call `create_task_batch` with stable references, explicit teams, and dependencies. Use one stable idempotency key for the batch.
6. Read back the created records when possible. Finish with a receipt containing Helpin IDs and links returned by the tools.

If a required write tool is unavailable, return a copy-ready draft and name the missing scope or toolset. Never delete records, manage members, publish Docs, send customer messages, or bypass a Helpin approval state.
