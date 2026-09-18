# Support Inbox Sidebar Views Implementation Plan

> Source review, 2026-09-17

Historical sidebar design, predating the September personal-read and shared
attention model. Consult the
[inbox-state design and implementation record](../specs/2026-09-02-first-class-support-inbox-state-design.md)
and [current repository](../../server/internal/repository/support_inbox.go).
The V2 path uses per-user unread state and materialized counters; rollout state
determines whether a workspace uses it. Do not infer shared unread semantics or
current production cutover from the older views described below.

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current overlapping support sidebar/status filter model with simple built-in saved views: Inbox, Mine, Waiting, Resolved, Spam, plus the existing Helpin AI views.

**Architecture:** Treat sidebar entries as first-class views that determine the primary conversation query. Keep team inboxes as mailbox scopes that can be applied to the active view. Move "Mine" and "Inbox" logic into backend-supported filters so assignment, mentions, ownership, AI handoff, access control, pagination, and counts all stay consistent.

**Tech Stack:** Go 1.24, Chi, GORM, PostgreSQL/SQLite tests, React 19, TanStack Router/Query, Zustand, TypeScript, Vitest.

---

## Product Decisions

The default support sidebar order is:

1. Inbox
2. Mine
3. Waiting
4. Resolved
5. Spam
6. Helpin AI
   - AI Handling
   - AI Resolved
7. Team Inboxes

View semantics:

- `Inbox`: active human work. Includes open human-owned, unassigned, assigned-to-human, after-hours queue, waiting-for-human, AI escalated, and customer-requested-human conversations. Excludes normal AI handling, AI resolved, waiting-on-customer, resolved, and spam.
- `Mine`: conversations personally relevant to the current user. Includes assigned to me, opened/taken over by me, and conversations where I was mentioned. Default lifecycle scope is open plus waiting-on-customer. Excludes spam.
- `Waiting`: conversations with `status = waiting_on_customer`.
- `Resolved`: conversations with `status = resolved`, excluding AI-resolved conversations from the main view so the existing Helpin AI / AI Resolved view remains meaningful.
- `Spam`: conversations with `status = spam`.
- `AI Handling`: active AI-owned conversations.
- `AI Resolved`: conversations resolved by AI.

Team inbox behavior:

- Clicking a team inbox applies a mailbox scope, not a separate ownership model.
- Default team inbox click should show that mailbox's `Inbox` view.
- URL should support mailbox-scoped views, for example `/support?view=inbox&inbox=<mailbox_id>`.
- The list header should make the scope clear, for example `Billing / Inbox`.

List toolbar behavior:

- Remove the global status dropdown from the conversation list. The main states now live in the sidebar.
- Keep search.
- Do not add secondary filters in v1 unless needed after using the simplified model.

Custom sidebar views:

- Do not build custom saved views in this first pass. The built-in views must be correct first.
- Leave the naming and route model compatible with custom views later: a custom view can become a saved `view definition` that reuses the same backend query predicates.

Product decision: `Mine` is an actionable work view, not a personal archive. Resolved conversations owned by me live under `Resolved`; AI-resolved conversations live under `AI Resolved`.

## Current Code Review

Current frontend state:

- `frontend/src/stores/supportInboxStore.ts`
  - `NavFilter` is currently `my_inbox | all | unassigned | mentions | ai_active | resolved_by_ai`.
  - Initial state is `navFilter: 'all'`.
  - `setNavFilter` resets the mailbox scope to `all`.
  - `setSelectedMailboxId` resets `navFilter` to `all`.
- `frontend/src/lib/supportInboxRouting.ts`
  - URL `view=assigned` maps to `my_inbox`.
  - URL `view=mentions` maps to `mentions`.
  - No `inbox`, `waiting`, `resolved`, or `spam` saved-view mapping exists.
- `frontend/src/components/layout/sidebar/config.ts`
  - Sidebar items are currently My Inbox, Unassigned, Mentions, All.
  - Statuses live in `supportStatusOptions`.
- `frontend/src/components/support/ConversationList.tsx`
  - The API query is driven by `statusFilter`, `navFilter`, `flow_state`, and `filter=mentions`.
  - The status dropdown can override the sidebar, which is the core UX confusion.
  - Mentions is the only special backend filter.
- `frontend/src/lib/supportInboxFilters.ts`
  - Client-side filtering keeps AI active and AI resolved out of human views.
  - It cannot correctly implement `Mine = assignment OR opened OR mentions` because mentions are not present on normal conversation rows.

