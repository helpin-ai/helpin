# Table Components Performance Analysis

> Historical implementation review. Its component names, line numbers, and bug
> claims describe an older table implementation. Current CRM tables use
> `useTableSurface`, and memoized row components are present; `StoryListView`
> has been replaced by `TaskListView`. Reproduce a performance issue against the
> current component before treating a recommendation below as an open defect.


## Current source comparison (2026-09-17)

The recommendations below are historical hypotheses, not a current defect list
or measured speedup. No browser profiling was performed for this docs review.

| Original recommendation | Current implementation |
| --- | --- |
| Memoize rows and group headers | Present in ContactsTable, CompaniesTable, DealsTable, and TaskListView. |
| Remove sizing props to prevent all resize renders | Current row comparators explicitly check sizing versions. Removing that invalidation without another width-update mechanism can leave stale rendering; a stable table row reference alone is insufficient. |
| Memoize `estimateSize` | All four current tables use `useCallback`. |
| Remove `localDeals` dependency | Deal edits now live in `useDealEdits`; its callback depends on current source deals, stages, scope, and callbacks, not its optimistic `localDeals` result. This does not make it permanently stable. |
| Defer pinned-group state in the scroll handler | TaskListView's handler handles pagination; pinned headers use StickyPinnedGroupOverlay. The old inline handler no longer applies. |
| Reduce all overscan values from 20 | Contacts uses 6; TaskListView uses 4 grouped / 6 flat; Companies and Deals retain 20. Profile the relevant view before changing these tradeoffs. |

Current sources: [contacts](src/components/crm/ContactsTable.tsx),
[companies](src/components/crm/CompaniesTable.tsx),
[deals](src/components/crm/DealsTable.tsx),
[tasks](src/components/pm/TaskListView.tsx), and
[deal edits](src/components/crm/useDealEdits.ts).

## Summary of Findings

Files analyzed:
- `src/components/pm/StoryListView.tsx` (1604 lines)
- `src/components/crm/ContactsTable.tsx` (721 lines)
- `src/components/crm/DealsTable.tsx` (898 lines)
- `src/components/crm/CompaniesTable.tsx` (628 lines)

---

## Issue #1: `DataRow` and `GroupHeaderRow` are NOT memoized (ALL 4 files) — **HIGH IMPACT**

**Problem:** None of the four files use `React.memo` anywhere. `DataRow` receives `row` and `columnSizing` as props. When any table state changes (sorting, selection, column resize, expand/collapse), React re-renders every visible `DataRow` even if its own data hasn't changed. With 500+ rows and virtualization showing ~30-40 rows, this means 30-40 unnecessary re-renders on every state toggle.

The `columnSizing` prop is particularly problematic — during column resize, `onColumnSizingChange` fires on every mouse move, causing all visible rows to re-render continuously.

**Fix:**

```tsx
import { memo } from 'react';

const DataRow = memo(function DataRow({
  row,
  columnSizing,
}: {
  row: Row<CRMContact>;
  columnSizing: ColumnSizingState;
}) {
  return (
    <div className={TABLE_ROW}>
      {row.getVisibleCells().map((cell) => {
        // ... existing cell rendering
      })}
    </div>
  );
});

const GroupHeaderRow = memo(function GroupHeaderRow({
  row,
}: {
  row: Row<CRMContact>;
}) {
  // ... existing implementation
});
```

**Why it works:** TanStack Table's `Row` object is referentially stable when data hasn't changed. `memo` will skip re-rendering rows whose `row` reference and `columnSizing` object haven't changed. For `columnSizing`, see Issue #2 below for further optimization.

**Impact:** During column resize: eliminates ~30-40 re-renders per mouse-move event. During sort/selection: eliminates re-renders of rows whose data didn't change.

---

## Issue #2: `columnSizing` state passed to every `DataRow` defeats memoization — **HIGH IMPACT**

**Problem:** `columnSizing` is a state object that changes reference on every column resize event. It's passed to every `DataRow`, so even with `React.memo`, ALL rows will re-render during resize because the `columnSizing` reference changes.

