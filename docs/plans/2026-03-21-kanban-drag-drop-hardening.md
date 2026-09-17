# Kanban drag-and-drop hardening plan

This historical audit explains the board ordering problems that motivated later
changes. Contributors should use the implementation notes below before treating
its findings as current defects or its checkboxes as unfinished work.

> Source review: 2026-09-18. Original story paths, Go 1.24 references, and execution
> instructions are historical. No drag simulation or browser QA was performed.

## Current implementation and limits

- The [task repository](../../server/internal/repository/pm_task.go) normalizes
  sibling positions and performs state move/reorder updates transactionally.
  Member columns order by workflow state type, state position, task position,
  and update time; they no longer compare only state-local positions across states.
- The [board store](../../frontend/src/stores/pmBoardStore.ts) makes same-member
  moves a no-op. Cross-member moves send only `owner_member_ids`, replacing the
  owners with the target member or an empty list for unassigned. The dropped
  index is an optimistic placement, not a persisted member ranking.
- The store blocks same-state Done reorders and sorts Done by recency. It still
  sends an index for cross-state moves into Done. For the precise state-board
  refresh limits, see the [ordering plan](2026-03-24-sprint-board-ordering.md).
- State and member mutation guards compare workflow ID and team ID. They do not
  compare workspace, filters, board mode, or a per-view generation token as the
  original proposal suggests. Do not describe them as protection against every
  possible view change or overlapping mutation.
- `moveMemberTask` does not itself refetch truncated columns after reassignment.
  It splices loaded arrays and updates counts; independent UI/realtime refresh
  paths must be considered before claiming pagination convergence.
- [KanbanBoard](../../frontend/src/components/pm/KanbanBoard.tsx) reveals available
  empty columns during dragging and retains the droppable reference on collapsed
  state/member sections. This does not prove an unloaded member column exists
  as a target. Its configured drag sensor is still `PointerSensor`.
- [TaskCard](../../frontend/src/components/pm/TaskCard.tsx) still attaches drag
  listeners to the article, with `shouldIgnoreTaskCardDrag` filtering pointer
  activation from interactive descendants. There is no separate handle in this
  path; keyboard click-to-open is not keyboard drag support.

Focused [store tests](../../frontend/src/stores/__tests__/pmBoardStore.test.ts),
[board ordering tests](../../frontend/src/components/pm/__tests__/KanbanBoard.ordering.test.tsx),
and [repository ordering tests](../../server/internal/repository/pm_task_board_test.go)
now exist. Their existence is not evidence that every original accessibility,
concurrency, pagination, or performance criterion has been exercised successfully.

## Original audit and plan

**Goal:** Make PM story-board drag-and-drop correct, predictable, and testable across both "By States" and "By Members" kanban modes.

**Architecture:** The biggest issue is that the current drag-and-drop flow is mixing two different ordering models. State-grouped columns use `position` as a per-state sort key, while member-grouped columns also sort by `position` even though they contain stories from many states. The fix should start by defining the correct persistence model per board mode, then align optimistic UI, server updates, and pagination behavior to that model.

**Tech Stack:** React 19, dnd-kit, Zustand, TanStack Query/Router, Go 1.24, GORM, PostgreSQL

---

## Audit Findings

### 1. Member-board reordering is conceptually wrong today

**Files:**
- `frontend/src/stores/pmBoardStore.ts`
- `server/internal/repository/pm_story.go`

The member board sorts each member column by `position ASC, updated_at DESC` even though each member column contains stories from many workflow states. That means two stories from different states are being compared with a state-local position field that has no consistent cross-state meaning.

Current consequences:
- same-member drag reorder is not anchored to a real persisted ordering model
- dragging inside a member column can silently mutate state-board ordering instead of member-board ordering
- cross-member drag preserves owner change, but not the dropped order in a meaningful way

### 2. State-board move/reorder only writes a raw `position`; siblings are never renumbered

**Files:**
- `frontend/src/stores/pmBoardStore.ts`
- `server/internal/repository/pm_story.go`
- `server/internal/service/pm_story.go`

The current backend `MoveToState` and `Reorder` calls just write the moved story’s `position` and stop. They do not shift sibling stories or normalize the column afterwards.

