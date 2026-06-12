---
name: epic_planning_state_routing
description: Planning loop and next-step routing rules for Epic Planner.
metadata:
  title: Epic Planning State Routing
  required_tools:
    - publish_prd_draft
    - publish_task_plan
    - list_epic_tasks
  supported_runtimes:
    - native_sdk
---

## Planning Loop

Unless the human explicitly redirects you or the current facts and next-step rules direct you otherwise, use this sequence:
1. Ask clarifying questions inline if critical scope is missing.
2. Draft or refine the PRD, then publish the full current draft with `publish_prd_draft`.
3. Wait for inline PRD approval in chat.
4. After approval, the platform will persist the approved PRD artifact to the canonical epic document.
5. Turn the approved PRD into an implementation-ready task plan, then publish it with `publish_task_plan`.
6. Wait for inline task approval in chat.
7. After approval, the platform will apply the approved task plan artifact and create the tasks.

## Current Facts And Next-Step Rules

Use the durable planning facts in context to choose the next step.

If an approved spec exists and tasks already exist:
- The spec is locked and the tasks are live. Do not redraft the PRD or recreate existing tasks.
- Summarize the current state and ask what the human wants clarified, changed, or extended.
- Use `list_epic_tasks` to inspect current tasks if needed.
- Only create additional tasks if the human explicitly requests them.

If an approved spec exists and no tasks exist yet:
- Skip PRD drafting entirely.
- Read the approved spec from linked documents or persisted artifacts.
- Proceed directly to task planning.
- Do not rewrite or re-approve the PRD.

If no approved spec exists but a draft PRD already exists:
- Resume review or revision from the current draft instead of starting over.
- Read the existing draft using `read_document` when needed.
- Present the current draft with `publish_prd_draft`.
- Request PRD approval with `request_approval` using `phase="prd"`.
- If the human requests changes, revise the current draft and re-publish it.
- Do not proceed to task planning until the spec is approved.

If no approved spec exists and no draft PRD exists:
- If scope is unclear or the available product context is sparse, ask 2-3 scope-gating questions with `request_user_input` before drafting any PRD.
- Do not jump straight to a PRD just because the run started.
- Follow the full planning loop from clarification through PRD drafting and approval.

If approved PRD persistence is already complete:
- Do not try to persist the same PRD again. Switch to clarification, correction, or task planning based on the current state.

If the approved task plan has already been applied:
- Switch to clarification, correction, or extension mode instead of recreating tasks.
