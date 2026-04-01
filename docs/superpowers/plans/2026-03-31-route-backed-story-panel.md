# Route-Backed Story Panel Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the duplicated story page + panel implementation with one canonical, route-backed story detail surface that opens as an overlay, preserves direct URLs, and does not break current PM workflows.

**Architecture:** Keep `/w/:slug/pm/stories/:storyId` as the canonical story route, but stop treating it as a separate full-page implementation. Instead, use a single shared story detail controller + view rendered by a route-backed overlay host. The implementation must first prove the routing/background strategy with TanStack Router in this codebase; if route masking/background routes are awkward or unstable, fall back to an explicit router-plus-background-state adapter while preserving the same product behavior.

**Tech Stack:** React 19, TanStack Router, TanStack Query, Zustand (only where still justified), TypeScript, shadcn/ui, existing PM services/components.

---

## File Structure

### Existing files to modify
- `frontend/src/components/pm/StoryDetailPanel.tsx`
  Thin shell only. Remove embedded data orchestration and duplicated business logic. Keep sheet/modal chrome and shell-only concerns.
- `frontend/src/pages/pm/StoryDetail.tsx`
  Delete after migration. During transition, use only as extraction reference.
- `frontend/src/components/pm/GlobalStoryPanel.tsx`
  Replace with a route-backed overlay host or retire after host migration.
- `frontend/src/stores/storyPanelStore.ts`
  Deprecate and remove once all open/close flows are route-backed.
- `frontend/src/routes/_authenticated/w/$slug.tsx`
  Mount the new global route-backed story overlay host at workspace layout level.
- `frontend/src/routes/_authenticated/w/$slug/pm/stories/index.tsx`
  Keep as stories background route and validate search preservation.
- `frontend/src/routes/_authenticated/w/$slug/pm/stories/$storyId.tsx`
  Stop rendering a separate full-page editor. Render fallback background content only.
- `frontend/src/lib/pmStoryLinks.ts`
  Centralize canonical path building plus new route-backed overlay navigation helpers.
- `frontend/src/components/pm/KanbanBoard.tsx`
  Replace store-based `openStoryPanel` calls with route-backed story navigation.
- `frontend/src/pages/pm/Sprints.tsx`
  Replace store-based story-open behavior with route-backed story navigation.
- `frontend/src/pages/pm/SprintDetail.tsx`
  Replace store-based story-open behavior with route-backed story navigation.
- `frontend/src/pages/pm/EpicDetail.tsx`
  Replace store-based story-open behavior with route-backed story navigation.
- `frontend/src/pages/pm/MyWork.tsx`
  Replace store-based story-open behavior with route-backed story navigation.
- `frontend/src/pages/notifications/NotificationsPage.tsx`
  Use the canonical story route helper so notifications open the same overlay-backed route.
- `frontend/src/components/support/SidebarAssociations.tsx`
  Use the same story route helper.
- `frontend/src/components/crm/AssociationsList.tsx`
  Use the same story route helper.
- `frontend/src/components/pm/RecurringTemplatesSettings.tsx`
  Use the same story route helper for run-history story links.
- `frontend/src/components/pm/StoryRelationshipsSection.tsx`
  Story-to-story navigation inside the open story overlay must swap stories without dropping background context.

### New files to create
- `frontend/src/components/pm/story-detail/storyRouteStrategy.ts`
  Small adapter that encapsulates the chosen background-route strategy after feasibility validation, so the rest of the migration does not depend directly on one TanStack Router mechanism.
- `frontend/src/components/pm/story-detail/__tests__/storyRouteStrategy.test.tsx`
  Feasibility and regression tests for the chosen background-route approach.
- `frontend/src/components/pm/story-detail/useStoryDetailController.ts`
  Single source of truth for story loading, autosave queueing, mutation orchestration, recurring state, activity/comments reload, attachment cleanup, archive handling, and compatibility events.
