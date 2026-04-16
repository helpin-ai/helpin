---
name: approval_protocol
description: Inline approval and review-checkpoint contract for planner-style runs.
metadata:
  title: Approval Protocol
  required_tools:
    - request_review_checkpoint
  supported_runtimes:
    - native_sdk
    - codex
    - opencode
---

Approval checkpoints happen inline in the same chat.

- When the PRD is ready for review, call `publish_prd_draft`, then call `request_review_checkpoint` with `phase="prd"`, then stop.
- When the task plan is ready for review, call `publish_task_plan`, then call `request_review_checkpoint` with `phase="tasks"`, then stop.
- When the task planning document is ready for review, call `publish_task_plan_doc`, then call `request_review_checkpoint` with `phase="task_doc"`, then stop.
- The human may approve or request changes with a normal chat reply. Do not tell them to use a separate approval state, button, or workflow.
- Treat `request_review_checkpoint` as the last action in that turn. Do not call more tools after it in the same turn. Do not add "what would you like to do next" or restate approval options after requesting the checkpoint.
- After explicit PRD approval, continue automatically into task planning in the same run. Do not ask whether you should proceed to tasks unless the human asked to change scope.
- After PRD approval is persisted, your next turn must continue into task planning. Either ask the next blocking questions with `request_user_input` or publish the task plan preview. Do not complete the run immediately after PRD approval.

## Required Approval Tool

When you are ready for approval, call `request_review_checkpoint` with this shape:

```json
{
  "phase": "prd|tasks|task_doc",
  "title": "...",
  "summary": "..."
}
```

Do not ask for approval in any other format.

## Request Changes

When the human requests changes, incorporate that feedback into the current phase output, revise it, republish the full replacement preview for that phase, and emit a fresh approval block again when ready.

For `phase="task_doc"`, this means: revise the planning document markdown, publish the full replacement draft with `publish_task_plan_doc`, then call `request_review_checkpoint` again with `phase="task_doc"` when the revision is ready.

Do not end the run with a prose-only acknowledgement after change feedback. The next step after change feedback is to revise the active artifact, ask blocking questions with `request_user_input`, or request another review checkpoint when the revision is ready.

Only treat the phase as approved when the human gives a clear, explicit approval. If the human asks for changes, raises concerns, asks follow-up questions, or gives mixed or ambiguous feedback, treat that as not approved and revise instead of moving forward.
