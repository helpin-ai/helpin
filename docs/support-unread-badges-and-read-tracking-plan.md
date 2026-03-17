# Support Unread Badges and Read Tracking Plan

**Status:** Proposed
**Date:** 2026-03-16
**Scope:** Support inbox, widget unread badges, read tracking, realtime sync
**Modules:** `server`, `frontend`, `packages/sdk-js`, `packages/widget-core`

---

## 1. Summary

This plan adds persistent unread tracking for both:

- internal Helpin users working in the support inbox
- external customers using the widget

The design uses conversation-level read cursors stored directly on `support_conversations`, following the same general pattern used by Chatwoot:

- `agent_last_seen_at`
- `assignee_last_seen_at`
- `contact_last_seen_at`

Unread counts are computed on read, not denormalized into a separate unread table.

Realtime behavior is provided by WebSocket fanout, but the database remains the durable source of truth. WebSocket is used for immediate sync, not as the storage layer.

---

## 2. Goals

- Persist unread state across refreshes and reconnects.
- Show unread badges in the internal support inbox conversation list.
- Show unread totals on the Support rail and inbox filter navigation.
- Show unread badges in the widget launcher and widget conversation list.
- Sync unread state across multiple browser tabs and active sessions.
- Keep the design support-specific and compatible with the current codebase.

---

## 3. Non-Goals

- Building a generic notification framework for all modules.
- Replacing all support HTTP reads with WebSocket reads.
- Solving support presence/typing snapshot issues in the same change set.
- Building a full per-agent read history table.

---

## 4. Current State

### 4.1 Widget

The widget unread badge is currently client-only:

- `WidgetManager.unreadCount` lives only in memory.
- It increments on `message:received` when the widget is closed.
- It resets to `0` when the widget opens.
- It is lost on refresh.

This means widget unread is not durable and cannot be server-correct.

### 4.2 Internal Support Inbox

The internal inbox currently has:

- no persistent unread state
- no unread badge in the support list
- no unread total on the support rail
- no read cursor persisted for agents

### 4.3 Realtime

Support already uses WebSocket for:

- message delivery
- typing indicators
- viewing presence

Unread must integrate into the same event pipeline, but should not depend on WebSocket as the only source of truth.

---

## 5. Recommended Design

### 5.1 Durable Source of Truth

Unread state is determined from timestamps stored on `support_conversations`.

Add these fields:

- `agent_last_seen_at`
- `assignee_last_seen_at`
- `contact_last_seen_at`

These timestamps represent the latest confirmed read state for:

- any internal agent viewing the conversation
- the currently assigned agent
- the widget contact/customer

### 5.2 Realtime Transport

Unread changes should be propagated in real time over WebSocket:

- new message arrives -> other sessions invalidate or patch unread state
- conversation marked read -> other sessions invalidate or patch unread state

### 5.3 Payload Shape

Unread should be carried on each conversation object as `unread_count`.

Do not introduce a separate widget-only `unread_counts` map. The current widget and frontend data models already consume conversation arrays, and `widget-core` already has `Conversation.unreadCount`.

### 5.4 Agent vs Widget Model

For v1:

- internal inbox unread is driven by `agent_last_seen_at`
- widget/customer unread is driven by `contact_last_seen_at`

`assignee_last_seen_at` should still be stored and maintained for future assignee-specific unread behavior, but the initial UI does not need to expose separate assignee unread counters unless explicitly required.

---

## 6. Unread Semantics

### 6.1 Internal Agent Unread

Recommended v1 definition:

`unread_count = count of non-internal messages created after agent_last_seen_at`

Notes:

- Exclude internal notes from unread in the main inbox.
- Include customer-visible messages only.
- This makes the queue reflect customer work, not internal commentary.

### 6.2 Contact/Widget Unread

Recommended definition:

`unread_count = count of non-internal agent/user/AI replies created after contact_last_seen_at`

Notes:

- Customer messages should not count as unread for the customer.
- Internal notes should never count as unread for the customer.

### 6.3 New Conversation

If the cursor is `NULL`, all qualifying messages are unread.

### 6.4 Closed Conversations

Unread totals should exclude `closed` conversations from aggregate counters unless product explicitly wants historical unread included.

---

## 7. Important Design Corrections

### 7.1 Do Not Blindly Throttle Read Writes by 1 Hour

A strict "only write if cursor is older than one hour" rule is unsafe.

Bad example:

1. Agent opens conversation at 10:00
2. New customer message arrives at 10:05
3. Agent reads it at 10:06
4. If the server skips the write because the cursor was updated less than 1 hour ago, unread remains incorrect

Recommended rule:

- If the new read action advances the cursor past unread messages, write immediately.
- Only throttle repeated no-op "still viewing" updates when the cursor does not need to advance.

### 7.2 Reset `assignee_last_seen_at` on Reassignment

If a conversation is reassigned, the new assignee must not inherit the previous assignee's read state.

When assignment changes:

- clear `assignee_last_seen_at`

### 7.3 Use Response Metadata for Aggregate Counts

Do not compute support rail unread totals by summing only the visible page of conversations in the frontend.

The conversation list endpoint is paginated, so totals must come from the server.

Recommended list response metadata:

- `meta.total`
- `meta.unread.total`
- `meta.unread.my_inbox`
- `meta.unread.unassigned`

---

## 8. Database Changes

### 8.1 Schema Changes via GORM AutoMigrate

Add fields to the `SupportConversation` GORM struct in `server/internal/model/support_inbox.go`. GORM AutoMigrate runs on startup and will add the columns automatically:

```go
AgentLastSeenAt    *time.Time `json:"agent_last_seen_at" gorm:"type:timestamptz"`
AssigneeLastSeenAt *time.Time `json:"assignee_last_seen_at" gorm:"type:timestamptz"`
ContactLastSeenAt  *time.Time `json:"contact_last_seen_at" gorm:"type:timestamptz"`
```

No SQL migration file needed for column additions — GORM handles this.

### 8.2 Indexes (SQL Migration)

Partial indexes cannot be created by GORM AutoMigrate, so these DO need a SQL migration file:

Create: `server/migrations/043_conversation_read_tracking_indexes.sql`

```sql
CREATE INDEX IF NOT EXISTS idx_support_messages_conversation_created_at
  ON support_messages(conversation_id, created_at);

CREATE INDEX IF NOT EXISTS idx_support_messages_conversation_public_created_at
  ON support_messages(conversation_id, created_at)
  WHERE is_internal = false;
```

If agent unread ever includes internal notes, the general index still covers that path.

---

## 9. Backend Model Changes

### 9.1 `server/internal/model/support_inbox.go`

Update `SupportConversation` with:

- `AgentLastSeenAt *time.Time`
- `AssigneeLastSeenAt *time.Time`
- `ContactLastSeenAt *time.Time`
- `UnreadCount int 'json:"unread_count" gorm:"-"'`

Optional future field:

- `AssigneeUnreadCount int 'json:"assignee_unread_count,omitempty" gorm:"-"'`

### 9.2 Widget Payload

Do not add a separate `UnreadCounts map[string]int` to `WidgetSessionJoinedPayload`.

Instead:

- include `unread_count` directly on each conversation in `Conversations`

This keeps widget and internal support payloads aligned.

---

## 10. Repository Layer Plan

File:

- `server/internal/repository/support_inbox.go`

### 10.1 Add Read-Cursor Update Methods

Add:

- `MarkAgentRead(ctx, conversationID, userID string) error`
- `MarkContactRead(ctx, conversationID string) error`

Behavior:

- write immediately when unread exists beyond the stored cursor
- skip or debounce repeated no-op updates
- update `assignee_last_seen_at` as well if the reading user is the current assignee

### 10.2 Extend Conversation List Queries

Update `List()` to populate:

