# PM Sprint Planning Workspace Implementation Plan

This historical plan explains the sprint planning workspace: view sprint contents, browse backlog work, and move tasks without leaving the page. The feature exists; “story” in the original examples is older terminology for today's task model.

## Source review: September 18, 2026

- [Current DTOs](../../server/internal/model/pm_sprint.go) use `SprintPlanningTaskPreview`, `preview_tasks`, `task_preview_overflow`, and `backlog_tasks`. The proposed story-named JSON examples are not the current wire contract.
- [The planning repository](../../server/internal/repository/pm_sprint.go) filters workspace/team access, excludes archived sprints, and batches statistics and previews using a per-sprint window query. Defaults are twenty preview tasks per sprint and fifty backlog tasks. The sprint set itself is loaded before grouping; “bounded preview” does not mean the entire planning response has a fixed sprint count.
- [Current routes](../../server/internal/router/router.go) include `GET /api/pm/sprints/planning` and the dedicated `GET /api/pm/sprints/backlog-tasks` under `pm.read`. The [client service](../../frontend/src/lib/services/pmSprintService.ts) and query hooks support backlog paging rather than the proposed `pmStoryService` fallback.
- [The Sprints page](../../frontend/src/pages/pm/Sprints.tsx) performs optimistic assignment through `pmTaskService.update`, clears membership with an empty `sprint_id`, restores caches on failure, and invalidates related task/board/sprint queries. [The workspace component](../../frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx) supplies drag-and-drop and the backlog panel; its card is `SprintPlanningTaskCard`, not the proposed story component.
- Rollover and historical reporting subsequently gained their own implementation, described in the [closeout plan](2026-04-13-pm-sprint-closeouts-rollover-reporting.md). Their original placement in this plan's follow-up list does not mean they are still wholly unimplemented.

Commands below assume the repository root; developer-specific paths were removed. [Go module requirements](../../server/go.mod) now declare Go 1.25.0. This review did not rerun application tests or browser drag-and-drop checks, and the unchecked historical steps do not represent a current backlog.

## Original implementation plan

**Goal:** Turn the top-level Sprint page into a real planning workspace with grouped sprint columns, visible sprint contents, and a backlog side panel for assigning stories without leaving the page.

**Architecture:** Keep the existing sprint detail page for deep editing, but replace the top-level sprint index with a dedicated planning workspace. Add one backend planning read model so the page can load grouped sprints plus preview stories efficiently, then layer a focused frontend workspace on top of it using existing story update endpoints for assignment. Reuse existing PM types, query hooks, and dnd-kit patterns instead of inventing a second planning system.

**Tech Stack:** React 19, TypeScript 5.9, TanStack Query, TanStack Router, dnd-kit, Go 1.24, Chi, GORM, PostgreSQL, Vitest

---

## Decision Summary

- The current sprint detail page stays in place as the deep-dive/editor view.
- The current top-level sprint page becomes the planning surface.
- The planning surface shows grouped sprint columns: `Active`, `Upcoming`, `Completed`.
- Each sprint column shows real story previews, not just aggregate counts.
- A right-side backlog panel shows unassigned stories and supports assigning them to sprints.
- Story assignment/removal reuses the existing story update path (`sprint_id`) wherever possible.
- The first version supports moving stories between backlog and visible sprints; bulk carry-forward and velocity/capacity forecasting come later.
- Engineering-heavy signals like points remain visible, but the UI must still work for teams that mostly think in story count rather than points.

## File Structure

### Backend

- Modify: `server/internal/model/pm_sprint.go`
  - Add planning DTOs for grouped sprint workspace responses.
- Modify: `server/internal/repository/pm_sprint.go`
  - Add a planning query that returns visible sprints plus preview stories/counts without N+1 fan-out.
- Modify: `server/internal/service/pm_sprint.go`
  - Add a planning workspace method with access checks and filter normalization.
- Modify: `server/internal/handler/pm_sprint.go`
  - Add the planning endpoint.
- Modify: `server/internal/router/router.go`
  - Register the new endpoint.
- Create: `server/internal/repository/pm_sprint_planning_test.go`
  - Repository coverage for grouping, preview stories, and backlog selection.
- Create: `server/internal/service/pm_sprint_planning_test.go`
  - Service coverage for access, grouping, and filters.
- Create: `server/internal/handler/pm_sprint_planning_test.go`
  - Handler coverage for query params and response shape.

### Frontend

