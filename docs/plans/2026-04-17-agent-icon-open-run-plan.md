# Plan — Agent icon click opens the Agent Run screen directly

Date: 2026-04-17
Owner: azhar@d4interactive.io
Branch: `feature/live-chat-events-pipeline` (or spin off)

## Problem

On a PM task card the assigned-agent bot badge (tooltip: "Forge") currently
bubbles its click up to the parent card, which opens the generic task detail
panel. When the agent is actively running, the user has to hunt for the Agent
Run section after the panel opens. Users expect: click the bot badge → jump
straight into the Agent Run screen for the live run.

Reference screenshot: bot icon at bottom-right of the task card, below an
orange "running" pill in the top-right corner.

## Current behavior (as of this commit)

- `frontend/src/components/pm/TaskCard.tsx:556` — badge is just a
  `TaskCardAgentBadge` wrapped in a `Tooltip`. No `onClick`. The outer
  `<article onClick={() => onOpen?.(task)}>` captures all clicks.
- `onOpen` resolves through
  `frontend/src/components/pm/task-detail/taskRouteNavigation.ts → openTaskRoute`
  which either navigates to `/w/$slug/pm/tasks/$taskId` or opens the overlay
  via `useTaskPanelStore`.
- `frontend/src/components/pm/AgentRunPanel.tsx` (rendered inside
  `TaskDetailPanel.tsx:1316`) already consumes `?run=<id>` via `useSearch` and
  auto-opens the run drawer. So the primitive for "open a specific run" is
  already there — we just need to route into it.
- Run status vocab is in
  `frontend/src/components/pm/agentRunConstants.ts`:
  `ACTIVE_RUN_STATUSES = {queued, running, paused}` (plus paused-variants
  `awaiting_input`, `awaiting_approval`, `awaiting_auth`).
- Task model: `PMTask.AssignedAgentID *string` only — no knowledge of whether
  a run is currently active. `agent_runs` rows reference the task via
  `(target_type='task', target_id=task.id)`.

## Desired behavior

1. Click bot badge on a card when an active run exists
   (`queued`/`running`/`paused` or any `paused` pause-reason) →
   navigate to `/w/$slug/pm/tasks/$taskId?run=<latest_active_run_id>`.
   This renders the task detail with the Agent Run drawer already open.
2. Click bot badge when no active run exists → open task detail (current
   behavior), but scroll `AgentRunPanel` into view so the user can start
   a run. (Optional second-step nicety, not required for v1.)
3. Click anywhere else on the card → unchanged (opens task detail).

## Approach

### Backend — expose latest active run on the task payload

File: `server/internal/model/pm_task.go`

Add two computed (`gorm:"-"`) fields to `PMTask`:

```go
LatestRunID     *string `json:"latest_run_id,omitempty" gorm:"-"`
LatestRunStatus *string `json:"latest_run_status,omitempty" gorm:"-"`
```

Keep the shape minimal — one id + one status string is enough for the card to
decide whether to route to the drawer. The full run object is fetched later by
`AgentRunPanel`.

File: `server/internal/repository/pm_task_repository.go` (and any list loader
that produces cards — e.g. board, sprint, My Work, epic detail task list)

Attach the latest non-terminal run in a single query per batch (no N+1):

```sql
SELECT DISTINCT ON (target_id) target_id, id, status
FROM agent_runs
WHERE target_type = 'task'
  AND target_id IN (:task_ids)
  AND status IN ('queued','running','paused')
ORDER BY target_id, started_at DESC NULLS LAST, created_at DESC;
```

Populate `LatestRunID` / `LatestRunStatus` on each task before returning.
Mirror this in the single-task detail loader.

**No new API endpoint needed.** `GET /api/pm/agent-runs?target_type=task&target_id=...`
still serves the full run list for the panel.

### Frontend — clickable agent badge + route with `?run=`

#### 1. Task type

`frontend/src/lib/pmTypes.ts` — extend `Task`:

```ts
latest_run_id?: string | null;
latest_run_status?: string | null;
```

#### 2. Navigation helper

`frontend/src/components/pm/task-detail/taskRouteNavigation.ts` —
extend `openTaskRoute` to accept optional search params:

