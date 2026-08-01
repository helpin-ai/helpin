---
name: coding_task_planning
description: Creates task-level coding plans with implementation context, open questions, risks, and a reviewable task-plan document.
metadata:
  title: Coding Task Planning
  required_tools:
    - publish_task_plan_doc
    - request_user_input
    - update_plan
  supported_runtimes:
    - native_sdk
---

Treat the run as a transcript-driven loop. Decide the next step from the task, parent epic context, linked docs, comments, code context, tool results, and the current chat.

Use parent epic details, the epic PRD, and epic-linked docs as background context only. They explain why the task exists and what constraints it inherits, but they should not dominate or be copied wholesale into the task planning document unless they directly change implementation for this task.

Use `request_user_input` to ask focused scope-gating questions when scope, acceptance criteria, dependencies, or implementation constraints are missing or ambiguous.

Use `update_plan` for the repository-inspection and drafting workflow. Publish a concise plan once the scope is sufficiently clear, keep its step statuses current as inspection and drafting progress, and finish the plan before requesting approval. The visible execution plan is separate from the task planning document.

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
5. After approval, stop. The platform will persist and link the approved preview to the canonical task planning document automatically.

If the human requests changes instead of approving, do not complete the run with prose-only acknowledgement. Revise the active planning document, republish the full replacement draft with `publish_task_plan_doc`, and request another approval request with `phase="task_doc"` when the revision is ready.

Produce a planning document, not code. The document should be implementation-ready and include concrete acceptance criteria, dependencies, implementation approach, risks, and open questions.

Ground the plan primarily in the task description, task comments, task-linked docs, and current codebase context. Use epic-level materials only to capture relevant constraints, non-goals, or dependencies. Keep the document focused on this task's implementation plan, not a restatement of the parent epic or PRD.
