# PRD: Support Agent Email Notifications for Unread Customer Replies

**Status:** Draft
**Date:** 2026-03-24
**Author:** Engineering
**Module:** Support + Notifications
**Inspiration:** Crisp, Intercom

## 1. Problem Statement

Today, Helpin sends fallback email to the customer when the customer is offline, but it does not send a teammate notification email when the customer sends a new message and no one on the support team notices it in time.

That leaves a gap:
- support conversations can sit unread when no agent is actively watching the inbox
- assigned agents can miss customer follow-ups outside working focus windows
- the customer may receive a fast fallback email from us, but the team still has no guaranteed nudge to respond

This is a different problem from visitor email fallback.

- **Visitor email fallback** answers: "How do we reach the customer when they are offline?"
- **Agent notification email** answers: "How do we alert the team when the customer replied and the team has not read it?"

We need the second behavior.

## 2. External Product Reference

### Crisp

Crisp sends email notifications for pending unread visitor messages after a **3 minute delay**. If the message is read before the delay expires, the notification email is not sent. Crisp routes the email to a single operator by priority: assigned operator first, then an available operator, then an offline operator.

### Intercom

Intercom also uses a **3 minute delay** for teammate email notifications for customer replies. Each teammate controls their own notification preferences, and Intercom suppresses the email if the teammate has already seen the reply in the inbox before the delay expires.

### Product takeaway

The common pattern is:
- notify teammates about **unread customer replies**
- wait **3 minutes**
- suppress the email if the reply gets read in-app
- send to a **specific responsible teammate**, not to everyone

Helpin should follow that pattern.

## 3. Goals

- Ensure customer replies do not sit unread when agents are away from the inbox
- Match the Crisp/Intercom behavior closely enough that the feature is familiar
- Reuse the existing Helpin notification and email infrastructure instead of building a parallel system
- Keep notification noise low through delayed delivery, suppression rules, and single-recipient routing
- Respect per-user notification preferences and do-not-disturb controls

## 4. Non-Goals

- Sending agent notification emails in real time with no delay
- Emailing the whole workspace or whole support team by default
- Letting agents reply to the customer directly from the notification email in v1
- Replacing in-app inbox notifications, unread badges, or websocket updates
- Building escalation chains or multi-reminder cadences in v1
- Exposing the delay as a workspace setting

## 5. User Stories

**Agent**
- As an assigned agent, when a customer replies and I have not seen it, I receive an email reminder after a short delay.
- As an agent, if I already read the conversation in Helpin, I do not get a redundant email.
- As an agent, if I disabled support email notifications for myself, I do not receive them.

**Support lead / admin**
- As a support lead, unread customer replies still reach one teammate even if the conversation is unassigned.
- As an admin, I can enable or disable this feature at the workspace level.

**Customer**
- As a customer, my reply should get the team’s attention even when no one is watching the inbox live.

## 6. Product Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Is this the same feature as visitor email fallback? | No | Different trigger, recipient, copy, and success criteria |
| Delay | Fixed at **180 seconds** | Matches Crisp and Intercom; no admin mental overhead |
| Recipient count | **One teammate per notification cycle** | Avoids blasting the whole team |
| Trigger | New unread **customer** reply only | Mirrors competitor behavior |
| Suppression | Do not send if read before timer fires | Core noise-control behavior |
| User preference model | Reuse existing notification preferences | Avoid a separate preferences system |
| Queueing | Reuse existing notification delivery infrastructure | Avoid another Redis outbox for a notifications problem |

## 7. Scope

### In scope

- Customer replies from widget chat
- Customer replies arriving through Postmark inbound email
- Assigned and unassigned support conversations
- Workspace-level master switch
- Per-user email notification preference
- 3-minute delayed unread check
- Email delivery audit through existing notification delivery records

### Out of scope

- SMS, push, Slack, or mobile notifications
- Multiple reminder rounds
- Escalation to managers if the first agent does not respond
- Agent replying from the notification email
- AI-specific routing differences

## 8. Core Behavior

### 8.1 Trigger

Create a delayed agent notification candidate when all of these are true:
- a new `support_message` is created
- `sender_type = customer`
- `is_internal = false`
- `message_type = reply` or empty/default reply
- the conversation is not terminal (`closed`, `spam`)
- the workspace-level feature is enabled

This applies to:
- live widget replies
- inbound email replies routed into the same support conversation

### 8.2 Delay

The system waits **180 seconds** from the customer message creation time before deciding whether to send the agent notification email.

### 8.3 Suppression rules

Do **not** send the notification email if, before the 180-second timer expires:
- a teammate reads the conversation in Helpin
- a teammate sends a public reply
- the conversation is closed or marked spam
- the customer message was already covered by a prior notification cycle