- `frontend/src/components/pm/story-detail/StoryDetailView.tsx`
  Shared story detail body used by both overlay shell and any fallback shell. Contains title, description, metadata, relationships, attachments, checklist, external links, comments, activity, recurring UI, delivery, Git, agent runs.
- `frontend/src/components/pm/story-detail/StoryDetailOverlayHost.tsx`
  Workspace-level host that inspects the router state, detects the active canonical story route, loads the story via the shared controller, and renders the story overlay.
- `frontend/src/components/pm/story-detail/storyRouteNavigation.ts`
  Central helper for “open story as overlay from current route”, close behavior, fallback navigation, and canonical new-tab/copy-link generation.
- `frontend/src/components/pm/story-detail/StoryRouteFallbackBackground.tsx`
  Minimal wrapper for direct story-route visits that renders the stories index background while the overlay host renders the active story.
- `frontend/src/components/pm/story-detail/__tests__/storyRouteNavigation.test.ts`
  Tests for route helper behavior.
- `frontend/src/components/pm/story-detail/__tests__/StoryDetailOverlayHost.test.tsx`
  Tests for overlay host behavior with direct route vs masked/background route.
- `frontend/src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx`
  Targeted behavior tests for autosave queueing / story switching / archive-close behavior.

### Existing tests to modify or add
- Add route integration tests near:
  - `frontend/src/routes/...` test pattern if one exists
  - otherwise colocate under `frontend/src/components/pm/story-detail/__tests__/`
- Add regression tests covering:
  - open from board keeps board visible under overlay
  - direct `/pm/stories/:id` loads stories background + overlay
  - close returns to previous view if opened from background route
  - linked story navigation inside overlay swaps story and keeps background route
  - archive from overlay closes and emits existing compatibility event

---

## Task 1: Prove The Route Strategy Before Refactoring

**Files:**
- Create: `frontend/src/components/pm/story-detail/storyRouteStrategy.ts`
- Test: `frontend/src/components/pm/story-detail/__tests__/storyRouteStrategy.test.tsx`
- Reference: `frontend/src/routes/_authenticated/w/$slug.tsx`, `frontend/src/routes/_authenticated/w/$slug/pm/stories/index.tsx`, `frontend/src/routes/_authenticated/w/$slug/pm/stories/$storyId.tsx`

- [ ] **Step 1: Write the failing route-strategy tests**

Test cases to add:
- `openStoryFromContext_returnsCanonicalStoryRouteWithBackgroundMask`
- `closeStoryOverlay_returnsToBackgroundRouteWhenMaskPresent`
- `directStoryRoute_usesStoriesIndexAsFallbackBackground`
- `refreshOnCanonicalStoryRoute_restoresOverlayWithoutPriorBackground`

- [ ] **Step 2: Run the tests to verify they fail**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/storyRouteStrategy.test.tsx
```

Expected:
- FAIL because the strategy adapter does not exist yet.

- [ ] **Step 3: Implement the smallest viable strategy adapter**

Implement one adapter that hides whether the app uses:
- TanStack Router route masking/background locations directly
- or a small explicit background-route state bridge

Success criteria:
- in-app open keeps current page as background
- close returns to that background
- direct entry and refresh fall back cleanly to stories index
- no product code outside the adapter should need to know which strategy won

- [ ] **Step 4: Run tests to verify the strategy**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/storyRouteStrategy.test.tsx
```

