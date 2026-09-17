# Sprint board ordering implementation plan

This historical plan explains the ordering decisions for contributors maintaining
the task board. Read the current implementation notes before using its original
steps; unchecked boxes do not establish that a feature is missing.

> Source review: 2026-09-18. Core ordering behavior exists. The original commands,
> story filenames, toolchain versions, and commit steps are historical, not a
> current execution checklist. No runtime tests or multi-session browser QA were
> performed for this documentation review.

## Current implementation and limits

- The [task repository](../../server/internal/repository/pm_task.go) sorts Done by
  `COALESCE(completed_at, moved_at, updated_at) DESC`, then `updated_at DESC` and
  `position ASC`. Position normalization counts non-archived tasks across the
  workspace/state, excluding the moved task; it is not scoped to the visible
  sprint or filtered board. A missing requested position appends to that slice;
  oversized positions are clamped. Sibling normalization and moves use transactions.
- The [task service](../../server/internal/service/pm_task.go) publishes `reordered`
  for reorders. [Realtime sync](../../frontend/src/hooks/useRealtimeSync.ts)
  schedules a debounced board refresh for both `moved` and `reordered` events.
- The [board store](../../frontend/src/stores/pmBoardStore.ts) reindexes loaded
  siblings for optimistic changes and makes same-column Done reorders a no-op.
  Cross-state moves still send `position: toIndex`, including moves into Done;
  the proposal to omit that field was not implemented. Done display order is
  still based on recency.
- Cross-state moves refresh when either column is truncated or the destination
  is Done. Same-column reorders return earlier, before this refresh condition;
  do not describe that condition as covering every reorder. Realtime invalidation
  is a separate refresh path.
- The [board component](../../frontend/src/components/pm/KanbanBoard.tsx) prevents
  same-column Done reordering while allowing cross-state moves. This source
  inspection does not establish visual drop-preview correctness or convergence
  across browser sessions.

Current regression test locations include the
[repository tests](../../server/internal/repository/pm_task_board_test.go),
[store tests](../../frontend/src/stores/__tests__/pmBoardStore.test.ts),
[realtime tests](../../frontend/src/hooks/__tests__/useRealtimeSync.test.tsx), and
[board ordering tests](../../frontend/src/components/pm/__tests__/KanbanBoard.ordering.test.tsx).
The old `pm_story` paths below were replaced by task paths. Use the repository's
current contributor setup instead of the historical `/root/teampulse`, Go 1.24,
and npm commands.

## Original implementation plan

**Goal:** Make sprint board ordering predictable and Trello-like: manual order in active columns, explicit recency order in done, and no silent position shifts after drop.

**Architecture:** Keep `position` as the only manual ordering source for non-done state columns. Treat done as a separate, explicitly auto-sorted column family ordered by completion/move recency. Align backend normalization, frontend optimistic updates, and websocket reconciliation so order-changing actions refresh or reindex the whole affected slice instead of patching only the moved story.

**Tech Stack:** React 19, dnd-kit, Zustand, Vitest, Go 1.24, GORM, PostgreSQL, websocket board sync

---

## Decision Summary

- Non-done state columns (`backlog`, `unstarted`, `started`) are manually ordered by `position`.
- Same-column reorder is supported only for non-done columns.
- Moving into a done column is a state transition, not a manual placement promise.
- Done columns stay sorted by `COALESCE(completed_at, moved_at, updated_at) DESC`, then `updated_at DESC`, then `position ASC`.
- Backend move/reorder endpoints must clamp requested positions to valid bounds.
- Websocket events that imply sibling renumbering must invalidate board order instead of patching a single story in place.

## Task 1: Normalize The Server Ordering Contract

**Files:**
- Modify: `server/internal/repository/pm_story.go`
- Modify: `server/internal/service/pm_story.go`
- Modify: `server/internal/model/pm_story.go`
- Test: `server/internal/service/pm_story_extended_test.go`
- Create: `server/internal/repository/pm_story_board_test.go`

- [ ] **Step 1: Write failing backend tests for bounded positions and board ordering**

