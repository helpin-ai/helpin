# PRD — Reply Delivery Card

**Date:** 2026-04-15
**Status:** Draft, pending scope confirmation
**Related plans:** `docs/plans/2026-04-15-reply-delivery-card-plan.md` (implementation)

## TL;DR
After a customer sends their first message on a new support conversation, and no teammate or AI has replied within a short window, the widget drops a persisted, muted-gray card that confirms where replies will land (widget + email), the email on file, and the expected reply time. The card survives reload. If a reply lands fast enough, the card never appears — so it only surfaces when the customer is actually waiting.

## Background
The support widget today gives a customer no explicit confirmation that their first message was captured or how quickly to expect a reply. They see their own bubble, then nothing. Two predictable anxieties kick in:

1. *Did my message actually go through?* — no visible acknowledgment the email on file was captured or that the support system knows how to reach them outside the widget.
2. *When should I expect a reply?* — the reply-time preset is shown in the conversation header subtitle, but it's easy to miss and doesn't anchor to *this specific* message.

Both anxieties are currently resolved only by the AI replying quickly or a teammate joining fast. When neither happens, the customer sits in silence. Intercom addresses this with a delayed delivery card and has, by inference from their funnel, validated it as a reassurance surface.

## Goals
- Confirm delivery mechanism (widget + email) when a customer has sent a first message and is now waiting.
- Restate the expected reply time inline with the customer's message, anchored to *this* conversation moment.
- Persist the card so reloading the widget does not lose the reassurance.
- Hide the card when a teammate or AI replies before it fires — no redundant noise for fast-response cases.

## Non-goals
- Not a "sorry for the wait" banner. The card is informational, not apologetic.
- Not an SLA guarantee surface. Existing reply-time presets remain the source of truth; the card restates them, doesn't promise them.
- Not a retention mechanic, email capture prompt, or marketing surface. It confirms information the customer has already provided.
- Not a customer-visible admin action log. Teammate-side the card is hidden by default.

## User stories

**Customer with identified email, waiting on a reply**
> I just sent "Our licenses were terminated." I see my bubble, then after a short pause a gray card appears showing that replies will land here and at my email, and that the team usually replies in a few minutes. I feel acknowledged.

