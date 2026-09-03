# useEffect Audit — where we should not be using `useEffect`

**Date:** 2026-09-03
**Scope:** every `useEffect(` call in `frontend/src`, `packages/widget-core/src`, `help-center/src`, `website/src` (`packages/sdk-js` has none).
**Method:** every file was read line by line and each effect classified against the React docs guide ["You Might Not Need an Effect"](https://react.dev/learn/you-might-not-need-an-effect). Findings were cross-checked against `eslint-plugin-react-hooks` v7 (React Compiler rules), which the frontend already has enabled via `reactHooks.configs.flat.recommended`.

## Summary

| Area | Files | Effects reviewed | High | Medium | Low | Legitimate |
|---|---|---|---|---|---|---|
| A. PM components (`components/pm`) | 36 | 145 | 42 | 46 | 13 | 44 |
| B. Docs (`components/docs`, `components/editor`, `pages/docs`) | 31 | 83 | 8 | 16 | 7 | 52 |
| C. Settings, auth, account pages | 45 | 88 | 33 | 29 | 13 | 13 |
| D. Support, CRM, agents, automation | 39 | 109 | 20 | 40 | 17 | 32 |
| E. PM pages, `ui/`, layout, hooks, routes | 39 | 90 | 18 | 32 | 13 | 27 |
| F. widget-core, help-center, website | 26 | 42 | 5 | 10 | 4 | 23 |
| **Total** | **216** | **557** | **126** | **173** | **67** | **191** |

**366 of 557 effects (66%) have a better alternative.** 191 are legitimate (WebSocket / DOM listeners / observers / timers / editor integrations / focus and scroll management).

Lint baseline (`cd frontend && npx eslint src`), for tracking progress:

| Rule | Count |
|---|---|
| `react-hooks/set-state-in-effect` | 211 |
| `react-hooks/exhaustive-deps` | 68 |
| `eslint-disable … react-hooks/*` comments | 15 |

## The patterns, by frequency

| # | Anti-pattern | Count | Replace with |
|---|---|---|---|
| 1 | **Data fetching in `useEffect`** (`service.list().then(setData)` + manual `loading`/`error` state) | ~120 | `useQuery` in `frontend/src/hooks/queries/` with a key from `queryKeys.ts`. Many hooks already exist (`useAgents`, `useWorkflows`, `useTasks`, `useComments`, `useLabels`, `useSprints`, …). widget-core has no TanStack Query, so it needs a small cancellable `useAsyncResource` hook. |
| 2 | **Resetting / syncing local state from props or query data** (`useEffect(() => setDraft(data), [data])`) | ~60 | `key={id}` on the form/dialog to remount, lazy `useState(() => …)`, or the "store previous prop, adjust during render" pattern. For settings drafts, `useQuery` `select` or a `useForm` `values`/`reset` call in the load handler. |
| 3 | **Event-specific logic in an effect** (toast/navigate/close after a `success` flag flips) | ~40 | Do it in the mutation `onSuccess` / the click handler. |
| 4 | **Derived state** (`useEffect(() => setFiltered(filter(items)), [items])`) | ~26 | Compute during render or `useMemo`. |
| 5 | **Reset form state when a dialog opens** (`useEffect(() => { if (open) reset() }, [open])`) | ~20 | Conditionally mount the dialog body (`{open && <Body/>}`) or `key={open}` so state starts fresh. |
| 6 | **Chained effects** (effect A sets state → effect B reacts) | ~20 | One handler or one render-time computation. |
| 7 | **Notifying parent of state changes** (`useEffect(() => onChange(v), [v])`) | ~15 | Call `onChange` in the handler that sets `v`, or lift state. |
| 8 | **Syncing Zustand store from query data / hook state** | ~12 | Read from the query cache directly; if a store must mirror it, do it in the query `onSuccess`/`select`, and note it as tech debt. |
| 9 | **`useEffect` that only calls `refetch()` / `invalidateQueries` on a param change** | ~7 | Put the param in the query key. |
| 10 | **Mount-time initialization / ref mirroring** | ~11 | Lazy `useState` initializer; for "latest callback" refs use a `useLatest`/`useEffectEvent` helper. |
| 11 | **Debounced value via effect** | ~4 | `useDeferredValue`, or a shared `useDebounce` hook (none exists in `frontend/src/hooks/` yet — worth adding once). |
| 12 | **Media query / external store** | ~2 | `useSyncExternalStore` (e.g. `use-mobile.ts`). |

## Where to start (highest payoff)

Files with the most High-severity findings. Fixing these first removes the majority of hand-rolled fetch/loading state:

1. `frontend/src/components/pm/CreateTaskModal.tsx` — 6 High, 6 Medium
2. `frontend/src/components/settings/TeamsTab.tsx` — 5 High, 2 Medium
3. `frontend/src/components/pm/TaskDetailPanel.tsx` — 4 High, 4 Medium
4. `frontend/src/components/pm/KanbanBoard.tsx` — 4 High, 2 Medium
5. `frontend/src/components/settings/WorkflowManager.tsx` — 4 High, 1 Medium
6. `frontend/src/components/agents/dock/ChatView.tsx` — 3 High, 3 Medium
7. `frontend/src/components/pm/CodingSession/CodingSessionSurface.tsx` — 3 High, 2 Medium
8. `frontend/src/pages/pm/EpicDetail.tsx` — 3 High, 1 Medium
9. `frontend/src/pages/automation/Agents.tsx` — 3 High, 1 Medium
10. `frontend/src/components/pm/GlobalTaskPanel.tsx` — 3 High, 1 Medium
11. `frontend/src/components/settings/WorkspaceRepositoriesTab.tsx` — 3 High
12. `frontend/src/components/pm/TaskListView.tsx` — 2 High, 4 Medium

Shared hooks that should be rewritten once and fix many call sites:

- `frontend/src/hooks/useWorkspaceMembers.ts` — module-level cache + effect fetch → `useQuery`
- `frontend/src/hooks/useWorkspaceTeams.ts` — same
- `frontend/src/hooks/useAssignableWorkspaceMembers.ts` — same
- `frontend/src/hooks/use-mobile.ts` — `useSyncExternalStore`
- Add `frontend/src/hooks/useDebounce.ts` (used by ~6 search boxes that currently debounce + fetch in effects)

Known bug-class findings (not just style):

- `frontend/src/components/docs/CreateDocumentDialog.tsx` — effect resets the user's chosen collection whenever the collections query refetches.
- `frontend/src/components/pm/CreateTaskModal.tsx` — `eslint-disable` exists specifically because including `teams` in deps resets the form on background refetch; the fix is keying the form, not disabling the lint.
- `frontend/src/components/support/SupportInboxLayout.tsx` — bidirectional URL ↔ store sync via two effects (loop risk).
- Several fetch effects have no cancellation / stale-response guard, so fast navigation can display the previous entity's data.

## Suggested process

1. Turn `react-hooks/set-state-in-effect` from warn to error once the count is under ~20, to stop regressions.
2. Migrate one area at a time in the order above; each migration should delete the `loading`/`error` `useState` pair alongside the effect.
3. Prefer `key=` remounts over prop-sync effects for dialogs and detail panels; it is the smallest diff and removes the stale-data class of bugs.

---

# Detailed findings

Line numbers refer to the `useEffect(` call on the branch `fix/preserve-read-state-on-inbox-move` at commit `7453bd522`. Files with no flagged effects are listed with the reason their effects are legitimate.


## A. PM components (components/pm)


### `frontend/src/components/pm/SaveViewDialog.tsx`
- **L33** — category: useEffect used to reset form state when dialog opens — When `open` flips true, copies `initialName`/`initialIsShared` props into local `name`/`isShared` state. → **Fix:** Render the dialog body only while open (or `key={String(open)}`) and use lazy `useState(() => initialName ?? '')`; callers (TaskFilters, ViewBar) already control `open`. Severity: Medium.

### `frontend/src/components/pm/EpicPlannerPanel.tsx`
- **L331** — category: Data fetching in useEffect — Calls `loadAgents()` (`agentService.list`) on mount / workspace change, storing into `agents` state. → **Fix:** Use the existing `useAgents(workspaceId)` query hook (already used by KanbanBoard) and drop the local `agents` state + `loadAgents`. Severity: High.
- **L335** — category: Data fetching in useEffect — Calls `loadRuns()` (`agentService.listTargetRuns`) with manual `loadingRuns`/`refreshingRuns` flags. → **Fix:** `useQuery` keyed on `[workspaceId,'agentRuns','epic',epicId]` with `select` to filter command-bar runs; the L358 poll becomes `refetchInterval` while any run is queued/running, and the L339 window listener becomes `invalidateQueries`. Selected-run reconciliation moves to a derived value (`selectedRunId ?? lastRunId ?? runs[0]?.id`). Severity: High.
- **L339** — legitimate (window event subscription with cleanup); note it would collapse into a query invalidation once L335 is a query.
- **L358** — legitimate (setInterval polling with cleanup); becomes `refetchInterval` after L335 migration.
- **L392** — category: Derived state — Sets `selectedAgentId` to `preferredPlanner.id` when nothing is selected. → **Fix:** Derive `const effectiveAgentId = selectedAgentId || preferredPlanner?.id ?? ''` during render; keep `selectedAgentId` only for explicit user picks. Severity: Medium.
- **L398** — category: Chained effects — If the selected agent is no longer in `plannerAgents`, sets `selectedAgentId` back to preferred (which then re-triggers L392). → **Fix:** Same derived value as L392: `plannerAgents.some(a => a.id === selectedAgentId) ? selectedAgentId : preferredPlanner?.id`. One computation replaces both effects. Severity: Medium.
- **L408** — category: Event-specific logic in an effect — Watches `runs[0]` for a transition to `completed` and calls `onRunCompleted?.()` once per run id via a ref. → **Fix:** Detect the transition inside `loadRuns` (compare previous vs. next runs after the fetch) or, post-migration, in the query's `onSuccess`/a `useEffect` on the query data is unavoidable-with-caveat; at minimum call the parent from the fetch code path rather than diffing state. Severity: Medium.

### `frontend/src/components/pm/CommentThread.tsx`
- **L384** — category: Derived state — On every `comments` change, auto-adds threads with replies to `expandedThreads` (tracking already-auto-expanded ids in a ref). → **Fix:** Track only user overrides (`collapsedThreadIds` / `manuallyExpandedIds`) and derive `isExpanded(comment) = manuallyExpanded.has(id) || (replyCount > 0 && !collapsed.has(id))` with `useMemo`. Severity: Medium.
- **L421** — category: Initializing app/state on mount (ref mirror) — Mirrors `pendingAttachments` into a ref for the unmount cleanup. → **Fix:** Assign `pendingAttachmentsRef.current = pendingAttachments` directly during render or use a `useLatest` hook; no effect needed. Severity: Low.
- **L425** — category: Initializing app/state on mount (ref mirror) — Same as L421 for `editPendingAttachments`. → **Fix:** Same. Severity: Low.
- **L429** — category: Initializing app/state on mount (ref mirror) — Same as L421 for `replyPendingAttachments`. → **Fix:** Same. Severity: Low.
- **L453** — legitimate (unmount cleanup that deletes orphaned pending uploads).
- **L827** — category: Event-specific logic in an effect — When `commentAnchor` prop is set, cancels any open reply composer and opens/re-keys the top composer. Because `replyComposerOpenFor` is a dep, opening a reply while `commentAnchor` is still set re-runs the effect and cancels it again. → **Fix:** Have the parent trigger this via a callback/imperative handle (`openTopComposer()`) from the click that sets the anchor, or key the composer on the anchor value and drop `replyComposerOpenFor` from the deps. Severity: Medium.

### `frontend/src/components/pm/AssociationsPanel.tsx`
- **L182** — category: Data fetching in useEffect — Debounced (250ms) search that calls `supportService.listConversations` / `crmSearchService.search` / `searchService.search` and stores results + `searching` flag in state. → **Fix:** Debounce `query` (`useDeferredValue` or a `useDebounce` hook) and use `useQuery({ queryKey: ['associationsSearch', workspaceId, pickerSection, debouncedQuery], enabled: !!pickerSection && (pickerSection==='support' || debouncedQuery.length>=2) })`; `isFetching` replaces `searching`. The support branch fetches all conversations and filters client-side — do the filter in `select`. Severity: High.

### `frontend/src/components/pm/TaskFilters.tsx`
- **L227** — category: Resetting/syncing local state from props — Hydrates `filterState` from `externalFilters` whenever the prop changes, using `internalChangeRef` to skip self-originated updates. → **Fix:** Make the component controlled: derive `filterState` from `externalFilters` with `useMemo` and call `onFiltersChange` from pill handlers (dropping the ref dance), or key `TaskFilters` on the applied view id so it remounts with fresh initial state. Severity: Medium.

### `frontend/src/components/pm/RecurringTemplatesSettings.tsx`
- **L120** — category: Data fetching in useEffect — `reload()` calls `pmRecurringTemplateService.list` whenever `status`/`teamFilter`/`search` change, with a manual `loading` flag and toast on error. → **Fix:** `useQuery` keyed on `[workspaceId,'recurringTemplates',{status,teamId,search: debouncedSearch}]`; mutations invalidate the key instead of calling `reload()`. Severity: High.

### `frontend/src/components/pm/CommentEditor.tsx`
- No issues (1 effect, legitimate: TipTap `editor.on/off` subscription with cleanup).

### `frontend/src/components/pm/TaskDetailPanel.tsx`
- **L311** — category: Initializing app/state on mount (ref mirror) — Mirrors `pendingPatch` into `pendingPatchRef`. → **Fix:** Assign during render (`pendingPatchRef.current = pendingPatch`) or `useLatest`. Severity: Low.
- **L315** — category: Initializing app/state on mount (ref mirror) — Mirrors `descriptionPendingUploads` into a ref. → **Fix:** Same as L311. Severity: Low.
- **L320** — category: Data fetching in useEffect — Fetches `gitService.listIntegrations` to compute `hasGitIntegration`; no cancellation. → **Fix:** `useQuery` (e.g. `useGitIntegrations(workspaceId)`) with `select: (list) => list.some(i => i.active)`. Severity: High.
- **L439** — legitimate (syncs `?task=` into the URL via `history.replaceState`, an external system) — consider routing through TanStack Router `navigate({ search, replace: true })` for consistency, but not an anti-pattern.
- **L462** — category: Data fetching in useEffect — Runs `reloadComments()` + `reloadActivity()` (`pmCommentService.list`, `pmTaskService.listActivity`) on task change, with `commentsLoading`/`activityLoading` flags. → **Fix:** `useComments(workspaceId,'task',taskId)` and `useTaskActivity(workspaceId,taskId)` queries; every `reloadX()` call site becomes `invalidateQueries`. Severity: High.
- **L467** — category: Resetting/syncing local state from props (with `eslint-disable react-hooks/exhaustive-deps`) — When `taskDetail.task.updated_at` changes, resets refs, refetches activity, and rebuilds `form` unless there are pending edits. → **Fix:** Use the "store previous prop, adjust during render" pattern: keep `lastUpdatedAt` in state and when it differs from `taskDetail.task.updated_at` call `setForm(buildFormState(taskDetail))` during render (guarded by pending-patch emptiness); move `reloadActivity` to an invalidation keyed on `updated_at`. Severity: Medium.
- **L482** — legitimate (window event listeners with cleanup).
- **L504** — category: Data fetching in useEffect — Fetches epics, sprints and labels into local state on workspace change. → **Fix:** Use `useEpics`/`useSprints`/`useLabels` query hooks (create them in `hooks/queries/` if absent); the same trio is fetched again in `CreateTaskModal` and `KanbanBoard`, so a shared query dedupes them. Severity: High.
- **L680** — category: Event-specific logic in an effect — When `form.team_id` changes (skipping the first run via a ref), clears `epic_id`/`sprint_id` if they are not selectable for the new team. → **Fix:** Perform the check inside the team-change handler (the `updateField('team_id', ...)` call site) and include the cleared fields in the same patch; removes the ref guard. Severity: Medium.
- **L736** — category: Data fetching in useEffect — `syncOptionalSectionContent()` fetches checklist, external links and associations just to decide which sections to show and to compute counts. Child components (`ChecklistItems`, `ExternalLinks`, `TaskRelationshipsSection`) fetch the same data again and push counts back up via effects. → **Fix:** Move these to shared `useQuery` hooks (`useChecklist`, `useExternalLinks`, `useTaskAssociations`); parent and children read the same cache and the parent derives visibility/counts with `useMemo`. Severity: High.
- **L768** — category: Event-specific logic in an effect — Debounced auto-save: whenever `pendingPatch` (or any of 9 deps, including `onTaskUpdated` identity and `saving`) changes, schedules a 650ms timer that calls `pmTaskService.update`. Dep churn resets the timer unnecessarily. → **Fix:** Wrap the save in a `useMutation` and schedule it from `updateField` with a debounced callback (`useDebouncedCallback`) so the timer is tied to edits, not renders; keep the L825 unmount flush. Severity: Medium.
- **L825** — legitimate (unmount cleanup flushes unsaved patch).
- **L986** — category: Event-specific logic in an effect — When `form.team_id` changes, computes labels invalid for the team and fires a server `syncLabels` mutation from the effect. → **Fix:** Do the label pruning in the team-change handler together with the L680 logic and call the sync mutation there; an effect that issues mutations based on state is prone to loops if the server response still contains a mismatched label. Severity: Medium.
- **L2033** — category: Event-specific logic in an effect — Stores `Date.now()` in `openedAtRef` when `open` becomes true. → **Fix:** Set the ref in the open/close handler (or in the parent that flips `open`). Severity: Low.

### `frontend/src/components/pm/KanbanBoard.tsx`
- **L183** — legitimate (IntersectionObserver for load-more with cleanup).
- **L387** — legitimate (IntersectionObserver for load-more with cleanup).
- **L587** — category: Initializing app/state on mount — `initDisplay(workspaceId)` hydrates the board display Zustand store. → **Fix:** Acceptable-with-caveat; ideally hydrate the store in the route `loader`/`beforeLoad` or make `useBoardDisplayStore` self-initialize per workspace so no component-level effect is needed. Severity: Low.
- **L616** — category: Data fetching in useEffect — Calls the store's `loadMemberBoard` whenever `groupBy==='members'` or `showEmptyColumns`/`activeMemberIds` change. → **Fix:** `useQuery` keyed on `[workspaceId,'board','members',{showEmptyColumns, memberIds}]`, `enabled: groupBy==='members'`. Severity: High.
- **L624** — category: Passing data to parent / syncing Zustand store from props — Mirrors the URL `teamId` prop into `usePMBoardStore.teamId`. → **Fix:** Treat the route search param as the source of truth and pass `teamId` into the board query key instead of copying it into the store; if the store must keep it, set it in the same navigation handler that changes the URL. Severity: Medium.
- **L628** — category: Passing data to parent / syncing Zustand store from props — Applies `initialFilters` into the store once per `initialFiltersKey` using a ref guard. → **Fix:** Same as L624: use `initialFilters` as query input / store initializer (`useState(() => ...)` or route loader) rather than a guarded effect. Severity: Medium.
- **L641** — category: Data fetching in useEffect — Fetches labels, epics and sprints into `refLabels`/`refEpics`/`refSprints`; no cancellation, so a fast workspace switch can apply stale data. → **Fix:** Shared `useLabels`/`useEpics`/`useSprints` query hooks (same as TaskDetailPanel L504). Severity: High.
- **L722** — category: Data fetching in useEffect — `loadBoard(workspaceId)` fetches the board through the Zustand store. → **Fix:** `useQuery` for the board (key includes `workspaceId`, `teamId`, `filters`); keep Zustand only for optimistic drag state. Severity: High.
- **L727** — category: Data fetching in useEffect — `loadViews(workspaceId, currentMemberId)` fetches saved views. → **Fix:** `useViews(workspaceId, memberId)` query, `enabled: !!currentMemberId`. Severity: High.
- **L734** — legitimate (window `task-created` listener with cleanup).
- **L788** — legitimate (window listeners patching store with cleanup).

### `frontend/src/components/pm/GlobalTaskPanel.tsx`
- **L114** — category: Event-specific logic in an effect — Closes the contextual task whenever a route task becomes active. → **Fix:** Call `closeContextualTask()` in the handler that opens the route task (`openTaskRoute`), or derive the displayed task as `routeTask ?? contextualTask` and drop the close. Severity: Medium.
- **L122** — category: Data fetching in useEffect — Reads `?task=` from `window.location`, fetches `pmTaskService.getByDisplayId`, then navigates to the canonical route. → **Fix:** Resolve this in the TanStack Router route (`validateSearch` + `beforeLoad`/`loader` that resolves the display id and `throw redirect(...)`), or a `useQuery` keyed on `[workspaceId,'taskByDisplayId',displayId]` whose data drives a navigate. Severity: High.
- **L153** — category: Data fetching in useEffect — Reads `?run=`, fetches the run, then task + workflows (+ recurring), pre-seeds `loadedTask`, and navigates. → **Fix:** Route loader/redirect as in L122; pre-seeding becomes `queryClient.setQueryData` in the loader, so the task route renders from cache. Severity: High.
- **L223** — category: Data fetching in useEffect — Fetches task detail + workflows (+ recurring summary) whenever `activeTaskId`/`requestKey` change, with manual `cancelled` flag and toast on failure. → **Fix:** `useTask(workspaceId, activeTaskId)` + `useWorkflows(workspaceId)` + `useRecurringByTask(...)`; `requestKey` becomes an `invalidateQueries` call; combine into `loadedTask` with `useMemo`. Severity: High.

### `frontend/src/components/pm/ExternalLinks.tsx`
- **L49** — category: Data fetching in useEffect — Fetches `pmExternalLinkService.listByEntity` into `links` state. → **Fix:** `useExternalLinks(workspaceId, entityType, entityId)` query; `reload()` call sites become `invalidateQueries`. Severity: High.
- **L63** — category: Notifying parent of state changes — Calls `onContentChange`/`onCountChange` whenever `links.length` changes. → **Fix:** Have the parent (TaskDetailPanel) read the same `useExternalLinks` query and derive the count; remove both callbacks. Severity: Medium.
- **L68** — legitimate (focuses the input after it mounts).
- **L73** — legitimate (window event listeners with cleanup).

### `frontend/src/components/pm/TaskTemplatesSettings.tsx`
- **L333** — category: Data fetching in useEffect — `reload()` fetches task templates on `scopeFilter` change with a manual `loading` flag. → **Fix:** `useQuery` keyed on `[workspaceId,'taskTemplates',{teamId,includeShared}]`; mutations invalidate. Severity: High.
- **L337** — category: Data fetching in useEffect — Fetches workflows to build a `workflowStateMap`. → **Fix:** `useWorkflows(workspaceId)` with `select` building the `Map`. Severity: High.

### `frontend/src/components/pm/AgentPickerCard.tsx`
- **L107** — category: Notifying parent of state changes — When `autoSelectDefault` and no `value`, calls `onChange(defaultAgent.id)` to push a default into the parent. → **Fix:** Let the parent initialize its value with the default (lazy `useState`, or derive `value ?? defaultAgent?.id` when reading) instead of a child effect that mutates parent state on render. Severity: Medium.

### `frontend/src/components/pm/LabelsSettings.tsx`
- **L118** — category: Resetting/syncing local state from props — `LabelForm` copies the `initial` prop into `form` state whenever `initial` changes; `initial` is rebuilt per render of the parent so this can clobber in-progress edits. → **Fix:** Key `<LabelForm key={editingEntry?.label.id ?? 'new'}>` and use `useState(initial)` only as the initializer. Severity: Medium.
- **L122** — legitimate (focuses the name input; would become `autoFocus` once L118 is keyed).
- **L216** — category: Data fetching in useEffect — `reload()` fetches `pmLabelService.listWithStats` on `scopeFilter` change with manual `loading`. → **Fix:** `useQuery` keyed on `[workspaceId,'labels','stats',{teamId,includeShared}]`. Severity: High.

### `frontend/src/components/pm/CreateTaskModal.tsx`
- **L490** — category: Data fetching in useEffect — Fetches repositories + team repo default when the planning repository section is visible. → **Fix:** `useRepositories(workspaceId)` and `useTeamRepoDefault(workspaceId, teamId)` queries with `enabled: open && showPlanningRepository && !!form.team_id`. Severity: High.
- **L514** — category: useEffect used to reset form state when dialog opens (also `eslint-disable react-hooks/exhaustive-deps`) — Rebuilds the whole form from 13 props whenever the modal opens or any initial* prop / `editingTemplate` changes, and bumps `descriptionEditorKey`. → **Fix:** Render the modal body only while `open` (or `key={`${open}-${editingTemplate?.id ?? 'new'}`}`) and compute the initial form in a lazy `useState(() => buildInitialForm(props))`; then no reset effect and no eslint override are needed. Severity: Medium.
- **L584** — category: Data fetching in useEffect — Resolves the team workflow (template mode) and chains `setStateId`. → **Fix:** `useTeamWorkflow(workspaceId, form.team_id)` query, `enabled: open && isTemplateMode`; derive the effective `stateId` from `stateId` + workflow states with `useMemo`. Severity: High.
- **L616** — category: Data fetching in useEffect — Same as L584 for task mode (`taskWorkflowOverride`), plus a chained `setStateId` remap. → **Fix:** Same `useTeamWorkflow` query (`enabled: open && !isTemplateMode && workflow.team_id !== form.team_id`); state remap becomes a derived value. Severity: High.
- **L655** — category: Data fetching in useEffect — Fetches template attachments for the template being edited. → **Fix:** `useAttachments(workspaceId,'task_template',editingTemplate?.id)` query, `enabled: open && isTemplateMode && !!editingTemplate?.id`. Severity: High.
- **L667** — category: Derived state — Overwrites `form.task_type` with the team default whenever `selectedTeamDefaultTaskType` changes and the user hasn't touched the type. → **Fix:** Derive `effectiveTaskType = taskTypeDirty ? form.task_type : selectedTeamDefaultTaskType` for display/submit, or set it in the team-change handler. Severity: Medium.
- **L675** — category: Data fetching in useEffect — Fetches epics/sprints/labels on open; both branches of the `isTemplateMode` conditional are identical apart from resetting template state. → **Fix:** Shared `useEpics`/`useSprints`/`useLabels` queries (`enabled: open`); template-state resets move into the open handler / keyed remount from L514. Severity: High.
- **L721** — category: Event-specific logic in an effect — Converts `form.description` HTML to markdown when `descriptionMode` becomes `'markdown'`. → **Fix:** Do `setSourceMarkdown(htmlToMarkdown(form.description))` in the mode-toggle handler; re-running on every description change while in markdown mode is wasted work. Severity: Medium.
- **L726** — category: Event-specific logic in an effect — Strips label ids that are invalid for the current team whenever `labels` or `form.team_id` change. → **Fix:** Filter in the team-change handler (and when labels load, via `select`/`useMemo` on the label picker's options) rather than mutating form state from an effect. Severity: Medium.
- **L739** — category: Event-specific logic in an effect — Clears `sprint_id` when its team no longer matches `form.team_id`. → **Fix:** Handle in the team-change handler together with L726/L747. Severity: Medium.
- **L747** — category: Event-specific logic in an effect — Clears `epic_id` when not selectable for the team. → **Fix:** Handle in the team-change handler together with L726/L739 (one handler, one `setForm`). Severity: Medium.
- **L755** — category: Data fetching in useEffect — Fetches the selected epic's delivery target. → **Fix:** `useEpicDeliveryTarget(workspaceId, form.epic_id)` query, `enabled: !!form.epic_id && !isTemplateMode`. Severity: High.

### `frontend/src/components/pm/StickyPinnedGroupOverlay.tsx`
- No issues (2 effects, all legitimate: rAF cleanup on unmount; scroll listener on the virtualizer container with cleanup).

### `frontend/src/components/pm/GlobalEpicPanel.tsx`
- **L82** — category: Data fetching in useEffect — Fetches epic, its tasks and delivery target when `activeEpicId`/`requestKey` change, with manual `loading` + `cancelled` flag. → **Fix:** `useEpic`, `useEpicTasks`, `useEpicDeliveryTarget` queries; `requestKey` becomes `invalidateQueries`; on error toast + close from `onError`/an `isError` branch. Severity: High.
- **L120** — category: Event-specific logic in an effect — Records `Date.now()` in `openedAtRef` when the panel opens. → **Fix:** Set the ref in the open handler / route change callback. Severity: Low.

### `frontend/src/components/pm/TaskDeliveryPanel.tsx`
- **L61** — category: Data fetching in useEffect — Loads repositories, task delivery target and epic target, then copies target fields into `repositoryId`/`baseBranch` form state. → **Fix:** `useRepositories`, `useTaskDeliveryTarget`, `useEpicDeliveryTarget` queries; initialize the editable fields from query data via a keyed form (`key={target?.id}`) or lazy `useState`. Severity: High.

### `frontend/src/components/pm/ColorPicker.tsx`
- **L236** — category: useEffect used to reset form state when dialog opens — When the popover opens, re-derives `hsv`/`hexInput` from `value`. → **Fix:** Mount the popover content only while `open` (Radix `PopoverContent` already unmounts when closed) and initialize state lazily from `value`; or reset in the `onOpenChange(true)` handler. Severity: Medium.
- **L336** — category: useEffect used to reset form state when dialog opens — Same pattern for the custom-color popover (`customOpen`). → **Fix:** Same as L236. Severity: Medium.

### `frontend/src/components/pm/CodingSession/UnicodeSpinner.tsx`
- No issues (1 effect, legitimate: setInterval animation with cleanup).

### `frontend/src/components/pm/CodingSession/CodingPreviewPanels.tsx`
- **L213** — category: Resetting/syncing local state from props — Opens the local dialog when the parent sets `openPreviewPanelKey==='docs_change'`; `openPreviewRequestId` is a counter used purely to re-trigger the effect. → **Fix:** Lift `dialogOpen` to the parent (controlled `open` + `onOpenChange`), or use the "previous prop" pattern (`if (requestId !== lastHandledRequestId) { setLastHandled(requestId); setDialogOpen(true) }` during render). Severity: Medium.
- **L487** — category: Resetting/syncing local state from props — Same pattern keyed on `preview.panelKey`. → **Fix:** Same as L213. Severity: Medium.
- **L582** — category: Resetting/syncing local state from props — Same pattern for `'task_plan'`. → **Fix:** Same as L213. Severity: Medium.
- **L763** — category: Resetting/syncing local state from props — Same pattern for `'prd_draft'`. → **Fix:** Same as L213. Severity: Medium.

### `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx`
- No issues (5 effects, all legitimate: scroll listener with cleanup; scroll-to-tail after render on session open / new items / streaming updates; textarea auto-resize measuring DOM after render).

### `frontend/src/components/pm/sprints/SprintPlanningColumn.tsx`
- **L127** — category: Passing data to parent / syncing Zustand store from query data — Seeds/overwrites the infinite query cache for sprint preview tasks from the `card.preview_tasks` prop on every change. → **Fix:** Acceptable-with-caveat. Prefer `initialData`/`placeholderData` inside `useInfiniteSprintPreviewTasks` (it already receives `previewTasks`), and have the parent sprint-planning query `setQueryData` for children when it refetches, so no component effect writes the cache. Severity: Low.
- **L175** — legitimate (measures the list after render and fetches the next page when content is shorter than the viewport).

### `frontend/src/components/pm/Attachments.tsx`
- **L121** — category: Data fetching in useEffect — Fetches `pmAttachmentService.list` into `attachments` state. → **Fix:** `useAttachments(workspaceId, entityType, entityId)` query; `reload()` sites become `invalidateQueries`. Severity: High.
- **L132** — legitimate (window event listeners with cleanup).
- **L192** — category: Notifying parent of state changes — Hands the parent a "open file picker" function via `onFilePickerReady` callback in an effect. → **Fix:** `forwardRef` + `useImperativeHandle({ openFilePicker, upload })`, or lift the upload logic into a hook the parent owns. Severity: Medium.
- **L198** — category: Notifying parent of state changes — Same as L192 for `onUploadReady(handleUpload)`; re-runs whenever `handleUpload` identity changes. → **Fix:** Same as L192. Severity: Medium.

### `frontend/src/components/pm/TaskRelationshipsSection.tsx`
- **L161** — legitimate (scroll/resize listeners to track anchor position, with cleanup).
- **L179** — legitimate (document mousedown outside-click listener with cleanup).
- **L196** — legitimate (document keydown Escape listener with cleanup).
- **L271** — category: Event-specific logic in an effect — When `composerOpen` turns true, inspects `document.activeElement` to decide whether the external trigger opened it. → **Fix:** Set `anchorSource` in the handlers that open the composer (the inline one already does; give the external trigger an `onOpen('external')` callback). Severity: Medium.
- **L281** — category: Data fetching in useEffect — Debounced (220ms) `searchService.search` with `searching`/results state; `popoverTab` change refires the search. → **Fix:** Debounce `query` and use `useQuery` keyed on `[workspaceId,'search',debouncedQuery]`, `enabled: composerOpen && debouncedQuery.length>=2`; pick `tasks` vs `documents` via `select` based on `popoverTab`. Severity: High.
- **L329** — category: Notifying parent of state changes — Calls `onContentChange`/`onCountChange` when relationship counts change. → **Fix:** Parent reads the same associations query (`useTaskAssociations`) and derives counts; remove callbacks. Severity: Medium.

### `frontend/src/components/pm/ViewBar.tsx`
- No issues (1 effect, legitimate: persists open view ids to localStorage on change; could be a `usePersistedState` hook but fine).

### `frontend/src/components/pm/ChecklistItems.tsx`
- **L199** — category: Data fetching in useEffect — `reload()` fetches `pmChecklistService.list` into `items`. → **Fix:** `useChecklist(workspaceId, taskId)` query with `select` sorting by position; `reload()` sites become `invalidateQueries`. Severity: High.
- **L201** — category: Notifying parent of state changes — Pushes `onContentChange`/`onStatsChange` to the parent whenever `items` change. → **Fix:** Parent derives content/stats from the shared `useChecklist` query. Severity: Medium.
- **L209** — legitimate (focus input after it mounts).
- **L214** — legitimate (window event listener with cleanup).

### `frontend/src/components/pm/AgentRunPanel.tsx`
- **L295** — category: Resetting/syncing local state from props — Copies `urlRunId` (router search param) into `selectedRunId` and `drawerOpen` state. → **Fix:** Make the URL the source of truth: `const selectedRunId = urlRunId ?? localSelectedRunId` and `drawerOpen = !!urlRunId || localDrawerOpen`; update the URL in the handlers that select a run. Severity: Medium.
- **L328** — category: Data fetching in useEffect — `fetchAgents()` with `loadingAgents` flag and toast. → **Fix:** `useAgents(workspaceId)` (exists). Severity: High.
- **L332** — category: Data fetching in useEffect — `fetchRuns()` (`agentService.listTargetRuns`) with `loading` flag and selection reconciliation. → **Fix:** `useQuery` keyed on `[workspaceId,'agentRuns','task',taskId]`; L345 listener becomes `invalidateQueries`; selection derived. Severity: High.
- **L345** — legitimate (window event listeners with cleanup).
- **L397** — category: Derived state — Reconciles `selectedAgentId` against `suggestedAgent`, `taskRunnableAgents` and the latest run status by calling `setSelectedAgentId` from an effect. → **Fix:** Compute `effectiveAgentId` with `useMemo` from `userSelectedAgentId` + the same rules; only store explicit user picks. Severity: Medium.

### `frontend/src/components/pm/ShortcutImportWizard.tsx`
- **L382** — legitimate (clears polling intervals on unmount).
- **L398** — category: Data fetching in useEffect — `loadImportHistory()` fetches import statuses with `historyLoading`. → **Fix:** `useQuery` keyed on `[workspaceId,'shortcutImports']`; polling via `refetchInterval` replaces the manual `pollRef` intervals. Severity: High.
- **L402** — category: Derived state — Defaults `docsSpaceId` to the first space from `useDocsSpaces` data and flips `importDocs` off when no spaces exist. → **Fix:** Derive `effectiveDocsSpaceId = docsSpaceId || docsSpaces[0]?.id ?? ''` and `canImportDocs = docsSpaces.length > 0` during render; only store explicit selections. Severity: Medium.
- **L411** — category: Derived state — Clears `docsCollectionId` when it is no longer in `docsCollections`. → **Fix:** Derive `effectiveCollectionId = docsCollections.some(c => c.id === docsCollectionId) ? docsCollectionId : ''`. Severity: Medium.
- **L417** — legitimate (window event listener with cleanup).
- **L1985** — category: Data fetching in useEffect — `loadDetail()` fetches import detail whenever `selectedId`/`rows` change (and auto-selects the first row). → **Fix:** `useQuery` keyed on `[workspaceId,'shortcutImport',selectedId ?? rows[0]?.import_id]`, `enabled` when an id exists; `detailLoading` becomes `isPending`. Severity: High.

### `frontend/src/components/pm/SaveIndicator.tsx`
- No issues (1 effect, legitimate: detects the saving→idle transition and runs a 2.5s timeout with cleanup to show "Saved").

### `frontend/src/components/pm/TaskListView.tsx`
- **L647** — category: Initializing app/state on mount — `displayInit(workspaceId)` hydrates the board display store. → **Fix:** Same as KanbanBoard L587: hydrate in the route loader or make the store self-initialize per workspace. Severity: Low.
- **L689** — category: Initializing app/state on mount (ref mirror) — Mirrors `onOpenTask` into a ref. → **Fix:** Assign during render or `useLatest`/`useEffectEvent`. Severity: Low.
- **L697** — category: Data fetching in useEffect — Fetches labels into `allLabels`. → **Fix:** Shared `useLabels(workspaceId)` query. Severity: High.
- **L701** — legitimate (window `task-created` listener with cleanup).
- **L947** — category: Event-specific logic in an effect — Resets `typeFilter`/`priorityFilter`/etc. to "all" when the corresponding field is hidden. → **Fix:** Reset the filter in the field-visibility toggle handler, or derive the effective filter (`fieldVis.task_type ? typeFilter : ALL`) when building the query. Severity: Medium.
- **L1105** — category: Data fetching in useEffect — Calls `fetchTasksByState()` / `fetchTasksFlat(1,false)` (`pmTaskService.list`/`listBoard`) whenever filters/team/workflow change, with manual `loading`, `hasMore`, `groupHasMore` state. → **Fix:** `useQuery`/`useInfiniteQuery` keyed on `[workspaceId,'tasks',workflowId,teamId,filters,mode]`; per-group load-more becomes per-state infinite queries; realtime listeners (L1239) become cache patches via `setQueryData`. Severity: High.
- **L1115** — category: Resetting/syncing local state from props — Copies `externalTasks` prop into `tasks` state when `isExternal`. → **Fix:** Derive `const tasks = isExternal ? externalTasks : fetchedTasks` during render; keep local state only for the self-fetching mode. Severity: Medium.
- **L1219** — category: Derived state — Prunes `rowSelection` entries whose task no longer exists whenever `tasks` change. → **Fix:** Derive `visibleSelection` with `useMemo` (filter by current task ids) when reading; prune the stored map lazily in the selection handlers. Severity: Medium.
- **L1229** — legitimate (window keydown Escape listener with cleanup).
- **L1239** — legitimate (window event listeners with cleanup; the in-handler `pmTaskService.get` is event-driven, not effect-driven).
- **L1686** — category: Derived state — Resets `groupBy` to `'workflow_state'` when it is not among `visibleGroupOptions`. → **Fix:** Derive `effectiveGroupBy = visibleGroupOptions.some(o => o.value === groupBy) ? groupBy : 'workflow_state'`; persist the fallback only when the user changes options. Severity: Medium.
- **L1861** — legitimate (ResizeObserver on the header with cleanup).
- **L2430** — category: Chained effects — Resets `triggered` ref when `stateId` changes so L2434 can fire again. → **Fix:** Key `<GroupLoadSentinel key={stateId}>` so the ref resets by remount, removing this effect. Severity: Low.
- **L2434** — category: Data fetching in useEffect — Sentinel calls `onLoadMore(stateId)` on mount (relies on the virtualizer only rendering it near the viewport). → **Fix:** Acceptable-with-caveat as a "visible sentinel"; an `IntersectionObserver` (as KanbanBoard does) or `fetchNextPage` from an infinite query is more robust than mount-time triggering. Severity: Low.

### `frontend/src/components/pm/GlobalCreateModals.tsx`
- **L141** — category: Data fetching in useEffect — Reads workflow from the board store if present, else fetches `pmWorkflowService.list` and takes the first; no cancellation. → **Fix:** `useWorkflows(workspaceId)` query with `select: (w) => w[0]` (or `initialData` from the store); the board store should also read from this query so the two sources agree. Severity: High.
- **L243** — category: Data fetching in useEffect — Fetches repositories when the planning repository section is shown. → **Fix:** `useRepositories(workspaceId)` query, `enabled: showPlanningRepository` (shared with CreateTaskModal L490 and TaskDeliveryPanel L61). Severity: High.
- **L1238** — category: Event-specific logic in an effect — Calls `onClose()` whenever `canEdit` is false. → **Fix:** Gate opening in the handler/store action that opens the modal (do not open without `pm.edit`), and render nothing when `!canEdit` as a defensive fallback instead of an effect that navigates/closes. Severity: Medium.

### `frontend/src/components/pm/useEpicDeliveryPlan.ts`
- **L30** — category: Data fetching in useEffect — `reload()` fetches `commandBarService.listEpicPlans` into `plans` state. → **Fix:** `useQuery` keyed on `[workspaceId,'epicPlans',epicId]`, `enabled: !!workspaceId`; the L41 listener becomes `invalidateQueries` and the L60 poll becomes `refetchInterval: plan?.status==='running' ? 5000 : false`. Severity: High.
- **L41** — legitimate (window event listeners with cleanup).
- **L60** — legitimate (setInterval poll with cleanup).

### `frontend/src/components/pm/CodingSession/CodingSessionSurface.tsx`
- **L79** — category: Resetting/syncing local state from props — Copies the `sessionId` prop into `activeSessionId` state. → **Fix:** If the surface can switch sessions internally, use the "previous prop" pattern or key the surface on `sessionId`; otherwise use the prop directly. Severity: Medium.
- **L133** — category: Chained effects — Clears `events`/`artifacts`/`session`/`streamSnapshotSeed` and the sequence ref when `activeSessionId` changes, immediately before L141 refetches them. → **Fix:** `key={activeSessionId}` on the surface (or its data-holding child) so state resets by remount; with queries, the key change does this naturally. Severity: Medium.
- **L141** — category: Data fetching in useEffect — `load()` fetches session, events and artifacts with manual `loading`/`error` state. → **Fix:** `useCodingSession`, `useCodingSessionEvents`, `useCodingSessionArtifacts` queries; websocket handlers (L154) patch the cache with `setQueryData`; L229 poll becomes `refetchInterval` while running. Severity: High.
- **L154** — legitimate (window event listeners with cleanup).
- **L229** — legitimate (setInterval reconcile poll with cleanup).
- **L417** — category: Data fetching in useEffect — Lazily fetches agents for handoff, using `handoffAgents !== null` as a cache flag. → **Fix:** `useAgents(workspaceId)` with `enabled: canSuggestHandoff`. Severity: High.
- **L428** — category: Data fetching in useEffect — Fetches task runs for handoff, caching by `handoffRunsTargetId`; the L441 listener resets the cache to force a refetch. → **Fix:** `useQuery` keyed on `[workspaceId,'agentRuns','task',session.target_id]`, `enabled: canSuggestHandoff`; L441 becomes `invalidateQueries`. Severity: High.
- **L441** — legitimate (window event listeners with cleanup; becomes invalidation after L428 migration).

### `frontend/src/components/pm/CodingSession/CodingSessionHeader.tsx`
- No issues (1 effect, legitimate: 1s setInterval clock with cleanup, restarted when `origin` changes).

### `frontend/src/components/pm/CodingSession/CodingInteractionCard.tsx`
- **L46** — category: Resetting/syncing local state from props — Resets five pieces of local answer/UI state whenever `interaction.interaction_id` changes. → **Fix:** Render with `key={interaction.interaction_id}` from the parent so the card remounts with fresh state; delete the effect. Severity: Medium.

TOTAL: 36 files reviewed, 145 effects reviewed, 42 flagged High, 46 flagged Medium, 13 flagged Low

## B. Docs (components/docs, components/editor, pages/docs)


Note: no `useDebounce*` hook exists under `frontend/src/hooks/`, and there are no TanStack Query hooks for docs comments, references, entity-ref resolution, shared docs, or embed resolution — all of those are fetched ad hoc in effects below.

### `frontend/src/components/docs/HtmlBlockNodeView.tsx`
- **L27** — category: Resetting/syncing local state from props — `setDraft(html ?? '')` whenever the node attr `html` changes (undo/redo) → **Fix:** use the "store previous prop and adjust during render" pattern (`const [prevHtml, setPrevHtml] = useState(html); if (html !== prevHtml) { setPrevHtml(html); setDraft(html ?? '') }`), or key the textarea on `html` when not in source mode. Severity: Medium.
- Other 4 effects (L29 window `message` listener, L42 editor `selectionUpdate` subscription, L60 document `mousedown` click-outside, L75 focus/autosize textarea after mode switch) are legitimate.

### `frontend/src/components/docs/TaskItemMetadataToolbar.tsx`
- No issues (1 effect, all legitimate: TipTap `selectionUpdate`/`transaction` subscription with cleanup; state is lazily initialised).

### `frontend/src/components/docs/EmojiPickerPopover.tsx`
- No issues (3 effects, all legitimate: click-outside listener, Escape key listener, DOM measurement of cursor coords via `editor.view.coordsAtPos` after render).

### `frontend/src/components/docs/MoveDocumentDialog.tsx`
- **L49** — category: useEffect used to reset form state when dialog opens — resets `spaceId`/`collectionId` from props whenever `open` (or the current ids) change → **Fix:** render the dialog body only when `open` (or `key={\`${currentSpaceId}:${currentCollectionId}\`}` on the content) so `useState(currentSpaceId)` initialisers do the reset; drop the effect. Severity: Medium.
- **L57** — category: Chained effects — when `spaceId` (set by the user or by L49) differs from `currentSpaceId`, nulls `collectionId` in a second effect → **Fix:** do it in the space `onValueChange` handler (`setSpaceId(v); setCollectionId(v === currentSpaceId ? currentCollectionId ?? null : null)`). Severity: Medium.

### `frontend/src/components/docs/DocsOutlineMinimap.tsx`
- **L77** — category: Chained effects — calls `computeActive()` when `items` change, but `computeActive` is a `useCallback` that already depends on `items`, so the scroll-listener effect at L56 (which also depends on `computeActive` and calls it on setup) already re-runs on every `items` change; this effect is redundant → **Fix:** delete L77–L79. Severity: Low.
- L56 (rAF-throttled scroll/resize listener with cleanup) is legitimate.

### `frontend/src/components/docs/VideoEmbedNodeView.tsx`
- No issues (2 effects, all legitimate: editor `selectionUpdate` subscription, document click-outside listener).

### `frontend/src/components/docs/CreateCollectionDialog.tsx`
- **L68** — category: useEffect used to reset form state when dialog opens — populates/clears name, description, icon, space, parent whenever `open`, `collection`, defaults, or `spaces` change. Because `spaces` (query data) and `collection` are in deps, a background refetch while the dialog is open wipes what the user typed → **Fix:** mount the form body only while `open` and key it on `collection?.id ?? 'new'`; initialise each `useState` from `collection`/defaults directly. Severity: Medium.
- **L87** — category: Chained effects — after L68 runs, a second effect back-fills `selectedSpaceId` when `spaces` load → **Fix:** derive during render: `const effectiveSpaceId = selectedSpaceId || defaultSpaceId || spaces?.[0]?.id || ''` and use that for the Select value and submit payload. Severity: Medium.

### `frontend/src/components/docs/SpaceDialog.tsx`
- **L48** — category: useEffect used to reset form state when dialog opens — populates form from `space` or resets to defaults on open → **Fix:** render dialog content conditionally on `open` with `key={space?.id ?? 'new'}` and seed `useState` initialisers from `space`/`defaultType`. Severity: Medium.

### `frontend/src/components/docs/DocsEditor.tsx`
- **L855** — category: Notifying parent of state changes — `onSaveStatusChange?.(saveStatus, lastSavedAt)` fires from an effect after every status change (parent `DocsDocumentDetail` mirrors it into its own state) → **Fix:** wrap the setters in a helper (`const updateSaveStatus = (s, at) => { setSaveStatus(s); setLastSavedAt(at); onSaveStatusChange?.(s, at) }`) and call it from the save/flush/error paths, or lift `saveStatus`/`lastSavedAt` to the parent and pass them down. Severity: Medium.
- **L1788** — category: Event-specific logic in an effect — on mount detects `initialContent._markdown_source`, calls `editor.commands.setContent`, `persistImportedImages` and `scheduleSave`; deps include two callbacks (`persistImportedImages`, `scheduleSave`), so any identity change re-imports and re-saves the doc → **Fix:** convert markdown before creating the editor (compute the real initial content in a `useMemo`/lazy `useState`, pass it to `useEditor({ content })`) and run the image persist + save once from `useEditor`'s `onCreate`; at minimum guard with a `didImportRef`. Severity: Medium.
- Remaining 11 effects are legitimate: L159 30s interval; L332 editor `selectionUpdate`/`blur` subscription; L449 click-outside; L953 unmount timer cleanup; L1002/L1007 presence emit to WebSocket on `readOnly`/unmount; L1530 `onEditorReady(editor)` (editor instance comes from `useEditor`, not local state — acceptable TipTap pattern); L1534 push `commentAnchors` into the ProseMirror plugin; L1549 `editor.setEditable`; L1558 sync `initialContent` into the editor after revert (external-system sync, first-run skip via ref); L1755 keyboard shortcuts.

### `frontend/src/components/docs/TableControls.tsx`
- **L222** — category: Derived state — `useEffect(() => { updateBarPositions() })` with no dependency array runs imperative DOM positioning after every render of the component → **Fix:** give it an explicit deps list (`[hover, activeCellRect, cursorTableRect, tableSelected, menu]`) and use `useLayoutEffect` to avoid a flicker frame; the ResizeObserver at L135 already covers table size changes. Severity: Low.
- L63 (wrapper `mousemove` with timer cleanup), L135 (editor subscriptions + ResizeObserver), L227 and L239 (click-outside listeners) are legitimate.

### `frontend/src/components/docs/DocumentLinksPanel.tsx`
- No issues (1 effect, all legitimate: focuses the search input after it renders).

### `frontend/src/components/docs/EntityEmbedDialog.tsx`
- **L93** — category: useEffect used to reset form state when dialog opens — clears `query`/`items`/`error` when `open` becomes false → **Fix:** mount the dialog body only while `open` (state is discarded on unmount) or reset in the `onOpenChange(false)` handler. Severity: Medium.
- **L101** — category: Data fetching in useEffect — hand-rolled 180ms debounce + `searchEntityEmbedItems` with `items`/`loading`/`error` useState triplet and a cancelled flag → **Fix:** `const deferred = useDeferredValue(query.trim())` (or a small `useDebouncedValue`) and `useQuery({ queryKey: queryKeys.docs.entityEmbedSearch(workspaceId, fixedEntityType, deferred), queryFn, enabled: open && deferred.length >= 2, placeholderData: keepPreviousData })`; drop the three local states. Severity: High.

### `frontend/src/components/docs/TableOfContentsNodeView.tsx`
- No issues (1 effect, all legitimate: editor `update`/`transaction` subscription; state lazily initialised).

### `frontend/src/components/docs/BlockHoverHandle.tsx`
- No issues (2 effects, all legitimate: MutationObserver + DOM listeners around the third-party drag-handle element with full cleanup; `mousemove` tracking on the editor DOM).

### `frontend/src/components/docs/InsertEmbedDialog.tsx`
- **L26** — category: Data fetching in useEffect — 250ms debounce then `docsService.resolveEmbed` into `serverResolved`/`resolving` state with cancel flag → **Fix:** `const deferredUrl = useDeferredValue(fallbackResolved?.url)` and `useQuery({ queryKey: ['docs','resolveEmbed', workspaceId, deferredUrl], queryFn, enabled: open && !!workspaceId && !!deferredUrl, staleTime: 5 * 60_000 })`; `resolved = data ?? fallbackResolved`, `resolving = isFetching`. Severity: High.

### `frontend/src/components/docs/CalloutNodeView.tsx`
- No issues (2 effects, all legitimate: editor `selectionUpdate` subscription, document click-outside listener).

### `frontend/src/components/docs/CommentSideGutter.tsx`
- No issues (1 effect, all legitimate: rAF-throttled scroll/resize/editor-update subscription with cleanup).

### `frontend/src/components/docs/BlockGapInserter.tsx`
- No issues (1 effect, all legitimate: wrapper `mousemove` listener with rAF throttling and timer cleanup).

### `frontend/src/components/docs/EntityMentionNodeView.tsx`
- **L51** — category: Data fetching in useEffect — calls `docsService.resolveEntityRefs` per mention node and stores the result in `resolved` state → **Fix:** `useQuery({ queryKey: ['docs','entityRef', workspaceId, entityType, entityId], queryFn, enabled: !!workspaceId && !!entityId, staleTime: 60_000 })` with `select` to apply the "unavailable" fallback; this also dedupes identical mentions across the document. Severity: High.

### `frontend/src/components/docs/SlashMenu.tsx`
- **L35** — category: Event-specific logic in an effect — detects the open→closed transition of the ProseMirror plugin state via `wasOpenRef` and then deletes the slash text and resets submenu state → **Fix:** the L71 `transaction` handler already sees every plugin-state change; perform the close side-effects there (compare previous/next `open`), or have the plugin delete the slash text itself in its close transaction. Severity: Medium.
- **L71** — category: Effects that could be `useSyncExternalStore` — subscribes to `editor` transactions and `forceUpdate()`s when a serialised plugin-state key changes → **Fix:** `useSyncExternalStore(cb => { editor.on('transaction', cb); return () => editor.off('transaction', cb) }, () => serialisedPluginState(editor))`. Severity: Low.
- **L159** — category: Event-specific logic in an effect — reacts to the plugin's `executeSelected` flag (Enter key) by running `executeCommand`; deps only list the flag → **Fix:** pass an `onExecute` callback into the plugin (via extension options / editor storage) and call `executeCommand` directly from the plugin's keydown handler instead of setting a flag and reacting in React. Severity: Medium.
- L60 (sync `commandCount` into the plugin), L173 (submenu keydown listener), L202 (scroll selected item into view), L222 (position menu from `coordsAtPos`), L249 (click-outside) are legitimate.

### `frontend/src/components/docs/CreateDocumentDialog.tsx`
- **L53** — category: useEffect used to reset form state when dialog opens — resets title/space/collection on open; `spaces` is in deps, so a refetch while open clears the user's title → **Fix:** render the dialog body only while `open` and seed `useState` initialisers from `defaultSpaceId ?? spaces?.[0]?.id`. Severity: Medium.
- **L60** — category: Chained effects — back-fills `spaceId` once `spaces` load → **Fix:** derive `const effectiveSpaceId = spaceId || defaultSpaceId || spaces?.[0]?.id || ''` during render. Severity: Medium.
- **L67** — category: Chained effects — whenever `collections` (query data) changes, overwrites `collectionId` with the first collection or `''`; a background refetch resets a collection the user explicitly picked → **Fix:** derive `const effectiveCollectionId = collectionId || collections?.[0]?.id || ''` during render and only reset `collectionId` in the space `onValueChange` handler. Severity: High.

### `frontend/src/components/docs/proposals/ProposalReviewView.tsx`
- No issues (1 effect, all legitimate: focuses the title heading after render when the proposal changes).

### `frontend/src/components/docs/ExcalidrawNodeView.tsx`
- **L54** — category: Resetting/syncing local state from props — copies `scene` into refs and `setEditorInitialScene(themedScene)` whenever `scene`/`themedScene` change while the dialog is closed → **Fix:** snapshot `themedScene` into `editorInitialScene` in the dialog open handler (`setEditorInitialScene(themedScene); setDialogOpen(true)`) or key the dialog content on `dialogOpen`; keep the refs updated inside that handler too. Severity: Medium.
- **L91** — category: Data fetching in useEffect — dynamically imports `@excalidraw/excalidraw` on first dialog open and stores the component in state → **Fix:** module-level `const Excalidraw = React.lazy(() => import('@excalidraw/excalidraw').then(m => ({ default: m.Excalidraw })))` rendered inside `<Suspense>` when the dialog is open. Severity: Low.
- L62 (async PNG export → object URL with revoke on cleanup) is legitimate.

### `frontend/src/components/editor/MermaidBlock.tsx`
- **L17** — category: Data fetching in useEffect — async `renderMermaidSvg(source)` with idle/loading/ready/error state and a cancelled flag → **Fix:** `useQuery({ queryKey: ['mermaid', trimmed], queryFn: () => renderMermaidSvg(trimmed), enabled: !!trimmed, staleTime: Infinity })` gives caching across re-renders/remounts of the same diagram and removes the state machine. Severity: Low.

### `frontend/src/components/editor/CodeBlockNodeView.tsx`
- **L74** — category: useEffect used to reset form state when dialog opens — `setSearch('')` plus focusing the input when the language dropdown opens → **Fix:** clear `search` in the click handler that calls `setOpen(true)` (or `autoFocus` on the input and mount it only while `open`); the focus call alone would be a legitimate effect. Severity: Low.
- L81 (click-outside listener) is legitimate.

### `frontend/src/pages/docs/DocsSpaceDetail.tsx`
- **L126** — category: Event-specific logic in an effect — after `collections` load, if the `?collection=` param no longer exists it toasts and `navigate({ replace: true })` → **Fix:** validate the param in the route's `loader`/`beforeLoad` (redirect with a search flag the page turns into a toast), or render `<Navigate replace>` during render when `collections` are loaded and the id is missing. Severity: Low.

### `frontend/src/pages/docs/SharedDocumentView.tsx`
- **L22** — category: Data fetching in useEffect — `docsService.getSharedDoc(shareToken)` into `doc`/`content`/`loading`/`error` useState quadruplet, no cancellation → **Fix:** add `useSharedDocsDocument(shareToken)` in `frontend/src/hooks/queries/` (`useQuery({ queryKey: ['docs','shared', shareToken], queryFn: async () => unwrap(await docsService.getSharedDoc(shareToken)), enabled: !!shareToken })`) and read `data.document`/`data.content`, `isPending`, `error`. Severity: High.

### `frontend/src/pages/docs/DocsDocumentDetail.tsx`
- **L549** — category: Data fetching in useEffect — loads `docsCommentService.list(wsId,'doc',docId)` into `comments`/`commentsLoading` state with a cancel flag → **Fix:** `useDocsComments(wsId, docId)` query hook (`queryKey: ['docs', wsId, docId, 'comments']`, `enabled: !!doc`); have `CommentSideGutter`'s create/remove calls become `useMutation`s that `invalidateQueries` that key instead of mutating local arrays. Severity: High.
- **L635** — category: Data fetching in useEffect — loads `docsService.listReferences` into `references`/`referencesLoading` when the rail switches to `references` → **Fix:** `useDocsReferences(wsId, docId, { enabled: railView === 'references' })` via `useQuery`; the rail condition becomes `enabled`, loading state comes from `isPending`. Severity: High.
- **L739** — category: Resetting/syncing local state from props — copies `doc.hc_og_*` fields into `sourceSocialDraft` whenever they change → **Fix:** use the same `{ docId, value }` pattern already used for `titleDraftState` (store the draft keyed by `doc.id` and fall back to the doc values during render: `const draft = sourceSocialDraft?.docId === doc.id ? sourceSocialDraft.value : docValues`), or key the social-metadata form on `doc.id`. Severity: Medium.
- Remaining 6 effects are legitimate: L202 editor subscription (in `useFocusedDocsBlockId`), L596 `popstate` listener, L735 unmount timer cleanup, L782 WebSocket presence start/stop, L946 scroll-to-`#block-` hash with `hashchange` listener, L1096 clears pending translation save timer on locale change.

### `frontend/src/components/docs/SearchReplaceBar.tsx`
- **L31** — category: Resetting/syncing local state from props — mirrors `initialShowReplace` into local `showReplace` state (Ctrl+F vs Ctrl+H) → **Fix:** make it controlled: `DocsEditor` already owns `showSearchReplace` (L906), so pass `showReplace`/`onShowReplaceChange` down instead of an "initial" prop; alternatively key `<SearchReplaceBar key={String(showSearchReplace)}>` in the parent. Severity: Medium.
- L26 (focus input on mount) and L36 (editor `transaction` subscription → plugin state) are legitimate.

### `frontend/src/components/docs/BlockCommentTrigger.tsx`
- No issues (2 effects, all legitimate: L25 resolves the `.docs-editor-wrapper` host after the TipTap view is mounted in the DOM — cannot be computed during render; L47 wrapper `mousemove`/`mouseleave` listeners with timer cleanup).

### `frontend/src/components/docs/EntityEmbedNodeView.tsx`
- **L114** — category: Data fetching in useEffect — per-node `docsService.resolveEntityRefs` call driving an idle/loading/loaded/error state union with a cancel flag → **Fix:** `useQuery({ queryKey: ['docs','entityRef', workspaceId, entityType, entityId], queryFn, enabled: access !== 'redacted' && !!workspaceId && !!entityId, staleTime: 60_000 })` and map `data`/`error`/`isPending` to the existing view states in render; share the hook with `EntityMentionNodeView` so identical refs are deduped. Severity: High.

TOTAL: 31 files reviewed, 83 effects reviewed, 8 flagged High, 16 flagged Medium, 7 flagged Low

## C. Settings, auth and account pages


Note: `frontend/src/hooks/` has no `useDebounce*` hook today; debounce findings below propose adding one (or `useDeferredValue`).

### `frontend/src/components/settings/CRMEmailSettingsTab.tsx`
- **L96** — category: Resetting/syncing local state from props — copies 8 fields from the `useEmailSyncSettings` query result into local `useState`s whenever `settings` changes (also flips `defaultsLoaded`) → **Fix:** hold a single `draft` object initialised lazily from `settings` and remount the form on `settings.updated_at`/`workspaceId` via `key`, or split the form into a child that receives `settings` as `initial` and uses `useState(() => initial)`. Severity: Medium.
- **L111** — category: Data fetching in useEffect — when the settings query errors and defaults are not loaded, calls `crmEmailSyncSettingsService.getDefaultPrefixes()` and stores the result → **Fix:** `useQuery({ queryKey: queryKeys.crm.emailSyncDefaultPrefixes(workspaceId), enabled: isError })` in `hooks/queries/`, and derive `blockedRecordPrefixes` initial value from either query. Severity: High.

### `frontend/src/components/settings/WorkflowsTab.tsx`
- **L52** — category: Data fetching in useEffect — calls `loadWorkflows()` (`pmWorkflowService.list`) on `workspaceId` change with manual `loading`/`workflows` state; `loadWorkflows` is also called after mutations to refresh → **Fix:** `useQuery` (`usePMWorkflows(workspaceId)`) and `invalidateQueries` after create/update/delete mutations. Severity: High.

### `frontend/src/components/settings/WorkspaceRepositoriesTab.tsx`
- **L130** — category: Data fetching in useEffect — runs `loadGitStatus` (`listIntegrations` + `listRepositories`) when the callback identity changes → **Fix:** two `useQuery` hooks (`useGitIntegrations`, `useGitRepositories(workspaceId, { all: true })`); invalidate after wiring repos. Severity: High.
- **L135** — category: Chained effects + eslint-disable — once integrations have loaded (effect above sets state), fires `loadAvailableRepos(defaultPickerIntegrationId)` to fetch the first provider's available repos; carries `eslint-disable-next-line react-hooks/exhaustive-deps` → **Fix:** derive `activeIntegrationId = repoPickerIntegrationId ?? repoProviderIntegrations[0]?.id` during render and fetch with `useQuery({ queryKey: ['git','available-repos', activeIntegrationId, debouncedSearch], enabled: !!activeIntegrationId })`. Severity: High.
- **L144** — category: useEffect to compute a debounced value — copies `repoSearchInput` into `debouncedRepoSearch` after 300 ms → **Fix:** `useDeferredValue(repoSearchInput)` or a shared `useDebounce(value, 300)` hook. Severity: Low.
- **L150** — category: useEffect that just calls refetch when a param changes (+ eslint-disable) — re-runs `loadAvailableRepos` when `debouncedRepoSearch` changes → **Fix:** include the search term in the query key of the `useQuery` proposed for L135; the effect disappears. Severity: High.

### `frontend/src/components/settings/ImportHistory.tsx`
- **L29** — category: Data fetching in useEffect — `docsImportService.listJobs(workspaceId)` then filters by `source` and stores into `jobs`/`loading` → **Fix:** `useQuery` keyed on `workspaceId`, with `select: (jobs) => source ? jobs.filter(...) : jobs`; drop the `loading` state (use `isLoading`). Severity: High.

### `frontend/src/components/settings/StickyFormFooter.tsx`
- No issues (1 effect, all legitimate: locates nearest scroll container and subscribes to its `scroll` event with cleanup).

### `frontend/src/components/settings/ChatGeneralTab.tsx`
- **L123** — category: Resetting/syncing local state from props — copies ~40 fields from the `useChatSettings` result into ~40 individual `useState`s whenever `data`/`billingLoading`/`canRemoveBranding` change, and mutates `initializedRef`/`lastSyncedDraftRef` → **Fix:** replace the 40 states with one `settingsDraft` state initialised lazily from `data.settings` (`useState(() => buildSettingsDraftFromServer(s))`) inside a child component rendered only when `data && !billingLoading`, keyed on `workspaceId`; `buildSettingsDraftFromServer` already exists. Severity: Medium.
- **L243** — category: Event-specific logic in an effect (notify-on-change) — watches the serialised draft key and schedules a debounced `updateMutation.mutate` 800 ms later, tracking save status via refs → **Fix:** route every field change through a single `updateDraft(patch)` handler that sets state and schedules the debounced save; the effect, `initializedRef`, `settingsDraftRef` and `mutateSettingsRef` all go away. Severity: Medium.
- (L458 and L518 are `useEffect` text inside the `reactSnippet`/`nextjsSnippet` template-string code samples, not real effects — not counted.)

### `frontend/src/components/settings/SupportContentSourcesField.tsx`
- **L122** — category: Event-specific logic in an effect (+ eslint-disable) — opens the create wizard whenever the parent bumps the `createRequestToken` counter prop → **Fix:** expose `openCreateWizard` via `useImperativeHandle`/`forwardRef` (or lift `wizardOpen` state to the parent) so the parent's click handler calls it directly instead of incrementing a token. Severity: Medium.
- **L129** — category: Event-specific logic in an effect (+ eslint-disable) — same token pattern for `editRequestToken`/`editSourceId`; also re-runs when `sources` refetches → **Fix:** same imperative-handle / lifted-state approach; pass the source object from the parent's handler. Severity: Medium.

### `frontend/src/components/settings/CRMAutonomySettingsTab.tsx`
- **L23** — category: Resetting/syncing local state from props — copies 5 fields from the `useAutonomySettings` result into local state on every `settings` change → **Fix:** lazy-init a single `draft` from `settings` in a child form rendered once `settings` exists, keyed on `workspaceId`. Severity: Medium.

### `frontend/src/components/settings/MembersTab.tsx`
- **L121** — category: Data fetching in useEffect — `loadData()` fetches members + invitations on `workspaceId` change with manual `loading` state; `loadData` is re-invoked after each mutation → **Fix:** `useWorkspaceMembers(workspaceId)` + `useInvitations(workspaceId, { enabled: editable })` via `useQuery`; invalidate after invite/remove/role-change mutations. Severity: High.

### `frontend/src/components/settings/WidgetPreview.tsx`
- No issues (2 effects, all legitimate: mounts/re-renders the Preact widget (third-party) on prop change, and unmounts it on component unmount).

### `frontend/src/components/settings/WorkflowStatesTab.tsx`
- **L66** — category: Data fetching in useEffect — `loadWorkflows()` fetches workflows on `workspaceId` change, then picks a selected workflow; manual `loading` state → **Fix:** `usePMWorkflows(workspaceId)` via `useQuery`; derive `selectedWorkflowID` during render (`initialWorkflowId` if present, else user selection if still valid, else first). Severity: High.

### `frontend/src/components/settings/RedirectsTab.tsx`
- **L190** — category: useEffect to compute a debounced value — debounces `search` into `debouncedSearch` and resets `page` to 1 → **Fix:** `useDebounce(search, 300)` hook (or `useDeferredValue`) and reset `page` in the search `onChange` handler. Severity: Low.
- **L218** — category: Data fetching in useEffect — runs `loadRedirects` (with `loading`/`redirects`/`total` state) whenever workspace/search/type/page change; also called manually after mutations → **Fix:** `useQuery({ queryKey: queryKeys.docs.redirects(workspaceId, { search, type, page }) })` with `placeholderData: keepPreviousData`; invalidate after create/update/delete. Severity: High.

### `frontend/src/components/settings/WorkflowManager.tsx`
- **L549** — category: Resetting/syncing local state from props — when `initialTemplate` changes (tracked via a JSON signature) copies 9 template fields into the "add rule" form state and marks the signature applied → **Fix:** render the add-rule form as a child keyed on `templateSignature` and lazily initialise its fields from `initialTemplate`; no signature bookkeeping needed. Severity: Medium.
- **L832** — category: Data fetching in useEffect — `loadWorkflows()` fetches workflows and computes `selectedId`; manual `loading` state → **Fix:** `usePMWorkflows(workspaceId)` via `useQuery`; derive `selectedId` during render from `initialWorkflowId`/`initialTeamId`/user selection. Severity: High.
- **L840** — category: Data fetching in useEffect — `loadAutomationRules()` (`automationRuleService.list`) on callback change → **Fix:** `useAutomationRules(workspaceId)` via `useQuery`; invalidate after rule mutations. Severity: High.
- **L841** — category: Data fetching in useEffect — `agentService.list(workspaceId)` into `agents` → **Fix:** `useAgents(workspaceId)` via `useQuery`. Severity: High.
- **L844** — category: Data fetching in useEffect — three parallel fetches (epics, tasks page, repositories) into local state for the event-rule pickers, with no cancellation → **Fix:** three `useQuery` hooks (`useEpics`, `useTasks`, `useGitRepositories`), ideally `enabled` only when the add-rule form is open. Severity: High.

### `frontend/src/components/settings/ImportTab.tsx`
- **L45** — category: Data fetching in useEffect — `workspacesService.listMembers(workspaceId)` into `members` → **Fix:** `useWorkspaceMembers(workspaceId)` via `useQuery`. Severity: High.

### `frontend/src/components/settings/AutomationsTab.tsx`
- **L29** — category: Data fetching in useEffect — fetches automations + epic states with a `cancelled` flag and manual `loading` → **Fix:** two `useQuery` hooks (`usePMAutomations`, `useEpicWorkflowStates`); `isLoading` replaces the flag. Severity: High.

### `frontend/src/components/settings/OrgGitConnectionsTab.tsx`
- **L101** — category: Data fetching in useEffect — `loadIntegrations()` fetches org integrations, workspace repos, and per-integration usage into 3 states → **Fix:** `useQuery` for integrations/repos and a dependent `useQueries` (or a backend endpoint returning usage inline) for per-integration usage; invalidate after connect/sync/disconnect. Severity: High.
- **L105** — category: Event-specific logic in an effect — on mount reads `?github_app`/`?github_message` from `window.location`, toasts, strips the params via `history.replaceState`, and reloads integrations; re-runs if `loadIntegrations` identity changes → **Fix:** declare `github_app`/`github_message` as validated route search params and handle them in the route's `beforeLoad`/`loader` (toast + `navigate({ search: {...}, replace: true })`), leaving the query to refetch via invalidation. Severity: Low.

### `frontend/src/components/settings/TeamsTab.tsx`
- **L82** — category: Resetting/syncing local state from props — mirrors `initialTeamId` prop into `selectedTeamId` state → **Fix:** "store previous prop and adjust during render" pattern, or make the parent own selection (it already passes `initialTeamId` from the URL). Severity: Medium.
- **L94** — category: Event-specific logic in an effect — auto-opens the sprint dialog when `initialSection === 'sprints'` and a team is selected → **Fix:** lazy-init `useState(() => initialSection === 'sprints' && !!initialTeamId)` (or open it in the navigation handler that sets `?section=`). Severity: Low.
- **L107** — category: Event-specific logic in an effect — same pattern for `initialSection === 'delivery'` → `repoDialogOpen` → **Fix:** same as L94. Severity: Low.
- **L136** — category: Data fetching in useEffect — loads workspace members + pending invitations with a `mounted` flag → **Fix:** `useWorkspaceMembers(workspaceId)` and `useInvitations(workspaceId, { enabled: editable || isSelectedTeamManager, select: pendingOnly })` via `useQuery`. Severity: High.
- **L165** — category: Data fetching in useEffect — `gitService.listRepositories` with `mounted` flag and `repositoriesLoading` → **Fix:** `useGitRepositories(workspaceId)` via `useQuery`. Severity: High.
- **L183** — category: Data fetching in useEffect — `pmWorkflowService.list` with `mounted` flag → **Fix:** `usePMWorkflows(workspaceId)` via `useQuery` (same hook as WorkflowsTab/WorkflowManager). Severity: High.
- **L200** — category: Data fetching in useEffect — `agentService.list` into `pipelineAgents` → **Fix:** `useAgents(workspaceId)` via `useQuery`. Severity: High.
- **L205** — category: Data fetching in useEffect — `pmAutomationService.list` into `automations` → **Fix:** `usePMAutomations(workspaceId)` via `useQuery`. Severity: High.
- **L230** — category: Derived state — sets `defaultTaskType` from `TEAM_TYPE_PRESETS[teamType]` whenever `teamType` changes and the user hasn't touched the field → **Fix:** set it inside the `teamType` change handler (`if (!storyTypeTouched) setDefaultTaskType(preset)`), or derive `effectiveTaskType = storyTypeTouched ? defaultTaskType : preset` during render. Severity: Medium.
- L361 — legitimate: clears the `workflowSavedTimerRef` timeout on unmount.

### `frontend/src/components/settings/NextraImportWizard.tsx`
- **L75** — category: Data fetching in useEffect — on mount reads a saved job id from `localStorage`, fetches its status, and starts a `setInterval` poll (`startPolling`) that itself fetches every 2 s → **Fix:** `useQuery({ queryKey: ['docs-import-job', workspaceId, jobId], enabled: !!jobId, refetchInterval: (q) => q.state.data?.status is running/pending ? 2000 : false })`, with `jobId` lazily initialised from `localStorage`; derive `step` from job status. Severity: High.
- L99 — legitimate: 1 s ticker (`setInterval`) for the ETA display with cleanup.

### `frontend/src/components/settings/KnowledgeTab.tsx`
- **L157** — category: Resetting/syncing local state from props — resets `companyDescription`/`companyContextExpanded` from the workspace store whenever the workspace's context/description changes → **Fix:** key the company-context editor on `${workspace.id}:${workspace.company_product_context ?? workspace.description}` (or on `workspace.updated_at`) and lazy-init the draft. Severity: Medium.

### `frontend/src/components/settings/SystemTab.tsx`
- **L21** — category: Resetting/syncing local state from props — resets three fields from the `config` prop on every `config` object change (the `useState` initialisers already read `config`) → **Fix:** have the parent render `<SystemTab key={config.updated_at ?? JSON.stringify(config)} />` (or key on `workspaceId`) and delete the effect. Severity: Medium.

### `frontend/src/components/settings/ConversationRoutingTab.tsx`
- **L945** — category: Resetting/syncing local state from props — hydrates the `draft` state from `installation.settings` (query data) on change; carries an explicit `set-state-in-effect` eslint-disable → **Fix:** move the editable routing form into a child rendered once `installation` is loaded, keyed on `workspaceId`, with `useState(() => buildRoutingDraft(installation.settings))`. Severity: Medium.

### `frontend/src/components/settings/helpcenter/AutoTranslateMissingDialog.tsx`
- **L77** — category: Event-specific logic in an effect (+ eslint-disable) — on every `open→true` transition resets results and fires one `autoTranslateMissing` request per locale, splicing results into the parent and toasting a summary → **Fix:** move the run into a `useMutation` (or plain async function) invoked by the parent's "Auto-translate" click handler that also opens the dialog; the dialog becomes a pure renderer of mutation state, and `runIdRef` becomes unnecessary. Severity: Medium.
- **L179** — category: Derived state (ref sync) — copies `results` into `resultsRef` after each render so the `allSettled` finaliser can read the latest snapshot → **Fix:** compute the summary from the `allSettled` return values (each locale's outcome) instead of reading state; or assign `resultsRef.current = results` during render. Severity: Low.

### `frontend/src/components/settings/helpcenter/HelpcenterTranslationsTable.tsx`
- **L116** — category: Data fetching in useEffect — `loadData()` fetches spaces, then collections and translations per space/collection into four `useState` maps with manual `loading` → **Fix:** `useQuery` for spaces plus dependent `useQueries` for per-space collections/translations (or a single backend "translations overview" endpoint); keep the optimistic splice helpers as `setQueryData` updates. Severity: High.

### `frontend/src/components/settings/HelpcenterTab.tsx`
- **L624** — category: Data fetching in useEffect — loads helpcenter config, external spaces, and (conditionally) collections, then derives defaults and writes `config`/`spaces`/`homepageSpaceSlug`/`spaceCollections`/`loading`; no cancellation on `workspaceId` change → **Fix:** `useHelpcenterConfig(workspaceId)`, `useDocsSpaces(workspaceId)` (already exists) and a dependent `useDocsCollections(spaceId)` via `useQuery`; initialise the editable `config` draft lazily from those results in a child keyed on `workspaceId`. Severity: High.
- **L709** — category: Chained effects (+ eslint-disable) — waits for the previous effect to set `loading=false`, then snapshots `config` into `savedSnapshot` for dirty detection → **Fix:** compute the baseline once where the loaded config is built (set `savedSnapshot` in the same place `setConfig` is called, or derive `isDirty` by comparing the draft to the query data). Severity: High.

### `frontend/src/components/settings/teams/SprintSettingsForm.tsx`
- **L67** — category: Resetting/syncing local state from props — re-copies six props into local state when `teamId`/configs change (the `useState` initialisers already do this) → **Fix:** have `TeamsTab` render `<SprintSettingsForm key={teamId} />` (and/or key on `autoCreateConfig?.updated_at`) and delete the effect. Severity: Medium.

### `frontend/src/components/settings/teams/FieldVisibilityForm.tsx`
- **L53** — category: Resetting/syncing local state from props — rebuilds `fields` from `initial` when `initial`/`teamId` change, duplicating the lazy initialiser → **Fix:** `key={teamId}` (or `initial?.updated_at`) on the form from the parent; delete the effect. Severity: Medium.

### `frontend/src/components/settings/GeneralTab.tsx`
- **L58** — category: Resetting/syncing local state from props — resets name/website/key/logo/timezone from the workspace store on `workspace.id`/`updated_at` change → **Fix:** key the form on `${workspace.id}:${workspace.updated_at}` and keep the existing lazy `useState` initialisers. Severity: Medium.
- **L66** — category: Data fetching in useEffect — `workspacesService.getKeyHistory(workspaceId)` into `keyHistory`, re-run on `updated_at` → **Fix:** `useQuery({ queryKey: queryKeys.workspaces.keyHistory(workspaceId) })` and invalidate it after the key-change mutation. Severity: High.
- L93 — legitimate: 60 s `setInterval` clock for the timezone preview with cleanup.

### `frontend/src/components/settings/HelpCenterImportSection.tsx`
- **L77** — category: Data fetching in useEffect — same resume-from-`localStorage` + `setInterval` polling as NextraImportWizard → **Fix:** `useQuery` with conditional `refetchInterval` keyed on the saved `jobId`; derive `step` from status. Severity: High.
- L100 — legitimate: 1 s ETA ticker with cleanup.

### `frontend/src/pages/settings/BillingSettingsPage.tsx`
- **L299** — category: Resetting/syncing local state from props — sets `choosingPlan=true` whenever the `openPlanChooser` prop becomes true (parent is also notified via `onPlanChooserChange`) → **Fix:** make the chooser fully controlled by the parent (`open={openPlanChooser}` + `onOpenChange`) and drop the local mirror. Severity: Medium.
- **L303** — category: Event-specific logic in an effect — on mount parses `?billing=`/`?checkout_session_id=` from `window.location`, rewrites history, fires `confirmCheckout.mutateAsync` and toasts; re-runs if `confirmCheckout`/`refetch` identities change → **Fix:** declare the params as route search params and handle the checkout return in the route's `loader`/`beforeLoad` (or a `useEffect([])`-free handler triggered from the search-param value with a `useRef` guard); keep the mutation in `useMutation`. Severity: Medium.
- **L347** — category: useEffect that just calls refetch (polling) — while `checkoutConfirming`, polls `refetch()` every 3 s up to 20 times until Stripe data appears → **Fix:** pass `refetchInterval: checkoutConfirming ? 3000 : false` to `useWorkspaceBilling` (and derive "confirmed" from the query data with a `dataUpdatedAt`-based attempt cap), removing the manual interval. Severity: Medium.

### `frontend/src/components/settings/teams/EstimateSettingsForm.tsx`
- **L22** — category: Resetting/syncing local state from props — re-copies five fields from `initial` on `initial`/`teamId` change (duplicating the initialisers) → **Fix:** `key={teamId}` on the form from `TeamsTab`; delete the effect. Severity: Medium.

### `frontend/src/pages/AccountSettings.tsx`
- **L43** — category: Resetting/syncing local state from props — copies `currentOrganization.name` into `name` state on org change → **Fix:** `useState(() => currentOrganization?.name ?? '')` inside a form keyed on `currentOrganization.id`. Severity: Medium.
- **L49** — category: Data fetching in useEffect — `organizationsService.listMembers(orgId)` with manual `loadingMembers` → **Fix:** `useOrganizationMembers(orgId)` via `useQuery({ enabled: !!orgId })`; invalidate after remove-member. Severity: High.
- **L101** — category: Resetting/syncing local state from props — mirrors `user.default_workspace_id` into `defaultWsId` → **Fix:** derive the select value from `user.default_workspace_id` directly (it is only "saved" via a mutation), or key the section on that value. Severity: Medium.

### `frontend/src/pages/Profile.tsx`
- **L101** — category: Resetting/syncing local state from props — resets six avatar/name fields from the auth store `user` on change (initialisers already read the same values) → **Fix:** key the profile form on `${user.id}:${user.updated_at}` (or on the persisted avatar tuple) and keep the lazy initialisers. Severity: Medium.
- L93 — legitimate: revokes the `pendingAvatar` object URL on change/unmount (resource cleanup).

### `frontend/src/components/profile/AvatarCropDialog.tsx`
- **L49** — category: useEffect used to reset form state when dialog opens — when `!open || !imageUrl` clears `loadedImage`/`loadError`/`transform`; otherwise loads an `Image()` (legitimate external load with cancel) → **Fix:** render the crop body conditionally / `key={imageUrl}` so the reset branch is unnecessary, keeping only the `Image` load with `cancelled` guard. Severity: Low.
- L84 — legitimate: paints the crop preview onto a `<canvas>` after render.

### `frontend/src/pages/SecuritySettings.tsx`
- **L117** — category: Data fetching in useEffect — `authService.get2FAStatus()` on mount with `cancelled` flag and `loadingTwoFAStatus` → **Fix:** `useTwoFAStatus()` via `useQuery`; invalidate after enable/disable. Severity: High.
- **L141** — category: Data fetching in useEffect — `passkeyService.listPasskeys()` on mount with `cancelled` flag and `loadingPasskeys` → **Fix:** `usePasskeys()` via `useQuery`; invalidate after add/delete. Severity: High.
- **L165** — category: Derived state (async) — generates a QR data-URL from `setupProvisioning.provisioning_uri` via `QRCode.toDataURL`, tracking `loading/ready/error` status by hand → **Fix:** `useQuery({ queryKey: ['qr', uri], queryFn: () => QRCode.toDataURL(uri, ...), enabled: !!uri, staleTime: Infinity })` gives status/data for free; or generate the QR in the same handler that receives `setupProvisioning`. Severity: Low.

### `frontend/src/pages/VerifyEmail.tsx`
- **L18** — category: Data fetching in useEffect — on mount calls `authService.verifyEmail(token)` and stores `status`/`errorMessage`; also updates the auth store → **Fix:** `useQuery({ queryKey: ['verify-email', token], queryFn, enabled: !!token, retry: 0, staleTime: Infinity })` (or a route `loader`) and update the auth store in `queryFn`/`onSuccess`; derive `status` from `isPending/isError`. Severity: High.

### `frontend/src/components/auth/EmailVerificationBanner.tsx`
- No issues (1 effect, all legitimate: 1 s countdown `setInterval` with cleanup while `cooldownSeconds > 0`).

### `frontend/src/pages/SetupSuccessPage.tsx`
- **L68** — category: Derived state — rebuilds `expandedJourneys` from `setup.data.journeys`, using two refs to detect workspace change and first initialisation → **Fix:** store only user overrides (`Record<key, boolean>`) in state, key the page content on `workspaceId`, and derive `expanded = overrides[key] ?? (key === defaultExpandedJourney(journeys))` during render. Severity: Medium.
- L56 — legitimate: fires a one-time analytics event per workspace once setup data is available.
- L89 — legitimate: subscribes to the `setup-journey-navigate` window event with cleanup.

### `frontend/src/pages/Login.tsx`
- No issues (1 effect, all legitimate: starts WebAuthn conditional-mediation (passkey autofill) via `passkeyService`, an external system, with cancellation cleanup).

### `frontend/src/pages/Workspaces.tsx`
- **L234** — category: Resetting/syncing local state from props — pre-fills `orgName`/`orgSlug` from `user.full_name` once it becomes available (initialisers already try this) → **Fix:** derive `defaultOrgName` during render and use `orgName || defaultOrgName` as the displayed/submitted value, or key the org form on `user?.id`. Severity: Medium.
- **L246** — category: Syncing Zustand store from query data — sets `currentOrganization` in the org store from the `useOrganizations` query when none is selected → acceptable-with-caveat; **Fix:** ideally do this once in the workspace/organization route `beforeLoad`/loader, or make the store derive "current" from `organizations` + saved id. Severity: Low.
- **L256** — category: Event-specific logic in an effect — auto-opens the create-workspace dialog when `?create=true` and data has loaded, then navigates to clear the param → **Fix:** handle `create` in the route's `beforeLoad`/`loader` (or lazily init `dialogOpen` from the search param once queries are settled via a `useRef` guard); clear the param in the same place. Severity: Low.
- **L656** — category: Initializing state on mount — on entering full-page onboarding calls `resetWorkspaceDialog({ useEmailDefaults: true })` once, tracked via `fullPageOnboardingInitialized` → **Fix:** lazily initialise the workspace form fields with `firstWorkspaceDefaults` when `isFullPageOnboarding` (`useState(() => ...)`) and drop the flag. Severity: Low.
- **L664** — category: Notifying parent / syncing URL from state — navigates to `/onboarding?step=<workspaceStep>` whenever `workspaceStep` changes → **Fix:** call `navigate({ search: { step } })` inside the handlers that advance the step (or make `step` a search param that is the source of truth). Severity: Medium.
- **L1040** — category: Event-specific logic in an effect (mutation in effect) — when organizations load empty, auto-creates an organization via `createOrgMutation` once (guarded by `autoOrgCreateAttempted`) → **Fix:** trigger the auto-create from the `useOrganizations` query's success path (e.g. in the route loader, or `onSuccess` of a wrapper hook) rather than an effect watching seven deps. Severity: Medium.

### `frontend/src/components/settings/PipelineBuilder.tsx`
- **L97** — category: Notifying parent of state changes — calls `onUnsavedChange?.(hasUnsavedChanges)` whenever the derived flag flips → **Fix:** compute `hasUnsavedChanges` in the parent (lift `editingId`/`editName`/`newName` or expose them), or call `onUnsavedChange` from the handlers that edit `editName`/`newName`/`addingAfterId`. Severity: Medium.

### `frontend/src/pages/JoinWorkspace.tsx`
- **L42** — category: Data fetching in useEffect — `inviteService.getInfo(token)` into `info`/`error`/`loading` triplet → **Fix:** `useInviteInfo(token)` via `useQuery` (or route `loader`); derive the error message from `error`. Severity: High.

### `frontend/src/components/settings/ProjectDeliveryTab.tsx`
- **L28** — category: Data fetching in useEffect — `loadRepositories()` (`gitService.listRepositories`) on callback change → **Fix:** `useGitRepositories(workspaceId)` via `useQuery` (shared with TeamsTab/WorkspaceRepositoriesTab). Severity: High.

### `frontend/src/pages/Dashboard.tsx`
- **L9** — category: Event-specific logic in an effect (redirect) — navigates to `/w/$slug/pm/my-work` once `currentWorkspace.slug` is known → **Fix:** perform the redirect in the route's `beforeLoad` (`throw redirect({...})`) using workspace data from the loader/store, so no component render + effect round-trip is needed. Severity: Low.

### `frontend/src/components/settings/teams/TeamRepoDefaultForm.tsx`
- **L46** — category: Resetting/syncing local state from props — re-copies seven fields from `initial` on change (duplicating the initialisers) → **Fix:** `key={teamId}` (or `initial?.updated_at`) on the form from `TeamsTab`; delete the effect. Severity: Medium.

### `frontend/src/components/auth/MFARequiredGate.tsx`
- **L46** — category: Derived state (async) — generates a QR data-URL from `setup.provisioning_uri` via `QRCode.toDataURL` with a `cancelled` guard → **Fix:** `useQuery({ queryKey: ['qr', uri], queryFn: () => QRCode.toDataURL(uri, ...), enabled: !!uri, staleTime: Infinity })`, or generate the QR in the handler that sets `setup`. Severity: Low.

TOTAL: 45 files reviewed, 88 effects reviewed, 33 flagged High, 29 flagged Medium, 13 flagged Low

## D. Support, CRM, agents, automation


### `frontend/src/components/support/ConversationActionsMenu.tsx`
- **L99** — category: Resetting/syncing local state from props + Notifying parent of state changes — Resets `subjectDraft` from `conversation.subject` whenever the dialog is closed, and calls `onSubjectDialogOpenChange` whenever `subjectDialogOpen` changes. → **Fix:** Seed `subjectDraft` in the handler that opens the dialog (`setSubjectDraft(conversation.subject); setSubjectDialogOpen(true)`) or key the dialog content on `conversation.subject`; call `onSubjectDialogOpenChange` inside the same open/close handler instead of an effect. Severity: Medium.

### `frontend/src/components/support/CreateConversationDialog.tsx`
- **L45** — category: useEffect used to reset form state when dialog opens — Sets `mailboxId` back to `initialMailboxId` each time `open` flips true. → **Fix:** Render the dialog body only when open (or `key={String(open)}`) and use `useState(initialMailboxId)` as the initializer; alternatively reset in the parent's open handler. Severity: Low.

### `frontend/src/components/support/CreateTaskDialog.tsx`
- **L46** — category: Derived state — Fills `selectedTeamId` from `defaultTeamId ?? teams[0]?.id` once async data arrives, guarded by a `userHasSelected` ref. → **Fix:** Keep only the user's explicit choice in state and derive `effectiveTeamId = selectedTeamId || defaultTeamId || teams[0]?.id || ''` during render; drop the ref and the effect. Severity: Medium.

### `frontend/src/components/support/LinkInsertModal.tsx`
- **L57** — category: useEffect used to reset form state when dialog opens — Resets label/url/search/tab/selection whenever `open` becomes true. → **Fix:** Mount the modal content conditionally on `open` (or `key` it on open + `initialUrl`) so `useState(initialLabel)` / `useState(initialUrl)` initializers do the reset. Severity: Low.

### `frontend/src/components/support/ConversationList.tsx`
- **L385** — category: Notifying parent of state changes — Pushes the derived boolean `shouldShowOnboardingEmptyState` up to the parent via `onOnboardingEmptyChange` on every change. → **Fix:** Lift the inputs (navFilter, mailbox, search, list emptiness) so the parent computes the same boolean, or have the parent read it from the shared store/query; avoid the child→parent effect round trip. Severity: Medium.
- **L389** — category: Event-specific logic in an effect — Auto-selects the first conversation (`handleSelect`, which navigates and marks read) whenever there is no selection and the list has items. → **Fix:** Perform the default selection where the data arrives (route loader / the place that sets `selectedConversationId`), or derive `effectiveSelectedId = selectedConversationId ?? filteredConversations[0]?.id` in render and only navigate on explicit user action. Severity: Medium.
- L395 — legitimate (WebSocket presence sync).

### `frontend/src/components/support/SupportAttachmentGallery.tsx`
- No issues (1 effect, all legitimate: window keydown listener with cleanup).

### `frontend/src/components/support/SupportInboxLayout.tsx`
- **L158** — category: Passing data to parent / syncing Zustand store from query data — 118-line effect that parses route search params, saved view filters and custom views into the Zustand inbox store and also performs `navigate(... replace: true)` redirects when `routeConversationId === 'inbox'`. → **Fix:** Move the `'inbox'` redirect into the route's `beforeLoad`/`loader`; derive store state from the route search via a selector (route search is already the source of truth) or sync it once in the route loader. If the store sync must stay, keep it but strip the navigation out of the effect. Severity: Medium.
- **L279** — category: Chained effects — Store→URL sync: when the store's `selectedConversationId` diverges from the route param it schedules a rAF + `setTimeout(0)` then navigates, which re-triggers the L158 URL→store effect (bidirectional sync loop mitigated by timers). → **Fix:** Navigate directly inside the handler that selects a conversation (`selectConversation` / `handleSelect`) and let the route param be the single source of truth; delete this effect and the rAF/timeout guard. Severity: High.

### `frontend/src/components/support/EmailBodyRenderer.tsx`
- No issues (3 effects, all legitimate: unmount timer cleanup, iframe stylesheet/DOM toggling, ResizeObserver on the iframe document).

### `frontend/src/components/support/ReplyComposer.tsx`
- L357 — legitimate (scrollIntoView after render).
- **L853** — category: Initializing app/state on mount — Reads `localStorage` via `loadSkipOfflineEmailConfirm(key)` into state whenever the storage key changes. → **Fix:** `useState(() => loadSkipOfflineEmailConfirm(key))` plus a `key`-based remount, or `useMemo(() => load(key), [key])` since the value is only re-read when the key changes; a `useLocalStorage`-style hook would also fit. Severity: Low.
- **L1003** — category: Resetting/syncing local state from props — Resets `panelIndex` to 0 whenever `manualShortcutQuery` changes. → **Fix:** Call `setPanelIndex(0)` in the `onChange` handler that calls `setManualShortcutQuery`. Severity: Low.
- L1007 — legitimate (focus after render).
- **L1012** — category: Event-specific logic in an effect — Consumes a store-driven `shortcutOpenRequest` counter (guarded by `handledShortcutOpenRequestRef`) to open the shortcut-create panel, then clears the request. → **Fix:** Replace the counter with an imperative channel: either subscribe via `useSupportInboxStore.subscribe` in one place, or expose `openShortcutCreate` through a ref/callback registered on the store so the originating click handler calls it directly. Severity: Medium.
- L1079 — legitimate (typing-indicator cleanup on unmount/conversation change).
- L1338 — legitimate (document pointerdown listener with cleanup).
- L1359 — legitimate (TipTap editor event subscriptions with cleanup).
- L1411 — legitimate (pushes persisted draft into the TipTap editor when the conversation changes; editor is an external system).
- L1428 — legitimate (window custom-event listener with cleanup).
- L1454 — legitimate (forces TipTap placeholder redecoration).

### `frontend/src/components/support/ConversationDetailSidebar.tsx`
- **L271** — category: Resetting/syncing local state from props — Resets four pieces of local UI state whenever `conversationId` changes. → **Fix:** Render the sidebar body with `key={conversationId}` so the state remounts naturally; delete the effect. Severity: Medium.

### `frontend/src/components/support/EmojiPicker.tsx`
- L58 — legitimate (focus input after catalog is ready).
- **L64** — category: Data fetching in useEffect — Lazily loads the emoji catalog (`loadEmojiCatalog()` from widget-core) with manual `isLoading` / `loadError` / cancelled-flag bookkeeping. → **Fix:** `useQuery({ queryKey: ['emoji-catalog'], queryFn: loadEmojiCatalog, enabled: open, staleTime: Infinity })`; drop the three useState fields and cancelled flag (the catalog is also cached across picker instances for free). Severity: High.

### `frontend/src/components/support/MessageThread.tsx`
- L356 — legitimate (thread-transition timer keyed on conversation change, with cleanup).
- **L370** — category: Data fetching in useEffect — Fetches `agentService.listRuns(workspaceId, assignedAgentId)` and filters to the current conversation into local `agentRuns` state (with a `queueMicrotask` reset when inputs are empty). → **Fix:** `useQuery({ queryKey: queryKeys.automation.runsRoot(...)/agent runs by agent, enabled: !!conversationId && !!assignedAgentId, select: runs => runs.filter(...) })`; the empty case becomes `enabled: false` with `data ?? []`. Severity: High.
- **L416** — category: useEffect that just calls refetch()/invalidateQueries when a param changes — Listens to the `agent_run-updated` DOM event and re-runs the same fetch as L370. → **Fix:** Once L370 is a `useQuery`, `useRealtimeSync` already invalidates the agent-run query keys on `agent_run` events (`hooks/useRealtimeSync.ts` L383-390), so this listener can be deleted. Severity: Medium.
- L441 — legitimate (WebSocket viewing presence start/stop).
- L449 — legitimate (global keyboard shortcut Cmd/Ctrl+Z).
- **L477** — category: Chained effects — Toggles `composerReady` false→true via rAF + `setTimeout(0)` on every conversation change purely to defer composer mount. → **Fix:** Use `key={conversationId}` on the composer with `useDeferredValue(conversationId)` (or `startTransition`) to defer the heavy subtree instead of a two-step effect. Severity: Low.
- **L617** — category: Chained effects — Sets `historyHydrated` false then true after a delay to stage rendering of long threads. → **Fix:** Same as L477: `useDeferredValue` / `startTransition` on the grouped-message list, or a `useTimeout`-style hook; if kept, note it is animation staging, not data. Severity: Low.
- L754 — acceptable-with-caveat (marks the open thread read when a new `lastMessageId` arrives while near bottom; there is no user event to hook, so a data-driven side effect is reasonable; could be moved into the messages query `onSuccess`-equivalent / mutation trigger in the WS handler).
- L769 — legitimate (scroll listener + rAF on the viewport, with cleanup).

### `frontend/src/components/support/CustomerProfileDrawer.tsx`
- **L124** — category: Chained effects — Manages `rendered` / `visible` two-step state for enter/exit animation with rAF and a timeout. → **Fix:** Derive `visible` from `open` and keep only an exit-delay (`useTimeout`/`usePresence`-style hook or Radix `Presence`), or use CSS `@starting-style`/`transition-behavior: allow-discrete`; at minimum collapse to a single `useState` for the exit timer. Severity: Low.
- **L135** — category: Data fetching in useEffect — Debounced CRM contact search (`crmSearchService.search`) with manual `contactResults` / `searchingContacts` state and reset-on-close logic. → **Fix:** Add a `useDebouncedValue` hook (none exists under `frontend/src/hooks/` — create `useDebounce.ts`), then `useQuery({ queryKey: queryKeys.crm.search(wsId, debouncedQuery), enabled: open && !hasLinkedContact && debouncedQuery.length >= 2, select: r => r.filter(type==='contact') })`; `isFetching` replaces `searchingContacts`, and the reset-on-close disappears with `enabled`. Severity: High.
- **L454** — category: Resetting/syncing local state from props — `setDraft(value)` whenever `value` changes in the inline editable field. → **Fix:** Store the previous `value` and adjust during render (`if (prev !== value) { setPrev(value); setDraft(value) }`), or key the field on `value` when not editing. Severity: Medium.

### `frontend/src/pages/support/coverage/SupportCoveragePage.tsx`
- **L170** — category: Data fetching in useEffect — Fires four `supportCoverageService` requests in parallel, writes summary/gaps/total/latestClusterRun/mergeSuggestionCount into six `useState`s plus `loading`, with a cancelled flag. → **Fix:** Four `useQuery` hooks (or `useQueries`) keyed on `[wsId, listFilters]`; `isLoading` replaces `loading`, and resetting `selectedGapId`/`selectedGap` on filter change should be done in the filter-change handler. Severity: High.

### `frontend/src/components/crm/ContactsTable.tsx`
- **L189** — category: Derived state — Prunes `optimisticPatches` / `optimisticRemovals` entries whose contact id is no longer in `contacts`. → **Fix:** Do the pruning inside the existing `useMemo` at L202-212 that merges patches into rows (ignore patches for ids not in `contacts`), and let the maps be cleared when the mutation settles; no effect needed. Severity: Medium.
- L529 — legitimate (infinite-scroll trigger driven by the virtualizer's rendered range).

### `frontend/src/components/crm/AssociationsList.tsx`
- **L214** — category: Data fetching in useEffect — Debounced search across support conversations / CRM / PM entities into three result arrays plus `searching`. → **Fix:** `useDebouncedValue(query)` + one `useQuery` keyed on `[pickerSection, debouncedQuery]` whose `queryFn` branches on section (or three `enabled`-gated queries); reset-on-close goes away because the picker content should be keyed on `pickerSection`. Severity: High.

### `frontend/src/components/crm/DealsTable.tsx`
- **L112** — category: Resetting/syncing local state from props — Copies the `deals` prop into `localDeals` state on every change. → **Fix:** Use `deals` directly (optimistic edits should patch the TanStack Query cache via `setQueryData` or a patch map merged in `useMemo`, as `ContactsTable` does) and drop `localDeals`. Severity: Medium.
- **L650** — category: Event-specific logic in an effect — When `editing` flips true, copies `deal.amount` into the input and selects it via `setTimeout`. → **Fix:** Set `value` in the click handler that calls `setEditing(true)` and use `autoFocus` + `onFocus={e => e.target.select()}` on the input. Severity: Medium.
- **L752** — category: Event-specific logic in an effect — Same pattern for `deal.probability`. → **Fix:** Same as L650. Severity: Medium.

### `frontend/src/components/crm/LinkedTasksPanel.tsx`
- **L117** — category: Data fetching in useEffect — Debounced `searchService.search` for tasks to link, with `linkResults` / `linkSearching` state and reset-on-close. → **Fix:** `useDebouncedValue(linkQuery)` + `useQuery({ enabled: linkDialogOpen && debounced.length >= 2, select: tasks => tasks.filter(t => !linkedIds.has(t.id)) })`; key the dialog content on open to drop the reset branch. Severity: High.
- **L138** — category: Derived state — Chooses `selectedTeamId` from current selection, localStorage default, or first team whenever `teams` load. → **Fix:** Keep only the explicit user choice in state and compute `effectiveTeamId` in render: `teams.some(t=>t.id===selected) ? selected : (storedDefault valid ? stored : teams[0]?.id ?? '')`. Severity: Medium.

### `frontend/src/components/crm/CreateContactDialog.tsx`
- **L82** — category: useEffect used to reset form state when dialog opens — Calls `resetForm()` when `open` or any `initialValues` field changes (8 deps). → **Fix:** Mount the form body only while open (or `key` it on `open` + a stable `initialValues` id) and use `useState(initialValues?.x ?? default)` initializers; delete `resetForm` + effect. Severity: Low.

### `frontend/src/components/support/SidebarAssociations.tsx`
- **L120** — category: Resetting/syncing local state from props — Collapses `tasksExpanded` whenever `conversationId` changes. → **Fix:** `key={conversationId}` on the component/subtree so the state remounts. Severity: Low.
- **L124** — category: Data fetching in useEffect — Debounced CRM / PM search into `results` / `crmResults` / `searching`. → **Fix:** `useDebouncedValue(query)` + `useQuery` keyed on `[pickerSection, debouncedQuery]` with `enabled: !!pickerSection && debounced.length >= 2`; key the picker on `pickerSection` to replace the reset branch. Severity: High.

### `frontend/src/components/crm/contact-detail/ContactHeader.tsx`
- **L59** — category: Resetting/syncing local state from props — Keeps `draft` in sync with `initialName` while not editing. → **Fix:** Render `editing ? draft : initialName` and seed `draft` in the handler that enters edit mode (`setDraft(initialName); setEditing(true)`). Severity: Medium.

### `frontend/src/components/support/NewConversationDialog.tsx`
- L311 — legitimate (clears the TipTap editor when the controlled `value` is emptied; editor is an external system).
- **L521** — category: useEffect used to reset form state when dialog opens — Resets ~15 state fields when `open` becomes true (also re-runs if `selectedMailboxId` changes while open) and focuses the recipient input. → **Fix:** Render the dialog content only when `open` (or `key={openCount}`) so `useState` initializers reset everything; keep a tiny focus-on-mount effect (`autoFocus`) for the recipient input. Severity: Medium.

### `frontend/src/components/support/TeamInboxDialog.tsx`
- **L245** — category: useEffect used to reset form state when dialog opens — Rebuilds the whole form from `mailbox` when the dialog opens and resets to defaults when it closes. → **Fix:** Key the dialog content on `${open}-${mailbox?.id}` and initialize `form` with `useState(() => mailbox ? buildFormState(mailbox) : DEFAULT_FORM)`. Severity: Medium.
- **L265** — category: Derived state — Computes `manualRuleDraft` and `deletedManualRuleIds` from `mailboxRoutingRules` when open. → **Fix:** Initialize both from the rules in the keyed component's `useState` initializers (the rules are query data available at mount), or `useMemo` the draft and store only user edits. Severity: Medium.
- **L277** — category: Chained effects — Overwrites `form.workspaceMemberIds` from `mailboxMembers` query data after L245 has already built the form (two effects writing the same `form` object; ordering depends on query timing). → **Fix:** Feed `mailboxMembers` into the same `buildFormState(mailbox, mailboxMembers)` initializer inside the keyed content (render it only once both queries resolve), removing the second write. Severity: High.
- **L331** — category: Derived state — Auto-adds the current workspace member to `workspaceMemberIds` for new inboxes when they belong to `additionalMembers`. → **Fix:** Include the current member in the initial form state (`DEFAULT_FORM` built with `currentWorkspaceMember`) inside the keyed content, or derive `displayedMemberIds` in render. Severity: Medium.

### `frontend/src/components/crm/CompaniesTable.tsx`
- **L102** — category: Resetting/syncing local state from props — Copies `companies` prop into `localCompanies` state. → **Fix:** Use the prop directly and route optimistic edits through the query cache (`setQueryData`) or a patch map merged in `useMemo`, as `ContactsTable` does. Severity: Medium.

### `frontend/src/components/support/coverage/GapDetailPane.tsx`
- L224 — legitimate (1s interval while a regenerate lock is active, with cleanup).
- **L230** — category: Resetting/syncing local state from props — Closes the quick-draft panel whenever `gap.id` changes. → **Fix:** `key={gap.id}` on the pane so local state remounts; delete the effect. Severity: Low.

### `frontend/src/pages/crm/CompanyDetail.tsx`
- **L67** — category: Resetting/syncing local state from props — Seeds `form` from the `company` query result once (`if (company && !form)`). → **Fix:** Split into a child `<CompanyForm key={company.id} company={company} />` rendered only when `company` is loaded, initializing `form` with `useState(() => toForm(company))`. Severity: Medium.
- **L80** — category: Event-specific logic in an effect — Debounced autosave: accumulates `pendingPatch` state, then an effect waits 650ms and calls `updateCompany.mutateAsync`, juggling `saving` / `saveError` and re-merging on failure. → **Fix:** Use a debounced callback (`useDebouncedCallback`, or a `useRef` timer inside the field `onChange` handler) that calls `updateCompany.mutate(patch)`; use the mutation's `isPending` / `error` instead of `saving` / `saveError`. Severity: Medium.

### `frontend/src/pages/crm/DealDetail.tsx`
- **L86** — category: Resetting/syncing local state from props — Seeds `form` from `deal` once. → **Fix:** Same as `CompanyDetail` L67: keyed child with lazy `useState` initializer. Severity: Medium.
- **L99** — category: Event-specific logic in an effect — Same debounced-autosave-in-effect pattern via `pendingPatch`. → **Fix:** Same as `CompanyDetail` L80: debounced callback in the change handler + mutation status. Severity: Medium.

### `frontend/src/pages/crm/Deals.tsx`
- **L208** — category: Passing data to parent / syncing Zustand store from query data — Calls `initDisplay(wsId)` (Zustand store loads persisted display prefs for the workspace) on mount / wsId change. → **Fix:** Acceptable-with-caveat: store initialization from `localStorage` is an external-system sync. Nicer: make the store lazily read prefs keyed by `wsId` inside its selectors, or use `zustand/persist` so no component-level init is needed. Severity: Low.

### `frontend/src/components/agents/dock/useAgentRunStream.ts`
- **L135** — category: Resetting/syncing local state from props — Resets refs and three pieces of state whenever `runId` changes, and flips a `cancelledRef` on cleanup. → **Fix:** Have the consumer mount the hook's owner with `key={runId}` so all state/refs reset naturally; keep only the cancel flag if the fetch stays imperative. Severity: Low.
- **L150** — category: Data fetching in useEffect — Performs the initial `refetch()` (snapshot + events, with a manual `loading` state and sequence cursor) and re-runs it on three window events. → **Fix:** Model as `useQuery` for the snapshot plus `useInfiniteQuery`/cursor-based `useQuery` for events keyed on `[workspaceId, runId, seq]`; `useRealtimeSync` already invalidates `queryKeys.automation.runsRoot` on `agent_run` events, so the DOM listeners can be replaced by query invalidation. Severity: High.
- **L169** — category: useEffect that just calls refetch() — Interval polling fallback while `active`. → **Fix:** `refetchInterval: active ? pollMs : false` on the query from L150. Severity: Low.

### `frontend/src/components/agents/AskAgentsDock.tsx`
- **L47** — category: Data fetching in useEffect — Loads the chat list (`dockChatService.listChats`) into the Zustand dock store and validates the persisted `activeChatId`. → **Fix:** `useQuery({ queryKey: queryKeys.dock.chats(workspaceId), queryFn })`; read chats from the query instead of the store, and reconcile `activeChatId` with a `select`/derived value (`activeChatId valid ? it : chats[0]?.id`). Severity: High.
- L83 — legitimate (global `/` keyboard shortcut).
- L99 — legitimate (window custom-event listener).
- L113 — legitimate (MutationObserver on Radix dialog state).

### `frontend/src/components/agents/dock/DockInput.tsx`
- No issues (2 effects, all legitimate: textarea auto-resize measuring `scrollHeight` after render; autoFocus on mount).

### `frontend/src/pages/crm/ContactDetail.tsx`
- **L825** — category: Passing data to parent / syncing Zustand store from query data — Pushes the contact name and a JSX actions node into the global page-header store on every `fullName`/`saving`/`saveError` change, resetting on unmount. → **Fix:** Acceptable-with-caveat (header portal pattern). A `<PageHeaderSlot>` portal component or `useTitle`-style hook would remove the manual store writes; at minimum memoize the actions node so the effect doesn't re-run on every save tick. Severity: Low.
- **L852** — category: Resetting/syncing local state from props — Seeds the 17-field `form` from `contact` once (`if (contact && !form)`). → **Fix:** Render a `<ContactForm key={contact.id} contact={contact} />` child once loaded with a lazy `useState(() => toForm(contact))` initializer. Severity: Medium.
- **L877** — category: Event-specific logic in an effect — Debounced autosave via `pendingPatch` state + 650ms timer + `updateContact.mutateAsync`, with manual `saving`/`saveError`. → **Fix:** Debounced callback invoked from the field change handlers; use `updateContact.isPending` / `.error`. Severity: Medium.
- **L902** — category: Data fetching in useEffect — Debounced CRM company search into `companyResults` / `companySearching`. → **Fix:** `useDebouncedValue(companyQuery)` + `useQuery({ enabled: companyPickerOpen && debounced.length >= 2, select: r => r.filter(company && !linked) })`. Severity: High.
- **L922** — category: Data fetching in useEffect — Debounced CRM deal search into `dealResults` / `dealSearching`. → **Fix:** Same as L902 with `type === 'deal'` in `select`. Severity: High.

### `frontend/src/pages/crm/Contacts.tsx`
- **L42** — category: Derived state — Opens the search box (`setShowSearch(true)`) whenever the `search` URL param is present. → **Fix:** Derive `const searchVisible = showSearch || !!searchParams.search` in render (or lazy-init `useState(() => !!searchParams.search)`). Severity: Medium.
- L48 — legitimate (focus the input after it is shown).

### `frontend/src/components/agents/dock/ChatView.tsx`
- **L52** — category: Effects with `// eslint-disable-next-line react-hooks/exhaustive-deps` — Copies `initialDraft` into the composer `value` and calls `onDraftConsumed` (a parent notification) whenever the prop changes; deps are suppressed. → **Fix:** Have the parent that sets the pending draft write it through a ref/imperative handle or key `ChatView` on the draft request id and use `useState(initialDraft)`; call `onDraftConsumed` at the point the draft is dispatched, not in an effect. Severity: Medium.
- **L81** — category: Data fetching in useEffect — On chat switch, clears `detail`/`plans`/`pendingEcho`, sets `detailLoading`, and calls `refreshDetail()` (`dockChatService.getChat`). → **Fix:** `useQuery({ queryKey: queryKeys.dock.chat(workspaceId, chatId) })`; `isLoading` replaces `detailLoading`, and per-chat local state resets via `key={chatId}`. Severity: High.
- **L91** — category: useEffect that just calls refetch()/invalidateQueries when a param changes — Refetches the chat detail on the `agent_run-updated` DOM event for the current run. → **Fix:** Rely on `useRealtimeSync` invalidating the run/chat query key (add the dock chat key to its `agent_run` branch) once L81 is a query. Severity: Medium.
- **L103** — category: Data fetching in useEffect — Fetches every plan in `detail.plan_ids` via `Promise.all(commandBarService.getPlan)` into `plans` state. → **Fix:** `useQueries({ queries: planIds.map(id => ({ queryKey: queryKeys.commandBar.plan(wsId,id), queryFn })) })` or a single query keyed on the sorted id list. Severity: High.
- **L127** — category: Data fetching in useEffect — When the run is paused on human input, fetches `listChatRunInteractions` and picks the latest pending row into `fallbackInteraction`. → **Fix:** `useQuery({ queryKey: [..., chatId, 'interactions'], enabled: pausedOnInteraction, select: rows => latestPending(rows) })`. Severity: High.
- **L155** — category: Derived state — Clears `pendingEcho` once the transcript contains the echoed user message. → **Fix:** Derive `const showEcho = pendingEcho && !transcriptContains(pendingEcho)` in render (`useMemo`) and clear `pendingEcho` only when a new message is sent. Severity: Medium.
- L164 — legitimate (pin scroll to bottom after content renders).

### `frontend/src/pages/automation/AutomationActivity.tsx`
- **L990** — category: Resetting/syncing local state from props — Mirrors the `run_id` URL param into `selectedRunId` / `drawerOpen` state. → **Fix:** Lazy-init both from the search param (as `AgentRuns.tsx` L164-165 already does) and set them in the click handlers that also update the URL; or derive `drawerOpen = !!(search.run_id ?? selectedRunId)`. Severity: Medium.
- **L1076** — category: Resetting/syncing local state from props — Resets `loadedExecutionPages` whenever the filter signature or `search.page` changes. → **Fix:** Store the "load more" count keyed by the filter signature (`useState` reset via `key={activityFilterSignature}` on the list, or the previous-value-during-render pattern), or reset it in the filter-change handler. Severity: Medium.
- **L1117** — category: useEffect that just calls refetch()/invalidateQueries when a param changes — Refetches three TanStack queries on `agent_run-created/updated` DOM events. → **Fix:** `useRealtimeSync` already invalidates `queryKeys.automation.runsRoot` on `agent_run` events; ensure the executions/overview keys live under that root (or add them to the invalidation list) and delete this listener. Severity: Medium.
- **L1211** — category: Derived state — Adds any active filter keys to `visibleActivityFilterKeys`. → **Fix:** Derive `visibleKeys = union(userVisibleKeys, activeActivityFilterKeys)` in a `useMemo` instead of writing state. Severity: Medium.

### `frontend/src/pages/automation/CustomAgentCreatePanel.tsx`
- **L175** — category: Derived state — Mirrors `form` into `latestFormRef` for use in async callbacks. → **Fix:** Low-priority stylistic: a `useLatest(form)` helper, or pass `form` into the callbacks explicitly / use functional `setForm` reads; acceptable as-is. Severity: Low.
- **L179** — category: Initializing app/state on mount — Sets `started = true` when `isEditMode` and focuses the name input. → **Fix:** `useState(isEditMode)` for `started`; keep a tiny focus-on-mount effect (or `autoFocus`) for the input. Severity: Low.
- L188 — legitimate (unmount cleanup bumping a request token to cancel in-flight drafts).
- L192 — legitimate (progress interval while drafting, with cleanup).

### `frontend/src/pages/automation/AgentRuns.tsx`
- **L200** — category: Data fetching in useEffect — Calls `loadData()` (two `automationService` requests) into `runs`/`agents`/`totalRuns` with a `loading`/`refreshing`/`error` triplet. → **Fix:** Two `useQuery` hooks (`queryKeys.automation.runsRoot(workspaceId)`, agents list); `isLoading`/`isFetching`/`error` replace the manual states. Severity: High.
- **L204** — category: useEffect that just calls refetch()/invalidateQueries when a param changes — Re-runs `loadData(true)` on `agent_run-*` DOM events. → **Fix:** Delete once L200 is a query; `useRealtimeSync` already invalidates the runs key on `agent_run` events. Severity: Medium.

### `frontend/src/pages/automation/AutomationFlows.tsx`
- **L1316** — category: Resetting/syncing local state from props — Re-syncs the textarea `text` from the normalized `value` prop when they diverge semantically. → **Fix:** Previous-prop-during-render pattern (`if (prevValue !== normalizedValue) { setPrev(...); setText(normalizedValue) }`) or key the editor on the row id. Severity: Medium.
- L2174 — legitimate (ResizeObserver truncation measurement).
- **L2678** — category: Notifying parent of state changes — Clears `draft.agentId` via `onDraftChange` when the selected agent disappears from `availableAgents`. → **Fix:** Validate in render (treat an unknown `agentId` as empty for display/validation) and clear it in the agent-select handler or at submit; avoid a parent-mutating effect. Severity: Medium.
- **L2684** — category: Notifying parent of state changes — Forces `draft.targetId = workspaceId` whenever `targetMode === 'workspace'`. → **Fix:** Set `targetId` in the handler that switches `targetMode` to `'workspace'`, or derive the effective target id at submit time. Severity: Medium.
- **L3485** — category: useEffect used to reset form state when dialog opens — Resets the install wizard `step` when opened / template changes. → **Fix:** `key={template?.key}` on the dialog content mounted only while open, with `useState<Step>('inputs')`. Severity: Low.
- **L4485** — category: Event-specific logic in an effect — Prefills the composer draft from URL search params once per `searchSignature` (guarded by `appliedSearchSignature` state). → **Fix:** Handle the prefill in the route `loader`/`beforeLoad` or in the navigation handler that sets those search params; if it must stay, lazy-init the draft from `search` inside a component keyed on `searchSignature`. Severity: Medium.

### `frontend/src/pages/automation/Agents.tsx`
- **L2834** — category: Data fetching in useEffect — Kicks off five loaders (`loadAgents`, `loadProviderOptions`, `loadPresets`, `loadToolCatalog`, `loadSkillCatalog`) that write into separate `useState`s plus `loading`. → **Fix:** Five `useQuery` hooks (or `useQueries`) under `queryKeys.automation.*`; drop the useCallback loaders and manual loading state. Severity: High.
- **L2912** — category: Data fetching in useEffect — Calls `loadFleetData()` (run stats + trigger usage) on mount. → **Fix:** `useQuery` keyed on `[workspaceId, 'fleet']`. Severity: High.
- **L2916** — category: Data fetching in useEffect — Loads per-agent analytics when the analytics drawer tab is open. → **Fix:** `useQuery({ queryKey: [..., editingAgent.id, agentAnalyticsRange], enabled: systemDrawerOpen && systemDrawerTab === 'analytics' && !!editingAgent?.id })`. Severity: High.
- **L2921** — category: useEffect that just calls refetch()/invalidateQueries when a param changes — Re-runs fleet + analytics loads on `agent_run-*` DOM events. → **Fix:** Delete once the above are queries; `useRealtimeSync` invalidates `queryKeys.automation.runsRoot` on `agent_run` events (add the fleet/analytics keys under that root). Severity: Medium.

TOTAL: 39 files reviewed, 109 effects reviewed, 20 flagged High, 40 flagged Medium, 17 flagged Low

## E. PM pages, ui/, layout, command-bar, search, hooks, routes


Note: no shared `frontend/src/hooks/useDebounce*` exists; `date-picker.tsx` defines a private `useDebounce`. Recommendations below that mention `useDebounce` assume promoting that one to `hooks/`.

### `frontend/src/pages/pm/Labels.tsx`
- **L55** — category: Resetting form state when dialog opens (+ eslint-disable) — `if (open) { setForm(initial); setTimeout(focus) }` keyed only on `open`, with `exhaustive-deps` disabled to dodge the unstable `initial` object → **Fix:** Render `LabelDialog` only while open (or `key={editingLabel?.label.id ?? 'new'}`) so `useState(initial)` seeds fresh each open; use `autoFocus` on the name `Input` instead of the `setTimeout`. Severity: Medium.
- **L179** — category: Data fetching in useEffect — `useEffect(() => { loadData() }, [loadData])` runs `pmLabelService.listWithStats` and manages a `labels`/`loading`/`error` triplet; no cancellation → **Fix:** `useQuery({ queryKey: queryKeys.pm.labelsWithStats(workspaceId, { scopeFilter, onlyArchived }), queryFn })` in `hooks/queries`; `handleSave`/`handleDelete` become `useMutation` + `invalidateQueries` (optimistic delete via `setQueryData`). Severity: High.

### `frontend/src/pages/pm/SupportSearch.tsx`
- **L338** — category: Resetting/syncing local state from props — `useEffect(() => setDraft(activeSearch), [activeSearch])` mirrors URL-derived `activeSearch` into editable `draft`, producing one render with a stale draft after every navigation → **Fix:** Keep `prevActiveSearch` in state and reset `draft` during render when it changes, or move `draft` into `SupportSearchToolbar` and `key` it on a serialized `activeSearch`. Severity: Medium.

### `frontend/src/pages/pm/SprintDetail.tsx`
- **L165** — category: Data fetching in useEffect — async IIFE fetching sprint, tasks, epics, sprints via `Promise.all` with `loading`/`error` triplet; no cancellation, so a fast `sprintId` change can let a stale response win → **Fix:** `useQuery` per resource (`useSprint`, `useSprintTasks`, existing epics/sprints list hooks) keyed on `[workspaceId, sprintId]`; seed `form` from `sprint` via the prev-prop pattern or `key` the editor on `sprint.sprint.id`. Severity: High.
- **L192** — category: Event-specific logic in an effect (chained) — auto-save effect watches `pendingPatch`/`saving`/`descriptionPendingUploads`, schedules a 650 ms timer that PUTs the patch, and its own `setSaving`/`setPendingPatch` re-trigger the effect → **Fix:** Debounce in the handler: `queuePatch` merges into a ref and (re)arms one timer that calls `useMutation(pmSprintService.update)`; keep the "wait for uploads" guard inside the flush function. Severity: Medium.
- **L335** — legitimate: `window` `task-panel-updated`/`task-panel-archived` listeners with cleanup (after L165 migrates, the handler should `invalidateQueries` instead of re-fetching by hand).

### `frontend/src/pages/pm/EpicDetail.tsx`
- **L314** — category: Data fetching in useEffect — `useEffect(() => { fetchData() }, [fetchData])` fires six parallel service calls into six `useState`s plus `loading`/`error`; also reused as a manual refetch from four call sites → **Fix:** `useQueries`/individual `useQuery` hooks (`useEpic`, `useEpicTasks`, epics/sprints/objectives/repositories lists) keyed on `[workspaceId, epicId]`; `fetchData(false)` call sites become `queryClient.invalidateQueries`. Severity: High.
- **L340** — category: Data fetching in useEffect (chained) — resets `commentsLoading`/`activityLoading`/`showAllActivity` then calls `reloadComments()`/`reloadActivity()` (both fetch and `setState`) whenever the callbacks' identity changes → **Fix:** `useQuery(queryKeys.pm.comments(workspaceId, epicId))` and a `useEpicActivity` query; the L348 window listeners then call `invalidateQueries`. Severity: High.
- **L348** — legitimate: `window` `epic-child-updated`/`epic-updated` listeners with cleanup.
- **L374** — category: Data fetching in useEffect (+ derived state) — fetches `pmExternalLinkService.listByEntity` to set `hasExternalLinkItems`, `externalLinkCount` and auto-open the section; no cancellation → **Fix:** `useQuery(queryKeys.pm.entityExternalLinks(workspaceId, 'epic', epicId))`; derive `hasExternalLinkItems`/`externalLinkCount` from `data.length` during render and initialize `showExternalLinks` from it. Severity: High.
- **L394** — category: Event-specific logic in an effect (chained) — same auto-save-by-state pattern as `SprintDetail` L192 → **Fix:** Debounce inside `queuePatch` with a ref-held timer and a `useMutation(pmEpicService.update)`. Severity: Medium.
- **L694** — legitimate: `window` `task-panel-updated`/`task-panel-archived` listeners with cleanup.

### `frontend/src/pages/pm/Roadmap.tsx`
- **L79** — category: Data fetching in useEffect — `pmRoadmapService.getData(...)` on every filter change with `data`/`loading` state and no cancellation (out-of-order responses can win) → **Fix:** `useQuery({ queryKey: ['pm', workspaceId, 'roadmap', { team_id, objective_id, health, show_completed }], queryFn })`; the L96 `epic-created` listener becomes `queryClient.invalidateQueries`. Severity: High.
- **L96** — legitimate: `window` `epic-created` listener with cleanup (its duplicated fetch body collapses once L79 is migrated).

### `frontend/src/pages/pm/Epics.tsx`
- **L1782** — category: Data fetching in useEffect — `useEffect(() => { void loadData() }, [loadData])` fetches epics, labels and objectives into three `useState`s with `loading`/`error`; `updateEpicField` hand-rolls optimistic updates against that state → **Fix:** `useQuery` for epics (`[workspaceId, teamId, showArchived]`), labels (`queryKeys.pm.labels`) and objectives; inline edits become `useMutation` with `onMutate` optimistic `setQueryData`; the L1807 `epic-created` listener calls `invalidateQueries`. Severity: High.
- **L1788** — category: Initializing state on mount / resetting state from props (chained) — on `storageKey` change it reads `localStorage` and calls `setGroupBy`/`setFilters`/`setVisibleColumns`, gated by a `hasLoadedViewStateRef` handshake with L1797 → **Fix:** Lazy-initialize the three states from `loadEpicViewState(storageKey)` in `useState(() => ...)` and `key` `EpicsPage` (or its inner body) on `storageKey` so a workspace/team switch remounts with the right view. Severity: Medium.
- **L1797** — category: Chained effects (persist-on-change) — re-reads storage and writes `groupBy`/`filters` back after every change, but only after L1788 has flipped the ref; `visibleColumns` is already persisted in its handler → **Fix:** Persist inside the `onGroupByChange` / `setFilters` handlers (or a `useLocalStorageState(storageKey, ...)` hook) exactly as `onVisiblePropertiesChange` already does, and delete the ref. Severity: Medium.
- **L1807** — legitimate: `window` `epic-created` listener with cleanup.
- **L1859** — category: Derived state — prunes `collapsedGroupKeys` to the keys present in `groupedEpics` via `setState` after every grouping change (extra render per filter/sort change) → **Fix:** Keep the raw set in state and compute `visibleCollapsed = useMemo(() => new Set([...collapsedGroupKeys].filter(k => groupKeys.has(k))))` during render; pass that to the table. Severity: Medium.

### `frontend/src/pages/pm/MyWork.tsx`
- **L114** — category: Data fetching in useEffect — `pmTaskService.list` on `[workspaceId, memberId, mode, refreshKey]` with `tasks`/`loading` state and a manual `refreshKey` counter bumped by window events → **Fix:** `useQuery({ queryKey: ['pm', workspaceId, 'tasks', 'my-work', { memberId, mode }], queryFn, enabled: !!memberId })`; the L163 listeners call `invalidateQueries` and the L132 patch uses `queryClient.setQueryData`. Severity: High.
- **L132** — legitimate: `window` `agent_run-updated` listener that patches task rows with cleanup.
- **L163** — legitimate: `window` `task-panel-updated`/`task-panel-archived`/`task-created` listeners with cleanup (`navigate`/`wsSlug` are read from a stale closure because deps only list `memberId` — worth adding them when touching this).

### `frontend/src/pages/pm/ObjectiveDetail.tsx`
- **L279** — category: Resetting/syncing local state from props — `KeyResultRow` mirrors `kr.name` into a local `name` input state after each save → **Fix:** Make the name input mount only while `editingName` (it already does) and seed it with `useState(kr.name)` inside a small `KeyResultNameEditor` child, or `key` the row on `kr.updated_at`. Severity: Medium.
- **L280** — category: Resetting/syncing local state from props — same pattern for `kr.current_value` → `currentValue` → **Fix:** `key={kr.updated_at}` on the row so the inputs remount with fresh `useState` initializers, or track `prevValue` in state and adjust during render. Severity: Medium.
- **L422** — category: Data fetching in useEffect — `LinkEpicPopover` fetches `pmEpicService.list` every time the popover opens with no cancellation or loading state → **Fix:** `useQuery({ queryKey: ['pm', workspaceId, 'epics', { archived: false }], enabled: open })` (shares the cache with the epics page). Severity: High.
- **L542** — category: Data fetching in useEffect — `useEffect(() => { loadData() }, [loadData])` fetching `pmObjectiveService.get` with `data`/`loading`/`error` triplet; `loadData()` is also called as a manual refetch → **Fix:** `useQuery(queryKeys.pm.objective(workspaceId, objectiveId))`; KR/team/owner/epic handlers become `useMutation` with `setQueryData`/`invalidateQueries`. Severity: High.
- **L545** — category: Event-specific logic in an effect (chained) — auto-save-by-state as in `SprintDetail`, plus a `pendingPatchRef` mirror to reconcile the form after save → **Fix:** Debounce in `queuePatch` with a ref-held timer and `useMutation(pmObjectiveService.update)`; the "don't overwrite fields still pending" merge then lives in `onSuccess`. Severity: Medium.

### `frontend/src/components/ui/resizable-image-component.tsx`
- **L53** — category: Resetting/syncing local state from props — copies `alt`/`caption`/`linkUrl`/`linkNewTab` node attrs into four local input states whenever they change → **Fix:** Initialize the alt/link inputs when their popovers mount (they are already conditionally rendered, so move `useState(alt)` into an `AltTextPopover`/`LinkSettingsPanel` child) and drive the caption `<input>` directly from `caption` via `updateAttributes` in `onChange`. Severity: Medium.
- **L118** — legitimate: `window` mouse/touch move+end listeners active only during resize, with cleanup.
- **L149** — category: Resetting/syncing local state from props — when not resizing, copies `width`/`height` attrs into `currentWidth`/`currentHeight` → **Fix:** Store only the in-progress drag size and render `isResizing ? dragSize : { width, height }` (derive during render); no state copy needed. Severity: Medium.
- **L157** — legitimate: `window` `keydown` (Escape) listener while fullscreen, with cleanup.
- **L167** — legitimate: `document` `mousedown` outside-click listener while the settings panel is open, with cleanup.

### `frontend/src/components/ui/favicon.tsx`
- **L28** — category: Resetting/syncing local state from props — `useEffect(() => setImageFailed(false), [computedSrc])` renders one frame with the stale fallback before resetting → **Fix:** Put `key={computedSrc}` on a small `FaviconImage` child that owns `imageFailed`, or store `prevSrc` in state and reset during render. Severity: Medium.

### `frontend/src/components/ui/sidebar.tsx`
- No issues (2 effects, all legitimate: Cmd/Ctrl+B keyboard shortcut with cleanup; cleanup-only effect that restores `document.body` cursor/user-select if unmounted mid-resize).

### `frontend/src/components/ui/date-picker.tsx`
- **L61** — category: useEffect to compute a debounced value — private `useDebounce` hook built on `setTimeout` + `setState` → **Fix:** Promote to `frontend/src/hooks/useDebounce.ts` (or use `useDeferredValue(drafts[activeKey])` since the NLP parse is cheap) so `SearchCommandPalette`/others stop re-implementing it. Severity: Low.
- **L163** — category: Resetting/syncing local state from props (redundant) — `if (!open) setSessionValues(externalValues)`; but `currentValues`/`applyValue` already read `externalValues` while closed and L209 re-seeds `sessionValues` on open, so this effect only causes an extra render → **Fix:** Delete it. Severity: Medium.
- **L209** — category: useEffect used to reset form state when popover opens (+ eslint-disable) — on `open` resets `activeField`, `sessionValues`, `drafts`, `parseError`, `calendarMonth` and preloads the parser → **Fix:** Move the per-open state into a `DatePickerPanel` child rendered inside `PopoverContent` (Radix unmounts it when closed) with `useState` initializers from props; call `preloadNaturalLanguageParser()` in `onOpenChange`. Severity: Medium.
- **L296** — legitimate: focuses the active field's input after render (guarded by the `advanceToRef` handshake).

### `frontend/src/components/ui/calendar.tsx`
- No issues (1 effect, legitimate: focuses the day button when `modifiers.focused` becomes true).

### `frontend/src/components/ui/tiptap-editor.tsx`
- **L449** — legitimate: subscribes to TipTap `update`/`blur` events with `editor.off` cleanup.
- **L503** — category: Notifying parent of state changes — `onEditorReady?.(editor)` + cleanup `onEditorReady?.(null)` on every `editor`/`onEditorReady` identity change; an inline callback from the parent makes it fire `null` then `editor` every render → **Fix:** Expose the editor via `useImperativeHandle`/forwarded ref, or call `onEditorReady` from `useEditor({ onCreate, onDestroy })` once. Severity: Medium.
- **L508** — category: Notifying parent of state changes — same pattern for `onUploadReady(handleFilesUpload)` → **Fix:** Same: expose `uploadFiles` on an imperative handle (the parent already stores it in a ref). Severity: Medium.
- **L518** — legitimate: pushes external `content` changes into the TipTap instance (external system sync).

### `frontend/src/components/ui/email-chip-input.tsx`
- **L78** — category: Focusing DOM node (could be declarative) — `if (autoFocus && !disabled) inputRef.current?.focus()` on mount/`disabled` change → **Fix:** Pass `autoFocus={autoFocus && !disabled}` straight to the `<input>`; keep the effect only if re-focusing on `disabled → false` is intentional. Severity: Low.

### `frontend/src/components/ui/query-builder/QueryBuilderPopover.tsx`
- **L173** — category: Resetting/syncing local state from props — `if (!open) setDraftRules(toDraftRules(value, fields))` re-seeds the draft on every `value`/`fields` change while closed → **Fix:** Seed in the open handler: `onOpenChange={(next) => { if (next) setDraftRules(seed(value, fields)); setOpen(next) }}`, or move `draftRules` into a child rendered inside `PopoverContent` with a `useState` initializer. Severity: Medium.
- **L179** — category: Chained effects / derived state — when open and the draft is empty, pushes a default rule via `setState` → **Fix:** Fold into the same seed function (`rules.length ? rules : [createDraftRule(fields[0])]`) used in the open handler. Severity: Medium.

### `frontend/src/components/layout/Header.tsx`
- No issues (1 effect, legitimate: Cmd/Ctrl+K keyboard shortcut with cleanup).

### `frontend/src/components/layout/AuthenticatedLayout.tsx`
- No issues (1 effect, legitimate: `ResizeObserver`/`resize` measurement of the banner node with cleanup).

### `frontend/src/components/layout/Sidebar.tsx`
- **L115** — category: Syncing Zustand store from query data — `setBuiltinViewFilters(builtinViewFilterMap)` pushes `useSupportBuiltinInboxViews` data into `supportInboxStore` → **Fix:** Acceptable-with-caveat; preferably have consumers read the query (or `queryClient.getQueryData`) instead of a mirrored store field. Severity: Low.
- **L165** — category: Derived state / resetting state from props — `setActiveSetupJourney(setup?.recommended?.journey_key ?? setup?.journeys[0]?.key)` whenever setup data or workspace changes, then user clicks override it → **Fix:** Store only the user override (`selectedJourney`), derive `activeSetupJourney = selectedJourney ?? recommendedKey` during render, and clear the override with the prev-workspaceId pattern. Severity: Medium.
- **L169** — category: Resetting state from props — reloads `expandedTeams` from storage when `workspaceId` changes (initializer already handles first mount) → **Fix:** `key={workspaceId}` on `<Sidebar />` in `$slug.tsx` (it remounts per workspace anyway), or the prev-prop pattern. Severity: Medium.
- **L178** — category: localStorage write on change (could be in handler) — `saveExpandedTeams` after every `expandedTeams` change, including the reset from L169 and the auto-expand from L248 → **Fix:** Persist inside `toggleTeam` (as `toggleSettingsGroup` already does) so only user actions write. Severity: Low.
- **L220** — category: Derived state (+ eslint-disable) — on route change, un-collapses any settings group containing the active link by calling `setCollapsedSettingsGroups` + `saveCollapsedSettingsGroups` → **Fix:** Compute `effectiveCollapsed = collapsed minus groups with an active item` in a `useMemo` and pass that to `SettingsRailNav`; persist only on user toggle. Severity: Medium.
- **L248** — category: Derived state — adds `?team=` param to `expandedTeams` via `setState` after navigation → **Fix:** Derive `visibleExpandedTeams = activeTeamParam ? new Set([...expandedTeams, activeTeamParam]) : expandedTeams` during render. Severity: Medium.

### `frontend/src/components/command-bar/PromotionDialog.tsx`
- **L105** — category: useEffect used to reset form state when dialog opens — resets seven pieces of state `if (open)` (also re-runs on `step`/`planPrompt` identity changes while open) → **Fix:** Render the dialog body as a `PromotionForm` child mounted only while `open` (or `key={run?.id}`) so `useState` initializers reset it. Severity: Medium.
- **L118** — category: Data fetching in useEffect — fetches `commandBarService.getAgentToolCatalog` when open with manual `cancelled` flag and `catalog`/`catalogLoading` state → **Fix:** `useQuery({ queryKey: ['automation', workspaceId, 'agent-tool-catalog', run?.agent_id, step?.allowed_tools], enabled: open && !!workspaceId && !!run?.agent_id })`. Severity: High.

### `frontend/src/components/command-bar/pageContext.tsx`
- **L85** — category: Derived state — prunes `selectedScopeKeys` to registered entry ids via `setState` on every `entries` change; `activeScopeKey` already validates the selection, so this is only GC → **Fix:** Prune inside `unregister` (`setSelectedScopeKeys(cur => omit(cur, id))`) and drop the effect (and the unrelated `fallback` dep). Severity: Medium.
- **L145** — legitimate: registers/unregisters the page context with the provider on mount and when the stable key changes (subscription-style with cleanup).

### `frontend/src/components/search/SearchCommandPalette.tsx`
- **L60** — category: useEffect used to reset state when dialog closes — clears `query`/`results`/`searching` when `open` flips false → **Fix:** Reset in the `onOpenChange` handler wrapper, or key the dialog contents on `open` so state remounts. Severity: Medium.
- **L69** — category: Data fetching in useEffect (+ debounce) — manual `setTimeout` + `AbortController` + `results`/`searching` state per keystroke → **Fix:** `const q = useDebounce(query.trim(), 400)` (or `useDeferredValue`) and `useQuery({ queryKey: ['search', workspaceId, q], queryFn: ({ signal }) => searchService.search(workspaceId, q, { signal }), enabled: !!q, placeholderData: keepPreviousData })`. Severity: High.

### `frontend/src/components/git/RepositoryBranchPicker.tsx`
- **L44** — category: Data fetching in useEffect — resets four states then fetches `gitService.listRepositoryBranches` with a `cancelled` flag and `loading`/`error` state → **Fix:** `useQuery({ queryKey: queryKeys.git.branches(workspaceId, repositoryId), enabled: !!repositoryId, select: sortDefaultFirst })`; `customMode` resets via the prev-`repositoryId` pattern. Severity: High.
- **L113** — category: Derived state — `setCustomMode(false)` whenever `value` is empty or matches a known branch → **Fix:** Derive `const effectiveCustom = customMode && !!value && !branchNames.has(value)` during render (the `selectValue` computation already does most of this) and only set `customMode` in the select handler. Severity: Medium.

### `frontend/src/components/workspace/WorkspaceSelector.tsx`
- **L95** — category: Data fetching in useEffect — loops over `workspaces` calling `workspacesService.listMembers` per workspace into a `membersMap` state; no cancellation, re-fires on every `workspaces` array identity → **Fix:** `useQueries({ queries: workspaces.map(ws => ({ queryKey: queryKeys.workspaces.members(ws.id), queryFn })) })` (same cache the `useWorkspaceMembers` migration would use). Severity: High.

### `frontend/src/routes/_authenticated/w/$slug.tsx`
- **L63** — category: Syncing Zustand store from query data — `setCurrentWorkspace(workspace)` after the `useWorkspaceBySlug` query resolves; the `loading` guard then waits one extra render for `currentWorkspace.id === workspace.id` → **Fix:** Acceptable-with-caveat; better to make `useWorkspaceStore` a thin selector over the query cache (or set the store inside the query's `queryFn`/`select`) so consumers never see the gap. Severity: Low.
- **L67** — legitimate: analytics identify calls when workspace/billing/org/user change.
- **L74** — category: Syncing Zustand store from query data — `setCurrentOrganization(currentOrganization)` mirrored from the orgs query → **Fix:** Acceptable-with-caveat; same as L63 (derive from cache or set within the query). Severity: Low.

### `frontend/src/routes/_authenticated/w/$slug/automation/flows.tsx`
- **L38** — category: Event-specific logic in an effect — on mount/search change, if the raw URL still carries a legacy `create_event_rule` param, `navigate({ search, replace: true })` to strip it → **Fix:** Handle in the route's `beforeLoad` (`throw redirect({ search: validated, replace: true })` when the raw search has `create_event_rule`); no runtime effect needed. Severity: Medium.

### `frontend/src/routes/_authenticated/w/$slug/automation/tools/connections.tsx`
- **L39** — category: Event-specific logic in an effect — reads `external_mcp_oauth` from the URL, fires `toast.success/error`, then `navigate({ search: {} })`; StrictMode dev double-invokes it (two toasts) → **Fix:** Move to the route `loader`/`beforeLoad` (toast, then `throw redirect({ search: {}, replace: true })`), which runs once per navigation. Severity: Medium.

### `frontend/src/hooks/useWorkspaceMembers.ts`
- **L15** — category: Data fetching in useEffect — `workspacesService.listMembers` with a hand-rolled module cache (`cachedWorkspaceId`/`cachedMembers`/`fetchPromise`) and `loading` state; no cancellation, never invalidates → **Fix:** Replace the hook body with `useQuery({ queryKey: queryKeys.workspaces.members(workspaceId), queryFn, staleTime: 60_000, enabled: !!workspaceId })` — TanStack Query already dedupes and caches. Severity: High.

### `frontend/src/hooks/use-mobile.ts`
- **L8** — category: Could be `useSyncExternalStore` — subscribes to `matchMedia` and mirrors the result into `useState` (initial `undefined` gives a first render with `isMobile=false`) → **Fix:** `useSyncExternalStore(subscribe(mql 'change'), () => window.matchMedia(q).matches)` — correct first render, no state/effect. Severity: Low.

### `frontend/src/hooks/useWebSocket.ts`
- No issues (2 effects, all legitimate: window/document activity listeners with cleanup; WebSocket connect/reconnect lifecycle with timer and interval cleanup).

### `frontend/src/components/ui/loading-image.tsx`
- **L17** — category: Resetting/syncing local state from props — `useEffect(() => setIsLoading(Boolean(src)), [src])` re-arms the spinner one render late when `src` changes → **Fix:** Track `prevSrc` in state and call `setIsLoading(Boolean(src))` during render when `src !== prevSrc` (React's documented pattern), or let callers pass `key={src}`. Severity: Medium.

### `frontend/src/hooks/useTitle.ts`
- No issues (1 effect, legitimate: sets `document.title` and restores it on cleanup).

### `frontend/src/components/layout/WorkspaceSwitcher.tsx`
- No issues (1 effect, legitimate: Cmd/Ctrl+1..9 keyboard shortcut with cleanup).

### `frontend/src/components/command-bar/StepToolPicker.tsx`
- **L63** — category: Data fetching in useEffect — lazily fetches `getAgentToolCatalog` on first open with a `catalog` guard, `cancelled` flag and `catalogLoading` state; `selectedTools` in deps means the request payload depends on whichever selection was current at first open → **Fix:** `useQuery({ queryKey: ['automation', workspaceId, 'agent-tool-catalog', agentId], enabled: open && !!workspaceId && !!agentId, staleTime: 5 * 60_000 })` (pass `selectedTools` in the key only if the server response depends on it). Severity: High.
- **L82** — category: localStorage write on change (latent bug) — writes `collapsedGroups` under `COLLAPSE_KEY(agentId)` on mount and on every change; because state is lazily seeded from the initial `agentId` only, an `agentId` prop change writes the previous agent's set under the new key → **Fix:** Persist inside `toggleGroup` and `key` the picker on `agentId` (or reload with the prev-prop pattern). Severity: Medium.

### `frontend/src/hooks/useRealtimeSync.ts`
- **L188** — category: Mirroring a value into a ref via effect — `useEffect(() => { selfIdRef.current = selfId }, [selfId])` → **Fix:** Assign `selfIdRef.current = selfId` directly in the hook body (the same file's `useWebSocket` already does this for its callback refs), or use a `useLatest` helper. Severity: Low.
- **L690** — legitimate: unmount cleanup for debounce/invalidate/typing timers.
- **L704** — category: Syncing Zustand store from hook state — pushes `wsSend` into `supportPresenceStore` (with `null` on cleanup); `useWebSocket` already publishes `send` via `useWSStore` → **Fix:** Acceptable-with-caveat; have `useWebSocket` set `supportPresenceStore.setWsSend` in `onopen`/`onclose` (where it already sets `useWSStore`) and drop this effect. Severity: Low.
- **L711** — category: Syncing Zustand store from hook state — mirrors `isConnected` into `supportPresenceStore.setWsConnected` → **Fix:** Same as L704: set it in `onopen`/`onclose` inside `useWebSocket`, or expose `isConnected` from `useWSStore` and read it there. Severity: Low.

### `frontend/src/hooks/useTableSettings.ts`
- **L95** — category: Chained effects / derived state — when `columnVisibility` (or `allColumnIds` identity) changes, calls `setColumnOrderRaw(prev => normalizeColumnOrder(prev, allColumnIds))`, i.e. a state change triggers an effect that sets more state → **Fix:** Normalize in a `setColumnVisibility` wrapper that updates both states in the same handler, or store the raw order and expose `useMemo(() => normalizeColumnOrder(columnOrder, allColumnIds))`. Severity: Medium.
- **L105** — legitimate: debounced `localStorage` persistence via `setTimeout` with cleanup (could become a small `useDebouncedPersist` hook).

### `frontend/src/hooks/useWorkspaceTeams.ts`
- **L44** — category: Could be `useSyncExternalStore` — registers a listener in a module-level `listeners` array so `invalidateWorkspaceTeamsCache()` can bump a `version` counter → **Fix:** Disappears once data lives in TanStack Query (`invalidateWorkspaceTeamsCache` becomes `queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.settings(wsId) })`). Severity: Low.
- **L85** — category: Data fetching in useEffect — `settingsService.getAll(workspaceId)` behind a hand-rolled module cache, `fetchPromise` dedupe, `version` bump and `loading` state; four `setX` calls per load, no cancellation → **Fix:** `useQuery({ queryKey: queryKeys.workspaces.settings(workspaceId), queryFn, select: mapTeamsPeopleMemberships })`; `getTeamMembers`/`findTeamName` remain `useCallback`s over `data`. Severity: High.

### `frontend/src/hooks/useAssignableWorkspaceMembers.ts`
- **L16** — category: Data fetching in useEffect — `listAssignableMembers` behind a module cache with a `queueMicrotask(() => setLoading(false))` workaround for the cached path → **Fix:** `useQuery({ queryKey: queryKeys.workspaces.assignableMembers(workspaceId), queryFn, staleTime: 60_000, enabled: !!workspaceId })` returning `{ members: data ?? [], loading: isPending }`. Severity: High.

### `frontend/src/main.tsx`
- **L62** — category: Initializing app/state on mount — `useEffect(() => useAuthStore.getState().initialize(), [])` kicks off the session bootstrap from a component effect (double-invoked under StrictMode) → **Fix:** Call `useAuthStore.getState().initialize()` once at module scope next to `initializeAppAnalytics()` (guarded by `!isLogoutPath`), or make it the root route's `beforeLoad`. Severity: Low.
- **L67** — legitimate: starts/stops the token-refresh timer and visibility listener with cleanup; `identifyAnalyticsUser` is analytics.
- **L83** — category: useEffect that just calls `invalidate` when a param changes — `router.invalidate()` whenever `user`/`loading`/`serverUnreachable` change so `beforeLoad` guards re-run → **Fix:** Subscribe the router to the store outside React (`useAuthStore.subscribe((s, prev) => { if (authChanged(s, prev)) router.invalidate() })` at module scope); acceptable-with-caveat as is. Severity: Low.

TOTAL: 39 files reviewed, 90 effects reviewed, 18 flagged High, 32 flagged Medium, 13 flagged Low

## F. widget-core, help-center, website


Context notes for this batch:
- `packages/widget-core` is a Preact library with **no TanStack Query dependency** (peer deps are React/Preact only). "Use `useQuery`" is therefore not the right fix there; the idiomatic fix is one shared cancellable async hook (e.g. `packages/widget-core/src/hooks/useAsyncResource.ts` — a `hooks/` dir does not exist yet) that replaces the four near-identical `cancelled`/`isLoading`/`error` blocks. Adding TanStack Query to an embeddable widget bundle is not recommended.
- `help-center` is a TanStack Start (SSR) app and `website` is Next.js; client-only DOM effects there are legitimate for hydration.
- `frontend/src/hooks/` has no `useDebounce*` hook; `help-center/src/hooks/useDebouncedValue.ts` exists and is the pattern to copy.

### `packages/widget-core/src/components/PreChatForm.tsx`
- No issues (1 effect, all legitimate: `scrollIntoView` on the form after the `step` render changes — DOM scroll after render).

### `packages/widget-core/src/components/MessageBubble.tsx`
- No issues (1 effect, all legitimate: document `mousedown`/`touchstart`/`keydown` listeners for outside-click/Escape close of the sources popover, with cleanup).

### `packages/widget-core/src/components/HelpSpaceView.tsx`
- **L41** — category: Data fetching in useEffect — Fetches `fetchHelpCollections(host, widgetKey, space.slug)` inside an effect with a `cancelled` flag, plus a manual `isLoading`/`showLoadingSkeleton`/`error` state triplet and a 150 ms skeleton-delay timer. → **Fix:** Extract a shared `useAsyncResource(key, fetcher)` hook in widget-core (returns `{ data, isLoading, error }`, handles cancellation and a small in-memory cache so revisiting a space does not refetch); keep the 150 ms skeleton delay inside that hook as an option. This same hook fixes HelpCollectionView, HelpArticleView and HelpView. Severity: High.

### `packages/widget-core/src/components/ConversationView.tsx`
- **L171** — category: Resetting/syncing local state from props — `setTranscriptEmailInput(transcriptEmail || '')` whenever the `transcriptEmail` prop changes, overwriting whatever the user typed. → **Fix:** Use the "store previous prop, adjust during render" pattern (`if (prevTranscriptEmail !== transcriptEmail) { setPrev(...); setTranscriptEmailInput(transcriptEmail || '') }`), or key the transcript form/email-capture block on `transcriptEmail`. Severity: Medium.
- **L175** — category: Resetting/syncing local state from props — Resets `autoExpandDismissed` and `autoExpandedConversationRef` whenever `conversationKey` changes. → **Fix:** Store the dismissal keyed by conversation (`const [dismissedFor, setDismissedFor] = useState<string|null>(null)`; `const autoExpandDismissed = dismissedFor === conversationKey`) so no reset effect is needed; alternatively render `<ConversationView key={conversationKey}>` from ChatWindow, which would also reset `preChatDone`/`pendingAttachments` per conversation (arguably desirable, verify). Severity: Medium.
- **L217** — category: Derived state (unstable dependency) — rAF DOM measurement for wide tables is legitimate, but `displayMessages` is rebuilt on every render (`[introMessage, ...messages]` without `useMemo`) and `introMessage` is a fresh object each render, so this effect re-runs and re-queries the DOM on every render of the component, not only when messages change. → **Fix:** `useMemo` `introMessage`/`displayMessages` on `[messages, hasTeamReply, welcomeMessage, introRole, introName, introAvatar, introCreatedAt]`, or depend on `messages` directly. Severity: Low.
- L180 is legitimate (document `mousedown` listener to close the header menu, with cleanup).

### `packages/widget-core/src/components/EmojiPicker.tsx`
- **L82** — category: Data fetching in useEffect — Lazily loads the emoji catalog (`loadEmojiCatalog()`) when `open` flips true, with manual `isLoading`/`loadError` state and a retry that works by clearing `loadError` so the effect re-fires. → **Fix:** This is a code-split load, not server data, so kick it off from the trigger's `onClick` (and from `handleRetry`) via a module-level memoised promise (`let catalogPromise: Promise<EmojiCatalog> | null`), setting state in the handler's `.then/.catch`; or use the shared `useAsyncResource` hook with `enabled: open`. Severity: Medium.

### `packages/widget-core/src/components/MessageList.tsx`
- No issues (1 effect, all legitimate: scroll-to-bottom / near-bottom auto-scroll after `messages` render).

### `packages/widget-core/src/components/ComposeBar.tsx`
- No issues (1 effect, all legitimate: textarea auto-height measurement after `message` renders; `field-sizing: content` CSS could replace it in the future, but that is not an effect anti-pattern).

### `packages/widget-core/src/components/ChatWindow.tsx`
- **L122** — category: Resetting/syncing local state from props — `setActiveView(initialView)` whenever the `initialView` prop changes, so `activeView` is half-controlled. → **Fix:** Either make the view fully controlled by the SDK (it already reports via `onViewChange`; accept `view` + `onViewChange` and drop local `activeView`), or use the previous-prop-during-render pattern (`if (prevInitialView !== initialView) { setPrevInitialView(initialView); setActiveView(initialView) }`). Severity: Medium.
- **L126** — category: Resetting/syncing local state from props — `setHumanSupportRequested(false)` whenever `activeConversation?.id` changes. → **Fix:** Store `humanSupportRequestedFor: string | undefined` and derive `humanSupportRequested = humanSupportRequestedFor === activeConversation?.id`; set it to the current conversation id in the `onEscalateToHuman` handler. Severity: Medium.
- **L130** — category: Event-specific logic in an effect — Treats `openArticleRequest` (a `{ key, articleKey }` counter object) as an event and navigates to `help-article`, calling `onViewChange`. Because `onViewChange` is in the deps, any parent re-render that passes a new `onViewChange` identity while `openArticleRequest` is set re-runs the effect and forces the view back to the article (the user presses Back, parent re-renders, they are bounced to the article again). → **Fix:** Expose an imperative API instead (SDK calls `openArticle(key)` via a ref/`useImperativeHandle`, or lifts `activeView`/`activeArticleKey` into the SDK and passes them as controlled props). If the request-object pattern must stay, drop `onViewChange` from deps (store it in a ref) and gate on `openArticleRequest.key` only. Severity: High.
- **L143** — category: Chained effects — Open/close presence logic: `isOpen` → `setShouldRender(true)` → effect re-runs because `shouldRender` is also a dependency → schedules rAF `setIsVisible(true)`; on close it sets `isVisible=false` and a 220 ms timer clears `shouldRender`. The `shouldRender` dependency makes the effect run twice per open and re-arm on its own state change. → **Fix:** Depend only on `[isOpen]` and use functional updates (`setShouldRender(prev => ...)`), or extract a `usePresence(isOpen, 220)` hook that returns `{ shouldRender, isVisible }`; alternatively drive the exit with `onTransitionEnd` instead of a hard-coded timeout. Severity: Medium.

### `packages/widget-core/src/components/ImageLightbox.tsx`
- No issues (1 effect, all legitimate: document `keydown` Escape listener with cleanup).

### `packages/widget-core/src/components/HelpView.tsx`
- **L38** — category: useEffect to compute a debounced value — 250 ms `setTimeout` effect mirroring `normalizedSearchQuery` into `debouncedSearchQuery`. → **Fix:** Add a `useDebouncedValue(value, delayMs)` hook to widget-core (copy `help-center/src/hooks/useDebouncedValue.ts`) and use `const debounced = useDebouncedValue(normalizedSearchQuery, 250)`. Severity: Low.
- **L46** — category: Data fetching in useEffect — Calls `fetchHelpSearchResults(...)` in an effect with `cancelled` flag and manual `searchResults`/`isSearching`/`searchError` triplet; also clears results by calling three setters inside the effect when the query is too short. → **Fix:** Use the shared `useAsyncResource` hook keyed on `[host, widgetKey, debouncedQuery]` with `enabled: showSearchResults && debouncedQuery.length >= 2`; derive the "empty" state from `enabled` rather than resetting state in an effect. Severity: High.

### `packages/widget-core/src/components/HelpArticleView.tsx`
- **L30** — category: Data fetching in useEffect — Fetches `fetchHelpArticle(host, widgetKey, articleKey)` with `cancelled` flag and manual `article`/`isLoading`/`error` triplet. → **Fix:** Replace with the shared `useAsyncResource(['article', host, widgetKey, articleKey], () => fetchHelpArticle(...))`; the component is already `key`ed on `articleKey` by ChatWindow, so the reset-on-change logic is redundant once the hook exists. Severity: High.

### `packages/widget-core/src/components/StreamingText.tsx`
- **L19** — category: Chained effects / Notifying parent of state changes — The typewriter effect depends on its own `displayedText` output, so each `setDisplayedText` re-runs the effect to schedule the next character (a state-driven chain, one effect run per character). When `!isStreaming` it synchronously sets state to `text` (derived state) and calls `onComplete?.()` from inside the effect (notify-parent). → **Fix:** Drive the animation from a single effect keyed on `[text, isStreaming, charDelayMs]` that owns an interval/rAF loop and a `useRef` index, calling `onComplete` from that loop when it finishes; when `!isStreaming`, render `text` directly instead of copying it into state. Severity: Medium.

### `packages/widget-core/src/components/HelpCollectionView.tsx`
- **L55** — category: Data fetching in useEffect — `Promise.all([fetchHelpArticles, fetchHelpCollections])` in an effect with `cancelled` flag and manual `articles`/`collections`/`isLoading`/`error` state. → **Fix:** Two calls to the shared `useAsyncResource` hook (articles keyed on `collectionSlug`, collections keyed on `spaceSlug`) — the collections list is the same request HelpSpaceView already made, so a small hook-level cache removes the duplicate network round-trip when drilling down. Severity: High.

### `packages/widget-core/src/components/SpecialNoticeBanner.tsx`
- **L20** — category: Initializing state on mount / syncing state from props — Reads `localStorage` for the dismissal key and copies it into `dismissed` state whenever `trimmed`/`workspaceId` change; the initial `useState(true)` means the banner always renders as hidden for one commit and then pops in. → **Fix:** Compute the stored value during render: `const key = dismissalKey(workspaceId, trimmed); const [dismissedKeys, setDismissedKeys] = useState<Set<string>>(new Set()); const dismissed = !trimmed || dismissedKeys.has(key) || readStorage(key)` (with `readStorage` memoised on `key` via `useMemo`), and add the key to the set in `handleDismiss`. No effect needed and no first-paint flash. Severity: Medium.

### `help-center/src/router.tsx`
- No issues (1 effect, all legitimate: `window.location.reload()` on a stale-chunk error, an external-system side effect from an error component with no event handler to move it into).

### `help-center/src/routes/__root.tsx`
- **L84** — category: Derived DOM (declarative alternative exists) — Imperatively creates/updates `<link rel="icon">` in `document.head` from `config.favicon_url` after hydration, so the favicon is absent in the SSR HTML and flashes in on the client. → **Fix:** Emit it declaratively from the route's `head()` (`links: [{ rel: 'icon', href: loaderData.config.favicon_url }, ...]`) — `loaderData` is already available in `head`, so `HeadContent` renders it on the server and keeps it in sync; delete the effect. Severity: Low.

### `help-center/src/components/ArticleContent.tsx`
- No issues (1 effect, all legitimate: post-processes `dangerouslySetInnerHTML` output — assigns heading ids and attaches copy buttons — which must run after the HTML is in the DOM).

### `help-center/src/components/navigation/MobileNav.tsx`
- No issues (1 effect, all legitimate: locks `document.body.style.overflow` while mounted, restored on cleanup).

### `help-center/src/components/layout/AppShell.tsx`
- No issues (3 effects, all legitimate: L14 injects/removes the third-party widget `<script>` keyed on config; L48 global Cmd/Ctrl+K keyboard shortcut; L60 `open-help-search` window custom-event subscription — all with cleanup).

### `help-center/src/components/search/SearchDialog.tsx`
- **L37** — category: useEffect used to reset form state when dialog opens — Clears `query` on both open and close and focuses the input via `requestAnimationFrame` when `open` becomes true. → **Fix:** Move `query`/`inputRef`/search hooks into an inner `SearchDialogBody` rendered inside `DialogContent`; Radix unmounts content when closed, so state resets by remounting, and the input can use `autoFocus` (or `DialogContent onOpenAutoFocus`) instead of the rAF. Severity: Medium.

### `help-center/src/hooks/useScrollSpy.ts`
- No issues (1 effect, all legitimate: window `scroll` subscription with cleanup plus an initial measurement; note callers should pass a stable/memoised `ids` array or the listener re-subscribes each render).

### `help-center/src/hooks/useDebouncedValue.ts`
- No issues (1 effect, all legitimate: this is the project's debounce hook itself — a timer subscription with cleanup; `useDeferredValue` is not a substitute when a real wall-clock delay before a network call is wanted).

### `website/src/components/Navbar.tsx`
- No issues (1 effect, all legitimate: window `scroll` listener driving the `scrolled` class, with cleanup; could be `useSyncExternalStore` but the current form is fine for a Next.js client component).

### `website/src/app/page.tsx`
- **L1548** — category: Chained effects — When `isVisible` becomes true it schedules `setStep(0)` after 600 ms, which then kicks the L1556 step-timer effect; it reads `step` without listing it in deps (stale-closure read of `step` at the moment visibility changed, no `eslint-disable` comment, so lint will flag it). Behaviour is intentional but the two effects form a chain. → **Fix:** Fold the "resume from off-screen" case into the L1556 state machine (track `wasHiddenRef` set in the IntersectionObserver callback; in the step effect use `delay = wasHiddenRef.current ? 600 : d[step]` and set `step` accordingly), or put the reset directly in the IntersectionObserver callback (`if (e.isIntersecting && stepRef.current !== 12) setStep(0)`). Severity: Low.
- L85 (`useReveal` IntersectionObserver), L415 (window scroll → progress), L431 (double-rAF measurement + ResizeObserver), L1302 (rAF animation loop along an SVG path with cleanup), L1537 (IntersectionObserver pause/resume) and L1556 (timer-driven step machine with cleanup) are legitimate external-system/animation subscriptions.

### `help-center/src/hooks/useDocumentTitle.ts`
- No issues (1 effect, all legitimate: syncs `document.title` and restores the brand name on cleanup).

### `help-center/src/hooks/useTheme.ts`
- **L26** — category: Resetting/syncing local state from props — When `configThemeMode` is forced (`light`/`dark`) the effect copies it into `theme` state, so for one commit the hook returns the stored theme before snapping to the forced one. → **Fix:** Derive during render: keep only the user preference in state (`const [userTheme, setUserTheme] = useState(getStoredTheme)`) and compute `const theme = isForced ? (configThemeMode as Theme) : userTheme`; the L32 `applyTheme(theme)` effect then follows the derived value with no intermediate state. Severity: Medium.
- L32 is legitimate (syncs the `dark` class on `document.documentElement` — an external DOM system — and must also run on hydration).

TOTAL: 26 files reviewed, 42 effects reviewed, 5 flagged High, 10 flagged Medium, 4 flagged Low
