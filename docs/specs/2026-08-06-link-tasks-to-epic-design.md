# Link Existing Tasks to an Epic

## Goal

Let an editor link multiple existing tasks to a team-scoped epic without leaving the epic detail page. Tasks already in another epic may be moved only after an explicit review step.

## Product behavior

- Add **Link tasks** beside **Create task** in the epic Tasks heading.
- In an empty task section, expose separate **Link tasks** and **Create task** actions instead of the ambiguous **Add task** action.
- Clicking **Link tasks** opens a searchable, paginated multi-select dialog.
- The epic must have a team. A legacy epic without a team cannot open the picker and explains that a team must be assigned first.
- Candidate tasks must be accessible, non-archived, and belong to the exact same team as the epic.
- Tasks already in the current epic are excluded.
- Every row shows task key, title, state, Team, and its current epic when present.
- Candidate tasks are grouped into **No epic** and **In another epic**.
- Selecting only unassigned tasks submits with **Link N tasks**.
- If any selected task belongs to another epic, the first action is **Review changes**. A confirmation view summarizes newly linked tasks and tasks moving from each prior epic. The final action is **Move and link N tasks**.
- Linking changes only `epic_id`. State, sprint, owners, labels, estimate, dates, and team remain unchanged.
- Explicit manual delivery targets remain unchanged. Delivery targets inherited from another epic follow the existing task-update behavior and switch to the new epic.
- The mutation is atomic: all selected tasks are linked or none are.
- Success closes the dialog, refreshes the epic task list, and shows a toast. Validation or mutation errors keep the dialog open and preserve the selection.

## Architecture

- Extend the existing PM task list filter to accept its already-supported repository `Search` field from the HTTP `search` query parameter.
- Add an epic-owned batch endpoint, `POST /api/pm/epics/{id}/tasks/link`, accepting a bounded deduplicated `task_ids` array.
- Validate epic edit access, a non-empty epic team, task existence, workspace, archive state, team equality, and current actor team access before mutating.
- Update every task inside one repository transaction, then run existing post-commit activity, websocket, and delivery-target synchronization behavior per changed task.
- Add a focused `LinkTasksToEpicDialog` frontend component and keep `EpicDetail` responsible only for opening it and refreshing the task section.

## Frontend direction

- **Visual thesis:** a calm, dense task-selection surface that reads like the existing PM tables, with team as a quiet metadata field and moves as the only warning color.
- **Content plan:** epic/team context, search, grouped task rows, persistent selection summary, then a compact review state when moves are involved.
- **Interaction thesis:** immediate checkbox feedback, deferred server search without layout shifts, and one deliberate transition from selection to move review.

## Safety and compatibility

- No cross-team or unassigned task may be linked to a team epic, even if a client bypasses the picker filter.
- No implicit team reassignment occurs.
- A task can have only one epic; moving replaces its previous `epic_id`.
- Repeated task IDs and tasks already in the target epic are harmless and do not duplicate work.
- The batch size is capped at 100.
- Existing single-task epic updates remain unchanged.

## Verification

- Service tests cover successful same-team linking and moving, atomic rejection for mixed teams/missing tasks, archived-task rejection, access enforcement, and legacy epics without teams.
- Handler/service contract tests cover bounded request validation.
- Frontend tests cover team metadata, grouping, selection, review-before-move, successful refresh, and legacy no-team guarding.
- Run focused Go and Vitest suites, full Go tests, frontend production build, and diff hygiene.
