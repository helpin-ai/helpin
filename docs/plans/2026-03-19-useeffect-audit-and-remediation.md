# useEffect Audit and Remediation Plan

> **For agentic workers:** REQUIRED: Use superpowers:executing-plans or superpowers:subagent-driven-development before implementing this plan. This document is an audit plus a prioritized remediation plan, not a mandate to delete every `useEffect`.

**Goal:** Reduce render-after-render synchronization, manual async state machines, and prop-to-state mirroring across the frontend and widget packages. Replace those cases with the owning primitive: TanStack Query for server data, TanStack Router for route state, derived values for pure computations, keyed remounts or form resets for draft state, and `useSyncExternalStore` for browser-history subscriptions.

**Architecture:** 
- Server state should live in TanStack Query, not in `useEffect` plus `useState`.
- Route/search-param state should come from TanStack Router, not `window.location` or post-render store sync.
- Local component state should only own mutable drafts, not mirror props/query data by default.
- Pure derivations should use `useMemo` or inline computation, not an effect that writes another piece of state.
- Dialog reset and focus behavior should prefer event handlers, keyed children, or autofocus refs over “watch `open` in an effect”.

**Tech Stack:** React 19, TanStack Router, TanStack Query, Zustand, TypeScript 5.9, Preact (widget packages)

---

## Audit Summary

This audit only marks **high-confidence** areas where `useEffect` is doing the wrong job or where the codebase already has a better primitive available.

### Effects that should mostly stay

These are not the target of this plan unless a concrete bug appears:

- `frontend/src/hooks/useTitle.ts`
- `frontend/src/hooks/useWebSocket.ts`
- `frontend/src/hooks/useRealtimeSync.ts`
- `frontend/src/hooks/usePlanningStream.ts`
- `frontend/src/hooks/use-mobile.ts`
- keyboard/subscription effects in layout components
- image loading / resize / fullscreen listener effects
- debounced autosave timers in detail pages for now

The plan below focuses on the cases that are currently creating avoidable complexity, duplicate caches, or delayed state synchronization.

---

## Area 1: Manual Remote Fetching In useEffect

These files are using `useEffect` to implement a query/cache/loading machine that should instead live in TanStack Query or a shared async resource hook.

### High-priority files

- `frontend/src/hooks/useAssignableWorkspaceMembers.ts`
  Better pattern: `useQuery` with `queryKey`, `staleTime`, `placeholderData`, and invalidation instead of module-level cache + `fetchPromise`.
- `frontend/src/hooks/useWorkspaceMembers.ts`
  Better pattern: same as above.
- `frontend/src/hooks/useWorkspaceTeams.ts`
  Better pattern: `useQuery` for settings-derived team data plus `queryClient.invalidateQueries` instead of module cache + manual listener fanout.
- `frontend/src/pages/Settings.tsx`
  Better pattern: consume `useWorkspaceSettings` directly and invalidate on mutation instead of `load()` inside an effect.
- `frontend/src/components/pm/RecurringTemplatesSettings.tsx`
  Better pattern: query keyed by `workspaceId`, `status`, `teamFilter`, and `search`.
- `frontend/src/pages/docs/SharedDocumentView.tsx`
  Better pattern: TanStack Router loader or `useQuery` keyed by `shareToken`.
- `frontend/src/pages/pm/MyWork.tsx`
  Better pattern: query keyed by `workspaceId`, `memberId`, and `mode`; invalidate/refetch from mutations or events instead of `refreshKey`.
- `frontend/src/pages/pm/Roadmap.tsx`
  Better pattern: query keyed by roadmap filters with invalidation on epic changes.
- `frontend/src/pages/pm/StoryDetail.tsx`
  Better pattern: split the large “load everything in parallel” effect into route/query hooks for story detail, comments, activity, checklist items, external links, labels, epics, and sprints.
- `frontend/src/components/pm/StoryDetailPanel.tsx`
  Better pattern: same split as the full-page detail, but keyed by `storyId`/panel-open state.
- `frontend/src/components/pm/GlobalStoryPanel.tsx`
  Better pattern: `enabled: open && !!storyId` queries or prefetch-on-open instead of fetching story/workflows/recurring summary inside an effect.
- `frontend/src/components/pm/StoryDeliveryPanel.tsx`
  Better pattern: `useQueries` or a dedicated `useStoryDeliveryData` hook instead of one effect loading agents, repos, and delivery target.