- Modify: `frontend/src/lib/pmTypes.ts`
  - Add types for the sprint planning workspace response.
- Modify: `frontend/src/lib/services/pmSprintService.ts`
  - Add a `planningWorkspace` fetcher.
- Modify: `frontend/src/lib/queryKeys.ts`
  - Add query keys for sprint planning workspace and backlog filters.
- Modify: `frontend/src/hooks/queries/useSprints.ts`
  - Add a `useSprintPlanningWorkspace` hook.
- Modify: `frontend/src/pages/pm/Sprints.tsx`
  - Replace the list-first page with the planning workspace shell.
- Create: `frontend/src/components/pm/sprints/SprintPlanningFilters.tsx`
  - Top bar filters and view controls.
- Create: `frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx`
  - Main grouped workspace shell.
- Create: `frontend/src/components/pm/sprints/SprintPlanningGroup.tsx`
  - Group sections for Active / Upcoming / Completed.
- Create: `frontend/src/components/pm/sprints/SprintPlanningColumn.tsx`
  - Individual sprint columns with headers, stats, and story previews.
- Create: `frontend/src/components/pm/sprints/SprintPlanningStoryCard.tsx`
  - Compact story card reused in columns/backlog.
- Create: `frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx`
  - Right-side backlog side panel with filters and assignment affordances.
- Create: `frontend/src/components/pm/sprints/SprintPlanningEmptyState.tsx`
  - Empty states for no sprints and no stories.
- Create: `frontend/src/components/pm/sprints/__tests__/SprintPlanningWorkspace.test.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintPlanningBacklogPanel.test.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintPlanningColumn.test.tsx`
- Modify: `frontend/src/hooks/queries/useStories.ts`
  - Reuse or extend the story list hook for unassigned sprint filtering if needed.
- Modify: `frontend/src/lib/services/pmStoryService.ts`
  - Support an explicit “no sprint” filter if the backend/story list path is reused for the backlog panel.

## Task 1: Add A Dedicated Sprint Planning Read Model

**Files:**
- Modify: `server/internal/model/pm_sprint.go`
- Modify: `server/internal/repository/pm_sprint.go`
- Create: `server/internal/repository/pm_sprint_planning_test.go`

- [ ] **Step 1: Write failing repository tests for the planning response**

Add coverage for:
- grouped sprint ordering by status/time bucket:
  - active sprints first
  - upcoming next
  - completed last
- each sprint includes a preview slice of stories ordered consistently
- each sprint includes summary stats already used by the UI
- the query can return unassigned/backlog stories without treating empty `sprint_id` as “ignore filter”
- archived sprints are excluded by default
- optional team/state filters narrow the response correctly

Run:
```bash
cd server && go test ./internal/repository -run 'TestPMSprintPlanningRepository'
```

Expected: FAIL because no planning DTO/query exists yet.

- [ ] **Step 2: Define focused planning DTOs**

In `server/internal/model/pm_sprint.go`, add DTOs similar to:

```go
type SprintPlanningStoryPreview struct {
	ID              string  `json:"id"`
	DisplayID       int     `json:"display_id"`
	Name            string  `json:"name"`
	WorkflowStateID string  `json:"workflow_state_id"`
	StateName       *string `json:"state_name,omitempty"`
	StateType       *string `json:"state_type,omitempty"`
	OwnerMemberID   *string `json:"owner_member_id,omitempty"`
	Estimate        *int    `json:"estimate,omitempty"`
	Priority        string  `json:"priority"`
}

type SprintPlanningBucket struct {
	Key     string            `json:"key"`
	Label   string            `json:"label"`
	Sprints []SprintPlanningCard `json:"sprints"`
}

type SprintPlanningCard struct {
	Sprint         PMSprint            `json:"sprint"`
	Stats          PMSprintStats       `json:"stats"`
	PreviewStories []SprintPlanningStoryPreview `json:"preview_stories"`
	StoryPreviewOverflow int           `json:"story_preview_overflow"`
}

type SprintPlanningWorkspace struct {
	Buckets        []SprintPlanningBucket        `json:"buckets"`
	BacklogStories []SprintPlanningStoryPreview `json:"backlog_stories"`
	BacklogTotal   int                          `json:"backlog_total"`
}
```

Keep this read model purpose-built for the planning page. Do not overload `SprintWithStats`.

- [ ] **Step 3: Implement the repository planning query**

