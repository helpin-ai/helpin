# Support Message Actions (Crisp-style) Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Crisp-style hover action menu to support messages with Edit (within email-fallback window), Copy, Reply (quote), Delete, and Info — plus a "Sent · Undo · M:SS" → "Delivered to email" footer affordance and Cmd/Ctrl+Z shortcut, all wired to the existing Redis email-fallback outbox so the customer-facing chat record matches the email.

**Architecture:** A new column `cancellable_until` on `support_messages` defines the window during which an agent-sent reply can still be undone before the email-fallback timer fires. The window matches the existing per-workspace `EmailFallbackDelaySecs` setting (30–600s, default 120s), set by `OnAgentReply` after it successfully enqueues. **Edit and Undo are the same flow:** the message is soft-deleted, the message's entry is removed from the per-conversation Redis outbox list (and the conversation entry from the sorted set is removed only when the list becomes empty so other queued messages in the same batch are unaffected), and the message's `Content` is restored into the existing Reply composer for the agent to rewrite and resend. Resend creates a fresh message with a fresh window — no in-place mutation of an existing message or its outbox row. After the window expires, the message becomes immutable; the bubble footer flips to a terminal status (`Delivered to email`, `Read in chat`, etc.); the menu's Edit item disappears but Delete remains (with a stronger warning that the email may already have been sent). The hover menu always exposes Copy, Reply (quote), Delete, and Info regardless of window state. All mutations broadcast WS events so any other open agent tab and the visitor's widget update in real time.

**Tech Stack:** Go 1.24 + Chi + GORM (backend), React 19 + TipTap + TanStack Query + shadcn/ui Popover/AlertDialog/Dialog (frontend), nhooyr.io/websocket (real-time), Redis outbox via `internal/service/email_fallback.go`.

---

## Out of scope (deliberate)

- Editing messages **after** the cancellable window closes — Crisp doesn't allow it either; we follow the same model.
- Editing messages that came **from the customer** (visitor) — only outbound agent messages are mutable.
- Editing AI-authored messages — the sender-ownership predicate (`sender_type = 'user' AND sender_user_id = current actor`) naturally excludes AI messages whose `sender_type = 'ai'`. Treated as v1-out-of-scope; a separate flow may add "rewrite as me" later.
- Visitor-side edit UI in `packages/widget-core/` — handled in a follow-up; this plan only broadcasts the events.
- In-place message-body mutation (edit existing message rather than soft-delete-and-rewrite) — explicitly avoided; the Crisp-style flow makes this unnecessary and removes the need to keep a Redis outbox payload in sync with edits.
- A separate "Edit" composer mode — Edit reuses the existing Reply composer with the message's markdown rehydrated.

---

## File Structure

### Backend — `server/`

