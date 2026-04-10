# PM Task List Team-Scoped Pickers Design

## Summary

Filter the inline pickers in `TaskListView` (Sprint, Epic, Owner, Team) so they only show values that are valid for the row's team.

Today, when you open an epic that belongs to one team and edit a task inline in the task table, the Sprint cell shows sprints from every team in the workspace. Epic and Owner cells have the same problem. This leads to tasks being assigned to sprints or epics that belong to the wrong team, which is silently wrong — the chosen sprint is never shown on that team's sprint board because the backend still treats the task as belonging to its original team.

Tasks are always team-scoped in Helpin: every task has exactly one `team_id`. That makes this a pure frontend fix — the options shown in inline pickers just need to match the row's team.

## Goals

- Inline Sprint, Epic, and Owner cells in `TaskListView` only show values valid for `task.team_id`.
- The Team cell remains changeable, but changing it triggers a confirm dialog if the current Sprint, Epic, or Owner would become invalid; on confirm, those fields are cleared in the same update.
- Currently-selected values in a picker remain visible even if they are now invalid for the task's team, so legacy data stays editable.
- `EpicDetail` passes `teamId={epic.team_id}` to `TaskListView` so the list view applies team-scoped field visibility and hides the (now redundant) Team column, matching how other team-scoped list contexts work today.

## Non-Goals

