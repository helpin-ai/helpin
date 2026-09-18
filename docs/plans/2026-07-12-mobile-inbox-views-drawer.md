# Mobile inbox views drawer implementation plan

This historical plan explains the mobile inbox drawer and shared web filtering approach for contributors. The drawer is implemented; use the source review below to distinguish current behavior from the original task checklist. The old worktree, commit instructions, and device checks are historical, not current execution instructions.

## Source review — 2026-09-18

- [The filter bridge](../../apps/support-mobile/src/inbox/use-inbox-filters.ts) reuses the web request builder for builtin, mailbox, and custom views. It now also accepts editable filter overrides. Matching request parameters does not prove byte-identical responses across different users, times, permissions, or caches.
- [The drawer model](../../apps/support-mobile/src/inbox/view-list-config.ts) shows **total workload counts plus unread dots**, rather than numeric unread badges. Inbox, Mine, Waiting, and AI Handling use unread-stat totals; Resolved, Spam, and AI Resolved omit badges. Custom views use view counts; team inboxes use scope totals with an unread fallback.
- [The store](../../apps/support-mobile/src/stores/support-view-store.ts) persists both selection and filter overrides in `sessionStorage`, under one storage key. Changing selection clears overrides. Persistence follows browser session-storage behavior; it is not a promise about every native app restart.
- [The inbox screen](../../apps/support-mobile/src/screens/inbox-screen.tsx) mounts the drawer, an edge-swipe opening strip, and a filter sheet. [The drawer](../../apps/support-mobile/src/inbox/views-drawer.tsx) also includes workspace switching and settings callbacks, beyond the original proposal.
- [Realtime handling](../../apps/support-mobile/src/lib/use-mobile-realtime.ts) invalidates view counts. Narrow web-file aliases remain in the mobile build configuration. Source inspection does not establish live web/mobile list parity, bundle size, device accessibility, or passing runtime tests; the original verification checklist still requires those checks when changing behavior.

## Original record


**Goal:** Replace the mobile inbox's Mine/Unassigned/All segmented control with a left slide-out drawer that mirrors the web support sidebar (Inbox/Mine/Waiting/Resolved/Spam · AI Handling/AI Resolved · Custom views · Team inboxes) with per-view unread counts, where each view returns byte-identical conversations to web.

**Architecture:** Parity is achieved by reusing the web's *existing* pure filter function `buildConversationListRequestFilters` in place (via narrow read-only build aliases), so there is one source of truth and the web is never edited. `support-core`'s conversation-list serialization and a few list/count service methods are widened additively. The mobile UI adds a drawer, a small view-state store, and a rulebook→filters bridge.

**Tech Stack:** React 19, Vite 7, TypeScript, TanStack Query, Zustand, `motion`, vitest — all already in `apps/support-mobile`.

**Spec:** `docs/specs/2026-07-12-mobile-inbox-views-drawer-design.md`

## Global Constraints

- Work in the worktree `/root/teampulse-mobile` on branch `feat/support-mobile`. Never edit anything under `frontend/` (constraint C2). Never edit `apps/support-desktop`.
- Parity mechanism: reuse the web rulebook via **explicit per-path aliases only**. Do NOT add a blanket `@ → frontend/src` alias.
- The per-view server query MUST be produced by the reused web function `buildConversationListRequestFilters` — do not re-derive per-view params by hand.
- `support-core` changes must be additive (web + desktop + mobile all consume it): after any `support-core` edit, run typecheck in `packages/support-core`, `apps/support-mobile`, `apps/support-desktop`, and `frontend` (the last via `cd frontend && npx tsc -b --noEmit`, NODE_OPTIONS=--max-old-space-size=4096 if it OOMs).
- Every interactive element ≥44×44pt; light+dark; `prefers-reduced-motion`; selection haptic on taps.
- Commit after every task. End commit messages with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- On-device verification is a handoff item (add to `apps/support-mobile/README.md` device checklist) — no emulator in the build environment.

## Key reused web APIs (verbatim signatures — confirmed in the codebase)