Current backend state:

- `server/internal/handler/support_inbox.go`
  - `GET /api/support/tickets?filter=mentions` uses a separate code path.
  - Normal list supports `status`, `priority`, `mailbox_id`, `flow_state`, `search`, and `ai_state`.
- `server/internal/service/support_inbox.go`
  - `ListConversationsWithMeta` delegates normal filters to the repository and returns unread stats.
  - `ListConversationsWithMentions` is not paginated and does not combine with assignment ownership.
- `server/internal/repository/support_inbox.go`
  - `conversationAIActiveCondition`, `conversationResolvedByAICondition`, and `conversationHumanQueueCondition` already exist.
  - `List` has duplicate count/fetch filter setup.
  - `GetUnreadStats` returns `total`, `my_inbox`, `unassigned`, and `ai_active`.
- `server/internal/model/support_inbox.go`
  - `UnreadStats` does not include the new view counts.

Current uncommitted worktree note:

- There is an existing uncommitted change in `frontend/src/components/support/ReplyComposer.tsx` from the shortcut Tab navigation fix.
- There are untracked `.superpowers/` and `docs/mockups/` paths.
- Keep this sidebar work in separate commits from those changes.

## File Structure

Backend files:

- Modify `server/internal/model/support_inbox.go`
  - Extend `UnreadStats` with new JSON fields.
  - Optionally add constants for conversation list filters.
- Modify `server/internal/handler/support_inbox.go`
  - Accept `filter=inbox` and `filter=mine`.
  - Keep `filter=mentions` as a legacy alias to `mine` or a compatibility path.
- Modify `server/internal/service/support_inbox.go`
  - Add a list method that passes the new filter into the repository.
  - Retire or reduce the special mentions list path after `mine` is supported by the normal paginated query.
- Modify `server/internal/repository/support_inbox.go`
  - Add reusable predicates for `inbox`, `mine`, `resolved human`, and mention membership.
  - Apply the predicates to both count and fetch queries.
  - Extend unread/view counts.
- Modify `server/internal/service/support_inbox_test.go`
  - Add or update backend tests for counts and reopening behavior.
- Modify or add `server/internal/repository/support_inbox_view_test.go`
  - Test repository predicates for new views.

Frontend files:

- Modify `frontend/src/stores/supportInboxStore.ts`
  - Replace the old `NavFilter` values with saved-view values.
  - Keep legacy URL compatibility in routing, not in store state.
- Modify `frontend/src/lib/supportInboxRouting.ts`
  - Map new view keys to URL search params.
  - Preserve legacy aliases.
- Modify `frontend/src/components/layout/sidebar/config.ts`
  - Replace sidebar items with Inbox, Mine, Waiting, Resolved, Spam.
  - Keep AI Handling and AI Resolved under Helpin AI.
- Modify `frontend/src/components/layout/sidebar/SupportRailNav.tsx`
  - Render new items and badges.
  - Replace the inline unread stats prop shape with the shared `UnreadStats` type, or update both shapes in the same task.
  - Keep team inboxes as mailbox scopes.
- Modify `frontend/src/components/layout/Sidebar.tsx`
  - Update default navigation behavior and URL building.
- Modify `frontend/src/components/support/SupportInboxLayout.tsx`
  - Sync URL/store for new views.
  - Make no-query `/support` default to Inbox.
- Modify `frontend/src/components/support/ConversationList.tsx`
  - Derive API filters from the active view.
  - Remove the status dropdown.
  - Show a minimal current-view label and search.
- Modify `frontend/src/lib/supportInboxFilters.ts`
  - Keep only client-side safety filtering, search fallback, and mailbox scope filtering.
  - Do not rely on client filtering for primary view correctness.
- Modify `frontend/src/components/support/helpers.ts`
  - Add pure helpers for `isHumanInboxConversation`, `isMineConversation`, and `isHumanResolvedConversation`.
- Modify `frontend/src/lib/pm-types/support.ts`
  - Update `UnreadStats` TypeScript shape.
- Modify `frontend/src/hooks/queries/useSupport.ts`
  - Update the fallback `ConversationListResponse.meta.unread` object to include the new stats keys.
- Update tests:
  - `frontend/src/lib/__tests__/supportInboxFilters.test.ts`
  - `frontend/src/stores/__tests__/supportInboxStore.test.ts`
  - Add `frontend/src/lib/__tests__/supportInboxRouting.test.ts` if missing.
  - Update `frontend/src/components/support/__tests__/ConversationList.test.tsx`.

