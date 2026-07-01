# Bulk Edit: Fix "Multiple" fields + add Team field (Phase 1 + 2)

Status: IMPLEMENTED (2026-06-30) — Phase 1 + 2 complete, 23 tests passing, tsc clean.
Date: 2026-06-30
Branch: waqar-fixes
Source: team feedback doc (share/589d7282b64161def57cb5087f4890d9), issues #1 & #2

## Problem

In the PM bulk-edit popover (`TaskBulkActionsBar.tsx`), when selected tasks have
**mixed** values for a field (Status / Priority / Severity / Epic / Sprint), the
field shows an italic "Multiple" label and **clicking it does nothing** — the
dropdown never opens. The team also reports there is **no Team field** in bulk edit.

## Root cause (verified)

- `ui/select.tsx` defaults `SelectContent` to `position="item-aligned"`.
- For mixed fields the trigger renders a plain `<span>Multiple</span>` **instead of
  `<SelectValue>`** (`TaskBulkActionsBar.tsx:466,547,574,604,635`).
- Radix item-aligned positioning (`@radix-ui/react-select`) runs `position()` only
  when `context.valueNode` exists (`index.js:589`); `valueNode` is registered **only
  by `<SelectValue>`**. Without it, `position()` bails → `onPlaced()` never fires →
  `isPositioned` stays `false` → the menu is never placed/shown.
- Affects every mixed field, only when values differ — matches the symptom.

Note: the per-task override logic already works (`onValueChange → setStaged… →
Apply` patches all selected tasks). So issue #1 is purely this rendering bug.

## Field scoping (verified in server/internal/model)

- Team-scoped: **Sprint** (`pm_sprint.go` TeamID), **Epic** (`pm_epic.go` TeamID),
  **Status/workflow** (`pm_workflow.go` TeamID), **Labels** (team_id + shared).
- Workspace-scoped (NOT team-scoped): **Priority, Severity, Deadline, Owners**
  (`ListAssignableMembers(ctx, workspaceID)` — no team filter; no owner/team
  validation on task Update).

## Decisions (locked with user)

1. Owners stay **workspace-scoped** for now. Bulk team-change does **not** touch owners.
   (Possible future: soft-scope owner picker to team members — separate task.)
2. Build **Phase 1 + 2** now.
3. Team selector goes at the **top** of the modal (above Status).
4. On team change, reset only team-scoped staged fields (Sprint/Status/Epic/Labels);
   leave Owners/Priority/Severity/Deadline.

## Implementation

### Phase 1 — fix the bug + add Team field

1. **Fix mixed dropdowns** (`TaskBulkActionsBar.tsx`): replace the
   `multiplePlaceholder` span branch with always rendering
   `<SelectValue placeholder="Multiple" />`. Keep italic/muted look via the
   trigger's `data-placeholder` styling (add `italic` utility if needed). Apply to
   Status, Priority, Severity, Epic, Sprint. Remove the now-dead `multiplePlaceholder`.
2. **Add Team field** at the top of the field list. Staged `stagedTeamId` state.
   Options from `teams` prop. Patch sends `team_id`.
3. **Wire props**: pass `teams` (already in `TaskListView`) into `TaskBulkActionsBar`.
4. **Apply**: include `team_id` in the per-task patch when staged.

### Phase 2 — live re-scope on team change

5. Also pass `workflows` (all, each with `team_id` + `default_state_id`) into the modal.
6. When `stagedTeamId` is set:
   - Clear staged Sprint/Status/Epic/Labels (and ignore their common values).
   - Re-scope option lists to the selected team:
     - Status: pick workflow from `workflows` where `team_id === stagedTeamId`.
     - Sprint: `useSprints(wsId, { team_id: stagedTeamId })`.
     - Epic: `useEpics(wsId, { team_id: stagedTeamId })`.
     - Labels: `useLabels(wsId, { teamId: stagedTeamId, includeShared: true })`.
   - When no team staged, use the existing props (current behavior).
7. User can set new Sprint/Status/Epic/Labels for the new team, then Apply once.

### Patch-builder rules (verified against backend `PMTaskService.Update`)

Backend constraints (`server/internal/service/pm_task.go`, `pm_task_scope.go`):
- On team change it revalidates the task's **existing** epic/sprint against the new
  team (`:1430-1439`) → a `{team_id}`-only patch is rejected if the task had an
  old-team epic/sprint.
- `workflow_state_id` is validated against `workflow_id` (`:1408-1416`); old workflow
  + new-team state fails.
- Labels only change when `label_ids` is present (`:1554`); `Label.team_id` = scope
  (shared label has no `team_id`).

When `stagedTeamId` is set, the per-task patch MUST be:
- `team_id` = stagedTeamId
- `epic_id` = stagedEpicId (if user picked a new-team epic) else `''` (clear)
- `sprint_id` = stagedSprintId (if picked) else `''` (clear)
- `workflow_id` = new team's workflow id; `workflow_state_id` = stagedStatus
  — **REQUIRED** (decision #3): Apply is disabled until the user picks a status from
  the new team's workflow. No default fallback.
- `label_ids` = `[]` — **clear all labels** (decision #4)

**Apply gating:** when team changed, Apply stays disabled until a status is selected
(valid in the new team's workflow). Show a short hint near the Status field.

When team is NOT changed, behavior is unchanged (only explicitly staged fields patch).

## Testing (TDD)

- Extract the patch builder into a **pure helper** (e.g. `buildBulkPatch(staged,
  task, { teamChanged, newWorkflow })`) and unit-test it directly — no React needed:
  - team change emits `team_id`, `epic_id:''`, `sprint_id:''`, `workflow_id` +
    `workflow_state_id` (default when unstaged), and `label_ids` filtered to
    shared/new-team labels.
  - no team change → only explicitly staged fields present.
- Extend `__tests__/TaskBulkActionsBar.test.tsx`: mixed-field dropdown opens and
  applies a value to all; Team field present at top; team change resets team-scoped
  fields. NOTE: component now uses query hooks → wrap renders in a
  `QueryClientProvider` (current harness only has `TooltipProvider`).
- `pnpm --filter frontend test` + `pnpm --filter frontend typecheck`.

## Out of scope (separate follow-ups)

- Team-specific statuses in single-story edit (feedback issue #3).
- Markdown-paste-as-code-block fix + Mermaid in PM task descriptions (#4, #5).
- Team-scoped / soft-scoped owners.
