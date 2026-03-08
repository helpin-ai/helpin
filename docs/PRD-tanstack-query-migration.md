# PRD: TanStack Query Migration

**Date:** 2026-03-08
**Status:** Draft

---

## 1. Overview

Migrate the frontend from manual Zustand-based server state management to TanStack Query (React Query v5) for all API data fetching, caching, and mutations. Zustand stores will be retained only for pure client-side state (UI preferences, display toggles, modal state).

## 2. Problem Statement

The current architecture uses Zustand stores as the primary mechanism for both server state and client state. Each store manually implements:

- `loading: boolean` and `error: string | null` tracking
- Manual `useEffect` triggers for data fetching on filter/workspace changes
- No automatic cache invalidation, deduplication, or background refetching
- Custom event listeners (`window.dispatchEvent`) for cross-component data synchronization
- Module-level caches with manual invalidation (e.g., `useWorkspaceTeams`)
- Duplicate fetch prevention logic scattered across components

This results in boilerplate-heavy code, inconsistent loading/error UX, stale data bugs, and unnecessary re-renders.

## 3. Goals

1. **Eliminate server-state boilerplate** — Remove manual loading/error/data tracking from Zustand stores
2. **Automatic caching & deduplication** — Same data requested by multiple components is fetched once
3. **Background refetching** — Stale data is refreshed automatically on window focus and reconnect
4. **Declarative invalidation** — Mutations automatically invalidate related queries
5. **Optimistic updates** — Instant UI feedback for common mutations (story moves, status changes)
6. **Preserve existing patterns** — Keep the `api.ts` client, token refresh, WebSocket real-time sync
7. **Incremental migration** — Stores can be migrated one at a time without breaking the app

## 4. Non-Goals

- Replacing the `api.ts` HTTP client (it handles token refresh well)
- Replacing Zustand for client-only state (display preferences, sidebar state, filters UI)
- Changing the backend API contract
- Adding SSR or server-side data loading
- Migrating WebSocket real-time sync to TanStack Query subscriptions (keep current approach)

## 5. Technical Design

### 5.1 Dependencies

```
@tanstack/react-query: ^5.x
@tanstack/react-query-devtools: ^5.x (dev only)
```

No additional dependencies required. TanStack Router integration (`@tanstack/react-router-with-query`) is optional and deferred — not needed for this migration.

### 5.2 Query Client Setup

**New file:** `src/lib/queryClient.ts`

```ts
import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,        // 30s before data is considered stale
      gcTime: 5 * 60_000,       // 5 min garbage collection
      retry: 1,                 // 1 retry on failure
      refetchOnWindowFocus: true,
      refetchOnReconnect: true,
    },
    mutations: {
      retry: 0,
    },
  },
})
```

Wrap the app in `<QueryClientProvider>` in `main.tsx` or the root layout route.

Add `<ReactQueryDevtools>` in dev mode for debugging.

### 5.3 Query Key Convention

Hierarchical key structure for granular invalidation:

```
['workspaces']                                    // all workspaces
['workspaces', slug]                              // single workspace
['workspaces', wsId, 'members']                   // workspace members
['workspaces', wsId, 'settings']                  // all settings
['workspaces', wsId, 'teams']                     // workspace teams
['workspaces', wsId, 'quarters']                  // quarters

['organizations']                                 // all orgs

['pm', wsId, 'stories', { filters }]              // story list (filtered)
['pm', wsId, 'stories', storyId]                  // single story
['pm', wsId, 'stories', 'displayId', displayId]   // story by display ID

['pm', wsId, 'board', workflowId, { filters }]    // board columns
['pm', wsId, 'board', workflowId, 'column', columnId, { offset }]  // column pagination

['pm', wsId, 'objectives']                        // objectives list
['pm', wsId, 'objectives', objectiveId]           // single objective

['pm', wsId, 'epics']                             // epics list
['pm', wsId, 'epics', epicId]                     // single epic
['pm', wsId, 'epics', epicId, 'stories']          // epic stories

['pm', wsId, 'sprints']                           // sprints list
['pm', wsId, 'workflows']                         // workflows list

['pm', wsId, 'labels']                            // labels
['pm', wsId, 'views']                             // saved views
['pm', wsId, 'templates']                         // story templates

['pm', wsId, 'automations']                       // automations

['user', 'me']                                    // current user profile
```