In `server/internal/repository/pm_sprint.go`:
- add a `ListPlanningWorkspace(...)` method
- reuse existing sprint status logic for bucket classification
- fetch visible sprints once
- fetch preview stories in one bounded query per visible sprint set, not one call per sprint
- include an explicit backlog selection (`sprint_id IS NULL`) instead of abusing empty-string filters
- keep the first version scoped to visible sprint preview only; avoid full story hydration

- [ ] **Step 4: Re-run repository verification**

Run:
```bash
cd server && go test ./internal/repository -run 'TestPMSprintPlanningRepository'
```

- [ ] **Step 5: Commit**

Run:
```bash
git add server/internal/model/pm_sprint.go server/internal/repository/pm_sprint.go server/internal/repository/pm_sprint_planning_test.go
git commit -m "feat: add sprint planning read model"
```

## Task 2: Expose The Planning Workspace API

**Files:**
- Modify: `server/internal/service/pm_sprint.go`
- Modify: `server/internal/handler/pm_sprint.go`
- Modify: `server/internal/router/router.go`
- Create: `server/internal/service/pm_sprint_planning_test.go`
- Create: `server/internal/handler/pm_sprint_planning_test.go`

- [ ] **Step 1: Write failing service and handler tests**

Add coverage for:
- workspace access and team access are enforced
- team filter is honored
- bucket labels are stable (`active`, `upcoming`, `completed`)
- backlog filtering only returns unassigned stories
- handler returns `400` when `workspace_id` is missing
- handler returns `200` with the planning payload shape

Run:
```bash
cd server && go test ./internal/service -run 'TestPMSprintService_ListPlanningWorkspace'
cd server && go test ./internal/handler -run 'TestPMSprintHandler_ListPlanningWorkspace'
```

Expected: FAIL because the service/handler path does not exist yet.

- [ ] **Step 2: Add the planning service method**

In `server/internal/service/pm_sprint.go`:
- add `ListPlanningWorkspace(ctx, workspaceID string, filters ...)`
- normalize filters and accessible team IDs
- call the new repository method
- keep the method read-only and side-effect free

- [ ] **Step 3: Add the planning endpoint**

In `server/internal/handler/pm_sprint.go`, add:

```go
// GET /api/pm/sprints/planning?workspace_id=...&team_id=...&include_completed=true
func (h *PMSprintHandler) PlanningWorkspace(w http.ResponseWriter, r *http.Request)
```

In `server/internal/router/router.go`, register it near the existing sprint routes under `pm.read`.

- [ ] **Step 4: Re-run backend API verification**

Run:
```bash
cd server && go test ./internal/service -run 'TestPMSprintService_ListPlanningWorkspace'
cd server && go test ./internal/handler -run 'TestPMSprintHandler_ListPlanningWorkspace'
cd server && go build ./cmd/api
```

- [ ] **Step 5: Commit**

Run:
```bash
git add server/internal/service/pm_sprint.go server/internal/handler/pm_sprint.go server/internal/router/router.go server/internal/service/pm_sprint_planning_test.go server/internal/handler/pm_sprint_planning_test.go
git commit -m "feat: expose sprint planning workspace endpoint"
```

## Task 3: Wire Frontend Data Contracts And Query Hooks

**Files:**
- Modify: `frontend/src/lib/pmTypes.ts`
- Modify: `frontend/src/lib/services/pmSprintService.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Modify: `frontend/src/hooks/queries/useSprints.ts`
- Create: `frontend/src/hooks/queries/__tests__/useSprintPlanningWorkspace.test.tsx`

- [ ] **Step 1: Write failing frontend data-layer tests**

Add coverage for:
- the service hits `/pm/sprints/planning`
- team/status filters are serialized correctly
- the query hook uses a stable query key and unwraps API errors

Run:
```bash
cd frontend && npm exec vitest run src/hooks/queries/__tests__/useSprintPlanningWorkspace.test.tsx
```

Expected: FAIL because the service and hook do not exist yet.

- [ ] **Step 2: Add TS types and service wiring**

In `frontend/src/lib/pmTypes.ts`, add exact frontend shapes matching the backend DTOs.

In `frontend/src/lib/services/pmSprintService.ts`, add:

```ts
planningWorkspace: (
  workspaceId: string,
  filters?: {
    team_id?: string
    include_completed?: boolean
  }
) => api.get<SprintPlanningWorkspace>(...)
```

Keep this separate from the existing `list()` API.

- [ ] **Step 3: Add query keys and hook**

In `frontend/src/lib/queryKeys.ts`, add a dedicated key:

```ts
planningWorkspace: (wsId: string, filters?: Record<string, unknown>) =>
  ['pm', wsId, 'sprints', 'planning', filters] as const
