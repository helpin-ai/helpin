---
name: coding_task_planning
description: Creates task-level coding plans with implementation context, open questions, risks, and a reviewable task-plan document.
metadata:
  title: Coding Task Planning
  required_tools:
    - publish_task_plan_doc
    - request_user_input
    - ensure_task_plan_doc
    - write_document_content
  supported_runtimes:
    - native_sdk
    - codex
---

Treat the run as a transcript-driven loop. Decide the next step from the task, parent epic context, linked docs, comments, code context, tool results, and the current chat.

Use parent epic details, the epic PRD, and epic-linked docs as background context only. They explain why the task exists and what constraints it inherits, but they should not dominate or be copied wholesale into the task planning document unless they directly change implementation for this task.

Use `request_user_input` to ask focused scope-gating questions when scope, acceptance criteria, dependencies, or implementation constraints are missing or ambiguous.

Use tools directly, but keep repository interactions read-only. Inspect code and documents to ground the plan. Do not modify code, create files, apply patches, or change git state in this run.

Tool contract for `publish_task_plan_doc`:
- Always send a JSON object.
- `content` is required and must contain the full current markdown draft being reviewed.
- `title` is optional metadata only. Never call the tool with only `title` or with empty `content`.

Required preview shape:
```json
{
  "title": "Task Planning Document",
  "content": "# Outcome\n...\n\n## Acceptance Criteria\n..."
}
```

Use this sequence unless the human explicitly redirects you:
1. If critical scope or implementation details are ambiguous, ask focused questions with `request_user_input` before drafting.
2. Inspect the codebase, task comments, linked docs, parent epic, and the epic PRD.
3. Draft or refine the task planning document and publish the full current markdown draft with `publish_task_plan_doc`.
4. Wait for inline approval in chat.
5. After approval, call `ensure_task_plan_doc` with `{}`. This creates or loads the canonical task planning document and attaches it to the current task. Read the `document_id` from its JSON result.
6. Call `write_document_content` with that `document_id` and the full approved markdown draft as `content`.
7. Finish only after both product tool calls succeed. Do not claim the document was persisted or attached based on the approval alone.

If the human requests changes instead of approving, do not complete the run with prose-only acknowledgement. Revise the active planning document, republish the full replacement draft with `publish_task_plan_doc`, and request another approval request with `phase="task_doc"` when the revision is ready.

Produce a planning document, not code. The document should be implementation-ready and include concrete acceptance criteria, dependencies, implementation approach, risks, and open questions.

Ground the plan primarily in the task description, task comments, task-linked docs, and current codebase context. Use epic-level materials only to capture relevant constraints, non-goals, or dependencies. Keep the document focused on this task's implementation plan, not a restatement of the parent epic or PRD.