From `frontend/src/lib/supportInboxFilters.ts`:
- `type NavFilter` (re-exported via the store) = `'inbox' | 'mine' | 'waiting' | 'resolved' | 'spam' | 'ai_active' | 'resolved_by_ai'`
- `type ConversationListRequestFilters = { status?: string; filter?: string; mailbox_id?: string; mailbox_ids?: string; flow_state?: string; search?: string; assigned_to?: string; sort?: string; statuses?: string; tag_ids?: string; ai?: string }`
- `type ConversationListFilters` (the rich per-view state)
- `defaultConversationListFiltersForNav(navFilter: NavFilter): ConversationListFilters`
- `buildConversationListRequestFilters(args: { navFilter: NavFilter; selectedMailboxId: string; searchQuery: string; listFilters?: ConversationListFilters }): ConversationListRequestFilters | undefined`
- `parseSupportInboxViewFilters(filters, navFilter)` (for custom views)

From `frontend/src/lib/supportInboxRouting.ts`:
- `navFilterFromView(view?: string): NavFilter`, `viewFromNavFilter(filter: NavFilter): string`

Per-view params emitted by `buildConversationListRequestFilters` with default `listFilters` (verified):
`inbox → {mailbox_id:'shared', filter:'inbox'}` · `mine → {filter:'mine'}` · `waiting → {status:'waiting_on_customer'}` · `resolved → {filter:'resolved'}` · `spam → {status:'spam'}` · `ai_active → {ai:'<default ai states>'}` · `resolved_by_ai → {ai:'<default ai states>'}`. (Exact `ai` string comes from `defaultAIStatesForNav`; the test asserts equality to the function's own output, so it is parity by construction.)

Data model (confirmed in `server/internal/model/support_inbox_view.go`):
- `SupportInboxViewCount = { view_id: string; total_count: number; unread_count: number }` — `GET /api/support/inbox/views/counts`
- `SupportInboxView` (builtin + custom) carries at least `id`, `view_key` (builtin: `'inbox'|'mine'|...`), `name`, `filters` — `GET /api/support/inbox/views/builtin` (builtin) and `GET /api/support/inbox/views` (custom). Confirm exact JSON field names against `server/internal/model/support_inbox_view.go` when defining the mobile type.
- Team-inbox unread counts: `SupportInboxScopeListResponse` (`shared_inbox.unread_count`, `mailboxes[].unread_count`) — already in `support-core` (`listInboxScopes` / `useInboxScopes`).

---

### Task 1: Read-in-place aliases + rulebook param-parity test

**Files:**
- Modify: `apps/support-mobile/vite.config.ts`
- Modify: `apps/support-mobile/tsconfig.app.json`
- Create: `apps/support-mobile/src/inbox/__tests__/web-rulebook-parity.test.ts`

**Interfaces:**
- Produces: the ability to `import { buildConversationListRequestFilters, defaultConversationListFiltersForNav, type NavFilter, type ConversationListRequestFilters } from '@/lib/supportInboxFilters'` and `import { navFilterFromView, viewFromNavFilter } from '@/lib/supportInboxRouting'` from mobile code.

- [ ] **Step 1: Add the explicit per-path aliases to `vite.config.ts`** (inside the existing `resolve.alias` array — add these entries; keep everything else). Use absolute paths to the web files:

```ts
      // Read-in-place reuse of the web support "rulebook" (pure logic only).
      // Explicit per-path aliases — deliberately NOT a blanket `@ → frontend/src`,
      // so web UI can never be accidentally imported into the mobile bundle.
      { find: '@/lib/supportInboxFilters', replacement: path.resolve(__dirname, '../../frontend/src/lib/supportInboxFilters.ts') },
      { find: '@/lib/supportInboxRouting', replacement: path.resolve(__dirname, '../../frontend/src/lib/supportInboxRouting.ts') },
      { find: '@/lib/pmTypes', replacement: path.resolve(__dirname, '../../frontend/src/lib/pmTypes.ts') },
      { find: '@/stores/supportInboxStore', replacement: path.resolve(__dirname, '../../frontend/src/stores/supportInboxStore.ts') },
      { find: '@/components/support/helpers', replacement: path.resolve(__dirname, '../../frontend/src/components/support/helpers.ts') },
```

- [ ] **Step 2: Mirror the same five aliases in `tsconfig.app.json`** under `compilerOptions.paths` (so `tsc` resolves them). Each maps the module specifier to the web file, e.g.:

```jsonc
      "@/lib/supportInboxFilters": ["../../frontend/src/lib/supportInboxFilters.ts"],
      "@/lib/supportInboxRouting": ["../../frontend/src/lib/supportInboxRouting.ts"],
      "@/lib/pmTypes": ["../../frontend/src/lib/pmTypes.ts"],
      "@/stores/supportInboxStore": ["../../frontend/src/stores/supportInboxStore.ts"],
      "@/components/support/helpers": ["../../frontend/src/components/support/helpers.ts"]
```

Keep the existing `@mobile/*` path. Ensure `tsconfig.app.json`'s `include` / `rootDir` settings don't reject files outside `src` — if `tsc` complains the web files are outside `rootDir`, remove an explicit `rootDir` constraint or add the web `lib`/`stores`/`components` dirs to `include` minimally; document what you changed in the report.

- [ ] **Step 3: Write the parity test** (this both proves the aliases resolve AND locks the per-view params). This test is intentionally tautological against the reused function — its value is that it (a) fails to compile/run if the alias breaks, and (b) documents the exact params, so a future web change surfaces here.

```ts
import { describe, expect, test } from 'vitest'
import {
  buildConversationListRequestFilters,
  defaultConversationListFiltersForNav,
  type NavFilter,
} from '@/lib/supportInboxFilters'

const BUILTIN: NavFilter[] = ['inbox', 'mine', 'waiting', 'resolved', 'spam', 'ai_active', 'resolved_by_ai']

describe('web rulebook is importable and deterministic per view', () => {
  test.each(BUILTIN)('view %s produces stable request filters', (navFilter) => {
    const params = buildConversationListRequestFilters({
      navFilter,
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav(navFilter),
    })
    expect(params).toMatchSnapshot()
  })

  test('inbox default targets the shared mailbox', () => {
    const params = buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('inbox'),
    })
    expect(params).toMatchObject({ mailbox_id: 'shared', filter: 'inbox' })
  })

  test('waiting maps to waiting_on_customer status', () => {
    const params = buildConversationListRequestFilters({
      navFilter: 'waiting',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('waiting'),
    })
    expect(params).toMatchObject({ status: 'waiting_on_customer' })
  })
})
```

- [ ] **Step 4: Run — expect FAIL first if aliases are wrong, then PASS.** `cd apps/support-mobile && pnpm test web-rulebook-parity` (writes snapshots on first pass) and `pnpm typecheck`. The `matchObject` assertions must pass with the exact values above; if `inbox` isn't `{mailbox_id:'shared',filter:'inbox'}` or `waiting` isn't `{status:'waiting_on_customer'}`, the alias is resolving a stale/wrong file — investigate before committing.
- [ ] **Step 5: Confirm no web UI leaked into the bundle.** `pnpm build` then check the built entry chunk did not balloon (compare gzip size to the ~210KB baseline from Task 22; a jump of >100KB means a heavy web import sneaked in — investigate). Record the size in the report.
- [ ] **Step 6: Commit.** `git add apps/support-mobile/vite.config.ts apps/support-mobile/tsconfig.app.json apps/support-mobile/src/inbox/__tests__/web-rulebook-parity.test.ts && git commit -m "feat(mobile): read web support filter rulebook in place via narrow aliases"`

---

### Task 2: Widen support-core conversation-list serialization for full parity params

**Files:**
- Modify: `packages/support-core/src/support-service.ts`
- Modify: `packages/support-core/src/__tests__/support-mutations.test.ts` (or a new `support-filters.test.ts`)

**Interfaces:**
- Consumes: nothing new.
- Produces: `ConversationFilters` widened to a superset of the web's `ConversationListRequestFilters`; `supportService.listConversations(workspaceId, filters)` serializes every present field with the **web/server param names**.

- [ ] **Step 1: Write the failing test** (fake api asserts the exact query string):

```ts
import { describe, expect, test, vi, beforeEach } from 'vitest'
import { configureSupportApi } from '../support-service'
import { supportService } from '../support-service'

function fakeApi() {
  const get = vi.fn().mockResolvedValue({ data: { data: [] }, error: null })
  configureSupportApi({ get, post: vi.fn(), put: vi.fn() } as never)
  return get
}

describe('listConversations serializes full parity params', () => {
  beforeEach(() => { vi.clearAllMocks() })
  test('forwards statuses, mailbox_ids, ai, assigned_to, sort, tag_ids, search with web param names', async () => {
    const get = fakeApi()
    await supportService.listConversations('ws1', {
      status: 'spam', statuses: 'open,waiting_on_customer', filter: 'inbox',
      mailbox_id: 'shared', mailbox_ids: 'm1,m2', ai: 'ai_active',
      flow_state: 'x', search: 'hello', assigned_to: 'me', sort: 'oldest', tag_ids: 't1',
    })
    const path = get.mock.calls[0][0] as string
    for (const frag of ['status=spam','statuses=open%2Cwaiting_on_customer','filter=inbox','mailbox_id=shared','mailbox_ids=m1%2Cm2','ai=ai_active','flow_state=x','search=hello','assigned_to=me','sort=oldest','tag_ids=t1']) {
      expect(path).toContain(frag)
    }
  })
})
```

- [ ] **Step 2: Run — FAIL.** `cd packages/support-core && pnpm test` (missing fields not serialized).
- [ ] **Step 3: Widen the `ConversationFilters` type and serialization.** In `support-service.ts`, extend `ConversationFilters` to include all optional string fields: `status, statuses, priority, filter, mailbox_id, mailbox_ids, ai, ai_state, flow_state, search, assigned_to, sort, tag_ids, system_tags`. Replace the hand-written `if` chain in `listConversations` with a generic serializer that appends every present, non-empty field with its own key name (skip `mailbox_id` only when it equals `'all'`, preserving existing behavior; `'shared'` must pass through):

```ts
const CONVERSATION_FILTER_KEYS = [
  'status','statuses','priority','filter','mailbox_id','mailbox_ids',
  'ai','ai_state','flow_state','search','assigned_to','sort','tag_ids','system_tags',
] as const

listConversations: (workspaceId: string, filters?: ConversationFilters) => {
  let path = `/support/inbox/conversations${qs(workspaceId)}`
  for (const key of CONVERSATION_FILTER_KEYS) {
    const value = filters?.[key]
    if (value == null || value === '') continue
    if (key === 'mailbox_id' && value === 'all') continue
    path += `&${key}=${encodeURIComponent(String(value))}`
  }
  return getApi().get<ConversationListResponse>(path)
},
```

- [ ] **Step 4: Run — PASS.** `pnpm test` in support-core.
- [ ] **Step 5: Four-way typecheck.** support-core, apps/support-mobile, apps/support-desktop, frontend (per Global Constraints). All clean.
- [ ] **Step 6: Commit.** `git commit -m "feat(support-core): serialize full support conversation filter param set"`

---

### Task 3: support-core view-counts, builtin-views, custom-views service + hooks

**Files:**
- Modify: `packages/support-core/src/support-service.ts`, `support-types.ts`, `support-query-keys.ts`, `use-support-core.ts`
- Test: `packages/support-core/src/__tests__/support-views.test.ts`

**Interfaces:**
- Produces:
  - `type SupportInboxViewCount = { view_id: string; total_count: number; unread_count: number }`
  - `type SupportInboxView = { id: string; view_key?: string; name: string; is_builtin?: boolean; filters?: unknown }` (confirm/expand field names against `server/internal/model/support_inbox_view.go`)
  - `supportService.listInboxViewCounts(workspaceId): ApiResponse<SupportInboxViewCount[]>` → `GET /support/inbox/views/counts`
  - `supportService.listBuiltinInboxViews(workspaceId): ApiResponse<SupportInboxView[]>` → `GET /support/inbox/views/builtin`
  - `supportService.listInboxViews(workspaceId): ApiResponse<SupportInboxView[]>` → `GET /support/inbox/views`
  - hooks `useSupportInboxViewCounts(workspaceId, enabled?)`, `useSupportBuiltinInboxViews(workspaceId, enabled?)`, `useSupportInboxViews(workspaceId, enabled?)` (query keys added to `supportQueryKeys`)

- [ ] **Step 1: Failing test** — fake api asserts each service method hits the right path and the hooks are exported:

```ts
import { describe, expect, test, vi } from 'vitest'
import { configureSupportApi, supportService } from '../support-service'
test('view endpoints hit the right paths', async () => {
  const get = vi.fn().mockResolvedValue({ data: [], error: null })
  configureSupportApi({ get, post: vi.fn(), put: vi.fn() } as never)
  await supportService.listInboxViewCounts('ws1')
  await supportService.listBuiltinInboxViews('ws1')
  await supportService.listInboxViews('ws1')
  const paths = get.mock.calls.map((c) => c[0])
  expect(paths[0]).toContain('/support/inbox/views/counts?workspace_id=ws1')
  expect(paths[1]).toContain('/support/inbox/views/builtin?workspace_id=ws1')
  expect(paths[2]).toContain('/support/inbox/views?workspace_id=ws1')
})
```

- [ ] **Step 2: Run — FAIL.** Methods undefined.
- [ ] **Step 3: Implement** the three service methods (mirroring existing `qs(workspaceId)` pattern), the two types in `support-types.ts` (confirm `SupportInboxView` fields against the backend model), the query keys, and the three hooks in `use-support-core.ts` (follow the existing `useSupportMailboxes` pattern: `staleTime: 15_000`, `enabled: !!workspaceId && enabled`). Counts hook uses `staleTime: 15_000`; builtin/custom views `staleTime: 30_000` (matches web).
- [ ] **Step 4: Run — PASS** + four-way typecheck.
- [ ] **Step 5: Commit.** `git commit -m "feat(support-core): inbox view-counts, builtin-views, custom-views service + hooks"`

---

### Task 4: Mobile view-state store + rulebook→filters bridge

**Files:**
- Create: `apps/support-mobile/src/stores/support-view-store.ts`
- Create: `apps/support-mobile/src/inbox/use-inbox-filters.ts`
- Test: `apps/support-mobile/src/inbox/__tests__/use-inbox-filters.test.ts`

**Interfaces:**
- Consumes: `buildConversationListRequestFilters`, `defaultConversationListFiltersForNav`, `parseSupportInboxViewFilters`, `NavFilter` (Task 1 aliases); support-core `ConversationFilters` (Task 2).
- Produces:
  - `useSupportViewStore` — zustand: `{ selection: ViewSelection; setSelection(s: ViewSelection): void }` where `ViewSelection = { kind: 'builtin'; navFilter: NavFilter; mailboxId: string } | { kind: 'custom'; viewId: string; filters: unknown } | { kind: 'mailbox'; mailboxId: string }`. Default `{ kind: 'builtin', navFilter: 'inbox', mailboxId: 'all' }`. Persist to `sessionStorage` (survives navigation, not restart).
  - `selectionToConversationFilters(selection: ViewSelection): ConversationFilters | undefined` — pure; for builtin, calls `buildConversationListRequestFilters({ navFilter, selectedMailboxId: mailboxId, searchQuery: '', listFilters: defaultConversationListFiltersForNav(navFilter) })`; for mailbox, `{ navFilter:'inbox', selectedMailboxId: mailboxId }` path; for custom, parse via `parseSupportInboxViewFilters`.

- [ ] **Step 1: Failing test** for `selectionToConversationFilters` — assert each builtin view yields the same object as calling the web function directly (parity guard), and mailbox selection sets `mailbox_id`:

```ts
import { describe, expect, test } from 'vitest'
import { buildConversationListRequestFilters, defaultConversationListFiltersForNav } from '@/lib/supportInboxFilters'
import { selectionToConversationFilters } from '../use-inbox-filters'

test.each(['inbox','mine','waiting','resolved','spam','ai_active','resolved_by_ai'] as const)(
  'builtin %s matches the web function output', (navFilter) => {
    const expected = buildConversationListRequestFilters({
      navFilter, selectedMailboxId: 'all', searchQuery: '',
      listFilters: defaultConversationListFiltersForNav(navFilter),
    })
    expect(selectionToConversationFilters({ kind: 'builtin', navFilter, mailboxId: 'all' })).toEqual(expected)
  })

test('mailbox selection scopes to that mailbox', () => {
  const f = selectionToConversationFilters({ kind: 'mailbox', mailboxId: 'mbx-1' })
  expect(f).toMatchObject({ mailbox_id: 'mbx-1' })
})
```

- [ ] **Step 2: Run — FAIL.** → **Step 3: Implement** the store + bridge. → **Step 4: PASS** + typecheck.
- [ ] **Step 5: Commit.** `git commit -m "feat(mobile): support view-state store and rulebook→filters bridge"`

---

### Task 5: View-list model + per-view count resolution

**Files:**
- Create: `apps/support-mobile/src/inbox/view-list-config.ts`
- Test: `apps/support-mobile/src/inbox/__tests__/view-list-config.test.ts`

**Interfaces:**
- Consumes: support-core hooks from Task 3 (`useSupportBuiltinInboxViews`, `useSupportInboxViewCounts`, `useSupportInboxViews`, `useSupportMailboxes`, `useInboxScopes`).
- Produces:
  - `BUILTIN_VIEW_ORDER: { navFilter: NavFilter; label: string; group: 'views' | 'ai' }[]` = Inbox/Mine/Waiting/Resolved/Spam (group `views`) then AI Handling (`ai_active`) / AI Resolved (`resolved_by_ai`) (group `ai`).
  - `resolveViewCount(navFilter, builtinViews, counts): number` — maps `navFilter → builtin view.id (by view_key)`, then `counts` entry `unread_count` for that `view_id`; `0` if absent.
  - `resolveCustomViewCount(viewId, counts): number` and `resolveMailboxUnread(scope): number` (from inbox scopes).
  - `buildDrawerGroups({...}): DrawerGroup[]` assembling the four groups for rendering (pure; takes the fetched arrays, returns `{ title, items: { key, label, count, selection }[] }[]`).

- [ ] **Step 1: Failing test** — builtin order/labels; count lookup joins `view_key → id → view_id`:

```ts
import { describe, expect, test } from 'vitest'
import { BUILTIN_VIEW_ORDER, resolveViewCount } from '../view-list-config'

test('builtin order and grouping match web', () => {
  expect(BUILTIN_VIEW_ORDER.map((v) => v.navFilter)).toEqual(
    ['inbox','mine','waiting','resolved','spam','ai_active','resolved_by_ai'])
  expect(BUILTIN_VIEW_ORDER.filter((v) => v.group === 'ai').map((v) => v.navFilter))
    .toEqual(['ai_active','resolved_by_ai'])
})

test('count joins view_key -> id -> view_id', () => {
  const builtin = [{ id: 'v-inbox', view_key: 'inbox', name: 'Inbox' }]
  const counts = [{ view_id: 'v-inbox', total_count: 10, unread_count: 3 }]
  expect(resolveViewCount('inbox', builtin as never, counts as never)).toBe(3)
  expect(resolveViewCount('mine', builtin as never, counts as never)).toBe(0)
})
```

- [ ] **Step 2: FAIL → Step 3: Implement → Step 4: PASS** + typecheck. (When implementing, confirm the builtin `view_key` string values by calling the running dev API at `http://91.98.85.12:8080/api/support/inbox/views/builtin` with a valid token, or by reading `server/internal/service/support_inbox_view_service.go` around the builtin seeding — the labels map `inbox→"Inbox", mine→"Mine", waiting→"Waiting", resolved→"Resolved", spam→"Spam", ai_active→"AI Handling", resolved_by_ai→"AI Resolved"`.)
- [ ] **Step 5: Commit.** `git commit -m "feat(mobile): inbox view-list model and per-view count resolution"`

---

### Task 6: ViewsDrawer component

**Files:**
- Create: `apps/support-mobile/src/inbox/views-drawer.tsx`
- Test: `apps/support-mobile/src/inbox/__tests__/views-drawer.test.tsx`

**Interfaces:**
- Consumes: Task 5 `buildDrawerGroups`; Task 4 store; `Pressable`, `Badge`, `Avatar`, `formatBadgeCount`, `haptic`, `stackSpring`.
- Produces: `<ViewsDrawer open, onOpenChange, workspaceId />` — left-anchored overlay (scrim + panel, ~82% width max 360px, `pt-[var(--safe-top)]`/`pb-[var(--safe-bottom)]`), grouped scrollable list with section headers (Views / AI / Custom views / Team inboxes), each row a `Pressable` (≥44pt, `haptic="selection"`) with label + trailing `unread` badge via `formatBadgeCount` (hidden at 0), active row highlighted (`bg-muted`/`text-primary`). Selecting a row calls the store's `setSelection` and `onOpenChange(false)`. Spring in from `x:-100%→0`; crossfade + no-slide under `prefers-reduced-motion`; scrim tap + left-swipe dismiss.

- [ ] **Step 1: Failing test** (render with a stub group model; assert groups/labels render, count badge shows, tapping a row fires selection + close). Mock the support-core hooks so the drawer receives deterministic data.
- [ ] **Step 2: FAIL → Step 3: Implement → Step 4: PASS** + typecheck + `pnpm build`.
- [ ] **Step 5: Commit.** `git commit -m "feat(mobile): views drawer overlay with grouped list and unread counts"`

---

### Task 7: Wire the drawer into the inbox screen (remove segments + mailbox sheet)

**Files:**
- Modify: `apps/support-mobile/src/screens/inbox-screen.tsx`
- Modify: `apps/support-mobile/src/lib/use-mobile-realtime.ts` (invalidate view-counts on events)
- Modify: `apps/support-mobile/README.md` (device checklist entries)
- Delete: `apps/support-mobile/src/inbox/mailbox-sheet.tsx` (absorbed) — or leave unused and note it.

**Interfaces:**
- Consumes: `ViewsDrawer`, `useSupportViewStore`, `selectionToConversationFilters`, view-list config, current-selection display name.

- [ ] **Step 1:** Replace the `SegmentedControl` (Mine/Unassigned/All) and the trailing mailbox button/`MailboxSheet` with: a `TopBar` whose title is the current selection's display name and whose **leading** slot is a menu (hamburger) `Pressable` that opens the drawer; mount `<ViewsDrawer>`; add a left-edge swipe on the inbox scroll container to open it (reuse the edge-swipe primitive pattern, opening the drawer instead of navigating back — inbox is a tab root, no back).
- [ ] **Step 2:** Source the conversations query filters from `selectionToConversationFilters(selection)` (Task 4) instead of the old segment→filter mapping. Keep the existing skeleton/empty/error/virtualized list, pull-to-refresh, and swipe actions unchanged.
- [ ] **Step 3:** In `use-mobile-realtime.ts`, add the view-counts query key to the set invalidated on support realtime events (alongside conversations + unread-stats).
- [ ] **Step 4:** Update tests that referenced the removed segmented control; add/adjust an inbox test asserting the menu button opens the drawer and selecting a view updates the query filters (mock hooks). Add the on-device checks to `README.md` (drawer open via button + edge-swipe; each view lists same conversations as web — spot check; counts match; team inbox + custom view selection; reduced motion).
- [ ] **Step 5: Verify** `pnpm test`, `pnpm typecheck`, `pnpm build` (record entry bundle gzip size — must stay well under 900KB). Four-way typecheck since Tasks 2–3 touched support-core.
- [ ] **Step 6: Commit.** `git commit -m "feat(mobile): mount views drawer in inbox, retire segmented control + mailbox sheet"`

---

## Self-review notes

- **Spec coverage:** drawer + grouping (Tasks 6–7); parity via reused rulebook (Tasks 1, 4); per-view counts (Tasks 3, 5); web untouched + single source (Task 1 aliases, no `frontend/` edits anywhere); out-of-scope items (custom-view authoring, sub-filters) not implemented, as specified.
- **Parity crux** is Task 4's test asserting `selectionToConversationFilters(builtin) === buildConversationListRequestFilters(...)` for every view — the guard that mobile and web send identical queries.
- **Open confirmations folded into tasks (not placeholders):** exact `SupportInboxView` JSON field names (Task 3, against the backend model) and builtin `view_key` strings/labels (Task 5, against the backend seeding or the live API) — each names the exact file/endpoint to read.
- **Cross-app safety:** every support-core task ends in a four-way typecheck.