- `frontend/src/pages/AccountSettings.tsx`
  Better pattern: org-members query hook rather than `organizationsService.listMembers` inside an effect.
- `frontend/src/components/workspace/WorkspaceSelector.tsx`
  Better pattern: batch member-count/member-preview query or `useQueries`; current effect creates an N+1 fetch pattern across all workspaces.

### Settings/management screens that should move to query hooks

- `frontend/src/components/settings/AutomationsTab.tsx`
- `frontend/src/components/settings/HelpcenterTab.tsx`
- `frontend/src/components/settings/ImportTab.tsx`
- `frontend/src/components/settings/HelpCenterImportSection.tsx`
- `frontend/src/components/settings/WorkflowManager.tsx`
- `frontend/src/components/settings/TeamsTab.tsx`
  Better pattern: colocated query hooks per screen or dialog, plus mutation invalidation instead of effect-driven bootstrap loads.

### Widget/package fetchers that should use a shared async resource primitive

- `packages/widget-core/src/components/HelpArticleView.tsx`
- `packages/widget-core/src/components/HelpCollectionView.tsx`
- `packages/widget-core/src/components/HelpSpaceView.tsx`
  Better pattern: shared `useAsyncResource` / `useHelpResource` hook so the loading/error/cancel logic is not repeated per view.

---

## Area 2: Route, URL, And Store Sync Happening After Render

These effects are syncing state after the component has already rendered. The source of truth should be route state, query data, or store initialization at the load boundary.

### Files

- `frontend/src/routes/_authenticated/w/$slug.tsx`
  Current issue: query data is written into Zustand stores via effects.
  Better pattern: initialize store/context at the route boundary or remove the store mirror entirely where query data is already sufficient.
- `frontend/src/pages/Workspaces.tsx`
  Current issue: auto-selecting organization and auto-opening the create dialog are effect-driven.
  Better pattern: move selection bootstrapping to store/router initialization; derive create-dialog open state from router search.
- `frontend/src/pages/docs/DocsSpaceDetail.tsx`
  Current issue: `collection` URL search param is mirrored into local state.
  Better pattern: read directly from TanStack Router search params.
- `frontend/src/components/pm/KanbanBoard.tsx`
  Current issue: URL team param is copied into the board store with an effect, and `window.location.search` is read directly for `?story=`.
  Better pattern: route search params should drive the board/store directly, and story-opening should be handled by router search state.
- `frontend/src/components/pm/StoryDetailPanel.tsx`
  Current issue: the panel writes `?story=` into `window.history` in an effect.
  Better pattern: route search state should own the open-story param.

### Package-level subscription cleanup

- `packages/react/src/usePageView.ts`
- `packages/nextjs/src/usePageView.ts`
  Current issue: both hooks monkeypatch `history.pushState`/`replaceState` in an effect and manually bridge URL changes into component state.
  Better pattern: a `useSyncExternalStore`-based history subscription or explicit router integration per package.

---

## Area 3: Prop Or Query Data Mirrored Into Local Form State

These are the classic “sync props into state in an effect” cases. They often work, but they create edge cases around stale drafts, reopen/reset behavior, and background refetches.

### Docs flows

- `frontend/src/components/docs/SpaceDialog.tsx`
- `frontend/src/components/docs/CreateCollectionDialog.tsx`
- `frontend/src/components/docs/MoveDocumentDialog.tsx`
- `frontend/src/pages/docs/DocsDocumentDetail.tsx`

Better pattern:
- key the dialog/form by `space?.id`, `collection?.id`, or `docId`
- use form-library reset APIs or a reducer with an explicit `RESET_FROM_SOURCE`
- only keep true local draft state, not a mirrored copy of props

### Settings forms

- `frontend/src/components/settings/GeneralTab.tsx`
- `frontend/src/pages/AccountSettings.tsx`
- `frontend/src/components/settings/ChatGeneralTab.tsx`
- `frontend/src/components/settings/ChatAITab.tsx`
- `frontend/src/components/settings/CRMAutonomySettingsTab.tsx`
- `frontend/src/components/settings/CRMEmailSettingsTab.tsx`
- `frontend/src/components/settings/AITab.tsx`
- `frontend/src/components/settings/SystemTab.tsx`
- `frontend/src/components/settings/TeamsTab.tsx`

Better pattern:
- extracted form components keyed by the source entity/version
- `react-hook-form` `reset()` for query-backed forms
- field initialization at form creation, not via “when data changes, overwrite local state”

### PM forms and inline editors

