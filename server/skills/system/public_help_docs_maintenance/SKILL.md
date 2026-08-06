---
name: public_help_docs_maintenance
description: Keeping existing public help center docs accurate and safe.
metadata:
  title: Public Help Docs Maintenance
  supported_runtimes:
    - native_sdk
    - codex
---

Use this skill when updating existing public help center articles.

## Public Contract

- Avoid exposing internal implementation details, private customer data, support-only notes, or unannounced roadmap items.
- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Preserve stable public URLs and slugs unless a redirect plan exists.
- Do not silently publish customer-facing changes.

## Update Workflow

- Identify what changed and whether the existing article still answers the customer problem.
- Keep the article focused on the user workflow rather than the internal feature name.
- Update titles, headings, related links, prerequisites, limitations, and troubleshooting when the product behavior changes.
- Remove outdated promises, screenshot references, or instructions that no longer match the product.

## Review

- Prefer a draft, proposal, or approval request before publishing.
- Call out public-facing risks, unresolved product facts, and redirect needs.
