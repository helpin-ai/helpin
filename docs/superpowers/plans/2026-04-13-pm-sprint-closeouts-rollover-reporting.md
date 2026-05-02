# PM Sprint Closeouts And Rollover Reporting Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve historical sprint results before rollover mutates task membership, then surface completed, unfinished, and rolled-over work in sprint detail and PM reports.

**Architecture:** Add a small closeout snapshot model that captures a sprint’s final outcome before any rollover moves happen. Keep current live sprint/task models unchanged for active planning, but introduce closeout read paths for historical reporting, sprint detail summaries, and “rolled into this sprint” context. Scope this to prospective closeouts only; do not attempt to reconstruct already-mutated historical sprints.

**Tech Stack:** Go 1.24, Chi, GORM, PostgreSQL, dbmigrate SQL, React 19, TypeScript 5.9, TanStack Query, TanStack Router, Vitest

---

## Decision Lock-Ins

- Closeout snapshots are the source of truth for historical sprint reporting. Live `pm_tasks.sprint_id` remains the source of truth for active/upcoming sprint views.
- A closeout is created before rollover moves any unfinished tasks.
- No new user-facing “Close sprint” action is added in this plan.
- No historical backfill is attempted for older closed sprints. We do not synthesize closeouts for already-ended sprints because rollover may already have rewritten membership.
- The automation runner only snapshots newly-ended sprints inside the existing post-end catch window. Legacy closed sprints without a closeout continue to fall back to current live stats.
- Reports v1 is a table, not charts. Show accurate numbers first; burnup/burndown can come later.
- Child closeout-task rows are stored now even though v1 UI only needs summary counts. This preserves exact task membership for future drill-down without overcomplicating the first UI.
- Points remain integer-based because the PM system already uses integer estimates.
- Outcome buckets are fixed:
  - `completed`
  - `unfinished_not_rolled`
  - `rolled_over`

## Current-State Notes

- Rollover currently rewrites history by directly updating `pm_tasks.sprint_id` in `server/internal/service/pm_automation.go`.
- Sprint stats are currently derived from current task membership via `PMSprintRepository.ComputeStats`, so historical counts drift after rollover.
- The PM reports route exists but is still a placeholder.
- `useRealtimeSync` already invalidates all `['pm', wsId, 'sprints', ...]` queries on `sprint` websocket events, so keep new closeout query keys under the same prefix.

## File Structure

### Backend

- Create: `server/internal/dbmigrate/sql/202604130001_add_pm_sprint_closeouts.sql`
  - Add `pm_sprint_closeouts` and `pm_sprint_closeout_tasks` tables plus indexes.
- Create: `server/internal/model/pm_sprint_closeout.go`
  - Add closeout models, outcome constants, and response DTOs.
- Create: `server/internal/repository/pm_sprint_closeout.go`
  - Add closeout persistence, lookup, and listing helpers.
- Modify: `server/cmd/api/main.go`
  - Wire the closeout repository into sprint and automation services.
- Modify: `server/cmd/temporal-worker/main.go`
  - Wire the closeout repository into the automation service used by the cron workflow.
- Modify: `server/internal/repository/pm_sprint.go`
  - Add small sprint lookup helpers needed by closeout creation, not a second stats system.
- Modify: `server/internal/service/pm_automation.go`
  - Create closeouts before rollover moves and publish sprint updates.
- Modify: `server/internal/service/pm_sprint.go`
  - Add read methods for single-sprint closeout, closeout list, and inbound rollover summaries.
- Modify: `server/internal/handler/pm_sprint.go`
  - Add `/closeout` and `/closeouts` endpoints.
- Modify: `server/internal/router/router.go`
  - Register the new sprint closeout routes in the correct order.
- Create: `server/internal/repository/pm_sprint_closeout_test.go`
  - Repository coverage for create/get/list semantics and idempotency.
- Create: `server/internal/service/pm_automation_test.go`
  - Automation coverage for snapshot-before-move and bucket counts.