## Task 1: Define The New View Contract

**Files:**
- Modify: `frontend/src/stores/supportInboxStore.ts`
- Modify: `frontend/src/lib/supportInboxRouting.ts`
- Test: `frontend/src/lib/__tests__/supportInboxRouting.test.ts`
- Test: `frontend/src/stores/__tests__/supportInboxStore.test.ts`

- [ ] **Step 1: Add routing tests**

Cover these mappings:

```ts
expect(navFilterFromView(undefined)).toBe('inbox');
expect(navFilterFromView('inbox')).toBe('inbox');
expect(navFilterFromView('mine')).toBe('mine');
expect(navFilterFromView('waiting')).toBe('waiting');
expect(navFilterFromView('resolved')).toBe('resolved');
expect(navFilterFromView('spam')).toBe('spam');
expect(navFilterFromView('ai-handling')).toBe('ai_active');
expect(navFilterFromView('ai-resolved')).toBe('resolved_by_ai');

// Legacy aliases.
expect(navFilterFromView('all')).toBe('inbox');
expect(navFilterFromView('assigned')).toBe('mine');
expect(navFilterFromView('my_inbox')).toBe('mine');
expect(navFilterFromView('mentions')).toBe('mine');
expect(navFilterFromView('unassigned')).toBe('inbox');
```

- [ ] **Step 2: Run the frontend routing/store tests and verify they fail**

Run:

```bash
cd frontend
npm test -- supportInboxRouting supportInboxStore
```

Expected: routing tests fail because the new view values do not exist yet.

- [ ] **Step 3: Update `NavFilter`**

Change:

```ts
export type NavFilter = 'my_inbox' | 'all' | 'unassigned' | 'mentions' | 'ai_active' | 'resolved_by_ai';
```

To:

```ts
export type NavFilter = 'inbox' | 'mine' | 'waiting' | 'resolved' | 'spam' | 'ai_active' | 'resolved_by_ai';
```

Set initial state:

```ts
navFilter: 'inbox',
statusFilter: 'all',
```

Keep `statusFilter` temporarily for compatibility during this refactor, but it should no longer be the main list state once Task 5 lands.

- [ ] **Step 4: Update store actions**

`setNavFilter(filter)` should:

```ts
set({
  navFilter: filter,
  statusFilter: 'all',
  selectedMailboxId: 'all',
  selectedConversationId: null,
  activePanel: 'list',
});
```

`setSelectedMailboxId(mailboxId)` should:

```ts
set({
  navFilter: 'inbox',
  selectedMailboxId: mailboxId,
  statusFilter: 'all',
  selectedConversationId: null,
  activePanel: 'list',
});
```

- [ ] **Step 5: Update route mapping**

Use these canonical URLs:

```ts
const VIEW_BY_FILTER: Record<NavFilter, string> = {
  inbox: 'inbox',
  mine: 'mine',
  waiting: 'waiting',
  resolved: 'resolved',
  spam: 'spam',
  ai_active: 'ai-handling',
  resolved_by_ai: 'ai-resolved',
};
```

`buildSupportInboxSearch` may omit `view=inbox` for the default global inbox, but should include it when `inbox=<mailbox_id>` is present so mailbox-scoped URLs are explicit.

- [ ] **Step 6: Run tests**

Run:

```bash
cd frontend
npm test -- supportInboxRouting supportInboxStore
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/stores/supportInboxStore.ts frontend/src/lib/supportInboxRouting.ts frontend/src/lib/__tests__/supportInboxRouting.test.ts frontend/src/stores/__tests__/supportInboxStore.test.ts
git commit -m "refactor: define support inbox saved views"
```

## Task 2: Add Backend View Filters

**Files:**
- Modify: `server/internal/model/support_inbox.go`
- Modify: `server/internal/handler/support_inbox.go`
- Modify: `server/internal/service/support_inbox.go`
- Modify: `server/internal/repository/support_inbox.go`
- Test: `server/internal/repository/support_inbox_view_test.go`

- [ ] **Step 1: Write repository tests for view predicates**

Create test data covering:

- open human queue conversation
- open AI handling conversation
- open AI escalated conversation
- waiting-on-customer conversation
- human resolved conversation
- AI resolved conversation
- spam conversation
- assigned-to-current-user conversation
- opened-by-current-user conversation
- conversation with a message metadata mention of current user

