# PRD: Helpin Widget SDK — Feature Parity & Architecture Improvements

## Context

When the help widget is opened via auto-boot (`data-widget-key` script attribute), the WebSocket never connects. Root cause: `boot({ key, host })` has no `user` object, so it only calls `fetchWidgetConfig()` — never `initializeSession()` or `connectWebSocket()`.

A comprehensive analysis of Crisp's production widget (`v4.4.4`) reveals we have **17/68 features** vs Crisp's **66/68**. This PRD covers the architecture fixes, the full feature gap, and a phased implementation plan.

---

## Identity Model

Four distinct primitives. Each has a single responsibility:

| Primitive | Scope | Lifetime | Storage | Purpose |
|-----------|-------|----------|---------|---------|
| `anonymous_id` | Browser identity | ~10 years | Cookie (`helpin_aid_{widget_key}`) | Durable anonymous identity. Ties a browser to all its conversations across visits. Shared with analytics pipeline for cross-system correlation. |
| `session_token` | Auth credential | 30 days (revocable) | localStorage (`helpin_ws_{widget_key}`) | Authenticates widget HTTP/WS calls. Can be revoked server-side. Cleared on `shutdown()`. Multiple concurrent sessions allowed (tabs). |
| `conversation_id` | Chat thread | Indefinite | Server-side (PostgreSQL) | Identifies a single conversation thread. A visitor can have multiple conversations. Widget receives messages scoped to their active `conversation_id` only. |
| `session_id` | Internal DB record | 30 days | Server-side only (never sent to client) | Primary key of `support_widget_sessions` table. Internal reference, not exposed in any client API. |

**Relationships:**
```
anonymous_id (1) ←→ (N) session_token     Multiple concurrent sessions allowed (e.g. multiple tabs)
anonymous_id (1) ←→ (N) conversation_id   A visitor can have many conversations over time
session_token (1) ←→ (1) session_id     Token maps to one internal session record
session_id (1) ←→ (0..1) conversation_id Session tracks active conversation, but visitor owns the history
conversation.anonymous_id ←→ anonymous_id   Direct ownership — conversations queryable without joining through sessions
```

**Key principles:**
- `anonymous_id` is the **only** client-facing identity. It goes in the cookie, it goes to the analytics pipeline, it's the key for session restore.
- `session_token` is a **credential**, not an identity. It proves the browser is authorized. It can be revoked without losing visitor identity.
- `conversation_id` is a **thread**, not a session. Starting a "new conversation" creates a new `conversation_id` without touching the session.
- `session_id` is **internal**. The client never sees it, never stores it, never sends it.

### `anonymous_id` Across the Stack

The same UUID flows end-to-end under consistent naming. This is the **single identity** that ties a browser to its analytics events, chat conversations, and CRM contacts.

| Layer | Location | Column / Field / Key | Notes |
|-------|----------|---------------------|-------|
| **Browser cookie** | Client-side | `helpin_aid_{widget_key}` | 10-year TTL. Source of truth for browser identity. |
| **SDK (analytics)** | `packages/sdk-js/src/core/client.ts` | `anonymous_id` in event payload | Sent with every analytics event as `user.anonymous_id` |
| **SDK (widget)** | `packages/sdk-js/src/core/widget.ts` | `this.anonymousId` | Read from shared identity module, sent in WS `session:create` |
| **SDK (identity)** | `packages/sdk-js/src/core/identity.ts` | `getOrCreateAnonymousId(widgetKey)` | Shared utility — single source of truth for both analytics + widget |
| **WS protocol** | Widget ↔ Server | `anonymous_id` field in `session:create` | Server uses it to look up/create sessions and conversations |
| **PostgreSQL** | `support_widget_sessions` | `anonymous_id TEXT NOT NULL` | Links session to browser identity |
| **PostgreSQL** | `support_conversations` | `anonymous_id TEXT` | Direct ownership — query conversation history by browser identity |
| **Rust Capture API** | `events-pipeline/rust-capture/` | `user_anonymous_id` in `TransformedEvent` | Same UUID, different field name (events pipeline convention) |
| **Kafka** | `incoming_events` topic | `user_anonymous_id` | Part of event payload flowing through Kafka |
| **KStreams** | `SessionEventWindowStream.java` | Key: `{project_id}:{user_anonymous_id}` | Session windowing groups events by this key |
| **ClickHouse** | Events table | `user_anonymous_id` column | Queryable — correlate pageviews with chat events |
| **CRM** | `crm_contacts` | Matched by email after `session:upgrade` | `anonymous_id` links pre-identification browsing to CRM contact |

**Why the events pipeline uses `user_anonymous_id` instead of `anonymous_id`:** The events pipeline prefixes all user fields with `user_` (e.g., `user_id`, `user_email`, `user_anonymous_id`). This is a namespace convention in the flattened ClickHouse schema — the underlying UUID is the same value from the `helpin_aid_{widget_key}` cookie.

---

## Existing SDK Infrastructure

The SDK already has persistence and identity mechanisms. Some need renaming, some need exposing.

### What exists today

| Mechanism | Location | Current key/format | Status |
|-----------|----------|--------------------|--------|
| Anonymous ID cookie | `src/utils/cookie.ts` via `CookieManager` | `__eventn_id_{apiKey}` | **Rename** → `helpin_aid_{widget_key}` (matches `user_anonymous_id` in events pipeline) |
| localStorage persistence | `src/persistence/local-storage.ts` | `helpin_{apiKey}_data` | **Rename** → `helpin_analytics_{widget_key}` (analytics only) |
| `getOrCreateAnonymousId()` | `src/core/client.ts:220` (private) | Generates UUID, stores in cookie | **Expose** via shared `identity.ts` module |
| `identify()` / `id()` | `src/core/client.ts:267` | Persists userId + sends `user_identify` event with `anonymous_id` | Keep as-is |
| `getAnonymousId()` | `src/core/widget.ts:178` | Reads from localStorage | **Remove** — replaced by cookie-based `anonymous_id` via `identity.ts` |
| `sessionToken` | `src/core/widget.ts` (in-memory) | Ephemeral, lost on refresh | **Persist** to localStorage |
| Widget embed script | `ChatGeneralTab.tsx:190` | `https://cdn.helpin.ai/lib.js` | **Standardize** (see Naming below) |

### Cookie & key naming standardization

**Before (inconsistent):**
```
__eventn_id_{apiKey}           → analytics anonymous ID cookie (legacy naming from Jitsu)
helpin_anonymous_id              → widget visitor ID in localStorage (different from above!)
helpin_{apiKey}_data           → analytics persistence
session_token                  → in-memory only, lost on refresh
```

**After (consistent `helpin_` prefix, keyed by `widget_key`):**
```
helpin_aid_{widget_key}        → anonymous_id cookie (same UUID as user_anonymous_id in events pipeline)
helpin_ws_{widget_key}         → widget session: { session_token, expires_at } in localStorage
helpin_wc_{widget_key}         → widget config cache: { config, cached_at } in localStorage
helpin_analytics_{widget_key}  → analytics persistence (userId, userProps, companyProps)
```

### Script & parameter naming standardization

| Term | Standardized name | Used in |
|------|------------------|---------|
| Widget API key | `widget_key` | Script attribute: `data-widget-key`, HTTP param, WS message field |
| Embed script file | `lib.js` | `https://cdn.helpin.ai/lib.js`, settings UI embed snippet |
| API host | `data-host` | Script attribute (defaults to `https://helpin.ai`) |

---

## Feature Parity Matrix: Helpin vs Crisp

### Widget Chrome & Navigation (1/9)

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Home view | Hub with help, chat, helpdesk, status, search | Basic home view | -- |
| Messages/Conversations list | Multiple conversations with history | Single conversation only | SHOULD |
| Helpdesk/Articles in widget | Searchable knowledge base | None | SHOULD |
| Unified search | Search across articles + content | None | SHOULD |
| Expand/Collapse (full screen) | Yes | None | NICE |
| Download transcript | Yes | None | NICE |
| New conversation button | Multiple concurrent conversations | None | SHOULD |
| Service status page | Healthy/slowdown/outage indicators | None | NICE |
| Quick Actions pad | Yes | None | NICE |