- Create: `server/internal/service/pm_sprint_closeout_test.go`
  - Service coverage for access, single closeout reads, and inbound rollover summaries.
- Create: `server/internal/handler/pm_sprint_closeout_test.go`
  - Handler coverage for response shape and query filtering.
- Modify: `server/internal/service/pm_sprint_test.go`
- Modify: `server/internal/service/pm_mentions_test.go`
- Modify: `server/internal/service/workspace_test.go`
- Modify: `server/internal/handler/pm_sprint_planning_test.go`
  - Update constructor call sites if closeout repository becomes a required dependency.

### Frontend

- Modify: `frontend/src/lib/pm-types/project.ts`
  - Add closeout and reports table DTOs.
- Modify: `frontend/src/lib/services/pmSprintService.ts`
  - Add closeout fetchers.
- Modify: `frontend/src/lib/queryKeys.ts`
  - Add stable query keys for closeout reads.
- Modify: `frontend/src/hooks/queries/useSprints.ts`
  - Add closeout hooks.
- Create: `frontend/src/components/pm/sprints/SprintCloseoutSummary.tsx`
  - Closed sprint summary cards.
- Create: `frontend/src/components/pm/sprints/SprintRolledInBanner.tsx`
  - Banner on the receiving sprint.
- Modify: `frontend/src/pages/pm/SprintDetail.tsx`
  - Load and render closeout summary / rolled-in banner.
- Create: `frontend/src/pages/pm/Reports.tsx`
  - Replace the placeholder page with a lightweight closeout reports screen.
- Modify: `frontend/src/routes/_authenticated/w/$slug/pm/reports.tsx`
  - Route delegates to the new page component.
- Create: `frontend/src/components/pm/reports/SprintCloseoutTable.tsx`
  - Filterable closeout table for PM reports.
- Create: `frontend/src/hooks/queries/__tests__/useSprintCloseouts.test.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintCloseoutSummary.test.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintRolledInBanner.test.tsx`
- Create: `frontend/src/components/pm/reports/__tests__/SprintCloseoutTable.test.tsx`

## API Shape

### Single closeout

`GET /api/pm/sprints/{id}/closeout?workspace_id=...`

Response shape:

```json
{
  "closeout": {
    "id": "uuid",
    "sprint_id": "uuid",
    "workspace_id": "uuid",
    "team_id": "uuid-or-null",
    "rolled_to_sprint_id": "uuid-or-null",
    "committed_count": 12,
    "completed_count": 8,
    "unfinished_count": 4,
    "rolled_over_count": 3,
    "committed_points": 34,
    "completed_points": 21,
    "unfinished_points": 13,
    "rolled_over_points": 11,
    "closed_at": "2026-04-13T00:00:00Z"
  },
  "rolled_in_from": [
    {
      "source_sprint_id": "uuid",
      "source_sprint_name": "Sprint 12",
      "rolled_over_count": 3,
      "rolled_over_points": 11
    }
  ]
}
```

### Reports list

`GET /api/pm/sprints/closeouts?workspace_id=...&team_id=...`

Response shape:

```json
{
  "items": [
    {
      "closeout_id": "uuid",
      "sprint_id": "uuid",
      "sprint_name": "Sprint 12",
      "team_id": "uuid-or-null",
      "team_name": "Growth",
      "start_date": "2026-04-01",
      "end_date": "2026-04-14",
      "committed_count": 12,
      "completed_count": 8,
      "unfinished_count": 4,
      "rolled_over_count": 3,
      "committed_points": 34,
      "completed_points": 21,
      "unfinished_points": 13,
      "rolled_over_points": 11,
      "completion_rate": 0.6667,
      "rolled_to_sprint_id": "uuid-or-null",
      "rolled_to_sprint_name": "Sprint 13",
      "closed_at": "2026-04-15T00:00:00Z"
    }
  ]
}
```