- `last_message`
- `unread_count`

Recommended approach:

- correlated subquery per row for `last_message`
- correlated subquery per row for `unread_count`

Agent unread count query:

```sql
SELECT COUNT(*)
FROM support_messages
WHERE support_messages.conversation_id = support_conversations.id
  AND support_messages.is_internal = false
  AND support_messages.created_at > COALESCE(support_conversations.agent_last_seen_at, to_timestamp(0))
```

### 10.3 Extend Widget Visitor Query

Update `ListByAnonymousID()` to populate widget-side `unread_count` using `contact_last_seen_at`.

Widget unread count query:

```sql
SELECT COUNT(*)
FROM support_messages
WHERE support_messages.conversation_id = support_conversations.id
  AND support_messages.is_internal = false
  AND support_messages.sender_type IN ('user', 'agent', 'ai')
  AND support_messages.created_at > COALESCE(support_conversations.contact_last_seen_at, to_timestamp(0))
```

### 10.4 Aggregate Stats

Add a repository method for aggregate counts used by the internal UI:

- `ListUnreadStats(ctx, workspaceID string, userID string) (...)`

It should return:

- total unread across visible support inbox scope
- unread in "my inbox"
- unread in "unassigned"

These values should be computed server-side and attached to the list response metadata.

---

## 11. Service Layer Plan

Files:

- `server/internal/service/support_inbox.go` — agent read methods (conversation-level)
- `server/internal/service/support_inbox_widget.go` — widget/contact read methods

### 11.1 Add Read APIs

Add:

- `MarkConversationRead(ctx, workspaceID, conversationID, userID string) error`
- `MarkConversationReadByVisitor(ctx, workspaceID, conversationID, anonymousID string) error`

Behavior:

- validate workspace ownership/access
- call the repository read methods
- publish a WebSocket support conversation update/read event

### 11.2 Reassignment Rule

When assigned agent changes:

- clear `assignee_last_seen_at`

### 11.3 Conversation List Serialization

Ensure list responses include:

- conversations with `unread_count`
- metadata with aggregate unread totals

---

## 12. Handler and Router Plan

Files:

- `server/internal/handler/support_inbox.go`
- `server/internal/router/router.go`

### 12.1 Agent Read Endpoint

Add:

- `POST /api/support/inbox/conversations/{id}/read`

Handler responsibilities:

- resolve workspace and actor
- call `MarkConversationRead`
- return success response

### 12.2 Permissions

Use support read permission, not support edit permission.

Reading a conversation should be enough to mark it read.

---

## 13. WebSocket Plan

Unread should be realtime, but the DB remains the source of truth.

### 13.1 Internal Agent WebSocket

No large protocol rewrite is required in v1.

Use the current model:

- agent clients fetch inbox over HTTP
- server broadcasts support conversation updates and support message events over WS
- frontend invalidates and refetches support queries on these events

For read fanout, publish a support conversation event after read updates.

Recommended event shape:

- `entity: "support_conversation"`
- `action: "updated"`
- `data.reason: "read"`

This avoids introducing a parallel event family just for unread.

### 13.2 Widget WebSocket

Widget is already bidirectional, so use WS for widget read acknowledgements.

Add support for:

- `conversation:read`

Also auto-mark read on:

- `conversation:select`

### 13.3 Widget Payloads

Update:

- `session:joined`
- `conversations:listed`

to include `unread_count` on each conversation object.

### 13.4 Message Events

When a new message is delivered:

- internal support clients should invalidate support conversation lists
- widget clients should update conversation unread state in memory, but server truth remains authoritative

---

## 14. Agent Frontend Plan

Files:

- `frontend/src/lib/pmTypes.ts`
- `frontend/src/lib/services/supportService.ts`
- `frontend/src/hooks/queries/useSupport.ts`
- `frontend/src/hooks/queries/index.ts`
- `frontend/src/components/support/ConversationList.tsx`
- `frontend/src/components/support/ConversationRow.tsx`
- `frontend/src/components/support/InboxNavSidebar.tsx`
- `frontend/src/components/layout/Sidebar.tsx`
- `frontend/src/hooks/useRealtimeSync.ts`