In all 4 files:
```tsx
<DataRow row={row} columnSizing={columnSizing} />
```

Inside `DataRow`, `columnSizing` is only used to compute `isResized = !!columnSizing[cell.column.id]`, which is then passed to `dynamicCellStyle`. The actual column sizes come from `cell.column.getSize()` which TanStack Table computes internally.

**Fix:** Remove `columnSizing` from `DataRow` props entirely. Use `cell.column.getIsResizing()` or compute `isResized` from `cell.column.getSize() !== cell.column.columnDef.size`:

```tsx
const DataRow = memo(function DataRow({ row }: { row: Row<Story> }) {
  return (
    <div className={TABLE_ROW}>
      {row.getVisibleCells().map((cell) => {
        if (cell.column.getIsGrouped()) return null;
        const defSize = cell.column.columnDef.size ?? 150;
        const runtimeSize = cell.column.getSize();
        const isResized = runtimeSize !== defSize;
        // ... rest unchanged
      })}
    </div>
  );
});
```

**Impact:** Combined with Issue #1, this eliminates ALL row re-renders during column resize. This is the single highest-impact optimization since column resize fires dozens of events per second.

---

## Issue #3: `estimateSize` callback recreated every render (ALL 4 files) — **MEDIUM IMPACT**

**Problem:** The `estimateSize` function is an inline arrow function that closes over `rows`:

```tsx
const virtualizer = useVirtualizer({
  count: rows.length,
  getScrollElement: () => parentRef.current,
  estimateSize: (index) => {
    const row = rows[index];
    return row?.getIsGrouped() ? GROUP_ROW_HEIGHT : ROW_HEIGHT;
  },
  overscan: 20,
});
```

Every time the component re-renders, a new `estimateSize` function is created. While `@tanstack/react-virtual` is somewhat resilient to this, it can trigger unnecessary virtualizer recalculations.

**Fix:** Memoize via `useCallback`:

```tsx
const estimateSize = useCallback(
  (index: number) => {
    const row = rows[index];
    return row?.getIsGrouped() ? GROUP_ROW_HEIGHT : ROW_HEIGHT;
  },
  [rows],
);

const virtualizer = useVirtualizer({
  count: rows.length,
  getScrollElement: () => parentRef.current,
  estimateSize,
  overscan: 20,
});
```

**Impact:** Prevents virtualizer from recalculating measurements when non-row-related state changes (selection, column sizing).

---

## Issue #4: `updateStoryField` in StoryListView has unstable dependencies — **MEDIUM IMPACT**

**Problem (StoryListView only):** `updateStoryField` is wrapped in `useCallback` but depends on several values that may change:

```tsx
const updateStoryField = useCallback(
  async (storyId: string, field: string, value: unknown) => { ... },
  [workspaceId, stories, /* potentially others */],
);
```

This callback is used in `tableColumns` useMemo dependencies:
```
[stateMap, statesByWorkflowId, ownerNameMap, teamMap, epicMap, sprintMap,
 onOpenStory, workflow.states, assignableMembers, teams, epics, sprints,
 updateStoryField, allLabels, workspaceId]
```

That's 15 dependencies. If any changes, ALL column definitions are recreated, which invalidates the entire table and forces a full re-render.

**Problem (DealsTable):** `updateDealField` depends on `localDeals` (line 140: `localDeals.find(...)`), which changes on every optimistic update. This means the callback reference changes after each edit, causing `tableColumns` to be recreated:

```tsx
const updateDealField = useCallback(
  async (dealId: string, patch: Partial<CRMDeal>) => {
    // ...
    else onDealUpdated?.({ ...localDeals.find((d) => d.id === dealId)!, ...optimisticPatch } as CRMDeal);
  },
  [workspaceId, stageMap, onDealUpdated, localDeals], // localDeals changes on every edit!
);
```

**Fix (DealsTable):** Use a ref for localDeals inside the callback:

```tsx
const localDealsRef = useRef(localDeals);
localDealsRef.current = localDeals;

const updateDealField = useCallback(
  async (dealId: string, patch: Partial<CRMDeal>) => {
    // ...
    else onDealUpdated?.({ ...localDealsRef.current.find((d) => d.id === dealId)!, ...optimisticPatch } as CRMDeal);
  },
  [workspaceId, stageMap, onDealUpdated], // localDeals removed
);
```

**Impact:** Prevents tableColumns recreation and full table re-render after every inline edit.

---

## Issue #5: `onScroll` handler does synchronous state update during scroll (StoryListView) — **MEDIUM IMPACT**

**Problem:** The `onScroll` handler in StoryListView (line 720) updates pinned group row state synchronously during scroll:

```tsx
onScroll={(e) => {
  // Infinite loading check
  if (!isExternal && hasMore && !loadingMore) {
    if (scrollTop + el.clientHeight >= el.scrollHeight - 200) {
      loadMore();
    }
  }

  // Pinned group row calculation
  // ... iterates through rows to find current group
  if (newPinnedIdx !== pinnedGroupRef.current) {
    pinnedGroupRef.current = newPinnedIdx;
    setPinnedGroupIdx(newPinnedIdx); // <-- state update during scroll!
  }
}}
```

`setPinnedGroupIdx` triggers a re-render on every scroll position that crosses a group boundary. While the ref guards against unnecessary updates, the state update itself is not debounced and happens synchronously in the scroll handler.

**Fix:** Use `startTransition` for non-urgent pinned header updates:

```tsx
import { startTransition } from 'react';

onScroll={(e) => {
  // ... infinite loading (keep as-is, this is urgent)

  // Pinned group row - non-urgent UI update
  if (newPinnedIdx !== pinnedGroupRef.current) {
    pinnedGroupRef.current = newPinnedIdx;
    startTransition(() => {
      setPinnedGroupIdx(newPinnedIdx);
    });
  }
}}
```

**Impact:** Prevents scroll jank when scrolling through group boundaries by deferring the pinned header update.

---

## Lower Priority Issues

### Issue #6: `handleDelete` not memoized in StoryListView

Unlike the CRM tables which wrap `handleDelete` in `useCallback`, check whether StoryListView has a similar handler without memoization. (From analysis: StoryListView does not appear to have a `handleDelete` — deletion is handled externally, so this is not an issue.)

### Issue #7: Overscan value of 20 may be high

All 4 files use `overscan: 20`. With `ROW_HEIGHT` of ~36px, this renders ~720px of off-screen rows above and below the viewport. For most screens, 10 would suffice, reducing the number of mounted row components by ~20.

### Issue #8: Inline arrow functions in cell definitions

Column `cell` renderers use inline arrow functions like `(info) => <InlinePriorityCell ... />`. These are recreated when `tableColumns` is recreated. However, since `tableColumns` is already memoized, and cell renderers don't cause child re-renders (they're just JSX factories), this is low priority.

---

## Prioritized Optimization Recommendations

| Priority | Issue | Expected Impact | Effort |
|----------|-------|----------------|--------|
| 1 | Wrap `DataRow` + `GroupHeaderRow` in `React.memo` (all 4 files) | Eliminates 30-40 unnecessary row re-renders per state change | Low |
| 2 | Remove `columnSizing` prop from `DataRow`, derive `isResized` from column API | Eliminates all row re-renders during column resize (~60fps of wasted work) | Low |
| 3 | Fix `updateDealField` dependency on `localDeals` in DealsTable | Prevents full table recreation after every inline edit | Low |
| 4 | Memoize `estimateSize` callback (all 4 files) | Prevents unnecessary virtualizer recalculation | Low |
| 5 | Wrap `onScroll` pinned-group state update in `startTransition` (StoryListView) | Reduces scroll jank at group boundaries | Low |
| 6 | Reduce `overscan` from 20 to 10 (all 4 files) | Renders ~10 fewer off-screen rows | Trivial |

These were proposed priorities for the old implementation. The expected impacts
above were estimates, not benchmark results, and do not establish current work to do.