## Task 1: Add Closeout Schema And Backend Models

**Files:**
- Create: `server/internal/dbmigrate/sql/202604130001_add_pm_sprint_closeouts.sql`
- Create: `server/internal/model/pm_sprint_closeout.go`
- Create: `server/internal/repository/pm_sprint_closeout_test.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/cmd/temporal-worker/main.go`

- [ ] **Step 1: Write failing repository tests for closeout persistence**

Add coverage for:
- create a closeout and read it back by `sprint_id`
- reject duplicate closeouts for the same sprint
- store child task rows with stable outcomes
- list closeout summaries filtered by workspace/team
- list inbound rollover summaries by `rolled_to_sprint_id`

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository -run 'TestPMSprintCloseoutRepository'
```

Expected: FAIL because closeout tables/models/repository methods do not exist yet.

- [ ] **Step 2: Add the dbmigrate migration**

In `server/internal/dbmigrate/sql/202604130001_add_pm_sprint_closeouts.sql`, create:

```sql
CREATE TABLE IF NOT EXISTS pm_sprint_closeouts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sprint_id UUID NOT NULL UNIQUE REFERENCES pm_sprints(id) ON DELETE CASCADE,
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  team_id UUID NULL REFERENCES teams(id) ON DELETE SET NULL,
  rolled_to_sprint_id UUID NULL REFERENCES pm_sprints(id) ON DELETE SET NULL,
  committed_count INTEGER NOT NULL DEFAULT 0,
  completed_count INTEGER NOT NULL DEFAULT 0,
  unfinished_count INTEGER NOT NULL DEFAULT 0,
  rolled_over_count INTEGER NOT NULL DEFAULT 0,
  committed_points INTEGER NOT NULL DEFAULT 0,
  completed_points INTEGER NOT NULL DEFAULT 0,
  unfinished_points INTEGER NOT NULL DEFAULT 0,
  rolled_over_points INTEGER NOT NULL DEFAULT 0,
  closed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pm_sprint_closeout_tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  closeout_id UUID NOT NULL REFERENCES pm_sprint_closeouts(id) ON DELETE CASCADE,
  task_id UUID NOT NULL REFERENCES pm_tasks(id) ON DELETE CASCADE,
  outcome TEXT NOT NULL CHECK (outcome IN ('completed', 'unfinished_not_rolled', 'rolled_over')),
  estimate INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (closeout_id, task_id)
);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeouts_workspace_closed_at
  ON pm_sprint_closeouts(workspace_id, closed_at DESC);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeouts_team_closed_at
  ON pm_sprint_closeouts(team_id, closed_at DESC);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeouts_rolled_to
  ON pm_sprint_closeouts(rolled_to_sprint_id);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeout_tasks_closeout
  ON pm_sprint_closeout_tasks(closeout_id);
```

Use `server/internal/dbmigrate/sql`, not `server/migrations`.

- [ ] **Step 3: Add focused model types**

In `server/internal/model/pm_sprint_closeout.go`, add:
- `PMSprintCloseout`
- `PMSprintCloseoutTask`
- outcome constants
- `SprintCloseoutResponse`
- `SprintCloseoutListItem`
- `SprintInboundRolloverSummary`

Keep these separate from `SprintWithStats`; historical reporting is a different read model.

- [ ] **Step 4: Implement the repository methods**

In `server/internal/repository/pm_sprint_closeout.go`, add:
- `CreateCloseout(ctx, closeout *model.PMSprintCloseout, tasks []model.PMSprintCloseoutTask) error`
- `GetCloseoutBySprintID(ctx, sprintID string) (*model.PMSprintCloseout, []model.PMSprintCloseoutTask, error)`
- `ListCloseoutSummaries(ctx, workspaceID string, teamID *string) ([]model.SprintCloseoutListItem, error)`
- `ListInboundRolloverSummaries(ctx, sprintID string) ([]model.SprintInboundRolloverSummary, error)`

`CreateCloseout` must be transactional and idempotent:
- if a closeout already exists for `sprint_id`, return the existing record without creating a second one
- child task rows must only be inserted once

- [ ] **Step 5: Re-run repository verification**

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository -run 'TestPMSprintCloseoutRepository'
cd /root/teampulse/server && go build ./cmd/api
cd /root/teampulse/server && go build ./cmd/temporal-worker
```