**Anonymous customer, waiting**
> I sent a question without providing email. After a short pause a gray card appears showing the expected reply time. No email line (I haven't given one).

**Customer whose message was answered fast by the AI**
> I sent "How do I reset my password?" and the AI replied in 3 seconds. I see my message and the reply. No delivery card — it would have been noise.

**Customer who reloads the page mid-wait**
> I sent a message 5 minutes ago. The card fired at the 15-second mark and has been in the thread since. I reload — the card is still there, along with my message.

**Teammate reading the conversation in admin inbox**
> I open a conversation in the support inbox. I see the customer's message and any replies. I do NOT see the delivery card — it's an artifact of customer-side reassurance, not conversation content I need to track.

## Functional requirements

### When the card fires
- Exactly **once per conversation**, on the customer's first non-internal `reply`-typed message.
- After a **15-second delay** (v1 default, configurable later).
- Only if no non-internal `reply`-typed message from any non-customer sender (agent, ai, user) has landed on the conversation before the delay elapses.
- Even if the customer sends a second or third message during the delay window, only one card ever fires.

### What the card says

| Customer state         | Business hours? | Copy |
|------------------------|-----------------|------|
| Identified w/ email    | In-hours        | `You'll get replies here and in your email:` / **email** / `Our usual reply time` / ⏱ replyTimeText |
| Identified w/ email    | Out-of-hours    | `You'll get replies here and in your email:` / **email** / outsideHoursMessage / optional "Back {day} {time}" |
| Anonymous              | In-hours        | `You'll get replies here.` / `Our usual reply time` / ⏱ replyTimeText |
| Anonymous              | Out-of-hours    | `You'll get replies here.` / outsideHoursMessage / optional "Back {day} {time}" |

Content is a **snapshot at fire time** — reply-time setting changes later don't rewrite historic cards.

### Where the card renders

- **Widget (customer-visible):** muted gray rounded card, full message-row width, left-aligned, distinct from chat bubbles and from the centered `teammate_joined` system pill. Dark-mode variant applies.
- **Admin inbox:** hidden by default. Teammates don't see the card in the thread. One-line dispatcher flip renders it as a muted routing pill if feedback changes later.

### Persistence
- Stored as a `message_type='system'`, `system_event_type='reply_delivery_info'` row in `support_messages`.
- `is_internal=false` so the widget fetches it on reload via `ListConversationMessages`.
- Structured payload in `metadata.delivery_card` is authoritative for widget rendering; `content` is the human-readable fallback for admin / legacy consumers.

### Interaction model
- Not dismissible. The card scrolls up the thread naturally as new messages arrive.
- Not interactive — no links, buttons, or follow-up prompts. Plain informational content.

## UX mockups

**Card — identified, in-hours**

```
┌────────────────────────────────────────────────┐
│  Hi there                              (you) → │
│                                                │
│  ┌──────────────────────────────────────────┐  │
│  │  You'll get replies here and in your     │  │
│  │  email:                                  │  │
│  │  azhar@contentstudio.io                  │  │
│  │                                          │  │
│  │  Our usual reply time                    │  │
│  │  ⏱  Usually replies in a few minutes     │  │
│  └──────────────────────────────────────────┘  │
└────────────────────────────────────────────────┘
```

**Card — anonymous, out-of-hours**

```
┌────────────────────────────────────────────────┐
│  How do I cancel my plan?              (you) → │
│                                                │
│  ┌──────────────────────────────────────────┐  │
│  │  You'll get replies here.                │  │
│  │                                          │  │
│  │  We're out — we'll get back first thing  │  │
│  │  in the morning.                         │  │
│  │  ⏱  Back Tue 9:00 AM                     │  │
│  └──────────────────────────────────────────┘  │
└────────────────────────────────────────────────┘
```

**Card — not fired (fast AI reply)**

```
┌────────────────────────────────────────────────┐
│  How do I reset my password?           (you) → │
│                                                │
│  [H] Helpin AI                                 │
│      Here's how to reset…                      │
└────────────────────────────────────────────────┘
```
(no delivery card — AI answered within the delay window)

## Success metrics

### Primary
- **Customer anxiety proxy**: % of conversations where the customer sends a second message within 30s of the first *without* having received any reply. Target: drop by ≥20% post-launch.
- **Delivery awareness**: % of post-conversation CSAT responses that mention "I didn't know where the reply would go" / similar confusion signals. Target: trend toward zero.

### Secondary
- **Fire rate**: % of conversations where the card actually fires. Expected 30–60%; if >80%, the delay is too short (cards firing before AI can reply); if <10%, the delay is too long or replies are very fast.
- **Email reassurance**: anonymous-vs-identified conversion on providing email later in the conversation. Not a primary goal but a signal worth watching.

### Counter-metrics
- Widget bundle size delta: must be < 2 KB gzip for the new component + CSS.
- Admin-thread load time: must not regress — card is hidden in admin and shouldn't add render cost beyond the existing system-event dispatcher.

## Edge cases & failure modes

| Scenario | Expected behavior |
|----------|-------------------|
| Customer sends 5 messages in a row before the delay elapses | Still one card fires after 15s of silence from any non-customer sender. Subsequent customer messages do not reset the timer. |
| AI replies at 14.8s, card-emit at 15.0s | Acceptable race — activity recheck sees the AI reply, exits silently. No double-card. |
| AI replies at 15.1s | Card fires at 15.0s first, AI reply lands after. Ordering in the thread reflects actual timestamps. |
| Customer's conversation is closed / archived during the 15s window | Workflow cancelled via signal; card not emitted. |
| Server restart between customer's first message and the 15s timer | Temporal workflow is durable — survives restart, fires at the originally scheduled time. |
| Reply-time preset is changed 30 minutes after the card was written | Existing card stays with its original snapshot copy. New conversations use the new preset. |
| `force_visitor_identity` on, pre-chat form captures email after first customer message | Card fires only after the first real `reply`-typed message, which requires the form to have completed. Email is always present on the card in this flow. |
| Customer identifies later via SDK | No backfill — the card already fired with whatever state was known at fire time. Acceptable: the card is a moment-in-time artifact. |
| Mid-conversation `ai_escalated` system message (existing) lands in the delay window | Counts as a "non-customer reply" for the purpose of the silence check — the card is suppressed because the customer has already seen *something* from the system side. |

## Open questions
1. **Delay value.** 15s is my default proposal. 10s, 30s, or configurable? Decision needed before build.
2. **Admin visibility.** Hidden by default. Teammate feedback after a week of live traffic decides whether to flip it to the muted routing-pill render.
3. **Scheduling primitive.** Temporal workflow (my recommendation — durable, cancellable, matches existing stack) vs. a cron-worker draining a pending-cards table (simpler infra, minute-granularity). Engineering call.
4. **Email rendering.** Plain bold text, no auto-`mailto:` link — consistent with Intercom and avoids sending customers to a dialer they didn't expect. Confirm.
5. **Offline-hours copy.** Reuse the existing `outsideHoursMessage` verbatim, or tweak for card context? Default: verbatim.

## Rollout plan
1. Ship backend (event type, emitter, Temporal workflow, tests) behind feature flag on staging.
2. Ship widget component + CSS + admin-hide branch.
3. Internal testing for 2 days on stage with varied reply times and configurations.
4. Gradual enable: 10% of workspaces for 3 days → 100%.
5. Week-4 review against success metrics.

## Not shipping in v1 (captured for future)
- Configurable delay per workspace (`reply_delivery_card_delay_seconds`). Pending usage data.
- Dynamic card content based on current queue depth ("5 people ahead of you"). Requires queue-depth infrastructure.
- Re-fire on subsequent customer messages after 10+ minutes of silence. Adds a "nag" feel; skip for v1.
- Custom admin copy ("Shared reply-time card at 15s with reply_time=few_minutes") beyond the single hidden/visible toggle.
- Tying the card to AI conversation-closure flows ("if AI can't answer, surface the card immediately instead of waiting"). Possible v2.