- `frontend/src/components/pm/ViewBar.tsx`
- `frontend/src/pages/pm/Labels.tsx`
- `frontend/src/components/pm/LabelsSettings.tsx`
- `frontend/src/components/pm/CreateStoryModal.tsx`
- `frontend/src/pages/pm/ObjectiveDetail.tsx` (`KeyResultRow`)

Better pattern:
- key form/dialog children on `open + entityId`
- perform defaulting in the event handler that changes the controlling field
- use reducer/form reset instead of multiple sync effects

### CRM detail screens

- `frontend/src/pages/crm/ContactDetail.tsx`
- `frontend/src/pages/crm/DealDetail.tsx`
- `frontend/src/pages/crm/CompanyDetail.tsx`

Better pattern:
- separate “server snapshot” and “draft” ownership
- initialize draft once per entity/version rather than mirroring query data after every render

---

## Area 4: Derived State Or Cross-Field Effects That Should Be Computation Or Event Logic

These effects are not fetching data, but they still should not exist because they compute one state from another or trigger business logic after render.

### Files

- `frontend/src/pages/pm/StoryDetail.tsx`
  - workflow states are derived from `workflows + storyDetail`
  - checklist/external-link visibility is derived from fetched lists
  - Git integration availability should be its own query hook
  - team-change label pruning/sync should happen inside the team change handler, not a watcher effect
- `frontend/src/components/pm/StoryDetailPanel.tsx`
  Same set of issues as the full-page detail.
- `frontend/src/components/pm/CreateStoryModal.tsx`
  - default story type follows team type
  - label pruning follows team choice
  Better pattern: reducer or explicit `handleTeamChange`.
- `frontend/src/components/settings/TeamsTab.tsx`
  - `defaultStoryType` follows `teamType`
  Better pattern: update both fields in the team-type change handler.

---

## Area 5: Local Copies Of Server Arrays Used As Editable Working State

These tables are copying server data into local state just so cells can edit optimistically. That makes refetches, selection, and rollback logic harder than necessary.

### Files

- `frontend/src/components/crm/CompaniesTable.tsx`
- `frontend/src/components/crm/DealsTable.tsx`
- `frontend/src/components/crm/ContactsTable.tsx`
- `frontend/src/components/pm/StoryListView.tsx` (the `externalStories` sync path is a smaller version of the same pattern)

Better pattern:
- update TanStack Query cache optimistically
- keep only row-level draft/edit state locally
- avoid a full prop-to-state mirror unless the component truly owns the dataset

---

## Area 6: Dialog/Search Reset And Focus Effects That Should Be Event-Driven

These effects are used only to notice “dialog opened/closed” after render and then reset local state or focus an input.

### Files

- `frontend/src/components/search/SearchCommandPalette.tsx`
- `frontend/src/components/pm/AssociationsPanel.tsx`
- `frontend/src/components/support/SidebarAssociations.tsx`
- `frontend/src/components/docs/DocumentLinksPanel.tsx`
- `packages/widget-core/src/components/ChatWindow.tsx`

Better pattern:
- clear state directly in `onOpenChange(false)` or the event handler that closes the surface
- use `autoFocus`, callback refs, or component keys for focus/reset
- make controlled/uncontrolled view ownership explicit instead of syncing `initialView` after render

---

## Prioritization

### P1: Highest ROI / lowest ambiguity

- Convert manual data-loading hooks and screens to TanStack Query
- Remove route/store/URL sync effects
- Replace derived-state effects in story detail and story panel
- Replace the CRM table prop-mirror pattern

### P2: Important, but can follow after the query cleanup

- Convert settings and dialog forms from prop-sync effects to keyed/resettable forms
- Move cross-field defaulting into reducers or explicit change handlers

### P3: Nice cleanup after app-level wins land

- Event-driven dialog/search resets
- Widget package async-resource hooks
- `useSyncExternalStore` pageview subscriptions

---

## Implementation Plan

### Task 1: Introduce shared primitives

**Files:**
- Create or expand query hooks under `frontend/src/hooks/queries/`
- Create a shared widget async-resource hook under `packages/widget-core/src/components/`
- Create shared route/search helpers where needed

- [ ] Add query hooks for workspace members, assignable members, workspace teams, recurring templates, roadmap, my work, story delivery, and shared docs.
- [ ] Add invalidation helpers so screen mutations no longer need bespoke `reload()` effects.
- [ ] Add a small `useHistoryLocationStore` or router-native pageview subscription abstraction for package hooks.