Current consequences:
- duplicate positions are easy to create
- sparse positions accumulate
- reordering can look correct optimistically and then come back in a different order after refresh
- using array indexes as positions becomes progressively less meaningful over time

### 3. The done column advertises manual ordering, but the sort order ignores drop intent

**Files:**
- `frontend/src/stores/pmBoardStore.ts`
- `server/internal/repository/pm_story.go`
- `frontend/src/components/pm/KanbanBoard.tsx`

Done columns are sorted by `completed_at/moved_at/updated_at DESC` first, and only then by `position`. The UI still allows drag-and-drop within done, and still sends a target index.

Current consequences:
- reordering within done is effectively ignored or unstable
- moving a story into done can jump to a different place than the drop target
- optimistic local ordering and server ordering diverge immediately

### 4. Dragging into partially loaded columns is not trustworthy

**Files:**
- `frontend/src/stores/pmBoardStore.ts`
- `frontend/src/components/pm/KanbanBoard.tsx`

The target index is computed from the currently loaded cards only, while the server column may have many more hidden cards behind `has_more`.

Current consequences:
- dropping at the “bottom” of a partially loaded column is only the bottom of the loaded slice, not the true column
- the source column can lose one visible card without backfilling from hidden items
- the target column can gain one optimistic card, changing client-side offsets and risking skipped or duplicated items on the next "load more"

### 5. Empty, hidden, and collapsed columns are not valid drop targets

**Files:**
- `frontend/src/components/pm/KanbanBoard.tsx`

Only rendered expanded columns become droppable. Empty columns are filtered out when `showEmptyColumns` is off, and collapsed columns render a non-droppable stub.

Current consequences:
- you cannot drag into an empty hidden state/member column
- you cannot drag into a collapsed column
- drag-and-drop capability depends on display settings, not on actual board permissions or workflow rules

### 6. The whole card is the drag handle, which conflicts with inline editing controls

**Files:**
- `frontend/src/components/pm/StoryCard.tsx`
- `frontend/src/components/pm/KanbanBoard.tsx`

`useSortable` listeners are attached to the entire `<article>`, and the board only installs a `PointerSensor`.

Current consequences:
- owner/priority/severity/estimate controls are competing with drag initiation
- there is no dedicated drag affordance
- keyboard drag is not supported
- the card is both a button and a drag handle, which is hard to reason about for accessibility

### 7. In-flight drag mutations are not scoped to the current board context

**Files:**
- `frontend/src/stores/pmBoardStore.ts`

Board fetches use a request ID to avoid stale responses winning, but `moveStory` and `moveMemberStory` do not. They snapshot the current columns and later restore or patch them without checking whether the user has switched workflow, team, filters, or board mode in the meantime.

Current consequences:
- a failed drag can restore stale columns over a newer board state
- a late success can patch the wrong board view
- move rollback behavior is weaker than fetch behavior

### 8. There is almost no targeted test coverage for board drag-and-drop semantics

**Files:**
- `frontend/src/components/pm/__tests__`
- `frontend/src/stores/__tests__`
- `server/internal/service/pm_story_extended_test.go`

I did not find focused frontend board drag tests, and the backend tests only assert that a raw `position` field gets written. They do not validate relative ordering, sibling renumbering, done-column semantics, member-board semantics, or pagination interactions.

---

## Recommended Product Rules

Before implementation, lock these rules down so the code can converge on one consistent model:

1. **State board**
   - Drag within a state should persist order within that state.
   - Drag across states should persist the target state and relative order within that target state.

2. **Done state**
   - Recommended: disable manual reorder inside done and sort it strictly by completion/move time.
   - Drag into done should change state only; the drop index should not promise manual ordering.

3. **Member board**
   - Recommended: drag across member columns should mean reassignment only.
   - Same-member reordering should be disabled unless the product introduces a separate member-board ordering model.
   - If a story is reassigned, its ordering should still be controlled by the state board, not by a fake cross-state member ordering.

4. **Partially loaded columns**
   - Recommended: either refresh the affected columns after every move into/out of a truncated column, or temporarily disable index-sensitive drops into truncated columns.

---

## Implementation Plan

### Task 1: Fix the ordering model first

**Files:**
- `server/internal/repository/pm_story.go`
- `server/internal/service/pm_story.go`
- `server/internal/model/pm_story.go`
- `frontend/src/stores/pmBoardStore.ts`