Expected results:

- `filter=inbox` returns open human queue and AI escalated, not waiting/resolved/spam/AI handling/AI resolved.
- `filter=mine` returns assigned/opened/mentioned conversations, not unrelated conversations or spam.
- `status=waiting_on_customer` returns waiting conversations.
- `status=resolved` plus the human-resolved predicate excludes AI resolved.
- `status=spam` returns spam.

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd server
go test ./internal/repository -run 'TestSupportConversationRepository.*View|TestSupportInboxViews' -count=1
```

Expected: FAIL because filters do not exist.

- [ ] **Step 3: Add filter constants**

In `server/internal/model/support_inbox.go`, add:

```go
const (
	SupportConversationListFilterInbox    = "inbox"
	SupportConversationListFilterMine     = "mine"
	SupportConversationListFilterMentions = "mentions"
)
```

- [ ] **Step 4: Add repository predicates**

In `server/internal/repository/support_inbox.go`, add:

```go
func conversationHumanInboxCondition(alias string) string {
	return fmt.Sprintf("(%s.status = '%s' AND (%s OR %s.ai_state = 'escalated' OR %s.customer_requested_human_at IS NOT NULL))",
		alias,
		model.SupportConversationStatusOpen,
		conversationHumanQueueCondition(alias),
		alias,
		alias,
	)
}
```

Reason: `conversationHumanQueueCondition` handles the normal human queue and most escalations. The explicit `ai_state = 'escalated'` and `customer_requested_human_at IS NOT NULL` checks protect the product promise that AI handoffs and "talk to a human" requests land in Inbox even if a transition briefly leaves older AI fields behind.

Add a helper for human-resolved:

```go
func conversationHumanResolvedCondition(alias string) string {
	return fmt.Sprintf("(%s.status = '%s' AND NOT (%s))",
		alias,
		model.SupportConversationStatusResolved,
		conversationResolvedByAICondition(alias),
	)
}
```

Add mention `EXISTS` predicate. For PostgreSQL:

```sql
EXISTS (
  SELECT 1
  FROM support_messages sm_mention
  WHERE sm_mention.conversation_id = support_conversations.id
    AND sm_mention.workspace_id = support_conversations.workspace_id
    AND sm_mention.deleted_at IS NULL
    AND sm_mention.metadata::jsonb @> ?::jsonb
)
```

For SQLite tests, use a `LIKE` fallback against metadata JSON text. Keep the fallback inside a helper so production still uses JSONB containment.

Use the existing GORM dialect pattern already present in this repository:

```go
if r.db.Dialector.Name() == "sqlite" {
	// SQLite test fallback.
}
```

- [ ] **Step 5: Thread `filter` through the normal list path**

Do not add another positional parameter to the already-long list methods. Introduce small params structs instead:

```go
type ConversationListParams struct {
	WorkspaceID string
	UserID      string
	Status      string
	Priority    string
	Pagination  model.PMPagination
	MailboxID   *string
	FlowState   string
	Search      string
	Filter      string
	AIState     []string
}

type ConversationRepositoryListParams struct {
	ConversationListParams
	WorkspaceMemberID string
	Role              string
}
```

Use the first shape at the handler/service boundary. The service should still resolve `WorkspaceMemberID` and `Role`, then pass the repository shape downward.

The handler should read:

```go
filter := strings.TrimSpace(r.URL.Query().Get("filter"))
```

Do not branch early for mentions anymore. Instead, pass `filter` into the normal paginated path.

Temporary compatibility:

- `filter=mentions` should be treated as an exact alias for `mine` during rollout.
- Once frontend no longer sends mentions, keep the alias because old URLs may exist.

- [ ] **Step 6: Apply filters in count and fetch queries**

Use a single helper:

```go
func (r *SupportConversationRepository) applyConversationListFilter(query *gorm.DB, alias, filter, userID string) *gorm.DB
```

Rules:

- `inbox`: apply `conversationHumanInboxCondition(alias)`.
- `mine`: apply assigned/opened/mentioned predicate and `status IN ('open', 'waiting_on_customer')`.
- `mentions`: exact legacy alias for `mine`; apply the same assigned/opened/mentioned predicate and `status IN ('open', 'waiting_on_customer')`.
- empty: no special filter.

Pass `userID` into repository list so `mine` can be server-side and paginated.

- [ ] **Step 7: Handle Resolved view cleanly**

For frontend `view=resolved`, prefer sending `status=resolved&filter=human_resolved` or `filter=resolved`.

Keep the backend surface simple by adding one filter value:

```go
SupportConversationListFilterResolved = "resolved"
```

Backend `filter=resolved` should apply `conversationHumanResolvedCondition(alias)`.

Add a short comment near the frontend filter builder or backend filter constants explaining the mixed query style:

```ts
// Sidebar views use backend filters when placement depends on ownership/AI state.
// Simple lifecycle views use status directly.
```

- [ ] **Step 8: Run repository tests**

Run:

```bash
cd server
go test ./internal/repository -run 'SupportConversation.*View|SupportInboxViews' -count=1
```

Expected: PASS.

- [ ] **Step 9: Run support service tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestSupportInbox|TestSupportFlowState' -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add server/internal/model/support_inbox.go server/internal/handler/support_inbox.go server/internal/service/support_inbox.go server/internal/repository/support_inbox.go server/internal/repository/support_inbox_view_test.go server/internal/service/support_inbox_test.go
git commit -m "feat: add support inbox view filters"
```