Add coverage for:
- `Reorder` clamps `position` to the last valid index in the current column.
- `MoveToState` clamps `position` into the target column bounds.
- `MoveToState` without a `position` defaults to a deterministic target-column position instead of reusing the source column’s old raw position.
- `ListByWorkflowState` returns non-done columns by `position ASC, updated_at DESC`.
- `ListByWorkflowState` returns done columns by completion/move recency.

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository -run 'TestPMStoryRepository_(ListByWorkflowState|MoveToState|Reorder)'
cd /root/teampulse/server && go test ./internal/service -run 'TestPMStoryService_(MoveToState|Reorder)'
```

Expected: failures proving out-of-range positions and board-order semantics are not fully specified by tests today.

- [ ] **Step 2: Implement position normalization in the repository/service path**

Implement a shared rule for index-based ordering:
- `requested < 0` is rejected at the service layer.
- `requested > lastIndex` is clamped to `lastIndex` for reorder.
- cross-state move clamps to `[0, targetCount]`.
- when `MoveToState.Position == nil`, compute a target-column default instead of inheriting `current.Position`.

Recommended shape:
- add a small helper near the move/reorder code in `server/internal/repository/pm_story.go`
- keep sibling renumbering transactional
- keep done-column persisted positions contiguous even though display order is timestamp-first

- [ ] **Step 3: Make websocket event semantics explicit for order-changing operations**

Change reorder publishing from generic `updated` to a dedicated `reordered` action:
- `MoveToState` keeps publishing `moved`
- `Reorder` publishes `reordered`
- plain field edits keep publishing `updated`

This gives the frontend a reliable way to know when sibling positions may have changed.

- [ ] **Step 4: Re-run backend verification**

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository -run 'TestPMStoryRepository_(ListByWorkflowState|MoveToState|Reorder)'
cd /root/teampulse/server && go test ./internal/service -run 'TestPMStoryService_(MoveToState|Reorder)'
```

- [ ] **Step 5: Commit**

Run:
```bash
git add server/internal/repository/pm_story.go server/internal/service/pm_story.go server/internal/model/pm_story.go server/internal/service/pm_story_extended_test.go server/internal/repository/pm_story_board_test.go
git commit -m "fix: normalize sprint board ordering contract"
```

## Task 2: Align The Board Store With The Canonical Ordering Model

**Files:**
- Modify: `frontend/src/stores/pmBoardStore.ts`
- Modify: `frontend/src/lib/pmTypes.ts`
- Modify: `frontend/src/lib/services/pmStoryService.ts`
- Test: `frontend/src/stores/__tests__/pmBoardStore.test.ts`

- [ ] **Step 1: Write failing store tests for optimistic ordering**

Add tests covering:
- same-column reorder in a non-done column keeps the dropped order stable
- optimistic reorder reindexes loaded siblings to contiguous `position` values
- cross-state move into a non-done column inserts at the intended visible index
- moving into done ignores manual drop ordering and renders according to recency semantics
- same-column reorder in done is a no-op

Run:
```bash
cd /root/teampulse/frontend && npx vitest run src/stores/__tests__/pmBoardStore.test.ts
```

Expected: failures showing the store only updates the moved story’s position and does not fully reconcile sibling positions.

- [ ] **Step 2: Reindex loaded non-done columns after optimistic splice operations**

In `frontend/src/stores/pmBoardStore.ts`:
- add a helper that rewrites loaded non-done column story positions from array order
- call it after same-column reorder
- call it for both source and target loaded columns after cross-state moves into non-done columns

This keeps local sort stable before the network round-trip completes.

- [ ] **Step 3: Stop sending a fake manual index for done moves**

Adjust the internal move payload so `toIndex` is optional or ignored for done-state moves:
- keep `position` for non-done reorder/move requests
- omit `position` when moving into done
- keep the optimistic done-column placement consistent with recency-first sorting

- [ ] **Step 4: Preserve safety refreshes where local knowledge is incomplete**

Keep or expand board refreshes when:
- source or target columns are truncated (`has_more`)
- the target column is done
- the server response indicates the local board slice is not enough to guarantee final order

- [ ] **Step 5: Re-run store verification**

Run:
```bash
cd /root/teampulse/frontend && npx vitest run src/stores/__tests__/pmBoardStore.test.ts
```

- [ ] **Step 6: Commit**

Run:
```bash
git add frontend/src/stores/pmBoardStore.ts frontend/src/lib/pmTypes.ts frontend/src/lib/services/pmStoryService.ts frontend/src/stores/__tests__/pmBoardStore.test.ts
git commit -m "fix: align board store ordering with server semantics"
```

## Task 3: Make Realtime Sync Order-Safe

**Files:**
- Modify: `frontend/src/hooks/useRealtimeSync.ts`
- Create: `frontend/src/hooks/__tests__/useRealtimeSync.test.ts`

- [ ] **Step 1: Write failing realtime sync tests**

