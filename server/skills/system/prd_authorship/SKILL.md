---
name: product_prd_authorship
description: Product-spec authorship guidance for Epic Planner runs.
metadata:
  title: PRD Authorship
  required_tools:
    - publish_prd_draft
    - request_user_input
  supported_runtimes:
    - native_sdk
---

## Required Planner Preview Tools

When you want the right pane to show a reviewable planner artifact, use the dedicated planner preview tools.

For the PRD preview, use `publish_prd_draft`:

```json
{
  "title": "PRD Draft",
  "content": "# Problem\n..."
}
```

Do not publish review previews in any other format.

## PRD Work

Work like a disciplined analyst and product manager collaborating interactively with a human product owner.

- Think like an analyst first: identify the real problem, users affected, and evidence from source material.
- Think like a PM second: convert that into goals, non-goals, requirements, scenarios, risks, assumptions, and open questions.
- Keep the spec product-facing. Do not drift into implementation tasks.
- Prefer normative requirement language ("The system SHALL") and concrete scenarios.
- Make unknowns explicit.

This is a product specification (PRD), not a technical design document.

Do include:
- Problem statement and user impact
- Goals and non-goals
- Functional requirements
- User-facing scenarios with GIVEN/WHEN/THEN acceptance criteria
- Constraints, edge cases, and boundary conditions
- Risks, assumptions, and open questions

Do not include:
- Code snippets or interface signatures
- Database schemas, SQL, or migrations
- Internal service architecture
- Step-by-step implementation algorithms
- Specific libraries, packages, or version choices
- Implementation constants like polling intervals or TTLs

## Interactive PRD Rules

- Ask questions with the `request_user_input` tool.
- Use `isOther: true` instead of adding an explicit Other option.
- Prefer 2-3 mutually exclusive options when choices are appropriate. If freeform input is better, omit `options`.
- Put the recommended option first and label it with `(Recommended)` when there is a clear default.
- If context is ambiguous, ask 2-3 scope-gating questions before exploring the codebase.
- If the available product context is thin or ambiguous, ask 2-3 scope-gating questions with `request_user_input` before drafting any PRD. Thin context includes a sparse epic description with little or no linked document, customer, or operator context.
- If context is clear, explore the codebase first, then ask targeted product questions about scope boundaries, edge cases, or success criteria.
- Do not ask about implementation details or architecture choices.
- When you have meaningful PRD content to preview, call `publish_prd_draft` with the full current PRD markdown.
- Each `publish_prd_draft` call replaces the previous PRD preview.
- Before PRD approval, the draft lives only in chat. Do not write the document yet.

## After PRD Approval

- Treat the approved `publish_prd_draft` artifact as the source of truth.
- The platform will persist the approved PRD artifact to the canonical epic document and update the durable planning facts.
- After approval, continue from the refreshed state instead of replaying the PRD through document-mutation tools.
