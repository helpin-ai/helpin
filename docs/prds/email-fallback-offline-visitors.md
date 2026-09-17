# Email fallback for offline visitors

**Status**: Implemented; unread/offline delivery amendments planned
**Date**: 2026-03-20
**Updated**: 2026-04-25
**Author**: Engineering

## 1. Problem Statement

When a visitor sends a message via the Helpin chat widget and closes their browser before receiving a response, the agent's reply is saved to the database but never delivered. The visitor has no way to know they received an answer unless they return to the website and reopen the widget.

**Impact:**
- **Lost engagement**: Visitors who asked questions never see the answers
- **Wasted agent effort**: Agents spend time crafting replies that are never read
- **Broken AI value**: AI can respond in under 3 seconds, but if the visitor already closed the page, that speed is meaningless
- **No continuity**: Chat is a dead end when the visitor leaves

**What happens today**: `CreateConversationMessage()` in `server/internal/service/support_inbox.go` creates the message, broadcasts via WebSocket, and queues a fallback email candidate. The queue currently checks visitor online status at fire time, but it does not fully model "unread after the visitor leaves." If the visitor is online but does not read the reply, the queue can be cleaned up too early. If the visitor has already read the reply in the widget, the fallback can still send later if they disconnect before the delay fires. Email delivery should be a fallback for unread replies, not merely a fallback for disconnected sockets.

### 1.1 2026-04-25 Amendment: Unread, Offline, and Delivery Visibility

The current email fallback implementation needs three product refinements:

1. **Unread-gated customer delivery.** A fallback email should send only when the customer has not read the outbound reply in the widget. The source of truth is `support_conversations.contact_last_seen_at`.
2. **Freshness cap.** If the customer stays online but does not read the reply for too long, the system should not send a surprising email much later when they finally disconnect. Default cap: 10 minutes per eligible unread outbound reply.
3. **Agent-visible email delivery status.** When a support agent replies while the customer is offline and the message is delivered by email, the agent should see that state in the message thread, e.g. `Delivered via email`, then `Read via email` if Postmark open tracking confirms it, and delivery failure states if the email bounces or is marked spam.

## 2. Design Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Where does debounce/batching run? | **Redis-backed outbox + single leader worker** | The WebSocket stack already runs multi-pod with `RedisRelay` for cross-pod broadcast and `RedisPresence` for shared visitor state (`hub.go`). Pod-local timers would cause duplicate emails (two pods both schedule for the same conversation) or stale presence reads. A Redis sorted-set outbox (`ZADD` keyed by fire-at timestamp) with a lease-based poller (one pod holds a `SET NX EX` lock) gives durable, de-duplicated scheduling that survives pod restarts. |
| Extend Postmark client or new service? | **New `EmailFallbackService`** consuming existing `email.Client`. Add `SendEmailWithHeaders()` to `email.Client`. | Keeps Postmark client generic. Service owns outbox, templates, offline detection. |
| How to detect "visitor is offline"? | **`hub.Presence.GetOnlineVisitors(ctx, workspaceID)`** (the `PresenceProvider` interface) checked when outbox entry fires, not at message creation. | The hub already exposes a pluggable `PresenceProvider` (in-memory or `RedisPresence`) that tracks connections across all pods. Using `hub.IsVisitorOnline()` alone reads only local-pod state — `Presence.GetOnlineVisitors()` reads the shared Redis set. Checking at fire time (not enqueue time) avoids unnecessary emails if the visitor reconnects during the delay. |
| How to detect "visitor has not read the reply"? | **`support_conversations.contact_last_seen_at`** compared against queued outbound message `created_at`. | The widget already marks selected/visible conversations read through `conversation:read`. Email fallback must use the same customer read cursor so email is only sent for messages the customer has not actually seen. |
| What happens if the visitor is online but has not read? | **Postpone, do not cleanup.** Retry shortly while the reply remains fresh. | A connected socket does not mean the customer saw the message. They may be on another tab, the widget may be closed, or the page may be unfocused. Cleaning up loses the email fallback permanently. |
| How long can postponed unread fallback remain eligible? | **10 minutes per eligible unread outbound reply.** | After that point, sending that reply by email when the visitor finally disconnects feels abrupt. A new agent reply later starts its own fallback window without reviving stale older replies. |
| Inbound email routing? | **Postmark Inbound Webhook** to `POST /api/webhooks/postmark/inbound`. Parse `MailboxHash` from `conv-{id}@replies.helpin.ai`. | Postmark handles MIME, attachments, signature stripping. No custom SMTP server needed. |
| Store raw inbound email? | **Yes**, in `support_email_logs` table. | Audit trail, debugging, future attachment support. |
| Email threading? | RFC 2822 headers: `Message-ID`, `In-Reply-To`, `References`. Thread IDs stored on `support_email_logs`, notification state tracked per-message via `email_notified_at`. | Gmail/Outlook thread emails correctly per conversation. See §5 Data Model for separation of outbound delivery IDs, inbound provider IDs, and per-message notification state. |
| Settings storage? | **Extend existing `SupportInboxSettings` JSONB**. No new table. | Same pattern as AI settings, business hours, branding. |
| Webhook authentication? | **Basic auth** in webhook URL + validate `MailboxHash` format. | Simple, effective. Postmark documents their IP ranges. |

## 3. User Stories

**Visitor:**
- US-1: As a visitor, when I ask a question and close the page, I receive an email with the agent's reply so I can read it without returning to the website.
- US-2: As a visitor, I can reply to the notification email and my response appears in the same conversation thread.
- US-3: As a visitor, when I return to the widget, I see both chat and email messages in a unified thread with "Via email" badges.
- US-4: As a visitor, if I come back online before the email is sent, I do NOT receive an unnecessary email.
- US-5: As a visitor, if multiple agents reply quickly, I receive one batched email instead of many separate ones.
- US-16: As a visitor, if I already read the reply in the widget, I do NOT receive a duplicate fallback email later.
- US-17: As a visitor, if I am technically online but never read the reply, I receive the fallback email only if I go offline within the freshness window.
- US-18: As a visitor, I do NOT receive an abrupt fallback email long after the reply was sent just because I finally closed the page.

**Agent:**
- US-6: As an agent, when I reply to an offline visitor with an email on file, I see a subtle indicator that an email notification will be sent.
- US-7: As an agent, I see visitor email replies in the same conversation thread with a "Via email" badge.
- US-8: As an agent, I see the full conversation history regardless of whether messages came from widget or email.
- US-19: As an agent, after I reply to an offline visitor, I can see whether that reply was delivered through chat, delivered via email, read in chat, read via email, or failed by email bounce/spam complaint.

**Admin:**
- US-9: As an admin, I can enable/disable email fallback notifications per workspace in Support settings.
- US-10: As an admin, I can configure the delay before sending (default: 2 minutes).
- US-11: As an admin, I can set a custom "From name" for outbound emails.

**System:**
- US-12: The system does not send email if the conversation has no `customer_email`.
- US-13: The system does not send email if the conversation is in a terminal state (closed, spam). For resolved conversations, see §12 Edge Cases.
- US-14: The system correctly threads all emails per conversation using RFC 2822 headers.
- US-15: Inbound webhook rejects malformed or unauthenticated requests.
- US-20: The system does not permanently discard a pending fallback only because the visitor is online; it discards only when the message is read, stale, terminal, unsubscribed, invalid, already emailed, or otherwise ineligible.

## 4. Architecture

### 4.1 Outbound Flow (Agent Reply → Offline Visitor)