Add coverage for:
- `story:moved` schedules board refresh instead of patching only the moved story
- `story:reordered` schedules board refresh instead of patching only the moved story
- `story:updated` still uses the lightweight single-story patch path

Run:
```bash
cd /root/teampulse/frontend && npx vitest run src/hooks/__tests__/useRealtimeSync.test.ts
```

Expected: failure because current sync fetches one story and re-sorts against stale sibling positions.

- [ ] **Step 2: Split structural board changes from simple story updates**

In `frontend/src/hooks/useRealtimeSync.ts`:
- treat `moved` and `reordered` as ordering invalidations
- debounce `refreshBoard()` for those events
- keep incremental patching for `created`, `updated`, and `deleted`

This avoids stale sibling positions after another user reorders or moves a story.

- [ ] **Step 3: Re-run realtime sync verification**

Run:
```bash
cd /root/teampulse/frontend && npx vitest run src/hooks/__tests__/useRealtimeSync.test.ts
```

- [ ] **Step 4: Commit**

Run:
```bash
git add frontend/src/hooks/useRealtimeSync.ts frontend/src/hooks/__tests__/useRealtimeSync.test.ts
git commit -m "fix: refresh board on order-changing story events"
```

## Task 4: Make The Board UI Honest About Done Ordering

**Files:**
- Modify: `frontend/src/components/pm/KanbanBoard.tsx`
- Create: `frontend/src/components/pm/__tests__/KanbanBoard.ordering.test.tsx`

- [ ] **Step 1: Write failing component tests for done-column drag behavior**

Add coverage for:
- same-column reorder is disabled in done
- cross-column drag into done remains allowed
- dropping into done does not promise arbitrary slot placement

Run:
```bash
cd /root/teampulse/frontend && npx vitest run src/components/pm/__tests__/KanbanBoard.ordering.test.tsx
```

Expected: failure because done is still part of generic drop calculations and the UI still computes target indexes before handing off to the store.

- [ ] **Step 2: Remove misleading done-column reorder affordances**

In `frontend/src/components/pm/KanbanBoard.tsx`:
- keep same-column reorder path disabled for done
- make cross-column done preview consistent with recency ordering rather than arbitrary slot placement
- avoid passing a fake same-column target index for done interactions

- [ ] **Step 3: Re-run component verification**

Run:
```bash
cd /root/teampulse/frontend && npx vitest run src/components/pm/__tests__/KanbanBoard.ordering.test.tsx
```

- [ ] **Step 4: Commit**

Run:
```bash
git add frontend/src/components/pm/KanbanBoard.tsx frontend/src/components/pm/__tests__/KanbanBoard.ordering.test.tsx
git commit -m "fix: make done-column ordering explicit"
```

## Task 5: Final Verification And Manual QA

**Files:**
- Verify only

- [ ] **Step 1: Run focused automated verification**

Run:
```bash
cd /root/teampulse/server && go test ./internal/repository -run 'TestPMStoryRepository_(ListByWorkflowState|MoveToState|Reorder)'
cd /root/teampulse/server && go test ./internal/service -run 'TestPMStoryService_(MoveToState|Reorder)'
cd /root/teampulse/frontend && npx vitest run src/stores/__tests__/pmBoardStore.test.ts src/hooks/__tests__/useRealtimeSync.test.ts src/components/pm/__tests__/KanbanBoard.ordering.test.tsx
```

- [ ] **Step 2: Run broader verification**

Run:
```bash
cd /root/teampulse/server && go test ./...
cd /root/teampulse/frontend && npm run build
```

- [ ] **Step 3: Manual QA in the board UI**

Verify all of the following:
- non-done same-column reorder stays where dropped after the API response returns
- non-done reorder still stays correct after a hard refresh
- moving into another active column preserves the intended visible placement
- moving into done lands according to newest-completed semantics, not arbitrary drop slot
- another browser session reordering or moving a card causes a board refresh and converges to the same order
- columns with more than 25 cards still recover correctly after moves because the board refreshes when local knowledge is incomplete

- [ ] **Step 4: Final commit**

Run:
```bash
git add .
git commit -m "fix: stabilize sprint board ordering"
```

## Recommended Rollout Order

1. Ship Task 1 and Task 3 together so backend event semantics and frontend reconciliation stay aligned.
2. Ship Task 2 next so local optimistic behavior matches the backend contract.
3. Ship Task 4 immediately after if product wants done behavior to be visually explicit in the same release.
4. Do not merge without the multi-session manual QA in Task 5; the user-visible bug is partly a realtime convergence problem, not just a local drag-and-drop problem.
