# Support Unread Badges and Read Tracking Plan

**Status:** Proposed
**Date:** 2026-03-17
**Scope:** Support inbox, widget unread badges, read tracking, realtime sync
**Modules:** `server`, `frontend`, `packages/sdk-js`, `packages/widget-core`

---

## 1. Summary

This plan adds persistent unread tracking for both:

- internal Helpin users working in the support inbox
- external customers using the widget

The design uses conversation-level read cursors stored directly on `support_conversations`, following the same shared-team-cursor pattern used by Crisp, Intercom, and Chatwoot:

- `agent_last_seen_at` — when any human support agent last read the conversation
- `assignee_last_seen_at` — when the currently assigned human agent last read it
- `contact_last_seen_at` — when the widget visitor last read it

Unread counts are computed on read via correlated subqueries, not denormalized into a separate counter column.

Realtime behavior is provided by WebSocket fanout, but the database remains the durable source of truth. WebSocket is used for immediate sync, not as the storage layer.

This plan is aligned to the product roadmap where AI handles support conversations first and only escalates some conversations into a human support queue. Internal unread badges apply to the human support queue; widget unread still reflects customer-visible replies from AI or humans.

---

## 2. Goals

- Persist unread state across refreshes and reconnects.
- Show unread badges in the internal support inbox conversation list.
- Show unread totals on the Support rail and inbox filter navigation.
- Show unread badges in the widget launcher and widget conversation list.
- Sync unread state across multiple browser tabs and active sessions.
- Keep the design support-specific and compatible with the current codebase.
- Keep the unread model aligned with the AI-first -> human-escalation roadmap so support filters do not need another semantic rewrite later.

---

## 3. Non-Goals

- Building a generic notification framework for all modules.
- Replacing all support HTTP reads with WebSocket reads.
- Solving support presence/typing snapshot issues in the same change set.

---

## 4. Current State

### 4.1 Widget

The widget unread badge is currently client-only:

- `WidgetManager.unreadCount` is a private local counter (line ~43 of `sdk-js/src/core/widget.ts`).
- It increments on `message:received` when `!this.isOpen`.
- It resets to `0` on `show()`.
- It is lost on refresh.
- It is a single global counter, not per-conversation.

This means widget unread is not durable and cannot be server-correct.

### 4.2 Internal Support Inbox

The internal inbox currently has:

- no persistent unread state
- no unread badge in the support list (`ConversationRow.tsx` only shows presence indicators)
- no unread total on the support rail (`Sidebar.tsx` has no badge on the support nav item)
- no read cursor persisted for agents
- `List()` already computes `last_message` via a correlated subquery (prefixes internal notes with "Note: "), so only `unread_count` needs to be added

### 4.3 Realtime

Support already uses WebSocket for:

- message delivery (`support_conversation_message` entity events)
- typing indicators (`support:typing:start/stop/update`)
- viewing presence (`support:viewing:start/stop`)

These are handled in `handler.go` (internal agents) and `widget_handler.go` (widget visitors).

Unread must integrate into the same event pipeline, but should not depend on WebSocket as the only source of truth.

### 4.4 Hub Filtering (Important Constraint)

The Hub's `shouldReceive()` function in `hub.go` filters events for widget clients:

- `support_conversation` events: only delivered to widget clients whose `client.ConversationID` matches `event.EntityID`
- `support_message` events: same conversation-scoped filtering

This means **widget clients do not receive events for conversations they are not actively viewing**. Any cross-conversation unread fanout to widget clients requires a new delivery path (see §7.4).

### 4.5 Existing Widget Handler Events

The widget handler (`widget_handler.go`) already handles these message types:

- `session:create`, `session:restore`
- `message:send`, `typing:start`, `typing:stop`
- `session:upgrade`, `session:revoke`
- `page:update`, `conversations:list`
- `conversation:new`, `conversation:select` (already exists — switches active conversation via `SetSessionConversation()`)
- `ping`

### 4.6 SupportMessage Fields

The `SupportMessage` model has these fields relevant to unread computation:

- `SenderType` (string): `customer`, `user`, `agent`, `ai`
- `IsInternal` (bool): filters whether message is visible to customer (default `false`)
- `MessageType` (string): `reply`, `csat_survey`, `system`

### 4.7 Widget-Core Types

`packages/widget-core/src/types.ts` already defines:

```typescript
interface Conversation {
  id: string;
  subject: string;
  status: string;
  lastMessage?: string;
  lastMessageAt?: string;
  unreadCount?: number;  // already present
}
```

The `WidgetAdapter` interface includes `markAsRead(messageId: string): void` — this is currently message-level and will need to change to conversation-level (see §16.5).

---

## 5. Recommended Design

### 5.1 Durable Source of Truth

Unread state is determined from timestamps stored directly on `support_conversations`:

- `agent_last_seen_at` — shared human-team cursor, updated when any human support agent reads the conversation
- `assignee_last_seen_at` — updated when the currently assigned human agent reads the conversation
- `contact_last_seen_at` — updated when the widget visitor reads the conversation

This is a **shared human-team cursor** model (same as Crisp, Intercom, Chatwoot). When any human support agent reads a conversation, it is marked as read for the human support team. This reflects the collaborative nature of a support inbox — the question is "has this conversation been attended to by a person," not "have I personally read it."

**v1 tradeoff:** Human Agent A reading a conversation clears unread for Human Agent B. This is intentional — in a shared human support inbox, one person triaging means the team has seen it. Per-agent read cursors can be added in v2 if needed (via a `support_conversation_reads` join table).

AI agents do **not** advance `agent_last_seen_at` or `assignee_last_seen_at`. Those cursors are reserved for human support read state.

### 5.2 Realtime Transport

Unread changes should be propagated in real time over WebSocket:

- new message arrives → other sessions invalidate or patch unread state
- conversation marked read → other sessions invalidate or patch unread state

Read events must flow through `hub.BroadcastAll()` (which uses the Redis relay) to ensure multi-pod propagation in Kubernetes deployments.

### 5.3 Payload Shape

Unread should be carried on each conversation object as `unread_count`.

Do not introduce a separate widget-only `unread_counts` map. The current widget and frontend data models already consume conversation arrays, and `widget-core` already has `Conversation.unreadCount`.

### 5.4 Agent vs Widget Model

For v1:

- internal inbox unread is driven by `agent_last_seen_at`, but only for conversations that need human attention
- widget/customer unread is driven by `contact_last_seen_at`
- `assignee_last_seen_at` is maintained only for human assignees

The initial UI still exposes only the shared human-team unread state. `assignee_last_seen_at` is maintained now so assignee-specific unread can be added later without a schema rewrite.

### 5.5 AI-First Routing Model

This plan should align with the future routing model:

- conversations start in an AI-handled lane
- only conversations escalated for human attention contribute to internal support unread
- widget unread still counts public replies from AI, users, and humans because those are customer-visible

To make unread semantics explicit, add minimal human-queue state to `support_conversations`:

- `needs_human` — whether the conversation currently belongs in the human support queue
- `escalated_at` — when the conversation most recently entered the human support queue

Recommended behavior:

- AI support agents can own first-response handling (`agent_kind = 'llm'`, `agent_class = 'support'`)
- when AI escalates to human support, set `needs_human = true` and `escalated_at = now()`
- if a human is not chosen in the same operation, clear `assigned_agent_id` so the conversation lands in the human `unassigned` bucket
- if a human is chosen immediately, write that human agent's `agents.id` into `assigned_agent_id`

### 5.6 Agent Identity Model

`assigned_agent_id` on `support_conversations` refers to `agents.id`, not `users.id`.

For this roadmap, the relevant agent identities are:

- AI support agents: `agent_kind = 'llm'`, `agent_class = 'support'`
- human support reps: `agent_kind = 'human'`, `agent_class = 'human'`, `backing_user_id = users.id`

Any logic that compares the authenticated user to `assigned_agent_id` must first resolve:

`current userID -> current human agent.ID`

---

## 6. Unread Semantics

### 6.1 Internal Agent Unread

Definition:

For internal human-support surfaces:

`unread_count = count of inbound customer messages created after agent_last_seen_at`

Filtering rules:

- Only conversations with `needs_human = true` contribute to internal unread
- Exclude `is_internal = true` (internal notes)
- Exclude `sender_type IN ('user', 'agent', 'ai')` — only count `sender_type = 'customer'`
- Exclude `message_type IN ('csat_survey', 'system')` — only count `message_type = 'reply'`
- This makes the human queue reflect customer work, not internal commentary or system messages

If `needs_human = false`, internal `unread_count` is treated as `0` and the conversation is excluded from internal unread aggregates.

### 6.2 Contact/Widget Unread

Definition:

`unread_count = count of non-internal reply messages from agents/users/AI created after contact_last_seen_at`

Filtering rules:

- Exclude `is_internal = true`
- Exclude `sender_type = 'customer'` — customer's own messages are not unread for them
- Exclude `message_type IN ('csat_survey', 'system')` — only count `message_type = 'reply'`
- This means only actual agent/user/AI replies count as unread for the customer, regardless of whether the conversation is still AI-handled or has escalated to a human

### 6.3 Human Escalation

When AI escalates a conversation into the human support queue:

- set `needs_human = true`
- set `escalated_at`
- do not advance `agent_last_seen_at` during AI handling

This means the first human handoff naturally appears as unread to the human support team without requiring a separate unread table.

If future routing allows repeated AI <-> human handoffs on the same conversation, revisit this with a dedicated `human_handoff_at` marker. That is not required for the initial rollout.

### 6.4 New Conversation

If `agent_last_seen_at` is `NULL`, all qualifying messages are unread for agents.

If `contact_last_seen_at` is `NULL`, all qualifying messages are unread for the widget visitor.

### 6.5 Closed Conversations

Closed conversations are **excluded** from aggregate unread counters (`meta.unread.total`, `meta.unread.my_inbox`, `meta.unread.unassigned`).

Per-conversation `unread_count` is still computed and returned for closed conversations if they appear in a list response, but they do not contribute to sidebar/rail badges.

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
- Implementation: compare incoming `now()` against latest message timestamp; if messages exist after current cursor, write. Otherwise, skip.

### 7.2 Use Response Metadata for Aggregate Counts

Do not compute support rail unread totals by summing only the visible page of conversations in the frontend.

The conversation list endpoint is paginated, so totals must come from the server.

Keep the existing paginated response fields for compatibility, and add unread aggregates under `meta`:

- `meta.unread.total`
- `meta.unread.my_inbox`
- `meta.unread.unassigned`

Do not move `total`, `page`, `per_page`, or `total_pages` under `meta`; keep those at the top level of the existing `PaginatedResponse`.

### 7.3 Provide a Dedicated Unread Stats Endpoint

To avoid refetching the full paginated conversation list just to update sidebar badges after a single read, add a lightweight endpoint:

`GET /api/support/inbox/unread-stats?workspace_id={workspaceId}`

Response:

```json
{
  "total": 12,
  "my_inbox": 5,
  "unassigned": 3
}
```

This endpoint is called:

- on initial load (alongside the conversation list)
- after a `support_conversation updated` event with `reason=read`
- as a WS fallback via polling (optional, low priority)

The `meta.unread` on the list response is still populated for convenience, but the dedicated endpoint avoids unnecessary full list refetches.

### 7.4 Widget Unread Requires Visitor-Scoped WS Fanout

The current Hub `shouldReceive()` logic delivers `support_conversation` events to widget clients only when `event.EntityID` matches `client.ConversationID`. This means a widget client viewing conversation A will not receive events about conversation B.

For visitor-scoped unread fanout, two changes are needed in `hub.go`:

**a) Add `support_visitor_conversations` to `shouldReceive()`:**

```go
case "support_visitor_conversations":
    // Deliver to all widget clients matching the visitor's AnonymousID
    return client.IsWidget && client.AnonymousID == event.EntityID
```

This allows the server to broadcast a `conversations:listed` payload to all of a visitor's active widget sessions (across tabs) without being blocked by the conversation-scoped filter.

**b) Add widget-native translation in `Broadcast()`:**

The existing `Broadcast()` method already translates a small set of events for widget clients (lines ~214-226 of `hub.go`): `message:received`, `typing:start/stop`, `config:updated`. Add the new entity to this translation switch:

```go
case event.Entity == "support_visitor_conversations" && event.Action == "updated" && len(event.Data) > 0:
    widgetData, _ = json.Marshal(widgetMessage{Type: "conversations:listed", Data: event.Data})
```

