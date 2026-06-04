# PM Task Multiple Owners Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make multiple owners a first-class concept on PM tasks. Drop the legacy single-owner columns (`owner_member_id`, `owner_id`) from `pm_tasks` and treat the `pm_task_owners` join table as the only source of truth — mirroring the pattern already in place for `pm_objectives`.

**Architecture:** Join table `pm_task_owners (task_id, user_id)` is the source of truth. Backend exposes owners on every PMTask response as `owner_member_ids: []string` (workspace-member-id space, translated from the user-id join table) and, on `TaskDetail`, as a hydrated `Owners: []User`. Frontend mirrors the Objectives pattern: `owner_member_ids: string[]` on `Task`, `MultiMemberPickerPopover` for selection, an avatar-stack for display.

**Migration strategy:** Backend changes are **additive** through Tasks 2–13 — every backend commit compiles and tests pass. The legacy `OwnerID` / `OwnerMemberID` struct fields and underlying DB columns remain in place but are no longer read or written by Task 4 onwards. Only Task 14 (the very last backend task) removes the legacy struct fields *and* applies the DB migration that drops the columns. Frontend Tasks 6–13 are one coordinated patch stack: typecheck may fail between checkpoints, so none of those frontend checkpoints are standalone merge commits until Task 13 passes `pnpm typecheck && pnpm test`.

**Tech Stack:** Go 1.24 + GORM + PostgreSQL (via `dbmigrate`), React 19 + TypeScript, TanStack Query, shadcn/ui. Tests: Go testing, vitest.

**Out of scope:** Owner fields on CRM (contacts/deals/companies), `pm_epics`, `pm_sprints`, `pm_recurring_templates`, `pm_task_templates`, and `support_inbox` are *not* touched in this plan. Each is a separate concept and should be migrated in its own plan if/when desired.

---

## Product Decisions (encoded in this plan)

| Decision | Choice | Rationale |
|---|---|---|
| Primary owner concept | **None.** Owners are a true set. | Linear-style. User explicitly chose first-class multi-owner. |
| Group-by-owner on board | Duplicate the card across each owner's lane; show "Unassigned" lane for tasks with no owners. | Standard for true-set tools. |
| MyWork filter | "I am one of the owners" (set membership). | Matches user intent. |
| Notification on owner change | Existing auto-follow on `AddOwner`/`RemoveOwner` already covers this. No new emission. | Service already emits `owner_added` / `owner_removed` activity events. |
| API field name on responses | `owner_member_ids: []string` (workspace-member-id space). | Matches existing `pm_objectives` API. |
| Join-table identifier | Keep `pm_task_owners.user_id` (user-id space). Translate to/from workspace-member-id at the service boundary. | Matches existing schema; lower-risk than re-keying the join table. |
| Branch-template `{owner}` token | Not used anywhere in the codebase (verified: no `{owner}` template token exists in branch-template logic; the `{ownerName}` matches in grep are JSX expressions inside React components, unrelated). No action needed. | Verified during planning. |

---

## File Map

### Backend — modify
- `server/internal/model/pm_task.go` — first add `OwnerMemberIDs []string` to `PMTask` (gorm `-`), `BoardTask`, `PMTaskFilters`, and accept `OwnerMemberIDs` on requests while keeping legacy fields. In Task 14 only, remove `OwnerID`, `OwnerMemberID`, `TaskDetail.OwnerMember`, and `BoardTask.OwnerName`.
- `server/internal/repository/pm_task.go` — rewrite owner filter (lines 309–310, 1421–1422) to use `EXISTS (SELECT 1 FROM pm_task_owners JOIN workspace_members ...)`. Rewrite `ListByMember` (~1615) and member-board aggregations (1634–1870) to use the join. Update board projection (1026–1153) to load `owner_member_ids`. Drop `loadAssignableMember(... task.OwnerMemberID)` in `buildTaskDetail` (~2007).
- `server/internal/service/pm_task.go` — simplify `CreateTask` (329, 363–407), `UpdateTask` (814, 862–894, 908–909), seed (698–699): stop writing legacy `OwnerID`/`OwnerMemberID`; only handle `OwnerIDs` plus user IDs translated from incoming `OwnerMemberIDs`.
- `server/internal/handler/pm_task.go` — prefer `owner_member_ids` query string for PM task filters. Legacy query fields may remain on DTOs until Task 14 but should no longer be used by PM-task list/search callers.
- `server/internal/router/router.go` — no change. `/owners` and `/owners/{userId}` endpoints (838, 839) stay as is.
- `server/internal/worker/tools_teampulse.go`, `server/internal/worker/context.go`, `server/internal/service/internal_command_service.go`, `server/internal/service/pm_import.go`, `server/internal/temporalapp/activities.go` — replace any PM-task creation that sets `OwnerMemberID` with `OwnerMemberIDs: []string{...}`. (Verify each file scopes to *PM tasks*, not CRM.)
- `server/internal/repository/pm_sprint_planning_test.go`, `server/internal/repository/pm_task_member_board_test.go`, `server/internal/service/pm_task_extended_test.go`, `server/internal/service/pm_import_test.go`, `server/internal/handler/pm_sprint_planning_test.go` — update fixtures and assertions.

### Backend — create
- `server/internal/dbmigrate/sql/202605050001_pm_task_owners_backfill_and_drop_legacy.sql` — new migration: backfill any rows where `pm_tasks.owner_member_id IS NOT NULL` into `pm_task_owners` (translating member-id → user-id), then `ALTER TABLE pm_tasks DROP COLUMN owner_id, DROP COLUMN owner_member_id`.