For read detection, use:
- `support_conversations.team_last_seen_at`

For teammate response detection, use:
- any later non-internal `support_message` with `sender_type != customer`

### 8.4 One notification cycle per unread burst

If the customer sends multiple messages before a teammate reads the conversation, Helpin should send **one** teammate notification email for that unread burst, not one email per message.

The email should summarize:
- customer name/email when available
- conversation display ID
- latest unread message preview
- count of unread customer messages in the burst
- direct CTA to open the conversation in Helpin

The unread burst ends when:
- a teammate reads the conversation, or
- a teammate sends a reply

## 9. Recipient Selection

Helpin should follow Crisp’s "single operator by priority" model.

### 9.1 Selection order

When the timer fires, choose **one** recipient in this order:

1. `AssignedAgentID`, if set and eligible
2. The most recent human teammate who publicly replied in the conversation, if eligible
3. An eligible teammate currently active in support
4. Any other eligible teammate in the workspace

### 9.2 Eligibility rules

A teammate is eligible only if all are true:
- has workspace access to support
- has a valid email address
- has not disabled email notifications globally
- has not disabled support customer reply emails specifically
- is not currently in do-not-disturb

### 9.3 "Currently active in support"

For v1, define active support teammates as users with recent support inbox presence within the last 5 minutes, using existing websocket/presence signals where available.

If we cannot reliably compute that in a first pass, fallback order for implementation v1 becomes:

1. assigned agent
2. most recent replying teammate
3. round-robin among eligible teammates

That fallback is acceptable as long as the product behavior still sends to one recipient only.

### 9.4 Tie-breaker

When multiple teammates are eligible in the same bucket, choose the teammate least recently notified for `support.customer_reply.email`.

This prevents one person from getting every unassigned conversation email.

## 10. Settings and Preferences

### 10.1 Workspace-level setting

Extend `SupportInboxSettings` with:

```go
SupportAgentEmailNotificationsEnabled bool `json:"support_agent_email_notifications_enabled"`
```

Behavior:
- master switch for the feature
- default `false` for existing workspaces at rollout
- default `true` for newly created workspaces after rollout confidence is established

### 10.2 Per-user preference

Reuse existing notification preference infrastructure.

Add a support-specific preference key in `notification_preferences.channel_preferences`, for example:

```json
{
  "support.customer_reply.email": true
}
```

This preference is evaluated together with:
- `user_notification_settings.email_enabled`
- workspace `notification_preferences.email_enabled`
- workspace/user DND

### 10.3 No workspace delay setting

Do not expose a delay control in support settings.

The delay is a product choice, not an admin tuning knob.

## 11. Notification Event Model

Reuse the existing notifications system.

### 11.1 New event type

Add:
- `support.customer_reply`

Suggested category:
- `comments` or a new support-specific category `support_replies`

Suggested priority:
- `high`

### 11.2 Entity model

Use:
- `entity_type = "support_conversation"`
- `entity_id = conversation_id`

This keeps the notification inbox and delivery history aligned with the support conversation.

### 11.3 Metadata

Notification metadata should include:
- `conversation_id`
- `conversation_display_id`
- `support_message_id`
- `customer_name`
- `customer_email`
- `latest_message_preview`
- `unread_customer_message_count`
- `assigned_agent_id`

## 12. Architecture

### 12.1 Event emission

When a customer message is created in support:
- emit `support.customer_reply`
- create/update the in-app notification row
- enqueue a delayed email-delivery check for `now + 180s`

The event should be emitted from the support message creation path, not from the email fallback service.

### 12.2 Delivery queue

Reuse the existing async notification delivery system rather than building a second Redis poller.

Recommended approach:
- create a dedicated delayed job type, eg `support_customer_reply_email_job`
- key jobs by `(workspace_id, conversation_id, unread_burst_start_message_id)`
- ensure only one pending job exists per unread burst

### 12.3 Worker behavior

When the delayed job runs:

1. Load the conversation and unread burst state
2. Confirm the feature is still enabled
3. Confirm the unread burst is still unread by the team
4. Resolve one eligible recipient using the priority rules
5. Re-check preferences and DND on that recipient
6. Render and send the email
7. Mark the delivery in `notification_deliveries`

### 12.4 Idempotency

The worker must be safe to retry.

Use DB-backed deduplication so a retry cannot send duplicate teammate emails for the same unread burst. The canonical dedup key should be:

`support_customer_reply_email:{conversation_id}:{burst_start_message_id}`

## 13. Data Model

### 13.1 No new standalone support-email table required in v1

