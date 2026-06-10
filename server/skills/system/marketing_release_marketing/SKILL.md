---
name: marketing_release_marketing
description: Turns GitHub and release context into customer-facing launch narratives, announcement drafts, changelog copy, sales notes, and follow-up tasks.
metadata:
  title: Marketing Release Marketing
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira creates marketing assets from shipped work, GitHub/release context, task history, or feature-release notes.

Adapted from the MIT-licensed `launch`, `copywriting`, `sales-enablement`, and `product-marketing` skills in `coreyhaines31/marketingskills`.

## Inputs

Use:

- `get_release_context` when available.
- `find_tasks_for_git_changes` when the run is tied to repository changes or PRs.
- PM tasks, docs, release notes, CRM/customer context, and explicit user input.

## Translation Rules

- Translate implementation detail into customer-visible outcomes.
- Separate shipped behavior from internal work.
- Identify who benefits and why it matters.
- Keep claims tied to evidence.
- Call out missing product details instead of inventing them.

## Output

Create one or more:

- Launch plan.
- Changelog entry.
- Release announcement.
- Email/social copy.
- Sales enablement notes.
- Help doc update suggestions.
- Customer segment follow-up tasks.

## Guardrails

- Do not publish, email, post, merge code, or mutate GitHub.
- Do not expose sensitive internal implementation details.
- Ask for review before customer-facing release messaging.