## Task 3: Update Sidebar Navigation

**Files:**
- Modify: `frontend/src/components/layout/sidebar/config.ts`
- Modify: `frontend/src/components/layout/sidebar/SupportRailNav.tsx`
- Modify: `frontend/src/components/layout/Sidebar.tsx`
- Modify: `frontend/src/lib/pm-types/support.ts`
- Modify: `frontend/src/hooks/queries/useSupport.ts`
- Test: existing sidebar/component tests if present

- [ ] **Step 1: Replace default support items**

In `config.ts`, replace current `supportFilterItems` with:

```ts
export const supportFilterItems = [
  { key: 'inbox' as const, label: 'Inbox', icon: InboxIcon },
  { key: 'mine' as const, label: 'Mine', icon: UserIcon },
  { key: 'waiting' as const, label: 'Waiting', icon: PauseIcon },
  { key: 'resolved' as const, label: 'Resolved', icon: CheckmarkCircle02Icon },
  { key: 'spam' as const, label: 'Spam', icon: OctagonXIcon },
];
```

Keep:

```ts
export const supportAiItems = [
  { key: 'ai_active' as const, label: 'AI Handling', icon: BotIcon },
  { key: 'resolved_by_ai' as const, label: 'AI Resolved', icon: CheckmarkCircle02Icon },
];
```

- [ ] **Step 2: Update `SupportNavFilter` type**

In `SupportRailNav.tsx`, update the union to:

```ts
type SupportNavFilter =
  | 'inbox'
  | 'mine'
  | 'waiting'
  | 'resolved'
  | 'spam'
  | 'ai_active'
  | 'resolved_by_ai';
```

- [ ] **Step 3: Update frontend unread stats type and badge mapping**

In `frontend/src/lib/pm-types/support.ts`, update the shared type first:

```ts
export interface UnreadStats {
  inbox: number;
  mine: number;
  waiting: number;
  ai_active: number;
  total?: number;
  my_inbox?: number;
  unassigned?: number;
}
```

In `frontend/src/hooks/queries/useSupport.ts`, update the fallback meta object:

```ts
meta: {
  unread: {
    inbox: 0,
    mine: 0,
    waiting: 0,
    ai_active: 0,
    total: 0,
    my_inbox: 0,
    unassigned: 0,
  },
},
```

In `SupportRailNav.tsx`, remove the duplicated inline unread stats shape and import the shared type:

```ts
import type { UnreadStats } from '@/lib/pmTypes';
```

Then use:

```ts
unreadStats?: UnreadStats | null;
```

Use these fields once Task 4 extends stats:

```ts
const badge =
  item.key === 'inbox' ? unreadStats?.inbox
  : item.key === 'mine' ? unreadStats?.mine
  : item.key === 'waiting' ? unreadStats?.waiting
  : undefined;
```

Do not show badges for Resolved or Spam by default. They are archive/review states, not action counters.

- [ ] **Step 4: Update sidebar click behavior**

In `Sidebar.tsx`:

- `onNavFilterChange('inbox')` should navigate to `/support` or `/support?view=inbox`.
- `onMailboxSelect(mailboxId)` should set selected mailbox and `view=inbox`.
- Do not reset team inbox selection when the user switches between Inbox/Mine/Waiting if preserving scope feels better after implementation. For v1, keep the current simpler behavior: clicking a top-level view returns to all inboxes; clicking a team inbox shows that team's Inbox.