The caller (service layer) must pre-serialize the `conversations:listed` payload into `event.Data` as the widget-native JSON. No separate `Broadcast2()` method is needed — the existing translation pattern in `Broadcast()` handles it.

**Note:** There is no `Broadcast2()` method today. The dual-payload approach is handled by the translation switch inside `Broadcast()`.

### 7.5 Provide HTTP Fallback for Mark-Read

The primary mark-read flow uses WebSocket (`support:conversation:read` for agents, `conversation:read` for widget). However, WebSocket connections can be temporarily down during reconnection.

Add an HTTP fallback endpoint:

`POST /api/support/inbox/conversations/{conversationId}/read?workspace_id={workspaceId}`

- Requires `PermSupportRead` (not edit)
- Calls the same `MarkConversationRead` service method
- Returns `204 No Content`

The frontend should prefer the WebSocket path but fall back to HTTP if the socket is disconnected.

### 7.6 AI Escalation Must Not Leave an AI Assignee in the Human Queue

Because `assigned_agent_id` refers to a generic `agents.id`, it may point to either an AI support agent or a human agent.

For human unread buckets to remain correct:

- a conversation in the human queue must either be assigned to a human agent or be unassigned
- when AI escalates a conversation without immediately choosing a human, clear `assigned_agent_id`
- do not treat an AI support agent assignment as satisfying `my_inbox` or `unassigned` semantics for the human support UI

---

## 8. Database Changes

### 8.1 Schema Changes via GORM AutoMigrate

Add fields to the `SupportConversation` GORM struct in `server/internal/model/support_inbox.go`. GORM AutoMigrate runs on startup and will add the columns automatically:

```go
NeedsHuman          bool       `json:"needs_human" gorm:"not null;default:false;index"`
EscalatedAt         *time.Time `json:"escalated_at" gorm:"type:timestamptz"`
AgentLastSeenAt    *time.Time `json:"agent_last_seen_at" gorm:"type:timestamptz"`
AssigneeLastSeenAt *time.Time `json:"assignee_last_seen_at" gorm:"type:timestamptz"`
ContactLastSeenAt  *time.Time `json:"contact_last_seen_at" gorm:"type:timestamptz"`
```

No SQL migration file needed for column additions — GORM handles this.

### 8.2 Indexes (SQL Migration)

Partial indexes cannot be created by GORM AutoMigrate, so these DO need a SQL migration file:

Create: `server/migrations/046_conversation_read_tracking_indexes.sql`

```sql
-- Message query indexes for unread count computation
CREATE INDEX IF NOT EXISTS idx_support_messages_conversation_created_at
  ON support_messages(conversation_id, created_at);

CREATE INDEX IF NOT EXISTS idx_support_messages_conversation_public_replies
  ON support_messages(conversation_id, created_at)
  WHERE is_internal = false AND message_type = 'reply';

-- Human queue unread / badge aggregation
CREATE INDEX IF NOT EXISTS idx_support_conversations_workspace_human_queue
  ON support_conversations(workspace_id, status, assigned_agent_id)
  WHERE needs_human = true;

-- Resolve current user -> current human agent
CREATE INDEX IF NOT EXISTS idx_agents_workspace_human_user
  ON agents(workspace_id, user_id)
  WHERE agent_kind = 'human' AND agent_class = 'human' AND user_id IS NOT NULL;
```

Note: The partial index `idx_support_messages_conversation_public_replies` covers both agent and widget unread queries since both filter on `is_internal = false` and `message_type = 'reply'`.

---

## 9. Backend Model Changes

### 9.1 `server/internal/model/support_inbox.go`

Update `SupportConversation` with:

- `NeedsHuman bool` — whether the conversation currently belongs in the human support queue
- `EscalatedAt *time.Time` — when the conversation most recently entered the human support queue
- `AgentLastSeenAt *time.Time` — shared team read cursor
- `AssigneeLastSeenAt *time.Time` — human-assignee-specific read cursor
- `ContactLastSeenAt *time.Time` — widget visitor read cursor
- `UnreadCount int 'json:"unread_count" gorm:"-"'` — virtual field, computed via subquery

Keep `AssignedAgentID` as `agents.id`. In the AI-first model it may point to either an AI support agent or a human agent, but human unread logic only treats it as a human assignment when the conversation is in the human queue.

### 9.2 Widget Payload

Do not add a separate `UnreadCounts map[string]int` to `WidgetSessionJoinedPayload`.

Instead:

- include `unread_count` directly on each conversation in `Conversations`

The existing `WidgetSessionJoinedPayload` struct (which contains `Conversations []SupportConversation`) will automatically include `unread_count` once the field is added to the model and the queries populate it.

---

## 10. Repository Layer Plan

Files:

- `server/internal/repository/support_inbox.go`

### 10.1 Add Read-Cursor Update Methods

Add:

- `MarkAgentRead(ctx, conversationID string, readerHumanAgentID *string) error`
- `MarkContactRead(ctx, conversationID, anonymousID string) error`

Behavior:

- `MarkAgentRead`: sets `agent_last_seen_at = NOW()`. Also sets `assignee_last_seen_at = NOW()` if the reading human agent is the current `assigned_agent_id`.
- `MarkContactRead`: sets `contact_last_seen_at = NOW()`
- Write immediately when unread exists beyond the stored cursor
- Skip or debounce repeated no-op updates when the cursor does not need to advance

This method is only used by the human support read path. AI agents must not update the human read cursors.

### 10.2 Extend Conversation List Query

`List()` already computes `last_message` via a correlated subquery (with "Note: " prefix for internal messages). Add `unread_count` to the existing SELECT:

Agent unread count subquery:

```sql
CASE
  WHEN sc.needs_human = false THEN 0
  ELSE (
    SELECT COUNT(*)
    FROM support_messages sm
    WHERE sm.conversation_id = sc.id
      AND sm.is_internal = false
      AND sm.sender_type = 'customer'
      AND sm.message_type = 'reply'
      AND sm.created_at > COALESCE(sc.agent_last_seen_at, to_timestamp(0))
  )
END AS unread_count
```

This is added alongside the existing `last_message` subquery in the `Select()` call.

### 10.3 Resolve Current User to Current Human Agent

Add to `server/internal/repository/agent.go`:

- `GetHumanByBackingUserID(ctx, workspaceID, userID string) (*model.Agent, error)`

Query requirements:

- `workspace_id = ?`
- `user_id = ?`
- `agent_kind = 'human'`
- `agent_class = 'human'`

This lookup is used by support unread stats and by assignee-specific cursor updates. It is the required bridge between authenticated users and `assigned_agent_id`.

### 10.4 Extend Widget Visitor Query

Update `ListByAnonymousID()` to populate widget-side `unread_count` using `contact_last_seen_at`.

Widget unread count subquery:

```sql
(SELECT COUNT(*)
 FROM support_messages sm
 WHERE sm.conversation_id = sc.id
   AND sm.is_internal = false
   AND sm.sender_type IN ('user', 'agent', 'ai')
   AND sm.message_type = 'reply'
   AND sm.created_at > COALESCE(sc.contact_last_seen_at, to_timestamp(0))
) AS unread_count
```

### 10.5 Aggregate Stats

Add a repository method:

- `GetUnreadStats(ctx, workspaceID string, userID string) (UnreadStats, error)`

The `userID` is used to resolve the current human support agent.

Returns:

```go
type UnreadStats struct {
    Total      int `json:"total"`
    MyInbox    int `json:"my_inbox"`
    Unassigned int `json:"unassigned"`
}
```

**Important: "My Inbox" semantic.** For the AI-first roadmap, `my_inbox` should mean:

`needs_human = true AND assigned_agent_id = currentHumanAgentID`

It should not mean `opened_by_user_id = currentUserID`.

The service should resolve `currentHumanAgentID` by looking up the current workspace's human agent row whose `backing_user_id = current userID`.

Query logic:

- `total`: count of non-closed human-queue conversations in the workspace with unread customer replies after `agent_last_seen_at`
- `my_inbox`: same, filtered to `assigned_agent_id = currentHumanAgentID`
- `unassigned`: same, filtered to `assigned_agent_id IS NULL`

This can be a single query with conditional aggregation:

```sql
SELECT
  COUNT(*) FILTER (WHERE unread > 0) AS total,
  COUNT(*) FILTER (WHERE unread > 0 AND sc.assigned_agent_id = ?) AS my_inbox,
  COUNT(*) FILTER (WHERE unread > 0 AND sc.assigned_agent_id IS NULL) AS unassigned
FROM support_conversations sc
CROSS JOIN LATERAL (
  SELECT COUNT(*) AS unread
  FROM support_messages sm
  WHERE sm.conversation_id = sc.id
    AND sm.is_internal = false
    AND sm.sender_type = 'customer'
    AND sm.message_type = 'reply'
    AND sm.created_at > COALESCE(sc.agent_last_seen_at, to_timestamp(0))
) u
WHERE sc.workspace_id = ?
  AND sc.needs_human = true
  AND sc.status != 'closed'
```

If the current user does not have a human agent record in the workspace, return `my_inbox = 0`.

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
- resolve `current userID -> current human agent.ID` for assignee-specific cursor updates
- check if unread messages exist beyond the current cursor (skip write if already up to date)
- call the repository mark-read methods
- publish websocket fanout after durable cursor updates
- internal agent flow emits the existing raw workspace event shape used by `useRealtimeSync`
- widget flow emits widget-native payload refreshes for the visitor conversation list

### 11.2 Agent Reply Fanout for Widget Unread

The existing `CreateConversationMessage()` in `support_inbox.go` publishes a single `support_conversation_message` event with a hydrated `WidgetMessageReceivedPayload`. The Hub translates this to `message:received` for widget clients, but only for the **active conversation** (due to `shouldReceive()` filtering).

For widget unread to work on **inactive conversations** (e.g., visitor has multiple conversations, or widget is closed), `CreateConversationMessage()` must also publish a visitor-scoped list refresh when the message is a qualifying public reply (non-internal, `sender_type IN ('user', 'agent', 'ai')`, `message_type = 'reply'`).

Add after the existing `support_conversation_message` publish:

```go
// If this is a public agent/user/AI reply and the conversation has an anonymous_id,
// push an authoritative conversations:listed refresh to all of the visitor's widget sessions.
if !msg.IsInternal && msg.SenderType != "customer" && msg.MessageType == "reply" {
    if conv.AnonymousID != nil && *conv.AnonymousID != "" {
        // Fetch updated conversation list with unread counts for this visitor
        conversations, _ := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, *conv.AnonymousID)
        listJSON, _ := json.Marshal(map[string]interface{}{"conversations": conversations})
        s.wsPublisher.Publish(websocket.Event{
            Action:      "updated",
            Entity:      "support_visitor_conversations",
            EntityID:    *conv.AnonymousID, // used by shouldReceive() to match widget clients
            WorkspaceID: workspaceID,
            Data:        listJSON,
        })
    }
}
```

This ensures that when an agent replies, all of the visitor's widget tabs get an updated conversation list with correct `unread_count` values — even for conversations the visitor is not currently viewing.

### 11.3 Reassignment Rule

When assigned agent changes (in existing `AssignConversationAgent()`):

- clear `assignee_last_seen_at` — the new human assignee must not inherit the previous assignee's read state
- `agent_last_seen_at` is NOT cleared — the team has still seen the conversation

When a conversation is escalated from AI to the human queue without an immediate human assignee:

- set `needs_human = true`
- set `escalated_at = NOW()`
- clear `assigned_agent_id`
- clear `assignee_last_seen_at`

### 11.4 Conversation List Serialization

Update `ListConversations` to:

- call `GetUnreadStats()` and attach results to the response as `meta.unread`

Ensure list responses include:

- conversations with `unread_count`, `last_message`, and `needs_human` / `escalated_at` if the frontend needs them for filter styling
- existing `PaginatedResponse` fields (`total`, `page`, `per_page`, `total_pages`)
- `meta.unread` aggregate totals

### 11.5 Unread Stats Endpoint

Add:

- `GetUnreadStats(ctx, workspaceID, userID string) (UnreadStats, error)`

Thin wrapper around the repository method. Used by both:

- the list response `meta.unread` population
- the dedicated `GET .../unread-stats` endpoint


---

## 12. Handler Plan

### 12.1 Existing Handler Updates

In `server/internal/handler/support_inbox.go`:

- The response shape for `ListConversations` changes: the `data` array now includes `unread_count` and `last_message` on each conversation, and a new `meta` object is added alongside the existing pagination fields.

### 12.2 New Endpoints

Add to `server/internal/handler/support_inbox.go`:

- `MarkConversationRead(w, r)` — `POST /support/inbox/conversations/{conversationId}/read`
  - Extract `conversationID` from URL params, `userID` from auth context
  - Call `service.MarkConversationRead()`
  - Return `204 No Content`
  - Permission: `PermSupportRead`

- `GetUnreadStats(w, r)` — `GET /support/inbox/unread-stats`
  - Extract `userID` from auth context
  - Call `service.GetUnreadStats()`
  - Return `200` with `UnreadStats` JSON

### 12.3 Router Registration

In `server/internal/router/router.go`, under the support inbox routes:

```go
r.With(requirePerm(authorization.PermSupportRead)).Get("/inbox/unread-stats", h.SupportInbox.GetUnreadStats)
r.With(requirePerm(authorization.PermSupportRead)).Post("/inbox/conversations/{conversationId}/read", h.SupportInbox.MarkConversationRead)
```

---

## 13. WebSocket Handler Plan

Files:

- `server/internal/websocket/handler.go`
- `server/internal/websocket/widget_handler.go`
- `server/internal/websocket/hub.go`

### 13.1 Agent Read Message

Add to `handler.go` message switch:

- `support:conversation:read`

Handler responsibilities:

- parse `{ conversation_id: string }` from message data
- call `service.MarkConversationRead(ctx, workspaceID, conversationID, userID)`
- the service publishes the broadcast event; the handler does not need to broadcast directly

### 13.2 Widget Read Message

Add to `widget_handler.go` message switch:

- `conversation:read` — marks the specified conversation read for the current widget visitor

Extend existing `conversation:select` handler:

- after calling `SetSessionConversation()`, also call `service.MarkConversationReadByVisitor()`
- ordering: set conversation first, then mark read, to avoid race conditions

### 13.3 Hub Routing Update

Add to `shouldReceive()` in `hub.go`:

```go
case "support_visitor_conversations":
    // Visitor-scoped: deliver to all widget clients with matching AnonymousID
    return client.IsWidget && client.AnonymousID == event.EntityID
```

This enables the server to push `conversations:listed` payloads to all of a visitor's widget sessions.

### 13.4 Permissions and Ownership

For internal agents: use `PermSupportRead` (not edit). Reading a conversation should be enough to mark it read.

For widget traffic: validate through the current session token and `anonymous_id` ownership checks (same pattern used by `conversation:select`) rather than workspace auth middleware.

---

## 14. WebSocket Event Plan

Unread should be realtime, but the DB remains the source of truth.

### 14.1 Internal Agent WebSocket

No large protocol rewrite is required in v1.

Use the current model:

- human support clients fetch inbox over HTTP
- server broadcasts support conversation updates and support message events over WS
- frontend invalidates and refetches support queries on these events
- human read acknowledgements are sent over the existing workspace websocket as `support:conversation:read`

For read fanout, publish a support conversation event after read updates.

Recommended event shape:

- `entity: "support_conversation"`
- `action: "updated"`
- `data.reason: "read"`

Use `hub.BroadcastAll()` to ensure the event propagates across Kubernetes pods via the Redis relay.

### 14.2 Widget WebSocket

Widget should stay WS-first for unread.

Add support for:

- `conversation:read` — explicit mark-read from widget client

Extend existing handler:

- `conversation:select` — also marks the selected conversation read (this handler already exists)

Important implementation notes:

- widget unread fanout must use widget-native websocket payloads
- do not make `sdk-js` or `widget-core` parse raw internal `entity/action` events
- push an authoritative visitor-scoped list refresh as `conversations:listed` after unread-affecting changes
- use the new `support_visitor_conversations` entity type in `shouldReceive()` for cross-conversation delivery

### 14.3 Widget Payloads

Update:

- `session:joined`
- `conversations:listed`

to include `unread_count` on each conversation object. Both payloads use `SupportConversation` which will gain the `unread_count` virtual field.

### 14.4 Message Events

When unread-affecting state changes:

- internal support clients should continue to receive raw workspace events and invalidate support queries
- widget clients should receive `message:received` only for the active conversation stream (existing behavior, unchanged)
- widget clients should receive an authoritative visitor-scoped list refresh (`conversations:listed`) for unread changes affecting any conversation — delivered via the `support_visitor_conversations` entity routing
- widget optimistic state is allowed for UX, but the server payload remains the source of truth

### 14.5 Scalability Note

The `conversations:listed` refresh sends the full visitor conversation list. For v1 this is acceptable because widget visitors typically have few conversations (< 20). If this becomes a bottleneck, a lighter `conversation:updated` delta event can be added in v2 as an incremental patch alongside the full refresh.

---

## 15. Agent Frontend Plan

Files:

- `frontend/src/lib/pmTypes.ts`
- `frontend/src/lib/services/supportService.ts`
- `frontend/src/hooks/queries/useSupport.ts`
- `frontend/src/hooks/queries/index.ts`
- `frontend/src/lib/queryKeys.ts`
- `frontend/src/components/support/ConversationList.tsx`
- `frontend/src/components/support/ConversationRow.tsx`
- `frontend/src/components/layout/Sidebar.tsx` (support rail filters: My Inbox, Unassigned, status filters)
- `frontend/src/hooks/useRealtimeSync.ts`

### 15.1 Types

Extend `SupportConversation` in `pmTypes.ts` with:

- `unread_count?: number`
- `needs_human?: boolean`
- `escalated_at?: string`

Add `'ai'` to `MessageSenderType`:

```typescript
export type MessageSenderType = 'customer' | 'user' | 'agent' | 'ai';
```

Optionally add `agent_last_seen_at?: string` and `assignee_last_seen_at?: string` for display purposes (e.g., "last seen by agent 2h ago"), but `unread_count` is the primary field for badge rendering.

Add response types:

```typescript
export interface UnreadStats {
  total: number;
  my_inbox: number;
  unassigned: number;
}

export interface ConversationListResponse {
  data: SupportConversation[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
  meta: {
    unread: UnreadStats;
  };
}
```

### 15.2 Service Layer

Add to `supportService.ts`:

```typescript
getUnreadStats: (wsId: string) => api.get<UnreadStats>(`/support/inbox/unread-stats?workspace_id=${wsId}`),
markConversationRead: (wsId: string, convId: string) => api.post(`/support/inbox/conversations/${convId}/read?workspace_id=${wsId}`, {}),
```

Update `listConversations` return type to `ConversationListResponse`.

### 15.3 Current Human Agent Resolution

The frontend currently defines `my_inbox` by comparing `opened_by_user_id` to the current user in `ConversationList.tsx`. That must change.

Use the existing `agentService.list(workspaceId)` data to derive:

- AI support agents: `agent_kind === 'llm' && agent_class === 'support'`
- human support agents: `agent_kind === 'human' && agent_class === 'human'`
- `currentHumanAgentId`: the human agent whose `backing_user_id === currentUserId`

The current `useSupportAgents()` hook filters to only LLM support agents. Replace or extend it so the support UI can:

- assign escalated conversations to human agents
- compute `my_inbox` locally as `assigned_agent_id === currentHumanAgentId`
- keep the assignment UI aligned with the backend unread stats semantics

### 15.4 Query Hooks

Update `useConversations` to return the full `ConversationListResponse`, not just a flattened array. Consumers need both conversation items and aggregate unread metadata.

Add new hooks:

```typescript
export function useUnreadStats(workspaceId?: string) {
  return useQuery({
    queryKey: queryKeys.support.unreadStats(workspaceId),
    queryFn: async () => unwrap(await supportService.getUnreadStats(workspaceId!)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}
```

Add to `queryKeys.ts`:

```typescript
support: {
  // ... existing keys ...
  unreadStats: (wsId?: string) => ['support', wsId, 'unread-stats'] as const,
}
```

### 15.5 WebSocket Command

Use the existing workspace websocket send path from `useRealtimeSync` / `useSupportPresenceStore`:

- send `support:conversation:read` via WebSocket as the primary path
- fall back to `supportService.markConversationRead()` HTTP call if the socket is disconnected

### 15.6 Conversation List

When a user selects a conversation:

- send `support:conversation:read`

If the selected conversation is already open and a new message arrives while the thread is visible:

- send `support:conversation:read` again with a small debounce (~500ms)

Update the `my_inbox` filter to use:

- `conversation.needs_human === true`
- `conversation.assigned_agent_id === currentHumanAgentId`

Do not keep using `opened_by_user_id` for this filter.

### 15.7 Conversation Row Styling

In `ConversationRow.tsx`, when `unread_count > 0`:

- bold the subject/name and message preview text
- apply a subtle blue background tint to the row
- render a blue unread pill badge showing the count

This is new DOM — the current `ConversationRow` only has presence indicators (green dot for online, typing dots).

### 15.8 Sidebar and Filter Counts

Use the `useUnreadStats` hook:

- support rail badge in `Sidebar.tsx` — show `stats.total` as a badge on the Support nav item
- per-filter counts in `Sidebar.tsx` support rail section (lines ~884-930) — show `stats.my_inbox` and `stats.unassigned` next to the "My Inbox" and "Unassigned" filter labels

Note: The support inbox filters (My Inbox, Unassigned, status filters) are rendered in `Sidebar.tsx` when `activeRail === 'support'`, not in a separate `InboxNavSidebar.tsx`. The layout component `SupportInboxLayout.tsx` only mounts the conversation list, message thread, and detail panels.

Do not derive these counts from the currently loaded page of conversations.

### 15.9 Realtime Sync

In `useRealtimeSync.ts`, the existing handling already covers:

- `support_conversation_message` → invalidates conversations list and messages (lines ~172-195)
- `support_conversation` with any action → invalidates conversations list and detail (lines ~161-164)

For unread, add:

- On any `support_conversation_message` event: also invalidate `queryKeys.support.unreadStats(wsId)`
- On `support_conversation` with `action: "updated"` and `data.reason: "read"`: invalidate `queryKeys.support.unreadStats(wsId)` and conversations list

---

## 16. Widget SDK and Widget-Core Plan

Files:

- `packages/sdk-js/src/core/widget.ts`
- `packages/widget-core/src/components/ConversationListView.tsx`
- `packages/widget-core/src/styles/widget.css`
- `packages/widget-core/src/types.ts`

### 16.1 Replace Local-Only Unread Counter

Current behavior to replace in `widget.ts`:

- `private unreadCount = 0` (line ~43) — remove as source of truth
- Increment on `message:received` when `!this.isOpen` (line ~756) — replace with server-driven count
- Reset to `0` on `show()` (line ~301) — replace with per-conversation read

New behavior:

- maintain a `conversations: Conversation[]` array as state (hydrated from server)
- derive launcher unread count as `conversations.reduce((sum, c) => sum + (c.unreadCount || 0), 0)`
- hydrate from `session:joined` payload conversations
- update from `conversations:listed` payload

### 16.2 Widget Read Behavior

When the customer opens or selects a conversation:

- send `conversation:read` over websocket
- optimistically set that conversation's `unreadCount` to `0` in local state
- recompute launcher total
- keep server as source of truth on the next `conversations:listed` refresh

### 16.3 Widget List UI

In `ConversationListView.tsx`:

- render a per-conversation unread badge when `conv.unreadCount > 0` — this is **new DOM** (the component currently has no badge element)
- bold or tint unread conversation rows
- show unread count number inside the badge

In `widget.css`:

- add badge styles (blue pill, matching the launcher badge treatment)

### 16.4 Widget New Message Handling

On `message:received` (existing handler at line ~725 of `widget.ts`):

- if the message belongs to the active visible conversation, keep that conversation's unread at `0` (or auto-send `conversation:read`)
- if the widget is closed (`!this.isOpen`), increment the affected conversation's `unreadCount` optimistically and recompute launcher total

For the authoritative state:

- on `conversations:listed` event, replace the entire conversations array from the server payload
- recompute launcher unread from the refreshed data
- this corrects any optimistic drift

### 16.5 Widget Adapter Interface Change

In `packages/widget-core/src/types.ts`, the `WidgetAdapter` interface currently has:

```typescript
markAsRead(messageId: string): void;
```

Change to:

```typescript
markConversationAsRead(conversationId: string): void;
```