Expected:
- PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/pm/story-detail/storyRouteStrategy.ts frontend/src/components/pm/story-detail/__tests__/storyRouteStrategy.test.tsx
git commit -m "feat: prove story route background strategy"
```

---

## Task 2: Lock The Current Behavior And Regression Matrix

**Files:**
- Modify: `docs/superpowers/plans/2026-03-31-route-backed-story-panel.md`
- Test: `frontend/src/components/pm/story-detail/__tests__/storyRouteNavigation.test.ts`
- Test: `frontend/src/components/pm/story-detail/__tests__/StoryDetailOverlayHost.test.tsx`

- [ ] **Step 1: Write the manual regression checklist in this plan**

Document concrete pre/post behaviors to preserve:
- board -> open story -> edit -> close -> board still intact
- sprints planning/detail -> open story -> close -> same sprint context preserved
- epic detail -> open story -> close -> same epic context preserved
- my work -> open story -> close -> same section preserved
- direct `/pm/stories/:id` URL -> loads and is shareable
- refresh on `/pm/stories/:id` -> still opens story
- notification/support/CRM story links -> still work
- relationships links -> still open the target story
- copy link -> still copies canonical route

- [ ] **Step 2: Write failing route/navigation tests**

Test cases to add:
- `openStoryFromContext_usesCanonicalRouteHelper`
- `closeStoryOverlay_preservesBackgroundRoute`
- `directStoryRoute_usesFallbackBackgroundShell`

- [ ] **Step 3: Run the new tests to verify they fail**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/storyRouteNavigation.test.ts src/components/pm/story-detail/__tests__/StoryDetailOverlayHost.test.tsx
```

Expected:
- FAIL because the helper/host are not implemented yet.

- [ ] **Step 4: Commit the failing tests**

```bash
git add docs/superpowers/plans/2026-03-31-route-backed-story-panel.md frontend/src/components/pm/story-detail/__tests__/
git commit -m "test: add story route-backed overlay regressions"
```

---

## Task 3: Build Canonical Story Route Navigation Helpers

**Files:**
- Create: `frontend/src/components/pm/story-detail/storyRouteNavigation.ts`
- Modify: `frontend/src/lib/pmStoryLinks.ts`
- Modify: `frontend/src/components/pm/story-detail/storyRouteStrategy.ts`
- Test: `frontend/src/components/pm/story-detail/__tests__/storyRouteNavigation.test.ts`

- [ ] **Step 1: Write the failing helper-level tests**

Cover:
- canonical path builder still returns `/w/:slug/pm/stories/:storyId`
- open helper delegates through the validated route strategy adapter
- close helper delegates through the validated route strategy adapter
- fallback route remains `/w/:slug/pm/stories`

- [ ] **Step 2: Run the helper tests and verify failure**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/storyRouteNavigation.test.ts
```

Expected:
- FAIL because the new helper API is not implemented yet.

- [ ] **Step 3: Implement minimal navigation helpers**

Implement:
- `buildStoryPath(slug, storyId)` remains canonical
- `openStoryRoute(router, { slug, storyId })`
- `closeStoryRoute(router, { slug, fallbackSearch? })`
- `buildCanonicalStoryUrl(...)`

Rules:
- one canonical route only
- preserve current pathname + search as background using the validated strategy adapter
- use fallback `/w/:slug/pm/stories` when direct-entry has no background
- do not duplicate URL-building logic across callers

- [ ] **Step 4: Run tests to green**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/storyRouteNavigation.test.ts
```

Expected:
- PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/pm/story-detail/storyRouteNavigation.ts frontend/src/lib/pmStoryLinks.ts frontend/src/components/pm/story-detail/__tests__/storyRouteNavigation.test.ts
git commit -m "feat: add canonical story route navigation helpers"
```

---

## Task 4: Extract Shared Story Detail Controller Phase 1 (Loading And Read State)

**Files:**
- Create: `frontend/src/components/pm/story-detail/useStoryDetailController.ts`
- Test: `frontend/src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx`
- Reference only: `frontend/src/components/pm/StoryDetailPanel.tsx`, `frontend/src/pages/pm/StoryDetail.tsx`

- [ ] **Step 1: Write the failing controller tests**

Cover:
- loads story detail + workflow states + recurring summary
- switching from story A to story B resets transient UI state correctly
- read-only section visibility and derived view state match current behavior

- [ ] **Step 2: Run tests and verify failure**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx
```