### Task 2: Replace the biggest manual fetch effects

**Files:**
- `frontend/src/hooks/useAssignableWorkspaceMembers.ts`
- `frontend/src/hooks/useWorkspaceMembers.ts`
- `frontend/src/hooks/useWorkspaceTeams.ts`
- `frontend/src/pages/Settings.tsx`
- `frontend/src/pages/pm/MyWork.tsx`
- `frontend/src/pages/pm/Roadmap.tsx`
- `frontend/src/pages/docs/SharedDocumentView.tsx`
- `frontend/src/components/pm/RecurringTemplatesSettings.tsx`
- `frontend/src/components/pm/StoryDeliveryPanel.tsx`

- [ ] Migrate each file to TanStack Query or a dedicated resource hook.
- [ ] Delete module-level caches and manual invalidation listeners once equivalent query invalidation exists.
- [ ] Keep loading and error UX equivalent or better.

### Task 3: Split story detail and global story panel into query-backed slices

**Files:**
- `frontend/src/pages/pm/StoryDetail.tsx`
- `frontend/src/components/pm/StoryDetailPanel.tsx`
- `frontend/src/components/pm/GlobalStoryPanel.tsx`

- [ ] Move story detail, comments, activity, checklist items, external links, labels, epics, sprints, and recurring summary into query hooks.
- [ ] Replace derived-state effects with `useMemo` or direct computation.
- [ ] Move team-change label cleanup into explicit update handlers.
- [ ] Remove direct `window.history` synchronization in favor of router-owned search state.

### Task 4: Remove prop-to-state mirror forms

**Files:**
- docs dialogs and doc detail
- settings forms listed above
- `frontend/src/components/pm/ViewBar.tsx`
- `frontend/src/pages/pm/Labels.tsx`
- `frontend/src/components/pm/LabelsSettings.tsx`
- `frontend/src/components/pm/CreateStoryModal.tsx`
- `frontend/src/pages/pm/ObjectiveDetail.tsx`
- CRM detail pages

- [ ] Convert dialogs/forms to keyed children or explicit `reset` semantics.
- [ ] Keep drafts local, but stop overwriting them from effects after render.
- [ ] Prefer one reset path per form instead of several sync effects.

### Task 5: Remove prop-mirror datasets in tables

**Files:**
- `frontend/src/components/crm/CompaniesTable.tsx`
- `frontend/src/components/crm/DealsTable.tsx`
- `frontend/src/components/crm/ContactsTable.tsx`
- `frontend/src/components/pm/StoryListView.tsx`

- [ ] Move optimistic updates into query cache utilities or row-level draft state.
- [ ] Eliminate “copy whole dataset into local state” where the parent already owns the server data.

### Task 6: Clean up event-driven UI effects

**Files:**
- `frontend/src/components/search/SearchCommandPalette.tsx`
- `frontend/src/components/pm/AssociationsPanel.tsx`
- `frontend/src/components/support/SidebarAssociations.tsx`
- `frontend/src/components/docs/DocumentLinksPanel.tsx`
- `packages/widget-core/src/components/ChatWindow.tsx`

- [ ] Reset state in close handlers rather than post-close effects.
- [ ] Replace timeout-based focus effects with autofocus/callback-ref patterns where possible.
- [ ] Make “controlled vs uncontrolled view state” explicit in widget navigation.

### Task 7: Verification

- [ ] Add focused regression tests for each migrated query hook and each converted form reset flow.
- [ ] Run targeted Vitest suites for PM, docs, CRM, settings, and widget packages.
- [ ] Run `./node_modules/.bin/tsc -p tsconfig.json --noEmit --incremental false` in `frontend`.
- [ ] Run `NODE_OPTIONS=--max-old-space-size=4096 npx vite build` in `frontend`.
- [ ] Run any relevant package builds/tests for `packages/react`, `packages/nextjs`, and `packages/widget-core`.
- [ ] Manually verify route/search-driven flows: workspace selection, `?story=` opening, docs collection filters, and recurring/settings pages.

---

## Recommended Order

1. Query-ify the shared workspace/member/team hooks.
2. Convert the biggest PM screens (`StoryDetail`, `StoryDetailPanel`, `GlobalStoryPanel`, `MyWork`, `Roadmap`).
3. Remove route/store/URL sync effects.
4. Convert settings/forms to keyed resettable components.
5. Refactor CRM tables and remaining dialog reset/focus effects.
6. Finish with widget/package cleanup.