This is a **breaking change** to the adapter interface. All adapter implementations must be updated. The old `markAsRead(messageId)` method should be removed.

---

## 17. API and Event Examples

### 17.1 Agent List Response

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
  "total": 48,
  "page": 1,
  "per_page": 50,
  "total_pages": 1,
  "meta": {
    "unread": {
      "total": 12,
      "my_inbox": 5,
      "unassigned": 3
    }
  }
}
```

### 17.2 Agent Read WebSocket Message

```json
{
  "type": "support:conversation:read",
  "data": {
    "conversation_id": "conv_123"
  }
}
```

### 17.3 Agent Read HTTP Fallback

```
POST /api/support/inbox/conversations/{convId}/read?workspace_id={workspaceId}
Authorization: Bearer <jwt>

Response: 204 No Content
```

### 17.4 Unread Stats Endpoint

```
GET /api/support/inbox/unread-stats?workspace_id={workspaceId}
Authorization: Bearer <jwt>

Response: 200
{
  "total": 12,
  "my_inbox": 5,
  "unassigned": 3
}
```

### 17.5 Widget Session Joined Payload

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

### 17.6 Realtime Read Event (Internal)

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

### 17.7 Widget Authoritative List Refresh

```json
{
  "type": "conversations:listed",
  "data": {
    "conversations": [
      {
        "id": "conv_123",
        "subject": "Billing issue",
        "status": "open",
        "last_message": "We have refunded the charge.",
        "updated_at": "2026-03-16T21:00:00Z",
        "unread_count": 0
      },
      {
        "id": "conv_456",
        "subject": "Question about plan limits",
        "status": "open",
        "last_message": "Can you confirm the seat cap?",
        "updated_at": "2026-03-16T21:05:00Z",
        "unread_count": 1
      }
    ]
  }
}
```

### 17.8 Widget Read Message

```json
{
  "type": "conversation:read",
  "data": {
    "conversation_id": "conv_123"
  }
}
```

---

## 18. Edge Cases

### 18.1 Page Refresh

- agent unread survives because it is DB-backed (`agent_last_seen_at` on the conversation)
- widget unread survives because `session:joined` / `conversations:listed` rehydrates from server state

### 18.2 Multiple Agent Tabs

- one tab marks conversation read
- server updates `agent_last_seen_at` (and `assignee_last_seen_at` if applicable)
- WS event (`support_conversation updated, reason=read`) broadcasts to all agents in the workspace
- other tabs refetch and sync immediately
- since this is a shared human-team cursor, all agents see the conversation as read

### 18.3 Multiple Widget Tabs

Simple v1 behavior:

- active tab marks read via `conversation:read`
- server pushes `conversations:listed` to all widget sessions for that visitor (via `support_visitor_conversations` entity routing)
- all tabs update to correct state

### 18.4 Active Thread Open During New Message

If the user is actively viewing the conversation:

- the new message should not remain unread after it is rendered
- use immediate or debounced (~500ms) mark-read behavior

### 18.5 Internal Notes

- excluded from internal inbox unread badges (filtered by `is_internal = false`)
- excluded from widget unread badges (filtered by `is_internal = false`)

### 18.6 CSAT and System Messages

- `message_type = 'csat_survey'` and `message_type = 'system'` are excluded from unread counts for both agents and widget visitors
- only `message_type = 'reply'` counts toward unread

### 18.7 Closed Conversations

- per-conversation `unread_count` is still computed if a closed conversation appears in a response
- aggregate counters (`meta.unread.*`, unread-stats endpoint, sidebar badges) **exclude** closed conversations

### 18.8 Conversation Select → Read Ordering

When a widget visitor selects a conversation:

1. `conversation:select` handler calls `SetSessionConversation()` first
2. Then calls `MarkConversationReadByVisitor()`
3. Server responds with messages for the selected conversation
4. Server pushes `conversations:listed` with updated unread counts

This ordering ensures the conversation is fully loaded before unread state is cleared.

### 18.9 Reassignment

When a conversation is reassigned to a different agent:

- `assignee_last_seen_at` is cleared (set to `NULL`)
- `agent_last_seen_at` is NOT cleared — the team has still seen the conversation
- The new assignee sees the conversation as attended-to (shared cursor) but their assignee-specific cursor is fresh

### 18.10 AI -> Human Escalation

When AI escalates a conversation into the human support queue:

- `needs_human` becomes `true`
- `escalated_at` is set
- if a human is not assigned in the same operation, `assigned_agent_id` is cleared
- the conversation starts contributing to internal support unread aggregates
- widget unread behavior does not change; customer-visible AI/human replies still count the same way

---

## 19. Testing Plan

### 19.1 Backend Tests

Add coverage for:

- unread count queries for internal human agents (shared team cursor via `agent_last_seen_at`)
- unread count queries for widget contacts (via `contact_last_seen_at`)
- `MarkAgentRead` updating `agent_last_seen_at` and conditionally `assignee_last_seen_at`
- `MarkContactRead` updating `contact_last_seen_at`
- throttled no-op reads (cursor already up to date)
- list endpoint populating `last_message` and `unread_count`
- list endpoint metadata totals (`meta.unread`)
- unread-stats endpoint
- `POST .../read` HTTP endpoint
- agent `support:conversation:read` websocket handling
- widget `conversation:read` websocket handling
- widget `conversation:select` auto-marking read
- widget visitor-scoped `conversations:listed` fanout after unread changes
- Hub `shouldReceive` for `support_visitor_conversations` entity type
- CSAT and system messages excluded from unread
- closed conversations excluded from aggregates
- AI-only conversations excluded from internal human unread aggregates
- `GetHumanByBackingUserID()` resolution by `agents.user_id`
- AI escalation clearing or replacing the AI assignee before the conversation enters the human queue
- reassignment clearing `assignee_last_seen_at`

Primary files:

- `server/internal/service/support_inbox_test.go`
- `server/internal/repository/support_inbox_test.go` (new)
- `server/internal/websocket/handler_test.go`
- `server/internal/websocket/widget_handler_test.go`
- `server/internal/websocket/hub_test.go`

### 19.2 Frontend Tests

Internal frontend:

- conversation row unread pill rendering
- conversation row bold text when unread
- support rail unread badge
- filter unread count rendering
- mark-read on conversation selection
- mark-read debounce on new message while viewing
- HTTP fallback when WS disconnected
- realtime invalidation of unread-stats on message events
- `my_inbox` filtering by `currentHumanAgentId`, not `opened_by_user_id`
- support agent hooks exposing both AI support agents and human agents where needed

Widget tests:

- launcher badge reflects server unread (sum of per-conversation counts)
- unread persists across reconnect / session restore
- selecting a conversation clears only that conversation's unread
- authoritative `conversations:listed` refresh updates unread for inactive conversations
- `markConversationAsRead` adapter method works correctly

Primary files:

- existing support component tests or new ones in `frontend/src/components/support`
- `packages/sdk-js/test/unit/core/widget.test.ts`
- `packages/widget-core` component tests

---

## 20. Rollout Plan

### Phase 1: Data Layer

- migration `046_conversation_read_tracking_indexes.sql` — message query indexes
- `NeedsHuman`, `EscalatedAt`, `AgentLastSeenAt`, `AssigneeLastSeenAt`, `ContactLastSeenAt` fields on `SupportConversation` (GORM AutoMigrate)
- repository: `MarkAgentRead`, `MarkContactRead`
- repository: update `List()` to compute `last_message` + `unread_count`
- repository: update `ListByAnonymousID()` to compute widget `unread_count`
- repository: `GetUnreadStats()`
- agent repository: `GetHumanByBackingUserID()`

### Phase 2: Service + HTTP Endpoints

- service: `MarkConversationRead`, `MarkConversationReadByVisitor`, `GetUnreadStats`
- service: update `ListConversations` to attach `meta.unread`
- service: update `AssignConversationAgent` to clear `assignee_last_seen_at` on reassignment
- handler: `POST .../conversations/{id}/read`, `GET .../unread-stats`
- router: register new endpoints

### Phase 3: Internal Agent Realtime + UI

- websocket handler: `support:conversation:read` in `handler.go`
- websocket hub: add `support_visitor_conversations` to `shouldReceive()`
- frontend types: extend `SupportConversation`, add `UnreadStats`, `ConversationListResponse`
- frontend service + hooks: `useUnreadStats`, update `useConversations` response shape
- frontend agent data: resolve `currentHumanAgentId`, expose human agents for escalated assignment, keep AI support agents available for first-response tooling
- frontend UI: `ConversationRow` unread styling + badge, `Sidebar` badge, `Sidebar` filter counts
- frontend realtime: invalidate unread-stats on message/read events
- frontend mark-read: send `support:conversation:read` on select + new message while viewing

### Phase 4: Widget

- widget handler: `conversation:read` handler, extend `conversation:select` to mark read
- widget service: update visitor conversation queries to include `unread_count`
- widget-core: update `WidgetAdapter` interface (`markConversationAsRead`)
- widget-core: `ConversationListView` unread badge DOM + CSS
- sdk-js: replace local `unreadCount` with server-driven per-conversation state
- sdk-js: handle `conversations:listed` for cross-conversation unread refresh
- widget fanout: `support_visitor_conversations` entity for visitor-scoped broadcasts

### Phase 5: Polish

- realtime refinement and optimistic patching where safe
- HTTP fallback for mark-read when WS is disconnected
- edge-case hardening (CSAT exclusion, closed conversation exclusion, etc.)
- full test coverage per §19

Each phase is independently deployable.

---

## 21. Acceptance Criteria

- Internal support conversations show unread badges and blue unread styling.
- Unread reflects a shared human-team cursor — any human support agent reading clears unread for the team.
- Support rail badge reflects total unread from server metadata.
- Inbox filter counters (my inbox, unassigned) reflect server totals, not only the visible page.
- `My Inbox` reflects conversations in the human queue assigned to the current user's human agent record.
- AI-only conversations do not contribute to internal human unread badges.
- Clicking or opening a conversation clears unread and syncs across all agent tabs.
- Widget launcher badge persists across refresh.
- Widget conversation list shows per-conversation unread badges.
- Opening a widget conversation clears only that conversation's unread.
- New support messages update unread state in near real time for both agents and widget users.
- CSAT survey and system messages do not count as unread.
- Closed conversations do not contribute to aggregate badge counts.

---

## 22. Breaking Changes

This section lists interface and response shape changes that require coordinated updates:

1. **`listConversations` response shape** — changes from a flat `SupportConversation[]` to `ConversationListResponse` with `data`, pagination fields, and `meta.unread`. All consumers must update.

2. **`List()` repository query** — gains `last_message` and `unread_count` subqueries (SELECT clause change, no signature change).

3. **`WidgetAdapter.markAsRead(messageId)`** — renamed to `markConversationAsRead(conversationId)`. All adapter implementations must update.

4. **`MessageSenderType`** — gains `'ai'` value. Existing type guards may need updating.

5. **Hub `shouldReceive()`** — gains a new entity type `support_visitor_conversations`. No external impact but requires hub code change.

6. **`My Inbox` semantic** — changes from the current `opened_by_user_id` behavior to `needs_human = true AND assigned_agent_id = currentHumanAgentId`.

7. **Support agent hooks / assignment UI** — cannot stay LLM-only. Any support-agent selector or helper that currently filters to `agent_kind = 'llm' && agent_class = 'support'` must be widened to include human agents where assignment and `my_inbox` semantics require it.

---

## 23. Follow-Up Work After This Plan

The following presence improvements were already completed in the live-chat-events-pipeline branch:

- ~~server-owned viewing presence snapshot~~ done `presence.go`
- ~~per-actor typing timer keys~~ done `useRealtimeSync.ts`
- ~~multiple-agent draft previews without race conditions~~ done multi-agent `agentTyping` map
- ~~disconnect-driven presence cleanup~~ done `Hub.Unregister` → `ClearAllForUser`

Remaining follow-up after unread is landed:

- Per-agent read cursors via `support_conversation_reads` join table (v2, if shared team cursor proves insufficient)
- Assignee-specific unread counters in the UI (data already stored via `assignee_last_seen_at`)
- Unread state in push notifications / email digests
- Per-conversation sort ordering (unread-first option in inbox)
- Lightweight `conversation:updated` delta event for widget (v2 optimization for `conversations:listed` scalability)
- If support later supports repeated AI <-> human handoffs on the same conversation, consider adding a dedicated `human_handoff_at` marker to keep unread semantics anchored to the latest escalation point