Use existing notification tables as the system of record:
- `notifications`
- `notification_events`
- `notification_deliveries`

This feature is fundamentally a notification workflow, so it should live in the notifications domain unless we discover a support-specific auditing need that those tables cannot cover.

### 13.2 Support settings

Add to `SupportInboxSettings`:

```go
SupportAgentEmailNotificationsEnabled bool `json:"support_agent_email_notifications_enabled"`
```

### 13.3 Optional future cursor

If round-robin fairness needs durable state, add later:

```go
type SupportNotificationRecipientCursor struct {
    WorkspaceID string
    UserID      string
    LastNotifiedAt time.Time
}
```

This is not required for v1 if delivery history is sufficient for selection.

## 14. Email UX

### 14.1 Subject

Examples:
- `New customer reply in support conversation #c376`
- `Muhammad Azhar replied in support conversation #c376`

### 14.2 Body

Keep the email operational and plain, similar to Crisp/Intercom teammate notifications:
- customer name/email
- one short summary line
- latest unread message preview
- optional count of additional unread messages
- primary CTA button or link: `Open conversation`

### 14.3 Reply handling

Do not support agent-to-customer reply directly from this notification email in v1.

The email should route the teammate back into Helpin, where assignment, context, AI notes, and history are available.

## 15. Frontend Requirements

### 15.1 Settings UI

Add a workspace-level toggle in support settings:
- `Email agents when customer replies go unread for 3 minutes`

Add a per-user notification preference control in the notifications/personal settings UI:
- `Support customer replies`
- channel: email

### 15.2 Inbox UI

No special inbox badge is required in v1 beyond existing unread state and notifications.

Optional future enhancement:
- show who was emailed for the unread conversation

## 16. Edge Cases

- **Assigned agent disabled email notifications:** fallback to the next eligible teammate
- **Assigned agent is in DND:** fallback to the next eligible teammate
- **No eligible teammates:** no email sent, but keep in-app unread state
- **Conversation read at 179 seconds:** no email should be sent
- **Customer sends 3 replies in 2 minutes:** send one email after 3 minutes summarizing the unread burst
- **Agent reads but does not reply:** suppress email, because the goal is awareness
- **Agent replies without opening thread first:** suppress email, because the conversation is no longer unattended
- **Inbound email reply from customer:** handled exactly like widget customer reply
- **Internal note added by teammate:** must not trigger this notification

## 17. Analytics and Observability

Track:
- unread customer reply events created
- delayed jobs scheduled
- emails suppressed due to read-before-delay
- emails suppressed due to teammate reply-before-delay
- emails sent
- emails skipped due to no eligible recipient
- recipient selection bucket used (`assigned`, `recent_replier`, `active_support`, `fallback`)

Add structured logs with:
- `workspace_id`
- `conversation_id`
- `support_message_id`
- `notification_event_id`
- `recipient_user_id`
- `selection_bucket`
- `suppression_reason`

Do not log customer message bodies.

## 18. Rollout Plan

### Phase 1

- backend event + delayed delivery worker
- workspace-level master switch
- no UI for per-user preference yet; assume default-on if global email notifications are enabled
- internal validation only

### Phase 2

- per-user notification preference UI
- recipient fairness improvements
- metrics dashboard

### Phase 3

- default-on for new workspaces
- optional admin visibility into who was notified

## 19. Acceptance Criteria

- A new customer reply schedules a delayed teammate email-notification check
- If a teammate reads the conversation within 3 minutes, no email is sent
- If a teammate replies within 3 minutes, no email is sent
- If the conversation remains unread after 3 minutes, exactly one eligible teammate receives an email
- Assigned agent is preferred when eligible
- Unassigned conversations route to one fallback teammate, not the whole team
- Multiple customer replies within one unread burst produce one email notification
- Per-user email notification preference is respected
- Global email disable and DND are respected
- Notification delivery is idempotent across retries

## 20. Open Questions

- Do we already have a reliable cross-tab/cross-device notion of "active in support" for teammate selection, or do we need to build it?
- Should resolved conversations still trigger teammate unread email notifications if a customer replies after resolution, or should they auto-reopen first?
- Should support leads be able to opt into being a fallback recipient pool only, without participating in all support routing?

## 21. References

- Crisp: How do email notifications work?
  https://help.crisp.chat/en/article/how-do-email-notifications-work-q3gel4/
- Intercom: How teammates get notifications
  https://www.intercom.com/help/en/articles/187-how-teammates-get-notifications
- Intercom: Never miss a conversation or sign up
  https://www.intercom.com/help/en/articles/448-never-miss-a-conversation-or-sign-up