- [ ] **Step 6: Commit**

Run:
```bash
git add server/internal/dbmigrate/sql/202604130001_add_pm_sprint_closeouts.sql server/internal/model/pm_sprint_closeout.go server/internal/repository/pm_sprint_closeout.go server/internal/repository/pm_sprint_closeout_test.go server/cmd/api/main.go server/cmd/temporal-worker/main.go
git commit -m "feat: add sprint closeout persistence"
```

## Task 2: Snapshot Sprint Outcomes Before Rollover Mutates Task Membership

**Files:**
- Modify: `server/internal/repository/pm_sprint.go`
- Modify: `server/internal/service/pm_automation.go`
- Create: `server/internal/service/pm_automation_test.go`
- Modify: `server/internal/service/workspace_test.go`

- [ ] **Step 1: Write failing automation tests**

Add coverage for:
- a sprint closeout is created before unfinished tasks are moved
- completed tasks stay in the ended sprint and count as `completed`
- non-done tasks moved to the next sprint count as `rolled_over`
- non-done tasks with no next sprint count as `unfinished_not_rolled`
- rerunning the automation does not create duplicate closeouts or double-move tasks

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run 'TestPMAutomationService_SprintCloseouts'
```

Expected: FAIL because closeout creation is not wired into automations yet.

- [ ] **Step 2: Add small sprint lookup helpers**

In `server/internal/repository/pm_sprint.go`, add narrowly-scoped helpers:
- `ListRecentlyEndedSprints(ctx, endedAfter, endedBefore time.Time) ([]model.PMSprint, error)`
- `FindNextSprint(ctx, workspaceID string, teamID *string, after time.Time, excludeSprintID string) (*model.PMSprint, error)`

These helpers exist to support closeout creation. Do not duplicate the whole sprint listing stack.

- [ ] **Step 3: Add closeout creation helper in the automation service**

In `server/internal/service/pm_automation.go`, add a helper like:

```go
func (s *PMAutomationService) ensureSprintCloseout(
  ctx context.Context,
  endedSprint model.PMSprint,
  nextSprint *model.PMSprint,
) (*model.PMSprintCloseout, error)
```

Helper rules:
- load tasks from the ended sprint before any moves
- resolve each task’s workflow state type
- compute counts/points into the three fixed buckets
- write the closeout + child task rows through the closeout repository
- set `rolled_to_sprint_id` only when a next sprint exists and at least one unfinished task will move

- [ ] **Step 4: Wire the helper into rollover**

Update `runSprintMoveUnfinished` in `server/internal/service/pm_automation.go`:
- call `ensureSprintCloseout(...)` before `UpdateSprintID(...)`
- only move tasks whose outcome bucket is `rolled_over`
- after snapshot creation and any moves, publish `sprint` websocket `updated` events for:
  - the ended sprint
  - the receiving sprint when one exists

Do not broaden the current catch-window behavior in this plan. This remains a prospective feature, not a historical backfill.

- [ ] **Step 5: Re-run automation verification**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run 'TestPMAutomationService_SprintCloseouts'
cd /root/teampulse/server && go test ./internal/temporalapp -run 'TestSprintAutomationCronWorkflow'
```

- [ ] **Step 6: Commit**

Run:
```bash
git add server/internal/repository/pm_sprint.go server/internal/service/pm_automation.go server/internal/service/pm_automation_test.go server/internal/temporalapp/sprint_automation_workflow_test.go server/internal/service/workspace_test.go
git commit -m "feat: snapshot sprint closeouts before rollover"
```

