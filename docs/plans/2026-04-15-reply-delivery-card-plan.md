# Reply delivery card plan

> Historical proposal, source-compared on 2026-09-17. This page describes a
> proposed first-message confirmation card for widget contributors. It is not
> implemented in the current codebase.

## Current status and implementation considerations

Current source contains no `reply_delivery_info` event, `delivery_card` payload,
`ReplyDeliveryCard` component, or proposed emitter. The
[event registry](../../server/internal/model/support_system_event.go) does not
recognize this type, and the [repository guard](../../server/internal/repository/support_inbox.go)
rejects system messages with an unrecognized event type. The tests listed below
are proposed coverage, not existing test results.

The adjacent [delayed team reply service](../../server/internal/service/support_delayed_team_reply.go)
implements a different behavior: a notice after an AI-to-team handoff remains
unanswered for the configured delay. It applies to open, escalated widget
conversations, records a marker for the escalation, and uses a transaction and
row lock. A teammate reply after escalation suppresses the notice. Email capture
and email-related copy depend on configured fallback services and unsubscribe
state. None of this provides an immediate first-message confirmation card.

The [widget message renderer](../../packages/widget-core/src/components/MessageBubble.tsx)
currently permits explicit public system events only for `teammate_joined` and
`delayed_team_reply`. A new card would need both event registration and deliberate
rendering/visibility changes. The [staff renderer](../../frontend/src/components/support/MessageBubble.tsx)
also has event-specific behavior; a one-line hide/show promise is not an adequate
implementation contract.

Before implementing this proposal, resolve its conflicting timing descriptions:
the Timing section places the emitter after the customer message is saved, while
the helper comment places it before. A prior-message scan alone is not a
concurrency-safe exactly-once guarantee. Specify a transactional marker or another
atomic deduplication mechanism and test simultaneous first messages. Also make
email promises conditional on actual delivery configuration, following the existing
delayed notice's distinction between widget-only and email-capable installations.

Competitor behavior and rollout observations below are historical context, not
verified current behavior. No runtime tests or live conversations were used for
this source review.

## Original proposal


Ship a persistence-backed "reply delivery" card that
reassures the customer right after they send their first message on a
new conversation. Card confirms (1) where replies will land (widget +
email), (2) the email on file, (3) the expected reply time.

## Problem
When a customer sends their first message today, the widget just shows
the bubble — no feedback about delivery mechanism or timing. Two
predictable anxieties kick in:

1. *"Did my message actually go through?"* — there's no confirmation the
   email on file was captured or that the system knows how to reach them
   outside the widget.
2. *"When should I expect a reply?"* — the header subtitle shows the
   reply time, but it's easy to miss and doesn't speak to *this*
   message in particular.

Intercom drops a multi-line card immediately after the first customer
message that addresses both. Reloading the conversation re-surfaces the
card, so it's persisted, not a client-side nudge.

## End State
A new `message_type='system'` row with
`system_event_type='reply_delivery_info'` is written on the first
customer message of a conversation. The row carries a snapshot of the
email + reply-time copy at send time. The widget renders it as a
distinct muted card (not a chat bubble). The admin thread hides it by
default but can be flipped on with a one-line change.

Snapshot-at-send means old conversations faithfully show what was
promised at the time — changing the workspace reply-time preset later
does not rewrite history.

## Timing
**Fire immediately on the first customer message per conversation.**

- No delay. The card is a *delivery confirmation*, not a "sorry for the
  wait" card — the primary function is confirming the email + setting
  the reply-time expectation, both of which are useful regardless of
  how fast the reply eventually comes.
- Reuses the same "first time in this conversation" detection pattern
  we already built for `teammate_joined`, just on the customer side.
  Service queries prior messages and emits only if this is the customer's
  first non-internal message on the conversation.
- Fires *before* any AI or human reply — the emitter runs synchronously
  inside `CreateConversationMessage` right after the customer's message
  is persisted, so the card lands ahead of the AI first-reply path.

Why not a delay:
- Intercom fires immediately; matching reduces surprise.
- A debounce introduces a race with fast AI replies and a "did the card
  fire?" bookkeeping burden.
- The card value is *informational*, not contingent on slow response.

## New taxonomy entry
Add to `server/internal/model/support_system_event.go`:

```go
const SystemEventReplyDeliveryInfo SupportSystemEventType = "reply_delivery_info"
```

Add to `allSupportSystemEventTypes`, mirror in `packages/shared/src/types/widget-config.ts`
`SYSTEM_EVENT_TYPES` tuple. The repo-level guardrail
(`SupportMessageRepository.Create`) automatically rejects any write that
forgets to set it.

## Emitter
New helper in `server/internal/service/support_inbox.go` parallel to
`emitTeammateJoinedIfFirstReply`:

```go
// emitReplyDeliveryInfoIfFirstCustomerMessage emits a widget-visible
// system card right before persisting a customer's first non-internal
// message on a conversation. Snapshots the email, reply-time text, and
// preset at send time so the card reads correctly on reload.
func (s *SupportInboxService) emitReplyDeliveryInfoIfFirstCustomerMessage(
    ctx context.Context,
    workspaceID, conversationID string,
    customerEmail string,
    availability *model.WidgetConfigAvailability,
)
```