```

In `frontend/src/hooks/queries/useSprints.ts`, add:
- `useSprintPlanningWorkspace`

- [ ] **Step 4: Re-run frontend data verification**

Run:
```bash
cd frontend && npm exec vitest run src/hooks/queries/__tests__/useSprintPlanningWorkspace.test.tsx
cd frontend && npm exec tsc --noEmit
```

- [ ] **Step 5: Commit**

Run:
```bash
git add frontend/src/lib/pmTypes.ts frontend/src/lib/services/pmSprintService.ts frontend/src/lib/queryKeys.ts frontend/src/hooks/queries/useSprints.ts frontend/src/hooks/queries/__tests__/useSprintPlanningWorkspace.test.tsx
git commit -m "feat: add sprint planning workspace query hook"
```

## Task 4: Replace The Sprint Index With A Planning Workspace Shell

**Files:**
- Modify: `frontend/src/pages/pm/Sprints.tsx`
- Create: `frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx`
- Create: `frontend/src/components/pm/sprints/SprintPlanningFilters.tsx`
- Create: `frontend/src/components/pm/sprints/SprintPlanningEmptyState.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintPlanningWorkspace.test.tsx`

- [ ] **Step 1: Write failing component tests for the page shell**

Add coverage for:
- grouped sections render for `Active`, `Upcoming`, and `Completed`
- the page still supports create sprint
- loading and error states render cleanly
- empty planning state renders a planning-specific CTA instead of the old listing copy

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningWorkspace.test.tsx
```

Expected: FAIL because the new planning components do not exist yet.

- [ ] **Step 2: Build the workspace shell**

In `frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx`:
- render the page title/description
- render grouped sprint buckets horizontally
- reserve a right-side slot for backlog
- keep the layout usable on narrower screens with horizontal scroll, not compressed cards

In `frontend/src/components/pm/sprints/SprintPlanningFilters.tsx`:
- include at minimum:
  - team filter
  - active/upcoming/completed visibility control
  - create sprint action

In `frontend/src/pages/pm/Sprints.tsx`:
- replace the cards/table-first experience with the new workspace shell
- keep route-level permissions and create modal behavior intact

- [ ] **Step 3: Re-run shell verification**

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningWorkspace.test.tsx
cd frontend && npm exec tsc --noEmit
```

- [ ] **Step 4: Commit**

Run:
```bash
git add frontend/src/pages/pm/Sprints.tsx frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx frontend/src/components/pm/sprints/SprintPlanningFilters.tsx frontend/src/components/pm/sprints/SprintPlanningEmptyState.tsx frontend/src/components/pm/sprints/__tests__/SprintPlanningWorkspace.test.tsx
git commit -m "feat: replace sprint index with planning workspace shell"
```

## Task 5: Add Rich Sprint Columns With Story Preview

**Files:**
- Create: `frontend/src/components/pm/sprints/SprintPlanningGroup.tsx`
- Create: `frontend/src/components/pm/sprints/SprintPlanningColumn.tsx`
- Create: `frontend/src/components/pm/sprints/SprintPlanningStoryCard.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintPlanningColumn.test.tsx`

- [ ] **Step 1: Write failing tests for sprint columns**

Add coverage for:
- sprint header shows name, date range, team, and status
- stats show progress plus count/points without overwhelming the column
- preview stories render compactly and open the story panel on click
- overflow indicator shows when more stories exist than the preview limit
- empty sprint columns show a useful CTA

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningColumn.test.tsx
```

Expected: FAIL because the column/story preview components do not exist yet.

- [ ] **Step 2: Implement compact planning columns**

In `SprintPlanningColumn.tsx`:
- build a header that answers:
  - what sprint is this?
  - when is it?
  - how full is it?
  - how is it progressing?
- show preview stories using a compact reusable card
- include a small CTA:
  - `Add story`
  - or `Create story in sprint`

In `SprintPlanningStoryCard.tsx`:
- render:
  - display id
  - title
  - status/state
  - owner avatar if present
  - estimate/priority only if space allows
- make the card reusable in both sprint columns and backlog panel