## Task 3: Expose Closeout Read APIs

**Files:**
- Modify: `server/internal/service/pm_sprint.go`
- Modify: `server/internal/handler/pm_sprint.go`
- Modify: `server/internal/router/router.go`
- Create: `server/internal/service/pm_sprint_closeout_test.go`
- Create: `server/internal/handler/pm_sprint_closeout_test.go`
- Modify: `server/internal/service/pm_sprint_test.go`
- Modify: `server/internal/service/pm_mentions_test.go`
- Modify: `server/internal/handler/pm_sprint_planning_test.go`

- [ ] **Step 1: Write failing service and handler tests**

Add coverage for:
- `GetSprintCloseout` enforces team access on the parent sprint
- single-sprint response returns both `closeout` and `rolled_in_from`
- reports list can filter by `team_id`
- handler returns `400` without `workspace_id`
- handler returns `404` for missing sprint closeout

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run 'TestPMSprintService_Closeouts'
cd /root/teampulse/server && go test ./internal/handler -run 'TestPMSprintHandler_Closeouts'
```

Expected: FAIL because the service/handler path does not exist yet.

- [ ] **Step 2: Add sprint closeout service methods**

In `server/internal/service/pm_sprint.go`, add:
- `GetCloseout(ctx, sprintID string) (*model.SprintCloseoutResponse, error)`
- `ListCloseouts(ctx, workspaceID string, teamID *string) ([]model.SprintCloseoutListItem, error)`

Rules:
- validate workspace/team access through the parent sprint/team
- return `nil, nil` when no closeout exists so the handler can map that cleanly to `404`
- do not modify the existing `GetByID` response shape

- [ ] **Step 3: Add HTTP endpoints**

In `server/internal/handler/pm_sprint.go`, add:
- `GET /api/pm/sprints/{id}/closeout`
- `GET /api/pm/sprints/closeouts`

Register routes in `server/internal/router/router.go` in this order:
- `/sprints/closeouts`
- `/sprints/{id}`
- `/sprints/{id}/closeout`

Do not place `/sprints/{id}` before `/sprints/closeouts`, or `closeouts` will be parsed as a sprint id.

- [ ] **Step 4: Re-run backend API verification**

Run:
```bash
cd /root/teampulse/server && go test ./internal/service -run 'TestPMSprintService_Closeouts'
cd /root/teampulse/server && go test ./internal/handler -run 'TestPMSprintHandler_Closeouts'
cd /root/teampulse/server && go build ./cmd/api
```

- [ ] **Step 5: Commit**

Run:
```bash
git add server/internal/service/pm_sprint.go server/internal/handler/pm_sprint.go server/internal/router/router.go server/internal/service/pm_sprint_closeout_test.go server/internal/handler/pm_sprint_closeout_test.go server/internal/service/pm_sprint_test.go server/internal/service/pm_mentions_test.go server/internal/handler/pm_sprint_planning_test.go
git commit -m "feat: expose sprint closeout read endpoints"
```

## Task 4: Wire Frontend Types, Services, And Query Hooks

**Files:**
- Modify: `frontend/src/lib/pm-types/project.ts`
- Modify: `frontend/src/lib/services/pmSprintService.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Modify: `frontend/src/hooks/queries/useSprints.ts`
- Create: `frontend/src/hooks/queries/__tests__/useSprintCloseouts.test.tsx`

- [ ] **Step 1: Write failing frontend data-layer tests**