Expected:
- FAIL because controller doesn’t exist yet.

- [ ] **Step 3: Implement minimal controller**

Move only shared read/loading logic out of both old surfaces:
- story loading
- comments/activity reload
- recurring summary loading
- Git-integration capability check

Do **not** extract autosave/mutation orchestration in this phase.
One controller should expose:
- `storyDetail`
- `form`
- `reload` helpers
- UI section booleans
- read-model helpers

- [ ] **Step 4: Run tests to green**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx
```

Expected:
- PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/pm/story-detail/useStoryDetailController.ts frontend/src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx
git commit -m "refactor: extract shared story detail controller"
```

---

## Task 5: Extract Shared Story Detail Controller Phase 2 (Autosave And Mutations)

**Files:**
- Modify: `frontend/src/components/pm/story-detail/useStoryDetailController.ts`
- Test: `frontend/src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx`
- Reference only: `frontend/src/components/pm/StoryDetailPanel.tsx`, `frontend/src/pages/pm/StoryDetail.tsx`

- [ ] **Step 1: Write the failing autosave/mutation tests**

Cover:
- queues patches and applies autosave serially
- attachment image cleanup stays correct
- keeps compatibility events:
  - `story-panel-updated`
  - `story-panel-archived`
- archive action closes/removes current story cleanly

- [ ] **Step 2: Run tests and verify failure**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx
```

Expected:
- FAIL on autosave/mutation assertions.

- [ ] **Step 3: Implement minimal mutation orchestration**

Move:
- autosave patch queue
- attachment image cleanup
- archive handling
- copy-link support inputs
- mutation handlers

Do not add new behaviors. Match current panel behavior exactly.

- [ ] **Step 4: Run tests to green**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx
```

Expected:
- PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/pm/story-detail/useStoryDetailController.ts frontend/src/components/pm/story-detail/__tests__/useStoryDetailController.test.tsx
git commit -m "refactor: move story autosave and mutation logic into shared controller"
```

---

## Task 6: Extract One Shared Story Detail View

**Files:**
- Create: `frontend/src/components/pm/story-detail/StoryDetailView.tsx`
- Modify: `frontend/src/components/pm/StoryDetailPanel.tsx`
- Reference: `frontend/src/pages/pm/StoryDetail.tsx`
- Test: `frontend/src/components/pm/story-detail/__tests__/StoryDetailView.test.tsx`

- [ ] **Step 1: Write the failing shared-view tests**

Cover:
- renders title/description/metadata
- renders relationships/associations
- renders attachments/checklist/external links
- renders comments + activity
- renders delivery + Git + agent runs when enabled
- shell-specific controls remain outside the shared view

- [ ] **Step 2: Run tests and verify failure**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/StoryDetailView.test.tsx
```

Expected:
- FAIL because shared view doesn’t exist yet.

- [ ] **Step 3: Implement the shared view**

Rules:
- migrate content from `StoryDetailPanel.tsx` and `StoryDetail.tsx` into one shared view
- the view owns no route-specific navigation assumptions
- the view accepts props from the controller
- keep existing UI components and styling; do not redesign the story UI

- [ ] **Step 4: Slim `StoryDetailPanel.tsx` into a shell**

Keep only:
- `Sheet` / open-close chrome
- panel sizing / header controls
- overlay-specific close behavior
- optional shell-only actions like open-in-new-tab

Everything else should delegate to:
- `useStoryDetailController`
- `StoryDetailView`

- [ ] **Step 5: Run tests**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/StoryDetailView.test.tsx
```

Expected:
- PASS

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/pm/story-detail/StoryDetailView.tsx frontend/src/components/pm/StoryDetailPanel.tsx frontend/src/components/pm/story-detail/__tests__/StoryDetailView.test.tsx
git commit -m "refactor: consolidate story detail into shared view"
```

---

## Task 7: Pilot The Route-Backed Overlay With One Origin Surface

