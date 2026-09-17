# Internal handoff system events for inbox

**Date:** 2026-04-30
**Branch:** waqar-work
**Owner:** Waqar

## Problem

When the AI hands off to a human, the inbox currently shows the
customer-facing escalation reply ("Let me connect you with a team
member who can help further.") as the only signal that a handoff
occurred. Teammates need to see *why* and *how* the handoff happened,
not the verbatim copy that was sent to the customer.

Today, `EscalateToHuman` (`server/internal/service/support_ai.go:993+`)
creates a single `SupportMessage` that is doing two jobs at once:

- `MessageType = "system"` with `SystemEventType = SystemEventAIEscalated`
- `Content` = the customer-facing `escalation_message` setting
- Not `IsInternal`, so it is also visible to the customer

The escalation reason (`customer_requested` / `customer_requested_human`
vs. AI-driven handoff) is known at call time but is not preserved on
the message. We need an explicit, internal-only system event that
records the cause.

## Goals

1. Inbox shows a clear internal pill that explains the handoff cause:
   - "Customer requested a human" — for `customer_requested*` reasons
   - "AI handed off to a human" — for AI-driven reasons (low confidence,
     stuck, action unavailable, etc.)
2. Customer-facing escalation reply continues to be sent through the
   widget exactly as today (no change to wording, settings, dedupe).
3. The internal signal is queryable and persisted (not just a render-time
   string), so analytics/digest/automation rules can use it later.

## Non-goals

- No change to the `escalation_message` setting in `ChatGeneralTab.tsx`
  or its default copy.
- No new chat-widget UI; visibility of the new event is internal only.
- No backend rewrite of the escalation pipeline — only the message
  emission inside `EscalateToHuman` and its readers.

## Approach

Split the existing single-purpose system message into two
audience-specific records inside `EscalateToHuman`:

1. **Customer-facing reply** — same content as today. Stays as a normal
   AI reply (`MessageType = "reply"`, `SenderType = "ai"`,
   `IsInternal = false`). The widget renders it like any AI message.
2. **Internal system event** — `MessageType = "system"`,
   `IsInternal = true`, `SystemEventType` is one of two new values:
   - `SystemEventCustomerRequestedHuman = "customer_requested_human"`
   - `SystemEventAIHandoff = "ai_handoff"` (renamed from existing
     `ai_escalated` so the pair is symmetric, with a backward-compat
     read for the old constant — see Migration below)
   - `Content` is left empty; the renderer derives the human-readable
     label from `SystemEventType`.

Selecting between the two new event types comes from the existing
`reason` argument already passed to `EscalateToHuman`:
`customer_requested` / `customer_requested_human` → human-requested,
anything else → AI handoff.

## Detailed plan

### Backend

1. **Add new system event constants** in
   `server/internal/model/support_system_event.go`:

   ```go
   SystemEventCustomerRequestedHuman = "customer_requested_human"
   SystemEventAIHandoff              = "ai_handoff"
   ```

   Add both to `allSupportSystemEventTypes`. Keep
   `SystemEventAIEscalated` in the map for read-side compatibility.

2. **Update `EscalateToHuman`** at
   `server/internal/service/support_ai.go:993` (function around
   line 1014):

   - Replace the dual-purpose message with two messages:
     a. `replyMsg` — `MessageType = "reply"`, `SenderType = "ai"`,
        `Content = escalationContent`, `IsInternal = false` — the
        customer-facing copy.
     b. `handoffSystemMsg` — `MessageType = "system"`,
        `IsInternal = true`, `SystemEventType` chosen from the reason,
        `Content = ""`.
   - **Broadcast both** at the existing `wsPublisher` call site
     (`support_ai.go:1137`):
     - Publish `replyMsg` first — `SupportMessageEvent`
       (`websocket/support_events.go:18`) only attaches `Data` when
       `IsInternal = false`, so this is the event that delivers the
       payload to widget and admin clients.
     - Publish `handoffSystemMsg` second — same factory, but Data is
       deliberately stripped because it's internal; the inbox uses
       its own message-list refetch / store update to render the
       new system event.
     - Keep the existing `escalated` `support_conversation` event
       and `publishVisitorConversationsRefresh` calls.
   - **Dedupe must see internal events too.** Switch the dedupe
     history load to `ListByConversation(ctx, ws, conv, true)`
     (`includeInternal=true`) so `hasEscalationMessageInHistory` /
     `isEscalationSystemEvent` can spot the new internal handoff
     events. Update `isEscalationSystemEvent` to recognize the new
     constants AND the legacy `ai_escalated` so a redundant call after
     rollout (or against pre-rollout rows) still no-ops.
   - Preserve the existing settings/timestamps/transition logic
     untouched.

3. **Reason → event type mapping helper** in `support_ai.go`:

   ```go
   func systemEventForEscalationReason(reason string) string {
       if reason == "customer_requested" || reason == "customer_requested_human" {
           return model.SystemEventCustomerRequestedHuman
       }
       return model.SystemEventAIHandoff
   }
   ```

4. **Tests** (Go, in-memory SQLite, follow existing patterns in
   `support_ai_*_test.go`):

   - Reason `"customer_requested"` produces a `customer_requested_human`
     internal system event AND a customer-visible AI reply.
   - Reason `"low_confidence"` produces an `ai_handoff` internal system
     event AND the same AI reply.
   - Dedupe still prevents a second pair when called twice.
   - The internal system event has `IsInternal = true` so it is
     filtered out of widget-facing fetches.

5. **Widget filtering check** — confirm that
   `messageRepo.ListByConversation(... includeInternal: false)` (or
   equivalent widget endpoint) excludes `IsInternal = true` rows.
   Adjust if needed; we likely already filter, but verify.

### Frontend

1. **Type:** `SupportMessage.system_event_type` is typed by the literal
   union `SupportSystemEventType`, sourced from
   `SUPPORT_SYSTEM_EVENT_TYPES` in
   `frontend/src/lib/pm-types/support.ts:273`. Add
   `'customer_requested_human'` and `'ai_handoff'` to that array (keep
   `'ai_escalated'` for legacy rows). Without this, any code branching
   on the new values is a TypeScript compile error.

2. **`MessageBubble.tsx` system-message dispatch**
   (`frontend/src/components/support/MessageBubble.tsx:268+`):

   - Add `'customer_requested_human'`, `'ai_handoff'`, and the legacy
     `'ai_escalated'` to `routingEventTypes` so the existing routing
     pill style is reused.
   - Build a small label resolver:

     ```ts
     const HANDOFF_LABELS: Record<string, string> = {
       customer_requested_human: 'Customer requested a human',
       ai_handoff: 'AI handed off to a human',
       ai_escalated: 'AI handed off to a human', // legacy rows
     };
     ```

     and pick label from `HANDOFF_LABELS[eventType] ?? message.content`.

3. **UI treatment** — the routing-event pill is the right home for
   these. Existing styling (lines 318–340):

   ```
   centered, my-3 row,
   small avatar (h-5 w-5) | text-xs text-muted-foreground content,
   tooltip showing fullTimestamp.
   ```

   Two adjustments specific to handoff events:

   - **Icon, not avatar** — there is no single sender to attribute.
     Replace the avatar with a small icon: handoff arrow (e.g.
     `ArrowExchange01Icon` or `BotIcon → ArrowRight → User`) sized
     `h-3.5 w-3.5`, wrapped in a 5×5 rounded-full neutral background
     so it visually balances the existing avatar slot.
   - **Sublabel direction**:
     - For `customer_requested_human`: icon = raised-hand or user icon
       (`UserIcon`), text = "Customer requested a human"
     - For `ai_handoff` / `ai_escalated`: icon = bot/handoff
       (`BotIcon`), text = "AI handed off to a human"

   Tooltip stays the same shape as other routing events (timestamp
   only). Centered alignment is fine — matches the recent tooltip
   centering work.

4. **Activity rail / digest** — search for any other consumers of
   `SystemEventAIEscalated` (e.g. notification or activity feed) and
   update them to handle the new event types and the legacy alias.
   Likely candidates: `support_events.go`, support websocket payloads,
   any digest builder.

5. **Tests** — extend `MessageBubble`-area tests if any, or add a
   pure helper test for the label resolver if it is split into a small
   util. (No React-rendering tests in this codebase, so keep logic
   in a testable function.)

### Migration / Rollout

- No DB migration. `system_event_type` stays a free `string` column
  with the existing partial index.
- Historical rows with `ai_escalated` continue to render correctly via
  the legacy alias in `HANDOFF_LABELS`.
- The customer-facing escalation reply now lands in `support_messages`
  as a `reply` row instead of a `system` row. Backfill is **not
  required** — old rows still display as system pills with the
  legacy label, and only new escalations follow the new shape.

### Verification

- `cd server && go test ./internal/service -run "EscalateToHuman|Escalat"`
  passes, including the new reason → event-type cases.
- `cd frontend && npx tsc -b` clean.
- Manual:
  1. Force a customer "I want to talk to a person" handoff →
     inbox shows centered pill "Customer requested a human" plus
     the customer-facing AI reply on its own line; widget shows
     only the AI reply.
  2. Force a low-confidence AI handoff → inbox shows "AI handed off
     to a human" pill plus the AI reply.
  3. Reload an old conversation that was escalated before the change
     → still renders correctly (legacy alias).

## Out of scope / follow-ups

- Surfacing the same handoff distinction in the digest email and the
  support activity rail.
- Letting workspaces customize the inbox label (e.g. translate
  "Customer requested a human" to localized copy).
- Recording the AI's structured failure reason (low_confidence,
  policy_blocked, etc.) on the system event metadata for analytics.
  Today the reason is in `EscalateToHuman` arguments only.