- [ ] **Step 3: Re-run column verification**

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningColumn.test.tsx
cd frontend && npm exec tsc --noEmit
```

- [ ] **Step 4: Commit**

Run:
```bash
git add frontend/src/components/pm/sprints/SprintPlanningGroup.tsx frontend/src/components/pm/sprints/SprintPlanningColumn.tsx frontend/src/components/pm/sprints/SprintPlanningStoryCard.tsx frontend/src/components/pm/sprints/__tests__/SprintPlanningColumn.test.tsx
git commit -m "feat: add sprint planning columns with story preview"
```

## Task 6: Add The Backlog Side Panel

**Files:**
- Create: `frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintPlanningBacklogPanel.test.tsx`
- Modify: `frontend/src/hooks/queries/useStories.ts`
- Modify: `frontend/src/lib/services/pmStoryService.ts`

- [ ] **Step 1: Write failing tests for the backlog panel**

Add coverage for:
- backlog panel renders unassigned stories
- backlog filters narrow the visible list
- assigning a story to a sprint removes it from backlog optimistically
- removing a story from a sprint adds it back to backlog optimistically

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningBacklogPanel.test.tsx
```

Expected: FAIL because the backlog panel and explicit unassigned-story handling do not exist yet.

- [ ] **Step 2: Support explicit unassigned story loading**

If the new planning endpoint already returns backlog preview stories plus total, use that for the first render.

If deeper backlog paging/filtering is needed:
- add an explicit unassigned-sprint filter to `pmStoryService.list`
- avoid relying on `sprint_id=''`
- use a clear param such as `sprint_scope=none`

Keep the change small and explicit so existing story queries are not broken.

- [ ] **Step 3: Implement the backlog panel**

In `SprintPlanningBacklogPanel.tsx`:
- render a collapsible right-side panel
- include filters:
  - assignee
  - label
  - priority
  - maybe epic in v1 if already easy to source
- show compact story cards
- include quick actions:
  - assign to sprint
  - open story
  - create story

- [ ] **Step 4: Re-run backlog verification**

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningBacklogPanel.test.tsx
cd frontend && npm exec tsc --noEmit
```

- [ ] **Step 5: Commit**

Run:
```bash
git add frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx frontend/src/components/pm/sprints/__tests__/SprintPlanningBacklogPanel.test.tsx frontend/src/hooks/queries/useStories.ts frontend/src/lib/services/pmStoryService.ts
git commit -m "feat: add sprint planning backlog panel"
```

## Task 7: Add Drag/Drop Story Assignment Between Backlog And Sprints

**Files:**
- Modify: `frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx`
- Modify: `frontend/src/components/pm/sprints/SprintPlanningColumn.tsx`
- Modify: `frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx`
- Create: `frontend/src/components/pm/sprints/__tests__/SprintPlanningDnD.test.tsx`

- [ ] **Step 1: Write failing DnD tests**

Add coverage for:
- backlog story dropped into a sprint updates that story’s `sprint_id`
- story dropped from one sprint to another updates `sprint_id`
- story removed from sprint back to backlog clears `sprint_id`
- optimistic UI updates without full-page flicker
- error path reverts the optimistic move and shows a toast

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningDnD.test.tsx
```

Expected: FAIL because DnD wiring does not exist yet.

- [ ] **Step 2: Implement sprint/backlog drag-and-drop**

Reuse existing dnd-kit patterns already used elsewhere in the PM module.

Keep the first version constrained:
- draggable story cards
- droppable sprint columns
- droppable backlog panel
- use the existing story update mutation with `sprint_id`

Do not introduce bulk carry-forward yet.

- [ ] **Step 3: Re-run DnD verification**

Run:
```bash
cd frontend && npm exec vitest run src/components/pm/sprints/__tests__/SprintPlanningDnD.test.tsx
cd frontend && npm exec tsc --noEmit
```

- [ ] **Step 4: Commit**

Run:
```bash
git add frontend/src/components/pm/sprints/SprintPlanningWorkspace.tsx frontend/src/components/pm/sprints/SprintPlanningColumn.tsx frontend/src/components/pm/sprints/SprintPlanningBacklogPanel.tsx frontend/src/components/pm/sprints/__tests__/SprintPlanningDnD.test.tsx
git commit -m "feat: support sprint planning drag and drop"
```

## Task 8: Preserve The Existing Sprint Detail Flow

**Files:**
- Modify: `frontend/src/pages/pm/SprintDetail.tsx`
- Modify: `frontend/src/components/search/SearchCommandPalette.tsx`
- Modify: `frontend/src/components/layout/Header.tsx`
- Create: `frontend/src/pages/pm/__tests__/SprintNavigation.test.tsx`