**Files:**
- Create: `frontend/src/components/pm/story-detail/StoryDetailOverlayHost.tsx`
- Create: `frontend/src/components/pm/story-detail/StoryRouteFallbackBackground.tsx`
- Modify: `frontend/src/routes/_authenticated/w/$slug.tsx`
- Modify: `frontend/src/routes/_authenticated/w/$slug/pm/stories/$storyId.tsx`
- Modify: `frontend/src/components/pm/GlobalStoryPanel.tsx`
- Modify: `frontend/src/stores/storyPanelStore.ts`
- Modify: `frontend/src/components/pm/KanbanBoard.tsx`
- Test: `frontend/src/components/pm/story-detail/__tests__/StoryDetailOverlayHost.test.tsx`
- Test: `frontend/src/components/pm/story-detail/__tests__/StoryRouteOrigins.test.tsx`

- [ ] **Step 1: Write the failing pilot tests**

Cover:
- board open uses canonical story route
- board stays mounted under the overlay
- close from overlay returns to the same board context
- direct `/pm/stories/:id` still works via fallback background

- [ ] **Step 2: Run tests and verify failure**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/StoryDetailOverlayHost.test.tsx src/components/pm/story-detail/__tests__/StoryRouteOrigins.test.tsx
```

Expected:
- FAIL because the route-backed host and board pilot are not implemented yet.

- [ ] **Step 3: Implement the overlay host**

Responsibilities:
- detect active story route
- load data through `useStoryDetailController`
- render `StoryDetailPanel` as the only story detail shell
- choose fallback background when direct route has no background
- close action uses route helper, not Zustand store

- [ ] **Step 4: Rewire workspace layout and story route**

In `frontend/src/routes/_authenticated/w/$slug.tsx`:
- replace `GlobalStoryPanel` with the new host
- ensure it mounts once at workspace layout level

In `frontend/src/routes/_authenticated/w/$slug/pm/stories/$storyId.tsx`:
- remove `StoryDetailPage` rendering
- render fallback stories background only
- let the global overlay host render the active story

- [ ] **Step 5: Migrate board as the pilot origin**

In `frontend/src/components/pm/KanbanBoard.tsx`:
- replace store-based opening with the route helper
- preserve current board context as background route

- [ ] **Step 6: Keep compatibility temporarily**

If needed during rollout:
- keep `GlobalStoryPanel` as a small adapter that forwards to route state
- keep `storyPanelStore` only until pilot verification is complete

- [ ] **Step 7: Run tests to green**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/StoryDetailOverlayHost.test.tsx src/components/pm/story-detail/__tests__/StoryRouteOrigins.test.tsx
```

Expected:
- PASS

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/pm/story-detail/StoryDetailOverlayHost.tsx frontend/src/components/pm/story-detail/StoryRouteFallbackBackground.tsx frontend/src/routes/_authenticated/w/$slug.tsx frontend/src/routes/_authenticated/w/$slug/pm/stories/$storyId.tsx frontend/src/components/pm/GlobalStoryPanel.tsx frontend/src/stores/storyPanelStore.ts frontend/src/components/pm/KanbanBoard.tsx frontend/src/components/pm/story-detail/__tests__/StoryDetailOverlayHost.test.tsx frontend/src/components/pm/story-detail/__tests__/StoryRouteOrigins.test.tsx
git commit -m "feat: pilot route-backed story overlay from board"
```

---

## Task 8: Migrate Remaining Story Open Entry Points To The Canonical Route Helper

**Files:**
- Modify: `frontend/src/pages/pm/Sprints.tsx`
- Modify: `frontend/src/pages/pm/SprintDetail.tsx`
- Modify: `frontend/src/pages/pm/EpicDetail.tsx`
- Modify: `frontend/src/pages/pm/MyWork.tsx`
- Modify: `frontend/src/pages/notifications/NotificationsPage.tsx`
- Modify: `frontend/src/components/support/SidebarAssociations.tsx`
- Modify: `frontend/src/components/crm/AssociationsList.tsx`
- Modify: `frontend/src/components/pm/RecurringTemplatesSettings.tsx`
- Modify: `frontend/src/components/pm/StoryRelationshipsSection.tsx`
- Test: route/navigation integration tests

- [ ] **Step 1: Write one integration test per origin class**

Cover at minimum:
- sprint/my-work origin
- relationship link inside story overlay
- direct notification link

- [ ] **Step 2: Run tests and verify failure**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/StoryRouteOrigins.test.tsx
```