- [ ] Define one canonical server-side move contract for state-board ordering.
- [ ] Change move/reorder persistence so sibling rows are renumbered transactionally instead of only updating one story’s raw `position`.
- [ ] Return enough updated data for the client to reconcile affected columns cleanly.
- [ ] Add explicit service/repository tests for sibling renumbering and relative ordering.

### Task 2: Remove misleading reorder semantics from the member board

**Files:**
- `frontend/src/components/pm/KanbanBoard.tsx`
- `frontend/src/stores/pmBoardStore.ts`
- `server/internal/repository/pm_story.go`

- [ ] Disable same-member reorder unless stories are in a mode where that order is genuinely persisted.
- [ ] Treat cross-member drag as assignment/reassignment, not as a positional reorder.
- [ ] Rebuild member-column ordering so it is derived from the state-board truth rather than pretending `position` is cross-state.
- [ ] Add tests covering reassignment between members and unassigned.

### Task 3: Make done-column drag behavior honest

**Files:**
- `frontend/src/components/pm/KanbanBoard.tsx`
- `frontend/src/stores/pmBoardStore.ts`
- `server/internal/repository/pm_story.go`

- [ ] Disable intra-done reorder in the UI, or change the done board sort model if product wants manual done ordering.
- [ ] When dragging into done, remove any index-based promise from the client/store/server path.
- [ ] Add regression tests so done ordering cannot silently drift back to index-based drag behavior.

### Task 4: Handle paginated/truncated columns safely

**Files:**
- `frontend/src/stores/pmBoardStore.ts`
- `frontend/src/components/pm/KanbanBoard.tsx`
- `frontend/src/lib/services/pmStoryService.ts`

- [ ] Detect when source or target columns are truncated (`has_more`).
- [ ] After a move touching truncated columns, refresh or refetch the affected columns instead of relying on pure optimistic splice logic.
- [ ] Prevent offset desynchronization between optimistic local arrays and `load more` requests.
- [ ] Add tests for moving into/out of partially loaded state and member columns.

### Task 5: Restore missing drop targets

**Files:**
- `frontend/src/components/pm/KanbanBoard.tsx`
- `frontend/src/stores/boardDisplayStore.ts`

- [ ] Keep empty columns droppable even when visually lightweight.
- [ ] Decide whether collapsed columns should remain droppable via slim drop strips or whether the UI should auto-expand on hover.
- [ ] Ensure member-board drag can still target empty or unassigned columns when those are logically available.

### Task 6: Improve drag ergonomics and accessibility

**Files:**
- `frontend/src/components/pm/StoryCard.tsx`
- `frontend/src/components/pm/KanbanBoard.tsx`

- [ ] Move drag listeners to an explicit drag handle instead of the whole card.
- [ ] Keep click-to-open and inline field controls independent from drag initiation.
- [ ] Add keyboard drag support or explicitly document that the board is pointer-only until keyboard support lands.
- [ ] Add auto-scroll support for horizontal board scrolling and tall column scrolling while dragging.

### Task 7: Scope optimistic drag mutations to the active board view

**Files:**
- `frontend/src/stores/pmBoardStore.ts`

- [ ] Add request/view tokens for move operations similar to the existing guarded fetch path.
- [ ] Prevent stale move success/failure handlers from restoring old board snapshots over a newer view.
- [ ] Refresh the smallest affected slice when context changed mid-flight.

### Task 8: Add real test coverage

**Files:**
- `frontend/src/stores/__tests__/`
- `frontend/src/components/pm/__tests__/`
- `server/internal/service/pm_story_extended_test.go`
- `server/internal/repository/pm_story.go` tests if repo-level tests exist nearby

- [ ] Add store-level tests for same-state reorder, cross-state move, rollback, and truncated-column refresh behavior.
- [ ] Add component-level tests for drag availability in empty/collapsed columns and for member-board reassignment.
- [ ] Add backend tests for sibling renumbering, done-state semantics, and member-board persistence rules.

---

## Recommended Order

1. Fix the persistence model for state-board ordering.
2. Remove or redesign member-board reorder semantics.
3. Make done-column drag behavior honest.
4. Make truncated-column moves safe.
5. Restore empty/collapsed drop targets.
6. Improve drag handle/accessibility and add tests around all of the above.
