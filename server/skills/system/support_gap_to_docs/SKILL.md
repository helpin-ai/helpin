---
name: support_gap_docs_update
description: Converting support coverage gaps into missing, weak, or stale documentation work.
metadata:
  title: Support Gap Docs Update
  supported_runtimes:
    - native_sdk
    - codex
---

Use this skill when working from a support coverage gap, repeated customer question, weak article signal, missing article signal, or stale article signal.

## Evidence First

- Read the gap evidence before deciding what to write.
- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Identify the customer question, failed search, weak answer, stale article, or conflicting article.
- Distinguish one-off confusion from a durable documentation gap.

## Decide The Doc Action

- Decide whether the gap needs a new article, an update to an existing article, or an information architecture change.
- Prefer updating an existing relevant article when the customer intent already has a home.
- Create a new article only when the topic is missing or the existing article would become unfocused.
- Link related articles when the answer spans multiple workflows.

## Completion Rules

- Do not close or mark a gap resolved until the doc work is actually created, updated, or explicitly handed off.
- Finish every support coverage gap run by calling `complete_support_coverage_gap` with the durable outcome.
- Use `resolved` only after the document was actually created or updated. Use `proposal_submitted` for a persisted review proposal, or `handoff` when a human must take over.
- Preserve the evidence trail so reviewers understand why the doc changed.
- For public help docs, draft or request approval before publishing.
