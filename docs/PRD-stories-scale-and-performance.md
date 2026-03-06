# Product Requirements Document: Stories Scale And Performance

**Product**: Helpin PM Module  
**Date**: March 5, 2026  
**Status**: Draft  
**Author**: Codex  
**Scope**: Stories board, stories list/table, realtime updates, supporting PM APIs, and frontend performance readiness for large workspaces

---

## 1. Executive Summary

Helpin's current stories experience is functional for small to medium datasets, but it is not production-ready for large workspaces with 10,000+ stories.

The current bottlenecks are structural:

1. The kanban board fetches and renders all stories in a workflow at once.
2. The list view uses row virtualization, but still fetches only a partial fixed page and performs grouping client-side.
3. Realtime updates trigger full board refreshes instead of targeted state patching.
4. Drag-and-drop, filtering, and grouping all assume in-memory full datasets.

This PRD defines the changes required to make stories views operationally safe and performant for large datasets while preserving current functionality.

---

## 2. Problem Statement

If a workspace contains 10,000 stories across all work, the current frontend and API approach will degrade in predictable ways:

- Board load time increases sharply because all cards are fetched and mounted.
- Drag-and-drop becomes expensive because every card participates in the sortable system.
- Realtime activity amplifies cost because a single story update can force a full board refetch.
- List view does not provide a complete large-dataset experience because it fetches a fixed 500-row slice.
- Memory, DOM node count, and React work scale with total dataset size instead of visible dataset size.

The product risk is not just slower UX. At this scale, some views will become unreliable, difficult to operate, and prone to browser-level jank.

---

## 3. Goals

### Primary Goals

1. Make stories board and list views usable in workspaces with 10,000+ stories.
2. Ensure the UI scales with visible data, not total data, wherever possible.
3. Reduce full-page and full-board refetches caused by websocket updates and inline edits.
4. Preserve current filtering, grouping, sorting, and detail interactions.
5. Establish measurable frontend performance budgets and acceptance criteria.

### Secondary Goals

1. Improve backend query efficiency for large filtered datasets.
2. Make scaling behavior explicit in API contracts.
3. Create a rollout path with low regression risk.

---

## 4. Non-Goals

This PRD does not include:

- Roadmap/Gantt scaling work outside stories views
- Full-text search redesign
- Cross-workspace analytics redesign
- Mobile-native optimization beyond reasonable responsive behavior
- Infinite historical activity redesign for story detail, except where necessary for story-list performance consistency

---

## 5. Current State Summary

### Frontend

- Stories page routes into a single `KanbanBoard` surface that owns both board and list modes.
- Board mode renders every story card in every visible column.
- Story cards participate in drag-and-drop individually.
- List mode uses `@tanstack/react-virtual` for rows.
- List mode currently self-fetches with `per_page=500`.
- Grouping and row-model construction happen client-side.
- Websocket story events trigger a full board refresh.

### Backend

- General stories list endpoint supports pagination.
- Board endpoint does not support pagination, cursors, or per-column incremental loading.
- Board endpoint loads all workflow stories matching filters in one query and returns complete columns.
- Filtered board responses are recomputed and returned in full on every refresh.

---

## 6. Product Principles

### 6.1 Visible Work First

The UI must prioritize rendering what the user can currently see and interact with. Hidden or off-screen work should not incur full render cost.

### 6.2 Progressive Data Loading

Large datasets must load incrementally, both at the API layer and in the UI.

### 6.3 Stable Interactions At Scale

Drag-and-drop, inline editing, filtering, and row expansion must remain responsive at large counts.

### 6.4 Realtime Must Be Incremental

Realtime updates should patch only affected records or columns whenever possible, not refetch entire board payloads.

### 6.5 Product Behavior Must Be Explicit

The user should understand when a view is partially loaded, loading more, or fully loaded. The system should not silently truncate datasets.

---

## 7. User Stories

1. As a PM user, I can open a stories board in a large workspace without the browser freezing.
2. As a PM user, I can scroll large columns and progressively load more cards.
3. As a PM user, I can use list/table view to browse all matching stories, not only an arbitrary first 500.
4. As a PM user, I can filter and group large result sets without major UI lag.
5. As a PM user, I can move a story between columns without the full board reloading.
6. As a PM user, I can receive websocket updates without the board visibly thrashing.
7. As an engineer, I can reason about scaling behavior through explicit APIs, metrics, and tests.

---

## 8. Success Metrics

### Functional

- Board view can open and operate on workflows whose filtered result set exceeds 10,000 stories.
- List view can navigate and display full filtered datasets above 10,000 stories.
- No stories are silently omitted because of a hardcoded frontend fetch limit.

### Performance