Add coverage for:
- `pmSprintService.getCloseout` hits `/pm/sprints/:id/closeout`
- `pmSprintService.listCloseouts` hits `/pm/sprints/closeouts`
- query hooks use stable query keys
- `team_id` filter is serialized correctly

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/hooks/queries/__tests__/useSprintCloseouts.test.tsx
```

Expected: FAIL because closeout types/services/hooks do not exist yet.

- [ ] **Step 2: Add closeout TS types**

In `frontend/src/lib/pm-types/project.ts`, add:
- `SprintCloseout`
- `SprintCloseoutTask`
- `SprintCloseoutResponse`
- `SprintInboundRolloverSummary`
- `SprintCloseoutListItem`

Match backend field names exactly using snake_case keys.

- [ ] **Step 3: Add service methods and query keys**

In `frontend/src/lib/services/pmSprintService.ts`, add:
- `getCloseout(workspaceId, sprintId)`
- `listCloseouts(workspaceId, filters?)`

In `frontend/src/lib/queryKeys.ts`, add:
- `pm.sprintCloseout(wsId, sprintId)`
- `pm.sprintCloseouts(wsId, filters?)`

Keep them under `['pm', wsId, 'sprints', ...]` so existing `sprint` websocket invalidation remains effective.

- [ ] **Step 4: Add hooks**

In `frontend/src/hooks/queries/useSprints.ts`, add:
- `useSprintCloseout(wsId, sprintId)`
- `useSprintCloseouts(wsId, filters?)`

Use `placeholderData: previous => previous` for the list hook so filter changes remain visually stable.

- [ ] **Step 5: Re-run frontend data-layer verification**

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/hooks/queries/__tests__/useSprintCloseouts.test.tsx
cd /root/teampulse/frontend && npm exec tsc --noEmit
```

- [ ] **Step 6: Commit**

Run:
```bash
git add frontend/src/lib/pm-types/project.ts frontend/src/lib/services/pmSprintService.ts frontend/src/lib/queryKeys.ts frontend/src/hooks/queries/useSprints.ts frontend/src/hooks/queries/__tests__/useSprintCloseouts.test.tsx
git commit -m "feat: add sprint closeout frontend data hooks"
```

## Task 5: Show Historical Closeout Results In Sprint Detail

**Files:**
- Create: `frontend/src/components/pm/sprints/SprintCloseoutSummary.tsx`
- Create: `frontend/src/components/pm/sprints/SprintRolledInBanner.tsx`
- Modify: `frontend/src/pages/pm/SprintDetail.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintCloseoutSummary.test.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintRolledInBanner.test.tsx`

- [ ] **Step 1: Write failing component tests**

Add coverage for:
- closed sprint summary shows committed/completed/unfinished/rolled-over cards
- receiving sprint banner shows “rolled in from Sprint X” with story/point counts
- components gracefully render nothing when no closeout data exists

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintCloseoutSummary.test.tsx src/components/pm/sprints/__tests__/SprintRolledInBanner.test.tsx
```

Expected: FAIL because the components do not exist yet.

- [ ] **Step 2: Add focused UI primitives**

Create `SprintCloseoutSummary.tsx`:
- four compact summary cards:
  - Committed
  - Completed
  - Unfinished
  - Rolled over
- include points as muted secondary text

Create `SprintRolledInBanner.tsx`:
- compact informational banner
- support 1 or more inbound source sprints
- each source sprint name should link to `/w/$slug/pm/sprints/$sprintId`

- [ ] **Step 3: Integrate into SprintDetail**

In `frontend/src/pages/pm/SprintDetail.tsx`:
- load `useSprintCloseout(workspaceId, sprintId)`
- when the sprint status is `done` and a closeout exists, render `SprintCloseoutSummary` above the story list
- when `rolled_in_from.length > 0`, render `SprintRolledInBanner` near the top of the page for the receiving sprint
- do not replace the existing task list or metadata sidebar

Keep the page additive. Do not refactor SprintDetail beyond what is required to add the two new read surfaces.

- [ ] **Step 4: Re-run sprint detail verification**

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintCloseoutSummary.test.tsx src/components/pm/sprints/__tests__/SprintRolledInBanner.test.tsx
cd /root/teampulse/frontend && npm exec tsc --noEmit
```

- [ ] **Step 5: Commit**

