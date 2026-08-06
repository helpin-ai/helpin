---
name: prd_task_plan_approval
description: Inline approval contract for PRD, task plan, and coding task planning artifacts.
metadata:
  title: PRD and Task Plan Approval
  required_tools:
    - request_approval
  supported_runtimes:
    - native_sdk
    - codex
    - opencode
---

Approval requests happen inline in the same chat.

- When the PRD is ready for approval, call `publish_prd_draft`, then call `request_approval` with `phase="prd"`, then stop.
- When the task plan is ready for approval, call `publish_task_plan`, then call `request_approval` with `phase="tasks"`, then stop.
- When the task planning document is ready for approval, call `publish_task_plan_doc`, then call `request_approval` with `phase="task_doc"`, then stop.
- An approval request must refer to a preview artifact published in that same turn. When there are multiple previews in the same turn, include `preview_panel_key` on the approval request so it binds to the correct preview.
- The human may approve or request changes with a normal chat reply. Do not tell them to use a separate approval state, button, or workflow.
- Treat `request_approval` as the last action in that turn. Do not call more tools after it in the same turn. Do not add "what would you like to do next" or restate approval options after requesting approval.
- After explicit PRD approval, call `ensure_epic_spec_doc` with `{}`, write the full approved markdown with `write_document_content`, then call `approve_epic_spec` with `{}`. Continue automatically into task planning only after all three calls succeed. Do not ask whether you should proceed to tasks unless the human asked to change scope.
- After PRD approval is persisted, your next turn must continue into task planning. Either ask the next blocking questions with `request_user_input` or publish the task plan preview. Do not complete the run immediately after PRD approval.
- After task-plan approval, call `create_task_batch` with the full approved `proposed_tasks` array. Preserve all approved task fields and `dependency_refs`; do not finish or claim task creation until the tool succeeds.
- After task planning document approval, call `ensure_task_plan_doc` with `{}`, then call `write_document_content` with the returned `document_id` and the full approved markdown. Do not finish or claim persistence until both calls succeed.

## Required Approval Tool

When you are ready for approval, call `request_approval` with this shape:

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

For `phase="task_doc"`, this means: revise the planning document markdown, publish the full replacement draft with `publish_task_plan_doc`, then call `request_approval` again with `phase="task_doc"` when the revision is ready.

Do not end the run with a prose-only acknowledgement after change feedback. The next step after change feedback is to revise the active artifact, ask blocking questions with `request_user_input`, or request another approval request when the revision is ready.

Only treat the phase as approved when the human gives a clear, explicit approval. If the human asks for changes, raises concerns, asks follow-up questions, or gives mixed or ambiguous feedback, treat that as not approved and revise instead of moving forward.
