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
- Search the current workspace documentation before drafting, even when the gap has no linked article.
- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Identify the customer question, failed search, weak answer, stale article, or conflicting article.
- Distinguish one-off confusion from a durable documentation gap.

## Verify The Relevant Source

- For product or feature questions, inspect a linked repository when one is available and implementation is the relevant source of truth.
- Search the source for the feature, then read matching implementation and tests to verify entry points, behavior, permissions, limitations, and user-visible terminology.
- A successful repository search with no relevant match is valid evidence for `feature_not_found`; do not force an unrelated file read merely to satisfy a checklist.
- Repository inspection is not applicable when policy, process, data, or another authoritative source governs the answer.
- Treat PRDs, architecture notes, and task descriptions as leads, not proof that behavior shipped.
- If a required source is unavailable, do not invent product behavior. Record a blocked disposition, or create review-ready work that clearly identifies the verification gap.

## Decide The Doc Action

- Decide whether the gap needs a new article, an update to an existing article, or an information architecture change.
- Prefer updating an existing relevant article when the customer intent already has a home.
- Create a new article only when the topic is missing or the existing article would become unfocused.
- Link related articles when the answer spans multiple workflows.

## Completion Rules

- Keep run completion separate from gap resolution: a run may complete with review-ready work, a routed finding, or a blocked disposition while the gap remains open.
- Finish every support coverage gap run by calling `complete_support_coverage_gap` with the durable outcome.
- Use `resolved` only for a verified document fix. Use `review_ready` for a durable draft or proposal, `routed` for a completed non-documentation finding, and `blocked` when a required source is unavailable.
- Preserve the evidence trail so reviewers understand why the doc changed.
- For public help docs, draft or request approval before publishing.