| File | Status | Responsibility |
|------|--------|----------------|
| `internal/model/support_inbox.go` | Modify | Add a single `CancellableUntil *time.Time` field to `SupportMessage`. (No `edited_at` in v1 — would require a new `edited_from_message_id` field on `CreateMessageRequest` which is out of scope. Info modal always shows `Edited: No` for v1.) |
| `internal/dbmigrate/sql/202604300010_support_message_cancellable.sql` | Create | Add `cancellable_until` column. If `deleted_at` is missing on `support_messages`, this migration also adds it. |
| `internal/repository/support_inbox.go` | Modify | New methods: `GetMessageForActor`, `SoftDeleteMessage(ctx, id, userID)`, `SetCancellableUntil(ctx, id, t)`. Ownership predicate is **`sender_type = 'user' AND sender_user_id = ? AND message_type = 'reply' AND is_internal = false`** (model's actual column names — there is no `sender_id`). |
| `internal/service/support_message_actions.go` | Create | New `SupportMessageActionsService` exposing `Delete(actorID, msgID, undo bool)` and `Info(actorID, msgID)`. Window expiry comes from the message's `cancellable_until`, which `OnAgentReply` writes from its actual computed `fireAt`; there is no separate fixed window constant. Takes `*EmailFallbackService` directly — no extra interface. |
| `internal/service/email_fallback.go` | Modify | Add `CancelForMessage(ctx, workspaceID, conversationID, msgID)` that LREMs the message ID from `email_fallback_msgs:{conversationID}`, then checks `LLEN`: if zero, ZREM `conversationID` from `email_fallback_outbox` and DEL the msgs key. If non-zero, leave the conversation entry — other queued replies in the same batch must still fire. Returns `(alreadySent bool, err error)`: alreadySent is true when **any** row exists in `support_email_logs` for this message ID (sent/delivered/opened/bounced/spam_complaint — all mean the email already fired). Uses `SupportEmailLogRepository.GetByMessageID(ctx, workspaceID, msgID)` (the real signature in `repository/support_email_log.go:45`). |
| `internal/handler/support_inbox.go` | Modify | Two new endpoints under `/api/support/inbox/conversations/{id}/messages/{msg_id}` (matches the existing inbox route style at `router.go:645`): `DELETE` (undo/remove) and `GET` (info). Sender-gated; cancellable window enforced only when `?undo=1`. |
| `internal/router/router.go` | Modify | Wire the two routes inside the existing `/support/inbox` group. |
| `internal/websocket/support_events.go` | Modify | Add a parallel `Entity: "support_conversation_message", Action: "deleted"` event next to the existing `created` event at line 18. |
| `cmd/api/main.go` | Modify | Construct `SupportMessageActionsService` and inject into the support inbox handler. |
| `internal/service/support_message_actions_test.go` | Create | Table-driven service tests: window enforcement on Undo, delete cancels outbox, non-owner rejected, Delete-after-window succeeds and reports `email_already_sent`. |
| `internal/handler/support_inbox_message_actions_test.go` | Create | HTTP tests for the two endpoints (200/403/404/410 cases). |

### Frontend — `frontend/`

| File | Status | Responsibility |
|------|--------|----------------|
| `src/lib/pm-types/support.ts` | Modify | Add `cancellable_until?: string` (ISO) to `SupportMessage`. |
| `src/lib/services/supportMessages.ts` | Create | Thin service wrapping the two new endpoints (`deleteMessage`, `getMessageInfo`). All paths are `/support/inbox/conversations/...` and append a locally defined query string helper like the existing `supportService.ts` (`const qs = (workspaceId: string) => ...`). |
| `src/hooks/queries/useSupportMessageActions.ts` | Create | TanStack mutation + query for the two endpoints, with optimistic cache updates and rollback. |
| `src/components/support/MessageActionsMenu.tsx` | Create | The 3-dots Popover menu component. Items: Edit + Undo (conditional on `cancellable_until > now`), Copy, Reply (quote), Delete, Info. **Edit and Undo from this menu both call the same delete-and-restore handler;** Edit is just the "make a correction" verb, Undo is the "scrap it" verb. They're presented as a single item labelled "Edit" inside the menu (matching Crisp); Undo lives in the message footer for quick keyboard-free access. |
| `src/components/support/MessageDeleteDialog.tsx` | Create | shadcn AlertDialog matching the screenshot copy and destructive button. Used only for the **Delete** action on messages outside the cancellable window — inside the window, Edit/Undo silently soft-delete without confirmation (Crisp parity). |
| `src/components/support/MessageInfoDialog.tsx` | Create | shadcn Dialog rendering the info table (Identifier with copy, Sent on, Sent by, From, Origin, Type, Delivered, Not delivered, Read, Edited, Translated, Automated). |
| `src/components/support/MessageBubble.tsx` | Modify | Render hover trigger for the actions menu opposite the bubble (anchored near the bottom for tall bubbles via measured height); render the `Sent · Undo · M:SS` / `Delivered to email` footer state alongside existing receipt rendering. The footer's Undo and the menu's Edit both call the same handler. |
| `src/components/support/ReplyComposer.tsx` | Modify | On undo/edit completion, the conversation's existing draft mechanism is reused: the deleted message's markdown is written into the draft for that conversation, the composer focuses, and `replyMode` is forced to `'reply'`. **No new composer mode**, no banner, no save-button label change. |
| `src/components/support/__tests__/MessageActionsMenu.test.tsx` | Create | Vitest tests: edit hidden after expiry, delete-after-expiry opens dialog with strong warning copy, info opens modal, copy uses navigator.clipboard. |

---

## Data model

```sql
-- Confirm whether `deleted_at` already exists on support_messages. If yes, the line below is a no-op.
-- Verify with: SELECT column_name FROM information_schema.columns WHERE table_name='support_messages';
ALTER TABLE support_messages
  ADD COLUMN IF NOT EXISTS deleted_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS cancellable_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_support_messages_cancellable_until
  ON support_messages (cancellable_until)
  WHERE cancellable_until IS NOT NULL;
```

`cancellable_until` is set by `OnAgentReply` to the **same `fireAt`** it computes from `settings.EmailFallbackDelaySecs` after a successful enqueue (see `email_fallback.go:242–253`). This guarantees the UI countdown matches the moment the email actually fires; there is no separate constant.

It is set only when:
1. `SenderType = 'user'` (i.e. a human operator).
2. `MessageType = 'reply'` and `IsInternal = false`.
3. The conversation has email fallback enabled and a customer email — the same predicates `OnAgentReply` already enforces. If `OnAgentReply` returns without enqueuing, `cancellable_until` stays NULL.

For all other messages (visitor, AI-authored, internal notes, system events), it stays NULL.

**Already-sent detection.** When `CancelForMessage` runs, "did this message already get emailed?" is answered by `SupportEmailLogRepository.GetByMessageID(ctx, workspaceID, msgID)` — `support_messages.email_delivery_status` is a virtual `gorm:"-"` field projected at read time, not a stored value, so we can't query it. If any outbound `support_email_log` row exists for this `message_id`, return `alreadySent = true`; `sent`, `delivered`, `opened`, `bounced`, and `spam_complaint` all mean the email already fired.

**No `edited_at` in v1.** Adding it would require a new `edited_from_message_id` field on `CreateMessageRequest` and matching plumbing in the create-message handler/service plus the frontend send mutation. That's net-new contract surface for one boolean in the info modal. Defer to a focused follow-up; v1 info modal renders `Edited: No` unconditionally.

Soft-delete reuses the `deleted_at` column. Existing list queries already filter via the standard GORM soft-delete tag if `DeletedAt gorm.DeletedAt` is on the model; if it isn't, the migration's `deleted_at` add is a no-op for existing rows and the repository read paths must be updated to filter it explicitly.

---

## API contract

### `DELETE /api/support/inbox/conversations/{cid}/messages/{mid}`
**Query:** `?undo=1` to enforce the cancellable window (returns the original markdown for restoration). Without `undo=1`, this is a "Remove Message" call (window not enforced; markdown not returned).
**Auth:** sender must equal current actor; 403 otherwise.
**Behaviour:** Soft-delete (sets `deleted_at`).
**Response 200 (with `?undo=1`):**
```json
{
  "id": "msg-uuid",
  "markdown": "<original body for composer restore>",
  "email_already_sent": false
}
```
**Response 200 (without `undo=1`):**
```json
{
  "id": "msg-uuid",
  "email_already_sent": true
}
```
**Errors:**
- `403 Forbidden` — actor is not the sender.
- `410 Gone` — only when `?undo=1` and `cancellable_until <= now()`. Without `undo=1`, deletion is always allowed.
**Side effects:** Calls `email_fallback.CancelForMessage` (no-op if already sent; returns `email_already_sent: true`). Broadcasts a WS `support_conversation_message` event with `Action: "deleted"` (see WebSocket events section).

### `GET /api/support/inbox/conversations/{cid}/messages/{mid}`
**Response 200:**
```json
{
  "id": "msg-uuid",
  "sent_at": "2026-04-30T17:58:30Z",
  "sender": { "id": "...", "name": "Waqar", "type": "operator", "avatar_url": "..." },
  "from": "Operator",
  "origin": "chat",
  "type": "text",
  "delivered": { "channel": "email" | "chat" | null, "delivered_at": "..." },
  "not_delivered_reason": null,
  "read": false,
  "read_at": null,
  "edited": false,           // always false in v1 — see "no edited_at" in Data model
  "translated": false,
  "automated": false
}
```

The shape mirrors Crisp's info modal one-to-one. Most fields are derived from existing message + receipt rows — no new storage needed for v1.

---

## WebSocket events

Reuse the existing event shape (`Entity` + `Action`) from `server/internal/websocket/support_events.go:18` — there is no `type: 'support.message.X'` namespace in this codebase. The created-message event uses:

```go
{ Entity: "support_conversation_message", Action: "created", EntityID: msg.ID, ParentID: conv.ID }
```

We add a parallel deletion event:

```go
{ Entity: "support_conversation_message", Action: "deleted", EntityID: msg.ID, ParentID: conv.ID }
```

Frontend `useRealtimeSync` already routes events by `event.entity === "support_conversation_message"` and invalidates the conversation's message list; ensure `deleted` uses that same branch and keep the conversation-list invalidation it already does. Edits are observed implicitly: an Edit produces a delete + a normal create; both flow through the same entity.

---

## UX states (frontend)

### Hover affordance
- `MessageBubble` wraps its existing layout in a `group` container with a `MessageActionsMenu` trigger absolutely-positioned outside the bubble (left for outbound, right for inbound) and `top-1/2 -translate-y-1/2` so it tracks bubble centre — for tall bubbles, switch to `bottom-2` once bubble height > `5rem` so the dots sit near the bottom (Crisp parity).

### Footer states for outbound messages
| Condition | Footer renders |
|-----------|---------------|
| `cancellable_until > now()` | `Sent · Undo · M:SS` (Undo as button, MM:SS countdown) |
| `cancellable_until <= now()` AND `email_delivery_status = 'sent'` | `Delivered to email` |
| `cancellable_until <= now()` AND `receipt = 'read'` | `Read in chat` (existing) |
| `email_delivery_status = 'bounced'` | (existing red banner) |

### Edit / Undo behaviour
- Both **menu Edit** and **footer Undo** call the same handler: `DELETE ?undo=1` → returns markdown → write into the conversation's draft → focus composer in `Reply` mode.
- No separate composer mode, no banner, no save-button label change — the agent simply has the message back in the composer to rewrite and resend (or abandon).
- If the agent abandons the draft (clears, navigates away), the message is gone and no email goes out. Clean.

### Cmd/Ctrl+Z behaviour
- Global keydown listener mounted while the inbox view is active.
- Triggers only when no input/textarea/contenteditable is focused (so it doesn't conflict with TipTap's own undo stack).
- Finds the latest outbound message in the open conversation with `cancellable_until > now()` and runs the same flow as the footer Undo.

---

## Phases & task breakdown

We execute in 6 phases. Each phase ends in a green test run and a commit. Phases 1–4 are backend/foundation. Phases 5–6 are frontend feature work.

> **No-overengineering scoreboard for reviewers:** there is no `PATCH /messages/{id}` endpoint, no `UpdateMessageBody` repo method, no `ReplaceForMessage` outbox helper, no `EmailFallbackCanceller` abstraction, no `editingMessageId` store field, no third "Edit" composer mode, no `markdownToTiptap.ts` helper task, no `edited_at` column or `edited_from_message_id` create-message field. Edit and Undo are the **same** delete-and-restore flow. If a future change adds any of these, it should justify why Crisp's simpler model isn't enough.

---

## Phase 1: DB migration + model + repository

### Task 1.1: SQL migration for new columns

**Files:**
- Create: `server/internal/dbmigrate/sql/202604300010_support_message_cancellable.sql`

- [ ] **Step 1: Check whether `deleted_at` already exists on `SupportMessage`**

Run: `grep -n "deleted_at\|DeletedAt" server/internal/model/support_inbox.go | head -5`

- If a match is present → `deleted_at` exists; the migration below leaves it as a no-op (`IF NOT EXISTS`).
- If no match → the migration below adds it. **Also** add `DeletedAt gorm.DeletedAt \`gorm:"index"\`` to the model in Task 1.2 so existing GORM read paths automatically filter soft-deleted rows.

- [ ] **Step 2: Create migration file**

Use the `migrate create` CLI scaffold.
```bash
cd server && go run ./cmd/migrate create support_message_cancellable
```

Expected: a new file under `internal/dbmigrate/sql/`. Replace its body with:

```sql
ALTER TABLE support_messages
  ADD COLUMN IF NOT EXISTS deleted_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS cancellable_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_support_messages_cancellable_until
  ON support_messages (cancellable_until)
  WHERE cancellable_until IS NOT NULL;
```

- [ ] **Step 3: Validate**

Run: `cd server && go run ./cmd/migrate validate`
Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add server/internal/dbmigrate/sql/202604300010_support_message_cancellable.sql
git commit -m "feat(support): add cancellable_until column to messages"
```

---

### Task 1.2: Add fields to GORM model

**Files:**
- Modify: `server/internal/model/support_inbox.go` (`SupportMessage` struct)

- [ ] **Step 1: Add fields**

Locate `type SupportMessage struct { ... }` and add (alongside other timestamps):

```go
CancellableUntil *time.Time     `json:"cancellable_until,omitempty" gorm:"index"`
// If grep in Task 1.1 showed no existing DeletedAt, also add:
// DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
```

- [ ] **Step 2: Build**

Run: `cd server && go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add server/internal/model/support_inbox.go
git commit -m "feat(support): cancellable_until on SupportMessage"
```

---

### Task 1.3: Repository methods (TDD)

**Files:**
- Modify: `server/internal/repository/support_inbox.go`
- Create or extend: `server/internal/repository/support_inbox_test.go`

- [ ] **Step 1: Write failing tests**

Append a new test function `TestSupportMessageActions` in the existing `support_inbox_test.go` with sub-tests:
- `soft_delete_message_sets_deleted_at`
- `get_message_for_actor_returns_nil_when_not_owner`
- `set_cancellable_until_writes_timestamp`

Each subtest uses the existing `setupSupportInboxTestDB(t)` helper. Assert via `gorm` reload that fields changed.

- [ ] **Step 2: Run tests → fail**

Run: `cd server && go test ./internal/repository -run TestSupportMessageActions -v`
Expected: `undefined: SoftDeleteMessage` etc.

- [ ] **Step 3: Implement repository methods**

Add to `SupportMessageRepository`. The ownership predicate uses the model's actual columns (`sender_type`, `sender_user_id`, `message_type`, `is_internal`); there is no `sender_id` column.

```go
const ownedByActor = `
    sender_type   = 'user'
AND sender_user_id = ?
AND message_type  = 'reply'
AND is_internal   = false
AND deleted_at IS NULL`

func (r *SupportMessageRepository) SoftDeleteMessage(ctx context.Context, id, userID string) error {
    res := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
        Where("id = ? AND "+ownedByActor, id, userID).
        Update("deleted_at", time.Now())
    if res.Error != nil { return fmt.Errorf("soft delete message: %w", res.Error) }
    if res.RowsAffected == 0 { return ErrNotFound }
    return nil
}

func (r *SupportMessageRepository) GetMessageForActor(ctx context.Context, id, userID string) (*model.SupportMessage, error) {
    var msg model.SupportMessage
    err := r.db.WithContext(ctx).
        Where("id = ? AND "+ownedByActor, id, userID).
        First(&msg).Error
    if errors.Is(err, gorm.ErrRecordNotFound) { return nil, nil }
    if err != nil { return nil, fmt.Errorf("get message for actor: %w", err) }
    return &msg, nil
}

func (r *SupportMessageRepository) SetCancellableUntil(ctx context.Context, id string, t time.Time) error {
    return r.db.WithContext(ctx).Model(&model.SupportMessage{}).
        Where("id = ?", id).
        Update("cancellable_until", t).Error
}
```

- [ ] **Step 4: Run tests → pass**

Run: `cd server && go test ./internal/repository -run TestSupportMessageActions -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/support_inbox.go server/internal/repository/support_inbox_test.go
git commit -m "feat(support): repo methods to edit/soft-delete agent messages"
```

---

## Phase 2: Email-fallback service: cancel

### Task 2.1: `CancelForMessage`

**Files:**
- Modify: `server/internal/service/email_fallback.go`
- Modify: `server/internal/service/email_fallback_test.go`

- [ ] **Step 1: Write failing tests**

Add to `email_fallback_test.go`:
- `cancel_for_message_removes_only_target_msg_id_when_others_are_pending`
- `cancel_for_message_removes_conversation_entry_when_list_becomes_empty`
- `cancel_for_message_returns_already_sent_when_email_log_exists`
- `cancel_for_message_noop_when_message_was_never_queued`

Use the existing `miniredis` test harness already in this file. Multi-message scenarios are essential — the queue is per-conversation, not per-message.

- [ ] **Step 2: Run tests → fail**

Run: `cd server && go test ./internal/service -run TestEmailFallback -v`
Expected: `undefined: CancelForMessage`.

- [ ] **Step 3: Implement method**

In `email_fallback.go`, add to `EmailFallbackService`. The Redis layout (per `email_fallback.go:84–87, 247–253, 1230–1233`) is:

- A sorted set `email_fallback_outbox` whose members are conversation IDs and scores are `fireAt` unix timestamps.
- A list `email_fallback_msgs:{conversationID}` of pending message IDs in that conversation.

So canceling one queued reply must LREM that single message ID, and only ZREM/DEL the conversation entry if the list becomes empty — otherwise we'd cancel emails for unrelated pending replies in the same batch.

```go
// CancelForMessage removes a single queued message from its conversation outbox.
// If this was the last pending message for the conversation, also removes the
// conversation entry from the sorted set.
//
// Returns (alreadySent=true, nil) when ANY outbound email log row exists for
// this message — `sent`, `delivered`, `opened`, `bounced`, `spam_complaint`
// all mean the email already fired and the agent can't take it back.
func (s *EmailFallbackService) CancelForMessage(
    ctx context.Context, workspaceID, conversationID, messageID string,
) (alreadySent bool, err error) {
    if log, _ := s.emailLogRepo.GetByMessageID(ctx, workspaceID, messageID); log != nil {
        return true, nil
    }

    msgKey := s.msgListKey(conversationID)
    if err := s.redis.LRem(ctx, msgKey, 0, messageID).Err(); err != nil {
        return false, fmt.Errorf("lrem queued message: %w", err)
    }
    remaining, err := s.redis.LLen(ctx, msgKey).Result()
    if err != nil { return false, fmt.Errorf("llen queued msgs: %w", err) }
    if remaining > 0 {
        return false, nil
    }
    if err := s.redis.ZRem(ctx, emailFallbackOutboxKey, conversationID).Err(); err != nil {
        return false, fmt.Errorf("zrem conversation: %w", err)
    }
    if err := s.redis.Del(ctx, msgKey).Err(); err != nil {
        return false, fmt.Errorf("del msg list: %w", err)
    }
    return false, nil
}
```

> The service constructor must accept `*repository.SupportEmailLogRepository` for the already-sent check. Wire this in `cmd/api/main.go` where `NewEmailFallbackService` is constructed; the email-log repo already exists there.

- [ ] **Step 4: Run tests → pass**

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/email_fallback.go server/internal/service/email_fallback_test.go
git commit -m "feat(support): CancelForMessage on email-fallback service"
```

---

## Phase 3: Action service + handler

### Task 3.1: `SupportMessageActionsService` (TDD)

**Files:**
- Create: `server/internal/service/support_message_actions.go`
- Create: `server/internal/service/support_message_actions_test.go`

- [ ] **Step 1: Write failing tests**

Cases:
- `delete_with_undo_within_window_cancels_outbox_and_returns_markdown`
- `delete_with_undo_after_window_returns_ErrCancellableExpired`
- `delete_without_undo_after_window_succeeds_and_reports_email_already_sent`
- `delete_by_non_owner_returns_ErrForbidden`
- `info_returns_message_metadata`

Tests use the real `*EmailFallbackService` against `miniredis` (matching `email_fallback_test.go`'s harness). No interface abstraction — keep dependencies concrete.

- [ ] **Step 2: Implement**

The service has no fixed window constant — the cancellable window is whatever `OnAgentReply` already wrote to `cancellable_until` based on `settings.EmailFallbackDelaySecs`.

```go
package service

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/helpin-ai/helpin/server/internal/model"
    "github.com/helpin-ai/helpin/server/internal/repository"
    "github.com/helpin-ai/helpin/server/internal/websocket"
)

type SupportMessageActionsService struct {
    repo     *repository.SupportMessageRepository
    fallback *EmailFallbackService
    pub      *websocket.Publisher
    now      func() time.Time // injectable for tests; defaults to time.Now
}

func NewSupportMessageActionsService(
    repo *repository.SupportMessageRepository,
    fallback *EmailFallbackService,
    pub *websocket.Publisher,
) *SupportMessageActionsService {
    return &SupportMessageActionsService{repo: repo, fallback: fallback, pub: pub, now: time.Now}
}

var (
    ErrCancellableExpired = errors.New("cancellable window expired")
    ErrForbidden          = errors.New("forbidden")
    ErrNotFound           = errors.New("not found")
)

// Delete soft-deletes a message the actor sent.
//   - undo=true: requires cancellable_until > now; returns the original Content
//     so the caller can rehydrate the composer.
//   - undo=false: a generic Remove Message; allowed any time the actor owns the message.
func (s *SupportMessageActionsService) Delete(ctx context.Context, actorUserID, msgID string, undo bool) (content string, alreadySent bool, err error) {
    msg, err := s.repo.GetMessageForActor(ctx, msgID, actorUserID)
    if err != nil { return "", false, fmt.Errorf("delete: load message: %w", err) }
    if msg == nil { return "", false, ErrForbidden }

    if undo {
        if msg.CancellableUntil == nil || !msg.CancellableUntil.After(s.now()) {
            return "", false, ErrCancellableExpired
        }
        content = msg.Content
    }

    if err := s.repo.SoftDeleteMessage(ctx, msg.ID, actorUserID); err != nil {
        return "", false, fmt.Errorf("delete: soft-delete: %w", err)
    }
    sent, err := s.fallback.CancelForMessage(ctx, msg.WorkspaceID, msg.ConversationID, msg.ID)
    if err != nil { return "", false, fmt.Errorf("delete: cancel email: %w", err) }
    s.publishDeleted(ctx, msg.WorkspaceID, msg.ConversationID, msg.ID)
    return content, sent, nil
}

// Info returns the message metadata for the info dialog.
func (s *SupportMessageActionsService) Info(ctx context.Context, actorUserID, msgID string) (*MessageInfoDTO, error) {
    // Conversation access is enforced upstream via RequireWorkspaceAccess +
    // PermSupportRead. Info is readable by any teammate with that perm.
    return s.repo.GetMessageInfo(ctx, msgID)
}
```

> The publish-then-return ordering in `Delete` is: soft-delete → cancel email → publish WS → return. Publishing last ensures cache invalidation arrives after the DB and Redis state have already changed.

> `CancelForMessage` requires the conversation ID because the Redis outbox is keyed per-conversation, not per-message.

- [ ] **Step 3: Run tests → pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(support): SupportMessageActionsService for undo/delete/info"
```

---

### Task 3.2: HTTP handler + router

**Files:**
- Modify: `server/internal/handler/support_inbox.go`
- Modify: `server/internal/router/router.go`
- Modify: `cmd/api/main.go`
- Create: `server/internal/handler/support_inbox_message_actions_test.go`

- [ ] **Step 1: Write failing handler tests**

Test cases via `httptest`:
- `DELETE ?undo=1 returns 200 with markdown when within window`
- `DELETE ?undo=1 returns 410 when window expired`
- `DELETE returns 403 when actor is not the sender`
- `DELETE without ?undo=1 returns 200 with email_already_sent`
- `GET returns info DTO`

- [ ] **Step 2: Implement handlers**

```go
func (h *SupportInboxHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
    actor := authorization.MustActor(r.Context())
    msgID := chi.URLParam(r, "msg_id")
    undo := r.URL.Query().Get("undo") == "1"

    markdown, alreadySent, err := h.actions.Delete(r.Context(), actor.UserID, msgID, undo)
    switch {
    case errors.Is(err, service.ErrCancellableExpired):
        writeError(w, 410, "undo window has expired")
    case errors.Is(err, service.ErrForbidden):
        writeError(w, 403, "forbidden")
    case errors.Is(err, service.ErrNotFound):
        writeError(w, 404, "not found")
    case err != nil:
        slog.ErrorContext(r.Context(), "delete message", "error", err, "message_id", msgID)
        writeError(w, 500, "delete failed")
    default:
        resp := map[string]any{"id": msgID, "email_already_sent": alreadySent}
        if undo { resp["markdown"] = markdown }
        writeJSON(w, 200, resp)
    }
}

func (h *SupportInboxHandler) GetMessageInfo(w http.ResponseWriter, r *http.Request) { /* parallel shape */ }
```

Routes (in `router.go` under the existing `/support/inbox` group, alongside the existing `Get/Post /inbox/conversations/{id}/messages` routes at line 645/650):

```go
r.With(requirePerm(authorization.PermSupportEdit)).Delete(
    "/inbox/conversations/{id}/messages/{msg_id}", h.SupportInbox.DeleteMessage)
r.With(requirePerm(authorization.PermSupportRead)).Get(
    "/inbox/conversations/{id}/messages/{msg_id}", h.SupportInbox.GetMessageInfo)
```

`main.go`: instantiate `SupportMessageActionsService` and pass to handler.

- [ ] **Step 3: Run tests → pass**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(support): DELETE/GET routes for message actions"
```

---

### Task 3.3: WS broadcast on edit/delete

**Files:**
- Modify: `server/internal/service/support_message_actions.go`
- Modify: `server/internal/websocket/events.go` (or wherever support events are typed)

- [ ] **Step 1: Add the deletion event**

In `server/internal/websocket/support_events.go`, mirror the existing `support_conversation_message` `created` publisher (line 18) with a new helper that publishes `Action: "deleted"` carrying `EntityID = msgID`, `ParentID = conversationID`, and the workspace ID.

- [ ] **Step 2: Publish in service**

After repo soft-delete + Redis cancel succeed, call the new publisher. Ordering: soft-delete → cancel email → publish → return.

- [ ] **Step 3: Add a service test asserting the publisher was called**

Mock the publisher (or use the existing test fake the support events tests already rely on) and assert one deletion event with the correct entity ID + parent ID.

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(support): broadcast support_conversation_message:deleted on undo/remove"
```

---

## Phase 4: Set `cancellable_until` from the actual fallback fireAt

### Task 4.1: Have `OnAgentReply` set `cancellable_until` after a successful enqueue

**Files:**
- Modify: `server/internal/service/email_fallback.go` (`OnAgentReply` already computes `fireAt` at L246 and successfully enqueues at L247–253)
- Modify: `server/internal/service/email_fallback_test.go` (extend existing tests)

- [ ] **Step 1: Locate the enqueue success branch**

Open `email_fallback.go` and find the block ending around L253 where `RPush` succeeds. After the push, the message is committed to fire at `fireAt`. That is the same instant we want `cancellable_until` to expire.

- [ ] **Step 2: Write a single line at the success point**

```go
if err := s.messageRepo.SetCancellableUntil(ctx, msg.ID, fireAt); err != nil {
    slog.WarnContext(ctx, "set cancellable_until failed", "error", err, "message_id", msg.ID)
    // non-fatal: queue is correct, UI just won't show Undo
}
```

If `OnAgentReply` doesn't yet hold a reference to `messageRepo`, inject it via `NewEmailFallbackService` — straightforward DI change in `cmd/api/main.go`.

- [ ] **Step 3: Test — `OnAgentReply` writes `cancellable_until = fireAt` on success**

Use the same in-memory SQLite + miniredis harness as the existing test file. Assert `msg.CancellableUntil.Equal(fireAt)`.

- [ ] **Step 4: Test — `OnAgentReply` does NOT set `cancellable_until` when not eligible**

Cases to cover (the existing `OnAgentReply` already returns early in these scenarios — assert `cancellable_until` stays NULL):
- conversation has email fallback disabled
- customer has no email
- message is internal note
- sender is AI (`SenderType != 'user'`)

- [ ] **Step 5: Run tests → pass**

- [ ] **Step 6: Commit**

```bash
git commit -m "feat(support): set cancellable_until from email-fallback fireAt"
```

---

## Phase 5: Frontend foundation — types, services, hooks, store

### Task 5.1: Type + service + mutation hooks

**Files:**
- Modify: `frontend/src/lib/pm-types/support.ts`
- Create: `frontend/src/lib/services/supportMessages.ts`
- Create: `frontend/src/hooks/queries/useSupportMessageActions.ts`
- Modify: `frontend/src/lib/queryKeys.ts` (add `messages.info(id)` key)

- [ ] **Step 1: Add field to `SupportMessage`**

```ts
cancellable_until?: string;
```

- [ ] **Step 2: Service**

```ts
const qs = (workspaceId: string, undo?: boolean) => {
  const params = new URLSearchParams({ workspace_id: workspaceId });
  if (undo) params.set('undo', '1');
  return `?${params.toString()}`;
};

export const supportMessagesService = {
  /** undo=true → server returns the original Content for composer restore + enforces window. */
  remove: (workspaceId: string, cid: string, mid: string, undo: boolean) =>
    api.del<{ id: string; markdown?: string; email_already_sent: boolean }>(
      `/support/inbox/conversations/${cid}/messages/${mid}${qs(workspaceId, undo)}`),
  info: (workspaceId: string, cid: string, mid: string) =>
    api.get<MessageInfo>(`/support/inbox/conversations/${cid}/messages/${mid}${qs(workspaceId)}`),
};
```

> Path style and local `qs(workspaceId)` helper match `frontend/src/lib/services/supportService.ts:39` and `:104`. Mutation hooks that wrap this service should pull the active workspace ID from `useWorkspaceStore()` rather than the caller threading it through every call site.

> Server-side response key is `markdown` for ergonomics, even though the underlying field is `Content`. The handler aliases on the wire so the frontend doesn't need to follow the column rename if/when one happens.

- [ ] **Step 3: TanStack mutation + query**

```ts
export function useDeleteMessage(conversationId: string) { /* mutationFn + onMutate optimistic remove + onError rollback */ }
export function useMessageInfo(conversationId: string, messageId: string | null) { /* useQuery, enabled when messageId != null */ }
```

- [ ] **Step 4: Vitest unit test for the service** (tiny, mock `api`)

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(support): types, service, hooks for message actions"
```

---

## Phase 6: Frontend UX — actions menu, dialogs, footer, edit mode, shortcut

### Task 6.1: `MessageActionsMenu` component

**Files:**
- Create: `frontend/src/components/support/MessageActionsMenu.tsx`
- Create: `frontend/src/components/support/__tests__/MessageActionsMenu.test.tsx`

- [ ] **Step 1: Implement component**

Use shadcn `Popover` + `Button` (icon variant) with the 3-dots `MoreVertical01Icon`. Items conditional on `cancellable_until > now`:

```tsx
{canEdit && <Item onSelect={onEdit}>Edit</Item>}
<Item onSelect={onCopy}>Copy</Item>
<Item onSelect={onQuoteReply}>Reply</Item>
<Separator />
<Item onSelect={onDelete} variant="destructive">Delete</Item>
<Item onSelect={onInfo}>Info</Item>
```

- [ ] **Step 2: Tests** (Vitest + Testing Library)

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(support): hover actions menu for messages"
```

---

### Task 6.2: Delete confirmation dialog

**Files:**
- Create: `frontend/src/components/support/MessageDeleteDialog.tsx`

- [ ] **Step 1: Build using shadcn `AlertDialog`**

Title: `Are you sure to remove this message?`
Description: `This message will be removed forever from our platform. If your message was already delivered via email or third party integrations, it might be impossible to remove it.`
Buttons: `Cancel` (ghost) / `Remove Message` (destructive, with `Delete02Icon`).

- [ ] **Step 2: Vitest test**

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(support): delete confirmation dialog"
```

---

### Task 6.3: Message info dialog

**Files:**
- Create: `frontend/src/components/support/MessageInfoDialog.tsx`

- [ ] **Step 1: Build using shadcn `Dialog`**

Two-column key/value layout matching the screenshot. Identifier row has a copy-to-clipboard button. Sections separated by `<Separator />`. Boolean fields render `Yes` / `No`. `Delivered: Yes (Email)` when `delivered.channel === 'email'`.

- [ ] **Step 2: Test**

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(support): message info dialog"
```

---

### Task 6.4: Wire menu into `MessageBubble`

**Files:**
- Modify: `frontend/src/components/support/MessageBubble.tsx`

- [ ] **Step 1: Add hover trigger + state**

Wrap the existing bubble in a `relative group` container. Add the `MessageActionsMenu` absolutely-positioned outside the bubble, hidden until `group-hover`. Anchor near the bottom for tall bubbles via a `useLayoutEffect` measuring `ref.current.offsetHeight` (threshold ≈ 5rem).

- [ ] **Step 2: Define a single `onUndo` handler reused by Edit, Undo, and the keyboard shortcut**

```tsx
const undoMutation = useDeleteMessage(conversationId);
const onUndo = useCallback(async () => {
  const res = await undoMutation.mutateAsync({ messageId: message.id, undo: true });
  if (res.markdown) setDraft(conversationId, res.markdown);  // existing draft store
  focusComposer();
}, [conversationId, message.id, undoMutation]);
```

- [ ] **Step 3: Wire menu actions**

```tsx
const onEdit = onUndo;                    // same flow as Undo footer
const onCopy = () => navigator.clipboard.writeText(message.content || '');
const onQuoteReply = () => insertQuote(message);
const onDelete = () => setDeletingId(message.id);   // opens AlertDialog (post-window remove)
const onInfo = () => setInfoId(message.id);
```

- [ ] **Step 4: Footer state for outbound messages**

Render order matters — extend the AI footer logic (we just refactored it) and the regular receipt logic with one new state:

```tsx
{cancellableActive ? (
  <button className="..." onClick={onUndo}>
    Sent · <span className="font-medium">Undo</span> · {formatCountdown(remaining)}
  </button>
) : isDeliveredViaEmail ? (
  <span>Delivered to email</span>
) : /* existing receipt rendering */}
```

`useCountdown` hook ticks every 1s; stops when `remaining <= 0`.

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(support): hover actions menu and undo footer in MessageBubble"
```

---

### Task 6.5: Cmd/Ctrl+Z undo shortcut

**Files:**
- Create: `frontend/src/hooks/useUndoLastSend.ts`
- Modify: a route component (likely `routes/_authenticated/w/$slug/support/...`) to mount the hook.

- [ ] **Step 1: Implement hook**

```tsx
export function useUndoLastSend(conversationId: string) {
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id ?? '');
  const messages = useConversationMessages(workspaceId, conversationId).data;
  const del = useDeleteMessage(conversationId);
  const setDraftRich = ...; // restore TipTap doc

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const meta = e.metaKey || e.ctrlKey;
      if (!meta || e.key.toLowerCase() !== 'z') return;
      const target = e.target as HTMLElement;
      if (target?.closest('[contenteditable], input, textarea')) return; // let TipTap own its undo
      const candidate = (messages ?? [])
        .filter((m) => m.cancellable_until && new Date(m.cancellable_until) > new Date())
        .at(-1);
      if (!candidate) return;
      e.preventDefault();
      del.mutate({ messageId: candidate.id, undo: true }, { onSuccess: ({ markdown }) => markdown && setDraftRich(markdown) });
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [messages, del]);
}
```

- [ ] **Step 2: Test (Vitest + jsdom)**

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(support): Cmd/Ctrl+Z undoes last cancellable reply"
```

---

### Task 6.6: WS event handlers

**Files:**
- Modify: `frontend/src/hooks/useRealtimeSync.ts` (or wherever support events dispatch)

- [ ] **Step 1: Extend the existing `support_conversation_message` branch with `Action === 'deleted'`**

The current handler already routes `event.entity === 'support_conversation_message'` and invalidates support queries whose keys match the workspace/conversation in `frontend/src/hooks/useRealtimeSync.ts`. Keep that existing branch and ensure `event.action === 'deleted'` follows the same invalidation path as `created` (plus the generic DOM custom event dispatch that already emits `${event.entity}-${event.action}`).

- [ ] **Step 2: Commit**

```bash
git commit -m "feat(support): real-time sync for support_conversation_message:deleted"
```

---

## Phase 7: End-to-end verification

### Task 7.1: Manual verification (with dev server)

- [ ] **Step 1: Boot the stack**

```bash
cd /root/teampulse && pnpm dev   # frontend
cd /root/teampulse/server && go run ./cmd/api  # backend (separate terminal)
```

- [ ] **Step 2: Reproduce each scenario**

| Scenario | Expected |
|----------|----------|
> All "within window" rows below assume the workspace's `EmailFallbackDelaySecs` (default 120s, configurable 30–600s). Adjust expected countdown to match.

| Send a reply → bubble shows `Sent · Undo · M:SS` | countdown matches workspace `EmailFallbackDelaySecs` and ticks every 1s |
| Click Undo within the window | message disappears, content lands in composer, focus returns to editor |
| Hover bubble within the window → 3 dots → Edit | same flow as Undo (Edit ≡ Undo + restore-to-composer) |
| Press Cmd/Ctrl+Z within the window while focus is outside the editor | same flow as Undo |
| Resend after edit | new bubble, new cancellable window, **no `Edited` tag in v1** (deferred — see Data model) |
| Wait past the window | footer flips to `Delivered to email` (or `Read in chat`); Edit/Undo no longer present |
| Click Delete on an out-of-window message → confirm | confirmation dialog appears with the warning copy; on confirm, message disappears (soft delete) |
| Click Info on any message | dialog matches the screenshot fields verbatim; `Edited` is always `No` in v1 |
| Two browser tabs on the same conversation | undo/delete propagates via the new `support_conversation_message:deleted` event |
| Multi-message batch: send three replies in quick succession, undo only the second | first and third still email at their original `fireAt`; second never emails; conversation entry stays in the sorted set as long as any messages remain in `email_fallback_msgs:{convID}` |

- [ ] **Step 3: Note any bugs in `docs/superpowers/plans/2026-04-30-support-message-actions-followups.md`**

- [ ] **Step 4: Final commit/push**

```bash
git push origin waqar-work
```

---

## Open questions / risks

1. **Content restored into the existing draft.** Undo writes the deleted message's `Content` (the only stored body field; there is no `body_markdown` column) into the conversation draft. The composer already supports rehydrating drafts (verify by grepping the existing draft load path before Phase 6 starts). Worst case: fall back to plain text with a one-line warning toast. Either way, no new helper file is required.
2. **Concurrent undo from two agent tabs.** Last-write-wins is fine: the second tab's DELETE returns 404 (already soft-deleted), the WS event syncs both views.
3. **AI-authored messages.** AI replies have `sender_type = 'ai'` and a non-null `sender_agent_id` (with `sender_user_id` NULL). The ownership predicate `sender_type = 'user' AND sender_user_id = ?` returns 0 rows for any human actor, which surfaces as `ErrForbidden` (HTTP 403). The actions menu hides Edit/Undo for any message where `sender_type !== 'user'` or where `sender_user_id !== currentUserId`. A future "rewrite as me" flow is its own feature.
4. **Visitor widget UX.** Out of scope here; we publish events but the widget's display logic lives in `packages/widget-core`. Follow-up plan should consume the `support_conversation_message:deleted` event to remove bubbles on the customer side.

---

## Done criteria

- [ ] All steps in phases 1–7 checked off.
- [ ] `go test ./...` and `cd frontend && pnpm test` green.
- [ ] `npx tsc -b` green.
- [ ] Manual verification matrix above passes.
- [ ] Plan doc updated with any deltas encountered during execution.
