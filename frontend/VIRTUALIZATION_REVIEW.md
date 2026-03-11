# Virtualization Review: Table Components

## Shared Constants (`src/lib/tableStyles.ts`)

- `TABLE_CONTAINER = 'min-h-0 flex-1 overflow-auto'` -- this IS a scroll container (overflow-auto)
- `TABLE_HEADER = 'sticky top-0 z-10 border-b border-border/60 bg-muted/50'` -- sticky within TABLE_CONTAINER
- `ROW_HEIGHT = 36`, `GROUP_ROW_HEIGHT = 36`

---

## CRITICAL BUG: Dual Scroll Containers (affects ALL 4 files)

All four files share the identical nesting structure:

```
<div className="flex min-h-0 flex-1 flex-col gap-2">     ← outer flex column
  <div> Group By controls </div>
  <div className={TABLE_CONTAINER}>                       ← SCROLL CONTAINER #1 (overflow-auto, flex-1)
    <div className="min-w-fit">                           ← min-w-fit wrapper
      <div className={TABLE_HEADER}>...</div>             ← sticky header (sticky top-0)
      <div ref={parentRef}                                ← SCROLL CONTAINER #2 (overflow-auto)
           className="overflow-auto scrollbar-hide"
           style={{ height: 'calc(100% - 30px)' }}>
        <div style={{ height: virtualizer.getTotalSize(), position: relative }}>
          {virtualizer.getVirtualItems().map(...)}         ← absolute positioned rows
        </div>
      </div>
    </div>
  </div>
</div>
```

### Problems Identified

#### 1. TWO COMPETING SCROLL CONTAINERS
- `TABLE_CONTAINER` has `overflow-auto` -- it IS a scroll container
- `parentRef` div also has `overflow-auto` -- it is ALSO a scroll container
- The virtualizer points `getScrollElement` at `parentRef` (the inner one)
- But `TABLE_CONTAINER` (outer) can ALSO scroll, creating confusion

**Impact**: The outer `TABLE_CONTAINER` may capture scroll events that should go to the inner `parentRef`. If the `min-w-fit` wrapper causes horizontal overflow, `TABLE_CONTAINER` scrolls horizontally while `parentRef` scrolls vertically -- these are SEPARATE scroll containers. This means:
- Horizontally scrolling the header does NOT scroll the body (they're in different scroll contexts)
- The sticky header (`sticky top-0`) is sticky within `TABLE_CONTAINER`, but the body rows are inside `parentRef` -- so the header scrolls away vertically inside `TABLE_CONTAINER` rather than staying pinned

#### 2. `parentRef` HEIGHT: `calc(100% - 30px)`
- The `parentRef` div uses `height: calc(100% - 30px)` to account for the header height
- But `100%` resolves relative to the `min-w-fit` wrapper div, which has NO explicit height
- A `min-w-fit` div with no height will size to its content, making `100%` either undefined or content-based
- This likely means `parentRef` gets an unbounded height, which **defeats virtualization entirely** -- all rows render because the scroll container is tall enough to fit them all

#### 3. HEADER AND BODY HORIZONTAL SCROLL DISCONNECT
- The header is inside `<div className="min-w-fit">` but OUTSIDE `parentRef`
- `parentRef` has its own `overflow-auto`
- If horizontal scrolling is needed, the header and body would scroll independently (unless TABLE_CONTAINER's overflow-auto handles it, but then the two vertical scroll containers conflict)

#### 4. `min-w-fit` WRAPPER PLACEMENT
- `min-w-fit` is between `TABLE_CONTAINER` and `parentRef`
- This is correct for making the content expand horizontally for wide tables
- But it creates a layer that prevents `parentRef` from getting a proper percentage height

---

## Per-File Details

### ContactsTable.tsx (lines 404-496)
- `parentRef` on: inner div (line 462)
- Scroll container CSS: `overflow-auto scrollbar-hide`, `height: calc(100% - 30px)`
- Virtualizer: `estimateSize` uses ROW_HEIGHT/GROUP_ROW_HEIGHT, `overscan: 20`
- `measureElement`: YES (line 475)
- translateY pattern: YES, correct (position absolute + translateY)
- **Same dual-scroll-container bug**

### DealsTable.tsx (lines 426-517)
- `parentRef` on: inner div (line 481)
- Scroll container CSS: `overflow-auto scrollbar-hide`, `height: calc(100% - 30px)`
- Virtualizer: identical config
- `measureElement`: YES (line 495)
- translateY pattern: YES, correct
- **Same dual-scroll-container bug**

### CompaniesTable.tsx (lines 375-466)
- `parentRef` on: inner div (line 430)
- Scroll container CSS: `overflow-auto scrollbar-hide`, `height: calc(100% - 30px)`
- Virtualizer: identical config
- `measureElement`: YES (line 444)
- translateY pattern: YES, correct
- **Same dual-scroll-container bug**

### StoryListView.tsx (lines 714-849)
- `parentRef` on: inner div (line 771)
- Scroll container CSS: `overflow-auto` (NO scrollbar-hide), `height: calc(100% - 30px)`
- Virtualizer: identical config
- `measureElement`: YES (line 820)
- translateY pattern: YES, correct
- Additional: has `onScroll` handler for infinite loading + pinned group header tracking
- Additional: has a sticky pinned group header inside the virtualizer total-size div
- **Same dual-scroll-container bug**

---

## Recommended Fix

The correct structure should use a SINGLE scroll container with the header sticky inside it:

```
<div className={TABLE_CONTAINER}>           ← SOLE scroll container (overflow-auto)
  ref={parentRef}                           ← virtualizer points HERE
  <div className="min-w-fit">              ← horizontal expansion
    <div className={TABLE_HEADER}>          ← sticky top-0 (within the scroll container)
    <div style={{ height: getTotalSize() }}> ← virtualizer total height
      {getVirtualItems()...}                ← absolute positioned rows
    </div>
  </div>
</div>
```

Key changes:
1. Merge `TABLE_CONTAINER` and `parentRef` into ONE element
2. Put `ref={parentRef}` on `TABLE_CONTAINER`
3. Remove the inner `overflow-auto` div entirely
4. Keep `min-w-fit` inside the scroll container for horizontal expansion
5. The header stays sticky (sticky top-0 works within the overflow-auto parent)
6. Adjust virtualizer `estimateSize` -- no need to subtract header height since the header is sticky, not part of the virtual list