Expected:
- FAIL because callers still use the old store or raw route navigation.

- [ ] **Step 3: Replace store-based openings with route helper**

Rules:
- every in-app story open should go through one helper
- preserve current path + search as background route
- do not manually hand-build route objects at each call site

- [ ] **Step 4: Migrate story-to-story links inside the open overlay**

Important:
- opening a related story from `StoryRelationshipsSection` should replace the active story route while keeping the same background route
- it must not drop users onto the standalone fallback unless there is no background

- [ ] **Step 5: Migrate direct links**

Notifications/support/CRM:
- use canonical route builder
- if launched from current workspace context, still prefer overlay
- direct navigation remains valid on refresh/open in new tab

- [ ] **Step 6: Run tests to green**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/StoryRouteOrigins.test.tsx
```

Expected:
- PASS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/pm/Sprints.tsx frontend/src/pages/pm/SprintDetail.tsx frontend/src/pages/pm/EpicDetail.tsx frontend/src/pages/pm/MyWork.tsx frontend/src/pages/notifications/NotificationsPage.tsx frontend/src/components/support/SidebarAssociations.tsx frontend/src/components/crm/AssociationsList.tsx frontend/src/components/pm/RecurringTemplatesSettings.tsx frontend/src/components/pm/StoryRelationshipsSection.tsx frontend/src/components/pm/story-detail/__tests__/StoryRouteOrigins.test.tsx
git commit -m "refactor: route all story openings through canonical overlay route"
```

---

## Task 9: Remove The Separate Story Page Implementation

**Files:**
- Delete: `frontend/src/pages/pm/StoryDetail.tsx`
- Modify: any imports/references still pointing to it
- Test: route + story detail regression tests

- [ ] **Step 1: Search for remaining `StoryDetailPage` references**

Run:
```bash
cd frontend
rg -n "StoryDetailPage|pages/pm/StoryDetail" src
```

Expected:
- only legacy references remain.

- [ ] **Step 2: Remove the page implementation**

Delete the file only after:
- overlay host is green
- all callers use route-backed navigation
- direct story route works with fallback background

- [ ] **Step 3: Re-run typecheck**

Run:
```bash
cd frontend
npm exec tsc --noEmit
```

Expected:
- PASS

- [ ] **Step 4: Commit**

```bash
git add -u frontend/src/pages/pm/StoryDetail.tsx frontend/src/routes/_authenticated/w/$slug/pm/stories/$storyId.tsx
git commit -m "refactor: remove standalone story detail page"
```

---

## Task 10: Clean Up Transitional State And Compatibility Layers

**Files:**
- Modify/Delete: `frontend/src/stores/storyPanelStore.ts`
- Modify/Delete: `frontend/src/components/pm/GlobalStoryPanel.tsx`
- Modify: any remaining `story-panel-*` event emitters/listeners if still required
- Test: targeted compatibility tests

- [ ] **Step 1: Audit remaining store usage**

Run:
```bash
cd frontend
rg -n "useStoryPanelStore|story-panel-updated|story-panel-archived" src
```

- [ ] **Step 2: Remove unused store code**

If nothing depends on it:
- delete `storyPanelStore.ts`
- delete `GlobalStoryPanel.tsx`

If some external listeners still depend on events:
- keep events in the controller for one release
- do not keep the store if route state fully replaces it

