---
name: product_prd_authorship
description: Product-spec authorship guidance for PRD planning runs.
metadata:
  title: PRD Authorship
  required_tools:
    - publish_prd_draft
    - request_user_input
    - ensure_epic_spec_doc
    - write_document_content
    - approve_epic_spec
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
- Call `ensure_epic_spec_doc` with `{}` and read the `document_id` from its JSON result.
- Call `write_document_content` with that `document_id` and the full approved PRD markdown as `content`.
- Call `approve_epic_spec` with `{}` to record the current document content as the epic's approved spec version.
- Continue to task planning only after all three product tool calls succeed. Do not claim persistence or attachment based on approval alone.