**Key factory pattern** — Create a `queryKeys.ts` file:

```ts
export const queryKeys = {
  workspaces: {
    all: ['workspaces'] as const,
    bySlug: (slug: string) => ['workspaces', slug] as const,
    members: (wsId: string) => ['workspaces', wsId, 'members'] as const,
    settings: (wsId: string) => ['workspaces', wsId, 'settings'] as const,
    teams: (wsId: string) => ['workspaces', wsId, 'teams'] as const,
    quarters: (wsId: string) => ['workspaces', wsId, 'quarters'] as const,
  },
  pm: {
    stories: (wsId: string, filters?: Record<string, any>) =>
      filters ? ['pm', wsId, 'stories', filters] as const : ['pm', wsId, 'stories'] as const,
    story: (wsId: string, id: string) => ['pm', wsId, 'stories', id] as const,
    board: (wsId: string, workflowId: string, filters?: Record<string, any>) =>
      ['pm', wsId, 'board', workflowId, filters] as const,
    objectives: (wsId: string) => ['pm', wsId, 'objectives'] as const,
    objective: (wsId: string, id: string) => ['pm', wsId, 'objectives', id] as const,
    epics: (wsId: string) => ['pm', wsId, 'epics'] as const,
    epic: (wsId: string, id: string) => ['pm', wsId, 'epics', id] as const,
    sprints: (wsId: string) => ['pm', wsId, 'sprints'] as const,
    workflows: (wsId: string) => ['pm', wsId, 'workflows'] as const,
    labels: (wsId: string) => ['pm', wsId, 'labels'] as const,
    views: (wsId: string) => ['pm', wsId, 'views'] as const,
    templates: (wsId: string) => ['pm', wsId, 'templates'] as const,
    automations: (wsId: string) => ['pm', wsId, 'automations'] as const,
  },
  organizations: {
    all: ['organizations'] as const,
  },
  user: {
    me: ['user', 'me'] as const,
  },
}
```

### 5.4 Custom Hook Pattern

Each service maps to a set of query/mutation hooks. Example:

**New file:** `src/hooks/queries/useObjectives.ts`

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmObjectiveService } from '@/lib/services/pmObjectiveService'
import { queryKeys } from '@/lib/queryKeys'

export function useObjectives(wsId: string, filters?: ObjectiveFilters) {
  return useQuery({
    queryKey: queryKeys.pm.objectives(wsId),
    queryFn: () => pmObjectiveService.list(wsId, filters),
    enabled: !!wsId,
  })
}

export function useObjective(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.pm.objective(wsId, id),
    queryFn: () => pmObjectiveService.get(wsId, id),
    enabled: !!wsId && !!id,
  })
}

export function useCreateObjective(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateObjectiveRequest) => pmObjectiveService.create(wsId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.pm.objectives(wsId) }),
  })
}
```

### 5.5 Service Layer Adaptation

The existing services return `ApiResponse<T>` (`{ data, error }`). Query functions need to throw on error so TanStack Query can track error state:

**New file:** `src/lib/queryUtils.ts`

```ts
import { ApiResponse } from './api'