### 14.1 Types

Extend `SupportConversation` with:

- `agent_last_seen_at?: string`
- `assignee_last_seen_at?: string`
- `contact_last_seen_at?: string`
- `unread_count?: number`

### 14.2 Service

Add:

- `markConversationRead(workspaceId, conversationId)`

### 14.3 Query Hook Shape

Update the support conversations query to return the full paginated response, not just a flattened array.

Needed because the UI requires:

- conversation items
- aggregate unread metadata

### 14.4 Conversation List

When a user selects a conversation:

- call `markConversationRead`

If the selected conversation is already open and a new message arrives while the thread is visible:

- mark it read again with a small debounce

### 14.5 Conversation Row Styling

When `unread_count > 0`:

- bold name and preview
- apply blue background tint to the row
- render a blue unread pill badge

### 14.6 Sidebar and Filter Counts

Use server-provided metadata:

- support rail badge in `Sidebar.tsx`
- per-filter counts in `InboxNavSidebar.tsx`

Do not derive these counts from the currently loaded page of conversations.

### 14.7 Realtime Sync

In `useRealtimeSync.ts`, on:

- `support_conversation_message`
- `support_conversation updated` with `reason=read`

invalidate:

- support conversations list
- support conversation detail
- support messages if needed

---

## 15. Widget SDK and Widget-Core Plan

Files:

- `packages/sdk-js/src/core/widget.ts`
- `packages/widget-core/src/components/ConversationListView.tsx`
- `packages/widget-core/src/styles/widget.css`
- `packages/widget-core/src/types.ts`

### 15.1 Replace Local-Only Unread Counter

Current widget behavior should be replaced:

- do not treat `WidgetManager.unreadCount` as the source of truth
- do not reset unread to `0` globally on `show()`

Instead:

- derive launcher unread count from the sum of `conversation.unreadCount`
- hydrate unread from `session:joined`
- hydrate unread from `conversations:listed`

### 15.2 Widget Read Behavior

When the customer opens or selects a conversation:

- send `conversation:read`
- optimistically set that conversation's unread to `0`
- keep server as source of truth on next payload refresh

### 15.3 Widget List UI

In `ConversationListView.tsx`:

- render per-conversation unread badge
- bold or tint unread conversations

In `widget.css`:

- add badge styles matching the launcher and the provided blue unread treatment

### 15.4 Widget New Message Handling

On `message:received`:

- if the message belongs to the active visible conversation, mark it read or keep unread at `0`
- if it belongs to another conversation or the widget is closed, increment that conversation's `unreadCount`
- recompute launcher unread as the sum across conversations

This should be optimistic, but the server-correct count should win on the next authoritative conversation payload.

---

## 16. API and Event Examples

### 16.1 Agent List Response

```json
{
  "data": [
    {
      "id": "conv_123",
      "subject": "Billing issue",
      "status": "open",
      "priority": "high",
      "last_message": "Can someone help me with this charge?",
      "updated_at": "2026-03-16T21:00:00Z",
      "unread_count": 2
    }
  ],
  "meta": {
    "total": 48,
    "unread": {
      "total": 12,
      "my_inbox": 5,
      "unassigned": 3
    }
  }
}
```

### 16.2 Agent Read Request

```http
POST /api/support/inbox/conversations/{id}/read?workspace_id=...
```

### 16.3 Widget Session Joined Payload

```json
{
  "type": "session:joined",
  "data": {
    "session_token": "...",
    "expires_at": "...",
    "is_anonymous": true,
    "conversations": [
      {
        "id": "conv_123",
        "subject": "Billing issue",
        "status": "open",
        "last_message": "We have refunded the charge.",
        "updated_at": "2026-03-16T21:00:00Z",
        "unread_count": 1
      }
    ],
    "messages": []
  }
}
```

