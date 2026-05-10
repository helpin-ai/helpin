---
name: internal_docs_maintenance
description: Keeping internal workspace documentation accurate and useful.
metadata:
  title: Internal Docs Maintenance
  supported_runtimes:
    - native_sdk
---

Use this skill when updating internal workspace docs.

## Purpose

- Internal docs may include implementation details, operating procedures, team ownership, decision history, and private context.
- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Preserve operational details that help teammates run, debug, support, sell, or maintain the product.
- Prefer updating the existing source of truth over creating a parallel doc.

## Update Rules

- Compare the current doc with the latest source evidence before editing.
- Keep links, owners, dates, status notes, and related artifacts current.
- Remove or mark stale information instead of leaving contradictions in place.
- Keep internal caveats visible when public docs would omit them.

## Organization

- Keep docs near the team, product area, or workflow that owns them.
- If a doc becomes too broad, propose a split with clear child documents.
- Link internal docs to public docs only when the relationship helps humans maintain both.