export function unwrapResponse<T>(response: ApiResponse<T>): T {
  if (response.error) {
    throw new Error(response.error)
  }
  return response.data as T
}
```

Usage in hooks:

```ts
queryFn: async () => {
  const res = await pmObjectiveService.list(wsId, filters)
  return unwrapResponse(res)
}
```

This preserves all existing service functions unchanged — only the hooks wrap them.

### 5.6 WebSocket Integration

The existing WebSocket real-time sync dispatches custom DOM events and calls `patchStory()` / `refreshBoard()` on the board store. With TanStack Query:

- WebSocket handlers call `queryClient.invalidateQueries()` or `queryClient.setQueryData()` for granular updates
- `patchStory()` becomes `queryClient.setQueryData()` with an updater function
- `refreshBoard()` becomes `queryClient.invalidateQueries({ queryKey: queryKeys.pm.board(...) })`
- The `useRealtimeSync` hook is updated to use `useQueryClient()` instead of Zustand store actions

### 5.7 Board Store Special Handling

The `pmBoardStore` is the most complex store with column-level pagination, incremental patching, and view management. Migration approach:

1. **Column data** → `useInfiniteQuery` per column with `offset`/`limit` pagination
2. **Board-level data** (workflows, filters) → Regular queries
3. **Views** (saved filters/sorting) → Queries + mutations
4. **Display preferences** (show empty columns, property toggles) → Keep in `boardDisplayStore` (client state)
5. **Optimistic story moves** → `useMutation` with `onMutate` for instant column updates

### 5.8 Stores to Migrate vs. Keep

#### Migrate to TanStack Query (server state)

| Store | Server State to Extract | Remaining Client State |
|-------|------------------------|----------------------|
| `authStore` | `user` (from `/me`), `initialize()`, `signIn()`, `signUp()` | `serverUnreachable` flag |
| `workspaceStore` | `workspaces[]`, `loadWorkspaces()`, `loadWorkspaceBySlug()` | `currentWorkspace` (selection) |
| `organizationStore` | `organizations[]`, `loadOrganizations()` | `currentOrganization` (selection, localStorage) |
| `sessionStore` | `membership` from API | Role helper methods (derived) |
| `quarterStore` | `quarters[]`, `loadQuarters()` | `currentQuarter` (selection) |
| `pmBoardStore` | `columns[]`, `workflows[]`, `views[]`, all load/refresh functions | `filters` (UI state), `activeViewId`, display toggles |
| `pmStoryStore` | `stories[]`, `currentStory`, `loadStories()`, `loadStory()` | `filters` (UI state), `page`, `perPage` |
| `pmWorkflowStore` | `workflows[]`, `loadWorkflows()` | `selectedWorkflow` (UI selection) |
| `teamEstimateStore` | Settings from API | None (can be removed entirely) |
| `teamFieldVisibilityStore` | Settings from API | None (can be removed entirely) |

#### Keep as Zustand (pure client state)

| Store | Reason |
|-------|--------|
| `boardDisplayStore` | localStorage-backed display preferences |
| `globalCreateStore` | Modal open/close state |

### 5.9 Hook File Organization

```
src/hooks/queries/
├── useAuth.ts              # useCurrentUser, useSignIn, useSignUp, useSignOut, useUpdateProfile
├── useWorkspaces.ts        # useWorkspaces, useWorkspaceBySlug, useWorkspaceMembers
├── useOrganizations.ts     # useOrganizations, useCreateOrg, useUpdateOrg
├── useSettings.ts          # useWorkspaceSettings, useTeams, useUpdateTeam, etc.
├── useQuarters.ts          # useQuarters
├── useSession.ts           # useSession (membership + role helpers)
├── useStories.ts           # useStories, useStory, useCreateStory, useUpdateStory
├── useBoard.ts             # useBoard, useBoardColumn, useMoveStory
├── useObjectives.ts        # useObjectives, useObjective, useCreateObjective, etc.
├── useEpics.ts             # useEpics, useEpic, useCreateEpic, etc.
├── useSprints.ts           # useSprints, useCreateSprint, etc.
├── useWorkflows.ts         # useWorkflows
├── useLabels.ts            # useLabels, useCreateLabel, etc.
├── useViews.ts             # useViews, useSaveView, etc.
├── useTemplates.ts         # useStoryTemplates
├── useComments.ts          # useComments, useCreateComment
├── useChecklists.ts        # useChecklists, useCreateChecklistItem
├── useAttachments.ts       # useAttachments, useUploadAttachment
├── useAutomations.ts       # useAutomations, useCreateAutomation
├── useSearch.ts            # useSearch
└── index.ts                # barrel export
```

**Supporting files:**

```
src/lib/
├── queryClient.ts          # QueryClient configuration
├── queryKeys.ts            # Query key factory
└── queryUtils.ts           # unwrapResponse helper
```

## 6. Migration Plan

### Phase 1: Foundation (no breaking changes)

1. Install `@tanstack/react-query` and `@tanstack/react-query-devtools`
2. Create `queryClient.ts`, `queryKeys.ts`, `queryUtils.ts`
3. Add `QueryClientProvider` to root layout
4. Add `ReactQueryDevtools` in dev mode

### Phase 2: Simple Read-Only Queries

Migrate stores that are primarily read-only with minimal complexity:

1. `organizationStore` → `useOrganizations()`
2. `quarterStore` → `useQuarters()`
3. `sessionStore` → `useSession()`
4. `teamEstimateStore` → absorbed into `useSettings()`
5. `teamFieldVisibilityStore` → absorbed into `useSettings()`
6. `pmWorkflowStore` → `useWorkflows()`

### Phase 3: Core Entity Queries + Mutations

1. `workspaceStore` → `useWorkspaces()`, `useWorkspaceBySlug()`
2. `authStore` → `useCurrentUser()`, `useSignIn()`, `useSignUp()`
3. Objectives → `useObjectives()`, `useCreateObjective()`, etc.
4. Epics → `useEpics()`, `useCreateEpic()`, etc.
5. Sprints → `useSprints()`, `useCreateSprint()`, etc.
6. Labels, Views, Templates, Automations

### Phase 4: Story List + Detail

1. `pmStoryStore` → `useStories()`, `useStory()`
2. Story mutations → `useCreateStory()`, `useUpdateStory()`
3. Comments, Checklists, Attachments, External Links
4. Migrate story detail page to use query hooks

### Phase 5: Board (most complex)

1. Board columns → query-per-column or `useInfiniteQuery`
2. Story moves → `useMoveStory()` with optimistic updates
3. Board refresh → query invalidation from WebSocket
4. Integrate `patchStory` as `setQueryData` updater
5. Views management → query + mutations
6. Pagination → infinite query with `fetchNextPage()`

### Phase 6: Cleanup

1. Remove emptied Zustand stores (server state removed)
2. Slim down remaining stores to client-only state
3. Remove custom event listeners replaced by query invalidation
4. Remove module-level caches (`useWorkspaceTeams` cache)
5. Remove manual loading/error state from components
6. Update `useRealtimeSync` to use `queryClient.invalidateQueries()`

## 7. Component Migration Pattern

**Before (Zustand):**

```tsx
function Objectives() {
  const { currentWorkspace } = useWorkspaceStore()
  const [objectives, setObjectives] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const load = useCallback(async () => {
    setLoading(true)
    const res = await pmObjectiveService.list(currentWorkspace.id)
    if (res.error) setError(res.error)
    else setObjectives(res.data)
    setLoading(false)
  }, [currentWorkspace])

  useEffect(() => { load() }, [load])

  useEffect(() => {
    const handler = () => load()
    window.addEventListener('objective-created', handler)
    return () => window.removeEventListener('objective-created', handler)
  }, [load])

  if (loading) return <Spinner />
  if (error) return <ErrorBanner message={error} />
  return <ObjectiveList items={objectives} />
}
```

**After (TanStack Query):**

```tsx
function Objectives() {
  const { currentWorkspace } = useWorkspaceStore()
  const { data, isLoading, error } = useObjectives(currentWorkspace.id)

  if (isLoading) return <Spinner />
  if (error) return <ErrorBanner message={error.message} />
  return <ObjectiveList items={data} />
}
```

Custom events like `'objective-created'` are replaced by `invalidateQueries` in the `useCreateObjective` mutation's `onSuccess`.

## 8. Error Handling Strategy

- `unwrapResponse()` converts `{ data, error }` API responses to thrown errors
- TanStack Query catches thrown errors and exposes via `error` / `isError`
- The existing `api.ts` 401/token-refresh logic continues to work inside service functions
- Network errors from `api.ts` (`isNetworkError: true`) are surfaced as query errors
- Optional: Add a global `onError` callback to `QueryClient` for toast notifications on mutation failures

## 9. Testing Considerations

- Wrap test components in `<QueryClientProvider>` with a fresh `QueryClient` per test
- Use `queryClient.setQueryData()` in tests to pre-populate cache
- Disable `retry` and `refetchOnWindowFocus` in test query clients
- Mock service functions at the module level (same as current approach)

## 10. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Board store complexity causes regressions | Migrate board last (Phase 5); thorough manual testing |
| Dual state management during migration | Phases are designed so each store migrates completely before moving on |
| WebSocket + query invalidation causes excessive refetches | Debounce invalidation (200ms, matching current behavior); use `setQueryData` for patches |
| Token refresh during query retry | `api.ts` already handles this transparently; queries see clean responses |
| Bundle size increase | `@tanstack/react-query` is ~13KB gzipped; offset by removing Zustand boilerplate |

## 11. Success Metrics

- All server state fetching uses TanStack Query hooks
- Zustand stores contain only client-side UI state
- No manual `loading`/`error` state tracking for API data
- No custom `window.dispatchEvent` for data synchronization
- DevTools show clean query tree with expected cache behavior
- No regressions in real-time board updates