```ts
export function openTaskRoute(
  navigate: TaskRouteNavigate,
  location: TaskOverlayLocationLike,
  slug: string,
  taskId: string,
  opts?: { run?: string; team?: string },
)
```

When `opts.run` is present, always go through the route form
(`/w/$slug/pm/tasks/$taskId?run=<id>`) rather than the overlay store — the
panel's `AgentRunPanel` reads the `run` param from `useSearch`, so URL must be
the source of truth. When no `run`, keep the existing overlay-vs-route logic.

#### 3. TaskCard badge becomes a real button

`frontend/src/components/pm/TaskCard.tsx` (~L556)

- Promote the badge wrapper from `<span>` to `<button type="button">` with
  `onClick` and `onKeyDown` handlers that call `e.stopPropagation()` (so the
  outer article click does not also fire) and `e.preventDefault()` on
  `pointerDown` to keep dnd-kit from starting a drag (mirror the existing
  pattern used by `MemberPickerPopover`).
- Introduce a new context callback `onOpenAgentRun?: (task: Task) => void`
  alongside the existing `onOpen` (add to `BoardCallbacksContext`).
- The badge only invokes `onOpenAgentRun`. Parent decides whether to jump to
  the drawer or fall through.

#### 4. Wire callback in every TaskCard host

Apply the same wiring at each render site:

- `frontend/src/components/pm/KanbanBoard.tsx` (boardCallbacks memo + dnd
  overlay card — disable the button when `isOverlay` so a drag preview is
  not interactive).
- `frontend/src/pages/pm/MyWork.tsx`
- `frontend/src/pages/pm/EpicDetail.tsx`
- `frontend/src/components/pm/sprints/SprintPlanningTaskCard.tsx`
- Any other host surfaced by `rg "<TaskCard" frontend/src`.

Each host defines:

```tsx
const openAgentRun = useCallback((task: Task) => {
  const isActive = task.latest_run_id
    && ACTIVE_RUN_STATUSES.has(task.latest_run_status ?? '');
  openTaskRoute(
    navigate as never,
    location as never,
    slug,
    task.id,
    isActive ? { run: task.latest_run_id! } : undefined,
  );
}, [navigate, location, slug]);
```

Reuse `ACTIVE_RUN_STATUSES` from `agentRunConstants.ts`.

#### 5. Tooltip copy (small polish)

When `latest_run_status` is active, change the badge tooltip from
`assignedAgent?.name ?? 'Agent assigned'` to
`` `${agent.name} — ${STATUS_META[status].label} · Open run` ``
so the behavior is discoverable.

## Corner cases to cover

1. **No active run on an assigned-agent task** → `onOpenAgentRun` opens the
   task detail without `?run=`. Same destination as a generic card click.
2. **Paused / awaiting_input / awaiting_approval / awaiting_auth** → all in
   `ACTIVE_RUN_STATUSES` → jump to drawer; user lands on the approval/input
   prompt that needs them.
3. **Terminal states (completed / failed / cancelled)** → do NOT route to
   drawer. Optionally v2: show last run summary — out of scope here.
4. **Multiple concurrent active runs** → repository picks the newest by
   `started_at DESC NULLS LAST, created_at DESC`. Document in the SQL.
5. **Drag-and-drop interference** → the badge button must
   `stopPropagation()` on `pointerdown`/`mousedown`/`click` so dnd-kit
   listeners on the `<article>` don't interpret a tap as the start of a
   drag. Gate interactivity on `!isOverlay` / `!isDragging`.
6. **DragOverlayCard preview** (`KanbanBoard.tsx:439`) — render the badge
   as a non-interactive span there; the preview should never fire clicks.
7. **Keyboard access** — `<button type="button">` natively supports
   Enter/Space; ensure `e.stopPropagation()` in `onKeyDown` so the parent
   article's Enter/Space handler does not also fire.
8. **Agent unassigned** → badge is not rendered at all
   (`vis.agent && task.assigned_agent_id` guard). No behavior change.
