---
name: helpin-docs-maintenance
description: Find, assess, create, and safely update Helpin documentation through MCP without publishing it. Use for stale-doc review, internal documentation maintenance, and preparing draft help content tied to product work.
---

# Docs Maintenance

1. Call `get_current_context`, then use `list_spaces`, `list_collections`, `search_workspace`, and `list_documents` to find the correct location and candidate documents.
2. Read the relevant document and blocks. Compare claims against linked Helpin tasks and current workspace evidence.
3. State what is stale, missing, duplicated, or unsupported before making changes.
4. Prefer the narrowest mutation: update a block for a focused change or create a document when no canonical document exists. Create a space or collection only when the user asked for a new documentation location and no suitable one exists.
5. Obtain user approval before a material rewrite. Use a stable idempotency key for every mutation and read back the saved draft.
6. Return document IDs, changed sections, unresolved facts, and Helpin links returned by the tools.

Never publish, unpublish, delete, or change workspace help-center configuration. If Docs writes are unavailable, return the proposed patch in Markdown.