- [ ] **Step 1: Write failing navigation tests**

Add coverage for:
- opening a sprint column still navigates to the existing sprint detail page
- search results still deep-link into sprint detail
- the new planning page does not break existing breadcrumbs/header behavior

Run:
```bash
cd frontend && npm exec vitest run src/pages/pm/__tests__/SprintNavigation.test.tsx
```

Expected: FAIL if navigation assumptions are hard-coded to the old list view.

- [ ] **Step 2: Adjust detail-entry affordances only as needed**

Keep the detail page intact.

Only make small changes if needed:
- ensure clicking a sprint header or “open sprint” affordance still goes to `SprintDetail`
- keep search palette and breadcrumbs consistent with the new planning page

- [ ] **Step 3: Re-run navigation verification**

Run:
```bash
cd frontend && npm exec vitest run src/pages/pm/__tests__/SprintNavigation.test.tsx
cd frontend && npm exec tsc --noEmit
```

- [ ] **Step 4: Commit**

Run:
```bash
git add frontend/src/pages/pm/SprintDetail.tsx frontend/src/components/search/SearchCommandPalette.tsx frontend/src/components/layout/Header.tsx frontend/src/pages/pm/__tests__/SprintNavigation.test.tsx
git commit -m "fix: preserve sprint detail navigation from planning workspace"
```

## Task 9: Final Verification And Product QA

**Files:**
- Verify only

- [ ] **Step 1: Run focused automated verification**

Run:
```bash
cd server && go test ./internal/repository -run 'TestPMSprintPlanningRepository'
cd server && go test ./internal/service -run 'TestPMSprintService_ListPlanningWorkspace'
cd server && go test ./internal/handler -run 'TestPMSprintHandler_ListPlanningWorkspace'
cd frontend && npm exec vitest run src/hooks/queries/__tests__/useSprintPlanningWorkspace.test.tsx src/components/pm/sprints/__tests__/SprintPlanningWorkspace.test.tsx src/components/pm/sprints/__tests__/SprintPlanningColumn.test.tsx src/components/pm/sprints/__tests__/SprintPlanningBacklogPanel.test.tsx src/components/pm/sprints/__tests__/SprintPlanningDnD.test.tsx src/pages/pm/__tests__/SprintNavigation.test.tsx
```

- [ ] **Step 2: Run broader verification**

Run:
```bash
cd server && go build ./cmd/api
cd frontend && npm exec tsc --noEmit
cd frontend && npm run build
```

- [ ] **Step 3: Manual QA checklist**

Verify all of the following:
- the Sprint page opens as a planning workspace, not a static card catalog
- sprints are grouped into Active / Upcoming / Completed
- each visible sprint shows real stories, progress, and enough context to plan
- the backlog side panel shows unassigned stories and can be filtered
- dragging backlog stories into a sprint works
- dragging a story from one sprint to another works
- removing a story back to backlog works
- opening a sprint still reaches the existing detail page
- create sprint still works
- create story in sprint still works
- the page remains usable on laptop width with horizontal scrolling instead of crushed columns

- [ ] **Step 4: Final commit**

Run:
```bash
git add server/internal/model/pm_sprint.go server/internal/repository/pm_sprint.go server/internal/service/pm_sprint.go server/internal/handler/pm_sprint.go server/internal/router/router.go server/internal/repository/pm_sprint_planning_test.go server/internal/service/pm_sprint_planning_test.go server/internal/handler/pm_sprint_planning_test.go frontend/src/lib/pmTypes.ts frontend/src/lib/services/pmSprintService.ts frontend/src/lib/queryKeys.ts frontend/src/hooks/queries/useSprints.ts frontend/src/hooks/queries/useStories.ts frontend/src/pages/pm/Sprints.tsx frontend/src/pages/pm/SprintDetail.tsx frontend/src/components/pm/sprints frontend/src/components/search/SearchCommandPalette.tsx frontend/src/components/layout/Header.tsx frontend/src/pages/pm/__tests__/SprintNavigation.test.tsx
git commit -m "feat: turn sprints into a planning workspace"
```

## Follow-Up After This Plan

These are intentionally out of scope for the first pass:
- sprint capacity forecasting based on historical completion
- bulk carry-forward / rollover actions
- auto-create next sprint from cadence rules
- multi-select story assignment
- sprint health charts on the planning page

Those should be separate follow-up specs/plans once the planning workspace is stable and adopted.