9. **Permissions** — viewing runs is already allowed for any member with
   workspace access (AgentRunPanel is shown in TaskDetailPanel without
   extra perm checks). No gating changes.
10. **Realtime staleness** — when the live run transitions to terminal, the
    card's `latest_run_id`/`latest_run_status` would be stale. Extend
    `useRealtimeSync` to invalidate the PM task list query on
    `agent_run-updated` / `agent_run-created` events (check it already
    invalidates `queryKeys.pm.tasks.*` — add if missing).
11. **Route already open on that task** — `openTaskRoute` in its current
    form navigates again; with the `opts.run` branch it should still emit
    a new navigation so TanStack picks up the search param change. Verify
    `replace: false` (default) to keep back-button working, OR use
    `replace: true` for in-place parameter swap. Recommend `replace: false`
    so users can back out to "task without run".
12. **Task detail overlay mode** — when a card on a board is clicked, the
    overlay store opens without changing URL; but when `?run=` is needed we
    must switch to the route form (see §2). Overlay state is discarded and
    the route state takes over — acceptable because the overlay *is* the
    task panel.
13. **Stale badge tooltip text** — tooltip is memoized on render of status;
    since TaskCard re-renders on `task.updated_at` change (memo compare),
    ensure `latest_run_status` is part of what triggers re-render (the
    backend should bump `updated_at` whenever a run status changes, OR add
    `prev.task.latest_run_status !== next.task.latest_run_status` to the
    memo predicate at `TaskCard.tsx:end`).
14. **Sprint planning / kanban list view / "list" mode** — same badge
    appears; same callback applies. Confirm list-mode renderer uses the
    same `TaskCard`.
15. **Mobile / touch** — long-press on the card is drag; a short tap on
    the badge must not trigger drag. Covered by the same
    `stopPropagation` + drag-gate.
16. **Server responses that predate the new fields** — `latest_run_id` is
    optional on the TS type, so older frontends / cached responses without
    the field keep working (badge just never routes to drawer).

## Tests

Frontend (vitest, `frontend/src/components/pm/__tests__/`):

- `TaskCard.agent-badge.test.tsx`
  - Active `latest_run_id` + active status: clicking badge calls
    `onOpenAgentRun` with the task; does NOT call `onOpen`.
  - No `latest_run_id`: clicking badge still calls `onOpenAgentRun` (host
    decides); host test asserts `openTaskRoute` called without `run`.
  - Terminal status (`completed`): treated like no active run.
  - Enter keydown on focused badge follows same branches.
  - Badge renders as non-interactive in `isOverlay` mode.

Backend (Go):

- `pm_task_repository_test.go` — list endpoint returns
  `latest_run_id` + `latest_run_status` only when an active run exists;
  filters out terminal runs; picks newest when multiple active.

## Rollout

1. Merge backend field additions first (safe: fields are optional on
   payload; zero impact if frontend ignores them).
2. Merge frontend badge/route change.
3. Cut a short Loom showing: board → click bot → land in Agent Run drawer
   mid-run. Share in #product.

## Out of scope (future)

- Scrolling to AgentRunPanel when no active run exists.
- Surface a "history" indicator (e.g. green check) for the most recent
  terminal run.
- Same treatment for CRM / support cards that show agent avatars.

## Files touched (summary)

Backend:
- `server/internal/model/pm_task.go`
- `server/internal/repository/pm_task_repository.go` (and siblings for
  list loaders — board, sprint, my-work, epic detail)
- `server/internal/repository/pm_task_repository_test.go`

Frontend:
- `frontend/src/lib/pmTypes.ts`
- `frontend/src/components/pm/task-detail/taskRouteNavigation.ts`
- `frontend/src/components/pm/TaskCard.tsx`
- `frontend/src/components/pm/KanbanBoard.tsx`
- `frontend/src/components/pm/sprints/SprintPlanningTaskCard.tsx`
- `frontend/src/pages/pm/MyWork.tsx`
- `frontend/src/pages/pm/EpicDetail.tsx`
- `frontend/src/hooks/useRealtimeSync.ts` (if invalidation missing)
- `frontend/src/components/pm/__tests__/TaskCard.agent-badge.test.tsx`