```
Agent replies via dashboard (any pod)
    │
    v
SupportInboxService.CreateConversationMessage()          [support_inbox.go]
    │
    ├── wsPublisher.Publish(SupportMessageEvent(...))    [broadcasts via RedisRelay to all pods]
    │
    └── emailFallbackService.OnAgentReply(ctx, msg, conv)
              │
              ├── if conv.CustomerEmail == nil → return
              ├── if conv.Status in [closed, spam] → return (terminal states — see §12)
              └── Redis ZADD GT email_fallback_outbox <now + delay_secs> conv.ID
                  Redis RPUSH email_fallback_msgs:{conv.ID} msg.ID

                  ZADD GT updates the score only if the new score is Greater
                  Than the existing one. This implements true debouncing:
                  - First reply at t=0: score = t+120s (fires at t=120)
                  - Second reply at t=90: score = t+210s (extends to t=210)
                  - Third reply at t=200: score = t+320s (extends to t=320)
                  Each new reply pushes the fire time forward by the full
                  delay window, batching everything into one email.

    ┌──────────────────────────────────────────────────┐
    │  Leader Poller Goroutine (one pod holds lease)    │
    │  Runs every 10s. Lease: SET email_fallback_lock   │
    │  NX EX 30 (auto-renews while healthy).            │
    │  On pod crash, another pod acquires within 30s.   │
    └──────────────────────────────────────────────────┘
              │
              ├── ZRANGEBYSCORE email_fallback_outbox -inf now → due entries
              │
              For each due conversation:
              │
              │  ┌─ PHASE 1: Claim (Redis) ────────────────────────┐
              ├── ZSCORE + check still due (guard against re-entry)
              ├── Move score to now + processing_ttl (e.g., +5 min)
              │   via ZADD XX GT — marks "in flight" so no other
              │   poller re-fires while we work, but entry stays
              │   in the sorted set until we explicitly remove it.
              ├── LRANGE email_fallback_msgs:{conv.ID} → msg IDs
              │   (do NOT delete yet)
              │  └──────────────────────────────────────────────────┘
              │
              │  ┌─ PHASE 2: Check & Send ─────────────────────────┐
              ├── Fetch accumulated messages by IDs
              ├── Re-fetch conversation (status/read cursor may have changed)
              ├── Filter to unread eligible outbound replies:
              │     - email_notified_at IS NULL
              │     - is_internal = false
              │     - message_type = reply
              │     - sender_type != customer
              │     - created_at > COALESCE(contact_last_seen_at, epoch)
              ├── If no eligible unread messages → go to PHASE 3 (cleanup)
              ├── Drop eligible messages older than max_delivery_age
              │   (default 10 min) → go to PHASE 3 (cleanup)
              ├── hub.Presence.GetOnlineVisitors(ctx, workspaceID)
              │     ├── Visitor online → postpone short retry, keep queue
              │     └── Visitor offline → continue
              ├── Render HTML email template with batched messages
              ├── emailClient.SendEmailWithHeaders(to, subject, html, text, headers)
              │     Headers: Reply-To, Message-ID, In-Reply-To, References, X-Conversation-ID
              │  └──────────────────────────────────────────────────┘
              │
              │  ┌─ PHASE 3: Record durably, then cleanup ─────────┐
              ├── BEGIN DB transaction:
              │     ├── INSERT support_email_logs (outbound, with
              │     │   RFCMessageID, PostmarkMessageID, MessageIDs)
              │     └── UPDATE support_messages SET email_notified_at
              ├── COMMIT
              ├── Only after COMMIT: ZREM + DEL email_fallback_msgs
              │  └──────────────────────────────────────────────────┘
              │
              │  On crash between PHASE 1 and PHASE 3:
              │  - Entry stays in sorted set with the in-flight score.
              │  - After processing_ttl expires, it becomes due again
              │    and the next leader poll re-processes it.
              │  - fireEmail checks email_notified_at on messages;
              │    if already set, it knows a prior attempt succeeded
              │    (DB committed) and just does cleanup (ZREM + DEL).
              │  - Net result: at-least-once delivery with DB-level
              │    dedup, never silent loss.
```

### 4.2 Inbound Flow (Visitor Replies via Email)

```
Visitor replies to notification email
    │
    v
Email → replies.helpin.ai MX → Postmark Inbound
    │
    v
POST /api/webhooks/postmark/inbound
    │
    v
WebhookHandler.PostmarkInbound()
    │
    ├── Authenticate (basic auth)
    ├── Parse MailboxHash: conv-{conversation_id}@replies.helpin.ai
    ├── Validate conversation exists and is not closed
    ├── Extract content: StrippedTextReply (fallback: TextBody)
    ├── Create SupportMessage (SenderType: customer, ViaChannel: email)
    ├── Store raw email in support_email_logs
    └── Broadcast to agents via wsPublisher
```

### 4.3 Outbox Architecture (Redis-Backed)

```go
type EmailFallbackService struct {
    redis          *redis.Client
    hub            *websocket.Hub
    emailClient    *email.Client
    messageRepo    *repository.SupportMessageRepository
    convRepo       *repository.SupportConversationRepository
    emailLogRepo   *repository.SupportEmailLogRepository
    installRepo    *repository.SupportInboxInstallationRepository
    logger         *slog.Logger
}
```

**Redis keys:**
- `email_fallback_outbox` — sorted set: member = conversationID, score = fire-at Unix timestamp
- `email_fallback_msgs:{conversationID}` — list of message IDs accumulated during debounce
- `email_fallback_lock` — leader lease (`SET NX EX 30`)

**Enqueue (`OnAgentReply`, runs on any pod):**
1. `ZADD GT email_fallback_outbox <now + delay_secs> <conversationID>` — GT (Greater Than) updates the fire time only if the new score is higher than the existing one, implementing true debounce: each new reply extends the window. If no entry exists, ZADD creates it.
2. `RPUSH email_fallback_msgs:{conversationID} <messageID>`

**Poll (`StartPoller`, runs on leader pod):**
1. Acquire lease: `SET email_fallback_lock <podID> NX EX 30` (renew every 10s)
2. Every 10s: `ZRANGEBYSCORE email_fallback_outbox -inf <now>` → due conversations
3. For each: bump score to `now + processing_ttl` via `ZADD XX GT` (marks in-flight, prevents re-fire by other pollers), `LRANGE` message list (no DEL yet), then `fireEmail()`
4. `fireEmail()` records durably to DB, then only on COMMIT does it `ZREM` + `DEL` from Redis
5. On pod crash mid-flight: entry stays in sorted set; after `processing_ttl` (5 min) it becomes due again and the next leader re-processes. `fireEmail()` checks `email_notified_at` to detect prior success and skips re-send (idempotent).
6. On pod crash while idle: lease expires in 30s, another pod acquires on next tick

**Why not Temporal?** The debounce window is short (2 min default). A Redis sorted-set outbox is simpler, has no workflow-engine overhead, and integrates naturally with the existing Redis infrastructure (already used for `RedisRelay` and `RedisPresence`).

**Why not pod-local timers?** In Kubernetes, any API pod can handle an agent reply. Pod-local timers would mean: (a) two pods could both schedule for the same conversation, sending duplicate emails; (b) `hub.IsVisitorOnline()` reads only local connection state, missing visitors connected to other pods; (c) pod restart loses all pending timers with no recovery path.

## 5. Data Model

### 5.1 New Columns on `support_messages`

GORM AutoMigrate adds these columns automatically from struct tags (no SQL migration file needed — see §5.4):

```go
// On SupportMessage in server/internal/model/support_inbox.go:
ViaChannel      *string    `json:"via_channel,omitempty" gorm:"size:20"`          // "email" | "widget" | nil
EmailNotifiedAt *time.Time `json:"email_notified_at,omitempty"`                   // set when message is included in an outbound email
```