### Messaging (5/12)

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Text messages | Yes | Yes | -- |
| File attachments | Upload, drag & drop, size limits | Basic | SHOULD (improve) |
| Emoji picker | Categorized (smileys, animals, food, etc.) | None | SHOULD |
| GIF picker | Search GIFs | None | NICE |
| Audio messages | Record from microphone | None | NICE |
| Typing indicators | compose_send/receive events | Basic isTyping | -- |
| Read receipts | Yes | None | SHOULD |
| Message editing | Yes, 'Edited' tag | None | NICE |
| Bot/AI messages | Bot tag, AI badge, sources | AI responses with sources | -- |
| Translated messages | 'Translated' tag | None | NICE |
| Rich message types | Pickers, fields, carousels, articles | None | SHOULD |
| Chat feedback/rating | Rate support + comment | None | SHOULD |

### Identity & Session (3/7) — CRITICAL GAP

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Anonymous sessions | Cookie-based, immediate on boot | None — requires user data | **MUST** |
| Session persistence | Configurable cookie domain/expiry | In-memory only, lost on refresh | **MUST** |
| In-chat email collection | Inline prompt "What is your email?" | Pre-chat form only | SHOULD |
| In-chat phone collection | Phone field with validation | None | NICE |
| Programmatic identify | `$crisp.push(['set', 'user:email'])` | `identify()` exists | -- |
| Returning user recognition | CRISP_TOKEN_ID restores full context | None | **MUST** |
| Cross-domain tracking | Likely | Yes (`_hp` param) | -- |

### Connection & Transport (2/7) — CRITICAL GAP

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| WebSocket connection | Socket.IO/Engine.IO with polling fallback | Raw WebSocket | -- |
| Immediate connection on load | Yes — connects before user interacts | No — only after session init | **MUST** |
| Relay/fallback servers | client.relay, stream.relay domains | None | NICE |
| Connection failure UI | "Support may be unavailable" message | Silent failure | **MUST** |
| Exponential backoff retry | Yes | Yes | -- |
| Socket affinity | Yes | None | NICE |
| Preconnect hints | `<link rel="preconnect">` | None | SHOULD |

### Loading & Performance (0/7) — CRITICAL GAP

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Two-phase loader | l.js (7.7KB) → async main bundle | Single ~86KB bundle | SHOULD |
| Separate CSS files | Base + lazy feature CSS | CSS inline in JS | SHOULD |
| Content-hashed filenames | `{name}_{theme}_{hash}.{ext}` | Plain `lib.js` | SHOULD |
| Lazy feature CSS loading | Chat/overlay/browsing/call on demand | None | NICE |
| Bot/crawler filtering | UA blocklist + DOM fingerprints | None | **MUST** |
| Browser compat checks | Chrome 63+, Firefox 67+, Safari 12+ | None | SHOULD |
| Locale/i18n | 50+ languages, separate locale files | English only, hardcoded | SHOULD |

### CSS & DOM Isolation (1/4) — CRITICAL GAP

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Style isolation | Aggressive CSS reset, `!important` on ALL props | No isolation | **MUST** |
| RTL support | `dir=rtl`, CSS direction | None | NICE |
| CSS custom properties | `--crisp-color-*` tokens | CSS variables for theming | -- |
| Namespace scoping | `.crisp-client` wrapper | `.helpin-*` classes (partial) | -- |

### Availability & Status (0/5)

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Online/Away status | Real-time agent availability | None | SHOULD |
| Response time metrics | "Typically replies under X" | None | SHOULD |
| Last active time | "Last active X ago" | None | NICE |
| Agent avatars | Team avatars with "And X more" | None | SHOULD |
| Business hours | Schedule-based availability | None | NICE |

### Calls & Video (0/3)

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Audio/Video calls | WebRTC with STUN/TURN | None | NICE (future) |
| Screen sharing | Yes | None | NICE (future) |
| Co-browsing | Live MutationObserver-based | None | NICE (future) |

### Engagement & UX (2/7)

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Proactive messages (entice) | Tooltip with custom text | None | SHOULD |
| Unread badge count | Yes | Yes | -- |
| Customizable theme text | 5+ variants per locale | Partial (via config) | -- |
| Welcome message variants | 6 templates | None | NICE |
| Continue on email/phone | Channel switching | None | NICE |
| Custom color theming | `--crisp-color` tokens | Yes (widget config) | -- |
| Connection failure message | "Support may be unavailable" | None | **MUST** |

### Developer API (4/7)

| Feature | Crisp | Helpin | Priority |
|---------|-------|--------|----------|
| Command-style API | `$crisp.push(['method', args])` | `helpin('method', args)` | -- |
| Event callbacks | Multiple event types | onShow, onHide, etc. | -- |
| Programmatic show/hide | Yes | Yes | -- |
| Ready trigger | CRISP_READY_TRIGGER callback | None | SHOULD |
| Runtime config override | CRISP_RUNTIME_CONFIG | None | NICE |
| Cookie domain/expiry config | CRISP_COOKIE_DOMAIN/EXPIRE | None | SHOULD |
| Reset/re-init | Full reset handler | shutdown() + boot() | -- |

---

## Priority Summary

| Priority | Count | Items |
|----------|-------|-------|
| **MUST** | 8 | Anonymous sessions, session persistence, returning user, immediate WS, connection failure UI, bot filtering, CSS isolation, connection failure message |
| **SHOULD** | 20 | Conversations list, helpdesk, search, new conversation, file improvements, emoji, read receipts, rich messages, feedback, in-chat email, preconnect, two-phase loader, separate CSS, hashing, compat checks, i18n, online/away, response metrics, agent avatars, proactive messages, ready trigger, cookie config |
| **NICE** | 23 | Expand/collapse, transcript, status, quick actions, GIF, audio, editing, translation, phone collection, relay servers, affinity, lazy CSS, RTL, last active, business hours, calls, video, screen share, co-browsing, welcome variants, channel switching, runtime config |

---

## Data Storage Architecture

### Current State

| Layer | Technology | What it stores |
|-------|-----------|----------------|
| In-memory | Go `sync.RWMutex` map in `Hub` | Active WS connections: `map[workspaceID]map[*Client]struct{}` |
| PostgreSQL | `support_widget_sessions` table | Session token, customer name/email, conversation_id, expires_at (24h) |
| PostgreSQL | `support_conversations` + `support_messages` | Chat history (created lazily on first message) |
| Cross-process | PG `LISTEN/NOTIFY` on `ws_events` channel | Bridges Temporal worker events → Hub → browser clients |

**No Redis.** Single Go process. Hub is ephemeral (rebuilt on restart).

### Target State

Three storage tiers — no new infrastructure required:

#### Tier 1: In-Memory (Hub — ephemeral, rebuilt on reconnect)

| Data | Structure | Purpose |
|------|-----------|---------|
| Active WS connections | `map[workspaceID]map[*Client]struct{}` | Already exists — route messages to connected clients |
| Widget client registry | `map[sessionToken]*WidgetClient` | Map connected widget clients to their active conversation_id for scoped routing |
| Operator support status | `map[workspaceID]map[userID]*SupportAgent` | Track which agents are actively available for support (not just WS-connected) |
| Response metrics cache | `map[workspaceID]*ResponseMetrics` | Cached AVG first-reply time, recomputed every 5 min |
| Typing indicators | Ephemeral, broadcast on receive | No storage needed — relay directly to conversation participants |

```go
// New: tracks widget WS clients for conversation-scoped routing
type WidgetClient struct {
    Client         *Client   // existing WS client
    SessionToken   string
    AnonymousID      string
    WorkspaceID    string
    ConversationID *string   // nil until first message creates a conversation
}

// New: agent support availability (distinct from WS connection)
type SupportAgent struct {
    UserID     string
    Nickname   string
    Avatar     *string
    LastActive time.Time
    Status     string    // "available", "away" — explicit, not derived from WS presence
}

type ResponseMetrics struct {
    Count       int
    MeanSeconds float64
    CachedAt    time.Time
}
```

**Why explicit agent status instead of deriving from WS connections:** The current `/api/ws` is used by all workspace members (PM, CRM, etc.), not just support agents. Deriving availability from WS presence would incorrectly show a PM manager as "available for support." Agents need to explicitly set support availability.

#### Tier 2: PostgreSQL (persistent, survives restarts)

| Table | New/Existing | What changes |
|-------|-------------|-------------|
| `support_widget_sessions` | Existing — extend | Add `anonymous_id`, `is_anonymous`, `user_agent`, `last_page_url`, `revoked_at` columns. Change TTL from 24h to 30 days. |
| `support_conversations` | Existing — **extend** | Add `anonymous_id` column. Direct ownership link — conversations queryable by visitor without joining through sessions. |
| `support_messages` | Existing — no changes | Individual messages |
| `support_widget_installations` | Existing — no changes | Widget config (key, branding, pre-chat settings) |

**Conversation ownership via `anonymous_id`:**