### 16.4 Realtime Read Event

```json
{
  "entity": "support_conversation",
  "entity_id": "conv_123",
  "workspace_id": "ws_123",
  "action": "updated",
  "data": {
    "reason": "read"
  }
}
```

---

## 17. Edge Cases

### 17.1 Page Refresh

- agent unread survives because it is DB-backed
- widget unread survives because `session:joined` / `conversations:listed` rehydrates from server state

### 17.2 Multiple Agent Tabs

- one tab marks conversation read
- server updates read cursor
- WS event invalidates other tabs
- other tabs refetch and sync immediately

### 17.3 Multiple Widget Tabs

Simple v1 behavior:

- active tab marks read
- other tabs pick up correct unread state from next conversation refresh or realtime conversation update

### 17.4 Reassignment

- assigned agent changes
- `assignee_last_seen_at` must be cleared

### 17.5 Active Thread Open During New Message

If the user is actively viewing the conversation:

- the new message should not remain unread after it is rendered
- use immediate or debounced mark-read behavior

### 17.6 Internal Notes

Recommended v1:

- excluded from internal inbox unread badges
- excluded from widget unread badges

### 17.7 Closed Conversations

Aggregate unread counters should exclude closed conversations unless product wants otherwise.

---

## 18. Testing Plan

### 18.1 Backend Tests

Add coverage for:

- unread count queries for internal agents
- unread count queries for widget contacts
- mark-read advancing cursors
- throttled no-op reads
- assignment change clearing `assignee_last_seen_at`
- list endpoint metadata totals
- widget `conversation:read`

Primary files:

- `server/internal/service/support_inbox_test.go`
- new repository-level support unread tests if needed
- widget handler tests

### 18.2 Frontend Tests

Internal frontend:

- conversation row unread pill rendering
- support rail unread badge
- filter unread count rendering
- mark-read on conversation selection
- realtime invalidation behavior

Widget tests:

- launcher badge reflects server unread
- unread persists across reconnect / session restore
- selecting a conversation clears only that conversation's unread
- incoming agent message increments unread for inactive conversations

Primary files:

- existing support component tests or new ones in `frontend/src/components/support`
- `packages/sdk-js/test/unit/core/widget.test.ts`
- `packages/widget-core` component tests

---

## 19. Rollout Plan

### Phase 1

- migration
- model changes
- repository unread computation

### Phase 2

- internal agent read endpoint
- support inbox unread UI
- support rail and filter badges

### Phase 3

- widget unread hydration from server
- widget conversation unread badges
- widget `conversation:read`

### Phase 4

- realtime refinement
- optimistic patching where safe
- polish, tests, and edge-case hardening

This keeps the system deployable at each step.

---

## 20. Acceptance Criteria

- Internal support conversations show unread badges and blue unread styling.
- Support rail badge reflects total unread from server metadata.
- Inbox filter counters reflect server totals, not only the visible page.
- Clicking or opening a conversation clears unread and syncs across agent tabs.
- Widget launcher badge persists across refresh.
- Widget conversation list shows per-conversation unread badges.
- Opening a widget conversation clears only that conversation's unread.
- New support messages update unread state in near real time for both agents and widget users.

---

## 21. Follow-Up Work After This Plan

The following presence improvements were already completed in the live-chat-events-pipeline branch:

- ~~server-owned viewing presence snapshot~~ ✅ `presence.go`
- ~~per-actor typing timer keys~~ ✅ `useRealtimeSync.ts`
- ~~multiple-agent draft previews without race conditions~~ ✅ multi-agent `agentTyping` map
- ~~disconnect-driven presence cleanup~~ ✅ `Hub.Unregister` → `ClearAllForUser`

Remaining follow-up after unread is landed:

- Assignee-specific unread counters (v2)
- Per-agent read history table for audit/compliance
- Unread state in push notifications / email digests