- Initial board payload loads counts and first slices without fetching all cards.
- Scrolling a large column does not cause visible sustained jank under normal desktop hardware.
- List scrolling remains smooth with large filtered result sets due to virtualization plus incremental fetch.
- A single story websocket update does not trigger full board reload in the common case.

### Reliability

- Drag-and-drop remains usable in large columns.
- Filters and grouping remain correct when data is loaded incrementally.
- Refreshes and pagination do not duplicate or lose items.

---

## 9. Requirements

### 9.1 Board View Requirements

#### R1. Column-level incremental loading

The board must stop loading every story for every column on initial page load.

Required behavior:

- Initial board load returns workflow metadata, column metadata, counts, and the first page of stories per column.
- Each column independently supports pagination or cursor-based loading.
- The UI can load more stories within a column through scroll-based or explicit "Load more" behavior.
- Collapsed columns load only summary metadata, not full card payloads.

#### R2. Board card rendering optimization

The board must reduce render cost for off-screen cards.

Required behavior:

- Only visible or near-visible cards should mount full interactive card content.
- Card drag registration must be limited to items currently rendered in the active viewport buffer.
- The board must support large columns without mounting thousands of full card components at once.

#### R3. Realtime incremental patching

The board must not default to full refresh on every story event.

Required behavior:

- Story create/update/move/delete websocket events patch local board state when the affected story is already known.
- If a local patch cannot be safely applied, fallback refresh must be scoped to the affected column or current query, not necessarily the whole board.
- Periodic or manual full refresh may remain as a recovery path.

#### R4. Scalable drag-and-drop

The board must support drag-and-drop in large columns without scanning the full dataset on every interaction.

Required behavior:

- Story-to-column lookup should use indexes/maps, not repeated full-array scans.
- Reorder/move operations should operate within loaded windows plus server-backed positions.
- Drag targets must remain correct when columns are partially loaded.

### 9.2 List/Table View Requirements

#### R5. Full-dataset browsing

The list view must support all matching stories, not only a fixed first page.

Required behavior:

- Replace the hardcoded 500-row fetch cap with paginated or cursor-based fetching.
- Continue using row virtualization.
- Support infinite scroll or explicit page loading.
- Preserve current grouping options.

#### R6. Grouping behavior for large data

Grouping must remain correct and performant on large datasets.

Required behavior:

- Grouped lists should operate on the full current query result, not only the first fetched chunk.
- If client-only grouping becomes too expensive, the API must support grouped responses or grouped pagination metadata.
- The UX must clearly communicate when a group is partially loaded.

#### R7. Stable inline edits

Inline edits in list mode must patch local rows and avoid broad refetches unless required.

### 9.3 API Requirements

#### R8. Board summary endpoint

Introduce a board summary contract that returns:

- workflow and state metadata
- filtered story counts per state
- aggregate metrics per state
- first-page story slices per state
- pagination metadata or cursors per state

#### R9. Per-column continuation endpoint

The API must support fetching the next slice for a specific state/column under the current filter set.

#### R10. Stable ordering contract

The API must define how story order is preserved across partial loading, moves, and reorders.

#### R11. Filter-consistent pagination

All pagination and continuation requests must be filter-stable. A column load-more request must apply the exact same active filters as the initial request.

### 9.4 Observability Requirements

#### R12. Performance instrumentation

The frontend and backend must expose enough information to diagnose large-workspace performance issues.

Required behavior:

- Log board payload size and request duration in development and staging.
- Track number of loaded cards/rows client-side in development.
- Add lightweight instrumentation around board initial load and load-more operations.

---

## 10. Proposed Solution

## 10.1 Board Architecture

Replace the current "fetch full board" approach with a two-level model:

### Level 1: Board Summary

Initial load returns:

- workflow
- columns/states
- total filtered count per state
- first N stories per state
- pagination metadata per state

Recommended defaults:

- 30 to 50 stories per column initial load
- smaller initial slice when many columns are open
- zero story payload for collapsed columns until expanded

### Level 2: Column Window Management

Each column owns:

- loaded stories
- total count
- next cursor or page
- loading state
- fully loaded flag

The frontend store should track columns independently instead of replacing the whole board blob every time.

## 10.2 Board Rendering Strategy

Recommended approach:

1. Keep columns horizontally rendered as today.
2. Within each column, virtualize or visibility-gate cards.
3. Use fixed or estimated card heights with measurement fallback if needed.
4. Restrict active drag/drop registration to rendered items.

Two acceptable implementation options:

- Option A: true vertical virtualization per column
- Option B: visibility-gated cards plus incremental column pagination

Recommendation:

- Phase 1 should ship incremental column pagination plus visibility gating.
- Phase 2 may add true per-column virtualization if needed after profiling.