- [ ] **Step 3: Run tests + typecheck**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/*.test.ts*
npm exec tsc --noEmit
```

Expected:
- PASS

- [ ] **Step 4: Commit**

```bash
git add -u frontend/src/stores/storyPanelStore.ts frontend/src/components/pm/GlobalStoryPanel.tsx
git commit -m "chore: remove legacy story panel state"
```

---

## Task 11: Full Verification And Manual QA

**Files:**
- No product code changes expected unless regressions are found

- [ ] **Step 1: Run focused frontend verification**

Run:
```bash
cd frontend
npm exec vitest run src/components/pm/story-detail/__tests__/*.test.ts*
npm exec eslint src/components/pm/StoryDetailPanel.tsx src/components/pm/story-detail src/routes/_authenticated/w/$slug.tsx src/routes/_authenticated/w/$slug/pm/stories/index.tsx src/routes/_authenticated/w/$slug/pm/stories/$storyId.tsx src/components/pm/KanbanBoard.tsx src/pages/pm/Sprints.tsx src/pages/pm/SprintDetail.tsx src/pages/pm/EpicDetail.tsx src/pages/pm/MyWork.tsx src/pages/notifications/NotificationsPage.tsx src/components/support/SidebarAssociations.tsx src/components/crm/AssociationsList.tsx src/components/pm/RecurringTemplatesSettings.tsx src/components/pm/StoryRelationshipsSection.tsx
npm exec tsc --noEmit
```

Expected:
- all pass

- [ ] **Step 2: Manual QA matrix**

Verify:
- board -> story opens overlay, URL updates, close returns to board
- sprint detail -> story opens overlay, close returns to sprint detail
- epic detail -> story opens overlay, close returns to epic detail
- my work -> story opens overlay, close returns to my work
- direct `/w/:slug/pm/stories/:id` -> stories background + overlay
- refresh on story URL -> overlay remains open
- related story link inside overlay -> swaps story, same background persists
- notifications/support/CRM story links -> open canonical overlay route
- copy link/open in new tab -> open canonical story URL correctly
- archive from overlay -> closes story and removes it from prior view
- browser back/forward navigation -> story overlay follows history properly

- [ ] **Step 3: Final commit if QA fixes were needed**

```bash
git add <files>
git commit -m "fix: polish route-backed story overlay regressions"
```

---

## Critical Implementation Notes

- Do not redesign the story UI. Preserve existing styling and components.
- Do not ship a half-migrated state where some flows use store-driven panel and others use route-backed overlay unless the compatibility layer is explicit and temporary.
- Keep the canonical story route path unchanged to avoid breaking links.
- Preserve copy-link behavior and direct-link shareability throughout the migration.
- Preserve current compatibility events until all downstream listeners are audited.
- Do not proceed past Task 1 until the route/background strategy is proven in tests; if that cannot be made reliable, revise the implementation approach before touching the detail surfaces.
- Treat `useStoryDetailController` extraction as the highest-risk step. Use the current panel behavior as the source-of-truth baseline, and move read logic before mutation logic.
- Do not migrate every caller at once. Board is the pilot surface; only after that path is stable should the rest of the origins switch over.
- The “maximize” affordance in the panel should be reconsidered:
  - likely replace with “Open in new tab”
  - or remove entirely once the route is already canonical
- If TanStack Router route masking APIs differ from expectations, adapt the strategy adapter but keep the product contract:
  - in-app open => overlay over current background
  - direct link/refresh => fallback background + overlay

## Rollback Plan

If the route strategy proves unstable during Task 1:
- stop and revise the plan before continuing
- prefer a small explicit background-route adapter over pushing ahead with brittle masking assumptions

If overlay routing becomes unstable later during implementation:
- keep the shared controller + shared view extraction
- restore the old page route temporarily
- keep panel openings on the new shared implementation
- do **not** ship until direct links and close/back behavior are correct

The route-backed overlay itself is required for completion. Shared implementation alone is not the end state.