The `support_conversations` table gets a `anonymous_id` column set when the conversation is created from a widget session. This enables direct queries like `SELECT * FROM support_conversations WHERE workspace_id = ? AND anonymous_id = ? ORDER BY created_at DESC` without fragile joins through sessions (which expire and get cleaned up).

**Concurrency model — multiple sessions per visitor allowed:**

Multiple tabs/devices can boot simultaneously with the same `anonymous_id`. Each gets its own `session_token`. This is intentional:
- Two tabs open = two sessions, both valid, both can send/receive
- All sessions share the same conversation history (via `anonymous_id` on `support_conversations`)
- On session restore (`POST /widget/session`), server creates a new session for this visitor — it does NOT try to find and reuse an existing one (avoids race conditions)
- The `anonymous_id` is the durable identity, the `session_token` is a disposable credential

**Session lifecycle in PostgreSQL:**
```
Anonymous boot (no stored session):
  → Client connects WS with widget_key (unauthenticated)
  → WS send: session:create { anonymous_id, page_url, ua, tz, locale }
  → Server creates session (is_anonymous=true, expires_at = now + 30d)
  → Server queries conversations by (workspace_id, anonymous_id)
  → WS recv: session:joined { session_token, expires_at, conversations[], messages[] }
  → Client persists { session_token, expires_at } to localStorage

Page reload (stored session exists):
  → Client reads { session_token, expires_at } from localStorage
  → Client checks expires_at > now (client-side expiry check)
  → If valid: connect WS → WS send: session:restore { session_token }
      → WS recv: session:joined { ... conversations[], messages[] }
  → If expired: discard stored session, fall back to session:create flow
  → If server returns session:error (revoked/invalid):
      → Clear localStorage, send session:create on same WS connection (one retry only)

Session upgrade (visitor provides email/name):
  → WS send: session:upgrade { email, name }
  → Server updates session: is_anonymous=false, email, name
  → Server links to CRM contact if email matches
  → WS recv: session:upgraded

Shutdown / logout:
  → WS send: session:revoke (or HTTP POST fallback if WS closed)
  → Server sets revoked_at = now()
  → Client clears localStorage: helpin_ws_{widget_key}
  → anonymous_id cookie is NOT cleared (browser identity persists, but session is gone)

New boot after shutdown:
  → No stored session → session:create flow → new session_token, same anonymous_id
  → Previous conversations returned in session:joined via anonymous_id on support_conversations

Session cleanup:
  → Cron job: DELETE FROM support_widget_sessions WHERE expires_at < now()
     OR (revoked_at IS NOT NULL AND revoked_at < now() - INTERVAL '7 days')
```

#### Tier 3: Client-Side (browser — localStorage + cookies)

| Key | Storage | What | TTL |
|-----|---------|------|-----|
| `helpin_aid_{widget_key}` | Cookie | Visitor ID (UUID) | 10 years |
| `helpin_ws_{widget_key}` | localStorage | `{ session_token, expires_at }` | 30 days (cleared on shutdown) |
| `helpin_wc_{widget_key}` | localStorage | `{ config, cached_at }` | 10 min |
| `helpin_analytics_{widget_key}` | localStorage | `{ userId, userProps, companyProps }` | Indefinite (analytics persistence) |

---

## Conversation-Scoped WebSocket Routing

### The Security Problem

**Current:** Hub broadcasts by `workspace_id`. When a `support_conversation_message` event fires, **every** widget client in that workspace receives it. The widget client currently appends any message it sees — no conversation filtering.

```
Agent replies to Visitor A's conversation
  → Hub.Broadcast(Event{Entity: "support_conversation_message", WorkspaceID: "ws1"})
  → ALL clients in ws1 receive it: agents, Visitor A, AND Visitor B
  → Visitor B's widget appends the message meant for Visitor A
  → SECURITY BUG: cross-conversation leakage
```

### The Fix: Conversation-Scoped Routing for Widget Clients

**New:** Widget clients are registered with their `conversation_id`. The Hub routes `support_conversation_message` events only to participants of that conversation.

```go
// Hub extension for conversation-scoped routing
func (h *Hub) BroadcastToConversation(event Event, conversationID string) {
    // 1. Send to all internal clients (agents) in the workspace — they see all conversations
    // 2. Send to widget clients ONLY if their active conversation_id matches
}
```

**When conversation_id is set:**
- On session restore: if the session has a `conversation_id`, set it on the widget client
- On first message: `WidgetCreateMessage()` creates a conversation and updates the widget client's `conversation_id`
- On conversation switch (Phase 3): widget sends a message to update its active `conversation_id`

**Routing rules:**