Rationale:

- It reduces risk relative to jumping immediately into fully virtualized DnD.
- It matches the practical strategy used by mature issue trackers more closely than all-at-once rendering.

## 10.3 List/Table Architecture

The list view should combine:

- server pagination or cursoring
- row virtualization
- client-side cached pages
- local row patching for edits

Recommended behavior:

- Load first 100 rows
- Load more as the user nears the end of the loaded window
- Maintain a flattened row cache for the current query
- Preserve grouping state locally where feasible

If client grouping across very large loaded sets becomes expensive, add an API-backed grouped list mode later.

## 10.4 Realtime Model

Replace unconditional board-wide refresh with targeted updates.

Examples:

- `story.updated`: patch story in current column/list cache
- `story.moved`: remove from old loaded column slice, insert into new loaded column slice if visible, adjust counts
- `story.created`: increment relevant count; insert only if the target column slice policy says the new story belongs in the loaded window
- `story.deleted`: remove locally if loaded and decrement counts

Fallback behavior:

- If the client cannot safely reconcile because ordering/filter membership is ambiguous, refetch only the affected column or current list query window.

---

## 11. Detailed Functional Requirements

### 11.1 Board UX

- Show per-column story counts immediately.
- Show loading skeletons inside columns while first slice loads.
- Show "Load more" or auto-load sentinel at the bottom of each column.
- Show a clear "All stories loaded" state when complete.
- Preserve collapsed state across sessions.
- Preserve current filters and views.

### 11.2 List UX

- Keep current group-by selector.
- Show total matching stories, not only loaded stories.
- Indicate loaded count versus total when not fully loaded.
- Support continuous scroll without visible jumps.

### 11.3 Saved Views

Saved views must remain compatible with the new pagination model.

Required behavior:

- Applying a saved view loads the board summary for that filter set.
- Changing filters resets column/list pagination state.
- Discarding changes restores the saved view query and resets loaded windows.

### 11.4 Error Handling

- Column load failures must not blank the full board.
- Failed load-more requests should allow retry at column level.
- Realtime patch failures should gracefully fallback to scoped refetch.

---

## 12. API Design

### 12.1 Proposed Board Summary Response

`GET /api/pm/stories/board`

Query:

- `workspace_id`
- `workflow_id`
- current filters
- optional `per_state_limit`
- optional `include_collapsed=false|true`

Example response shape:

```json
{
  "workflow": { "id": "wf_1", "name": "Default" },
  "columns": [
    {
      "state": { "id": "s_1", "name": "Backlog", "position": 0 },
      "story_count": 2480,
      "point_total": 7312,
      "stories": [],
      "page_size": 0,
      "next_cursor": null,
      "fully_loaded": false
    },
    {
      "state": { "id": "s_2", "name": "In Progress", "position": 1 },
      "story_count": 312,
      "point_total": 921,
      "stories": [{ "...": "..." }],
      "page_size": 30,
      "next_cursor": "cursor_abc",
      "fully_loaded": false
    }
  ]
}
```

### 12.2 Proposed Column Continuation Response

`GET /api/pm/stories/board/columns/{stateId}`

Query:

- `workspace_id`
- `workflow_id`
- current filters
- `cursor` or `page`
- `limit`

Response:

```json
{
  "state_id": "s_2",
  "stories": [{ "...": "..." }],
  "next_cursor": "cursor_def",
  "fully_loaded": false
}
```

### 12.3 Proposed List Endpoint Contract

Continue using `/api/pm/stories`, but standardize for large datasets:

- keep `page/per_page` or migrate to cursoring
- always return `total`
- ensure filters are fully supported
- optionally add `group_by` later if server grouping becomes necessary

Recommendation:

- Keep paginated list endpoint first.
- Do not redesign the list API beyond what is needed for infinite loading unless profiling proves grouping is the bottleneck.

---

## 13. Data Model And Backend Considerations

### 13.1 Query Efficiency

Backend work must include:

- indexed filtering paths for workflow, state, team, epic, sprint, owner, priority, severity, archived, updated_at
- efficient label filtering join strategy
- stable ordering suitable for cursoring or paginated continuation

### 13.2 Ordering Strategy

Current integer `position` ordering is workable, but partial loading requires a clearer contract.

Requirements:

- reorders must remain deterministic within a state
- moves between states must place the story predictably
- continuation requests must use the same ordering as initial load

### 13.3 Count Accuracy

Per-column counts must reflect the full filtered dataset, not only the loaded window.

---

## 14. Frontend Architecture Changes

### 14.1 Store Redesign

Replace monolithic board state with:

- board metadata
- filters and active view
- per-column slices and pagination state
- story index map by ID
- per-column load status