- No backend changes. `pm_sprints`, `pm_epics`, `pm_tasks` schemas are unchanged.
- No change to `TaskDetailPanel` or `CreateTaskModal` — they already filter by the task's team via the helpers in `taskPlanningScope.ts`.
- No change to the Priority, Severity, Estimate, State, or CRM association cells — they are either global or workflow-scoped, not team-scoped.
- No migration or cleanup of legacy rows where a task already points to a sprint or epic in another team. Those rows stay editable and fixable by hand; we do not auto-repair them.
- No change to how tasks are created under an epic (they already inherit the epic's team).

## Current Behavior

- `frontend/src/pages/pm/EpicDetail.tsx` loads every non-archived sprint, epic, and task for the workspace and passes the full lists to `TaskListView` as props.
- `frontend/src/components/pm/TaskListView.tsx` renders one inline popover cell per editable column:
  - `InlineSprintCell` maps over the raw `sprints` prop — no team filter.
  - `InlineEpicCell` maps over the raw `epics` prop — no team filter.
  - `InlineOwnerCell` maps over the raw `assignableMembers` prop — no team filter.
  - `InlineTeamCell` maps over every workspace team — no downstream cleanup when the team is changed.
- `frontend/src/components/pm/TaskDetailPanel.tsx` (the side panel that opens when a row is clicked) already filters sprints and epics correctly using the helpers in `frontend/src/components/pm/task-detail/taskPlanningScope.ts`:
  - `isSprintSelectableForTaskTeam(sprintTeamId, taskTeamId)`
  - `isEpicSelectableForTaskTeam(epicTeamId, taskTeamId)`
  - It also auto-clears `sprint_id` when the task's team changes and the current sprint is no longer valid.
- `EpicDetail.tsx` does not pass a `teamId` prop to `TaskListView`, so the task table inside an epic shows the Team column and uses workspace-wide field visibility instead of the team's settings.

## Product Decision

Inside any task list row, the options shown in Sprint, Epic, and Owner pickers are filtered to `task.team_id`. The Team cell is still editable, but changing it is a deliberate action that cascades through the dependent fields via a confirm dialog.

The rules are the same everywhere (detail panel, list view, create modal): one set of helpers in `taskPlanningScope.ts` is the source of truth.

## Approaches Considered

### 1. Recommended: Filter each inline cell by `task.team_id` using the existing helpers

Wire `InlineSprintCell`, `InlineEpicCell`, and `InlineOwnerCell` into the existing `taskPlanningScope.ts` helpers. The cells already receive the full option lists via props — we just filter inside the cell using `task.team_id`.

Why this is preferred:

- Reuses the helpers already trusted by `TaskDetailPanel` and `CreateTaskModal`. One rule, one place.
- No new data loads, no new props up the tree. `EpicDetail` already fetches the full lists.
- Works uniformly in every context `TaskListView` is used (epic detail, sprint detail, global boards).
- Handles the legacy-data edge case cleanly: always include the currently-selected value in the rendered list so it stays visible and removable.

Trade-offs:

- `InlineOwnerCell` needs access to "who is on this team" rather than "who is assignable in this workspace". We add a lightweight team-member lookup (see Implementation).

### 2. Scope the entire `TaskListView` by a single `scopeTeamId` prop

Pass a single team ID into `TaskListView` and filter every cell by it, regardless of individual task teams.

Rejected because:

- Tasks are team-scoped in Helpin: the epic's team and each task's team are the same by definition. Introducing a second source of truth adds complexity for zero benefit.
- Diverges from how `TaskDetailPanel` filters today (by task team), so we end up with two rules.

### 3. Move the filtering into the parent (`EpicDetail`) before passing props

Filter the `sprints`, `epics`, and `assignableMembers` arrays in `EpicDetail` before passing them to `TaskListView`.

Rejected because:

- Only works for the epic context. Sprint detail, global boards, and any other future caller would have to duplicate the logic.
- Can't handle per-row filtering — every row in the list would see the same pre-filtered set, which breaks if any row has a different team (which shouldn't happen, but shouldn't crash if it does).

## Implementation

### Shared helpers

`frontend/src/components/pm/task-detail/taskPlanningScope.ts` already exports the two predicates we need. No new helpers are required for sprints and epics.

Add one small helper for the owner filter and a team-change impact check:

```ts
// Returns true if the member is assignable to a task in the given team.
export function isMemberAssignableForTaskTeam(
  member: AssignableMember,
  taskTeamId: string | null | undefined,
  teamMemberIds: Set<string>,
): boolean {
  if (!taskTeamId) return true;
  return teamMemberIds.has(member.id);
}

// Describes which of the task's current assignments become invalid if the team changes.
export interface TeamChangeImpact {
  clearsSprint: boolean;
  clearsEpic: boolean;
  clearsOwner: boolean;
}

export function computeTeamChangeImpact(params: {
  task: Task;
  nextTeamId: string | null;
  sprints: SprintWithStats[];
  epics: EpicWithStats[];
  nextTeamMemberIds: Set<string>;
}): TeamChangeImpact {
  // ... returns which dependent fields would become invalid.
}
```

Add matching unit tests in `frontend/src/components/pm/task-detail/__tests__/taskPlanningScope.test.ts`.

### `InlineSprintCell`

- Filter the popover list to sprints where `isSprintSelectableForTaskTeam(sp.sprint.team_id, task.team_id)` is true.
- If `task.sprint_id` is set and the current sprint is not in the filtered list, prepend it to the rendered list so the user can still see and remove it.
- Empty state text: "No sprints for this team".

### `InlineEpicCell`

- Filter the popover list to epics where `isEpicSelectableForTaskTeam(e.epic.team_id, task.team_id)` is true.
- Always include the current `task.epic_id` even if filtered out.
- Empty state text: "No epics for this team".

### `InlineOwnerCell`

- Needs a team-members lookup. The cleanest path is to pass the already-existing `getTeamMembers(teamId)` function (from `useAccessibleTeams`) down to the cell as a prop from `TaskListView`.
- Cell uses `getTeamMembers(task.team_id)` to build a `Set<string>` of allowed member IDs, then filters `assignableMembers` by `isMemberAssignableForTaskTeam`.
- Always include the currently-assigned owner even if they're no longer on the team (legacy data, membership revocation).
- Empty state text: "No members on this team".

### `InlineTeamCell`

- Keeps showing all teams.
- On selection, compute `TeamChangeImpact` using the next team.
- If any of `clearsSprint | clearsEpic | clearsOwner` is true, open a confirm dialog before the mutation. The dialog lists which fields will be cleared and names them, e.g.:

  > Moving this task to **Engineering** will clear:
  > - Sprint: *Sprint 14 — Marketing*
  > - Owner: *Maria Gomez*
  >
  > [Cancel] [Move and clear]

- On confirm, send a single `updateTaskField` call with `{ team_id, sprint_id: null, epic_id: null, owner_member_id: null }` as needed.
- On cancel, leave everything untouched.
- Reuse the existing `ConfirmDialog` component from `frontend/src/components/pm/ConfirmDialog.tsx` to stay consistent.

### `TaskListView` prop wiring

- `TaskListView` gains an optional `getTeamMembers?: (teamId: string | null) => AssignableMember[]` prop. It threads this prop into `InlineOwnerCell` and `InlineTeamCell` (the latter uses it to compute the impact).
- All other prop signatures stay the same.

### `EpicDetail` one-line fix

- `EpicDetail.tsx` currently renders:
  ```tsx
  <TaskListView
    workspaceId={workspaceId!}
    workflow={workflow}
    workflows={workflows}
    teams={teams}
    assignableMembers={assignableMembers}
    epics={allEpics}
    sprints={allSprints}
    externalTasks={tasks}
    onOpenTask={openTask}
  />
  ```
- Add `teamId={epic.epic.team_id ?? null}` and `getTeamMembers={getTeamMembers}`. `getTeamMembers` is already imported from `useAccessibleTeams` in this file.
- Effects of `teamId`:
  - `TaskListView` already hides the Team column when `teamId` is set.
  - It also runs the team's field visibility through `useTeamFieldVisibilityForTeam(workspaceId, teamId)`, so (for example) sprint/estimate columns won't appear in a team that has them turned off.

## Legacy Data Handling

A task inside an epic could in theory point to a sprint that belongs to a different team (e.g., left over from an earlier bug). We don't auto-repair these; we:

- Always render the currently-selected value in the picker, even if it doesn't match the team filter, so it's visible and removable.
- Make it obvious that the value is the "current" one (it's at the top of the list with the checkmark as today).
- On the next edit, the user can clear it and select a valid one; the next save will make the row consistent.

No explicit "warning" visual — this is a rare legacy case and a sticky warning would add noise for the common path.

## Testing

**Unit tests** — `frontend/src/components/pm/task-detail/__tests__/taskPlanningScope.test.ts`

- `isMemberAssignableForTaskTeam` — in-team, out-of-team, null task team.
- `computeTeamChangeImpact`:
  - Clears sprint when old sprint is not in new team's sprints.
  - Clears epic when old epic is not in new team's epics.
  - Clears owner when old owner is not in new team's members.
  - Returns all-false when nothing would change.

**Component tests** — new files under `frontend/src/components/pm/__tests__/`

- `InlineSprintCell.test.tsx`:
  - Shows only sprints that match `task.team_id`.
  - Shows a currently-selected but off-team sprint pinned at the top.
  - Empty state when the team has no matching sprints.
  - Selecting a sprint calls `onUpdate` with the sprint ID.
- Mirror tests for `InlineEpicCell` and `InlineOwnerCell`.
- `InlineTeamCell.test.tsx`:
  - No confirm dialog when the team change has no impact.
  - Confirm dialog lists each field that would be cleared.
  - On confirm, the update includes the cleared fields.
  - On cancel, no update is sent.

**Manual verification**

1. Create an Engineering epic, add a task under it, open the inline Sprint cell — only Engineering sprints show.
2. Create a Marketing sprint, then try to assign it inline from the same task — it should not be in the list.
3. Open the same task's detail panel — same filtering (already works).
4. Change the task's team to Marketing — confirm dialog says the Engineering sprint will be cleared. Confirm. Sprint is cleared and team is updated in one operation.
5. On a task that has a legacy mismatched sprint, open the inline picker — the current sprint is still visible at the top and can be removed.

## Rollout

- Pure frontend change. Ships with the next frontend deploy.
- No feature flag: the new behavior is strictly narrower than the old (fewer/more correct options), so there is no safe "off" state worth preserving.
- No migration.

## Risks and Mitigations

- **Risk:** Filtering breaks a team that has no sprints at all, leaving users unable to pick anything.
  - *Mitigation:* This is the intended state — the picker empty state tells them to create a sprint in that team first. Matches existing `TaskDetailPanel` behavior.
- **Risk:** `getTeamMembers` returns stale data if team memberships change mid-session.
  - *Mitigation:* `useAccessibleTeams` already refreshes on the same signals used elsewhere. Not a new problem.
- **Risk:** Confirm dialog adds friction when the user changes a task's team often.
  - *Mitigation:* Only appears when the change would actually clear a field — silent when there's nothing to clear.