**Design note — why no `email_message_id` on `SupportMessage`:**
An outbound email batches multiple messages into one RFC 2822 `Message-ID`. If `email_message_id` were stored per-message, a batch of 3 messages would either (a) share a single ID (breaking a unique index) or (b) each get a different ID (none of which match the actual sent email). Instead:
- **Outbound delivery IDs** (RFC 2822 `Message-ID`, Postmark `MessageID`) live on `support_email_logs` — one row per sent email, referencing N message IDs.
- **Inbound provider IDs** (Postmark's `MessageID` from the webhook payload) also live on `support_email_logs` with a unique index for idempotent dedup.
- **Per-message notification state** is tracked via `email_notified_at` on `support_messages` — a simple timestamp indicating this message was included in an outbound email.

### 5.2 New Table: `support_email_logs`

Schema applied via GORM AutoMigrate from new model, plus a Go migration helper for the unique index (see §5.4):

```go
// server/internal/model/support_email_log.go
type SupportEmailLog struct {
    ID                string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    WorkspaceID       string     `json:"workspace_id" gorm:"type:uuid;not null;index:idx_sel_workspace"`
    ConversationID    string     `json:"conversation_id" gorm:"type:uuid;not null;index:idx_sel_conversation"`
    Direction         string     `json:"direction" gorm:"size:10;not null"`              // "outbound" or "inbound"
    MessageIDs        DocsStringArray `json:"message_ids" gorm:"type:text[]"`              // outbound: batch of message IDs included (reuses existing Scanner/Valuer type from model/docs.go)
    FromEmail         string     `json:"from_email" gorm:"size:255"`
    ToEmail           string     `json:"to_email" gorm:"size:255"`
    Subject           string     `json:"subject" gorm:"size:500"`
    RFCMessageID      string     `json:"rfc_message_id" gorm:"size:255"`                 // outbound: generated Message-ID header
    InReplyTo         string     `json:"in_reply_to" gorm:"size:255"`                    // RFC 2822 In-Reply-To header
    PostmarkMessageID *string    `json:"postmark_message_id,omitempty" gorm:"size:255"`   // Postmark's server-assigned ID; nullable (set after successful send/receive)
    RawBody           string     `json:"-" gorm:"type:text"`                             // never serialized to JSON (PII)
    StrippedText      string     `json:"stripped_text,omitempty" gorm:"type:text"`
    Status            string     `json:"status" gorm:"size:20;not null;default:'sent'"`  // sent, failed, rejected
    ErrorMessage      string     `json:"error_message,omitempty" gorm:"type:text"`
    CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (SupportEmailLog) TableName() string { return "support_email_logs" }
```

**Idempotency:** `PostmarkMessageID` is a nullable `*string`. A **partial unique index** (`CREATE UNIQUE INDEX ... WHERE postmark_message_id IS NOT NULL`) ensures uniqueness only on non-null values — rows with `NULL` (e.g., failed sends before Postmark returns an ID) do not collide. Applied via a Go migration helper in `MigrateEmailFallbackSchema()`, since GORM AutoMigrate does not support partial unique indexes from struct tags.

For **inbound** emails, the webhook handler attempts the INSERT; if the unique index rejects it (duplicate `PostmarkMessageID`), the handler returns 200 OK (idempotent). For **outbound** emails, Postmark returns a unique `MessageID` per send which is stored here after a successful API call.

### 5.3 Migration Strategy

This repo uses **GORM AutoMigrate** at startup (`cmd/api/main.go`) — SQL migration files in `server/migrations/` are reference documentation only and are not executed. Schema changes are applied by:

1. **Adding GORM struct tags** to models → AutoMigrate adds columns/tables automatically.
2. **Go migration helpers** (e.g., `MigrateEmailFallbackSchema(db *gorm.DB) error`) for anything AutoMigrate cannot handle: custom indexes, column renames, data backfills. Called sequentially from `main.go` after AutoMigrate, following the existing pattern (`MigrateCRMSignalSchema`, `MigrateAutomationHealthSchema`, etc.).

For this feature:
- `SupportMessage.ViaChannel`, `SupportMessage.EmailNotifiedAt` → AutoMigrate (struct tag addition)
- `SupportEmailLog` table → AutoMigrate (new model registered in AutoMigrate call)
- Partial unique index on `PostmarkMessageID` → **Go migration helper** `MigrateEmailFallbackSchema(db)` using `db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sel_postmark_msg_id ON support_email_logs(postmark_message_id) WHERE postmark_message_id IS NOT NULL")`. GORM AutoMigrate cannot express partial indexes via struct tags.
- A reference SQL file `server/migrations/055_add_email_fallback.sql` is written for documentation but is **not executed at runtime**.

### 5.4 Extended Settings (JSONB — no migration needed)

Add to `SupportInboxSettings` in `server/internal/model/support_inbox.go`:
```go
EmailFallbackEnabled    bool   `json:"email_fallback_enabled"`
EmailFallbackDelaySecs  int    `json:"email_fallback_delay_secs"`  // default: 120
EmailFallbackFromName   string `json:"email_fallback_from_name"`   // falls back to workspace name
EmailFallbackMaxDeliveryAgeSecs int `json:"email_fallback_max_delivery_age_secs"` // default: 600
```

Add to `DefaultSupportInboxSettings()` and `UpdateInstallationSettingsRequest`.
Also update `mergeSettingsUpdate()` and any new validation in `server/internal/service/support_inbox_settings.go`, since support settings persistence is centralized there.

Validation:
- `email_fallback_delay_secs`: min 30, max 600, default 120.
- `email_fallback_max_delivery_age_secs`: min 120, max 1800, default 600.
- `email_fallback_max_delivery_age_secs` must be greater than or equal to `email_fallback_delay_secs`. The send window must never expire before the first eligible send attempt.

## 6. API Endpoints

### New Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/webhooks/postmark/inbound` | Basic auth (webhook secret) | Postmark inbound webhook |

Registered outside JWT-protected route group (like `/api/git/webhook`).

### Modified Endpoints

The actual support settings API uses installation-based routes (not workspace-nested), per `router.go` and `supportService.ts`:

| Method | Path | Change |
|--------|------|--------|
| `PATCH` | `/api/support/inbox/installations?workspace_id={id}` | Accepts `email_fallback_*` fields in settings JSONB (handler: `UpdateInstallationSettings`) |
| `GET` | `/api/support/inbox/installations?workspace_id={id}` | Returns `email_fallback_*` fields in settings JSONB (handler: `GetInstallation`) |

Frontend uses `supportService.updateInstallationSettings(workspaceId, settings)` → `useChatSettings(workspaceId)` / `useUpdateChatSettings(workspaceId)` hooks.

## 7. Email Format & Delivery Specification

Reference: Crisp's email notification pattern (screenshot: `crisp-email-screenshot.png`). The email should feel like a personal reply from the agent — not a system notification. This is critical for open rates and reply engagement.

### 7.1 Subject Line

| Scenario | Format | Example |
|----------|--------|---------|
| First email in conversation | `{conversation_subject} (#{short_id})` | `Pricing question (#a3f)` |
| Subsequent emails (threading) | `Re: {conversation_subject} (#{short_id})` | `Re: Pricing question (#a3f)` |

- `short_id` = first 3–6 chars of conversation UUID (for human reference, like Crisp's `#7f2`)
- If no explicit subject exists, derive from the visitor's first message (truncated at 60 chars + `...`)
- The `Re:` prefix is critical for Gmail/Outlook to thread correctly alongside RFC 2822 headers

### 7.2 From Address

**Format**: `{Agent Name} - {Workspace Name} <messages@replies.helpin.ai>`

| Component | Source | Example |
|-----------|--------|---------|
| Agent Name | `SupportMessage.SenderDisplayName` or agent's profile name | `Sarah Chen` |
| Workspace Name | `workspace.Name` or `settings.EmailFallbackFromName` override | `Acme Support` |
| Email Address | Static Postmark-verified sender on reply domain | `messages@replies.helpin.ai` |

**Rendered**: `Sarah Chen - Acme Support <messages@replies.helpin.ai>`

This matches Crisp's pattern (`Fabi Pina - Usermaven <messages@crisp.usermaven.com>`). The visitor sees a real person's name in their inbox, not "noreply" or a generic system address.

**Custom email domain** (future, not v1): Workspaces could configure `messages@support.acme.com` via Postmark's custom domain feature + DNS verification. For v1, all workspaces share the `replies.helpin.ai` domain.

### 7.3 Email Body — Layout

The email uses a minimal, personal layout — no heavy branding, marketing headers, or card UI. It should look like a human sent it.

```
┌─────────────────────────────────────────────┐
│                                             │
│  {Message content — plain text/basic HTML}  │
│                                             │
│  {Message 2 content}        (if batched)    │
│                                             │
│  --                                         │
│                                             │
│  ● {Agent Name} via {Workspace Name}.       │
│                                             │
│  Reply directly to this email, or go to     │
│  chat.                                      │
│                                             │
│  Sent from Helpin. Unsubscribe from these   │
│  emails.                                    │
│                                             │
└─────────────────────────────────────────────┘
```

#### Body Content

- **Message text**: Rendered as-is (plain text with line breaks preserved). No "X replied to your conversation:" wrapper — the message IS the email body, like a real email.
- **Batched messages**: If multiple agent messages accumulated during debounce, include all sequentially separated by a blank line. No per-message headers or timestamps — keep it natural.
- **Signature separator**: `--` (standard email signature delimiter, RFC 3676)
- **No conversation history quoted in body**: Gmail/Outlook handle quoting via `In-Reply-To` header threading. Including prior messages in the body would duplicate what the email client already shows.

#### Footer Block

| Element | Format | Notes |
|---------|--------|-------|
| Agent identity | `● {Agent Name} via {Workspace Name}.` | Small avatar circle (CSS) + agent name + "via" + workspace. Mirrors Crisp's "Fabi Pina via Usermaven." |
| Reply CTA | `Reply directly to this email, or go to chat.` | "chat" is a hyperlink to `{LastPageURL}#helpin-conv={conversation_id}` |
| Attribution | `Sent from Helpin. Unsubscribe from these emails.` | "Helpin" links to `https://helpin.ai`. "Unsubscribe" is a `mailto:` link (see §7.6) |

#### Chat Deep-Link

The "chat" link in the footer points to `{LastPageURL}#helpin-conv={conversation_id}`:
- `LastPageURL` = the page URL currently stored on `SupportWidgetSession.LastPageURL` (the existing schema already persists this; the conversation model does not)
- The SDK (`packages/sdk-js/src/core/widget.ts`) checks `window.location.hash` on init — if it contains `helpin-conv=`, it auto-opens the widget and navigates to that conversation via `openConversation(conversationId)`
- Fallback: if `last_page_url` is unavailable, link to the workspace's base URL
- If product later needs an immutable "origin page" per conversation, add a dedicated conversation/session snapshot field rather than assuming one already exists
- The email copy emphasizes "Reply directly to this email" as the **primary** action — the chat link is secondary

### 7.4 RFC 2822 Headers (Email Threading)

These headers ensure all emails for a conversation thread correctly in Gmail, Outlook, and Apple Mail:

```
Reply-To: conv-{conversation_id}@replies.helpin.ai
Message-ID: <helpin-{email_log_id}@replies.helpin.ai>
In-Reply-To: <helpin-{previous_email_log_id}@replies.helpin.ai>
References: <helpin-{first_email_log_id}@replies.helpin.ai> ... <helpin-{prev_email_log_id}@replies.helpin.ai>
X-Conversation-ID: {conversation_id}
List-Unsubscribe: <mailto:unsubscribe-{conversation_id}@replies.helpin.ai>
```

| Header | Purpose | Source |
|--------|---------|--------|
| `Reply-To` | Routes visitor replies to Postmark inbound webhook | Conversation-specific address with `MailboxHash` |
| `Message-ID` | Unique ID for this email (used by future `In-Reply-To`) | Generated from `SupportEmailLog.ID` |
| `In-Reply-To` | Points to the previous email in this conversation thread | Previous `SupportEmailLog.RFCMessageID` for this conversation |
| `References` | Full chain of message IDs (required by some clients) | All prior `RFCMessageID` values for this conversation |
| `X-Conversation-ID` | Internal tracking | Conversation UUID |
| `List-Unsubscribe` | One-click unsubscribe (Gmail shows button) | RFC 2369 |

### 7.5 Plain Text Version

Every email includes both HTML and plain-text bodies (Postmark requires both for deliverability). The plain-text version:

```
{Message content}

{Message 2 content}

--
{Agent Name} via {Workspace Name}

Reply directly to this email, or go to chat:
{LastPageURL}#helpin-conv={conversation_id}

Sent from Helpin (https://helpin.ai).
Unsubscribe: mailto:unsubscribe-{conversation_id}@replies.helpin.ai
```

### 7.6 Unsubscribe Handling (v1)

For v1, unsubscribe is a `mailto:` link that creates an inbound email to `unsubscribe-{conversation_id}@replies.helpin.ai`. The inbound webhook recognizes the `unsubscribe-` prefix and sets a flag on the conversation (`email_unsubscribed = true`). Future emails for that conversation are suppressed.

This is per-conversation, not per-visitor. Full visitor-level unsubscribe is a non-goal for v1.

### 7.7 HTML Template Guidelines

- **No images or logos in header** — keeps it personal, avoids spam filters
- **System font stack**: `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif`
- **Max width**: 600px (email standard)
- **Background**: White (`#ffffff`), text: dark grey (`#1a1a1a`)
- **Footer text**: Light grey (`#6b7280`), 13px
- **Links**: Workspace accent color (falls back to `#2563eb`)
- **Agent avatar**: 24px circle with initials (CSS-generated, no image dependency)
- **No tracking pixels** — respect visitor privacy

### Postmark Client Extension

Add to `server/internal/email/postmark.go`:
```go
type EmailHeader struct {
    Name  string `json:"Name"`
    Value string `json:"Value"`
}

// SendEmailWithHeaders sends an email with custom From, Reply-To, and RFC 2822 headers.
// The `from` parameter overrides the client's default fromEmail for this send.
func (c *Client) SendEmailWithHeaders(from, to, subject, htmlBody, textBody, replyTo string, headers []EmailHeader) error
```

**From field composition** (in `EmailFallbackService.fireEmail()`):

The existing `email.Client` stores a static `fromEmail` (e.g., `messages@replies.helpin.ai`) set at construction. For fallback emails, the `From` header uses the format defined in §7.2:

```go
// Compose the From field per §7.2: "{Agent Name} - {Workspace Name} <address>"
agentName := lastMessage.SenderDisplayName
workspaceName := settings.EmailFallbackFromName
if workspaceName == "" {
    workspaceName = workspace.Name
}
// RFC 5322 display name format
from := fmt.Sprintf("%s - %s <%s>", agentName, workspaceName, s.emailClient.FromEmail())
// Result: "Sarah Chen - Acme Support <messages@replies.helpin.ai>"
```

This requires exposing `FromEmail()` as a getter on `email.Client` (currently private `fromEmail` field). The actual sending address stays the same (verified in Postmark), only the display name changes per workspace/agent.

## 8. Inbound Processing

### Webhook Handler

New file: `server/internal/handler/webhook_postmark.go`

```go
type PostmarkInboundHandler struct {
    emailFallbackService *service.EmailFallbackService
    webhookSecret        string
}

func (h *PostmarkInboundHandler) PostmarkInbound(w http.ResponseWriter, r *http.Request)
```

### Postmark Inbound Payload

New file: `server/internal/model/webhook_postmark.go`

```go
type PostmarkInboundPayload struct {
    From              string             `json:"From"`
    FromFull          PostmarkAddress    `json:"FromFull"`
    To                string             `json:"To"`
    ToFull            []PostmarkAddress  `json:"ToFull"`
    Subject           string             `json:"Subject"`
    MessageID         string             `json:"MessageID"`
    MailboxHash       string             `json:"MailboxHash"`
    TextBody          string             `json:"TextBody"`
    HtmlBody          string             `json:"HtmlBody"`
    StrippedTextReply string             `json:"StrippedTextReply"`
    Date              string             `json:"Date"`
    Headers           []PostmarkHeader   `json:"Headers"`
}
```

### Service Processing (`EmailFallbackService.ProcessInboundEmail`)

1. Parse conversation ID from `MailboxHash` (strip `conv-` prefix, validate UUID)
2. Verify conversation exists and is not in a terminal state (`closed`, `spam`)
3. Validate `FromFull.Email` matches `conversation.customer_email` (case-insensitive)
4. Extract content: `StrippedTextReply` preferred, fallback to `TextBody`
5. Check `PostmarkMessageID` uniqueness via `support_email_logs` unique index (idempotency — Postmark may redeliver)
6. Create `SupportMessage` with `SenderType: "customer"`, `ViaChannel: "email"`
7. Create `SupportEmailLog` with `Direction: "inbound"`, `PostmarkMessageID` from payload
8. Broadcast to agents via wsPublisher (uses `hub.BroadcastAll` for cross-pod delivery)

## 9. Offline Detection & Debouncing

### Integration Point

In `CreateConversationMessage()` at `server/internal/service/support_inbox.go`, after existing broadcast logic:

```go
if !msg.IsInternal && msg.SenderType != "customer" && msg.MessageType == "reply" {
    if s.emailFallbackService != nil {
        go s.emailFallbackService.OnAgentReply(context.Background(), workspaceID, msg, conv)
    }
}
```

### OnAgentReply Flow

1. Check email fallback enabled for workspace
2. Check conversation has `customer_email`
3. Check conversation status is not terminal (`closed` or `spam`). **Note:** `resolved` is NOT terminal — the backend `CreateConversationMessage()` accepts messages on resolved conversations (agents can reply without unresolving first), so email fallback must also fire for resolved conversations to avoid silent delivery gaps.
4. Enqueue to Redis outbox (ZADD + RPUSH) with configured delay (default 2min)

### fireEmail Flow (runs on leader pod)

Called after the poller has bumped the outbox score (in-flight) and read the message ID list.

1. Fetch messages by IDs. If none exist, skip to step 12 (cleanup only).
2. Re-fetch conversation (status, email, unsubscribe state, read cursor, and visitor identity may have changed during debounce).
3. If status is terminal (`closed`, `spam`) → skip to step 12 (cleanup), log cancellation.
4. If `customer_email` is missing, `email_unsubscribed = true`, or the CRM contact email is invalid → skip to step 12 (cleanup).
5. Filter to **eligible unread outbound replies**:
   - `email_notified_at IS NULL`
   - `is_internal = false`
   - `message_type = "reply"`
   - `sender_type != "customer"`
   - `created_at > COALESCE(conversation.contact_last_seen_at, epoch)`
6. If no messages remain after filtering, the customer has either read them in chat or they were already notified → skip to step 12 (cleanup).
7. Drop any eligible unread outbound reply where `now > message.created_at + email_fallback_max_delivery_age_secs`. If no eligible messages remain, skip to step 12 (cleanup). This prevents abrupt emails long after the agent replied while still allowing a newer reply to send on its own window.
8. Check `hub.Presence.GetOnlineVisitors(ctx, workspaceID)`.
   - If the visitor is online and eligible messages are still fresh, **postpone** the sorted-set score to `now + emailFallbackOnlineRetryDelay` (default 30s) and return without deleting `email_fallback_msgs:{conversationID}`.
   - If the visitor is online but the eligible messages are now stale, cleanup.
   - If the visitor is offline, continue.
9. Build RFC 2822 threading headers from previous `support_email_logs` for this conversation.
10. Render HTML template, send via `emailClient.SendEmailWithHeaders()`.
11. **DB transaction** (atomic):
   - INSERT `SupportEmailLog` row (outbound, with `RFCMessageID`, `PostmarkMessageID`, `MessageIDs` batch)
   - UPDATE `email_notified_at = NOW()` on each included `SupportMessage`
   - COMMIT
12. **Redis cleanup** (only after DB commit or final skip decision): `ZREM email_fallback_outbox {convID}` + `DEL email_fallback_msgs:{convID}`

**Crash safety**: If the pod dies after Postmark accepts the email but before the DB transaction commits, the outbox entry's in-flight score expires after `processing_ttl`, making it due again. The next leader re-enters `fireEmail()`, sees the messages are not yet notified, and retries. Worst case: email is sent twice. This is acceptable — a duplicate notification is better than a lost one. If the DB commit succeeded but Redis cleanup did not, re-processing sees `email_notified_at` set and cleans up without re-sending.

### 9.1 Unread Filtering Helper

Implement the unread filtering as a small, testable helper in `server/internal/service/email_fallback.go`:

```go
func unreadFallbackMessages(conv *model.SupportConversation, messages []model.SupportMessage) []model.SupportMessage
```

The helper preserves message order and returns only public outbound reply messages that are newer than `conv.ContactLastSeenAt` and have not been email-notified. This helper must not perform Redis, DB, or Postmark work.

### 9.2 Online Retry Helper

Add:

```go
const emailFallbackOnlineRetryDelay = 30 * time.Second

func (s *EmailFallbackService) postpone(ctx context.Context, conversationID string, delay time.Duration) error
```

`postpone` updates only `email_fallback_outbox` score. It must not append message IDs and must not delete `email_fallback_msgs:{conversationID}`. This preserves the original queued messages while allowing a future send if the visitor leaves before the max delivery age expires.

### 9.3 Agent-Visible Delivery Status

The dashboard message thread must expose delivery state for the latest outbound reply:

| Backend state | Agent-facing status |
|---------------|---------------------|
| `conversation.contact_last_seen_at >= message.created_at` for widget conversations | `Read in chat` |
| `message.email_notified_at != nil`, no later email status | `Delivered via email` |
| matching outbound `support_email_logs.opened_at != nil` or `message.email_read_at != nil` | `Read via email` |
| matching outbound `support_email_logs.delivered_at != nil` | Still `Delivered via email` unless a more specific status is shown |
| matching outbound `support_email_logs.status = bounced` | `Delivery failed` with error detail |
| matching outbound `support_email_logs.status = spam_complaint` | `Marked as spam` with error detail |

Current implementation already has most of this plumbing:
- `support_messages.email_notified_at`
- `support_messages.email_read_at`
- `support_email_logs.delivered_at`, `opened_at`, `bounced_at`, `status`, `error_message`
- frontend receipt rendering in `frontend/src/components/support/MessageThread.tsx` and `frontend/src/components/support/MessageBubble.tsx`

Acceptance requirement: when a support agent sends a reply and that reply is delivered as fallback email, the visible receipt under the latest outbound bubble must update to `Delivered via email` without requiring the agent to inspect the email details modal. Bounce/spam complaint states must supersede generic delivered/read receipts.

## 10. Settings UI

Add "Email Notifications" card to `ChatGeneralTab.tsx` (`frontend/src/components/settings/ChatGeneralTab.tsx`):

- **Toggle**: "Send email when visitor is offline" (`email_fallback_enabled`)
- **Input**: "Delay before sending (seconds)" (`email_fallback_delay_secs`, min 30, max 600)
- **Input**: "Send window (minutes)" (`email_fallback_max_delivery_age_secs`, default 10 minutes, min 2, max 30)
- **Input**: "From name" (`email_fallback_from_name`, placeholder: workspace name)

Uses existing `useChatSettings` / `useUpdateChatSettings` hooks.

### Message Badge — Full Propagation Path

Widget clients do **not** consume backend `SupportMessage` directly. The field has to cross both backend payload shapes and both SDK mapping paths:

**1. Backend payload types** (`server/internal/model/support_inbox.go`):
Add `ViaChannel` to `WidgetMessageReceivedPayload`:
```go
type WidgetMessageReceivedPayload struct {
    // ... existing fields ...
    ViaChannel string `json:"via_channel,omitempty"`
}
```
Also add `ViaChannel` and `EmailNotifiedAt` to `SupportMessage` itself so the dashboard API and initial widget history can expose them.

**2. Backend payload population**:
- `server/internal/websocket/support_events.go` must populate `via_channel` when building the `SupportMessageEvent` payload for dashboard/widget fan-out
- `server/internal/websocket/widget_handler.go` must populate `via_channel` on the direct `message:received` echo path used by widget-originated sends

**3. SDK message mapper** (`packages/sdk-js/src/core/widget.ts`):
The SDK maps snake_case backend payloads to the shared `Message` interface. Add `via_channel` mapping in both places:
- Initial session history (`payload.messages.map(...)`)
- Live `message:received` handling

```typescript
viaChannel: raw.via_channel ?? undefined,
```

and for the live event path:
```typescript
viaChannel: msg.via_channel ?? undefined,
```

**4. Shared type** (`packages/shared/src/types/message.ts`):
Add to the `Message` interface:
```typescript
viaChannel?: 'email' | 'widget';
```

`packages/widget-core/src/types.ts` already aliases the shared `Message` type, so no separate widget-core type definition is needed.

**5. Widget-core rendering** (`packages/widget-core/src/components/MessageBubble.tsx`):
Read `message.viaChannel` and render a small "Via email" badge (same pattern as `aiConfidence` display).

**6. Dashboard rendering** (`frontend/src/components/support/MessageBubble.tsx`):
Read `via_channel` from the backend `SupportMessage` type and render badge.

**7. Dashboard type** (`frontend/src/lib/pmTypes.ts`):
Add to `SupportMessage`:
```typescript
via_channel?: 'email' | 'widget' | null;
email_notified_at?: string;
```

## 11. Security

1. **Webhook Auth**: Basic auth in URL, validated in handler. Secret stored as `POSTMARK_INBOUND_WEBHOOK_SECRET`.
2. **Rate Limiting**: 100 req/min per IP on webhook endpoint. Implemented via **Traefik `RateLimit` middleware** annotation on the webhook Ingress route (`k8s/prod/server.yaml`, `k8s/stage/server.yaml`). This is ingress-level, so it applies before traffic reaches any API pod — no per-pod coordination needed and no new Go dependencies. The codebase has no existing rate limiting library; adding one for a single webhook is unnecessary when Traefik handles it natively. Configuration: `traefik.ingress.kubernetes.io/router.middlewares: default-postmark-ratelimit@kubernetescrd` with a Traefik `Middleware` CRD (`average: 100, period: 1m, burst: 20`).
3. **Conversation ID Validation**: UUID format validation on `MailboxHash`. Invalid = 200 OK + WARN log.
4. **Sender Validation**: `FromFull.Email` must match `conversation.customer_email` (case-insensitive).
5. **Content Sanitization**: Strip HTML from inbound. Use Postmark's `StrippedTextReply`.
6. **No PII in Logs**: Never log email body content.
7. **Duplicate Protection**: DB-enforced unique index on `support_email_logs.postmark_message_id`. Inbound webhook checks this before creating a message — duplicate Postmark deliveries are safely rejected (200 OK).

### New Config Fields (`server/internal/config/config.go`)

```go
PostmarkInboundWebhookSecret string  // POSTMARK_INBOUND_WEBHOOK_SECRET
SupportEmailReplyDomain      string  // SUPPORT_EMAIL_REPLY_DOMAIN (default: replies.helpin.ai)
```

## 12. Edge Cases

| Edge Case | Behavior |
|-----------|----------|
| Visitor comes back online during debounce and reads the reply | `fireEmail()` filters against `contact_last_seen_at`, sees no unread eligible outbound replies, and cleans up without sending |
| Visitor comes back online during debounce but does not read the reply | `fireEmail()` checks `hub.Presence.GetOnlineVisitors()` (cross-pod), postpones the queue with a short retry, and keeps message IDs intact |
| Visitor stays online and unread past max delivery age | Queue is cleaned up silently. Do not send a stale email later when the visitor disconnects |
| Visitor is online unread, then disconnects within max delivery age | Next retry sends the fallback email because the reply is still unread, fresh, and the visitor is offline |
| Visitor reads in widget after fallback was queued but before send | No email is sent; `contact_last_seen_at` suppresses the queued message |
| No email on file | `OnAgentReply()` returns immediately, no outbox entry |
| Conversation closed/spam during debounce | `fireEmail()` re-fetches and checks status; discards if terminal |
| Conversation resolved | **Not suppressed.** Backend allows agent replies on resolved conversations; suppressing email fallback would create a silent delivery gap where the reply shows in-app but never reaches an offline visitor. If the conversation transitions to `closed` or `spam` during debounce, `fireEmail()` catches it on re-fetch. |
| Agent replies to resolved conversation in UI | Same path as any agent reply. The `resolved` status does not block `CreateConversationMessage()` in the backend, so email fallback must also honor it. |
| Multiple agents reply quickly | Messages accumulate in Redis list; ZADD GT extends fire time with each reply (true debounce); single batched email |
| One queued reply is read, then another reply arrives | The read message is filtered out by `contact_last_seen_at`; the newer unread message remains eligible and may be emailed |
| A new agent reply arrives after an older unread reply became stale | The new message starts a fresh debounce and freshness window. The stale older reply is excluded from the fallback email batch. |
| Email reply to closed/spam conversation | 200 OK, warning logged, no message created |
| Inbound from non-matching address | 200 OK, warning logged, rejected (no message created) |
| Duplicate webhook delivery | Idempotent via unique index on `support_email_logs.postmark_message_id` — INSERT fails, handler returns 200 OK |
| Pod restart during debounce | Outbox entries are in Redis and survive pod restarts. Another pod acquires the leader lease within 30s and processes due entries. |
| Pod crash mid-send (after Postmark, before DB commit) | Outbox entry stays in-flight (score bumped). After `processing_ttl` (5 min), it becomes due again. Re-processing detects `email_notified_at` unset and retries. Worst case: visitor receives a duplicate email (acceptable — better than lost notification). |
| Pod crash after DB commit, before Redis cleanup | Outbox entry re-fires. `fireEmail` step 1 sees all messages already have `email_notified_at` set → skips to cleanup (ZREM + DEL). No duplicate email. |
| Two pods handle replies for same conversation | Both RPUSH message IDs to the same Redis list. ZADD GT keeps one debounced fire-time per conversation and extends it when later replies arrive. The leader poller claim prevents duplicate processing. |
| Email client nil (Postmark not configured) | `OnAgentReply` returns immediately, no outbox entry |
| AI auto-reply to offline visitor | Same path as agent replies |
| Agent sends reply and fallback email is sent | Dashboard receipt under latest outbound bubble shows `Delivered via email` |
| Postmark delivery webhook arrives | Dashboard remains `Delivered via email`; details modal can show provider delivery timestamp |
| Postmark open webhook arrives | Dashboard receipt can upgrade to `Read via email` |
| Postmark bounce or spam complaint arrives | Dashboard receipt shows `Delivery failed` or `Marked as spam`, superseding generic delivered/read status |
| Very long email reply | Truncate at 50KB, log warning |
| Redis unavailable | `OnAgentReply` logs error and returns (no email). Visitor gets reply on next widget visit. Acceptable degradation. |

## 13. Implementation Phases

### Phase 1: Schema & Postmark Extension
| File | Change |
|------|--------|
| `server/internal/model/support_inbox.go` | Add `ViaChannel`, `EmailNotifiedAt` to `SupportMessage`; email fallback fields to `SupportInboxSettings` + `DefaultSupportInboxSettings()` |
| `server/internal/model/support_email_log.go` | New model (AutoMigrate creates table) |
| `server/internal/model/webhook_postmark.go` | Postmark inbound payload DTOs |
| `server/internal/email/postmark.go` | Add `EmailHeader`, `SendEmailWithHeaders()` |
| `server/internal/config/config.go` | Add webhook secret + reply domain config |
| `server/internal/service/support_inbox_settings.go` | Merge and validate new `email_fallback_*` settings in the JSONB settings flow |
| `server/cmd/api/main.go` | Register `SupportEmailLog` in `AutoMigrate()` call |
| `server/migrations/055_add_email_fallback.sql` | Reference-only SQL documentation (not executed at runtime) |

### Phase 2: Email Fallback Service
| File | Change |
|------|--------|
| `server/internal/repository/support_email_log.go` | New repository (Create, GetByConversation, GetByPostmarkMessageID) |
| `server/internal/repository/support_inbox.go` | Add `GetByIDs()` to message repo, add `UpdateEmailNotifiedAt()` |
| `server/internal/service/email_fallback.go` | New service: Redis outbox (enqueue/poll), leader lease, `OnAgentReply`, `fireEmail`, `ProcessInboundEmail`, HTML templates |
| `server/internal/service/email_fallback_test.go` | Unit tests per §15.1 |
| `server/internal/service/support_inbox.go` | Add `emailFallbackService` field, call in `CreateConversationMessage` |

### Phase 2b: Unread-Gated Fresh Fallback Amendments
| File | Change |
|------|--------|
| `server/internal/model/support_inbox.go` | Add `EmailFallbackMaxDeliveryAgeSecs` to `SupportInboxSettings`, defaults, and update request DTOs |
| `server/internal/service/support_inbox_settings.go` | Validate max delivery age and preserve defaults during partial settings updates |
| `server/internal/service/email_fallback.go` | Add `unreadFallbackMessages`, max delivery age check, and online `postpone` behavior instead of online cleanup |
| `server/internal/service/email_fallback_test.go` | Add unread/read/freshness/online-postpone tests |
| `frontend/src/components/settings/ChatGeneralTab.tsx` | Add send-window control if settings UI exposes advanced fallback options |

### Phase 3: Webhook & Wiring
| File | Change |
|------|--------|
| `server/internal/handler/webhook_postmark.go` | New handler with basic auth validation |
| `server/internal/handler/webhook_postmark_test.go` | Handler tests per §15.1 |
| `server/internal/router/router.go` | Register webhook route (outside JWT group) |
| `server/cmd/api/main.go` | Wire `EmailFallbackService` (with Redis client, Hub, repos), `PostmarkInboundHandler`, start leader poller goroutine |

### Phase 4: Frontend & Widget Propagation
| File | Change |
|------|--------|
| `server/internal/model/support_inbox.go` | Add `ViaChannel` to `WidgetMessageReceivedPayload` and propagate `ViaChannel` / `EmailNotifiedAt` on `SupportMessage` |
| `server/internal/websocket/support_events.go` | Populate `via_channel` in support message broadcast payloads |
| `server/internal/websocket/widget_handler.go` | Populate `via_channel` on direct widget `message:received` echoes |
| `packages/shared/src/types/message.ts` | Add `viaChannel?: 'email' \| 'widget'` to `Message` interface |
| `packages/sdk-js/src/core/widget.ts` | Map `via_channel` → `viaChannel` in both initial history and live message handlers; add `#helpin-conv=` hash fragment handler on init for email CTA deep-links |
| `packages/widget-core/src/components/MessageBubble.tsx` | "Via email" badge when `viaChannel === "email"` |
| `frontend/src/lib/pmTypes.ts` | Add `via_channel`, `email_notified_at` to `SupportMessage` |
| `frontend/src/components/settings/ChatGeneralTab.tsx` | Email Notifications settings card |
| `frontend/src/components/support/MessageThread.tsx` | Derive latest outbound receipt state from `contact_last_seen_at`, `email_notified_at`, `email_read_at`, and email delivery status fields |
| `frontend/src/components/support/MessageBubble.tsx` | "Via email" badge, `Delivered via email`, `Read via email`, bounce, and spam complaint states |

### Phase 5: Infrastructure (DNS & Postmark)
- MX record for `replies.helpin.ai` → `inbound.postmarkapp.com`
- Configure Postmark inbound webhook URL with basic auth
- Add env vars to Doppler: `POSTMARK_INBOUND_WEBHOOK_SECRET`, `SUPPORT_EMAIL_REPLY_DOMAIN`
- Add Traefik `Middleware` CRD for rate limiting (`k8s/prod/server.yaml`, `k8s/stage/server.yaml`) and annotate webhook ingress route

## 14. Non-Goals (Out of Scope)

- Attachments in email replies (v1 = text only)
- Rich HTML rendering of inbound emails
- Per-conversation email opt-out / unsubscribe processing
- Multiple email addresses per visitor
- Outbound email for internally-created conversations
- Email-first conversations (new conversations via inbound email)

## 15. Verification

### 15.1 Automated Tests (Go)

**Unit tests** (`server/internal/service/email_fallback_test.go`):

| Test | Validates |
|------|-----------|
| `TestOnAgentReply_NoEmail_Skips` | No outbox entry when `customer_email` is nil |
| `TestOnAgentReply_TerminalStatus_Skips` | No outbox entry when status is `closed` or `spam` |
| `TestOnAgentReply_ResolvedStatus_Enqueues` | Outbox entry IS created for `resolved` conversations (not terminal) |
| `TestOnAgentReply_FeatureDisabled_Skips` | No outbox entry when `email_fallback_enabled` is false |
| `TestUnreadFallbackMessages_FiltersReadAndIneligibleReplies` | Helper excludes internal messages, customer messages, already-notified messages, and messages at/before `contact_last_seen_at` |
| `TestFireEmail_VisitorOnlineUnread_Postpones` | Email not sent when visitor is online and messages are still unread/fresh; Redis score moves to short retry and message list remains |
| `TestFireEmail_VisitorOnlineRead_CleansUp` | Email not sent and queue is cleaned up when `contact_last_seen_at` covers all queued outbound replies |
| `TestFireEmail_UnreadFreshOffline_Sends` | Email sends when messages are unread, visitor is offline, and eligible messages are within max delivery age |
| `TestFireEmail_UnreadStale_CleansUp` | Email not sent when all eligible unread outbound replies are older than `email_fallback_max_delivery_age_secs` |
| `TestFireEmail_DropsStaleMessagesFromMixedBatch` | Mixed fresh/stale unread batch sends only the fresh replies and leaves stale replies unnotified |
| `TestFireEmail_SendsOnlyMessagesNewerThanContactLastSeen` | Mixed queued messages produce an email containing only unread messages newer than the customer read cursor |
| `TestFireEmail_PreviouslyEmailNotifiedMessagesNotResent` | Previously emailed messages are excluded from future batches |
| `TestFireEmail_BatchesMessages` | Multiple message IDs accumulated → single email with all content |
| `TestFireEmail_StatusChangedDuringDebounce` | Re-fetched conversation is now closed → email not sent |
| `TestFireEmail_ThreadingHeaders` | Correct `Message-ID`, `In-Reply-To`, `References` built from prior `support_email_logs` |
| `TestProcessDeliveryEvent_MarksDelivered` | Postmark delivery webhook updates `support_email_logs.delivered_at/status` for dashboard receipt/detail surfaces |
| `TestProcessOpenEvent_MarksReadEmail` | Postmark open webhook updates `support_email_logs.opened_at` and message `email_read_at`, allowing `Read via email` receipt |
| `TestProcessBounceEvent_MarksFailed` | Bounce webhook updates log status/error so the dashboard can show `Delivery failed` |
| `TestProcessInbound_HappyPath` | Parses `MailboxHash`, creates message with `ViaChannel: "email"`, logs to `support_email_logs` |
| `TestProcessInbound_DuplicatePostmarkID` | Second call with same `PostmarkMessageID` → no duplicate message, 200 OK |
| `TestProcessInbound_SenderMismatch` | `FromFull.Email` ≠ `conversation.customer_email` → rejected |
| `TestProcessInbound_ClosedConversation` | Returns 200 OK, warning logged, no message created |
| `TestProcessInbound_MalformedMailboxHash` | Invalid UUID → 200 OK, warning logged |

**Integration tests** (`server/internal/service/email_fallback_integration_test.go`):

| Test | Validates |
|------|-----------|
| `TestOutboxEnqueueAndPoll_RaceCondition` | Two goroutines enqueue for same conversation → exactly one email sent |
| `TestLeaderFailover` | Kill leader mid-poll → new leader acquires lease and processes pending entries |
| `TestCrashRecovery_InFlightRetry` | Simulate crash after send but before DB commit → entry re-fires after processing_ttl, `fireEmail` detects unset `email_notified_at` and retries |
| `TestCrashRecovery_AlreadyCommitted` | Simulate crash after DB commit but before Redis cleanup → entry re-fires, `fireEmail` detects `email_notified_at` already set, skips re-send, cleans up Redis |
| `TestDebounceExtension` | Reply at t=0, reply at t=90s → fire time is t+210s (not t+120s); verifies ZADD GT extends window |
| `TestRedisUnavailable_GracefulDegradation` | Redis down → `OnAgentReply` returns error, no panic |

**Webhook handler tests** (`server/internal/handler/webhook_postmark_test.go`):

| Test | Validates |
|------|-----------|
| `TestPostmarkInbound_AuthFailure` | Missing/wrong basic auth → 401 |
| `TestPostmarkInbound_InvalidJSON` | Malformed body → 200 OK (Postmark expects 200 to avoid retries) |
| `TestPostmarkInbound_RateLimit` | Traefik middleware returns 429 when >100 req/min from same IP (tested via k8s integration or Traefik config validation) |

### 15.2 Frontend Tests

| Test | File | Validates |
|------|------|-----------|
| `MessageBubble renders "Via email" badge` | `frontend/src/components/support/MessageBubble.test.tsx` | Badge visible when `via_channel === "email"`, hidden otherwise |
| `MessageBubble renders email fallback receipts` | `frontend/src/components/support/MessageBubble.test.tsx` | Shows `Delivered via email`, `Read via email`, `Delivery failed`, and `Marked as spam` based on receipt props and message delivery fields |
| `MessageThread derives delivered via email for latest outbound reply` | `frontend/src/components/support/MessageThread.test.tsx` | Latest outbound message with `email_notified_at` renders an email-delivery receipt even when customer is offline |
| `MessageThread prefers read in chat over email delivered` | `frontend/src/components/support/MessageThread.test.tsx` | `contact_last_seen_at >= message.created_at` renders `Read in chat` instead of `Delivered via email` |
| `MessageThread prefers email failure over generic receipt` | `frontend/src/components/support/MessageThread.test.tsx` | Bounce/spam complaint states supersede delivered/read receipts |
| `Widget MessageBubble renders "Via email" badge` | `packages/widget-core/src/components/MessageBubble.test.tsx` | Badge visible when `viaChannel === "email"` |
| `SDK maps via_channel from payload` | `packages/sdk-js/test/unit/core/widget.test.ts` | `via_channel` in raw payload → `viaChannel` in Message object |
| `ChatGeneralTab renders email fallback settings` | `frontend/src/components/settings/ChatGeneralTab.test.tsx` | Toggle, delay input, from-name input rendered and update correctly |

### 15.3 Operational Observability

| Metric / Log | Type | Purpose |
|--------------|------|---------|
| `email_fallback.enqueued` | Counter (workspace_id) | Outbox entries created |
| `email_fallback.sent` | Counter (workspace_id) | Emails successfully sent |
| `email_fallback.postponed_online_unread` | Counter (workspace_id) | Visitor was online but reply was still unread/fresh, so queue was retried |
| `email_fallback.cancelled_read` | Counter (workspace_id) | Cancelled because customer read the reply in the widget before email send |
| `email_fallback.cancelled_stale` | Counter (workspace_id) | Cancelled because all eligible unread replies exceeded max delivery age |
| `email_fallback.cancelled_status` | Counter (workspace_id) | Cancelled because conversation status changed |
| `email_fallback.send_failed` | Counter (workspace_id) | Postmark send failures |
| `email_fallback.inbound_processed` | Counter (workspace_id) | Inbound emails successfully processed |
| `email_fallback.inbound_rejected` | Counter (reason) | Inbound emails rejected (auth, sender, closed, duplicate) |
| `email_fallback.leader_acquired` | Gauge | Which pod holds the leader lease |
| `email_fallback.outbox_depth` | Gauge | ZCARD of outbox sorted set |

**Alert**: `email_fallback.send_failed` > 5 in 5 min → PagerDuty (Postmark may be down or misconfigured).

### 15.4 Manual QA Checklist
- [ ] Agent replies to offline visitor → email received within configured delay
- [ ] Two quick agent replies → single batched email
- [ ] Visitor returns during debounce → no email sent
- [ ] Visitor returns during debounce, leaves widget closed/unfocused, then disconnects within 10 minutes → email sent
- [ ] Visitor reads reply in widget before disconnecting → no email sent
- [ ] Visitor stays online unread for more than 10 minutes, then disconnects → no email sent
- [ ] Reply to email → message appears in agent inbox with "Via email" badge
- [ ] Reply to email → message appears in widget when visitor returns
- [ ] Emails thread correctly in Gmail/Outlook
- [ ] Disable feature in settings → no emails sent
- [ ] Change delay in settings → new delay respected
- [ ] Custom from name appears in email
- [ ] Invalid webhook payload → 200 OK, warning logged
- [ ] Agent replies on resolved conversation to offline visitor → email sent (not suppressed)
- [ ] Agent replies on spam conversation → no email sent
- [ ] Agent replies to offline visitor → latest outbound bubble shows `Delivered via email` after fallback send
- [ ] Postmark open webhook for the fallback email → latest outbound bubble upgrades to `Read via email`
- [ ] Postmark bounce/spam webhook → latest outbound bubble shows the failure state