Suggested store shape:

- `columnsByStateId`
- `orderedStateIds`
- `storyIndex`
- `columnPagination`
- `totalCounts`

### 14.2 Lookup Optimization

Introduce O(1) or near-O(1) lookup helpers for:

- story ID to state ID
- story ID to loaded position
- state ID to column metadata

This removes repeated full-array scans during drag events and local patching.

### 14.3 Realtime Update Layer

Create a dedicated reconciliation layer that:

- applies websocket events to the story index and affected column slices
- tracks whether a fallback fetch is required
- prevents repeated duplicate refreshes

### 14.4 Performance Guards

Add development-only warnings when:

- rendered board cards exceed a threshold
- a full board refresh occurs after a single-story event
- a list query result is truncated unexpectedly

---

## 15. Rollout Plan

### Phase 1: Safe Scaling Baseline

Goal: remove the largest blockers quickly.

Deliverables:

- board endpoint supports initial per-column slices
- board UI supports per-column load-more
- collapsed columns skip story payloads
- websocket story events stop forcing whole-board refresh by default
- list view removes fixed 500-row cap and adopts paginated infinite loading

### Phase 2: Rendering Optimization

Goal: reduce UI cost of loaded datasets.

Deliverables:

- visibility-gated or virtualized board cards
- optimized drag/drop indexing
- reduced rerenders in board columns and cards
- memoization or store selectors where profiling justifies them

### Phase 3: Advanced Scaling And Hardening

Goal: productionize behavior under heavy usage.

Deliverables:

- column-level retry and recovery
- performance instrumentation dashboards or logs
- optional server-backed grouped list behavior if profiling requires it
- staging validation with seeded large datasets

---

## 16. Acceptance Criteria

### Board

- Opening a board with a filtered result set above 10,000 stories does not fetch all story cards in one response.
- Expanding or scrolling a large column loads additional slices without blocking the rest of the board.
- Updating a single story does not trigger a whole-board refetch in the common case.
- Dragging a story between loaded columns remains responsive.

### List

- List mode can browse beyond the first 500 stories.
- Row virtualization remains enabled.
- Loaded-row count and total-result count are both visible or derivable.
- Inline row edits do not trigger full list reload by default.

### Backend

- Board endpoint returns counts plus partial slices.
- Column continuation endpoint or equivalent exists and respects all active filters.
- Query performance is acceptable on large filtered datasets in staging.

---

## 17. Testing Strategy

### Backend

- Unit tests for board summary and column continuation filters
- Tests for count correctness under filtered queries
- Tests for move/reorder behavior with partial loading

### Frontend

- Store tests for local patching and column pagination
- Component tests for board load-more behavior
- Component tests for list infinite loading and grouping
- Regression tests for saved views and filter changes resetting pagination

### Staging Validation

- Seed workspace with 10,000+ stories distributed across multiple states
- Validate board open, column scrolling, filtering, list browsing, and move operations
- Validate websocket updates during active board/list sessions

---

## 18. Risks

### Risk 1: DnD plus virtualization complexity

Fully virtualized drag-and-drop on kanban can be fragile.

Mitigation:

- Ship incremental loading first
- Use visibility gating before full virtualization if it meets targets

### Risk 2: Client-side grouping remains expensive

Large grouped list datasets may still be heavy even with row virtualization.

Mitigation:

- Profile after paginated infinite loading
- Add API-backed grouping only if necessary

### Risk 3: Realtime ordering ambiguity

A websocket update may not provide enough data to locally place a story correctly under active filters.

Mitigation:

- Patch locally when safe
- fallback to scoped refetch when ordering/filter membership is uncertain

---

## 19. Open Questions

1. Should board column loading be auto-scroll based, explicit button based, or both?
2. Do we want page-based continuation or opaque cursors for board columns?
3. Should grouped list mode remain client-side for v1 of this work, or do we want to invest in grouped API responses immediately?
4. What is the target desktop hardware baseline for performance acceptance?

Recommendation:

- Use explicit plus sentinel-based column load-more.
- Keep page-based list pagination for now.
- Keep client-side grouping initially.
- Introduce cursoring later only if ordering stability or pagination performance demands it.

---

## 20. Recommendation

The highest-value path is:

1. Replace full-board fetching with per-column initial slices plus continuation.
2. Remove whole-board refreshes for single-story realtime updates.
3. Upgrade list view from "virtualized first 500" to "virtualized full query via incremental fetch."
4. Add board card visibility gating or virtualization only after the data-loading model is fixed.

This sequence addresses the real bottlenecks first. Rendering optimizations alone are not enough if the product still fetches and reconciles the entire dataset on every meaningful action.