Run:
```bash
git add frontend/src/components/pm/sprints/SprintCloseoutSummary.tsx frontend/src/components/pm/sprints/SprintRolledInBanner.tsx frontend/src/pages/pm/SprintDetail.tsx frontend/src/components/pm/sprints/__tests__/SprintCloseoutSummary.test.tsx frontend/src/components/pm/sprints/__tests__/SprintRolledInBanner.test.tsx
git commit -m "feat: show sprint closeout history in sprint detail"
```

## Task 6: Replace The Reports Placeholder With A Closeout Table

**Files:**
- Create: `frontend/src/pages/pm/Reports.tsx`
- Create: `frontend/src/components/pm/reports/SprintCloseoutTable.tsx`
- Modify: `frontend/src/routes/_authenticated/w/$slug/pm/reports.tsx`
- Create: `frontend/src/components/pm/reports/__tests__/SprintCloseoutTable.test.tsx`

- [ ] **Step 1: Write failing reports UI tests**

Add coverage for:
- table renders sprint/team/date columns
- completion rate and rolled-over counts are shown
- empty state appears when there are no closeouts
- team filter updates the query

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/pm/reports/__tests__/SprintCloseoutTable.test.tsx
```

Expected: FAIL because the reports page/table do not exist yet.

- [ ] **Step 2: Build a lightweight reports screen**

Create `frontend/src/pages/pm/Reports.tsx` with:
- page title `Reports`
- team filter using existing workspace/team data patterns
- `useSprintCloseouts(workspaceId, filters)` query
- empty state copy:
  - “No sprint closeouts yet”
  - explain that closeouts are created when a sprint ends and rollover is evaluated

Create `SprintCloseoutTable.tsx` with columns:
- Sprint
- Team
- End date
- Committed
- Completed
- Unfinished
- Rolled over
- Completion %
- Velocity (completed points)

Keep the first version sortable only by the server’s default order (`closed_at DESC`). No client-side table framework work in this plan.

- [ ] **Step 3: Replace the placeholder route component**

In `frontend/src/routes/_authenticated/w/$slug/pm/reports.tsx`, import and render the new page component instead of the inline “Coming Soon” block.

- [ ] **Step 4: Re-run reports verification**

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/components/pm/reports/__tests__/SprintCloseoutTable.test.tsx
cd /root/teampulse/frontend && npm exec tsc --noEmit
```

- [ ] **Step 5: Commit**

Run:
```bash
git add frontend/src/pages/pm/Reports.tsx frontend/src/components/pm/reports/SprintCloseoutTable.tsx frontend/src/routes/_authenticated/w/$slug/pm/reports.tsx frontend/src/components/pm/reports/__tests__/SprintCloseoutTable.test.tsx
git commit -m "feat: add sprint closeout reports page"
```

## Final Verification

- [ ] **Step 1: Run backend verification**

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository ./internal/service ./internal/handler ./internal/temporalapp
cd /root/teampulse/server && go build ./cmd/api
```

- [ ] **Step 2: Run frontend verification**

Run:
```bash
cd /root/teampulse/frontend && npm exec vitest run src/hooks/queries/__tests__/useSprintCloseouts.test.tsx src/components/pm/sprints/__tests__/SprintCloseoutSummary.test.tsx src/components/pm/sprints/__tests__/SprintRolledInBanner.test.tsx src/components/pm/reports/__tests__/SprintCloseoutTable.test.tsx
cd /root/teampulse/frontend && npm exec tsc --noEmit
```

- [ ] **Step 3: Manual verification**

Verify in the app:
- create two adjacent sprints for a team with rollover enabled
- add done + not-done tasks into sprint A
- let sprint A end and run the automation path
- confirm:
  - sprint A shows committed/completed/unfinished/rolled-over closeout cards
  - sprint B shows a rolled-in banner
  - PM Reports shows sprint A with the correct counts

- [ ] **Step 4: Final commit**

Run:
```bash
git status --short
```

Confirm only the intended files are staged/committed. Do not use `git add .`.