Called from `CreateConversationMessage` when:
- `senderType == "customer"`
- `!req.IsInternal`
- `messageType == "reply"`
- the prior-messages scan finds no prior non-internal customer reply on
  this conversation

Behavior:
- Idempotent via the prior-scan: re-sending a customer message never
  re-emits.
- Handles anonymous customer: if `customerEmail == ""`, the card omits
  the email line and keeps only the reply-time line.
- Handles offline-hours: if `availability.IsOnline == false`, uses
  `availability.OutsideHoursMessage` + `NextOnlineAt` copy instead of
  `ReplyTimeText`.

## Message shape

`content` (for admin / legacy consumers / fallback rendering):

> `You'll get replies here and in your email: visitor@example.com. Usually replies in a few minutes.`

Variants:
- Identified online: `You'll get replies here and in your email: {email}. {replyTimeText}.`
- Identified offline: `You'll get replies here and in your email: {email}. {outsideHoursMessage}.`
- Anonymous online: `You'll get replies here. {replyTimeText}.`
- Anonymous offline: `You'll get replies here. {outsideHoursMessage}.`

`metadata` (JSON, authoritative for widget rendering):

```json
{
  "delivery_card": {
    "email": "visitor@example.com",
    "reply_time_text": "Usually replies in a few minutes",
    "reply_time_preset": "few_minutes",
    "reply_time_minutes": null,
    "offline": false,
    "next_online_at": null,
    "outside_hours_message": null
  }
}
```

Widget prefers metadata when present; falls back to parsing `content`
if the metadata is missing or malformed (defense-in-depth for old
rows backfilled by content match).

## Widget rendering
New `packages/widget-core/src/components/ReplyDeliveryCard.tsx`:

- Dispatched from `MessageBubble` when
  `role === 'system' && systemEventType === 'reply_delivery_info'`,
  short-circuits before the default system-pill branch.
- Renders a muted gray rounded card:
  - Line 1: "You'll get replies here and in your email:" *(hidden for
    anonymous; line 1 becomes just "You'll get replies here.")*
  - Line 2: bold email *(hidden for anonymous)*
  - Line 3: "Our usual reply time"
  - Line 4: clock icon + reply-time-text *(or `outside_hours_message`
    + optional "Back {day} {time}" if `offline`)*
- CSS class family: `.helpin-reply-delivery-card*` in `widget.css`;
  dark-mode variant under `.helpin-theme-dark`.
- Occupies full message-row width, left-aligned, distinct from the
  centered system pill so the two never look confusable.

## Admin rendering
Default: **hidden**. `frontend/src/components/support/MessageBubble.tsx`
returns `null` when `message.system_event_type === 'reply_delivery_info'`
so teammates don't see "customer was told X" on every conversation.

If later feedback says teammates want visibility, flipping one branch
in the dispatcher swaps the hide for a muted routing-pill render
("Sent reply-time card to customer") — matches `mailbox_moved`.

## Tests

### Backend

- Table-driven `TestEmitReplyDeliveryInfo_FirstCustomerMessage` covers
  the four content variants (identified × online, identified × offline,
  anonymous × online, anonymous × offline) and verifies `content`,
  `system_event_type`, and structured `metadata.delivery_card`.
- `TestEmitReplyDeliveryInfo_IdempotentOnSecondCustomerMessage` sends
  two customer messages on the same conversation, asserts exactly one
  `reply_delivery_info` row exists.
- `TestEmitReplyDeliveryInfo_ExcludedFromRepoGuardrail` — implicitly
  covered by the existing invariant test (every new emitter has to
  pass that).
- `TestSystemEventTypeConstantsAreValid` picks up the new constant
  automatically via the enumerated list.

### Widget
- `ReplyDeliveryCard.test.tsx` — four render variants (identified ×
  online/offline, anonymous × online/offline), metadata-preferred
  rendering, fallback to `content` when metadata missing.
- `MessageBubble` integration test: a message with
  `role='system' && systemEventType='reply_delivery_info'` routes to
  the card and bypasses the centered system pill.

### Admin
- `MessageBubble.test.tsx` asserts `reply_delivery_info` messages
  render as `null` (hidden-by-default policy).

## Rollout
1. Backend migration-free change — all additive. Ship backend.
2. Widget rebuild + admin hide branch.
3. Smoke-test a few conversations on stage.
4. After a week, collect teammate feedback on the hidden-in-admin
   default; optionally flip the admin dispatcher to render muted pill.

## Open questions
- **Frequency**: only emit once per conversation, never per session.
  Agreed — document this behavior in the comment on the emitter.
- **Reload vs live**: the message flows over WS on creation and is
  fetched via `ListConversationMessages` on reload — both paths
  already go through `SupportMessageEvent` and the widget's normal
  message array, so no extra plumbing. Confirmed by tracing the
  `teammate_joined` pattern we shipped last week.
- **Admin visibility**: hidden by default via dispatcher, one-line
  flip to show later. No migration needed either way.
- **Email formatting**: we render `{email}` verbatim without
  auto-linking — safer than sending customers to a `mailto:` they
  didn't expect. Use bold plain text.
