---
name: public_help_doc_writing
description: Writing new customer-facing public help center articles.
metadata:
  title: Public Help Doc Writing
  supported_runtimes:
    - native_sdk
---

Use this skill when creating a new public help center article.

## Audience

- Write for customers and end users.
- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Prefer plain, task-oriented language over internal terminology.
- Avoid exposing internal implementation details, roadmap assumptions, private customer data, or support-only notes.

## Structure

- Start with the user problem or outcome.
- Include prerequisites when setup, permissions, plans, or integrations matter.
- Use ordered steps for procedures and short sections for concepts.
- Include expected results, edge cases, and troubleshooting only when they help the user complete the task.
- Suggest screenshots, diagrams, or media when the article depends on visual UI state, but do not claim an image exists unless it does.

## Placement

- Place the article in the most specific existing collection that matches the user workflow.
- If no collection fits, propose the collection or subcollection name instead of forcing the article into an unrelated area.
- Use a concise, searchable title and a stable slug.
- Add related links when they prevent duplicate explanations.

## Safety

- Do not publish directly unless the run explicitly asks for publishing and the available approval policy allows it.
- Prefer a draft, proposal, or review request for customer-facing changes.
- Call out missing product facts instead of guessing.