### Frontend — modify
- `frontend/src/lib/pmTypes.ts` — replace `owner_member_id?: string` with `owner_member_ids?: string[]` on `Task`. Drop task `owner_id` and `owner_name`. Ensure `TaskDetail` keeps hydrated `owners?: User[]`; do not add a duplicate top-level `owner_member_ids` because callers read `detail.task.owner_member_ids`. Update `CreateTaskRequest`/`UpdateTaskRequest` payload types.
- `frontend/src/lib/pm-types/project.ts`, `frontend/src/lib/pm-types/objectives.ts`, `frontend/src/lib/pm-types/support.ts` — same shape changes if they re-declare PM Task. `objectives.ts` already has the multi-owner pattern (reference, not edit). `support.ts`'s `owner_member_id` belongs to support tickets (out of scope) — leave it. Verify each file before touching.
- `frontend/src/lib/services/pmTaskService.ts` — update create/update payload type from `owner_member_id?: string` (line 56) to `owner_member_ids?: string[]`.
- `frontend/src/components/pm/TaskDetailPanel.tsx` (lines 164, 190, 234, 912–915, 1451–1477) — replace single picker with `MultiMemberPickerPopover`; replace `currentOwnerName` derivation with avatar-stack render.
- `frontend/src/components/pm/TaskCard.tsx` (lines 55, 57, 153–154, 240–258, 665–706) — replace `ownerNameMap.get(owner_member_id)` with iteration over `owner_member_ids`; render avatar stack; replace inline single-picker with `MultiMemberPickerPopover`.
- `frontend/src/components/pm/CreateTaskModal.tsx` (lines 95, 132, 259, 335, 356, 373, 489–492, 583, 617, 695, 725, 1155, 1230–1239) — `initialOwnerMemberId?: string` → `initialOwnerMemberIds?: string[]`; form state and payload `owner_member_ids: string[]`.
- `frontend/src/components/pm/GlobalCreateModals.tsx` (task path: 131, 156, 168, 203, 288, 607–629) — mirror the objective path (1205, 1234, 1376–1384) which already uses `MultiMemberPickerPopover`.
- `frontend/src/components/pm/TaskFilters.tsx` (line 506) — keep array UI, rename filter key from `owner_member_id` to `owner_member_ids`.
- `frontend/src/components/pm/TaskListView.tsx`, `frontend/src/components/pm/KanbanBoard.tsx`, `frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx`, `frontend/src/components/pm/sprints/SprintPlanningColumn.tsx`, `frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx`, `frontend/src/components/pm/RoadmapEpicBar.tsx`, `frontend/src/components/docs/TaskItemMetadataToolbar.tsx` — replace PM-task single-owner display/payload with multi. Leave `RecurringTemplateList.tsx` and `TaskTemplatesSettings.tsx` single-owner because template owner fields are out of scope.
- `frontend/src/stores/pmBoardStore.ts` (line 145) — `matchesCsv(task.owner_member_id, …)` → array-overlap match.
- `frontend/src/stores/globalCreateStore.ts` — owner field shape change.
- `frontend/src/hooks/queries/useTasks.ts`, `frontend/src/hooks/useRealtimeSync.ts` — payload/event field rename.
- `frontend/src/pages/pm/MyWork.tsx` (line 118) — `{ owner_member_id: memberId, … }` → `{ owner_member_ids: [memberId], … }`.
- `frontend/src/pages/pm/SprintDetail.tsx`, `frontend/src/pages/pm/EpicDetail.tsx`, `frontend/src/pages/pm/Sprints.tsx`, `frontend/src/pages/pm/Epics.tsx`, `frontend/src/pages/pm/ObjectiveDetail.tsx` (verify no PM-task owner reads remain) — replace single-owner reads with array reads when reading PM tasks (epics' own owner field is out of scope).
- `frontend/src/lib/pmDefaultViews.ts`, `frontend/src/lib/__tests__/queryBuilder.test.ts` — filter-key rename.
- `frontend/src/stores/__tests__/pmBoardFetchScheduler.test.ts`, `frontend/src/stores/__tests__/pmBoardStore.test.ts` — fixture/assertion updates.

### Frontend — create
- `frontend/src/components/pm/OwnerAvatarStack.tsx` — small reusable avatar-stack component (max 3 visible + `+N` overflow). Used everywhere a single owner avatar was rendered.

---

## Tasks

### Task 1: Confirm join-table backfill is complete and write the drop migration

**Files:**
- Create: `server/internal/dbmigrate/sql/202605050001_pm_task_owners_backfill_and_drop_legacy.sql`

- [ ] **Step 1: Verify the join table is currently in sync with both legacy columns on staging.**

Run on a staging psql session:
```sql
-- Members not in join table:
SELECT COUNT(*) FROM pm_tasks t
WHERE t.owner_member_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM pm_task_owners po
    JOIN workspace_members wm ON wm.user_id = po.user_id
    WHERE po.task_id = t.id AND wm.id = t.owner_member_id
  );

-- Pre-member-era owner_id not in join table:
SELECT COUNT(*) FROM pm_tasks t
WHERE t.owner_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM pm_task_owners po WHERE po.task_id = t.id AND po.user_id = t.owner_id
  );
```
Expected: both `0`. If non-zero, the migration's backfill blocks must run. If zero, backfill is a no-op safety net.

- [ ] **Step 2: Write the migration.**

Note: backfill BOTH legacy columns. `owner_id` (user-id) predates `owner_member_id` (member-id). Tasks created in the pre-`workspace_members` era may have `owner_id` set with `owner_member_id` NULL — without this second backfill those owners are lost when columns drop.

```sql
-- 202605050001_pm_task_owners_backfill_and_drop_legacy.sql
-- Backfill any legacy owner rows into pm_task_owners, then drop legacy columns.

-- Backfill #1 (idempotent): translate workspace_member_id → user_id and insert.
INSERT INTO pm_task_owners (task_id, user_id, created_at)
SELECT t.id, wm.user_id, NOW()
FROM pm_tasks t
JOIN workspace_members wm ON wm.id = t.owner_member_id
WHERE t.owner_member_id IS NOT NULL
ON CONFLICT (task_id, user_id) DO NOTHING;

-- Backfill #2 (idempotent): pre-member-era rows that only set owner_id (user-id directly).
INSERT INTO pm_task_owners (task_id, user_id, created_at)
SELECT t.id, t.owner_id, NOW()
FROM pm_tasks t
WHERE t.owner_id IS NOT NULL
ON CONFLICT (task_id, user_id) DO NOTHING;

-- Drop legacy single-owner columns from pm_tasks.
ALTER TABLE pm_tasks DROP COLUMN IF EXISTS owner_member_id;
ALTER TABLE pm_tasks DROP COLUMN IF EXISTS owner_id;
```

- [ ] **Step 3: Run `migrate validate` from `server/`.**

Run: `cd server && go run ./cmd/migrate validate`
Expected: exits 0.

- [ ] **Step 4: Do NOT apply the migration yet.** The migration file is created here, but applying it now would break Task 2's additive model (which still references `OwnerID` / `OwnerMemberID` GORM columns). The migration is applied in Task 14 *after* every reader migrates and the legacy struct fields are deleted. This task ends with the SQL committed but unapplied.

- [ ] **Step 5: Commit.**

```bash
git add server/internal/dbmigrate/sql/202605050001_pm_task_owners_backfill_and_drop_legacy.sql
git commit -m "feat(pm): backfill pm_task_owners and drop legacy owner columns"
```

---

### Task 2: Backend — additive model changes (no breaking removals yet)

This task is **purely additive**: add the new multi-owner fields alongside the legacy single-owner fields. Every commit in the backend stack must compile and pass tests; the legacy fields are removed in Task 14 *after* every reader has migrated.

**Files:**
- Modify: `server/internal/model/pm_task.go` (around lines 39–40, 131–132, 165–166, 178, 207–208, 221, 252–254, 280)

- [ ] **Step 1: On `PMTask`, ADD** `OwnerMemberIDs []string \`json:"owner_member_ids" gorm:"-"\`` next to existing `OwnerID` / `OwnerMemberID` fields (do NOT remove them yet).

- [ ] **Step 2: On `PMTaskFilters`, ADD** `OwnerMemberIDs []string` next to `OwnerID` / `OwnerMemberID`.

- [ ] **Step 3: On `CreateTaskRequest`, ADD** `OwnerMemberIDs []string \`json:"owner_member_ids"\`` (alongside existing `OwnerID`, `OwnerMemberID`, and `OwnerIDs`).

- [ ] **Step 4: On `UpdateTaskRequest`, ADD** the same field (alongside existing).

- [ ] **Step 5: On `TaskDetail` — no new field needed.** Frontend reads `detail.task.owner_member_ids` from the embedded task. Existing `Owners []User` (for richer hydration) is kept. `OwnerMember *AssignableMember` is removed in Task 14.

- [ ] **Step 6: On `BoardTask`, ADD** `OwnerMemberIDs []string \`json:"owner_member_ids"\``. Keep `OwnerName *string` for now (removed in Task 14).

- [ ] **Step 7: Run `cd server && go build ./... && go test ./...`.** Expected: PASS (purely additive — nothing existing is broken).

- [ ] **Step 8: Commit.**

```bash
git add server/internal/model/pm_task.go
git commit -m "feat(pm/model): add OwnerMemberIDs slice alongside legacy owner fields"
```

---

### Task 3: Backend — repository owner reads/writes (filter, board, list-by-member, task detail)

**Files:**
- Modify: `server/internal/repository/pm_task.go`
- Test: `server/internal/repository/pm_task_member_board_test.go` (existing, update)
- Test: `server/internal/repository/pm_task_owner_filter_test.go` (new)

- [ ] **Step 1: Write a failing test for the new EXISTS owner filter.** Create `pm_task_owner_filter_test.go`. Seed two tasks: A with two owners (Alice + Bob via `pm_task_owners`), B with no owners. Call `repo.List(ctx, PMTaskFilters{OwnerMemberIDs: []string{aliceMemberID}})`. Assert exactly `[A]`. Call again with `{bobMemberID, charlieMemberID}` (Charlie not on any task). Assert `[A]` (set-overlap). Call with empty `OwnerMemberIDs`. Assert `[A, B]` (no filter).

- [ ] **Step 2: Run** `cd server && go test ./internal/repository/ -run TestOwnerFilter -v`. Expect FAIL (filter not implemented yet — `OwnerMemberIDs` is on the struct from Task 2 but not wired).

- [ ] **Step 3: Update filter helpers (309–310, 1421–1422).** Replace the deleted single-value owner filter with:
```go
if len(filters.OwnerMemberIDs) > 0 {
    q = q.Where(`EXISTS (
        SELECT 1 FROM pm_task_owners po
        JOIN workspace_members wm ON wm.user_id = po.user_id
        WHERE po.task_id = pm_tasks.id AND wm.id IN ?
    )`, filters.OwnerMemberIDs)
}
```
Apply at both List (309–310) and Search (1421–1422) sites. Re-run test from Step 2. Expect PASS.

- [ ] **Step 4: Update board projection (1026–1153) — remove the legacy owner-name path entirely.** Specifically:
  - Delete `legacyOwnerNameMap := r.batchOwnerNames(ctx, ownerIDs)` at line 1037.
  - Drop `legacyOwnerNameMap` from the `enrichBoardTasks` signature at 1123 and call site at 1047.
  - Delete the `if name, ok := legacyOwnerNameMap[*task.OwnerID]` block at 1153–1154 and stop writing `bs.OwnerName`; `OwnerName` stays on the struct until Task 14 but should no longer be populated by PM-task board reads.
  - Delete the now-unused `batchOwnerNames` function at 1509–1510 (and any call sites).
  - Add a new helper that loads owner member IDs per task ID:
    ```go
    SELECT po.task_id, wm.id AS member_id
    FROM pm_task_owners po
    JOIN workspace_members wm ON wm.user_id = po.user_id
    WHERE po.task_id IN (?)
    ORDER BY po.task_id, po.created_at ASC
    ```
    Group into `map[taskID][]memberID` and assign to each `BoardTask.OwnerMemberIDs`.

- [ ] **Step 5: Rewrite `ListByMember` (~1615) and member-board aggregations (1634, 1636, 1642, 1643, 1651, 1652, 1680, 1682, 1868, 1870).** Filtering with EXISTS from Step 3 is *not enough* for grouping — multi-owner means a single task may appear under multiple members' columns, and unassigned tasks must surface in their own bucket. Use explicit JOIN + GROUP BY for "tasks owned by member X" and `NOT EXISTS` for unassigned:
```sql
-- Tasks owned by a specific member (returns rows; one task may surface for many members):
SELECT t.* FROM pm_tasks t
JOIN pm_task_owners po ON po.task_id = t.id
JOIN workspace_members wm ON wm.user_id = po.user_id AND wm.workspace_id = t.workspace_id
WHERE wm.id = ? AND t.archived = false
ORDER BY t.position ASC

-- Member-board aggregation: count tasks per owning member in workspace:
SELECT wm.id AS member_id, COUNT(DISTINCT t.id) AS task_count
FROM workspace_members wm
LEFT JOIN pm_task_owners po ON po.user_id = wm.user_id
LEFT JOIN pm_tasks t ON t.id = po.task_id AND t.workspace_id = wm.workspace_id AND t.archived = false
WHERE wm.workspace_id = ?
GROUP BY wm.id

-- Unassigned bucket (tasks with NO owners):
SELECT t.* FROM pm_tasks t
WHERE t.workspace_id = ? AND t.archived = false
  AND NOT EXISTS (SELECT 1 FROM pm_task_owners po WHERE po.task_id = t.id)
ORDER BY t.position ASC
```
Apply the same shape to every aggregation site (1634–1870). The previous `WHERE pm_tasks.owner_member_id = ?` patterns must all be replaced — the column will not exist after Task 5b.

- [ ] **Step 6: Update `buildTaskDetail` (~2007).** Drop `loadAssignableMember(... task.OwnerMemberID)` and the `OwnerMember` assignment. Keep the existing `Owners []User` join (already at line 1989). Populate `OwnerMemberIDs` via the same workspace-member translation as Step 4's helper.

- [ ] **Step 7: Update `pm_task_member_board_test.go`.** Fixtures must seed `pm_task_owners` instead of setting `OwnerMemberID`. Assertions read `task.OwnerMemberIDs` (slice, ordered by `created_at` ASC for determinism).

- [ ] **Step 8: Run `cd server && go test ./internal/repository/...`.** Expected: full pass.

- [ ] **Step 9: Commit.**

```bash
git add server/internal/repository/pm_task.go server/internal/repository/pm_task_member_board_test.go server/internal/repository/pm_task_owner_filter_test.go
git commit -m "refactor(pm/repo): read owners exclusively from pm_task_owners join"
```

---

### Task 4: Backend — service create/update/seed paths

Two real bugs to avoid: (a) the actual API is `s.followerService.Follow(ctx, userID, "task", taskID, workspaceID, "owner")` — NOT a `followService.Follow(taskID, userID)` two-arg call. (b) `ReplaceOwners` is a bulk swap on the repo and does NOT emit `owner_added` / `owner_removed` activity events. `UpdateTask` must diff the owner set and call the **service-level** `AddOwner` / `RemoveOwner` (lines 1333, 1390) per added/removed id — those service methods already log activity AND auto-follow.

Tasks 2–5 are additive: legacy `OwnerID` / `OwnerMemberID` struct fields stay in place (they're removed in Task 5b after every reader has migrated). This task stops *writing* to them. Reads of legacy fields are dropped from the service.

**Files:**
- Modify: `server/internal/service/pm_task.go` (329, 363–364, 401–419, 514–516, 698–699, 814, 862–868, 888–915)
- Test: `server/internal/service/pm_task_extended_test.go`, `server/internal/service/pm_import_test.go`

- [ ] **Step 1: Add helpers in `pm_task.go` (or `pm_task_helpers.go`).**
```go
// resolveMemberIDsToUserIDs translates workspace_member.id → workspace_member.user_id.
// Returns error if any member id is missing or belongs to a different workspace.
func (s *PMTaskService) resolveMemberIDsToUserIDs(ctx context.Context, workspaceID string, memberIDs []string) ([]string, error) { ... }

func dedupeStrings(in []string) []string { ... } // if no equivalent helper already exists
```

- [ ] **Step 2: `CreateTask` (363–364, 401–419).** Stop assigning legacy `task.OwnerID` / `task.OwnerMemberID` from the request. Compute the merged owner-user-id set and write through the join table:
```go
memberOwnerUserIDs, err := s.resolveMemberIDsToUserIDs(ctx, newTask.WorkspaceID, req.OwnerMemberIDs)
if err != nil { return nil, err }
ownerUserIDs := dedupeStrings(append(memberOwnerUserIDs, req.OwnerIDs...))
for _, uid := range ownerUserIDs {
    if err := s.taskRepo.AddOwner(ctx, newTask.ID, uid); err != nil { return nil, err }
}
// Auto-follow each owner via the existing followerService.Follow path
// (mirrors the creator auto-follow at line 514–516).
if s.followerService != nil {
    for _, uid := range ownerUserIDs {
        if err := s.followerService.Follow(ctx, uid, "task", newTask.ID, newTask.WorkspaceID, "owner"); err != nil {
            s.logger.ErrorContext(ctx, "failed to auto-follow owner", "error", err, "task_id", newTask.ID, "user_id", uid)
        }
    }
}
```
The existing follower-list block at 412–419 that appended `*newTask.OwnerID` to `followerIDs` becomes dead — delete those lines (lines 417, 419 reference `OwnerID`).

- [ ] **Step 3: Seed path (698–699).** Same treatment — owners written via `AddOwner` only.

- [ ] **Step 4: `UpdateTask` (814, 862–868, 888–915) — diff owners and emit activity per change.** Replace the existing 888–894 `ReplaceOwners` block with a diff that calls the service-level `AddOwner` / `RemoveOwner` (so `owner_added` / `owner_removed` events fire, and auto-follow happens):
```go
if req.OwnerMemberIDs != nil || req.OwnerIDs != nil {
    nextMemberUserIDs, err := s.resolveMemberIDsToUserIDs(ctx, current.WorkspaceID, req.OwnerMemberIDs)
    if err != nil { return nil, err }
    next := dedupeStrings(append(nextMemberUserIDs, req.OwnerIDs...))
    nextSet := toSet(next)
    currentOwnerIDs, err := s.taskRepo.ListOwnerUserIDs(ctx, current.ID)
    if err != nil { return nil, err }
    currentSet := toSet(currentOwnerIDs)
    for _, uid := range next {
        if _, ok := currentSet[uid]; !ok {
            if _, err := s.AddOwner(ctx, current.ID, uid, actorID); err != nil { return nil, err }
        }
    }
    for _, uid := range currentOwnerIDs {
        if _, ok := nextSet[uid]; !ok {
            if _, err := s.RemoveOwner(ctx, current.ID, uid, actorID); err != nil { return nil, err }
        }
    }
}
```
Then delete the legacy follower-list reference at 909 (`followers = append(followers, *current.OwnerID)`) — `AddOwner` already auto-follows.

- [ ] **Step 5: Update `pm_task_extended_test.go` and `pm_import_test.go`** — fixtures use `OwnerMemberIDs: []string{...}`; assertions verify activity log emits `owner_added` / `owner_removed` on `UpdateTask` paths that change owners (this is the regression test for the bug Step 4 fixes).

- [ ] **Step 6: Run `cd server && go test ./internal/service/...`.** Expected: pass.

- [ ] **Step 7: Commit.**

```bash
git add server/internal/service/pm_task.go server/internal/service/pm_task_extended_test.go server/internal/service/pm_import_test.go
git commit -m "refactor(pm/service): owners written through join table; UpdateTask diffs and logs activity"
```

---

### Task 5: Backend — handler, router, and remaining backend touchpoints

**Files:**
- Modify: `server/internal/handler/pm_task.go` (53, 54, 217, 218)
- Modify: `server/internal/worker/tools_teampulse.go`, `server/internal/worker/context.go`, `server/internal/service/internal_command_service.go`, `server/internal/service/pm_import.go`, `server/internal/temporalapp/activities.go` (only the PM-task-touching paths; CRM/support paths stay)
- Test: `server/internal/handler/pm_sprint_planning_test.go`, `server/internal/repository/pm_sprint_planning_test.go`

- [ ] **Step 1: Handler filter pass-through (53, 54, 217, 218).** Replace `owner_member_id` query parsing with `owner_member_ids` (CSV split or repeated query param). Pass to `PMTaskFilters.OwnerMemberIDs`.

- [ ] **Step 2: Worker / temporal / internal-command paths — exact sites (verified PM-task contexts).**
  - `server/internal/worker/tools_teampulse.go:139` — agent-tool input struct field `OwnerMemberID *string` → rename to `OwnerMemberIDs []string \`json:"owner_member_ids"\``.
  - `server/internal/worker/tools_teampulse.go:172` — drop the single-value `trimPtr(&params.OwnerMemberID)`; trim each element of the new slice instead.
  - `server/internal/worker/tools_teampulse.go:199` — JSON map entry `"owner_member_id": params.OwnerMemberID` → `"owner_member_ids": params.OwnerMemberIDs`.
  - `server/internal/worker/tools_teampulse.go:227` — `OwnerMemberID: params.OwnerMemberID` (PM task create payload) → `OwnerMemberIDs: params.OwnerMemberIDs`.
  - `server/internal/worker/context.go:325` — context payload field rename + slice.
  - `server/internal/temporalapp/activities.go:2417` — `OwnerID: req.OwnerID` (PM task create payload) → drop; rely on `OwnerMemberIDs`/`OwnerIDs` slices.
  - `server/internal/temporalapp/activities.go:2465` — emitted JSON map key `"owner_member_id"` → `"owner_member_ids"` (slice).
  - `server/internal/service/internal_command_service.go:340` — request struct field rename + slice.
  - `server/internal/service/internal_command_service.go:362` — drop the single-value normalization; map across slice.
  - `server/internal/service/internal_command_service.go:387` — `OwnerMemberID: req.OwnerMemberID` → `OwnerMemberIDs: req.OwnerMemberIDs`.

  CRM contact/deal/company owner assignments and any `support_inbox` / `pm_epic` / `pm_sprint` / `pm_objective` / `pm_recurring_template` / `pm_task_template` files are unaffected.

- [ ] **Step 3: Update sprint-planning tests' PM-task fixtures** (the only tests in this list that touch PM tasks).

- [ ] **Step 4: Run `cd server && go build ./... && go test ./...`.** Expected: full backend pass.

- [ ] **Step 5: Commit.**

```bash
git add server/internal/handler/pm_task.go server/internal/worker/ server/internal/service/internal_command_service.go server/internal/service/pm_import.go server/internal/temporalapp/activities.go server/internal/handler/pm_sprint_planning_test.go server/internal/repository/pm_sprint_planning_test.go
git commit -m "refactor(pm/handlers): switch PM-task callers to owner_member_ids"
```

---

### Task 6: Frontend — types

Frontend Tasks 6–13 are intentionally one coordinated frontend patch stack. After Task 6 changes the shared types, `pnpm typecheck` is expected to fail until the usage sites are migrated in Tasks 8–13. The commit steps in this frontend section are local checkpoints for handoff/review only; they are not standalone safe commits. If your workflow requires every commit to pass CI independently, skip the frontend checkpoint commits and create one verified frontend commit after Task 13.

**Files:**
- Modify: `frontend/src/lib/pmTypes.ts`
- Modify: `frontend/src/lib/pm-types/project.ts` (verify whether `Task` is re-declared here)
- Modify: `frontend/src/lib/pm-types/objectives.ts` (verify; objectives are out of scope, but if it re-exports a shared `Task` type, follow the rename)
- Modify: `frontend/src/lib/services/pmTaskService.ts` (line 56)

- [ ] **Step 1: In `pmTypes.ts`,** replace `owner_member_id?: string` and `owner_id?: string` on `Task` with `owner_member_ids?: string[]`. Drop `owner_name`. Ensure `TaskDetail` has `owners?: User[]` for hydrated owner users, but do not add a duplicate top-level `owner_member_ids`; callers read `detail.task.owner_member_ids`. On `CreateTaskRequest` / `UpdateTaskRequest`, replace `owner_member_id` with `owner_member_ids?: string[]`.

- [ ] **Step 2: In `pm-types/project.ts`,** apply the same changes if `Task` is re-declared. Skip if the file only re-exports.

- [ ] **Step 3: In `pmTaskService.ts:56`,** change `owner_member_id?: string` → `owner_member_ids?: string[]`.

- [ ] **Step 4: Run `cd frontend && pnpm typecheck`.** Expected: FAIL with usage-site errors only. Do not leave unrelated errors in place. The owner-field usage errors are fixed in Tasks 8–13, and the frontend stack is mergeable only after Task 13 passes.

- [ ] **Step 5: Create a local checkpoint commit only if your workflow allows non-standalone frontend patch-stack commits.**

```bash
git add frontend/src/lib/pmTypes.ts frontend/src/lib/pm-types/project.ts frontend/src/lib/services/pmTaskService.ts
git commit -m "refactor(pm/types): owner_member_ids array replaces single owner field"
```

---

### Task 7: Frontend — `OwnerAvatarStack` component

**Files:**
- Create: `frontend/src/components/pm/OwnerAvatarStack.tsx`
- Test: `frontend/src/components/pm/__tests__/OwnerAvatarStack.test.tsx`

- [ ] **Step 1: Write a failing test** asserting that for `[a, b, c, d]` it renders 3 avatars + a `+1` overflow chip with tooltip listing all four names.

- [ ] **Step 2: Run** `cd frontend && pnpm test --filter OwnerAvatarStack`. Expect FAIL.

- [ ] **Step 3: Implement.** Props: `memberIds: string[]`, `nameMap: Map<string, string>`, `size?: 'sm' | 'md'`, `max?: number` (default 3). Use the existing `UserAvatar` primitive; stack with `-ml-2` overlap; tooltip.

- [ ] **Step 4: Re-run test.** Expect PASS.

- [ ] **Step 5: Commit.**

```bash
git add frontend/src/components/pm/OwnerAvatarStack.tsx frontend/src/components/pm/__tests__/OwnerAvatarStack.test.tsx
git commit -m "feat(pm/ui): add OwnerAvatarStack component"
```

---

### Task 8: Frontend — TaskDetailPanel multi-owner picker

**Files:**
- Modify: `frontend/src/components/pm/TaskDetailPanel.tsx` (164, 190, 234, 912–915, 1451–1477)

- [ ] **Step 1: Form state.** Replace `owner_member_id: string` with `owner_member_ids: string[]`. Initialize from `detail.task.owner_member_ids ?? []`.

- [ ] **Step 2: Replace render block (1451–1477).** Use `MultiMemberPickerPopover` with `values={form.owner_member_ids}` and `onChange={(ids) => updateField('owner_member_ids', ids)}`. Replace single-name display with `<OwnerAvatarStack memberIds={form.owner_member_ids} nameMap={...} />`.

- [ ] **Step 3: On save**, send `owner_member_ids` in payload via `pmTaskService.update(...)`.

- [ ] **Step 4: Run** `cd frontend && pnpm typecheck`. Expected: this file is clean.

- [ ] **Step 5: Manual test:** open a task, add two owners, reload, confirm both persist; remove one; confirm activity log shows two `owner_added` and one `owner_removed`.

- [ ] **Step 6: Commit.**

```bash
git add frontend/src/components/pm/TaskDetailPanel.tsx
git commit -m "feat(pm/ui): TaskDetailPanel supports multiple owners"
```

---

### Task 9: Frontend — TaskCard inline picker + display

**Files:**
- Modify: `frontend/src/components/pm/TaskCard.tsx` (55, 57, 153–154, 240–258, 665–706)

- [ ] **Step 1: Props.** Replace `ownerNameMap?: Map<string, string>` callback shape to also accept the new field. Drop `onOwnerChanged` (replaced by onTaskPatched).

- [ ] **Step 2: `currentOwnerName` (240–244).** Remove. Replace with `OwnerAvatarStack` render at 697–698.

- [ ] **Step 3: `handleAssignOwner` (245–258).** Accept `string[]`, call `pmTaskService.update(workspaceId, task.id, { owner_member_ids: ids })`.

- [ ] **Step 4: Inline picker (665–675).** Swap `MemberPickerPopover` for `MultiMemberPickerPopover`.

- [ ] **Step 5: Manual test:** kanban board card, click avatar stack → multi-select dropdown opens → toggle two owners → both visible on card.

- [ ] **Step 6: Commit.**

```bash
git add frontend/src/components/pm/TaskCard.tsx
git commit -m "feat(pm/ui): TaskCard avatar stack + multi-select picker"
```

---

### Task 10: Frontend — CreateTaskModal + GlobalCreateModals (task path)

**Files:**
- Modify: `frontend/src/components/pm/CreateTaskModal.tsx` (95, 132, 259, 335, 356, 373, 489–492, 583, 617, 695, 725, 1155, 1230–1239)
- Modify: `frontend/src/components/pm/GlobalCreateModals.tsx` (task path: 131, 156, 168, 203, 288, 607–629)

- [ ] **Step 1: CreateTaskModal — props.** `initialOwnerMemberId?: string` → `initialOwnerMemberIds?: string[]`. Form default → `owner_member_ids: []`.

- [ ] **Step 2: CreateTaskModal — render (1230–1239).** Switch to `MultiMemberPickerPopover`.

- [ ] **Step 3: CreateTaskModal — payload (583, 617).** Send `owner_member_ids` (omit if empty).

- [ ] **Step 4: CreateTaskModal — template hydration (335, 1155).** Templates stay single-owner and out of scope. When applying a task template, hydrate the new task's multi-owner form state from the template's single field:
```ts
owner_member_ids: editingTemplate.owner_member_id ? [editingTemplate.owner_member_id] : []
```

- [ ] **Step 5: GlobalCreateModals — task path.** Mirror the **objective** path already at 1205, 1234, 1376–1384 (it's the working reference for multi-owner).

- [ ] **Step 6: Run** `cd frontend && pnpm typecheck`.

- [ ] **Step 7: Manual test:** create a new task from the global "+" modal with two owners; confirm both appear on the resulting card.

- [ ] **Step 8: Commit.**

```bash
git add frontend/src/components/pm/CreateTaskModal.tsx frontend/src/components/pm/GlobalCreateModals.tsx
git commit -m "feat(pm/ui): create-task modals accept multiple owners"
```

---

### Task 11: Frontend — list/kanban/sprint-planning surfaces

**Files:**
- Modify: `frontend/src/components/pm/TaskListView.tsx`
- Modify: `frontend/src/components/pm/KanbanBoard.tsx`
- Modify: `frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx`, `SprintPlanningColumn.tsx`, `SprintPlanningBacklogPanel.tsx`
- Modify: `frontend/src/components/pm/RoadmapEpicBar.tsx`
- Modify: `frontend/src/components/docs/TaskItemMetadataToolbar.tsx`

**Out of scope here:** `RecurringTemplateList.tsx` and `TaskTemplatesSettings.tsx` — templates own a *single* `owner_member_id` field on their own tables (`pm_task_templates`, `pm_recurring_templates`) and are out of scope for this plan. The template UI keeps its single picker. Template-driven task creation is handled in Task 10 Step 4: when a template is applied, hydrate the new task's `owner_member_ids` from the template's `owner_member_id` as a single-element array.

- [ ] **Step 1: Per file**, replace single `task.owner_member_id` reads with `task.owner_member_ids ?? []`. Replace single-avatar render with `<OwnerAvatarStack ... />`. Replace inline picker (if any) with `MultiMemberPickerPopover`.

- [ ] **Step 2: Group-by-owner on board.** In `KanbanBoard.tsx`, when grouping by owner, duplicate the card across each owner ID's lane: `task.owner_member_ids.length === 0 ? ['__unassigned__'] : task.owner_member_ids`. **Use a composite React key** to avoid duplicate-key warnings: `key={\`${task.id}::${ownerId}\`}` on each rendered card.

- [ ] **Step 3: Run** `cd frontend && pnpm typecheck && pnpm test`.

- [ ] **Step 4: Manual test:** list view with owner column shows stacks; sprint planning drag-drop preserves owners; group-by-owner lanes duplicate cards correctly.

- [ ] **Step 5: Commit.**

```bash
git add frontend/src/components/pm/ frontend/src/components/docs/TaskItemMetadataToolbar.tsx
git commit -m "feat(pm/ui): list/kanban/sprint surfaces render multi-owner"
```

---

### Task 12: Frontend — filters, board store, MyWork

**Files:**
- Modify: `frontend/src/components/pm/TaskFilters.tsx` (line 506)
- Modify: `frontend/src/stores/pmBoardStore.ts` (line 145)
- Modify: `frontend/src/stores/globalCreateStore.ts`
- Modify: `frontend/src/hooks/queries/useTasks.ts`, `frontend/src/hooks/useRealtimeSync.ts`
- Modify: `frontend/src/pages/pm/MyWork.tsx` (line 118)
- Modify: `frontend/src/lib/pmDefaultViews.ts`, `frontend/src/lib/__tests__/queryBuilder.test.ts`
- Modify: `frontend/src/stores/__tests__/pmBoardFetchScheduler.test.ts`, `frontend/src/stores/__tests__/pmBoardStore.test.ts`

- [ ] **Step 1: Write a failing test for the array-overlap matcher.** Extend `frontend/src/stores/__tests__/pmBoardStore.test.ts` with a case: task `{ id: 't1', owner_member_ids: ['a', 'b'] }`, filter `{ owner_member_ids: ['b', 'c'] }` → matches. Filter `{ owner_member_ids: ['c'] }` → does not match. Empty filter → matches.

- [ ] **Step 2: Run** `cd frontend && pnpm test --filter pmBoardStore`. Expect FAIL.

- [ ] **Step 3: `pmBoardStore.ts:145`.** Replace `matchesCsv(task.owner_member_id, filters.owner_member_id)` with array-overlap match:
```ts
if ((filters.owner_member_ids?.length ?? 0) > 0 &&
    !filters.owner_member_ids!.some((id) => task.owner_member_ids?.includes(id))) {
  return false;
}
```
Re-run test. Expect PASS.

- [ ] **Step 4: `TaskFilters.tsx:506`.** Rename filter key from `owner_member_id` to `owner_member_ids`. UI is already array-based.

- [ ] **Step 5: `MyWork.tsx:118`.** `{ owner_member_id: memberId }` → `{ owner_member_ids: [memberId] }`.

- [ ] **Step 6: `useRealtimeSync.ts:216–218`.** Delete the legacy enrichment block:
```ts
// Enrich with owner_name from task detail owners for board display.
if (task.owner_member_id && !task.owner_name && res.data.owner_member) {
  task.owner_name = res.data.owner_member.display_name || res.data.owner_member.email
}
```
The new `OwnerAvatarStack` reads `task.owner_member_ids` directly against `ownerNameMap` (populated by board init from members), so no enrichment is needed. Backend WebSocket payloads do not carry an `owner_member_id` field (verified — `server/internal/websocket/` has no owner-field references), so no backend rename is needed.

- [ ] **Step 7: Update remaining store tests and `pmDefaultViews.ts` defaults.**

- [ ] **Step 8: Run** `cd frontend && pnpm typecheck && pnpm test`. Expected: pass.

- [ ] **Step 9: Manual test:** open `/w/{slug}/pm/my-work` — confirm only my tasks appear; open board, apply owner filter with two owners — confirm tasks owned by either appear.

- [ ] **Step 10: Commit.**

```bash
git add frontend/src/components/pm/TaskFilters.tsx frontend/src/stores/ frontend/src/hooks/ frontend/src/pages/pm/MyWork.tsx frontend/src/lib/pmDefaultViews.ts frontend/src/lib/__tests__/queryBuilder.test.ts
git commit -m "feat(pm/filters): filters/MyWork match against owner set"
```

---

### Task 13: Frontend — sprint/epic detail pages (PM-task reads only)

**Files:**
- Modify: `frontend/src/pages/pm/SprintDetail.tsx` (line 271, swimlane grouping)
- Modify: `frontend/src/pages/pm/EpicDetail.tsx`
- Modify: `frontend/src/pages/pm/Sprints.tsx`, `frontend/src/pages/pm/Epics.tsx` (grid columns / aggregations)

- [ ] **Step 1: Swimlane grouping.** Same duplicate-across-lanes rule as Task 11 Step 2 — show `Unassigned` lane for tasks with no owners, duplicate across lanes for tasks with multiple.

- [ ] **Step 2: Confirm Epic/Sprint *own* owner field is unchanged.** The epic/sprint's own `owner_member_id` is **out of scope** — this task only changes how *PM tasks within those views* are read.

- [ ] **Step 3: Manual test:** sprint detail with group-by-owner view shows expected lanes.

- [ ] **Step 4: Commit or finalize the frontend patch stack.** If Tasks 6–12 used local checkpoint commits, verify this whole frontend stack before push/merge. If checkpoint commits were skipped to keep every commit CI-clean, make this the single frontend multi-owner commit after `pnpm typecheck && pnpm test` passes.

```bash
git add frontend/src/pages/pm/SprintDetail.tsx frontend/src/pages/pm/EpicDetail.tsx frontend/src/pages/pm/Sprints.tsx frontend/src/pages/pm/Epics.tsx
git commit -m "feat(pm/views): sprint/epic detail group PM tasks by owner set"
```

---

### Task 14: Backend — remove legacy owner fields and apply DB migration

This is the final breaking change. By now, every reader/writer in the backend uses `OwnerMemberIDs` / `pm_task_owners`, and the frontend (Tasks 6–13) reads `owner_member_ids`. Now we delete the legacy fields from the model and apply the DB migration that drops the columns.

**Files:**
- Modify: `server/internal/model/pm_task.go` — remove `OwnerID`, `OwnerMemberID` from `PMTask`, `PMTaskFilters`, `CreateTaskRequest`, `UpdateTaskRequest`. Remove `OwnerMember *AssignableMember` from `TaskDetail`. Remove `OwnerName *string` from `BoardTask`.
- Modify: `server/internal/repository/pm_task.go` — delete any remaining read of `task.OwnerID` / `task.OwnerMemberID` (should be none after Task 3, but verify).
- Modify: `server/internal/service/pm_task.go` — same verification.

- [ ] **Step 1: Delete legacy fields from model.** Run `cd server && go build ./...`. Any remaining references surface as compile errors — fix each one.

- [ ] **Step 2: Apply the migration locally.** `cd server && go run ./cmd/migrate up`. Confirm `\d pm_tasks` no longer has `owner_id` / `owner_member_id`.

- [ ] **Step 3: Run full backend tests.** `cd server && go test ./...`. Expected: pass.

- [ ] **Step 4: Commit.**

```bash
git add server/internal/model/pm_task.go server/internal/repository/pm_task.go server/internal/service/pm_task.go
git commit -m "refactor(pm/model): remove legacy single-owner fields"
```

---

### Task 15: End-to-end verification and final cleanup

- [ ] **Step 1: Full backend test.** `cd server && go test ./...`. Expected: pass.

- [ ] **Step 2: Full frontend test.** `cd frontend && pnpm typecheck && pnpm test`. Expected: pass.

- [ ] **Step 3: Search for stragglers.**
```bash
grep -rn "owner_member_id\b" server/internal/ --include='*.go' | grep -v 'crm_\|_test.go\|pm_epic\|pm_sprint\|pm_objective\|pm_task_template\|pm_recurring_template\|support_inbox'
grep -rn "owner_member_id\b" frontend/src/ --include='*.ts' --include='*.tsx' | grep -v 'crm\|epic\|sprint\|objective\|template\|support'
```
Expected: empty (PM-task code paths only). Anything that comes back is either (a) a legitimate non-PM-task reference (CRM, epic-as-entity, etc.) or (b) a missed migration site — fix it.

- [ ] **Step 4: Manual smoke test on staging.**
  - Create a task with two owners → both visible on board card.
  - Update a task to set three owners → confirm three avatars stack with `+1` overflow at small size.
  - Filter board by "Owner = Alice OR Bob" → tasks owned by either appear.
  - MyWork page shows tasks where current user is in owner set.
  - Group board by owner → cards duplicate across owner lanes.
  - Activity log on a task shows `owner_added` events for each addition and `owner_removed` for removal.
  - WebSocket: open task in two browsers; add an owner in one; confirm the other updates.

- [ ] **Step 5: Update `CLAUDE.md`** — add a one-line note under "Frontend Patterns" linking PM tasks to multi-owner via `owner_member_ids`. Update memory `MEMORY.md` to reflect the change.

- [ ] **Step 6: Commit and PR.**

```bash
git add CLAUDE.md
git commit -m "docs: note PM tasks support multiple owners"
```

Open a PR against `develop`. Title: `feat(pm): first-class multi-owner PM tasks`. Body links this plan.

---

## Risks & Open Questions

1. **`pm_task_owners.user_id` vs API's `owner_member_ids`** — every read path translates user-id → workspace-member-id via `workspace_members.user_id = po.user_id`. If a user is removed from a workspace, the join may return empty members; decide whether stale rows in `pm_task_owners` should be cleaned up by a separate worker (out of scope here, surface as a follow-up issue).
2. **AutoMigrate vs explicit drop** — GORM AutoMigrate cannot drop columns, so the `dbmigrate` SQL in Task 1 is the only path. Confirm `dbmigrate up` runs *after* AutoMigrate in `cmd/api/main.go` startup order so the dropped columns aren't re-added. (They won't be: AutoMigrate doesn't re-add columns it doesn't see in the struct.)
3. **External API consumers** — none known. Widget SDK and help center don't read PM tasks. Confirm before merging.