| Event entity | Route to internal clients (agents) | Route to widget clients |
|---|---|---|
| `support_conversation_message` | All agents in workspace (they need to see all conversations) | Only widget client whose `conversation_id` matches `event.ParentID` |
| `support_conversation` (status change, assignment) | All agents in workspace | Only widget client whose `conversation_id` matches `event.EntityID` |
| Other entities (PM, CRM, etc.) | All internal clients in workspace | Never (widget doesn't care) |

**Implementation approach — extend Hub, not replace it:**

```go
type Client struct {
    Conn           *websocket.Conn
    UserID         string
    WorkspaceID    string
    IsWidget       bool     // NEW: distinguishes widget from internal clients
    ConversationID *string  // NEW: set for widget clients, nil for internal clients
}

func (h *Hub) Broadcast(event Event) {
    h.mu.RLock()
    clients := h.clients[event.WorkspaceID]
    // copy target list
    h.mu.RUnlock()

    for client := range targets {
        if h.shouldReceive(client, event) {
            // send event
        }
    }
}

func (h *Hub) shouldReceive(client *Client, event Event) bool {
    // Internal clients (agents): receive everything in their workspace
    if !client.IsWidget {
        return true
    }
    // Widget clients: only receive conversation-scoped support events
    if event.Entity == "support_conversation_message" {
        return client.ConversationID != nil && *client.ConversationID == event.ParentID
    }
    if event.Entity == "support_conversation" {
        return client.ConversationID != nil && *client.ConversationID == event.EntityID
    }
    // Widget clients don't receive non-support events
    return false
}
```

---

## WebSocket Connection Design

### Design Principle: Match Crisp's Transport Split

Following Crisp's architecture, operations are split by what makes sense for each transport:

| Operation | Transport | Why |
|-----------|-----------|-----|
| Fetch widget config | **HTTP** `GET /widget/config` | Cacheable (CDN + localStorage), needed before WS connects |
| File uploads | **HTTP** `POST /widget/upload` | Binary multipart — not suited for WS |
| Revoke session | **HTTP** `POST /widget/session/revoke` | WS already closed during shutdown (HTTP fallback) |
| **Session create** | **WS** `session:create` | Matches Crisp — session created over WS after connect |
| **Session restore** | **WS** `session:restore` | Returning visitor restores session over existing connection |
| **Conversation history** | **WS** `conversations:list` | Requested after session:joined, returned over WS |
| **Send message** | **WS** `message:send` | Real-time, low-latency |
| **Receive message** | **WS** `message:received` | Real-time, pushed by server |
| **Typing indicators** | **WS** (both directions) | Ephemeral, real-time |
| **Session upgrade** | **WS** `session:upgrade` | Already connected |
| **Page update** | **WS** `page:update` | Already connected |
| **Session revoke** | **WS** `session:revoke` | Sent before closing WS (HTTP fallback if WS already dead) |

### Connection Flow

```
Client                                            Server
  |                                                  |
  |--- HTTP GET /widget/config?widget_key=KEY   ---->|  (cacheable, CDN + localStorage 10min)
  |<-- 200 { color, branding, pre_chat_form }   ----|
  |                                                  |
  |--- WS /widget/ws?key={widget_key}           ---->|  (unauthenticated — just widget_key)
  |                                                  |
  |--- WS: session:create { anonymous_id,          --->|  (new visitor — no stored session)
  |         page_url, page_title, ua, tz, locale }   |
  |  OR                                              |
  |--- WS: session:restore { session_token }     --->|  (returning visitor — token from localStorage)
  |                                                  |
  |<-- WS: session:joined { session_token,       ---|  (server creates/restores session in PG)
  |         expires_at, conversations[],             |
  |         messages[], is_anonymous }               |
  |                                                  |
  |  [Client persists session_token to localStorage] |
  |  [Render widget with config + session data]      |
  |                                                  |
  |--- WS: message:send { content }             ---->|  (visitor sends message)
  |<-- WS: message:received { id, content,       ---|  (echo with server-assigned ID)
  |         sender_type, sender_name, created_at }   |
  |                                                  |
  |<-- WS: message:received { ... sender:agent } ---|  (agent replies — hydrated payload)
  |                                                  |
  |--- WS: typing:start                         ---->|  (visitor typing)
  |<-- WS: typing:start { agent_name }           ---|  (agent typing)
  |                                                  |
  |--- WS: session:upgrade { email, name }       --->|  (anonymous → identified)
  |<-- WS: session:upgraded {}                   ----|
  |                                                  |
  |--- WS: page:update { url, title }           ---->|  (SPA navigation)
  |                                                  |
  |--- WS: conversations:list {}                 --->|  (request past conversations)
  |<-- WS: conversations:listed { conversations[] } -|
  |                                                  |
  |--- WS: session:revoke {}                     --->|  (on shutdown, before close)
  |<-- WS: session:revoked {}                    ----|
  |  [Client closes WS]                              |
  |                                                  |
```

### WS Message Types

#### Client → Server (widget sends)

| Type | Payload | When |
|------|---------|------|
| `session:create` | `{ anonymous_id, page_url, page_title, user_agent, timezone, locale }` | First visit (no stored session) |
| `session:restore` | `{ session_token }` | Returning visit (token in localStorage) |
| `session:upgrade` | `{ email, name }` | Visitor provides identity |
| `session:revoke` | `{}` | On shutdown (before closing WS) |
| `message:send` | `{ content, type?, attachments? }` | Visitor sends a chat message |
| `typing:start` | `{}` | Visitor starts typing (debounce: send once per 3s) |
| `typing:stop` | `{}` | Visitor stops typing (5s after last keystroke) |
| `page:update` | `{ url, title }` | SPA navigation / page change |
| `conversations:list` | `{}` | Request past conversations for this visitor |

#### Server → Client (widget receives)

| Type | Payload | When |
|------|---------|------|
| `session:joined` | `{ session_token, expires_at, is_anonymous, conversations[], messages[] }` | After session:create or session:restore |
| `session:error` | `{ code, message }` | Invalid widget_key, expired/revoked token, rate limit |
| `session:upgraded` | `{ email, name }` | Upgrade confirmed |
| `session:revoked` | `{}` | Revoke confirmed |
| `message:received` | `{ id, conversation_id, content, sender_type, sender_name, sender_avatar, created_at, attachments? }` | New message (from agent, bot, or echo of own with server-assigned ID) |
| `typing:start` | `{ agent_name }` | Agent is typing |
| `typing:stop` | `{}` | Agent stopped typing |
| `conversation:created` | `{ conversation_id }` | First message created a new conversation |
| `conversations:listed` | `{ conversations[] }` | Response to conversations:list |
| `connection:error` | `{ code, message }` | Server-side error |

### WS Authentication & Session Handshake

**Connection accepts with just `widget_key`** (unauthenticated). The first message MUST be `session:create` or `session:restore` within 10 seconds, or the server disconnects.

```go
// widget_handler.go — connection flow
func (h *WidgetWSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    widgetKey := r.URL.Query().Get("key")
    // 1. Validate widget_key exists (lightweight lookup)
    installation, err := h.service.GetInstallationByWidgetKey(ctx, widgetKey)
    if err != nil { /* reject WS upgrade */ return }

    // 2. Accept WS connection (no session yet — unauthenticated)
    conn, err := websocket.Accept(w, r, nil)

    // 3. Wait for first message: session:create or session:restore (10s timeout)
    firstMsgCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    _, data, err := conn.Read(firstMsgCtx)
    cancel()
    if err != nil { conn.Close(...); return } // timeout or read error

    var msg WidgetWSMessage
    json.Unmarshal(data, &msg)

    var session *model.SupportWidgetSession
    switch msg.Type {
    case "session:create":
        session, err = h.service.CreateWidgetSession(ctx, installation.WorkspaceID, msg.Data.AnonymousID, ...)
    case "session:restore":
        session, err = h.service.GetWidgetSession(ctx, msg.Data.SessionToken)
        if err != nil {
            // Token invalid/expired/revoked — tell client to recreate
            sendToClient(conn, "session:error", map[string]string{"code": "invalid_token", "message": "Session expired"})
            // Client will close WS, clear localStorage, reconnect with session:create
            return
        }
    default:
        conn.Close(...) // unexpected first message
        return
    }

    // 4. Send session:joined with full context
    conversations, _ := h.service.GetVisitorConversations(ctx, session)
    messages, _ := h.service.GetConversationMessages(ctx, session)
    sendToClient(conn, "session:joined", SessionJoinedPayload{
        SessionToken:  session.SessionToken,
        ExpiresAt:     session.ExpiresAt,
        IsAnonymous:   session.IsAnonymous,
        Conversations: conversations,
        Messages:      messages,
    })

    // 5. Register client and enter bidirectional message loop
    h.handleConnection(conn, session)
}
```

### WS Payload Hydration for Widget Clients

**The problem:** The current Hub broadcasts thin `Event` structs with IDs only (`entity_id`, `parent_id`). Internal frontend clients handle this fine — they use TanStack Query invalidation and refetch. But widget clients need the **full message content** to render immediately without an extra HTTP round-trip.

**Solution:** Two broadcast paths depending on client type:

```go
// Event struct gets an optional Data field for hydrated payloads
type Event struct {
    Action      string          `json:"action"`
    Entity      string          `json:"entity"`
    EntityID    string          `json:"entity_id"`
    WorkspaceID string          `json:"workspace_id"`
    ActorID     string          `json:"actor_id"`
    ParentType  string          `json:"parent_type,omitempty"`
    ParentID    string          `json:"parent_id,omitempty"`
    Data        json.RawMessage `json:"data,omitempty"` // NEW — hydrated payload for widget clients
}
```

**Internal clients (agents):** Receive thin event (existing behavior). `Data` field omitted. Frontend invalidates TanStack Query cache and refetches.

**Widget clients:** Receive hydrated event. The widget WS handler intercepts outgoing Hub events, looks up the full message from the service, and marshals it into a widget-specific `message:received` WS frame:
```json
{
  "type": "message:received",
  "data": {
    "id": "msg-uuid",
    "conversation_id": "conv-uuid",
    "content": "Hello, how can I help?",
    "sender_type": "agent",
    "sender_name": "Amad",
    "sender_avatar": null,
    "created_at": "2026-03-14T12:00:00Z"
  }
}
```

**Where hydration happens:** In the **handler layer** (not the service). The widget WS handler wraps the Hub's broadcast with a hydration step: when a thin event arrives for a widget client, the handler fetches the full entity from the service and sends a typed WS message. The service has no knowledge of WS framing or payload format.

### Widget WS Handler — Bidirectional Message Loop

The current widget WS handler is read-only (just keeps the connection alive). It needs to become a bidirectional message router:

```go
// widget_handler.go — bidirectional message loop (entered after session:joined)
func (h *WidgetWSHandler) handleConnection(conn *websocket.Conn, session *model.SupportWidgetSession) {
    client := &Client{
        Conn:           conn,
        UserID:         "widget:" + session.ID,
        WorkspaceID:    session.WorkspaceID,
        IsWidget:       true,
        ConversationID: session.ConversationID,
    }
    h.hub.Register(client)
    defer h.hub.Unregister(client)

    for {
        _, data, err := conn.Read(ctx)
        if err != nil { break }

        var msg WidgetWSMessage
        json.Unmarshal(data, &msg)

        switch msg.Type {
        case "message:send":
            // Delegate to service (handler stays in transport layer)
            result, err := h.service.WidgetCreateMessage(ctx, session.SessionToken, msg.Data.Content)
            if err != nil { sendError(conn, err); continue }
            // Update hub routing if new conversation was created
            if client.ConversationID == nil {
                convID := result.ConversationID
                h.hub.SetWidgetConversationByUserID(client.UserID, convID)
                sendToClient(conn, "conversation:created", map[string]string{"conversation_id": convID})
            }
            // Echo back to sender with server-assigned ID
            sendToClient(conn, "message:received", result)

        case "typing:start":
            h.hub.BroadcastTyping(session.WorkspaceID, session.CustomerName, true)

        case "typing:stop":
            h.hub.BroadcastTyping(session.WorkspaceID, session.CustomerName, false)

        case "session:upgrade":
            err := h.service.UpgradeWidgetSession(ctx, session.SessionToken, msg.Data.Email, msg.Data.Name)
            if err != nil { sendError(conn, err); continue }
            session.CustomerEmail = &msg.Data.Email
            session.CustomerName = &msg.Data.Name
            sendToClient(conn, "session:upgraded", nil)

        case "session:revoke":
            h.service.RevokeWidgetSession(ctx, session.SessionToken)
            sendToClient(conn, "session:revoked", nil)
            return // exit loop, connection will close

        case "page:update":
            // Update in-memory tracking (no DB write needed)

        case "conversations:list":
            convs, _ := h.service.GetVisitorConversations(ctx, session)
            sendToClient(conn, "conversations:listed", convs)
        }
    }
}
```

**Key architectural point:** The handler calls the service for business logic (message creation, session upgrade, revoke). The handler manages WS routing (hub registration, conversation_id updates, message framing). This preserves the handler→service→repository layering.

### Server-Side Token Recovery (Boot Fallback)

When a stored `session_token` is rejected during `session:restore`, the client must recover gracefully:

```
boot() → stored session_token found in localStorage
  → fetchWidgetConfig() via HTTP (cached)
  → connect WS with widget_key
  → WS send: session:restore { session_token }
  → WS recv: session:error { code: "invalid_token" }
  → clearSession(widget_key)  // remove invalid token from localStorage
  → WS send: session:create { anonymous_id, ... } (reuse same WS connection — no reconnect needed)
  → WS recv: session:joined { ... }
  → persist new session_token → continue normally
  → if session:create ALSO fails → close WS, show connection error UI
  → NEVER retry more than once (prevents infinite loop)
```

**Key insight:** Unlike HTTP-based recovery, WS-based recovery doesn't need to reconnect — the WS connection is already open (authenticated by widget_key, not session_token). The client just sends `session:create` as a follow-up on the same connection.

---

## Token Revocation & Shutdown Semantics

### The Problem

Current `shutdown()` only clears in-memory widget state. On shared devices (kiosks, shared computers), the next user could see the previous user's conversation.

### Shutdown Contract

When `shutdown()` is called:

**Client-side (immediate):**
1. If WS is open: send `session:revoke` over WS, then close WS
2. If WS is already closed: fire `POST /widget/session/revoke` via HTTP (best-effort fallback)
3. Unmount widget from DOM
4. Clear `helpin_ws_{widget_key}` from localStorage (session token)
5. Clear `helpin_wc_{widget_key}` from localStorage (config cache)
6. Do **NOT** clear `helpin_aid_{widget_key}` cookie (visitor identity persists)

**Server-side (on revoke call):**
1. Set `revoked_at = now()` on the session record
2. Any subsequent WS connection or HTTP call with this token returns 401
3. Session record retained for 7 days (audit trail), then cleaned up by cron

**When `identify()` is called with a different user:**
1. If active widget session exists with different email → call `shutdown()` first
2. Then `boot()` fresh with new identity
3. This prevents conversation leakage when switching users without explicit logout

### New Boot After Shutdown

```
shutdown() cleared localStorage → no stored session_token
boot() → reads anonymous_id from cookie (same browser)
  → fetchWidgetConfig() via HTTP (cached)
  → connect WS with widget_key
  → WS send: session:create { anonymous_id, ... }
  → WS recv: session:joined { new session_token, conversations[] }
  → New session, clean slate — but previous conversations listed via anonymous_id
```

---

## Agent Availability Model

### The Problem

The original PRD derived operator availability from active `/api/ws` connections. But `/api/ws` is used by **all** workspace members (PM, CRM, settings, etc.), not just support agents. A PM manager browsing sprint boards would incorrectly show as "available for support."

### Solution: Explicit Support Availability

**New field on workspace membership or user settings:**
```go
// In the support agent's workspace member record or a separate table
type SupportAvailability struct {
    WorkspaceID string
    UserID      string
    Status      string    // "available", "away", "offline"
    UpdatedAt   time.Time
}
```

**How it's set:**
- Agent clicks "Available for support" toggle in the support inbox UI
- Status stored in DB, broadcast to widget clients via WS event
- Auto-set to "away" after 15 min of no support inbox activity
- Auto-set to "offline" when agent's `/api/ws` connection drops

**What the widget receives:**
```json
{
    "users_available": true,
    "active_operators": [
        { "nickname": "Amad", "avatar": null, "status": "available" },
        { "nickname": "Arooj", "avatar": "https://...", "status": "available" }
    ],
    "response_metrics": { "count": 47, "mean_seconds": 300 }
}
```

**Response metrics computation:**
- New nullable column on `support_conversations`: `first_reply_at *time.Time`
- Set when the first `sender_type=agent` message is created in a conversation
- Metric query: `SELECT COUNT(*), AVG(EXTRACT(EPOCH FROM (first_reply_at - created_at))) FROM support_conversations WHERE first_reply_at IS NOT NULL AND workspace_id = ?`
- Cached in-memory per workspace, recomputed every 5 minutes

**Phase 1 scope:** We do NOT implement the full availability system in Phase 1. Phase 1 focuses on the core session/WS/security fixes. Availability is a Phase 3 feature.

---

## Events Pipeline Integration

### Current State: Two Separate Systems

```
┌─────────────────────────────────────────────────────────────┐
│  Analytics Events Pipeline                                  │
│                                                             │
│  SDK (HelpinClient)                                         │
│    → pageview, $pageleave, user_identify, track()           │
│    → POST /api/v1/event → Rust Capture API (:3000)          │
│    → Enrichment (geo, UA, bot detect)                       │
│    → Kafka (incoming_events topic)                          │
│    → KStreams sessionization (30-min inactivity window)      │
│    → ClickHouse (sessionized events)                        │
│                                                             │
│  Session key: SHA-256(timestamp + "project_id:anonymous_id")│
│  Session gap: 30 minutes of inactivity                      │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  Widget Chat Pipeline (COMPLETELY SEPARATE)                 │
│                                                             │
│  SDK (WidgetManager)                                        │
│    → POST /widget/session → session_token (24h)             │
│    → WS /widget/ws → messages                               │
│    → Go API server → PostgreSQL                             │
│                                                             │
│  Session key: random 32-byte token                          │
│  Session TTL: 24 hours (soon 30 days)                       │
└─────────────────────────────────────────────────────────────┘

Shared identity: anonymous_id cookie (but different keys and no cross-referencing)
```

### Problem

- A visitor's pageviews and their chat conversations **can't be correlated** in analytics
- Widget interactions (opened, message sent, rating given) are **invisible** to the events pipeline
- Support agents can't see "this visitor viewed pricing page 3 times before opening chat"

### Solution: Widget Emits Analytics Events via Explicit Interface

**The interface problem:** Today `WidgetManager` has no reference to `HelpinClient`, and `getOrCreateAnonymousId()` is private. We need an explicit boundary.

**New: `AnonymousIdentity` interface**

```typescript
// packages/sdk-js/src/core/identity.ts — NEW FILE
// Shared identity utilities used by both HelpinClient and WidgetManager

export interface AnonymousIdentity {
  getOrCreateAnonymousId(widgetKey: string): string;
  clearSession(widgetKey: string): void;
}

// Cookie-based implementation
export function getOrCreateAnonymousId(widgetKey: string): string {
  const cookieName = `helpin_aid_${widgetKey}`;
  const existing = CookieManager.get(cookieName);
  if (existing) return existing;

  // Check URL params for cross-domain linking (_hp parameter)
  const urlId = new URLSearchParams(window.location.search).get('_hp');
  const id = urlId || crypto.randomUUID();

  CookieManager.set(cookieName, id, 3650); // 10 years
  return id;
}
```

**HelpinClient changes:**
- Refactor `getOrCreateAnonymousId()` to call `getOrCreateAnonymousId(this.apiKey)` internally
- Maintain backwards compatibility: existing analytics events still flow with the same identity

**WidgetManager changes:**
- Import `getOrCreateAnonymousId` from shared identity module
- On boot: `this.anonymousId = getOrCreateAnonymousId(this.widgetKey)`
- Pass `anonymous_id` to `POST /widget/session`
- Optionally emit analytics events via `HelpinClient.track()` if client is available

**Widget events to emit (when HelpinClient is available):**

| Event Type | When | Key Properties |
|-----------|------|----------------|
| `widget:opened` | User clicks bubble | `widget_key` |
| `widget:closed` | User closes widget | `duration_open_ms` |
| `chat:started` | First message in new conversation | `conversation_id`, `is_anonymous` |
| `chat:message_sent` | User sends any message | `conversation_id` |
| `chat:rated` | User submits rating | `conversation_id`, `rating` |
| `chat:upgraded` | Anonymous → identified | — |

**How the bridge works:**

Today `index.ts` holds both `analyticsClient` (HelpinClient) and `widgetManager` (WidgetManager) as module-scoped singletons, but they don't reference each other. The fix:

```typescript
// In index.ts — pass analytics client reference to widget on boot
if (analyticsClient) {
  widgetManager.setAnalyticsClient(analyticsClient);
}

// In WidgetManager
private analyticsClient?: HelpinClient;

setAnalyticsClient(client: HelpinClient) {
  this.analyticsClient = client;
}

private emitAnalyticsEvent(eventType: string, properties: Record<string, any>) {
  // HelpinClient may or may not be injected (widget can work standalone)
  this.analyticsClient?.track(eventType, properties);
  // If no analytics client, events simply aren't tracked — widget still works fine
}
```

**Why this works:**
- Explicit dependency injection (not magic globals) — `index.ts` wires the two singletons together
- Same `anonymous_id` in both systems (shared cookie via `identity.ts`) → events get sessionized together by KStreams
- Analytics events are fire-and-forget — widget chat works independently even if analytics pipeline is down
- No chat message content goes through analytics (privacy) — only metadata events
- Widget still works standalone (e.g., embedded without analytics) — `analyticsClient` is optional

---

## Implementation Plan

### Phase 1: Core Infrastructure (MUST HAVE)

#### 1A. Bot/Crawler Filtering

**New file: `packages/sdk-js/src/utils/bot-detect.ts`**
- UA blocklist: Googlebot, Bingbot, HeadlessChrome, Puppeteer, Selenium, PhantomJS, etc.
- Export `isBot(): boolean`

**Modify: `packages/sdk-js/src/core/widget.ts` → `boot()`**
- Early-return if `isBot()` is true

#### 1B. Visitor Identity & Cookie Rename

**New file: `packages/sdk-js/src/core/identity.ts`**
- `getOrCreateAnonymousId(widgetKey)` — reads/writes `helpin_aid_{widget_key}` cookie
- `getStoredSession(widgetKey)` — reads `helpin_ws_{widget_key}` from localStorage
- `persistSession(widgetKey, token, expiresAt)` — writes to localStorage
- `clearSession(widgetKey)` — removes from localStorage
- `clearConfigCache(widgetKey)` — removes `helpin_wc_{widget_key}`

**Modify: `packages/sdk-js/src/core/client.ts`**
- Refactor `getOrCreateAnonymousId()` to use `getOrCreateAnonymousId()` internally
- Add migration: on first load, if `__eventn_id_{apiKey}` cookie exists, copy value to `helpin_aid_{widget_key}` and delete the old cookie
- Update `helpin_{apiKey}_data` localStorage key to `helpin_analytics_{widget_key}` (with migration)

**Modify: `packages/sdk-js/src/core/widget.ts`**
- Import and use `getOrCreateAnonymousId()` from identity module
- Remove `getAnonymousId()` that reads `helpin_anonymous_id` (replaced by cookie-based anonymous_id)

#### 1C. Anonymous Sessions + Session Persistence

**Modify: `packages/sdk-js/src/core/widget.ts` — new `boot()` flow:**
```
boot({ key, host })
  → isBot()? return early
  → anonymousId = getOrCreateAnonymousId(key)
  → storedSession = getStoredSession(key)
  → fetchWidgetConfig() via HTTP [cache in localStorage helpin_wc_{key}, 10min TTL]
  → connectWebSocket(key)  ← connects with just widget_key, unauthenticated
  → if storedSession AND storedSession.expires_at > now:
      → WS send: session:restore { session_token }
      → WS recv: session:joined { session_token, conversations[], messages[] }
          → on session:error (invalid/revoked token):
              → clearSession(key)
              → WS send: session:create { anonymous_id, ... } (reuse same WS connection)
              → WS recv: session:joined { ... }
              → if ALSO fails: disconnect, show error UI
      → persistSession(key, session_token, expires_at)
  → else:
      → WS send: session:create { anonymous_id, page_url, page_title, ua, tz, locale }
      → WS recv: session:joined { session_token, expires_at, conversations[], messages[] }
      → persistSession(key, session_token, expires_at)
  → render widget with config + session data (conversations, messages)
  → all chat via WS from here (message:send, typing:*, session:upgrade)
```

**Modify: `handlePreChatSubmit(data)` — upgrade via WS:**
```
handlePreChatSubmit({ email, name })
  → WS send: session:upgrade { email, name }
  → WS recv: session:upgraded
  → session is now identified (is_anonymous=false)
  → conversation continues in same thread
```

**Modify: `handleSendMessage(content)` — send via WS:**
```
handleSendMessage(content)
  → optimistic update: add message to local state with temp ID
  → WS send: message:send { content }
  → WS recv: message:received { id, conversation_id, ... } (echo with server-assigned ID)
  → replace temp ID with server ID in local state
  → if first message: also receive conversation:created { conversation_id }
  → if WS send fails: show error, allow retry
```

**Backend model changes — `server/internal/model/support_inbox.go`:**
```go
type SupportWidgetSession struct {
    ID             string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID    string     `json:"workspace_id" gorm:"type:uuid;not null;index"`
    ConversationID *string    `json:"conversation_id" gorm:"type:uuid;index"`
    SessionToken   string     `json:"-" gorm:"uniqueIndex;not null"` // never sent to client except on create
    AnonymousID    string     `json:"anonymous_id" gorm:"not null"`    // NEW — same UUID as user_anonymous_id in events pipeline
    IsAnonymous    bool       `json:"is_anonymous" gorm:"default:true"` // NEW
    CustomerName   *string    `json:"customer_name"`
    CustomerEmail  *string    `json:"customer_email"`
    UserAgent      *string    `json:"-"`                              // NEW, not sent to client
    LastPageURL    *string    `json:"last_page_url"`                  // NEW
    RevokedAt      *time.Time `json:"-" gorm:"index"`                // NEW — for shutdown/revoke
    ExpiresAt      time.Time  `json:"expires_at"`                     // changed: 30 days instead of 24h
    CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
}
```

**Backend model changes — `server/internal/model/support_inbox.go` (SupportConversation):**

Add `anonymous_id` to the existing `SupportConversation` model:
```go
// Added to existing SupportConversation struct
AnonymousID *string `json:"anonymous_id" gorm:"index"` // NEW — set when conversation created from widget
```

This enables direct conversation history queries by visitor without joining through sessions.

**Backend service changes — `server/internal/service/support_inbox.go`:**

`CreateWidgetSession(ctx, workspaceID, visitorID, ua, pageURL)`:
1. Generate 32-byte secure token
2. Create session with `is_anonymous=true`, `expires_at = now + 30d`, `anonymous_id`, `user_agent`, `last_page_url`
3. No upsert, no race conditions — multiple concurrent sessions per visitor are allowed
4. Returns session record (used by WS handler to build `session:joined` response)

`GetVisitorConversations(ctx, session)`:
1. Uses session's `workspace_id` + `anonymous_id`
2. Query: `SELECT * FROM support_conversations WHERE workspace_id = ? AND anonymous_id = ? ORDER BY updated_at DESC`
3. Returns conversation list (id, subject/preview, last message timestamp, unread count)

`UpgradeWidgetSession(ctx, sessionToken, email, name)`:
1. Validate session (not expired, not revoked)
2. Update: `customer_email = email`, `customer_name = name`, `is_anonymous = false`
3. If conversation exists → update conversation contact info
4. Bridge to CRM: auto-match/create contact by email

`RevokeWidgetSession(ctx, sessionToken)`:
1. Set `revoked_at = now()`
2. Any subsequent calls with this token return 401

`WidgetCreateMessage` (existing, modify):
- When creating a new `SupportConversation`, set `anonymous_id` from the session's `anonymous_id`
- This is the only place conversations are created for widget visitors

**Backend handler changes — `server/internal/handler/support_inbox_widget.go`:**

Session create, restore, upgrade, conversations list, and revoke are ALL handled over WebSocket now (see "Widget WS Handler" section). The HTTP handler retains only:

Keep `GET /widget/config?widget_key=KEY`:
- Returns widget configuration (color, branding, pre-chat form settings)
- Cacheable — should set `Cache-Control` headers for CDN/browser caching

Keep `POST /widget/upload` (Phase 3 — file attachments):
- Multipart file upload — binary data not suited for WS
- Requires valid `session_token` in header/body

New `POST /widget/session/revoke` (HTTP fallback only):
- Used when WS is already closed during shutdown
- Request: `{ session_token }`
- Response: 204 No Content

Remove `POST /widget/session` (moved to WS `session:create`):
- Session creation now happens over WebSocket in widget_handler.go

Remove `POST /widget/messages` (moved to WS `message:send`):
- Message sending now happens over WebSocket

Keep `GET /widget/messages?session_token=TOKEN` (backwards compat, may remove later):
- Still useful for initial message fetch if session:joined payload is too large

**New migration: `server/migrations/0XX_widget_session_anonymous.sql`:**
```sql
-- Widget sessions: add visitor identity, anonymous flag, metadata, revocation
ALTER TABLE support_widget_sessions
  ADD COLUMN IF NOT EXISTS anonymous_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS is_anonymous BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS user_agent TEXT,
  ADD COLUMN IF NOT EXISTS last_page_url TEXT,
  ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- Index for session cleanup cron
CREATE INDEX IF NOT EXISTS idx_widget_sessions_revoked
  ON support_widget_sessions(revoked_at)
  WHERE revoked_at IS NOT NULL;

-- Conversations: add visitor ownership for direct history queries
ALTER TABLE support_conversations
  ADD COLUMN IF NOT EXISTS anonymous_id TEXT;

CREATE INDEX IF NOT EXISTS idx_conversations_visitor_workspace
  ON support_conversations(workspace_id, anonymous_id)
  WHERE anonymous_id IS NOT NULL;
```

#### 1D. Conversation-Scoped WebSocket Routing (SECURITY FIX)

**Modify: `server/internal/websocket/hub.go`**

Extend `Client` struct:
```go
type Client struct {
    Conn           *websocket.Conn
    UserID         string
    WorkspaceID    string
    IsWidget       bool     // NEW
    ConversationID *string  // NEW — set for widget clients
}
```

Modify `Broadcast()` to filter widget clients by conversation:
```go
func (h *Hub) shouldReceive(client *Client, event Event) bool {
    if !client.IsWidget {
        return true // internal clients see everything in their workspace
    }
    // Widget clients only see their own conversation's events
    switch event.Entity {
    case "support_conversation_message":
        return client.ConversationID != nil && *client.ConversationID == event.ParentID
    case "support_conversation":
        return client.ConversationID != nil && *client.ConversationID == event.EntityID
    default:
        return false // widget doesn't need PM/CRM/other events
    }
}
```

Add method to update a widget client's conversation_id (keyed by UserID, not client pointer):
```go
func (h *Hub) SetWidgetConversationByUserID(userID string, conversationID string) {
    h.mu.Lock()
    defer h.mu.Unlock()
    for _, clients := range h.clients {
        for client := range clients {
            if client.UserID == userID {
                client.ConversationID = &conversationID
            }
        }
    }
}
```

**Modify: `server/internal/websocket/widget_handler.go`**
- Set `client.IsWidget = true` on registration
- Set `client.ConversationID` from session's `conversation_id` (if session already has one)
- **Bidirectional message loop**: handle `message:send`, `typing:*`, `session:upgrade`, `page:update` (see "Widget WS Handler" section above)
- After `service.WidgetCreateMessage()` returns a new conversation_id, the **handler** (not the service) calls `hub.SetWidgetConversationByUserID()` — this keeps transport concerns in the handler layer

**Service layer stays clean — no WS/Hub references:**
- `WidgetCreateMessage()` creates conversation + message, returns the result
- `UpgradeWidgetSession()` updates session fields, returns success/error
- The service has no knowledge of WebSocket, Hub, or Client structs

#### 1E. Shutdown & Token Revocation

**Modify: `packages/sdk-js/src/core/widget.ts` — `shutdown()`:**
```typescript
shutdown() {
  this.isShutdown = true;

  // 1. Revoke session — prefer WS, fall back to HTTP
  if (this.ws?.readyState === WebSocket.OPEN) {
    // WS is open — revoke over WS, then close
    this.wsSend('session:revoke', {});
    this.disconnectWebSocket();
  } else {
    // WS already closed — HTTP fallback (fire-and-forget)
    if (this.sessionToken && this.host) {
      const url = this.host.startsWith('http') ? this.host : `https://${this.host}`;
      fetch(`${url}/widget/session/revoke`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_token: this.sessionToken }),
      }).catch(() => {}); // best-effort
    }
  }

  // 2. Clear persisted session (but NOT anonymous_id cookie)
  clearSession(this.widgetKey);
  clearConfigCache(this.widgetKey);

  // 3. Reset in-memory state + unmount widget
  this.cleanup();
}
```

**Modify: `packages/sdk-js/src/index.ts` — `identify()` guard:**
```typescript
// If widget is active and identify() is called with a different user email,
// shutdown the existing widget session first to prevent conversation leakage
function identify(userData: UserData) {
  if (widgetManager?.isActive() && widgetManager?.currentEmail !== userData.email) {
    widgetManager.shutdown();
    // Widget will re-initialize on next show() or boot()
  }
  helpinClient.id(userData);
}
```

#### 1F. Connection Failure UI

**Modify: `packages/widget-core/src/components/ChatWindow.tsx` (or equivalent)**
- Accept new prop: `connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'failed'`
- When `connectionStatus === 'failed'` (after max retries exhausted):
  - Show banner at top of chat: "Unable to connect. Support may be unavailable."
  - Show "Retry" button that calls `reconnectWebSocket()`
- When `connectionStatus === 'connecting'`:
  - Show subtle "Connecting..." indicator

**Modify: `packages/sdk-js/src/core/widget.ts`**
- Track connection status state
- Pass `connectionStatus` to widget-core component via props/callbacks
- After max retries (10) with exponential backoff: set status to `'failed'`

#### 1G. Attribute & Storage Key Migration

Since we have **no production customers**, all naming changes happen in one cut — no dual-read compatibility layer.

**Script attributes (breaking change, single release):**
```
data-key            → data-widget-key
data-tracking-host  → data-host
```

**Modify: `packages/sdk-js/src/core/widget.ts`**
- `boot()` reads `data-widget-key` and `data-host` from script tag
- Remove support for old `data-key` and `data-tracking-host` attributes

**Modify: `packages/sdk-js/src/index.ts`**
- Update auto-boot detection to look for `data-widget-key`
- `boot({ key, host })` API stays the same (internal property names don't change)

**Modify: `frontend/src/components/settings/ChatGeneralTab.tsx`**
- Update embed snippet to use `data-widget-key` and `data-host`
- Keep file as `lib.js`

**Cookie/localStorage migration (automatic, one-time on first load):**
```typescript
// In identity.ts — called once during getOrCreateAnonymousId()
function migrateFromLegacyKeys(widgetKey: string): void {
  // Cookie: __eventn_id_{widgetKey} → helpin_aid_{widgetKey}
  const oldCookie = CookieManager.get(`__eventn_id_${widgetKey}`);
  if (oldCookie) {
    CookieManager.set(`helpin_aid_${widgetKey}`, oldCookie, 3650);
    CookieManager.delete(`__eventn_id_${widgetKey}`);
  }

  // localStorage: helpin_anonymous_id → removed (replaced by cookie)
  const oldVisitorId = localStorage.getItem('helpin_anonymous_id');
  if (oldVisitorId) {
    // If no cookie exists yet, use this value
    if (!CookieManager.get(`helpin_aid_${widgetKey}`)) {
      CookieManager.set(`helpin_aid_${widgetKey}`, oldVisitorId, 3650);
    }
    localStorage.removeItem('helpin_anonymous_id');
  }

  // localStorage: helpin_{widgetKey}_data → helpin_analytics_{widgetKey}
  const oldAnalytics = localStorage.getItem(`helpin_${widgetKey}_data`);
  if (oldAnalytics) {
    localStorage.setItem(`helpin_analytics_${widgetKey}`, oldAnalytics);
    localStorage.removeItem(`helpin_${widgetKey}_data`);
  }
}
```

#### 1H. CSS Isolation via Shadow DOM

**Modify: `packages/sdk-js/src/core/widget.ts` → `ensureWidget()`**
- Create host `<div id="helpin-widget-container">` with `position:fixed; z-index:2147483647`
- `host.attachShadow({ mode: 'open' })`
- Inject `<style>` inside shadow root (not `document.head`)
- Mount Preact container inside shadow root
- Remove current `injectStyles()` that adds `<style>` to `document.head`

**Modify: `packages/widget-core/src/styles/widget.css`**
- Add shadow DOM reset at top:
```css
:host { all: initial; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
```

---

### Phase 2: Two-Phase Loader + Hashing + Preconnect (SHOULD HAVE)

No backwards compat needed — no existing customers.

#### 2A. Loader Script

**New file: `packages/sdk-js/src/loader.ts`** (~3-5KB)
- Parse `data-widget-key`, `data-host` from script tag
- Inline bot check (minimal)
- Inject `<link rel="preconnect" href="https://{host}">` for API/WS domain
- Queue API calls: `window[ns + 'Q'] = []; window[ns] = function() { ... push to queue }`
- Async-load main bundle: `<script src="https://{host}/sdk/helpin.{hash}.js" async>`
- The `lib.js` path stays the same for users — it just becomes the tiny loader

#### 2B. Build Changes

**Modify: `packages/sdk-js/vite.config.ts`**
- Two entry points: `loader.ts` → `dist/lib.js`, `index.ts` → `dist/helpin.[hash].js`
- CSS extraction: remove `?inline` import, output `dist/helpin.[hash].css`
- Generate build manifest (`manifest.json`) mapping entry names to hashed filenames

#### 2C. Backend/CDN Asset Serving

- Serve `lib.js` (loader, same path always)
- Serve `helpin.[hash].js` and `helpin.[hash].css` (content-hashed, long cache)
- Could be static file server endpoint or S3+CDN

---

### Phase 3: Widget Features (SHOULD HAVE)

Items to build out after infrastructure is solid:

1. **Conversations list** — UI to show past conversations via `GET /widget/conversations` (anonymous_id query), navigate between them (sends `page:update` or new WS message type to switch active `conversation_id` on hub)
2. **Helpdesk/Articles** — search and display knowledge base articles in widget
3. **Agent availability** — explicit support availability toggle, response time metrics (requires `first_reply_at` column, `SupportAvailability` table)
4. **Emoji picker** — categorized emoji selection in compose bar
5. **Read receipts** — show message delivery/read status
6. **Rich message types** — support for buttons, carousels, cards from bot/agents
7. **Chat feedback/rating** — post-conversation rating with optional comment
8. **Proactive messages** — tooltip entice messages when widget is minimized
9. **In-chat email collection** — inline "What is your email?" prompt (not just pre-chat form)
10. **Ready trigger event** — `onReady` callback when widget fully initialized + connected
11. **i18n/Locale** — externalize strings, support multiple languages
12. **Browser compat checks** — graceful degradation for old browsers
13. **Cookie domain/expiry config** — `data-cookie-domain`, `data-cookie-expire` attributes

---

## Critical Files

| File | Changes |
|------|---------|
| **SDK (client-side)** | |
| `packages/sdk-js/src/core/identity.ts` | **New:** shared `getOrCreateAnonymousId()`, session persistence utilities, cookie/localStorage migration from legacy keys |
| `packages/sdk-js/src/core/widget.ts` | New boot flow (anonymous session + 401 recovery), `shutdown()` with token revocation + localStorage clear, bidirectional WS message loop (`message:send`, `typing:*`, `session:upgrade`), connection status tracking, `data-widget-key`/`data-host` attribute reading |
| `packages/sdk-js/src/core/client.ts` | Refactor `getOrCreateAnonymousId()` to use shared identity module |
| `packages/sdk-js/src/utils/bot-detect.ts` | **New:** `isBot()` — UA blocklist + headless browser detection |
| `packages/sdk-js/src/utils/cookie.ts` | Existing CookieManager — reuse as-is |
| `packages/sdk-js/src/index.ts` | `identify()` guard: shutdown widget if user changes, wire `analyticsClient` into `widgetManager`, update auto-boot to `data-widget-key` |
| `packages/sdk-js/src/loader.ts` | **New (Phase 2):** lightweight two-phase loader |
| `packages/sdk-js/vite.config.ts` | Phase 2: multi-entry build, CSS extraction, manifest |
| **Widget UI** | |
| `packages/widget-core/src/styles/widget.css` | Shadow DOM CSS reset (`:host { all: initial }`) |
| `packages/widget-core/src/components/ChatWindow.tsx` | Connection failure banner + retry button |
| **Backend (Go)** | |
| `server/internal/model/support_inbox.go` | Add `AnonymousID`, `IsAnonymous`, `UserAgent`, `LastPageURL`, `RevokedAt` to `SupportWidgetSession`. Add `AnonymousID` to `SupportConversation`. |
| `server/internal/service/support_inbox.go` | Modify `CreateWidgetSession` (always create new, no upsert), `WidgetCreateMessage` (set `anonymous_id` on conversation), new `UpgradeWidgetSession()`, `RevokeWidgetSession()`, `GetVisitorConversations()` |
| `server/internal/handler/support_inbox_widget.go` | Remove `POST /widget/session` and `POST /widget/messages` (moved to WS). Keep config endpoint. Add `POST /widget/session/revoke` (HTTP fallback). |
| `server/internal/websocket/hub.go` | Add `IsWidget`, `ConversationID` to `Client`, `Data json.RawMessage` to `Event`, conversation-scoped `shouldReceive()` filtering, `SetWidgetConversationByUserID()` |
| `server/internal/websocket/widget_handler.go` | WS-first: accept with `?key=` (unauthenticated), session:create/restore handshake, bidirectional message loop (`message:send`, `typing:*`, `session:upgrade`, `session:revoke`, `conversations:list`), hydrate `Event.Data` for widget clients |
| `server/internal/router/router.go` | Update widget WS path, add revoke HTTP route, remove old session/message HTTP routes |
| `server/migrations/0XX_widget_session_anonymous.sql` | Sessions: anonymous_id, is_anonymous, user_agent, last_page_url, revoked_at. Conversations: anonymous_id + index. |
| `frontend/src/components/settings/ChatGeneralTab.tsx` | Update embed snippet: `data-widget-key`, `data-host` |

---

## Verification

1. **Anonymous boot (WS-first)**: Load page with just `data-widget-key` — verify: HTTP config fetch, then WS connects with `?key=`, then `session:create` sent, then `session:joined` received with `is_anonymous=true`, no HTTP session/message calls
2. **Session:joined payload**: Verify `session:joined` includes `session_token`, `expires_at`, `conversations[]`, `messages[]` — widget renders immediately from this payload
3. **Message send via WS**: Type message → verify `message:send` sent over WS (not HTTP POST), `message:received` echo back with server-assigned ID
4. **Message receive via WS**: Agent replies in dashboard → verify widget receives hydrated `message:received` with full content (not thin event), renders immediately
5. **Session persistence**: Open widget, send message, refresh page — verify `session:restore` sent over WS on reload, `session:joined` returns conversation history
6. **Returning visitor**: Close tab, reopen hours later — verify `session:restore` works, conversations listed via `anonymous_id`
7. **Token recovery**: Revoke a session server-side → reload page → verify `session:restore` gets `session:error`, widget auto-sends `session:create` on same WS connection (no reconnect), new session created
8. **Conversation isolation**: Open widget in two different browsers for same workspace — verify each only sees their own messages via conversation-scoped routing
9. **Conversation ownership**: Verify `support_conversations.anonymous_id` is set when conversation created from widget, and `conversations:list` returns all conversations for that visitor
10. **Multiple tabs**: Open widget in two tabs — verify both get separate sessions, both can send/receive independently
11. **Session upgrade via WS**: Open widget anonymously, submit email via pre-chat → verify `session:upgrade` sent over WS, `session:upgraded` received, conversation continues
12. **Typing indicators**: Type in widget → agent sees typing. Agent types → widget sees `typing:start { agent_name }`. Both via WS.
13. **Shutdown via WS revoke**: Call `shutdown()` with WS open → verify `session:revoke` sent over WS, then WS closed, localStorage cleared
14. **Shutdown HTTP fallback**: Call `shutdown()` with WS already closed → verify HTTP `POST /widget/session/revoke` fired as fallback
15. **Shutdown on identify change**: Call `identify({email: 'new@user.com'})` while widget active with different email → verify old session shut down
16. **10s handshake timeout**: Connect WS but don't send `session:create/restore` → verify server disconnects after 10 seconds
17. **Bot filtering**: Set UA to "Googlebot" — verify widget does not load
18. **CSS isolation**: Add `* { color: red !important; }` to host page — verify widget unaffected inside Shadow DOM
19. **Connection failure**: Kill server → verify "Unable to connect" banner appears after retries, "Retry" button works
20. **Attribute migration**: Embed snippet uses `data-widget-key` and `data-host` → verify widget boots correctly
21. **Cookie migration**: Site with old `__eventn_id_{key}` cookie → verify value migrated to `helpin_aid_{key}`, old cookie deleted
22. **Existing tests pass**: `cd packages/sdk-js && pnpm test` and `cd packages/widget-core && pnpm test`