- [ ] **Step 5: Run TypeScript checks**

Run:

```bash
cd frontend
npx tsc -b --noEmit
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/layout/sidebar/config.ts frontend/src/components/layout/sidebar/SupportRailNav.tsx frontend/src/components/layout/Sidebar.tsx frontend/src/lib/pm-types/support.ts frontend/src/hooks/queries/useSupport.ts
git commit -m "feat: simplify support sidebar views"
```

## Task 4: Extend Counts For New Views

**Files:**
- Modify: `server/internal/model/support_inbox.go`
- Modify: `server/internal/repository/support_inbox.go`
- Test: `server/internal/service/support_inbox_test.go`

- [ ] **Step 1: Extend backend stats model**

Change `UnreadStats` to:

```go
type UnreadStats struct {
	Inbox    int `json:"inbox"`
	Mine     int `json:"mine"`
	Waiting  int `json:"waiting"`
	AIActive int `json:"ai_active"`

	// Legacy fields during rollout.
	Total      int `json:"total"`
	MyInbox    int `json:"my_inbox"`
	Unassigned int `json:"unassigned"`
}
```

Keep legacy fields temporarily so older frontend code does not break during deploy order.

- [ ] **Step 2: Define count semantics**

Counts are actionable unread counts, not total archive counts:

- `inbox`: unread customer replies in the human inbox view.
- `mine`: unread customer replies in open or waiting conversations assigned/opened/mentioned for the user.
- `waiting`: unread customer replies in waiting conversations. This matters because a customer reply should bring it back to attention.
- `ai_active`: unread customer replies while AI is handling.

Resolved and Spam do not need badges in v1.

- [ ] **Step 3: Update SQL counts**

Reuse the existing unread subquery and conditions:

- `inbox` uses `conversationHumanInboxCondition("sc")`.
- `mine` uses assigned/opened/mentioned predicate plus `sc.status IN ('open', 'waiting_on_customer')`.
- `waiting` uses `sc.status = 'waiting_on_customer'`.
- `ai_active` keeps existing condition.

Set legacy aliases:

```go
stats.Total = stats.Inbox
stats.MyInbox = stats.Mine
```

Keep `Unassigned` only if an old component still reads it.

- [ ] **Step 4: Run tests**

Run:

```bash
cd server
go test ./internal/service -run 'GetUnreadStats' -count=1
```

Then:

```bash
cd frontend
npx tsc -b --noEmit
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/support_inbox.go server/internal/repository/support_inbox.go server/internal/service/support_inbox_test.go
git commit -m "feat: add support inbox view counts"
```

## Task 5: Make Conversation List View-Driven

**Files:**
- Modify: `frontend/src/components/support/ConversationList.tsx`
- Modify: `frontend/src/lib/supportInboxFilters.ts`
- Modify: `frontend/src/components/support/helpers.ts`
- Test: `frontend/src/lib/__tests__/supportInboxFilters.test.ts`
- Test: `frontend/src/components/support/__tests__/ConversationList.test.tsx`

- [ ] **Step 1: Update filter tests**

Expected client-side fallback behavior:

- `inbox` excludes AI handling, AI resolved, waiting, resolved, and spam.
- `mine` includes assigned or opened conversations when `userId` is present.
- `waiting` returns only `status=waiting_on_customer`.
- `resolved` returns human-resolved only.
- `spam` returns only spam.
- `ai_active` and `resolved_by_ai` keep current behavior.

- [ ] **Step 2: Add pure helper predicates**

In `helpers.ts`, add:

```ts
export function isHumanInboxConversation(conversation: SupportConversation): boolean {
  return conversation.status === 'open' && isHumanQueueConversation(conversation);
}

export function isHumanResolvedConversation(conversation: SupportConversation): boolean {
  return conversation.status === 'resolved' && !isResolvedByAIConversation(conversation);
}

export function isUserOwnedConversation(conversation: SupportConversation, userId?: string): boolean {
  if (!userId) return false;
  return conversation.assigned_user_id === userId || conversation.opened_by_user_id === userId;
}
```

Do not try to detect mentions client-side in v1 unless the API row includes mention metadata. Backend `filter=mine` owns that.

- [ ] **Step 3: Derive API filters from view**

In `ConversationList.tsx`, replace status/nav filter construction with view-driven logic:

```ts
const filters = useMemo(() => {
  const f: Record<string, string> = {};
  if (selectedMailboxId !== 'all') f.mailbox_id = selectedMailboxId;
  if (searchQuery.trim()) f.search = searchQuery.trim();

  switch (navFilter) {
    case 'inbox':
      f.filter = 'inbox';
      break;
    case 'mine':
      f.filter = 'mine';
      break;
    case 'waiting':
      f.status = 'waiting_on_customer';
      break;
    case 'resolved':
      f.filter = 'resolved';
      break;
    case 'spam':
      f.status = 'spam';
      break;
    case 'ai_active':
      f.flow_state = 'ai_handling';
      break;
    case 'resolved_by_ai':
      f.flow_state = 'resolved_by_ai';
      break;
  }

  return Object.keys(f).length > 0 ? f : undefined;
}, [navFilter, searchQuery, selectedMailboxId]);
```

- [ ] **Step 4: Remove the status dropdown UI**

Remove imports and JSX for:

- `DropdownMenu`
- `DropdownMenuTrigger`
- `DropdownMenuContent`
- `DropdownMenuCheckboxItem`
- `supportStatusOptions`
- `statusFilter`
- `setStatusFilter`

Keep the toolbar compact:

- left side: current view/scope label
- right side: search icon
- expanded state: search input

Suggested label helper:

```ts
function titleForView(navFilter: NavFilter): string {
  switch (navFilter) {
    case 'inbox': return 'Inbox';
    case 'mine': return 'Mine';
    case 'waiting': return 'Waiting';
    case 'resolved': return 'Resolved';
    case 'spam': return 'Spam';
    case 'ai_active': return 'AI Handling';
    case 'resolved_by_ai': return 'AI Resolved';
  }
}
```

- [ ] **Step 5: Update empty states**

Use concise copy:

- Inbox: `Inbox is clear` / `New conversations needing a teammate will appear here.`
- Mine: `Nothing for you` / `Assigned conversations and mentions will appear here.`
- Waiting: `No waiting conversations` / `Conversations waiting on a customer will appear here.`
- Resolved: `No resolved conversations` / `Resolved conversations will appear here.`
- Spam: `No spam` / `Spam conversations will appear here.`
- AI Handling: current copy is fine.
- AI Resolved: current copy is fine.

- [ ] **Step 6: Update onboarding empty state**

The onboarding empty state should trigger only when:

```ts
navFilter === 'inbox'
selectedMailboxId === 'all'
!searchQuery.trim()
```

Do not depend on `statusFilter`.

- [ ] **Step 7: Run frontend tests**

Run:

```bash
cd frontend
npm test -- supportInboxFilters ConversationList
```

Expected: PASS.

- [ ] **Step 8: Run TypeScript**

Run:

```bash
cd frontend
npx tsc -b --noEmit
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/components/support/ConversationList.tsx frontend/src/lib/supportInboxFilters.ts frontend/src/components/support/helpers.ts frontend/src/lib/__tests__/supportInboxFilters.test.ts frontend/src/components/support/__tests__/ConversationList.test.tsx
git commit -m "refactor: drive support list from sidebar views"
```

## Task 6: Sync URL, Store, And Team Inbox Scope

**Files:**
- Modify: `frontend/src/components/support/SupportInboxLayout.tsx`
- Modify: `frontend/src/components/layout/Sidebar.tsx`
- Modify: `frontend/src/lib/supportInboxRouting.ts`
- Test: `frontend/src/lib/__tests__/supportInboxRouting.test.ts`

- [ ] **Step 1: Update default URL behavior**

No query means:

```ts
view = 'inbox'
inbox = 'all'
```

Canonical examples:

- `/w/acme/support` means global Inbox.
- `/w/acme/support?view=mine` means global Mine.
- `/w/acme/support?view=waiting` means global Waiting.
- `/w/acme/support?view=inbox&inbox=<mailbox_id>` means team inbox scope plus Inbox view.

- [ ] **Step 2: Remove default status logic from route sync**

Delete the old logic:

```ts
const defaultStatusFilter = nextNavFilter === 'my_inbox' || nextNavFilter === 'unassigned' || nextNavFilter === 'mentions'
  ? 'open'
  : 'all';
```

Set:

```ts
const nextStatusFilter = routeSearch.status || 'all';
```

The store can keep this field for backward compatibility, but new UI should not use it for primary view selection.

- [ ] **Step 3: Preserve legacy URLs**

Legacy URL handling:

- `view=assigned`, `view=my`, `view=my_inbox`, `view=mentions` -> Mine.
- `view=all`, `view=unassigned`, missing view -> Inbox.
- `status=waiting_on_customer` with no view -> Waiting if this old URL is common.
- `status=resolved` with no view -> Resolved.
- `status=spam` with no view -> Spam.

When replacing the URL, use canonical view names.

- [ ] **Step 4: Run route tests**

Run:

```bash
cd frontend
npm test -- supportInboxRouting
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/support/SupportInboxLayout.tsx frontend/src/components/layout/Sidebar.tsx frontend/src/lib/supportInboxRouting.ts frontend/src/lib/__tests__/supportInboxRouting.test.ts
git commit -m "fix: canonicalize support inbox view routing"
```

## Task 7: Manual UX Verification

**Files:**
- No source changes unless issues are found.

- [ ] **Step 1: Start backend**

Run:

```bash
cd server
go run ./cmd/api
```

Expected: API starts without migration or route errors.

- [ ] **Step 2: Start frontend**

Run:

```bash
cd frontend
npm run dev
```

Expected: Vite dev server starts.

- [ ] **Step 3: Verify global sidebar behavior**

In the browser:

- `/support` opens Inbox.
- Clicking Mine shows owned/mentioned conversations.
- Clicking Waiting shows waiting-on-customer conversations.
- Clicking Resolved shows resolved human conversations.
- Clicking Spam shows spam conversations.
- AI Handling and AI Resolved still work under Helpin AI.
- No global status dropdown appears in the list toolbar.

- [ ] **Step 4: Verify team inbox behavior**

In the browser:

- Clicking Billing opens Billing / Inbox.
- Billing / Inbox excludes global/shared-only unrelated conversations.
- Search works inside Billing / Inbox.
- Switching back to Inbox returns to all inboxes.

- [ ] **Step 5: Verify customer reply behavior**

Use an existing waiting conversation or create one:

- Set conversation to Waiting.
- Send a customer reply through widget/email path.
- Expected: backend reopens it and it appears in Inbox.

The backend currently has reopening logic in `CreateConversationMessage`; verify the new filters surface it correctly.

- [ ] **Step 6: Run full checks**

Run:

```bash
cd frontend
npx tsc -b --noEmit
```

Run:

```bash
cd server
go test ./internal/repository ./internal/service -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit fixes if manual verification required changes**

```bash
git add <changed-files>
git commit -m "fix: polish support inbox view behavior"
```

## Task 8: Optional Follow-Up For Custom Sidebar Views

Do not implement this in the first pass unless explicitly requested after the built-in view refactor ships.

Future minimal design:

- Create `support_saved_views` table:
  - `id`
  - `workspace_id`
  - `created_by_id`
  - `name`
  - `icon`
  - `filter_json`
  - `position`
  - `pinned`
  - `created_at`
  - `updated_at`
- The frontend renders built-in views first, then pinned custom views.
- A custom view stores the same filter shape the backend already supports:

```json
{
  "mailbox_id": "billing",
  "status": "open",
  "assigned_user_id": "me",
  "priority": "urgent"
}
```

Important: do this only after built-in views are stable. Otherwise the custom view builder will encode confusion into saved records.

## Verification Checklist

- [ ] `cd frontend && npm test -- supportInboxRouting supportInboxStore supportInboxFilters ConversationList`
- [ ] `cd frontend && npx tsc -b --noEmit`
- [ ] `cd server && go test ./internal/repository ./internal/service -count=1`
- [ ] Manual browser check for Inbox, Mine, Waiting, Resolved, Spam, AI Handling, AI Resolved.
- [ ] Manual browser check for one team inbox scope.
- [ ] Confirm old URLs do not break: `view=assigned`, `view=mentions`, `view=all`, `view=ai-handling`, `view=ai-resolved`.

## Risks And Guardrails

- Mine cannot be correct with frontend-only filtering because mentions are not included on normal conversation rows. Implement backend `filter=mine`.
- Do not keep the status dropdown in the list after moving states to the sidebar; it recreates the same confusion.
- Do not show Resolved and Spam badges by default; those counts are not urgent work.
- Do not remove legacy `total`, `my_inbox`, or `unassigned` stat fields until frontend and production deploy order is confirmed.
- Keep AI Handling and AI Resolved separate for now, per product decision.
- Keep this work separate from the existing uncommitted shortcut Tab navigation fix.
